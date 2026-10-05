package docker

import (
	"bufio"
	"cmp"
	"context"
	"errors"
	"fmt"
	"io"
	"regexp"
	"strings"
	"time"

	"github.com/moby/moby/api/pkg/stdcopy"
	"github.com/moby/moby/client"

	noryxv1 "github.com/QwikByte/noryx/api/noryx/v1"
	mcnet "github.com/QwikByte/noryx/internal/agent/network"
	"github.com/QwikByte/noryx/internal/agent/runtime"
)

const (
	// reloadTimeout is how long a proxy may take to answer its reload command.
	reloadTimeout = 30 * time.Second
	// sendTimeout is how long BungeeCord may take to answer its send command, which it
	// answers right away.
	sendTimeout = 5 * time.Second
)

// notSent are parts of the answers of BungeeCord to a send command that can't send anyone.
var notSent = []string{"That user is not online", "The specified server does not exist", "Only in game players"}

// Reload sends the reload command of a running proxy to its console and waits for its
// answer. A proxy created by an older agent can't read commands, so it is created again.
func (d *Docker) Reload(ctx context.Context, id string) error {
	c, spec, err := d.inspect(ctx, id)
	if err != nil {
		return err
	}
	p, ok := mcnet.ProxyOf(spec.Type)
	switch {
	case !ok:
		return runtime.ErrUnsupported
	case !c.State.Running:
		return runtime.ErrNotRunning
	case !c.Config.OpenStdin:
		return d.recreate(ctx, spec, true, placement(c, spec))
	}
	ctx, cancel := context.WithTimeout(ctx, reloadTimeout)
	defer cancel()
	return d.console(ctx, id, p.Reload, reloaded(p))
}

// reloaded returns what reads the answer of a proxy to its reload command, line by line,
// until it tells whether the proxy reloaded. Velocity logs why it couldn't before it
// answers, BungeeCord the exception after.
func reloaded(p mcnet.Proxy) func(line string) (bool, error) {
	var reason string
	failed := false
	return func(line string) (bool, error) {
		line = plainLine(line)
		if _, message, ok := strings.Cut(line, "Exception: "); ok {
			reason = message
		} else if _, message, ok := strings.Cut(line, "ERROR]: "); ok {
			reason = message
		}
		switch {
		case strings.Contains(line, p.Reloaded):
			return true, nil
		case strings.Contains(line, p.Failed):
			if failed = true; reason == "" {
				return false, nil
			}
		case !failed:
			return false, nil
		}
		return true, fmt.Errorf("%w: %s", runtime.ErrReload, cmp.Or(reason, "its console tells why"))
	}
}

// command writes a command to the console of a proxy. BungeeCord's answer to sending a player
// tells whether it could, so it is awaited: without its module cmd_send, which Waterfall can't
// download anymore, BungeeCord has no send command. Velocity only answers if it can't send.
func (d *Docker) command(ctx context.Context, id string, typ noryxv1.ServerType, command string) error {
	if args := strings.Fields(command); !typ.Bungee() || len(args) != 3 || !strings.EqualFold(args[0], "send") {
		return d.console(ctx, id, command, nil)
	}
	ctx, cancel := context.WithTimeout(ctx, sendTimeout)
	defer cancel()
	return d.console(ctx, id, command, sent)
}

// sent reads the answer of BungeeCord to a send command, line by line, until it tells whether
// BungeeCord sends the player.
func sent(line string) (bool, error) {
	line = plainLine(line)
	switch {
	case strings.Contains(line, "Attempting to send"):
		return true, nil
	case strings.Contains(line, "Command not found"):
		return true, runtime.ErrNoSend
	}
	for _, answer := range notSent {
		if i := strings.Index(line, answer); i >= 0 {
			return true, fmt.Errorf("%w: %s", runtime.ErrNotSent, line[i:])
		}
	}
	return false, nil
}

// console writes a command to the standard input of a proxy, which reads its console
// commands from there. If answer is set, it reads the console's output until answer accepts
// a line, and returns the error answer returns for it.
func (d *Docker) console(ctx context.Context, id, command string, answer func(line string) (bool, error)) error {
	res, err := d.cli.ContainerAttach(ctx, containerName(id), client.ContainerAttachOptions{
		Stream: true, Stdin: true, Stdout: answer != nil, Stderr: answer != nil,
	})
	if err != nil {
		return notFound(err)
	}
	defer res.Close()
	stop := context.AfterFunc(ctx, res.Close) // unblocks reading once ctx is done
	defer stop()
	if _, err := io.WriteString(res.Conn, command+"\n"); err != nil || answer == nil {
		return err
	}
	r, w := io.Pipe()
	defer r.Close()
	go func() {
		_, err := stdcopy.StdCopy(w, w, res.Reader)
		w.CloseWithError(err)
	}()
	lines := bufio.NewScanner(r)
	lines.Buffer(make([]byte, 0, 64<<10), maxLineBytes)
	for lines.Scan() {
		if done, err := answer(lines.Text()); done {
			return err
		}
	}
	return cmp.Or(ctx.Err(), lines.Err(), errors.New("the proxy's console closed"))
}

// escapes are the terminal escape sequences that colour the output of BungeeCord.
var escapes = regexp.MustCompile(`\x1b\[[0-9;?]*[ -/]*[@-~]`)

// plainLine removes colours and the prompt from a line of a proxy's console.
func plainLine(line string) string {
	return strings.TrimLeft(escapes.ReplaceAllString(line, ""), "> ")
}
