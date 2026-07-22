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

- **Work is maintainer-supervised (D-22)** — follow the plan's §2 protocol. The
  main session (Fable) plans and implements *in dialogue with the maintainer*,
  who supervises every step and must understand every line. Per WP: divide it
  into small logical subtasks; for each subtask, explain what/why/Go-idioms and
  show the proposed code, **wait for the maintainer's confirmation, then write
  it** — never write ahead of confirmation. The copy-paste kickoff prompt is in
  §2.
- Implementation is **not** delegated to an autonomous agent (that hides the
  reasoning the maintainer needs to see). The Sonnet subagents in
  `.claude/agents/` are an **independent quality pass, run once per WP after all
  subtasks are assembled**: `askrelay-test` (adversarial), `askrelay-review`
  (fresh-context), `askrelay-security` (red-team on security-touching WPs).
  Their findings return to the maintainer for a fix decision. (`askrelay-dev`
  was retired at D-22.)
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
- Errors (T-17): exported sentinel values matched with `errors.Is`; propagate by
  wrapping `fmt.Errorf("<pkg>: …: %w", err)`; zero-value+error, never both; panic
  only for unrecoverable faults. Pure core (`a2a`/`envelope`/`gate`) returns
  errors and never logs; only boundaries log. Remote-facing errors are sanitized.
- Logging (T-15/T-18): `slog` only; NEVER log content, secrets, keys, tokens, or
  PII — ids and shapes only; structured fields (not formatted strings); log at
  boundaries; security refusals at `Warn` without the offending content.

## Common commands

Via the Makefile (`make help`): `make build` / `make run ARGS="..."` /
`make test` / `make tidy` / `make lint` / `make overview`.

CI (`.github/workflows/ci.yml`): build, vet, test, golangci-lint.

## Documentation upkeep

`docs/askrelay-overview.html` is generated — **never edit by hand**. After
editing any markdown doc listed in `docs/tools/build-overview.py` (or
README.md / this file), run `make overview` and commit the regenerated HTML in
the same commit.
