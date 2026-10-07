package overlay

import (
	"fmt"
	"net/netip"
	"reflect"
	"testing"

	"golang.zx2c4.com/wireguard/wgctrl/wgtypes"

	noryxv1 "github.com/QwikByte/noryx/api/noryx/v1"
)

// fakeKernel keeps the state that was applied last.
type fakeKernel struct{ applied State }

func (*fakeKernel) Check() error { return nil }

func (k *fakeKernel) Apply(st State, _ wgtypes.Key, _ *State) error {
	k.applied = st
	return nil
}

func (*fakeKernel) Peers() ([]PeerStatus, error) { return nil, nil }

func (*fakeKernel) Remove() error { return nil }

func (*fakeKernel) HostNets() ([]netip.Prefix, error) { return nil, nil }

// A node that leaves the network no longer reaches the ports published for it, as a node
// that joins later may get its address.
func TestClientsLeave(t *testing.T) {
	dataDir := t.TempDir()
	if err := Allow(dataDir); err != nil {
		t.Fatal(err)
	}
	k := &fakeKernel{}
	s := NewService(dataDir, k)
	peer := func(n int) *noryxv1.OverlayPeer {
		key, err := wgtypes.GeneratePrivateKey()
		if err != nil {
			t.Fatal(err)
		}
		return &noryxv1.OverlayPeer{PublicKey: key.PublicKey().String(), Address: fmt.Sprintf("10.213.0.%d", n), Endpoint: fmt.Sprintf("203.0.113.%d:51820", n)}
	}
	configure := func(peers ...*noryxv1.OverlayPeer) {
		t.Helper()
		req := &noryxv1.ConfigureOverlayRequest{Address: "10.213.0.3/24", Port: 51820, Mtu: 1420, Peers: peers}
		if _, err := s.ConfigureOverlay(t.Context(), req); err != nil {
			t.Fatal(err)
		}
	}
	first, second := peer(1), peer(2)
	configure(first, second)
	if _, err := s.Admit("server", 25570, "10.213.0.1"); err != nil {
		t.Fatal(err)
	}
	if _, err := s.Admit("datastore", 3306, "10.213.0.1", "10.213.0.2"); err != nil {
		t.Fatal(err)
	}

	configure(second)
	want := map[string]Client{"datastore": {Port: 3306, Address: netip.MustParseAddr("10.213.0.2")}}
	st, err := s.state()
	if err != nil || !reflect.DeepEqual(k.applied.Clients, want) || !reflect.DeepEqual(st.Clients, want) {
		t.Fatalf("clients applied %+v, stored %+v (%v)", k.applied.Clients, st, err)
	}
	// The address may go to another node, which reaches none of the ports.
	configure(second, peer(1))
	if !reflect.DeepEqual(k.applied.Clients, want) {
		t.Fatalf("clients = %+v", k.applied.Clients)
	}
}
