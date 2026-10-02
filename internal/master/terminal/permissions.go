package terminal

import (
	"context"
	"errors"
	"strings"

	"github.com/spf13/cobra"

	"github.com/QwikByte/mc-server-manager/internal/master/access"
)

// check returns why a command with its arguments may not run, e.g. the permission the
// grants lack, or nil.
type check func(ctx context.Context, g access.Grants, args []string) error

var errUnchecked = errors.New("this command can't be run in the panel")

// need is the result of a check for permission p.
func need(p access.Permission, ok bool) error {
	if !ok {
		return access.Denied(p)
	}
	return nil
}

// guarded returns a root command whose commands only run if their check allows it.
// Commands without a check don't run at all, so new commands can't slip through.
func guarded(use, short string, checks map[string]check) *cobra.Command {
	root := &cobra.Command{Use: use, Short: short, SilenceUsage: true, SilenceErrors: true}
	root.CompletionOptions.DisableDefaultCmd = true
	root.PersistentPreRunE = func(cmd *cobra.Command, args []string) error {
		if cmd.Name() == "help" {
			return nil
		}
		c, ok := checks[strings.TrimPrefix(cmd.CommandPath(), use+" ")]
		if !ok {
			return errUnchecked
		}
		return c(cmd.Context(), access.From(cmd.Context()), args)
	}
	return root
}

// agentChecks are the checks of the agent's commands on a node. Commands that list all
// servers need the permission on the whole node, those for one server on that server.
// Commands that change a server also ask moving, which refuses them while the server
// moves to another node, as the changes would be lost.
func agentChecks(nodeID string, moving func(serverID string) error) map[string]check {
	node := func(p access.Permission) check {
		return func(_ context.Context, g access.Grants, _ []string) error {
			return need(p, g.On(p, nodeID, ""))
		}
	}
	server := func(p access.Permission) check {
		return func(_ context.Context, g access.Grants, args []string) error {
			return need(p, len(args) > 0 && g.On(p, nodeID, args[0]))
		}
	}
	change := func(p access.Permission) check {
		return func(ctx context.Context, g access.Grants, args []string) error {
			if err := server(p)(ctx, g, args); err != nil {
				return err
			}
			return moving(args[0])
		}
	}
	return map[string]check{
		"status":         node(access.ServersView),
		"server list":    node(access.ServersView),
		"server start":   change(access.ServersStart),
		"server stop":    change(access.ServersStop),
		"server restart": change(access.ServersRestart),
		"server logs":    server(access.ConsoleView),
		"server command": change(access.ConsoleCommands),
		"backup list":    server(access.BackupsView),
		"backup create":  change(access.BackupsCreate),
		"backup restore": change(access.BackupsRestore),
		"logs":           node(access.LogsView),
	}
}

// masterChecks are the checks of the master's commands. node list shows the nodes the user
// may see.
func (h *Handler) masterChecks() map[string]check {
	return map[string]check{
		"status": func(_ context.Context, g access.Grants, _ []string) error {
			return need(access.SettingsView, g.Has(access.SettingsView))
		},
		"node list": func(context.Context, access.Grants, []string) error { return nil },
		// The entries are limited to the scope of the permission.
		"logs": func(_ context.Context, g access.Grants, _ []string) error {
			return need(access.LogsView, g.Somewhere(access.LogsView, ""))
		},
		"node renew": func(ctx context.Context, g access.Grants, args []string) error {
			n, err := h.findNode(ctx, args[0])
			return need(access.NodesCertificates, err != nil || g.On(access.NodesCertificates, n.ID, "")) // unknown: renew reports it
		},
	}
}
