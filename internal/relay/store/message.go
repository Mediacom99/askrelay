package store

import (
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"

	"github.com/Mediacom99/askrelay/internal/a2a"
	"github.com/Mediacom99/askrelay/internal/envelope"
)

var (
	// ErrReplay: the message id is empty, malformed, or already seen
	// (WP-01 F1/F2 — dedup on the SIGNED id, and the tombstone outlives the
	// body so a re-send after deletion is still caught).
	ErrReplay = errors.New("store: replayed message")
	// ErrNotFresh: sent_at is outside the accepted window — too old (past the
	// retention horizon) or too far future (clock-skew cap).
	ErrNotFresh = errors.New("store: message not fresh")
	// ErrNotParticipant: the message's sender/recipient (or a draft's author)
	// are not the two parties of the thread being written into — the store
	// enforces the 1:1 thread invariant (D-05), not just FK existence.
	ErrNotParticipant = errors.New("store: not a thread participant")
)

// Freshness bounds a sender-asserted sent_at both ways (WP-01 note): a message
// older than MaxAge is stale; one more than MaxSkew in the future is refused so
// a post-dated envelope can't sit in the window forever. WP-04 injects these
// from config; the store holds no clock or config.
type Freshness struct {
	MaxAge  time.Duration
	MaxSkew time.Duration
}

// InboxItem is one undelivered-or-unacked message for a device. Envelope is the
// stored canonical blob; the caller Decodes + Verifies it before use.
type InboxItem struct {
	ID         string
	ThreadID   string
	SenderID   string
	Envelope   []byte
	ReceivedAt time.Time
}

// IngestMessage persists a VERIFIED inbound envelope (the caller has already
// Decoded, resolved the signing device, and Verified). It dedups on the signed
// id, bounds sent_at, stores the canonical form (never raw wire), writes the
// replay tombstone, ensures the thread row, and fans out a delivery row per
// active recipient device — all in one transaction.
func (s *Store) IngestMessage(e envelope.Envelope, senderID string, fresh Freshness, now time.Time) error {
	// Pure pre-checks (no DB): an empty or non-UUID id is a replay by policy
	// (WP-01), and freshness needs only the clock.
	if _, err := uuid.Parse(e.ID); err != nil {
		return ErrReplay
	}
	if e.SentAt.Before(now.Add(-fresh.MaxAge)) || e.SentAt.After(now.Add(fresh.MaxSkew)) {
		return ErrNotFresh
	}
	return s.writeTx(func(tx *sql.Tx) error {
		return ingestTx(tx, e, senderID, now)
	})
}

// ingestTx writes a verified/attested envelope inside an existing transaction:
// dedup, thread ensure, message row, replay tombstone, delivery fan-out. Shared
// by IngestMessage (the signed inbound path) and DeliverDraft (relay-attested
// delivery); the caller owns the tx and any freshness/id pre-checks.
func ingestTx(tx *sql.Tx, e envelope.Envelope, senderID string, now time.Time) error {
	seen, err := tombstoneSeen(tx, e.ID)
	if err != nil {
		return err
	}
	if seen {
		return ErrReplay
	}
	if err := ensureThread(tx, e, senderID, now); err != nil {
		return err
	}
	// Canonical form = the re-marshaled decoded struct (arch §4.1 invariant),
	// sig included so the recipient can re-Verify. Never the raw received bytes.
	blob, err := json.Marshal(e)
	if err != nil {
		return fmt.Errorf("store: marshal envelope: %w", err)
	}
	if _, err := tx.Exec(
		`INSERT INTO messages (id, thread_id, sender_id, envelope, sent_at, received_at, via_grant)
		 VALUES (?, ?, ?, ?, ?, ?, 0)`,
		e.ID, e.Thread, senderID, blob, e.SentAt.Unix(), now.Unix()); err != nil {
		return fmt.Errorf("store: insert message: %w", err)
	}
	// The tombstone records the SIGNED sent_at (not the relay clock) so it
	// can be pruned against the same freshness horizon a replay is checked
	// against (retention.go PruneTombstones).
	if _, err := tx.Exec(
		`INSERT INTO message_tombstones (id, sent_at) VALUES (?, ?)`, e.ID, e.SentAt.Unix()); err != nil {
		return fmt.Errorf("store: insert tombstone: %w", err)
	}
	return fanOutDeliveries(tx, e.ID, e.To)
}

// tombstoneSeen reports whether id already has a replay tombstone — the
// authoritative dedup record, which outlives the message body the sweeper
// deletes.
func tombstoneSeen(tx *sql.Tx, id string) (bool, error) {
	var one int
	err := tx.QueryRow(`SELECT 1 FROM message_tombstones WHERE id = ?`, id).Scan(&one)
	if err == nil {
		return true, nil
	}
	if errors.Is(err, sql.ErrNoRows) {
		return false, nil
	}
	return false, fmt.Errorf("store: dedup check: %w", err)
}

// ensureThread creates the thread on its first message (at input-required,
// from the action — never from e.State, F3) or, for an existing thread,
// verifies the message's two ends are its two parties. A reply legitimately
// flows recipient→initiator, so either ordering is allowed; an unrelated
// person who knows the thread id is refused with ErrNotParticipant, holding
// the 1:1 invariant (D-05). It never transitions an existing thread's state —
// the gate does that (approval.go).
func ensureThread(tx *sql.Tx, e envelope.Envelope, senderID string, now time.Time) error {
	var st, initiator, recipient string
	err := tx.QueryRow(`SELECT state, initiator_id, recipient_id FROM threads WHERE id = ?`, e.Thread).
		Scan(&st, &initiator, &recipient)
	switch {
	case errors.Is(err, sql.ErrNoRows):
		if _, err := tx.Exec(
			`INSERT INTO threads (id, initiator_id, recipient_id, state, created_at, updated_at)
			 VALUES (?, ?, ?, ?, ?, ?)`,
			e.Thread, senderID, e.To, string(a2a.StateInputRequired), now.Unix(), now.Unix()); err != nil {
			return fmt.Errorf("store: create thread: %w", err)
		}
		return nil
	case err != nil:
		return fmt.Errorf("store: find thread: %w", err)
	default:
		parties := (senderID == initiator && e.To == recipient) ||
			(senderID == recipient && e.To == initiator)
		if !parties {
			return ErrNotParticipant
		}
		return nil
	}
}

// fanOutDeliveries inserts one delivery row per ACTIVE recipient device — the
// T-09 "all devices acked" set, fixed at ingest time. The cursor is drained
// before the writes because modernc/SQLite dislikes an open read cursor
// mid-write on the same transaction.
func fanOutDeliveries(tx *sql.Tx, messageID, recipientID string) error {
	rows, err := tx.Query(
		`SELECT id FROM devices WHERE person_id = ? AND revoked_at IS NULL`, recipientID)
	if err != nil {
		return fmt.Errorf("store: recipient devices: %w", err)
	}
	var deviceIDs []string
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			rows.Close()
			return fmt.Errorf("store: scan device: %w", err)
		}
		deviceIDs = append(deviceIDs, id)
	}
	if err := rows.Err(); err != nil {
		rows.Close()
		return fmt.Errorf("store: iterate devices: %w", err)
	}
	rows.Close()

	for _, id := range deviceIDs {
		if _, err := tx.Exec(
			`INSERT INTO deliveries (message_id, device_id) VALUES (?, ?)`, messageID, id); err != nil {
			return fmt.Errorf("store: insert delivery: %w", err)
		}
	}
	return nil
}

// Inbox lists a device's messages with an unacked delivery, oldest first.
func (s *Store) Inbox(deviceID string) ([]InboxItem, error) {
	rows, err := s.db.Query(
		`SELECT m.id, m.thread_id, m.sender_id, m.envelope, m.received_at
		 FROM messages m
		 JOIN deliveries d ON d.message_id = m.id
		 WHERE d.device_id = ? AND d.acked_at IS NULL
		 ORDER BY m.received_at, m.id`, deviceID)
	if err != nil {
		return nil, fmt.Errorf("store: inbox: %w", err)
	}
	defer rows.Close()
	var items []InboxItem
	for rows.Next() {
		var it InboxItem
		var received int64
		if err := rows.Scan(&it.ID, &it.ThreadID, &it.SenderID, &it.Envelope, &received); err != nil {
			return nil, fmt.Errorf("store: scan inbox: %w", err)
		}
		it.ReceivedAt = time.Unix(received, 0).UTC()
		items = append(items, it)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("store: iterate inbox: %w", err)
	}
	return items, nil
}

// MarkDelivered records that a device received a message (idempotent).
// ErrNotFound if no such delivery row exists.
func (s *Store) MarkDelivered(messageID, deviceID string, now time.Time) error {
	return s.writeTx(func(tx *sql.Tx) error {
		if err := requireDelivery(tx, messageID, deviceID); err != nil {
			return err
		}
		if _, err := tx.Exec(
			`UPDATE deliveries SET delivered_at = COALESCE(delivered_at, ?)
			 WHERE message_id = ? AND device_id = ?`,
			now.Unix(), messageID, deviceID); err != nil {
			return fmt.Errorf("store: mark delivered: %w", err)
		}
		return nil
	})
}

// Ack records that a device acknowledged a message — the retention sweeper's
// per-device signal (T-09). Implies delivery. Idempotent. ErrNotFound if no
// such delivery row exists.
func (s *Store) Ack(messageID, deviceID string, now time.Time) error {
	return s.writeTx(func(tx *sql.Tx) error {
		if err := requireDelivery(tx, messageID, deviceID); err != nil {
			return err
		}
		if _, err := tx.Exec(
			`UPDATE deliveries SET acked_at = COALESCE(acked_at, ?),
			                      delivered_at = COALESCE(delivered_at, ?)
			 WHERE message_id = ? AND device_id = ?`,
			now.Unix(), now.Unix(), messageID, deviceID); err != nil {
			return fmt.Errorf("store: ack: %w", err)
		}
		return nil
	})
}

// requireDelivery maps a missing (message, device) delivery pair to
// ErrNotFound. The COALESCE-based UPDATEs are idempotent, so RowsAffected can't
// distinguish "already set" from "missing" — existence is probed explicitly.
func requireDelivery(tx *sql.Tx, messageID, deviceID string) error {
	var one int
	err := tx.QueryRow(
		`SELECT 1 FROM deliveries WHERE message_id = ? AND device_id = ?`,
		messageID, deviceID).Scan(&one)
	if errors.Is(err, sql.ErrNoRows) {
		return ErrNotFound
	}
	if err != nil {
		return fmt.Errorf("store: delivery lookup: %w", err)
	}
	return nil
}
