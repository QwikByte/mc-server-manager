package server

import (
	"testing"

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
			s.JVMOptions = []string{"-Dfile.encoding=UTF-8", "-XX:+UseZGC", "-XX:+HeapDumpOnOutOfMemoryError", "-Dcom.example.agent=x"}
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
	}
	for _, tt := range tests {
		spec := valid
		tt.change(&spec)
		if msg := checkSettings(spec, 8); (msg == "") != tt.ok {
			t.Errorf("%s: checkSettings() = %q, want ok = %v", tt.name, msg, tt.ok)
		}
	}
	// Changing settings can't run code: no commands, agents or debugging and management ports.
	for _, option := range []string{
		"-javaagent:/data/agent.jar", "-agentpath:/data/libx.so", "-agentlib:jdwp=transport=dt_socket,server=y,address=5005",
		"-Xrunjdwp:server=y", "-Xbootclasspath/a:/data/x.jar", "-XX:OnOutOfMemoryError=/data/x.sh", "-XX:OnError=/data/x.sh",
		"-Dcom.sun.management.jmxremote.port=9010",
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
