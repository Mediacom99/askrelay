---
title: Security & threat model
description: What the relay sees and doesn't, the approval model, revocation, redaction, and responsible disclosure.
---

askrelay connects AI sessions across people, so it is built
**authenticated-but-never-benign**: every party is identified, and every message
is still treated as potentially adversarial. The controls are architectural, not
heuristic.

## What the relay sees — and doesn't

- **Never** your AI vendor's credentials. Clients authenticate to the relay with
  their own device credential / OAuth token; the relay is not in the vendor auth
  path.
- **Ephemerally**, only coordination state (roster, threads, pending approvals,
  in-flight messages), swept on the horizons you configure.
- It never drives a consumer web session, and it is single-team — no
  cross-tenant federation.

## The approval model

Both directions are human-gated (see [Concepts](/concepts/)): inbound before a
message is acted on, outbound before a reply leaves. Per-message by default;
per-thread grants are revocable and audit-logged. No unattended auto-reply in v1.

## Identity, tokens, and revocation

- Per-device **Ed25519** signatures, verified at the relay *and* the recipient.
- Access tokens are **short-lived (≤1h) and single-audience**, signed EdDSA JWTs
  (no algorithm agility — EdDSA only, so no algorithm-confusion forgery).
- **Revocation is immediate** for signatures, the WebSocket credential, and new
  token issuance; the relay re-checks the device on every token mint and refresh.
  An already-issued access token lingers only until it expires.

## Untrusted inbound (spotlighting)

Message content is quarantined when rendered for your AI: per-message nonce +
preamble, de-linked URLs, inert non-text parts, no client auto-fetch. Whitelisted
part types only. No ML guardrail classifiers — the human gate is the backstop.

## Client-side secret redaction

Outbound content is scanned for common secret shapes and redacted with visible
markers **before it leaves your machine**. This is a best-effort safety net, not
a guarantee — bare high-entropy blobs and secrets embedded in prose are accepted
ceilings; the approval gate is the real backstop.

## Known limits (honest)

- **Redaction** is pattern-based, not an entropy/NLP detector — see above.
- **Device-credential login (Option A):** the OAuth "login" is pasting your
  device credential, which folds authentication and consent into one step. A
  look-alike phishing page that harvests a pasted credential is the sharpest
  residual risk; a two-step explicit-consent screen is planned. Treat your
  device credential like a password.
- **Open registration:** the DCR endpoint is unauthenticated by spec — operators
  **must** front it with a rate limit (see [Self-host](/self-host/)).

## Reporting a vulnerability

Please report privately via the repository's **`SECURITY.md`** (GitHub private
vulnerability reporting). For a consent-and-messaging product, security response
is a first-order commitment: reports are acknowledged quickly and handled under
coordinated disclosure. Do not open a public issue for a suspected vulnerability.
