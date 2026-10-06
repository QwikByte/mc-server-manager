package runtime

import (
	"context"
	"errors"
	"io"

	noryxv1 "github.com/QwikByte/noryx/api/noryx/v1"
)

var (
	// ErrDatastoreNotRunning is returned for what needs a datastore that runs and is ready.
	ErrDatastoreNotRunning = errors.New("the datastore isn't running")
	// ErrNoTable is returned for a table that a database lacks.
	ErrNoTable = errors.New("table not found")
)

// DatastoreSpec describes a datastore: a MariaDB or PostgreSQL server of a network.
type DatastoreSpec struct {
	ID        string                  `json:"id"`
	Engine    noryxv1.DatastoreEngine `json:"engine"`
	Version   string                  `json:"version"`
	MemoryMB  uint32                  `json:"memoryMb"`
	CPUMillis uint32                  `json:"cpuMillis,omitempty"`
	Storage   string                  `json:"storage,omitempty"`
	// Port is published at Overlay, the node's address in the private network of the nodes,
	// for other nodes; 0 for none.
	Port    uint32 `json:"port,omitempty"`
	Overlay string `json:"overlay,omitempty"`
	// Previous is the version whose data a change of the major version left.
	Previous string `json:"previous,omitempty"`
}

// Datastore is a datastore managed by a runtime.
type Datastore struct {
	DatastoreSpec
	State noryxv1.DatastoreState
	// Size is the size of its data in bytes.
	Size int64
}

// Datastores runs datastores. Each database has a user of the same name with rights on it
// alone, and the superuser's password never leaves the runtime.
type Datastores interface {
	// DatastoreVersions returns the major versions of each engine, oldest first.
	DatastoreVersions() map[noryxv1.DatastoreEngine][]string
	ListDatastores(ctx context.Context) ([]Datastore, error)
	// CreateDatastore creates a stopped datastore on new data.
	CreateDatastore(ctx context.Context, spec DatastoreSpec) error
	StartDatastore(ctx context.Context, id string) error
	StopDatastore(ctx context.Context, id string) error
	// UpdateDatastore creates the container of a datastore again with spec, e.g. other limits
	// or another port, on the data of its version; with pull, with its image pulled again. A
	// running datastore starts again. Another version changes to it: the databases are dumped
	// and loaded into a datastore on new data, with their users and passwords, and the old
	// data stays as Previous. A spec without Previous removes the data of a previous version.
	UpdateDatastore(ctx context.Context, spec DatastoreSpec, pull bool) error
	// RemoveDatastore removes a datastore with its container, network and data.
	RemoveDatastore(ctx context.Context, id string) error
	// Databases returns the databases of a running datastore, sorted.
	Databases(ctx context.Context, id string) ([]string, error)
	// EnsureDatabase creates a database with its user, or sets the user's password.
	EnsureDatabase(ctx context.Context, id, name, password string) error
	DropDatabase(ctx context.Context, id, name string) error
	// Dump writes an SQL dump of a database to w, without its owner and privileges.
	Dump(ctx context.Context, id, name string, w io.Writer) error
	// Load creates a database anew and loads a dump into it as the database's user, so that
	// the dump gets no more rights than the user has. The user keeps its password.
	Load(ctx context.Context, id, name string, r io.Reader) error
	// Tables returns the tables of a database, sorted.
	Tables(ctx context.Context, id, name string) ([]*noryxv1.Table, error)
	// Browse returns the columns of a table and at most limit of its rows from offset on, in
	// the order of its primary key if it has one, as text with long values cut short. It only
	// reads, with names that noryxv1.TableName matches.
	Browse(ctx context.Context, id, name, schema, table string, offset uint64, limit uint32) (*noryxv1.BrowseTableResponse, error)
}
