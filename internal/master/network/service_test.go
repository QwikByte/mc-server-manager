package network

import "testing"

func TestBackendName(t *testing.T) {
	existing := []Backend{{Name: "survival"}, {Name: "survival-2"}}
	for server, want := range map[string]string{
		"Lobby":                                "lobby",
		"Survival":                             "survival-3",
		"  Skyblock #1 ":                       "skyblock-1",
		"Try":                                  "try-server",
		"Ünïcode ✓":                            "n-code",
		"!!!":                                  "server",
		"A very long server name that is long": "a-very-long-server-name-that",
	} {
		if got := backendName(server, existing); got != want {
			t.Errorf("backendName(%q) = %q, want %q", server, got, want)
		}
	}
}
