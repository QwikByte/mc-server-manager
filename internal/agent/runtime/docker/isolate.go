package docker

import (
	"github.com/google/nftables"
	"github.com/google/nftables/expr"
	"golang.org/x/sys/unix"
)

// Servers in the shared network can't reach each other. Docker keeps them apart itself, as
// the network doesn't let its containers communicate. Podman ignores that option of the
// network, but names its bridge after it, so an nftables table of the agent drops what that
// bridge would forward from one container to another. Podman starts servers at boot itself,
// so a systemd unit sets up the table before (noryx-agent isolate), and the agent again as it
// starts servers.

// sharedBridge is the name of the bridge of the shared network on Podman.
const sharedBridge = sharedNetwork

var isolation = &nftables.Table{Family: nftables.TableFamilyBridge, Name: "noryx"}

// Isolate keeps the servers in the shared network of Podman from reaching each other. It
// replaces the table in one transaction, so it may run any time. It needs the bridge support
// of nftables in the kernel (nft_meta_bridge).
func Isolate() error {
	conn, err := nftables.New()
	if err != nil {
		return err
	}
	conn.AddTable(isolation) // so that deleting it never fails
	conn.DelTable(isolation)
	conn.AddTable(isolation)
	forward := conn.AddChain(&nftables.Chain{
		Name: "forward", Table: isolation, Type: nftables.ChainTypeFilter, Hooknum: nftables.ChainHookForward,
		Priority: nftables.ChainPriorityFilter, Policy: new(nftables.ChainPolicyAccept),
	})
	name := make([]byte, unix.IFNAMSIZ)
	copy(name, sharedBridge)
	conn.AddRule(&nftables.Rule{Table: isolation, Chain: forward, Exprs: []expr.Any{
		// meta ibrname "noryx-servers" drop
		&expr.Meta{Key: expr.MetaKeyBRIIIFNAME, Register: 1},
		&expr.Cmp{Op: expr.CmpOpEq, Register: 1, Data: name},
		&expr.Verdict{Kind: expr.VerdictDrop},
	}})
	return conn.Flush()
}
