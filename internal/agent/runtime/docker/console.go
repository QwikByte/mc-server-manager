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

	mcnet "github.com/QwikByte/mc-server-manager/internal/agent/network"
	"github.com/QwikByte/mc-server-manager/internal/agent/runtime"
)

// reloadTimeout is how long a proxy may take to answer its reload command.
const reloadTimeout = 30 * time.Second

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
