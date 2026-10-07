package datadir

import (
	"cmp"
	"context"
	"errors"
	"io"
	"io/fs"
	"os"
	"syscall"

	"github.com/QwikByte/noryx/internal/agent/progress"
)

// Copy copies the data directory src into dst, which must not exist yet. Files keep their
// permissions and, if the agent runs as root, their owners. Symbolic links and other
// special files are left out, so the copy can't reach anything outside of src.
func Copy(ctx context.Context, src, dst string) error {
	from, err := os.OpenRoot(src)
	if err != nil {
		return err
	}
	defer from.Close()
	if err := os.Mkdir(dst, dirPerm); err != nil {
		return err
	}
	to, err := os.OpenRoot(dst)
	if err != nil {
		return err
	}
	defer to.Close()
	if progress.Active(ctx) {
		progress.Step(ctx, "copy", Size(from.FS(), "."))
	}
	return fs.WalkDir(from.FS(), ".", func(name string, d fs.DirEntry, err error) error {
		if err != nil || ctx.Err() != nil {
			return cmp.Or(err, ctx.Err())
		}
		info, err := d.Info()
		switch {
		case err != nil:
			return err
		case d.IsDir() && name != ".":
			err = to.Mkdir(name, dirPerm)
		case d.Type().IsRegular():
			err = copyFile(ctx, from, to, name)
		case !d.IsDir():
			return nil
		}
		if err != nil {
			return err
		}
		return keepMode(to, name, info)
	})
}

func copyFile(ctx context.Context, from, to *os.Root, name string) error {
	in, err := openPlain(from, name) // the server may have put a named pipe there meanwhile
	if err != nil {
		return err
	}
	defer in.Close()
	out, err := to.OpenFile(name, os.O_WRONLY|os.O_CREATE|os.O_EXCL, filePerm)
	if err != nil {
		return err
	}
	_, err = io.Copy(out, progress.Reader(ctx, in))
	return errors.Join(err, out.Close())
}

// Size returns the size of the regular files at paths of fsys and below them.
func Size(fsys fs.FS, paths ...string) int64 {
	var size int64
	for _, p := range paths {
		_ = fs.WalkDir(fsys, p, func(_ string, d fs.DirEntry, err error) error {
			if info, ierr := d.Info(); err == nil && ierr == nil && d.Type().IsRegular() {
				size += info.Size()
			}
			return nil
		})
	}
	return size
}

// keepMode gives a copied entry the permissions and owner of the original.
func keepMode(root *os.Root, name string, info fs.FileInfo) error {
	if err := root.Chmod(name, info.Mode().Perm()); err != nil {
		return err
	}
	if st, ok := info.Sys().(*syscall.Stat_t); ok && os.Geteuid() == 0 {
		return root.Lchown(name, int(st.Uid), int(st.Gid))
	}
	return nil
}
