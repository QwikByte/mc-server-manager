package terminal

import (
	"reflect"
	"strings"
	"testing"

	"github.com/spf13/cobra"

	"github.com/QwikByte/noryx/internal/agentcli"
	"github.com/QwikByte/noryx/internal/master/access"
)

// The panel completes the commands as their definitions describe them, and only those the
// user may run.
func TestDescribe(t *testing.T) {
	h := &Handler{}
	describeAll := func(name string, commands []*cobra.Command, checks map[string]check, g access.Grants) []command {
		root := guarded(name, "", checks)
		root.AddCommand(commands...)
		root.InitDefaultHelpCmd()
		return describe(root, checks, g)
	}
	agent := describeAll("noryx-agent", agentcli.Commands(nil), agentChecks("n1", nil), access.Admin())
	master := describeAll("noryx-master", h.masterCommands(), h.masterChecks(), access.Admin())

	for path, want := range map[string][]arg{
		"server start":     {{Name: "id", Kind: "server"}},
		"server command":   {{Name: "id", Kind: "server"}, {Name: "command", Kind: "command", Repeated: true}},
		"backup restore":   {{Name: "server-id", Kind: "server"}, {Name: "backup-id", Kind: "backup"}},
		"datastore backup": {{Name: "datastore-id", Kind: "datastore"}, {Name: "database", Kind: "database", Optional: true, Repeated: true}},
		"help":             {{Name: "command", Kind: "command", Optional: true}},
		"status":           nil,
	} {
		if c, ok := find(agent, path); !ok || !reflect.DeepEqual(c.Args, want) {
			t.Errorf("agent %s: %+v, found %t; want arguments %+v", path, c.Args, ok, want)
		}
	}
	if c, _ := find(master, "node renew"); !reflect.DeepEqual(c.Args, []arg{{Name: "node", Kind: "node"}}) {
		t.Errorf("node renew: %+v", c.Args)
	}

	logs, _ := find(agent, "logs")
	if want := []flag{
		{Name: "follow", Shorthand: "f", Usage: "keep showing new entries until Ctrl+C"},
		{Name: "level", Usage: "least important level shown: debug, info, warn or error", Value: "level"},
		{Name: "lines", Shorthand: "n", Usage: "number of entries shown before following", Value: "lines"},
	}; !reflect.DeepEqual(logs.Flags, want) {
		t.Errorf("flags of logs: %+v", logs.Flags)
	}
	logs, _ = find(master, "logs")
	values := map[string]string{}
	for _, f := range logs.Flags {
		values[f.Name] = f.Value
	}
	if values["server"] != "server" || values["node"] != "node" || values["json"] != "" {
		t.Errorf("values of the master's logs flags: %v", values)
	}

	// The panel never restores datastores, so it doesn't offer it.
	if _, ok := find(agent, "datastore restore"); ok {
		t.Error("datastore restore offered")
	}
	// Without permissions, only what needs none is left, and groups without commands are left out.
	if got := names(describeAll("noryx-agent", agentcli.Commands(nil), agentChecks("n1", nil), access.Grants{})); got != "help" {
		t.Errorf("agent without permissions: %s", got)
	}
	if got := names(describeAll("noryx-master", h.masterCommands(), h.masterChecks(), access.Grants{})); got != "help, node, node list" {
		t.Errorf("master without permissions: %s", got)
	}
}

// find returns the command at a path such as "server start".
func find(list []command, path string) (command, bool) {
	var c command
	for name := range strings.FieldsSeq(path) {
		found := false
		for _, sub := range list {
			if sub.Name == name {
				c, list, found = sub, sub.Commands, true
				break
			}
		}
		if !found {
			return command{}, false
		}
	}
	return c, true
}

// names lists the paths of all commands, e.g. "help, node, node list".
func names(list []command) string {
	var paths []string
	var walk func(prefix string, list []command)
	walk = func(prefix string, list []command) {
		for _, c := range list {
			paths = append(paths, prefix+c.Name)
			walk(prefix+c.Name+" ", c.Commands)
		}
	}
	walk("", list)
	return strings.Join(paths, ", ")
}
