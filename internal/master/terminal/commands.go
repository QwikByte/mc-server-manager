package terminal

import (
	"net/http"
	"regexp"
	"strings"

	"github.com/spf13/cobra"
	"github.com/spf13/pflag"

	"github.com/QwikByte/noryx/internal/master/access"
	"github.com/QwikByte/noryx/internal/master/httpapi"
)

// command describes a command for the panel, which completes it: its subcommands, or its
// arguments and flags.
type command struct {
	Name     string    `json:"name"`
	Short    string    `json:"short"`
	Args     []arg     `json:"args,omitempty"`
	Flags    []flag    `json:"flags,omitempty"`
	Commands []command `json:"commands,omitempty"`
}

// arg is an argument as the usage line of its command names it, e.g. <server-id> or [database]...
type arg struct {
	Name string `json:"name"`
	// Kind is what the argument names: its name without -id, e.g. server or database, and for
	// <id> what the parent command is about, e.g. server for server start <id>.
	Kind     string `json:"kind"`
	Optional bool   `json:"optional,omitempty"`
	Repeated bool   `json:"repeated,omitempty"`
}

type flag struct {
	Name      string `json:"name"`
	Shorthand string `json:"shorthand,omitempty"`
	Usage     string `json:"usage"`
	// Value is the kind of value the flag takes, which is its name, e.g. server for --server;
	// empty for switches such as --follow.
	Value string `json:"value,omitempty"`
}

// placeholder is an argument in a usage line: <required> or [optional], with ... if it repeats.
var placeholder = regexp.MustCompile(`([<[])([\w-]+)[>\]](\.\.\.)?`)

// commands describes the commands of a target that the user may run, for the panel to
// complete them. Running a command checks its permission again, with its arguments.
func (h *Handler) commands(w http.ResponseWriter, r *http.Request) {
	root, checks, err := h.root(r.Context(), r.URL.Query().Get("target"))
	if err != nil {
		httpapi.WriteError(w, r, err)
		return
	}
	root.InitDefaultHelpCmd()
	httpapi.WriteJSON(w, http.StatusOK, describe(root, checks, access.From(r.Context())))
}

// describe returns the subcommands of parent that the grants allow with some arguments, and
// those with such subcommands. help, which guarded always runs, is allowed too.
func describe(parent *cobra.Command, checks map[string]check, g access.Grants) []command {
	list := []command{}
	for _, c := range parent.Commands() {
		d := command{Name: c.Name(), Short: c.Short, Args: args(c), Flags: flags(c), Commands: describe(c, checks, g)}
		offer := checks[strings.TrimPrefix(c.CommandPath(), c.Root().Name()+" ")].offer
		runs := c.Runnable() && (c.Name() == "help" || offer != nil && offer(g))
		if !c.Hidden && (runs || len(d.Commands) > 0) {
			list = append(list, d)
		}
	}
	return list
}

// args reads the arguments of a command from its usage line.
func args(c *cobra.Command) []arg {
	var list []arg
	for _, m := range placeholder.FindAllStringSubmatch(c.Use, -1) {
		kind := strings.TrimSuffix(m[2], "-id")
		if kind == "id" && c.HasParent() {
			kind = c.Parent().Name()
		}
		list = append(list, arg{Name: m[2], Kind: kind, Optional: m[1] == "[", Repeated: m[3] != ""})
	}
	return list
}

func flags(c *cobra.Command) []flag {
	var list []flag
	c.Flags().VisitAll(func(f *pflag.Flag) {
		if f.Hidden || f.Deprecated != "" {
			return
		}
		value := f.Name
		if f.NoOptDefVal != "" { // a switch, which needs no value
			value = ""
		}
		list = append(list, flag{Name: f.Name, Shorthand: f.Shorthand, Usage: f.Usage, Value: value})
	})
	return list
}
