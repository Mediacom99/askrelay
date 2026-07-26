-- Schema v1 (WP-03). All tables STRICT: SQLite enforces column types
-- instead of coercing. Times are INTEGER unix seconds (UTC) so retention
-- arithmetic stays in SQL. Vocabulary CHECKs mirror internal/a2a and
-- internal/gate as defense-in-depth: the gate is the authority; the DB
-- refuses what no code path should ever write.

-- The roster (arch §4.3): no row, no mail.
CREATE TABLE persons (
    id         TEXT PRIMARY KEY,              -- uuidv7
    email      TEXT NOT NULL UNIQUE,
    label      TEXT NOT NULL DEFAULT '',      -- display name
    created_at INTEGER NOT NULL
) STRICT;

-- Ed25519 device keys; several per person; any may approve (arch §4.3).
-- Revocation is a mark, effective immediately — rows are never deleted.
CREATE TABLE devices (
    id         TEXT PRIMARY KEY,              -- uuidv7
    person_id  TEXT NOT NULL REFERENCES persons(id),
    pubkey     BLOB NOT NULL UNIQUE,          -- raw 32-byte Ed25519 public key
    label      TEXT NOT NULL DEFAULT '',
    created_at INTEGER NOT NULL,
    revoked_at INTEGER                        -- NULL = active
) STRICT;
CREATE INDEX idx_devices_person ON devices(person_id);

-- Single-use enrollment invites. Only the token's SHA-256 is stored: a
-- leaked database cannot mint enrollments. used_at flips exactly once via
-- a conditional UPDATE (the single-use race guard).
CREATE TABLE invites (
    token_hash TEXT PRIMARY KEY,              -- hex SHA-256 of the invite token
    email      TEXT NOT NULL,
    created_at INTEGER NOT NULL,
    expires_at INTEGER NOT NULL,
    used_at    INTEGER                        -- NULL until consumed
) STRICT;

-- Threads are 1:1 in v1 (D-05: no groups). state is THE authoritative A2A
-- state (F3): transitions happen only through the store's gate-calling
-- methods, never from an envelope's asserted state.
CREATE TABLE threads (
    id           TEXT PRIMARY KEY,            -- uuidv7 (the envelope thread id)
    initiator_id TEXT NOT NULL REFERENCES persons(id),
    recipient_id TEXT NOT NULL REFERENCES persons(id),
    state        TEXT NOT NULL CHECK (state IN
                   ('submitted','working','input-required','completed',
                    'failed','canceled','rejected')),
    created_at   INTEGER NOT NULL,
    updated_at   INTEGER NOT NULL,
    CHECK (initiator_id <> recipient_id)
) STRICT;

-- Inbound messages. envelope is the CANONICAL form + signature (the
-- re-marshaled decoded struct) — never raw wire bytes (arch §4.1 invariant).
-- id is the SIGNED envelope id: the replay/dedup key (WP-01 F1/F2).
-- via_grant is the inbound audit mark (WP-02: the store records that the
-- grant path, not a human tap, approved it).
CREATE TABLE messages (
    id          TEXT PRIMARY KEY,
    thread_id   TEXT NOT NULL REFERENCES threads(id),
    sender_id   TEXT NOT NULL REFERENCES persons(id),
    envelope    BLOB NOT NULL,
    sent_at     INTEGER NOT NULL,             -- sender-asserted; bounded both ways at ingest
    received_at INTEGER NOT NULL,             -- relay clock; retention math keys on this
    via_grant   INTEGER NOT NULL DEFAULT 0 CHECK (via_grant IN (0, 1))
) STRICT;
CREATE INDEX idx_messages_thread   ON messages(thread_id);
CREATE INDEX idx_messages_received ON messages(received_at);

-- Replay identity that OUTLIVES body deletion: the sweeper deletes a message
-- row, the tombstone stays behind. sent_at is the SIGNED sender clock (the
-- same value the ingest freshness check compares) — NOT the relay's — so the
-- tombstone can be pruned exactly when a replay carrying that sent_at would be
-- refused as stale anyway, with no dependence on relay-clock skew (see
-- retention.go PruneTombstones).
CREATE TABLE message_tombstones (
    id      TEXT PRIMARY KEY,
    sent_at INTEGER NOT NULL
) STRICT;

-- Per-recipient-device delivery/ack state — T-09's "all devices acked"
-- input. Dies with its message (CASCADE is the one delete path).
CREATE TABLE deliveries (
    message_id   TEXT NOT NULL REFERENCES messages(id) ON DELETE CASCADE,
    device_id    TEXT NOT NULL REFERENCES devices(id),
    delivered_at INTEGER,
    acked_at     INTEGER,
    PRIMARY KEY (message_id, device_id)
) STRICT, WITHOUT ROWID;
CREATE INDEX idx_deliveries_unacked ON deliveries(device_id) WHERE acked_at IS NULL;

-- Outbound replies awaiting release: the gate.DraftState lifecycle made
-- durable. state spellings are gate's; via_grant mirrors Release.ViaGrant().
CREATE TABLE drafts (
    id         TEXT PRIMARY KEY,              -- uuidv7; becomes the released envelope's id
    thread_id  TEXT NOT NULL REFERENCES threads(id),
    author_id  TEXT NOT NULL REFERENCES persons(id),
    envelope   BLOB NOT NULL,                 -- canonical draft body (unsigned until release)
    state      TEXT NOT NULL CHECK (state IN ('pending_review','sent','discarded')),
    via_grant  INTEGER NOT NULL DEFAULT 0 CHECK (via_grant IN (0, 1)),
    created_at INTEGER NOT NULL,
    decided_at INTEGER                        -- when released or discarded
) STRICT;
CREATE INDEX idx_drafts_thread ON drafts(thread_id);

-- Standing grants: IMMORTAL audit rows (arch §4.1 — "never deleted, they
-- are the audit log"). Revocation marks; history accumulates. The partial
-- unique index enforces at most ONE ACTIVE grant per thread × person ×
-- direction while allowing any number of revoked predecessors.
CREATE TABLE grants (
    id         TEXT PRIMARY KEY,              -- uuidv7
    thread_id  TEXT NOT NULL REFERENCES threads(id),
    person_id  TEXT NOT NULL REFERENCES persons(id),
    direction  TEXT NOT NULL CHECK (direction IN ('inbound','outbound')),
    granted_at INTEGER NOT NULL,
    revoked_at INTEGER                        -- NULL = active
) STRICT;
CREATE UNIQUE INDEX idx_grants_one_active
    ON grants(thread_id, person_id, direction) WHERE revoked_at IS NULL;

-- oauth_* tables deliberately absent: WP-05 owns their design and ships
-- them as 0002_oauth.sql (plan §6, moved 2026-07-24).
