package store

import (
	"database/sql"
	"embed"
	"fmt"
	"sync"

	_ "modernc.org/sqlite" // registers the pure-Go "sqlite" driver (T-03)
)

//go:embed migrations/*.sql
var migrationFS embed.FS

// Store is the single owner of the relay's SQLite database. All writes go
// through its methods, serialized by mu (T-03 single-writer discipline: WAL
// allows concurrent readers, but one writer at a time avoids SQLITE_BUSY
// churn under modernc's driver). Every write method routes through writeTx.
type Store struct {
	db *sql.DB
	mu sync.Mutex // held for the duration of every write transaction
}

// Open opens (creating if absent) the database at path, applies the pragmas
// every connection needs, and runs any pending migrations. The pragmas ride
// the DSN because database/sql pools connections — an Exec'd PRAGMA would
// apply to one pooled connection, not all of them.
func Open(path string) (*Store, error) {
	dsn := "file:" + path + "?_pragma=journal_mode(WAL)" +
		"&_pragma=busy_timeout(5000)" +
		"&_pragma=foreign_keys(1)" +
		"&_pragma=synchronous(NORMAL)"
	db, err := sql.Open("sqlite", dsn)
	if err != nil {
		return nil, fmt.Errorf("store: open %q: %w", path, err)
	}
	if err := migrate(db, migrationFS); err != nil {
		db.Close()
		return nil, err
	}
	return &Store{db: db}, nil
}

// Close releases the underlying database handle.
func (s *Store) Close() error {
	if err := s.db.Close(); err != nil {
		return fmt.Errorf("store: close: %w", err)
	}
	return nil
}
