package store

import (
	"crypto/ed25519"
	"database/sql"
	"errors"
	"path/filepath"
	"testing"
	"testing/fstest"
	"time"

	"github.com/google/uuid"

	"github.com/Mediacom99/askrelay/internal/gate"
)

// Regression tests for the WP-03 three-agent quality pass (Fixes 1–7). Each
// pins a finding the pass surfaced so it cannot silently return.

// Fix 1 (A) — cross-thread / cross-tenant injection.
func TestIngestRejectsNonParticipant(t *testing.T) {
	s := newStore(t)
	now := time.Unix(1_700_000_000, 0).UTC()
	a, _ := enrollPerson(t, s, "a@example.com", now)
	b, _ := enrollPerson(t, s, "b@example.com", now)
	c, _ := enrollPerson(t, s, "c@example.com", now)
	thread := uuid.Must(uuid.NewV7()).String()

	// A opens the thread to B.
	if err := s.IngestMessage(mkEnvelope(thread, a, b, now), a, testFresh, now); err != nil {
		t.Fatalf("seed thread: %v", err)
	}
	// Unrelated C tries to inject into A↔B's thread — refused.
	inject := mkEnvelope(thread, c, b, now)
	if err := s.IngestMessage(inject, c, testFresh, now); !errors.Is(err, ErrNotParticipant) {
		t.Errorf("C injecting into A/B thread: err = %v, want ErrNotParticipant", err)
	}
	// A legitimate reply B→A on the existing thread still works.
	if err := s.IngestMessage(mkEnvelope(thread, b, a, now), b, testFresh, now); err != nil {
		t.Errorf("legit reply B->A refused: %v", err)
	}
}

func TestCreateDraftRejectsNonParticipant(t *testing.T) {
	s := newStore(t)
	now := time.Unix(1_700_000_000, 0).UTC()
	a, _ := enrollPerson(t, s, "a@example.com", now)
	b, _ := enrollPerson(t, s, "b@example.com", now)
	c, _ := enrollPerson(t, s, "c@example.com", now)
	thread := uuid.Must(uuid.NewV7()).String()
	if err := s.IngestMessage(mkEnvelope(thread, a, b, now), a, testFresh, now); err != nil {
		t.Fatalf("seed thread: %v", err)
	}

	reply := mkEnvelope(thread, c, a, now)
	if _, err := s.CreateDraft(thread, c, reply, now); !errors.Is(err, ErrNotParticipant) {
		t.Errorf("C authoring on A/B thread: err = %v, want ErrNotParticipant", err)
	}
	// A real participant can author.
	if _, err := s.CreateDraft(thread, b, mkEnvelope(thread, b, a, now), now); err != nil {
		t.Errorf("participant B refused: %v", err)
	}
}

// Fix 2 (B) — a swept-but-still-fresh message cannot be replayed, because the
// tombstone is pruned only at the freshness horizon (PruneTombstones takes the
// same Freshness the ingest path uses — no separate knob to misconfigure).
func TestReplayRefusedWhileFresh(t *testing.T) {
	s := newStore(t)
	t0 := time.Unix(1_700_000_000, 0).UTC()
	a, _ := enrollPerson(t, s, "a@example.com", t0)
	b, _ := enrollPerson(t, s, "b@example.com", t0)
	thread := uuid.Must(uuid.NewV7()).String()
	e := mkEnvelope(thread, a, b, t0)
	if err := s.IngestMessage(e, a, testFresh, t0); err != nil {
		t.Fatalf("ingest: %v", err)
	}
	// Simulate the body being swept while the message is still within the
	// freshness window (testFresh.MaxAge == 24h).
	if _, err := s.db.Exec(`DELETE FROM messages WHERE id=?`, e.ID); err != nil {
		t.Fatalf("delete body: %v", err)
	}
	now := t0.Add(time.Hour)
	if n, _ := s.PruneTombstones(testFresh, now); n != 0 {
		t.Fatalf("pruned %d tombstone inside the freshness window", n)
	}
	// A replay is still refused by the surviving tombstone.
	if err := s.IngestMessage(e, a, testFresh, now); !errors.Is(err, ErrReplay) {
		t.Errorf("replay of swept-but-fresh message: err = %v, want ErrReplay", err)
	}
}

// Fix 3 (C) — ApproveInboundViaGrant marks via_grant only on a message that
// actually belongs to the granted thread.
func TestApproveInboundViaGrantScopedToThread(t *testing.T) {
	s := newStore(t)
	now := time.Unix(1_700_000_000, 0).UTC()
	// Thread 1 (granted) and thread 2 (its own message), same recipient.
	a, _ := enrollPerson(t, s, "a@example.com", now)
	b, _ := enrollPerson(t, s, "b@example.com", now)
	t1 := uuid.Must(uuid.NewV7()).String()
	t2 := uuid.Must(uuid.NewV7()).String()
	e1 := mkEnvelope(t1, a, b, now)
	e2 := mkEnvelope(t2, a, b, now)
	if err := s.IngestMessage(e1, a, testFresh, now); err != nil {
		t.Fatalf("ingest t1: %v", err)
	}
	if err := s.IngestMessage(e2, a, testFresh, now); err != nil {
		t.Fatalf("ingest t2: %v", err)
	}
	if err := s.SetThreadGrant(t1, b, gate.Inbound, now); err != nil {
		t.Fatalf("grant: %v", err)
	}
	// Caller mix-up: grant is on t1 but the messageID belongs to t2.
	if _, err := s.ApproveInboundViaGrant(t1, b, e2.ID, now); err != nil {
		t.Fatalf("ApproveInboundViaGrant: %v", err)
	}
	// t2's message must NOT have been marked via_grant.
	var vg int
	if err := s.db.QueryRow(`SELECT via_grant FROM messages WHERE id=?`, e2.ID).Scan(&vg); err != nil {
		t.Fatalf("read via_grant: %v", err)
	}
	if vg != 0 {
		t.Error("message from an ungranted thread was falsely marked via_grant")
	}
}

// Fix 4 (D) — migrate refuses a database whose user_version is ahead of the
// embedded set.
func TestMigrateRefusesFutureVersion(t *testing.T) {
	db, err := sql.Open("sqlite", "file:"+filepath.Join(t.TempDir(), "relay.db"))
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	defer db.Close()
	if _, err := db.Exec(`PRAGMA user_version = 5`); err != nil {
		t.Fatalf("set version: %v", err)
	}
	fsys := fstest.MapFS{"migrations/0001_init.sql": {Data: []byte("CREATE TABLE t (id INTEGER PRIMARY KEY) STRICT;")}}
	if err := migrate(db, fsys); !errors.Is(err, ErrSchemaTooNew) {
		t.Errorf("migrate against future schema: err = %v, want ErrSchemaTooNew", err)
	}
}

// Fix 7 (G) — migrate refuses a gapped/non-contiguous migration set.
func TestMigrateRejectsNonContiguous(t *testing.T) {
	db, err := sql.Open("sqlite", "file:"+filepath.Join(t.TempDir(), "relay.db"))
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	defer db.Close()
	fsys := fstest.MapFS{
		"migrations/0001_a.sql": {Data: []byte("CREATE TABLE a (id INTEGER PRIMARY KEY) STRICT;")},
		"migrations/0003_b.sql": {Data: []byte("CREATE TABLE b (id INTEGER PRIMARY KEY) STRICT;")}, // 0002 missing
	}
	if err := migrate(db, fsys); err == nil {
		t.Error("migrate accepted a gapped migration set (0002 missing)")
	}
}

// Fix 5 (E) — Enroll refuses a wrong-length pubkey at the boundary.
func TestEnrollRejectsBadPubkeyLength(t *testing.T) {
	s := newStore(t)
	now := time.Unix(1_700_000_000, 0).UTC()
	token, _ := s.CreateInvite("x@example.com", time.Hour, now)
	if _, _, err := s.Enroll(token, ed25519.PublicKey(make([]byte, 16)), "dev", now); err == nil {
		t.Error("Enroll accepted a 16-byte pubkey")
	}
}

// Fix 6 (F) — a duplicate pubkey returns the ErrKeyInUse sentinel, not raw SQL.
func TestEnrollDuplicateKey(t *testing.T) {
	s := newStore(t)
	now := time.Unix(1_700_000_000, 0).UTC()
	pub := newPubkey(t)
	t1, _ := s.CreateInvite("a@example.com", time.Hour, now)
	if _, _, err := s.Enroll(t1, pub, "first", now); err != nil {
		t.Fatalf("first enroll: %v", err)
	}
	t2, _ := s.CreateInvite("b@example.com", time.Hour, now)
	if _, _, err := s.Enroll(t2, pub, "second", now); !errors.Is(err, ErrKeyInUse) {
		t.Errorf("duplicate key: err = %v, want ErrKeyInUse", err)
	}
}
