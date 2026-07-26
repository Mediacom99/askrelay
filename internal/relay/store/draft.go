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

// ReleaseReply applies a human release to a draft's outbound gate and returns
// the minted capability. No state parameter (F3); load + gate + write in one
// transaction (TOCTOU). WP-08 signs Release.Payload() and delivers, matching
// Thread()+ID().
func (s *Store) ReleaseReply(draftID string, now time.Time) (*gate.Release, error) {
	var rel *gate.Release
	err := s.writeTx(func(tx *sql.Tx) error {
		thread, cur, payload, err := loadDraft(tx, draftID)
		if err != nil {
			return err
		}
		appr := gate.Approvable{
			Kind: gate.KindMessage, Direction: gate.Outbound,
			Thread: thread, ID: draftID, Payload: payload,
		}
		next, r, err := gate.ApplyOutbound(appr, cur, gate.Approve)
		if err != nil {
			return fmt.Errorf("store: release reply: %w", err)
		}
		if err := setDraftDecided(tx, draftID, next, false, now); err != nil {
			return err
		}
		rel = r
		return nil
	})
	if err != nil {
		return nil, err
	}
	return rel, nil
}

// DiscardReply rejects a draft (→ discarded); mints no capability.
func (s *Store) DiscardReply(draftID string, now time.Time) error {
	return s.writeTx(func(tx *sql.Tx) error {
		thread, cur, payload, err := loadDraft(tx, draftID)
		if err != nil {
			return err
		}
		appr := gate.Approvable{
			Kind: gate.KindMessage, Direction: gate.Outbound,
			Thread: thread, ID: draftID, Payload: payload,
		}
		next, _, err := gate.ApplyOutbound(appr, cur, gate.Reject)
		if err != nil {
			return fmt.Errorf("store: discard reply: %w", err)
		}
		return setDraftDecided(tx, draftID, next, false, now)
	})
}

// ReleaseReplyViaGrant auto-releases IF granter holds an active outbound grant
// on the thread; a nil *Release with nil error means "no grant — ask the
// human". The minted Release carries ViaGrant()==true and the draft row is
// marked via_grant.
func (s *Store) ReleaseReplyViaGrant(draftID, granterID string, now time.Time) (*gate.Release, error) {
	var rel *gate.Release
	err := s.writeTx(func(tx *sql.Tx) error {
		thread, cur, payload, err := loadDraft(tx, draftID)
		if err != nil {
			return err
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
		rel = r
		return nil
	})
	if err != nil {
		return nil, err
	}
	return rel, nil
}

// loadDraft reads a draft's authoritative state and decodes its stored
// (unsigned) envelope. ErrNotFound if the draft is absent.
func loadDraft(tx *sql.Tx, draftID string) (thread string, cur gate.DraftState, payload envelope.Envelope, err error) {
	var state string
	var blob []byte
	err = tx.QueryRow(`SELECT thread_id, state, envelope FROM drafts WHERE id=?`, draftID).
		Scan(&thread, &state, &blob)
	if errors.Is(err, sql.ErrNoRows) {
		return "", "", envelope.Envelope{}, ErrNotFound
	}
	if err != nil {
		return "", "", envelope.Envelope{}, fmt.Errorf("store: load draft: %w", err)
	}
	e, err := envelope.Decode(blob)
	if err != nil {
		return "", "", envelope.Envelope{}, fmt.Errorf("store: decode draft: %w", err)
	}
	return thread, gate.DraftState(state), e, nil
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
