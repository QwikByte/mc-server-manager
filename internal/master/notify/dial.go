package notify

import (
	"errors"
	"fmt"
	"net"
	"net/http"
	"net/netip"
	"net/url"
	"slices"
	"strings"
	"syscall"
	"time"

	"github.com/QwikByte/noryx/internal/master/httpapi"
)

const dialTimeout = 10 * time.Second

// IPv4 ranges that aren't public (RFC 6890 and its updates): this network, private networks,
// which include the private network of the nodes as it must be one of them, shared addresses of
// carrier-grade NAT, loopback, link-local, IETF protocol assignments, documentation, the former
// 6to4 relays, benchmarking, multicast, and the reserved rest with the broadcast address.
var refused4 = prefixes("0.0.0.0/8", "10.0.0.0/8", "100.64.0.0/10", "127.0.0.0/8", "169.254.0.0/16", "172.16.0.0/12",
	"192.0.0.0/24", "192.0.2.0/24", "192.88.99.0/24", "192.168.0.0/16", "198.18.0.0/15", "198.51.100.0/24", "203.0.113.0/24",
	"224.0.0.0/4", "240.0.0.0/4")

// IPv6 addresses are public in the global unicast range, except IETF protocol assignments such
// as Teredo, documentation and 6to4, which tunnels to IPv4 addresses. NAT64 addresses are as
// public as the IPv4 address they hold.
var (
	global6  = netip.MustParsePrefix("2000::/3")
	refused6 = prefixes("2001::/23", "2001:db8::/32", "2002::/16", "3fff::/20")
	nat64    = netip.MustParsePrefix("64:ff9b::/96")
)

func prefixes(ranges ...string) []netip.Prefix {
	p := make([]netip.Prefix, len(ranges))
	for i, r := range ranges {
		p[i] = netip.MustParsePrefix(r)
	}
	return p
}

func within(ip netip.Addr, ranges []netip.Prefix) bool {
	return slices.ContainsFunc(ranges, func(p netip.Prefix) bool { return p.Contains(ip) })
}

// public reports whether ip is a public address, in any of its IPv6 forms.
func public(ip netip.Addr) bool {
	ip = ip.Unmap()
	switch {
	case ip.Is4():
		return !within(ip, refused4)
	case nat64.Contains(ip):
		b := ip.As16()
		return public(netip.AddrFrom4([4]byte(b[12:])))
	}
	return global6.Contains(ip) && !within(ip, refused6)
}

// refusedError tells that a channel would connect to an address that isn't public.
type refusedError struct{ ip netip.Addr }

func (e *refusedError) Error() string {
	return fmt.Sprintf("%s isn't a public address. Notifications only go to public addresses.", e.ip)
}

// failed is the error of a channel that couldn't send, which the panel shows.
func failed(format string, args ...any) error {
	return httpapi.Errorf(http.StatusBadRequest, format, args...)
}

// newDialer returns a dialer that only connects to public addresses, and to those allow
// reports, e.g. of servers in tests. It checks the address of each connection right before it
// is made, after the name was resolved, so that DNS can't point to another address later.
func newDialer(allow func(netip.Addr) bool) *net.Dialer {
	return &net.Dialer{Timeout: dialTimeout, Control: func(_, address string, _ syscall.RawConn) error {
		ap, err := netip.ParseAddrPort(address)
		if err != nil {
			return err
		}
		if ip := ap.Addr(); !public(ip) && (allow == nil || !allow(ip)) {
			return &refusedError{ip}
		}
		return nil
	}}
}

// reason returns what users may learn about a failed connection: never the URL of a webhook,
// which is secret, even if the error of a library should name it.
func reason(err error, secret string) error {
	var refused *refusedError
	if errors.As(err, &refused) {
		return failed("%s", refused.Error())
	}
	var ue *url.Error
	if errors.As(err, &ue) {
		if ue.Timeout() {
			return failed("The target didn't answer in time.")
		}
		err = ue.Err
	}
	msg := err.Error()
	if secret != "" {
		msg = strings.ReplaceAll(msg, secret, "…")
		if u, perr := url.Parse(secret); perr == nil && len(u.Path) > 1 {
			msg = strings.ReplaceAll(msg, u.Path, "…")
		}
	}
	return failed("Can't reach the target: %s", msg)
}
