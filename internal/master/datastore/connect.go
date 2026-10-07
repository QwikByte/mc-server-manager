package datastore

import (
	"context"
	"net/http"
	"slices"

	"github.com/QwikByte/noryx/internal/master/httpapi"
	"github.com/QwikByte/noryx/internal/master/overlay"
)

// Connection is where and as whom a server reaches a database of a datastore of its network,
// e.g. for the placeholders of file sets.
type Connection struct {
	Host     string
	Port     uint32
	Database string // its user has the same name
	Password string
}

// Connections tell the servers of networks how they reach the databases of their datastores.
type Connections struct {
	datastores []Datastore
	members    map[string]overlay.Member
}

// Connections returns how servers reach the databases of the datastores of their networks.
func (s *Store) Connections(ctx context.Context) (Connections, error) {
	list, err := s.list(ctx, "")
	if err != nil {
		return Connections{}, err
	}
	members, err := s.overlay.ByNode(ctx)
	return Connections{datastores: list, members: members}, err
}

// Connect returns how a server of a network on a node reaches a database of a datastore of
// the network, both given by name: on the datastore's node by the name of its container, from
// other nodes at the port it publishes in the private network of the nodes.
func (c Connections) Connect(networkID, nodeID, name, database string) (Connection, error) {
	i := slices.IndexFunc(c.datastores, func(ds Datastore) bool { return ds.NetworkID == networkID && ds.Name == name })
	if i < 0 {
		return Connection{}, httpapi.Errorf(http.StatusConflict, "The network of the server has no datastore %s.", name)
	}
	ds := c.datastores[i]
	j := slices.IndexFunc(ds.Databases, func(db Database) bool { return db.Name == database })
	if j < 0 {
		return Connection{}, httpapi.Errorf(http.StatusConflict, "The datastore %s has no database %s.", name, database)
	}
	at := endpoints(ds, c.members[ds.NodeID].Address)
	switch {
	case nodeID == ds.NodeID:
	case !reaches(c.members, nodeID, ds.NodeID):
		return Connection{}, httpapi.Errorf(http.StatusConflict, "The server can't reach the datastore %s: its node and the datastore's aren't both in the private network of the nodes.", name)
	case len(at) < 2:
		return Connection{}, httpapi.Errorf(http.StatusConflict, "The datastore %s isn't published in the private network of the nodes yet. Apply its network again.", name)
	default:
		at = at[1:]
	}
	return Connection{Host: at[0].Host, Port: at[0].Port, Database: database, Password: ds.Databases[j].password}, nil
}
