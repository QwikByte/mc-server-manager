package noryxv1

import (
	"regexp"
	"slices"
	"strings"
)

var (
	// DatabaseName matches the names of databases, which their users share. They need no
	// quoting in SQL and fit both engines.
	DatabaseName = regexp.MustCompile(`^[a-z][a-z0-9_]{0,31}$`)
	// DatabasePassword matches the passwords the master generates for the users of databases.
	DatabasePassword = regexp.MustCompile(`^[a-z2-7]{32}$`)
	// DatastoreName matches the names of the datastores of a network.
	DatastoreName = regexp.MustCompile(`^[a-z0-9][a-z0-9-]{0,31}$`)
	// TableName matches the names of the tables, schemas and columns that the agent shows,
	// which need no escaping in SQL.
	TableName = regexp.MustCompile(`^[A-Za-z0-9_$-]{1,64}$`)
)

// Limits of datastores, which master and agent both check.
const (
	MaxDatabases         = 50 // of a datastore
	MaxNetworkDatastores = 10
)

// reservedDatabases are the databases and users of the engines themselves.
var reservedDatabases = []string{"information_schema", "mysql", "performance_schema", "sys", "root", "postgres", "template0", "template1", "public"}

// DatabaseNameProblem returns why a database can't have a name, or "".
func DatabaseNameProblem(name string) string {
	switch {
	case !DatabaseName.MatchString(name):
		return "Use up to 32 lower-case letters, digits and underscores, starting with a letter."
	case slices.Contains(reservedDatabases, name) || strings.HasPrefix(name, "pg_"):
		return "The database engine uses this name itself."
	}
	return ""
}

// Slug returns the short lower-case name, e.g. "mariadb" for DATASTORE_ENGINE_MARIADB.
func (e DatastoreEngine) Slug() string { return slug(e.String(), "DATASTORE_ENGINE_") }

// Slug returns the short lower-case name, e.g. "running" for DATASTORE_STATE_RUNNING.
func (s DatastoreState) Slug() string { return slug(s.String(), "DATASTORE_STATE_") }
