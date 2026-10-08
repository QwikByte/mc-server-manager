package archive

import (
	"archive/tar"
	"archive/zip"
	"bytes"
	"compress/gzip"
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"maps"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"github.com/QwikByte/noryx/internal/agent/datadir"
	"github.com/QwikByte/noryx/internal/agent/secrets"
)

// file is an entry of an archive made by a test.
type file struct {
	name, content string
	mode          fs.FileMode
}

func zipOf(t *testing.T, files ...file) []byte {
	t.Helper()
	var buf bytes.Buffer
	zw := zip.NewWriter(&buf)
	for _, f := range files {
		h := &zip.FileHeader{Name: f.name, Method: zip.Deflate, Modified: time.Date(2024, 5, 1, 12, 0, 0, 0, time.UTC)}
		h.SetMode(f.mode | 0o644)
		w, err := zw.CreateHeader(h)
		check(t, err)
		_, err = w.Write([]byte(f.content))
		check(t, err)
	}
	check(t, zw.Close())
	return buf.Bytes()
}

func tarGzOf(t *testing.T, headers ...*tar.Header) []byte {
	t.Helper()
	var buf bytes.Buffer
	gz := gzip.NewWriter(&buf)
	tw := tar.NewWriter(gz)
	for _, h := range headers {
		content := h.Linkname
		if h.Typeflag != tar.TypeReg {
			content = ""
		} else {
			h.Size, h.Linkname = int64(len(content)), ""
		}
		if h.Typeflag != tar.TypeXGlobalHeader {
			h.Mode |= 0o644
		}
		check(t, tw.WriteHeader(h))
		_, err := tw.Write([]byte(content))
		check(t, err)
	}
	check(t, tw.Close())
	check(t, gz.Close())
	return buf.Bytes()
}

// reg is a regular file of a tar archive; its content travels as Linkname until it is written.
func reg(name, content string) *tar.Header {
	return &tar.Header{Name: name, Typeflag: tar.TypeReg, Linkname: content}
}

func check(t *testing.T, err error) {
	t.Helper()
	if err != nil {
		t.Fatal(err)
	}
}

// data is the data directory of a server in a test.
func data(t *testing.T) (*datadir.Dir, string) {
	t.Helper()
	path := t.TempDir()
	dir, err := datadir.Open(path)
	check(t, err)
	t.Cleanup(func() { dir.Close() })
	return dir, path
}

func extract(t *testing.T, dir *datadir.Dir, archive []byte, dest string, o Options) (Result, error) {
	a, err := Open(bytes.NewReader(archive), int64(len(archive)), Uploads)
	if err != nil {
		return Result{}, err
	}
	return a.Extract(t.Context(), dir, dest, o)
}

// written lists the files and folders below path, with the content of files.
func written(t *testing.T, path string) map[string]string {
	t.Helper()
	files := map[string]string{}
	check(t, filepath.WalkDir(path, func(p string, d fs.DirEntry, err error) error {
		if err != nil || p == path {
			return err
		}
		rel, _ := filepath.Rel(path, p)
		if d.IsDir() {
			files[rel+"/"] = ""
			return nil
		}
		content, err := os.ReadFile(p)
		files[rel] = string(content)
		return err
	}))
	return files
}

func wantCode(t *testing.T, err error, code codes.Code, within string) {
	t.Helper()
	if status.Code(err) != code || !strings.Contains(status.Convert(err).Message(), within) {
		t.Fatalf("error = %v, want %s with %q", err, code, within)
	}
}

// A world packed on Windows or Linux, as ZIP or .tar.gz, ends up in the chosen folder with its
// times, and existing files are only replaced on request.
func TestExtract(t *testing.T) {
	dir, path := data(t)
	world := zipOf(t, file{name: "world/", mode: fs.ModeDir}, file{name: "world/level.dat", content: "level"},
		file{name: `world\region\r.0.0.mca`, content: "chunks"}, file{name: "./world/data/raids.dat", content: "raids"})
	res, err := extract(t, dir, world, "imported", Options{Protected: Guarded(secrets.With())})
	check(t, err)
	want := map[string]string{
		"imported/": "", "imported/world/": "", "imported/world/level.dat": "level", "imported/world/region/": "",
		"imported/world/region/r.0.0.mca": "chunks", "imported/world/data/": "", "imported/world/data/raids.dat": "raids",
	}
	if got := written(t, path); !maps.Equal(got, want) || res.Files != 3 || res.Size != int64(len("levelchunksraids")) {
		t.Fatalf("extracted %v, %+v", got, res)
	}
	if info, _ := os.Stat(filepath.Join(path, "imported/world/level.dat")); !info.ModTime().Equal(time.Date(2024, 5, 1, 12, 0, 0, 0, time.UTC)) {
		t.Fatalf("modified %v", info.ModTime())
	}

	_, err = extract(t, dir, world, "imported", Options{})
	wantCode(t, err, codes.AlreadyExists, "imported/world/level.dat exists already")
	newer := zipOf(t, file{name: "world/level.dat", content: "newer"})
	_, err = extract(t, dir, newer, "imported", Options{Overwrite: true})
	check(t, err)

	pack := tarGzOf(t, &tar.Header{Typeflag: tar.TypeXGlobalHeader, PAXRecords: map[string]string{"comment": "abc"}},
		&tar.Header{Name: "./", Typeflag: tar.TypeDir}, reg("./config/mod.toml", "a = 1"), reg("./mods/a.jar", "jar"))
	_, err = extract(t, dir, pack, "", Options{})
	check(t, err)
	want["imported/world/level.dat"], want["config/"], want["config/mod.toml"], want["mods/"], want["mods/a.jar"] = "newer", "", "a = 1", "", "jar"
	if got := written(t, path); !maps.Equal(got, want) {
		t.Fatalf("extracted %v", got)
	}
}

// Archives whose entries leave the folder, links, special files and entries that clash are
// refused before anything is written.
func TestMaliciousArchives(t *testing.T) {
	ok := file{name: "a.txt", content: "a"}
	for name, archive := range map[string][]byte{
		"zip slip":                 zipOf(t, ok, file{name: "../evil.sh", content: "x"}),
		"zip slip inside":          zipOf(t, ok, file{name: "world/../../evil.sh", content: "x"}),
		"zip slip with backslash":  zipOf(t, ok, file{name: `..\evil.sh`, content: "x"}),
		"absolute":                 zipOf(t, ok, file{name: "/etc/cron.d/evil", content: "x"}),
		"control character":        zipOf(t, ok, file{name: "a\nb", content: "x"}),
		"symbolic link in a ZIP":   zipOf(t, ok, file{name: "link", content: "/etc/passwd", mode: fs.ModeSymlink}),
		"device in a ZIP":          zipOf(t, ok, file{name: "dev", mode: fs.ModeDevice}),
		"named pipe in a ZIP":      zipOf(t, ok, file{name: "pipe", mode: fs.ModeNamedPipe}),
		"twice":                    zipOf(t, ok, file{name: "a.txt", content: "b"}),
		"file and folder":          zipOf(t, ok, file{name: "a.txt/b", content: "b"}),
		"tar slip":                 tarGzOf(t, reg("a.txt", "a"), reg("../../evil.sh", "x")),
		"symbolic link in a tar":   tarGzOf(t, reg("a.txt", "a"), &tar.Header{Name: "link", Typeflag: tar.TypeSymlink, Linkname: "/etc"}),
		"link then file in a tar":  tarGzOf(t, &tar.Header{Name: "plugins", Typeflag: tar.TypeSymlink, Linkname: "/"}, reg("plugins/x", "x")),
		"hard link in a tar":       tarGzOf(t, reg("a.txt", "a"), &tar.Header{Name: "hard", Typeflag: tar.TypeLink, Linkname: "a.txt"}),
		"character device in tar":  tarGzOf(t, reg("a.txt", "a"), &tar.Header{Name: "null", Typeflag: tar.TypeChar, Devmajor: 1, Devminor: 3}),
		"plain text, no archive":   []byte("not an archive at all"),
		"damaged gzip":             {0x1f, 0x8b, 8, 0, 0, 0, 0, 0},
		"damaged ZIP":              []byte("PK\x03\x04 and nothing else"),
		"encrypted entry of a ZIP": encrypted(t),
	} {
		t.Run(name, func(t *testing.T) {
			dir, path := data(t)
			_, err := extract(t, dir, archive, "", Options{})
			if status.Code(err) != codes.InvalidArgument {
				t.Fatalf("error = %v", err)
			}
			if got := written(t, path); len(got) != 0 {
				t.Fatalf("wrote %v", got)
			}
		})
	}
}

func encrypted(t *testing.T) []byte {
	var buf bytes.Buffer
	zw := zip.NewWriter(&buf)
	w, err := zw.CreateRaw(&zip.FileHeader{Name: "secret.txt", Method: zip.Deflate, Flags: 1, CompressedSize64: 4, UncompressedSize64: 4})
	check(t, err)
	_, err = w.Write([]byte("abcd"))
	check(t, err)
	check(t, zw.Close())
	return buf.Bytes()
}

// Archives that would unpack to far more than their size, or to more than the limit, are
// refused by what their lists say, before anything is unpacked.
func TestZipBombs(t *testing.T) {
	declared := func(size uint64) []byte {
		var buf bytes.Buffer
		zw := zip.NewWriter(&buf)
		w, err := zw.CreateRaw(&zip.FileHeader{Name: "zeros", Method: zip.Deflate, CompressedSize64: 4, UncompressedSize64: size})
		check(t, err)
		_, err = w.Write([]byte{1, 2, 3, 4})
		check(t, err)
		check(t, zw.Close())
		return buf.Bytes()
	}
	// A tar header that announces 1 GB of zeros; what follows is never read.
	var header bytes.Buffer
	gz := gzip.NewWriter(&header)
	tw := tar.NewWriter(gz)
	check(t, tw.WriteHeader(&tar.Header{Name: "zeros", Typeflag: tar.TypeReg, Size: 1 << 30, Mode: 0o644}))
	check(t, gz.Close())

	for name, test := range map[string]struct {
		archive []byte
		code    codes.Code
		within  string
	}{
		"ratio of a ZIP":        {declared(freeSize + 1), codes.InvalidArgument, "zip bomb"},
		"ratio of a .tar.gz":    {header.Bytes(), codes.InvalidArgument, "zip bomb"},
		"size of a ZIP":         {declared(uint64(Uploads.Size) + 1), codes.ResourceExhausted, "64 GB"},
		"many entries of zeros": {zeros(t), codes.InvalidArgument, "zip bomb"},
	} {
		t.Run(name, func(t *testing.T) {
			dir, path := data(t)
			_, err := extract(t, dir, test.archive, "", Options{})
			wantCode(t, err, test.code, test.within)
			if got := written(t, path); len(got) != 0 {
				t.Fatalf("wrote %v", got)
			}
		})
	}

	// A real one: 65 MB of zeros, which deflate packs into 64 KB.
	var bomb bytes.Buffer
	zw := zip.NewWriter(&bomb)
	w, err := zw.Create("zeros")
	check(t, err)
	_, err = w.Write(make([]byte, freeSize+1<<20))
	check(t, err)
	check(t, zw.Close())
	dir, _ := data(t)
	_, err = extract(t, dir, bomb.Bytes(), "", Options{})
	wantCode(t, err, codes.InvalidArgument, "zip bomb")
}

// zeros is a ZIP archive of 100 entries of 1 MB of zeros each, which together unpack to far
// more than the archive's size.
func zeros(t *testing.T) []byte {
	var buf bytes.Buffer
	zw := zip.NewWriter(&buf)
	w, err := zw.Create("first")
	check(t, err)
	_, err = w.Write(make([]byte, 1<<20))
	check(t, err)
	check(t, zw.Close())
	zr, err := zip.NewReader(bytes.NewReader(buf.Bytes()), int64(buf.Len()))
	check(t, err)
	first := zr.File[0]
	var out bytes.Buffer
	zw = zip.NewWriter(&out)
	for i := range 100 {
		h := first.FileHeader
		h.Name = "copy" + strings.Repeat("x", i)
		raw, err := first.OpenRaw()
		check(t, err)
		w, err := zw.CreateRaw(&h)
		check(t, err)
		data := new(bytes.Buffer)
		_, err = data.ReadFrom(raw)
		check(t, err)
		_, err = w.Write(data.Bytes())
		check(t, err)
	}
	check(t, zw.Close())
	return out.Bytes()
}

// The file manager extracts nothing into the files with secrets of the server, those that
// Noryx writes itself and the agent's own, nor through a link; an import leaves them out.
func TestProtectedPaths(t *testing.T) {
	hidden := secrets.With("noryx-filesets.json", "plugins/Sync/token.yml")
	for _, name := range []string{
		".rcon-cli.env", "forwarding.secret", "plugins/floodgate/key.pem", "noryx-filesets.json", "noryx-pending-players.json",
		"plugins/Sync/token.yml", "server.properties", "config/paper-global.yml", "world/.noryx-abc/x", "forwarding.secret/inside",
	} {
		t.Run(name, func(t *testing.T) {
			dir, path := data(t)
			archive := zipOf(t, file{name: "world/level.dat", content: "level"}, file{name: name, content: "mine"})
			_, err := extract(t, dir, archive, "", Options{Protected: Guarded(hidden)})
			wantCode(t, err, codes.InvalidArgument, "holds secrets of the server or which Noryx writes itself")
			if got := written(t, path); len(got) != 0 {
				t.Fatalf("wrote %v", got)
			}
		})
	}

	// Into another folder, such files are files like any other.
	dir, path := data(t)
	_, err := extract(t, dir, zipOf(t, file{name: "server.properties", content: "motd=a"}), "old", Options{Protected: Guarded(hidden)})
	check(t, err)

	// An archive from elsewhere loses them, but keeps its settings.
	archive := zipOf(t, file{name: "server.properties", content: "motd=a"}, file{name: ".rcon-cli.env", content: "password=theirs"},
		file{name: "noryx-filesets.json", content: "{}"}, file{name: "plugins/Sync/token.yml", content: "token"},
		file{name: "plugins/floodgate/key.pem", content: "key"}, file{name: "eula.txt", content: "eula=true"})
	res, err := extract(t, dir, archive, "", Options{Protected: Foreign(hidden), Skip: true})
	check(t, err)
	want := map[string]string{"old/": "", "old/server.properties": "motd=a", "server.properties": "motd=a", "eula.txt": "eula=true"}
	if got := written(t, path); !maps.Equal(got, want) || len(res.LeftOut) != 4 {
		t.Fatalf("extracted %v, left out %v", got, res.LeftOut)
	}

	// A link of the server can't lead an entry elsewhere, e.g. to the manifest of file sets.
	check(t, os.Symlink(".", filepath.Join(path, "data")))
	_, err = extract(t, dir, zipOf(t, file{name: "data/noryx-filesets.json", content: "{}"}), "", Options{Protected: Guarded(hidden)})
	wantCode(t, err, codes.InvalidArgument, "data is a file or a link")
	_, err = extract(t, dir, zipOf(t, file{name: "x", content: "x"}), "data", Options{Protected: Guarded(hidden)})
	wantCode(t, err, codes.InvalidArgument, "data is a file or a link")
	if _, err := os.Lstat(filepath.Join(path, "x")); !errors.Is(err, fs.ErrNotExist) {
		t.Fatal("wrote through the link")
	}
}

// An archive with too many entries is refused.
func TestTooManyEntries(t *testing.T) {
	if testing.Short() {
		t.Skip("writes an archive with 100,001 entries")
	}
	var buf bytes.Buffer
	zw := zip.NewWriter(&buf)
	for i := range Uploads.Entries + 1 {
		_, err := zw.CreateHeader(&zip.FileHeader{Name: fmt.Sprintf("d%d/f%d", i%100, i), Method: zip.Store})
		check(t, err)
	}
	check(t, zw.Close())
	_, err := Open(bytes.NewReader(buf.Bytes()), int64(buf.Len()), Uploads)
	wantCode(t, err, codes.ResourceExhausted, "100000 files and folders")
	// Backups of large servers from other nodes may hold more.
	a, err := Open(bytes.NewReader(buf.Bytes()), int64(buf.Len()), Backups)
	if err != nil || len(a.Entries) != Uploads.Entries+1 {
		t.Fatalf("backup: %v", err)
	}
}

// sparse reads as size bytes of zeros that end with tail, like a sparse file.
type sparse struct {
	size int64
	tail []byte
}

func (s sparse) ReadAt(p []byte, off int64) (int, error) {
	if off < 0 {
		return 0, errors.New("negative offset")
	}
	n := int(max(min(int64(len(p)), s.size-off), 0))
	clear(p[:n])
	start := s.size - int64(len(s.tail))
	for i := max(off, start); i < off+int64(n); i++ {
		p[i-off] = s.tail[i-start]
	}
	if n < len(p) {
		return n, io.EOF
	}
	return n, nil
}

// A ZIP64 archive whose end announces more entries than the limits allow is refused before
// archive/zip reserves memory for them: for one per 30 bytes, 270 MiB for 1 GiB of zeros.
func TestHugeDirectoryEnd(t *testing.T) {
	const size = 1 << 30
	le := binary.LittleEndian
	end := le.AppendUint32(nil, 0x06064b50)
	end = le.AppendUint64(end, 44)
	end = append(end, 45, 0, 45, 0, 0, 0, 0, 0, 0, 0, 0, 0)
	end = le.AppendUint64(end, size/31) // entries on this disk
	end = le.AppendUint64(end, size/31) // and in all
	end = le.AppendUint64(end, 46)      // the size of the directory
	end = le.AppendUint64(end, size-56-20-22-46)
	end = le.AppendUint32(end, 0x07064b50) // the locator of the ZIP64 end
	end = le.AppendUint32(end, 0)
	end = le.AppendUint64(end, size-56-20-22)
	end = le.AppendUint32(end, 1)
	end = le.AppendUint32(end, 0x06054b50) // the end, which leaves the numbers to the ZIP64 end
	end = append(end, 0, 0, 0, 0, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0, 0)
	archive := sparse{size, end}

	var before, after runtime.MemStats
	runtime.ReadMemStats(&before)
	_, err := OpenZip(archive, size, Backups)
	runtime.ReadMemStats(&after)
	wantCode(t, err, codes.ResourceExhausted, "1000000 files and folders")
	if allocated := after.TotalAlloc - before.TotalAlloc; allocated > 1<<20 {
		t.Fatalf("allocated %d bytes", allocated)
	}
}
