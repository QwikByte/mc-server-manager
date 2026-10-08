package docker

import (
	"io"
	"regexp"
	"strings"
	"testing"
	"testing/iotest"
)

// A dump runs in psql's restricted mode, without the \restrict and \unrestrict with which
// pg_dump wraps it, whose key its author knows; any other one stays and fails the load.
func TestRestricted(t *testing.T) {
	long := strings.Repeat("x", 10_000)
	for in, want := range map[string]string{
		"--\n-- PostgreSQL database dump\n--\n\n\\restrict abc123\n\n-- Dumped by pg_dump\n\nSET a = 1;\nCOPY t FROM stdin;\n\\unrestrict abc\n\n\\.\n\n" +
			"--\n-- PostgreSQL database dump complete\n--\n\n\\unrestrict abc123\n\n": "--\n-- PostgreSQL database dump\n--\n\n\n-- Dumped by pg_dump\n\nSET a = 1;\n" +
			"COPY t FROM stdin;\n\\unrestrict abc\n\n\\.\n\n--\n-- PostgreSQL database dump complete\n--\n",
		"SELECT 1;\r\n\\restrict abc\r\n\\! id\r\n":     "SELECT 1;\r\n\\restrict abc\r\n\\! id\r\n",
		"\\unrestrict abc\n\\! id\n":                    "\\unrestrict abc\n\\! id\n",
		"\\restrict a\n\\restrict b\n":                  "\\restrict b\n",
		"SELECT '" + long + "';\n\\unrestrict a\n":      "SELECT '" + long + "';\n",
		long + "\n\\unrestrict a\n" + long:              long + "\n\\unrestrict a\n" + long,
		"SELECT 1;\n" + strings.Repeat("\n", maxHeld+1): "SELECT 1;\n" + strings.Repeat("\n", maxHeld+1),
		"": "",
	} {
		out, err := io.ReadAll(iotest.OneByteReader(restricted(strings.NewReader(in))))
		if err != nil {
			t.Fatal(err)
		}
		first, rest, _ := strings.Cut(string(out), "\n")
		if !regexp.MustCompile(`^\\restrict [A-Z2-7]{26}$`).MatchString(first) || rest != want {
			t.Errorf("restricted(%.80q) = %.200q, want %.200q", in, out, want)
		}
	}
}
