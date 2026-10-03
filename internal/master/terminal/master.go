package terminal

import (
	"context"
	"fmt"
	"slices"
	"strings"
	"sync"
	"text/tabwriter"
	"time"

	"github.com/spf13/cobra"

	"github.com/QwikByte/mc-server-manager/internal/master/access"
	"github.com/QwikByte/mc-server-manager/internal/master/logs"
	"github.com/QwikByte/mc-server-manager/internal/master/node"
)

const (
	probeTimeout = 3 * time.Second
	renewTimeout = 30 * time.Second
)

// masterCommands describe the master and manage the nodes.
func (h *Handler) masterCommands() []*cobra.Command {
	nodes := &cobra.Command{Use: "node", Short: "Manage the nodes"}
	nodes.AddCommand(
		&cobra.Command{
			Use:   "list",
			Short: "List the nodes and whether their agents are online",
			Args:  cobra.NoArgs,
			RunE:  h.listNodes,
		},
		&cobra.Command{
			Use:   "renew <node>",
			Short: "Renew the certificate of a node right away, e.g. when its key may have leaked",
			Args:  cobra.ExactArgs(1),
			RunE:  h.renewNode,
		},
	)
	status := &cobra.Command{
		Use:   "status",
		Short: "Show the master and how many nodes are online",
		Args:  cobra.NoArgs,
		RunE:  h.status,
	}
	return []*cobra.Command{status, nodes, logs.Command(func() (*logs.Store, error) { return h.logs, nil })}
}

func (h *Handler) status(cmd *cobra.Command, _ []string) error {
	probes, err := h.probe(cmd.Context())
	if err != nil {
		return err
	}
	online := 0
	for _, p := range probes {
		if p.state == "online" {
			online++
		}
	}
	m := h.master.Master()
	panel := "HTTP, TLS by a reverse proxy"
	if m.PanelTLS {
		panel = "HTTPS"
	}
	fmt.Fprintf(cmd.OutOrStdout(), "Master   %s, running since %s\nPanel    %s (%s)\nEnroll   %s, join tokens name %s\nCA       %s\nCert     valid until %s, renewed automatically\nNodes    %d of %d online\n",
		m.Version, m.StartedAt.Local().Format(time.DateTime), m.PanelAddr, panel, m.EnrollListenAddr, h.master.EnrollAddr(),
		m.CAFingerprint, m.CertificateExpiresAt.Local().Format(time.DateOnly), online, len(probes))
	return nil
}

func (h *Handler) listNodes(cmd *cobra.Command, _ []string) error {
	probes, err := h.probe(cmd.Context())
	if err != nil {
		return err
	}
	if len(probes) == 0 {
		fmt.Fprintln(cmd.OutOrStdout(), "No nodes yet. Add one in the panel.")
		return nil
	}
	w := tabwriter.NewWriter(cmd.OutOrStdout(), 0, 0, 2, ' ', 0)
	fmt.Fprintln(w, "NAME\tID\tADDRESS\tSTATE\tAGENT\tCERT VALID UNTIL")
	for _, p := range probes {
		fmt.Fprintf(w, "%s\t%s\t%s\t%s\t%s\t%s\n", p.node.Name, p.node.ID, p.node.Address, p.state, p.version, p.certificate)
	}
	return w.Flush()
}

func (h *Handler) renewNode(cmd *cobra.Command, args []string) error {
	ctx, cancel := context.WithTimeout(cmd.Context(), renewTimeout)
	defer cancel()
	n, err := h.findNode(ctx, args[0])
	if err != nil {
		return err
	}
	cert, err := h.nodes.RenewCertificate(ctx, n.ID)
	if err == nil {
		fmt.Fprintf(cmd.OutOrStdout(), "Renewed the certificate of %s, valid until %s.\n", n.Name, cert.NotAfter.Local().Format(time.DateOnly))
	}
	return err
}

// findNode finds a node by its ID or name.
func (h *Handler) findNode(ctx context.Context, idOrName string) (node.Node, error) {
	nodes, err := h.nodes.List(ctx)
	if err != nil {
		return node.Node{}, err
	}
	for _, n := range nodes {
		if n.ID == idOrName || strings.EqualFold(n.Name, idOrName) {
			return n, nil
		}
	}
	return node.Node{}, fmt.Errorf("no node is named %q; node list shows all nodes", idOrName)
}

// nodeProbe is the state of a node as its agent reports it.
type nodeProbe struct {
	node                        node.Node
	state, version, certificate string
}

// probe asks the agents of the nodes the user may see, as a whole, for their state at once.
func (h *Handler) probe(ctx context.Context) ([]nodeProbe, error) {
	nodes, err := h.nodes.List(ctx)
	if err != nil {
		return nil, err
	}
	grants := access.From(ctx)
	nodes = slices.DeleteFunc(nodes, func(n node.Node) bool { return !grants.On(access.NodesView, n.ID, "") })
	probes := make([]nodeProbe, len(nodes))
	var wg sync.WaitGroup
	for i, n := range nodes {
		probes[i] = nodeProbe{node: n, state: "pending", version: "-", certificate: "-"}
		if n.EnrolledAt == nil {
			continue
		}
		wg.Go(func() {
			ctx, cancel := context.WithTimeout(ctx, probeTimeout)
			defer cancel()
			probes[i].state = "offline"
			if info, cert, err := h.nodes.Status(ctx, n.ID); err == nil {
				probes[i].state, probes[i].version = "online", info.GetAgentVersion()
				probes[i].certificate = cert.NotAfter.Local().Format(time.DateOnly)
			}
		})
	}
	wg.Wait()
	return probes, nil
}
