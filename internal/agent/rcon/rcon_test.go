package rcon

import (
	"context"
	"encoding/binary"
	"errors"
	"io"
	"net"
	"slices"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

// long is an answer that the server splits into packets of 4096 bytes, like Minecraft.
var long = strings.Repeat("x", chunk+100)

// serve runs a console like Minecraft's with the password "secret" on a new port, and
// returns its address, how many connections it accepted and the commands it ran in order.
func serve(t *testing.T) (string, *atomic.Int32, func() []string) {
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { ln.Close() })
	var accepted atomic.Int32
	var mu sync.Mutex
	var commands []string
	ran := func(command string) {
		mu.Lock()
		defer mu.Unlock()
		commands = append(commands, command)
	}
	go func() {
		for {
			conn, err := ln.Accept()
			if err != nil {
				return
			}
			accepted.Add(1)
			go answer(conn, ran)
		}
	}()
	return ln.Addr().String(), &accepted, func() []string {
		mu.Lock()
		defer mu.Unlock()
		return slices.Clone(commands)
	}
}

func answer(conn net.Conn, ran func(command string)) {
	defer conn.Close()
	for {
		var header [12]byte
		if _, err := io.ReadFull(conn, header[:]); err != nil {
			return
		}
		data := make([]byte, binary.LittleEndian.Uint32(header[:4])-8)
		if _, err := io.ReadFull(conn, data); err != nil {
			return
		}
		body := string(data[:len(data)-2])
		id, answers := header[4:8], []string{"ran " + body}
		typ := binary.LittleEndian.Uint32(header[8:12])
		if typ == typeCommand {
			ran(body)
		}
		switch {
		case typ == typeResponse:
			answers = []string{"Unknown request 0"}
		case typ == typeAuth && body != "secret":
			id, answers = []byte{0xff, 0xff, 0xff, 0xff}, []string{""}
		case body == "long":
			answers = []string{long[:chunk], long[chunk:]}
		case body == "slow":
			time.Sleep(200 * time.Millisecond)
		case body == "stop":
			return // like a server that stops
		}
		for _, a := range answers {
			res := binary.LittleEndian.AppendUint32(nil, uint32(10+len(a))) //nolint:gosec // short
			res = append(append(append(res, id...), 0, 0, 0, 0), a...)
			_, _ = conn.Write(append(res, 0, 0))
		}
	}
}

func TestConn(t *testing.T) {
	addr, _, _ := serve(t)
	ctx, cancel := context.WithTimeout(t.Context(), time.Second)
	defer cancel()
	if _, err := dial(ctx, addr, "wrong"); !errors.Is(err, errAuth) {
		t.Fatalf("wrong password: %v", err)
	}
	r, err := dial(ctx, addr, "secret")
	if err != nil {
		t.Fatal(err)
	}
	defer r.Close()
	// The packets of a long answer are joined, and the connection stays usable.
	for _, command := range []string{"long", "tps"} {
		if out, err := r.call(ctx, typeCommand, command); err != nil || command == "long" && out != long || command == "tps" && out != "ran tps" {
			t.Fatalf("%s: %d bytes, %v", command, len(out), err)
		}
	}
}

func TestConsoles(t *testing.T) {
	addr, accepted, ran := serve(t)
	c := NewConsoles()
	targets := 0
	target := func(context.Context) (string, string, error) {
		targets++ // only called with the console's turn
		return addr, "secret", nil
	}
	run := func(run, command string) string {
		t.Helper()
		out, err := c.Command(t.Context(), "server", run, target, command)
		if err != nil {
			t.Fatal(err)
		}
		return out
	}

	// Commands share one connection, and run one after the other in the order they come.
	if out := run("1", "list"); out != "ran list" {
		t.Fatalf("answer = %q", out)
	}
	var wg sync.WaitGroup
	for _, command := range []string{"slow", "a", "b", "c", "d"} {
		wg.Go(func() {
			if out := run("1", command); out != "ran "+command {
				t.Errorf("answer to %s = %q", command, out)
			}
		})
		time.Sleep(20 * time.Millisecond) // so that they come in order, while the slow one runs
	}
	wg.Wait()
	if order, want := strings.Join(ran(), ","), "list,slow,a,b,c,d"; order != want {
		t.Errorf("commands ran in order %q, want %q", order, want)
	}
	if accepted.Load() != 1 || targets != 1 {
		t.Fatalf("%d connections, %d targets after the first run, want 1", accepted.Load(), targets)
	}

	// A new run of the server, a closed console and a broken connection connect again.
	run("2", "list")
	c.Close("server")
	run("2", "list")
	if _, err := c.Command(t.Context(), "server", "2", target, "stop"); err == nil {
		t.Fatal("no error from a connection that broke")
	}
	run("2", "list")
	if accepted.Load() != 4 {
		t.Fatalf("%d connections, want 4", accepted.Load())
	}

	// A command that waits for its turn gives up with its context.
	ctx, cancel := context.WithTimeout(t.Context(), 10*time.Millisecond)
	defer cancel()
	con := c.console("server")
	con.turn <- struct{}{}
	if _, err := c.Command(ctx, "server", "2", target, "list"); !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("waiting command: %v", err)
	}
	<-con.turn

	// Closing doesn't wait for a command that runs, e.g. on a server that crashed.
	slow := make(chan error)
	go func() {
		_, err := c.Command(t.Context(), "server", "2", target, "slow")
		slow <- err
	}()
	time.Sleep(50 * time.Millisecond)
	start := time.Now()
	c.Close("server")
	if waited := time.Since(start); waited > 100*time.Millisecond {
		t.Errorf("closing waited %v for the command", waited)
	}
	if err := <-slow; err != nil {
		t.Fatalf("slow command: %v", err)
	}
}
