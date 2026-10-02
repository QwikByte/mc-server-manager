package terminal

import (
	"slices"
	"testing"
)

func TestSplitArgs(t *testing.T) {
	for line, want := range map[string][]string{
		"":                 nil,
		"  server   list ": {"server", "list"},
		`backup create abc --label "before update"`: {"backup", "create", "abc", "--label", "before update"},
		`server command abc say 'it''s "fine"'`:     {"server", "command", "abc", "say", `its "fine"`},
		`backup create abc --label ""`:              {"backup", "create", "abc", "--label", ""},
		"server list; rm -rf / $(id)":               {"server", "list;", "rm", "-rf", "/", "$(id)"},
	} {
		got, err := splitArgs(line)
		if err != nil || !slices.Equal(got, want) {
			t.Errorf("splitArgs(%q) = %q, %v; want %q", line, got, err, want)
		}
	}
	if _, err := splitArgs(`server command abc say "hi`); err == nil {
		t.Error("unclosed quote accepted")
	}
}
