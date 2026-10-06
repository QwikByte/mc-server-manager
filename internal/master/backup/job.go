package backup

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"regexp"
	"slices"
	"strings"
	"sync"
	"time"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	noryxv1 "github.com/QwikByte/noryx/api/noryx/v1"
	"github.com/QwikByte/noryx/internal/logging"
	"github.com/QwikByte/noryx/internal/master/httpapi"
	"github.com/QwikByte/noryx/internal/master/schedule"
)

// TaskKind identifies backup jobs among the scheduled tasks.
const TaskKind = "backup"

const (
	maxKeep       = 1000
	maxDatastores = 50
)

var (
	locationPattern = regexp.MustCompile(`^[a-z0-9][a-z0-9-]{0,31}$`)
	idPattern       = regexp.MustCompile(`^[a-z2-7]{26}$`)
)

// JobSettings are what a backup job backs up, and where.
type JobSettings struct {
	Selection Selection `json:"selection"`
	// Datastores are dumped with all their databases. Deleted ones are left out.
	Datastores []string `json:"datastores"`
	// Location is the storage location on each node; empty means the default one.
	Location string `json:"location"`
	// Keep is the number of backups of the job kept per server; 0 keeps all.
	Keep uint32 `json:"keep"`
}

// Datastore is a datastore of a network, which backup jobs dump.
type Datastore struct{ ID, Name, NodeID, NodeName string }

// Datastores find the datastores of backup jobs.
type Datastores interface {
	// Locate returns those of the datastores with the IDs that exist.
	Locate(ctx context.Context, ids []string) ([]Datastore, error)
}

// Jobs is the kind of task that backs up servers and dumps datastores.
type Jobs struct {
	nodes      Nodes
	datastores Datastores
}

func NewJobs(nodes Nodes, datastores Datastores) Jobs {
	return Jobs{nodes: nodes, datastores: datastores}
}

func (Jobs) Check(raw json.RawMessage) (json.RawMessage, error) {
	var s JobSettings
	msg := "Choose what to back up."
	if err := json.Unmarshal(raw, &s); err == nil {
		msg = s.Selection.check()
		if s.Selection.empty() && len(s.Datastores) > 0 {
			msg = "" // a job that only dumps datastores
		}
	}
	slices.Sort(s.Datastores)
	s.Datastores = slices.Compact(append([]string{}, s.Datastores...))
	switch {
	case msg != "":
	case len(s.Datastores) > maxDatastores || slices.ContainsFunc(s.Datastores, func(id string) bool { return !idPattern.MatchString(id) }):
		msg = "Choose up to 50 datastores."
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

// TargetsOptional tells that a job that dumps datastores needs no servers.
func (Jobs) TargetsOptional(raw json.RawMessage) bool {
	var s JobSettings
	return json.Unmarshal(raw, &s) == nil && len(s.Datastores) > 0
}

func (Jobs) Category() slog.Attr { return logging.Backups }

// Run backs up the servers of a job and dumps its datastores. Each node backs up one at a
// time, to spare its disks; nodes work in parallel.
func (j Jobs) Run(ctx context.Context, t schedule.Task, servers schedule.Servers, _ time.Time) error {
	var s JobSettings
	if err := json.Unmarshal(t.Settings, &s); err != nil {
		return err
	}
	var list []schedule.Server
	var err error
	if !s.Selection.empty() {
		list, err = servers(ctx)
	}
	datastores, dsErr := j.datastores.Locate(ctx, s.Datastores)
	byNode := map[string][]func() error{}
	for _, srv := range list {
		byNode[srv.NodeID] = append(byNode[srv.NodeID], func() error {
			return srv.Report(t, logging.Backups, "Back up server", j.backUp(ctx, srv, t, s))
		})
	}
	for _, ds := range datastores {
		byNode[ds.NodeID] = append(byNode[ds.NodeID], func() error { return j.dump(ctx, ds, t, s) })
	}
	var (
		mu   sync.Mutex
		wg   sync.WaitGroup
		errs = []error{err, dsErr}
	)
	for _, onNode := range byNode {
		wg.Go(func() {
			for _, backUp := range onNode {
				err := backUp()
				mu.Lock()
				errs = append(errs, err)
				mu.Unlock()
			}
		})
	}
	wg.Wait()
	return errors.Join(errs...)
}

// dump dumps the databases of a datastore and logs how it went. One without databases yet
// needs none.
func (j Jobs) dump(ctx context.Context, ds Datastore, t schedule.Task, s JobSettings) error {
	ctx, cancel := context.WithTimeout(ctx, backupTimeout)
	defer cancel()
	conn, err := j.nodes.Conn(ctx, ds.NodeID)
	if err == nil {
		_, err = noryxv1.NewDatastoreServiceClient(conn).CreateDump(ctx, &noryxv1.CreateDumpRequest{
			Id: ds.ID, Label: t.Name, Location: s.Location, JobId: t.ID, Keep: s.Keep,
		})
	}
	attrs := []any{logging.Backups, "task", t.Name, "datastore", ds.ID, "datastore_name", ds.Name, logging.KeyNode, ds.NodeID, logging.KeyNodeName, ds.NodeName}
	switch {
	case err == nil:
		slog.Info("Dump datastore", attrs...)
		return nil
	case status.Code(err) == codes.FailedPrecondition && strings.Contains(status.Convert(err).Message(), "no databases"):
		return nil
	}
	slog.Warn("Dump datastore failed", append(attrs, "err", httpapi.Message(err))...)
	return fmt.Errorf("%s on %s: %s", ds.Name, ds.NodeName, httpapi.Message(err))
}

func (j Jobs) backUp(ctx context.Context, srv schedule.Server, t schedule.Task, s JobSettings) error {
	ctx, cancel := context.WithTimeout(ctx, backupTimeout)
	defer cancel()
	conn, err := j.nodes.Conn(ctx, srv.NodeID)
	if err != nil {
		return err
	}
	res, err := noryxv1.NewBackupServiceClient(conn).CreateBackup(ctx, &noryxv1.CreateBackupRequest{
		ServerId: srv.GetId(), Label: t.Name, Selection: s.Selection.proto(), Location: s.Location, JobId: t.ID, Keep: s.Keep,
		SkipWithoutData: true,
	})
	if err == nil && res.GetBackup() == nil {
		return schedule.Skipped("It has none of the selected data yet, e.g. as it never started.")
	}
	return err
}
