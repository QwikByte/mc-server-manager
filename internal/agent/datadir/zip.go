package datadir

import (
	"archive/zip"
	"cmp"
	"context"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path"
	"path/filepath"
	"strings"
)

// stored lists extensions of files that are compressed already; deflating them again
// costs time for nothing, which matters for worlds of several gigabytes.
var stored = map[string]bool{".mca": true, ".jar": true, ".zip": true, ".gz": true, ".png": true, ".jpg": true, ".ogg": true}

// WriteZip writes files and folders of root as a ZIP archive; "." writes all of it.
// Symbolic links, other special files and temporary files of the agent are left out.
func WriteZip(ctx context.Context, w io.Writer, root *os.Root, paths ...string) error {
	zw := zip.NewWriter(w)
	var err error
	for _, p := range paths {
		if err = fs.WalkDir(root.FS(), p, func(name string, d fs.DirEntry, err error) error {
			switch {
			case err != nil || ctx.Err() != nil:
				return cmp.Or(err, ctx.Err())
			case IsTemp(d.Name()) && d.IsDir():
				return fs.SkipDir
			case name == "." || IsTemp(d.Name()) || (!d.IsDir() && !d.Type().IsRegular()):
				return nil
			}
			return addFile(zw, root, name, d)
		}); err != nil {
			break
		}
	}
	return errors.Join(err, zw.Close())
}

func addFile(zw *zip.Writer, root *os.Root, name string, d fs.DirEntry) error {
	info, err := d.Info()
	if err != nil {
		return err
	}
	header, err := zip.FileInfoHeader(info)
	if err != nil {
		return err
	}
	header.Name = name
	if d.IsDir() {
		header.Name += "/"
	} else if !stored[strings.ToLower(path.Ext(name))] {
		header.Method = zip.Deflate
	}
	fw, err := zw.CreateHeader(header)
	if err != nil || d.IsDir() {
		return err
	}
	f, err := root.Open(name)
	if err != nil {
		return err
	}
	_, err = io.Copy(fw, f)
	return errors.Join(err, f.Close())
}

// ExtractZip extracts an archive written by WriteZip into the directory dest. Entries
// belong to the owner of the data directory and keep their permissions, without special
// bits, and their modification times.
func (d *Dir) ExtractZip(ctx context.Context, zr *zip.Reader, dest string) error {
	if err := d.MkdirAll(dest); err != nil {
		return err
	}
	for _, f := range zr.File {
		name := filepath.FromSlash(strings.TrimSuffix(f.Name, "/"))
		if !filepath.IsLocal(name) {
			return fmt.Errorf("invalid name in archive: %q", f.Name)
		}
		if err := ctx.Err(); err != nil {
			return err
		}
		target := filepath.Join(dest, name)
		if f.FileInfo().IsDir() {
			if err := d.MkdirAll(target); err != nil {
				return err
			}
			continue
		}
		if err := d.MkdirAll(filepath.Dir(target)); err != nil {
			return err
		}
		if err := d.extract(f, target); err != nil {
			return err
		}
	}
	return nil
}

func (d *Dir) extract(f *zip.File, name string) error {
	r, err := f.Open()
	if err != nil {
		return err
	}
	defer r.Close()
	out, err := d.OpenFile(name, os.O_WRONLY|os.O_CREATE|os.O_EXCL, filePerm)
	if err != nil {
		return err
	}
	_, err = io.Copy(out, r) //nolint:gosec // archives are written by the agent itself
	if err := errors.Join(err, out.Close(), d.Chmod(name, f.Mode().Perm()), d.own(name)); err != nil {
		return err
	}
	return d.Chtimes(name, f.Modified, f.Modified)
}
