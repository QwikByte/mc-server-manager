package app

import (
	"context"
	"fmt"
	"os"
	"strings"
	"text/tabwriter"
	"time"

	"github.com/spf13/cobra"
	"google.golang.org/grpc"

	mcsmv1 "github.com/QwikByte/mc-server-manager/api/mcsm/v1"
)

// backupCommand backs up and restores servers locally, e.g. while the master is unreachable.
// Creating and restoring have no time limit, as large worlds take a while.
func backupCommand(cfg *config) *cobra.Command {
	cmd := &cobra.Command{Use: "backup", Short: "Back up and restore the servers of this node"}
	var label string
	create := &cobra.Command{
		Use:   "create <server-id>",
		Short: "Back up all data of a server; a running server keeps running",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return withLocal(cmd.Context(), *cfg, func(_ context.Context, conn *grpc.ClientConn) error {
				req := &mcsmv1.CreateBackupRequest{ServerId: args[0], Label: label, Selection: &mcsmv1.BackupSelection{Everything: true}}
				res, err := mcsmv1.NewBackupServiceClient(conn).CreateBackup(cmd.Context(), req)
				if err == nil {
					fmt.Printf("Created backup %s (%.1f MiB).\n", res.GetBackup().GetId(), float64(res.GetBackup().GetSize())/(1<<20))
				}
				return err
			})
		},
	}
	create.Flags().StringVar(&label, "label", "", "describes the backup, e.g. before an update")
	cmd.AddCommand(
		&cobra.Command{
			Use:   "list <server-id>",
			Short: "List the backups of a server",
			Args:  cobra.ExactArgs(1),
			RunE: func(cmd *cobra.Command, args []string) error {
				return withLocal(cmd.Context(), *cfg, func(ctx context.Context, conn *grpc.ClientConn) error {
					res, err := mcsmv1.NewBackupServiceClient(conn).ListBackups(ctx, &mcsmv1.ListBackupsRequest{ServerId: args[0]})
					if err != nil {
						return err
					}
					w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
					fmt.Fprintln(w, "ID\tCREATED\tSIZE\tLOCATION\tCONTENT\tLABEL")
					for _, b := range res.GetBackups() {
						fmt.Fprintf(w, "%s\t%s\t%.1f MiB\t%s\t%s\t%s\n", b.GetId(), time.Unix(b.GetCreatedUnix(), 0).Format(time.DateTime),
							float64(b.GetSize())/(1<<20), b.GetLocation(), strings.Join(b.GetPaths(), " "), b.GetLabel())
					}
					return w.Flush()
				})
			},
		},
		create,
		&cobra.Command{
			Use:   "restore <server-id> <backup-id>",
			Short: "Replace the backed up data of a server; a running server restarts",
			Args:  cobra.ExactArgs(2),
			RunE: func(cmd *cobra.Command, args []string) error {
				return withLocal(cmd.Context(), *cfg, func(_ context.Context, conn *grpc.ClientConn) error {
					req := &mcsmv1.RestoreBackupRequest{ServerId: args[0], BackupId: args[1]}
					if _, err := mcsmv1.NewBackupServiceClient(conn).RestoreBackup(cmd.Context(), req); err != nil {
						return err
					}
					fmt.Printf("Restored backup %s.\n", args[1])
					return nil
				})
			},
		},
	)
	return cmd
}
