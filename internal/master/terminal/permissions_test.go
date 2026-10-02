package terminal

import (
	"maps"
	"slices"
	"strings"
	"testing"

	"github.com/spf13/cobra"

	"github.com/QwikByte/mc-server-manager/internal/agentcli"
)

// Commands without a check can't run, so every command needs one, and every check a command.
func TestEveryCommandIsChecked(t *testing.T) {
	h := &Handler{}
	for name, tc := range map[string]struct {
		commands []*cobra.Command
		checks   map[string]check
	}{
		"mcsm-agent":  {agentcli.Commands(nil), agentChecks("n1")},
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
