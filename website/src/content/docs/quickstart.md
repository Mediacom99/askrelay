---
title: Quickstart
description: Run a relay, enroll a device, connect an AI client, and reach your first tools — in a few minutes.
---

This gets a relay running and one AI client connected. The full "ask → approve →
answer" loop needs a second party (or a second identity of your own — see
[Use cases](/use-cases/)); this page gets you to *connected, with tools*.

:::note
v1 CLI verbs today are `serve`, `invite`, and `version`. A one-command
`enroll` and human-facing `inbox`/`approve` verbs arrive with the daemon and CLI
work packages; until then, enrollment is a single HTTP call and approvals happen
through your AI client's tools.
:::

## 1. Build the binary

Go 1.26+. The shipped binary is a single static executable (no cgo):

```sh
CGO_ENABLED=0 go build -o askrelay ./cmd/askrelay
```

## 2. Run the relay

It listens on localhost by default; TLS is the reverse proxy's job (see
[Self-host & operate](/self-host/)). For a local try-out:

```sh
./askrelay serve -base-url http://127.0.0.1:8080 -db ./askrelay.db
```

## 3. Mint an invite

Enrollment is invite-gated — no open signup. Run this on the relay host (it opens
the SQLite file directly):

```sh
./askrelay invite you@example.com -base-url http://127.0.0.1:8080 -db ./askrelay.db
# → http://127.0.0.1:8080/enroll/<token>
```

Deliver that URL to the person out-of-band (Slack, email — your existing trusted
channel). It is single-use and expires.

## 4. Enroll a device

Enrolling generates an Ed25519 keypair, registers the public key, and returns a
long-lived **device credential** (the thing you'll paste to connect a client):

```sh
curl -sX POST http://127.0.0.1:8080/enroll/<token> \
  -H 'content-type: application/json' \
  -d '{"pubkey":"<base64-std of the 32-byte Ed25519 public key>","label":"laptop"}'
# → {"person_id":"…","device_id":"…","base_url":"…","device_credential":"…"}
```

Keep `device_credential` — it is how you authenticate a browser client in the
next step.

## 5. Connect an AI client

Add the relay as a remote MCP server in your client (claude.ai, ChatGPT, or
Claude Code). The client walks the OAuth flow, shows the relay's login page, and
you **paste your device credential** to authorize. Full per-client steps are in
[Connect a client](/connect-a-client/).

Once connected, the client can call the askrelay tools (`send_message`,
`check_inbox`, `approve_message`, …). To exercise a real round trip, connect a
second identity or a colleague and follow a scenario in [Use cases](/use-cases/).

## Next

- [Connect a client](/connect-a-client/) — per-client setup
- [Self-host & operate](/self-host/) — TLS, config, backup, rate limiting
- [Concepts](/concepts/) — how the approval gate and identities work
