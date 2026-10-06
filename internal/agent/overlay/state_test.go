package overlay

import (
	"net/netip"
	"testing"
)

func TestCheck(t *testing.T) {
	host := []netip.Prefix{netip.MustParsePrefix("203.0.113.0/24"), netip.MustParsePrefix("172.17.0.0/16")}
	valid := func() State {
		return State{
			Address: netip.MustParsePrefix("10.213.0.3/24"), Port: 51820, MTU: 1420,
			Peers: []Peer{
				{PublicKey: "b", Address: netip.MustParseAddr("10.213.0.1"), Endpoint: netip.MustParseAddrPort("203.0.113.1:51820")},
				{PublicKey: "c", Address: netip.MustParseAddr("10.213.0.2"), Endpoint: netip.MustParseAddrPort("198.51.100.2:51820")},
			},
		}
	}
	if err := valid().check("a", host); err != nil {
		t.Fatal(err)
	}
	for name, edit := range map[string]func(st *State){
		"public network":          func(st *State) { st.Address = netip.MustParsePrefix("100.64.0.3/24") },
		"too wide":                func(st *State) { st.Address = netip.MustParsePrefix("10.213.0.3/8") },
		"too narrow":              func(st *State) { st.Address = netip.MustParsePrefix("10.213.0.1/30") },
		"IPv6":                    func(st *State) { st.Address = netip.MustParsePrefix("fd00::3/64") },
		"network address":         func(st *State) { st.Address = netip.MustParsePrefix("10.213.0.0/24") },
		"broadcast address":       func(st *State) { st.Address = netip.MustParsePrefix("10.213.0.255/24") },
		"overlaps a Docker route": func(st *State) { st.Address = netip.MustParsePrefix("172.17.5.3/24") },
		"privileged port":         func(st *State) { st.Port = 123 },
		"MTU":                     func(st *State) { st.MTU = 9000 },
		"peer outside":            func(st *State) { st.Peers[0].Address = netip.MustParseAddr("192.168.1.1") },
		"peer with own address":   func(st *State) { st.Peers[0].Address = st.Address.Addr() },
		"peer with own key":       func(st *State) { st.Peers[0].PublicKey = "a" },
		"address twice":           func(st *State) { st.Peers[1].Address = st.Peers[0].Address },
		"key twice":               func(st *State) { st.Peers[1].PublicKey = "b" },
		"peer broadcast":          func(st *State) { st.Peers[0].Address = netip.MustParseAddr("10.213.0.255") },
	} {
		st := valid()
		edit(&st)
		if st.check("a", host) == nil {
			t.Errorf("%s: accepted", name)
		}
	}
}
