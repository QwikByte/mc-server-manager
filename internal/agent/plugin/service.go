// Package plugin implements the PluginService of the agent, which manages the plugins
// and mods of servers. Files are confined to the plugin folder of the server's type.
package plugin

import (
	"context"
	"crypto/sha512"
	"encoding/hex"
	"errors"
	"io"
	"io/fs"
	"path/filepath"
	"regexp"
	"sync"
	"time"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	noryxv1 "github.com/QwikByte/noryx/api/noryx/v1"
	"github.com/QwikByte/noryx/internal/agent/datadir"
	"github.com/QwikByte/noryx/internal/agent/runtime"
)

const (
	maxPlugins = 1000
	// MaxSize limits plugin files; even the largest mods stay well below it.
	MaxSize = 256 << 20
)

// folders are where the server types load plugins or mods from.
var folders = map[noryxv1.ServerType]string{
	noryxv1.ServerType_SERVER_TYPE_PAPER:      "plugins",
	noryxv1.ServerType_SERVER_TYPE_PURPUR:     "plugins",
	noryxv1.ServerType_SERVER_TYPE_VELOCITY:   "plugins",
	noryxv1.ServerType_SERVER_TYPE_BUNGEECORD: "plugins",
	noryxv1.ServerType_SERVER_TYPE_WATERFALL:  "plugins",
	noryxv1.ServerType_SERVER_TYPE_FABRIC:     "mods",
	noryxv1.ServerType_SERVER_TYPE_FORGE:      "mods",
	noryxv1.ServerType_SERVER_TYPE_NEOFORGE:   "mods",
}

// Folder returns the folder that servers of a type load plugins or mods from.
func Folder(t noryxv1.ServerType) (string, bool) {
	folder, ok := folders[t]
	return folder, ok
}

// fileName matches plugin files: a plain .jar file name without paths.
var fileName = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9 ._+()\[\]-]{0,127}\.jar$`)

type Service struct {
	noryxv1.UnimplementedPluginServiceServer
	rt runtime.Runtime

	mu sync.Mutex
	// sums caches the hashes of plugin files per server, as modpacks can have hundreds.
	sums map[string]map[string]sum
}

type sum struct {
	size     int64
	modified time.Time
	sha512   string
}

func NewService(rt runtime.Runtime) *Service {
	return &Service{rt: rt, sums: map[string]map[string]sum{}}
}

func (s *Service) ListPlugins(ctx context.Context, req *noryxv1.ListPluginsRequest) (*noryxv1.ListPluginsResponse, error) {
	dir, folder, err := s.open(ctx, req.GetServerId())
	if err != nil {
		return nil, err
	}
	defer dir.Close()
	res := &noryxv1.ListPluginsResponse{Folder: folder}
	entries, err := fs.ReadDir(dir.FS(), folder)
	if errors.Is(err, fs.ErrNotExist) {
		return res, nil
	}
	if err != nil {
		return nil, toStatus(err)
	}
	s.mu.Lock()
	known := s.sums[req.GetServerId()]
	s.mu.Unlock()
	current := map[string]sum{}
	for _, e := range entries {
		info, err := e.Info()
		if err != nil || !e.Type().IsRegular() || !fileName.MatchString(e.Name()) || len(res.Plugins) == maxPlugins {
			continue
		}
		cached, ok := known[e.Name()]
		if !ok || cached.size != info.Size() || !cached.modified.Equal(info.ModTime()) {
			hash, err := hashFile(dir, filepath.Join(folder, e.Name()))
			if err != nil {
				return nil, toStatus(err)
			}
			cached = sum{info.Size(), info.ModTime(), hash}
		}
		current[e.Name()] = cached
		res.Plugins = append(res.Plugins, &noryxv1.PluginFile{FileName: e.Name(), Size: info.Size(), Sha512: cached.sha512})
	}
	s.mu.Lock()
	s.sums[req.GetServerId()] = current
	s.mu.Unlock()
	return res, nil
}

func hashFile(dir *datadir.Dir, name string) (string, error) {
	f, err := dir.Open(name)
	if err != nil {
		return "", err
	}
	defer f.Close()
	h := sha512.New()
	if _, err := io.Copy(h, f); err != nil {
		return "", err
	}
	return hex.EncodeToString(h.Sum(nil)), nil
}

func (s *Service) InstallPlugin(stream noryxv1.PluginService_InstallPluginServer) error {
	first, err := stream.Recv()
	if err != nil {
		return err
	}
	header := first.GetHeader()
	switch {
	case header == nil:
		return status.Error(codes.InvalidArgument, "the first message must name the file")
	case !fileName.MatchString(header.GetFileName()), header.GetReplaces() != "" && !fileName.MatchString(header.GetReplaces()):
		return status.Error(codes.InvalidArgument, "Plugins are .jar files with a name of letters, digits, spaces and . _ - + ( ) [ ].")
	}
	dir, folder, err := s.open(stream.Context(), header.GetServerId())
	if err != nil {
		return err
	}
	defer dir.Close()
	if err := dir.MkdirAll(folder); err != nil {
		return toStatus(err)
	}
	h := sha512.New()
	var size int64
	err = dir.Replace(filepath.Join(folder, header.GetFileName()), true, func(w io.Writer) error {
		for {
			msg, err := stream.Recv()
			switch {
			case errors.Is(err, io.EOF) && size == 0:
				return status.Error(codes.InvalidArgument, "The plugin file is empty.")
			case errors.Is(err, io.EOF):
				return nil
			case err != nil:
				return err
			}
			if size += int64(len(msg.GetData())); size > MaxSize {
				return status.Errorf(codes.ResourceExhausted, "Plugins can have up to %d MB.", MaxSize>>20)
			}
			if _, err := io.MultiWriter(w, h).Write(msg.GetData()); err != nil {
				return err
			}
		}
	})
	if err != nil {
		return toStatus(err)
	}
	if old := header.GetReplaces(); old != "" && old != header.GetFileName() {
		if err := dir.Remove(filepath.Join(folder, old)); err != nil && !errors.Is(err, fs.ErrNotExist) {
			return toStatus(err)
		}
	}
	return stream.SendAndClose(&noryxv1.InstallPluginResponse{Plugin: &noryxv1.PluginFile{
		FileName: header.GetFileName(), Size: size, Sha512: hex.EncodeToString(h.Sum(nil)),
	}})
}

func (s *Service) RemovePlugin(ctx context.Context, req *noryxv1.RemovePluginRequest) (*noryxv1.RemovePluginResponse, error) {
	if !fileName.MatchString(req.GetFileName()) {
		return nil, status.Error(codes.InvalidArgument, "invalid plugin file name")
	}
	dir, folder, err := s.open(ctx, req.GetServerId())
	if err != nil {
		return nil, err
	}
	defer dir.Close()
	return &noryxv1.RemovePluginResponse{}, toStatus(dir.Remove(filepath.Join(folder, req.GetFileName())))
}

// open opens the data directory of a server and returns its plugin folder.
func (s *Service) open(ctx context.Context, id string) (*datadir.Dir, string, error) {
	if !runtime.ValidID(id) {
		return nil, "", status.Error(codes.InvalidArgument, "invalid server ID")
	}
	srv, err := runtime.Find(ctx, s.rt, id)
	if err != nil {
		return nil, "", toStatus(err)
	}
	folder, ok := Folder(srv.Type)
	if !ok {
		return nil, "", status.Error(codes.FailedPrecondition, "Vanilla servers can't load plugins or mods.")
	}
	dir, err := s.rt.Data(ctx, id)
	if err != nil {
		return nil, "", toStatus(err)
	}
	return dir, folder, nil
}

func toStatus(err error) error {
	switch {
	case err == nil:
		return nil
	case status.Code(err) != codes.Unknown:
		return err
	case errors.Is(err, runtime.ErrNotFound):
		return status.Error(codes.NotFound, "Server not found.")
	case errors.Is(err, fs.ErrNotExist):
		return status.Error(codes.NotFound, "Plugin not found.")
	case errors.Is(err, context.Canceled), errors.Is(err, context.DeadlineExceeded):
		return status.FromContextError(err).Err()
	}
	return status.Error(codes.Internal, err.Error())
}
