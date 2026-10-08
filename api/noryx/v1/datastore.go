package noryxv1

import (
	"fmt"
	"regexp"
	"slices"
	"strings"
	"unicode/utf8"
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
	// MaxImportedDump is the most bytes of an uploaded dump.
	MaxImportedDump = 16 << 30
	// MaxFilterValue is the most bytes of the value that a filter of a table compares.
	MaxFilterValue = 1 << 10
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

// Problem returns why a filter of a table can't be used, or "". The agent puts the value into
// statements in hexadecimal, so it needs no escaping, but it must be text that both engines
// take: UTF-8 without NUL.
func (f *TableFilter) Problem() string {
	switch v := f.GetValue(); {
	case !TableName.MatchString(f.GetColumn()):
		return "Choose a column to filter by."
	case len(v) > MaxFilterValue:
		return fmt.Sprintf("Filter by up to %d bytes.", MaxFilterValue)
	case !utf8.ValidString(v) || strings.ContainsRune(v, 0):
		return "Filter by text without NUL characters."
	}
	return ""
}

// Slug returns the short lower-case name, e.g. "mariadb" for DATASTORE_ENGINE_MARIADB.
func (e DatastoreEngine) Slug() string { return slug(e.String(), "DATASTORE_ENGINE_") }

// Slug returns the short lower-case name, e.g. "running" for DATASTORE_STATE_RUNNING.
func (s DatastoreState) Slug() string { return slug(s.String(), "DATASTORE_STATE_") }
