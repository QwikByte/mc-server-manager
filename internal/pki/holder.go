package pki

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"log/slog"
	"sync/atomic"
	"time"
)

// Holder holds the certificate of a long-running process, so that it can be renewed
// without restarting listeners or dropping connections.
type Holder struct {
	cert atomic.Pointer[tls.Certificate]
}

func NewHolder(cert tls.Certificate) (*Holder, error) {
	h := &Holder{}
	return h, h.Set(cert)
}

// Get returns the current certificate. Its Leaf is always set.
func (h *Holder) Get() *tls.Certificate { return h.cert.Load() }

// Set replaces the certificate for all following handshakes.
func (h *Holder) Set(cert tls.Certificate) error {
	if cert.Leaf == nil {
		leaf, err := x509.ParseCertificate(cert.Certificate[0])
		if err != nil {
			return err
		}
		cert.Leaf = leaf
	}
	h.cert.Store(&cert)
	return nil
}

// Maintain renews the certificate with issue whenever NeedsRenewal says so, checking
// at the given interval until ctx is done.
func (h *Holder) Maintain(ctx context.Context, interval time.Duration, issue func() (tls.Certificate, error)) {
	ticker := time.NewTicker(interval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case now := <-ticker.C:
			if !NeedsRenewal(h.Get().Leaf, now) {
				continue
			}
			cert, err := issue()
			if err == nil {
				err = h.Set(cert)
			}
			if err != nil {
				slog.Error("renew certificate", "name", h.Get().Leaf.Subject.CommonName, "err", err)
				continue
			}
			slog.Info("renewed certificate", "name", h.Get().Leaf.Subject.CommonName, "not_after", h.Get().Leaf.NotAfter)
		}
	}
}

// NeedsRenewal reports whether less than a third of the certificate's lifetime is left.
func NeedsRenewal(cert *x509.Certificate, now time.Time) bool {
	return now.After(cert.NotAfter.Add(-cert.NotAfter.Sub(cert.NotBefore) / 3))
}
