# Phase 2 decision log — pivot deep dive

Working log of the live deep-dive decisions (started 2026-07-15). Each entry gets a
maintainer verdict when settled. This file seeds the implementation plan's decision
log in Phase 4; evidence citations point at `docs/research/`.

---

## D-01 — Product shape: async mailbox between people's AI sessions

**Decision:** The product is an asynchronous mailbox, not real-time chat. Users
address a *person*, not a machine or session; messages land in a durable per-person
inbox on a relay and surface in whatever session that person attaches. Delivery
latency is honest per client: push where the platform allows (Claude Code), on next
human prompt where it doesn't (claude.ai, ChatGPT — pull-only platforms). Inbound
messages are untrusted data until the recipient's human approves.

**Evidence:** chatgpt-extension.md + chatgpt-field.md (ChatGPT strictly pull-only,
60s tool cap), mcp.md (no server push in core MCP; going stateless), transport.md
(offline delivery is the binding constraint), security.md (approval gate table stakes).

**Verdict:** APPROVED (maintainer, 2026-07-15) — implied by D-02..D-04 selections.

## D-02 — v1 client matrix: Claude Code first-class, rest degraded but real

**Decision:** Claude Code gets the full experience (send, receive, push, in-session
approval). claude.ai and ChatGPT connect as remote MCP connectors: full senders,
answering requires their human to open the session — documented honestly. Codex CLI
like Claude Code minus push. The cross-vendor demo (A's Claude Code ↔ B's ChatGPT)
ships in v1.

**Options considered:** (a) Claude Code first-class + rest degraded — CHOSEN;
(b) Claude-stack only v1 (loses the cross-vendor pitch); (c) all clients equal
(forces unverified scheduled-task hacks on ChatGPT).

**Verdict:** APPROVED (maintainer, 2026-07-15)

## D-03 — Approval model: per-message approval + revocable per-thread grants

**Decision:** Default: every inbound message requires the recipient's tap before
their AI treats it as anything but untrusted text. Convenience: revocable
"auto-approve this thread" grants, fully logged. Unattended API-key auto-reply
deferred to v1.1, pending the Anthropic "ordinary individual usage" clarification
(vendor-tos.md open question — ask Anthropic before GA).

**Options considered:** (a) per-message + per-thread grants — CHOSEN; (b) per-message
only (kills multi-turn UX); (c) unattended auto-reply in v1 (full lethal-trifecta
surface on day one + ToS gray zone).

**Verdict:** APPROVED (maintainer, 2026-07-15)

## D-04 — v1 primitives: DMs + threads only

**Decision:** Person-to-person messages threaded to their question, with an
A2A-aligned lifecycle underneath (pending → input-required → completed/rejected).
No group constructs in v1. Team broadcast channel is the first v1.1 candidate.

**Options considered:** (a) DMs + threads — CHOSEN; (b) + one team broadcast;
(c) full rooms model (scope explosion).

**Verdict:** APPROVED (maintainer, 2026-07-15)

## D-05 — v1 non-goals

**Decision:** Explicitly out of scope for v1: arbitrary file transfer (text +
fenced code snippets with size cap only), presence/read receipts, message search,
cross-org federation, E2EE (messages signed, transport TLS; encryption-at-relay
revisited if company IT requires), any UI beyond what AI clients render.

**Verdict:** APPROVED (maintainer, 2026-07-15) — presented with D-02..D-04; no objection.

## D-06 — Architecture: thin relay + optional daemon, relay IS the remote MCP server

**Decision:** Three components. (1) Relay: small server, per-person durable inboxes
(SQLite), team roster, grant/approval state; serves Streamable HTTP MCP + OAuth 2.1
directly so claude.ai/ChatGPT web connect with zero local install; stateless MCP
handling from day one (2026-07-28 revision). (2) Local daemon: OPTIONAL Go single
binary — hard design rule: nothing core may require it. Upgrades Claude Code with
push (channels bridge + hooks fallback), offline queueing, in-terminal approvals;
Codex attaches via stdio. (3) Envelope: A2A-aligned (Message/Parts, task lifecycle
pending → input-required → completed/rejected), Ed25519-signed with per-device keys
minted at invite enrollment. Signature proves origin, never intent.

**Also fixed here:** relay never sees vendor credentials; one relay per team, no
federation in v1.

**Evidence:** mcp.md, a2a.md (steal lifecycle, not transport), transport.md,
vendor-tos.md ("inside the vendor's client" pattern).

**Verdict:** APPROVED (maintainer, 2026-07-15)

## D-07 — Relay hosting: Go binary, VPS/docker as flagship deployment

**Decision:** One Go binary + SQLite behind any HTTPS reverse proxy; `docker run`
or bare VPS. Single language across relay + daemon. Cloudflare Durable Objects port
is a v2 aspiration, not a v1 target.

**Options considered:** (a) Go binary/VPS — CHOSEN; (b) Cloudflare DO first
(bilingual repo, weaker self-host story); (c) both from day one (rejected: two
implementations of untested software).

**Verdict:** APPROVED (maintainer, 2026-07-15)

## D-08 — Project-operated hosted relay: at OSS launch, not before

**Decision:** v1 development + Kosmoy dogfooding on Kosmoy's self-hosted relay. A
project-operated free multi-tenant instance ships as a launch asset (the 60-second
demo path for claude.ai/ChatGPT users), after the dogfooding period hardens
multi-tenancy. (Any self-hosted relay is still publicly reachable over HTTPS — that
is a protocol requirement, not tenancy.)

**Options considered:** (a) at OSS launch — CHOSEN; (b) from the start (strangers'
traffic on wet-cement isolation code; Asana MCP cautionary tale); (c) never
(amputates the launch demo).

**Verdict:** APPROVED (maintainer, 2026-07-15)

## D-09 — Stack: Go everywhere in v1; the challenged default survives

**Decision:** Go for relay and daemon (settled by D-07 + stack-fitness.md: official
MCP Go SDK is the healthiest of the three, single-binary cross-compilation,
tsnet option open). TypeScript appears nowhere in v1. Distribution priority per
oss-adoption-dynamics.md: paste-a-URL (remote MCP, zero install) → brew/curl
(GoReleaser) → docker → thin npm wrapper package for npx compatibility as a launch
asset. `go install` never promoted.

**Verdict:** APPROVED (maintainer, 2026-07-15) — Go picked explicitly in D-07 with
language consequence stated in the question.

## D-10 — Security posture: authenticated-but-never-benign, architectural controls only

**Decision:** Colleagues' sessions are authenticated but never benign (signature
proves origin, not intent — sender's model may be injected upstream). Inbound
messages: spotlighting delimiters + provenance banner, no tool triggering, no
action without approval (D-03), clients never auto-fetch URLs/images in message
content (EchoLeak/AgentFlayer exfil channel). No ML content-scanning guardrails in
v1 (false-confidence per security.md). Relay sees plaintext (E2EE deferred, D-05);
mitigations: self-host in company, TLS, **ephemeral retention — relay deletes
messages after delivery-ack, configurable grace, no history**. Threat-model doc
ships in the repo at launch. Identity: admin invite links mint per-device Ed25519
keys; roster-only messaging; admin revocation; OIDC/GitHub-org verification is v1.5.

**Verdict:** APPROVED (maintainer, 2026-07-15) — presented as posture memo; no objection.

## D-11 — Outbound reply review: on by default, grantable off per thread

**Decision:** B's human reviews the AI-drafted reply before it returns to A;
revocable per-thread auto-send grants, logged. Symmetric with D-03 — one mental
model: "my AI never receives or sends without me, unless I said this thread is fine."

**Options considered:** (a) review + grants — CHOSEN; (b) auto-send after inbound
approval (leaks ride in helpful answers); (c) always-review, no grants.

**Verdict:** APPROVED (maintainer, 2026-07-15)

## D-12 — Secret redaction: built-in patterns + pluggable hook, client-side, v1

**Decision:** Deterministic local scrubbing before send: known token shapes (cloud
keys, JWTs, PEM blocks, .env assignments) redacted with visible markers + sender
warning; config hook for company-specific patterns. Runs client-side; relay not
involved.

**Options considered:** (a) built-ins + hook — CHOSEN; (b) hook only (default
protects nobody); (c) defer to v2.

**Verdict:** APPROVED (maintainer, 2026-07-15)

## D-13 — License: Apache-2.0 for everything; DCO; no CLA ever

**Decision:** Whole repo Apache-2.0 with NOTICE file; DCO sign-off enforced from
first external PR; explicitly no CLA (stated in CONTRIBUTING.md as a trust signal);
GOVERNANCE.md with maintainer list, decision process, and a no-relicense pledge.
No commercial hosted-relay intent — verdict given knowing DCO makes the license
effectively immutable at first outside contribution. CC-BY-4.0 for spec text if the
protocol doc is ever split out (MCP/A2A pattern).

**Options considered:** (a) Apache-2.0 everything — CHOSEN; (b) AGPL relay + Apache
clients (kept as documented fallback, not taken); (c) undecided (rejected — blocks
outside contributions).

**Verdict:** APPROVED (maintainer, 2026-07-15)

## D-14 — Repo home: personal account now, neutral org after Kosmoy adoption

**Decision (maintainer's own framing):** Repo stays on the personal GitHub account
(Mediacom99) through the rename and initial dogfooding. Once colleagues use it and
Kosmoy actually adopts it, migrate to a neutral GitHub org named after the project,
with Kosmoy credited as founding sponsor. GitHub transfer redirects make the later
move cheap; the org-name availability is checked as part of the naming decision so
the namespace is claimable when the time comes. Org migration becomes a launch/
post-dogfooding checklist item, not a Phase 3 step.

**Verdict:** APPROVED (maintainer, 2026-07-15) — custom option, verbatim intent recorded.

## D-15 — Positioning: demo-shaped hook

**Decision:** Headline "Your AI can ask my AI." Subtitle "Async, approval-gated
messaging between your team's AI sessions — Claude Code, Claude, ChatGPT, Codex."
Used consistently across README, repo description, launch post.

**Options considered:** (a) demo-shaped hook — CHOSEN; (b) category-first;
(c) protocol-first (overpromises with one implementation).

**Verdict:** APPROVED (maintainer, 2026-07-15)

## D-16 — Launch playbook (adoption evidence)

**Decision:** 60–90s cross-vendor demo video seeded on X; MCP Registry publication
on launch day (subregistries auto-propagate); PR to awesome-mcp-servers; Show HN
week 1 with low expectations; threat model visible in README from day one; Discord
only after ~50 real users. Hosted instance timing per D-08.

**Verdict:** APPROVED (maintainer, 2026-07-15) — presented in memo; no objection.

## D-17 — Trademark: no pre-launch filing; EU first, then US, once traction is shown

**Decision (maintainer's own framing):** No trademark filing before public launch.
Trigger: when the project demonstrates a future (colleague/Kosmoy adoption and
external traction), first ensure EU compliance/registration (EUIPO — company is
Italy-based), then US. Knockout search still happens now as part of naming (D-18).
Accepted risk, recorded: between launch and filing, the name is defensible only by
first-use; a bad-faith filing by a third party would force a rename or a dispute.

**Verdict:** APPROVED (maintainer, 2026-07-15) — custom option, verbatim intent recorded.

## D-18 — Project name: askrelay

**Decision:** The project is named **askrelay**: repo `Mediacom99/askrelay` (per
D-14, personal account now; the free `askrelay` GitHub org gets claimed at the
D-14 migration moment), binary `askrelay`, npm wrapper `askrelay`, module path
`github.com/Mediacom99/askrelay`, domains from `.dev/.io/.sh` (registrar
confirmation on the checklist; `.com` is squatter-parked since 2026-07-02 —
ignore or negotiate later).

**Process:** 18 candidates from three naming philosophies, each collision-checked
live (GitHub, npm, domains, existing products, trademark web knockout); 10 viable,
ranked; 4 finalists presented. Full evidence: `docs/research/naming.md`. Known
accepted risks: descriptive name in the broader 'Relay' product family; `.com`
unavailable.

**Options considered:** askrelay (8.2) — CHOSEN; drumtalk (7.7); peerping (7.4);
peerpost (7.3); 14 others documented in naming.md.

**Verdict:** APPROVED (maintainer, 2026-07-15) — explicit sign-off; gates Phase 3.

## Open action items from Phase 2 (feed the plan/checklist in Phase 4)

- Ask Anthropic (channel named on their legal page) whether human-approved relay
  from a Pro/Max Claude Code seat is "ordinary individual usage" — before GA, and
  before any v1.1 unattended mode (vendor-tos.md).
- Human-pull the OpenAI ToS/help pages that 403'd to fetchers before GA (vendor-tos.md).
- Re-check MCP 2026-07-28 revision when it ships; re-verify long-poll tool-call
  timeouts per client empirically (mcp.md: the ~28h Claude Code figure is unverified).
- Kosmoy legal: owner-of-record for copyright/trademark; employer-IP terms for
  contributors (oss-licensing.md).
- Verify Kosmoy team's actual ChatGPT plan mix (write-MCP is Business/Enterprise/Edu
  beta; Plus unresolved) before promising the ChatGPT answerer flow (chatgpt-*.md).
