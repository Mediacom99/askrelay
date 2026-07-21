# CLAUDE.md — askrelay

## Project overview

askrelay is an open-source tool for async, approval-gated messaging between
people's AI sessions ("Your AI can ask my AI"): developer A's Claude Code
session sends a question to colleague B; B approves; B's AI answers with its
own context — no human copy-paste. Architecture: a thin self-hostable Go relay
(single binary + SQLite) serving Streamable HTTP MCP + OAuth 2.1 directly
(claude.ai/ChatGPT connect with zero install), plus an optional Go daemon that
upgrades Claude Code with push and in-terminal approvals. Kosmoy colleagues are
the first users; OSS (Apache-2.0 + DCO, no CLA) from day one.

## Sources of truth (read in this order)

1. [`docs/askrelay-architecture.md`](docs/askrelay-architecture.md) — design.
2. [`docs/askrelay-implementation-plan.md`](docs/askrelay-implementation-plan.md)
   — execution: work packages, **verified dependency pins (§3)**, technical
   decisions (T-xx), risks, and the §2 session protocol. The plan wins over
   the architecture doc where they differ.
3. [`docs/phase2-decision-log.md`](docs/phase2-decision-log.md) — product
   verdicts (D-01..D-18) binding both.
4. [`docs/research/`](docs/research/README.md) — evidence briefs; every key
   fact live-verified and independently re-checked.

## How work happens here

- **Implementation sessions follow the plan's §2 protocol** — one work
  package per session, status board as claim marker, learnings appended. The
  copy-paste kickoff prompt is in that section.
- The pipeline is planner + subagents: the main session (Fable) plans and
  orchestrates; the Sonnet subagents in `.claude/agents/` do the labor —
  `askrelay-dev` (implements one WP), `askrelay-test` (independent adversarial
  test pass), `askrelay-review` (fresh-context diff review), and
  `askrelay-security` (red-team lens on security-touching WPs: envelope, gate,
  oauth, mcp, daemon, retention). The orchestrator runs dev → test → review
  (+ security where it applies) per WP and loops until all pass.
- `go.mod` intentionally lists no dependencies; `cmd/askrelay/main.go` and the
  `internal/*/doc.go` files are buildable placeholders citing their
  architecture sections. Dependencies enter only at the plan §3 pins, only
  when a WP first imports them.

## Standing constraints (full rationale in the decision log)

- Approval gates both directions (per-message default, revocable per-thread
  grants); no code path may bypass them. Unattended auto-reply is v1.1,
  API-key-only, pending Anthropic ToS clarification.
- Inbound = untrusted data: spotlighting, no tool triggering, clients never
  auto-fetch URLs/images from message content. Client-side secret redaction;
  ephemeral relay retention. No ML guardrail classifiers.
- MCP: client-initiated tool calls only; stateless Streamable HTTP; protocol
  2025-11-25 via go-sdk v1.6.1 until spike S-03 says otherwise. No sampling,
  no server-push assumptions.
- Relay never sees vendor credentials; never drive a consumer web session
  (vendor ToS).
- Go 1.26, `CGO_ENABLED=0` everywhere; single-binary distribution.

## Common commands

Via the Makefile (`make help`): `make build` / `make run ARGS="..."` /
`make test` / `make tidy` / `make lint` / `make overview`.

CI (`.github/workflows/ci.yml`): build, vet, test, golangci-lint.

## Documentation upkeep

`docs/askrelay-overview.html` is generated — **never edit by hand**. After
editing any markdown doc listed in `docs/tools/build-overview.py` (or
README.md / this file), run `make overview` and commit the regenerated HTML in
the same commit.
