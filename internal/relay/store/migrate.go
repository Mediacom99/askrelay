package store

import (
	"database/sql"
	"fmt"
	"io/fs"
	"sort"
)

// migrate applies migrations from fsys newer than the database's
// PRAGMA user_version, in filename order (0001_*.sql, 0002_*.sql, …), each
// inside its own transaction. Re-running is a no-op: idempotency comes from
// the version check, not from the SQL. fsys is a parameter (Open passes the
// embedded set) so failure paths are testable with a synthetic fs.
func migrate(db *sql.DB, fsys fs.FS) error {
	names, err := fs.Glob(fsys, "migrations/*.sql")
	if err != nil {
		return fmt.Errorf("store: list migrations: %w", err)
	}
	sort.Strings(names)

	var version int
	if err := db.QueryRow("PRAGMA user_version").Scan(&version); err != nil {
		return fmt.Errorf("store: read user_version: %w", err)
	}

	for i, name := range names {
		v := i + 1 // migration N is the file at sorted position N-1
		if v <= version {
			continue
		}
		ddl, err := fs.ReadFile(fsys, name)
		if err != nil {
			return fmt.Errorf("store: read %s: %w", name, err)
		}
		tx, err := db.Begin()
		if err != nil {
			return fmt.Errorf("store: begin migration %d: %w", v, err)
		}
		if _, err := tx.Exec(string(ddl)); err != nil {
			_ = tx.Rollback()
			return fmt.Errorf("store: apply %s: %w", name, err)
		}
		// PRAGMA cannot be parameterized; v is derived from the embedded
		// file list, never from input.
		if _, err := tx.Exec(fmt.Sprintf("PRAGMA user_version = %d", v)); err != nil {
			_ = tx.Rollback()
			return fmt.Errorf("store: bump user_version to %d: %w", v, err)
		}
		if err := tx.Commit(); err != nil {
			return fmt.Errorf("store: commit migration %d: %w", v, err)
		}
	}
	return nil
}
