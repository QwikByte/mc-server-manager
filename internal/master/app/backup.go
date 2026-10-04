package app

import (
	"archive/tar"
	"compress/gzip"
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"time"

	"golang.org/x/term"
)

// backupMaster writes the master's database and certificate authority to a .tar.gz file, or to
// stdout for "-". The database is copied with VACUUM INTO, so the master may keep running.
// The archive holds the CA's private key, so the file is only readable by its owner.
func backupMaster(ctx context.Context, cfg config, target string) (err error) {
	if target == "-" && term.IsTerminal(int(os.Stdout.Fd())) {
		return errors.New("redirect the backup to a file, e.g. noryx-master backup - > master-backup.tar.gz")
	}
	if _, err := os.Stat(filepath.Join(cfg.dataDir, "master.db")); err != nil {
		return fmt.Errorf("no master in %s, set its --data-dir: %w", cfg.dataDir, err)
	}
	db, err := openDB(cfg.dataDir)
	if err != nil {
		return err
	}
	defer db.Close()
	tmp, err := os.MkdirTemp("", "noryx-master-backup-")
	if err != nil {
		return err
	}
	defer os.RemoveAll(tmp)
	snapshot := filepath.Join(tmp, "master.db")
	if _, err := db.ExecContext(ctx, "VACUUM INTO ?", snapshot); err != nil {
		return fmt.Errorf("copy the database: %w", err)
	}

	out := os.Stdout
	if target != "-" {
		//nolint:gosec // the administrator's choice on the command line
		if out, err = os.OpenFile(target, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o600); err != nil {
			return err
		}
		defer func() {
			if err = errors.Join(err, out.Close()); err != nil {
				os.Remove(target)
				return
			}
			fmt.Fprintf(os.Stderr, "Saved the database and the certificate authority to %s. Keep it as safe as the master, as it holds the CA's private key.\n", target)
		}()
	}
	gz := gzip.NewWriter(out)
	archive := tar.NewWriter(gz)
	if err := addFile(archive, "master.db", snapshot); err != nil {
		return err
	}
	pkiDir := filepath.Join(cfg.dataDir, "pki")
	entries, err := os.ReadDir(pkiDir)
	if err != nil {
		return err
	}
	if err := archive.WriteHeader(&tar.Header{Typeflag: tar.TypeDir, Name: "pki/", Mode: 0o700, ModTime: time.Now()}); err != nil {
		return err
	}
	for _, e := range entries {
		if e.Type().IsRegular() {
			if err := addFile(archive, "pki/"+e.Name(), filepath.Join(pkiDir, e.Name())); err != nil {
				return err
			}
		}
	}
	return errors.Join(archive.Close(), gz.Close())
}

func addFile(archive *tar.Writer, name, path string) error {
	f, err := os.Open(path) //nolint:gosec // files of the data directory
	if err != nil {
		return err
	}
	defer f.Close()
	info, err := f.Stat()
	if err != nil {
		return err
	}
	if err := archive.WriteHeader(&tar.Header{Name: name, Mode: 0o600, Size: info.Size(), ModTime: info.ModTime()}); err != nil {
		return err
	}
	_, err = io.Copy(archive, f)
	return err
}
