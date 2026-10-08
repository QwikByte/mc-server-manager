package agentcli

import (
	"context"
	"fmt"
	"io"
	"strings"
	"text/tabwriter"
	"time"

	"github.com/spf13/cobra"
	"google.golang.org/grpc"

	noryxv1 "github.com/QwikByte/noryx/api/noryx/v1"
)

func (c cli) overlay() *cobra.Command {
	cmd := &cobra.Command{Use: "overlay", Short: "The private network of the nodes"}
	cmd.AddCommand(&cobra.Command{
		Use:   "status",
		Short: "Show this node's part of the private network and its peers",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			return c.call(cmd, func(ctx context.Context, conn grpc.ClientConnInterface) error {
				res, err := noryxv1.NewOverlayServiceClient(conn).GetOverlay(ctx, &noryxv1.GetOverlayRequest{})
				if err != nil {
					return err
				}
				return printOverlay(cmd.OutOrStdout(), res, time.Now())
			})
		},
	})
	return cmd
}

func printOverlay(out io.Writer, res *noryxv1.GetOverlayResponse, now time.Time) error {
	switch {
	case !res.GetAllowed():
		_, err := fmt.Fprintln(out, "This node may not join the private network. Its administrator allows it with: noryx-agent overlay allow")
		return err
	case res.GetUnsupported() != "":
		_, err := fmt.Fprintf(out, "This node can't join the private network: %s\n", res.GetUnsupported())
		return err
	case res.GetAddress() == "":
		_, err := fmt.Fprintf(out, "This node may join the private network, which happens in the panel.\nKey      %s\n", res.GetPublicKey())
		return err
	}
	fmt.Fprintf(out, "Address  %s\nKey      %s\n", res.GetAddress(), res.GetPublicKey())
	const restore = "The master writes it again within 5 minutes, or now: noryx-agent overlay up"
	switch res.GetFirewall() {
	case noryxv1.OverlayFirewall_OVERLAY_FIREWALL_IN_PLACE:
		fmt.Fprintln(out, "Firewall in place")
	case noryxv1.OverlayFirewall_OVERLAY_FIREWALL_MISSING:
		fmt.Fprintf(out, "Firewall missing. %s\n", restore)
	case noryxv1.OverlayFirewall_OVERLAY_FIREWALL_INCOMPLETE:
		fmt.Fprintf(out, "Firewall incomplete: %s. %s\n", res.GetFirewallProblem(), restore)
	}
	w := tabwriter.NewWriter(out, 0, 0, 2, ' ', 0)
	fmt.Fprintln(w, "\nPEER\tENDPOINT\tHANDSHAKE\tRECEIVED\tSENT")
	for _, p := range res.GetPeers() {
		handshake := "never"
		if t := p.GetLatestHandshakeUnix(); t > 0 {
			handshake = now.Sub(time.Unix(t, 0)).Round(time.Second).String() + " ago"
		}
		fmt.Fprintf(w, "%s\t%s\t%s\t%.1f MiB\t%.1f MiB\n", p.GetPublicKey(), p.GetEndpoint(), handshake,
			float64(p.GetReceivedBytes())/(1<<20), float64(p.GetSentBytes())/(1<<20))
	}
	if len(res.GetPublished()) > 0 {
		fmt.Fprintln(w, "\nPUBLISHED\tFOR\tCLIENTS")
	}
	for _, p := range res.GetPublished() {
		owner := "server " + p.GetServerId()
		if p.GetDatastoreId() != "" {
			owner = "datastore " + p.GetDatastoreId()
		}
		var clients []string
		for _, c := range p.GetClients() {
			clients = append(clients, c.GetAddress())
		}
		fmt.Fprintf(w, "%d\t%s\t%s\n", p.GetPort(), owner, strings.Join(clients, ", "))
	}
	return w.Flush()
}
