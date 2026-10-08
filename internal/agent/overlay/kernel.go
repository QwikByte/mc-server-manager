package overlay

import (
	"context"
	"errors"
	"net"
	"net/netip"
	"os"
	"slices"
	"syscall"
	"time"

	"github.com/jsimonetti/rtnetlink/v2"
	"golang.org/x/sys/unix"
	"golang.zx2c4.com/wireguard/wgctrl"
	"golang.zx2c4.com/wireguard/wgctrl/wgtypes"
)

// Interface is the WireGuard interface of the network.
const Interface = "noryx0"

// keepalive keeps the sessions of nodes behind NAT open.
const keepalive = 25 * time.Second

// Kernel puts the state of the network into effect on the node; tests replace it.
type Kernel interface {
	// Check returns why the node can't run WireGuard, or nil if it can.
	Check() error
	// Apply creates or updates the interface and its firewall rules. A peer keeps the
	// endpoint it roamed to unless the master changed it since previous, which may be nil.
	Apply(st State, key wgtypes.Key, previous *State) error
	// Peers returns the state of the interface's peers.
	Peers() ([]PeerStatus, error)
	// Remove removes the interface and its firewall rules.
	Remove() error
	// HostNets returns the addresses and routes of the node outside the interface, except
	// default routes.
	HostNets() ([]netip.Prefix, error)
	// Firewall returns nil if the firewall rules are in place as Apply wrote them for st,
	// ErrNoTable if they are missing, or else what differs.
	Firewall(st State) error
	// Connect opens a TCP connection through the interface to addr and closes it at once,
	// without sending anything.
	Connect(ctx context.Context, addr netip.AddrPort) error
}

// PeerStatus is the state of a peer of the interface.
type PeerStatus struct {
	PublicKey       string
	Endpoint        string
	LatestHandshake time.Time
	Received, Sent  int64
	// AllowedIPs are the addresses the interface sends to the peer, and accepts from it.
	AllowedIPs []netip.Prefix
}

var errNoWireGuard = errors.New("the kernel has no WireGuard. Linux 5.6 and newer have it, RHEL 9 only as an unsupported Technology Preview")

// Linux is the kernel of the node: a WireGuard interface, configured through rtnetlink and
// WireGuard's netlink API, and a table of nftables. Both outlive the agent.
type Linux struct{}

func (Linux) Check() error {
	return withLink(func(conn *rtnetlink.Conn, index uint32) error {
		if index != 0 {
			return nil
		}
		index, err := createLink(conn, 1420)
		if err != nil {
			return err
		}
		return conn.Link.Delete(index)
	})
}

func (Linux) Apply(st State, key wgtypes.Key, previous *State) error {
	return withLink(func(conn *rtnetlink.Conn, index uint32) error {
		var err error
		if index == 0 {
			index, err = createLink(conn, st.MTU)
		} else {
			err = conn.Link.Set(&rtnetlink.LinkMessage{Family: unix.AF_UNSPEC, Index: index, Attributes: &rtnetlink.LinkAttributes{MTU: uint32(st.MTU)}}) //nolint:gosec // checked
		}
		// The firewall is in place before the interface takes packets.
		if err == nil {
			err = writeFirewall(st)
		}
		if err == nil {
			err = setAddress(conn, index, st.Address)
		}
		if err == nil {
			err = configureDevice(st, key, previous)
		}
		if err != nil {
			return err
		}
		return conn.Link.Set(&rtnetlink.LinkMessage{Family: unix.AF_UNSPEC, Index: index, Flags: unix.IFF_UP, Change: unix.IFF_UP})
	})
}

func (Linux) Peers() ([]PeerStatus, error) {
	c, err := wgctrl.New()
	if err != nil {
		return nil, err
	}
	defer c.Close()
	dev, err := c.Device(Interface)
	if errors.Is(err, os.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	peers := make([]PeerStatus, 0, len(dev.Peers))
	for _, p := range dev.Peers {
		s := PeerStatus{PublicKey: p.PublicKey.String(), LatestHandshake: p.LastHandshakeTime, Received: p.ReceiveBytes, Sent: p.TransmitBytes}
		if p.Endpoint != nil {
			s.Endpoint = p.Endpoint.String()
		}
		for _, n := range p.AllowedIPs {
			if ip, ok := netip.AddrFromSlice(n.IP); ok {
				bits, _ := n.Mask.Size()
				s.AllowedIPs = append(s.AllowedIPs, netip.PrefixFrom(ip.Unmap(), bits))
			}
		}
		peers = append(peers, s)
	}
	return peers, nil
}

func (Linux) Firewall(st State) error { return checkFirewall(st) }

// Connect binds the connection to the interface, so that it leaves the node only through the
// tunnel, to the peer that WireGuard sends addr to.
func (Linux) Connect(ctx context.Context, addr netip.AddrPort) error {
	d := net.Dialer{Control: func(_, _ string, c syscall.RawConn) error {
		var err error
		if ctrlErr := c.Control(func(fd uintptr) { err = unix.BindToDevice(int(fd), Interface) }); ctrlErr != nil { //nolint:gosec // a file descriptor
			return ctrlErr
		}
		return err
	}}
	conn, err := d.DialContext(ctx, "tcp4", addr.String())
	if err != nil {
		return err
	}
	return conn.Close()
}

func (Linux) Remove() error {
	err := withLink(func(conn *rtnetlink.Conn, index uint32) error {
		if index == 0 {
			return nil
		}
		return conn.Link.Delete(index)
	})
	return errors.Join(err, removeFirewall())
}

func (Linux) HostNets() ([]netip.Prefix, error) {
	var nets []netip.Prefix
	err := withLink(func(conn *rtnetlink.Conn, index uint32) error {
		addrs, err := conn.Address.List()
		if err != nil {
			return err
		}
		for _, a := range addrs {
			if ip, ok := netip.AddrFromSlice(a.Attributes.Address); ok && a.Family == unix.AF_INET && a.Index != index {
				nets = append(nets, netip.PrefixFrom(ip.Unmap(), int(a.PrefixLength)).Masked())
			}
		}
		routes, err := conn.Route.List()
		if err != nil {
			return err
		}
		for _, r := range routes {
			if ip, ok := netip.AddrFromSlice(r.Attributes.Dst); ok && r.Family == unix.AF_INET && r.DstLength > 0 && r.Attributes.OutIface != index {
				nets = append(nets, netip.PrefixFrom(ip.Unmap(), int(r.DstLength)).Masked())
			}
		}
		return nil
	})
	return nets, err
}

// withLink calls fn with a connection to rtnetlink and the index of the interface, 0 if
// there is none.
func withLink(fn func(conn *rtnetlink.Conn, index uint32) error) error {
	conn, err := rtnetlink.Dial(nil)
	if err != nil {
		return err
	}
	defer conn.Close()
	links, err := conn.Link.List()
	if err != nil {
		return err
	}
	i := slices.IndexFunc(links, func(l rtnetlink.LinkMessage) bool { return l.Attributes != nil && l.Attributes.Name == Interface })
	if i < 0 {
		return fn(conn, 0)
	}
	if info := links[i].Attributes.Info; info == nil || info.Kind != "wireguard" {
		return errors.New(Interface + " exists, but isn't a WireGuard interface")
	}
	return fn(conn, links[i].Index)
}

// createLink creates the interface, which stays down until it is configured.
func createLink(conn *rtnetlink.Conn, mtu int) (uint32, error) {
	err := conn.Link.New(&rtnetlink.LinkMessage{Family: unix.AF_UNSPEC, Attributes: &rtnetlink.LinkAttributes{
		Name: Interface, MTU: uint32(mtu), Info: &rtnetlink.LinkInfo{Kind: "wireguard"}, //nolint:gosec // checked
	}})
	if errors.Is(err, unix.EOPNOTSUPP) {
		return 0, errNoWireGuard
	}
	if err != nil {
		return 0, err
	}
	ifc, err := net.InterfaceByName(Interface)
	if err != nil {
		return 0, err
	}
	return uint32(ifc.Index), nil //nolint:gosec // never negative
}

// setAddress gives the interface the node's address as its only one.
func setAddress(conn *rtnetlink.Conn, index uint32, prefix netip.Prefix) error {
	addrs, err := conn.Address.List()
	if err != nil {
		return err
	}
	found := false
	for _, a := range addrs {
		if a.Index != index {
			continue
		}
		if ip, ok := netip.AddrFromSlice(a.Attributes.Address); ok && netip.PrefixFrom(ip.Unmap(), int(a.PrefixLength)) == prefix {
			found = true
		} else if err := conn.Address.Delete(&a); err != nil {
			return err
		}
	}
	if found {
		return nil
	}
	ip := prefix.Addr().AsSlice()
	return conn.Address.New(&rtnetlink.AddressMessage{
		Family: unix.AF_INET, PrefixLength: uint8(prefix.Bits()), Index: index, //nolint:gosec // at most 32
		Attributes: &rtnetlink.AddressAttributes{Address: ip, Local: ip},
	})
}

// configureDevice sets the key, port and peers of the interface. It changes only what
// differs, so that the sessions with the peers stay.
func configureDevice(st State, key wgtypes.Key, previous *State) error {
	c, err := wgctrl.New()
	if err != nil {
		return err
	}
	defer c.Close()
	dev, err := c.Device(Interface)
	if err != nil {
		return err
	}
	cfg := wgtypes.Config{PrivateKey: &key, ListenPort: new(int(st.Port))}
	known := map[wgtypes.Key]bool{}
	for _, p := range dev.Peers {
		known[p.PublicKey] = true
		if !slices.ContainsFunc(st.Peers, func(q Peer) bool { return q.PublicKey == p.PublicKey.String() }) {
			cfg.Peers = append(cfg.Peers, wgtypes.PeerConfig{PublicKey: p.PublicKey, Remove: true})
		}
	}
	for _, p := range st.Peers {
		pk, _ := wgtypes.ParseKey(p.PublicKey) // checked
		pc := wgtypes.PeerConfig{
			PublicKey: pk, ReplaceAllowedIPs: true, PersistentKeepaliveInterval: new(keepalive),
			AllowedIPs: []net.IPNet{{IP: p.Address.AsSlice(), Mask: net.CIDRMask(32, 32)}},
		}
		if old, ok := previous.endpoint(p.PublicKey); !known[pk] || !ok || old != p.Endpoint {
			pc.Endpoint = net.UDPAddrFromAddrPort(p.Endpoint)
		}
		cfg.Peers = append(cfg.Peers, pc)
	}
	return c.ConfigureDevice(Interface, cfg)
}

// endpoint returns the endpoint of the peer with the key in the state, which may be nil.
func (st *State) endpoint(key string) (netip.AddrPort, bool) {
	if st == nil {
		return netip.AddrPort{}, false
	}
	i := slices.IndexFunc(st.Peers, func(p Peer) bool { return p.PublicKey == key })
	if i < 0 {
		return netip.AddrPort{}, false
	}
	return st.Peers[i].Endpoint, true
}
