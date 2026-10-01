// Package properties implements the PropertiesService of the agent, with which the
// panel edits the server.properties of game servers.
package properties

import (
	"cmp"
	"context"
	"errors"
	"io/fs"
	"regexp"
	"slices"
	"strings"
	"unicode"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	mcsmv1 "github.com/QwikByte/mc-server-manager/api/mcsm/v1"
	"github.com/QwikByte/mc-server-manager/internal/agent/datadir"
	"github.com/QwikByte/mc-server-manager/internal/agent/runtime"
)

const (
	file       = "server.properties"
	maxChanges = 200
	maxValue   = 4096
)

var keyPattern = regexp.MustCompile(`^[a-zA-Z0-9][a-zA-Z0-9._-]{0,63}$`)

// secret properties are never sent to the panel, nor changed through it.
var secret = map[string]bool{"rcon.password": true, "management-server-secret": true, "management-server-tls-keystore-password": true}

// locked returns the properties the manager sets itself, with the reason; changing
// them would break the server or its console.
func locked(spec runtime.Spec) map[string]string {
	const rcon = "The console sends its commands through RCON."
	l := map[string]string{
		"server-port":   "Inside its container, the server always uses this port. Change the port in the server's settings.",
		"server-ip":     "The server listens on all addresses of its container.",
		"enable-rcon":   rcon,
		"rcon.port":     rcon,
		"rcon.password": rcon,
	}
	if spec.BehindProxy {
		l["online-mode"] = "The proxy of the server's network authenticates players."
	}
	return l
}

type Service struct {
	mcsmv1.UnimplementedPropertiesServiceServer
	rt runtime.Runtime
}

func NewService(rt runtime.Runtime) *Service { return &Service{rt: rt} }

func (s *Service) GetServerProperties(ctx context.Context, req *mcsmv1.GetServerPropertiesRequest) (*mcsmv1.GetServerPropertiesResponse, error) {
	spec, dir, err := s.open(ctx, req.GetServerId())
	if err != nil {
		return nil, err
	}
	defer dir.Close()
	data, err := dir.ReadFile(file)
	if err != nil && !errors.Is(err, fs.ErrNotExist) {
		return nil, status.Error(codes.Internal, err.Error())
	}
	res := &mcsmv1.GetServerPropertiesResponse{Exists: err == nil, Properties: map[string]string{}}
	for _, e := range parse(string(data)) {
		if e.key != "" && !secret[e.key] {
			res.Properties[e.key] = e.value
		}
	}
	for key, reason := range locked(spec) {
		res.Locked = append(res.Locked, &mcsmv1.LockedProperty{Key: key, Reason: reason})
	}
	slices.SortFunc(res.Locked, func(a, b *mcsmv1.LockedProperty) int { return cmp.Compare(a.GetKey(), b.GetKey()) })
	return res, nil
}

func (s *Service) UpdateServerProperties(ctx context.Context, req *mcsmv1.UpdateServerPropertiesRequest) (*mcsmv1.UpdateServerPropertiesResponse, error) {
	spec, dir, err := s.open(ctx, req.GetServerId())
	if err != nil {
		return nil, err
	}
	defer dir.Close()
	if msg := validate(req.GetProperties(), locked(spec)); msg != "" {
		return nil, status.Error(codes.InvalidArgument, msg)
	}
	data, err := dir.ReadFile(file)
	if errors.Is(err, fs.ErrNotExist) {
		return nil, status.Error(codes.FailedPrecondition, "Start the server once to create server.properties.")
	}
	if err != nil {
		return nil, status.Error(codes.Internal, err.Error())
	}
	if err := dir.WriteFile(file, []byte(update(parse(string(data)), req.GetProperties()))); err != nil {
		return nil, status.Error(codes.Internal, err.Error())
	}
	return &mcsmv1.UpdateServerPropertiesResponse{}, nil
}

// validate returns a message for the operator if the changes are invalid.
func validate(changes, locked map[string]string) string {
	if len(changes) > maxChanges {
		return "Too many changes at once."
	}
	for key, value := range changes {
		switch {
		case !keyPattern.MatchString(key):
			return "Invalid property name " + key + "."
		case locked[key] != "":
			return key + " can't be changed: " + locked[key]
		case secret[key]:
			return key + " can only be changed in the file manager."
		case len(value) > maxValue || strings.ContainsFunc(value, func(r rune) bool { return r != '\n' && unicode.IsControl(r) }):
			return "The value of " + key + " is too long or contains control characters."
		}
	}
	return ""
}

// open finds a game server and opens its data directory.
func (s *Service) open(ctx context.Context, id string) (runtime.Spec, *datadir.Dir, error) {
	if !runtime.ValidID(id) {
		return runtime.Spec{}, nil, status.Error(codes.InvalidArgument, "invalid server ID")
	}
	servers, err := s.rt.List(ctx)
	if err != nil {
		return runtime.Spec{}, nil, status.Error(codes.Internal, err.Error())
	}
	i := slices.IndexFunc(servers, func(srv runtime.Server) bool { return srv.ID == id })
	if i < 0 {
		return runtime.Spec{}, nil, status.Error(codes.NotFound, "Server not found.")
	}
	spec := servers[i].Spec
	if spec.Type.Proxy() {
		return spec, nil, status.Error(codes.FailedPrecondition, "Proxies have no server.properties.")
	}
	dir, err := s.rt.Data(ctx, id)
	if err != nil {
		return spec, nil, status.Error(codes.Internal, err.Error())
	}
	return spec, dir, nil
}
