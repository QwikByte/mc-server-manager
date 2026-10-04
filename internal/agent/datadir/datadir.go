// Package datadir gives the agent safe access to the data directory of a server. Paths
// can't leave the directory (os.Root), and whatever the agent creates belongs to the
// directory's owner: the agent runs as root, the server as the user its image chose,
// which can't read files owned by root.
package datadir

import (
	"crypto/rand"
	"errors"
	"io"
	"io/fs"
	"os"
	"path"
	"path/filepath"
	"slices"
	"strings"
	"syscall"
)

const (
	filePerm = 0o640
	dirPerm  = 0o750
	// tempPrefix starts the names of temporary files and folders of the agent.
	tempPrefix = ".noryx-"
	maxPath    = 1024
)

// Name turns a slash-separated path relative to a data directory into a name in it, "."
// for the directory itself. os.Root rejects escaping paths too, but ".." has no use here,
// so such paths are refused upfront, like backslashes.
func Name(p string) (string, bool) {
	if len(p) > maxPath || strings.ContainsAny(p, "\x00\\") || slices.Contains(strings.Split(p, "/"), "..") {
		return "", false
	}
	name := strings.TrimPrefix(path.Clean("/"+p), "/")
	if name == "" {
		return ".", true
	}
	return filepath.FromSlash(name), true
}

// IsTemp reports whether a file or folder name is one of the agent's temporary ones.
func IsTemp(name string) bool { return strings.HasPrefix(name, tempPrefix) }

// TempName returns a new name for a temporary file or folder in the folder dir.
func TempName(dir string) string { return filepath.Join(dir, tempPrefix+strings.ToLower(rand.Text())) }

// Dir is the data directory of a server. Its methods that create files and
// directories set their owner; files created otherwise belong to the agent.
type Dir struct {
	*os.Root
	uid, gid int // owner of new entries, -1 if the agent can't change owners
}

// Open opens an existing data directory.
func Open(path string) (*Dir, error) {
	root, err := os.OpenRoot(path)
	if err != nil {
		return nil, err
	}
	d := &Dir{Root: root, uid: -1, gid: -1}
	info, err := root.Stat(".")
	if err != nil {
		return nil, errors.Join(err, root.Close())
	}
	if st, ok := info.Sys().(*syscall.Stat_t); ok && os.Geteuid() == 0 {
		d.uid, d.gid = int(st.Uid), int(st.Gid)
	}
	return d, nil
}

// WriteFile replaces a file atomically, so the server never reads a partial file.
func (d *Dir) WriteFile(name string, data []byte) error {
	return d.Replace(name, true, func(w io.Writer) error {
		_, err := w.Write(data)
		return err
	})
}

// Replace writes a file through a temporary file next to it, which takes its place once
// write succeeded. An existing file keeps its permissions. Without overwrite, an
// existing file is left alone and fs.ErrExist is returned.
func (d *Dir) Replace(name string, overwrite bool, write func(io.Writer) error) (err error) {
	tmp := TempName(filepath.Dir(name))
	f, err := d.OpenFile(tmp, os.O_WRONLY|os.O_CREATE|os.O_EXCL, filePerm)
	if err != nil {
		return err
	}
	defer func() {
		if err != nil {
			_ = d.Remove(tmp) // best effort; the write error matters
		}
	}()
	if err := errors.Join(write(f), f.Close(), d.own(tmp)); err != nil {
		return err
	}
	if !overwrite {
		// A hard link never replaces an existing file, unlike a rename.
		if err := d.Link(tmp, name); err != nil {
			return err
		}
		return d.Remove(tmp)
	}
	if info, err := d.Stat(name); err == nil {
		if err := d.Chmod(tmp, info.Mode().Perm()); err != nil {
			return err
		}
	}
	return d.Rename(tmp, name)
}

// MkdirAll creates a directory and its missing parents.
func (d *Dir) MkdirAll(name string) error {
	parts := strings.Split(filepath.Clean(name), string(filepath.Separator))
	for i := range parts {
		dir := filepath.Join(parts[:i+1]...)
		if err := d.Mkdir(dir, dirPerm); errors.Is(err, fs.ErrExist) {
			continue
		} else if err != nil {
			return err
		}
		if err := d.own(dir); err != nil {
			return err
		}
	}
	return nil
}

// ReadOptional reads a file; a missing file reads as empty.
func (d *Dir) ReadOptional(name string) ([]byte, error) {
	data, err := d.ReadFile(name)
	if errors.Is(err, fs.ErrNotExist) {
		return nil, nil
	}
	return data, err
}

func (d *Dir) own(name string) error {
	if d.uid < 0 {
		return nil
	}
	return d.Lchown(name, d.uid, d.gid)
}
