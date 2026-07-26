package store

import (
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/Mediacom99/askrelay/internal/a2a"
	"github.com/Mediacom99/askrelay/internal/gate"
)

// ingestOne enrolls sender+recipient, ingests one message on a fresh thread,
// and returns the thread id, message id, and recipient person id.
func ingestOne(t *testing.T, s *Store, now time.Time) (threadID, messageID, recipID string) {
	t.Helper()
	sender, _ := enrollPerson(t, s, uuid.Must(uuid.NewV7()).String()+"@s.example", now)
	recip, _ := enrollPerson(t, s, uuid.Must(uuid.NewV7()).String()+"@r.example", now)
	threadID = uuid.Must(uuid.NewV7()).String()
	e := mkEnvelope(threadID, sender, recip, now)
	if err := s.IngestMessage(e, sender, testFresh, now); err != nil {
		t.Fatalf("IngestMessage: %v", err)
	}
	return threadID, e.ID, recip
}

func threadState(t *testing.T, s *Store, threadID string) string {
	t.Helper()
	var st string
	if err := s.db.QueryRow(`SELECT state FROM threads WHERE id=?`, threadID).Scan(&st); err != nil {
		t.Fatalf("read thread state: %v", err)
	}
	return st
}

func TestApproveInbound(t *testing.T) {
	s := newStore(t)
	now := time.Unix(1_700_000_000, 0).UTC()

	thread, _, _ := ingestOne(t, s, now)
	if err := s.ApproveInbound(thread, now); err != nil {
		t.Fatalf("ApproveInbound: %v", err)
	}
	if st := threadState(t, s, thread); st != string(a2a.StateWorking) {
		t.Errorf("after approve: state = %q, want working", st)
	}

	other, _, _ := ingestOne(t, s, now)
	if err := s.DeclineInbound(other, now); err != nil {
		t.Fatalf("DeclineInbound: %v", err)
	}
	if st := threadState(t, s, other); st != string(a2a.StateRejected) {
		t.Errorf("after decline: state = %q, want rejected", st)
	}
}

func TestApproveInboundIllegalAndUnknown(t *testing.T) {
	s := newStore(t)
	now := time.Unix(1_700_000_000, 0).UTC()

	thread, _, _ := ingestOne(t, s, now)
	if err := s.ApproveInbound(thread, now); err != nil {
		t.Fatalf("first approve: %v", err)
	}
	// Second approve from working is illegal — and the gate sentinel survives
	// the store's wrapping.
	err := s.ApproveInbound(thread, now)
	if !errors.Is(err, gate.ErrIllegalTransition) {
		t.Errorf("approve from working: err = %v, want wrapped gate.ErrIllegalTransition", err)
	}
	if st := threadState(t, s, thread); st != string(a2a.StateWorking) {
		t.Errorf("illegal approve mutated state to %q, want unchanged working", st)
	}

	if err := s.ApproveInbound("no-such-thread", now); !errors.Is(err, ErrNotFound) {
		t.Errorf("approve unknown thread: err = %v, want ErrNotFound", err)
	}
}

func TestThreadGrantIssueRevoke(t *testing.T) {
	s := newStore(t)
	now := time.Unix(1_700_000_000, 0).UTC()
	thread, _, recip := ingestOne(t, s, now)

	if err := s.SetThreadGrant(thread, recip, gate.Inbound, now); err != nil {
		t.Fatalf("SetThreadGrant: %v", err)
	}
	// Idempotent: a second issue must not add a row.
	if err := s.SetThreadGrant(thread, recip, gate.Inbound, now); err != nil {
		t.Fatalf("SetThreadGrant #2: %v", err)
	}
	var active int
	if err := s.db.QueryRow(
		`SELECT count(*) FROM grants WHERE thread_id=? AND person_id=? AND direction='inbound' AND revoked_at IS NULL`,
		thread, recip).Scan(&active); err != nil {
		t.Fatalf("count active grants: %v", err)
	}
	if active != 1 {
		t.Errorf("active inbound grants = %d, want 1 (idempotent issue)", active)
	}

	if err := s.RevokeThreadGrant(thread, recip, gate.Inbound, now); err != nil {
		t.Fatalf("RevokeThreadGrant: %v", err)
	}
	// Revoking again — nothing active — is ErrNotFound.
	if err := s.RevokeThreadGrant(thread, recip, gate.Inbound, now); !errors.Is(err, ErrNotFound) {
		t.Errorf("revoke with no active grant: err = %v, want ErrNotFound", err)
	}
	// Bad direction is refused up front.
	if err := s.SetThreadGrant(thread, recip, gate.Direction("both"), now); !errors.Is(err, gate.ErrWrongDirection) {
		t.Errorf("bad direction: err = %v, want wrapped gate.ErrWrongDirection", err)
	}
}

func TestApproveInboundViaGrant(t *testing.T) {
	s := newStore(t)
	now := time.Unix(1_700_000_000, 0).UTC()
	thread, msg, recip := ingestOne(t, s, now)

	// No grant yet: does not fire, thread stays input-required.
	fired, err := s.ApproveInboundViaGrant(thread, recip, msg, now)
	if err != nil {
		t.Fatalf("ApproveInboundViaGrant (no grant): %v", err)
	}
	if fired {
		t.Error("fired with no grant present")
	}
	if st := threadState(t, s, thread); st != string(a2a.StateInputRequired) {
		t.Errorf("state = %q after no-op, want input-required", st)
	}

	// With an active inbound grant: fires, thread → working, message via_grant.
	if err := s.SetThreadGrant(thread, recip, gate.Inbound, now); err != nil {
		t.Fatalf("SetThreadGrant: %v", err)
	}
	fired, err = s.ApproveInboundViaGrant(thread, recip, msg, now)
	if err != nil {
		t.Fatalf("ApproveInboundViaGrant (granted): %v", err)
	}
	if !fired {
		t.Error("did not fire despite active grant")
	}
	if st := threadState(t, s, thread); st != string(a2a.StateWorking) {
		t.Errorf("state = %q after grant fire, want working", st)
	}
	var viaGrant int
	if err := s.db.QueryRow(`SELECT via_grant FROM messages WHERE id=?`, msg).Scan(&viaGrant); err != nil {
		t.Fatalf("read via_grant: %v", err)
	}
	if viaGrant != 1 {
		t.Errorf("message via_grant = %d, want 1", viaGrant)
	}
}

func TestApproveInboundViaGrantRevoked(t *testing.T) {
	s := newStore(t)
	now := time.Unix(1_700_000_000, 0).UTC()
	thread, msg, recip := ingestOne(t, s, now)

	if err := s.SetThreadGrant(thread, recip, gate.Inbound, now); err != nil {
		t.Fatalf("SetThreadGrant: %v", err)
	}
	if err := s.RevokeThreadGrant(thread, recip, gate.Inbound, now); err != nil {
		t.Fatalf("RevokeThreadGrant: %v", err)
	}
	fired, err := s.ApproveInboundViaGrant(thread, recip, msg, now)
	if err != nil {
		t.Fatalf("ApproveInboundViaGrant: %v", err)
	}
	if fired {
		t.Error("fired despite the grant being revoked")
	}
}
