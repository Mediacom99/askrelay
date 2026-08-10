---
title: Self-host & operate
description: Deploy the relay behind TLS, configure retention, rate-limit the OAuth endpoints, and back up what matters.
---

The relay is one static binary plus one SQLite file. It listens on localhost by
default; **TLS is the reverse proxy's job.** localhost, LAN, and a public VPS are
the same code — the difference is only where you point the proxy.

## Run

```sh
CGO_ENABLED=0 go build -o askrelay ./cmd/askrelay

ASKRELAY_BASE_URL=https://relay.example.com \
ASKRELAY_DB=/var/lib/askrelay/askrelay.db \
./askrelay serve
```

`base-url` is required and must be the public `https://` URL clients reach — it
is the OAuth issuer/audience and the base for enrollment links, so it must match
what the proxy serves.

## TLS via reverse proxy

Terminate TLS at Caddy, nginx, or Traefik and proxy to the relay on localhost.
Caddy auto-provisions certificates; a minimal `Caddyfile` ships in the
repository's `docs/deploy/`.

## Configuration (flags + env only)

Precedence is flag > env > default. Retention/freshness durations have a **1-hour
floor**; `serve` refuses to start below it.

| Flag | Env | Default | Meaning |
|---|---|---|---|
| `-listen` | `ASKRELAY_LISTEN` | `127.0.0.1:8080` | listen address |
| `-db` | `ASKRELAY_DB` | `askrelay.db` | SQLite path |
| `-base-url` | `ASKRELAY_BASE_URL` | *(required)* | public https URL |
| `-signing-key` | `ASKRELAY_SIGNING_KEY` | `signing.key` beside the DB | Ed25519 token key |
| `-invite-ttl` | `ASKRELAY_INVITE_TTL` | `24h` | invite lifetime |
| `-ack-grace` | `ASKRELAY_ACK_GRACE` | `72h` | delete acked messages this long after last ack |
| `-hard-ttl` | `ASKRELAY_HARD_TTL` | `720h` | delete any message this long after receipt |
| `-fresh-max-age` | `ASKRELAY_FRESH_MAX_AGE` | `24h` | reject/forget messages older than this |
| `-fresh-max-skew` | `ASKRELAY_FRESH_MAX_SKEW` | `5m` | reject messages this far in the future |

## Rate-limit the OAuth endpoints (required for browser connectors)

Dynamic Client Registration (`POST /oauth/register`) is **unauthenticated by the
OAuth spec** — a client must register before it holds a token. The relay caps the
resulting growth (abandoned client rows are pruned after ~30 days), but you
**must** also rate-limit that endpoint at the proxy before exposing the
browser-connector path publicly. Example (nginx):

```nginx
limit_req_zone $binary_remote_addr zone=oauth_reg:10m rate=6r/m;
server {
    location = /oauth/register {
        limit_req zone=oauth_reg burst=3 nodelay;
        proxy_pass http://127.0.0.1:8080;
    }
    location / { proxy_pass http://127.0.0.1:8080; }
}
```

Caddy needs the `caddy-ratelimit` module; Traefik has a `rateLimit` middleware.
Until the endpoint is rate-limited, keep the relay on localhost/LAN.

## Backup

Back up **two** things together:

- **The SQLite database** — a *consistent* snapshot, not a raw copy of a live
  WAL file: `sqlite3 askrelay.db ".backup '/backup/askrelay.db'"`. It mostly
  holds transient messages by design (ephemeral retention).
- **`signing.key`** — the Ed25519 token-signing key beside the DB. **Losing it
  forces everyone to re-enroll; leaking it enables token forgery.** Back it up
  encrypted and keep its `0600` permissions.

## Operating notes

- **Revocation is immediate** for signatures, the WebSocket credential, and new
  token issuance; an already-issued access token lasts until it expires (≤1h).
- **Health:** `GET /healthz` returns status + version for your load balancer.
- **Logs** are structured (`slog`) and never contain message content, secrets,
  or tokens — ids and shapes only.
- systemd unit and Dockerfile stubs ship in the repository's `docs/deploy/`.
