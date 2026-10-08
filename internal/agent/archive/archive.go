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
	"encoding/binary"
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
	// MaxRatio limits how many times larger the files of an archive are than the archive,
	// unless they have up to freeSize bytes. Worlds and plugins compress far less; zip bombs
	// far more.
	MaxRatio = 100
	freeSize = 64 << 20
	maxName  = 1024
)

// Limits are what an archive may hold at most.
type Limits struct {
	// Entries are its files and folders, Size the size of its files.
	Entries int
	Size    int64
	// Directory is what archive/zip reads of the directory of a ZIP archive, which it keeps in
	// memory: about 100 bytes for each entry.
	Directory int64
}

var (
	// Uploads limits the archives that users upload, e.g. to the file manager.
	Uploads = Limits{Entries: 100_000, Size: 64 << 30, Directory: 64 << 20}
	// Backups limits backups from elsewhere when they are restored, e.g. those that other
	// agents made of servers that moved here, which can be larger. Uploaded backups were
	// checked against Uploads before.
	Backups = Limits{Entries: 1_000_000, Size: 1 << 40, Directory: 256 << 20}
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
	// Exec tells that its owner may run the file.
	Exec bool
	file *zip.File
}

// Archive is an untrusted ZIP or .tar.gz archive whose entries were checked.
type Archive struct {
	Entries []Entry
	// Size is that of all its files.
	Size   int64
	r      io.ReaderAt
	size   int64
	limits Limits
	zip    bool
	// plain tells that it is a ZIP archive whose entries have clean names, see Plain.
	plain bool
}

// Plain reports whether the archive is a ZIP archive whose entries have clean names, as in the
// backups of the agent, e.g. one downloaded from Noryx, rather than ./world or world\level.dat.
func (a *Archive) Plain() bool { return a.plain }

// Open reads the list of the archive in r, which has size bytes, and checks its entries
// against limits.
func Open(r io.ReaderAt, size int64, limits Limits) (*Archive, error) {
	a := &Archive{r: r, size: size, limits: limits}
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
	a.zip, a.plain = true, true
	zr, err := OpenZip(a.r, a.size, a.limits)
	if err != nil {
		return err
	}
	kinds := map[string]bool{}
	for _, f := range zr.File {
		if f.Method != zip.Store && f.Method != zip.Deflate || f.Flags&1 != 0 {
			return refused("%q is encrypted or compressed in a way that can't be read. Pack the archive again, e.g. as a plain ZIP archive.", f.Name)
		}
		e, err := a.entry(f.Name, f.Mode().Type(), f.UncompressedSize64, f.Modified)
		if err != nil {
			return err
		}
		e.file, e.Exec = f, f.Mode()&0o100 != 0
		a.plain = a.plain && e.Name != "" && strings.TrimSuffix(f.Name, "/") == e.Name
		if err := a.add(kinds, e); err != nil {
			return err
		}
	}
	return nil
}

// OpenZip opens the ZIP archive in r, which has size bytes, reading at most the directory that
// limits allow, and refuses one whose end announces more entries or a larger directory than
// they allow before archive/zip reserves memory for its entries. It doesn't check the entries,
// as Open does; their names may be unsafe.
func OpenZip(r io.ReaderAt, size int64, limits Limits) (*zip.Reader, error) {
	if err := checkEnd(r, size, limits); err != nil {
		return nil, err
	}
	limited := &limitedAt{r: r, left: limits.Directory}
	zr, err := zip.NewReader(limited, size)
	limited.left = math.MaxInt64 // the files are read through it later
	switch {
	case errors.Is(err, errLimit):
		return nil, tooMany(limits)
	case err != nil && !errors.Is(err, zip.ErrInsecurePath):
		return nil, errDamaged
	}
	return zr, nil
}

// checkEnd refuses a ZIP64 archive whose end announces more entries or a larger directory than
// limits allow: archive/zip reserves memory for as many entries as the end of the directory
// announces, up to one per 30 bytes of the archive, before it reads any, and only the end of
// a ZIP64 archive can announce more than 65,535. Like archive/zip, it looks for the end in the
// last 65 KiB, where it checks each one with the ZIP64 end that a locator right before it
// names, so also the one that archive/zip takes.
func checkEnd(r io.ReaderAt, size int64, limits Limits) error {
	tail, loc, end64 := make([]byte, min(size, 65<<10)), make([]byte, 20), make([]byte, 56)
	start := size - int64(len(tail))
	if !read(r, tail, start) {
		return errDamaged
	}
	le := binary.LittleEndian
	for i := len(tail) - 22; i >= 0; i-- {
		at := start + int64(i) - int64(len(loc))
		if string(tail[i:i+4]) != "PK\x05\x06" || at < 0 || !read(r, loc, at) || string(loc[:4]) != "PK\x06\x07" ||
			!read(r, end64, int64(le.Uint64(loc[8:]))) || string(end64[:4]) != "PK\x06\x06" { //nolint:gosec // ReadAt refuses negative offsets
			continue
		}
		if le.Uint64(end64[32:]) > uint64(limits.Entries) || le.Uint64(end64[40:]) > uint64(limits.Directory) { //nolint:gosec // positive limits
			return tooMany(limits)
		}
	}
	return nil
}

// read reads len(p) bytes at off and tells whether it could.
func read(r io.ReaderAt, p []byte, off int64) bool {
	n, err := r.ReadAt(p, off)
	return n == len(p) && (err == nil || errors.Is(err, io.EOF))
}

// walkTar reads a .tar.gz archive: without extract, it lists and checks its entries,
// otherwise it passes each entry listed before to extract, with its content.
func (a *Archive) walkTar(extract func(i int, r io.Reader) error) error {
	gz, err := gzip.NewReader(io.NewSectionReader(a.r, 0, a.size))
	if err != nil {
		return errDamaged
	}
	defer gz.Close()
	// Besides its files, it unpacks to a header of 512 bytes and up to 511 bytes after each,
	// and some long names.
	tr := tar.NewReader(&limited{r: gz, left: a.limit() + int64(a.limits.Entries)<<10 + 16<<20})
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
		e, err := a.entry(h.Name, tarType(h.Typeflag), uint64(max(h.Size, 0)), h.ModTime) //nolint:gosec // not negative
		e.Exec = h.Mode&0o100 != 0
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
func (a *Archive) entry(raw string, typ fs.FileMode, size uint64, modified time.Time) (Entry, error) {
	name, ok := clean(raw)
	switch {
	case !ok:
		return Entry{}, refused("The archive holds %q, a path that leaves the folder or isn't valid.", raw)
	case typ != 0 && typ != fs.ModeDir:
		return Entry{}, refused("The archive holds %s, a link or another special file, which can't be extracted.", name)
	case size > uint64(a.limits.Size): //nolint:gosec // a positive limit
		return Entry{}, tooLarge("The files of an archive can have up to %d GB.", a.limits.Size>>30)
	}
	e := Entry{Name: name, Dir: typ == fs.ModeDir, Modified: modified}
	if !e.Dir {
		e.Size = int64(size) //nolint:gosec // at most the limit, see above
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
	if len(a.Entries) == a.limits.Entries {
		return a.tooMany()
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
func (a *Archive) limit() int64 { return min(a.limits.Size, max(MaxRatio*a.size, freeSize)) }

func (a *Archive) tooLarge() error {
	if a.limit() == a.limits.Size {
		return tooLarge("The files of an archive can have up to %d GB.", a.limits.Size>>30)
	}
	return refused("The archive unpacks to more than %d times its size, like a zip bomb.", MaxRatio)
}

func (a *Archive) tooMany() error { return tooMany(a.limits) }

func tooMany(limits Limits) error {
	return tooLarge("An archive can hold up to %d files and folders.", limits.Entries)
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
