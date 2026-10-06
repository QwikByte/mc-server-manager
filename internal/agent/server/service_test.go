package server

import (
	"context"
	"slices"
	"sync"
	"testing"
	"time"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	noryxv1 "github.com/QwikByte/noryx/api/noryx/v1"
	"github.com/QwikByte/noryx/internal/agent/runtime"
)

func TestPlainRemovesFormatting(t *testing.T) {
	for in, want := range map[string]string{
		"\x1b[0;32m[12:00:00 INFO]: Done (3.2s)!\x1b[m\r": "[12:00:00 INFO]: Done (3.2s)!",
		"§6There are §c2§6 of a max of §A20§r players":    "There are 2 of a max of 20 players",
		"plain line": "plain line",
	} {
		if got := plain(in); got != want {
			t.Errorf("plain(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestCheckSettings(t *testing.T) {
	valid := runtime.Spec{Name: "Lobby", Type: noryxv1.ServerType_SERVER_TYPE_PAPER, Version: "LATEST", MemoryMB: 2048, Port: 25565}
	tests := []struct {
		name   string
		change func(*runtime.Spec)
		ok     bool
	}{
		{"defaults", func(*runtime.Spec) {}, true},
		{"all settings", func(s *runtime.Spec) {
			s.Java, s.AikarFlags, s.CPUMillis = "17", true, 2500
			s.RestartPolicy = noryxv1.RestartPolicy_RESTART_POLICY_ON_CRASH
			s.JVMOptions = []string{
				"-Dfile.encoding=UTF-8", "-XX:+UseZGC", "-XX:+HeapDumpOnOutOfMemoryError", "-Dcom.example.agent=x", "-Djava.awt.headless=true",
				"-Dlog4j2.formatMsgNoLookups=true", "-XX:+PrintFlagsFinal", "--add-modules=jdk.incubator.vector", "--enable-native-access=ALL-UNNAMED",
				"-Dusing.aikars.flags=https://mcflags.emc.gs", "-Xss4M",
			}
		}, true},
		{"unknown Java", func(s *runtime.Spec) { s.Java = "22" }, false},
		{"Java for a proxy", func(s *runtime.Spec) { s.Type, s.Java = noryxv1.ServerType_SERVER_TYPE_VELOCITY, "21" }, false},
		{"option with a space", func(s *runtime.Spec) { s.JVMOptions = []string{"-Dx=a b"} }, false},
		{"shell syntax", func(s *runtime.Spec) { s.JVMOptions = []string{"-Dx=$(id)"} }, false},
		{"glob", func(s *runtime.Spec) { s.JVMOptions = []string{"-cp*"} }, false},
		{"no dash", func(s *runtime.Spec) { s.JVMOptions = []string{"evil.jar"} }, false},
		{"memory override", func(s *runtime.Spec) { s.JVMOptions = []string{"-Xmx64G"} }, false},
		{"more CPUs than the node", func(s *runtime.Spec) { s.CPUMillis = 8001 }, false},
		{"tiny CPU limit", func(s *runtime.Spec) { s.CPUMillis = 50 }, false},
		{"unknown restart policy", func(s *runtime.Spec) { s.RestartPolicy = 9 }, false},
		{"loader version", func(s *runtime.Spec) { s.Type, s.LoaderVersion = noryxv1.ServerType_SERVER_TYPE_FORGE, "1.20.1-47.3.0" }, true},
		{"loader version without loader", func(s *runtime.Spec) { s.LoaderVersion = "0.16.10" }, false},
		{"loader version with shell syntax", func(s *runtime.Spec) { s.Type, s.LoaderVersion = noryxv1.ServerType_SERVER_TYPE_FABRIC, "$(id)" }, false},
	}
	for _, tt := range tests {
		spec := valid
		tt.change(&spec)
		if msg := checkSettings(spec, 8); (msg == "") != tt.ok {
			t.Errorf("%s: checkSettings() = %q, want ok = %v", tt.name, msg, tt.ok)
		}
	}
	// Changing settings can't run code: no commands, agents or debugging and management ports,
	// and no options, classes, libraries or configurations loaded from files or URLs.
	for _, option := range []string{
		"-javaagent:/data/agent.jar", "-agentpath:/data/libx.so", "-agentlib:jdwp=transport=dt_socket,server=y,address=5005",
		"-Xrunjdwp:server=y", "-Xbootclasspath/a:/data/x.jar", "-XX:OnOutOfMemoryError=/data/x.sh", "-XX:OnError=/data/x.sh",
		"-Dcom.sun.management.jmxremote.port=9010", "-Dcom.sun.management.config.file=/data/x",
		"-XX:VMOptionsFile=/data/x", "-XX:Flags=/data/x", "-XX:SharedArchiveFile=/data/x.jsa", "-XX:AOTCache=/data/x.aot",
		"-XX:+EnableJVMCI", "-XX:JVMCILibPath=/data", "-XX:CRaCRestoreFrom=/data/cr",
		"-Dlog4j2.configurationFile=https://example.com/x.xml", "-Dlog4j.configurationFile=/data/x.xml", "-DLOG4J2_configurationFile=/data/x",
		"-Dlogback.configurationFile=/data/x.xml", "-Djava.system.class.loader=x.Loader", "-Djava.library.path=/data",
		"-Djava.util.logging.config.class=x.Config", "-Djna.library.path=/data", "-Djdk.module.patch.0=java.base=/data/x",
		"-cp", "--class-path=/data/x.jar", "--module-path=/data/mods", "--patch-module=java.base=/data/x", "-jar",
	} {
		spec := valid
		spec.JVMOptions = []string{option}
		if checkSettings(spec, 8) == "" {
			t.Errorf("%s accepted", option)
		}
	}
}

func TestNetworkOf(t *testing.T) {
	const id, secret = "abcdefghijklmnopqrstuvwxyz", "S3cretS3cretS3cret"
	backends := []*noryxv1.NetworkBackend{
		{Name: "lobby", Target: &noryxv1.NetworkBackend_ServerId{ServerId: "bcdefghijklmnopqrstuvwxyz2"}},
		{Name: "survival", Target: &noryxv1.NetworkBackend_Address{Address: "203.0.113.7:25566"}, Restricted: true, Motd: "&aSurvival"},
	}
	valid := func() *noryxv1.ConfigureNetworkRequest {
		return &noryxv1.ConfigureNetworkRequest{
			Id: id, ForwardingSecret: secret, Forwarding: noryxv1.Forwarding_FORWARDING_MODERN, Backends: backends,
			Try: []string{"lobby", "survival"}, ForcedHosts: []*noryxv1.ForcedHost{{Host: "survival.example.com", Servers: []string{"survival"}}},
		}
	}
	tests := []struct {
		name   string
		change func(*noryxv1.ConfigureNetworkRequest)
		ok     bool
	}{
		{"valid", func(*noryxv1.ConfigureNetworkRequest) {}, true},
		{"legacy without secret", func(r *noryxv1.ConfigureNetworkRequest) {
			r.Forwarding, r.ForwardingSecret = noryxv1.Forwarding_FORWARDING_LEGACY, ""
		}, true},
		{"modern without secret", func(r *noryxv1.ConfigureNetworkRequest) { r.ForwardingSecret = "" }, false},
		{"unknown forwarding", func(r *noryxv1.ConfigureNetworkRequest) { r.Forwarding = 9 }, false},
		{"try names an unknown server", func(r *noryxv1.ConfigureNetworkRequest) { r.Try = []string{"nope"} }, false},
		{"try names a server twice", func(r *noryxv1.ConfigureNetworkRequest) { r.Try = []string{"lobby", "lobby"} }, false},
		{"forced host with a port", func(r *noryxv1.ConfigureNetworkRequest) { r.ForcedHosts[0].Host = "survival.example.com:25565" }, false},
		{"forced host in capitals", func(r *noryxv1.ConfigureNetworkRequest) { r.ForcedHosts[0].Host = "Survival.example.com" }, false},
		{"forced host without servers", func(r *noryxv1.ConfigureNetworkRequest) { r.ForcedHosts[0].Servers = nil }, false},
		{"duplicate forced host", func(r *noryxv1.ConfigureNetworkRequest) { r.ForcedHosts = append(r.ForcedHosts, r.ForcedHosts[0]) }, false},
		{"MOTD with control characters", func(r *noryxv1.ConfigureNetworkRequest) {
			r.Backends = []*noryxv1.NetworkBackend{{Name: "lobby", Target: backends[0].Target, Motd: "\x1b[31m"}}
			r.Try, r.ForcedHosts = []string{"lobby"}, nil
		}, false},
	}
	for _, tt := range tests {
		req := valid()
		tt.change(req)
		if _, msg := networkOf(req); (msg == "") != tt.ok {
			t.Errorf("%s: networkOf() = %q, want ok = %v", tt.name, msg, tt.ok)
		}
	}

	// Older masters send a secret without forwarding, and let players join the first backend.
	n, msg := networkOf(&noryxv1.ConfigureNetworkRequest{Id: id, ForwardingSecret: secret, Backends: backends})
	if msg != "" || n.Forwarding != runtime.ForwardingModern || len(n.Try) != 1 || n.Try[0] != "lobby" {
		t.Fatalf("network of an older master = %+v, %q", n, msg)
	}
	if n, _ := networkOf(&noryxv1.ConfigureNetworkRequest{Id: id, ProxyOnNode: true}); n.Forwarding != runtime.ForwardingNone || n.ProxyOnNode {
		t.Fatalf("leaving a network = %+v", n)
	}
}

// slowRuntime creates servers slowly, as Docker does while it pulls an image.
type slowRuntime struct {
	runtime.Runtime
	mu      sync.Mutex
	servers []runtime.Server
}

func (r *slowRuntime) Info(context.Context) (runtime.Info, error) { return runtime.Info{CPUs: 4}, nil }

func (r *slowRuntime) ListDatastores(context.Context) ([]runtime.Datastore, error) { return nil, nil }

func (r *slowRuntime) List(context.Context) ([]runtime.Server, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	return slices.Clone(r.servers), nil
}

func (r *slowRuntime) Create(_ context.Context, spec runtime.Spec) error {
	time.Sleep(50 * time.Millisecond)
	r.mu.Lock()
	defer r.mu.Unlock()
	r.servers = append(r.servers, runtime.Server{Spec: spec})
	return nil
}

// Of concurrent requests for the same port, e.g. after a double click, only one creates a
// server.
func TestCreateServerReservesThePort(t *testing.T) {
	rt := &slowRuntime{}
	s := NewService(rt, nil, nil)
	errs := make(chan error, 3)
	for range cap(errs) {
		go func() {
			_, err := s.CreateServer(t.Context(), &noryxv1.CreateServerRequest{
				Name: "Lobby", Type: noryxv1.ServerType_SERVER_TYPE_PAPER, MemoryMb: 1024, Port: 25565, AcceptEula: true,
			})
			errs <- err
		}()
	}
	var refused int
	for range cap(errs) {
		if err := <-errs; status.Code(err) == codes.AlreadyExists {
			refused++
		} else if err != nil {
			t.Fatal(err)
		}
	}
	if servers, _ := rt.List(t.Context()); refused != 2 || len(servers) != 1 {
		t.Fatalf("%d servers created, %d refused", len(servers), refused)
	}
	// Once the server exists, the port is free to reserve for it again, e.g. to change it.
	if len(s.reserved) != 0 {
		t.Fatalf("ports still reserved: %v", s.reserved)
	}
}

// A server keeps JVM options set before the agent refused them, and the list reports them.
func TestRefusedJVMOptions(t *testing.T) {
	options := []string{"-Dfile.encoding=UTF-8", "-XX:VMOptionsFile=/data/opts", "-Djava.system.class.loader=Evil"}
	rt := &slowRuntime{servers: []runtime.Server{{Spec: runtime.Spec{ID: runtime.NewID(), JVMOptions: options}}}}
	res, err := NewService(rt, nil, nil).ListServers(t.Context(), &noryxv1.ListServersRequest{})
	if err != nil {
		t.Fatal(err)
	}
	if got := res.GetServers()[0].GetRefusedJvmOptions(); !slices.Equal(got, options[1:]) {
		t.Fatalf("refused options = %q, want %q", got, options[1:])
	}
	if got := res.GetServers()[0].GetJvmOptions(); !slices.Equal(got, options) {
		t.Fatalf("options = %q, want all of them", got)
	}
}

// Game servers run Minecraft, so they need the EULA accepted; proxies don't.
func TestCreateServerNeedsTheEULAForGameServers(t *testing.T) {
	s := NewService(&slowRuntime{}, nil, nil)
	create := func(typ noryxv1.ServerType, port uint32) error {
		_, err := s.CreateServer(t.Context(), &noryxv1.CreateServerRequest{Name: "Server", Type: typ, MemoryMb: 1024, Port: port})
		return err
	}
	if err := create(noryxv1.ServerType_SERVER_TYPE_PAPER, 25565); status.Code(err) != codes.InvalidArgument {
		t.Fatalf("game server without the EULA: %v", err)
	}
	if err := create(noryxv1.ServerType_SERVER_TYPE_VELOCITY, 25577); err != nil {
		t.Fatalf("proxy without the EULA: %v", err)
	}
}
