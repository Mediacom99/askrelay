package store

import (
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/Mediacom99/askrelay/internal/a2a"
	"github.com/Mediacom99/askrelay/internal/envelope"
	"github.com/Mediacom99/askrelay/internal/gate"
)

// ---------------------------------------------------------------------------
// WP-08 quality pass, finding M1 (all three agents): release and delivery used
// to be two separate write transactions, so a crash/error between them could
// strand a released-but-undelivered draft forever. FIXED by folding delivery
// into the release transaction (deliverInTx inside ReleaseDraft /
// ReleaseReplyViaGrant). These tests pin the folded behaviour: a released draft
// is, by construction, a delivered message — the strandable state cannot exist.
// ---------------------------------------------------------------------------

// TestReleaseDeliversAtomically: a successful ReleaseDraft leaves the draft
// "sent" AND the message delivered in the SAME commit — there is no store or
// tool API that can produce a sent-but-undelivered draft.
func TestReleaseDeliversAtomically(t *testing.T) {
	s := newStore(t)
	now := time.Unix(1_700_000_000, 0).UTC()
	thread, author, draft := setupDraft(t, s, now)

	recipient, rel, err := s.ReleaseDraft(author, draft, nil, now)
	if err != nil || rel == nil {
		t.Fatalf("ReleaseDraft: rel=%v err=%v", rel, err)
	}
	if recipient == "" {
		t.Fatal("ReleaseDraft returned an empty recipient id")
	}
	if st := draftState(t, s, draft); st != "sent" {
		t.Fatalf("draft state = %q, want sent", st)
	}
	// Same commit delivered the message row (draft id == message id).
	var one int
	if err := s.db.QueryRow(`SELECT 1 FROM messages WHERE id=?`, draft).Scan(&one); err != nil {
		t.Fatalf("released draft was not delivered atomically: %v", err)
	}
	// ...and the recipient's thread is input-required (their turn).
	items, err := s.InboundAwaiting(recipient)
	if err != nil {
		t.Fatalf("InboundAwaiting: %v", err)
	}
	if len(items) != 1 || items[0].MessageID != draft || items[0].ThreadID != thread {
		t.Fatalf("InboundAwaiting = %+v, want the delivered reply %q on thread %q", items, draft, thread)
	}
}

// TestReleaseConcurrentDoubleRelease: N goroutines racing to release the SAME
// draft — exactly one wins (releases + delivers once); the rest are refused by
// the gate (ErrIllegalTransition) or the ingest dedup (ErrReplay). No race, no
// double delivery.
func TestReleaseConcurrentDoubleRelease(t *testing.T) {
	s := newStore(t)
	now := time.Unix(1_700_000_000, 0).UTC()
	_, author, draft := setupDraft(t, s, now)

	const n = 10
	var wg sync.WaitGroup
	results := make([]error, n)
	wg.Add(n)
	for i := range n {
		go func(i int) {
			defer wg.Done()
			_, _, results[i] = s.ReleaseDraft(author, draft, nil, now)
		}(i)
	}
	wg.Wait()

	oks, refused, other := 0, 0, 0
	for _, err := range results {
		switch {
		case err == nil:
			oks++
		case errors.Is(err, gate.ErrIllegalTransition) || errors.Is(err, ErrReplay):
			refused++
		default:
			other++
			t.Errorf("unexpected error from concurrent release: %v", err)
		}
	}
	if oks != 1 {
		t.Errorf("concurrent release: %d succeeded, want exactly 1 (%d refused, %d other)", oks, refused, other)
	}
	// Exactly one delivered message row exists.
	var count int
	if err := s.db.QueryRow(`SELECT COUNT(*) FROM messages WHERE id=?`, draft).Scan(&count); err != nil {
		t.Fatalf("count messages: %v", err)
	}
	if count != 1 {
		t.Errorf("delivered message rows = %d, want exactly 1", count)
	}
}

// TestReleaseViaGrantDelivers: the send_message auto-release path (via an
// outbound grant) must deliver just like a manual approve_reply release.
func TestReleaseViaGrantDelivers(t *testing.T) {
	s := newStore(t)
	now := time.Unix(1_700_000_000, 0).UTC()
	thread, author, draft := setupDraft(t, s, now)
	if err := s.SetThreadGrant(thread, author, gate.Outbound, now); err != nil {
		t.Fatalf("SetThreadGrant: %v", err)
	}
	recipient, rel, err := s.ReleaseReplyViaGrant(draft, author, now)
	if err != nil || rel == nil {
		t.Fatalf("ReleaseReplyViaGrant: rel=%v err=%v", rel, err)
	}
	if recipient == "" {
		t.Fatal("via-grant release returned an empty recipient id")
	}
	var one int
	if err := s.db.QueryRow(`SELECT 1 FROM messages WHERE id=?`, draft).Scan(&one); err != nil {
		t.Fatalf("via-grant release did not deliver: %v", err)
	}
}

// TestReleaseEditedTextIsWhatGetsDelivered: the EDITED payload is what lands in
// the recipient's message (the "approved-content-is-what-sends" guarantee),
// never the pre-edit draft body.
func TestReleaseEditedTextIsWhatGetsDelivered(t *testing.T) {
	s := newStore(t)
	now := time.Unix(1_700_000_000, 0).UTC()
	_, author, draft := setupDraft(t, s, now)

	edited := "this is the human-edited final text"
	if _, _, err := s.ReleaseDraft(author, draft, &edited, now); err != nil {
		t.Fatalf("ReleaseDraft with edit: %v", err)
	}

	var blob []byte
	if err := s.db.QueryRow(`SELECT envelope FROM messages WHERE id=?`, draft).Scan(&blob); err != nil {
		t.Fatalf("read delivered message: %v", err)
	}
	e, err := envelope.Decode(blob)
	if err != nil {
		t.Fatalf("decode delivered envelope: %v", err)
	}
	if len(e.Body.Parts) != 1 || e.Body.Parts[0].Text != edited {
		t.Errorf("delivered body = %+v, want the edited text %q", e.Body, edited)
	}
}

// TestReleaseSentAtIsReleaseTime: the delivered message's sent_at reflects the
// release/delivery instant, not the draft-creation timestamp.
func TestReleaseSentAtIsReleaseTime(t *testing.T) {
	s := newStore(t)
	created := time.Unix(1_700_000_000, 0).UTC()
	_, author, draft := setupDraft(t, s, created)

	releasedAt := created.Add(3 * time.Hour)
	if _, _, err := s.ReleaseDraft(author, draft, nil, releasedAt); err != nil {
		t.Fatalf("ReleaseDraft: %v", err)
	}

	var sentAt int64
	if err := s.db.QueryRow(`SELECT sent_at FROM messages WHERE id=?`, draft).Scan(&sentAt); err != nil {
		t.Fatalf("read sent_at: %v", err)
	}
	if got := time.Unix(sentAt, 0).UTC(); !got.Equal(releasedAt) {
		t.Errorf("delivered sent_at = %v, want the release time %v (not draft creation %v)", got, releasedAt, created)
	}
}

// TestSelfThreadBlockedAtSchema: a self-addressed thread (initiator_id ==
// recipient_id) is refused by a DB CHECK constraint, independent of the
// send_message tool-level guard. Pinned so a future migration can't silently
// drop the constraint.
func TestSelfThreadBlockedAtSchema(t *testing.T) {
	s := newStore(t)
	now := time.Unix(1_700_000_000, 0).UTC()
	author, _ := enrollPerson(t, s, "solo@example.com", now)
	thread := uuid.Must(uuid.NewV7()).String()
	self := envelope.Envelope{
		V: 1, ID: uuid.Must(uuid.NewV7()).String(), Thread: thread,
		From: envelope.Party{Person: author}, To: author,
		State: a2a.StateCompleted, SentAt: now,
		Body: envelope.Message{Role: "user", Parts: []envelope.Part{{Type: "text", Text: "note to self"}}},
	}
	if err := s.IngestMessage(self, author, testFresh, now); err == nil {
		t.Error("self-addressed thread (initiator_id == recipient_id) was accepted; want the CHECK constraint to refuse it")
	}
}
