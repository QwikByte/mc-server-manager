package node

import (
	"bytes"
	"encoding/json"
	"log/slog"
	"strings"
	"testing"
)

// A node that goes offline is logged once, and once when it is back; a single failed check
// isn't.
func TestNotePresence(t *testing.T) {
	var buf bytes.Buffer
	prev := slog.Default()
	slog.SetDefault(slog.New(slog.NewJSONHandler(&buf, nil)))
	t.Cleanup(func() { slog.SetDefault(prev) })

	failed, n := map[string]int{}, Node{ID: "n1", Name: "alpha"}
	for _, online := range []bool{true, false, true, false, false, false, false, true, true} {
		notePresence(failed, n, online)
	}
	var levels []string
	for line := range strings.Lines(buf.String()) {
		var e map[string]any
		if err := json.Unmarshal([]byte(line), &e); err != nil {
			t.Fatal(err)
		}
		if e["node"] != "n1" || e["node_name"] != "alpha" {
			t.Errorf("entry %v isn't about the node", e)
		}
		levels = append(levels, e["level"].(string))
	}
	if strings.Join(levels, " ") != "WARN INFO" {
		t.Fatalf("logged %v, want the node going offline and coming back", levels)
	}
}
