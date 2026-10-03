package terminal

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/spf13/cobra"

	"github.com/QwikByte/mc-server-manager/internal/master/access"
)

// Commands that follow output end after maxFollow; others run until they are done.
func TestFollowLimit(t *testing.T) {
	defer func(d time.Duration) { maxFollow = d }(maxFollow)
	maxFollow = 10 * time.Millisecond
	allow := func(context.Context, access.Grants, []string) error { return nil }
	for _, tc := range []struct {
		args []string
		want error
	}{
		{[]string{"server", "logs", "s1"}, errFollowLimit},
		{[]string{"logs", "-f"}, errFollowLimit},
		{[]string{"logs"}, context.DeadlineExceeded},
	} {
		root := guarded("mcsm-agent", "", map[string]check{"server logs": allow, "logs": allow})
		wait := func(cmd *cobra.Command, _ []string) error { <-cmd.Context().Done(); return cmd.Context().Err() }
		server := &cobra.Command{Use: "server"}
		server.AddCommand(&cobra.Command{Use: "logs", RunE: wait})
		logs := &cobra.Command{Use: "logs", RunE: wait}
		logs.Flags().BoolP("follow", "f", false, "")
		root.AddCommand(server, logs)
		root.SetArgs(tc.args)
		ctx, cancel := context.WithTimeout(t.Context(), time.Second)
		if err := root.ExecuteContext(ctx); !errors.Is(err, tc.want) {
			t.Errorf("%v: got %v, want %v", tc.args, err, tc.want)
		}
		cancel()
	}
}
