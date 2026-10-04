package docker

import (
	"testing"

	"github.com/moby/moby/api/types/container"
	"github.com/moby/moby/api/types/network"

	noryxv1 "github.com/QwikByte/mc-server-manager/api/noryx/v1"
	"github.com/QwikByte/mc-server-manager/internal/agent/runtime"
)

// A proxy has a network of its own, which its backends keep when their containers are
// created again; other servers are in the shared network, where they can't reach each other.
func TestPlacement(t *testing.T) {
	in := func(names ...string) container.InspectResponse {
		c := container.InspectResponse{NetworkSettings: &container.NetworkSettings{Networks: map[string]*network.EndpointSettings{}}}
		for _, n := range names {
			c.NetworkSettings.Networks[n] = &network.EndpointSettings{}
		}
		return c
	}
	proxy := runtime.Spec{ID: "proxy", Type: noryxv1.ServerType_SERVER_TYPE_VELOCITY}
	backend := runtime.Spec{ID: "lobby", Type: noryxv1.ServerType_SERVER_TYPE_PAPER, BehindProxy: true}
	standalone := runtime.Spec{ID: "survival", Type: noryxv1.ServerType_SERVER_TYPE_PAPER}
	for _, tc := range []struct {
		name string
		c    container.InspectResponse
		spec runtime.Spec
		want string
	}{
		{"proxy", in(sharedNetwork), proxy, "noryx-proxy-proxy"},
		{"backend", in("noryx-proxy-proxy"), backend, "noryx-proxy-proxy"},
		{"backend of an older agent", in(legacyNetwork), backend, sharedNetwork},
		{"backend that left its network", in("noryx-proxy-proxy"), standalone, sharedNetwork},
		{"standalone", in(sharedNetwork), standalone, sharedNetwork},
	} {
		if got := placement(tc.c, tc.spec); got != tc.want {
			t.Errorf("%s: placement = %s, want %s", tc.name, got, tc.want)
		}
	}
}
