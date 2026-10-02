package backup

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"regexp"
	"sync"
	"time"

	mcsmv1 "github.com/QwikByte/mc-server-manager/api/mcsm/v1"
	"github.com/QwikByte/mc-server-manager/internal/master/httpapi"
	"github.com/QwikByte/mc-server-manager/internal/master/schedule"
)

// TaskKind identifies backup jobs among the scheduled tasks.
const TaskKind = "backup"

const maxKeep = 1000

var locationPattern = regexp.MustCompile(`^[a-z0-9][a-z0-9-]{0,31}$`)

// JobSettings are what a backup job backs up, and where.
type JobSettings struct {
	Selection Selection `json:"selection"`
	// Location is the storage location on each node; empty means the default one.
	Location string `json:"location"`
	// Keep is the number of backups of the job kept per server; 0 keeps all.
	Keep uint32 `json:"keep"`
}

// Jobs is the kind of task that backs up servers.
type Jobs struct{ nodes Nodes }

func NewJobs(nodes Nodes) Jobs { return Jobs{nodes: nodes} }

func (Jobs) Check(raw json.RawMessage) (json.RawMessage, error) {
	var s JobSettings
	msg := "Choose what to back up."
	if err := json.Unmarshal(raw, &s); err == nil {
		msg = s.Selection.check()
	}
	switch {
	case msg != "":
	case s.Location != "" && !locationPattern.MatchString(s.Location):
		msg = "Choose a storage location of the nodes."
	case s.Keep > maxKeep:
		msg = "Keep at most 1000 backups per server, or 0 for all."
	default:
		return json.Marshal(s)
	}
	return nil, httpapi.Errorf(http.StatusBadRequest, "%s", msg)
}

func (Jobs) Lead(json.RawMessage) time.Duration { return 0 }

// Run backs up the servers of a job. Each node backs up one server at a time, to spare its
// disks; nodes work in parallel.
func (j Jobs) Run(ctx context.Context, t schedule.Task, servers schedule.Servers, _ time.Time) error {
	var s JobSettings
	if err := json.Unmarshal(t.Settings, &s); err != nil {
		return err
	}
	list, err := servers(ctx)
	byNode := map[string][]schedule.Server{}
	for _, srv := range list {
		byNode[srv.NodeID] = append(byNode[srv.NodeID], srv)
	}
	var (
		mu   sync.Mutex
		wg   sync.WaitGroup
		errs = []error{err}
	)
	for _, onNode := range byNode {
		wg.Go(func() {
			for _, srv := range onNode {
				err := srv.Wrap(j.backUp(ctx, srv, t, s))
				mu.Lock()
				errs = append(errs, err)
				mu.Unlock()
			}
		})
	}
	wg.Wait()
	return errors.Join(errs...)
}

func (j Jobs) backUp(ctx context.Context, srv schedule.Server, t schedule.Task, s JobSettings) error {
	ctx, cancel := context.WithTimeout(ctx, backupTimeout)
	defer cancel()
	conn, err := j.nodes.Conn(ctx, srv.NodeID)
	if err != nil {
		return err
	}
	_, err = mcsmv1.NewBackupServiceClient(conn).CreateBackup(ctx, &mcsmv1.CreateBackupRequest{
		ServerId: srv.GetId(), Label: t.Name, Selection: s.Selection.proto(), Location: s.Location, JobId: t.ID, Keep: s.Keep,
	})
	return err
}
