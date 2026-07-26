package store

import (
	"crypto/ed25519"
	"crypto/rand"
	"crypto/sha256"
	"database/sql"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
)

// Sentinel errors (T-17). ErrInviteInvalid deliberately collapses unknown /
// expired / already-used: enrollment is unauthenticated, so the error must
// not oracle which tokens exist.
var (
	ErrNotFound      = errors.New("store: not found")
	ErrInviteInvalid = errors.New("store: invite invalid")
	// ErrKeyInUse means the Ed25519 public key is already registered to a
	// device (they are globally unique). WP-04's re-enrollment path branches
	// on it rather than a raw SQLite constraint string.
	ErrKeyInUse = errors.New("store: device key already in use")
)

// Person is a roster entry (arch §4.3: no row, no mail).
type Person struct {
	ID        string
	Email     string
	Label     string
	CreatedAt time.Time
}

// Device is an enrolled Ed25519 key. RevokedAt.IsZero() == active.
type Device struct {
	ID        string
	PersonID  string
	PubKey    ed25519.PublicKey
	Label     string
	CreatedAt time.Time
	RevokedAt time.Time
}

// writeTx serializes all writers (T-03 single-writer discipline) and wraps
// fn in a transaction: commit on nil, rollback on error.
func (s *Store) writeTx(fn func(tx *sql.Tx) error) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	tx, err := s.db.Begin()
	if err != nil {
		return fmt.Errorf("store: begin: %w", err)
	}
	if err := fn(tx); err != nil {
		_ = tx.Rollback()
		return err
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("store: commit: %w", err)
	}
	return nil
}

func hashToken(token string) string {
	sum := sha256.Sum256([]byte(token))
	return hex.EncodeToString(sum[:])
}

// CreateInvite mints a single-use enrollment token for email, valid for ttl.
// The returned token goes into the invite URL and is never stored — only its
// SHA-256 is (a leaked DB cannot mint enrollments).
func (s *Store) CreateInvite(email string, ttl time.Duration, now time.Time) (string, error) {
	raw := make([]byte, 32)
	if _, err := rand.Read(raw); err != nil {
		return "", fmt.Errorf("store: invite token: %w", err)
	}
	token := base64.RawURLEncoding.EncodeToString(raw)
	err := s.writeTx(func(tx *sql.Tx) error {
		_, err := tx.Exec(
			`INSERT INTO invites (token_hash, email, created_at, expires_at) VALUES (?, ?, ?, ?)`,
			hashToken(token), email, now.Unix(), now.Add(ttl).Unix())
		if err != nil {
			return fmt.Errorf("store: create invite: %w", err)
		}
		return nil
	})
	if err != nil {
		return "", err
	}
	return token, nil
}

// Enroll consumes an invite and registers a device in one transaction: the
// conditional UPDATE is the single-use guard (two racing enrolls — one
// affected row, one winner), the person is found-or-created by the invite's
// email, and the device row is inserted. Any invalid invite — unknown,
// expired, used — is the same ErrInviteInvalid.
func (s *Store) Enroll(token string, pubkey ed25519.PublicKey, label string, now time.Time) (Person, Device, error) {
	if len(pubkey) != ed25519.PublicKeySize {
		return Person{}, Device{}, fmt.Errorf("store: enroll: pubkey must be %d bytes, got %d", ed25519.PublicKeySize, len(pubkey))
	}
	var p Person
	var d Device
	err := s.writeTx(func(tx *sql.Tx) error {
		res, err := tx.Exec(
			`UPDATE invites SET used_at = ? WHERE token_hash = ? AND used_at IS NULL AND expires_at > ?`,
			now.Unix(), hashToken(token), now.Unix())
		if err != nil {
			return fmt.Errorf("store: consume invite: %w", err)
		}
		n, err := res.RowsAffected()
		if err != nil {
			return fmt.Errorf("store: consume invite: %w", err)
		}
		if n == 0 {
			return ErrInviteInvalid
		}
		var email string
		if err := tx.QueryRow(
			`SELECT email FROM invites WHERE token_hash = ?`, hashToken(token),
		).Scan(&email); err != nil {
			return fmt.Errorf("store: invite email: %w", err)
		}

		p = Person{Email: email}
		var created int64
		err = tx.QueryRow(`SELECT id, label, created_at FROM persons WHERE email = ?`, email).
			Scan(&p.ID, &p.Label, &created)
		switch {
		case errors.Is(err, sql.ErrNoRows):
			p.ID = uuid.Must(uuid.NewV7()).String()
			p.CreatedAt = now
			if _, err := tx.Exec(
				`INSERT INTO persons (id, email, label, created_at) VALUES (?, ?, '', ?)`,
				p.ID, email, now.Unix()); err != nil {
				return fmt.Errorf("store: create person: %w", err)
			}
		case err != nil:
			return fmt.Errorf("store: find person: %w", err)
		default:
			p.CreatedAt = time.Unix(created, 0).UTC()
		}

		// Pre-check the globally-unique pubkey so a duplicate returns the
		// ErrKeyInUse sentinel, not a raw SQLite UNIQUE-constraint string
		// (T-17). Safe inside writeTx: the writer mutex serializes this
		// check-then-insert, so no racing enroll can slip between them.
		var dupe int
		err = tx.QueryRow(`SELECT 1 FROM devices WHERE pubkey = ?`, []byte(pubkey)).Scan(&dupe)
		if err == nil {
			return ErrKeyInUse
		}
		if !errors.Is(err, sql.ErrNoRows) {
			return fmt.Errorf("store: device dup check: %w", err)
		}

		d = Device{
			ID: uuid.Must(uuid.NewV7()).String(), PersonID: p.ID,
			PubKey: pubkey, Label: label, CreatedAt: now,
		}
		if _, err := tx.Exec(
			`INSERT INTO devices (id, person_id, pubkey, label, created_at) VALUES (?, ?, ?, ?, ?)`,
			d.ID, d.PersonID, []byte(pubkey), label, now.Unix()); err != nil {
			return fmt.Errorf("store: create device: %w", err)
		}
		return nil
	})
	if err != nil {
		return Person{}, Device{}, err
	}
	return p, d, nil
}

// RevokeDevice marks a device revoked, effective immediately (arch §4.3:
// the relay refuses signatures and tokens from revoked devices). Revoking
// an already-revoked device is a no-op success, not an error — revocation
// is idempotent by intent.
func (s *Store) RevokeDevice(deviceID string, now time.Time) error {
	return s.writeTx(func(tx *sql.Tx) error {
		res, err := tx.Exec(
			`UPDATE devices SET revoked_at = ? WHERE id = ? AND revoked_at IS NULL`,
			now.Unix(), deviceID)
		if err != nil {
			return fmt.Errorf("store: revoke device: %w", err)
		}
		n, err := res.RowsAffected()
		if err != nil {
			return fmt.Errorf("store: revoke device: %w", err)
		}
		if n == 0 {
			// Distinguish "no such device" (caller bug → ErrNotFound)
			// from "already revoked" (idempotent success).
			var one int
			err := tx.QueryRow(`SELECT 1 FROM devices WHERE id = ?`, deviceID).Scan(&one)
			if errors.Is(err, sql.ErrNoRows) {
				return ErrNotFound
			}
			if err != nil {
				return fmt.Errorf("store: revoke device: %w", err)
			}
		}
		return nil
	})
}

// ActiveDeviceByPubkey resolves a signing key to its ACTIVE device — the
// lookup every signature check routes through (WP-04/07/08). A revoked or
// unknown key is the same ErrNotFound: refusal, with no revocation oracle.
func (s *Store) ActiveDeviceByPubkey(pubkey ed25519.PublicKey) (Device, error) {
	var d Device
	var created int64
	err := s.db.QueryRow(
		`SELECT id, person_id, label, created_at FROM devices
		 WHERE pubkey = ? AND revoked_at IS NULL`, []byte(pubkey)).
		Scan(&d.ID, &d.PersonID, &d.Label, &created)
	if errors.Is(err, sql.ErrNoRows) {
		return Device{}, ErrNotFound
	}
	if err != nil {
		return Device{}, fmt.Errorf("store: device by pubkey: %w", err)
	}
	d.PubKey = pubkey
	d.CreatedAt = time.Unix(created, 0).UTC()
	return d, nil
}
