# CLAUDE.md — askrelay

## Project overview

askrelay is an open-source tool for async, approval-gated messaging between
people's AI sessions ("Your AI can ask my AI"): developer A's Claude Code
session sends a question to colleague B; B approves; B's AI answers with its
own context — no human copy-paste. Architecture: a thin self-hostable Go relay
(single binary + SQLite) that serves Streamable HTTP MCP + OAuth 2.1 directly
(claude.ai/ChatGPT connect with zero install), plus an optional Go daemon that
upgrades Claude Code with push and in-terminal approvals. Kosmoy colleagues are
the first users; OSS (Apache-2.0 + DCO, no CLA) from day one.

## Current status

**Decided, not yet documented or implemented.** Every design decision is
recorded with a maintainer verdict in
[`docs/phase2-decision-log.md`](docs/phase2-decision-log.md) (D-01..D-18),
grounded in the live-verified research briefs under
[`docs/research/`](docs/research/README.md). Next: write the architecture doc,
implementation plan, launch checklist, full README, and the `.claude/agents/`
roster — application code starts only after those land and only per the plan's
work packages.

`go.mod` intentionally lists no dependencies; `cmd/askrelay/main.go` is a
buildability placeholder. Dependencies are added when the first work package
imports them, at versions pinned in the implementation plan.

## Key decided constraints (full rationale in the decision log)

- Async mailbox; DMs + threads only; A2A-aligned signed envelopes (Ed25519,
  per-device keys from invite-link enrollment).
- Approval gates both directions: per-message default, revocable per-thread
  grants; unattended auto-reply deferred to v1.1 (API-key-only, pending
  Anthropic ToS clarification).
- Claude Code first-class; claude.ai/ChatGPT as remote-MCP senders with
  honestly-documented pull-only answering; Codex like Claude Code minus push.
- Inbound = untrusted data: spotlighting, no tool triggering, clients never
  auto-fetch URLs/images from message content. Client-side secret redaction
  (built-in patterns + hook). Relay retention is ephemeral (delete after
  delivery-ack). No ML guardrail classifiers.
- Go everywhere in v1. Relay never sees vendor credentials; never drive a
  consumer web session; unattended modes must use API keys (vendor ToS).
- Design MCP integration for client-initiated tool calls only (no sampling, no
  server-push assumptions); the relay handles MCP statelessly.

## Common commands

Run via the Makefile (`make help` lists targets): `make build` / `make run
ARGS="..."` / `make test` / `make tidy` / `make lint` / `make overview`.

CI (`.github/workflows/ci.yml`) runs build, vet, test, and golangci-lint.

## Documentation

- [`docs/phase2-decision-log.md`](docs/phase2-decision-log.md) — all decisions,
  with verdicts; **the source of truth until the architecture doc lands**.
- [`docs/research/`](docs/research/README.md) — decision briefs; every key fact
  was verified against a live source and independently re-checked.
- `docs/askrelay-architecture.md`, `docs/askrelay-implementation-plan.md`,
  `docs/askrelay-launch-checklist.md` — being written next; they become the
  sources of truth for design and execution respectively.
- `docs/askrelay-overview.html` — generated single-page reading copy.
  **Generated — never edit by hand.** After editing any markdown doc listed in
  `docs/tools/build-overview.py` (or README.md / this file), run
  `make overview` and commit the regenerated HTML in the same commit.
