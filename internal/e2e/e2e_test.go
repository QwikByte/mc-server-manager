// Package e2e tests master and agent together over real mutually authenticated connections.
package e2e

import (
	"context"
	"net"
	"path/filepath"
	"sync"
	"testing"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/status"

	mcsmv1 "github.com/QwikByte/mc-server-manager/api/mcsm/v1"
	"github.com/QwikByte/mc-server-manager/internal/agent/app"
	"github.com/QwikByte/mc-server-manager/internal/agent/enroll"
	"github.com/QwikByte/mc-server-manager/internal/agent/runtime"
	"github.com/QwikByte/mc-server-manager/internal/master/database"
	"github.com/QwikByte/mc-server-manager/internal/master/node"
	"github.com/QwikByte/mc-server-manager/internal/pki"
)

func TestEnrollAndControlNode(t *testing.T) {
	ctx, dir := t.Context(), t.TempDir()

	// Master with its enrollment endpoint.
	db, err := database.Open(filepath.Join(dir, "master.db"))
	check(t, err)
	t.Cleanup(func() { db.Close() })
	ca, err := pki.LoadOrCreateCA(filepath.Join(dir, "master"))
	check(t, err)
	masterCert, err := ca.MasterCertificate()
	check(t, err)
	enrollListener := listen(t)
	nodes := node.NewService(db, ca, masterCert, enrollListener.Addr().String())
	t.Cleanup(nodes.Close)
	enrollServer := grpc.NewServer(grpc.Creds(credentials.NewTLS(pki.MasterServerTLS(masterCert))))
	mcsmv1.RegisterEnrollmentServiceServer(enrollServer, nodes)
	serve(t, enrollServer, enrollListener)

	// Register the node and enroll its agent; the join token works only once.
	agentListener := listen(t)
	n, token, err := nodes.Create(ctx, "node-1", agentListener.Addr().String())
	check(t, err)
	agentDir := filepath.Join(dir, "agent")
	check(t, enroll.Run(ctx, token.String(), agentDir))
	if err := enroll.Run(ctx, token.String(), t.TempDir()); status.Code(err) != codes.PermissionDenied {
		t.Fatalf("reused join token: got %v, want PermissionDenied", err)
	}

	// Agent serving with its new credentials.
	nodeCert, err := pki.LoadKeyPair(agentDir, enroll.NodeCert)
	check(t, err)
	agentCA, err := pki.LoadCert(agentDir, enroll.CACert)
	check(t, err)
	serve(t, app.NewGRPCServer(&fakeRuntime{}, grpc.Creds(credentials.NewTLS(pki.AgentServerTLS(nodeCert, agentCA)))), agentListener)

	// The master controls the node.
	conn, err := nodes.Conn(ctx, n.ID)
	check(t, err)
	info, err := mcsmv1.NewNodeServiceClient(conn).GetInfo(ctx, &mcsmv1.GetInfoRequest{})
	check(t, err)
	if info.GetRuntime() != "fake" {
		t.Fatalf("runtime = %q, want fake", info.GetRuntime())
	}

	servers := mcsmv1.NewServerServiceClient(conn)
	lobby := &mcsmv1.CreateServerRequest{Name: "Lobby", Type: mcsmv1.ServerType_SERVER_TYPE_PAPER, MemoryMb: 2048, Port: 25565, AcceptEula: true}
	created, err := servers.CreateServer(ctx, lobby)
	check(t, err)
	_, err = servers.StartServer(ctx, &mcsmv1.StartServerRequest{Id: created.GetServer().GetId()})
	check(t, err)
	list, err := servers.ListServers(ctx, &mcsmv1.ListServersRequest{})
	check(t, err)
	if len(list.GetServers()) != 1 || list.GetServers()[0].GetState() != mcsmv1.ServerState_SERVER_STATE_RUNNING {
		t.Fatalf("servers = %v, want one running server", list.GetServers())
	}

	// Invalid requests are rejected by the agent.
	for _, tc := range []struct {
		name string
		want codes.Code
		call func() error
	}{
		{"missing EULA", codes.InvalidArgument, func() error {
			_, err := servers.CreateServer(ctx, &mcsmv1.CreateServerRequest{Name: "Survival", Type: lobby.Type, MemoryMb: 2048, Port: 25566})
			return err
		}},
		{"port in use", codes.AlreadyExists, func() error {
			_, err := servers.CreateServer(ctx, lobby)
			return err
		}},
		{"path traversal", codes.InvalidArgument, func() error {
			_, err := servers.DeleteServer(ctx, &mcsmv1.DeleteServerRequest{Id: "../../etc"})
			return err
		}},
	} {
		if got := status.Code(tc.call()); got != tc.want {
			t.Errorf("%s: got %v, want %v", tc.name, got, tc.want)
		}
	}
}

func check(t *testing.T, err error) {
	t.Helper()
	if err != nil {
		t.Fatal(err)
	}
}

func listen(t *testing.T) net.Listener {
	t.Helper()
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	check(t, err)
	return ln
}

func serve(t *testing.T, s *grpc.Server, ln net.Listener) {
	go func() { _ = s.Serve(ln) }()
	t.Cleanup(s.Stop)
}

// fakeRuntime keeps servers in memory.
type fakeRuntime struct {
	mu      sync.Mutex
	servers []runtime.Server
}

func (f *fakeRuntime) Info(context.Context) (runtime.Info, error) {
	return runtime.Info{Name: "fake"}, nil
}

func (f *fakeRuntime) List(context.Context) ([]runtime.Server, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	return append([]runtime.Server(nil), f.servers...), nil
}

func (f *fakeRuntime) Create(_ context.Context, spec runtime.Spec) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.servers = append(f.servers, runtime.Server{Spec: spec, State: mcsmv1.ServerState_SERVER_STATE_STOPPED})
	return nil
}

func (f *fakeRuntime) Start(_ context.Context, id string) error {
	return f.setState(id, mcsmv1.ServerState_SERVER_STATE_RUNNING)
}

func (f *fakeRuntime) Stop(_ context.Context, id string) error {
	return f.setState(id, mcsmv1.ServerState_SERVER_STATE_STOPPED)
}

func (f *fakeRuntime) Remove(context.Context, string) error { return runtime.ErrNotFound }

func (f *fakeRuntime) setState(id string, state mcsmv1.ServerState) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	for i := range f.servers {
		if f.servers[i].ID == id {
			f.servers[i].State = state
			return nil
		}
	}
	return runtime.ErrNotFound
}
