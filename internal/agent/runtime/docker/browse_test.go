package docker

import (
	"context"
	"errors"
	"slices"
	"strings"
	"testing"

	noryxv1 "github.com/QwikByte/noryx/api/noryx/v1"
	"github.com/QwikByte/noryx/internal/agent/runtime"
)

func TestEveryDatastoreEngineHasABrowser(t *testing.T) {
	for engine := range engines {
		if _, ok := browsers[engine]; !ok {
			t.Errorf("%s has no browser", engine)
		}
	}
}

// TestBrowseLive browses the tables of both engines in Docker: in the order of the primary
// key, with long values cut short, binary values in hexadecimal, and a session that only
// reads. It only runs with NORYX_DOCKER_TEST set, like TestDatastoresLive.
func TestBrowseLive(t *testing.T) {
	d := live(t)
	tests := map[noryxv1.DatastoreEngine]struct{ blob, bytes, hex string }{
		noryxv1.DatastoreEngine_DATASTORE_ENGINE_MARIADB:  {"BLOB", "X'00FF'", "0x00FF"},
		noryxv1.DatastoreEngine_DATASTORE_ENGINE_POSTGRES: {"bytea", `'\x00ff'`, `\x00ff`},
	}
	for engine, tc := range tests {
		t.Run(engine.Slug(), func(t *testing.T) {
			ctx := t.Context()
			versions := engines[engine].versions
			spec := runtime.DatastoreSpec{ID: runtime.NewID(), Engine: engine, Version: versions[len(versions)-1], MemoryMB: 512}
			must(t, d.CreateDatastore(ctx, spec))
			t.Cleanup(func() { must(t, d.RemoveDatastore(context.WithoutCancel(ctx), spec.ID)) })
			must(t, d.StartDatastore(ctx, spec.ID))
			must(t, d.waitReady(ctx, spec.ID))
			const password = "abcdefghijklmnopqrstuvwxyz234567"
			must(t, d.EnsureDatabase(ctx, spec.ID, "shop", password))
			long := strings.Repeat("ä", maxValue+1)
			asUser(t, d, spec.ID, password, "CREATE TABLE items (v INT); CREATE TABLE notes (id INT PRIMARY KEY, body TEXT, raw "+tc.blob+");"+
				" INSERT INTO notes VALUES (2, NULL, "+tc.bytes+"), (1, 'line\none \"quoted\" "+long+"', NULL);")

			tables, err := d.Tables(ctx, spec.ID, "shop")
			must(t, err)
			if len(tables) != 2 || tables[0].GetName() != "items" || tables[1].GetName() != "notes" || tables[1].GetSize() <= 0 {
				t.Fatalf("tables %v", tables)
			}

			first, err := d.Browse(ctx, spec.ID, "shop", tables[1].GetSchema(), "notes", 0, 1)
			must(t, err)
			names := []string{}
			for _, c := range first.GetColumns() {
				names = append(names, c.GetName())
			}
			if !slices.Equal(names, []string{"id", "body", "raw"}) || !first.GetColumns()[0].GetPrimaryKey() || first.GetColumns()[1].GetPrimaryKey() {
				t.Fatalf("columns %v", first.GetColumns())
			}
			if len(first.GetRows()) != 1 || !first.GetMore() {
				t.Fatalf("first page %v", first)
			}
			row := first.GetRows()[0].GetValues()
			body := []rune(row[1].GetText())
			if row[0].GetText() != "1" || !row[1].GetTruncated() || len(body) != maxValue || !strings.HasPrefix(string(body), "line\none \"quoted\" ä") || !row[2].GetNull() {
				t.Errorf("first row %v", row)
			}
			second, err := d.Browse(ctx, spec.ID, "shop", tables[1].GetSchema(), "notes", 1, 1)
			must(t, err)
			if row := second.GetRows()[0].GetValues(); second.GetMore() || row[0].GetText() != "2" || !row[1].GetNull() || row[2].GetText() != tc.hex {
				t.Errorf("second page %v", second)
			}

			for _, table := range []string{"missing", "notes; DROP TABLE notes", "`notes`"} {
				if _, err := d.Browse(ctx, spec.ID, "shop", "", table, 0, 1); !errors.Is(err, runtime.ErrNoTable) {
					t.Errorf("browsing %q: %v", table, err)
				}
			}
			write := func(browser) string { return "INSERT INTO items VALUES (1);" }
			if err := d.browse(ctx, spec.ID, "shop", write, func([]byte) error { return nil }); err == nil {
				t.Error("a browsing session wrote")
			}
		})
	}
}
