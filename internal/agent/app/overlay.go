package app

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/QwikByte/noryx/internal/agent/overlay"
)

// overlayCommands manage the private network of the nodes on the node itself. Only its
// administrator lets the master add it, so that the master can't open a port and an
// interface on a host that didn't agree.
func overlayCommands(cfg *config) []*cobra.Command {
	return []*cobra.Command{
		{
			Use:   "allow",
			Short: "Let the master add this node to the private network of the nodes",
			Args:  cobra.NoArgs,
			RunE: func(*cobra.Command, []string) error {
				if err := ensureDataDir(*cfg); err != nil {
					return err
				}
				if err := overlay.Allow(cfg.dataDir); err != nil {
					return err
				}
				fmt.Println("Allowed. Add the node to the private network on its page in the panel, and open the network's UDP port (51820 by default) for the other nodes.")
				return nil
			},
		},
		{
			Use:   "deny",
			Short: "Keep the master from adding this node to the private network, and remove its key",
			Args:  cobra.NoArgs,
			RunE:  func(*cobra.Command, []string) error { return overlay.Deny(cfg.dataDir) },
		},
		{
			Use:   "up",
			Short: "Restore the private network of a member, e.g. at boot before Docker or Podman starts",
			Args:  cobra.NoArgs,
			RunE:  func(*cobra.Command, []string) error { return overlay.Up(cfg.dataDir, overlay.Linux{}) },
		},
	}
}
