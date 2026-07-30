package store

import (
	"database/sql"
	"errors"
	"fmt"
	"time"
)

// Person-level read queries backing the MCP surface (WP-07). The MCP client is
// authenticated as a person (OAuth), not a device, so these aggregate across
// the person's threads rather than a single device's deliveries.

// InboundItem is a message awaiting a person's inbound verdict. SenderEmail is
// joined in for the spotlight provenance label.
type InboundItem struct {
	MessageID   string
	ThreadID    string
	SenderEmail string
	Envelope    []byte
}

// InboundAwaiting returns the latest not-sent-by-me message in each thread the
// person participates in that sits in input-required — i.e. everything someone
// else has sent me that I have not yet acted on (a new ask or an arrived
// reply). It drops off the moment I approve (thread → working).
func (s *Store) InboundAwaiting(personID string) ([]InboundItem, error) {
	rows, err := s.db.Query(
		`SELECT m.id, m.thread_id, p.email, m.envelope
		 FROM messages m
		 JOIN threads t ON t.id = m.thread_id
		 JOIN persons p ON p.id = m.sender_id
		 WHERE t.state = 'input-required'
		   AND (t.initiator_id = ? OR t.recipient_id = ?)
		   AND m.sender_id != ?
		   AND m.received_at = (SELECT MAX(m2.received_at) FROM messages m2 WHERE m2.thread_id = m.thread_id)
		 ORDER BY m.received_at`, personID, personID, personID)
	if err != nil {
		return nil, fmt.Errorf("store: inbound awaiting: %w", err)
	}
	defer rows.Close()
	var items []InboundItem
	for rows.Next() {
		var it InboundItem
		if err := rows.Scan(&it.MessageID, &it.ThreadID, &it.SenderEmail, &it.Envelope); err != nil {
			return nil, fmt.Errorf("store: scan inbound: %w", err)
		}
		items = append(items, it)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("store: iterate inbound: %w", err)
	}
	return items, nil
}

// DraftItem is one of the person's own drafts awaiting their outbound review.
type DraftItem struct {
	DraftID  string
	ThreadID string
	Envelope []byte
}

// DraftsAwaitingReview returns the person's pending_review drafts.
func (s *Store) DraftsAwaitingReview(personID string) ([]DraftItem, error) {
	rows, err := s.db.Query(
		`SELECT id, thread_id, envelope FROM drafts
		 WHERE author_id = ? AND state = 'pending_review' ORDER BY created_at`, personID)
	if err != nil {
		return nil, fmt.Errorf("store: drafts awaiting review: %w", err)
	}
	defer rows.Close()
	var items []DraftItem
	for rows.Next() {
		var it DraftItem
		if err := rows.Scan(&it.DraftID, &it.ThreadID, &it.Envelope); err != nil {
			return nil, fmt.Errorf("store: scan draft: %w", err)
		}
		items = append(items, it)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("store: iterate drafts: %w", err)
	}
	return items, nil
}

// ThreadMessage is one message in a thread view.
type ThreadMessage struct {
	MessageID   string
	SenderEmail string
	Envelope    []byte
	ReceivedAt  time.Time
}

// ThreadView is a thread the caller participates in: its messages and the
// caller's own drafts.
type ThreadView struct {
	ThreadID string
	State    string
	Messages []ThreadMessage
	Drafts   []DraftItem
}

// ThreadFor returns a thread the person participates in — its messages and the
// person's OWN drafts (an unreleased draft is private to its author). A
// non-participant or unknown id is ErrNotFound (no oracle): the two are
// indistinguishable to the caller.
func (s *Store) ThreadFor(personID, threadID string) (ThreadView, error) {
	var v ThreadView
	var initiator, recipient string
	err := s.db.QueryRow(`SELECT state, initiator_id, recipient_id FROM threads WHERE id = ?`, threadID).
		Scan(&v.State, &initiator, &recipient)
	if errors.Is(err, sql.ErrNoRows) {
		return ThreadView{}, ErrNotFound
	}
	if err != nil {
		return ThreadView{}, fmt.Errorf("store: load thread: %w", err)
	}
	if personID != initiator && personID != recipient {
		return ThreadView{}, ErrNotFound
	}
	v.ThreadID = threadID

	mrows, err := s.db.Query(
		`SELECT m.id, p.email, m.envelope, m.received_at
		 FROM messages m JOIN persons p ON p.id = m.sender_id
		 WHERE m.thread_id = ? ORDER BY m.received_at`, threadID)
	if err != nil {
		return ThreadView{}, fmt.Errorf("store: thread messages: %w", err)
	}
	defer mrows.Close()
	for mrows.Next() {
		var m ThreadMessage
		var received int64
		if err := mrows.Scan(&m.MessageID, &m.SenderEmail, &m.Envelope, &received); err != nil {
			return ThreadView{}, fmt.Errorf("store: scan thread message: %w", err)
		}
		m.ReceivedAt = time.Unix(received, 0).UTC()
		v.Messages = append(v.Messages, m)
	}
	if err := mrows.Err(); err != nil {
		return ThreadView{}, fmt.Errorf("store: iterate thread messages: %w", err)
	}

	drows, err := s.db.Query(
		`SELECT id, thread_id, envelope FROM drafts
		 WHERE thread_id = ? AND author_id = ? ORDER BY created_at`, threadID, personID)
	if err != nil {
		return ThreadView{}, fmt.Errorf("store: thread drafts: %w", err)
	}
	defer drows.Close()
	for drows.Next() {
		var d DraftItem
		if err := drows.Scan(&d.DraftID, &d.ThreadID, &d.Envelope); err != nil {
			return ThreadView{}, fmt.Errorf("store: scan thread draft: %w", err)
		}
		v.Drafts = append(v.Drafts, d)
	}
	if err := drows.Err(); err != nil {
		return ThreadView{}, fmt.Errorf("store: iterate thread drafts: %w", err)
	}
	return v, nil
}
