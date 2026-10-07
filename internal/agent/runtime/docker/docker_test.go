package docker

import (
	"slices"
	"strings"
	"testing"

	noryxv1 "github.com/QwikByte/noryx/api/noryx/v1"
	"github.com/QwikByte/noryx/internal/agent/runtime"
)

func TestEveryServerTypeHasAnImage(t *testing.T) {
	for value, name := range noryxv1.ServerType_name {
		if typ := noryxv1.ServerType(value); typ != noryxv1.ServerType_SERVER_TYPE_UNSPECIFIED && images[typ].ref == "" {
			t.Errorf("%s has no image", name)
		}
	}
}

func TestEveryModdedTypeHasALoaderVariable(t *testing.T) {
	for value, name := range noryxv1.ServerType_name {
		if typ := noryxv1.ServerType(value); typ.Modded() != (loaderVariables[typ] != "") {
			t.Errorf("%s: loader variable %q", name, loaderVariables[typ])
		}
	}
}

// Proxies run as the user of their image, which owns their data, without capabilities, so
// that they can write to it on every start; game servers start as root to hand their data
// to the server's user.
func TestContainerUser(t *testing.T) {
	for value, name := range noryxv1.ServerType_name {
		typ := noryxv1.ServerType(value)
		if typ == noryxv1.ServerType_SERVER_TYPE_UNSPECIFIED {
			continue
		}
		opts, err := containerOptions(runtime.Spec{ID: "server", Type: typ, Port: 25565}, "/data", sharedNetwork)
		if err != nil {
			t.Fatal(err)
		}
		user, caps := "", serverCapabilities
		if typ.Proxy() {
			user, caps = proxyUser, nil
		}
		if opts.Config.User != user || !slices.Equal(opts.HostConfig.CapAdd, caps) || !slices.Equal(opts.HostConfig.CapDrop, []string{"ALL"}) {
			t.Errorf("%s: user %q, capabilities +%v -%v", name, opts.Config.User, opts.HostConfig.CapAdd, opts.HostConfig.CapDrop)
		}
	}
}

// A backend reached over the private network of the nodes publishes its port only at the
// node's address there; one whose proxy runs on the node doesn't publish it at all.
func TestPublishedPort(t *testing.T) {
	for _, tc := range []struct {
		spec runtime.Spec
		want string // host IP, or "none"
	}{
		{runtime.Spec{}, ""},
		{runtime.Spec{BehindProxy: true, Overlay: "10.213.0.3"}, "10.213.0.3"},
		{runtime.Spec{BehindProxy: true, ProxyOnNode: true}, "none"},
	} {
		tc.spec.ID, tc.spec.Type, tc.spec.Port = "server", noryxv1.ServerType_SERVER_TYPE_PAPER, 25566
		opts, err := containerOptions(tc.spec, "/data", sharedNetwork)
		if err != nil {
			t.Fatal(err)
		}
		got := "none"
		for _, bindings := range opts.HostConfig.PortBindings {
			for _, b := range bindings {
				got = ""
				if b.HostIP.IsValid() {
					got = b.HostIP.String()
				}
				if b.HostPort != "25566" {
					t.Errorf("%+v: host port %s", tc.spec, b.HostPort)
				}
			}
		}
		if got != tc.want {
			t.Errorf("%+v: host IP %q, want %q", tc.spec, got, tc.want)
		}
	}
}

// The time zone is the only variable of the container besides those the agent sets from
// checked settings, and the stop timeout also applies when Docker stops the container itself.
func TestStopTimeoutAndTimeZone(t *testing.T) {
	for _, tc := range []struct {
		spec    runtime.Spec
		tz      string
		seconds int
	}{
		{runtime.Spec{}, "", 60},
		{runtime.Spec{StopTimeout: 300, TimeZone: "Europe/Berlin"}, "TZ=Europe/Berlin", 300},
	} {
		tc.spec.ID, tc.spec.Type, tc.spec.Port = "server", noryxv1.ServerType_SERVER_TYPE_PAPER, 25565
		opts, err := containerOptions(tc.spec, "/data", sharedNetwork)
		if err != nil {
			t.Fatal(err)
		}
		tz := ""
		if i := slices.IndexFunc(opts.Config.Env, func(v string) bool { return strings.HasPrefix(v, "TZ=") }); i >= 0 {
			tz = opts.Config.Env[i]
		}
		if tz != tc.tz || *opts.Config.StopTimeout != tc.seconds {
			t.Errorf("%+v: time zone %q, stop timeout %d", tc.spec, tz, *opts.Config.StopTimeout)
		}
	}
}
