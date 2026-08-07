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
//
// ErrRefreshReused is the replay of an already-consumed refresh token — a theft
// signal; by the time it is returned the family (person × client) has already
// been revoked. It maps to the same opaque invalid_grant at the boundary; the
// distinct sentinel only lets the boundary log the reuse at Warn.
var (
	ErrClientUnknown  = errors.New("store: oauth client unknown")
	ErrCodeInvalid    = errors.New("store: oauth authorization code invalid")
	ErrRefreshInvalid = errors.New("store: oauth refresh token invalid")
	ErrRefreshReused  = errors.New("store: oauth refresh token reused")
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

// RefreshGrant is the binding a refresh token carries. Returned by ConsumeRefreshToken.
type RefreshGrant struct {
	ClientID   string
	PersonID   string
	DeviceID   string
	ClientType string
	Resource   string
}

// CreateRefreshToken stores a rotating refresh token (only its SHA-256).
func (s *Store) CreateRefreshToken(token string, g RefreshGrant, expiresAt, now time.Time) error {
	err := s.writeTx(func(tx *sql.Tx) error {
		_, err := tx.Exec(
			`INSERT INTO oauth_refresh_tokens
			 (token_hash, client_id, person_id, device_id, client_type, resource, created_at, expires_at)
			 VALUES (?, ?, ?, ?, ?, ?, ?, ?)`,
			hashToken(token), g.ClientID, g.PersonID, g.DeviceID, g.ClientType, g.Resource,
			now.Unix(), expiresAt.Unix())
		return err
	})
	if err != nil {
		return fmt.Errorf("store: create refresh token: %w", err)
	}
	return nil
}

// ConsumeRefreshToken rotates a refresh token. A live, unexpired token is marked
// consumed and its binding returned for the caller to mint a successor. Unknown
// or expired → ErrRefreshInvalid. An already-consumed token is a replay: the
// whole family (person × client) is revoked and ErrRefreshReused is returned.
func (s *Store) ConsumeRefreshToken(token string, now time.Time) (RefreshGrant, error) {
	var g RefreshGrant
	// outcome carries the invalid/reused sentinel out of the closure. It must NOT
	// be returned from fn: writeTx rolls back on any non-nil fn error, which would
	// undo the family-revoke write. fn returns nil (commit) and we surface the
	// sentinel afterward; only real DB errors return non-nil and roll back.
	var outcome error
	err := s.writeTx(func(tx *sql.Tx) error {
		res, err := tx.Exec(
			`UPDATE oauth_refresh_tokens SET consumed_at = ? WHERE token_hash = ? AND consumed_at IS NULL AND expires_at > ?`,
			now.Unix(), hashToken(token), now.Unix())
		if err != nil {
			return fmt.Errorf("store: consume refresh: %w", err)
		}
		n, err := res.RowsAffected()
		if err != nil {
			return fmt.Errorf("store: consume refresh: %w", err)
		}
		if n == 1 {
			return tx.QueryRow(
				`SELECT client_id, person_id, device_id, client_type, resource
				 FROM oauth_refresh_tokens WHERE token_hash = ?`, hashToken(token)).
				Scan(&g.ClientID, &g.PersonID, &g.DeviceID, &g.ClientType, &g.Resource)
		}
		// n == 0: unknown/expired, OR a replay of a consumed token. A consumed
		// row present → replay: revoke the family and signal it.
		var pid, cid string
		qerr := tx.QueryRow(
			`SELECT person_id, client_id FROM oauth_refresh_tokens
			 WHERE token_hash = ? AND consumed_at IS NOT NULL`, hashToken(token)).Scan(&pid, &cid)
		if errors.Is(qerr, sql.ErrNoRows) {
			outcome = ErrRefreshInvalid
			return nil // nothing written; commit is a no-op
		}
		if qerr != nil {
			return fmt.Errorf("store: consume refresh: %w", qerr)
		}
		// ponytail: family = person × client (coarse). Reuse forces full re-auth
		// for that person on that client; a per-chain family_id would narrow the
		// blast radius if that ever proves too aggressive.
		if _, err := tx.Exec(
			`UPDATE oauth_refresh_tokens SET consumed_at = ?
			 WHERE person_id = ? AND client_id = ? AND consumed_at IS NULL`,
			now.Unix(), pid, cid); err != nil {
			return fmt.Errorf("store: revoke refresh family: %w", err)
		}
		outcome = ErrRefreshReused
		return nil // commit the family revoke, then surface ErrRefreshReused below
	})
	if err != nil {
		return RefreshGrant{}, err
	}
	if outcome != nil {
		return RefreshGrant{}, outcome
	}
	return g, nil
}

// SweepOAuth deletes expired authorization codes and expired refresh tokens.
// Called from the retention sweeper (T-09); returns rows removed, for logging.
func (s *Store) SweepOAuth(now time.Time) (int64, error) {
	var total int64
	err := s.writeTx(func(tx *sql.Tx) error {
		for _, q := range []string{
			`DELETE FROM oauth_codes WHERE expires_at <= ?`,
			`DELETE FROM oauth_refresh_tokens WHERE expires_at <= ?`,
		} {
			res, err := tx.Exec(q, now.Unix())
			if err != nil {
				return fmt.Errorf("store: sweep oauth: %w", err)
			}
			n, _ := res.RowsAffected()
			total += n
		}
		return nil
	})
	return total, err
}
