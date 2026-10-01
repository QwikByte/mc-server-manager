package app

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"strings"
	"text/tabwriter"
	"time"

	"github.com/spf13/cobra"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials/local"
	"google.golang.org/grpc/status"

	mcsmv1 "github.com/QwikByte/mc-server-manager/api/mcsm/v1"
)

const localTimeout = 2 * time.Minute

func statusCommand(cfg *config) *cobra.Command {
	return &cobra.Command{
		Use:   "status",
		Short: "Show this node and its servers",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			return withLocal(cmd.Context(), *cfg, func(ctx context.Context, conn *grpc.ClientConn) error {
				info, err := mcsmv1.NewNodeServiceClient(conn).GetInfo(ctx, &mcsmv1.GetInfoRequest{})
				if err != nil {
					return err
				}
				fmt.Printf("Node     %s\nAgent    %s\nRuntime  %s\nSystem   %s, %d CPUs, %.1f GiB\n\n",
					info.GetHostname(), info.GetAgentVersion(), info.GetRuntime(), info.GetOs(), info.GetCpuCount(), float64(info.GetMemoryBytes())/(1<<30))
				return printServers(ctx, conn)
			})
		},
	}
}

func serverCommand(cfg *config) *cobra.Command {
	cmd := &cobra.Command{Use: "server", Short: "Manage the servers of this node"}
	cmd.AddCommand(
		&cobra.Command{
			Use:   "list",
			Short: "List all servers",
			Args:  cobra.NoArgs,
			RunE: func(cmd *cobra.Command, _ []string) error {
				return withLocal(cmd.Context(), *cfg, printServers)
			},
		},
		&cobra.Command{
			Use:   "start <id>",
			Short: "Start a server",
			Args:  cobra.ExactArgs(1),
			RunE: func(cmd *cobra.Command, args []string) error {
				return withLocal(cmd.Context(), *cfg, func(ctx context.Context, conn *grpc.ClientConn) error {
					_, err := mcsmv1.NewServerServiceClient(conn).StartServer(ctx, &mcsmv1.StartServerRequest{Id: args[0]})
					return err
				})
			},
		},
		&cobra.Command{
			Use:   "stop <id>",
			Short: "Stop a server gracefully",
			Args:  cobra.ExactArgs(1),
			RunE: func(cmd *cobra.Command, args []string) error {
				return withLocal(cmd.Context(), *cfg, func(ctx context.Context, conn *grpc.ClientConn) error {
					_, err := mcsmv1.NewServerServiceClient(conn).StopServer(ctx, &mcsmv1.StopServerRequest{Id: args[0]})
					return err
				})
			},
		},
		&cobra.Command{
			Use:   "logs <id>",
			Short: "Follow the console of a server until Ctrl+C",
			Args:  cobra.ExactArgs(1),
			RunE: func(cmd *cobra.Command, args []string) error {
				// Following has no time limit, so it uses the command's context directly.
				return withLocal(cmd.Context(), *cfg, func(_ context.Context, conn *grpc.ClientConn) error {
					return followLogs(cmd.Context(), conn, args[0])
				})
			},
		},
		&cobra.Command{
			Use:   "command <id> <command>...",
			Short: "Run a console command, e.g.: server command <id> say Hello",
			Args:  cobra.MinimumNArgs(2),
			RunE: func(cmd *cobra.Command, args []string) error {
				return withLocal(cmd.Context(), *cfg, func(ctx context.Context, conn *grpc.ClientConn) error {
					req := &mcsmv1.SendCommandRequest{Id: args[0], Command: strings.Join(args[1:], " ")}
					res, err := mcsmv1.NewServerServiceClient(conn).SendCommand(ctx, req)
					if err == nil {
						fmt.Println(strings.TrimRight(res.GetOutput(), "\n"))
					}
					return err
				})
			},
		},
	)
	return cmd
}

// withLocal connects to the running agent through its Unix socket.
func withLocal(ctx context.Context, cfg config, fn func(context.Context, *grpc.ClientConn) error) error {
	conn, err := grpc.NewClient("unix://"+cfg.socket(), grpc.WithTransportCredentials(local.NewCredentials()))
	if err != nil {
		return err
	}
	defer conn.Close()
	ctx, cancel := context.WithTimeout(ctx, localTimeout)
	defer cancel()
	return fn(ctx, conn)
}

func followLogs(ctx context.Context, conn *grpc.ClientConn, id string) error {
	stream, err := mcsmv1.NewServerServiceClient(conn).StreamLogs(ctx, &mcsmv1.StreamLogsRequest{Id: id, Tail: 100})
	if err != nil {
		return err
	}
	for {
		res, err := stream.Recv()
		if errors.Is(err, io.EOF) || status.Code(err) == codes.Canceled {
			return nil
		}
		if err != nil {
			return err
		}
		fmt.Println(res.GetLine())
	}
}

func printServers(ctx context.Context, conn *grpc.ClientConn) error {
	res, err := mcsmv1.NewServerServiceClient(conn).ListServers(ctx, &mcsmv1.ListServersRequest{})
	if err != nil {
		return err
	}
	if len(res.GetServers()) == 0 {
		fmt.Println("No servers on this node yet.")
		return nil
	}
	w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
	fmt.Fprintln(w, "ID\tNAME\tTYPE\tVERSION\tPORT\tMEMORY\tSTATE")
	for _, s := range res.GetServers() {
		fmt.Fprintf(w, "%s\t%s\t%s\t%s\t%d\t%d MB\t%s\n", s.GetId(), s.GetName(), s.GetType().Slug(),
			s.GetVersion(), s.GetPort(), s.GetMemoryMb(), s.GetState().Slug())
	}
	return w.Flush()
}
