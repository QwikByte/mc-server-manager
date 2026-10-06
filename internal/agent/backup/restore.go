package backup

import (
	"archive/zip"
	"context"
	"errors"
	"io/fs"
	"path/filepath"
	"slices"
	"strings"

	"github.com/QwikByte/noryx/internal/agent/datadir"
	"github.com/QwikByte/noryx/internal/agent/fileset"

	"github.com/QwikByte/noryx/internal/agent/progress"
)

// stage extracts a backup into a temporary folder of the data directory, which the caller
// removes unless its name is empty. Nothing of the server changes yet, so it can keep
// running meanwhile.
func stage(ctx context.Context, dir *datadir.Dir, b Archive) (string, error) {
	zr, err := zip.OpenReader(b.Path())
	if err != nil {
		return "", err
	}
	defer zr.Close()
	var size int64
	for _, f := range zr.File {
		size += int64(f.UncompressedSize64) //nolint:gosec // archives are written by the agent itself
	}
	progress.Step(ctx, "restore", size)
	tmp := datadir.TempName(".")
	return tmp, dir.ExtractZip(ctx, &zr.Reader, tmp)
}

// keepMarked keeps the files that held secrets of file sets as they are now, rather than as a
// backup has them: their sets put them on the server, and a backup must not bring back the
// secrets of a set that the server is no longer a target of. It links them into the staged
// backup, which swap moves into place, so it runs while the server is stopped; if it fails,
// they stay where they are.
func keepMarked(dir *datadir.Dir, b Archive, staged string, marked []string) error {
	for _, m := range marked {
		if !slices.ContainsFunc(b.Paths, func(p string) bool { return p == "." || m == p || strings.HasPrefix(m, p+string(filepath.Separator)) }) {
			continue // the backup doesn't touch it
		}
		target := filepath.Join(staged, m)
		if err := dir.RemoveAll(target); err != nil {
			return err
		}
		if _, err := dir.Lstat(m); errors.Is(err, fs.ErrNotExist) {
			continue
		} else if err != nil {
			return err
		}
		if err := dir.MkdirAll(filepath.Dir(target)); err != nil {
			return err
		}
		if err := dir.Link(m, target); err != nil {
			return err
		}
	}
	return nil
}

// swap replaces the files and folders of a backup with their staged state. Those that
// didn't exist when the backup was made are removed, except for the current manifest of file
// sets, which keeps the files hidden that held secrets.
func swap(dir *datadir.Dir, b Archive, staged string) error {
	paths := b.Paths
	if slices.Equal(paths, []string{"."}) {
		current, err := names(dir, ".")
		if err != nil {
			return err
		}
		restored, err := names(dir, staged)
		if err != nil {
			return err
		}
		paths = append(current, restored...)
		slices.Sort(paths)
		paths = slices.Compact(paths) // a second removal would hit the restored entry
	}
	paths = slices.DeleteFunc(slices.Clone(paths), func(p string) bool { return p == fileset.ManifestFile })
	for _, p := range paths {
		if err := dir.RemoveAll(p); err != nil {
			return err
		}
		from := filepath.Join(staged, p)
		if _, err := dir.Lstat(from); errors.Is(err, fs.ErrNotExist) {
			continue
		}
		if err := dir.MkdirAll(filepath.Dir(p)); err != nil {
			return err
		}
		if err := dir.Rename(from, p); err != nil {
			return err
		}
	}
	return nil
}

// names lists a folder of the data directory, without temporary files.
func names(dir *datadir.Dir, folder string) ([]string, error) {
	entries, err := fs.ReadDir(dir.FS(), folder)
	var list []string
	for _, e := range entries {
		if !datadir.IsTemp(e.Name()) {
			list = append(list, e.Name())
		}
	}
	return list, err
}
