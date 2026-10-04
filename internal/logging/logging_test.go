package logging

import (
	"errors"
	"log/slog"
	"maps"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestHandler(t *testing.T) {
	var got []Entry
	log := slog.New(NewHandler(slog.LevelInfo, func(e Entry) { got = append(got, e) }))
	log.Debug("hidden")
	log.With(Servers, KeyUser, "alice").WithGroup("req").Warn("restart failed", KeyNode, "n1", "err", errors.New("boom"), slog.Group("peer", "ip", "10.0.0.1"))
	log.Log(t.Context(), slog.LevelError+2, "plain")

	if len(got) != 2 {
		t.Fatalf("got %d entries, want 2", len(got))
	}
	e := got[0]
	want := map[string]string{"req.node": "n1", "req.err": "boom", "req.peer.ip": "10.0.0.1"}
	if e.Category != "servers" || e.User != "alice" || e.Level != slog.LevelWarn || !maps.Equal(e.Attrs, want) {
		t.Fatalf("entry = %+v", e)
	}
	if got[1].Category != "system" || got[1].Level != slog.LevelError {
		t.Fatalf("entry without category = %+v", got[1])
	}
}

func TestRotatingFile(t *testing.T) {
	name := filepath.Join(t.TempDir(), "noryx.log")
	f, err := openRotating(name)
	if err != nil {
		t.Fatal(err)
	}
	line := []byte(strings.Repeat("x", maxFileSize/2-1) + "\n")
	for range 2 * (keepFiles + 2) {
		if _, err := f.Write(line); err != nil {
			t.Fatal(err)
		}
	}
	if err := f.Close(); err != nil {
		t.Fatal(err)
	}
	files, _ := filepath.Glob(name + "*")
	if len(files) != keepFiles+1 {
		t.Fatalf("files = %v, want the log and %d older ones", files, keepFiles)
	}
	if info, err := os.Stat(name); err != nil || info.Mode().Perm() != 0o600 || info.Size() > maxFileSize {
		t.Fatalf("log file: %v, %v", info, err)
	}
}

func TestLevelsAndLines(t *testing.T) {
	if l, err := ParseLevel("WARN"); err != nil || l != slog.LevelWarn {
		t.Fatalf("ParseLevel(WARN) = %v, %v", l, err)
	}
	if _, err := ParseLevel("loud"); err == nil {
		t.Fatal("ParseLevel accepted an unknown level")
	}
	at := time.Date(2026, 10, 2, 10, 0, 0, 0, time.Local)
	line := Line(at, slog.LevelError+1, "auth", "Sign-in failed", map[string]string{"ip": "10.0.0.1", "attempt": "bob\nx"})
	if want := `2026-10-02 10:00:00  ERROR  auth       Sign-in failed  attempt="bob\nx"  ip="10.0.0.1"`; line != want {
		t.Fatalf("line = %q, want %q", line, want)
	}
}
