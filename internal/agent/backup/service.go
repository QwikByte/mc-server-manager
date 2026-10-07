// Package backup implements the BackupService of the agent, which keeps backups of the
// servers in the storage locations of the node. Backups are only reachable through the
// agent: containers don't mount them, and the master can't choose where they are kept
// beyond the locations the node's administrator allowed.
package backup

import (
	"archive/zip"
	"cmp"
	"context"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"log/slog"
	"os"
	"regexp"
	"strings"
	"sync"
	"time"
	"unicode"
	"unicode/utf8"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	noryxv1 "github.com/QwikByte/noryx/api/noryx/v1"
	"github.com/QwikByte/noryx/internal/agent/datadir"
	"github.com/QwikByte/noryx/internal/agent/fileset"
	"github.com/QwikByte/noryx/internal/agent/runtime"
	"github.com/QwikByte/noryx/internal/agent/storage"
	"github.com/QwikByte/noryx/internal/logging"

	"github.com/QwikByte/noryx/internal/agent/progress"
)

const (
	chunkSize = 256 << 10
	maxLabel  = 64
	maxKeep   = 1000
)

var jobPattern = regexp.MustCompile(`^[a-z0-9]{1,64}$`)

type Service struct {
	noryxv1.UnimplementedBackupServiceServer
	rt    runtime.Runtime
	store Store

	mu   sync.Mutex
	busy map[string]bool // servers with a backup or restore in progress
}

func NewService(rt runtime.Runtime, locations *storage.Locations) *Service {
	return &Service{rt: rt, store: Store{locations}, busy: map[string]bool{}}
}

func (s *Service) ListBackups(ctx context.Context, req *noryxv1.ListBackupsRequest) (*noryxv1.ListBackupsResponse, error) {
	if _, err := s.find(ctx, req.GetServerId()); err != nil {
		return nil, err
	}
	backups, err := s.store.List(req.GetServerId())
	if err != nil {
		return nil, toStatus(err)
	}
	res := &noryxv1.ListBackupsResponse{}
	for _, b := range backups {
		res.Backups = append(res.Backups, b.Proto())
	}
	return res, nil
}

func (s *Service) CreateBackup(ctx context.Context, req *noryxv1.CreateBackupRequest) (*noryxv1.CreateBackupResponse, error) {
	label := strings.TrimSpace(req.GetLabel())
	if err := CheckDetails(label, req.GetJobId(), req.GetKeep()); err != nil {
		return nil, err
	}
	srv, release, err := s.lock(ctx, req.GetServerId())
	if err != nil {
		return nil, err
	}
	defer release()
	data, err := s.rt.Data(ctx, srv.ID)
	if err != nil {
		return nil, toStatus(err)
	}
	defer data.Close()
	paths, err := selected(data, srv.Type, req.GetSelection())
	if err != nil {
		return nil, toStatus(err)
	}
	switch {
	case len(paths) == 0 && req.GetSkipWithoutData():
		return &noryxv1.CreateBackupResponse{}, nil
	case len(paths) == 0:
		return nil, status.Error(codes.FailedPrecondition, "The server has none of the selected data.")
	}
	resume, err := runtime.PauseSaving(ctx, s.rt, srv)
	if errors.Is(err, runtime.ErrNotReady) {
		return nil, status.Error(codes.FailedPrecondition, "Wait until the server has started, or stop it, to back it up.")
	}
	if err != nil {
		return nil, toStatus(err)
	}
	location := cmp.Or(req.GetLocation(), storage.Default)
	b, err := s.store.create(ctx, data, srv.ID, location, Details{Label: label, Created: time.Now(), Paths: paths, JobID: req.GetJobId()})
	resume()
	if err != nil {
		return nil, toStatus(err)
	}
	if req.GetJobId() != "" && req.GetKeep() > 0 {
		if err := s.store.Prune(srv.ID, req.GetJobId(), int(req.GetKeep())); err != nil {
			slog.Warn("Can't delete old backups", logging.Backups, logging.KeyServer, srv.ID, "job", req.GetJobId(), "err", err)
		}
	}
	return &noryxv1.CreateBackupResponse{Backup: b.Proto()}, nil
}

// RestoreBackup extracts the backup before it stops the server, so the server is only
// down while the files are swapped. The restored data keeps the secrets of file sets and of
// the server's network, and its forwarding settings, as they are now.
func (s *Service) RestoreBackup(ctx context.Context, req *noryxv1.RestoreBackupRequest) (*noryxv1.RestoreBackupResponse, error) {
	srv, release, err := s.lock(ctx, req.GetServerId())
	if err != nil {
		return nil, err
	}
	defer release()
	b, err := s.store.Find(srv.ID, req.GetBackupId())
	if err != nil {
		return nil, toStatus(err)
	}
	data, err := s.rt.Data(ctx, srv.ID)
	if err != nil {
		return nil, toStatus(err)
	}
	defer data.Close()
	staged, err := stage(ctx, data, b)
	if staged != "" {
		defer data.RemoveAll(staged) //nolint:errcheck // best effort; the result of restoring matters
	}
	if err != nil {
		return nil, toStatus(err)
	}
	running := srv.State != noryxv1.ServerState_SERVER_STATE_STOPPED
	if running {
		progress.Step(ctx, "stop", 0)
		if err := s.rt.Stop(ctx, srv.ID); err != nil {
			return nil, toStatus(err)
		}
	}
	err = keep(data, srv.Type, b, staged)
	if err == nil {
		err = swap(data, b, staged)
	}
	if running { // also after a failure, which leaves the server as it was or partly restored
		progress.Step(ctx, "start", 0)
		err = errors.Join(err, s.rt.Start(context.WithoutCancel(ctx), srv.ID))
	}
	return &noryxv1.RestoreBackupResponse{}, toStatus(err)
}

func (s *Service) DeleteBackup(ctx context.Context, req *noryxv1.DeleteBackupRequest) (*noryxv1.DeleteBackupResponse, error) {
	srv, release, err := s.lock(ctx, req.GetServerId())
	if err != nil {
		return nil, err
	}
	defer release()
	b, err := s.store.Find(srv.ID, req.GetBackupId())
	if err == nil {
		err = s.store.Remove(b)
	}
	return &noryxv1.DeleteBackupResponse{}, toStatus(err)
}

func (s *Service) DownloadBackup(req *noryxv1.DownloadBackupRequest, stream noryxv1.BackupService_DownloadBackupServer) error {
	srv, err := s.find(stream.Context(), req.GetServerId())
	if err != nil {
		return err
	}
	b, err := s.store.Find(srv.ID, req.GetBackupId())
	if err != nil {
		return toStatus(err)
	}
	f, err := os.Open(b.Path())
	if err != nil {
		return toStatus(err)
	}
	defer f.Close()
	r, size := io.Reader(f), b.Size
	if req.GetHideSecrets() {
		// What the server marks now is hidden, also in backups from before.
		data, err := s.rt.Data(stream.Context(), srv.ID)
		if err != nil {
			return toStatus(err)
		}
		hidden := fileset.Read(data).Secrets()
		data.Close()
		zr, err := zip.NewReader(f, b.Size)
		if err != nil {
			return toStatus(err)
		}
		pr, pw := io.Pipe()
		defer pr.Close() // stops CopyZip if the client goes away
		go func() { pw.CloseWithError(datadir.CopyZip(pw, zr, hidden.Censor("."))) }()
		r, size = pr, 0
	}
	return Send(r, size, func(size int64, data []byte) error {
		return stream.Send(&noryxv1.DownloadBackupResponse{Size: size, Data: data})
	})
}

// Send sends what r reads in chunks, the first with size, until its end.
func Send(r io.Reader, size int64, send func(size int64, data []byte) error) error {
	buf := make([]byte, chunkSize)
	for first := true; ; first = false {
		n, err := io.ReadFull(r, buf)
		if n > 0 || first {
			if err := send(size, buf[:n]); err != nil {
				return err
			}
			size = 0
		}
		if errors.Is(err, io.EOF) || errors.Is(err, io.ErrUnexpectedEOF) {
			return nil
		}
		if err != nil {
			return toStatus(err)
		}
	}
}

// CheckDetails checks the label of a new backup, the job that creates it and how many of
// the job's backups are kept.
func CheckDetails(label, jobID string, keep uint32) error {
	switch {
	case utf8.RuneCountInString(label) > maxLabel || strings.ContainsFunc(label, unicode.IsControl):
		return status.Errorf(codes.InvalidArgument, "Enter a label with up to %d characters.", maxLabel)
	case jobID != "" && !jobPattern.MatchString(jobID):
		return status.Error(codes.InvalidArgument, "invalid job ID")
	case keep > maxKeep:
		return status.Errorf(codes.InvalidArgument, "Keep at most %d backups.", maxKeep)
	}
	return nil
}

// ImportBackup adds a backup that a server had on another node, with its ID and Details.
func (s *Service) ImportBackup(stream noryxv1.BackupService_ImportBackupServer) error {
	first, err := stream.Recv()
	if err != nil {
		return err
	}
	h := first.GetHeader()
	b := h.GetBackup()
	if b == nil {
		return status.Error(codes.InvalidArgument, "the first message must describe the backup")
	}
	d, err := importedDetails(b)
	if err != nil {
		return status.Error(codes.InvalidArgument, err.Error())
	}
	srv, release, err := s.lock(stream.Context(), h.GetServerId())
	if err != nil {
		return err
	}
	defer release()
	imported, err := s.store.Add(srv.ID, cmp.Or(b.GetLocation(), storage.Default), b.GetId(), d, b.GetSize(), func(w io.Writer) error {
		for {
			msg, err := stream.Recv()
			if errors.Is(err, io.EOF) {
				return nil
			}
			if err != nil {
				return err
			}
			if _, err := w.Write(msg.GetData()); err != nil {
				return err
			}
		}
	})
	if err != nil {
		return toStatus(err)
	}
	return stream.SendAndClose(&noryxv1.ImportBackupResponse{Backup: imported.Proto()})
}

// importedDetails validates a backup from another node like those made here.
func importedDetails(b *noryxv1.Backup) (Details, error) {
	d := Details{Label: b.GetLabel(), Created: time.Unix(b.GetCreatedUnix(), 0), JobID: b.GetJobId()}
	for _, p := range b.GetPaths() {
		name, ok := datadir.Name(p)
		if !ok {
			return d, fmt.Errorf("invalid path %q in backup", p)
		}
		d.Paths = append(d.Paths, name)
	}
	switch {
	case !idPattern.MatchString(b.GetId()):
		return d, errors.New("invalid backup ID")
	case len(d.Paths) == 0:
		return d, errors.New("the backup has no files")
	case utf8.RuneCountInString(d.Label) > maxLabel || strings.ContainsFunc(d.Label, unicode.IsControl):
		return d, errors.New("invalid label")
	case d.JobID != "" && !jobPattern.MatchString(d.JobID):
		return d, errors.New("invalid job ID")
	}
	return d, nil
}

// RemoveAll deletes all backups of a server, which is done when the server is deleted.
func (s *Service) RemoveAll(serverID string) error {
	if !runtime.ValidID(serverID) {
		return errors.New("invalid server ID")
	}
	return s.store.RemoveAll(serverID)
}

func (s *Service) find(ctx context.Context, id string) (runtime.Server, error) {
	if !runtime.ValidID(id) {
		return runtime.Server{}, status.Error(codes.InvalidArgument, "invalid server ID")
	}
	srv, err := runtime.Find(ctx, s.rt, id)
	return srv, toStatus(err)
}

// lock finds a server and reserves it for one backup, restore or deletion at a time.
func (s *Service) lock(ctx context.Context, id string) (runtime.Server, func(), error) {
	srv, err := s.find(ctx, id)
	if err != nil {
		return srv, nil, err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.busy[id] {
		return srv, nil, status.Error(codes.FailedPrecondition, "Another backup or restore of this server is in progress. Try again when it is done.")
	}
	s.busy[id] = true
	return srv, func() {
		s.mu.Lock()
		defer s.mu.Unlock()
		delete(s.busy, id)
	}, nil
}

func toStatus(err error) error {
	switch {
	case err == nil:
		return nil
	case status.Code(err) != codes.Unknown:
		return err
	case errors.Is(err, runtime.ErrNotFound):
		return status.Error(codes.NotFound, "Server not found.")
	case errors.Is(err, fs.ErrNotExist):
		return status.Error(codes.NotFound, "Backup not found.")
	case errors.Is(err, fs.ErrExist):
		return status.Error(codes.AlreadyExists, "The backup exists already.")
	case errors.Is(err, storage.ErrUnknown):
		return status.Errorf(codes.InvalidArgument, "%s. Add it on the node with: noryx-agent storage add", err)
	case errors.As(err, new(storage.FullError)):
		return status.Error(codes.ResourceExhausted, err.Error())
	case errors.Is(err, context.Canceled), errors.Is(err, context.DeadlineExceeded):
		return status.FromContextError(err).Err()
	}
	return status.Error(codes.Internal, err.Error())
}
