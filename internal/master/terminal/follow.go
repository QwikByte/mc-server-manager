package terminal

import (
	"context"
	"errors"
	"time"

	"github.com/spf13/cobra"
)

// maxFollow ends commands that follow output, such as a console, so that they don't outlive
// the session or the permissions of the user: running them again checks both again.
var maxFollow = 5 * time.Minute

var errFollowLimit = errors.New("stopped following after 5 minutes, so that your access is checked again; run the command again to continue")

// following are the commands that run until they are stopped, with the flag that makes
// them follow, if they need one.
var following = map[string]string{"server logs": "", "logs": "follow"}

// follows reports whether the command at path follows output with the flags it got.
func follows(cmd *cobra.Command, path string) bool {
	flag, ok := following[path]
	if !ok || flag == "" {
		return ok
	}
	follow, _ := cmd.Flags().GetBool(flag)
	return follow
}

// limit ends cmd after maxFollow.
func limit(cmd *cobra.Command) {
	run := cmd.RunE
	cmd.RunE = func(cmd *cobra.Command, args []string) error {
		ctx, cancel := context.WithTimeoutCause(cmd.Context(), maxFollow, errFollowLimit)
		defer cancel()
		cmd.SetContext(ctx)
		if err := run(cmd, args); !errors.Is(context.Cause(ctx), errFollowLimit) {
			return err
		}
		return errFollowLimit
	}
}
