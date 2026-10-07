package agentcli

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"text/tabwriter"
	"time"

	"github.com/spf13/cobra"
	"google.golang.org/grpc"

	noryxv1 "github.com/QwikByte/noryx/api/noryx/v1"
)

// maxLogLines is the most past lines of a datastore's log that the agent sends.
const maxLogLines = 1000

// datastore lists the datastores of the node, dumps and restores their databases, locally
// e.g. while the master is unreachable, and follows their logs.
func (c cli) datastore() *cobra.Command {
	cmd := &cobra.Command{Use: "datastore", Short: "Back up and restore the databases of the networks of this node, and follow their logs"}
	var label string
	backup := &cobra.Command{
		Use:   "backup <datastore-id> [database]...",
		Short: "Back up the databases of a datastore as SQL dumps, or some of them; it keeps running",
		Args:  cobra.MinimumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return c.long(cmd, func(ctx context.Context, conn grpc.ClientConnInterface) error {
				req := &noryxv1.CreateDumpRequest{Id: args[0], Databases: args[1:], Label: label}
				res, err := noryxv1.NewDatastoreServiceClient(conn).CreateDump(ctx, req)
				if err == nil {
					fmt.Fprintf(cmd.OutOrStdout(), "Created backup %s (%.1f MiB).\n", res.GetDump().GetId(), float64(res.GetDump().GetSize())/(1<<20))
				}
				return err
			})
		},
	}
	backup.Flags().StringVar(&label, "label", "", "describes the backup, e.g. before an update")
	var lines int
	logs := &cobra.Command{
		Use:   "logs <datastore-id>",
		Short: "Follow the log of a datastore until Ctrl+C, with passwords hidden",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if lines < 0 {
				return errors.New("--lines can't be negative")
			}
			return c.long(cmd, func(ctx context.Context, conn grpc.ClientConnInterface) error {
				req := &noryxv1.StreamDatastoreLogsRequest{Id: args[0], Tail: uint32(min(lines, maxLogLines))} //nolint:gosec // 0 to maxLogLines
				stream, err := noryxv1.NewDatastoreServiceClient(conn).StreamDatastoreLogs(ctx, req)
				if err != nil {
					return err
				}
				return printLines(stream, cmd.OutOrStdout())
			})
		},
	}
	logs.Flags().IntVarP(&lines, "lines", "n", 100, fmt.Sprintf("number of past lines shown first, up to %d", maxLogLines))
	cmd.AddCommand(
		&cobra.Command{
			Use:   "list",
			Short: "List the datastores of this node",
			Args:  cobra.NoArgs,
			RunE: func(cmd *cobra.Command, _ []string) error {
				return c.call(cmd, func(ctx context.Context, conn grpc.ClientConnInterface) error {
					res, err := noryxv1.NewDatastoreServiceClient(conn).ListDatastores(ctx, &noryxv1.ListDatastoresRequest{})
					if err != nil || len(res.GetDatastores()) == 0 {
						if err == nil {
							fmt.Fprintln(cmd.OutOrStdout(), "No datastores on this node.")
						}
						return err
					}
					w := tabwriter.NewWriter(cmd.OutOrStdout(), 0, 0, 2, ' ', 0)
					fmt.Fprintln(w, "ID\tENGINE\tVERSION\tMEMORY\tSIZE\tSTATE\tDATABASES")
					for _, ds := range res.GetDatastores() {
						fmt.Fprintf(w, "%s\t%s\t%s\t%d MB\t%.1f MiB\t%s\t%s\n", ds.GetId(), ds.GetEngine().Slug(), ds.GetVersion(), ds.GetMemoryMb(),
							float64(ds.GetSize())/(1<<20), ds.GetState().Slug(), strings.Join(ds.GetDatabases(), " "))
					}
					return w.Flush()
				})
			},
		},
		backup,
		&cobra.Command{
			Use:   "backups <datastore-id>",
			Short: "List the backups of a datastore",
			Args:  cobra.ExactArgs(1),
			RunE: func(cmd *cobra.Command, args []string) error {
				return c.call(cmd, func(ctx context.Context, conn grpc.ClientConnInterface) error {
					res, err := noryxv1.NewDatastoreServiceClient(conn).ListDumps(ctx, &noryxv1.ListDumpsRequest{Id: args[0]})
					if err != nil {
						return err
					}
					w := tabwriter.NewWriter(cmd.OutOrStdout(), 0, 0, 2, ' ', 0)
					fmt.Fprintln(w, "ID\tCREATED\tSIZE\tLOCATION\tDATABASES\tLABEL")
					for _, b := range res.GetDumps() {
						fmt.Fprintf(w, "%s\t%s\t%.1f MiB\t%s\t%s\t%s\n", b.GetId(), time.Unix(b.GetCreatedUnix(), 0).Format(time.DateTime),
							float64(b.GetSize())/(1<<20), b.GetLocation(), strings.Join(b.GetPaths(), " "), b.GetLabel())
					}
					return w.Flush()
				})
			},
		},
		&cobra.Command{
			Use:   "restore <datastore-id> <backup-id> [database]...",
			Short: "Replace databases with their backup, all in it or some; their users keep their passwords",
			Args:  cobra.MinimumNArgs(2),
			RunE: func(cmd *cobra.Command, args []string) error {
				return c.long(cmd, func(ctx context.Context, conn grpc.ClientConnInterface) error {
					req := &noryxv1.RestoreDumpRequest{Id: args[0], DumpId: args[1], Databases: args[2:]}
					if _, err := noryxv1.NewDatastoreServiceClient(conn).RestoreDump(ctx, req); err != nil {
						return err
					}
					fmt.Fprintf(cmd.OutOrStdout(), "Restored backup %s.\n", args[1])
					return nil
				})
			},
		},
		logs,
	)
	return cmd
}
