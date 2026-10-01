package storage

import (
	"errors"
	"path/filepath"
	"testing"
)

func TestLocations(t *testing.T) {
	dataDir, ssd := t.TempDir(), filepath.Join(t.TempDir(), "ssd")
	l := New(dataDir)
	for _, tc := range []struct{ name, path string }{
		{Default, ssd},
		{"Fast SSD", ssd},
		{"inside", filepath.Join(dataDir, "servers2")},
		{"root", "/"},
	} {
		if err := l.Add(tc.name, tc.path); err == nil {
			t.Errorf("Add(%q, %q) succeeded", tc.name, tc.path)
		}
	}
	if err := l.Add("ssd", ssd); err != nil {
		t.Fatal(err)
	}
	if err := l.Add("ssd", ssd); err == nil {
		t.Fatal("added a location twice")
	}
	locations, err := l.List()
	if err != nil || len(locations) != 2 || locations[0].Name != Default || locations[1].Path != ssd || locations[1].TotalBytes == 0 {
		t.Fatalf("List() = %+v, %v", locations, err)
	}
	if path, err := l.Path("ssd"); path != ssd || err != nil {
		t.Fatalf("Path(ssd) = %q, %v", path, err)
	}
	if path, err := l.Path(""); path != filepath.Join(dataDir, "servers") || err != nil {
		t.Fatalf("Path(\"\") = %q, %v", path, err)
	}
	if err := l.Remove("ssd"); err != nil {
		t.Fatal(err)
	}
	if _, err := l.Path("ssd"); !errors.Is(err, ErrUnknown) {
		t.Fatalf("Path of removed location: %v", err)
	}
}
