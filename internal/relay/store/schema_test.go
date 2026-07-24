package store

import (
	"path/filepath"
	"strings"
	"testing"
)

// openTestStore opens a fresh store and seeds the rows most schema tests
// need: two persons, one device each, one thread, one message with one
// delivery and its tombstone.
func openTestStore(t *testing.T) *Store {
	t.Helper()
	s, err := Open(filepath.Join(t.TempDir(), "relay.db"))
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	t.Cleanup(func() { s.Close() })

	seed := []string{
		`INSERT INTO persons VALUES ('p1', 'a@example.com', 'A', 100)`,
		`INSERT INTO persons VALUES ('p2', 'b@example.com', 'B', 100)`,
		`INSERT INTO devices VALUES ('d1', 'p1', x'01', 'laptop', 100, NULL)`,
		`INSERT INTO devices VALUES ('d2', 'p2', x'02', 'laptop', 100, NULL)`,
		`INSERT INTO threads VALUES ('t1', 'p1', 'p2', 'input-required', 100, 100)`,
		`INSERT INTO messages VALUES ('m1', 't1', 'p1', x'7b7d', 100, 101, 0)`,
		`INSERT INTO deliveries VALUES ('m1', 'd2', NULL, NULL)`,
		`INSERT INTO message_tombstones VALUES ('m1', 101)`,
	}
	for _, q := range seed {
		if _, err := s.db.Exec(q); err != nil {
			t.Fatalf("seed %q: %v", q, err)
		}
	}
	return s
}

func TestSchemaRefusesBadRows(t *testing.T) {
	s := openTestStore(t)
	cases := []struct {
		name, insert, wantErr string
	}{
		{"strict rejects mistyped column",
			`INSERT INTO persons VALUES ('p3', 'c@example.com', 'C', 'not-a-time')`,
			"cannot store TEXT value in INTEGER column"},
		{"unknown thread state refused",
			`INSERT INTO threads VALUES ('t2', 'p1', 'p2', 'Working', 100, 100)`,
			"CHECK constraint failed"},
		{"self-thread refused",
			`INSERT INTO threads VALUES ('t2', 'p1', 'p1', 'submitted', 100, 100)`,
			"CHECK constraint failed"},
		{"unknown draft state refused",
			`INSERT INTO drafts VALUES ('dr1', 't1', 'p2', x'7b7d', 'released', 0, 100, NULL)`,
			"CHECK constraint failed"},
		{"unknown grant direction refused",
			`INSERT INTO grants VALUES ('g1', 't1', 'p2', 'both', 100, NULL)`,
			"CHECK constraint failed"},
		{"orphan device refused (FK on)",
			`INSERT INTO devices VALUES ('d3', 'nobody', x'03', '', 100, NULL)`,
			"FOREIGN KEY constraint failed"},
		{"duplicate message id refused (replay key)",
			`INSERT INTO messages VALUES ('m1', 't1', 'p1', x'7b7d', 100, 102, 0)`,
			"UNIQUE constraint failed"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			_, err := s.db.Exec(c.insert)
			if err == nil {
				t.Fatal("insert succeeded; schema must refuse it")
			}
			if !strings.Contains(err.Error(), c.wantErr) {
				t.Errorf("err = %v, want it to contain %q", err, c.wantErr)
			}
		})
	}
}

func TestGrantsOneActivePerTriple(t *testing.T) {
	s := openTestStore(t)
	mustExec := func(q string) {
		t.Helper()
		if _, err := s.db.Exec(q); err != nil {
			t.Fatalf("%s: %v", q, err)
		}
	}
	mustExec(`INSERT INTO grants VALUES ('g1', 't1', 'p2', 'inbound', 100, NULL)`)

	// A second ACTIVE grant on the same triple must hit the partial index.
	if _, err := s.db.Exec(`INSERT INTO grants VALUES ('g2', 't1', 'p2', 'inbound', 101, NULL)`); err == nil {
		t.Fatal("two active grants on one thread×person×direction allowed")
	}
	// Same triple, other direction: fine.
	mustExec(`INSERT INTO grants VALUES ('g3', 't1', 'p2', 'outbound', 101, NULL)`)
	// Revoke g1 (a mark, not a delete), then re-granting the triple is legal
	// and history keeps both rows.
	mustExec(`UPDATE grants SET revoked_at = 102 WHERE id = 'g1'`)
	mustExec(`INSERT INTO grants VALUES ('g4', 't1', 'p2', 'inbound', 103, NULL)`)

	var n int
	if err := s.db.QueryRow(`SELECT count(*) FROM grants`).Scan(&n); err != nil {
		t.Fatalf("count grants: %v", err)
	}
	if n != 3 {
		t.Errorf("grant rows = %d, want 3 (g1 revoked + g3 + g4; nothing deleted)", n)
	}
}

func TestSweepDeleteCascadesDeliveriesNotTombstone(t *testing.T) {
	s := openTestStore(t)
	if _, err := s.db.Exec(`DELETE FROM messages WHERE id = 'm1'`); err != nil {
		t.Fatalf("delete message: %v", err)
	}
	var n int
	if err := s.db.QueryRow(`SELECT count(*) FROM deliveries WHERE message_id = 'm1'`).Scan(&n); err != nil {
		t.Fatalf("count deliveries: %v", err)
	}
	if n != 0 {
		t.Error("deliveries survived their message; CASCADE missing")
	}
	if err := s.db.QueryRow(`SELECT count(*) FROM message_tombstones WHERE id = 'm1'`).Scan(&n); err != nil {
		t.Fatalf("count tombstones: %v", err)
	}
	if n != 1 {
		t.Error("tombstone died with the message; replay identity must outlive the body")
	}
}
