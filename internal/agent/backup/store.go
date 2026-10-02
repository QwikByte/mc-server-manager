package backup

import (
	"cmp"
	"context"
	"crypto/rand"
	"encoding/json"
	"errors"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strings"
	"time"

	mcsmv1 "github.com/QwikByte/mc-server-manager/api/mcsm/v1"
	"github.com/QwikByte/mc-server-manager/internal/agent/datadir"
	"github.com/QwikByte/mc-server-manager/internal/agent/storage"
)

// A backup of a server is kept as <backups of the location>/<server ID>/<ID>.zip, with its
// details in <ID>.json next to it.

// idPattern matches backup IDs, which start with the time of creation, e.g. 20261002-040000-k3x7qa.
var idPattern = regexp.MustCompile(`^\d{8}-\d{6}-[a-z2-7]{6}$`)

func newID(t time.Time) string {
	return t.UTC().Format("20060102-150405") + "-" + strings.ToLower(rand.Text()[:6])
}

type details struct {
	Label   string    `json:"label"`
	Created time.Time `json:"created"`
	Paths   []string  `json:"paths"` // "." is everything
	JobID   string    `json:"jobId,omitempty"`
}

type backup struct {
	details
	ID       string
	Location string
	Size     int64
	dir      string
}

func (b backup) archive() string { return filepath.Join(b.dir, b.ID+".zip") }
func (b backup) info() string    { return filepath.Join(b.dir, b.ID+".json") }

func (b backup) proto() *mcsmv1.Backup {
	paths := make([]string, len(b.Paths))
	for i, p := range b.Paths {
		paths[i] = filepath.ToSlash(p)
	}
	return &mcsmv1.Backup{
		Id: b.ID, Label: b.Label, CreatedUnix: b.Created.Unix(), Size: b.Size, Location: b.Location, Paths: paths, JobId: b.JobID,
	}
}

// store keeps the backups of servers in the storage locations of the node.
type store struct{ storage *storage.Locations }

// list returns the backups of a server in all locations, newest first.
func (s store) list(serverID string) ([]backup, error) {
	locations, err := s.storage.List()
	if err != nil {
		return nil, err
	}
	var backups []backup
	for _, l := range locations {
		root, err := s.storage.BackupPath(l.Name)
		if err != nil {
			return nil, err
		}
		dir := filepath.Join(root, serverID)
		entries, err := os.ReadDir(dir)
		if errors.Is(err, fs.ErrNotExist) {
			continue
		}
		if err != nil {
			return nil, err
		}
		for _, e := range entries {
			id, ok := strings.CutSuffix(e.Name(), ".json")
			b := backup{ID: id, Location: l.Name, dir: dir}
			if ok && idPattern.MatchString(id) && b.load() == nil {
				backups = append(backups, b)
			}
		}
	}
	slices.SortFunc(backups, func(a, b backup) int { return cmp.Or(b.Created.Compare(a.Created), strings.Compare(b.ID, a.ID)) })
	return backups, nil
}

// load reads the details of a backup; a backup whose archive is missing is incomplete.
func (b *backup) load() error {
	data, err := os.ReadFile(b.info())
	if err != nil {
		return err
	}
	info, err := os.Stat(b.archive())
	if err != nil {
		return err
	}
	b.Size = info.Size()
	return json.Unmarshal(data, &b.details)
}

func (s store) find(serverID, id string) (backup, error) {
	backups, err := s.list(serverID)
	if err != nil {
		return backup{}, err
	}
	i := slices.IndexFunc(backups, func(b backup) bool { return b.ID == id })
	if !idPattern.MatchString(id) || i < 0 {
		return backup{}, fs.ErrNotExist
	}
	return backups[i], nil
}

// create archives paths of a server's data into a new backup in a location.
func (s store) create(ctx context.Context, data *datadir.Dir, serverID, location string, d details) (backup, error) {
	return s.add(serverID, location, newID(d.Created), d, func(w io.Writer) error { return datadir.WriteZip(ctx, w, data.Root, d.Paths...) })
}

// add adds a backup to a location, whose archive write writes. It only shows up once its
// archive is complete, and never replaces another one.
func (s store) add(serverID, location, id string, d details, write func(io.Writer) error) (backup, error) {
	root, err := s.storage.BackupPath(location)
	if err != nil {
		return backup{}, err
	}
	b := backup{details: d, ID: id, Location: location, dir: filepath.Join(root, serverID)}
	if err := os.MkdirAll(b.dir, 0o700); err != nil {
		return b, err
	}
	if _, err := os.Lstat(b.info()); err == nil {
		return b, fs.ErrExist
	}
	tmp := datadir.TempName(b.dir)
	f, err := os.OpenFile(tmp, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o600) //nolint:gosec // a new file in the backups folder
	if err != nil {
		return b, err
	}
	defer os.Remove(tmp) // fails once the archive is in place
	info, err := json.Marshal(b.details)
	if err = errors.Join(err, write(f), f.Close()); err != nil {
		return b, err
	}
	if err := os.WriteFile(b.info(), info, 0o600); err != nil {
		return b, err
	}
	if err := os.Rename(tmp, b.archive()); err != nil {
		return b, errors.Join(err, os.Remove(b.info()))
	}
	return b, b.load()
}

// remove deletes a backup; it disappears with its archive.
func (s store) remove(b backup) error {
	if err := os.Remove(b.archive()); err != nil {
		return err
	}
	return os.Remove(b.info())
}

// prune deletes the oldest backups of a job beyond the newest keep.
func (s store) prune(serverID, jobID string, keep int) error {
	backups, err := s.list(serverID)
	if err != nil {
		return err
	}
	backups = slices.DeleteFunc(backups, func(b backup) bool { return b.JobID != jobID })
	for _, b := range backups[min(keep, len(backups)):] {
		err = errors.Join(err, s.remove(b))
	}
	return err
}

// removeAll deletes all backups of a server.
func (s store) removeAll(serverID string) error {
	locations, err := s.storage.List()
	for _, l := range locations {
		root, pathErr := s.storage.BackupPath(l.Name)
		err = errors.Join(err, pathErr)
		if pathErr == nil {
			err = errors.Join(err, os.RemoveAll(filepath.Join(root, serverID)))
		}
	}
	return err
}
