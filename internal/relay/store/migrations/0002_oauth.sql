-- Schema v2 (WP-06): embedded OAuth 2.1 authorization-server state. Access and
-- device tokens stay STATELESS (WP-05, EdDSA JWTs verified offline); only the
-- AS's own short-lived and rotating state lives here. All tables STRICT; times
-- are INTEGER unix seconds (UTC), matching 0001. Only SHA-256 hashes of codes
-- and refresh tokens are stored — a leaked DB cannot replay either.

-- Dynamically-registered clients (RFC 7591 / DCR — claude.ai's path). CIMD
-- clients (ChatGPT) are deliberately NOT stored: their client_id IS an HTTPS
-- metadata URL, checked structurally at authorize time (same-origin redirect).
CREATE TABLE oauth_clients (
    client_id     TEXT PRIMARY KEY,          -- opaque, relay-minted (uuidv7)
    redirect_uris TEXT NOT NULL,             -- JSON array of exact-match redirect URIs
    client_name   TEXT NOT NULL DEFAULT '',
    created_at    INTEGER NOT NULL
) STRICT;

-- Authorization codes: single-use, short-lived (~60s), PKCE-bound. consumed_at
-- flips exactly once via a conditional UPDATE (the single-use race guard, as
-- invites do). code_challenge is always S256 (method is not stored: a plain
-- challenge is rejected at /authorize, never persisted — PKCE downgrade guard).
CREATE TABLE oauth_codes (
    code_hash      TEXT PRIMARY KEY,          -- hex SHA-256 of the authorization code
    client_id      TEXT NOT NULL,
    redirect_uri   TEXT NOT NULL,             -- must match at token exchange
    code_challenge TEXT NOT NULL,             -- PKCE S256 challenge (base64url)
    resource       TEXT NOT NULL DEFAULT '',  -- RFC 8707 resource indicator as sent at authorize
    person_id      TEXT NOT NULL REFERENCES persons(id),
    device_id      TEXT NOT NULL REFERENCES devices(id),
    client_type    TEXT NOT NULL DEFAULT '',  -- §5.4 profile driver, carried into the token
    created_at     INTEGER NOT NULL,
    expires_at     INTEGER NOT NULL,
    consumed_at    INTEGER                    -- NULL until exchanged
) STRICT;

-- Refresh tokens: rotating, hashed, person+device+client bound. On use the row
-- is consumed and a successor issued (rotation). Presenting an ALREADY-consumed
-- token is the RFC 6819 / OAuth 2.1 replay signal → the whole family
-- (person × client) is revoked. The device is re-checked live at each refresh.
-- Go methods for this table arrive in subtask 2; the schema ships here (one
-- atomic migration).
CREATE TABLE oauth_refresh_tokens (
    token_hash   TEXT PRIMARY KEY,            -- hex SHA-256 of the refresh token
    client_id    TEXT NOT NULL,
    person_id    TEXT NOT NULL REFERENCES persons(id),
    device_id    TEXT NOT NULL REFERENCES devices(id),
    client_type  TEXT NOT NULL DEFAULT '',
    resource     TEXT NOT NULL DEFAULT '',
    created_at   INTEGER NOT NULL,
    expires_at   INTEGER NOT NULL,
    consumed_at  INTEGER                       -- NULL = live; set when rotated or revoked
) STRICT;
CREATE INDEX idx_oauth_refresh_family
    ON oauth_refresh_tokens(person_id, client_id) WHERE consumed_at IS NULL;
