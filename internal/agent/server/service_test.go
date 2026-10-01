package server

import (
	"testing"

	mcsmv1 "github.com/QwikByte/mc-server-manager/api/mcsm/v1"
	"github.com/QwikByte/mc-server-manager/internal/agent/runtime"
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
	valid := runtime.Spec{Name: "Lobby", Type: mcsmv1.ServerType_SERVER_TYPE_PAPER, Version: "LATEST", MemoryMB: 2048, Port: 25565}
	tests := []struct {
		name   string
		change func(*runtime.Spec)
		ok     bool
	}{
		{"defaults", func(*runtime.Spec) {}, true},
		{"all settings", func(s *runtime.Spec) {
			s.Java, s.AikarFlags, s.CPUMillis = "17", true, 2500
			s.RestartPolicy = mcsmv1.RestartPolicy_RESTART_POLICY_ON_CRASH
			s.JVMOptions = []string{"-Dfile.encoding=UTF-8", "-XX:+UseZGC", "-javaagent:/data/agent.jar"}
		}, true},
		{"unknown Java", func(s *runtime.Spec) { s.Java = "22" }, false},
		{"Java for a proxy", func(s *runtime.Spec) { s.Type, s.Java = mcsmv1.ServerType_SERVER_TYPE_VELOCITY, "21" }, false},
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
}
