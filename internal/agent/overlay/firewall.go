package overlay

import (
	"maps"
	"net/netip"
	"slices"

	"github.com/google/nftables"
	"github.com/google/nftables/binaryutil"
	"github.com/google/nftables/expr"
	"golang.org/x/sys/unix"
)

// table is the nftables table of the network. It works next to Docker's rules, whether
// Docker writes them with iptables or nftables, as its chains only drop.
var table = &nftables.Table{Family: nftables.TableFamilyINet, Name: "noryx"}

// writeFirewall replaces the table of the network in one transaction:
//   - Packets for the node's address in the network only come in through the interface, not
//     from neighbours on other interfaces, which Docker would forward to published ports.
//   - Nothing in the network reaches the services of the node itself, such as SSH or the agent.
//   - From the network, a published port is only reached by its client, and nothing else is
//     forwarded, e.g. directly to a container.
//   - TCP connections through the interface fit its MTU.
func writeFirewall(st State) error {
	conn, err := nftables.New()
	if err != nil {
		return err
	}
	addr := st.Address.Addr()
	conn.AddTable(table) // so that deleting it never fails
	conn.DelTable(table)
	conn.AddTable(table)
	chain(conn, "prerouting", nftables.ChainHookPrerouting, -150, join(ipv4, daddr(addr), iifname(expr.CmpOpNeq, Interface), iifname(expr.CmpOpNeq, "lo"), drop))
	chain(conn, "input", nftables.ChainHookInput, -1, join(iifname(expr.CmpOpEq, Interface), ctNew, drop))
	forward := [][]expr.Any{clampMSS(expr.MetaKeyIIFNAME), clampMSS(expr.MetaKeyOIFNAME)}
	for _, id := range slices.Sorted(maps.Keys(st.Clients)) {
		c := st.Clients[id]
		forward = append(forward, join(iifname(expr.CmpOpEq, Interface), ipv4, tcp, ctOriginal(expr.CtKeyDST, addr.AsSlice()),
			ctOriginal(expr.CtKeyPROTODST, binaryutil.BigEndian.PutUint16(c.Port)), saddr(c.Address), accept))
	}
	forward = append(forward, join(iifname(expr.CmpOpEq, Interface), ctNew, drop))
	chain(conn, "forward", nftables.ChainHookForward, -1, forward...)
	return conn.Flush()
}

// removeFirewall removes the table of the network, if there is one.
func removeFirewall() error {
	conn, err := nftables.New()
	if err != nil {
		return err
	}
	conn.AddTable(table)
	conn.DelTable(table)
	return conn.Flush()
}

// chain adds a base chain that accepts what its rules don't drop.
func chain(conn *nftables.Conn, name string, hook *nftables.ChainHook, priority nftables.ChainPriority, rules ...[]expr.Any) {
	c := conn.AddChain(&nftables.Chain{
		Name: name, Table: table, Type: nftables.ChainTypeFilter, Hooknum: hook,
		Priority: nftables.ChainPriorityRef(priority), Policy: new(nftables.ChainPolicyAccept),
	})
	for _, r := range rules {
		conn.AddRule(&nftables.Rule{Table: table, Chain: c, Exprs: r})
	}
}

func join(parts ...[]expr.Any) []expr.Any { return slices.Concat(parts...) }

var (
	drop   = []expr.Any{&expr.Verdict{Kind: expr.VerdictDrop}}
	accept = []expr.Any{&expr.Verdict{Kind: expr.VerdictAccept}}
	ipv4   = meta(expr.MetaKeyNFPROTO, unix.NFPROTO_IPV4)
	tcp    = meta(expr.MetaKeyL4PROTO, unix.IPPROTO_TCP)
	ctNew  = []expr.Any{
		&expr.Ct{Register: 1, Key: expr.CtKeySTATE},
		&expr.Bitwise{SourceRegister: 1, DestRegister: 1, Len: 4, Mask: binaryutil.NativeEndian.PutUint32(expr.CtStateBitNEW), Xor: make([]byte, 4)},
		&expr.Cmp{Op: expr.CmpOpNeq, Register: 1, Data: make([]byte, 4)},
	}
)

func meta(key expr.MetaKey, value byte) []expr.Any {
	return []expr.Any{&expr.Meta{Key: key, Register: 1}, &expr.Cmp{Op: expr.CmpOpEq, Register: 1, Data: []byte{value}}}
}

// iifname compares the name of the interface a packet came in through.
func iifname(op expr.CmpOp, name string) []expr.Any {
	return []expr.Any{&expr.Meta{Key: expr.MetaKeyIIFNAME, Register: 1}, &expr.Cmp{Op: op, Register: 1, Data: ifname(name)}}
}

func ifname(name string) []byte {
	b := make([]byte, unix.IFNAMSIZ)
	copy(b, name)
	return b
}

// daddr and saddr match the destination and source of IPv4 packets.
func daddr(a netip.Addr) []expr.Any { return address(16, a) }
func saddr(a netip.Addr) []expr.Any { return address(12, a) }

func address(offset uint32, a netip.Addr) []expr.Any {
	return []expr.Any{
		&expr.Payload{DestRegister: 1, Base: expr.PayloadBaseNetworkHeader, Offset: offset, Len: 4},
		&expr.Cmp{Op: expr.CmpOpEq, Register: 1, Data: a.AsSlice()},
	}
}

// ctOriginal matches a part of the connection as it was before Docker forwarded it to a
// container, e.g. its destination port.
func ctOriginal(key expr.CtKey, value []byte) []expr.Any {
	return []expr.Any{
		&expr.Ct{Register: 1, Key: key, Direction: 0}, // IP_CT_DIR_ORIGINAL
		&expr.Cmp{Op: expr.CmpOpEq, Register: 1, Data: value},
	}
}

// clampMSS fits TCP connections that come in or go out through the interface to its MTU.
func clampMSS(direction expr.MetaKey) []expr.Any {
	return join(
		[]expr.Any{&expr.Meta{Key: direction, Register: 1}, &expr.Cmp{Op: expr.CmpOpEq, Register: 1, Data: ifname(Interface)}},
		tcp,
		[]expr.Any{
			// tcp flags & (syn | rst) == syn
			&expr.Payload{DestRegister: 1, Base: expr.PayloadBaseTransportHeader, Offset: 13, Len: 1},
			&expr.Bitwise{SourceRegister: 1, DestRegister: 1, Len: 1, Mask: []byte{0x06}, Xor: []byte{0}},
			&expr.Cmp{Op: expr.CmpOpEq, Register: 1, Data: []byte{0x02}},
			// tcp option maxseg size set rt mtu
			&expr.Rt{Register: 1, Key: expr.RtTCPMSS},
			&expr.Exthdr{SourceRegister: 1, Type: 2, Offset: 2, Len: 2, Op: expr.ExthdrOpTcpopt},
		},
	)
}
