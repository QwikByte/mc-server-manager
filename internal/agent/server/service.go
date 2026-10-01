// Package server implements the ServerService of the agent on top of a runtime.
// All input from the master and the local CLI is validated here.
package server

import (
	"cmp"
	"context"
	"crypto/rand"
	"errors"
	"fmt"
	"net"
	"regexp"
	"slices"
	"strconv"
	"strings"
	"unicode"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	mcsmv1 "github.com/QwikByte/mc-server-manager/api/mcsm/v1"
	"github.com/QwikByte/mc-server-manager/internal/agent/runtime"
	"github.com/QwikByte/mc-server-manager/internal/agent/storage"
)

const (
	minMemoryMB = 512
	maxMemoryMB = 64 * 1024
	minPort     = 1024
	maxPort     = 65535
	maxTail     = 1000
	maxCommand  = 1000
)

var (
	idPattern      = regexp.MustCompile(`^[a-z2-7]{26}$`) // lower-cased crypto/rand.Text
	namePattern    = regexp.MustCompile(`^[\pL\pN][\pL\pN _.-]{0,31}$`)
	versionPattern = regexp.MustCompile(`^[A-Za-z0-9._-]{1,32}$`)
	secretPattern  = regexp.MustCompile(`^[A-Za-z0-9]{16,128}$`)
	// Names of backends a proxy sends players to; "try" is the list of these names.
	backendPattern = regexp.MustCompile(`^[a-z0-9][a-z0-9_-]{0,31}$`)
	hostPattern    = regexp.MustCompile(`^[A-Za-z0-9.:-]{1,253}$`)
	// Terminal escape sequences and Minecraft formatting codes (§a, §l, ...).
	formatting = regexp.MustCompile(`\x1b\[[0-9;?]*[ -/]*[@-~]|§[0-9a-fk-orxA-FK-ORX]|\r`)
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
		Storage:  req.GetStorage(),
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

func (s *Service) StreamLogs(req *mcsmv1.StreamLogsRequest, stream mcsmv1.ServerService_StreamLogsServer) error {
	if !idPattern.MatchString(req.GetId()) {
		return status.Error(codes.InvalidArgument, "invalid server ID")
	}
	for line, err := range s.rt.Logs(stream.Context(), req.GetId(), int(min(req.GetTail(), maxTail))) {
		if err != nil {
			return toStatus(err)
		}
		if err := stream.Send(&mcsmv1.StreamLogsResponse{Line: plain(line)}); err != nil {
			return err
		}
	}
	return nil
}

func (s *Service) SendCommand(ctx context.Context, req *mcsmv1.SendCommandRequest) (*mcsmv1.SendCommandResponse, error) {
	if !idPattern.MatchString(req.GetId()) {
		return nil, status.Error(codes.InvalidArgument, "invalid server ID")
	}
	command := strings.TrimPrefix(strings.TrimSpace(req.GetCommand()), "/")
	if command == "" || len(command) > maxCommand || strings.ContainsFunc(command, unicode.IsControl) {
		return nil, status.Errorf(codes.InvalidArgument, "Enter a single command with up to %d characters.", maxCommand)
	}
	output, err := s.rt.SendCommand(ctx, req.GetId(), command)
	if errors.Is(err, runtime.ErrUnsupported) {
		return nil, status.Error(codes.FailedPrecondition, "Proxies don't accept console commands yet.")
	}
	if err != nil {
		return nil, toStatus(err)
	}
	return &mcsmv1.SendCommandResponse{Output: plain(output)}, nil
}

func (s *Service) ConfigureNetwork(ctx context.Context, req *mcsmv1.ConfigureNetworkRequest) (*mcsmv1.ConfigureNetworkResponse, error) {
	network, msg := networkOf(req)
	if msg != "" {
		return nil, status.Error(codes.InvalidArgument, msg)
	}
	err := s.rt.Configure(ctx, req.GetId(), network)
	if errors.Is(err, runtime.ErrUnsupported) {
		return nil, status.Error(codes.FailedPrecondition, "Only Velocity proxies and Paper or Purpur servers can be part of a network.")
	}
	if err != nil {
		return nil, toStatus(err)
	}
	return &mcsmv1.ConfigureNetworkResponse{}, nil
}

// networkOf validates a network configuration, which ends up in configuration files.
func networkOf(req *mcsmv1.ConfigureNetworkRequest) (runtime.Network, string) {
	network := runtime.Network{ForwardingSecret: req.GetForwardingSecret()}
	if !idPattern.MatchString(req.GetId()) {
		return network, "invalid server ID"
	}
	if network.ForwardingSecret != "" && !secretPattern.MatchString(network.ForwardingSecret) {
		return network, "invalid forwarding secret"
	}
	seen := map[string]bool{}
	for _, b := range req.GetBackends() {
		backend := runtime.NetworkBackend{Name: b.GetName(), ServerID: b.GetServerId(), Address: b.GetAddress()}
		switch {
		case !backendPattern.MatchString(backend.Name) || backend.Name == "try" || seen[backend.Name]:
			return network, fmt.Sprintf("invalid or duplicate backend name %q", backend.Name)
		case backend.ServerID != "" && !idPattern.MatchString(backend.ServerID):
			return network, "invalid backend server ID"
		case backend.ServerID == "" && !validAddress(backend.Address):
			return network, fmt.Sprintf("invalid backend address %q", backend.Address)
		}
		seen[backend.Name] = true
		network.Backends = append(network.Backends, backend)
	}
	return network, ""
}

func validAddress(address string) bool {
	host, port, err := net.SplitHostPort(address)
	n, perr := strconv.Atoi(port)
	return err == nil && perr == nil && hostPattern.MatchString(host) && n > 0 && n <= maxPort
}

// plain removes colours and formatting so that console output reads as plain text.
func plain(text string) string { return formatting.ReplaceAllString(text, "") }

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
	return &mcsmv1.Server{
		Id: s.ID, Name: s.Name, Type: s.Type, Version: s.Version, MemoryMb: s.MemoryMB, Port: s.Port, State: s.State,
		Storage: cmp.Or(s.Storage, storage.Default),
	}
}

func toStatus(err error) error {
	switch {
	case err == nil:
		return nil
	case errors.Is(err, runtime.ErrNotFound):
		return status.Error(codes.NotFound, "Server not found.")
	case errors.Is(err, runtime.ErrNotRunning):
		return status.Error(codes.FailedPrecondition, "Start the server to send commands.")
	case errors.Is(err, runtime.ErrUnsupported):
		return status.Error(codes.FailedPrecondition, "This type of server does not support that.")
	case errors.Is(err, storage.ErrUnknown):
		return status.Errorf(codes.InvalidArgument, "%s. Add it on the node with: mcsm-agent storage add", err)
	case errors.Is(err, context.Canceled), errors.Is(err, context.DeadlineExceeded):
		return status.FromContextError(err).Err()
	default:
		return status.Error(codes.Internal, err.Error())
	}
}
