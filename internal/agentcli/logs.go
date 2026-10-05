package agentcli

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"maps"
	"time"

	"github.com/spf13/cobra"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	noryxv1 "github.com/QwikByte/noryx/api/noryx/v1"
	"github.com/QwikByte/noryx/internal/logging"
)

// logs shows the agent's own log: the calls it received, with their origin, and what failed.
func (c cli) logs() *cobra.Command {
	var (
		follow bool
		level  string
		lines  int
	)
	cmd := &cobra.Command{
		Use:   "logs",
		Short: "Show the latest entries of the agent's log: the calls it received and what failed",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			if lines < 0 {
				return errors.New("--lines can't be negative")
			}
			least, err := logging.ParseLevel(level)
			if err != nil {
				return err
			}
			out := cmd.OutOrStdout()
			return c.long(cmd, func(ctx context.Context, conn grpc.ClientConnInterface) error {
				client := noryxv1.NewLogServiceClient(conn)
				var kept []*noryxv1.LogEntry
				last, err := readLog(ctx, client, &noryxv1.ReadLogRequest{}, func(e *noryxv1.LogEntry) {
					if slog.Level(e.GetLevel()) >= least {
						kept = append(kept, e)
					}
				})
				if err != nil {
					return err
				}
				if len(kept) == 0 && !follow {
					fmt.Fprintln(out, "No entries yet.")
				}
				for _, e := range kept[max(0, len(kept)-lines):] {
					printEntry(out, e)
				}
				if !follow {
					return nil
				}
				req := &noryxv1.ReadLogRequest{Boot: last.GetBoot(), After: last.GetSeq(), Follow: true}
				_, err = readLog(ctx, client, req, func(e *noryxv1.LogEntry) {
					if slog.Level(e.GetLevel()) >= least {
						printEntry(out, e)
					}
				})
				return err
			})
		},
	}
	f := cmd.Flags()
	f.BoolVarP(&follow, "follow", "f", false, "keep showing new entries until Ctrl+C")
	f.StringVar(&level, "level", "info", "least important level shown: debug, info, warn or error")
	f.IntVarP(&lines, "lines", "n", 50, "number of entries shown before following")
	return cmd
}

// readLog passes the entries of the log to fn and returns the last one.
func readLog(ctx context.Context, client noryxv1.LogServiceClient, req *noryxv1.ReadLogRequest, fn func(*noryxv1.LogEntry)) (*noryxv1.LogEntry, error) {
	stream, err := client.ReadLog(ctx, req)
	var last *noryxv1.LogEntry
	for err == nil {
		var res *noryxv1.ReadLogResponse
		if res, err = stream.Recv(); err == nil {
			for _, last = range res.GetEntries() {
				fn(last)
			}
		}
	}
	if errors.Is(err, io.EOF) || status.Code(err) == codes.Canceled {
		err = nil
	}
	return last, err
}

func printEntry(out io.Writer, e *noryxv1.LogEntry) {
	attrs := map[string]string{}
	maps.Copy(attrs, e.GetAttrs())
	if id := e.GetServerId(); id != "" {
		attrs[logging.KeyServer] = id
	}
	fmt.Fprintln(out, logging.Line(time.Unix(0, e.GetTimeUnixNano()), slog.Level(e.GetLevel()), e.GetCategory(), e.GetMessage(), attrs))
}
