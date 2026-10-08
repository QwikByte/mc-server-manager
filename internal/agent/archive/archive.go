// Package archive extracts archives from elsewhere into the data of servers: archives
// uploaded to the file manager, uploaded backups and the data of servers imported from
// elsewhere. Unlike the backups the agent made itself, they are untrusted. So the agent reads
// the list of an archive and checks each entry before it writes anything: links, special
// files, paths that leave the folder, protected paths, too many entries, too much data and a
// too high compression ratio refuse it, or protected entries are left out.
package archive

import (
	"archive/tar"
	"archive/zip"
	"bytes"
	"compress/gzip"
	"errors"
	"io"
	"io/fs"
	"math"
	"path"
	"slices"
	"strings"
	"time"
	"unicode"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

const (
	// MaxUpload limits archives uploaded to the agent, like the files of the file manager.
	MaxUpload = 16 << 30
	// MaxEntries limits the files and folders of an archive.
	MaxEntries = 100_000
	// MaxSize limits the size of the files of an archive.
	MaxSize = 64 << 30
	// MaxRatio limits how many times larger the files of an archive are than the archive,
	// unless they have up to freeSize bytes. Worlds and plugins compress far less; zip bombs
	// far more.
	MaxRatio = 100
	freeSize = 64 << 20
	// maxDirectory limits what archive/zip reads of the directory of a ZIP archive, which it
	// keeps in memory: about 100 bytes for each entry.
	maxDirectory = 64 << 20
	// slack is what a .tar.gz archive unpacks to besides its files: a header of 512 bytes
	// and up to 511 bytes after each, and some long names.
	slack   = MaxEntries<<10 + 16<<20
	maxName = 1024
)

var (
	errFormat  = status.Error(codes.InvalidArgument, "Only ZIP and .tar.gz archives can be extracted.")
	errDamaged = status.Error(codes.InvalidArgument, "The archive is damaged.")
	errChanged = status.Error(codes.Aborted, "The archive changed while it was extracted.")
	errLimit   = errors.New("the archive is too large")
)

// Entry is a file or folder of an archive.
type Entry struct {
	// Name is its clean path with forward slashes, relative to where it is extracted.
	Name     string
	Dir      bool
	Size     int64
	Modified time.Time
	file     *zip.File
}

// Archive is an untrusted ZIP or .tar.gz archive whose entries were checked.
type Archive struct {
	Entries []Entry
	// Size is that of all its files.
	Size int64
	r    io.ReaderAt
	size int64
	zip  bool
}

// Open reads the list of the archive in r, which has size bytes, and checks its entries.
func Open(r io.ReaderAt, size int64) (*Archive, error) {
	a := &Archive{r: r, size: size}
	head := make([]byte, 4)
	if _, err := r.ReadAt(head, 0); err != nil {
		return nil, errFormat
	}
	switch {
	case bytes.HasPrefix(head, []byte("PK\x03\x04")), bytes.HasPrefix(head, []byte("PK\x05\x06")):
		return a, a.listZip()
	case bytes.HasPrefix(head, []byte{0x1f, 0x8b}):
		return a, a.walkTar(nil)
	}
	return nil, errFormat
}

func (a *Archive) listZip() error {
	a.zip = true
	limited := &limitedAt{r: a.r, left: maxDirectory}
	zr, err := zip.NewReader(limited, a.size)
	limited.left = math.MaxInt64 // the files are read through it later
	switch {
	case errors.Is(err, errLimit):
		return tooMany()
	case err != nil && !errors.Is(err, zip.ErrInsecurePath): // add checks the names
		return errDamaged
	}
	kinds := map[string]bool{}
	for _, f := range zr.File {
		if f.Method != zip.Store && f.Method != zip.Deflate || f.Flags&1 != 0 {
			return refused("%q is encrypted or compressed in a way that can't be read. Pack the archive again, e.g. as a plain ZIP archive.", f.Name)
		}
		e, err := entry(f.Name, f.Mode().Type(), f.UncompressedSize64, f.Modified)
		if err != nil {
			return err
		}
		e.file = f
		if err := a.add(kinds, e); err != nil {
			return err
		}
	}
	return nil
}

// walkTar reads a .tar.gz archive: without extract, it lists and checks its entries,
// otherwise it passes each entry listed before to extract, with its content.
func (a *Archive) walkTar(extract func(i int, r io.Reader) error) error {
	gz, err := gzip.NewReader(io.NewSectionReader(a.r, 0, a.size))
	if err != nil {
		return errDamaged
	}
	defer gz.Close()
	tr := tar.NewReader(&limited{r: gz, left: a.limit() + slack})
	kinds := map[string]bool{}
	for i := 0; ; {
		h, err := tr.Next()
		switch {
		case errors.Is(err, io.EOF) && extract != nil && i < len(a.Entries):
			return errChanged
		case errors.Is(err, io.EOF):
			return nil
		case errors.Is(err, errLimit):
			return a.tooLarge()
		case err != nil && !errors.Is(err, tar.ErrInsecurePath):
			return errDamaged
		case h.Typeflag == tar.TypeXGlobalHeader: // e.g. the commit of git archive
			continue
		}
		e, err := entry(h.Name, tarType(h.Typeflag), uint64(max(h.Size, 0)), h.ModTime) //nolint:gosec // not negative
		switch {
		case err != nil && extract != nil:
			return errChanged
		case err != nil:
			return err
		case e.Name == "":
			continue
		case extract == nil:
			if err := a.add(kinds, e); err != nil {
				return err
			}
			continue
		case i == len(a.Entries) || a.Entries[i].Name != e.Name || a.Entries[i].Dir != e.Dir || a.Entries[i].Size != e.Size:
			return errChanged
		}
		if err := extract(i, tr); errors.Is(err, errLimit) {
			return a.tooLarge()
		} else if err != nil {
			return err
		}
		i++
	}
}

func tarType(flag byte) fs.FileMode {
	switch flag {
	case tar.TypeReg:
		return 0
	case tar.TypeDir:
		return fs.ModeDir
	}
	return fs.ModeIrregular // links, devices and the like
}

// entry checks the name and type of an entry of an archive. The folder itself has no name.
func entry(raw string, typ fs.FileMode, size uint64, modified time.Time) (Entry, error) {
	name, ok := clean(raw)
	switch {
	case !ok:
		return Entry{}, refused("The archive holds %q, a path that leaves the folder or isn't valid.", raw)
	case typ != 0 && typ != fs.ModeDir:
		return Entry{}, refused("The archive holds %s, a link or another special file, which can't be extracted.", name)
	case size > MaxSize:
		return Entry{}, tooLarge("The files of an archive can have up to %d GB.", MaxSize>>30)
	}
	e := Entry{Name: name, Dir: typ == fs.ModeDir, Modified: modified}
	if !e.Dir {
		e.Size = int64(size)
	}
	return e, nil
}

// clean returns the clean path of a name in an archive with forward slashes, "" for the
// folder itself, or false if it leaves the folder or isn't valid. Some archives of Windows
// separate folders with backslashes.
func clean(raw string) (string, bool) {
	name := strings.ReplaceAll(raw, `\`, "/")
	if name == "" || len(name) > maxName || strings.HasPrefix(name, "/") || strings.ContainsFunc(name, unicode.IsControl) ||
		slices.Contains(strings.Split(name, "/"), "..") {
		return "", false
	}
	if name = path.Clean(name); name == "." {
		return "", true
	}
	return name, true
}

// add adds an entry to the list. It checks the limits, and that no path is there twice or
// both as a file and a folder: kinds are the paths listed so far, true for folders.
func (a *Archive) add(kinds map[string]bool, e Entry) error {
	if e.Name == "" {
		return nil
	}
	if len(a.Entries) == MaxEntries {
		return tooMany()
	}
	if a.Size += e.Size; a.Size > a.limit() {
		return a.tooLarge()
	}
	for p := path.Dir(e.Name); p != "."; p = path.Dir(p) {
		dir, seen := kinds[p]
		if seen && !dir {
			return refused("The archive holds %s both as a file and as a folder.", p)
		}
		if seen {
			break // and the folders it is in
		}
		kinds[p] = true
	}
	if dir, seen := kinds[e.Name]; seen && (!dir || !e.Dir) {
		return refused("The archive holds %s twice, or both as a file and as a folder.", e.Name)
	}
	kinds[e.Name] = e.Dir
	a.Entries = append(a.Entries, e)
	return nil
}

// limit is the size of the files of the archive at most.
func (a *Archive) limit() int64 { return min(MaxSize, max(MaxRatio*a.size, freeSize)) }

func (a *Archive) tooLarge() error {
	if a.limit() == MaxSize {
		return tooLarge("The files of an archive can have up to %d GB.", MaxSize>>30)
	}
	return refused("The archive unpacks to more than %d times its size, like a zip bomb.", MaxRatio)
}

func tooMany() error {
	return tooLarge("An archive can hold up to %d files and folders.", MaxEntries)
}

func refused(format string, args ...any) error {
	return status.Errorf(codes.InvalidArgument, format, args...)
}

func tooLarge(format string, args ...any) error {
	return status.Errorf(codes.ResourceExhausted, format, args...)
}

// limitedAt reads from r while left lasts.
type limitedAt struct {
	r    io.ReaderAt
	left int64
}

func (l *limitedAt) ReadAt(p []byte, off int64) (int, error) {
	if l.left < int64(len(p)) {
		return 0, errLimit
	}
	l.left -= int64(len(p))
	return l.r.ReadAt(p, off)
}

// limited reads from r while left lasts.
type limited struct {
	r    io.Reader
	left int64
}

func (l *limited) Read(p []byte) (int, error) {
	if l.left <= 0 {
		return 0, errLimit
	}
	n, err := l.r.Read(p[:min(int64(len(p)), l.left)])
	l.left -= int64(n)
	return n, err
}
