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

	"github.com/QwikByte/noryx/internal/agent/progress"
)

// stored lists extensions of files that are compressed already; deflating them again
// costs time for nothing, which matters for worlds of several gigabytes.
var stored = map[string]bool{".mca": true, ".jar": true, ".zip": true, ".gz": true, ".png": true, ".jpg": true, ".ogg": true}

// maxEdited limits the files a Censor changes, which are read at once.
const maxEdited = 1 << 20

// A Censor decides about each file of an archive, by its name, whether to omit it and how
// to edit its content, if at all. WriteZip asks it once the file is open, so that what marks
// a file as secret before it is written always counts.
type Censor func(name string) (omit bool, edit func([]byte) []byte)

// WriteZip writes files and folders of root as a ZIP archive; "." writes all of it.
// Symbolic links, other special files and temporary files of the agent are left out, and
// censor, if not nil, omits or edits files.
func WriteZip(ctx context.Context, w io.Writer, root *os.Root, censor Censor, paths ...string) error {
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
			return addFile(ctx, zw, root, name, d, censor)
		}); err != nil {
			break
		}
	}
	return errors.Join(err, zw.Close())
}

// CopyZip copies the archive zr to w, without compressing it again, as censor says.
func CopyZip(w io.Writer, zr *zip.Reader, censor Censor) error {
	zw := zip.NewWriter(w)
	for _, f := range zr.File {
		omit, edit := censor(f.Name)
		var err error
		switch {
		case omit:
			continue
		case edit == nil:
			err = zw.Copy(f)
		default:
			err = copyEdited(zw, f, edit)
		}
		if err != nil {
			return err
		}
	}
	return zw.Close()
}

func copyEdited(zw *zip.Writer, f *zip.File, edit func([]byte) []byte) error {
	r, err := f.Open()
	if err != nil {
		return err
	}
	defer r.Close()
	data, err := readEdited(r)
	if err != nil {
		return err
	}
	header := f.FileHeader
	fw, err := zw.CreateHeader(&header)
	if err == nil {
		_, err = fw.Write(edit(data))
	}
	return err
}

func readEdited(r io.Reader) ([]byte, error) {
	data, err := io.ReadAll(io.LimitReader(r, maxEdited+1))
	if err == nil && len(data) > maxEdited {
		err = fmt.Errorf("a file with secrets has more than %d bytes", maxEdited)
	}
	return data, err
}

func addFile(ctx context.Context, zw *zip.Writer, root *os.Root, name string, d fs.DirEntry, censor Censor) error {
	info, err := d.Info()
	if err != nil {
		return err
	}
	var f *os.File
	var edit func([]byte) []byte
	if !d.IsDir() {
		if f, err = root.Open(name); err != nil {
			return err
		}
		defer f.Close()
		if censor != nil {
			var omit bool
			if omit, edit = censor(name); omit {
				return nil
			}
		}
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
	if edit == nil {
		_, err = io.Copy(fw, progress.Reader(ctx, f))
		return err
	}
	data, err := readEdited(f)
	if err == nil {
		_, err = fw.Write(edit(data))
	}
	return err
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
		if err := d.extract(ctx, f, target); err != nil {
			return err
		}
	}
	return nil
}

func (d *Dir) extract(ctx context.Context, f *zip.File, name string) error {
	r, err := f.Open()
	if err != nil {
		return err
	}
	defer r.Close()
	out, err := d.OpenFile(name, os.O_WRONLY|os.O_CREATE|os.O_EXCL, filePerm)
	if err != nil {
		return err
	}
	_, err = io.Copy(out, progress.Reader(ctx, r)) //nolint:gosec // archives are written by the agent itself
	if err := errors.Join(err, out.Close(), d.Chmod(name, f.Mode().Perm()), d.own(name)); err != nil {
		return err
	}
	return d.Chtimes(name, f.Modified, f.Modified)
}
