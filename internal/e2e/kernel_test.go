package e2e

import (
	"context"
	"maps"
	"net/netip"
	"slices"
	"sync"
	"syscall"
	"time"

	"golang.zx2c4.com/wireguard/wgctrl/wgtypes"

	"github.com/QwikByte/noryx/internal/agent/overlay"
)

// fakeKernel keeps the state of the private network that an agent applied, and says that
// every peer shook hands just now.
type fakeKernel struct {
	mu    sync.Mutex
	state *overlay.State
	key   wgtypes.Key
	// host are the addresses and routes of the node.
	host []netip.Prefix
	// firewall is what Firewall finds, and listening are the addresses that connections reach.
	firewall  error
	listening []netip.AddrPort
}

func (k *fakeKernel) Check() error { return nil }

func (k *fakeKernel) Apply(st overlay.State, key wgtypes.Key, _ *overlay.State) error {
	k.mu.Lock()
	defer k.mu.Unlock()
	st.Peers, st.Clients = slices.Clone(st.Peers), maps.Clone(st.Clients)
	k.state, k.key = &st, key
	return nil
}

func (k *fakeKernel) Peers() ([]overlay.PeerStatus, error) {
	k.mu.Lock()
	defer k.mu.Unlock()
	if k.state == nil {
		return nil, nil
	}
	peers := []overlay.PeerStatus{}
	for _, p := range k.state.Peers {
		peers = append(peers, overlay.PeerStatus{
			PublicKey: p.PublicKey, Endpoint: p.Endpoint.String(), LatestHandshake: time.Now(), AllowedIPs: []netip.Prefix{netip.PrefixFrom(p.Address, 32)},
		})
	}
	return peers, nil
}

func (k *fakeKernel) Remove() error {
	k.mu.Lock()
	defer k.mu.Unlock()
	k.state = nil
	return nil
}

func (k *fakeKernel) HostNets() ([]netip.Prefix, error) {
	k.mu.Lock()
	defer k.mu.Unlock()
	return slices.Clone(k.host), nil
}

func (k *fakeKernel) Firewall(overlay.State) error {
	k.mu.Lock()
	defer k.mu.Unlock()
	return k.firewall
}

func (k *fakeKernel) Connect(_ context.Context, addr netip.AddrPort) error {
	k.mu.Lock()
	defer k.mu.Unlock()
	if !slices.Contains(k.listening, addr) {
		return syscall.ECONNREFUSED
	}
	return nil
}

// set changes what the kernel finds of the firewall, and which addresses connections reach.
func (k *fakeKernel) set(firewall error, listening ...netip.AddrPort) {
	k.mu.Lock()
	defer k.mu.Unlock()
	k.firewall, k.listening = firewall, listening
}

// applied returns the state the agent applied last, or nil if it isn't a member.
func (k *fakeKernel) applied() *overlay.State {
	k.mu.Lock()
	defer k.mu.Unlock()
	if k.state == nil {
		return nil
	}
	st := *k.state
	return &st
}
