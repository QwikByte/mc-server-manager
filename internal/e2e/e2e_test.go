// Package e2e tests master and agent together over real mutually authenticated connections.
package e2e

import (
	"bytes"
	"cmp"
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"io"
	"iter"
	"net"
	"net/http"
	"net/http/cookiejar"
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
	agentlogs "github.com/QwikByte/mc-server-manager/internal/agent/logs"
	agentnode "github.com/QwikByte/mc-server-manager/internal/agent/node"
	"github.com/QwikByte/mc-server-manager/internal/agent/runtime"
	"github.com/QwikByte/mc-server-manager/internal/agent/storage"
	"github.com/QwikByte/mc-server-manager/internal/master/access"
	masterapp "github.com/QwikByte/mc-server-manager/internal/master/app"
	"github.com/QwikByte/mc-server-manager/internal/master/auth"
	"github.com/QwikByte/mc-server-manager/internal/master/backup"
	"github.com/QwikByte/mc-server-manager/internal/master/database"
	"github.com/QwikByte/mc-server-manager/internal/master/logs"
	"github.com/QwikByte/mc-server-manager/internal/master/modrinth"
	"github.com/QwikByte/mc-server-manager/internal/master/network"
	"github.com/QwikByte/mc-server-manager/internal/master/node"
	"github.com/QwikByte/mc-server-manager/internal/master/plugin"
	"github.com/QwikByte/mc-server-manager/internal/master/policy"
	"github.com/QwikByte/mc-server-manager/internal/master/schedule"
	"github.com/QwikByte/mc-server-manager/internal/master/server"
	"github.com/QwikByte/mc-server-manager/internal/master/settings"
	"github.com/QwikByte/mc-server-manager/internal/master/template"
	"github.com/QwikByte/mc-server-manager/internal/master/update"
	"github.com/QwikByte/mc-server-manager/internal/master/usage"
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
	cert       *pki.Holder
	settings   *settings.Service
	nodes      *node.Service
	logs       *logs.Store
	enrollAddr string
	modrinth   *fakeModrinth
	update     update.Options
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
	conf, err := settings.Load(t.Context(), db, settings.Master{Version: "test", EnrollAddr: ln.Addr().String()}, masterCert)
	check(t, err)
	nodes := node.NewService(db, ca, masterCert, conf)
	t.Cleanup(nodes.Close)
	logStore := logs.NewStore(db, logs.NewNames(nodes), conf.LogRetention)
	logStore.Start(t.Context())
	t.Cleanup(logStore.Close)
	enrollServer := grpc.NewServer(grpc.Creds(credentials.NewTLS(pki.MasterServerTLS(masterCert))))
	mcsmv1.RegisterEnrollmentServiceServer(enrollServer, nodes)
	serve(t, enrollServer, ln)
	return &master{
		db: db, ca: ca, cert: masterCert, settings: conf, nodes: nodes, logs: logStore,
		enrollAddr: ln.Addr().String(), modrinth: startModrinth(t), update: update.Options{DataDir: dir},
	}
}

// services are those of a master's panel.
func (m *master) services(t *testing.T) masterapp.Services {
	nodes := m.nodes
	plugins := plugin.NewService(nodes, modrinth.New(m.modrinth.URL+"/v2", m.modrinth.URL+"/cdn/"))
	moves := server.NewMoves()
	tasks := schedule.NewService(m.db, nodes, map[string]schedule.Kind{backup.TaskKind: backup.NewJobs(nodes), policy.TaskKind: policy.New(nodes)}, moves.Busy)
	check(t, tasks.Start(t.Context()))
	return masterapp.Services{
		Users: auth.NewService(m.db), Access: access.NewService(m.db), Settings: m.settings, Nodes: nodes,
		Networks: network.NewService(m.db, nodes), Plugins: plugins, Templates: template.NewService(m.db, plugins), Tasks: tasks,
		Logs: m.logs, Updates: update.New(nodes, m.settings, m.update), Usage: usage.NewStore(m.db, nodes),
		Moves: moves,
	}
}

// panel serves the REST API of the master for an administrator, without signing in.
func (m *master) panel(t *testing.T) *httptest.Server {
	api := masterapp.API(m.services(t))
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		api.ServeHTTP(w, r.WithContext(access.WithGrants(r.Context(), access.Admin())))
	}))
	t.Cleanup(srv.Close)
	return srv
}

// agent is an enrolled agent that serves a fake runtime.
type agent struct {
	node     node.Node
	token    node.JoinToken
	dir      string
	identity *agentnode.Identity
	runtime  *fakeRuntime
	log      *agentlogs.Buffer
}

// startAgent registers a node and enrolls its agent with a join token.
func (m *master) startAgent(t *testing.T, name string) agent {
	ln := listen(t)
	n, token, err := m.nodes.Create(t.Context(), name, ln.Addr().String())
	check(t, err)
	a := agent{node: n, token: token, dir: t.TempDir(), runtime: &fakeRuntime{dir: t.TempDir()}, log: agentlogs.NewBuffer()}
	check(t, enroll.Run(t.Context(), token.String(), a.dir))
	a.identity, err = agentnode.LoadIdentity(a.dir)
	check(t, err)
	creds := credentials.NewTLS(pki.AgentServerTLS(a.identity.Holder, a.identity.CA))
	serve(t, app.NewGRPCServer(a.runtime, a.identity, storage.New(a.dir), a.log, grpc.Creds(creds)), ln)
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

// browser is a client of a panel served over HTTPS, for its secure session cookie, that
// keeps its own cookies like the browser of a user.
func browser(t *testing.T, srv *httptest.Server) apiClient {
	client := *srv.Client()
	jar, err := cookiejar.New(nil)
	check(t, err)
	client.Jar = jar
	return apiClient{t: t, url: srv.URL, client: &client}
}

type apiClient struct {
	t      *testing.T
	url    string
	client *http.Client // http.DefaultClient if nil
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
	res, err := cmp.Or(c.client, http.DefaultClient).Do(req)
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
	commands []string
	// createErr makes creating servers fail, e.g. on a node that is full.
	createErr error
}

func (f *fakeRuntime) Info(context.Context) (runtime.Info, error) {
	return runtime.Info{Name: "fake", CPUs: 4, MemoryBytes: 8 << 30}, nil
}

func (f *fakeRuntime) List(context.Context) ([]runtime.Server, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	return append([]runtime.Server(nil), f.servers...), nil
}

func (f *fakeRuntime) Create(_ context.Context, spec runtime.Spec) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.createErr != nil {
		return f.createErr
	}
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

func (f *fakeRuntime) Remove(_ context.Context, id string) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	i := slices.IndexFunc(f.servers, func(s runtime.Server) bool { return s.ID == id })
	if i < 0 {
		return runtime.ErrNotFound
	}
	f.servers = slices.Delete(f.servers, i, i+1)
	return os.RemoveAll(filepath.Join(f.dir, id))
}

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
	f.mu.Lock()
	defer f.mu.Unlock()
	f.commands = append(f.commands, command)
	return "§6ran " + command, nil
}

func (f *fakeRuntime) Usage(_ context.Context, id string) (runtime.Usage, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	i := slices.IndexFunc(f.servers, func(s runtime.Server) bool { return s.ID == id })
	if i < 0 || f.servers[i].State == mcsmv1.ServerState_SERVER_STATE_STOPPED {
		return runtime.Usage{}, runtime.ErrNotRunning
	}
	return runtime.Usage{MemoryBytes: 512 << 20, MemoryLimit: 1 << 30}, nil
}

func (f *fakeRuntime) consoleCommands() []string {
	f.mu.Lock()
	defer f.mu.Unlock()
	return slices.Clone(f.commands)
}

func (f *fakeRuntime) Duplicate(ctx context.Context, from string, spec runtime.Spec) error {
	if err := datadir.Copy(ctx, filepath.Join(f.dir, from), filepath.Join(f.dir, spec.ID)); err != nil {
		return err
	}
	f.mu.Lock()
	defer f.mu.Unlock()
	f.servers = append(f.servers, runtime.Server{Spec: spec, State: mcsmv1.ServerState_SERVER_STATE_STOPPED})
	return nil
}

func (f *fakeRuntime) spec(id string) runtime.Spec {
	f.mu.Lock()
	defer f.mu.Unlock()
	for _, s := range f.servers {
		if s.ID == id {
			return s.Spec
		}
	}
	return runtime.Spec{}
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
