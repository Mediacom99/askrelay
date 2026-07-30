package store

import "fmt"

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
