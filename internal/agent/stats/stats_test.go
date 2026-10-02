package stats

import (
	"bufio"
	"context"
	"encoding/binary"
	"errors"
	"io"
	"net"
	"strings"
	"testing"
	"time"
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

func TestRCONAndTPS(t *testing.T) {
	addr := serve(t, func(conn net.Conn) {
		for {
			var header [12]byte
			if _, err := io.ReadFull(conn, header[:]); err != nil {
				return
			}
			body := make([]byte, binary.LittleEndian.Uint32(header[:4])-8)
			if _, err := io.ReadFull(conn, body); err != nil {
				return
			}
			id, answer := header[4:8], ""
			switch string(body[:len(body)-2]) {
			case "wrong":
				id = []byte{0xff, 0xff, 0xff, 0xff}
			case "tps":
				answer = "§6TPS from last 1m, 5m, 15m: §a*20.01, §a19.5, §a19.8"
			}
			res := binary.LittleEndian.AppendUint32(nil, uint32(10+len(answer))) //nolint:gosec // short
			res = append(append(append(res, id...), 0, 0, 0, 0), answer...)
			_, _ = conn.Write(append(res, 0, 0))
		}
	})
	ctx, cancel := context.WithTimeout(t.Context(), time.Second)
	defer cancel()
	if _, err := dialRCON(ctx, addr.String(), "wrong"); !errors.Is(err, errRCONAuth) {
		t.Fatalf("wrong password: %v", err)
	}
	r, err := dialRCON(ctx, addr.String(), "secret")
	if err != nil {
		t.Fatal(err)
	}
	defer r.Close()
	out, err := r.call(ctx, rconCommand, "tps")
	if err != nil {
		t.Fatal(err)
	}
	if m := tpsPattern.FindStringSubmatch(plain(out)); m == nil || m[1] != "20.01" {
		t.Fatalf("tps output %q, match %v", out, m)
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
