package store

import (
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"

	"github.com/Mediacom99/askrelay/internal/envelope"
	"github.com/Mediacom99/askrelay/internal/gate"
)

// CreateDraft stores an outbound reply composed by the author's AI, at
// pending_review. The envelope is unsigned (signing happens at release-and-
// deliver time, WP-08); its id — minted by envelope.New — becomes the draft
// id and thus the released message's id. Returns the draft id.
func (s *Store) CreateDraft(threadID, authorID string, e envelope.Envelope, now time.Time) (string, error) {
	if _, err := uuid.Parse(e.ID); err != nil {
		return "", fmt.Errorf("store: create draft: bad envelope id: %w", err)
	}
	// T-16 caps: the draft pipeline never signs, so it doesn't inherit the
	// Sign/Verify cap check — enforce it here so no over-cap draft is stored.
	if err := envelope.Validate(e); err != nil {
		return "", fmt.Errorf("store: create draft: %w", err)
	}
	blob, err := json.Marshal(e)
	if err != nil {
		return "", fmt.Errorf("store: marshal draft: %w", err)
	}
	err = s.writeTx(func(tx *sql.Tx) error {
		// The author must be one of the thread's two parties — an unrelated
		// person cannot author a reply on someone else's 1:1 thread (D-05).
		var initiator, recipient string
		err := tx.QueryRow(`SELECT initiator_id, recipient_id FROM threads WHERE id = ?`, threadID).
			Scan(&initiator, &recipient)
		if errors.Is(err, sql.ErrNoRows) {
			return ErrNotFound
		}
		if err != nil {
			return fmt.Errorf("store: find thread: %w", err)
		}
		if authorID != initiator && authorID != recipient {
			return ErrNotParticipant
		}
		// The message's recipient (e.To) must be the thread's OTHER party — a
		// participant cannot draft a message addressed to an unrelated person
		// onto this thread.
		other := recipient
		if authorID == recipient {
			other = initiator
		}
		if e.To != other {
			return ErrNotParticipant
		}
		if _, err := tx.Exec(
			`INSERT INTO drafts (id, thread_id, author_id, envelope, state, created_at)
			 VALUES (?, ?, ?, ?, 'pending_review', ?)`,
			e.ID, threadID, authorID, blob, now.Unix()); err != nil {
			return fmt.Errorf("store: insert draft: %w", err)
		}
		return nil
	})
	if err != nil {
		return "", err
	}
	return e.ID, nil
}

// ReleaseDraft applies the author's release to their pending draft, optionally
// replacing the body with editedText first — a FRESH envelope, never mutating
// the one handed to the gate (WP-02) — AND delivers it, all in ONE transaction:
// release (pending_review→sent) and delivery to the recipient commit atomically,
// so a released draft is by definition a delivered message (no strandable
// sent-but-undelivered state). Author-gated: a draft that isn't the caller's (or
// is absent) is ErrNotFound (no oracle). No state parameter (F3). Returns the
// recipient person id (for the delivery push) and the minted *gate.Release.
func (s *Store) ReleaseDraft(personID, draftID string, editedText *string, now time.Time) (recipientID string, rel *gate.Release, err error) {
	err = s.writeTx(func(tx *sql.Tx) error {
		thread, author, cur, payload, err := loadDraft(tx, draftID)
		if err != nil {
			return err
		}
		if author != personID {
			return ErrNotFound
		}
		if editedText != nil {
			payload.Body = envelope.Message{Role: payload.Body.Role, Parts: []envelope.Part{{Type: "text", Text: *editedText}}}
			// T-16 caps on the edited payload (the draft pipeline never signs, so
			// it doesn't inherit Sign/Verify's cap check — enforce it here).
			if verr := envelope.Validate(payload); verr != nil {
				return fmt.Errorf("store: release draft: %w", verr)
			}
			blob, merr := json.Marshal(payload)
			if merr != nil {
				return fmt.Errorf("store: marshal edited draft: %w", merr)
			}
			if _, uerr := tx.Exec(`UPDATE drafts SET envelope=? WHERE id=?`, blob, draftID); uerr != nil {
				return fmt.Errorf("store: apply edit: %w", uerr)
			}
		}
		appr := gate.Approvable{
			Kind: gate.KindMessage, Direction: gate.Outbound,
			Thread: thread, ID: draftID, Payload: payload,
		}
		next, r, err := gate.ApplyOutbound(appr, cur, gate.Approve)
		if err != nil {
			return fmt.Errorf("store: release draft: %w", err)
		}
		if err := setDraftDecided(tx, draftID, next, false, now); err != nil {
			return err
		}
		rid, err := deliverInTx(tx, payload, author, now)
		if err != nil {
			return err
		}
		recipientID, rel = rid, r
		return nil
	})
	if err != nil {
		return "", nil, err // T-17: zero value + error, never both
	}
	return recipientID, rel, nil
}

// deliverInTx ingests a released draft's (unsigned) envelope as the recipient's
// inbound message, in the caller's transaction — relay-attested: no device
// signature. The author was OAuth-authenticated when the draft was created and
// released, and on a self-hosted relay the relay is the trust anchor (D-10/D-23),
// so the recipient trusts that attestation. ingestTx flips the thread to
// input-required (the recipient's turn). Returns the recipient person id.
//
// ponytail: relay-attested, no signature — correct while the relay is self-hosted
// and trusted and daemon-signed delivery (WP-09) does not yet exist. Upgrade path:
// the signed path returns with the daemon, gated through this same released-draft.
func deliverInTx(tx *sql.Tx, payload envelope.Envelope, author string, now time.Time) (string, error) {
	payload.SentAt = now // "sent" = when released, not when drafted
	if err := ingestTx(tx, payload, author, now); err != nil {
		return "", err
	}
	return payload.To, nil
}

// DiscardDraft rejects the author's pending draft (→ discarded); mints no
// capability. Author-gated (ErrNotFound if not theirs).
func (s *Store) DiscardDraft(personID, draftID string, now time.Time) error {
	return s.writeTx(func(tx *sql.Tx) error {
		thread, author, cur, payload, err := loadDraft(tx, draftID)
		if err != nil {
			return err
		}
		if author != personID {
			return ErrNotFound
		}
		appr := gate.Approvable{
			Kind: gate.KindMessage, Direction: gate.Outbound,
			Thread: thread, ID: draftID, Payload: payload,
		}
		next, _, err := gate.ApplyOutbound(appr, cur, gate.Reject)
		if err != nil {
			return fmt.Errorf("store: discard draft: %w", err)
		}
		return setDraftDecided(tx, draftID, next, false, now)
	})
}

// ReleaseReplyViaGrant auto-releases AND delivers IF granter holds an active
// outbound grant on the thread; a nil *Release with nil error means "no grant —
// ask the human" (and recipientID is then ""). When it fires, release and
// delivery commit atomically in one transaction (see ReleaseDraft). The minted
// Release carries ViaGrant()==true and the draft row is marked via_grant.
func (s *Store) ReleaseReplyViaGrant(draftID, granterID string, now time.Time) (recipientID string, rel *gate.Release, err error) {
	err = s.writeTx(func(tx *sql.Tx) error {
		thread, author, cur, payload, err := loadDraft(tx, draftID)
		if err != nil {
			return err
		}
		if author != granterID {
			return nil // you only auto-release your OWN drafts
		}
		var gid string
		err = tx.QueryRow(
			`SELECT id FROM grants
			 WHERE thread_id=? AND person_id=? AND direction='outbound' AND revoked_at IS NULL`,
			thread, granterID).Scan(&gid)
		if errors.Is(err, sql.ErrNoRows) {
			return nil // no grant → rel stays nil
		}
		if err != nil {
			return fmt.Errorf("store: grant lookup: %w", err)
		}
		g := gate.Grant{Thread: thread, Direction: gate.Outbound}
		appr := gate.Approvable{
			Kind: gate.KindMessage, Direction: gate.Outbound,
			Thread: thread, ID: draftID, Payload: payload,
		}
		next, r, err := gate.ApplyOutboundGrant(g, appr, cur)
		if err != nil {
			return fmt.Errorf("store: grant release: %w", err)
		}
		if err := setDraftDecided(tx, draftID, next, true, now); err != nil {
			return err
		}
		rid, err := deliverInTx(tx, payload, author, now)
		if err != nil {
			return err
		}
		recipientID, rel = rid, r
		return nil
	})
	if err != nil {
		return "", nil, err // T-17: zero value + error, never both
	}
	return recipientID, rel, nil
}

// loadDraft reads a draft's thread, author, authoritative state, and decoded
// (unsigned) envelope. ErrNotFound if the draft is absent.
func loadDraft(tx *sql.Tx, draftID string) (thread, author string, cur gate.DraftState, payload envelope.Envelope, err error) {
	var state string
	var blob []byte
	err = tx.QueryRow(`SELECT thread_id, author_id, state, envelope FROM drafts WHERE id=?`, draftID).
		Scan(&thread, &author, &state, &blob)
	if errors.Is(err, sql.ErrNoRows) {
		return "", "", "", envelope.Envelope{}, ErrNotFound
	}
	if err != nil {
		return "", "", "", envelope.Envelope{}, fmt.Errorf("store: load draft: %w", err)
	}
	e, err := envelope.Decode(blob)
	if err != nil {
		return "", "", "", envelope.Envelope{}, fmt.Errorf("store: decode draft: %w", err)
	}
	return thread, author, gate.DraftState(state), e, nil
}

func setDraftDecided(tx *sql.Tx, draftID string, next gate.DraftState, viaGrant bool, now time.Time) error {
	mark := 0
	if viaGrant {
		mark = 1
	}
	if _, err := tx.Exec(
		`UPDATE drafts SET state=?, via_grant=?, decided_at=? WHERE id=?`,
		string(next), mark, now.Unix(), draftID); err != nil {
		return fmt.Errorf("store: update draft: %w", err)
	}
	return nil
}
