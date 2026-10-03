package auth

import (
	"fmt"
	"net"
	"net/http"
	"net/netip"
	"slices"
	"strings"
)

// ClientIP is the address a request came from. Behind a reverse proxy, that of the proxy,
// unless Proxies.Handler serves the request.
func ClientIP(r *http.Request) string {
	if ip, _, err := net.SplitHostPort(r.RemoteAddr); err == nil {
		return ip
	}
	return r.RemoteAddr
}

// Proxies are the reverse proxies whose X-Forwarded-For or X-Real-IP header tells the
// address of the client.
type Proxies []netip.Prefix

// ParseProxies parses IP addresses and CIDR networks.
func ParseProxies(list []string) (Proxies, error) {
	proxies := make(Proxies, 0, len(list))
	for _, s := range list {
		p, err := netip.ParsePrefix(s)
		if addr, aerr := netip.ParseAddr(s); aerr == nil {
			addr = addr.Unmap()
			p, err = netip.PrefixFrom(addr, addr.BitLen()), nil
		}
		if err != nil {
			return nil, fmt.Errorf("trusted proxy %q is neither an IP address nor a CIDR network", s)
		}
		proxies = append(proxies, p.Masked())
	}
	return proxies, nil
}

// Handler sets the address of requests from a proxy to that of the client: the last
// address in X-Forwarded-For that isn't a proxy, or else X-Real-IP. As anyone can send
// these headers, they count only from the proxies.
func (p Proxies) Handler(next http.Handler) http.Handler {
	if len(p) == 0 {
		return next
	}
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if client, ok := p.client(r); ok {
			r = r.Clone(r.Context())
			r.RemoteAddr = netip.AddrPortFrom(client, 0).String()
		}
		next.ServeHTTP(w, r)
	})
}

// client walks the addresses the proxies added from the right while they are proxies.
func (p Proxies) client(r *http.Request) (netip.Addr, bool) {
	peer, err := netip.ParseAddrPort(r.RemoteAddr)
	client := peer.Addr().Unmap()
	if err != nil || !p.trusted(client) {
		return client, false
	}
	hops := r.Header.Values("X-Forwarded-For")
	if len(hops) == 0 {
		hops = r.Header.Values("X-Real-IP")
	}
	hops = strings.Split(strings.Join(hops, ","), ",")
	for i := len(hops) - 1; i >= 0 && p.trusted(client); i-- {
		addr, err := netip.ParseAddr(strings.TrimSpace(hops[i]))
		if err != nil {
			break // not added by a proxy; the last proxy is the closest known address
		}
		client = addr.Unmap()
	}
	return client, true
}

func (p Proxies) trusted(addr netip.Addr) bool {
	return slices.ContainsFunc(p, func(network netip.Prefix) bool { return network.Contains(addr) })
}
