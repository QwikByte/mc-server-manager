package datadir

import (
	"context"
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"testing"
)

func TestCopy(t *testing.T) {
	src, dst := t.TempDir(), filepath.Join(t.TempDir(), "copy")
	for name, content := range map[string]string{"server.properties": "motd=hi\n", "world/region/r.0.0.mca": "chunks", "start.sh": "#!/bin/sh\n"} {
		if err := os.MkdirAll(filepath.Join(src, filepath.Dir(name)), 0o700); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(src, name), []byte(content), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	for name, mode := range map[string]fs.FileMode{".": 0o700, "start.sh": 0o750} {
		if err := os.Chmod(filepath.Join(src, name), mode); err != nil {
			t.Fatal(err)
		}
	}
	// A link to a file outside of the data directory must not be followed.
	secret := filepath.Join(t.TempDir(), "secret")
	if err := os.WriteFile(secret, []byte("secret"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(secret, filepath.Join(src, "world", "link")); err != nil {
		t.Fatal(err)
	}

	if err := Copy(t.Context(), src, dst); err != nil {
		t.Fatal(err)
	}
	if data, err := os.ReadFile(filepath.Join(dst, "world/region/r.0.0.mca")); err != nil || string(data) != "chunks" {
		t.Fatalf("copied world = %q, %v", data, err)
	}
	for name, mode := range map[string]fs.FileMode{".": 0o700, "world": 0o700, "start.sh": 0o750, "server.properties": 0o600} {
		if info, err := os.Stat(filepath.Join(dst, name)); err != nil || info.Mode().Perm() != mode {
			t.Errorf("mode of %s = %v, %v; want %v", name, info.Mode().Perm(), err, mode)
		}
	}
	if _, err := os.Lstat(filepath.Join(dst, "world", "link")); !errors.Is(err, fs.ErrNotExist) {
		t.Fatalf("symbolic link was copied: %v", err)
	}

	// An existing directory is never copied into, and a cancelled copy stops.
	if err := Copy(t.Context(), src, dst); !errors.Is(err, fs.ErrExist) {
		t.Fatalf("copy into an existing directory: %v", err)
	}
	ctx, cancel := context.WithCancel(t.Context())
	cancel()
	if err := Copy(ctx, src, filepath.Join(t.TempDir(), "cancelled")); !errors.Is(err, context.Canceled) {
		t.Fatalf("cancelled copy: %v", err)
	}
}
