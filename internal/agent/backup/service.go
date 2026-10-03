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

	mcsmv1 "github.com/QwikByte/mc-server-manager/api/mcsm/v1"
	"github.com/QwikByte/mc-server-manager/internal/agent/datadir"
	"github.com/QwikByte/mc-server-manager/internal/agent/runtime"
	"github.com/QwikByte/mc-server-manager/internal/agent/secrets"
	"github.com/QwikByte/mc-server-manager/internal/agent/storage"
	"github.com/QwikByte/mc-server-manager/internal/logging"
)

const (
	chunkSize = 256 << 10
	maxLabel  = 64
	maxKeep   = 1000
)

var jobPattern = regexp.MustCompile(`^[a-z0-9]{1,64}$`)

type Service struct {
	mcsmv1.UnimplementedBackupServiceServer
	rt    runtime.Runtime
	store store

	mu   sync.Mutex
	busy map[string]bool // servers with a backup or restore in progress
}

func NewService(rt runtime.Runtime, locations *storage.Locations) *Service {
	return &Service{rt: rt, store: store{locations}, busy: map[string]bool{}}
}

func (s *Service) ListBackups(ctx context.Context, req *mcsmv1.ListBackupsRequest) (*mcsmv1.ListBackupsResponse, error) {
	if _, err := s.find(ctx, req.GetServerId()); err != nil {
		return nil, err
	}
	backups, err := s.store.list(req.GetServerId())
	if err != nil {
		return nil, toStatus(err)
	}
	res := &mcsmv1.ListBackupsResponse{}
	for _, b := range backups {
		res.Backups = append(res.Backups, b.proto())
	}
	return res, nil
}

func (s *Service) CreateBackup(ctx context.Context, req *mcsmv1.CreateBackupRequest) (*mcsmv1.CreateBackupResponse, error) {
	label := strings.TrimSpace(req.GetLabel())
	switch {
	case utf8.RuneCountInString(label) > maxLabel || strings.ContainsFunc(label, unicode.IsControl):
		return nil, status.Errorf(codes.InvalidArgument, "Enter a label with up to %d characters.", maxLabel)
	case req.GetJobId() != "" && !jobPattern.MatchString(req.GetJobId()):
		return nil, status.Error(codes.InvalidArgument, "invalid job ID")
	case req.GetKeep() > maxKeep:
		return nil, status.Errorf(codes.InvalidArgument, "Keep at most %d backups.", maxKeep)
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
	if len(paths) == 0 {
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
	b, err := s.store.create(ctx, data, srv.ID, location, details{Label: label, Created: time.Now(), Paths: paths, JobID: req.GetJobId()})
	resume()
	if err != nil {
		return nil, toStatus(err)
	}
	if req.GetJobId() != "" && req.GetKeep() > 0 {
		if err := s.store.prune(srv.ID, req.GetJobId(), int(req.GetKeep())); err != nil {
			slog.Warn("Can't delete old backups", logging.Backups, logging.KeyServer, srv.ID, "job", req.GetJobId(), "err", err)
		}
	}
	return &mcsmv1.CreateBackupResponse{Backup: b.proto()}, nil
}

// RestoreBackup extracts the backup before it stops the server, so the server is only
// down while the files are swapped.
func (s *Service) RestoreBackup(ctx context.Context, req *mcsmv1.RestoreBackupRequest) (*mcsmv1.RestoreBackupResponse, error) {
	srv, release, err := s.lock(ctx, req.GetServerId())
	if err != nil {
		return nil, err
	}
	defer release()
	b, err := s.store.find(srv.ID, req.GetBackupId())
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
	running := srv.State != mcsmv1.ServerState_SERVER_STATE_STOPPED
	if running {
		if err := s.rt.Stop(ctx, srv.ID); err != nil {
			return nil, toStatus(err)
		}
	}
	err = swap(data, b, staged)
	if running { // also after a failure, which leaves the server as it was or partly restored
		err = errors.Join(err, s.rt.Start(context.WithoutCancel(ctx), srv.ID))
	}
	return &mcsmv1.RestoreBackupResponse{}, toStatus(err)
}

func (s *Service) DeleteBackup(ctx context.Context, req *mcsmv1.DeleteBackupRequest) (*mcsmv1.DeleteBackupResponse, error) {
	srv, release, err := s.lock(ctx, req.GetServerId())
	if err != nil {
		return nil, err
	}
	defer release()
	b, err := s.store.find(srv.ID, req.GetBackupId())
	if err == nil {
		err = s.store.remove(b)
	}
	return &mcsmv1.DeleteBackupResponse{}, toStatus(err)
}

func (s *Service) DownloadBackup(req *mcsmv1.DownloadBackupRequest, stream mcsmv1.BackupService_DownloadBackupServer) error {
	srv, err := s.find(stream.Context(), req.GetServerId())
	if err != nil {
		return err
	}
	b, err := s.store.find(srv.ID, req.GetBackupId())
	if err != nil {
		return toStatus(err)
	}
	f, err := os.Open(b.archive())
	if err != nil {
		return toStatus(err)
	}
	defer f.Close()
	r, size := io.Reader(f), b.Size
	if req.GetHideSecrets() {
		zr, err := zip.NewReader(f, b.Size)
		if err != nil {
			return toStatus(err)
		}
		pr, pw := io.Pipe()
		defer pr.Close() // stops CopyZip if the client goes away
		go func() { pw.CloseWithError(datadir.CopyZip(pw, zr, secrets.Censor("."))) }()
		r, size = pr, 0
	}
	res := &mcsmv1.DownloadBackupResponse{Size: size}
	buf := make([]byte, chunkSize)
	for first := true; ; first = false {
		n, err := io.ReadFull(r, buf)
		if n > 0 || first {
			res.Data = buf[:n]
			if err := stream.Send(res); err != nil {
				return err
			}
			res = &mcsmv1.DownloadBackupResponse{}
		}
		if errors.Is(err, io.EOF) || errors.Is(err, io.ErrUnexpectedEOF) {
			return nil
		}
		if err != nil {
			return toStatus(err)
		}
	}
}

// ImportBackup adds a backup that a server had on another node, with its ID and details.
func (s *Service) ImportBackup(stream mcsmv1.BackupService_ImportBackupServer) error {
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
	imported, err := s.store.add(srv.ID, cmp.Or(b.GetLocation(), storage.Default), b.GetId(), d, func(w io.Writer) error {
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
	return stream.SendAndClose(&mcsmv1.ImportBackupResponse{Backup: imported.proto()})
}

// importedDetails validates a backup from another node like those made here.
func importedDetails(b *mcsmv1.Backup) (details, error) {
	d := details{Label: b.GetLabel(), Created: time.Unix(b.GetCreatedUnix(), 0), JobID: b.GetJobId()}
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
	return s.store.removeAll(serverID)
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
		return status.Errorf(codes.InvalidArgument, "%s. Add it on the node with: mcsm-agent storage add", err)
	case errors.Is(err, context.Canceled), errors.Is(err, context.DeadlineExceeded):
		return status.FromContextError(err).Err()
	}
	return status.Error(codes.Internal, err.Error())
}
