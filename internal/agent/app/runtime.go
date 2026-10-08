package app

import (
	"fmt"

	"github.com/spf13/cobra"
	"github.com/spf13/pflag"

	noryxv1 "github.com/QwikByte/noryx/api/noryx/v1"
	"github.com/QwikByte/noryx/internal/agent/runtime/docker"
)

// runtimeFlags choose the runtime of the servers: Docker, or Podman as root. Each node has
// one, which runs all its servers and datastores.
type runtimeFlags struct {
	name   string
	socket string
}

func (r *runtimeFlags) add(flags *pflag.FlagSet) {
	flags.StringVar(&r.name, "runtime", noryxv1.RuntimeDocker, "runtime of the servers: docker, or podman as root")
	flags.StringVar(&r.socket, "runtime-socket", "", "Unix socket of the runtime (default: Docker's, or "+docker.PodmanSocket+" for Podman)")
}

// options returns the options of the runtime, or why they are wrong.
func (r runtimeFlags) options() (docker.Options, error) {
	switch r.name {
	case noryxv1.RuntimeDocker, noryxv1.RuntimePodman:
		return docker.Options{Podman: r.name == noryxv1.RuntimePodman, Socket: r.socket}, nil
	}
	return docker.Options{}, fmt.Errorf("--runtime must be %s or %s, not %q", noryxv1.RuntimeDocker, noryxv1.RuntimePodman, r.name)
}

// isolateCommand keeps the servers of Podman apart at boot, before Podman starts them.
func isolateCommand() *cobra.Command {
	return &cobra.Command{
		Use:   "isolate",
		Short: "Keep the servers of Podman from reaching each other, e.g. at boot before Podman starts them",
		Args:  cobra.NoArgs,
		RunE:  func(*cobra.Command, []string) error { return isolate() },
	}
}

func isolate() error {
	if err := docker.Isolate(); err != nil {
		return fmt.Errorf("can't keep the servers of Podman apart, which needs the bridge support of nftables in the kernel (nft_meta_bridge): %w", err)
	}
	return nil
}
