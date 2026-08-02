package store

import (
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/Mediacom99/askrelay/internal/gate"
)

// setupDraft enrolls asker + author, seeds a thread with an inbound ask, and
// creates one pending_review reply draft. Returns thread, author person id,
// and the draft id.
func setupDraft(t *testing.T, s *Store, now time.Time) (threadID, authorID, draftID string) {
	t.Helper()
	asker, _ := enrollPerson(t, s, uuid.Must(uuid.NewV7()).String()+"@a.example", now)
	author, _ := enrollPerson(t, s, uuid.Must(uuid.NewV7()).String()+"@b.example", now)
	threadID = uuid.Must(uuid.NewV7()).String()
	if err := s.IngestMessage(mkEnvelope(threadID, asker, author, now), asker, testFresh, now); err != nil {
		t.Fatalf("seed thread: %v", err)
	}
	reply := mkEnvelope(threadID, author, asker, now)
	reply.Body.Role = "agent"
	id, err := s.CreateDraft(threadID, author, reply, now)
	if err != nil {
		t.Fatalf("CreateDraft: %v", err)
	}
	if id != reply.ID {
		t.Fatalf("draft id = %q, want the envelope id %q", id, reply.ID)
	}
	return threadID, author, id
}

func draftState(t *testing.T, s *Store, draftID string) string {
	t.Helper()
	var st string
	if err := s.db.QueryRow(`SELECT state FROM drafts WHERE id=?`, draftID).Scan(&st); err != nil {
		t.Fatalf("read draft state: %v", err)
	}
	return st
}

func TestCreateAndReleaseDraft(t *testing.T) {
	s := newStore(t)
	now := time.Unix(1_700_000_000, 0).UTC()
	thread, author, draft := setupDraft(t, s, now)

	if st := draftState(t, s, draft); st != "pending_review" {
		t.Fatalf("new draft state = %q, want pending_review", st)
	}

	rel, err := s.ReleaseDraft(author, draft, nil, now)
	if err != nil {
		t.Fatalf("ReleaseDraft: %v", err)
	}
	if rel == nil {
		t.Fatal("ReleaseDraft returned nil capability")
	}
	if rel.Thread() != thread || rel.ID() != draft {
		t.Errorf("release identity = (thread %q, id %q), want (%q, %q)", rel.Thread(), rel.ID(), thread, draft)
	}
	if rel.ViaGrant() {
		t.Error("manual release marked via_grant")
	}
	if st := draftState(t, s, draft); st != "sent" {
		t.Errorf("released draft state = %q, want sent", st)
	}
}

func TestReleaseReplyIllegalAndUnknown(t *testing.T) {
	s := newStore(t)
	now := time.Unix(1_700_000_000, 0).UTC()
	_, author, draft := setupDraft(t, s, now)

	if _, err := s.ReleaseDraft(author, draft, nil, now); err != nil {
		t.Fatalf("first release: %v", err)
	}
	// Releasing an already-sent draft is illegal; the gate sentinel survives.
	if _, err := s.ReleaseDraft(author, draft, nil, now); !errors.Is(err, gate.ErrIllegalTransition) {
		t.Errorf("release from sent: err = %v, want wrapped gate.ErrIllegalTransition", err)
	}
	if _, err := s.ReleaseDraft(author, "no-such-draft", nil, now); !errors.Is(err, ErrNotFound) {
		t.Errorf("release unknown: err = %v, want ErrNotFound", err)
	}
	if err := s.DiscardDraft(author, "no-such-draft", now); !errors.Is(err, ErrNotFound) {
		t.Errorf("discard unknown: err = %v, want ErrNotFound", err)
	}
}

func TestDiscardReply(t *testing.T) {
	s := newStore(t)
	now := time.Unix(1_700_000_000, 0).UTC()
	_, author, draft := setupDraft(t, s, now)

	if err := s.DiscardDraft(author, draft, now); err != nil {
		t.Fatalf("DiscardDraft: %v", err)
	}
	if st := draftState(t, s, draft); st != "discarded" {
		t.Errorf("discarded draft state = %q, want discarded", st)
	}
	// A discarded draft can't then be released.
	if _, err := s.ReleaseDraft(author, draft, nil, now); !errors.Is(err, gate.ErrIllegalTransition) {
		t.Errorf("release after discard: err = %v, want wrapped gate.ErrIllegalTransition", err)
	}
}

func TestReleaseReplyViaGrant(t *testing.T) {
	s := newStore(t)
	now := time.Unix(1_700_000_000, 0).UTC()
	thread, author, draft := setupDraft(t, s, now)

	// No outbound grant: does not fire, draft stays pending_review.
	rel, err := s.ReleaseReplyViaGrant(draft, author, now)
	if err != nil {
		t.Fatalf("ReleaseReplyViaGrant (no grant): %v", err)
	}
	if rel != nil {
		t.Error("released via grant with no grant present")
	}
	if st := draftState(t, s, draft); st != "pending_review" {
		t.Errorf("draft state = %q after no-op, want pending_review", st)
	}

	// With an active outbound grant: fires, marked via_grant both ways.
	if err := s.SetThreadGrant(thread, author, gate.Outbound, now); err != nil {
		t.Fatalf("SetThreadGrant: %v", err)
	}
	rel, err = s.ReleaseReplyViaGrant(draft, author, now)
	if err != nil {
		t.Fatalf("ReleaseReplyViaGrant (granted): %v", err)
	}
	if rel == nil || !rel.ViaGrant() {
		t.Fatalf("granted release = %v, want non-nil with ViaGrant()==true", rel)
	}
	if st := draftState(t, s, draft); st != "sent" {
		t.Errorf("draft state = %q after grant release, want sent", st)
	}
	var viaGrant int
	if err := s.db.QueryRow(`SELECT via_grant FROM drafts WHERE id=?`, draft).Scan(&viaGrant); err != nil {
		t.Fatalf("read via_grant: %v", err)
	}
	if viaGrant != 1 {
		t.Errorf("draft via_grant = %d, want 1", viaGrant)
	}
}

func TestDeliverDraft(t *testing.T) {
	s := newStore(t)
	now := time.Unix(1_700_000_000, 0).UTC()
	// setupDraft: asker asked author; author drafted a reply back to asker on an
	// already-existing thread (so this exercises the input-required flip).
	thread, author, draft := setupDraft(t, s, now)

	var initiator, recipient string
	if err := s.db.QueryRow(`SELECT initiator_id, recipient_id FROM threads WHERE id=?`, thread).
		Scan(&initiator, &recipient); err != nil {
		t.Fatalf("read thread parties: %v", err)
	}
	asker := initiator
	if asker == author {
		asker = recipient
	}

	if _, err := s.ReleaseDraft(author, draft, nil, now); err != nil {
		t.Fatalf("ReleaseDraft: %v", err)
	}
	deliverAt := now.Add(time.Minute)
	got, err := s.DeliverDraft(draft, deliverAt)
	if err != nil {
		t.Fatalf("DeliverDraft: %v", err)
	}
	if got != asker {
		t.Errorf("DeliverDraft recipient = %q, want the asker %q", got, asker)
	}

	// The reply is now the asker's inbound, on an input-required thread.
	items, err := s.InboundAwaiting(asker)
	if err != nil {
		t.Fatalf("InboundAwaiting: %v", err)
	}
	if len(items) != 1 || items[0].ThreadID != thread || items[0].MessageID != draft {
		t.Fatalf("InboundAwaiting = %+v, want one item for draft %q on thread %q", items, draft, thread)
	}

	// Idempotent: redelivery is a replay (dedup on the envelope id).
	if _, err := s.DeliverDraft(draft, deliverAt); !errors.Is(err, ErrReplay) {
		t.Errorf("redeliver: err = %v, want ErrReplay", err)
	}

	// An unreleased (still pending_review) draft is not deliverable.
	_, _, pending := setupDraft(t, s, now)
	if _, err := s.DeliverDraft(pending, now); !errors.Is(err, ErrNotFound) {
		t.Errorf("deliver pending draft: err = %v, want ErrNotFound", err)
	}
}

func TestCreateDraftBadID(t *testing.T) {
	s := newStore(t)
	now := time.Unix(1_700_000_000, 0).UTC()
	author, _ := enrollPerson(t, s, "b@example.com", now)
	thread := uuid.Must(uuid.NewV7()).String()

	e := mkEnvelope(thread, author, author, now)
	e.ID = "not-a-uuid"
	if _, err := s.CreateDraft(thread, author, e, now); err == nil {
		t.Error("CreateDraft accepted a non-UUID envelope id")
	}
}
