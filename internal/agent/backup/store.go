package backup

import (
	"cmp"
	"context"
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

	noryxv1 "github.com/QwikByte/noryx/api/noryx/v1"
	"github.com/QwikByte/noryx/internal/agent/datadir"
	"github.com/QwikByte/noryx/internal/agent/fileset"
	"github.com/QwikByte/noryx/internal/agent/storage"

	"github.com/QwikByte/noryx/internal/agent/progress"
)

// A backup is kept as <backups of the location>/<owner>/<ID>.zip, with its details in
// <ID>.json next to it. The owner is the ID of a server, or datastores/<ID> for the dumps of
// a datastore.

// idPattern matches backup IDs, which start with the time of creation; see noryxv1.NewBackupID.
var idPattern = regexp.MustCompile(`^\d{8}-\d{6}-[a-z2-7]{6}$`)

// Details describe a backup.
type Details struct {
	Label   string    `json:"label"`
	Created time.Time `json:"created"`
	Paths   []string  `json:"paths"` // "." is everything
	// Exclude are files and folders inside Paths that were left out.
	Exclude []string `json:"exclude,omitempty"`
	JobID   string   `json:"jobId,omitempty"`
	// Kept backups are never deleted by their job.
	Kept bool `json:"kept,omitempty"`
}

// Archive is a kept backup, a ZIP archive.
type Archive struct {
	Details
	ID       string
	Location string
	Size     int64
	dir      string
}

// Path returns the path of the archive.
func (b Archive) Path() string { return filepath.Join(b.dir, b.ID+".zip") }
func (b Archive) info() string { return filepath.Join(b.dir, b.ID+".json") }

func (b Archive) Proto() *noryxv1.Backup {
	return &noryxv1.Backup{
		Id: b.ID, Label: b.Label, CreatedUnix: b.Created.Unix(), Size: b.Size, Location: b.Location, Paths: slashed(b.Paths), JobId: b.JobID,
		Exclude: slashed(b.Exclude), Kept: b.Kept,
	}
}

func slashed(names []string) []string {
	paths := make([]string, len(names))
	for i, p := range names {
		paths[i] = filepath.ToSlash(p)
	}
	return paths
}

// Store keeps the backups of servers in the storage locations of the node, and those of
// datastores, each by the folder of its owner.
type Store struct{ storage *storage.Locations }

func NewStore(locations *storage.Locations) Store { return Store{locations} }

// List returns the backups of an owner in all locations, newest first.
func (s Store) List(owner string) ([]Archive, error) {
	locations, err := s.storage.List()
	if err != nil {
		return nil, err
	}
	var backups []Archive
	for _, l := range locations {
		root, err := s.storage.BackupPath(l.Name)
		if err != nil {
			return nil, err
		}
		dir := filepath.Join(root, owner)
		entries, err := os.ReadDir(dir)
		if errors.Is(err, fs.ErrNotExist) {
			continue
		}
		if err != nil {
			return nil, err
		}
		for _, e := range entries {
			id, ok := strings.CutSuffix(e.Name(), ".json")
			b := Archive{ID: id, Location: l.Name, dir: dir}
			if ok && idPattern.MatchString(id) && b.load() == nil {
				backups = append(backups, b)
			}
		}
	}
	slices.SortFunc(backups, func(a, b Archive) int { return cmp.Or(b.Created.Compare(a.Created), strings.Compare(b.ID, a.ID)) })
	return backups, nil
}

// load reads the details of a backup; a backup whose archive is missing is incomplete.
func (b *Archive) load() error {
	data, err := os.ReadFile(b.info())
	if err != nil {
		return err
	}
	info, err := os.Stat(b.Path())
	if err != nil {
		return err
	}
	b.Size = info.Size()
	return json.Unmarshal(data, &b.Details)
}

// Find returns a backup of an owner, or fs.ErrNotExist.
func (s Store) Find(owner, id string) (Archive, error) {
	backups, err := s.List(owner)
	if err != nil {
		return Archive{}, err
	}
	i := slices.IndexFunc(backups, func(b Archive) bool { return b.ID == id })
	if !idPattern.MatchString(id) || i < 0 {
		return Archive{}, fs.ErrNotExist
	}
	return backups[i], nil
}

// create archives paths of a server's data, without what they exclude, into a new backup in
// a location.
func (s Store) create(ctx context.Context, data *datadir.Dir, serverID, location string, d Details) (Archive, error) {
	// The archive hardly exceeds the size of the files.
	size := datadir.Size(data.FS(), d.Paths...) - datadir.Size(data.FS(), d.Exclude...)
	progress.Step(ctx, "archive", size)
	return s.Add(serverID, location, noryxv1.NewBackupID(d.Created), d, size, func(w io.Writer) error {
		return datadir.WriteZip(ctx, w, data.Root, func(name string) (bool, func([]byte) []byte) {
			// The manifest of file sets tells which files hold secrets: restoring an older one
			// would hide less.
			return name == fileset.ManifestFile || slices.Contains(d.Exclude, name), nil
		}, d.Paths...)
	})
}

// Add adds a backup of an owner to a location, whose archive write writes, if size bytes
// fit. It only shows up once its archive is complete, and never replaces another one.
func (s Store) Add(owner, location, id string, d Details, size int64, write func(io.Writer) error) (Archive, error) {
	root, err := s.storage.BackupPath(location)
	if err != nil {
		return Archive{}, err
	}
	b := Archive{Details: d, ID: id, Location: location, dir: filepath.Join(root, owner)}
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
	if err := storage.Fits(f, size); err != nil {
		return b, errors.Join(err, f.Close())
	}
	info, err := json.Marshal(b.Details)
	if err = errors.Join(err, write(f), f.Close()); err != nil {
		return b, err
	}
	if err := os.WriteFile(b.info(), info, 0o600); err != nil {
		return b, err
	}
	if err := os.Rename(tmp, b.Path()); err != nil {
		return b, errors.Join(err, os.Remove(b.info()))
	}
	return b, b.load()
}

// Temp creates a temporary file next to the backups of an owner in a location, if size bytes
// fit, e.g. for an archive to check before it becomes a backup. The caller removes it.
func (s Store) Temp(owner, location string, size int64) (*os.File, error) {
	root, err := s.storage.BackupPath(location)
	if err != nil {
		return nil, err
	}
	dir := filepath.Join(root, owner)
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return nil, err
	}
	f, err := os.OpenFile(datadir.TempName(dir), os.O_RDWR|os.O_CREATE|os.O_EXCL, 0o600) //nolint:gosec // a new file in the backups folder
	if err != nil {
		return nil, err
	}
	if err := storage.Fits(f, size); err != nil {
		return nil, errors.Join(err, f.Close(), os.Remove(f.Name()))
	}
	return f, nil
}

// Update stores the changed details of a backup.
func (s Store) Update(b Archive) error {
	info, err := json.Marshal(b.Details)
	if err != nil {
		return err
	}
	tmp := datadir.TempName(b.dir)
	if err = os.WriteFile(tmp, info, 0o600); err == nil {
		err = os.Rename(tmp, b.info())
	}
	if err != nil {
		return errors.Join(err, os.Remove(tmp))
	}
	return nil
}

// Remove deletes a backup; it disappears with its archive.
func (s Store) Remove(b Archive) error {
	if err := os.Remove(b.Path()); err != nil {
		return err
	}
	return os.Remove(b.info())
}

// Prune deletes the backups of an owner's job that its retention doesn't keep. Backups
// marked to keep are left alone, and don't count.
func (s Store) Prune(owner, jobID string, r *noryxv1.BackupRetention) error {
	if r.KeepsAll() {
		return nil
	}
	backups, err := s.List(owner)
	if err != nil {
		return err
	}
	backups = slices.DeleteFunc(backups, func(b Archive) bool { return b.JobID != jobID || b.Kept })
	keep := retained(backups, r)
	for i, b := range backups {
		if !keep[i] {
			err = errors.Join(err, s.Remove(b))
		}
	}
	return err
}

// retained tells which of backups, newest first, a retention keeps: the newest ones, and the
// newest of each of the last days, weeks and months that have backups.
func retained(backups []Archive, r *noryxv1.BackupRetention) []bool {
	loc, err := time.LoadLocation(r.GetTimeZone())
	if err != nil {
		loc = time.UTC
	}
	keep := make([]bool, len(backups))
	for i := range min(int(r.GetLast()), len(backups)) {
		keep[i] = true
	}
	for _, rule := range []struct {
		count  uint32
		period func(time.Time) [3]int
	}{
		{r.GetDays(), func(t time.Time) [3]int { y, m, d := t.Date(); return [3]int{y, int(m), d} }},
		{r.GetWeeks(), func(t time.Time) [3]int { y, w := t.ISOWeek(); return [3]int{y, w} }},
		{r.GetMonths(), func(t time.Time) [3]int { y, m, _ := t.Date(); return [3]int{y, int(m)} }},
	} {
		seen := map[[3]int]bool{}
		for i, b := range backups {
			period := rule.period(b.Created.In(loc))
			if seen[period] {
				continue
			}
			if len(seen) == int(rule.count) {
				break
			}
			seen[period], keep[i] = true, true
		}
	}
	return keep
}

// RemoveAll deletes all backups of an owner.
func (s Store) RemoveAll(owner string) error {
	locations, err := s.storage.List()
	for _, l := range locations {
		root, pathErr := s.storage.BackupPath(l.Name)
		err = errors.Join(err, pathErr)
		if pathErr == nil {
			err = errors.Join(err, os.RemoveAll(filepath.Join(root, owner)))
		}
	}
	return err
}
