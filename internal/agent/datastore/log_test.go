package datastore

import (
	"context"
	"fmt"
	"slices"
	"strings"
	"testing"
	"unicode/utf8"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	noryxv1 "github.com/QwikByte/noryx/api/noryx/v1"
	"github.com/QwikByte/noryx/internal/agent/runtime"
)

// The statements of the agent, of the images' entrypoints and of plugins show no passwords or
// hashes, also when a line cuts one off.
func TestCleanHidesPasswords(t *testing.T) {
	for _, tc := range []struct{ line, want string }{
		// PostgreSQL logs the statement that failed, also each of a DO block.
		{
			`STATEMENT:  DO $$ BEGIN IF NOT EXISTS (SELECT FROM pg_roles WHERE rolname = 'shop') THEN CREATE ROLE shop LOGIN PASSWORD '` + password + `'; ELSE ALTER ROLE shop LOGIN PASSWORD '` + password + `'; END IF; END $$;`,
			`STATEMENT:  DO $$ BEGIN IF NOT EXISTS (SELECT FROM pg_roles WHERE rolname = 'shop') THEN CREATE ROLE shop LOGIN PASSWORD '<hidden>'; ELSE ALTER ROLE shop LOGIN PASSWORD '<hidden>'; END IF; END $$;`,
		},
		{`CONTEXT:  SQL statement "ALTER ROLE shop LOGIN PASSWORD '` + password + `'"`, `CONTEXT:  SQL statement "ALTER ROLE shop LOGIN PASSWORD '<hidden>'"`},
		{`STATEMENT:  ALTER ROLE shop PASSWORD 'SCRAM-SHA-256$4096:c2FsdA==$c3RvcmVk:c2VydmVy';`, `STATEMENT:  ALTER ROLE shop PASSWORD '<hidden>';`},
		{`alter user shop with encrypted password 'secret' valid until 'infinity'`, `alter user shop with encrypted password '<hidden>' valid until 'infinity'`},
		{`ALTER ROLE shop PASSWORD	'it''s '' secret' VALID UNTIL 'infinity'`, `ALTER ROLE shop PASSWORD	'<hidden>' VALID UNTIL 'infinity'`},
		{`ALTER ROLE shop PASSWORD ''`, `ALTER ROLE shop PASSWORD '<hidden>'`},
		// MariaDB's statements of the agent and of the image's entrypoint.
		{
			`CREATE USER IF NOT EXISTS 'shop'@'%' IDENTIFIED BY '` + password + `'; ALTER USER 'shop'@'%' identified  by '` + password + `';`,
			`CREATE USER IF NOT EXISTS 'shop'@'%' IDENTIFIED BY '<hidden>'; ALTER USER 'shop'@'%' identified  by '<hidden>';`,
		},
		{`ALTER USER 'shop'@'%' IDENTIFIED VIA mysql_native_password USING '*2470C0C06DEE42FD1618BB99005ADCA2EC9D1E19';`, `ALTER USER 'shop'@'%' IDENTIFIED VIA mysql_native_password USING '<hidden>';`},
		{`SET PASSWORD FOR 'root'@'localhost'= PASSWORD( 'secret') ;`, `SET PASSWORD FOR 'root'@'localhost'= PASSWORD( '<hidden>') ;`},
		{`IDENTIFIED VIA ed25519 USING PASSWORD('secret') OR unix_socket`, `IDENTIFIED VIA ed25519 USING PASSWORD('<hidden>') OR unix_socket`},
		{`ALTER USER 'shop'@'%' IDENTIFIED BY 'it\'s secret';`, `ALTER USER 'shop'@'%' IDENTIFIED BY '<hidden>';`},
		// Where a literal ends is unclear, the rest of the line is hidden.
		{`ERROR 1064 (42000): You have an error in your SQL syntax near 'IDENTIFIED BY 'secret'' at line 1`, `ERROR 1064 (42000): You have an error in your SQL syntax near 'IDENTIFIED BY '<hidden>'`},
		// A literal that the line cuts off is hidden to the end of the line.
		{`STATEMENT:  CREATE ROLE shop LOGIN PASSWORD 'abcdefghij`, `STATEMENT:  CREATE ROLE shop LOGIN PASSWORD '<hidden>'`},
		{`ALTER ROLE shop PASSWORD 'it''`, `ALTER ROLE shop PASSWORD '<hidden>'`},
		{`ALTER ROLE shop PASSWORD 'it\'s; SELECT 1`, `ALTER ROLE shop PASSWORD '<hidden>'`},
		{`ALTER ROLE shop PASSWORD '`, `ALTER ROLE shop PASSWORD '<hidden>'`},
		// Docker cuts lines into parts of 16 KiB and sends them joined, with the time of the
		// next part in between.
		{`ALTER ROLE shop PASSWORD 'abc2026-10-07T10:00:00.123456789Z def';`, `ALTER ROLE shop PASSWORD '<hidden>';`},
		// Control characters can't split a keyword.
		{"ALTER ROLE shop PASS\x00WORD '\x1bsecret';", `ALTER ROLE shop PASSWORD '<hidden>';`},
		// Lines without passwords stay as they are.
		{`FATAL:  password authentication failed for user "shop"`, `FATAL:  password authentication failed for user "shop"`},
		{`Access denied for user 'shop'@'172.18.0.3' (using password: YES)`, `Access denied for user 'shop'@'172.18.0.3' (using password: YES)`},
		{`LOG:  database system is ready to accept connections`, `LOG:  database system is ready to accept connections`},
	} {
		if got := clean(tc.line); got != tc.want {
			t.Errorf("clean(%q)\n got %q\nwant %q", tc.line, got, tc.want)
		}
	}
}

// Lines can't control the terminal of the local CLI, and are valid UTF-8, as gRPC requires.
func TestCleanKeepsText(t *testing.T) {
	for line, want := range map[string]string{
		"\x1b]0;title\x07LOG:\tready\r": "]0;titleLOG:\tready",
		"caf\xe9 ünïcode":               "caf� ünïcode",
		"\u009b31mred":                  "31mred",
	} {
		if got := clean(line); got != want || !utf8.ValidString(got) {
			t.Errorf("clean(%q) = %q, want %q", line, got, want)
		}
	}
}

type logStream struct {
	grpc.ServerStream
	ctx   context.Context
	lines []string
	times []int64
}

func (s *logStream) Context() context.Context { return s.ctx }

func (s *logStream) Send(res *noryxv1.StreamDatastoreLogsResponse) error {
	s.lines, s.times = append(s.lines, res.GetLine()), append(s.times, res.GetTimeUnixNano())
	return nil
}

func TestStreamDatastoreLogs(t *testing.T) {
	s, rt, _, id := newService(t)
	stream := func(req *noryxv1.StreamDatastoreLogsRequest) (*logStream, error) {
		st := &logStream{ctx: t.Context()}
		return st, s.StreamDatastoreLogs(req, st)
	}
	for _, req := range []*noryxv1.StreamDatastoreLogsRequest{{Id: "../x", Tail: 10}, {Id: "", Tail: 10}} {
		if _, err := stream(req); status.Code(err) != codes.InvalidArgument {
			t.Errorf("%v: %v", req, err)
		}
	}
	if _, err := stream(&noryxv1.StreamDatastoreLogsRequest{Id: runtime.NewID(), Tail: 10}); status.Code(err) != codes.NotFound {
		t.Errorf("unknown datastore: %v", err)
	}

	for i := range maxTail + 100 {
		rt.Log(id, fmt.Sprintf("line %d", i))
	}
	rt.Log(id, "STATEMENT:  ALTER ROLE shop LOGIN PASSWORD '"+password+"';")
	st, err := stream(&noryxv1.StreamDatastoreLogsRequest{Id: id, Tail: 1 << 31})
	must(t, err)
	if len(st.lines) != maxTail || st.lines[0] != "line 101" {
		t.Errorf("got %d lines from %q, want %d", len(st.lines), st.lines[0], maxTail)
	}
	st, err = stream(&noryxv1.StreamDatastoreLogsRequest{Id: id, Tail: 2})
	must(t, err)
	if want := []string{"line 1099", "STATEMENT:  ALTER ROLE shop LOGIN PASSWORD '<hidden>';"}; !slices.Equal(st.lines, want) {
		t.Errorf("got %q, want %q", st.lines, want)
	}
	// A client that connects again continues after the last line it got.
	st, err = stream(&noryxv1.StreamDatastoreLogsRequest{Id: id, Tail: 2, AfterUnixNano: st.times[0]})
	must(t, err)
	if len(st.lines) != 1 || strings.Contains(st.lines[0], password) {
		t.Errorf("after the first line: %q", st.lines)
	}
}
