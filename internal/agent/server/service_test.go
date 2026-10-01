package server

import "testing"

func TestPlainRemovesFormatting(t *testing.T) {
	for in, want := range map[string]string{
		"\x1b[0;32m[12:00:00 INFO]: Done (3.2s)!\x1b[m\r": "[12:00:00 INFO]: Done (3.2s)!",
		"§6There are §c2§6 of a max of §A20§r players":    "There are 2 of a max of 20 players",
		"plain line": "plain line",
	} {
		if got := plain(in); got != want {
			t.Errorf("plain(%q) = %q, want %q", in, got, want)
		}
	}
}
