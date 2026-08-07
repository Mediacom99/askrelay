package store

import (
	"database/sql"
	"path/filepath"
	"testing"
	"testing/fstest"
)

func TestOpenAppliesPragmasAndMigrations(t *testing.T) {
	path := filepath.Join(t.TempDir(), "relay.db")
	s, err := Open(path)
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	defer s.Close()

	var journal string
	if err := s.db.QueryRow("PRAGMA journal_mode").Scan(&journal); err != nil {
		t.Fatalf("read journal_mode: %v", err)
	}
	if journal != "wal" {
		t.Errorf("journal_mode = %q, want %q", journal, "wal")
	}
	var fk int
	if err := s.db.QueryRow("PRAGMA foreign_keys").Scan(&fk); err != nil {
		t.Fatalf("read foreign_keys: %v", err)
	}
	if fk != 1 {
		t.Errorf("foreign_keys = %d, want 1", fk)
	}
	var version int
	if err := s.db.QueryRow("PRAGMA user_version").Scan(&version); err != nil {
		t.Fatalf("read user_version: %v", err)
	}
	if version != 2 {
		t.Errorf("user_version = %d, want 2 (two embedded migrations)", version)
	}
}

func TestOpenIsIdempotent(t *testing.T) {
	path := filepath.Join(t.TempDir(), "relay.db")
	for i := range 2 {
		s, err := Open(path)
		if err != nil {
			t.Fatalf("Open #%d: %v", i+1, err)
		}
		if err := s.Close(); err != nil {
			t.Fatalf("Close #%d: %v", i+1, err)
		}
	}
}

func TestBrokenMigrationRollsBack(t *testing.T) {
	db, err := sql.Open("sqlite", "file:"+filepath.Join(t.TempDir(), "relay.db"))
	if err != nil {
		t.Fatalf("open raw db: %v", err)
	}
	defer db.Close()

	fsys := fstest.MapFS{
		"migrations/0001_good.sql": {Data: []byte("CREATE TABLE ok (id INTEGER PRIMARY KEY) STRICT;")},
		"migrations/0002_bad.sql":  {Data: []byte("CREATE TABLE broken (id INTEGER PRIMARY KEY) STRICT; THIS IS NOT SQL;")},
	}
	if err := migrate(db, fsys); err == nil {
		t.Fatal("migrate with broken SQL must fail")
	}
	var version int
	if err := db.QueryRow("PRAGMA user_version").Scan(&version); err != nil {
		t.Fatalf("read user_version: %v", err)
	}
	if version != 1 {
		t.Errorf("user_version = %d, want 1 (bad migration must not be stamped)", version)
	}
	// The failed migration's partial DDL must not survive its rollback.
	var n int
	err = db.QueryRow("SELECT count(*) FROM sqlite_schema WHERE name = 'broken'").Scan(&n)
	if err != nil {
		t.Fatalf("inspect schema: %v", err)
	}
	if n != 0 {
		t.Error("table from failed migration survived the rollback")
	}
}
