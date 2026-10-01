package pki

import (
	"crypto/tls"
	"errors"
	"testing"
)

func TestLoadOrCreateCAIsStable(t *testing.T) {
	dir := t.TempDir()
	first, err := LoadOrCreateCA(dir)
	if err != nil {
		t.Fatal(err)
	}
	second, err := LoadOrCreateCA(dir)
	if err != nil {
		t.Fatal(err)
	}
	if Fingerprint(first.Cert) != Fingerprint(second.Cert) {
		t.Fatal("CA changed between starts")
	}
}

func TestMutualTLS(t *testing.T) {
	ca, err := LoadOrCreateCA(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	master, err := ca.MasterCertificate()
	if err != nil {
		t.Fatal(err)
	}
	node1, node2 := enrollNode(t, ca, "node-1"), enrollNode(t, ca, "node-2")
	agent := AgentServerTLS(node1, ca.Cert)
	fingerprint := Fingerprint(ca.Cert)

	tests := []struct {
		name           string
		client, server *tls.Config
		wantErr        bool
	}{
		{"master reaches agent", NodeClientTLS(master, ca.Cert, "node-1"), agent, false},
		{"node cannot command agent", NodeClientTLS(node2, ca.Cert, "node-1"), agent, true},
		{"master rejects wrong node identity", NodeClientTLS(master, ca.Cert, "node-2"), agent, true},
		{"enrollment with pinned CA", PinnedMasterTLS(fingerprint), MasterServerTLS(master), false},
		{"enrollment rejects other CA", PinnedMasterTLS(fingerprint[1:] + "0"), MasterServerTLS(master), true},
		{"enrollment rejects node posing as master", PinnedMasterTLS(fingerprint), MasterServerTLS(withCA(node2, ca)), true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if err := handshake(t, tt.client, tt.server); (err != nil) != tt.wantErr {
				t.Fatalf("handshake error = %v, want error %v", err, tt.wantErr)
			}
		})
	}
}

func enrollNode(t *testing.T, ca *CA, nodeID string) tls.Certificate {
	t.Helper()
	key := NewKey()
	csr, err := NewCSR(key)
	if err != nil {
		t.Fatal(err)
	}
	der, err := ca.SignNodeCSR(csr, nodeID)
	if err != nil {
		t.Fatal(err)
	}
	return tls.Certificate{Certificate: [][]byte{der}, PrivateKey: key}
}

func withCA(cert tls.Certificate, ca *CA) tls.Certificate {
	cert.Certificate = append(cert.Certificate, ca.Cert.Raw)
	return cert
}

// handshake runs a TLS handshake over loopback and returns the errors of both sides.
func handshake(t *testing.T, client, server *tls.Config) error {
	t.Helper()
	ln, err := tls.Listen("tcp", "127.0.0.1:0", server)
	if err != nil {
		t.Fatal(err)
	}
	defer ln.Close()

	serverErr := make(chan error, 1)
	go func() {
		conn, err := ln.Accept()
		if err != nil {
			serverErr <- err
			return
		}
		defer conn.Close()
		serverErr <- conn.(*tls.Conn).Handshake()
	}()

	conn, err := tls.Dial("tcp", ln.Addr().String(), client)
	if err == nil {
		conn.Close()
	}
	return errors.Join(err, <-serverErr)
}
