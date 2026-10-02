package app

import (
	"fmt"
	"os"
	"text/tabwriter"

	"github.com/spf13/cobra"
	"google.golang.org/grpc"

	mcsmv1 "github.com/QwikByte/mc-server-manager/api/mcsm/v1"
	"github.com/QwikByte/mc-server-manager/internal/agent/storage"
)

// storageCommand manages the storage locations. They are only configurable here, on
// the node itself, so that the master can't choose arbitrary host directories.
func storageCommand(cfg *config) *cobra.Command {
	cmd := &cobra.Command{Use: "storage", Short: "Manage the directories where servers keep their data"}
	cmd.AddCommand(
		&cobra.Command{
			Use:   "list",
			Short: "List the storage locations",
			Args:  cobra.NoArgs,
			RunE: func(*cobra.Command, []string) error {
				locations, err := storage.New(cfg.dataDir).List()
				if err != nil {
					return err
				}
				w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
				fmt.Fprintln(w, "NAME\tPATH\tFREE")
				for _, l := range locations {
					fmt.Fprintf(w, "%s\t%s\t%.1f of %.1f GiB\n", l.Name, l.Path, float64(l.FreeBytes)/(1<<30), float64(l.TotalBytes)/(1<<30))
				}
				return w.Flush()
			},
		},
		&cobra.Command{
			Use:   "add <name> <path>",
			Short: "Allow a directory for server data, e.g. on another disk",
			Args:  cobra.ExactArgs(2),
			RunE: func(_ *cobra.Command, args []string) error {
				if err := ensureDataDir(*cfg); err != nil {
					return err
				}
				if err := storage.New(cfg.dataDir).Add(args[0], args[1]); err != nil {
					return err
				}
				fmt.Printf("Added %q. The panel offers it when you create a server on this node.\n", args[0])
				return nil
			},
		},
		&cobra.Command{
			Use:   "remove <name>",
			Short: "Remove a location that no server uses; its directory is kept",
			Args:  cobra.ExactArgs(1),
			RunE: func(cmd *cobra.Command, args []string) error {
				// The running agent knows which servers use the location.
				return cfg.local(cmd.Context(), func(conn grpc.ClientConnInterface) error {
					res, err := mcsmv1.NewServerServiceClient(conn).ListServers(cmd.Context(), &mcsmv1.ListServersRequest{})
					if err != nil {
						return err
					}
					for _, s := range res.GetServers() {
						if s.GetStorage() == args[0] {
							return fmt.Errorf("server %s (%s) still keeps its data there", s.GetName(), s.GetId())
						}
					}
					return storage.New(cfg.dataDir).Remove(args[0])
				})
			},
		},
	)
	return cmd
}
