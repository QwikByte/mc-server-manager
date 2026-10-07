package backup

import (
	"archive/zip"
	"errors"
	"os"
	"path/filepath"
	"slices"
	"testing"
	"time"

	noryxv1 "github.com/QwikByte/noryx/api/noryx/v1"
	"github.com/QwikByte/noryx/internal/agent/datadir"
	"github.com/QwikByte/noryx/internal/agent/fileset"
	"github.com/QwikByte/noryx/internal/agent/network"
	"github.com/QwikByte/noryx/internal/agent/runtime"
	"github.com/QwikByte/noryx/internal/agent/storage"
)

const serverID = "aaaaaaaaaaaaaaaaaaaaaaaaaa"

// paperData creates the data directory of a Paper server.
func paperData(t *testing.T) (string, *datadir.Dir) {
	path := t.TempDir()
	files := map[string]string{
		"server.properties": "motd=hi\n", "bukkit.yml": "a: 1\n", "paper.jar": "jar", ".rcon-cli.env": "secret",
		"world/level.dat": "w", "world/region/r.0.0.mca": "chunks", "world_nether/level.dat": "n",
		"plugins/LuckPerms.jar": "lp", "plugins/LuckPerms/config.yml": "storage: h2\n",
		"config/paper-global.yml": "x: 1\n", "logs/latest.log": "log", "cache/x": "x", fileset.ManifestFile: "{}",
	}
	for name, content := range files {
		write(t, filepath.Join(path, name), content)
	}
	dir, err := datadir.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { dir.Close() })
	return path, dir
}

func write(t *testing.T, name, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(name), 0o750); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(name, []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}
}

func TestSelected(t *testing.T) {
	path, dir := paperData(t)
	if err := os.Symlink(t.TempDir(), filepath.Join(path, "outside")); err != nil {
		t.Fatal(err)
	}
	paper := noryxv1.ServerType_SERVER_TYPE_PAPER
	for _, tc := range []struct {
		name string
		typ  noryxv1.ServerType
		sel  *noryxv1.BackupSelection
		want []string
	}{
		{"worlds", paper, &noryxv1.BackupSelection{Worlds: true}, []string{"world", "world_nether"}},
		{"plugins", paper, &noryxv1.BackupSelection{Plugins: true}, []string{"plugins"}},
		{"mods keep their settings in config", noryxv1.ServerType_SERVER_TYPE_FABRIC, &noryxv1.BackupSelection{Plugins: true}, []string{"config"}},
		{"config", paper, &noryxv1.BackupSelection{Config: true}, []string{"bukkit.yml", "config", "server.properties"}},
		{"paths inside others", paper, &noryxv1.BackupSelection{Worlds: true, Paths: []string{"world/region", "/logs/", "missing", "outside", fileset.ManifestFile}}, []string{"logs", "world", "world_nether"}},
		{"everything", paper, &noryxv1.BackupSelection{Everything: true, Worlds: true}, []string{"."}},
		{"root as a path", paper, &noryxv1.BackupSelection{Paths: []string{"/"}}, []string{"."}},
	} {
		got, err := selected(dir, tc.typ, tc.sel)
		if err != nil || !slices.Equal(got, tc.want) {
			t.Errorf("%s: got %q, %v; want %q", tc.name, got, err, tc.want)
		}
	}
	if _, err := selected(dir, paper, &noryxv1.BackupSelection{Paths: []string{"../other"}}); err == nil {
		t.Error("a path outside of the data directory was accepted")
	}
}

func TestBackupAndRestore(t *testing.T) {
	path, dir := paperData(t)
	s := Store{storage.New(t.TempDir())}
	create := func(paths []string, jobID string) Archive {
		t.Helper()
		b, err := s.create(t.Context(), dir, serverID, storage.Default, Details{Label: "test", Created: time.Now(), Paths: paths, JobID: jobID})
		if err != nil {
			t.Fatal(err)
		}
		return b
	}
	restore := func(b Archive) {
		t.Helper()
		staged, err := stage(t.Context(), dir, b)
		if err == nil {
			err = swap(dir, b, staged)
		}
		if err := dir.RemoveAll(staged); err != nil {
			t.Fatal(err)
		}
		if err != nil {
			t.Fatal(err)
		}
	}
	read := func(name string) string {
		data, _ := os.ReadFile(filepath.Join(path, name))
		return string(data)
	}

	// Restoring worlds replaces them, including files added since, and leaves the rest.
	worlds := create([]string{"world", "world_nether"}, "")
	write(t, filepath.Join(path, "world/region/r.0.0.mca"), "griefed")
	write(t, filepath.Join(path, "world/region/r.1.0.mca"), "new")
	write(t, filepath.Join(path, "server.properties"), "motd=changed\n")
	restore(worlds)
	if read("world/region/r.0.0.mca") != "chunks" || read("world/region/r.1.0.mca") != "" || read("server.properties") != "motd=changed\n" {
		t.Fatal("the worlds were not restored, or more than the worlds")
	}

	// Restoring everything brings the data directory back to the state of the backup, except
	// for the manifest of file sets, which isn't backed up: what it marks stays hidden.
	all := create([]string{"."}, "")
	write(t, filepath.Join(path, "plugins/Grief.jar"), "x")
	write(t, filepath.Join(path, fileset.ManifestFile), `{"marked":["bukkit.yml"]}`)
	if err := os.RemoveAll(filepath.Join(path, "world_nether")); err != nil {
		t.Fatal(err)
	}
	restore(all)
	if read("plugins/Grief.jar") != "" || read("world_nether/level.dat") != "n" || read(".rcon-cli.env") != "secret" {
		t.Fatal("everything was not restored")
	}
	if read(fileset.ManifestFile) != `{"marked":["bukkit.yml"]}` {
		t.Fatal("restoring replaced the manifest of file sets")
	}
	if zr, err := zip.OpenReader(all.Path()); err != nil || slices.ContainsFunc(zr.File, func(f *zip.File) bool { return f.Name == fileset.ManifestFile }) {
		t.Fatalf("the backup has the manifest of file sets: %v", err)
	} else {
		zr.Close()
	}
	if entries, _ := os.ReadDir(path); slices.ContainsFunc(entries, func(e os.DirEntry) bool { return datadir.IsTemp(e.Name()) }) {
		t.Fatal("temporary files were left behind")
	}

	// Backups are listed newest first; a job keeps its newest backups only.
	for range 3 {
		create([]string{"world"}, "job1")
	}
	if err := s.Prune(serverID, "job1", 2); err != nil {
		t.Fatal(err)
	}
	backups, err := s.List(serverID)
	if err != nil {
		t.Fatal(err)
	}
	var jobs int
	for _, b := range backups {
		if b.JobID == "job1" {
			jobs++
		}
	}
	if len(backups) != 4 || jobs != 2 || backups[len(backups)-1].ID != worlds.ID || backups[0].Size == 0 {
		t.Fatalf("backups after pruning = %+v", backups)
	}
	if _, err := s.Find(serverID, "../../etc/passwd"); err == nil {
		t.Fatal("found a backup with an invalid ID")
	}
	if err := s.RemoveAll(serverID); err != nil {
		t.Fatal(err)
	}
	if backups, _ := s.List(serverID); len(backups) != 0 {
		t.Fatalf("backups after removing all = %v", backups)
	}
}

// Files that held secrets of file sets stay as they are when a backup is restored: one that
// the server lost as it left a set doesn't come back, and one it has keeps its content.
func TestRestoreKeepsMarkedFiles(t *testing.T) {
	path, dir := paperData(t)
	s := Store{storage.New(t.TempDir())}
	write(t, filepath.Join(path, "plugins/Sync/token.yml"), "token: old")
	b, err := s.create(t.Context(), dir, serverID, storage.Default, Details{Created: time.Now(), Paths: []string{"."}})
	check(t, err)
	check(t, os.Remove(filepath.Join(path, "plugins/LuckPerms/config.yml")))
	write(t, filepath.Join(path, "plugins/Sync/token.yml"), "token: new")
	marked := []string{filepath.FromSlash("plugins/LuckPerms/config.yml"), filepath.FromSlash("plugins/Sync/token.yml")}

	staged, err := stage(t.Context(), dir, b)
	defer dir.RemoveAll(staged) //nolint:errcheck // a temporary folder
	check(t, err)
	check(t, keepFiles(dir, b, staged, marked))
	check(t, swap(dir, b, staged))
	if _, err := os.Stat(filepath.Join(path, "plugins/LuckPerms/config.yml")); !os.IsNotExist(err) {
		t.Error("restoring brought back a file with secrets the server lost")
	}
	if data, _ := os.ReadFile(filepath.Join(path, "plugins/Sync/token.yml")); string(data) != "token: new" {
		t.Errorf("a file with secrets was restored as %q", data)
	}
	if data, _ := os.ReadFile(filepath.Join(path, "bukkit.yml")); string(data) != "a: 1\n" {
		t.Error("the other files weren't restored")
	}
}

// Restoring a backup of a server brings back neither the secrets of a network it left since,
// nor the trust in its proxy or offline mode, and keeps the newer secret of its network.
func TestRestoreKeepsNetwork(t *testing.T) {
	velocity, paper := noryxv1.ServerType_SERVER_TYPE_VELOCITY, noryxv1.ServerType_SERVER_TYPE_PAPER
	joined := func(secret string) runtime.Network {
		return runtime.Network{Forwarding: runtime.ForwardingModern, ForwardingSecret: secret, Backends: []runtime.NetworkBackend{{Name: "lobby", Address: "lobby:25565"}}, Try: []string{"lobby"}}
	}
	proxy := func(n runtime.Network, key string) func(*datadir.Dir) error {
		return func(dir *datadir.Dir) error {
			_, _, err := network.WriteProxy(dir, velocity, n)
			if key == "" {
				return errors.Join(err, dir.RemoveAll(network.FloodgateKeyFile))
			}
			return errors.Join(err, dir.WriteFile(network.FloodgateKeyFile, []byte(key)))
		}
	}
	for _, tc := range []struct {
		name          string
		typ           noryxv1.ServerType
		before, after func(*datadir.Dir) error // the server as it is backed up, and as it is restored
		kept          []string                 // files that stay as they are then, or missing
	}{
		{"proxy that left", velocity, proxy(joined("old"), "old key"), proxy(runtime.Network{}, ""), network.SecretFiles(velocity)},
		{"proxy with a newer secret", velocity, proxy(joined("old"), "old key"), proxy(joined("new"), "new key"), network.SecretFiles(velocity)},
		{
			"game server that left", paper,
			func(dir *datadir.Dir) error {
				_, err := network.WriteBackend(dir, paper, runtime.ForwardingModern, "old")
				return errors.Join(err, dir.WriteFile("server.properties", []byte("online-mode=false\n")))
			},
			func(dir *datadir.Dir) error {
				_, err := network.WriteBackend(dir, paper, runtime.ForwardingNone, "")
				return errors.Join(err, network.Leave(dir, true, false))
			},
			[]string{network.PaperGlobalFile, "spigot.yml", "server.properties"},
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			path := t.TempDir()
			dir, err := datadir.Open(path)
			check(t, err)
			defer dir.Close()
			check(t, dir.MkdirAll("plugins/floodgate"))
			check(t, dir.WriteFile("motd.txt", []byte("then")))
			check(t, tc.before(dir))
			s := Store{storage.New(t.TempDir())}
			b, err := s.create(t.Context(), dir, serverID, storage.Default, Details{Created: time.Now(), Paths: []string{"."}})
			check(t, err)
			check(t, dir.WriteFile("motd.txt", []byte("now")))
			check(t, tc.after(dir))
			now := map[string]string{}
			for _, name := range tc.kept {
				data, _ := os.ReadFile(filepath.Join(path, name))
				now[name] = string(data)
			}

			staged, err := stage(t.Context(), dir, b)
			defer dir.RemoveAll(staged) //nolint:errcheck // a temporary folder
			check(t, err)
			check(t, keep(dir, tc.typ, b, staged))
			check(t, swap(dir, b, staged))
			for name, want := range now {
				if got, _ := os.ReadFile(filepath.Join(path, name)); string(got) != want {
					t.Errorf("%s was restored as %q, want %q", name, got, want)
				}
			}
			if got, _ := os.ReadFile(filepath.Join(path, "motd.txt")); string(got) != "then" {
				t.Error("the other files weren't restored")
			}
		})
	}
}

func check(t *testing.T, err error) {
	t.Helper()
	if err != nil {
		t.Fatal(err)
	}
}
