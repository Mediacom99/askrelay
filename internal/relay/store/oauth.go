package store

import (
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
)

// OAuth AS sentinel errors (T-17). Unknown/expired/consumed collapse into one
// opaque refusal — the token endpoint must not oracle which codes or clients exist.
var (
	ErrClientUnknown = errors.New("store: oauth client unknown")
	ErrCodeInvalid   = errors.New("store: oauth authorization code invalid")
)

// OAuthClient is a DCR-registered client (RFC 7591). CIMD clients are not stored.
type OAuthClient struct {
	ID           string
	RedirectURIs []string
	Name         string
	CreatedAt    time.Time
}

// AuthCode is the binding an authorization code carries from /authorize to
// /token. Returned by ConsumeAuthCode; the code value itself never leaves the caller.
type AuthCode struct {
	ClientID      string
	RedirectURI   string
	CodeChallenge string
	Resource      string
	PersonID      string
	DeviceID      string
	ClientType    string
}

// RegisterClient stores a DCR client and returns its relay-minted client_id.
// redirectURIs is validated by the caller; the store only persists it (as JSON).
func (s *Store) RegisterClient(redirectURIs []string, name string, now time.Time) (string, error) {
	id := uuid.Must(uuid.NewV7()).String()
	uris, err := json.Marshal(redirectURIs)
	if err != nil {
		return "", fmt.Errorf("store: marshal redirect_uris: %w", err)
	}
	err = s.writeTx(func(tx *sql.Tx) error {
		_, err := tx.Exec(
			`INSERT INTO oauth_clients (client_id, redirect_uris, client_name, created_at) VALUES (?, ?, ?, ?)`,
			id, string(uris), name, now.Unix())
		return err
	})
	if err != nil {
		return "", fmt.Errorf("store: register client: %w", err)
	}
	return id, nil
}

// ClientByID returns a DCR-registered client. Unknown id → ErrClientUnknown.
func (s *Store) ClientByID(clientID string) (OAuthClient, error) {
	var c OAuthClient
	var uris string
	var created int64
	err := s.db.QueryRow(
		`SELECT client_id, redirect_uris, client_name, created_at FROM oauth_clients WHERE client_id = ?`,
		clientID).Scan(&c.ID, &uris, &c.Name, &created)
	if errors.Is(err, sql.ErrNoRows) {
		return OAuthClient{}, ErrClientUnknown
	}
	if err != nil {
		return OAuthClient{}, fmt.Errorf("store: client by id: %w", err)
	}
	if err := json.Unmarshal([]byte(uris), &c.RedirectURIs); err != nil {
		return OAuthClient{}, fmt.Errorf("store: unmarshal redirect_uris: %w", err)
	}
	c.CreatedAt = time.Unix(created, 0).UTC()
	return c, nil
}

// CreateAuthCode stores a single-use PKCE-bound code (only its SHA-256).
func (s *Store) CreateAuthCode(code string, b AuthCode, expiresAt, now time.Time) error {
	err := s.writeTx(func(tx *sql.Tx) error {
		_, err := tx.Exec(
			`INSERT INTO oauth_codes
			 (code_hash, client_id, redirect_uri, code_challenge, resource, person_id, device_id, client_type, created_at, expires_at)
			 VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
			hashToken(code), b.ClientID, b.RedirectURI, b.CodeChallenge, b.Resource,
			b.PersonID, b.DeviceID, b.ClientType, now.Unix(), expiresAt.Unix())
		return err
	})
	if err != nil {
		return fmt.Errorf("store: create auth code: %w", err)
	}
	return nil
}

// ConsumeAuthCode atomically marks a code used and returns its binding. Unknown,
// expired, or already-consumed all collapse to ErrCodeInvalid (no oracle). The
// conditional UPDATE is the single-use race guard.
func (s *Store) ConsumeAuthCode(code string, now time.Time) (AuthCode, error) {
	var b AuthCode
	err := s.writeTx(func(tx *sql.Tx) error {
		res, err := tx.Exec(
			`UPDATE oauth_codes SET consumed_at = ? WHERE code_hash = ? AND consumed_at IS NULL AND expires_at > ?`,
			now.Unix(), hashToken(code), now.Unix())
		if err != nil {
			return fmt.Errorf("store: consume auth code: %w", err)
		}
		n, err := res.RowsAffected()
		if err != nil {
			return fmt.Errorf("store: consume auth code: %w", err)
		}
		if n == 0 {
			return ErrCodeInvalid
		}
		return tx.QueryRow(
			`SELECT client_id, redirect_uri, code_challenge, resource, person_id, device_id, client_type
			 FROM oauth_codes WHERE code_hash = ?`, hashToken(code)).
			Scan(&b.ClientID, &b.RedirectURI, &b.CodeChallenge, &b.Resource, &b.PersonID, &b.DeviceID, &b.ClientType)
	})
	if err != nil {
		return AuthCode{}, err
	}
	return b, nil
}
