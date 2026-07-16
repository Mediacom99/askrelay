# askrelay — v1 Implementation Plan

**Status:** execution source of truth. Design rationale lives in
[`askrelay-architecture.md`](askrelay-architecture.md) (§ references below);
product verdicts in [`phase2-decision-log.md`](phase2-decision-log.md) (D-xx).
Where a work-package entry differs from the architecture doc, the entry wins and
cites a decision. Dependency pins in §3 were verified against live sources on
**2026-07-16**.

## 1. Status board

### Work packages

| WP | Package | Status | Depends on | Gated by |
|---|---|---|---|---|
| WP-01 | `internal/envelope` — signed envelope | TODO | — | T-01 T-02 |
| WP-02 | `internal/gate` — approval/grant state machine | TODO | — | — |
| WP-03 | `internal/relay/store` — SQLite persistence | TODO | WP-01 | T-03 T-09 |
| WP-04 | relay HTTP skeleton + enrollment | TODO | WP-03 | T-11 T-14 T-15 |
| WP-05 | relay OAuth: resource server + tokens | TODO | WP-04 | T-06 |
| WP-06 | relay OAuth: embedded AS + client registration | TODO | WP-05 | T-06 |
| WP-07 | relay MCP surface (tools + spotlighting) | TODO | WP-01 WP-02 WP-03 WP-05 | T-07 T-08 T-10 |
| WP-08 | WS hub, delivery, retention sweeper | TODO | WP-03 WP-04 | T-04 T-09 |
| WP-09 | daemon core (enroll, queue, stdio MCP) | TODO | WP-01 WP-07 WP-08 WP-11 | T-11 |
| WP-10 | daemon ↔ Claude Code push (channels + hooks) | TODO | WP-09 | S-01 |
| WP-11 | `internal/redact` — secret redaction | TODO | — | T-12 |
| WP-12 | CLI verbs (inbox, approve, device, status) | TODO | WP-04 WP-08 | T-05 |
| WP-13 | end-to-end harness + golden flows | TODO | WP-06 WP-07 WP-08 WP-09 | — |
| WP-14 | packaging (Docker, GoReleaser, brew, npm wrapper) | TODO | WP-13 | T-13 |
| WP-15 | docs & security finalization | TODO | WP-13 | — |
| WP-16 | hosted-demo hardening | TODO | WP-14 WP-15 | checklist Gate 0 done (D-08) |

Spikes (timeboxed, produce a Learnings entry + possibly decision revisions):

| Spike | Question | Due |
|---|---|---|
| S-01 | Channels research preview: does the daemon's `claude/channel` stdio server inject into a live session with acceptable flag/allowlist friction? (arch §6) | before WP-10 |
| S-02 | Real long-poll ceilings per client for `wait_for_activity` (arch §5.4; the ~28 h Claude Code figure is unverified) | before WP-07 exit |
| S-03 | MCP 2026-07-28 final release + go-sdk v1.7.0 stable: what changes for us? | ~2026-07-28 |
| S-04 | Live ChatGPT connector validation on the team's actual plans (write-MCP gating, Plus behavior) | with WP-13 |

### Decisions (summary — full entries in §5)

Product D-01..D-18: all APPROVED (see phase2-decision-log.md). Technical
T-01..T-15: recorded below as **PROPOSED** pending maintainer verdicts; WPs
gated on a T-entry may not start until it is APPROVED.

## 2. Execution protocol (agent pipeline)

askrelay is built by a **planner + subagent pipeline**: the main session
(Fable) plans and orchestrates; Sonnet subagents in `.claude/agents/` do the
labor. Roles: `askrelay-dev` (implements one WP), `askrelay-test`
(independently exercises the WP against its test plan — writes the tests the
dev agent didn't think of), `askrelay-review` (fresh-context diff review
against the WP entry + cited architecture sections). The roster files define
each role's contract; **creating the roster is a prerequisite to WP-01** — no
implementation session starts before `.claude/agents/` exists.

**For every implementation session:**

1. Read `CLAUDE.md`, then this document. Do not re-derive from chat history —
   this document plus the architecture sections each WP cites are the full
   context.
2. **Pick work:** lowest-numbered WP with Status `TODO`, all *Depends on*
   `DONE`, all *Gated by* decisions `APPROVED` (and gating spikes done). A gate
   may also name a launch-checklist condition; it counts only when that item
   is checked off with evidence. One WP per session — they are sized to one
   reviewable PR.
3. Set Status `IN_PROGRESS` in §1 and commit the edit (claim marker).
4. Orchestrate: `askrelay-dev` implements exactly what the entry says →
   `askrelay-test` runs the full test plan and adds adversarial tests →
   `askrelay-review` reviews the diff against entry + architecture §§. The
   planner fixes or loops agents until all three pass.
5. Add dependencies **only** at §3 pins, only when first imported
   (`go get module@pin`).
6. Green bar before PR: `make build && make test && make lint && go vet ./...`
   plus both build-tag sets once WP-14 lands.
7. Update the status board (`DONE`, PR/commit refs). Append to §7 Learnings
   anything that changes a later WP — and edit that WP in the same commit.
8. New design questions become PROPOSED T-entries, never silent choices. Wire
   format, security posture, retention, or anything user-visible **waits** for
   APPROVED; internal code-structure choices may proceed as PROPOSED.
9. Standing constraints (never violate): approval gates cannot be bypassed in
   code paths (D-03/D-11); inbound content is data (D-10, arch §5.2); relay
   never sees vendor credentials (D-06); `go.mod` gains nothing unpinned; one
   PR per WP; the tree stays green.

**Kickoff prompt (copy-paste to start an implementation session):**

```
Read CLAUDE.md, then docs/askrelay-implementation-plan.md. Following its §2
protocol, pick the next eligible work package, set it IN_PROGRESS, and drive
the askrelay-dev / askrelay-test / askrelay-review agents through it per its
entry and the architecture sections it cites. Green bar, status board update,
learnings, one PR. One package only. If nothing is eligible, report exactly
what blocks.
```

**Maintainer workflow:** review §5 T-entries, append APPROVED/VETOED (+reason).
A veto blocks dependent WPs until a REVISED entry exists.

## 3. Verified dependency matrix

Ground truth as of **2026-07-16**, each row fetched from releases/pkg.go.dev/
license files that day (workflow `verify-dependency-pins`). `go.mod` stays
empty until each WP's first import.

| Module | Pin | Enters at | Verified facts that matter |
|---|---|---|---|
| `github.com/modelcontextprotocol/go-sdk` | **v1.6.1** (2026-05-22) | WP-05 (auth helpers), WP-07 (server) | Protocol 2025-11-25 (+ 3 older) in `mcp/shared.go`. LICENSE is a tri-license (new code Apache-2.0, legacy MIT, docs CC-BY-4.0) — that's why GitHub says NOASSERTION; all permissive. `auth` pkg ships `RequireBearerToken` + RFC 9728 PRM handler, but `TokenVerifier` is BYO → T-06. v1.7.0-pre.2 (2026-07-09) targets 2026-07-28 spec; adopt v1.7.0 stable via S-03, not before. |
| `modernc.org/sqlite` | **v1.54.0** (2026-07-15) | WP-03 | Pure-Go (transpiled amalgamation), BSD-3, wraps SQLite 3.53.3; 10 releases in 4 months. Keeps `CGO_ENABLED=0` true; mattn/go-sqlite3 still needs cgo (re-verified). |
| `github.com/coder/websocket` | **v1.8.15** (2026-06-15) | WP-08 (relay), WP-09 (daemon) | ISC, zero deps, canonical successor to nhooyr.io/websocket (old repo 301s here). Import path must be `github.com/coder/websocket`. |
| `github.com/golang-jwt/jwt/v5` | **v5.3.1** (2026-01-28) | WP-05 | MIT. EdDSA-signed, audience-bound access tokens (T-06); pairs with go-sdk's BYO `TokenVerifier`. |
| `github.com/google/uuid` | latest at import (verify then) | WP-01 | UUIDv7 for envelope/thread IDs (T-02). Verify tag + license at WP-01 and record here. |
| `golang.org/x/crypto` | not needed in v1 | — | stdlib `crypto/ed25519` covers signing (arch §3). If ever imported: v0.54.0 (2026-07-08, BSD-3) was current; avoid deprecated `openpgp` (GO-2026-5932). |
| ~~`github.com/spf13/cobra`~~ | not adopted | — | v1.10.2 healthy (Apache-2.0); not adopted per T-05 (PROPOSED — stdlib `flag` + subcommand dispatch); revisit only if verb count sprawls or T-05 is vetoed. |

Release/CI tooling (not go.mod): Go **1.26.5** (2026-07-07) toolchain;
GoReleaser **v2.17.0** (2026-07-04, MIT); golangci-lint **v2.12.2**
(2026-05-06) via `golangci/golangci-lint-action@v9` (v9.3.0; the pre-scaffold
@v6 pin was fixed in Phase 4 — see §7).

**MCP protocol:** target revision **2025-11-25** (current released; the
2026-07-28 revision is an RC due in ~2 weeks that removes sessions/initialize
and deprecates Roots+Sampling+Logging — it *confirms* our stateless,
client-initiated-calls design; S-03 handles adoption).

## 4. Dependency graph

The §1 status-board *Depends on* column is authoritative; this list restates it
as build-order tracks (a WP is eligible when everything left of it is DONE):

- **Track A (pure libraries, parallel roots):** WP-01 envelope · WP-02 gate ·
  WP-11 redact — no dependencies; can all run first, in any order.
- **Track B (relay spine):** WP-01 → WP-03 store → WP-04 http → WP-05 RS →
  WP-06 AS.
- **Track C (surfaces):** {WP-01, WP-02, WP-03, WP-05} → WP-07 MCP surface;
  {WP-03, WP-04} → WP-08 ws/delivery; {WP-04, WP-08} → WP-12 CLI.
- **Track D (daemon):** {WP-01, WP-07, WP-08, WP-11} → WP-09 daemon →
  (+ spike S-01) WP-10 Claude Code push.
- **Convergence:** {WP-06, WP-07, WP-08, WP-09} → WP-13 e2e → WP-14 packaging
  and WP-15 docs → {WP-14, WP-15} → WP-16 hosted (launch-gated, D-08).

## 5. Decision log

Product decisions **D-01..D-18**: recorded with APPROVED verdicts in
[`phase2-decision-log.md`](phase2-decision-log.md); they are imported here by
reference and bind every WP.

### Technical decisions (maintainer verdict pending where PROPOSED)

- **T-01 — Envelope wire format: JSON + RFC 8785 (JCS) canonical form for
  signing.** MCP's wire language, human-debuggable, no gossip-style size
  pressure; JCS gives deterministic bytes for Ed25519 without a second codec.
  Rejected: CBOR (second codec for no win here), protobuf (schema toolchain
  tax). *PROPOSED*
- **T-02 — IDs: UUIDv7 (`github.com/google/uuid`).** Time-sortable (inbox
  ordering, index locality), standard, one tiny dep. Rejected: ULID lib
  (equivalent, less standard), stdlib-only random UUIDs (no ordering).
  *PROPOSED*
- **T-03 — SQLite via `modernc.org/sqlite`.** Pure-Go keeps CGO_ENABLED=0 and
  trivial cross-compilation (D-09). WAL mode; single-writer discipline via a
  store-level mutex; `busy_timeout` set. *PROPOSED*
- **T-04 — WebSocket via `github.com/coder/websocket`.** Maintained canonical
  fork, ISC, zero deps. *PROPOSED*
- **T-05 — CLI: stdlib `flag` + hand-rolled subcommand dispatch.** ~10 verbs,
  no dynamic completion needs in v1; keeps the dep tree at 5 modules. Cobra
  re-considered if verbs sprawl (matrix row kept). *PROPOSED*
- **T-06 — Tokens: EdDSA-signed JWTs (`golang-jwt/jwt/v5`), audience-bound
  (RFC 8707 resource), 1 h access / refresh at AS; device WS credential is a
  distinct long-lived token bound to the device record.** go-sdk `auth`
  supplies PRM + bearer plumbing; verification callback is ours. *PROPOSED*
- **T-07 — MCP: go-sdk v1.6.1, stateless Streamable HTTP mode, protocol
  2025-11-25; migrate to v1.7.0/2026-07-28 only via S-03 outcome.** *PROPOSED*
- **T-08 — Spotlighting format as specified in arch §5.2** (fresh nonce per
  rendering, static warning preamble, URLs as plain text, provenance line).
  Golden-tested in WP-13. *PROPOSED*
- **T-09 — Retention defaults: delete on all-devices-ack + 72 h grace; hard
  TTL 30 days; grants/audit rows immortal** (D-10). Operator-tunable, floor
  1 h. *PROPOSED*
- **T-10 — `wait_for_activity` caps: daemon n/a (WS); claude.ai 240 s; ChatGPT
  45 s; Claude Code remote 25 min default** — revised by S-02 evidence, not by
  hope. *PROPOSED*
- **T-11 — Config: relay = flags + env only; daemon/user =
  `~/.config/askrelay/config.json` (0600) + key file beside it** (arch §7).
  *PROPOSED*
- **T-12 — Redaction v1 pattern set:** AWS access keys, GCP service-account
  JSON markers, GitHub `ghp_`/`gho_`/fine-grained `github_pat_`, Slack `xox*`,
  generic `-----BEGIN … PRIVATE KEY-----` blocks, JWT triple-dot shape, and
  `.env`-style `KEY=value` lines for names matching `(TOKEN|SECRET|KEY|
  PASSWORD)`. Visible `⟦redacted:<kind>⟧` markers; sender warned; no entropy
  heuristics (false-positive machine). Hook: executable at
  `redact_hook` config path, receives text on stdin, returns replacement.
  *PROPOSED*
- **T-13 — Dev/prod build split retained** (`-tags dev`: verbose tracing,
  localhost debug endpoint; prod artifacts contain neither; CI builds both).
  *PROPOSED*
- **T-14 — HTTP routing: stdlib `net/http` ServeMux (1.22+ method patterns).**
  No router dep. *PROPOSED*
- **T-15 — Logging: stdlib `log/slog`, JSON handler on the relay, text on the
  daemon; no third-party logger.** *PROPOSED*

## 6. Work packages

### WP-01 — `internal/envelope`

**Status:** TODO · **Depends on:** — · **Gated by:** T-01 T-02 ·
**Spec:** arch §3, §12.

**Goal:** the envelope as the single shared artifact: types, JCS
canonicalization, Ed25519 sign/verify, size caps, append-only versioning.

**Files:** `internal/envelope/{envelope.go,canon.go,sign.go}` + tests +
`fuzz_test.go`.

**Dependency added:** `github.com/google/uuid` — pin per its §3 row: verify
latest tag + license at import and record the result in the matrix.

**Key API:**
```go
type Envelope struct { V int; ID, Thread string; From Party; To string;
    State ThreadState; SentAt time.Time; AIGenerated bool; Body Message;
    Sig []byte }
type Party struct { Person, Device, Agent string }
type Message struct { Role string; Parts []Part }        // arch §3
type Part struct { Type string; Text string }            // "text" only in v1
const MaxBodyBytes = 32 << 10
func New(...) Envelope                                   // UUIDv7 ids
func Canonical(e Envelope) ([]byte, error)               // RFC 8785, sig omitted
func Sign(e *Envelope, priv ed25519.PrivateKey) error
func Verify(e Envelope, pub ed25519.PublicKey) error     // sig + caps + version
var ErrTooLarge, ErrBadVersion, ErrBadSignature error
```

**Test plan:** round-trip; canonical stability across field order & unicode;
signature tamper matrix (every field); size-cap edges; fuzz Decode/Canonical;
cross-check one golden JCS vector against the RFC 8785 example set.

**Exit:** another package can build a signed message and verify a tampered one
fails, with zero non-stdlib deps beyond `google/uuid`.

### WP-02 — `internal/gate`

**Status:** TODO · **Depends on:** — · **Gated by:** — ·
**Spec:** arch §5.3; D-03, D-11.

**Goal:** the approval/grant state machine, pure and I/O-free: approving an
inbound message transitions its thread `input-required → working`, declining
transitions it `→ rejected` (A2A-verbatim hyphenated names on the wire and in
audit rows, whatever the Go identifiers are — arch §3, §5.3); outbound drafts
`pending_review → sent | discarded`; per-thread × direction grants with
`via_grant` audit marks.

**Files:** `internal/gate/{gate.go,states.go}` + exhaustive table-driven tests.

**Exit:** every legal and illegal transition is table-tested; grants
short-circuit exactly one gate direction; no way to construct a "sent" without
approval-or-grant in the type surface.

### WP-03 — `internal/relay/store`

**Status:** TODO · **Depends on:** WP-01 · **Gated by:** T-03 T-09 ·
**Spec:** arch §4.1, §4.2, §4.3.

**Goal:** SQLite persistence: embedded-SQL schema + migrations; persons,
devices (revocation), invites (single-use, TTL), threads, messages
(per-device delivery/ack), grants (immortal), oauth tables; retention sweeper
queries (ack+grace, hard TTL).

**Dependency added:** `modernc.org/sqlite v1.54.0`.

**Test plan:** migration idempotency; revoked-device refusal; invite
single-use race; sweeper boundary cases (partial acks, grace edges, hard TTL
overrides un-fetched); grants survive message deletion.

### WP-04 — relay HTTP skeleton + enrollment

**Status:** TODO · **Depends on:** WP-03 · **Gated by:** T-11 T-14 T-15 ·
**Spec:** arch §4.3, §4.5, §7, §11.

**Goal:** `askrelay serve`: config from flags/env; ServeMux routes
(`/healthz`, `POST /enroll/{token}`); invite creation
(`askrelay invite` against admin socket or local DB); slog; graceful
shutdown; systemd/docker docs stubs.

**Exit:** a fresh VPS walkthrough (documented) reaches "device enrolled" with
only the binary and a reverse proxy.

### WP-05 — relay OAuth: resource server

**Status:** TODO · **Depends on:** WP-04 · **Gated by:** T-06 ·
**Spec:** arch §4.4.

**Goal:** RFC 9728 PRM endpoint (go-sdk `auth` handler), bearer validation
middleware (EdDSA JWT, audience = relay base URL, person-identity claims),
token issuance internals, device-credential tokens for `/ws`.

**Dependencies added:** `github.com/modelcontextprotocol/go-sdk v1.6.1`,
`github.com/golang-jwt/jwt/v5 v5.3.1`.

**Test plan:** RFC 8707 audience mismatch rejected; expired/not-yet-valid;
revoked device's tokens die; PRM document matches spec examples.

### WP-06 — relay OAuth: embedded AS + client registration

**Status:** TODO · **Depends on:** WP-05 · **Gated by:** T-06 ·
**Spec:** arch §4.4.

**Goal:** authorization-code + PKCE flow whose "login" is invite-token/device
credential entry (no passwords, no signup); DCR endpoint (claude.ai path);
CIMD acceptance (ChatGPT path); JWKS; refresh tokens.

**Test plan:** full code+PKCE happy path scripted; PKCE downgrade attacks
rejected; DCR'd and CIMD clients both reach a working token; state/nonce
handling; a claude.ai and ChatGPT connector each complete auth in S-04.

### WP-07 — relay MCP surface

**Status:** TODO · **Depends on:** WP-01 WP-02 WP-03 WP-05 · **Gated by:**
T-07 T-08 T-10 · **Spec:** arch §5 (all), §9.

**Goal:** the nine §5.1 tools on go-sdk stateless Streamable HTTP:
`send_message`, `check_inbox`, `get_thread`, `approve_message`/
`decline_message`, `approve_reply`/`discard_reply`, `set_thread_grant`,
`wait_for_activity` (profile caps); `readOnlyHint` annotations; terse
descriptions; `structuredContent` responses; spotlighting renderer (nonce,
preamble, URL de-linking) for every content-bearing response.

**Test plan:** per-tool table tests through a real go-sdk client; spotlight
golden files; a "malicious message" suite (fake closing tags, tool-invoking
prose, URLs) verifying rendered output keeps the quarantine intact; profile
cap enforcement; every call completes < 45 s with a ChatGPT-profile token.

### WP-08 — WS hub, delivery, retention sweeper

**Status:** TODO · **Depends on:** WP-03 WP-04 · **Gated by:** T-04 T-09 ·
**Spec:** arch §4.2, §4.5, §6.

**Goal:** `/ws` (device-credential auth): push new-mail/approval events,
accept outbound envelopes, track per-device acks; resend-on-reconnect;
retention sweeper goroutine wired to T-09 knobs.

**Dependency added:** `github.com/coder/websocket v1.8.15`.

**Test plan:** kill/reconnect matrix (nothing lost, nothing duplicated beyond
at-least-once + id dedupe); ack bookkeeping vs sweeper; revocation severs live
sockets.

### WP-09 — daemon core

**Status:** TODO · **Depends on:** WP-01 WP-07 WP-08 WP-11 · **Gated by:**
T-11 · **Spec:** arch §6, §7.

**Goal:** `askrelay daemon`: enroll flow writing config+key (0600); WS client
with jittered reconnect; offline outbound queue; redaction before signing;
stdio MCP server exposing the same §5.1 toolset for Codex/local clients; OS
notification emit (macOS/Linux).

**Test plan:** offline→online queue flush; redaction applied before signature
(tamper check proves order); stdio server passes the WP-07 tool table run
locally.

### WP-10 — daemon ↔ Claude Code push

**Status:** TODO · **Depends on:** WP-09 · **Gated by:** spike S-01 ·
**Spec:** arch §6 (push paths).

**Goal:** channels bridge (stdio MCP server with `claude/channel` capability
emitting `notifications/claude/channel` on mail) behind a config flag with the
research-preview caveats printed loudly; `askrelay hooks install`
(UserPromptSubmit additionalContext one-liner + settings snippet, idempotent,
documented uninstall).

**Test plan:** S-01-derived integration script against a live Claude Code
session (manual, documented); hooks template lints against the current hooks
schema; both paths degrade gracefully when Claude Code is absent.

### WP-11 — `internal/redact`

**Status:** TODO · **Depends on:** — · **Gated by:** T-12 · **Spec:** D-12,
arch §5.3, §6.

**Goal:** the T-12 pattern set with visible `⟦redacted:<kind>⟧` markers,
sender warning surface, and the external-hook runner; relay-side backstop mode
(flag, don't rewrite).

**Test plan:** corpus of true positives per pattern; false-positive corpus
(UUIDs, git SHAs, base64 blobs) stays untouched; hook contract (stdin/stdout,
timeout, failure = block send, never silently pass).

### WP-12 — CLI verbs

**Status:** TODO · **Depends on:** WP-04 WP-08 · **Gated by:** T-05 ·
**Spec:** arch §7.

**Goal:** `inbox`, `approve`/`decline` (both gates), `device list|revoke`,
`status`, `version` — thin calls over the device-credential HTTP/WS API;
stdlib flag dispatch.

**Test plan:** golden output tests; exit codes; WP-12 spins up its own minimal
in-process relay fixture for tests (WP-13 later adopts and extends that
fixture — it is the seed of the e2e harness, stated here so the dependency
direction stays WP-12 → WP-13).

### WP-13 — end-to-end harness + golden flows

**Status:** TODO · **Depends on:** WP-06 WP-07 WP-08 WP-09 · **Gated by:** — ·
**Spec:** arch §2, §4.4, §5, §6; D-01..D-05.

**Goal:** in-process relay + two fake devices driving the full loop:
question → inbound approval → drafted reply → outbound review → delivery →
acks → sweeper deletion. Includes the injection suite end-to-end, the
grant-path variants, and adversarial runs against the WP-05/06 auth surface
(R-06: PKCE downgrade, audience confusion, revoked-device tokens). This is the
tree's conscience; it runs in CI.

**Exit:** `go test ./e2e/...` proves the D-03/D-11 gates cannot be skipped by
any tool sequence.

### WP-14 — packaging

**Status:** TODO · **Depends on:** WP-13 · **Gated by:** T-13 · **Spec:**
arch §10, §11; D-09; oss-adoption-dynamics.md.

**Goal:** Dockerfile (distroless, `serve` default); GoReleaser v2.17.0 config
(darwin/linux/windows × amd64/arm64, brew tap); thin npm wrapper package
(`npx askrelay` fetches the platform binary); dev/prod tag builds in CI;
install docs matching D-09's priority order.

### WP-15 — docs & security finalization

**Status:** TODO · **Depends on:** WP-13 · **Gated by:** — · **Spec:** D-15,
D-16; arch §8, §9.

**Goal:** threat-model document (from arch §8 + security.md, incident-cited);
per-client setup guides (Claude Code / claude.ai / ChatGPT / Codex with the
honest matrix); README final pass against the working product; SECURITY.md
full version; AUP for the future hosted instance drafted.

### WP-16 — hosted-demo hardening

**Status:** TODO · **Depends on:** WP-14 WP-15 · **Gated by:** launch-checklist
Gate 0 fully checked (the §2 checklist-condition rule; D-08) ·
**Spec:** arch §11; D-08; D-16.

**Goal:** multi-team isolation audit, per-IP/per-team rate limits, abuse
contact + takedown path, terms page, backup/rotation runbook. Exit = the
public "try it in 60 seconds" relay of D-08/D-16.

## 7. Learnings log

*(append-only; every entry names the WPs it changed)*

- 2026-07-16 — Plan drafted. Pin sweep found CI's golangci-lint-action two
  majors stale (@v6 → @v9): fixed in the Phase 4 re-scaffold commit, affects
  no WP. The 2026-07-28 MCP RC deprecates Roots/Sampling/Logging and removes
  sessions — reinforces T-07/S-03; no WP change.
- 2026-07-16 — Five-reviewer fresh-context pass (arch §15 records it): WP-13
  gained WP-06 as a dependency + adversarial auth runs; WP-02 state names
  hyphenated to A2A-verbatim; WP-01 gained its uuid dependency line; WP-12
  now owns the in-process relay fixture that WP-13 adopts; WP-16's gate made
  checklist-checkable; R-10 (DCR churn) added; §4 graph replaced by track
  list (the ASCII art disagreed with the authoritative table in four places).

## 8. Risk register

| # | Risk | Exposure | Mitigation / trigger |
|---|---|---|---|
| R-01 | ChatGPT connector regressions (documented ~quarterly) break the asker flow | Cross-vendor demo dies at a bad moment | S-04 before launch; troubleshooting page (chatgpt-field.md); relay logs per-client tool-call success |
| R-02 | Channels stays preview / dev-flag-gated / allowlist-closed | Push UX on Claude Code degraded to hooks | Hooks path is first-class, not fallback-shaped (WP-10 ships both); S-01 before WP-10; track graduation |
| R-03 | Anthropic answers "approved relay ≠ ordinary usage" | v1.1 unattended mode dies; v1 unaffected (human taps) | Question already queued (Phase 2 action item); v1 ships without unattended mode by design |
| R-04 | MCP 2026-07-28 final diverges from RC; go-sdk v1.7.0 slips | Rework in WP-07 if adopted early | Pin 2025-11-25 (T-07); S-03 gates any migration |
| R-05 | Solo maintainer + agent pipeline = bus factor 1 | Project stalls on maintainer absence | Docs-as-source-of-truth discipline (§2); D-14 org migration once colleagues adopt |
| R-06 | OAuth AS implementation bugs (the scariest code we own) | Account takeover on a security product | WP-05/06 get the harshest review loop; e2e auth attacks in WP-13; consider external audit pre-launch (checklist) |
| R-07 | SQLite single-writer contention at team scale | Latency blips | WAL + short transactions (T-03); team-sized load test in WP-13 |
| R-08 | Squatted `askrelay.com` causes brand confusion | Mild, cosmetic | askrelay.dev owned (D-18); revisit .com post-traction (D-17) |
| R-09 | A vendor ships first-party cross-person session messaging | Niche compresses (prior-art.md: Claude Tag expansion) | Phase 6 market research watches this; our moat is cross-vendor + self-hosted + OSS |
| R-10 | DCR removed from the final MCP auth story before claude.ai migrates to CIMD | WP-06 registration path churn | Ship both DCR + CIMD (arch §4.4); S-03 re-checks the auth chapter of the 2026-07-28 final |
