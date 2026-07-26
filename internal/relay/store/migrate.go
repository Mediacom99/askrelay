package store

import (
	"database/sql"
	"errors"
	"fmt"
	"io/fs"
	"path"
	"sort"
	"strconv"
	"strings"
)

// ErrSchemaTooNew means the database's user_version is ahead of the embedded
// migration set — an older binary opening a database a newer binary already
// migrated (a self-host rollback). The store refuses rather than run against a
// schema it does not fully understand.
var ErrSchemaTooNew = errors.New("store: database schema is newer than this binary")

// migrate applies migrations from fsys newer than the database's
// PRAGMA user_version, each inside its own transaction. Re-running is a no-op:
// idempotency comes from the version check, not from the SQL. fsys is a
// parameter (Open passes the embedded set) so failure paths are testable with
// a synthetic fs. Each migration's version is parsed from its NNNN_ filename
// prefix and asserted contiguous from 1, so a deleted or gapped file fails
// loudly rather than silently renumbering later migrations.
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
	if version > len(names) {
		return fmt.Errorf("store: user_version %d exceeds %d embedded migrations: %w",
			version, len(names), ErrSchemaTooNew)
	}

	for i, name := range names {
		v, err := migrationVersion(name)
		if err != nil {
			return err
		}
		if v != i+1 {
			return fmt.Errorf("store: migration %q is version %d, expected %d (non-contiguous)", name, v, i+1)
		}
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

// migrationVersion parses the leading NNNN of a migration filename
// (migrations/0001_init.sql → 1). The version is authoritative — never the
// file's position in the list.
func migrationVersion(name string) (int, error) {
	prefix, _, ok := strings.Cut(path.Base(name), "_")
	if !ok {
		return 0, fmt.Errorf("store: migration %q has no NNNN_ prefix", name)
	}
	v, err := strconv.Atoi(prefix)
	if err != nil || v < 1 {
		return 0, fmt.Errorf("store: migration %q has a bad version prefix %q", name, prefix)
	}
	return v, nil
}
