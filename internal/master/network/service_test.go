package network

import (
	"slices"
	"testing"
)

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

func TestValidate(t *testing.T) {
	proxy, other := Ref{"n1", "proxy"}, Ref{"n2", "survival"}
	valid := func(proxyType string) Network {
		return Network{
			Name: "Main", Proxy: proxy, ProxyType: proxyType, Forwarding: Modern,
			Backends:    []Backend{{Ref: Ref{"n1", "lobby"}, Name: "lobby", Restricted: true, Motd: "Hi"}, {Ref: other, Name: "survival"}},
			Try:         []string{"lobby"},
			ForcedHosts: []ForcedHost{{Host: " Survival.Example.com ", Servers: []string{"survival", "lobby"}}},
		}
	}
	n := valid("velocity")
	if err := n.validate(); err != nil {
		t.Fatal(err)
	}
	// Host names are compared in lower case, and Velocity has no settings of BungeeCord.
	if n.ForcedHosts[0].Host != "survival.example.com" || n.Backends[0].Restricted || n.Backends[0].Motd != "" {
		t.Fatalf("canonical network = %+v", n)
	}
	for name, edit := range map[string]func(n *Network){
		"no name":               func(n *Network) { n.Name = " " },
		"unknown forwarding":    func(n *Network) { n.Forwarding = "none" },
		"modern BungeeCord":     func(n *Network) { n.ProxyType = "bungeecord"; n.ForcedHosts[0].Servers = n.ForcedHosts[0].Servers[:1] },
		"no servers":            func(n *Network) { n.Backends, n.Try, n.ForcedHosts = nil, nil, nil },
		"nobody joins":          func(n *Network) { n.Try = nil },
		"unknown server in try": func(n *Network) { n.Try = []string{"nope"} },
		"server twice in try":   func(n *Network) { n.Try = []string{"lobby", "lobby"} },
		"name in capitals":      func(n *Network) { n.Backends[0].Name, n.Try = "Lobby", []string{"Lobby"} },
		"name try":              func(n *Network) { n.Backends[0].Name, n.Try = "try", []string{"try"} },
		"same name twice":       func(n *Network) { n.Backends[1].Name = "lobby" },
		"same server twice":     func(n *Network) { n.Backends[1].Ref = n.Backends[0].Ref },
		"proxy as server":       func(n *Network) { n.Backends[1].Ref = proxy },
		"host name with port":   func(n *Network) { n.ForcedHosts[0].Host = "a.example.com:25565" },
		"host name twice": func(n *Network) {
			n.ForcedHosts = append(n.ForcedHosts, ForcedHost{"SURVIVAL.example.com", []string{"lobby"}})
		},
		"host name without server": func(n *Network) { n.ForcedHosts[0].Servers = nil },
		"exposed legacy servers":   func(n *Network) { n.Forwarding = Legacy },
	} {
		n := valid("velocity")
		edit(&n)
		if n.validate() == nil {
			t.Errorf("%s: accepted", name)
		}
	}

	// BungeeCord sends a host name to one server and keeps the settings of the servers.
	n = valid("waterfall")
	n.Forwarding, n.Firewalled = Legacy, true
	n.ForcedHosts[0].Servers = []string{"survival"}
	if err := n.validate(); err != nil || !n.Backends[0].Restricted || n.Backends[0].Motd != "Hi" {
		t.Fatalf("BungeeCord network = %+v, %v", n, err)
	}
	n.ForcedHosts[0].Servers = []string{"survival", "lobby"}
	if n.validate() == nil {
		t.Error("BungeeCord accepted two servers for a host name")
	}
}

func TestExposed(t *testing.T) {
	n := Network{Proxy: Ref{"n1", "proxy"}, Forwarding: Legacy, Backends: []Backend{{Ref: Ref{"n1", "lobby"}}}}
	if n.exposed() {
		t.Fatal("a server on the proxy's node is exposed")
	}
	// Moving the server, or the proxy, to another node exposes the server.
	for _, move := range []string{"lobby", "proxy"} {
		moved := n
		moved.Backends = slices.Clone(n.Backends)
		moved.moved(move, "n1", "n2")
		if !moved.exposed() {
			t.Errorf("moving %s didn't expose the server", move)
		}
		if moved.Firewalled = true; moved.exposed() {
			t.Errorf("moving %s exposed the server behind a firewall", move)
		}
	}
	if n.Forwarding = Modern; n.exposed() {
		t.Fatal("modern forwarding is exposed")
	}
}

func TestSendCommand(t *testing.T) {
	backends := []Backend{{Name: "lobby"}, {Name: "survival-2"}}
	for _, tt := range []struct {
		proxy, player string
		alone         bool
	}{
		{"velocity", "Steve", true},
		{"velocity", "Lobby", true},
		{"velocity", "all", false},
		{"bungeecord", "Current", false},
		{"bungeecord", "Lobby", false}, // BungeeCord sends the players of the server lobby
		{"waterfall", "survival_2", true},
	} {
		n := Network{ProxyType: tt.proxy, Backends: backends}
		if command, alone := n.SendCommand(tt.player, "survival-2"); command != "send "+tt.player+" survival-2" || alone != tt.alone {
			t.Errorf("%s: SendCommand(%q) = %q, %v", tt.proxy, tt.player, command, alone)
		}
	}
}
