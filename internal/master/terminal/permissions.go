package terminal

import (
	"context"
	"errors"
	"strings"

	"github.com/spf13/cobra"

	"github.com/QwikByte/noryx/internal/master/access"
)

// check decides who may run a command.
type check struct {
	// run returns why the command may not run with its arguments, e.g. the permission the
	// grants lack, or nil.
	run func(ctx context.Context, g access.Grants, args []string) error
	// offer reports whether the grants allow the command with some arguments, so that the
	// panel offers it; nil for commands it never offers.
	offer func(g access.Grants) bool
}

var (
	errUnchecked        = errors.New("this command can't be run in the panel")
	errDatastoreRestore = errors.New("restore datastores on the Databases tab of their network, which gives the users of the databases their passwords first")
)

// need is the result of a check for permission p.
func need(p access.Permission, ok bool) error {
	if !ok {
		return access.Denied(p)
	}
	return nil
}

// grantsOnly is the check of a command that needs p whatever its arguments are; ok reports
// whether the grants allow it.
func grantsOnly(p access.Permission, ok func(access.Grants) bool) check {
	return check{
		run:   func(_ context.Context, g access.Grants, _ []string) error { return need(p, ok(g)) },
		offer: ok,
	}
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
		path := strings.TrimPrefix(cmd.CommandPath(), use+" ")
		c, ok := checks[path]
		if !ok {
			return errUnchecked
		}
		if err := c.run(cmd.Context(), access.From(cmd.Context()), args); err != nil {
			return err
		}
		if follows(cmd, path) {
			limit(cmd)
		}
		return nil
	}
	return root
}

// agentChecks are the checks of the agent's commands on a node. Commands that list all
// servers need the permission on the whole node, those for one server on that server, and
// those for datastores the global permission.
// Commands that change a server also ask moving, which refuses them while the server
// moves to another node, as the changes would be lost.
func agentChecks(nodeID string, moving func(serverID string) error) map[string]check {
	node := func(p access.Permission) check {
		return grantsOnly(p, func(g access.Grants) bool { return g.On(p, nodeID, "") })
	}
	server := func(p access.Permission) check {
		return check{
			run: func(_ context.Context, g access.Grants, args []string) error {
				return need(p, len(args) > 0 && g.On(p, nodeID, args[0]))
			},
			offer: func(g access.Grants) bool { return g.Somewhere(p, nodeID) },
		}
	}
	change := func(p access.Permission) check {
		c := server(p)
		run := c.run
		c.run = func(ctx context.Context, g access.Grants, args []string) error {
			if err := run(ctx, g, args); err != nil {
				return err
			}
			return moving(args[0])
		}
		return c
	}
	global := func(p access.Permission) check {
		return grantsOnly(p, func(g access.Grants) bool { return g.Has(p) })
	}
	return map[string]check{
		"datastore list":    global(access.DatastoresView),
		"datastore backups": global(access.DatastoresView),
		"datastore backup":  global(access.DatastoresManage),
		// Only the panel's Databases tab restores, as it gives the users of the databases their
		// passwords first; the local CLI is for emergencies.
		"datastore restore": {run: func(context.Context, access.Grants, []string) error { return errDatastoreRestore }},
		"status":            node(access.ServersView),
		"server list":       node(access.ServersView),
		"server start":      change(access.ServersStart),
		"server stop":       change(access.ServersStop),
		"server restart":    change(access.ServersRestart),
		"server logs":       server(access.ConsoleView),
		"server command":    change(access.ConsoleCommands),
		"backup list":       server(access.BackupsView),
		"backup create":     change(access.BackupsCreate),
		"backup restore":    change(access.BackupsRestore),
		"logs":              node(access.LogsView),
		"overlay status":    node(access.NodesView),
	}
}

// masterChecks are the checks of the master's commands. node list shows the nodes the user
// may see.
func (h *Handler) masterChecks() map[string]check {
	return map[string]check{
		"status":    grantsOnly(access.SettingsView, func(g access.Grants) bool { return g.Has(access.SettingsView) }),
		"node list": grantsOnly("", func(access.Grants) bool { return true }),
		// The entries are limited to the scope of the permission.
		"logs": grantsOnly(access.LogsView, func(g access.Grants) bool { return g.Somewhere(access.LogsView, "") }),
		"node renew": {
			run: func(ctx context.Context, g access.Grants, args []string) error {
				n, err := h.findNode(ctx, args[0])
				return need(access.NodesCertificates, err != nil || g.On(access.NodesCertificates, n.ID, "")) // unknown: renew reports it
			},
			offer: func(g access.Grants) bool {
				all, nodes, _ := g.Scope(access.NodesCertificates)
				return all || len(nodes) > 0
			},
		},
	}
}
