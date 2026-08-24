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

**Revision 2026-07-19 (commercialization-posture review — substance UNCHANGED,
guardrails added). Evidence: `docs/research/commercial-timing-effects.md`,
`commercial-hosted-economics.md`.** Four additions:

1. **Hard quotas from day one** — small per-org daily message caps, short message
   TTLs, rate limits, enforced at launch, not retrofitted. Cautionary precedent
   for a relay-shaped free service: ntfy's free hosted relay reached ~25,000
   daily users, converted 20 to paid, and had to retreat quotas from 17k to 250
   msgs/day against entrenched expectations. Hosted-tier expectations lock at
   public launch and cannot be walked back.
2. **Written launch framing** (README + hosted-instance page): "Self-hosting is
   free forever. This hosted instance is a quota-capped demo that MAY become a
   paid tier." Verified: stating paid intent at day one cost adoption nowhere
   (Plausible, n8n); backlash attaches only to dishonest labeling / retroactive
   enclosure (n8n, MinIO, Caddy, Cal.com). **No pricing is published** — category
   WTP is undemonstrated, and a price would precede (and prejudice) any future
   employer-IP/duty-of-loyalty review.
3. **WP-16 built paid-tier-READY** — multi-tenancy boundaries, per-org quota
   hooks, billing seams stubbed, billing-neutral config — but **no billing,
   invoicing, or SLA goes live**. At the modal outcome a live tier earns
   ~€0–200/month against real ops burden.
4. **Pre-registered trigger to revisit paid hosting:** sustained multi-team
   organic usage of the free instance (25+ active external, non-Kosmoy orgs) or a
   first unsolicited purchase request, whichever comes first. Until then the
   first-dollar path is sponsorship/GitHub Sponsors, not hosting invoices.

**Verdict:** APPROVED (maintainer, 2026-07-20) — adopted as drafted.

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

**Revision 2026-07-19 (commercialization-posture review — decision UNCHANGED,
recorded premise corrected, fallback retired). Evidence:
`docs/research/commercial-license-mechanics.md`, `commercial-posture-verdict.md`.**

The original verdict was given "knowing DCO makes the license effectively
immutable at first outside contribution." Live-verified license mechanics
**invert** that premise. Apache-2.0 §2 grants every licensee — including the
maintainer — a perpetual, irrevocable right to sublicense and to distribute
derivative works under different terms (§4 requires only notice retention), and
FSF/ASF confirm one-way Apache-2.0 → (A)GPLv3 compatibility. So even after
external DCO contributions, **future versions can lawfully move to AGPL, BSL,
open-core, or proprietary without contributor consent** (executed precedent:
Synapse, DCO-only, Apache→AGPL Dec 2023; MinIO 2021 — no consent campaign, no
litigation). What locks irrevocably — at first *publication*, cemented
line-by-line thereafter — is only *exclusivity*: every published line stays
Apache-2.0 for everyone forever, so any later restrictive pivot competes against
its own free fork, and anyone (a cloud vendor, or Kosmoy) may host askrelay
commercially. Recorded caveat: no court has ruled on unilateral Apache→AGPL
forward relicensing; this rests on license text plus settled practice.

Option (b), the AGPL-relay fallback, is hereby **RETIRED**. It is the *only*
direction that genuinely hard-locks — AGPL-inbound with DCO forecloses returning
to permissive or selling proprietary exceptions without unanimous consent or
rewrite (Element added a CLA at its switch for exactly this reason), so it was
only ever exercisable before the first external contribution. Not worth
exercising: AGPL carries documented enterprise-adoption friction (Google's hard
ban, CNCF's Apache-2.0 default) fatal to a category-creation tool that must
spread frictionlessly inside companies, and would not stop the realistic clone
vector — clean-room reimplementation of an open protocol (Vaultwarden,
DocumentDB). Apache-2.0 + DCO + no CLA is therefore the *option-maximizing*
posture: paid hosting, maintainer-authored closed add-ons (Sidekiq/Caddy pattern,
separate repos), and Element-playbook forward relicensing of future versions all
remain available indefinitely; the binding relicensing clock is social (cost
scales with community size), not legal.

Wording constraints carried forward: the GOVERNANCE.md no-relicense pledge stays,
phrased as a **binding promise** ("the relay core stays Apache-2.0"), never as
structural impossibility — the candidate README line "no CLA means we structurally
cannot pull a Cal.com" is false under Apache inbound (and its Cal.com premise
failed primary-source verification: Cal.com's core was AGPL without a CLA); it
must not ship. Where the docs assert license mechanics, cite primary sources
(Apache-2.0 §2/§4, FSF/ASF compatibility pages, Synapse v1.48 contributing guide,
Element relicensing announcement), not research-brief URLs.

**Verdict:** APPROVED (maintainer, 2026-07-20) — adopted as drafted.

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
`github.com/Mediacom99/askrelay`, domains from `.dev/.io/.sh` (`.com` is
squatter-parked since 2026-07-02 — ignore or negotiate later).

**Status update 2026-07-16:** maintainer purchased and owns **askrelay.dev** —
the registrar-confirmation caveat is closed.

**Process:** 18 candidates from three naming philosophies, each collision-checked
live (GitHub, npm, domains, existing products, trademark web knockout); 10 viable,
ranked; 4 finalists presented. Full evidence: `docs/research/naming.md`. Known
accepted risks: descriptive name in the broader 'Relay' product family; `.com`
unavailable.

**Options considered:** askrelay (8.2) — CHOSEN; drumtalk (7.7); peerping (7.4);
peerpost (7.3); 14 others documented in naming.md.

**Verdict:** APPROVED (maintainer, 2026-07-15) — explicit sign-off; gates Phase 3.

## D-19 — Market verdict recorded: GO_WITH_CHANGES

**Decision:** The market go/no-go research (six adversarially-verified briefs +
three-lens judge panel, 2026-07-16, evidence in `docs/research/market-*.md`,
synthesis in `docs/research/market-verdict.md`) returned **GO_WITH_CHANGES**
(panel 2–1: bull 72%, base-rate 70% for; bear 60% against). The maintainer
records the verdict and its evidence; the ten recommended changes are decided
**individually** in a follow-up session (each adopted change gets its own
D-entry or doc amendment; each rejected one is noted with reason). Until those
decisions land, the verdict's two hard sequencing facts stand as advisory:
the askmesh autopsy belongs before application code, and the kill criteria in
market-verdict.md are the reference framing for v1 success metrics.

**Verdict:** APPROVED (maintainer, 2026-07-16) — "record verdict only, decide
changes together afterwards."

**Resolution (maintainer, 2026-07-16 — each change decided individually):**

- C1 askmesh autopsy — **ADOPTED as a gate before WP-01** (plan spike S-05,
  kill criterion dated 2026-08-01). Maintainer extension: S-05 also produces a
  differentiation memo — askrelay positions as a different solution to a
  different problem (consented asking across trust boundaries, not a shared
  knowledge mesh) while keeping the core functionality; extraction of
  askmesh's lessons is an explicit spike output. Guard: positioning cannot
  rescue a dead category — the autopsy verdict still governs kill criterion 1.
- C2 positioning — **ADOPTED MODIFIED**: "Your AI can ask my AI." stays the
  headline; subtitle and README opening foreground the moat surfaces
  (cross-vendor, cross-org, self-hosted, approval-as-product) and the
  answering-side differentiator (B's live local session context). *This
  amends D-15.*
- C3 same-owner cross-machine scenario — **ADOPTED** as a documented day-one
  scenario (architecture §2, README), vendor-exposure argued explicitly.
- C4 `Approvable{Kind, Payload}` gate abstraction — **ADOPTED** (architecture
  §5.3, WP-02).
- C5 ship discipline — **ADOPTED**: v1 deployed at Kosmoy by 2026-10-15 as a
  kill criterion; maintainer commits to ≥monthly releases for six months.
- C6 kill criteria — **ADOPTED verbatim** into plan §8, pre-registered before
  code.
- C7 launch package — **ADOPTED** (co-maintainer recruitment, public cadence
  statement, non-dismissive vuln-response precommit in SECURITY.md).
- C8 Kosmoy sponsorship — **ADOPTED MODIFIED**: paid-maintenance-hours
  conversation deferred until dogfooding proves value ("ask later").
- C9 cross-org tester pairs — **ADOPTED MODIFIED**: 2–3 pairs post-v1, timing
  flexible (stability before goodwill-burn).
- C10 planning-hygiene corrections — **N/A**: no governing doc cites the
  corrected figures; corrections live in the market briefs' verification
  sections.

## D-20 — Commercialization posture: pure OSS now, paid-tier optionality preserved

**Decision:** askrelay stays **Apache-2.0 pure OSS** with a free, quota-capped
hosted demo (D-13 and D-08 as revised 2026-07-19). Open-core-with-pricing-now and
proprietary are both **rejected**: verified willingness-to-pay in the exact
category is zero (BAND, the only direct competitor with $17M seed, has no published
pricing and no named customers; HumanLayer abandoned its approval *API* for a
per-seat IDE — approval-gating reads as a feature, not a purchasable product), and
at the modal outcome a paid tier earns ~€0 while spending the project's two real
assets: the flagship-OSS goal and the trust story of a consent/security product.
Because Apache-2.0 inbound preserves every future posture (paid hosted tier,
maintainer-authored closed add-ons in separate repos, Element-playbook forward
relicensing of future versions — see the D-13 revision), **deferring the
commercial decision is legally free**; the clocks that bind early are social
(D-08 launch framing), not licensing.

**Options considered:** (a) pure OSS + paid-tier-ready seams — CHOSEN; (b) AGPL-
relay split now (rejected/retired with the D-13 revision); (c) open-core with
published pricing at launch (rejected — prices an undemonstrated product; its two
sound elements, hard quotas and honest "may become paid" framing, are absorbed
into the D-08 revision); (d) proprietary (rejected — nothing to monetize, certain
trust cost for a consent product).

**Employer-IP / entity questions — DEFERRED (maintainer, 2026-07-20):** the
research flagged Italian employer-IP (art. 12-bis) and ownership/entity structure
as items to settle before launch. The maintainer has parked these for a later
dedicated pass ("we'll figure it out later"); they are **not** committed as
decision or checklist gates here. Single risk flag retained for that future pass:
if a Kosmoy colleague contributes before the employer-IP question is settled, the
default assignment rule could complicate provenance — worth resolving before, not
after, first colleague contributions. Full analysis and draft actions preserved in
`docs/research/commercial-entity-sponsor.md` and `commercial-posture-verdict.md`.

**Verdict:** APPROVED (maintainer, 2026-07-20) — posture recorded; employer-IP
deferred by maintainer choice.

## D-21 — Strategic framing: why we build, what success means, how we position

**Decision (maintainer + cofounder, 2026-07-20 — after the S-05 askmesh autopsy
cleared the go/no-go):** the honest framing the whole project now runs on.

**Why we build it (stated plainly, no market-pull pretense).** Direct demand is
unproven, not disproven (askmesh, the one prior attempt, was never launched —
`docs/research/askmesh-autopsy.md`). We build anyway, for three legitimate
reasons: (1) it is an excellent testbed for this repo's agent-pipeline
experiment (Fable planning, Sonnet subagents building it WP-by-WP); (2) it is a
flagship that demonstrates real rigor — the verified research trail, threat
model, and decision log are portfolio-grade regardless of stars; (3) it is a
cheap option on a category that may matter in ~2 years, with a high-salvage
pivot (approval-gateway) if it doesn't.

**Definition of success (non-numeric, binding).** Success = "shipped a rigorous,
secure, genuinely useful tool that a real team (Kosmoy) uses, learned to build
via the agent pipeline, and handled scrutiny well." Explicitly **not** stars or
revenue — the research is honest that those are a low-probability tail (D-19,
D-20). If a number ever becomes the felt measure of success, that is the signal
to revisit scope, not to push harder.

**Positioning (sharpens D-15/C2).** Lead the demo and the pitch with the
**answering-side moat**: the colleague's AI answers from inside their *live
local session* — uncommitted code, terminal state, private repos — which no
shared channel bot or org assistant can reach. That is the "oh" moment. The
memorable hook ("Your AI can ask my AI") stays; the moat surfaces
(cross-vendor, cross-org, self-hosted, approval-as-product) remain the subtitle.

**Wedge (elevates D-19/C3).** Same-owner cross-machine messaging (your own agents
across your machines) is promoted from "documented day-one scenario" to a
**candidate front door**: it is the only flavor with demonstrated demand (the
#28300 cluster, mcp_agent_mail) and it makes first-run useful before any
teammate replies — directly softening the cold-start hazard (R-11). Cross-person
is the differentiating superset. Which of the two leads the public launch is a
Phase-7+ call to make with the working tool in hand.

**Differentiation from askmesh (the "different problem, same core" the maintainer
asked for).** askmesh automates answering (closed Claude-only cloud, no
inbound-trust model); askrelay makes **consented, safe, cross-vendor asking**
the product. Same core mechanic, opposite center of gravity. Used in docs, never
as public comparison marketing (nobody knows askmesh).

**Top operating commitment.** Above release cadence, above everything: fast,
non-dismissive security-vulnerability response. For a consent/approval product, a
slow or defensive answer to a consent-bypass report is the one outcome that
actively damages the maintainer's reputation in the exact dimension the flagship
showcases. This outranks the other C7 commitments.

**Verdict:** APPROVED (maintainer, 2026-07-20) — "I agree with you on everything."

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

## D-22 — Development process: maintainer-supervised, subtask-level implementation

**Decision (maintainer, 2026-07-23):** work packages are no longer implemented
autonomously by a subagent. Instead the main session (Fable) divides each WP
into small, logically-coherent subtasks and, for each, **before writing any
code**, explains what it builds, why this way (alternatives rejected) and the Go
idioms involved, then shows the proposed code and waits for the maintainer's
confirmation; only then is it written. Goal: the maintainer is a full supervisor
and understands the whole codebase as if they wrote it. The three quality agents
(`askrelay-test`, `askrelay-review`, `askrelay-security`) still run — once per WP
after all subtasks are assembled — as an independent adversarial pass whose
findings return to the maintainer. Teaching depth: design rationale + Go idioms,
assuming fluent code reading.

**Options considered:** (a) main session implements in-conversation, keep the
quality agents — CHOSEN; (b) drop the quality agents too (rejected — loses the
independent adversarial passes that already caught T-16 and the WP-01 security
review); (c) keep a reshaped autonomous-ish dev agent (rejected — indirection
between maintainer and code defeats the "understand as if I wrote it" goal).

**Consequences:** `askrelay-dev` is **RETIRED** (its autonomous whole-WP role
contradicts supervision; recoverable from git history). Plan §2 protocol and
CLAUDE.md rewritten to this model. `askrelay-test`/`-review`/`-security` keep
their contracts with only the stale "after askrelay-dev" phrasing updated to
"after the WP is implemented." The prompt-crafting skill was **not** invoked:
the surviving agents needed only a trivial contextual edit, not a prompt-quality
rework, so using it would have been ceremony. Applies from **WP-02 onward**;
WP-01 was the last (and only) autonomous run.

**Verdict:** APPROVED (maintainer, 2026-07-23) — process co-designed via three
confirmed choices.

## D-23 — Local-first framing + "the other party can be a session you own"

**Decision (maintainer, 2026-07-31):** adopt **local-first** as the primary
go-to-market framing, and recognise **self-talk** (one operator, multiple of
their own model-sessions) as the first-user on-ramp. Neither is a pivot away
from the cross-person thesis (D-01) — both fall out of the existing
architecture unchanged.

**Rationale / why this holds:**
- *Local is not a separate mode.* The relay is one binary over HTTPS; localhost,
  LAN, and VPS are the same code (D-06/D-07). "Push it for local use first" costs
  zero engineering — it is a positioning choice, not a rebuild.
- *Self-talk needs no protocol change.* The §5.1 tool surface (`send_message` /
  `wait_for_activity` / `check_inbox`) already works when the party on the other
  end is another identity the same operator owns (e.g. a Claude Code session
  asking a second-opinion model, async, and pulling the answer back in). This
  lets the maintainer be user #1 and exercise the whole loop **solo**, directly
  de-risking the cold-start/retention risk (R-11, D-21) instead of removing the
  one guaranteed counterparty.

**Scope boundaries (what this decision does NOT endorse):**
- *No driving consumer web sessions.* Open ChatGPT/KIMI/Claude browser tabs
  cannot be puppeted (vendor ToS, D-10 constraint; brittle scraping). Such tools
  participate only as MCP **clients that connect out to the relay**, never as
  surfaces the relay reaches into. A "see all my open AI tabs and wire them
  together" dashboard is therefore out of scope for that reason, not just
  priority.
- *Auto-approving your own sessions is the v1.1 auto-reply path* (API-key-only,
  pending Anthropic ToS clarification — D-03/standing constraints), not a v1
  bypass of the approval gate.
- *A dashboard is endorsed only as an operator / approval-and-observability
  console* (threads, pending approvals, grants, enrolled devices), built once
  messages actually flow (after WP-08) — not as a drag-drop session-orchestration
  canvas.

**Sequencing consequence:** the local-first path is **WP-08 (delivery) →
WP-09 (daemon)** on device credentials (WP-05, shipped). **WP-06 (embedded
OAuth AS) is off this critical path** — it authenticates only the zero-install
*browser*-connector flow. Deferring browser clients also lets WP-08 defer its
open **browser-client-signing** sub-decision (same "keyless browser client"
cluster as WP-06). WP-08 is thus scoped to device-credential delivery for now;
WP-06 + browser signing are taken together when the browser-connector path is
wanted.

**Verdict:** APPROVED (maintainer, 2026-07-31).

## D-24 — Browser AS "login" is device-credential paste (Option A); authN+consent are one step in v1

**Decision (maintainer, 2026-08-08, during the WP-06 build):** the embedded
OAuth authorization server's "login page" authenticates the person by having
them paste their existing **device credential** (§4.3), which the AS verifies
and re-checks live (`ActiveDeviceByID`) before minting an authorization code.
This is the smallest path that makes WP-06's device-re-check obligation exact
and keeps the browser-client-signing sub-decision deferred (D-23) — no
relay-held browser key, delivery stays relay-attested (D-10/D-23).

**Known tradeoff (surfaced by the WP-06 security pass):** authentication and
consent collapse into a single POST — pasting the long-lived device credential
both proves identity *and* authorizes the requesting client, with no separate
"authorize THIS app for THESE permissions" screen and no scope shown. Because a
legitimate connect flow trains the same paste-your-credential gesture, a
phishing link to `/oauth/authorize?client_id=<attacker>&…` served from the
*real* relay domain can harvest a full-privilege access+refresh token if the
human obliges. This is inherent to any bearer-credential-authorizes-any-client
OAuth model (the "Sign in with X" consent-phishing class), sharpened here by the
zero-scope, single-step design. It is not a code defect — the AS enforces PKCE,
device re-check, CSP, and html escaping correctly.

**Deferred mitigation (v1.1, not blocking WP-06):** separate authN from consent —
after credential verification, establish a short-lived AS-side session and render
a second, explicit consent screen naming the resolved client (CIMD origin, or DCR
`client_name` + redirect host) and the capability being granted, requiring a
distinct confirming action before the code is minted; plus a cap on how many
*new* OAuth clients one device credential may authorize per window (a cheap
mass-authorization tripwire). No ML classifier (D-10).

**Verdict:** APPROVED (maintainer, 2026-08-08).

## D-25 — Post-loop sequencing: all of WP-09 next (sign-at-release), validation trial gated behind it

**Decision (maintainer, 2026-08-22, after the first real-client A→B→A loop
worked — `ca5718c`):** the next engineering block is **the whole of WP-09**, and
the cross-person trial that kill criterion 3 (§8) measures is gated behind it.
The WP order until now predated a working loop; this re-sequences it. Nine
sub-decisions, each taken deliberately:

1. **WP-09 in full**, not a notify-only slice. Rejected: shipping only the
   listen+notify part before the trial. It is the only path that puts
   `internal/redact` and `envelope.Sign` on a live path and so closes the last
   two legs of **R-12** — the credibility gap on a security product.
2. **Signing happens at release, not at submit — T-19.** The relay holds the
   draft unsigned, and on human approval requests a signature over the final
   (possibly edited) bytes from the author's daemon. A signature then means
   exactly "this text was approved". Accepted costs: a relay-side signing-request
   queue, and release depending on daemon liveness. Reinstating a daemon submit
   path does **not** reinstate the WP-08 **C1** gate bypass — the gate stays
   server-side and authoritative, and the daemon never releases.
3. **D-12 stands unchanged**: redaction is client-side, the relay gets no
   backstop. Redacting at the `/mcp` boundary was considered as a cheap way to
   close R-12's widest leg early (the built-and-tested `internal/redact` has no
   caller, so a pasted secret reaches the wire today) and **rejected**, because
   WP-09 puts redaction where D-12 always said it belongs — on the sender's
   machine, before the bytes leave it — making the relay-side variant a claim
   that would have to be narrowed later.
4. **The trial waits for the finished daemon.** Rejected: recruiting in parallel
   and letting participants upgrade mid-trial. The cohort is a one-shot resource;
   they get the finished product.
5. **Cohort clients: Claude Code + Codex.** Claude Code is the only client
   validated end-to-end; Codex arrives free with ST-7's stdio server. Browser
   clients (claude.ai / ChatGPT) stay out — they would pull in a browser
   enrollment page (today even a browser-only user must run the CLI once to get a
   device credential) plus spike S-04. Revisit only if a real recruit needs it.
6. **Docker/Compose is the documented default deploy** (a Coolify-class host),
   with the binary + Caddy walkthrough kept as the no-container alternative. The
   container needs **one persistent volume covering both the SQLite file and
   `signing.key`**; TLS moves to the platform. This changes `docs/deploy/` and
   the site's self-host page.
7. **Measurement instrumentation lands before the trial, not after.** Thread id
   added to the `send_message` / `approve_message` / `approve_reply` INFO lines
   (ids only — T-18 intact). Distinct pairs come from the permanent `threads`
   table, per-pair volume and approval latency from those lines. It cannot be
   backfilled, which is the whole reason it is a decision.
8. **WP-10 deferred past the trial.** The daemon's own OS notification is the v1
   push UX; the channels bridge and prompt-submit hook wait for observed friction,
   which also lets **R-02** (the preview's flag/allowlist status) resolve itself.
9. **Public proof is a recorded demo, not a landing page.** A 60–90 s terminal
   capture of the real two-session loop, into the README slot and the docs home;
   docs.askrelay.dev already does the landing-page job. Cohort *sourcing* is
   deliberately left open until WP-09 lands.

**Owed before the next push (R-12, honesty standard):** README claims signed
envelopes, client-side secret redaction, and "deleted after delivery" as live —
none is true on the shipped path (nothing acks, so bodies live to the 30-day hard
TTL), and it still lists `device` among pending CLI verbs. docs.askrelay.dev is
already correct on all of it. The `ca5718c` docs mirror is also still owed.

**Recorded risk, knowingly accepted (maintainer, 2026-08-22):** choosing the full
daemon over an earlier trial puts kill criterion 2's **2026-10-15** deploy date
at risk, and criterion 3 needs 8 observation weeks against a 2027-01-31 hard
stop. The maintainer accepted this after seeing the arithmetic and declined to
re-sequence around the dates. If criterion 2 is missed, the response is that
criterion's own instruction — **cut v1 scope, never extend the timeline**.

**Verdict:** APPROVED (maintainer, 2026-08-22).
