// Package server implements the ServerService of the agent on top of a runtime.
// All input from the master and the local CLI is validated here.
package server

import (
	"cmp"
	"context"
	"crypto/rand"
	"errors"
	"fmt"
	"regexp"
	"slices"
	"strings"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	mcsmv1 "github.com/QwikByte/mc-server-manager/api/mcsm/v1"
	"github.com/QwikByte/mc-server-manager/internal/agent/runtime"
)

const (
	minMemoryMB = 512
	maxMemoryMB = 64 * 1024
	minPort     = 1024
	maxPort     = 65535
)

var (
	idPattern      = regexp.MustCompile(`^[a-z2-7]{26}$`) // lower-cased crypto/rand.Text
	namePattern    = regexp.MustCompile(`^[\pL\pN][\pL\pN _.-]{0,31}$`)
	versionPattern = regexp.MustCompile(`^[A-Za-z0-9._-]{1,32}$`)
)

type Service struct {
	mcsmv1.UnimplementedServerServiceServer
	rt runtime.Runtime
}

func NewService(rt runtime.Runtime) *Service { return &Service{rt: rt} }

func (s *Service) ListServers(ctx context.Context, _ *mcsmv1.ListServersRequest) (*mcsmv1.ListServersResponse, error) {
	servers, err := s.rt.List(ctx)
	if err != nil {
		return nil, toStatus(err)
	}
	res := &mcsmv1.ListServersResponse{}
	for _, srv := range servers {
		res.Servers = append(res.Servers, toProto(srv))
	}
	return res, nil
}

func (s *Service) CreateServer(ctx context.Context, req *mcsmv1.CreateServerRequest) (*mcsmv1.CreateServerResponse, error) {
	if msg := validate(req); msg != "" {
		return nil, status.Error(codes.InvalidArgument, msg)
	}
	existing, err := s.rt.List(ctx)
	if err != nil {
		return nil, toStatus(err)
	}
	if i := slices.IndexFunc(existing, func(srv runtime.Server) bool { return srv.Port == req.GetPort() }); i >= 0 {
		return nil, status.Errorf(codes.AlreadyExists, "Port %d is already used by %q. Choose another port.", req.GetPort(), existing[i].Name)
	}
	spec := runtime.Spec{
		ID:       strings.ToLower(rand.Text()),
		Name:     req.GetName(),
		Type:     req.GetType(),
		Version:  cmp.Or(req.GetVersion(), "LATEST"),
		MemoryMB: req.GetMemoryMb(),
		Port:     req.GetPort(),
	}
	if err := s.rt.Create(ctx, spec); err != nil {
		return nil, toStatus(err)
	}
	return &mcsmv1.CreateServerResponse{Server: toProto(runtime.Server{Spec: spec, State: mcsmv1.ServerState_SERVER_STATE_STOPPED})}, nil
}

func (s *Service) StartServer(ctx context.Context, req *mcsmv1.StartServerRequest) (*mcsmv1.StartServerResponse, error) {
	return &mcsmv1.StartServerResponse{}, s.apply(ctx, req.GetId(), s.rt.Start)
}

func (s *Service) StopServer(ctx context.Context, req *mcsmv1.StopServerRequest) (*mcsmv1.StopServerResponse, error) {
	return &mcsmv1.StopServerResponse{}, s.apply(ctx, req.GetId(), s.rt.Stop)
}

func (s *Service) DeleteServer(ctx context.Context, req *mcsmv1.DeleteServerRequest) (*mcsmv1.DeleteServerResponse, error) {
	return &mcsmv1.DeleteServerResponse{}, s.apply(ctx, req.GetId(), s.rt.Remove)
}

func (s *Service) apply(ctx context.Context, id string, op func(context.Context, string) error) error {
	if !idPattern.MatchString(id) {
		return status.Error(codes.InvalidArgument, "invalid server ID")
	}
	return toStatus(op(ctx, id))
}

// validate returns a message for the operator if the request is invalid.
func validate(req *mcsmv1.CreateServerRequest) string {
	_, knownType := mcsmv1.ServerType_name[int32(req.GetType())]
	switch {
	case !req.GetAcceptEula():
		return "Accept the Minecraft EULA to create a server."
	case !namePattern.MatchString(req.GetName()):
		return "Use 1-32 letters, digits, spaces, '.', '_' or '-' for the name."
	case !knownType || req.GetType() == mcsmv1.ServerType_SERVER_TYPE_UNSPECIFIED:
		return "Choose a server type."
	case req.GetVersion() != "" && !versionPattern.MatchString(req.GetVersion()):
		return "Enter a Minecraft version like 1.21.4, or leave it empty for the latest."
	case req.GetMemoryMb() < minMemoryMB || req.GetMemoryMb() > maxMemoryMB:
		return fmt.Sprintf("Memory must be between %d and %d MB.", minMemoryMB, maxMemoryMB)
	case req.GetPort() < minPort || req.GetPort() > maxPort:
		return fmt.Sprintf("Port must be between %d and %d.", minPort, maxPort)
	}
	return ""
}

func toProto(s runtime.Server) *mcsmv1.Server {
	return &mcsmv1.Server{Id: s.ID, Name: s.Name, Type: s.Type, Version: s.Version, MemoryMb: s.MemoryMB, Port: s.Port, State: s.State}
}

func toStatus(err error) error {
	switch {
	case err == nil:
		return nil
	case errors.Is(err, runtime.ErrNotFound):
		return status.Error(codes.NotFound, "Server not found.")
	default:
		return status.Error(codes.Internal, err.Error())
	}
}
