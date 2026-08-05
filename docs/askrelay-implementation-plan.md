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
| WP-01 | `internal/envelope` — signed envelope | DONE | — | T-01 T-02 T-16 S-05 ✓ |
| WP-02 | `internal/gate` — approval/grant state machine | DONE | — | — |
| WP-03 | `internal/relay/store` — SQLite persistence | DONE | WP-01 | T-03 T-09 |
| WP-04 | relay HTTP skeleton + enrollment | DONE | WP-03 | T-11 T-14 T-15 |
| WP-05 | relay OAuth: resource server + tokens | DONE | WP-04 | T-06 |
| WP-06 | relay OAuth: embedded AS + client registration — **off the critical path (D-23)**, taken with browser-client signing when the browser-connector path is wanted | TODO | WP-05 | T-06 |
| WP-07 | relay MCP surface (tools + spotlighting) | DONE | WP-01 WP-02 WP-03 WP-05 | T-07 T-08 T-10 |
| WP-08 | WS hub, delivery, retention sweeper | DONE | WP-03 WP-04 | T-04 T-09 |
| WP-09 | daemon core (enroll, queue, stdio MCP) | TODO | WP-01 WP-07 WP-08 WP-11 | T-11 |
| WP-10 | daemon ↔ Claude Code push (channels + hooks) | TODO | WP-09 | S-01 |
| WP-11 | `internal/redact` — secret redaction | DONE | — | T-12 |
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
| S-03 | MCP 2026-07-28 final release + go-sdk v1.7.0 stable: what changes for us? | ~2026-07-28 — **OVERDUE, unrun as of 2026-08-03**; whether the revision and v1.7.0 stable actually shipped is unverified, so §3's pin stays at go-sdk v1.6.1 / protocol 2025-11-25 until this spike runs |
| S-04 | Live ChatGPT connector validation on the team's actual plans (write-MCP gating, Plus behavior) | with WP-13 |
| S-05 | askmesh autopsy + differentiation memo (D-19/C1) | **DONE 2026-07-20** — verdict **PROCEED** (kill criterion 1 does not fire: askmesh was closed-source, unlisted, never launched — invisibility, not rejection). Two findings carried forward: demand is now *unproven not disproven* (no positive signal), and the cold-start/retention hazard is inherited (→ new risk R-11; sharpened kill criterion 3). Differentiation memo: askmesh automates answering (closed Claude-only cloud, no inbound-trust model); askrelay makes consented, safe, cross-vendor asking the product. Full writeup: `docs/research/askmesh-autopsy.md` |

### Decisions (summary — full entries in §5)

Product D-01..D-23: all APPROVED (see phase2-decision-log.md; D-19..D-23 were
added 2026-07-16..07-31 — market verdict, commercialization posture, strategic
framing, this supervised process, and the local-first/self-talk framing).
Technical
T-01..T-15: all **APPROVED** (maintainer, 2026-07-16 — walked through and
approved one by one). T-16 (whole-artifact size caps) APPROVED 2026-07-22 from
the WP-01 test pass. T-17 (error-handling convention) + T-18 (logging policy)
APPROVED 2026-07-23 — cross-cutting, bind every WP from WP-03 on. Future
T-entries start as PROPOSED; WPs gated on a T-entry may not start until APPROVED.

## 2. Execution protocol (maintainer-supervised)

askrelay is built **maintainer-supervised** (D-22): the main session (Fable)
plans and implements *in dialogue with the maintainer*, who supervises every
step and must understand every line as if they wrote it. Implementation happens
in the main conversation — **not** delegated to an autonomous agent — because a
subagent's reasoning is invisible to the maintainer, which defeats the goal.
The Sonnet subagents in `.claude/agents/` serve as an **independent quality
pass, not as implementers**: `askrelay-test` (adversarial testing),
`askrelay-review` (fresh-context review), and `askrelay-security` (red-team on
security-touching WPs). (`askrelay-dev` was retired at D-22 — recoverable from
git history if an autonomous mode is ever wanted again.)

**For every work package:**

1. Read `CLAUDE.md`, then this document. The WP entry plus the architecture
   sections it cites are the full contract.
2. **Pick work:** lowest-numbered WP with Status `TODO`, all *Depends on*
   `DONE`, all *Gated by* decisions `APPROVED` (and gating spikes done); a gate
   may name a launch-checklist condition, which counts only when checked off
   with evidence. One WP at a time.
3. Set Status `IN_PROGRESS` in §1 and commit the claim marker on a feature
   branch.
4. **Divide the WP into small, logically-coherent subtasks** (a cohesive
   type / concept / file) and present the subtask list to the maintainer.
5. **For each subtask, in order — the supervised loop:**
   a. *Explain* what it does, *why this design* (and the alternatives
      rejected), the security/architecture reasoning, and any Go idiom worth
      knowing (assume the maintainer reads Go fluently — teach the "why", not
      the language).
   b. *Show* the proposed code.
   c. *Wait* for the maintainer's confirmation or redirection. **Never write
      code ahead of confirmation.**
   d. On confirmation, write exactly that code.
6. Add dependencies **only** at §3 pins, only when first imported.
7. When all subtasks are assembled and green
   (`make build && make test && make lint && go vet ./...`, both build-tag sets
   once WP-14 lands), run the **quality pass**: `askrelay-test`,
   `askrelay-review`, and `askrelay-security` (when the WP touches signatures,
   approval state, tokens, untrusted content, secrets, or auth). Bring every
   finding to the maintainer; route fixes back through the step-5 loop.
8. Update the status board (`DONE`, PR ref); append to §7 Learnings anything
   that changes a later WP (edit that WP in the same commit); one PR into
   `staging`.
9. New design questions become PROPOSED T-entries surfaced to the maintainer;
   wire-format, security-posture, retention, or anything user-visible **waits**
   for APPROVED.
10. Standing constraints (never violate): approval gates cannot be bypassed in
    code paths (D-03/D-11); inbound content is data (D-10, arch §5.2); relay
    never sees vendor credentials (D-06); `go.mod` gains nothing unpinned; the
    tree stays green.

**Kickoff prompt (copy-paste to start a work package):**

```
Read CLAUDE.md, then docs/askrelay-implementation-plan.md. Following its §2
maintainer-supervised protocol, pick the next eligible work package, set it
IN_PROGRESS on a feature branch, and divide it into small logical subtasks.
Then for each subtask: explain what/why/Go-idioms, show the proposed code, and
WAIT for my confirmation before writing it. When the WP is assembled and green,
run the askrelay-test / askrelay-review / askrelay-security quality pass and
bring me the findings. One package; one PR into staging.
```

**Maintainer workflow:** you are in the loop at every subtask (confirm/redirect
the shown code) and at every T-entry (append APPROVED/VETOED + reason in §5).
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
| `github.com/google/uuid` | **v1.6.0** (latest tag) | WP-01 | BSD-3-Clause; `uuid.NewV7()` present. UUIDv7 for envelope/thread IDs (T-02). Verified + pinned at WP-01 (2026-07-22); the only non-stdlib dependency in `internal/envelope`. |
| `golang.org/x/crypto` | not needed in v1 | — | stdlib `crypto/ed25519` covers signing (arch §3). If ever imported: v0.54.0 (2026-07-08, BSD-3) was current; avoid deprecated `openpgp` (GO-2026-5932). |
| ~~`github.com/spf13/cobra`~~ | not adopted | — | v1.10.2 healthy (Apache-2.0); not adopted per T-05 (APPROVED — stdlib `flag` + subcommand dispatch); revisit only if verb count sprawls. |

Release/CI tooling (not go.mod): Go **1.26.5** (2026-07-07) toolchain;
GoReleaser **v2.17.0** (2026-07-04, MIT); golangci-lint **v2.12.2**
(2026-05-06) via `golangci/golangci-lint-action@v9` (v9.3.0; the pre-scaffold
@v6 pin was fixed in Phase 4 — see §7).

**MCP protocol:** target revision **2025-11-25** — the revision go-sdk v1.6.1
speaks, and what WP-07 shipped against. The 2026-07-28 revision removes
sessions/initialize and deprecates Roots+Sampling+Logging, so it *confirms* our
stateless, client-initiated-calls design. **Its target date has now passed**
(the "RC due in ~2 weeks" wording above was written 2026-07-16); whether that
revision and go-sdk v1.7.0 stable actually shipped is **unverified here** — the
overdue S-03 owns that check, and the pin does not move before it runs.

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

Product decisions **D-01..D-23**: recorded with APPROVED verdicts in
[`phase2-decision-log.md`](phase2-decision-log.md); they are imported here by
reference and bind every WP.

### Technical decisions

- **T-01 — Envelope wire format: JSON + RFC 8785 (JCS) canonical form for
  signing.** MCP's wire language, human-debuggable, no gossip-style size
  pressure; JCS gives deterministic bytes for Ed25519 without a second codec.
  Rejected: CBOR (second codec for no win here), protobuf (schema toolchain
  tax). *APPROVED (maintainer, 2026-07-16)*
- **T-02 — IDs: UUIDv7 (`github.com/google/uuid`).** Time-sortable (inbox
  ordering, index locality), standard, one tiny dep. Rejected: ULID lib
  (equivalent, less standard), stdlib-only random UUIDs (no ordering).
  *APPROVED (maintainer, 2026-07-16)*
- **T-03 — SQLite via `modernc.org/sqlite`.** Pure-Go keeps CGO_ENABLED=0 and
  trivial cross-compilation (D-09). WAL mode; single-writer discipline via a
  store-level mutex; `busy_timeout` set. *APPROVED (maintainer, 2026-07-16)*
- **T-04 — WebSocket via `github.com/coder/websocket`.** Maintained canonical
  fork, ISC, zero deps. *APPROVED (maintainer, 2026-07-16)*
- **T-05 — CLI: stdlib `flag` + hand-rolled subcommand dispatch.** ~10 verbs,
  no dynamic completion needs in v1; keeps the dep tree at 5 modules. Cobra
  re-considered if verbs sprawl (matrix row kept). *APPROVED (maintainer, 2026-07-16)*
- **T-06 — Tokens: EdDSA-signed JWTs (`golang-jwt/jwt/v5`), audience-bound
  (RFC 8707 resource), 1 h access / refresh at AS; device WS credential is a
  distinct long-lived token bound to the device record.** go-sdk `auth`
  supplies PRM + bearer plumbing; verification callback is ours. *APPROVED (maintainer, 2026-07-16)*
- **T-07 — MCP: go-sdk v1.6.1, stateless Streamable HTTP mode, protocol
  2025-11-25; migrate to v1.7.0/2026-07-28 only via S-03 outcome.** *APPROVED (maintainer, 2026-07-16)*
- **T-08 — Spotlighting format as specified in arch §5.2** (fresh nonce per
  rendering, static warning preamble, URLs as plain text, provenance line).
  Golden-tested in WP-13. *APPROVED (maintainer, 2026-07-16)*
- **T-09 — Retention defaults: delete on all-devices-ack + 72 h grace; hard
  TTL 30 days; grants/audit rows immortal** (D-10). Operator-tunable, floor
  1 h. *APPROVED (maintainer, 2026-07-16)*
- **T-10 — `wait_for_activity` caps: daemon n/a (WS); claude.ai 240 s; ChatGPT
  45 s; Claude Code remote 25 min default** — revised by S-02 evidence, not by
  hope. *APPROVED (maintainer, 2026-07-16)*
- **T-11 — Config: relay = flags + env only; daemon/user =
  `~/.config/askrelay/config.json` (0600) + key file beside it** (arch §7).
  *APPROVED (maintainer, 2026-07-16)*
- **T-12 — Redaction v1 pattern set:** AWS access keys, GCP service-account
  JSON markers, GitHub `ghp_`/`gho_`/fine-grained `github_pat_`, Slack `xox*`,
  generic `-----BEGIN … PRIVATE KEY-----` blocks, JWT triple-dot shape, and
  `.env`-style `KEY=value` lines for names matching `(TOKEN|SECRET|KEY|
  PASSWORD)`. Visible `⟦redacted:<kind>⟧` markers; sender warned; no entropy
  heuristics (false-positive machine). Hook: executable at
  `redact_hook` config path, receives text on stdin, returns replacement.
  *APPROVED (maintainer, 2026-07-16)* · **Amended 2026-08-05 (WP-11 quality
  pass):** the assignment matcher was widened past `.env` `KEY=value` to also
  cover YAML `name: value`, JSON `"name": "value"`, shell `export`/`set`, and
  quoted values — engineers paste configs in those forms, and the original
  line-anchored `=`-only rule leaked them. Name-word set gained `PASSWD` and
  `CREDENTIAL`; Slack gained `xapp-`/`xoxe-`; truncated PEMs and alg=none JWTs
  are caught. Still fixed-pattern, no entropy. Accepted permanent ceilings
  (would need entropy/NLP, rejected): bare high-entropy blobs, secrets in prose,
  raw-newline-split tokens, zero-width-char injection, percent-encoding — the
  approval gate is the backstop for these.
- **T-13 — Dev/prod build split retained** (`-tags dev`: verbose tracing,
  localhost debug endpoint; prod artifacts contain neither; CI builds both).
  *APPROVED (maintainer, 2026-07-16)*
- **T-14 — HTTP routing: stdlib `net/http` ServeMux (1.22+ method patterns).**
  No router dep. *APPROVED (maintainer, 2026-07-16)*
- **T-15 — Logging: stdlib `log/slog`, JSON handler on the relay, text on the
  daemon; no third-party logger.** *APPROVED (maintainer, 2026-07-16)*
- **T-16 — Envelope size cap is enforced on the whole artifact, in the
  `internal/envelope` type itself.** Three caps, all checked in the same path as
  Sign/Verify so neither can produce nor accept an over-limit envelope:
  `MaxBodyBytes = 32 << 10` (sum of `Part.Text` UTF-8 bytes, existing),
  `MaxWireBytes = 64 << 10` (whole canonical envelope), and `MaxParts = 16`
  (part count). Body-text and whole-wire overruns return `ErrTooLarge`;
  part-count overruns return a new `ErrTooManyParts`. Rationale: boundedness is
  a property of the artifact, so every downstream consumer (store, delivery, MCP)
  inherits it and none can forget it — the WP-01 test pass demonstrated a signed,
  verifying 4.2 MB envelope (128× the intended cap) built from metadata/part-count
  bloat, which contradicted arch §3/§8's exfil-mitigation claim. Relay ingress
  (WP-03/WP-07) still adds a raw-read cap as defense-in-depth.
  *APPROVED (maintainer, 2026-07-22 — WP-01 test pass finding).*
- **T-17 — Error-handling convention (ratifies the WP-01/WP-02 style, made
  binding).** Sentinel error *values* (`errors.New`, exported `ErrXxx`) for
  outcomes callers branch on, matched with `errors.Is`; typed errors only when a
  caller needs structured data out of the error. Propagate by wrapping:
  `fmt.Errorf("<pkg>: <context>: %w", err)` — `%w` preserves the chain, the
  `<pkg>:` prefix makes origin legible. Return zero value + error, never both.
  Panic only for unrecoverable faults (e.g. CSPRNG failure, cf. `uuid.Must`),
  never for expected runtime conditions. The pure core packages (`a2a`,
  `envelope`, `gate`) never log — they return errors; only the boundary layers
  (relay handlers, daemon, CLI, sweeper) log. Errors crossing to a remote MCP
  client are sanitized (no internal detail or content leaks); the client-facing
  error taxonomy is decided at WP-07, not now (boundary-errors fork —
  sentinels-sanitized-at-boundary chosen, structured codes deferred).
  *APPROVED (maintainer, 2026-07-23).*
- **T-18 — Logging policy (mechanism per T-15: `slog`, JSON on relay / text on
  daemon).** NEVER log message bodies/content, secrets, tokens, invite links,
  private keys, or PII — log identifiers and shapes only (thread id, message id,
  state, byte counts, error kind). Structured fields, not formatted strings
  (`slog.String("thread", id)`), so a secret cannot be accidentally interpolated
  into a message. Levels: `Error` (real failure), `Warn` (recoverable/suspicious
  — signature reject, over-cap envelope, gate refusal, revoked-device attempt),
  `Info` (lifecycle: startup, enrollment, delivery), `Debug` (dev builds only,
  `-tags dev` per T-13). Security-relevant refusals are logged at `Warn` WITHOUT
  the offending content; the durable consent audit trail (grant/approval rows)
  lives in the store (WP-03), not in logs. Logging happens only at boundaries
  (T-17). *APPROVED (maintainer, 2026-07-23).*

## 6. Work packages

### WP-01 — `internal/envelope`

**Status:** DONE (2026-07-22) · **Depends on:** — · **Gated by:** T-01 T-02 T-16
(all APPROVED), S-05 (§8 kill criterion 1 — **cleared 2026-07-20, PROCEED**) ·
**Spec:** arch §3, §12.

**Goal:** the envelope as the single shared artifact: types, JCS
canonicalization, Ed25519 sign/verify, whole-artifact size caps (T-16:
`MaxBodyBytes`/`MaxWireBytes`/`MaxParts`, enforced in the Sign/Verify path),
append-only versioning.

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
const MaxBodyBytes = 32 << 10   // sum of Part.Text UTF-8 bytes
const MaxWireBytes = 64 << 10   // whole canonical envelope (T-16)
const MaxParts     = 16         // Body.Parts count (T-16)
func New(...) Envelope                                   // UUIDv7 ids
func Canonical(e Envelope) ([]byte, error)               // RFC 8785, sig omitted
func Sign(e *Envelope, priv ed25519.PrivateKey) error    // enforces all caps + version
func Verify(e Envelope, pub ed25519.PublicKey) error     // sig + all caps + version
var ErrTooLarge, ErrTooManyParts, ErrBadVersion, ErrBadSignature error
```
Caps are enforced in the shared check path so Sign cannot produce, and Verify
cannot accept, an over-limit envelope (T-16). `ErrTooLarge` covers body-text and
whole-wire overruns; `ErrTooManyParts` covers the part-count cap.

**Test plan:** round-trip; canonical stability across field order & unicode;
signature tamper matrix (every field); size-cap edges; fuzz Decode/Canonical;
cross-check one golden JCS vector against the RFC 8785 example set.

**Exit:** another package can build a signed message and verify a tampered one
fails, with zero non-stdlib deps beyond `google/uuid`.

### WP-02 — `internal/gate`

**Status:** DONE (2026-07-23) · **Depends on:** — · **Gated by:** — ·
**Spec:** arch §5.3; D-03, D-11.

**Goal:** the approval/grant state machine, pure and I/O-free — and
payload-agnostic by design (D-19/C4): the machine operates on
`Approvable{Kind, Payload}`, with `kind = "message"` the only v1 kind; gates,
grants, and audit rows never assume message-ness (this is the deliberate hinge
to the approval-gateway adjacency). Approving an inbound message transitions
its thread `input-required → working`, declining transitions it `→ rejected`
(A2A-verbatim hyphenated names on the wire and in audit rows, whatever the Go
identifiers are — arch §3, §5.3); outbound drafts `pending_review → sent |
discarded`; per-thread × direction grants with `via_grant` audit marks.

**Files:** `internal/gate/{gate.go,states.go}` + exhaustive table-driven tests.

**Exit:** every legal and illegal transition is table-tested; grants
short-circuit exactly one gate direction; no way to construct a "sent" without
approval-or-grant in the type surface.

**Security note (WP-01 review F3):** the envelope's `state` field is an
**untrusted sender assertion** — the envelope does not validate it. WP-02 must
never let an inbound envelope's `state` drive a gate transition or
short-circuit approval (a forged `state="completed"` must not close a thread);
derive state from the gate's own authoritative record, validate against the
seven A2A states, and treat unknown/attacker-chosen values as invalid.

### WP-03 — `internal/relay/store`

**Status:** DONE (2026-07-26, PR #3) · **Depends on:** WP-01 · **Gated by:** T-03 T-09 ·
**Spec:** arch §4.1, §4.2, §4.3.

**Goal:** SQLite persistence: embedded-SQL schema + migrations; persons,
devices (revocation), invites (single-use, TTL), threads, messages
(per-device delivery/ack), grants (immortal); retention sweeper
queries (ack+grace, hard TTL). *(`oauth_*` tables moved to WP-05,
2026-07-24: their shape belongs to the OAuth design that WP owns; the
migration harness applies them later as `0002_oauth.sql`.)*

**Dependency added:** `modernc.org/sqlite v1.54.0`.

**Test plan:** migration idempotency; revoked-device refusal; invite
single-use race; sweeper boundary cases (partial acks, grace edges, hard TTL
overrides un-fetched); grants survive message deletion.

**Security notes (WP-01 review F1/F2 + canonical-form invariant):**
- Store the **canonical form / re-marshaled decoded struct** in the `messages`
  blob, **never the raw received wire** (arch §4.1 invariant).
- Replay/dedup tombstone keys on the signed `id` string — never a wire hash
  (the wire is malleable: whitespace/key-order changes verify to the same `id`).
  Treat empty, non-UUID, or duplicate `id` as replay; do not rely on UUIDv7
  monotonicity for anything security-relevant.
- `sent_at` is a self-asserted sender clock: bound it **both** past (retention
  window) **and** future (clock-skew cap), else a post-dated envelope re-enters
  the freshness window indefinitely.
- Wrap ingress reads in `io.LimitReader` set **above** `MaxWireBytes` (canonical
  cap + ~97 bytes sig framing + slack for non-canonical whitespace/escaping) —
  `envelope.Decode` assumes pre-bounded input and imposes no size limit itself.

**Security notes (WP-02 quality pass):**
- The state passed to `gate.ApplyInbound` MUST be the store's authoritative
  thread row, **never** `envelope.State` — F3 as an integration contract: the
  call `gate.ApplyInbound(env.State, v)` typechecks and silently reintroduces
  the forged-state bypass. Consider a store-minted state type so the wrong
  call does not compile.
- Approval must be transactional: read current state → `gate.Apply*` → commit
  as **one unit**, or two concurrent approvals of the same draft each read
  `pending_review` and double-mint a `Release` (TOCTOU double-send).
- A payload handed to the gate is **immutable afterwards**: the minted
  `Release` aliases the caller's value (an Envelope's slices are shared, not
  copied — the gate cannot deep-copy an opaque `any`). `approve_reply`'s
  edit-before-release flow must construct a fresh envelope, never mutate the
  one already passed to the gate.

### WP-04 — relay HTTP skeleton + enrollment

**Status:** DONE (2026-07-26, PR #4) · **Depends on:** WP-03 · **Gated by:** T-11 T-14 T-15 ·
**Spec:** arch §4.3, §4.5, §7, §11.

**Goal:** `askrelay serve`: config from flags/env; ServeMux routes
(`/healthz`, `POST /enroll/{token}`); invite creation
(`askrelay invite` against admin socket or local DB); slog; graceful
shutdown; systemd/docker docs stubs.

**Exit:** a fresh VPS walkthrough (documented) reaches "device enrolled" with
only the binary and a reverse proxy.

### WP-05 — relay OAuth: resource server

**Status:** DONE (2026-07-29, PR #5) · **Depends on:** WP-04 · **Gated by:** T-06 ·
**Spec:** arch §4.4.

**Goal:** RFC 9728 PRM endpoint (go-sdk `auth` handler), bearer validation
middleware (EdDSA JWT, audience = relay base URL, person-identity claims),
token issuance internals, device-credential tokens for `/ws`.

*(Tokens are STATELESS — decision 2026-07-26: access + device credentials are
self-contained EdDSA JWTs, revocation via short life + a live
`store.ActiveDeviceByID` check, so the resource server needs no oauth tables.
Consequently `0002_oauth.sql` moved WP-05 → WP-06, which designs the AS's
clients / PKCE-codes / refresh-token schema. Signing key = an ed25519 key file
beside the DB, generated on first `serve`.)*

**Dependencies added:** `github.com/modelcontextprotocol/go-sdk v1.6.1`,
`github.com/golang-jwt/jwt/v5 v5.3.1`.

**Test plan:** RFC 8707 audience mismatch rejected; expired/not-yet-valid;
PRM document matches spec examples. *("Revoked device's tokens die" is split
by the stateless-token decision: device credentials die immediately via the
live `ActiveDeviceByID` check at `/ws` connect (WP-08); an already-issued
access token is bounded by its ≤1h TTL and dies at expiry, since access
tokens carry no device claim — WP-06 must re-check the device at issuance to
stop new tokens. WP-05 itself proves only the primitive: `ActiveDeviceByID`
refuses a revoked device.)*

**Security notes (WP-05 quality pass):**
- Bearer-auth failures return a **bare** `auth.ErrInvalidToken` (go-sdk echoes
  the error string into the 401 body — the reason must not travel to the
  caller, T-17); the reason is logged server-side as a code only (T-18).
- The signing key is loaded by re-deriving the public half from the seed
  (`ed25519.NewKeyFromSeed`) and created atomically (`O_CREATE|O_EXCL`) so a
  corrupted-but-right-length file or a concurrent first-start can't produce a
  silently-broken or split-brained signer.

### WP-06 — relay OAuth: embedded AS + client registration

**Status:** TODO · **Depends on:** WP-05 · **Gated by:** T-06 ·
**Spec:** arch §4.4.

**Goal:** authorization-code + PKCE flow whose "login" is invite-token/device
credential entry (no passwords, no signup); DCR endpoint (claude.ai path);
CIMD acceptance (ChatGPT path); JWKS; refresh tokens. **Owns `0002_oauth.sql`**
(moved from WP-05, 2026-07-26): the clients, PKCE authorization-codes, and
refresh-token tables — all AS state, shaped by this WP's design (WP-05's
resource server is stateless and ships no oauth tables).

**Security obligations (from WP-05 quality pass):**
- **Re-check the device on every mint and refresh** — call
  `store.ActiveDeviceByID` (or an equivalent person-active check) at the token
  endpoint, so a revoked device cannot mint fresh access tokens. Without this,
  device revocation only bites new *device credentials*, not the access-token
  path (arch §4.3, T-06).
- **Never mint multi-audience access tokens.** `Issuer.Verify` accepts a token
  whose `aud` array contains the relay's audience among others (RFC 8707
  "one match is enough"); keep `aud` single-valued so a token minted here is
  not replayable at another resource sharing the signing key.

**Test plan:** full code+PKCE happy path scripted; PKCE downgrade attacks
rejected; DCR'd and CIMD clients both reach a working token; state/nonce
handling; a claude.ai and ChatGPT connector each complete auth in S-04.

### WP-07 — relay MCP surface

**Status:** DONE (2026-07-31, PR #6) · **Depends on:** WP-01 WP-02 WP-03 WP-05 ·
**Gated by:** T-07 T-08 T-10 · **Spec:** arch §5 (all), §9.

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

**Security note (WP-01 review F3):** `part.type` is an unvalidated sender
string. The no-auto-fetch / inbound-quarantine invariant (arch §8) lives here,
not in the envelope: whitelist known part types and render any unknown type
(e.g. a sender-supplied `"image"`) as inert text — never as a fetchable/renderable
resource. Also apply the ingress `io.LimitReader` (see WP-03) on this surface.

**Security note (WP-02 quality pass):** exactly ONE handler
(`approve_message`) may call `gate.ApplyInbound` with a human verdict.
Inbound deliberately has no `Release`-style capability token — the async
design doesn't need one — so this is a wiring discipline the WP-07 review
must explicitly check, not a type-level guarantee.

**Security note (WP-03 quality pass):** the boundary MUST verify the signing
device belongs to the claimed `envelope.From.Person` before calling
`store.IngestMessage` with the resolved `senderID`. The store enforces that
the *resolved* sender is one of the thread's two parties (`ErrNotParticipant`,
D-05 1:1), but it does **not** check that the signature matches the claimed
`From.Person` — that binding (`ActiveDeviceByPubkey` → device's person ==
`From.Person`) is the caller's job. Same obligation on WP-04's `/enroll` and
any other ingest path.

### WP-08 — WS hub, delivery, retention sweeper

**Status:** DONE (2026-08-03, PR #7) · **Depends on:** WP-03 WP-04 ·
**Gated by:** T-04 T-09 · **Spec:** arch §4.2, §4.5, §6.

**Goal:** `/ws` (device-credential auth): push new-mail/approval events,
accept outbound envelopes, track per-device acks; resend-on-reconnect;
retention sweeper goroutine wired to T-09 knobs.

**Dependency added:** `github.com/coder/websocket v1.8.15`.

**Delivery obligations (surfaced by WP-07):**
- **Consume `sent` drafts** — DONE, folded into release. `send_message`/
  `approve_reply` release a draft to `sent`; that release now **also delivers**
  in the *same* transaction (`ReleaseDraft`/`ReleaseReplyViaGrant` →
  `deliverInTx` → the shared `ingestTx`), so a `sent`-but-undelivered state
  cannot exist. Delivery is **relay-attested** — the released envelope is
  unsigned; the author was OAuth-authenticated at create+release and the relay
  is the trust anchor (D-10/D-23). (Delivery does *not* sign; the daemon-signed
  variant returns with WP-09.)
- **Set the recipient's thread to `input-required` on delivery.**
  `IngestMessage` only sets `input-required` when it *creates* a thread; a
  delivered ask/reply onto an already-existing thread must transition the
  recipient's side to `input-required`, or `check_inbox`/`InboundAwaiting`
  won't surface it (WP-07 subtask-4 deliberately left existing-thread state to
  the delivery layer).
- **Browser-client signing** — **deferred out of WP-08 by D-23** (2026-07-31).
  A browser MCP client (claude.ai/ChatGPT) has no local device key to sign its
  outbound envelope, and the signing model (relay-held browser-device key signed
  server-side vs. read/approve-only browser clients) is still an open
  wire-format/security-posture call. D-23 scopes WP-08 to **device-credential
  delivery only** and takes this together with WP-06 when the browser-connector
  path is wanted, so it no longer blocks this WP.

**Security notes (WP-02 quality pass):** delivery MUST match
`gate.Release.Thread()` **and** `gate.Release.ID()` against the message it
transmits — thread alone permits replaying an approved Release for a
*different* message in the same thread, and a non-nil `*Release` alone is
forgeable as an inert `new(gate.Release)` (empty thread/id, nil payload;
pinned by `internal/gate/surface_test.go`). Do **not** rely on payload
identity: `Payload()` is an uncomparable `any` (naive `==` panics on an
Envelope) and the minted value aliases the caller's slices — the
approved-content-is-what-sends guarantee rests on WP-03's
payload-immutability note.

**Test plan:** kill/reconnect matrix (nothing lost, nothing duplicated beyond
at-least-once + id dedupe); ack bookkeeping vs sweeper; revocation severs live
sockets.

**Subtasks (branch `wp-08-delivery`):** (1) relay-attested delivery of released
drafts, (2) `/ws` endpoint + device-cred auth + connection hub, (3) inbound push
+ ack over `/ws`, (4) daemon signed-submit + identity boundary — **removed by the
quality pass (C1), deferred to WP-09**, (5) retention sweeper + graceful WS
drain. Code in `internal/relay/{ws.go,server.go}` + `internal/relay/store/
{draft.go,message.go}`.

**Quality pass (2026-08-03, three agents — test/review/security):** the trust
core held (signing/identity boundary, `/ws` auth with use-claim separation,
replay/freshness with the tombstone tied to the freshness horizon, cross-thread
injection refused, no auth oracle, no content in logs). Fixes applied on-branch
(commit `4f7073b`); the adversarial suites are committed
(`internal/relay/ws_adversarial_test.go`, `internal/relay/store/
draft_adversarial_test.go`):
- **C1 (critical) — outbound gate bypass:** the `/ws` `submit` frame ingested
  raw signed envelopes, skipping the outbound approval gate (D-11, arch §5.3).
  **Removed**; daemon-signed outbound is deferred to WP-09, gated through a
  released draft there.
- **H2 (high) — revocation severs live sockets:** `pushInbox` re-checks
  `ActiveDeviceByID`, the read loop re-checks per frame, and a 15 s reconcile
  (`severRevoked`) closes idle / out-of-process-revoked sockets.
- **H3 (high) — goroutine leak:** one coalescing writer per connection with
  bounded writes replaces the per-`Notify` `context.Background()` goroutine
  (also collapses the O(n²) inbox re-reads).
- **H1 (high) — `input-required` on delivery** moved into the shared `ingestTx`,
  so both delivery paths flip an existing thread to the recipient's turn.
- **M1 (med) — atomic release+deliver** (folded — see the consume-`sent`-drafts
  obligation) and **M2 (med) — T-16 caps on the unsigned draft pipeline**
  (`envelope.Validate` in `CreateDraft`/`ReleaseDraft`).
- Lower-tier: `dev.PersonID == person` assert on `/ws` connect (L2),
  zero-value+error on the release error path (L1), honest "`closeAll` is abrupt,
  not a drain" wording (L6). L5 (nil-conn `hub.add`) left as unreachable.

**D-23 self-talk note:** the self-talk on-ramp needs **two distinct enrolled
person identities** owned by the same operator, not one identity talking to
itself — `threads` carries `CHECK (initiator_id <> recipient_id)`
(`store/migrations/0001_init.sql`), pinned by `TestSelfThreadBlockedAtSchema`.
Nothing to change; worth stating because D-23 makes self-talk the first-user path.

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

**Status:** DONE (2026-08-05, PR #10) · **Depends on:** — · **Gated by:** T-12 ·
**Spec:** D-12, arch §5.3, §6.

**Goal:** the T-12 pattern set with visible `⟦redacted:<kind>⟧` markers, sender
warning surface (`Result.Summary`), and the fail-closed external-hook runner.
**Client-side only** — no relay-side backstop (D-12; arch §5.3 removed it as a
blocker, so the earlier "backstop mode" phrasing here was stale and is dropped).

**Test plan:** corpus of true positives per pattern; false-positive corpus
(UUIDs, git SHAs, base64 blobs) stays untouched; hook contract (stdin/stdout,
timeout, failure = block send, never silently pass). *(Delivered, plus the
quality-pass adversarial suites + a fuzz idempotency/no-panic target with a
committed regression corpus.)*

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

**Security note (WP-01 review F4):** the signed envelope has no relay/audience
binding — safe under D-06 (single-team, per-device keys), but a signed envelope
is replayable onto any relay sharing a device key. Before this multi-tenant
hosted instance ships, add an audience/relay-identifier to the signed envelope
(the RFC 8707 analog) so cross-relay replay is structurally impossible, not
merely precluded by the single-relay assumption. This is an envelope `v`-bump
(arch §12), so plan it deliberately.

**Paid-tier-ready, not paid (D-08 revision 2026-07-19):** enforce hard quotas
from day one — small per-org daily message caps, short message TTLs, rate
limits (the ntfy expectation-lock is the cautionary tale for a relay-shaped
free service). Build the tenancy/quota/billing *seams* — per-org isolation
boundaries, quota hooks, billing-neutral config, stubbed billing interface —
but ship **no** live billing, invoicing, or SLA. The hosted-instance page and
README carry the framing "self-hosting is free forever; this instance is a
quota-capped demo that MAY become a paid tier"; no pricing is published.
Revisit paid hosting only on the pre-registered trigger (25+ active external
non-Kosmoy orgs, or a first unsolicited purchase request).

## 7. Learnings log

*(append-only; every entry names the WPs it changed)*

- 2026-07-22 — **WP-01 DONE** (first pipeline run: dev → adversarial test →
  T-16 hardening → review → security, each pass verified green by the
  orchestrator). Deps: `github.com/google/uuid v1.6.0` (BSD-3) pinned — matrix
  updated. The test pass found the text-only size cap let a signed 4.2 MB
  envelope verify → raised as **T-16** (whole-artifact caps `MaxWireBytes`
  64 KiB + `MaxParts` 16, enforced in the shared Sign/Verify check path);
  approved and implemented. Security red-team found no consent-bypass and no
  integrity break (canonicalization unambiguous for the envelope; Decode-then-
  Verify collapses all wire malleability; exfil-by-bulk closed). It surfaced
  four **cross-WP** rules, now written into the affected entries: **arch §4.1**
  — store the canonical form, never raw wire (a signature authenticates the
  decoded struct's canonical projection, not the wire; every consumer must
  Decode→Verify→read-from-struct). **WP-02** — envelope `state` is an untrusted
  sender assertion; never let it drive a gate. **WP-03** — replay-tombstone on
  the signed `id` (never a wire hash); bound `sent_at` past AND future; ingress
  `io.LimitReader` set *above* `MaxWireBytes`+sig-overhead+slack (Decode assumes
  pre-bounded input). **WP-07** — client-side `part.type` whitelist, render
  unknown types inert (the no-auto-fetch invariant lives here). **WP-16** —
  add relay/audience binding to the signed envelope before the multi-tenant
  hosted instance (an envelope `v`-bump).
- 2026-07-16 — Plan drafted. Pin sweep found CI's golangci-lint-action two
  majors stale (@v6 → @v9): fixed in the Phase 4 re-scaffold commit, affects
  no WP. The 2026-07-28 MCP RC deprecates Roots/Sampling/Logging and removes
  sessions — reinforces T-07/S-03; no WP change.
- 2026-07-16 — Market verdict GO_WITH_CHANGES adopted (D-19 resolution): S-05
  askmesh-autopsy gate added before WP-01; §8 kill criteria pre-registered;
  WP-02 now centers `Approvable{Kind, Payload}` (C4); positioning amended
  (hook + moat-forward subtitle, C2); same-owner cross-machine documented as a
  day-one scenario (C3, arch §2). Sections renumbered: risk register is §9.
- 2026-07-16 — Five-reviewer fresh-context pass (arch §15 records it): WP-13
  gained WP-06 as a dependency + adversarial auth runs; WP-02 state names
  hyphenated to A2A-verbatim; WP-01 gained its uuid dependency line; WP-12
  now owns the in-process relay fixture that WP-13 adopts; WP-16's gate made
  checklist-checkable; R-10 (DCR churn) added; §4 graph replaced by track
  list (the ASCII art disagreed with the authoritative table in four places).
- 2026-07-20 — S-05 askmesh autopsy DONE early → **PROCEED** (kill criterion 1
  cleared: askmesh was never launched, so its low adoption is not a demand
  signal against us). WP-01's S-05 gate satisfied. Two consequences: risk R-11
  (cold-start/retention) added as the core existential risk; kill criterion 3
  re-scoped to measure retention past novelty, not trial. Also adopted the D-21
  strategic framing (success definition, why-we-build, answering-side-moat lead,
  same-owner front-door candidate, security-response as top commitment,
  askmesh differentiation). Writeup: `docs/research/askmesh-autopsy.md`.
- 2026-07-23 — **WP-02 DONE** (PR #2; first D-22 supervised run: 5 subtasks +
  three-agent quality pass). T-17/T-18 ratified mid-WP. The pass converged on
  one real hole — direction was caller-asserted, never validated against the
  path called — fixed in-WP (ErrWrongDirection asserts, `Release.ID()`
  identity). Changed later WPs: WP-03 (F3 integration contract, transactional
  approval, payload immutability), WP-07 (single approve handler), WP-08
  (Thread+ID match, never payload identity). Grant deliberately has no Kind
  field — the WP that ships Kind #2 must widen Grant + Covers.
- 2026-07-24 — WP-03 subtask 2: `oauth_*` DDL moved WP-03 → WP-05 (the schema
  is migration-versioned; table shapes freeze only when the owning WP designs
  them — WP-05 ships `0002_oauth.sql`). Changed: WP-03, WP-05.
- 2026-07-26 — **WP-03 DONE** (PR #3; 7 subtasks + three-agent quality pass +
  maintainer-approved fixes). Dep added: `modernc.org/sqlite v1.54.0` (§3 pin,
  pure-Go, CGO stays off). F3 + TOCTOU close by API shape — the approval
  methods take no state parameter, so a forged-state call can't be written, and
  read/gate/write share one `writeTx`. The quality pass (all three agents
  converged) produced two in-WP fixes and one cross-WP obligation: **cross-thread
  injection** — `IngestMessage`/`CreateDraft` now enforce thread membership
  (`ErrNotParticipant`, D-05); **replay-on-misconfig** — deleted the
  `TombstoneTTL` knob, `PruneTombstones` takes the ingest `Freshness` so the
  prune horizon *is* the freshness horizon; and a **WP-04/07 note** (recorded in
  the WP-07 entry): the boundary must bind the signing device to
  `envelope.From.Person`, since the store only checks the resolved sender is a
  participant. Retention keys the tombstone on the signed `sent_at`. Changed:
  WP-04, WP-07.
- 2026-07-26 — **WP-04 DONE** (PR #4; relay HTTP skeleton + enrollment). No new
  dependency — stdlib `net/http` ServeMux (T-14), `slog` (T-15), `flag` (T-11).
  Landed `askrelay serve` (`/healthz`, `POST /enroll/{token}`, graceful
  shutdown), `askrelay invite`, `docs/deploy/` stubs. Config `Validate` enforces
  the T-09 1h floor in code (the WP-03 lesson). Two calls: the WS device
  credential is deferred to WP-05 (needs T-06 tokens); `invite` opens SQLite
  directly (WAL handles contention, no admin socket). Quality pass skipped by
  maintainer decision (direct code review); the batch "show all code" process
  was a one-off — D-22 per-subtask loop resumes at WP-05. Exit criterion
  verified end-to-end against the binary (real HTTP enroll → device enrolled;
  token reuse → 403). Changed: WP-05 (inherits the deferred WS credential +
  `0002_oauth.sql`).
- 2026-07-29 — **WP-05 DONE** (PR #5; OAuth resource server + tokens). Deps:
  `golang-jwt/jwt/v5 v5.3.1` + `modelcontextprotocol/go-sdk v1.6.1` (§3 pins).
  Stateless EdDSA JWTs (access 1h + long-lived device credential), `use`-claim
  separation, RFC 9728 PRM + bearer middleware, `/enroll` now returns the
  device credential. **Decisions:** tokens are stateless so `0002_oauth.sql`
  moved WP-05 → WP-06; and (maintainer, Option 1) access-token revocation is
  bounded by the ≤1h TTL, not immediate — arch §4.3/§8 reworded honestly. The
  R-06 quality pass caught the T-17 blocker (bearer 401 body leaked the
  validation reason — now bare `auth.ErrInvalidToken` + T-18 server-side log)
  and a corrupted-signing-key footgun (now re-derive pub from seed + atomic
  create); core confirmed forgery-proof under ~9M fuzz inputs. **Binding
  obligations pushed to WP-06:** re-check `ActiveDeviceByID` on every token
  mint/refresh, and never mint multi-audience tokens (both recorded in the
  WP-06 entry). Changed: WP-06.
- 2026-07-31 — **WP-07 DONE** (PR #6; relay MCP surface, taken ahead of WP-06 to
  reach a testable surface sooner). No new dependency — the go-sdk `mcp` server
  on stateless Streamable HTTP behind the WP-05 bearer middleware. Landed the
  nine §5.1 tools (`send_message`, `check_inbox`, `get_thread`,
  `approve_message`/`decline_message`, `approve_reply`/`discard_reply`,
  `set_thread_grant`, `wait_for_activity`), the §5.2 spotlighting renderer, and
  the person-level store reads (`ThreadFor`, inbound-awaiting) the tools needed.
  The three-agent pass confirmed the trust core sound (no cross-person access,
  no gate bypass, nonce/attribute quarantine holds) and produced the fixes in
  9cacac2. **Blockers fixed:** (1) handlers returned a nil `CallToolResult` so
  go-sdk auto-filled `Content` with a *duplicate* JSON copy of
  `structuredContent` — breaks the ChatGPT single-object contract; now every
  handler sets a non-nil `Content` via `emptyResult`/`textResult`. (2) URL
  defang was http(s)-only — now scheme-agnostic (`scheme://` → `scheme[:]//`,
  plus `data:`/`javascript:`/`vbscript:`) and the whole spotlight block is
  wrapped in a backtick-sized code fence so markdown clients can't auto-link a
  protocol-relative URL. (3) the spotlight `state=` echoed the sender's
  self-asserted `e.State`; now emits the **authoritative** thread state
  (`input-required` for `check_inbox`, `view.State` for `get_thread`) and drops
  `e.State` from the tag. (4) DoS: `http.MaxBytesReader` (128 KiB) on `/mcp` +
  a `MaxBodyBytes` cap on `send_message`/`approve_reply` text (the WP-07 ingress
  note). (5) `send_message` on an existing thread now requires `e.To` to be the
  thread's *other* party, not merely that the author is a participant.
  **Decisions (maintainer):** `approve/decline_message` bind to the thread's
  **latest** not-mine message (bind-now — a stale id is `ErrNotFound`);
  `wait_for_activity` gets a per-person concurrency cap of 2; the roster
  membership signal in `send_message` is accepted as-is for now. The WP-03
  device↔`From.Person` binding obligation and the WP-02 single-`approve_message`
  wiring discipline were both honoured and checked by the pass. **Pushed to
  WP-08:** consume `sent` drafts + sign/deliver, set the recipient thread to
  `input-required` on delivery onto an existing thread, and the still-open
  browser-client signing model (all recorded in the WP-08 entry). Changed: WP-08.
- 2026-08-03 — **WP-08 DONE** (PR #7; WS hub, delivery, retention sweeper —
  completes the cross-person A→B→A loop). Dep: `github.com/coder/websocket
  v1.8.15` (§3 pin). Landed `/ws` (device-cred auth + live `ActiveDeviceByID`
  kill switch + `PersonID` binding), inbound push + ack over the socket, the
  T-09 retention sweeper + graceful WS drain, and **relay-attested** delivery of
  released drafts (unsigned; the author was OAuth-authed at create+release and
  the relay is the trust anchor — D-10/D-23). The three-agent quality pass
  confirmed the trust core sound (signing/identity boundary, `/ws` auth with
  use-claim separation, replay/freshness, cross-thread injection refused, no
  oracle, no content logged) and produced the fixes in `4f7073b`. **Key catch —
  C1 (CRITICAL):** the `/ws` `submit` frame ingested raw signed envelopes,
  bypassing the outbound approval gate (D-11, arch §5.3) — the exact promise the
  product exists to keep. It was the design I proposed at subtask 4 ("relay
  trusts the signed submit; gate is sender-side") and it was wrong: a relay-side
  gate exists precisely so correctness doesn't depend on every client behaving,
  and the daemon that would use `submit` doesn't exist yet. **Removed; the
  daemon-signed path returns in WP-09, gated through a released draft.** Other
  fixes: **M1** release+deliver folded into ONE transaction (a `sent` draft is by
  construction delivered — no strandable state; `DeliverDraft`→`deliverInTx`);
  **H1** the `input-required` flip moved into the shared `ingestTx` (both
  delivery paths); **H2** revocation now severs live sockets (push re-check +
  per-frame read re-check + 15 s `severRevoked` reconcile, catching idle and
  out-of-process CLI revokes — the store can't call the hub, and revoke often
  runs in another process); **H3** a per-`Notify` `context.Background()` goroutine
  (no write deadline) leaked on a stalled socket — replaced by one coalescing
  writer per connection with bounded writes (also killed the O(n²) inbox
  re-reads); **M2** `envelope.Validate` enforces the T-16 caps on the unsigned
  draft pipeline. **Pushed to WP-09:** the gated daemon-signed `submit` (with
  `From.Device` binding, L3). Changed: WP-06 (browser-client signing travels
  with it, D-23), WP-09 (daemon-signed submit).
- 2026-08-05 — **WP-11 DONE** (PR #10; `internal/redact` — client-side secret
  redaction). Stdlib-only (no dep). `Redact` applies the T-12 fixed-pattern set
  with visible `⟦redacted:<kind>⟧` markers + a `Summary()` warning surface;
  `Hook` runs an operator's external scanner fail-closed. **Confirmed
  client-side only** (D-12) — the stale "relay-side backstop" line in the WP-11
  entry was dropped. The three-agent pass found the trust core sound but caught
  real holes, all fixed in `eb14851` and pinned by the agents' own adversarial
  suites + a fuzz target (idempotency/no-panic) with a committed regression
  corpus: **(CRITICAL)** `Hook` ignored its ctx deadline and returned a *false
  success* when a hook backgrounded a descendant holding stdout open — fixed
  with `cmd.WaitDelay`; **(HIGH)** a marker-forgery bypass leaked the real
  secret (the leading-`⟦` value exclusion made the whole rule fail) — fixed by
  matching values whole + a markerRe idempotency skip in `apply`; **(HIGH)**
  real config formats leaked — the maintainer approved **widening T-12** to
  YAML/JSON/quoted/`export`/`set` forms (amended in the T-12 entry), which also
  catches named AWS *secret* keys, truncated PEMs, and Slack `xapp-`. Empty hook
  output for non-empty input now blocks the send (maintainer decision); stderr
  is discarded (can't leak a secret a hook echoes there). **Honest framing
  recorded:** fixed-pattern redaction is a best-effort safety net, not a
  guarantee — bare-entropy blobs, prose secrets, zero-width-char and
  percent-encoding evasions are accepted ceilings (T-12 rejects entropy/NLP);
  the human approval gate is the real backstop. Changed: T-12 (amended).

## 8. Kill criteria & market checkpoints (D-19)

Adopted verbatim from [`research/market-verdict.md`](research/market-verdict.md)
(maintainer, 2026-07-16), pre-registered **before code** so the
employer-as-first-user relationship cannot quietly substitute for
disconfirmation. Read from evidence (relay metrics, dated events), never from
impressions.

1. **Pre-code (by 2026-08-01)** — S-05 askmesh autopsy shows a competent,
   discoverable, actually-distributed product that teammates simply declined
   (demand absence, not execution failure): **stop before application code**.
   → **RESOLVED 2026-07-20: PROCEED.** askmesh was never launched (closed-source,
   unlisted, zero footprint); its low adoption is invisibility, not rejection,
   so it is not evidence against us. Recorded caveat: demand is unproven, not
   proven — the burden now shifts to kill criterion 3.
2. **Ship date (2026-10-15)** — v1 relay not deployed and in use at Kosmoy:
   cut v1 drastically or stop; **never extend the docs phase instead**.
3. **Core demand — now the load-bearing test (8 weeks post-deploy; hard stop
   2027-01-31)** — fewer than 3 distinct Kosmoy pairs exchanging cross-person
   approval-gated messages in ≥ 2 of any 4 consecutive weeks without maintainer
   prompting: execute the approval-gateway pivot or wind down to internal tool +
   portfolio artifact. **This measures RETENTION past novelty, not trial** —
   the askmesh autopsy (S-05) showed trial bursts with no stickiness, and its
   cold-start network effect (R-11) is the hazard our OSS/self-host edges do not
   fix; the whole-team Kosmoy dogfood exists precisely to test this.
4. **External validation (~2027-04)** — zero unsolicited external deployments
   AND zero organic cross-org/cross-vendor demand signals: stop
   category-creation marketing; maintain for Kosmoy; decide pivot vs sunset.
5. **Vendor tripwire (continuous)** — Anthropic ships org-scoped cross-session
   / agent-teams messaging, or session sharing becomes per-person
   session-to-session messaging. Pre-ship: reposition cross-vendor/cross-org-
   only or stop. Post-ship: drop the same-org pitch, hold the moat quadrant.
6. **Capacity floor (rolling, first 6 months)** — monthly releases
   unsustainable, or a security report unacknowledged beyond 72 h: cut scope
   immediately or execute a documented wind-down. For a consent product, a
   neglected security posture is worse than an archived repo.

Supporting commitments from the same resolution: ≥ monthly releases for six
months (C5); co-maintainer recruitment + public cadence statement +
non-dismissive-response precommit at launch (C7); Kosmoy paid-hours
conversation after dogfooding proves value (C8, modified); 2–3 cross-org
tester pairs post-v1, timing flexible (C9, modified). The watch list lives in
market-verdict.md.

## 9. Risk register

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
| R-11 | Cold-start / retention network effect — value requires the other person also on it and responsive; a solo installer churns (askmesh's likely killer, S-05) | The core existential risk; OSS/self-host/zero-install do NOT fix it | Kosmoy whole-team dogfood adopts all-at-once (not solo trials); kill criterion 3 measures retention not trial; onboard in pairs; make first-run useful even before a reply lands (e.g. same-owner cross-machine, D-19/C3) |
