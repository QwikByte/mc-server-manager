package datastore

import (
	"testing"

	"github.com/QwikByte/noryx/internal/master/overlay"
)

func TestConnect(t *testing.T) {
	ds := Datastore{ID: "d1", NetworkID: "n1", NodeID: "a", Name: "main", Engine: "postgres", Databases: []Database{{Name: "lp", password: "pw"}}}
	c := Connections{
		datastores: []Datastore{ds, {ID: "d2", NetworkID: "n2", NodeID: "a", Name: "other", Engine: "mariadb"}},
		members:    map[string]overlay.Member{"a": {Address: "10.0.0.1"}, "b": {Address: "10.0.0.2"}},
	}
	// On its node by the name of its container, which the network's servers join.
	if got, err := c.Connect("n1", "a", "main", "lp"); err != nil || got != (Connection{Host: "noryx-db-d1", Port: 5432, Database: "lp", Password: "pw"}) {
		t.Errorf("on its node: %+v, %v", got, err)
	}
	// From another node only once it is published in the private network.
	if _, err := c.Connect("n1", "b", "main", "lp"); err == nil {
		t.Error("reached before it was published")
	}
	c.datastores[0].Port = 23306
	if got, err := c.Connect("n1", "b", "main", "lp"); err != nil || got.Host != "10.0.0.1" || got.Port != 23306 {
		t.Errorf("from another node: %+v, %v", got, err)
	}
	for name, try := range map[string][4]string{
		"outside the private network": {"n1", "c", "main", "lp"},
		"of another network":          {"n1", "a", "other", "lp"},
		"unknown database":            {"n1", "a", "main", "nope"},
		"without network":             {"", "a", "main", "lp"},
	} {
		if _, err := c.Connect(try[0], try[1], try[2], try[3]); err == nil {
			t.Errorf("%s: reached", name)
		}
	}
}
