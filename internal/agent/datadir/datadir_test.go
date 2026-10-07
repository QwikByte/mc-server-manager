package datadir

import (
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"syscall"
	"testing"
	"time"
)

func TestDir(t *testing.T) {
	path := t.TempDir()
	d, err := Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer d.Close()

	if err := d.MkdirAll("plugins/Example"); err != nil {
		t.Fatal(err)
	}
	if err := d.WriteFile("plugins/Example/config.yml", []byte("a: 1\n")); err != nil {
		t.Fatal(err)
	}
	// Replacing a file keeps its permissions, e.g. the execute bit of a start script.
	if err := os.Chmod(filepath.Join(path, "plugins/Example/config.yml"), 0o750); err != nil {
		t.Fatal(err)
	}
	if err := d.WriteFile("plugins/Example/config.yml", []byte("a: 2\n")); err != nil {
		t.Fatal(err)
	}
	if info, err := d.Stat("plugins/Example/config.yml"); err != nil || info.Mode().Perm() != 0o750 {
		t.Fatalf("mode after replace = %v, %v", info.Mode(), err)
	}

	// Without overwrite, an existing file stays as it is.
	err = d.Replace("plugins/Example/config.yml", false, func(w io.Writer) error {
		_, err := w.Write([]byte("lost"))
		return err
	})
	if !errors.Is(err, fs.ErrExist) {
		t.Fatalf("replace without overwrite: %v", err)
	}
	if data, _ := d.ReadOptional("plugins/Example/config.yml"); string(data) != "a: 2\n" {
		t.Fatalf("content = %q", data)
	}
	if entries, _ := os.ReadDir(filepath.Join(path, "plugins/Example")); len(entries) != 1 {
		t.Fatalf("temporary files left behind: %v", entries)
	}

	// Paths can't leave the data directory.
	for _, name := range []string{"../escape", "/etc/passwd", "plugins/../../escape"} {
		if err := d.WriteFile(name, nil); err == nil {
			t.Errorf("wrote %q outside the data directory", name)
		}
	}
	if data, err := d.ReadOptional("missing.txt"); data != nil || err != nil {
		t.Fatalf("missing file = %q, %v", data, err)
	}
}

// The server owns its files, so the agent neither waits for a named pipe nor reads a huge
// file at once.
func TestServerFiles(t *testing.T) {
	path := t.TempDir()
	d, err := Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer d.Close()
	if err := syscall.Mkfifo(filepath.Join(path, "server.properties"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(path, "huge.yml"), nil, 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.Truncate(filepath.Join(path, "huge.yml"), 1<<40); err != nil { // sparse
		t.Fatal(err)
	}
	done := make(chan string, 1)
	go func() {
		_, err := d.Open("server.properties")
		_, optionalErr := d.ReadOptional("server.properties")
		_, hugeErr := d.ReadFile("huge.yml")
		dir, dirErr := d.Open(".") // folders open
		if dirErr == nil {
			dirErr = dir.Close()
		}
		if err == nil || optionalErr == nil || hugeErr == nil || dirErr != nil {
			done <- fmt.Sprintf("pipe: %v, %v; huge file: %v; folder: %v", err, optionalErr, hugeErr, dirErr)
		}
		close(done)
	}()
	select {
	case problem := <-done:
		if problem != "" {
			t.Fatal(problem)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("waited for a writer of the named pipe")
	}
}

// A directory handed over to the user of a server belongs to it with everything in it,
// also what the agent writes later; a link to a file outside isn't followed.
func TestHandOver(t *testing.T) {
	if os.Geteuid() != 0 {
		t.Skip("changing owners needs root")
	}
	path, outside := t.TempDir(), filepath.Join(t.TempDir(), "outside")
	if err := os.WriteFile(outside, nil, 0o600); err != nil {
		t.Fatal(err)
	}
	d, err := Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer d.Close()
	if err := errors.Join(d.MkdirAll("plugins"), d.WriteFile("plugins/a.jar", nil), os.Symlink(outside, filepath.Join(path, "link"))); err != nil {
		t.Fatal(err)
	}

	if err := d.HandOver(1000, 1001); err != nil {
		t.Fatal(err)
	}
	if err := d.WriteFile("velocity.toml", nil); err != nil {
		t.Fatal(err)
	}
	owner := func(name string) [2]uint32 {
		info, err := os.Lstat(name)
		if err != nil {
			t.Fatal(err)
		}
		st := info.Sys().(*syscall.Stat_t)
		return [2]uint32{st.Uid, st.Gid}
	}
	for _, name := range []string{".", "plugins", "plugins/a.jar", "link", "velocity.toml"} {
		if got := owner(filepath.Join(path, name)); got != [2]uint32{1000, 1001} {
			t.Errorf("owner of %s = %v", name, got)
		}
	}
	if got := owner(outside); got != [2]uint32{0, 0} {
		t.Errorf("followed the link: owner of its target = %v", got)
	}
}
