package store

import (
	"database/sql"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"

	"github.com/Mediacom99/askrelay/internal/a2a"
	"github.com/Mediacom99/askrelay/internal/gate"
)

// ApproveInbound / DeclineInbound apply a human verdict to a thread's inbound
// gate. There is deliberately NO state parameter: the authoritative state is
// read from the store's own row, never supplied by the caller (F3). The read,
// the gate decision, and the write-back share one transaction, so concurrent
// verdicts cannot both act on a stale state (TOCTOU).
func (s *Store) ApproveInbound(threadID string, now time.Time) error {
	return s.transitionInbound(threadID, gate.Approve, now)
}

func (s *Store) DeclineInbound(threadID string, now time.Time) error {
	return s.transitionInbound(threadID, gate.Reject, now)
}

func (s *Store) transitionInbound(threadID string, v gate.Verdict, now time.Time) error {
	return s.writeTx(func(tx *sql.Tx) error {
		var cur string
		err := tx.QueryRow(`SELECT state FROM threads WHERE id = ?`, threadID).Scan(&cur)
		if errors.Is(err, sql.ErrNoRows) {
			return ErrNotFound
		}
		if err != nil {
			return fmt.Errorf("store: load thread: %w", err)
		}
		_, err = applyInboundTx(tx, threadID, a2a.ThreadState(cur), v, now)
		return err
	})
}

// applyInboundTx applies a verdict to threadID within tx: the gate decision
// (the single authority on legality — wrapped so callers can still
// errors.Is(err, gate.ErrIllegalTransition)) plus the write-back. Returns the
// resulting thread state.
func applyInboundTx(tx *sql.Tx, threadID string, cur a2a.ThreadState, v gate.Verdict, now time.Time) (a2a.ThreadState, error) {
	next, err := gate.ApplyInbound(cur, v)
	if err != nil {
		return "", fmt.Errorf("store: inbound verdict: %w", err)
	}
	if _, err := tx.Exec(`UPDATE threads SET state = ?, updated_at = ? WHERE id = ?`,
		string(next), now.Unix(), threadID); err != nil {
		return "", fmt.Errorf("store: update thread: %w", err)
	}
	return next, nil
}

// ApproveInboundMessage / DeclineInboundMessage apply the caller's inbound
// verdict to the thread of messageID. They confirm the message is awaiting
// THIS person's verdict — they participate in its thread and did not send it —
// so no one can act on a message that isn't theirs (D-03; the single-handler
// discipline, WP-02). ErrNotFound if the message is absent or not the caller's
// to act on (no oracle). Returns the resulting thread state.
func (s *Store) ApproveInboundMessage(personID, messageID string, now time.Time) (a2a.ThreadState, error) {
	return s.inboundMessageVerdict(personID, messageID, gate.Approve, now)
}

func (s *Store) DeclineInboundMessage(personID, messageID string, now time.Time) (a2a.ThreadState, error) {
	return s.inboundMessageVerdict(personID, messageID, gate.Reject, now)
}

func (s *Store) inboundMessageVerdict(personID, messageID string, v gate.Verdict, now time.Time) (a2a.ThreadState, error) {
	var next a2a.ThreadState
	err := s.writeTx(func(tx *sql.Tx) error {
		var threadID, sender, initiator, recipient, state string
		err := tx.QueryRow(
			`SELECT m.thread_id, m.sender_id, t.initiator_id, t.recipient_id, t.state
			 FROM messages m JOIN threads t ON t.id = m.thread_id WHERE m.id = ?`, messageID).
			Scan(&threadID, &sender, &initiator, &recipient, &state)
		if errors.Is(err, sql.ErrNoRows) {
			return ErrNotFound
		}
		if err != nil {
			return fmt.Errorf("store: load message: %w", err)
		}
		if (personID != initiator && personID != recipient) || personID == sender {
			return ErrNotFound // not the caller's to act on — no oracle
		}
		var terr error
		next, terr = applyInboundTx(tx, threadID, a2a.ThreadState(state), v, now)
		return terr
	})
	if err != nil {
		return "", err
	}
	return next, nil
}

// requireParticipant confirms personID is one of threadID's two parties.
// "Thread absent" and "not a participant" both collapse to ErrNotParticipant
// (no oracle) — a caller cannot probe which threads exist.
func requireParticipant(tx *sql.Tx, threadID, personID string) error {
	var initiator, recipient string
	err := tx.QueryRow(`SELECT initiator_id, recipient_id FROM threads WHERE id = ?`, threadID).
		Scan(&initiator, &recipient)
	if errors.Is(err, sql.ErrNoRows) {
		return ErrNotParticipant
	}
	if err != nil {
		return fmt.Errorf("store: participant check: %w", err)
	}
	if personID != initiator && personID != recipient {
		return ErrNotParticipant
	}
	return nil
}

// SetThreadGrant issues a standing grant (idempotent: an existing active grant
// on the same thread × person × direction is left as-is). Grants are immortal
// audit rows; issuing never deletes.
func (s *Store) SetThreadGrant(threadID, personID string, dir gate.Direction, now time.Time) error {
	if dir != gate.Inbound && dir != gate.Outbound {
		return fmt.Errorf("store: set grant: %w", gate.ErrWrongDirection)
	}
	return s.writeTx(func(tx *sql.Tx) error {
		if err := requireParticipant(tx, threadID, personID); err != nil {
			return err
		}
		var one int
		err := tx.QueryRow(
			`SELECT 1 FROM grants WHERE thread_id=? AND person_id=? AND direction=? AND revoked_at IS NULL`,
			threadID, personID, string(dir)).Scan(&one)
		if err == nil {
			return nil // already active — idempotent
		}
		if !errors.Is(err, sql.ErrNoRows) {
			return fmt.Errorf("store: grant lookup: %w", err)
		}
		if _, err := tx.Exec(
			`INSERT INTO grants (id, thread_id, person_id, direction, granted_at) VALUES (?, ?, ?, ?, ?)`,
			uuid.Must(uuid.NewV7()).String(), threadID, personID, string(dir), now.Unix()); err != nil {
			return fmt.Errorf("store: create grant: %w", err)
		}
		return nil
	})
}

// RevokeThreadGrant marks the active grant on this triple revoked (a mark, not
// a delete — the row stays as audit history). ErrNotFound if none is active.
func (s *Store) RevokeThreadGrant(threadID, personID string, dir gate.Direction, now time.Time) error {
	return s.writeTx(func(tx *sql.Tx) error {
		if err := requireParticipant(tx, threadID, personID); err != nil {
			return err
		}
		res, err := tx.Exec(
			`UPDATE grants SET revoked_at=? WHERE thread_id=? AND person_id=? AND direction=? AND revoked_at IS NULL`,
			now.Unix(), threadID, personID, string(dir))
		if err != nil {
			return fmt.Errorf("store: revoke grant: %w", err)
		}
		n, err := res.RowsAffected()
		if err != nil {
			return fmt.Errorf("store: revoke grant: %w", err)
		}
		if n == 0 {
			return ErrNotFound
		}
		return nil
	})
}

// ApproveInboundViaGrant auto-approves an inbound message IF granter holds an
// active inbound grant on the thread, returning whether it fired. false means
// "no grant — ask the human". Same F3/TOCTOU discipline as the human path:
// authoritative state read + gate decision + write-back in one transaction.
func (s *Store) ApproveInboundViaGrant(threadID, granterID, messageID string, now time.Time) (bool, error) {
	fired := false
	err := s.writeTx(func(tx *sql.Tx) error {
		var gid string
		err := tx.QueryRow(
			`SELECT id FROM grants
			 WHERE thread_id=? AND person_id=? AND direction='inbound' AND revoked_at IS NULL`,
			threadID, granterID).Scan(&gid)
		if errors.Is(err, sql.ErrNoRows) {
			return nil // no grant → fired stays false
		}
		if err != nil {
			return fmt.Errorf("store: grant lookup: %w", err)
		}
		var cur string
		if err := tx.QueryRow(`SELECT state FROM threads WHERE id=?`, threadID).Scan(&cur); err != nil {
			return fmt.Errorf("store: load thread: %w", err)
		}
		// Route through the gate even though the grant exists — the gate is
		// the one authority on the transition (mirrors how gate composes the
		// human path; the direction asserts stay enforced).
		g := gate.Grant{Thread: threadID, Direction: gate.Inbound}
		appr := gate.Approvable{Kind: gate.KindMessage, Direction: gate.Inbound, Thread: threadID, ID: messageID}
		next, err := gate.ApplyInboundGrant(g, appr, a2a.ThreadState(cur))
		if err != nil {
			return fmt.Errorf("store: grant verdict: %w", err)
		}
		if _, err := tx.Exec(`UPDATE threads SET state=?, updated_at=? WHERE id=?`,
			string(next), now.Unix(), threadID); err != nil {
			return fmt.Errorf("store: update thread: %w", err)
		}
		// Scope the audit mark by thread too: a messageID from a different
		// thread than the granted one must never have its immortal via_grant
		// row flipped by a caller mix-up.
		if _, err := tx.Exec(`UPDATE messages SET via_grant=1 WHERE id=? AND thread_id=?`,
			messageID, threadID); err != nil {
			return fmt.Errorf("store: mark via_grant: %w", err)
		}
		fired = true
		return nil
	})
	return fired, err
}
