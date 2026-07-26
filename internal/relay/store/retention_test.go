package store

import (
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/Mediacom99/askrelay/internal/gate"
)

var testPolicy = RetentionPolicy{
	AckGrace:     72 * time.Hour,
	HardTTL:      30 * 24 * time.Hour,
	TombstoneTTL: 24 * time.Hour,
}

func count(t *testing.T, s *Store, q string, args ...any) int {
	t.Helper()
	var n int
	if err := s.db.QueryRow(q, args...).Scan(&n); err != nil {
		t.Fatalf("count %q: %v", q, err)
	}
	return n
}

// seedInbound ingests one message and returns its id plus the recipient's
// person id and its (single) device id.
func seedInbound(t *testing.T, s *Store, now time.Time) (msgID, recipID, recipDev, threadID string) {
	t.Helper()
	sender, _ := enrollPerson(t, s, uuid.Must(uuid.NewV7()).String()+"@s.example", now)
	recip, dev := enrollPerson(t, s, uuid.Must(uuid.NewV7()).String()+"@r.example", now)
	thread := uuid.Must(uuid.NewV7()).String()
	e := mkEnvelope(thread, sender, recip, now)
	if err := s.IngestMessage(e, sender, testFresh, now); err != nil {
		t.Fatalf("IngestMessage: %v", err)
	}
	return e.ID, recip, dev, thread
}

func TestSweepMessagesAckGrace(t *testing.T) {
	s := newStore(t)
	t0 := time.Unix(1_700_000_000, 0).UTC()
	msg, _, dev, _ := seedInbound(t, s, t0)
	if err := s.Ack(msg, dev, t0); err != nil {
		t.Fatalf("Ack: %v", err)
	}

	// Before the grace elapses: survives.
	n, err := s.SweepMessages(testPolicy, t0.Add(72*time.Hour-time.Minute))
	if err != nil {
		t.Fatalf("sweep (pre-grace): %v", err)
	}
	if n != 0 || count(t, s, `SELECT count(*) FROM messages WHERE id=?`, msg) != 1 {
		t.Fatalf("swept %d before grace; message should survive", n)
	}

	// After the grace: swept.
	n, err = s.SweepMessages(testPolicy, t0.Add(72*time.Hour+time.Minute))
	if err != nil {
		t.Fatalf("sweep (post-grace): %v", err)
	}
	if n != 1 || count(t, s, `SELECT count(*) FROM messages WHERE id=?`, msg) != 0 {
		t.Errorf("swept %d after grace; message should be gone", n)
	}
}

func TestSweepMessagesHardTTLOverridesUnacked(t *testing.T) {
	s := newStore(t)
	t0 := time.Unix(1_700_000_000, 0).UTC()
	msg, _, _, _ := seedInbound(t, s, t0) // never acked

	// Long past the ack grace but unacked: the ack path does not fire.
	if n, _ := s.SweepMessages(testPolicy, t0.Add(80*time.Hour)); n != 0 {
		t.Fatalf("swept %d unacked message before hard TTL, want 0", n)
	}
	// Hard TTL deletes it regardless of acks.
	if n, _ := s.SweepMessages(testPolicy, t0.Add(30*24*time.Hour+time.Minute)); n != 1 {
		t.Errorf("swept %d at hard TTL, want 1", n)
	}
	if count(t, s, `SELECT count(*) FROM messages WHERE id=?`, msg) != 0 {
		t.Error("message survived the hard TTL")
	}
}

func TestSweepMessagesPartialAck(t *testing.T) {
	s := newStore(t)
	t0 := time.Unix(1_700_000_000, 0).UTC()

	sender, _ := enrollPerson(t, s, "s@example.com", t0)
	recip, devA := enrollPerson(t, s, "r@example.com", t0)
	tokenB, _ := s.CreateInvite("r@example.com", time.Hour, t0)
	if _, _, err := s.Enroll(tokenB, newPubkey(t), "second", t0); err != nil {
		t.Fatalf("enroll second device: %v", err)
	}
	thread := uuid.Must(uuid.NewV7()).String()
	e := mkEnvelope(thread, sender, recip, t0)
	if err := s.IngestMessage(e, sender, testFresh, t0); err != nil {
		t.Fatalf("ingest: %v", err)
	}
	// Only one of the recipient's two devices acks.
	if err := s.Ack(e.ID, devA, t0); err != nil {
		t.Fatalf("Ack devA: %v", err)
	}

	// Grace path must not fire while a device remains unacked.
	if n, _ := s.SweepMessages(testPolicy, t0.Add(72*time.Hour+time.Minute)); n != 0 {
		t.Errorf("swept %d partially-acked message on the grace path, want 0", n)
	}
	// Hard TTL still gets it.
	if n, _ := s.SweepMessages(testPolicy, t0.Add(30*24*time.Hour+time.Minute)); n != 1 {
		t.Errorf("swept %d at hard TTL, want 1", n)
	}
}

func TestSweepCascadeAndSurvivors(t *testing.T) {
	s := newStore(t)
	t0 := time.Unix(1_700_000_000, 0).UTC()
	msg, recip, dev, thread := seedInbound(t, s, t0)
	if err := s.Ack(msg, dev, t0); err != nil {
		t.Fatalf("Ack: %v", err)
	}
	if err := s.SetThreadGrant(thread, recip, gate.Inbound, t0); err != nil {
		t.Fatalf("SetThreadGrant: %v", err)
	}

	if n, err := s.SweepMessages(testPolicy, t0.Add(72*time.Hour+time.Minute)); err != nil || n != 1 {
		t.Fatalf("sweep: n=%d err=%v, want 1, nil", n, err)
	}

	if c := count(t, s, `SELECT count(*) FROM deliveries WHERE message_id=?`, msg); c != 0 {
		t.Errorf("deliveries survived the message: %d (CASCADE missing)", c)
	}
	if c := count(t, s, `SELECT count(*) FROM message_tombstones WHERE id=?`, msg); c != 1 {
		t.Error("tombstone died with the message; replay guard must outlive the body")
	}
	if c := count(t, s, `SELECT count(*) FROM threads WHERE id=?`, thread); c != 1 {
		t.Error("thread died with its message; metadata must outlive bodies")
	}
	if c := count(t, s, `SELECT count(*) FROM grants WHERE thread_id=?`, thread); c != 1 {
		t.Error("grant died with the message; grants are immortal audit rows")
	}
}

func TestSweepDrafts(t *testing.T) {
	s := newStore(t)
	t0 := time.Unix(1_700_000_000, 0).UTC()
	_, _, draft := setupDraft(t, s, t0)
	if _, err := s.ReleaseReply(draft, t0); err != nil {
		t.Fatalf("ReleaseReply: %v", err)
	}
	// A live pending_review draft alongside it must never be swept.
	_, _, live := setupDraft(t, s, t0)

	if n, err := s.SweepDrafts(testPolicy, t0.Add(30*24*time.Hour+time.Minute)); err != nil || n != 1 {
		t.Fatalf("SweepDrafts: n=%d err=%v, want 1, nil", n, err)
	}
	if count(t, s, `SELECT count(*) FROM drafts WHERE id=?`, draft) != 0 {
		t.Error("decided draft survived the hard TTL")
	}
	if count(t, s, `SELECT count(*) FROM drafts WHERE id=?`, live) != 1 {
		t.Error("pending_review draft was swept; live work must survive")
	}
}

func TestPruneTombstones(t *testing.T) {
	s := newStore(t)
	t0 := time.Unix(1_700_000_000, 0).UTC()
	msg, _, _, _ := seedInbound(t, s, t0) // tombstone.sent_at == t0

	// Younger than TombstoneTTL: survives (a replay could still be fresh).
	if n, _ := s.PruneTombstones(testPolicy, t0.Add(24*time.Hour-time.Minute)); n != 0 {
		t.Fatalf("pruned %d fresh tombstone, want 0", n)
	}
	if count(t, s, `SELECT count(*) FROM message_tombstones WHERE id=?`, msg) != 1 {
		t.Fatal("tombstone pruned too early")
	}
	// Past TombstoneTTL: a replay would be stale anyway, so prune.
	if n, _ := s.PruneTombstones(testPolicy, t0.Add(24*time.Hour+time.Minute)); n != 1 {
		t.Errorf("pruned %d stale tombstone, want 1", n)
	}
}
