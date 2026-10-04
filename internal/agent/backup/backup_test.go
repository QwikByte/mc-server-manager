package backup

import (
	"os"
	"path/filepath"
	"slices"
	"testing"
	"time"

	noryxv1 "github.com/QwikByte/mc-server-manager/api/noryx/v1"
	"github.com/QwikByte/mc-server-manager/internal/agent/datadir"
	"github.com/QwikByte/mc-server-manager/internal/agent/storage"
)

const serverID = "aaaaaaaaaaaaaaaaaaaaaaaaaa"

// paperData creates the data directory of a Paper server.
func paperData(t *testing.T) (string, *datadir.Dir) {
	path := t.TempDir()
	files := map[string]string{
		"server.properties": "motd=hi\n", "bukkit.yml": "a: 1\n", "paper.jar": "jar", ".rcon-cli.env": "secret",
		"world/level.dat": "w", "world/region/r.0.0.mca": "chunks", "world_nether/level.dat": "n",
		"plugins/LuckPerms.jar": "lp", "plugins/LuckPerms/config.yml": "storage: h2\n",
		"config/paper-global.yml": "x: 1\n", "logs/latest.log": "log", "cache/x": "x",
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
		{"paths inside others", paper, &noryxv1.BackupSelection{Worlds: true, Paths: []string{"world/region", "/logs/", "missing", "outside"}}, []string{"logs", "world", "world_nether"}},
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
	s := store{storage.New(t.TempDir())}
	create := func(paths []string, jobID string) backup {
		t.Helper()
		b, err := s.create(t.Context(), dir, serverID, storage.Default, details{Label: "test", Created: time.Now(), Paths: paths, JobID: jobID})
		if err != nil {
			t.Fatal(err)
		}
		return b
	}
	restore := func(b backup) {
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

	// Restoring everything brings the data directory back to the state of the backup.
	all := create([]string{"."}, "")
	write(t, filepath.Join(path, "plugins/Grief.jar"), "x")
	if err := os.RemoveAll(filepath.Join(path, "world_nether")); err != nil {
		t.Fatal(err)
	}
	restore(all)
	if read("plugins/Grief.jar") != "" || read("world_nether/level.dat") != "n" || read(".rcon-cli.env") != "secret" {
		t.Fatal("everything was not restored")
	}
	if entries, _ := os.ReadDir(path); slices.ContainsFunc(entries, func(e os.DirEntry) bool { return datadir.IsTemp(e.Name()) }) {
		t.Fatal("temporary files were left behind")
	}

	// Backups are listed newest first; a job keeps its newest backups only.
	for range 3 {
		create([]string{"world"}, "job1")
	}
	if err := s.prune(serverID, "job1", 2); err != nil {
		t.Fatal(err)
	}
	backups, err := s.list(serverID)
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
	if _, err := s.find(serverID, "../../etc/passwd"); err == nil {
		t.Fatal("found a backup with an invalid ID")
	}
	if err := s.removeAll(serverID); err != nil {
		t.Fatal(err)
	}
	if backups, _ := s.list(serverID); len(backups) != 0 {
		t.Fatalf("backups after removing all = %v", backups)
	}
}
