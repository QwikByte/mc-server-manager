package logs

import (
	"cmp"
	"encoding/json"
	"errors"
	"fmt"
	"maps"
	"slices"
	"time"

	"github.com/spf13/cobra"

	"github.com/QwikByte/noryx/internal/logging"
)

// pollEvery is how often a followed log is read again, for a store written by another process.
const pollEvery = time.Second

// Command returns the logs command of noryx-master and of the panel's terminal. It shows the
// entries that the grants of its context allow to see.
func Command(store func() (*Store, error)) *cobra.Command {
	var (
		f      Filter
		since  time.Duration
		lines  int
		follow bool
		asJSON bool
	)
	cmd := &cobra.Command{
		Use:   "logs",
		Short: "Show the log of the master and its agents: actions, warnings and errors",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			if lines < 0 || since < 0 {
				return errors.New("--lines and --since can't be negative")
			}
			if f.Level != "" {
				if _, err := logging.ParseLevel(f.Level); err != nil {
					return err
				}
			}
			s, err := store()
			if err != nil {
				return err
			}
			if since > 0 {
				f.Since = time.Now().Add(-since)
			}
			ctx, out := cmd.Context(), cmd.OutOrStdout()
			show := func(e Entry) {
				if asJSON {
					data, _ := json.Marshal(e)
					fmt.Fprintln(out, string(data))
					return
				}
				attrs := maps.Clone(e.Attrs)
				for k, v := range map[string]string{"user": e.User, "node": cmp.Or(e.NodeName, e.NodeID), "server": cmp.Or(e.ServerName, e.ServerID)} {
					if v != "" {
						attrs[k] = v
					}
				}
				if e.Source == FromAgent {
					attrs["source"] = FromAgent
				}
				fmt.Fprintln(out, logging.Line(e.Time, e.Level, e.Category, e.Message, attrs))
			}

			entries, err := s.List(ctx, f, false, min(lines, maxExport))
			if err != nil {
				return err
			}
			if len(entries) == 0 && !follow {
				fmt.Fprintln(out, "No entries.")
			}
			for _, e := range slices.Backward(entries) {
				show(e)
				f.After = max(f.After, e.ID)
			}
			if !follow {
				return nil
			}
			if f.After == 0 {
				if f.After, err = s.last(ctx); err != nil {
					return err
				}
			}
			for {
				changed := s.Changed()
				entries, err := s.List(ctx, f, true, batchSize)
				if err != nil && ctx.Err() == nil { // Ctrl+C cancels the query, see below
					return err
				}
				for _, e := range entries {
					show(e)
					f.After = e.ID
				}
				select {
				case <-ctx.Done():
					return nil
				case <-changed:
				case <-time.After(pollEvery):
				}
			}
		},
	}
	fl := cmd.Flags()
	fl.StringVar(&f.Level, "level", "", "least important level shown: debug, info, warn or error")
	fl.StringVar(&f.Category, "category", "", "only entries of a category, e.g. servers, files, auth or users")
	fl.StringVar(&f.Source, "source", "", "only entries of the master or the agents: master or agent")
	fl.StringVar(&f.User, "user", "", "only entries about what a user did")
	fl.StringVar(&f.NodeID, "node", "", "only entries about a node, by its ID as node list shows it")
	fl.StringVar(&f.ServerID, "server", "", "only entries about a server, by its ID")
	fl.StringVar(&f.Search, "search", "", "only entries that contain a text")
	fl.DurationVar(&since, "since", 0, "only entries of the given time, e.g. 1h or 30m")
	fl.IntVarP(&lines, "lines", "n", 50, "number of entries shown before following")
	fl.BoolVarP(&follow, "follow", "f", false, "keep showing new entries until Ctrl+C")
	fl.BoolVar(&asJSON, "json", false, "show the entries as JSON lines")
	return cmd
}
