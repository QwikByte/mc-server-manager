package overlay

import (
	"errors"
	"net/netip"
	"os"
	"os/exec"
	"strings"
	"testing"
)

// TestLinux runs against the kernel, in a network namespace of its own:
// NORYX_KERNEL_TEST=1 unshare -n go test ./internal/agent/overlay/
func TestLinux(t *testing.T) {
	if os.Getenv("NORYX_KERNEL_TEST") == "" {
		t.Skip("set NORYX_KERNEL_TEST and run in a network namespace of its own")
	}
	var k Linux
	err := k.Check()
	if err != nil && !errors.Is(err, errNoWireGuard) {
		t.Fatal(err)
	}
	t.Logf("WireGuard: %v", err)
	if _, err := k.HostNets(); err != nil {
		t.Fatal(err)
	}
	st := State{
		Address: netip.MustParsePrefix("10.213.0.3/24"), Port: 51820, MTU: 1420,
		Clients: map[string]Client{
			"a": {Port: 25570, Address: netip.MustParseAddr("10.213.0.1")},
			"b": {Port: 3306, Address: netip.MustParseAddr("10.213.0.1"), Others: []netip.Addr{netip.MustParseAddr("10.213.0.2")}},
		},
	}
	if err := writeFirewall(st); err != nil {
		t.Fatal(err)
	}
	if nft, err := exec.LookPath("nft"); err == nil {
		out, err := exec.Command(nft, "list", "table", "inet", "noryx").CombinedOutput()
		if err != nil {
			t.Fatalf("%v: %s", err, out)
		}
		t.Logf("%s", out)
		for _, want := range []string{
			`iifname != "noryx0" iifname != "lo" drop`,
			`iifname "noryx0" ct state new drop`,
			`ct original ip daddr 10.213.0.3 ct original proto-dst 25570 ip saddr 10.213.0.1 accept`,
			`ct original ip daddr 10.213.0.3 ct original proto-dst 3306 ip saddr 10.213.0.2 accept`,
			`tcp option maxseg size set rt mtu`,
		} {
			if !strings.Contains(string(out), want) {
				t.Errorf("the table lacks %q", want)
			}
		}
	}
	if err := k.Remove(); err != nil {
		t.Fatal(err)
	}
}
