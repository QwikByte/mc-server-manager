package stats

import (
	"bufio"
	"context"
	"encoding/binary"
	"io"
	"net"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/QwikByte/noryx/internal/agent/datadir"
	"github.com/QwikByte/noryx/internal/agent/runtime"
)

// serve accepts one connection on a new port and hands it to handle.
func serve(t *testing.T, handle func(net.Conn)) *net.TCPAddr {
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { ln.Close() })
	go func() {
		for {
			conn, err := ln.Accept()
			if err != nil {
				return
			}
			handle(conn)
			conn.Close()
		}
	}()
	return ln.Addr().(*net.TCPAddr)
}

func TestPing(t *testing.T) {
	addr := serve(t, func(conn net.Conn) {
		r := bufio.NewReader(conn)
		for range 2 { // handshake and status request
			n, _ := binary.ReadUvarint(r)
			_, _ = io.CopyN(io.Discard, r, int64(n))
		}
		status := `{"version":{"name":"1.21.4"},"players":{"max":20,"online":2,"sample":[{"name":"§aAlex"},{"name":"Steve"}]}}`
		_, _ = conn.Write(packet(appendString([]byte{0x00}, status)))
	})
	ctx, cancel := context.WithTimeout(t.Context(), time.Second)
	defer cancel()
	players, err := ping(ctx, uint32(addr.Port)) //nolint:gosec // a port
	if err != nil {
		t.Fatal(err)
	}
	if players.GetOnline() != 2 || players.GetMax() != 20 || strings.Join(players.GetNames(), ",") != "Alex,Steve" {
		t.Fatalf("players = %v", players)
	}
}

func TestTPS(t *testing.T) {
	out := plain("§6TPS from last 1m, 5m, 15m: §a*20.01, §a19.5, §a19.8")
	if m := tpsPattern.FindStringSubmatch(out); m == nil || m[1] != "20.01" {
		t.Fatalf("tps output %q, match %v", out, m)
	}
}

func TestListed(t *testing.T) {
	for out, want := range map[string]string{
		"There are 3 of a max of 20 players online: Alex, Steve, .Bedrock_1": "Alex,Steve,.Bedrock_1",
		"There are 0 of a max of 20 players online: ":                        "",
		"There are 2/20 players online:\nAlex, [Admin] Steve":                "Alex,Steve",
		"There are 2 of a max of 20 players online: @a, Alex":                "Alex",
	} {
		if names, ok := listed(out); !ok || strings.Join(names, ",") != want {
			t.Errorf("listed(%q) = %q, %v; want %q", out, names, ok, want)
		}
	}
	if _, ok := listed("Unknown or incomplete command, see below for error"); ok {
		t.Error("an unknown command lists players")
	}
}

func TestHost(t *testing.T) {
	cpu, err := readCPU()
	if err != nil {
		t.Skip("no /proc:", err)
	}
	used, total, err := readMemory()
	if err != nil || cpu.cores == 0 || cpu.busy > cpu.total || total == 0 || used > total {
		t.Fatalf("cpu = %+v, memory %d of %d, %v", cpu, used, total, err)
	}
}

// dataRuntime only serves the data directory of a server.
type dataRuntime struct {
	runtime.Runtime
	dir string
}

func (r dataRuntime) Data(context.Context, string) (*datadir.Dir, error) { return datadir.Open(r.dir) }

// The online mode of a server is read once per run, as a change applies when it starts again.
func TestOfflineMode(t *testing.T) {
	rt := dataRuntime{dir: t.TempDir()}
	write := func(props string) {
		if err := os.WriteFile(filepath.Join(rt.dir, "server.properties"), []byte(props), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	var st serverState
	write("online-mode=false\n")
	if !st.offlineMode(t.Context(), rt, "server") {
		t.Fatal("offline mode not reported")
	}
	write("online-mode=true\n")
	if !st.offlineMode(t.Context(), rt, "server") {
		t.Fatal("a change applied before the server started again")
	}
	st.reset()
	if st.offlineMode(t.Context(), rt, "server") {
		t.Fatal("offline mode reported after a restart in online mode")
	}
}
