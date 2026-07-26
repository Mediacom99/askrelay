# Deploying askrelay (self-host)

The relay is a single static binary (`CGO_ENABLED=0`) plus one SQLite file. TLS
is the reverse proxy's job; the relay listens on localhost by default.

> Status: WP-04 skeleton. `/healthz` and `POST /enroll/{token}` are live. The
> MCP surface (`/mcp`), OAuth (`/oauth/*`), and the delivery WebSocket (`/ws`)
> land in later work packages.

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
5. **Enroll a device** (the colleague's client does this; shown here with curl
   for the walkthrough): generate an Ed25519 keypair, then
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

## Files here

- `askrelay.service` — systemd unit stub
- `Dockerfile` — static build + minimal runtime image
- `Caddyfile` — reverse-proxy + automatic TLS example

Backup = copy the SQLite file — though it mostly holds *transient* messages by
design (ephemeral retention, arch §4.2).
