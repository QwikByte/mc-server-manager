package backup

import (
	"archive/zip"
	"context"
	"errors"
	"io/fs"
	"path/filepath"
	"slices"

	"github.com/QwikByte/noryx/internal/agent/datadir"
	"github.com/QwikByte/noryx/internal/agent/fileset"

	"github.com/QwikByte/noryx/internal/agent/progress"
)

// stage extracts a backup into a temporary folder of the data directory, which the caller
// removes unless its name is empty. Nothing of the server changes yet, so it can keep
// running meanwhile.
func stage(ctx context.Context, dir *datadir.Dir, b backup) (string, error) {
	zr, err := zip.OpenReader(b.archive())
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

// swap replaces the files and folders of a backup with their staged state. Those that
// didn't exist when the backup was made are removed, except for the current manifest of file
// sets, which keeps the files hidden that held secrets.
func swap(dir *datadir.Dir, b backup, staged string) error {
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
