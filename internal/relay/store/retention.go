package store

import (
	"database/sql"
	"fmt"
	"time"
)

// RetentionPolicy is the sweeper's knobs (T-09), injected by WP-08 from config.
// Durations are operator-tunable with a 1 h floor enforced upstream.
type RetentionPolicy struct {
	AckGrace time.Duration // delete an all-acked message this long after its last ack
	HardTTL  time.Duration // delete ANY message / decided draft this long after receipt / decision
	// TombstoneTTL prunes a replay tombstone this long after its SIGNED sent_at.
	// It MUST be >= the ingest Freshness.MaxAge: pruning on sent_at means a
	// tombstone is dropped exactly when a replay carrying it would already be
	// refused as stale, so the replay guard is never shortened below the
	// freshness window.
	TombstoneTTL time.Duration
}

// SweepMessages deletes message bodies that are either fully acked past the
// grace, or past the hard TTL regardless of acks. Per-device delivery rows go
// with them (ON DELETE CASCADE); threads, grants, and tombstones survive.
// Returns the number of messages deleted.
func (s *Store) SweepMessages(p RetentionPolicy, now time.Time) (int, error) {
	hardCutoff := now.Add(-p.HardTTL).Unix()
	graceCutoff := now.Add(-p.AckGrace).Unix()
	var n int64
	err := s.writeTx(func(tx *sql.Tx) error {
		res, err := tx.Exec(
			`DELETE FROM messages
			 WHERE received_at <= ?
			    OR id IN (
			        SELECT m.id FROM messages m
			        WHERE EXISTS (SELECT 1 FROM deliveries d WHERE d.message_id = m.id)
			          AND NOT EXISTS (
			              SELECT 1 FROM deliveries d WHERE d.message_id = m.id AND d.acked_at IS NULL)
			          AND (SELECT MAX(d.acked_at) FROM deliveries d WHERE d.message_id = m.id) <= ?
			    )`,
			hardCutoff, graceCutoff)
		if err != nil {
			return fmt.Errorf("store: sweep messages: %w", err)
		}
		n, err = res.RowsAffected()
		if err != nil {
			return fmt.Errorf("store: sweep messages: %w", err)
		}
		return nil
	})
	return int(n), err
}

// SweepDrafts deletes decided (sent/discarded) drafts past the hard TTL, keyed
// on decided_at. pending_review drafts are live work and never swept.
func (s *Store) SweepDrafts(p RetentionPolicy, now time.Time) (int, error) {
	cutoff := now.Add(-p.HardTTL).Unix()
	var n int64
	err := s.writeTx(func(tx *sql.Tx) error {
		res, err := tx.Exec(
			`DELETE FROM drafts
			 WHERE state IN ('sent','discarded') AND decided_at IS NOT NULL AND decided_at <= ?`,
			cutoff)
		if err != nil {
			return fmt.Errorf("store: sweep drafts: %w", err)
		}
		n, err = res.RowsAffected()
		if err != nil {
			return fmt.Errorf("store: sweep drafts: %w", err)
		}
		return nil
	})
	return int(n), err
}

// PruneTombstones deletes replay tombstones whose SIGNED sent_at is far enough
// past that a replay carrying that id would be refused as stale by the ingest
// freshness check anyway — so the guard is bounded without ever being
// shortened below the freshness window (see RetentionPolicy.TombstoneTTL).
func (s *Store) PruneTombstones(p RetentionPolicy, now time.Time) (int, error) {
	cutoff := now.Add(-p.TombstoneTTL).Unix()
	var n int64
	err := s.writeTx(func(tx *sql.Tx) error {
		res, err := tx.Exec(`DELETE FROM message_tombstones WHERE sent_at < ?`, cutoff)
		if err != nil {
			return fmt.Errorf("store: prune tombstones: %w", err)
		}
		n, err = res.RowsAffected()
		if err != nil {
			return fmt.Errorf("store: prune tombstones: %w", err)
		}
		return nil
	})
	return int(n), err
}
