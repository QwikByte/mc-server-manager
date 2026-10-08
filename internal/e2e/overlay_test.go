package e2e

import (
	"net"
	"net/http"
	"net/http/httptest"
	"net/netip"
	"reflect"
	"slices"
	"testing"
	"time"

	noryxv1 "github.com/QwikByte/noryx/api/noryx/v1"
	agentoverlay "github.com/QwikByte/noryx/internal/agent/overlay"
	"github.com/QwikByte/noryx/internal/master/access"
	masterapp "github.com/QwikByte/noryx/internal/master/app"
	"github.com/QwikByte/noryx/internal/master/auth"
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
	// The port is bound to node-1's key, so that no other node that gets its address reaches it.
	client := agentoverlay.Client{Port: 25570, Address: netip.MustParseAddr("10.213.0.1"), Keys: []string{a1.kernel.key.PublicKey().String()}}
	if got := a2.runtime.network(survival.ServerID).Overlay; got != "10.213.0.2" || !reflect.DeepEqual(a2.kernel.applied().Clients[survival.ServerID], client) {
		t.Fatalf("survival publishes its port at %q, clients %+v", got, a2.kernel.applied().Clients)
	}
	// A move to a node outside the network needs the firewall again.
	api.do("POST", "/api/nodes/"+survival.NodeID+"/servers/"+survival.ServerID+"/move", map[string]any{"node": a3.node.ID}, http.StatusConflict, nil)

	var status overlay.Status
	api.do("GET", path(a1), nil, http.StatusOK, &status)
	if status.Member == nil || len(status.Peers) != 1 || status.Peers[0].NodeID != a2.node.ID || status.Peers[0].LatestHandshake == nil {
		t.Fatalf("status = %+v", status)
	}
	// node-2 tells that it publishes survival's port for node-1, and whether its firewall is in place.
	api.do("GET", path(a2), nil, http.StatusOK, &status)
	published := []overlay.Published{{Port: 25570, ServerID: survival.ServerID, Clients: []overlay.Client{{NodeID: a1.node.ID, Address: "10.213.0.1"}}}}
	if status.Firewall != "in_place" || !reflect.DeepEqual(status.Published, published) {
		t.Fatalf("node-2 = %+v", status)
	}
	a2.kernel.set(agentoverlay.ErrNoTable)
	if api.do("GET", path(a2), nil, http.StatusOK, &status); status.Firewall != "missing" {
		t.Fatalf("node-2 without its firewall = %+v", status)
	}
	a2.kernel.set(nil)
	// node-1 tests the port: refused while nothing listens there, reached once something does.
	// It learns the port as one that node-2 publishes for it.
	test := func() []overlay.PortTest {
		t.Helper()
		var tests []overlay.PortTest
		api.do("POST", path(a1)+"/peers/"+a2.node.ID+"/test", nil, http.StatusOK, &tests)
		return tests
	}
	if got := test(); len(got) != 1 || got[0].Port != 25570 || got[0].ServerID != survival.ServerID || got[0].Error != "connection refused" {
		t.Fatalf("tests %+v", got)
	}
	if ports := a1.kernel.applied().Peers[0].Ports; !slices.Equal(ports, []uint16{25570}) {
		t.Fatalf("node-1 has the ports %v of node-2", ports)
	}
	a1.kernel.set(nil, netip.MustParseAddrPort("10.213.0.2:25570"))
	if got := test(); len(got) != 1 || got[0].Error != "" {
		t.Fatalf("tests %+v", got)
	}
	api.do("POST", path(a1)+"/peers/"+a3.node.ID+"/test", nil, http.StatusNotFound, nil)
	// Like the overview, the status names only the peers on the nodes a user may see, and only
	// the servers the user may see.
	viewerAPI := masterapp.API(m.services(t))
	viewer := func(name string, nodeID string) apiClient {
		t.Helper()
		var group access.Group
		api.do("POST", "/api/groups", map[string]any{"name": name, "permissions": []string{"nodes.view"}, "targets": []access.Target{{NodeID: nodeID}}}, http.StatusCreated, &group)
		var invited struct{ User auth.User }
		api.do("POST", "/api/users", map[string]any{"username": name, "groups": []string{group.ID}}, http.StatusCreated, &invited)
		grants, err := access.NewService(m.db).Grants(t.Context(), invited.User.ID)
		check(t, err)
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			viewerAPI.ServeHTTP(w, r.WithContext(access.WithGrants(r.Context(), grants)))
		}))
		t.Cleanup(srv.Close)
		return apiClient{t: t, url: srv.URL}
	}
	var seen overlay.Status
	viewer1 := viewer("viewer1", a1.node.ID)
	viewer1.do("GET", path(a1), nil, http.StatusOK, &seen)
	if seen.Member == nil || len(seen.Peers) != 0 {
		t.Fatalf("status for a user who sees node-1 only = %+v", seen)
	}
	viewer("viewer2", a2.node.ID).do("GET", path(a2), nil, http.StatusOK, &seen)
	if len(seen.Published) != 1 || seen.Published[0].ServerID != "" || len(seen.Published[0].Clients) != 0 {
		t.Fatalf("published for a user who sees node-2 but not its servers = %+v", seen.Published)
	}
	// Only those who manage the private network test it.
	viewer1.do("POST", path(a1)+"/peers/"+a2.node.ID+"/test", nil, http.StatusForbidden, nil)
	// A new key of node-2 reaches node-1.
	before := a1.kernel.applied().Peers[0].PublicKey
	api.do("POST", path(a2)+"/rotate", nil, http.StatusOK, nil)
	if after := a1.kernel.applied().Peers[0].PublicKey; after == before || after != a2.kernel.key.PublicKey().String() {
		t.Fatalf("node-1 has the key %s of node-2, before %s", after, before)
	}
	// A new key of node-1, the proxy's, is let in again, as its network is applied again.
	old := client.Keys[0]
	api.do("POST", path(a1)+"/rotate", nil, http.StatusOK, nil)
	client.Keys = []string{a1.kernel.key.PublicKey().String()}
	if got := a2.kernel.applied().Clients[survival.ServerID]; client.Keys[0] == old || !reflect.DeepEqual(got, client) {
		t.Fatalf("survival's client after node-1's new key: %+v, want %+v, before %s", got, client, old)
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
