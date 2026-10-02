package access

import (
	"encoding/json"
	"os"
	"regexp"
	"slices"
	"strings"
	"testing"
)

func group(perms []Permission, targets ...Target) Grants {
	return Group{Permissions: perms, AllServers: len(targets) == 0, Targets: targets}.grants()
}

func TestGrantsScope(t *testing.T) {
	lobby, survival := Target{"n1", "lobby"}, Target{"n1", "survival"}
	var g Grants
	g.merge(group([]Permission{ServersRestart, NetworksView}, lobby))
	g.merge(group([]Permission{ServersStart}, Target{NodeID: "n2"}))

	for _, tc := range []struct {
		name string
		got  bool
		want bool
	}{
		{"restart lobby", g.On(ServersRestart, "n1", "lobby"), true},
		{"restart survival", g.On(ServersRestart, "n1", "survival"), false},
		{"restart on the whole node", g.On(ServersRestart, "n1", ""), false},
		{"restart somewhere on n1", g.Somewhere(ServersRestart, "n1"), true},
		{"restart somewhere on n2", g.Somewhere(ServersRestart, "n2"), false},
		{"restart everywhere", g.Has(ServersRestart), false},
		{"start any server of n2", g.On(ServersStart, "n2", "later"), true},
		{"global permissions ignore the scope", g.Has(NetworksView), true},
		{"other permissions", g.Somewhere(ServersDelete, ""), false},
		{"administrators", Admin().On(ServersDelete, "n9", "x"), true},
	} {
		if tc.got != tc.want {
			t.Errorf("%s: got %v", tc.name, tc.got)
		}
	}

	// Covering: a scope covers its parts, a node covers its servers, nothing covers admins.
	for _, tc := range []struct {
		name  string
		actor Grants
		other Grants
		want  bool
	}{
		{"same", g, g, true},
		{"part", g, group([]Permission{ServersRestart}, lobby), true},
		{"other server", g, group([]Permission{ServersRestart}, survival), false},
		{"wider scope", g, group([]Permission{ServersRestart}), false},
		{"server of a node", g, group([]Permission{ServersStart}, Target{"n2", "x"}), true},
		{"other permission", g, group([]Permission{ServersDelete}, lobby), false},
		{"admins", g, Admin(), false},
		{"admin covers all", Admin(), g, true},
		{"nothing", Grants{}, Grants{}, true},
	} {
		if got := tc.actor.Covers(tc.other); got != tc.want {
			t.Errorf("covers %s: got %v", tc.name, got)
		}
	}
}

func TestRequiredPermissions(t *testing.T) {
	got := withRequired([]Permission{ConsoleCommands, FilesWrite, ConsoleCommands})
	want := []Permission{ConsoleCommands, ConsoleView, FilesRead, FilesWrite, ServersView}
	slices.Sort(want)
	if !slices.Equal(got, want) {
		t.Fatalf("withRequired = %v, want %v", got, want)
	}
	seen := map[Permission]bool{}
	for _, a := range Catalog {
		for _, p := range a.Permissions {
			if seen[p.ID] {
				t.Errorf("%s is listed twice", p.ID)
			}
			seen[p.ID] = true
			for _, r := range p.Requires {
				if info, ok := lookup(r); !ok || p.Scoped != info.Scoped && info.Scoped {
					t.Errorf("%s requires %s, which is unknown or scoped while it isn't", p.ID, r)
				}
			}
		}
	}
}

// The panel lists the permissions to check them before offering actions; both lists agree.
func TestPanelKnowsEveryPermission(t *testing.T) {
	source, err := os.ReadFile("../../../web/src/features/access/permissions.ts")
	if err != nil {
		t.Fatal(err)
	}
	var panel, catalog []string
	for _, m := range regexp.MustCompile(`"([a-z]+\.[a-z]+)"`).FindAllStringSubmatch(string(source), -1) {
		panel = append(panel, m[1])
	}
	for _, a := range Catalog {
		for _, p := range a.Permissions {
			catalog = append(catalog, string(p.ID))
		}
	}
	slices.Sort(panel)
	slices.Sort(catalog)
	if !slices.Equal(panel, catalog) {
		t.Fatalf("panel %v\ncatalog %v", panel, catalog)
	}
}

func TestGrantsViewHasNoNullLists(t *testing.T) {
	data, err := json.Marshal(group([]Permission{ServersRestart}, Target{"n1", "lobby"}).view())
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(data), "null") {
		t.Fatalf("view = %s", data)
	}
}
