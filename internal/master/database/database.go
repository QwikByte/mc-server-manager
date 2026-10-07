// Package database opens the SQLite database of the master and keeps its schema up to date.
package database

import (
	"database/sql"
	"embed"
	"errors"
	"fmt"
	"io/fs"

	_ "modernc.org/sqlite" // pure Go driver, keeps the master a static binary
)

//go:embed migrations/*.sql
var migrations embed.FS

// Open opens the database at path and applies all pending migrations. The space of deleted
// rows is reused, and the write-ahead log shrinks back to 16 MiB after a large transaction,
// such as deleting many log entries.
func Open(path string) (*sql.DB, error) {
	db, err := sql.Open("sqlite", "file:"+path+"?_pragma=foreign_keys(1)&_pragma=journal_mode(WAL)&_pragma=busy_timeout(5000)&_pragma=journal_size_limit(16777216)")
	if err != nil {
		return nil, err
	}
	if err := migrate(db); err != nil {
		db.Close()
		return nil, fmt.Errorf("migrate database: %w", err)
	}
	return db, nil
}

// migrate runs every migration newer than the schema version stored in user_version.
func migrate(db *sql.DB) error {
	files, err := fs.Glob(migrations, "migrations/*.sql")
	if err != nil {
		return err
	}
	var version int
	if err := db.QueryRow("PRAGMA user_version").Scan(&version); err != nil {
		return err
	}
	if version > len(files) {
		return fmt.Errorf("schema version %d is newer than this binary", version)
	}
	for ; version < len(files); version++ {
		script, err := migrations.ReadFile(files[version])
		if err != nil {
			return err
		}
		tx, err := db.Begin()
		if err != nil {
			return err
		}
		if _, err := tx.Exec(fmt.Sprintf("%s;\nPRAGMA user_version = %d", script, version+1)); err != nil {
			return errors.Join(fmt.Errorf("%s: %w", files[version], err), tx.Rollback())
		}
		if err := tx.Commit(); err != nil {
			return err
		}
	}
	return nil
}
