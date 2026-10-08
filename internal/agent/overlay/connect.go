package overlay

import (
	"context"
	"errors"
	"fmt"
	"net/netip"
	"os"
	"slices"
	"sync"
	"syscall"
	"time"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	noryxv1 "github.com/QwikByte/noryx/api/noryx/v1"
)

// A test connects to at most maxTestPorts ports, each within testTimeout. The node opens
// testBurst test connections at once, and then testRate a second, so that tests scan nothing.
const (
	maxTestPorts = 32
	testTimeout  = 3 * time.Second
	testBurst    = 64
	testRate     = 4
)

// TestOverlayPeer connects to ports that a peer publishes for this node, e.g. to find out why
// a proxy doesn't reach a server on it. So that a master can't use it to scan networks, it
// connects only through the interface, to the address that the configuration gives the peer
// and that WireGuard sends to it, and only to the ports that the configuration says the peer
// publishes for this node.
func (s *Service) TestOverlayPeer(ctx context.Context, req *noryxv1.TestOverlayPeerRequest) (*noryxv1.TestOverlayPeerResponse, error) {
	peer, err := s.testedPeer(req)
	if err != nil {
		return nil, err
	}
	if !s.tests.AllowN(time.Now(), len(req.GetPorts())) {
		return nil, status.Error(codes.ResourceExhausted, "This node tested many connections just now. Try again in a few seconds.")
	}
	res := &noryxv1.TestOverlayPeerResponse{Results: make([]*noryxv1.OverlayPortTest, len(req.GetPorts()))}
	var wg sync.WaitGroup
	for i, port := range req.GetPorts() {
		addr := netip.AddrPortFrom(peer.Address, uint16(port)) //nolint:gosec // checked
		wg.Go(func() {
			ctx, cancel := context.WithTimeout(ctx, testTimeout)
			defer cancel()
			start := time.Now()
			err := s.kernel.Connect(ctx, addr)
			took := time.Since(start).Microseconds()
			res.Results[i] = &noryxv1.OverlayPortTest{Port: port, Error: describe(err), DurationMicros: uint32(took)} //nolint:gosec // within testTimeout
		})
	}
	wg.Wait()
	return res, nil
}

// testedPeer returns the peer with the key of a test if it publishes all ports of the test
// for this node, and if the interface sends its address to it.
func (s *Service) testedPeer(req *noryxv1.TestOverlayPeerRequest) (Peer, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if !allowed(s.dir) {
		return Peer{}, errNotAllowed
	}
	st, err := s.state()
	switch {
	case err != nil:
		return Peer{}, status.Error(codes.Internal, err.Error())
	case st == nil:
		return Peer{}, status.Error(codes.FailedPrecondition, "This node is no member of the private network.")
	case len(req.GetPorts()) == 0 || len(req.GetPorts()) > maxTestPorts:
		return Peer{}, status.Errorf(codes.InvalidArgument, "Test from 1 to %d ports at once.", maxTestPorts)
	}
	i := slices.IndexFunc(st.Peers, func(p Peer) bool { return p.PublicKey == req.GetPublicKey() })
	if i < 0 {
		return Peer{}, status.Error(codes.NotFound, "No peer of this node has this key.")
	}
	peer := st.Peers[i]
	for _, port := range req.GetPorts() {
		if port > 65535 || !slices.Contains(peer.Ports, uint16(port)) {
			return Peer{}, status.Errorf(codes.InvalidArgument, "The peer %s publishes no port %d for this node.", peer.Address, port)
		}
	}
	peers, err := s.kernel.Peers()
	if err != nil {
		return Peer{}, status.Error(codes.Internal, err.Error())
	}
	j := slices.IndexFunc(peers, func(p PeerStatus) bool { return p.PublicKey == peer.PublicKey })
	if j < 0 || !slices.Contains(peers[j].AllowedIPs, netip.PrefixFrom(peer.Address, 32)) {
		return Peer{}, status.Errorf(codes.FailedPrecondition, "The interface %s has no peer %s with this key.", Interface, peer.Address)
	}
	return peer, nil
}

// describe returns why a connection failed, or "" if it didn't.
func describe(err error) string {
	var errno syscall.Errno
	switch {
	case err == nil:
		return ""
	case errors.Is(err, context.DeadlineExceeded) || os.IsTimeout(err):
		return fmt.Sprintf("no answer within %v", testTimeout)
	case errors.As(err, &errno):
		return errno.Error()
	}
	return err.Error()
}
