// Package e2e tests master and agent together over real mutually authenticated connections.
package e2e

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"io"
	"iter"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"slices"
	"sync"
	"testing"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/status"

	mcsmv1 "github.com/QwikByte/mc-server-manager/api/mcsm/v1"
	"github.com/QwikByte/mc-server-manager/internal/agent/app"
	"github.com/QwikByte/mc-server-manager/internal/agent/datadir"
	"github.com/QwikByte/mc-server-manager/internal/agent/enroll"
	agentnode "github.com/QwikByte/mc-server-manager/internal/agent/node"
	"github.com/QwikByte/mc-server-manager/internal/agent/runtime"
	"github.com/QwikByte/mc-server-manager/internal/agent/storage"
	"github.com/QwikByte/mc-server-manager/internal/enrollment"
	"github.com/QwikByte/mc-server-manager/internal/master/database"
	"github.com/QwikByte/mc-server-manager/internal/master/files"
	"github.com/QwikByte/mc-server-manager/internal/master/network"
	"github.com/QwikByte/mc-server-manager/internal/master/node"
	"github.com/QwikByte/mc-server-manager/internal/master/properties"
	"github.com/QwikByte/mc-server-manager/internal/master/server"
	"github.com/QwikByte/mc-server-manager/internal/pki"
)

func TestEnrollAndControlNode(t *testing.T) {
	ctx := t.Context()
	m := startMaster(t)
	a := m.startAgent(t, "node-1")
	if err := enroll.Run(ctx, a.token.String(), t.TempDir()); status.Code(err) != codes.PermissionDenied {
		t.Fatalf("reused join token: got %v, want PermissionDenied", err)
	}

	// The master controls the node.
	conn, err := m.nodes.Conn(ctx, a.node.ID)
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

	// The console arrives without colour codes, and commands return their output.
	id := created.GetServer().GetId()
	logs, err := servers.StreamLogs(ctx, &mcsmv1.StreamLogsRequest{Id: id, Tail: 10})
	check(t, err)
	var lines []string
	for {
		res, err := logs.Recv()
		if errors.Is(err, io.EOF) {
			break
		}
		check(t, err)
		lines = append(lines, res.GetLine())
	}
	if want := []string{"[INFO]: Starting", "[INFO]: Done"}; !slices.Equal(lines, want) {
		t.Fatalf("log lines = %q, want %q", lines, want)
	}
	out, err := servers.SendCommand(ctx, &mcsmv1.SendCommandRequest{Id: id, Command: "/say hi"})
	check(t, err)
	if out.GetOutput() != "ran say hi" {
		t.Fatalf("command output = %q", out.GetOutput())
	}

	// The master relays the console to the browser as Server-Sent Events.
	res, err := http.Get(m.panel(t).URL + "/api/nodes/" + a.node.ID + "/servers/" + id + "/logs")
	check(t, err)
	body, err := io.ReadAll(res.Body)
	res.Body.Close()
	check(t, err)
	if want := "data: [INFO]: Starting\n\ndata: [INFO]: Done\n\nevent: end\ndata:\n\n"; string(body) != want {
		t.Fatalf("event stream = %q, want %q", body, want)
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
		{"command with several lines", codes.InvalidArgument, func() error {
			_, err := servers.SendCommand(ctx, &mcsmv1.SendCommandRequest{Id: id, Command: "say hi\nop attacker"})
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

	// The master renews the node certificate: the agent stores it and presents it to
	// new connections without a restart.
	enrolled := a.identity.Get().Leaf
	renewed, err := m.nodes.RenewCertificate(ctx, a.node.ID)
	check(t, err)
	_, presented, err := m.nodes.Status(ctx, a.node.ID)
	check(t, err)
	stored, err := pki.LoadKeyPair(a.dir, enroll.NodeCert)
	check(t, err)
	if renewed.Equal(enrolled) || !presented.Equal(renewed) || !stored.Leaf.Equal(renewed) {
		t.Fatal("the renewed certificate is not in use")
	}

	// A certificate for a key the agent did not create is rejected.
	nodeClient := mcsmv1.NewNodeServiceClient(conn)
	_, err = nodeClient.CreateCSR(ctx, &mcsmv1.CreateCSRRequest{})
	check(t, err)
	foreignCSR, err := pki.NewCSR(pki.NewKey())
	check(t, err)
	foreign, err := m.ca.SignNodeCSR(foreignCSR, a.node.ID)
	check(t, err)
	if _, err := nodeClient.InstallCertificate(ctx, &mcsmv1.InstallCertificateRequest{CertificateDer: foreign}); status.Code(err) != codes.FailedPrecondition {
		t.Fatalf("foreign certificate: got %v, want FailedPrecondition", err)
	}
}

// master is a master with its enrollment endpoint.
type master struct {
	db         *sql.DB
	ca         *pki.CA
	nodes      *node.Service
	enrollAddr string
}

func startMaster(t *testing.T) *master {
	dir := t.TempDir()
	db, err := database.Open(filepath.Join(dir, "master.db"))
	check(t, err)
	t.Cleanup(func() { db.Close() })
	ca, err := pki.LoadOrCreateCA(filepath.Join(dir, "pki"))
	check(t, err)
	cert, err := ca.MasterCertificate()
	check(t, err)
	masterCert, err := pki.NewHolder(cert)
	check(t, err)
	ln := listen(t)
	nodes := node.NewService(db, ca, masterCert, ln.Addr().String())
	t.Cleanup(nodes.Close)
	enrollServer := grpc.NewServer(grpc.Creds(credentials.NewTLS(pki.MasterServerTLS(masterCert))))
	mcsmv1.RegisterEnrollmentServiceServer(enrollServer, nodes)
	serve(t, enrollServer, ln)
	return &master{db: db, ca: ca, nodes: nodes, enrollAddr: ln.Addr().String()}
}

// panel serves the REST API of the master without authentication.
func (m *master) panel(t *testing.T) *httptest.Server {
	networks := network.NewService(m.db, m.nodes)
	mux := http.NewServeMux()
	server.NewHandler(m.nodes, networks).Register(mux)
	network.NewHandler(networks).Register(mux)
	files.NewHandler(m.nodes).Register(mux)
	properties.NewHandler(m.nodes).Register(mux)
	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)
	return srv
}

// agent is an enrolled agent that serves a fake runtime.
type agent struct {
	node     node.Node
	token    enrollment.Token
	dir      string
	identity *agentnode.Identity
	runtime  *fakeRuntime
}

// startAgent registers a node and enrolls its agent with a join token.
func (m *master) startAgent(t *testing.T, name string) agent {
	ln := listen(t)
	n, token, err := m.nodes.Create(t.Context(), name, ln.Addr().String())
	check(t, err)
	a := agent{node: n, token: token, dir: t.TempDir(), runtime: &fakeRuntime{dir: t.TempDir()}}
	check(t, enroll.Run(t.Context(), token.String(), a.dir))
	a.identity, err = agentnode.LoadIdentity(a.dir)
	check(t, err)
	creds := credentials.NewTLS(pki.AgentServerTLS(a.identity.Holder, a.identity.CA))
	serve(t, app.NewGRPCServer(a.runtime, a.identity, storage.New(a.dir), grpc.Creds(creds)), ln)
	return a
}

// createServer creates a server on the agent of a node.
func (m *master) createServer(t *testing.T, a agent, name string, typ mcsmv1.ServerType, port uint32) network.Ref {
	conn, err := m.nodes.Conn(t.Context(), a.node.ID)
	check(t, err)
	res, err := mcsmv1.NewServerServiceClient(conn).CreateServer(t.Context(), &mcsmv1.CreateServerRequest{
		Name: name, Type: typ, MemoryMb: 1024, Port: port, AcceptEula: true,
	})
	check(t, err)
	return network.Ref{NodeID: a.node.ID, ServerID: res.GetServer().GetId()}
}

type apiClient struct {
	t   *testing.T
	url string
}

// do sends a request with an optional body, JSON unless it is []byte, checks the status and decodes the
// response into out unless it is nil. It returns the response body.
func (c apiClient) do(method, path string, in any, wantStatus int, out any) string {
	c.t.Helper()
	var body io.Reader
	switch in := in.(type) {
	case nil:
	case []byte: // sent as is, e.g. an uploaded file
		body = bytes.NewReader(in)
	default:
		data, err := json.Marshal(in)
		check(c.t, err)
		body = bytes.NewReader(data)
	}
	req, err := http.NewRequestWithContext(c.t.Context(), method, c.url+path, body)
	check(c.t, err)
	res, err := http.DefaultClient.Do(req)
	check(c.t, err)
	defer res.Body.Close()
	data, err := io.ReadAll(res.Body)
	check(c.t, err)
	if res.StatusCode != wantStatus {
		c.t.Fatalf("%s %s: status %d, want %d: %s", method, path, res.StatusCode, wantStatus, data)
	}
	if out != nil {
		check(c.t, json.Unmarshal(data, out))
	}
	return string(data)
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

// fakeRuntime keeps servers and their network configuration in memory and their data
// in a directory.
type fakeRuntime struct {
	dir      string
	mu       sync.Mutex
	servers  []runtime.Server
	networks map[string]runtime.Network
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
	return os.Mkdir(filepath.Join(f.dir, spec.ID), 0o750)
}

func (f *fakeRuntime) Data(_ context.Context, id string) (*datadir.Dir, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if !slices.ContainsFunc(f.servers, func(s runtime.Server) bool { return s.ID == id }) {
		return nil, runtime.ErrNotFound
	}
	return datadir.Open(filepath.Join(f.dir, id))
}

func (f *fakeRuntime) Start(_ context.Context, id string) error {
	return f.setState(id, mcsmv1.ServerState_SERVER_STATE_RUNNING)
}

func (f *fakeRuntime) Stop(_ context.Context, id string) error {
	return f.setState(id, mcsmv1.ServerState_SERVER_STATE_STOPPED)
}

func (f *fakeRuntime) Update(_ context.Context, spec runtime.Spec) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	for i := range f.servers {
		if f.servers[i].ID == spec.ID {
			f.servers[i].Spec = spec
			return nil
		}
	}
	return runtime.ErrNotFound
}

func (f *fakeRuntime) Restart(_ context.Context, id string) error {
	return f.setState(id, mcsmv1.ServerState_SERVER_STATE_RUNNING)
}

func (f *fakeRuntime) Remove(context.Context, string) error { return runtime.ErrNotFound }

func (f *fakeRuntime) Logs(context.Context, string, int) iter.Seq2[string, error] {
	return func(yield func(string, error) bool) {
		for _, line := range []string{"\x1b[32m[INFO]: Starting\x1b[m", "[INFO]: Done\r"} {
			if !yield(line, nil) {
				return
			}
		}
	}
}

func (f *fakeRuntime) SendCommand(_ context.Context, _, command string) (string, error) {
	return "§6ran " + command, nil
}

func (f *fakeRuntime) Configure(_ context.Context, id string, network runtime.Network) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.networks == nil {
		f.networks = map[string]runtime.Network{}
	}
	f.networks[id] = network
	return nil
}

func (f *fakeRuntime) network(id string) runtime.Network {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.networks[id]
}

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
