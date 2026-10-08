package archive

import (
	"cmp"
	"context"
	"errors"
	"io"
	"io/fs"
	"path"
	"path/filepath"
	"slices"
	"strings"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	noryxv1 "github.com/QwikByte/noryx/api/noryx/v1"
	"github.com/QwikByte/noryx/internal/agent/datadir"
	"github.com/QwikByte/noryx/internal/agent/progress"
	"github.com/QwikByte/noryx/internal/agent/secrets"
	"github.com/QwikByte/noryx/internal/agent/storage"
)

// Options decide how an archive is extracted.
type Options struct {
	// Protected tells the paths of the data directory, clean names in it, that no entry may
	// be written to, also not inside them. Such an entry refuses the archive, or with Skip is
	// left out.
	Protected func(name string) bool
	Skip      bool
	// Overwrite replaces files that exist; otherwise such a file refuses the archive.
	Overwrite bool
	// Only, if set, chooses the entries to extract by their names.
	Only func(name string) bool
	// Step names the progress of extracting; "extract" if empty.
	Step string
}

// Result tells what Extract extracted, and the entries it left out, up to maxLeftOut.
type Result struct {
	Files   int64
	Size    int64
	LeftOut []string
}

const maxLeftOut = 100

// Extract extracts the archive into the folder dest of dir, a clean name in it, which is
// created if it is missing. It decides first where each entry goes, and refuses the archive
// before it writes anything if one can't go there: to a protected path, through a link, to a
// file where a folder is or the other way round, or to a file that exists unless Overwrite
// is set; or if the files don't fit. New files and folders belong to the owner of dir.
func (a *Archive) Extract(ctx context.Context, dir *datadir.Dir, dest string, o Options) (Result, error) {
	targets, res, err := a.plan(dir, dest, o)
	if err != nil {
		return res, err
	}
	top, err := dir.Open(".")
	if err != nil {
		return res, err
	}
	defer top.Close()
	if err := storage.Fits(top, res.Size); err != nil {
		return res, err
	}
	progress.Step(ctx, cmp.Or(o.Step, "extract"), res.Size)
	made := map[string]bool{} // folders created or found
	write := func(i int, r io.Reader) error {
		e, target := a.Entries[i], targets[i]
		switch {
		case ctx.Err() != nil:
			return ctx.Err()
		case target == "":
			return nil
		case e.Dir:
			return folder(dir, target, made)
		}
		if err := folder(dir, filepath.Dir(target), made); err != nil {
			return err
		}
		err := dir.Replace(target, o.Overwrite, func(w io.Writer) error {
			_, err := io.CopyN(storage.Guard(w, top), progress.Reader(ctx, r), e.Size)
			if errors.Is(err, io.EOF) {
				return errChanged
			}
			return err
		})
		if err == nil && !e.Modified.IsZero() {
			err = dir.Chtimes(target, e.Modified, e.Modified)
		}
		if err == nil && e.Exec {
			err = executable(dir, target)
		}
		return err
	}
	if !a.zip {
		return res, a.walkTar(write)
	}
	for i, e := range a.Entries {
		if targets[i] == "" || e.Dir {
			err = write(i, nil)
		} else {
			err = writeZip(e, func(r io.Reader) error { return write(i, r) })
		}
		if err != nil {
			return res, err
		}
	}
	return res, nil
}

func writeZip(e Entry, write func(io.Reader) error) error {
	r, err := e.file.Open()
	if err != nil {
		return errDamaged
	}
	defer r.Close()
	return write(r)
}

// plan returns where each entry goes in dir, "" if nowhere, and what that extracts.
func (a *Archive) plan(dir *datadir.Dir, dest string, o Options) ([]string, Result, error) {
	var res Result
	targets := make([]string, len(a.Entries))
	direct := map[string]bool{} // folders of dir that are no links
	for i, e := range a.Entries {
		if o.Only != nil && !o.Only(e.Name) {
			continue
		}
		target, ok := datadir.Name(path.Join(filepath.ToSlash(dest), e.Name))
		if !ok {
			return nil, res, refused("The path of %s is too long.", e.Name)
		}
		if p, ok := protected(target, o.Protected); ok && o.Skip {
			if len(res.LeftOut) < maxLeftOut {
				res.LeftOut = append(res.LeftOut, e.Name)
			}
			continue
		} else if ok {
			return nil, res, refused("The archive holds %s, which holds secrets of the server or which Noryx writes itself. "+
				"Leave it out of the archive, or extract the archive into another folder.", filepath.ToSlash(p))
		}
		if err := folders(dir, filepath.Dir(target), direct); err != nil {
			return nil, res, err
		}
		if err := vacant(dir, target, e, o.Overwrite); err != nil {
			return nil, res, err
		}
		targets[i] = target
		if !e.Dir {
			res.Files++
			res.Size += e.Size
		}
	}
	return targets, res, nil
}

// executable lets the owner of a file run it, as an entry of the archive says, and gives no other
// permission, so that it is no more than the server's user could do itself.
func executable(dir *datadir.Dir, name string) error {
	info, err := dir.Lstat(name)
	if err == nil && info.Mode().IsRegular() {
		err = dir.Chmod(name, info.Mode().Perm()|0o100)
	}
	return err
}

// protected returns the path at or above name that is protected, if any.
func protected(name string, is func(string) bool) (string, bool) {
	for p := name; is != nil && p != "."; p = filepath.Dir(p) {
		if is(p) {
			return p, true
		}
	}
	return "", false
}

// folders fails if a folder of dir or one it is in is something else, such as a link, which
// could lead an entry to another place, e.g. a protected one. direct are those checked.
func folders(dir *datadir.Dir, name string, direct map[string]bool) error {
	for p := name; p != "." && !direct[p]; p = filepath.Dir(p) {
		info, err := dir.Lstat(p)
		switch {
		case errors.Is(err, fs.ErrNotExist):
		case err != nil:
			return err
		case !info.IsDir():
			return refused("%s is a file or a link on the server, where the archive has a folder.", filepath.ToSlash(p))
		}
		direct[p] = true
	}
	return nil
}

// vacant fails unless an entry can be written to target: nothing is there, a folder for a
// folder, or a file that may be replaced.
func vacant(dir *datadir.Dir, target string, e Entry, overwrite bool) error {
	info, err := dir.Lstat(target)
	switch {
	case errors.Is(err, fs.ErrNotExist):
		return nil
	case err != nil:
		return err
	case e.Dir && info.IsDir():
		return nil
	case e.Dir:
		return refused("%s is a file or a link on the server, where the archive has a folder.", filepath.ToSlash(target))
	case !info.Mode().IsRegular():
		return refused("%s is a folder or a link on the server, where the archive has a file.", filepath.ToSlash(target))
	case !overwrite:
		return status.Errorf(codes.AlreadyExists, "%s exists already. Choose to replace existing files, or another folder.", filepath.ToSlash(target))
	}
	return nil
}

// folder creates a folder and those it is in, unless made says it exists.
func folder(dir *datadir.Dir, name string, made map[string]bool) error {
	if name == "." || made[name] {
		return nil
	}
	made[name] = true
	return dir.MkdirAll(name)
}

// Guarded tells the paths of a server's data that the file manager extracts nothing into: the
// files with secrets of the server, which hidden names, and those that Noryx writes itself,
// such as server.properties, or that belong to the agent.
func Guarded(hidden secrets.Files) func(string) bool {
	return func(name string) bool { return hidden.Hidden(name) || noryxv1.NoryxFile(filepath.ToSlash(name)) }
}

// Foreign tells what a server leaves out of an archive from elsewhere, e.g. its data at a host
// or an uploaded backup: the files with secrets of the server, which hidden names, and the
// files of the agent, such as the manifest of file sets, which the agent writes again. The
// secrets in other files and the forwarding settings are replaced once they are extracted.
func Foreign(hidden secrets.Files) func(string) bool {
	return func(name string) bool {
		slash := filepath.ToSlash(name)
		return hidden.Hidden(name) || !strings.Contains(slash, "/") && strings.HasPrefix(slash, "noryx-") ||
			slices.ContainsFunc(strings.Split(slash, "/"), datadir.IsTemp)
	}
}

// Receive writes the data of the messages of an upload to w until the client ends it, up to
// MaxUpload bytes, and returns how many it wrote.
func Receive[T interface{ GetData() []byte }](recv func() (T, error), w io.Writer) (int64, error) {
	var n int64
	for {
		msg, err := recv()
		if errors.Is(err, io.EOF) {
			return n, nil
		}
		if err != nil {
			return n, err
		}
		if n += int64(len(msg.GetData())); n > MaxUpload {
			return n, tooLarge("Archives can have up to %d GB.", MaxUpload>>30)
		}
		if _, err := w.Write(msg.GetData()); err != nil {
			return n, err
		}
	}
}
