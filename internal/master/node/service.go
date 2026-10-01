// Package node manages the machines running an agent: registration, enrollment
// and the mutually authenticated connections the master uses to command them.
package node

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"database/sql"
	"errors"
	"net"
	"net/http"
	"strings"
	"sync"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/backoff"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/status"

	mcsmv1 "github.com/QwikByte/mc-server-manager/api/mcsm/v1"
	"github.com/QwikByte/mc-server-manager/internal/enrollment"
	"github.com/QwikByte/mc-server-manager/internal/master/httpapi"
	"github.com/QwikByte/mc-server-manager/internal/pki"
)

const joinTokenTTL = time.Hour

// reconnect caps the retry delay, so a node shows up soon after its agent (re)starts.
var reconnect = grpc.ConnectParams{Backoff: backoff.Config{BaseDelay: time.Second, Multiplier: 1.6, Jitter: 0.2, MaxDelay: 10 * time.Second}}

var errNotFound = httpapi.Errorf(http.StatusNotFound, "Node not found.")

type Node struct {
	ID         string     `json:"id"`
	Name       string     `json:"name"`
	Address    string     `json:"address"`
	EnrolledAt *time.Time `json:"enrolledAt,omitempty"`
	CreatedAt  time.Time  `json:"createdAt"`
}

type Service struct {
	mcsmv1.UnimplementedEnrollmentServiceServer

	db         *sql.DB
	ca         *pki.CA
	cert       *pki.Holder // the master's own certificate
	enrollAddr string      // published in join tokens

	mu      sync.Mutex
	conns   map[string]*grpc.ClientConn
	renewMu sync.Mutex // one certificate renewal at a time
}

func NewService(db *sql.DB, ca *pki.CA, cert *pki.Holder, enrollAddr string) *Service {
	return &Service{db: db, ca: ca, cert: cert, enrollAddr: enrollAddr, conns: make(map[string]*grpc.ClientConn)}
}

// Create registers a node and returns the join token its agent enrolls with.
func (s *Service) Create(ctx context.Context, name, address string) (Node, enrollment.Token, error) {
	n := Node{ID: strings.ToLower(rand.Text()), Name: strings.TrimSpace(name), Address: strings.TrimSpace(address), CreatedAt: time.Now()}
	if n.Name == "" || len(n.Name) > 64 {
		return n, enrollment.Token{}, httpapi.Errorf(http.StatusBadRequest, "Enter a name with up to 64 characters.")
	}
	if _, _, err := net.SplitHostPort(n.Address); err != nil {
		return n, enrollment.Token{}, httpapi.Errorf(http.StatusBadRequest, "Enter the agent address as host:port, for example 203.0.113.10:7443.")
	}
	_, err := s.db.ExecContext(ctx, `INSERT INTO nodes (id, name, address, created_at) VALUES (?, ?, ?, ?)`,
		n.ID, n.Name, n.Address, n.CreatedAt.Unix())
	if err != nil {
		if strings.Contains(err.Error(), "UNIQUE") {
			err = httpapi.Errorf(http.StatusConflict, "A node named %q already exists.", n.Name)
		}
		return n, enrollment.Token{}, err
	}
	token, err := s.NewJoinToken(ctx, n.ID)
	return n, token, err
}

// NewJoinToken replaces the pending join token of a node, e.g. to reinstall its agent.
func (s *Service) NewJoinToken(ctx context.Context, id string) (enrollment.Token, error) {
	secret := rand.Text()
	hash := sha256.Sum256([]byte(secret))
	res, err := s.db.ExecContext(ctx, `UPDATE nodes SET join_secret_hash = ?, join_expires_at = ? WHERE id = ?`,
		hash[:], time.Now().Add(joinTokenTTL).Unix(), id)
	if err == nil && rowsAffected(res) == 0 {
		err = errNotFound
	}
	return enrollment.Token{Master: s.enrollAddr, NodeID: id, Secret: secret, CAFingerprint: pki.Fingerprint(s.ca.Cert)}, err
}

// Enroll implements mcsmv1.EnrollmentServiceServer. The join token is consumed atomically.
func (s *Service) Enroll(ctx context.Context, req *mcsmv1.EnrollRequest) (*mcsmv1.EnrollResponse, error) {
	cert, err := s.ca.SignNodeCSR(req.GetCsrDer(), req.GetNodeId())
	if err != nil {
		return nil, status.Error(codes.InvalidArgument, "invalid certificate signing request")
	}
	hash := sha256.Sum256([]byte(req.GetSecret()))
	now := time.Now().Unix()
	res, err := s.db.ExecContext(ctx, `
		UPDATE nodes SET join_secret_hash = NULL, join_expires_at = NULL, enrolled_at = ?
		WHERE id = ? AND join_secret_hash = ? AND join_expires_at > ?`, now, req.GetNodeId(), hash[:], now)
	if err != nil {
		return nil, err
	}
	if rowsAffected(res) != 1 {
		return nil, status.Error(codes.PermissionDenied, "join token is invalid, expired or already used")
	}
	return &mcsmv1.EnrollResponse{CertificateDer: cert, CaCertificateDer: s.ca.Cert.Raw}, nil
}

func (s *Service) List(ctx context.Context) ([]Node, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT id, name, address, enrolled_at, created_at FROM nodes ORDER BY name`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	nodes := []Node{}
	for rows.Next() {
		n, err := scanNode(rows)
		if err != nil {
			return nil, err
		}
		nodes = append(nodes, n)
	}
	return nodes, rows.Err()
}

func (s *Service) Get(ctx context.Context, id string) (Node, error) {
	n, err := scanNode(s.db.QueryRowContext(ctx, `SELECT id, name, address, enrolled_at, created_at FROM nodes WHERE id = ?`, id))
	if errors.Is(err, sql.ErrNoRows) {
		err = errNotFound
	}
	return n, err
}

// Delete removes a node. Its agent keeps running but is no longer contacted.
func (s *Service) Delete(ctx context.Context, id string) error {
	res, err := s.db.ExecContext(ctx, `DELETE FROM nodes WHERE id = ?`, id)
	if err == nil && rowsAffected(res) == 0 {
		err = errNotFound
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if conn, ok := s.conns[id]; ok {
		conn.Close()
		delete(s.conns, id)
	}
	return err
}

// Conn returns the mutually authenticated connection to the agent of an enrolled node.
func (s *Service) Conn(ctx context.Context, id string) (grpc.ClientConnInterface, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if conn, ok := s.conns[id]; ok {
		return conn, nil
	}
	n, err := s.Get(ctx, id)
	if err != nil {
		return nil, err
	}
	if n.EnrolledAt == nil {
		return nil, httpapi.Errorf(http.StatusConflict, "Node %q has not been enrolled yet.", n.Name)
	}
	creds := credentials.NewTLS(pki.NodeClientTLS(s.cert, s.ca.Cert, n.ID))
	conn, err := grpc.NewClient(n.Address, grpc.WithTransportCredentials(creds), grpc.WithConnectParams(reconnect))
	if err != nil {
		return nil, err
	}
	s.conns[id] = conn
	return conn, nil
}

// Close closes all agent connections.
func (s *Service) Close() {
	s.mu.Lock()
	defer s.mu.Unlock()
	for id, conn := range s.conns {
		conn.Close()
		delete(s.conns, id)
	}
}

type scanner interface{ Scan(dest ...any) error }

func scanNode(row scanner) (Node, error) {
	var n Node
	var enrolledAt sql.NullInt64
	var createdAt int64
	if err := row.Scan(&n.ID, &n.Name, &n.Address, &enrolledAt, &createdAt); err != nil {
		return n, err
	}
	n.CreatedAt = time.Unix(createdAt, 0)
	if enrolledAt.Valid {
		n.EnrolledAt = new(time.Unix(enrolledAt.Int64, 0))
	}
	return n, nil
}

func rowsAffected(res sql.Result) int64 {
	n, _ := res.RowsAffected()
	return n
}
