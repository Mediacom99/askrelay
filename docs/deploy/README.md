# Deploying askrelay (self-host)

The relay is a single static binary (`CGO_ENABLED=0`) plus one SQLite file. TLS
is the reverse proxy's job; the relay listens on localhost by default.

> Status: the relay core is complete — `/healthz`, enrollment
> (`POST /enroll/{token}`), the MCP surface (`/mcp`), the embedded OAuth 2.1
> authorization server (`/oauth/*`, `/.well-known/*`), and the delivery
> WebSocket (`/ws`) are all live. Release packaging (published Docker image,
> prebuilt binaries) is finalized in WP-14; the `Dockerfile` beside this file
> builds today.

**Docker is the default deploy** (D-25): build the `Dockerfile` here and run it
on whatever host you like — a Coolify/Dokku-class PaaS terminates TLS for you, so
the `Caddyfile` below is only needed for the no-container route. Whatever you
use, give the container **one persistent volume covering both the SQLite file and
`signing.key`**; losing either loses every enrollment.

## Fresh-VPS walkthrough → "device enrolled"

Only the binary and a reverse proxy are required.

1. **Build / fetch the binary** and put it on the host:
   ```
   CGO_ENABLED=0 go build -o askrelay ./cmd/askrelay
   ```
2. **Run the relay** (behind the proxy, on localhost):
   ```
   ASKRELAY_BASE_URL=https://relay.example.com \
   ASKRELAY_DB=/var/lib/askrelay/askrelay.db \
   ./askrelay serve
   ```
   or via flags: `./askrelay serve -base-url https://relay.example.com -db /var/lib/askrelay/askrelay.db`
3. **Front it with TLS** — see `Caddyfile` (Caddy auto-provisions certificates).
4. **Mint an invite** for a colleague (run on the relay host; opens the DB
   directly):
   ```
   ./askrelay invite marco@example.com -base-url https://relay.example.com -db /var/lib/askrelay/askrelay.db
   # → https://relay.example.com/enroll/<token>
   ```
   Deliver that URL out-of-band (Slack/email — your existing trusted channel).
5. **Enroll a device** — on the colleague's own machine:
   ```
   askrelay enroll -name "Marco" "https://relay.example.com/enroll/<token>"
   ```
   That writes `~/.config/askrelay/config.json` and the device key, both 0600.
   **The config file holds a long-lived device credential** — it is a secret at
   rest, because `askrelay daemon` and `askrelay mcp` authenticate to the relay
   with it. Back it up like a key, not like a config.

   The raw HTTP call the verb makes, for reference: generate an Ed25519 keypair,
   then
   ```
   curl -sX POST https://relay.example.com/enroll/<token> \
     -H 'content-type: application/json' \
     -d '{"pubkey":"<base64-std of the 32-byte public key>","label":"laptop"}'
   # → {"person_id":"…","device_id":"…","base_url":"https://relay.example.com"}
   ```
   A `200` with a `device_id` is **"device enrolled"** — the exit criterion.

## Configuration (flags + env only — T-11)

| Flag | Env | Default | Meaning |
|---|---|---|---|
| `-listen` | `ASKRELAY_LISTEN` | `127.0.0.1:8080` | listen address |
| `-db` | `ASKRELAY_DB` | `askrelay.db` | SQLite path |
| `-base-url` | `ASKRELAY_BASE_URL` | — (required) | public https URL |
| `-invite-ttl` | `ASKRELAY_INVITE_TTL` | `24h` | invite lifetime |
| `-ack-grace` | `ASKRELAY_ACK_GRACE` | `72h` | delete acked messages this long after last ack |
| `-hard-ttl` | `ASKRELAY_HARD_TTL` | `720h` | delete any message this long after receipt |
| `-fresh-max-age` | `ASKRELAY_FRESH_MAX_AGE` | `24h` | reject/forget messages older than this |
| `-fresh-max-skew` | `ASKRELAY_FRESH_MAX_SKEW` | `5m` | reject messages this far in the future |

Retention/freshness durations have a **1h floor** (T-09); `serve` refuses to
start below it. Precedence is flag > env > default.

## Rate limiting the OAuth endpoints (required for browser connectors)

Dynamic Client Registration (`POST /oauth/register`) is **unauthenticated by the
OAuth spec** — a client must be able to register before it holds any token. The
relay caps the resulting growth (abandoned client rows are pruned after ~30 days,
store `SweepOAuth`), but that only bounds the *total*; you **must** also
rate-limit the endpoint at the proxy before exposing the browser-connector path
publicly, or an anonymous caller can still churn registrations. Example (nginx):

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

Caddy needs the `caddy-ratelimit` module (build with `xcaddy`); Traefik has a
`rateLimit` middleware. Until the endpoint is rate-limited, keep the relay on
localhost/LAN.

## Backup

Back up two things together:

- **The SQLite database** — take a *consistent* snapshot, not a raw `cp` of a
  live WAL-mode file: `sqlite3 askrelay.db ".backup '/backup/askrelay.db'"` (or
  `VACUUM INTO`). It mostly holds *transient* messages by design (ephemeral
  retention, arch §4.2).
- **`signing.key`** (beside the DB) — the Ed25519 token-signing key. **Losing it
  invalidates every device credential and access token** (everyone must
  re-enroll); **leaking it enables token forgery / account takeover.** Back it up
  encrypted and preserve its `0600` permissions.

## Files here

- `askrelay.service` — systemd unit stub
- `Dockerfile` — static build + minimal runtime image
- `Caddyfile` — reverse-proxy + automatic TLS example
