package terminal

import (
	"errors"
	"maps"
	"slices"
	"strings"
	"testing"

	"github.com/spf13/cobra"

	"github.com/QwikByte/mc-server-manager/internal/agentcli"
	"github.com/QwikByte/mc-server-manager/internal/master/access"
)

// Commands without a check can't run, so every command needs one, and every check a command.
func TestEveryCommandIsChecked(t *testing.T) {
	h := &Handler{}
	for name, tc := range map[string]struct {
		commands []*cobra.Command
		checks   map[string]check
	}{
		"mcsm-agent":  {agentcli.Commands(nil), agentChecks("n1", nil)},
		"mcsm-master": {h.masterCommands(), h.masterChecks()},
	} {
		root := &cobra.Command{Use: name}
		root.AddCommand(tc.commands...)
		var runnable []string
		var walk func(*cobra.Command)
		walk = func(c *cobra.Command) {
			for _, sub := range c.Commands() {
				if sub.Runnable() {
					runnable = append(runnable, strings.TrimPrefix(sub.CommandPath(), name+" "))
				}
				walk(sub)
			}
		}
		walk(root)
		slices.Sort(runnable)
		if checked := slices.Sorted(maps.Keys(tc.checks)); !slices.Equal(runnable, checked) {
			t.Errorf("%s: commands %q, checks %q", name, runnable, checked)
		}
	}
}

// Commands that change a server are refused while it moves, as the changes would be lost.
func TestMovingServers(t *testing.T) {
	errMoving := errors.New("moving")
	checks := agentChecks("n1", func(id string) error {
		if id == "moving" {
			return errMoving
		}
		return nil
	})
	changes := []string{"server start", "server stop", "server restart", "server command", "backup create", "backup restore"}
	for name, c := range checks {
		want := error(nil)
		if slices.Contains(changes, name) {
			want = errMoving
		}
		if err := c(t.Context(), access.Admin(), []string{"moving"}); !errors.Is(err, want) {
			t.Errorf("%s while moving: %v, want %v", name, err, want)
		}
		if err := c(t.Context(), access.Admin(), []string{"other"}); err != nil {
			t.Errorf("%s: %v", name, err)
		}
	}
	if err := checks["server start"](t.Context(), access.Grants{}, []string{"moving"}); errors.Is(err, errMoving) || err == nil {
		t.Errorf("without permission: %v", err)
	}
}
