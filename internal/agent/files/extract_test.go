package files

import (
	"archive/zip"
	"bytes"
	"os"
	"path/filepath"
	"slices"
	"testing"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	noryxv1 "github.com/QwikByte/noryx/api/noryx/v1"
	"github.com/QwikByte/noryx/internal/agent/runtime"
)

// files writes files into the data of a server.
func files(t *testing.T, data string, files map[string]string) {
	t.Helper()
	for name, content := range files {
		if err := os.MkdirAll(filepath.Dir(filepath.Join(data, name)), 0o750); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(data, name), []byte(content), 0o600); err != nil {
			t.Fatal(err)
		}
	}
}

func zipped(t *testing.T, files map[string]string) string {
	t.Helper()
	var buf bytes.Buffer
	zw := zip.NewWriter(&buf)
	for name, content := range files {
		w, err := zw.Create(name)
		if err == nil {
			_, err = w.Write([]byte(content))
		}
		if err != nil {
			t.Fatal(err)
		}
	}
	if err := zw.Close(); err != nil {
		t.Fatal(err)
	}
	return buf.String()
}

// An uploaded world extracts into a folder; archives that would write files with secrets or
// those that Noryx writes itself, or that leave the folder, write nothing.
func TestExtractArchive(t *testing.T) {
	rt := dataRuntime{dir: t.TempDir()}
	svc, id := NewService(rt), runtime.NewID()
	data := filepath.Join(rt.dir, id)
	files(t, data, map[string]string{
		"uploads/world.zip":   zipped(t, map[string]string{"world/level.dat": "level", "world/region/r.0.0.mca": "chunks"}),
		"uploads/evil.zip":    zipped(t, map[string]string{"plugins/x.yml": "x", ".rcon-cli.env": "password=mine"}),
		"uploads/slip.zip":    zipped(t, map[string]string{"../../escaped": "x"}),
		"uploads/notes.txt":   "not an archive",
		"noryx-filesets.json": `{"marked":["secret.tar.gz"]}`,
		"secret.tar.gz":       zipped(t, map[string]string{"a": "a"}),
	})
	extract := func(path, dest string, overwrite bool) (*noryxv1.ExtractArchiveResponse, error) {
		return svc.ExtractArchive(t.Context(), &noryxv1.ExtractArchiveRequest{ServerId: id, Path: path, Destination: dest, Overwrite: overwrite})
	}
	res, err := extract("uploads/world.zip", "", false)
	if err != nil || res.GetFiles() != 2 || res.GetSize() != int64(len("levelchunks")) {
		t.Fatalf("extract: %v, %v", res, err)
	}
	if got, _ := os.ReadFile(filepath.Join(data, "world/region/r.0.0.mca")); string(got) != "chunks" {
		t.Fatalf("r.0.0.mca = %q", got)
	}
	if _, err := extract("uploads/world.zip", "", false); status.Code(err) != codes.AlreadyExists {
		t.Fatalf("extract again: %v", err)
	}
	if _, err := extract("uploads/world.zip", "", true); err != nil {
		t.Fatalf("extract again, replacing: %v", err)
	}

	for path, code := range map[string]codes.Code{
		"uploads/evil.zip": codes.InvalidArgument, "uploads/slip.zip": codes.InvalidArgument, "uploads/notes.txt": codes.InvalidArgument,
		"uploads": codes.InvalidArgument, "secret.tar.gz": codes.PermissionDenied, "missing.zip": codes.NotFound, "../x.zip": codes.InvalidArgument,
	} {
		if _, err := extract(path, "", false); status.Code(err) != code {
			t.Errorf("extract %s: %v, want %v", path, err, code)
		}
	}
	for _, name := range []string{"plugins", ".rcon-cli.env", filepath.Join("..", "escaped")} {
		if _, err := os.Lstat(filepath.Join(data, name)); err == nil {
			t.Errorf("%s was written", name)
		}
	}

	// Not through a link of the server either.
	if err := os.Symlink("..", filepath.Join(data, "up")); err != nil {
		t.Fatal(err)
	}
	if _, err := extract("uploads/world.zip", "up", false); status.Code(err) != codes.InvalidArgument {
		t.Fatalf("extract through a link: %v", err)
	}
}

// Copies follow the rules of moving: nothing with secrets of the server is copied, and no
// copy goes where they would show.
func TestCopyFile(t *testing.T) {
	rt := dataRuntime{dir: t.TempDir()}
	svc, id := NewService(rt), runtime.NewID()
	data := filepath.Join(rt.dir, id)
	files(t, data, map[string]string{
		"world/level.dat": "level", "world/region/r.0.0.mca": "chunks", "server.properties": "rcon.password=secret\n",
		"config/paper-global.yml": "secret: s\n", "plugins/Sync/token.yml": "token: t\n", "noryx-filesets.json": `{"marked":["plugins/Sync/token.yml"]}`,
		"keys/key.pem": "mine",
	})
	copyFile := func(from, to string) error {
		_, err := svc.CopyFile(t.Context(), &noryxv1.CopyFileRequest{ServerId: id, From: from, To: to})
		return err
	}
	if err := copyFile("world", "world_copy"); err != nil {
		t.Fatal(err)
	}
	if got, _ := os.ReadFile(filepath.Join(data, "world_copy/region/r.0.0.mca")); string(got) != "chunks" {
		t.Fatalf("copy = %q", got)
	}
	if err := copyFile("world/level.dat", "level.dat"); err != nil {
		t.Fatal(err)
	}
	for c, code := range map[[2]string]codes.Code{
		{"world", "world_copy"}:                             codes.AlreadyExists,
		{"world", "world/inside"}:                           codes.InvalidArgument,
		{"", "all"}:                                         codes.InvalidArgument,
		{"server.properties", "props.txt"}:                  codes.PermissionDenied,
		{"config", "config2"}:                               codes.PermissionDenied,
		{"plugins", "plugins2"}:                             codes.PermissionDenied,
		{"noryx-filesets.json", "sets.json"}:                codes.PermissionDenied,
		{"keys", "plugins/floodgate"}:                       codes.PermissionDenied,
		{"keys/key.pem", ".rcon-cli.env"}:                   codes.PermissionDenied,
		{"missing", "x"}:                                    codes.NotFound,
		{"world", "../x"}:                                   codes.InvalidArgument,
		{"world/level.dat", "world_copy/missing/level.dat"}: codes.NotFound,
	} {
		if err := copyFile(c[0], c[1]); status.Code(err) != code {
			t.Errorf("copy %s to %s: %v, want %v", c[0], c[1], err, code)
		}
	}
	entries, _ := os.ReadDir(data)
	var names []string
	for _, e := range entries {
		names = append(names, e.Name())
	}
	if want := []string{"config", "keys", "level.dat", "noryx-filesets.json", "plugins", "server.properties", "world", "world_copy"}; !slices.Equal(names, want) {
		t.Fatalf("server folder = %v", names)
	}
}

// A search finds text in the files of a folder as the panel shows them, without the files that
// only hold secrets and those of file sets with secrets, and with secrets hidden.
func TestSearchFiles(t *testing.T) {
	rt := dataRuntime{dir: t.TempDir()}
	svc, id := NewService(rt), runtime.NewID()
	data := filepath.Join(rt.dir, id)
	files(t, data, map[string]string{
		"server.properties":      "motd=Welcome\nrcon.password=welcome-secret\n",
		".rcon-cli.env":          "password=welcome-secret\n",
		"plugins/Sync/token.yml": "token: welcome-token\n",
		"noryx-filesets.json":    `{"marked":["plugins/Sync/token.yml"],"welcome":1}`,
		"plugins/Essentials.yml": "first-join: WELCOME new players\nother: x\n" + string(make([]byte, 0)),
		"plugins/data.bin":       "welcome\x00binary",
		"world/region/r.0.0.mca": "\x00welcome",
		"long.txt":               string(bytes.Repeat([]byte("ab"), 400)) + "welcome" + string(bytes.Repeat([]byte("cd"), 400)),
	})
	search := func(path, query string) (*noryxv1.SearchFilesResponse, error) {
		return svc.SearchFiles(t.Context(), &noryxv1.SearchFilesRequest{ServerId: id, Path: path, Query: query})
	}
	res, err := search("", "welcome")
	if err != nil {
		t.Fatal(err)
	}
	got := map[string]string{}
	for _, m := range res.GetMatches() {
		got[m.GetPath()] = m.GetText()
	}
	if len(got) != 3 || got["server.properties"] != "motd=Welcome" || got["plugins/Essentials.yml"] != "first-join: WELCOME new players" ||
		len(got["long.txt"]) > maxLine+2*len("…") || !bytes.Contains([]byte(got["long.txt"]), []byte("welcome")) {
		t.Fatalf("matches = %q", got)
	}
	if res, _ := search("", "secret"); len(res.GetMatches()) != 0 {
		t.Fatalf("found secrets: %v", res.GetMatches())
	}
	if res, _ := search("plugins", "players"); len(res.GetMatches()) != 1 || res.GetMatches()[0].GetLine() != 1 {
		t.Fatalf("matches in plugins = %v", res.GetMatches())
	}
	for path, query := range map[string]string{"": "", "server.properties": "x", "../": "x", "missing": "x"} {
		if _, err := search(path, query); err == nil {
			t.Errorf("search for %q in %q succeeded", query, path)
		}
	}
}
