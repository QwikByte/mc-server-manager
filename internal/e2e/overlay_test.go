package e2e

import (
	"net"
	"net/http"
	"net/netip"
	"slices"
	"testing"
	"time"

	noryxv1 "github.com/QwikByte/noryx/api/noryx/v1"
	agentoverlay "github.com/QwikByte/noryx/internal/agent/overlay"
	"github.com/QwikByte/noryx/internal/master/network"
	"github.com/QwikByte/noryx/internal/master/overlay"
)

// Nodes whose administrators allowed it join the private network, and proxies reach the
// backends of other members over it, without a firewall for legacy forwarding.
func TestOverlay(t *testing.T) {
	m := startMaster(t)
	a1, a2, a3 := m.startAgent(t, "node-1"), m.startAgent(t, "node-2"), m.startAgent(t, "node-3")
	proxy := m.createServer(t, a1, "Proxy", noryxv1.ServerType_SERVER_TYPE_VELOCITY, 25577)
	survival := m.createServer(t, a2, "Survival", noryxv1.ServerType_SERVER_TYPE_PAPER, 25570)
	api := apiClient{t: t, url: m.panel(t).URL}
	path := func(a agent) string { return "/api/nodes/" + a.node.ID + "/overlay" }
	join := func(a agent, endpoint string, status int) overlay.Member {
		t.Helper()
		var member overlay.Member
		api.do("POST", path(a), map[string]string{"endpoint": endpoint}, status, &member)
		return member
	}

	// The agents listen at 127.0.0.1, where no other node reaches them, and only nodes whose
	// administrator allowed it join.
	join(a1, "", http.StatusBadRequest)
	join(a1, "203.0.113.1:51820", http.StatusConflict)
	for _, a := range []agent{a1, a2, a3} {
		check(t, agentoverlay.Allow(a.dir))
	}
	if got := join(a1, "203.0.113.1:51820", http.StatusOK); got.Address != "10.213.0.1" {
		t.Fatalf("member = %+v", got)
	}
	join(a2, "203.0.113.2:51820", http.StatusOK)
	// A node whose own networks overlap the range doesn't join.
	a3.kernel.host = []netip.Prefix{netip.MustParsePrefix("10.0.0.0/8")}
	join(a3, "203.0.113.3:51820", http.StatusConflict)
	peers := func(a agent) []string {
		var addrs []string
		if st := a.kernel.applied(); st != nil {
			for _, p := range st.Peers {
				addrs = append(addrs, p.Address.String()+"@"+p.Endpoint.String())
			}
		}
		return addrs
	}
	if p1, p2 := peers(a1), peers(a2); !slices.Equal(p1, []string{"10.213.0.2@203.0.113.2:51820"}) || !slices.Equal(p2, []string{"10.213.0.1@203.0.113.1:51820"}) {
		t.Fatalf("peers %q and %q", p1, p2)
	}

	// The proxy reaches survival at node-2's address in the network, without a firewall
	// for legacy forwarding, and only node-1 may reach its port there.
	var n network.Network
	api.do("POST", "/api/networks", map[string]any{"name": "Main", "proxy": proxy, "forwarding": "legacy", "servers": []network.Ref{survival}}, http.StatusCreated, &n)
	if got := a1.runtime.network(proxy.ServerID).Backends[0].Address; got != "10.213.0.2:25570" {
		t.Fatalf("the proxy reaches survival at %s", got)
	}
	client := agentoverlay.Client{Port: 25570, Address: netip.MustParseAddr("10.213.0.1")}
	if got := a2.runtime.network(survival.ServerID).Overlay; got != "10.213.0.2" || a2.kernel.applied().Clients[survival.ServerID] != client {
		t.Fatalf("survival publishes its port at %q, clients %+v", got, a2.kernel.applied().Clients)
	}
	// A move to a node outside the network needs the firewall again.
	api.do("POST", "/api/nodes/"+survival.NodeID+"/servers/"+survival.ServerID+"/move", map[string]any{"node": a3.node.ID}, http.StatusConflict, nil)

	var status overlay.Status
	api.do("GET", path(a1), nil, http.StatusOK, &status)
	if status.Member == nil || len(status.Peers) != 1 || status.Peers[0].NodeID != a2.node.ID || status.Peers[0].LatestHandshake == nil {
		t.Fatalf("status = %+v", status)
	}
	// A new key of node-2 reaches node-1.
	before := a1.kernel.applied().Peers[0].PublicKey
	api.do("POST", path(a2)+"/rotate", nil, http.StatusOK, nil)
	if after := a1.kernel.applied().Peers[0].PublicKey; after == before || after != a2.kernel.key.PublicKey().String() {
		t.Fatalf("node-1 has the key %s of node-2, before %s", after, before)
	}
	// The range only changes without members, the port right away.
	api.do("PUT", "/api/overlay", overlay.Settings{Subnet: "10.214.0.0/24", Port: 51820, MTU: 1420}, http.StatusConflict, nil)
	api.do("PUT", "/api/overlay", overlay.Settings{Subnet: "10.213.0.0/24", Port: 51821, MTU: 1380}, http.StatusOK, nil)
	if st := a1.kernel.applied(); st.Port != 51821 || st.MTU != 1380 {
		t.Fatalf("node-1 = %+v", st)
	}

	// Leaving would expose survival, until a firewall is confirmed. Then the network reaches
	// it at its public port again.
	api.do("DELETE", path(a2), nil, http.StatusConflict, nil)
	change := network.Change{Name: n.Name, Forwarding: network.Legacy, Firewalled: true, Backends: n.Backends, Try: n.Try, ForcedHosts: n.ForcedHosts}
	api.do("PUT", "/api/networks/"+n.ID, change, http.StatusOK, nil)
	api.do("DELETE", path(a2), nil, http.StatusNoContent, nil)
	host, _, _ := net.SplitHostPort(a2.node.Address)
	if got := a1.runtime.network(proxy.ServerID).Backends[0].Address; got != net.JoinHostPort(host, "25570") {
		t.Fatalf("the proxy reaches survival at %s after node-2 left", got)
	}
	if a2.runtime.network(survival.ServerID).Overlay != "" || a2.kernel.applied() != nil || len(peers(a1)) != 0 {
		t.Fatalf("after node-2 left: survival %+v, node-2 %+v, peers of node-1 %q", a2.runtime.network(survival.ServerID), a2.kernel.applied(), peers(a1))
	}

	// The others drop a member that is removed as node right away.
	a3.kernel.host = nil
	join(a3, "203.0.113.3:51820", http.StatusOK)
	api.do("DELETE", "/api/nodes/"+a3.node.ID, nil, http.StatusNoContent, nil)
	for deadline := time.Now().Add(5 * time.Second); len(peers(a1)) != 0; time.Sleep(10 * time.Millisecond) {
		if time.Now().After(deadline) {
			t.Fatalf("node-1 still has the peers %q", peers(a1))
		}
	}
}
