package fileset

import (
	"path/filepath"
	"slices"
	"testing"

	"github.com/QwikByte/noryx/internal/master/database"
)

// Values and targets of servers and networks that are gone leave the sets; the others stay.
func TestLinks(t *testing.T) {
	db, err := database.Open(filepath.Join(t.TempDir(), "master.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { db.Close() })
	s := &Service{store: store{db}}
	vars := `[{"name":"motd","values":[{"kind":"server","scope":"` + server1 + `","value":"a"},{"kind":"network","scope":"` + gone + `","value":"b"},{"kind":"all","value":"c"}]}]`
	if _, err := db.Exec(`INSERT INTO file_sets (id, name, description, version, variables, created_at) VALUES ('s1', 'Motd', '', 1, ?, 0)`, []byte(vars)); err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec(`INSERT INTO file_set_versions (set_id, version, files, username, created_at) VALUES ('s1', 1, '[]', 'admin', 0)`); err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec(`INSERT INTO file_set_targets (set_id, kind, value, role) VALUES ('s1', ?, ?, ?), ('s1', ?, 'lobby', '')`, KindNetwork, gone, RoleProxy, KindTag); err != nil {
		t.Fatal(err)
	}
	kinds := func() []string {
		set, err := s.store.get(t.Context(), "s1")
		if err != nil {
			t.Fatal(err)
		}
		var got []string
		for _, v := range set.Variables[0].Values {
			got = append(got, v.Kind)
		}
		for _, target := range set.Targets {
			got = append(got, "target "+target.Kind)
		}
		return got
	}
	if err := s.Prune(t.Context()); err != nil {
		t.Fatal(err)
	}
	if got := kinds(); !slices.Equal(got, []string{KindServer, KindAll, "target " + KindTag}) {
		t.Fatalf("after pruning = %v", got)
	}
	if err := s.Forget(t.Context(), "n1", server1); err != nil {
		t.Fatal(err)
	}
	if got := kinds(); !slices.Equal(got, []string{KindAll, "target " + KindTag}) {
		t.Fatalf("after forgetting the server = %v", got)
	}
}
