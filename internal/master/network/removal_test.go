package network

import (
	"reflect"
	"testing"
)

// A network goes on without the game servers of a removed node, unless it loses its proxy or
// all of them, and players no longer join or reach them.
func TestWithoutNode(t *testing.T) {
	n := Network{
		Proxy:    Ref{"n1", "proxy"},
		Backends: []Backend{{Ref: Ref{"n2", "lobby"}, Name: "lobby"}, {Ref: Ref{"n1", "survival"}, Name: "survival"}, {Ref: Ref{"n3", "skyblock"}, Name: "skyblock"}},
		Try:      []string{"lobby", "survival"},
		ForcedHosts: []ForcedHost{
			{Host: "lobby.example.com", Servers: []string{"lobby"}},
			{Host: "play.example.com", Servers: []string{"lobby", "skyblock"}},
		},
	}
	if !n.endsWith("n1") || n.endsWith("n2") || n.endsWith("n4") {
		t.Fatal("a network ends with a node other than its proxy's")
	}
	kept := n.without("n2")
	want := Network{
		Proxy:       n.Proxy,
		Backends:    []Backend{n.Backends[1], n.Backends[2]},
		Try:         []string{"survival"},
		ForcedHosts: []ForcedHost{{Host: "play.example.com", Servers: []string{"skyblock"}}},
	}
	if !reflect.DeepEqual(kept, want) {
		t.Fatalf("without n2 = %+v, want %+v", kept, want)
	}
	// Without the servers players join, they join the first one left.
	if kept = kept.without("n1"); !reflect.DeepEqual(kept.Try, []string{"skyblock"}) || !kept.endsWith("n3") {
		t.Fatalf("without n2 and n1 = %+v", kept)
	}
	if len(n.Backends) != 3 || len(n.Try) != 2 || len(n.ForcedHosts[1].Servers) != 2 {
		t.Fatalf("the network changed: %+v", n)
	}
}
