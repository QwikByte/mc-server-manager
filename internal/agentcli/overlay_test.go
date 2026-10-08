package agentcli

import (
	"slices"
	"strings"
	"testing"
	"time"

	noryxv1 "github.com/QwikByte/noryx/api/noryx/v1"
)

// The status of a member names its firewall and the ports it publishes, with their clients.
func TestPrintOverlay(t *testing.T) {
	res := &noryxv1.GetOverlayResponse{
		Allowed: true, PublicKey: "key", Address: "10.213.0.3/24",
		Firewall: noryxv1.OverlayFirewall_OVERLAY_FIREWALL_INCOMPLETE, FirewallProblem: "the chain input has 0 rules instead of 1",
		Published: []*noryxv1.OverlayPublished{
			{Port: 3306, DatastoreId: "main", Clients: []*noryxv1.OverlayClient{{Address: "10.213.0.1"}, {Address: "10.213.0.2"}}},
			{Port: 25570, ServerId: "survival", Clients: []*noryxv1.OverlayClient{{Address: "10.213.0.1"}}},
		},
	}
	var out strings.Builder
	if err := printOverlay(&out, res, time.Now()); err != nil {
		t.Fatal(err)
	}
	var lines []string // with single spaces between columns
	for line := range strings.Lines(out.String()) {
		lines = append(lines, strings.Join(strings.Fields(line), " "))
	}
	for _, want := range []string{
		"Firewall incomplete: the chain input has 0 rules instead of 1. The master writes it again within 5 minutes, or now: noryx-agent overlay up",
		"PUBLISHED FOR CLIENTS",
		"3306 datastore main 10.213.0.1, 10.213.0.2",
		"25570 server survival 10.213.0.1",
	} {
		if !slices.Contains(lines, want) {
			t.Errorf("no line %q in:\n%s", want, out.String())
		}
	}
}
