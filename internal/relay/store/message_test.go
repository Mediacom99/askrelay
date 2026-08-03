package store

import (
	"crypto/ed25519"
	"crypto/rand"
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/Mediacom99/askrelay/internal/a2a"
	"github.com/Mediacom99/askrelay/internal/envelope"
)

var testFresh = Freshness{MaxAge: 24 * time.Hour, MaxSkew: 5 * time.Minute}

// enrollPerson creates a person with one enrolled device, returning both ids.
func enrollPerson(t *testing.T, s *Store, email string, now time.Time) (personID, deviceID string) {
	t.Helper()
	pub, _, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatalf("keygen: %v", err)
	}
	token, err := s.CreateInvite(email, time.Hour, now)
	if err != nil {
		t.Fatalf("CreateInvite(%s): %v", email, err)
	}
	p, d, err := s.Enroll(token, pub, "dev", now)
	if err != nil {
		t.Fatalf("Enroll(%s): %v", email, err)
	}
	return p.ID, d.ID
}

// mkEnvelope builds an inbound envelope. State is deliberately NOT
// input-required so tests can prove the thread state comes from the action,
// not from e.State (F3).
func mkEnvelope(threadID, senderID, recipientID string, sentAt time.Time) envelope.Envelope {
	return envelope.Envelope{
		V:      1,
		ID:     uuid.Must(uuid.NewV7()).String(),
		Thread: threadID,
		From:   envelope.Party{Person: senderID, Device: "d", Agent: "cc"},
		To:     recipientID,
		State:  a2a.StateCompleted,
		SentAt: sentAt,
		Body:   envelope.Message{Role: "user", Parts: []envelope.Part{{Type: "text", Text: "hello"}}},
	}
}

func TestIngestAndInbox(t *testing.T) {
	s := newStore(t)
	now := time.Unix(1_700_000_000, 0).UTC()
	sender, _ := enrollPerson(t, s, "sender@example.com", now)
	recip, recipDev := enrollPerson(t, s, "recip@example.com", now)

	thread := uuid.Must(uuid.NewV7()).String()
	e := mkEnvelope(thread, sender, recip, now)
	if err := s.IngestMessage(e, sender, testFresh, now); err != nil {
		t.Fatalf("IngestMessage: %v", err)
	}

	inbox, err := s.Inbox(recipDev)
	if err != nil {
		t.Fatalf("Inbox: %v", err)
	}
	if len(inbox) != 1 || inbox[0].ID != e.ID {
		t.Fatalf("inbox = %+v, want one item with id %s", inbox, e.ID)
	}

	// F3: thread state comes from the action (input-required), never e.State.
	var st string
	if err := s.db.QueryRow(`SELECT state FROM threads WHERE id = ?`, thread).Scan(&st); err != nil {
		t.Fatalf("read thread state: %v", err)
	}
	if st != string(a2a.StateInputRequired) {
		t.Errorf("thread state = %q, want input-required (e.State was %q)", st, e.State)
	}

	// The stored blob round-trips through Decode.
	got, err := envelope.Decode(inbox[0].Envelope)
	if err != nil {
		t.Fatalf("Decode stored blob: %v", err)
	}
	if got.ID != e.ID || got.Body.Parts[0].Text != "hello" {
		t.Errorf("decoded blob = %+v, want id %s and body 'hello'", got, e.ID)
	}
}

func TestIngestReplay(t *testing.T) {
	s := newStore(t)
	now := time.Unix(1_700_000_000, 0).UTC()
	sender, _ := enrollPerson(t, s, "s@example.com", now)
	recip, _ := enrollPerson(t, s, "r@example.com", now)
	thread := uuid.Must(uuid.NewV7()).String()

	e := mkEnvelope(thread, sender, recip, now)
	if err := s.IngestMessage(e, sender, testFresh, now); err != nil {
		t.Fatalf("first ingest: %v", err)
	}
	// Same id again.
	if err := s.IngestMessage(e, sender, testFresh, now); !errors.Is(err, ErrReplay) {
		t.Errorf("duplicate id: err = %v, want ErrReplay", err)
	}
	// Empty and non-UUID ids.
	bad := e
	bad.ID = ""
	if err := s.IngestMessage(bad, sender, testFresh, now); !errors.Is(err, ErrReplay) {
		t.Errorf("empty id: err = %v, want ErrReplay", err)
	}
	bad.ID = "not-a-uuid"
	if err := s.IngestMessage(bad, sender, testFresh, now); !errors.Is(err, ErrReplay) {
		t.Errorf("non-uuid id: err = %v, want ErrReplay", err)
	}
	// Replay after the body is swept: the tombstone still refuses it.
	if _, err := s.db.Exec(`DELETE FROM messages WHERE id = ?`, e.ID); err != nil {
		t.Fatalf("delete message: %v", err)
	}
	if err := s.IngestMessage(e, sender, testFresh, now); !errors.Is(err, ErrReplay) {
		t.Errorf("replay after deletion: err = %v, want ErrReplay (tombstone)", err)
	}
}

func TestIngestNotFresh(t *testing.T) {
	s := newStore(t)
	now := time.Unix(1_700_000_000, 0).UTC()
	sender, _ := enrollPerson(t, s, "s@example.com", now)
	recip, _ := enrollPerson(t, s, "r@example.com", now)
	thread := uuid.Must(uuid.NewV7()).String()

	tooOld := mkEnvelope(thread, sender, recip, now.Add(-48*time.Hour))
	if err := s.IngestMessage(tooOld, sender, testFresh, now); !errors.Is(err, ErrNotFresh) {
		t.Errorf("too old: err = %v, want ErrNotFresh", err)
	}
	tooFuture := mkEnvelope(thread, sender, recip, now.Add(time.Hour))
	if err := s.IngestMessage(tooFuture, sender, testFresh, now); !errors.Is(err, ErrNotFresh) {
		t.Errorf("too future: err = %v, want ErrNotFresh", err)
	}
}

func TestIngestExistingThreadSetsInputRequired(t *testing.T) {
	s := newStore(t)
	now := time.Unix(1_700_000_000, 0).UTC()
	sender, _ := enrollPerson(t, s, "s@example.com", now)
	recip, _ := enrollPerson(t, s, "r@example.com", now)
	thread := uuid.Must(uuid.NewV7()).String()

	if err := s.IngestMessage(mkEnvelope(thread, sender, recip, now), sender, testFresh, now); err != nil {
		t.Fatalf("first ingest: %v", err)
	}
	// The recipient acts on it, driving the thread to a non-input-required state.
	if _, err := s.db.Exec(`UPDATE threads SET state = 'working' WHERE id = ?`, thread); err != nil {
		t.Fatalf("set thread working: %v", err)
	}
	// A delivered message onto the existing thread is the recipient's turn again:
	// ingest flips it back to input-required (WP-08 finding H1) so check_inbox /
	// InboundAwaiting surface it — otherwise a follow-up onto a live thread would
	// be invisible.
	if err := s.IngestMessage(mkEnvelope(thread, sender, recip, now), sender, testFresh, now); err != nil {
		t.Fatalf("second ingest: %v", err)
	}
	var st string
	if err := s.db.QueryRow(`SELECT state FROM threads WHERE id = ?`, thread).Scan(&st); err != nil {
		t.Fatalf("read thread state: %v", err)
	}
	if st != "input-required" {
		t.Errorf("thread state = %q, want input-required (a delivered message is the recipient's turn)", st)
	}
}

func TestAckRemovesFromInboxPerDevice(t *testing.T) {
	s := newStore(t)
	now := time.Unix(1_700_000_000, 0).UTC()
	sender, _ := enrollPerson(t, s, "s@example.com", now)

	// Recipient with two devices: enroll one, then add a second via a fresh
	// invite for the same email.
	recip, devA := enrollPerson(t, s, "r@example.com", now)
	pubB, _, _ := ed25519.GenerateKey(rand.Reader)
	tokenB, _ := s.CreateInvite("r@example.com", time.Hour, now)
	_, dB, err := s.Enroll(tokenB, pubB, "second", now)
	if err != nil {
		t.Fatalf("enroll second device: %v", err)
	}
	devB := dB.ID

	thread := uuid.Must(uuid.NewV7()).String()
	e := mkEnvelope(thread, sender, recip, now)
	if err := s.IngestMessage(e, sender, testFresh, now); err != nil {
		t.Fatalf("ingest: %v", err)
	}

	// Both devices see it.
	for _, dev := range []string{devA, devB} {
		if in, _ := s.Inbox(dev); len(in) != 1 {
			t.Fatalf("device %s inbox = %d items, want 1", dev, len(in))
		}
	}
	// Ack on devA clears only devA.
	if err := s.Ack(e.ID, devA, now); err != nil {
		t.Fatalf("Ack: %v", err)
	}
	if in, _ := s.Inbox(devA); len(in) != 0 {
		t.Errorf("devA inbox after ack = %d, want 0", len(in))
	}
	if in, _ := s.Inbox(devB); len(in) != 1 {
		t.Errorf("devB inbox after devA ack = %d, want 1 (per-device)", len(in))
	}
}

func TestDeliveryNotFound(t *testing.T) {
	s := newStore(t)
	now := time.Unix(1_700_000_000, 0).UTC()
	if err := s.MarkDelivered("no-msg", "no-dev", now); !errors.Is(err, ErrNotFound) {
		t.Errorf("MarkDelivered missing pair: err = %v, want ErrNotFound", err)
	}
	if err := s.Ack("no-msg", "no-dev", now); !errors.Is(err, ErrNotFound) {
		t.Errorf("Ack missing pair: err = %v, want ErrNotFound", err)
	}
}

func TestIngestSkipsRevokedDevice(t *testing.T) {
	s := newStore(t)
	now := time.Unix(1_700_000_000, 0).UTC()
	sender, _ := enrollPerson(t, s, "s@example.com", now)
	recip, recipDev := enrollPerson(t, s, "r@example.com", now)

	if err := s.RevokeDevice(recipDev, now); err != nil {
		t.Fatalf("RevokeDevice: %v", err)
	}
	thread := uuid.Must(uuid.NewV7()).String()
	e := mkEnvelope(thread, sender, recip, now)
	if err := s.IngestMessage(e, sender, testFresh, now); err != nil {
		t.Fatalf("ingest: %v", err)
	}
	// The revoked device gets no delivery row.
	if in, _ := s.Inbox(recipDev); len(in) != 0 {
		t.Errorf("revoked device inbox = %d, want 0 (no delivery fan-out)", len(in))
	}
}
