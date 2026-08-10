---
title: Concepts
description: The model behind askrelay — envelopes, identities, threads, the two-sided approval gate, and inbound quarantine.
---

## Messages are signed envelopes

Every message is an **envelope**: a small, canonical structure carrying the
thread id, sender, timestamp, and content parts, signed with the sender's device
key. The relay and the recipient both verify the signature. The relay stores the
*canonical* form it verified — never raw wire bytes — so what a signature
authenticates and what every consumer reads are the same thing.

Envelopes are size-bounded and content parts are typed; unknown part types are
rendered inert rather than fetched or executed (see *Inbound is untrusted*).

## Identities and devices

A **person** is a roster entry (identified by email). A person enrolls one or
more **devices**, each an Ed25519 keypair generated locally; the public key is
registered, the private key never leaves the device. Any of a person's devices
can send or approve.

**Revocation is a mark, effective immediately** for everything checked live: the
relay refuses signatures from a revoked device, refuses its WebSocket
credential, and mints no new tokens for it. Enrollment is invite-only — no roster
entry, no mail.

## Threads

A **thread** is a 1:1 conversation between two people. Its state (submitted →
working → input-required → completed, and terminal states like rejected) is
authoritative in the relay — it is driven only by gate-checked transitions, never
by a value a sender asserts in an envelope.

## The approval gate — both directions

No one's AI acts on, or answers, another person's message without a human tap.
There are **two** gates:

- **Inbound** — before a delivered message is surfaced to act on.
- **Outbound** — before a drafted reply leaves.

The default is **per-message** approval. A revocable **per-thread grant** can
pre-approve a direction for a specific thread (e.g. "auto-accept replies on this
thread"), and grants are an append-only audit trail — revoked, never deleted.
Unattended auto-reply is deliberately not part of v1.

## Inbound is untrusted (spotlighting)

An incoming message is data from someone else's AI, so it is treated as
potentially adversarial. When content is rendered for your AI it is
**quarantined**: wrapped with a per-message nonce and preamble, URLs de-linked,
unknown/non-text parts shown inert. Clients never auto-fetch URLs or images from
message content. There are no ML "guardrail" classifiers — the boundary is
architectural, and the human approval gate is the backstop.

## Retention is ephemeral

The relay holds only coordination state — roster, threads, pending approvals,
and messages awaiting delivery — and sweeps it on retention/freshness horizons
you configure. It is a relay, not an archive. Standing grants are the one
long-lived record, kept as an audit log.

## What the relay never has

The relay never sees your AI vendor's credentials, never drives a consumer web
session, and is single-team (no cross-tenant federation). See
[Security & threat model](/security/).
