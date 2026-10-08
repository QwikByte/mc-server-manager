package noryxv1

import (
	"crypto/rand"
	"strings"
	"time"
)

// NewBackupID returns the ID of a backup created at t, e.g. 20261002-040000-k3x7qa.
func NewBackupID(t time.Time) string {
	return t.UTC().Format("20060102-150405") + "-" + strings.ToLower(rand.Text()[:6])
}

// MaxBackupPaths limits the further paths of a backup's selection, those it leaves out, and
// the paths of a restore.
const MaxBackupPaths = 20

// MaxRetention limits each count of a BackupRetention.
const MaxRetention = 1000

// Retention returns which backups of a job a request with keep and retention keeps:
// retention if it is set, otherwise the newest keep.
func Retention(keep uint32, retention *BackupRetention) *BackupRetention {
	if retention != nil {
		return retention
	}
	return &BackupRetention{Last: keep}
}

// KeepsAll reports whether a retention keeps every backup.
func (r *BackupRetention) KeepsAll() bool {
	return r.GetLast() == 0 && r.GetDays() == 0 && r.GetWeeks() == 0 && r.GetMonths() == 0
}

// Problem returns what is wrong with a retention, or "".
func (r *BackupRetention) Problem() string {
	switch {
	case max(r.GetLast(), r.GetDays(), r.GetWeeks(), r.GetMonths()) > MaxRetention:
		return "Keep at most 1000 backups, days, weeks or months per server."
	case !ValidTimeZone(r.GetTimeZone()):
		return "Choose a time zone."
	}
	return ""
}
