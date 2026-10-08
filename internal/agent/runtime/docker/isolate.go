package docker

import (
	"crypto/sha256"
	"encoding/hex"

	"github.com/google/nftables"
	"github.com/google/nftables/expr"
	"golang.org/x/sys/unix"
)

// Servers can't reach each other unless they share the network of a proxy, nor the networks of
// datastores they don't use. Docker keeps its networks apart itself, and the containers in the
// shared network, as the network doesn't let them communicate. Podman ignores that option of a
// network, and keeps networks apart only while its API remembers it: some versions, e.g. 4.9.3
// of Ubuntu 24.04, forget it once anyone inspects the network through Docker's API, as the
// agent does. So on Podman, the bridges of the agent's networks have names that start with
// bridgePrefix, and nftables tables of the agent drop what the bridge of the shared network
// would forward from one container to another, and what the node would route from one of these
// bridges to another. Podman starts servers at boot itself, so a systemd unit sets up the
// tables before (noryx-agent isolate), and the agent again as it starts containers.

const (
	// sharedBridge is the name of the bridge of the shared network on Podman.
	sharedBridge = sharedNetwork
	// bridgePrefix starts the names of the bridges of the agent's networks on Podman.
	bridgePrefix = "noryx-"
)

// bridge returns the name of the bridge of a network of the agent on Podman: the shared
// network's is its name, the others' a hash of theirs, as the name of an interface has at most
// 15 bytes. Podman refuses a name in use, so networks never share a bridge.
func bridge(network string) string {
	if network == sharedNetwork {
		return sharedBridge
	}
	sum := sha256.Sum256([]byte(network))
	return bridgePrefix + hex.EncodeToString(sum[:])[:unix.IFNAMSIZ-1-len(bridgePrefix)]
}

// isolation is a table of the agent with the rule of its forward chain.
type isolation struct {
	table *nftables.Table
	rule  []expr.Any
}

var (
	// meta ibrname "noryx-servers" drop
	sharedIsolation = isolation{&nftables.Table{Family: nftables.TableFamilyBridge, Name: "noryx"}, []expr.Any{
		&expr.Meta{Key: expr.MetaKeyBRIIIFNAME, Register: 1},
		&expr.Cmp{Op: expr.CmpOpEq, Register: 1, Data: append([]byte(sharedBridge), make([]byte, unix.IFNAMSIZ-len(sharedBridge))...)},
		&expr.Verdict{Kind: expr.VerdictDrop},
	}}
	// iifname "noryx-*" oifname "noryx-*" fib daddr . iif oif missing drop: the destination
	// isn't behind the bridge the packet came from, which tells routed packets from those that
	// a bridge forwards within itself, which pass here too while br_netfilter is loaded.
	networkIsolation = isolation{&nftables.Table{Family: nftables.TableFamilyINet, Name: "noryx-networks"}, []expr.Any{
		&expr.Meta{Key: expr.MetaKeyIIFNAME, Register: 1},
		&expr.Cmp{Op: expr.CmpOpEq, Register: 1, Data: []byte(bridgePrefix)},
		&expr.Meta{Key: expr.MetaKeyOIFNAME, Register: 1},
		&expr.Cmp{Op: expr.CmpOpEq, Register: 1, Data: []byte(bridgePrefix)},
		&expr.Fib{Register: 1, FlagDADDR: true, FlagIIF: true, ResultOIF: true},
		&expr.Cmp{Op: expr.CmpOpEq, Register: 1, Data: make([]byte, 4)},
		&expr.Verdict{Kind: expr.VerdictDrop},
	}}
)

// Isolate keeps the servers of Podman apart. It replaces the tables in one transaction, so it
// may run any time. It needs nftables with its bridge support (nft_meta_bridge) and its lookups
// of routes (nft_fib_inet) in the kernel.
func Isolate() error { return isolate(sharedIsolation, networkIsolation) }

func isolate(tables ...isolation) error {
	conn, err := nftables.New()
	if err != nil {
		return err
	}
	for _, t := range tables {
		conn.AddTable(t.table) // so that deleting it never fails
		conn.DelTable(t.table)
		conn.AddTable(t.table)
		forward := conn.AddChain(&nftables.Chain{
			Name: "forward", Table: t.table, Type: nftables.ChainTypeFilter, Hooknum: nftables.ChainHookForward,
			Priority: nftables.ChainPriorityFilter, Policy: new(nftables.ChainPolicyAccept),
		})
		conn.AddRule(&nftables.Rule{Table: t.table, Chain: forward, Exprs: t.rule})
	}
	return conn.Flush()
}
