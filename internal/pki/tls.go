package pki

import (
	"crypto/tls"
	"crypto/x509"
	"errors"
	"slices"
)

// AgentServerTLS is used by agents and only admits clients that present a master
// certificate issued by ca. Node certificates are rejected, so agents can never
// command each other.
func AgentServerTLS(node tls.Certificate, ca *x509.Certificate) *tls.Config {
	return &tls.Config{
		MinVersion:   tls.VersionTLS13,
		Certificates: []tls.Certificate{node},
		ClientAuth:   tls.RequireAndVerifyClientCert,
		ClientCAs:    pool(ca),
		VerifyConnection: func(cs tls.ConnectionState) error {
			return cs.PeerCertificates[0].VerifyHostname(MasterName)
		},
	}
}

// NodeClientTLS lets the master connect to the agent of the node with the given ID.
func NodeClientTLS(master tls.Certificate, ca *x509.Certificate, nodeID string) *tls.Config {
	return &tls.Config{
		MinVersion:   tls.VersionTLS13,
		Certificates: []tls.Certificate{master},
		RootCAs:      pool(ca),
		ServerName:   NodeName(nodeID),
	}
}

// MasterServerTLS serves the enrollment endpoint of the master. Clients do not
// have a certificate yet; they authenticate with their join token instead.
func MasterServerTLS(master tls.Certificate) *tls.Config {
	return &tls.Config{MinVersion: tls.VersionTLS13, Certificates: []tls.Certificate{master}}
}

// PinnedMasterTLS is used by an agent during enrollment, before it holds the CA
// certificate. The master must present a master certificate whose chain contains
// the CA with the given SHA-256 fingerprint (taken from the join token).
func PinnedMasterTLS(caFingerprint string) *tls.Config {
	return &tls.Config{
		MinVersion: tls.VersionTLS13,
		// Standard verification is replaced by the pinned verification below.
		InsecureSkipVerify: true, //nolint:gosec // see VerifyConnection
		VerifyConnection: func(cs tls.ConnectionState) error {
			certs := cs.PeerCertificates
			i := slices.IndexFunc(certs, func(c *x509.Certificate) bool { return Fingerprint(c) == caFingerprint })
			if i < 0 {
				return errors.New("master certificate does not match the pinned CA fingerprint")
			}
			_, err := certs[0].Verify(x509.VerifyOptions{DNSName: MasterName, Roots: pool(certs[i])})
			return err
		},
	}
}

func pool(cert *x509.Certificate) *x509.CertPool {
	p := x509.NewCertPool()
	p.AddCert(cert)
	return p
}
