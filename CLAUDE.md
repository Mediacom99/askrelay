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
   verdicts (D-01..D-23) binding both.
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
- Dependencies enter only at the plan §3 pins, only when a WP first imports
  them. `go.mod` now carries the five pins those WPs pulled in (`go-sdk`,
  `modernc.org/sqlite`, `coder/websocket`, `golang-jwt/v5`, `google/uuid`) —
  all matching §3 exactly. Every package is implemented; `internal/daemon` is
  live through WP-09 ST-6 — listener + desktop notification, the stdio MCP proxy
  (`askrelay mcp`), redaction, and sign-at-release — with the offline queue
  (ST-7) and the quality pass (ST-8) outstanding.
- **R-12 as it now stands.** `internal/redact` and `envelope.Sign`/`Verify` have
  live callers at last, but only on the **daemon-mediated path** (`askrelay
  mcp`). A client connected straight to the relay over HTTPS still sends
  unredacted text and gets relay-attested delivery, because no daemon is in that
  path to redact or sign — structural, not a gap to close; see the T-19
  amendment. `store.RevokeDevice` got its CLI in `ca5718c`. So never write
  "askrelay redacts and signs" flatly: it does when the sender runs the daemon,
  and recipients are told which attestation each message carries. Signature
  verification is the **relay's**, never the recipient's (T-21).

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

### The public site must not lag a push

**Commit locally as much as you like — nothing is required.** But **before
`git push`**, check whether the pushed commits change anything
[docs.askrelay.dev](https://docs.askrelay.dev) states, and update the
[`askrelay-docs`](https://github.com/Mediacom99/askrelay-docs) repo in the same
sitting if so. That site auto-deploys on push to its `main`, so a stale claim
goes public the moment it merges — and it is a security product, where a wrong
claim about signing, redaction, or revocation is worse than no claim.

Push touches the docs whenever it changes any of:

- a CLI verb, a `serve` flag, or a default
- an HTTP route, an MCP tool, or a tool annotation
- a thread/draft state transition, a retention or freshness horizon, or a size cap
- a work-package status — a WP closing usually turns a `<Readiness>` block into
  plain prose, or `partial` into `shipped`
- anything listed in risk **R-12** becoming reachable on the shipped path

Then, in `askrelay-docs`: fix the affected pages, update their `<Readiness>`
blocks and the `readiness:` frontmatter mirror, and re-stamp every page's
`askrelayVersion` / `askrelayCommit` / `verifiedOn`. The procedure — including
how to derive the version and why a SHA beats an invented tag — is under
"Re-stamping the docs" in that repo's README.md. `npm run build` enforces the
stamp and the readiness rules, so a half-done update fails rather than ships.

Standing rule from R-12: **never describe a control as live before its WP
closes** — not here, not in README.md, not on the site.

This is enforced, not just documented. Run **`make hooks`** once per clone to
enable `.githooks/pre-push`, which blocks a push whose documented surface has
moved ahead of the docs' stamp. It fires only on files that define something the
site states — and for the implementation plan, only when the §1 work-package
status board actually changes, so ordinary learnings-log and risk-register edits
pass. Bypass with `git push --no-verify` or `ASKRELAY_SKIP_DOCS_CHECK=1` when a
push genuinely changes nothing the site states; if you find yourself bypassing
often, the trigger list in the hook is wrong — narrow it rather than habituating
to the escape hatch.
