package overlay

import (
	"context"
	"errors"
	"net"
	"net/netip"
	"os"
	"os/exec"
	"strings"
	"testing"
	"time"

	"github.com/google/nftables"
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

	// The check finds the table as it was written, and what differs or is missing.
	if err := k.Firewall(st); err != nil {
		t.Fatalf("after writing the table: %v", err)
	}
	other := st
	other.Clients = map[string]Client{"a": st.Clients["a"]}
	if err := k.Firewall(other); err == nil || errors.Is(err, ErrNoTable) {
		t.Errorf("for other clients: %v", err)
	}
	conn, err := nftables.New()
	if err != nil {
		t.Fatal(err)
	}
	conn.FlushChain(&nftables.Chain{Table: table, Name: "input"})
	if err := conn.Flush(); err != nil {
		t.Fatal(err)
	}
	if err := k.Firewall(st); err == nil || !strings.Contains(err.Error(), "input") {
		t.Errorf("without the rule of input: %v", err)
	}

	// Connections leave only through the interface, which this namespace lacks.
	ln, err := net.Listen("tcp4", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer ln.Close()
	ctx, cancel := context.WithTimeout(t.Context(), time.Second)
	defer cancel()
	if err := k.Connect(ctx, netip.MustParseAddrPort(ln.Addr().String())); err == nil {
		t.Error("connected without the interface")
	}

	if err := k.Remove(); err != nil {
		t.Fatal(err)
	}
	if err := k.Firewall(st); !errors.Is(err, ErrNoTable) {
		t.Errorf("after removing the table: %v", err)
	}
}
