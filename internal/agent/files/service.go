// Package files implements the FileService of the agent, which backs the file manager
// of the panel. All paths are confined to the data directory of the server.
package files

import (
	"bytes"
	"cmp"
	"context"
	"errors"
	"io"
	"io/fs"
	"path"
	"slices"
	"strings"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	mcsmv1 "github.com/QwikByte/mc-server-manager/api/mcsm/v1"
	"github.com/QwikByte/mc-server-manager/internal/agent/datadir"
	"github.com/QwikByte/mc-server-manager/internal/agent/runtime"
	"github.com/QwikByte/mc-server-manager/internal/agent/secrets"
)

const (
	chunkSize  = 256 << 10
	maxEntries = 5000
	// MaxFileSize limits uploads; worlds and modpacks stay well below it.
	MaxFileSize = 16 << 30
	// maxRedacted limits files with secrets, which are read at once to hide them.
	maxRedacted = 1 << 20
)

// errSecret refuses access to the files that only hold secrets, and moves of those with secrets.
var errSecret = status.Error(codes.PermissionDenied, "This file holds secrets of the server, such as the RCON password, which the panel doesn't show.")

type Service struct {
	mcsmv1.UnimplementedFileServiceServer
	rt runtime.Runtime
}

func NewService(rt runtime.Runtime) *Service { return &Service{rt: rt} }

func (s *Service) ListFiles(ctx context.Context, req *mcsmv1.ListFilesRequest) (*mcsmv1.ListFilesResponse, error) {
	dir, name, err := s.open(ctx, req.GetServerId(), req.GetPath())
	if err != nil {
		return nil, err
	}
	defer dir.Close()
	f, err := dir.Open(name)
	if err != nil {
		return nil, toStatus(err)
	}
	defer f.Close()
	entries, err := f.ReadDir(maxEntries + 1)
	if err != nil && !errors.Is(err, io.EOF) {
		return nil, toStatus(err)
	}
	res := &mcsmv1.ListFilesResponse{Truncated: len(entries) > maxEntries}
	for _, e := range entries[:min(len(entries), maxEntries)] {
		if info, err := e.Info(); err == nil && !secrets.Hidden(path.Join(name, e.Name())) { // also skips entries deleted in the meantime
			res.Files = append(res.Files, fileInfo(info))
		}
	}
	slices.SortFunc(res.Files, func(a, b *mcsmv1.FileInfo) int {
		if a.GetDirectory() != b.GetDirectory() {
			return map[bool]int{true: -1, false: 1}[a.GetDirectory()]
		}
		return cmp.Compare(strings.ToLower(a.GetName()), strings.ToLower(b.GetName()))
	})
	return res, nil
}

func (s *Service) ReadFile(req *mcsmv1.ReadFileRequest, stream mcsmv1.FileService_ReadFileServer) error {
	dir, name, err := s.open(stream.Context(), req.GetServerId(), req.GetPath())
	if err != nil {
		return err
	}
	defer dir.Close()
	if secrets.Hidden(name) {
		return errSecret
	}
	f, err := dir.Open(name)
	if err != nil {
		return toStatus(err)
	}
	defer f.Close()
	info, err := f.Stat()
	if err != nil {
		return toStatus(err)
	}
	if info.IsDir() {
		return status.Error(codes.InvalidArgument, "This is a folder. Download it as a ZIP archive instead.")
	}
	r, size := io.Reader(f), info.Size()
	if secrets.Redacted(name) {
		data, err := readRedacted(f)
		if err != nil {
			return err
		}
		data = secrets.Redact(name, data)
		r, size = bytes.NewReader(data), int64(len(data))
	}
	res := &mcsmv1.ReadFileResponse{Size: size}
	return sendChunks(r, func(data []byte) error {
		res.Data = data
		err := stream.Send(res)
		res = &mcsmv1.ReadFileResponse{}
		return err
	})
}

func (s *Service) WriteFile(stream mcsmv1.FileService_WriteFileServer) error {
	first, err := stream.Recv()
	if err != nil {
		return err
	}
	header := first.GetHeader()
	if header == nil {
		return status.Error(codes.InvalidArgument, "the first message must name the file")
	}
	dir, name, err := s.open(stream.Context(), header.GetServerId(), header.GetPath())
	if err != nil {
		return err
	}
	defer dir.Close()
	switch {
	case name == ".":
		return status.Error(codes.InvalidArgument, "Choose a file name.")
	case secrets.Hidden(name):
		return errSecret
	}
	err = dir.Replace(name, header.GetOverwrite(), func(w io.Writer) error {
		if !secrets.Redacted(name) {
			return receive(stream, w, MaxFileSize)
		}
		// The secrets the panel showed as placeholders stay as they are.
		var data bytes.Buffer
		if err := receive(stream, &data, maxRedacted); err != nil {
			return err
		}
		current, err := dir.ReadOptional(name)
		if err == nil {
			_, err = w.Write(secrets.Restore(name, data.Bytes(), current))
		}
		return err
	})
	if err != nil {
		return toStatus(err)
	}
	info, err := dir.Stat(name)
	if err != nil {
		return toStatus(err)
	}
	return stream.SendAndClose(&mcsmv1.WriteFileResponse{File: fileInfo(info)})
}

func (s *Service) ArchiveDirectory(req *mcsmv1.ArchiveDirectoryRequest, stream mcsmv1.FileService_ArchiveDirectoryServer) error {
	dir, name, err := s.open(stream.Context(), req.GetServerId(), req.GetPath())
	if err != nil {
		return err
	}
	defer dir.Close()
	sub, err := dir.OpenRoot(name)
	if err != nil {
		return toStatus(err)
	}
	defer sub.Close()
	var censor datadir.Censor
	if req.GetHideSecrets() {
		censor = secrets.Censor(name)
	}
	r, w := io.Pipe()
	defer r.Close() // stops WriteZip if the client goes away
	go func() { w.CloseWithError(datadir.WriteZip(stream.Context(), w, sub, censor, ".")) }()
	return sendChunks(r, func(data []byte) error {
		return stream.Send(&mcsmv1.ArchiveDirectoryResponse{Data: data})
	})
}

func (s *Service) CreateDirectory(ctx context.Context, req *mcsmv1.CreateDirectoryRequest) (*mcsmv1.CreateDirectoryResponse, error) {
	dir, name, err := s.open(ctx, req.GetServerId(), req.GetPath())
	if err != nil {
		return nil, err
	}
	defer dir.Close()
	if _, err := dir.Lstat(name); err == nil {
		return nil, toStatus(fs.ErrExist)
	}
	return &mcsmv1.CreateDirectoryResponse{}, toStatus(dir.MkdirAll(name))
}

func (s *Service) MoveFile(ctx context.Context, req *mcsmv1.MoveFileRequest) (*mcsmv1.MoveFileResponse, error) {
	dir, from, err := s.open(ctx, req.GetServerId(), req.GetFrom())
	if err != nil {
		return nil, err
	}
	defer dir.Close()
	to, err := clean(req.GetTo())
	if err != nil {
		return nil, err
	}
	switch {
	case from == "." || to == ".":
		return nil, status.Error(codes.InvalidArgument, "The server folder itself can't be moved.")
	case slices.ContainsFunc(secrets.Under(from), func(p string) bool { _, err := dir.Lstat(p); return err == nil }):
		return nil, errSecret // the secrets would show at the new place
	}
	if _, err := dir.Lstat(to); err == nil {
		return nil, toStatus(fs.ErrExist)
	}
	return &mcsmv1.MoveFileResponse{}, toStatus(dir.Rename(from, to))
}

func (s *Service) DeleteFile(ctx context.Context, req *mcsmv1.DeleteFileRequest) (*mcsmv1.DeleteFileResponse, error) {
	dir, name, err := s.open(ctx, req.GetServerId(), req.GetPath())
	if err != nil {
		return nil, err
	}
	defer dir.Close()
	if name == "." {
		return nil, status.Error(codes.InvalidArgument, "The server folder itself can't be deleted.")
	}
	if _, err := dir.Lstat(name); err != nil {
		return nil, toStatus(err)
	}
	return &mcsmv1.DeleteFileResponse{}, toStatus(dir.RemoveAll(name))
}

// open opens the data directory of a server and returns the path as a name in it.
func (s *Service) open(ctx context.Context, id, p string) (*datadir.Dir, string, error) {
	if !runtime.ValidID(id) {
		return nil, "", status.Error(codes.InvalidArgument, "invalid server ID")
	}
	name, err := clean(p)
	if err != nil {
		return nil, "", err
	}
	dir, err := s.rt.Data(ctx, id)
	if errors.Is(err, runtime.ErrNotFound) {
		return nil, "", status.Error(codes.NotFound, "Server not found.")
	}
	if err != nil {
		return nil, "", status.Error(codes.Internal, err.Error())
	}
	return dir, name, nil
}

func clean(p string) (string, error) {
	name, ok := datadir.Name(p)
	if !ok {
		return "", status.Error(codes.InvalidArgument, "invalid path")
	}
	return name, nil
}

func fileInfo(info fs.FileInfo) *mcsmv1.FileInfo {
	return &mcsmv1.FileInfo{Name: info.Name(), Directory: info.IsDir(), Size: info.Size(), ModifiedUnix: info.ModTime().Unix()}
}

// receive writes the data of a WriteFile stream to w, up to limit bytes.
func receive(stream mcsmv1.FileService_WriteFileServer, w io.Writer, limit int64) error {
	var written int64
	for {
		msg, err := stream.Recv()
		if errors.Is(err, io.EOF) {
			return nil
		}
		if err != nil {
			return err
		}
		if written += int64(len(msg.GetData())); written > limit {
			return tooLarge(limit)
		}
		if _, err := w.Write(msg.GetData()); err != nil {
			return err
		}
	}
}

// readRedacted reads a file with secrets, up to maxRedacted bytes.
func readRedacted(r io.Reader) ([]byte, error) {
	data, err := io.ReadAll(io.LimitReader(r, maxRedacted+1))
	if err == nil && len(data) > maxRedacted {
		return nil, tooLarge(maxRedacted)
	}
	return data, toStatus(err)
}

func tooLarge(limit int64) error {
	if limit < MaxFileSize {
		return status.Errorf(codes.ResourceExhausted, "Files with secrets can have up to %d MB.", limit>>20)
	}
	return status.Errorf(codes.ResourceExhausted, "Files can have up to %d GB.", limit>>30)
}

// sendChunks reads r to the end and passes it on in chunks. It always sends at least
// one chunk, so that the receiver gets the first message of an empty file.
func sendChunks(r io.Reader, send func([]byte) error) error {
	buf := make([]byte, chunkSize)
	for first := true; ; first = false {
		n, err := io.ReadFull(r, buf)
		if n > 0 || first {
			if err := send(buf[:n]); err != nil {
				return err
			}
		}
		if errors.Is(err, io.EOF) || errors.Is(err, io.ErrUnexpectedEOF) {
			return nil
		}
		if err != nil {
			return toStatus(err)
		}
	}
}

func toStatus(err error) error {
	switch {
	case err == nil:
		return nil
	case status.Code(err) != codes.Unknown:
		return err
	case errors.Is(err, fs.ErrNotExist):
		return status.Error(codes.NotFound, "File or folder not found.")
	case errors.Is(err, fs.ErrExist):
		return status.Error(codes.AlreadyExists, "A file or folder with this name already exists.")
	case errors.Is(err, fs.ErrPermission):
		return status.Error(codes.PermissionDenied, "The agent isn't allowed to access this file.")
	case errors.Is(err, context.Canceled), errors.Is(err, context.DeadlineExceeded):
		return status.FromContextError(err).Err()
	case strings.Contains(err.Error(), "path escapes from parent"): // os.Root, e.g. through a symbolic link
		return status.Error(codes.InvalidArgument, "This path leads outside of the server folder.")
	}
	return status.Error(codes.Internal, err.Error())
}
