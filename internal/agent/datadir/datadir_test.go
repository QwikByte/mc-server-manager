package datadir

import (
	"errors"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"testing"
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
