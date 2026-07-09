# pchat — v1 Implementation Plan

**This document is the source of truth for *execution*.**
[`pchat-architecture.md`](pchat-architecture.md) remains the source of truth for *design
intent*; where the two disagree, an **APPROVED** entry in the [Decision log](#5-decision-log)
wins (that is the mechanism by which the spec's own contradictions — e.g. §4's custom
signing vs §13's critique of it — get resolved).

Every claim in this plan about a third-party API (import paths, signatures, defaults,
versions) was verified against live sources on **2026-07-09** — module-cache source
reading plus compile-and-run proof programs — not taken from the spec or from memory.
Corrections to previously committed assumptions are recorded in the
[Learnings log](#7-learnings-log).

How to work from this document: see [§2 Execution protocol](#2-execution-protocol).

---

## 1. Status board

### Work packages

| WP | Title | Depends on | Gated by decisions | Status | Commits / PR |
|----|-------|-----------|--------------------|--------|--------------|
| [WP-01](#wp-01--internalidentity-glyphcolor-identity) | `internal/identity` — glyph+color identity | — | T-30 | TODO | |
| [WP-02](#wp-02--internalprotocol-wire-envelope--guards) | `internal/protocol` — wire envelope + guards | — | T-01 T-02 T-03 T-04 T-05 T-33 | TODO | |
| [WP-03](#wp-03--internalp2p-host) | `internal/p2p` host | WP-01 | T-07 T-25(shim) | TODO | |
| [WP-04](#wp-04--internalp2p-room-gossipsub--mdns--validators) | room: gossipsub + mDNS + validators | WP-02 WP-03 | T-08 T-13 T-16 T-17 | TODO | |
| [WP-05](#wp-05--dht-discovery-amino) | DHT discovery (Amino) | WP-04 | T-09 T-10 | TODO | |
| [WP-06](#wp-06--rendezvous-client) | rendezvous client | WP-04 WP-07 | T-11 T-12 | TODO | |
| [WP-07](#wp-07--cmdpchat-point-rendezvousrelay-vps-daemon) | `cmd/pchat-point` VPS daemon + deploy | — (WP-03 recommended first) | T-28 T-29 | TODO | |
| [WP-08](#wp-08--ui-core--cli-wiring-chat-mvp) | UI core + CLI wiring (chat MVP) | WP-04 | T-14 T-15 T-18 T-23 T-31 P-01 | TODO | |
| [WP-09](#wp-09--roster-presence-typing-mute-churn) | roster, presence, typing, mute, churn | WP-08 | T-16 T-17 T-19 P-02 | TODO | |
| [WP-10](#wp-10--command-system--ux-polish) | command system + UX polish | WP-09 | T-24 P-01 P-04 P-07 | TODO | |
| [WP-11](#wp-11--devprod-build-split-15) | dev/prod build split (§15) | WP-08 | T-25 T-26 | TODO | |
| [WP-12](#wp-12--release-pipeline-goreleaser--signing) | release pipeline (goreleaser + signing) | WP-07 WP-11 | T-27 | TODO | |
| [WP-13](#wp-13--readme-threat-model-positioning-aup) | README, threat model, positioning, AUP | — (docs only) | T-01 P-01 P-02 P-03 P-04 P-06 P-08 | TODO | |
| [WP-14](#wp-14--platform-matrix--launch-verification) | platform matrix + launch verification | WP-05 WP-06 WP-10 WP-12 WP-13, WP-07 deployed | T-22, P-05 complete | TODO | |

Package status lifecycle: `TODO → IN_PROGRESS → DONE` (plus `BLOCKED(reason)` when a
session discovers an impediment). Only one package `IN_PROGRESS` at a time unless the
[dependency graph](#4-dependency-graph) shows the packages on independent tracks.

### Decisions (summary — full entries in §5)

All technical decisions **T-01…T-33** and product decisions **P-01…P-08** are
**PROPOSED** as of 2026-07-09 and await maintainer verdicts, except where noted.
Decision status lifecycle: `PROPOSED → APPROVED | VETOED | REVISED` (an entry is never
deleted; verdicts are appended with a date). A `VETOED` decision blocks every package
that lists it as a gate until a `REVISED` replacement is APPROVED.

---

## 2. Execution protocol

**For every fresh session picking up work** (the maintainer's kickoff prompt is at the
end of this section):

1. Read `CLAUDE.md`, then this document. Do not re-derive anything from chat history —
   there is none; this document plus the spec sections each package cites are the full
   context.
2. **Pick work:** choose the lowest-numbered WP whose Status is `TODO`, whose
   *Depends on* packages are all `DONE`, and whose *Gated by* decisions are all
   `APPROVED`. If two packages are eligible on independent tracks and the session is
   confident, still do only one — packages are sized to one reviewable PR.
3. Set its Status to `IN_PROGRESS` in the status board (commit this edit — it is the
   claim marker for concurrent sessions).
4. Implement exactly what the package entry says, consulting the spec sections it
   cites. The package entry wins over the spec where they differ (that difference is
   always backed by a decision entry).
5. Add dependencies with `go get module@version` **only** at the versions pinned in
   [§3](#3-verified-dependency-matrix), only when first imported.
6. Run the package's full test plan, plus `make build`, `make lint`, `go vet ./...`,
   and (once WP-11 exists) both tag sets. The tree must be green before the PR.
7. Update the status board row: `DONE`, commit refs / PR number. Append to the
   [Learnings log](#7-learnings-log) anything discovered that changes a later package —
   and if it invalidates a later package's content, edit that package *in the same
   commit* and note the edit in the log entry.
8. If the session discovers a genuinely new design question, add a `PROPOSED` decision
   entry rather than silently choosing. Rule of thumb: wire format, security posture,
   or anything user-visible at launch **waits** for APPROVED; internal code-structure
   choices may proceed and be recorded as PROPOSED for retroactive veto.
9. Never violate the standing constraints: nothing written to disk at runtime by the
   client, no new flags beyond the §6 set without a decision, prod builds contain no
   dev instrumentation (§15), one PR per package, build always green.

**Kickoff prompt (copy-paste to start an implementation session):**

```
Read CLAUDE.md, then docs/pchat-implementation-plan.md. Following its §2 Execution
protocol, pick the next eligible work package (lowest-numbered TODO with all
dependencies DONE and all gating decisions APPROVED), set it IN_PROGRESS, implement
it per its entry and the spec sections it cites, run its full test plan, mark it
DONE with commit refs, record any learnings that affect later packages, and commit.
One package only. If no package is eligible, report exactly what is blocking.
```

**Maintainer's decision workflow:** review §5, append `APPROVED` / `VETOED` (+ reason)
to each entry (or ask a session to record verdicts). Vetoing a technical decision:
the affected packages stay blocked until a REVISED entry exists. Vetoing a product
decision (P-xx): the affected acceptance criteria are struck and the packages updated
in the same commit.

---

## 3. Verified dependency matrix

Ground truth as of 2026-07-09, established by fetching each module and
compiling/running proof programs against it (see Learnings log for what this
corrected). **These supersede the CLAUDE.md table**; CLAUDE.md is updated in the same
commit as this plan. `go.mod` stays empty until each package's first import (unchanged
policy).

| Module | Pin | Enters at | Verified facts that matter |
|---|---|---|---|
| `github.com/libp2p/go-libp2p` | v0.48.0 (latest; needs go ≥ 1.25.7) | WP-03 | Explicit `Transport`/`Security` options **replace** defaults (drops WS/WebTransport/WebRTC/TLS — desired). Relay client on by default; hole punching opt-in; AutoNAT v1 always on, v2 opt-in via `EnableAutoNATv2()`. |
| `github.com/libp2p/go-libp2p-pubsub` | v0.16.0 | WP-04 | StrictSign is the default (see T-01). Flood-publish **off** by default — must enable. Do **not** lower `WithMaxMessageSize` (protocol-ID coupling). v0.17.0 released 2026-07-09 — not adopted (T-32). |
| `github.com/libp2p/go-libp2p-kad-dht` | v0.41.0 (2026-07-01) | WP-05 | go.mod requires exactly go-libp2p v0.48.0 — zero skew. Amino bootstrap peers; provide works from ModeClient; records linger ≤48 h. |
| `github.com/waku-org/go-libp2p-rendezvous` | v0.0.0-20240110193335-a67d1cc760a0 (no tags; pin pseudo-version) | WP-06 (client), WP-07 (server) | **Root package is cgo-free** (verified `go list -deps`: no sqlite/`database/sql`) — the earlier CLAUDE.md claim was wrong. Proven working against go-libp2p v0.48.0 end-to-end with `CGO_ENABLED=0`. Server storage is an interface (`db` dir, package `dbi`, 6 methods); sqlite impl is a *subpackage* we never import. |
| `github.com/fxamacker/cbor/v2` | v2.9.2 (latest) | WP-02 | No CVEs (OSV). Decode guards exist as `DecOptions` fields. `keyasint` needs ≥v2.9.1 (key-confusion fixes). Trailing bytes rejected by default. |
| `charm.land/bubbletea/v2` | v2.0.8 | WP-08 | **v2 lives at `charm.land/*`, not `github.com/charmbracelet/*/v2`** — mixing paths double-compiles bubbletea. ≥v2.0.7 required (Windows resize regression fixed). See T-31. |
| `charm.land/bubbles/v2` | v2.1.1 | WP-08 | viewport with native `SoftWrap` (v1 viewport cannot wrap); textinput. Missing from the old CLAUDE.md table entirely. |
| `charm.land/lipgloss/v2` | v2.0.5 | WP-08 | Color degradation now happens in bubbletea's renderer (`colorprofile`); model gets `tea.ColorProfileMsg` at startup. |
| `golang.org/x/term` | latest at import time | WP-08 | Hidden passphrase prompt (`term.ReadPassword`) for `--room -` (T-14). |
| Dev-only (behind `//go:build dev`): `github.com/prometheus/client_golang` | latest at import time | WP-11 | Present in go.mod but compiled only into dev builds. |

Release tooling (not go.mod): goreleaser **v2.17.0** (config `version: 2`),
`goreleaser/goreleaser-action@v7`, `actions/checkout@v7`, `actions/setup-go@v6`,
`sigstore/cosign-installer@v4` (cosign v3: `sign-blob --bundle` → one
`.sigstore.json` per artifact; the old `.sig`+`.pem` layout is legacy),
`actions/attest-build-provenance@v4`.

---

## 4. Dependency graph

```
WP-01 identity ──► WP-03 host ──────────────► WP-04 room ─┬─► WP-05 DHT ─────────┐
WP-02 protocol ─────────────────────────────►             │                      │
                                                          ├─► WP-06 rendezvous ──┤
WP-07 pchat-point (independent; memstore) ────────────────┘        ▲             │
        │  (WP-06's integration test uses WP-07's memstore) ───────┘             │
        │                                                                        │
        │                                  ┌─► WP-08 UI+wiring ─► WP-09 ─► WP-10 ┤
        │                                  │         │                           │
        │                                  │         └─► WP-11 dev/prod ─► WP-12 ┤
        └─► (deployed VPS) ────────────────┼─────────────────────────────────────┤
                                           │   WP-13 docs (parallel, any time) ──┤
                                           │                                     ▼
                                           └──────────────────────────────► WP-14 launch gate
```

- **Roots:** WP-01, WP-02 (mutually independent — the envelope no longer carries
  identity, per T-01), WP-07 (server-side; only soft-ordered after WP-03 to reuse
  host-config patterns), WP-13 (pure docs; needs only decision verdicts).
- **Critical path:** WP-01 → WP-03 → WP-04 → WP-08 → WP-09 → WP-10 → WP-14.
- **Parallel tracks after WP-04:** (a) discovery: WP-05, WP-06; (b) UI: WP-08→09→10;
  (c) WP-11→WP-12; (d) WP-13. A solo maintainer + Claude sessions should default to
  finishing the critical path first: LAN-only chat (through WP-08) is the first
  end-to-end dogfoodable milestone.
- **Recommended overall order for sequential sessions:**
  01, 02, 03, 04, 08 *(dogfood on LAN)*, 07, 06, 05, 09, 10, 11, 12, 13, 14.
  WP-07 deliberately early after the MVP: the deployed VPS is needed for any
  real-world NAT testing, which is risk R1.

---

## 5. Decision log

Format: each entry has **PROPOSED** (the decision), **Rationale**, and **Reversal
cost**. Verdicts are appended, never rewritten. Technical (T-xx) and product (P-xx)
entries are separated so they can be vetoed independently.

**Spec-coverage map** (every §11/§12/§13 item → where it is resolved):

| Spec item | Resolved by |
|---|---|
| §11 multi-room | T-20 |
| §11 message ordering | T-06 |
| §11 max payload size | T-04 |
| §11 connection churn | T-16 |
| §11 terminal compatibility | T-22 (+ WP-14) |
| §11 testing strategy | T-21 |
| §11 graceful shutdown | T-15 |
| §11 onboarding UX ("listening for peers…") | T-10 + WP-08 acceptance |
| §11 passphrase sharing (README line) | WP-13 acceptance (no decision needed) |
| §11 version mismatch UX | T-19 |
| §11 empty/whitespace input | WP-10 acceptance (no decision needed) |
| §12 cold start / commons room | P-01 |
| §12 abuse & recourse / local mute | P-02 |
| §13 redundant envelope signing | **T-01** |
| §13 gossipsub mesh at small scale | T-08 |
| §13 unbounded connections/watermarks | T-07 |
| §13 CBOR decode limits | T-03 |
| §13 DHT IP exposure disclosure | P-03 + WP-13 (mechanics verified & sharpened: records linger ≤48 h; the ~20 DHT servers closest to the key see participant IPs without any passphrase) |
| §13 replay protection | T-05 |
| §13 Sybil (zero-cost identities) | P-08 |
| §13 passphrase in argv | T-14 |
| §13 supply chain / signed releases | T-27 (+ §3 pinning policy) |
| §14 first-user items | P-03, P-07, WP-08/WP-13 acceptance |
| §15 dev/prod split | T-25, T-26 |
| §16 license | P-06 |
| §16 operator legal exposure | P-05 |
| §16 commons acceptable-use | P-04 |
| §16 Windows terminal quirks | T-22 + WP-14 matrix |
| §16 scaling reality | WP-07 deploy doc (scale-out sketch); risk R7 |

### Technical decisions

---

**T-01 — Drop the custom envelope signing layer; rely on gossipsub StrictSign.**
*(resolves the §4 vs §13 contradiction — the single biggest design decision in this plan)*

- **PROPOSED:** Delete `sender` and `sig` from the §4 envelope. The per-session
  Ed25519 keypair (§2) *is* the libp2p host key. The wire envelope becomes
  `{v, type, ts, payload}`. Sender identity comes from the pubsub message's verified
  `GetFrom()` peer ID; the glyph derives from the Ed25519 pubkey extracted from that
  peer ID. `internal/protocol/sign.go` is deleted (scaffold placeholder). SECURITY.md's
  "signatures over every wire message" wording is updated in WP-13.
- **Rationale (verified against pubsub v0.16.0 source, not asserted):** gossipsub's
  default `StrictSign` policy signs `"libp2p-pubsub:" + protobuf(From, Data, Seqno, Topic)`
  with the host key, verifies **before** the app or any forwarding sees the message,
  drops unsigned/badly-signed messages, and rejects self-origin spoofs. The signature
  binds the message to its room (topic is inside the signed blob) — strictly stronger
  than §4's custom sig, which did not cover the topic. Ed25519 peer IDs embed the
  pubkey (identity multihash, ≤42-byte inline rule), so `peer.ID → ExtractPublicKey()
  → Raw()` yields the 32-byte key with no key exchange. Two overlapping signature
  systems = double CPU per message, double attack surface, zero gain — §13 was right.
- **What we keep at the app layer** (what pubsub does *not* give us): wall-clock `ts`
  (seqno is a counter), long-horizon replay protection (T-05), payload caps (T-04) —
  all enforced in a topic validator, which is also the §9 PoW hook.
- **Reversal cost:** moderate — re-adding fields is a protocol version bump (`v=2`)
  plus reintroducing sign/verify helpers; no architectural rework because the
  validator chain and envelope decode path stay where they are.

**T-02 — Envelope CBOR representation: integer map keys (`keyasint`), forward-compatible decode.**

- **PROPOSED:** All wire structs use `keyasint` struct tags (integer map keys).
  Unknown map keys are **ignored** on decode (i.e. `ExtraDecErrorUnknownField` is NOT
  set) — that is the forward-compatibility mechanism that lets old clients read new
  messages. Encoder mode is `cbor.CoreDetEncOptions()` (deterministic; not a
  correctness requirement since nothing is re-encoded for signing after T-01, but it
  is free and removes a class of future footguns). `toarray` is rejected.
- **Rationale (measured):** for this envelope shape, `keyasint` costs ~1 byte/field
  over `toarray` (130 B vs 124 B on a 6-field test envelope) and is ~15 % smaller than
  string keys. `toarray` is self-defeating for a versioned protocol — verified
  empirically that adding a field breaks decode *in both directions* and even makes
  the `v` field unreadable to old clients. Rejecting-unknown-fields (a §13-adjacent
  hardening instinct) directly destroys evolvability; consciously not enabled.
- **Reversal cost:** low before first release (edit tags); a wire break after launch.

**T-03 — CBOR decode guards (hardened DecMode singleton).** *(§13)*

- **PROPOSED:** one package-level `cbor.DecMode` in `internal/protocol` built from:
  `MaxNestedLevels: 8`, `MaxArrayElements: 64`, `MaxMapPairs: 64`,
  `IndefLength: IndefLengthForbidden`, `TagsMd: TagsForbidden`,
  `DupMapKey: DupMapKeyEnforcedAPF`, `UTF8: UTF8RejectInvalid` (defaults are 32 /
  131072 / 131072 / allowed / allowed / quiet — all verified in v2.9.2 source).
  Every decode goes through this mode; the package-level `cbor.Unmarshal` is banned
  (lint note in package doc). Input is pre-sliced: reject before decode when
  `len(data) > MaxWireBytes` (T-04) — there is **no** total-size DecOption (verified),
  so the pre-check is the only byte bound. Trailing garbage is rejected by
  `Unmarshal` by default (verified) — no extra check needed.
- **Rationale:** `DupMapKeyEnforcedAPF` matters specifically because `keyasint`
  decodes a CBOR map — the default "quiet" mode silently keeps one of two duplicate
  keys, a classic parser-differential vector. Depth 8 is ~3× actual envelope depth.
- **Reversal cost:** trivial (constants).

**T-04 — Size caps.** *(§11 "max payload size")*

- **PROPOSED:** `MaxWireBytes = 4608` (whole encoded envelope, checked before decode
  and before publish), `MaxPayloadBytes = 4096` (§11's 4 KB), `MaxChatTextBytes = 4000`
  (UTF-8 bytes, checked pre-publish and in the validator). The pubsub transport limit
  `WithMaxMessageSize` is **left at its 1 MiB default** — verified that changing it
  couples to pubsub protocol IDs (doc warning in source) and that oversize *outbound*
  messages are silently dropped with no error; therefore the enforcement point on send
  is our own pre-publish check, and on receive our validator, never the transport knob.
- **Reversal cost:** trivial (constants), as long as caps only ever *increase*.

**T-05 — Replay protection: freshness window + app-level dedupe.** *(§13)*

- **PROPOSED:** validator rejects (with `ValidationIgnore`, not `Reject`) any message
  whose `ts` is outside **±10 minutes** of local clock; an app-level seen-cache keyed
  by pubsub message ID (`from+seqno`) with **20-minute TTL** drops duplicates inside
  the window. Stale/duplicate drops are silent in prod (counted in dev builds).
- **Rationale (verified):** pubsub's own seen-cache holds IDs for only **120 s**, and a
  replayed old signed message after that is delivered and forwarded — the §13 concern
  is real. `ValidationIgnore` (not `Reject`) because `Reject` penalizes the *forwarding*
  peer under peer-scoring; and because a user with a skewed clock must not get their
  relaying peers punished. ±10 min (not the spec's implied "±N ≈ 2") because a too-tight
  window silently mutes users with wrong clocks — a worse failure mode than a 10-minute
  replay horizon in an ephemeral room. The 20-min dedupe TTL = 2× window, so a message
  cannot be replayed inside its own freshness window.
- **Reversal cost:** trivial (constants); the pubsub `BasicSeqnoValidator` is a
  drop-in alternative if this proves insufficient (noted, not chosen — it breaks on
  identity restart semantics we don't have, and doesn't give us wall-clock freshness).

**T-06 — Message ordering: arrival order, no re-sorting.** *(§11)*

- **PROPOSED:** render messages in local arrival order. No per-sender sequence
  numbers in the envelope, no reordering buffer. (Pubsub already carries a per-sender
  seqno if v2 ever wants stable sorting — the hook exists for free.)
- **Rationale:** casual chat; any reorder window is sub-second at this scale; a
  reordering buffer adds latency and complexity with no user-visible payoff at 2–10
  peers.
- **Reversal cost:** low — v2 adds sorting keyed on the already-present pubsub seqno.

**T-07 — Connection manager watermarks + room-peer protection.** *(§13)*

- **PROPOSED:** `connmgr.NewConnManager(32, 96, connmgr.WithGracePeriod(1*time.Minute))`;
  every peer in the joined room gets `ConnManager().Protect(pid, "pchat-room")` on
  join and `Unprotect` on leave. Resource manager: left at go-libp2p's autoscaling
  defaults for the client (verified on-by-default: 128 conns / 128 MiB base, scaling
  up); explicitly raised on the VPS (WP-07).
- **Rationale:** libp2p's own fallback watermarks (160/192) are sized for DHT server
  nodes; a chat client needs room peers + DHT ambient churn only. Protection ensures
  trimming can only ever evict discovery churn, never a room member — the §13 "hostile
  peer exhausts your fds" scenario gets bounded at 96 conns with the room intact.
  Keep high watermark < rcmgr's 128-conn default so the connmgr, with its polite
  eviction, acts before the rcmgr's hard refusals.
- **Reversal cost:** trivial (constants).

**T-08 — Gossipsub at 2–3 peers: default mesh params + flood-publish + mandatory small-room tests.** *(§13)*

- **PROPOSED:** keep `DefaultGossipSubParams` untouched (D=6/Dlo=5/Dhi=12 verified);
  add `pubsub.WithFloodPublish(true)`; WP-04's acceptance criteria include in-process
  2-peer and 3-peer convergence tests. No hand-tuning of D/Dlo/Dhi in v1. Peer scoring
  stays off (it is off by default — verified; if ever enabled, small rooms need
  `MeshMessageDeliveriesWeight=0` — recorded here so nobody trips it later).
- **Rationale (verified in source, resolving the §13 "don't assume" instruction):**
  below Dlo the mesh is simply *all* topic peers — gossipsub degenerates gracefully to
  a full clique; the historical small-mesh heartbeat panic (#634) is fixed before
  v0.16.0. The one real gap: flood-publish is **off** by default, leaving a ~1 s
  window where a freshly-subscribed peer misses publishes — closed by enabling it,
  at negligible cost in rooms this small.
- **Reversal cost:** trivial (one option), unless tests reveal deeper issues — which
  is exactly what the mandatory tests are for.

**T-09 — DHT: piggyback the public IPFS Amino DHT, `ModeClient`.** *(§3)*

- **PROPOSED:** `dht.New(ctx, h, dht.Mode(dht.ModeClient),
  dht.BootstrapPeers(dht.GetDefaultBootstrapPeerAddrInfos()...))` — default `/ipfs`
  protocol prefix (Amino), public bootstrappers. Add `dht.AddressFilter` stripping
  private/LAN addrs from what we advertise (hygiene: NAT'd clients otherwise publish
  their RFC1918 addrs; Kubo servers strip them anyway). Do **not** run our own DHT
  network. Advertise is gated on reachability (WP-05 detail): wait until `host.Addrs()`
  contains a confirmed public or relay address before the first `Advertise`, else the
  provider record is undialable garbage (verified failure mode).
- **Rationale:** a custom `ProtocolPrefix` DHT whose only server nodes are our VPS *is*
  a centralized rendezvous with Kademlia overhead — it dies with the VPS, defeating
  the §3 point of DHT as the resilient fallback. Amino gives thousands of independent
  always-on nodes for zero ops. The privacy cost (room-key provider records visible on
  the public DHT) is exactly the §13 IP-exposure limitation, already accepted and
  disclosed (P-03) — with two *sharpened* facts for the threat model: records linger
  up to **48 h** after leaving (nothing un-provides), and the ~20 DHT servers closest
  to the key see participant IPs without knowing any passphrase.
- **Reversal cost:** moderate — own-DHT later means running bootstrap infrastructure
  and a coordinated client migration; nothing in the code shape blocks it (the
  discovery source is behind an interface).

**T-10 — Discovery orchestration: run all sources concurrently with priority labels, not serial fallthrough.** *(§3's "falling through automatically", reinterpreted)*

- **PROPOSED:** `internal/p2p/discovery.go` defines
  `type Source interface { Name() string; Run(ctx, ns string, found chan<- peer.AddrInfo) }`
  and a manager that runs **rendezvous, DHT, and mDNS concurrently** from the start,
  deduplicates found peers, dials, and reports per-source status to the UI. The §3
  "order" (rendezvous → DHT → mDNS) survives as *status-line priority* (the UI shows
  the healthiest/fastest source), not as sequential gating.
- **Rationale:** mDNS is free and instant; serially waiting for a rendezvous timeout
  before even starting mDNS punishes the LAN demo case for no benefit. DHT bootstrap
  takes 30 s–2 min cold (measured expectations from live research) — starting it
  lazily after a rendezvous failure would double worst-case join time. Concurrency
  costs nothing here; "fallthrough" in the spec is about *resilience*, which
  concurrent sources deliver strictly better.
- **Reversal cost:** trivial — the manager can trivially serialize if concurrency
  ever causes a problem.

**T-11 — Rendezvous point address: compile-time default + env override; the VPS doubles as the static relay.**

- **PROPOSED:** the default rendezvous point multiaddr (with peer ID) is a compile-time
  constant injected via `-ldflags -X` at release; `PCHAT_RENDEZVOUS` env var overrides
  (power users / self-hosters); no new required CLI flag (§6 keep-simple). The same
  AddrInfo is passed to `libp2p.EnableAutoRelayWithStaticRelays(...)` so the VPS
  serves as the §3 relay fallback.
- **Rationale:** §6 fixes the flag surface; an env var covers self-hosting without
  flag creep. Baking the address in is what makes `pchat --room x` work with zero
  config; ldflags injection means the repo carries a dev default and releases carry
  the production VPS.
- **Reversal cost:** trivial.

**T-12 — Rendezvous dependency stance + pre-approved contingency.**

- **PROPOSED:** use the Waku fork (§3 as amended by CLAUDE.md), pinned by
  pseudo-version (`v0.0.0-20240110193335-a67d1cc760a0` — the module has no tags).
  Importing the **root package** is correct and cgo-free (the CLAUDE.md claim that it
  pulls sqlite was verified WRONG and is corrected in this plan's commit); the sqlite
  subpackage is simply never imported. **Contingency, pre-approved so no session
  stalls:** if WP-06's opening spike finds the fork unusable in practice, v1 ships
  with DHT+mDNS discovery only, the VPS remains relay-only, and this entry flips to
  REVISED with a note — no maintainer round-trip required mid-session.
- **Rationale:** the fork compiled and ran end-to-end against go-libp2p v0.48.0 with
  `CGO_ENABLED=0` during research (Register/Discover/adapter, real TCP), so the risk
  is low — but its last real commit is 2024-01 and it has no tags, so the escape
  hatch is written down *now*.
- **Reversal cost:** the contingency **is** the reversal; losing rendezvous costs
  join latency on WAN (DHT is 30 s–2 min vs rendezvous ~instant) but no capability.

**T-13 — mDNS namespace: fixed service name `"pchat"`, never room-derived.** *(§3, privacy)*

- **PROPOSED:** `mdns.NewMdnsService(host, "pchat", notifee)` — one shared LAN
  namespace for all pchat instances. Room separation happens at the gossipsub topic
  layer (connecting to a co-located peer in a different room is harmless — no topic
  in common). Never derive the service name from the passphrase/topic.
- **Rationale:** mDNS broadcasts cleartext on the LAN; embedding a topic hash would
  advertise "someone here is in room H" to every LAN observer — an offline
  brute-force target the §13 threat model doesn't need. (Verified: the service name
  is used verbatim as the DNS-SD service type in register+browse; empty string would
  discover *any* libp2p node — also wrong for us.) The cost — discovering pchat peers
  from other rooms — is one wasted TCP connection each; the LAN presence signal
  "someone runs pchat" remains and is noted in the threat model (WP-13).
- **Reversal cost:** trivial.

**T-14 — Passphrase intake: argv stays but is no longer the only path.** *(§13 argv exposure; §6)*

- **PROPOSED:** resolution order: `--room <phrase>` (kept — scripting/demos; `--help`
  carries a process-list-visibility warning) → `--room -` reads the passphrase from
  the terminal with echo off (`golang.org/x/term.ReadPassword`) → `PCHAT_ROOM` env
  var → no room given at all ⇒ **commons room** (P-01; if P-01 is vetoed, this
  becomes "interactive prompt"). Normalization: `strings.TrimSpace` only; passphrases
  are otherwise byte-exact, README documents "prefer ASCII" (Unicode NFC
  normalization considered and deliberately deferred — an x/text dependency for an
  edge case; revisit on real-world reports).
- **Reversal cost:** trivial; adding NFC later is backward-incompatible only for
  passphrases containing combining characters (assessed: acceptable).

**T-15 — Graceful shutdown: intercept everything interceptable, best-effort leave.** *(§11)*

- **PROPOSED:** Ctrl+C arrives as an ordinary key event under Bubbletea's raw mode
  (verified — it does **not** auto-quit): handle it in `Update` → publish
  `presence{left}` with a 500 ms timeout → `tea.Quit`. External SIGINT/SIGTERM
  short-circuit Bubbletea's event loop *without* reaching `Update` (verified), so the
  program is constructed with `tea.WithFilter(...)` translating Quit/Interrupt into a
  leave-first sequence. The leave broadcast must NOT be routed through
  `Program.Send()` during teardown (verified: silent no-op after ctx cancel).
  SIGKILL/crash is not interceptable; peers must treat silent departure as normal
  (T-16 covers the UX).
- **Reversal cost:** none meaningful.

**T-16 — Connection churn UX: "left" vs "lost connection" are distinct states.** *(§11)*

- **PROPOSED:** roster removal is driven by pubsub `PeerLeave` events. If a
  `presence{left}` message preceded the event (within ~10 s), render "◆ left"; if
  not, render "◆ lost connection". No app-level heartbeats in v1 (pubsub/transport
  keepalives detect death; latency of detection is acceptable and honest).
- **Reversal cost:** low; heartbeats can be added as a new message type (reserved
  type space) without a version bump.

**T-17 — Roster source: pubsub topic peer events, reconciled; presence messages are semantics-only.**

- **PROPOSED:** the roster's *existence* source is `Topic.EventHandler()`
  PeerJoin/PeerLeave plus periodic `Topic.ListPeers()` reconciliation (every 15 s) —
  peer ID alone yields the glyph (T-01), so no message exchange is needed to render a
  peer. `presence` messages (joined/left) drive *narrative* lines and the T-16
  distinction, not roster membership. Typing indicators auto-expire client-side after
  4 s.
- **Rationale (verified):** PeerJoin fires on a directly-connected peer's subscription
  announce; handlers are pre-seeded with current peers; edge cases (fast join+leave
  coalescing, transitively-connected peers being invisible) are reconciled by the
  ListPeers sweep. In rooms where everyone is discovered via rendezvous/mDNS/relay,
  all peers are directly connected — the transitive case is rare and self-heals.
- **Reversal cost:** low.

**T-18 — Send-state feedback: derived from topic peers + publish errors; no auto-republish.** *(§5)*

- **PROPOSED:** three states — **pending** (rendered immediately from self-delivery;
  stays pending while `len(Topic.ListPeers()) == 0`, shown as "no peers yet — message
  not delivered"), **sent** (published while ≥1 topic peer; wording in UI/docs is
  "handed to the mesh", never "delivered" — there are no per-message acks in
  gossipsub, verified), **failed** (a real `Publish` error). Messages published into
  an empty room are *not* automatically republished when someone joins (surprising +
  stale `ts` would trip T-05); the pending marker is the honest truth. First publish
  after join may use `WithReadiness(pubsub.MinTopicSize(1))` with a 3 s context as a
  politeness gate (verified working without discovery wiring despite a stale doc
  comment upstream).
- **Rationale:** `Publish` returns `nil` with zero peers (verified — silent success),
  so return-value-driven UX would lie. This design states exactly what is knowable.
- **Reversal cost:** low; v2 could add app-level acks as a new message type.

**T-19 — Version/decode mismatch UX: rate-limited per-peer notice.** *(§11)*

- **PROPOSED:** messages failing envelope decode or carrying `v != 1` increment a
  per-peer counter; the first occurrence per peer per session renders one system line
  ("⟡'s messages use an incompatible protocol version — hidden"), further ones are
  silent. Unknown `type` values within `v == 1` are silently ignored (that is the §4
  reserved-type mechanism working as intended, not a mismatch).
- **Reversal cost:** none.

**T-20 — One room per process.** *(§11)*

- **PROPOSED:** confirmed as specced. The UI keeps the pluggable message-renderer
  seam and `Room` as a self-contained value so v2 tabs are additive.
- **Reversal cost:** v2 feature work, not rework — the p2p layer already isolates all
  room state in one struct.

**T-21 — Testing strategy.** *(§11)*

- **PROPOSED:** (a) unit tests per package (identity distribution/determinism,
  protocol round-trip + guard enforcement); (b) **Go fuzz tests** on
  `protocol.Decode` (corpus: valid envelopes, depth bombs, dup keys, truncations);
  (c) integration tests = **in-process multi-host libp2p** networks (2 and 3 hosts,
  direct `host.Connect`, discovery disabled) asserting room convergence, forged-`From`
  rejection, oversize/stale drops — no mocked libp2p interfaces, ever; (d) `-race` on
  all CI test runs; (e) the manual cross-platform matrix is WP-14, with results
  recorded in-repo. Rationale: mocks would test our assumptions about libp2p rather
  than libp2p; in-process hosts are fast enough (<5 s per test, verified in research
  scratch programs).
- **Reversal cost:** n/a (process decision).

**T-22 — Terminal compatibility strategy.** *(§11, §16-Windows)*

- **PROPOSED:** conservative curated glyph set (T-30) drawn from widely-covered
  Unicode blocks; detect the terminal's color capability at startup (Bubbletea v2
  delivers `tea.ColorProfileMsg` — verified) and show a one-line warning when
  < 256 colors ("colors approximate on this terminal — glyph shapes still
  distinguish peers"; on 16-color terminals distinct 256-color identities *do*
  collapse, verified in the degradation path). `NO_COLOR` is respected automatically
  by the renderer. Windows: minimum supported = Windows 10 1511+ (Bubbletea requires
  VT — verified it hard-errors otherwise); Windows Terminal is the recommended host
  in the README; conhost (cmd.exe *and* PowerShell — same console host) gets a manual
  QA pass in WP-14.
- **Reversal cost:** n/a (strategy).

**T-23 — Mouse capture OFF; alt-screen ON.** *(§5's explicit "decide on purpose")*

- **PROPOSED:** no mouse capture (native terminal text selection/copy is sacred for
  this audience — capture ON breaks it, verified upstream issue #162); alt-screen ON
  (the viewport *is* the scrollback; leaving the alt screen wipes the chat from the
  visible terminal — which is pchat's ephemerality made visceral). Scrolling:
  PgUp/PgDn (+ wheel where terminals translate it to arrows in alt-screen, which most
  modern ones do). A `--mouse` flag is explicitly deferred to v1.1+.
- **Reversal cost:** trivial (in Bubbletea v2, mouse mode is a `View` field —
  runtime-togglable).

**T-24 — Narrow-terminal behavior.** *(§5's "pick one on purpose")*

- **PROPOSED:** roster sidebar hidden below **70 columns** (its glyphs then appear
  only in message prefixes); hard floor **40×8** — below it, a centered "terminal too
  narrow" notice replaces the view (input stays alive). Reflow on every
  `WindowSizeMsg` (guaranteed initially and on resize on all platforms — verified,
  including the Windows event path).
- **Reversal cost:** trivial.

**T-25 — Resolve §6 `--debug` vs §15 "no logging code path in prod".**

- **PROPOSED:** prod keeps `--debug`, implemented as **framework-free, generic,
  fmt-based one-line events to stderr** (discovery phase changes, connect/disconnect
  counts, publish errors — no stack traces, no peer internals, no logging library
  linked). The dev build tag adds the real instrumentation: `log/slog` structured
  logging, localhost-only metrics + pprof, debug commands (§15 list). §15's "no
  logging code path" is thereby interpreted as "no logging *framework/instrumentation*
  path"; the spirit (nothing to leak, nothing listening, no disk) is kept.
- **Rationale:** shipping v1 to strangers with *zero* diagnostic capability in the
  prod binary would make every launch-day "can't connect" report undebuggable —
  a support disaster the launch checklist implicitly depends on avoiding. This is a
  deliberate, veto-able softening of §15's letter.
- **Reversal cost:** trivial (delete the prod `--debug` branch) — but do it before
  launch or not at all; removing a flag after launch breaks scripts.

**T-26 — Prod-purity CI check: build-info tags + canary string, NOT symbol tables.** *(§15's CI verification requirement)*

- **PROPOSED:** `ci/check_prod.sh` asserts, for every release binary: (1)
  `go version -m` shows no `-tags=dev` build setting; (2) `strings` finds no
  `PCHAT_DEV_BUILD_CANARY` (a unique constant *used* — not just declared — inside
  dev-only code); plus (3) a self-test that builds a dev binary and asserts both
  detectors fire. Runs in the release workflow between build and publish, and in CI.
- **Rationale (verified empirically — this corrects §15's own suggestion):** the
  spec's "grep the binary for known dev-only strings/symbols" idea half-works;
  symbol-based checks (`go tool nm`) are provably unreliable — stripped (`-s -w`)
  release binaries have *no symbol section* (nm errors → vacuous pass), and even
  unstripped dev binaries lose small dev functions to inlining. Embedded build info
  survives stripping on all five target platforms (verified against real goreleaser
  output: 5 prod binaries accepted, dev binary rejected, self-test green).
- **Reversal cost:** n/a.

**T-27 — Release signing: cosign keyless (Sigstore bundle) primary; GitHub attestations secondary; sha256 floor.** *(§13 supply chain)*

- **PROPOSED:** goreleaser `signs` block runs cosign v3 keyless over `checksums.txt`
  producing `checksums.txt.sigstore.json` (v3 bundle format — the `.sig`+`.pem` pair
  is legacy, verified); README documents the two-command verification
  (`cosign verify-blob --certificate-identity …/release.yml@refs/tags/v… --bundle …`
  then `sha256sum -c`). Secondary: `actions/attest-build-provenance@v4` with
  `subject-checksums` (one workflow step, SLSA provenance). Floor: plain
  `sha256sum -c` documented for users who install nothing. Minisign rejected:
  permanent solo-maintainer key custody (loss/compromise/no-revocation) for a worse
  tooling story. Note honestly in docs: `gh attestation verify` requires GitHub auth
  even for public repos (verified) — hence it is not the primary for this audience;
  and keyless's trust anchor is "GitHub ran this repo's release workflow", stated
  plainly.
- **Reversal cost:** low — mechanisms are additive; switching primary is a docs
  change.

**T-28 — Repo layout additions beyond §8's fixed tree.**

- **PROPOSED:** §8's tree is kept verbatim (no renames) and extended additively:
  `cmd/pchat-point/` (VPS daemon main), `internal/point/` (in-memory rendezvous
  store + daemon config), `internal/devtools/` (build-tagged instrumentation, T-25),
  `internal/p2p/discovery_dht.go` + `discovery_rendezvous.go` (sources beside the
  §8-named `discovery.go`), `internal/ui/commands.go` (WP-10), `deploy/`
  (systemd unit + provisioning doc), `ci/check_prod.sh`, `.goreleaser.yaml`,
  `.github/workflows/release.yml`, `docs/threat-model.md`, `docs/commons-room.md`,
  `docs/testing-matrix.md`.
- **Rationale:** §8 describes the *client*; the VPS component (§3) and release/dev
  tooling (§13/§15) have no home in it. Additive-only keeps every §8 path valid.
- **Reversal cost:** n/a.

**T-29 — pchat-point service parameters (rendezvous store, relay limits, stable identity).**

- **PROPOSED:**
  - **Store:** hand-written in-memory implementation of the fork's `dbi.DB` interface
    (6 methods; cookie = 8-byte BE counter ‖ SHA-256(nonce‖ns‖counter), per the sqlite
    reference impl; random per-boot nonce). Sqlite `:memory:` rejected: cgo for
    nothing. Registrations die with the process — clients self-heal via their refresh
    loop (verified: `RendezvousClient.Register` auto-re-registers at ttl−30 s).
  - **Client TTL:** 300 s (client-side minimum is 120 s — verified; server default
    2 h is far too sticky for "you exist only while you're in the room").
    `Unregister` on graceful leave.
  - **Relay limits:** the library defaults (2 min / 128 KiB per circuit — verified)
    would reset any relay-fallback chat session; the point runs
    `relay.WithLimit(&relay.RelayLimit{Duration: 6h, Data: 256 MiB})`, plus
    `WithResources` raising `MaxReservations` to 512. Bounded on purpose — not
    `WithInfiniteLimits` — because the VPS is a public, unauthenticated relay.
  - **Identity:** the point needs a **stable peer ID** (it is baked into the client's
    default multiaddr, T-11), so it accepts `PCHAT_POINT_KEY` (base64 Ed25519 key) /
    `--key-file`; generates ephemeral + prints if absent (dev mode). This is
    infrastructure identity, not user data — it does not violate the §3 no-storage
    rule, which is about *registrations*; the deploy doc states this distinction.
- **Reversal cost:** all constants/config; trivial.

**T-30 — Identity glyph/color sets and the collision rule.** *(§2, §5)*

- **PROPOSED:** derivation `h = SHA-256(pubkey)`; glyph = `glyphs[h[0] % 48]`,
  color = `palette[h[1] % 8]` → 384 combinations. **Glyph candidates (48):** drawn
  from Geometric Shapes / Misc Symbols with wide font coverage —
  `■ □ ▲ △ ▼ ▽ ◆ ◇ ● ○ ◐ ◑ ★ ☆ ♠ ♣ ♥ ♦ ♪ ♭ ☘ ☾ ☉ ☍ ♆ ♞ ♜ ⚑ ⚙ ⚛ ✚ ✦ ✧ ✪ ✿ ❖ Ω Δ Σ Φ Ψ π λ ξ Ж Ѫ ѯ Ԃ` —
  WP-01 locks the final set after the font pass (acceptance criterion: every glyph
  renders in Menlo, Consolas, JetBrains Mono, Cascadia Mono; anything showing tofu is
  swapped out). **Color palette (8):** Okabe–Ito colorblind-safe set mapped to
  256-color indices — `208` (orange), `39` (sky blue), `35` (bluish green), `221`
  (yellow), `27` (blue), `166` (vermillion), `176` (reddish purple), `245` (grey) —
  WP-01 verifies pairwise distinguishability under deuteranopia/protanopia simulation
  and on light + dark backgrounds. **Collision rule:** if two *live* roster peers
  share glyph+color, both get a 4-char base32 suffix of `SHA-256(pubkey)[2:5]`
  appended (`◆a3xf`); expected collision odds ~11 % in a 10-peer room — acceptable
  with the disambiguation rule. Color is never the sole differentiator (glyphs are
  distinct shapes by construction, §5).
- **Reversal cost:** changing sets after launch re-rolls everyone's identity
  *appearance* between sessions — which pchat users are told to expect anyway
  (identities are per-session); still, lock sets at first release and treat as
  append-only.

**T-31 — Adopt the Charm v2 stack (supersedes CLAUDE.md's v1 pins).**

- **PROPOSED:** `charm.land/bubbletea/v2 v2.0.8`, `charm.land/bubbles/v2 v2.1.1`,
  `charm.land/lipgloss/v2 v2.0.5`. **The v2 modules live at `charm.land/*` vanity
  paths** — the `github.com/charmbracelet/*/v2` paths are a trap (verified: v2.0.8
  refuses the github path; mixing paths compiles two incompatible bubbleteas).
- **Rationale:** CLAUDE.md pinned bubbletea v1.3.10/lipgloss v1.1.0 (verified: still
  the latest v1 tags — but the v1 line is frozen since 2025-09/2025-03; v2 went
  stable 2026-02-24 and gets all fixes). Concrete v2 wins for pchat: viewport with
  native `SoftWrap` (v1 viewport cannot wrap chat lines at all — we'd hand-roll
  wrapping), `tea.ColorProfileMsg` for T-22's degradation warning, runtime-togglable
  view properties. v2.0.8 > the v2.0.7 floor that fixes the Windows-resize
  regression (#1601). Launching a new 2026 tool on a frozen UI line to honor a
  6-month-old version table would be planning malpractice; the table is corrected
  instead.
- **Reversal cost:** falling back to the v1 set mid-build is a mechanical but
  wide-reaching UI rewrite (message/key/view API differences) plus hand-rolled
  wrapping — decide before WP-08 starts, not after.

**T-32 — go-libp2p-pubsub stays at v0.16.0 (not v0.17.0).**

- **PROPOSED:** pin v0.16.0. v0.17.0 was released **on the day of this research**
  (2026-07-09) and swaps the internal protobuf library — zero soak time. v0.16.0 is
  proven against go-libp2p v0.48.0 (compiled + ran in research). Revisit at v1.1.
- **Reversal cost:** trivial version bump later.

**T-33 — Topic/namespace string carries a protocol version prefix.**

- **PROPOSED:** the gossipsub topic and the rendezvous/DHT namespace are both
  `"pchat/1/" + hex(SHA-256(TrimSpace(passphrase)))` — not the bare hash. §3's
  "topic_id = SHA-256(passphrase)" is preserved as the *secret* component; the prefix
  only namespaces the protocol generation.
- **Rationale:** lets a future wire-incompatible v2 coexist on the same
  infrastructure without cross-version message noise (T-19 then only handles
  *same-generation* skew), and makes pchat traffic self-describing in our own
  rendezvous store. No secrecy loss — the hash is unchanged.
- **Reversal cost:** none before launch; a wire break after.

### Product decisions

---

**P-01 — Commons room: yes; `pchat` with no room joins it.** *(§12 cold start, §14)*

- **PROPOSED:** running `pchat` with no `--room`/`PCHAT_ROOM` joins the **commons** —
  the well-known passphrase is the literal string **`pchat-commons`** (named in the
  README per the launch checklist). The UI shows a persistent banner line: "the
  commons — public room, be kind: see /help" linking the AUP (P-04). Private rooms
  remain the headline feature; the commons is the front door.
- **Rationale:** §12 is explicit that without this the product has no cold start; the
  launch checklist assumes it ("seed first", "commons-room passphrase called out").
- **Veto consequence:** T-14's no-room path becomes an interactive prompt; README and
  launch plan lose the live-demo mechanism; §12's cold-start paradox ships unsolved.

**P-02 — Local mute ships in v1; the abuse stance is stated, not implied.** *(§12)*

- **PROPOSED:** `/who` lists roster peers with stable per-session indices; `/mute <n>`
  (and `/unmute <n>`) drops rendering of that peer's current pubkey for the rest of
  the session (messages + typing; roster shows the peer greyed with a ⊘ mark).
  Nothing persists. The README/threat model states the stance plainly: anonymity
  means **no bans and no server-side recourse exist or can exist**; mute is local and
  dies with the session; reconnecting attackers get fresh identities. This is the
  §12 "minimum viable safety valve", chosen deliberately.
- **Veto consequence:** v1 ships with zero abuse handling and the launch checklist
  item "basic abuse handling in place" fails — vetoing this means accepting that in
  writing.

**P-03 — Positioning: "drop-in ephemeral hangout", and the IP caveat is front-matter, not a footnote.** *(§14, §13)*

- **PROPOSED:** README hook and all launch copy position pchat as an ephemeral
  drop-in space — explicitly *not* a daily-driver ("nothing persists: not the chat,
  not you"). The DHT/IP-exposure caveat appears in the README's usage section
  (immediately at the point of telling users about passphrases), in concrete
  user-language ("anyone who guesses your passphrase can see participants' IP
  addresses — and can keep seeing them for up to ~48 h after you leave"), linking the
  full threat model. First-user trust-cliff logic from §14: being told upfront is a
  feature; discovering it later is a scandal.
- **Veto consequence:** softer copy; accept the §14 trust-cliff risk explicitly.

**P-04 — Commons acceptable-use note.** *(§16)*

- **PROPOSED:** `docs/commons-room.md`: ~10 lines, plain language — no harassment, no
  illegal content, operator may shut the commons down at any time, no logs exist so
  enforcement is social not technical. Linked from the commons banner and README.
- **Veto consequence:** operator has nothing to point to when (not if) the commons
  gets questioned — §16 explicitly warns against this.

**P-05 — Operator legal exposure: external action item, launch gate.** *(§16)*

- **PROPOSED:** the plan cannot close this. Recorded as a **maintainer action item
  gating WP-14/launch**: consult a lawyer (relay/common-carrier exposure in the
  operator's jurisdiction) before any public launch. The engineering posture that
  conversation needs (relay = encrypted transit only, no logs by construction,
  in-memory registrations, AUP) is what WP-07 + WP-13 build.
- **Veto consequence:** launching without the consult is the maintainer's explicit,
  recorded risk acceptance — flip this entry to VETOED and WP-14 loses the gate.

**P-06 — License: MIT (already committed in the scaffold).** *(§16)*

- **PROPOSED (ratify the fait accompli):** keep MIT — the scaffold already ships
  `LICENSE` (MIT © 2026 Mediacom99), which §16 left open. MIT maximizes what §16
  cares about: anyone can fork and run independent rendezvous points.
- **Veto consequence:** swap `LICENSE` before first release (after release it's a
  relicensing mess).

**P-07 — In-app discoverability is v1 scope.** *(§14)*

- **PROPOSED:** input placeholder reads `type a message — /help for commands`;
  `/help` exists from WP-10 with the full v1 command surface. Not deferred as polish.
- **Veto consequence:** §14's "bare input bar" churn risk accepted.

**P-08 — Sybil: unmitigated in v1, tracked and disclosed; PoW hook is structural.** *(§13, §9)*

- **PROPOSED:** no PoW/rate limiting in v1 (per §9's deferral) — but (a) the threat
  model states plainly that one person can appear as N peers at zero cost, and (b)
  the validator chain in WP-04 is explicitly ordered so a PoW validator slots in
  front of the decode step in v2 without wire changes. A per-peer message rate cap is
  noted as the v1.1 candidate (§7 already flags it).
- **Veto consequence (i.e. demanding v1 mitigation):** adds a package; flag it and
  the plan grows a WP.

---

## 6. Work packages

Conventions for every package: one PR; `make build`, `make test`, `make lint`,
`go vet ./...` green at merge; new deps only from §3 at the moment of first import;
client runtime writes nothing to disk, ever. "Spec:" cites the architecture sections
that motivate the package — read them before implementing.

---

### WP-01 — `internal/identity`: glyph+color identity

**Status:** TODO · **Depends on:** — · **Gated by:** T-30 · **Spec:** §2, §5
(accessibility), §10 step 1.

**Goal:** pure, dependency-free derivation of the per-session visual identity from an
Ed25519 public key, plus session keypair generation. No networking, no libp2p import.

**Files:** `internal/identity/identity.go` (replace placeholder),
`internal/identity/glyphs.go` (curated sets as data), `internal/identity/identity_test.go`.

**Key API (signature level):**

```go
package identity

// Identity is the session-visible form of a peer: derived deterministically
// from the Ed25519 public key (§2). Pure value type.
type Identity struct {
    PubKey [32]byte
    Glyph  rune  // from the curated 48-glyph set
    Color  uint8 // 256-color index from the curated 8-color palette
}

// GenerateKeypair returns a fresh Ed25519 keypair (stdlib crypto/ed25519).
// Never persisted anywhere (§1, §2). The p2p layer converts priv into a libp2p
// key via crypto.UnmarshalEd25519PrivateKey (WP-03).
func GenerateKeypair() (ed25519.PublicKey, ed25519.PrivateKey, error)

// FromPubKey derives the Identity for any 32-byte Ed25519 public key
// (ours or a remote peer's — remote keys come from peer IDs, see WP-04).
// Errors iff len(pub) != 32.
func FromPubKey(pub []byte) (Identity, error)

// Short returns the 4-char base32 disambiguation suffix used when two live
// peers collide on glyph+color (T-30), e.g. "a3xf".
func (id Identity) Short() string
```

Derivation (T-30): `h = sha256(pub)`; `Glyph = glyphSet[h[0] % 48]`;
`Color = palette[h[1] % 8]`; `Short = base32(h[2:5])[:4]` lowercased. Rendering
(color codes, styling) is **not** this package's job — UI renders; identity stays
pure so tests need no terminal.

**Implementation notes:**
- Start from T-30's candidate glyph and palette sets; the font pass may swap members
  (record swaps in the Learnings log). Sets are package-level `var` slices with a
  comment marking them **append-only after first release**.
- No global state, no randomness outside `GenerateKeypair`.

**Acceptance criteria:**
1. Determinism: same pubkey → same Identity, across runs and platforms.
2. Distribution: over 100 k random keys, each glyph within 48±15 % of uniform, each
   color within 8±10 % (chi-squared sanity, not crypto claims).
3. Font pass done and recorded: every final glyph renders (no tofu, no
   double-width surprises — must be terminal-cell-width 1) in Menlo, Consolas,
   JetBrains Mono, Cascadia Mono. Record the check in the PR description with a
   screenshot per font.
4. Palette pass: pairwise distinct under deuteranopia + protanopia simulation and on
   light + dark backgrounds (record tool used).
5. `Short()` is stable and 4 chars for all keys.

**Test plan:** table-driven derivation tests with fixed vectors (lock the mapping —
any later change to sets breaks these tests *on purpose*); distribution test (seeded,
deterministic); fuzz `FromPubKey` (must never panic on any input length).

---

### WP-02 — `internal/protocol`: wire envelope + guards

**Status:** TODO · **Depends on:** — · **Gated by:** T-01 T-02 T-03 T-04 T-05 T-33 ·
**Spec:** §4, §13 (CBOR guards, replay), §10 step 2.

**Goal:** the complete wire format: envelope encode/decode with hardened CBOR modes,
typed payloads, size caps, freshness/dedupe guard. Deletes `sign.go` (T-01). No
libp2p imports — the package is keyed on opaque strings/bytes so it stays pure.

**Files:** `internal/protocol/message.go` (replace placeholder),
`internal/protocol/replay.go`, delete `internal/protocol/sign.go`,
`internal/protocol/message_test.go`, `internal/protocol/fuzz_test.go`.

**Dependency added:** `github.com/fxamacker/cbor/v2 v2.9.2`.

**Key API:**

```go
package protocol

const (
    Version uint8 = 1

    TypeChat     uint8 = 0
    TypePresence uint8 = 1
    TypeTyping   uint8 = 2
    // 3, 4 reserved for v2 widgets / file chunks (§4). Unknown types are
    // ignored by receivers (T-19), which is what makes them reservable.

    MaxWireBytes     = 4608 // whole encoded envelope; checked BEFORE decode and publish
    MaxPayloadBytes  = 4096 // §11's 4KB cap on Envelope.Payload
    MaxChatTextBytes = 4000 // UTF-8 bytes of Chat.Text

    FreshnessWindow = 10 * time.Minute // T-05
    DedupeTTL       = 20 * time.Minute // T-05: 2× window
)

// Envelope is the §4 envelope minus sender/sig (decision T-01: gossipsub
// StrictSign carries and verifies sender identity).
type Envelope struct {
    V       uint8  `cbor:"1,keyasint"`
    Type    uint8  `cbor:"2,keyasint"`
    TS      uint64 `cbor:"3,keyasint"` // unix millis, sender clock, informational (§4)
    Payload []byte `cbor:"4,keyasint"` // type-specific CBOR body
}

type Chat struct     { Text string `cbor:"1,keyasint"` }
type Presence struct { State uint8 `cbor:"1,keyasint"` } // 0=joined 1=left
type Typing struct   { State uint8 `cbor:"1,keyasint"` } // 0=started 1=stopped

func Encode(e Envelope) ([]byte, error)          // enforces MaxPayloadBytes + MaxWireBytes
func Decode(data []byte) (Envelope, error)       // enforces MaxWireBytes pre-decode, guards, version
func EncodeChat(text string, now time.Time) ([]byte, error)   // convenience: payload+envelope
func EncodePresence(state uint8, now time.Time) ([]byte, error)
func EncodeTyping(state uint8, now time.Time) ([]byte, error)
func DecodeChat(payload []byte) (Chat, error)    // per-type payload decoders, same DecMode
func DecodePresence(payload []byte) (Presence, error)
func DecodeTyping(payload []byte) (Typing, error)

// Errors the p2p validator switches on:
var (
    ErrTooLarge      = errors.New(...)
    ErrBadVersion    = errors.New(...) // e.V != Version → T-19 UX path
    ErrUnknownType   = errors.New(...) // within v1: silently ignorable
)

// ReplayGuard implements T-05. Key = pubsub message ID (from+seqno), supplied
// by the p2p layer; this package never imports libp2p.
type Verdict int
const (VerdictFresh Verdict = iota; VerdictStale; VerdictDuplicate)

func NewReplayGuard(window, ttl time.Duration, now func() time.Time) *ReplayGuard
func (g *ReplayGuard) Check(msgID string, ts uint64) Verdict // thread-safe; self-GCs
```

**Implementation notes:**
- Package-level singletons: `decMode` from T-03's hardened options; `encMode` from
  `cbor.CoreDetEncOptions()` (both verified concurrency-safe; build in `init` and
  panic on error — misconfiguration is a programmer bug).
- Do NOT set `ExtraDecErrorUnknownField` (T-02 — forward compatibility is the point).
- `Decode` order: length check → CBOR decode (guards fire here) → version check →
  payload length check. Unknown `Type` is *not* an error from `Decode` itself; the
  caller decides (validator ignores, T-19).
- `ReplayGuard`: map + monotonic sweep (sweep every `ttl/4` on access, no goroutine —
  keep the package inert). `now` injected for tests.

**Acceptance criteria:**
1. Round-trip for all three payload types; encoded chat envelope ("hello") ≤ 60 bytes.
2. Guards demonstrably fire: depth-9 nesting, 65-element array, dup map keys,
   indefinite-length items, tagged items, trailing bytes, 5000-byte payload — each
   rejected with a distinguishable error.
3. A v=2 envelope decodes as `ErrBadVersion` (not a CBOR error); an unknown key `9`
   in the map decodes cleanly (forward compat proven in a test).
4. `ReplayGuard`: fresh accepted; same ID again → Duplicate; ts 11 min old → Stale;
   entries expire after TTL (fake clock).
5. `sign.go` gone; package has zero non-stdlib deps besides cbor.

**Test plan:** unit + the guard matrix above; `go test -fuzz=FuzzDecode -fuzztime=60s`
locally (fuzz target committed; CI runs the corpus as regular tests); `-race` on the
ReplayGuard concurrency test.

---

### WP-03 — `internal/p2p` host

**Status:** TODO · **Depends on:** WP-01 · **Gated by:** T-07, T-25 (logging shim
only) · **Spec:** §3 (transports/NAT), §6 (`--relay-only`), §10 step 3.

**Goal:** a configured libp2p host: TCP+QUIC, Noise-only, watermarked connection
manager, AutoNAT(+v2), optional hole punching, static-relay client wiring, and the
identity bridge from WP-01. Plus the tiny build-tagged logging shim the rest of the
networking code will use (full dev tooling comes in WP-11).

**Files:** `internal/p2p/host.go` (replace placeholder),
`internal/p2p/host_test.go`, `internal/devtools/log_dev.go` (`//go:build dev`),
`internal/devtools/log_prod.go` (`//go:build !dev`).

**Dependency added:** `github.com/libp2p/go-libp2p v0.48.0`.

**Key API:**

```go
package p2p

type HostConfig struct {
    Priv        ed25519.PrivateKey // from identity.GenerateKeypair
    RelayOnly   bool               // §6 --relay-only
    Rendezvous  *peer.AddrInfo     // optional; doubles as static relay (T-11)
    ListenAddrs []string           // default: tcp+quic-v1 on 0.0.0.0 (v4+v6), port 0
}

// NewHost converts the raw key via crypto.UnmarshalEd25519PrivateKey (verify the
// exact seed/priv layout expected — first task of this WP) and assembles the host.
func NewHost(ctx context.Context, cfg HostConfig) (host.Host, error)
```

Assembly (all option names verified against v0.48.0 — see §3 matrix):

```go
libp2p.New(
    libp2p.Identity(priv),
    libp2p.ListenAddrStrings(cfg.ListenAddrs...),
    libp2p.Transport(tcp.NewTCPTransport),      // p2p/transport/tcp
    libp2p.Transport(libp2pquic.NewTransport),  // p2p/transport/quic
    libp2p.Security(noise.ID, noise.New),       // p2p/security/noise
    libp2p.ConnectionManager(cm),               // connmgr.NewConnManager(32, 96, WithGracePeriod(time.Minute))
    libp2p.EnableAutoNATv2(),                   // v1 client is always-on regardless (verified)
    libp2p.UserAgent("pchat/"+version),
    // conditionally:
    libp2p.EnableHolePunching(),                          // iff !cfg.RelayOnly
    libp2p.EnableAutoRelayWithStaticRelays([]peer.AddrInfo{*cfg.Rendezvous}), // iff Rendezvous != nil
    libp2p.ForceReachabilityPrivate(),                    // iff cfg.RelayOnly
)
```

**Implementation notes (verified gotchas):**
- Passing any explicit `Transport`/`Security` **replaces** the default set (drops
  WebSocket/WebTransport/WebRTC/TLS) — exactly what §3 wants; leave a comment saying
  this is deliberate.
- There is no `DisableHolePunching` — relay-only means *omitting* the enable option.
  `ForceReachabilityPrivate` does not prevent outbound direct dials; that is
  acceptable for a debug flag — document the semantics in the flag help ("forces
  relayed reachability; outbound direct connections may still occur").
- Relay *client* is enabled by default in libp2p; `EnableAutoRelayWithStaticRelays`
  is what makes the host reserve a slot and advertise `/p2p-circuit` addrs.
- Room peers get `Protect`ed in WP-04, not here — but expose the ConnManager.
- devtools shim: `devtools.Logf(format string, args ...any)` — dev build: slog to
  stderr; prod build: no-op with no fmt import. The `PCHAT_DEV_BUILD_CANARY` constant
  (T-26) is declared **and used** in `log_dev.go` from day one.

**Acceptance criteria:**
1. Host starts and reports an Ed25519 peer ID; `peer.ID → ExtractPublicKey → Raw()`
   round-trips to the identity pubkey (locks the T-01 identity bridge).
2. Two in-process hosts connect over TCP and over QUIC (force each transport by
   listen addr).
3. `RelayOnly` host reports private reachability (event bus
   `EvtLocalReachabilityChanged`) and, given a static relay, obtains a
   `/p2p-circuit` address.
4. Both build tag sets compile (`go build ./...` and `go build -tags dev ./...`).

**Test plan:** in-process host tests as above (no network beyond loopback); the
relay-only test may use a third in-process host running
`libp2p.EnableRelayService()` as the relay. `-race`.

---

### WP-04 — `internal/p2p` room: gossipsub + mDNS + validators

**Status:** TODO · **Depends on:** WP-02 WP-03 · **Gated by:** T-08 T-13 T-16 T-17 ·
**Spec:** §3 (rooms, gossipsub), §4 (verification duty), §13 (validator duties),
§10 step 4.

**Goal:** everything between a host and the UI: topic derivation, gossipsub join with
StrictSign + flood-publish, the validator chain, typed send helpers, the event stream
(chat/presence/typing/peer-join/leave), roster state, mDNS discovery, and the
discovery-source interface later packages plug into. After this package, two
processes on one LAN can chat (headless, via tests).

**Files:** `internal/p2p/room.go` (replace placeholder), `internal/p2p/discovery.go`
(replace placeholder: `Source` interface + manager + mDNS source),
`internal/p2p/room_test.go`, `internal/p2p/integration_test.go`.

**Dependency added:** `github.com/libp2p/go-libp2p-pubsub v0.16.0`.

**Key API:**

```go
package p2p

// TopicID implements T-33: "pchat/1/" + hex(sha256(TrimSpace(passphrase))).
// The passphrase itself never leaves this function (§3).
func TopicID(passphrase string) string

type Event any // one of:
type ChatEvent struct     { From identity.Identity; Self bool; Text string; TS time.Time }
type PresenceEvent struct { From identity.Identity; Joined bool } // explicit presence msg
type TypingEvent struct   { From identity.Identity; Started bool }
type PeerEvent struct     { Peer identity.Identity; Joined bool; ExplicitLeave bool } // roster (T-16/T-17)
type IncompatibleEvent struct { Peer identity.Identity } // first bad-version msg per peer (T-19)

type Room struct { /* topic, sub, event handler, replay guard, roster, mute set */ }

func JoinRoom(ctx context.Context, h host.Host, passphrase string) (*Room, error)
func (r *Room) Events() <-chan Event          // buffered; see drain note below
func (r *Room) SendChat(ctx context.Context, text string) error   // pre-publish caps (T-04)
func (r *Room) SendTyping(ctx context.Context, started bool) error
func (r *Room) Peers() []identity.Identity    // roster snapshot (ListPeers-reconciled)
func (r *Room) PeerCount() int                // for send-state UX (T-18)
func (r *Room) Mute(pub [32]byte, muted bool) // P-02 mechanism (UI command in WP-10)
func (r *Room) Leave(ctx context.Context) error // presence{left} best-effort → unsubscribe → Unprotect all

// Discovery plumbing (§3, T-10): sources feed found peers; the manager dials.
type Source interface {
    Name() string // "mdns" | "dht" | "rendezvous" — also the UI status label
    Run(ctx context.Context, ns string, found chan<- peer.AddrInfo) error
}
type SourceStatus struct { Name string; State string; Err error } // → UI status line
func NewDiscovery(h host.Host, sources ...Source) *Discovery
func (d *Discovery) Run(ctx context.Context, ns string) <-chan SourceStatus
func NewMDNSSource() Source // mdns.NewMdnsService(h, "pchat", notifee) — T-13; must call Start()
```

**Validator chain** (registered via
`RegisterTopicValidator(topic, validate, pubsub.WithValidatorInline(true))` —
inline because every check is cheap; order is load-shedding-first and doubles as the
§9 PoW insertion point, which goes *first* in v2):

1. wire size ≤ `MaxWireBytes` → `ValidationReject`
2. `protocol.Decode` (all T-03 guards fire inside) → `Reject` on malformed;
   `ErrBadVersion` → `Ignore` + emit `IncompatibleEvent` once per peer
3. unknown `Type` → `Ignore` (reserved-type mechanism)
4. `ReplayGuard.Check(msg.ID, env.TS)` → Stale/Duplicate ⇒ `Ignore` (never `Reject` —
   verified: Reject penalizes the *forwarder* under scoring; also protects
   skewed-clock users, T-05)
5. accept.

**Implementation notes (verified):**
- `pubsub.NewGossipSub(ctx, h, pubsub.WithFloodPublish(true))` — StrictSign is
  already the default; leave `WithMaxMessageSize` alone (T-04).
- Sender identity: `msg.GetFrom()` (author, not `ReceivedFrom` the forwarder) →
  `ExtractPublicKey()`; non-extractable peer IDs (RSA peers — cannot be a pchat
  client) → treat as incompatible, drop in validator. Own messages:
  `msg.GetFrom() == h.ID()` (the `Local` field means something else — verified trap).
- Self-delivery is real (verified: publisher's own subscription receives its
  messages even at zero peers) → `ChatEvent{Self: true}` comes from the same path as
  remote messages; the UI renders exactly one code path.
- The subscription channel buffer is 32 and **drops silently when full** (verified)
  → the room's read loop must be a tight goroutine that only moves messages into
  `Events()` (buffered 256; if *that* overflows, drop-oldest and count — dev builds
  log it).
- Roster: `Topic.EventHandler()` loop + 15 s `ListPeers()` reconciliation (T-17);
  `Protect(pid, "pchat-room")` on join, `Unprotect` on leave/lost (T-07).
  PeerLeave→`PeerEvent{Joined: false, ExplicitLeave: sawRecentLeftMsg}` (T-16).
- Presence: `presence{joined}` published once on join; `presence{left}` in `Leave`.
- mDNS notifee just hands `peer.AddrInfo` to the manager's dial queue; dedupe dials
  by peer ID with backoff.

**Acceptance criteria:**
1. **2-peer and 3-peer in-process convergence tests pass** (§13's explicit demand):
   all peers see all chat messages; join order permuted; a peer joining *after*
   messages were sent sees only new ones (no history — §1 honored by construction).
2. Impersonation test: a raw pubsub message hand-crafted with a forged `From` never
   reaches `Events()` (StrictSign drop — locks T-01's security claim).
3. Guard tests: oversize, stale-ts, duplicate, and v=2 messages published by a
   cooperating test peer are not delivered; v=2 yields exactly one
   `IncompatibleEvent`.
4. Roster: join/leave reflected on all peers; `Leave()` produces
   `ExplicitLeave: true` on remote peers, a killed connection produces `false`.
5. Two real processes on one machine discover each other via mDNS and exchange chat
   (manual check, recorded in PR).

**Test plan:** in-process multi-host tests per T-21 (direct `Connect`, mDNS off in
tests); a `-tags dev` run of the same suite (shim compiles both ways); `-race`
mandatory (this package is the concurrency hot spot).

---

### WP-05 — DHT discovery (Amino)

**Status:** TODO · **Depends on:** WP-04 · **Gated by:** T-09 T-10 · **Spec:** §3
(DHT fallback), §10 step 5, §13 (IP exposure — mechanics feed WP-13).

**Goal:** the DHT `Source`: advertise + find on the public Amino DHT, correctly gated
on reachability so we never publish undialable records.

**Files:** `internal/p2p/discovery_dht.go`, `internal/p2p/discovery_dht_test.go`.

**Dependency added:** `github.com/libp2p/go-libp2p-kad-dht v0.41.0`.

**Key API:**

```go
func NewDHTSource(h host.Host) (Source, error)
// internally:
//   kdht, _ := dht.New(ctx, h, dht.Mode(dht.ModeClient),
//       dht.BootstrapPeers(dht.GetDefaultBootstrapPeerAddrInfos()...),
//       dht.AddressFilter(stripPrivate))            // T-09 hygiene
//   rd := drouting.NewRoutingDiscovery(kdht)        // p2p/discovery/routing
//   dutil.Advertise(ctx, rd, ns)                    // p2p/discovery/util — self-renewing loop
//   ch, _ := rd.FindPeers(ctx, ns)                  // repeat with backoff
```

**Implementation notes (verified):**
- Bootstrap: `Connect` to the bootstrap peers, then wait on `RefreshRoutingTable()`
  (or poll `RoutingTable().Size() > 0`) before first use — `Bootstrap(ctx)` alone is
  non-blocking and querying an empty table errors.
- **Gate `Advertise`** until `host.Addrs()` contains a public or `/p2p-circuit`
  address (subscribe to `event.EvtLocalReachabilityChanged` / autorelay events);
  advertising instantly publishes private-LAN junk or errors with "no known
  addresses for self" (verified failure mode). Until then, run FindPeers only.
- Filter self out of FindPeers results (verified: you find yourself); entries with
  empty addrs → `kdht.FindPeer(ctx, id)` fallback.
- Expectation-setting for the status line: cold-start to discovery is
  ~30 s–2 min (measured ecosystem numbers) — the source reports
  `SourceStatus{State: "bootstrapping…"}` so the UI can be honest (§14).
- `EnableOptimisticProvide` exists but is doc-marked EXPERIMENTAL — not used in v1
  (noted for v1.1: cuts provide from ~20 s median to <1 s).

**Acceptance criteria:**
1. In-process 3-host private DHT (one `ModeServer` host as bootstrap in the test):
   host A advertises ns, host B finds A and connects; room converges end-to-end
   through the manager.
2. Advertise-gating proven: a host with only private addrs does not call Provide
   (assert via test hook), then does after gaining a relay addr.
3. (Manual, recorded) two machines on different networks with the real Amino DHT
   find each other with rendezvous disabled — expected latency documented.

**Test plan:** the in-process private-DHT test (uses `ProtocolPrefix` override *in
tests only* to isolate from Amino — the prod path keeps `/ipfs`); race; manual WAN
check recorded in PR.

---

### WP-06 — rendezvous client

**Status:** TODO · **Depends on:** WP-04, WP-07 (its memstore is this package's test
double — see graph) · **Gated by:** T-11 T-12 · **Spec:** §3 (rendezvous primary).

**Goal:** the rendezvous `Source` (primary discovery): register with auto-refresh,
discover with restart-on-close, wired to the point address from T-11.

**Files:** `internal/p2p/discovery_rendezvous.go`,
`internal/p2p/discovery_rendezvous_test.go`.

**Dependency added:** `github.com/waku-org/go-libp2p-rendezvous
v0.0.0-20240110193335-a67d1cc760a0` (root package **only** — cgo-free, verified;
never import its `db/sqlite`/`db/sqlcipher` subpackages).

**Opening spike (T-12 gate, ≤1 h):** `go get` the pin, compile the source against the
tree, run the WP-07-memstore round-trip test. Green → proceed. Red → enact the T-12
contingency (flip decision to REVISED, mark this WP `BLOCKED(contingency enacted)`,
note in Learnings log, notify maintainer in the PR).

**Key API:**

```go
func NewRendezvousSource(h host.Host, point peer.AddrInfo) Source
// internally (verified API):
//   client := rendezvous.NewRendezvousClient(h, point.ID)
//   ttl, err := client.Register(ctx, ns, 300)   // seconds; self-refreshes at ttl-30s
//                                               // (min 120 — do NOT go lower)
//   ch, err := client.DiscoverAsync(ctx, ns)    // ~2 min poll cadence
func ResolvePoint() (*peer.AddrInfo, error) // T-11: PCHAT_RENDEZVOUS env → baked-in
                                            // defaultPointAddr (var, ldflags-injected)
```

**Implementation notes (verified):**
- `Register` is called **once per room ctx** — it spawns its own refresh goroutine
  tied to that ctx; calling it repeatedly leaks refreshers. Cancel ctx on leave (and
  call `Unregister` best-effort — T-29 ephemerality).
- `DiscoverAsync` has **no error recovery upstream** (explicit TODO in its source):
  the channel closes silently on any stream error or cookie invalidation (which a
  point restart causes, since the in-memory store rotates its cookie nonce). The
  source's loop must treat channel-close as "reconnect and restart discovery with a
  nil cookie" with backoff.
- Also `Connect` to the point at startup (it is the static relay too — T-11).

**Acceptance criteria:**
1. In-process test: a host running `rendezvous.NewRendezvousService(h, memstore)`
   (WP-07's store) + two clients → both register, discover each other, room
   converges.
2. Restart resilience: kill and restart the in-process service → clients re-register
   (refresh loop) and re-discover (channel-close restart path) within TTL.
3. Manager priority: with rendezvous and mDNS both live, peers dedupe (one dial).

**Test plan:** in-process service tests as above; race. Cross-network acceptance
against the *deployed* point happens in WP-14 (not gated here).

---

### WP-07 — `cmd/pchat-point`: rendezvous/relay VPS daemon

**Status:** TODO · **Depends on:** — (WP-03 first is convenient, not required) ·
**Gated by:** T-28 T-29 · **Spec:** §3 (VPS role, no-storage, firewall), §16
(scaling note).

**Goal:** the single hosted component: one binary running the rendezvous service (in-
memory store) + circuit relay v2 with chat-appropriate limits, plus deploy artifacts
(systemd unit, provisioning doc). Nothing it does ever touches disk at runtime —
enforced by the unit file, not just promised.

**Files:** `cmd/pchat-point/main.go`, `internal/point/store.go`,
`internal/point/store_test.go`, `internal/point/config.go`, `deploy/pchat-point.service`,
`deploy/README.md`.

**Dependencies added (if WP-03/06 haven't already):** go-libp2p, the rendezvous fork.

**Key API / shape:**

```go
package point

// MemStore implements the fork's storage interface (import path
// github.com/waku-org/go-libp2p-rendezvous/db, package name dbi — note the
// name≠dir quirk). Six methods (verified): Close, Register, Unregister,
// CountRegistrations, Discover, ValidCookie.
type MemStore struct { /* mu, counter uint64, nonce [32]byte, byNs map[string]... */ }
func NewMemStore() *MemStore
```

Semantics mirrored from the sqlite reference (verified): `Register` replaces the
`(peer, ns)` row and returns a monotonic counter; `Discover` paginates by cookie =
8-byte BE counter ‖ SHA-256(nonce‖ns‖counter); `ValidCookie` checks the MAC; expiry =
filter-on-read + 15-min sweep; goroutine-safe. Nonce is random per boot (cookie
invalidation on restart is expected — the client side handles it, WP-06).

`main.go`: flags `--listen` (default `/ip4/0.0.0.0/tcp/4001` +
`/ip4/0.0.0.0/udp/4001/quic-v1`), `--key-file` / `PCHAT_POINT_KEY` (T-29 stable
identity; `--gen-key` prints a fresh one and exits), `--max-reservations` (default
512). Host: same transport/security set as the client, `libp2p.EnableNATService()`
(be a useful AutoNAT server), **no** connmgr protection games — instead raised rcmgr
limits (`PartialLimitConfig{System: {Conns: 1024, ...}}`). Services:
`rendezvous.NewRendezvousService(h, store)` +
`relay.New(h, relay.WithLimit(&relay.RelayLimit{Duration: 6*time.Hour, Data: 256<<20}),
relay.WithResources(...MaxReservations: 512...))` (T-29 — defaults verified
unusable: 2 min/128 KiB). On start, print the full multiaddr including peer ID (what
T-11 bakes into client releases).

`deploy/pchat-point.service`: `DynamicUser=yes`, `ProtectSystem=strict`,
`ProtectHome=yes`, `PrivateTmp=yes`, `NoNewPrivileges=yes`, **no writable paths** —
the no-disk claim is enforced by the sandbox; key comes via
`Environment=PCHAT_POINT_KEY=` drop-in. `deploy/README.md`: provision steps, firewall
(allow only 4001 tcp+udp; §3), how to rotate the key (= client release bump),
bandwidth expectations (registration ~KBs; relay budgeted separately — §3), and the
§16 scale-out sketch (N points behind multiple baked addrs; community-run points via
`PCHAT_RENDEZVOUS` — one paragraph each, next-step not launch-work).

**Acceptance criteria:**
1. `MemStore` passes a contract test exercising all six methods incl. pagination
   cookies, replacement semantics, TTL expiry (fake clock), and `-race` concurrency.
2. Daemon starts, prints its multiaddr, serves rendezvous + relay (in-process client
   smoke test); zero disk writes during operation (verified in test via strace-less
   proxy: run with `TMPDIR` pointed at a read-only dir, or assert no `os.Create`/
   `WriteFile` in the package — plus the systemd sandbox in prod).
3. Deployed once for real, by the maintainer or a session with access: doc followed
   start-to-finish on a fresh VPS; resulting multiaddr recorded (feeds T-11 ldflags
   at WP-12). *(If no VPS access, this criterion moves to WP-14 and the package is
   DONE-pending-deploy — note it in the status board.)*
4. Relay usefulness: an in-process relay-only client (WP-03's mode) sustains a
   >2-minute, >128 KiB chat session through the point (proves T-29 limits took).

**Test plan:** store contract tests; in-process daemon test (rendezvous register/
discover + relayed connection); `-race`.

---

### WP-08 — UI core + CLI wiring (chat MVP)

**Status:** TODO · **Depends on:** WP-04 · **Gated by:** T-14 T-15 T-18 T-23 T-31
P-01 · **Spec:** §5, §6, §10 step 6, §14 (onboarding).

**Goal:** the first runnable pchat: Bubbletea v2 shell (room pane + input bar),
wired to a real room — send publishes, receive renders, onboarding states are
honest, quitting is graceful. After this package the maintainer can dogfood on a
LAN daily.

**Files:** `internal/ui/model.go`, `internal/ui/room_view.go`, `internal/ui/input.go`
(replace placeholders; roster.go waits for WP-09), `internal/ui/ring.go`,
`cmd/pchat/main.go` (replace placeholder).

**Dependencies added:** `charm.land/bubbletea/v2 v2.0.8`, `charm.land/bubbles/v2
v2.1.1`, `charm.land/lipgloss/v2 v2.0.5`, `golang.org/x/term` (all `charm.land`
paths — never `github.com/charmbracelet/*/v2`, T-31).

**Key shapes:**

```go
package ui

type Model struct {
    view     viewport.Model   // SoftWrap: true (native wrapping — the v2 win)
    input    textinput.Model  // placeholder set in WP-10 (P-07)
    ring     *Ring            // in-memory scrollback, cap from --ring-size (§5)
    room     RoomHandle       // narrow interface over *p2p.Room (testability)
    state    Phase            // Connecting | Alone | Chatting | TooNarrow
    profile  colorprofile.Profile // from tea.ColorProfileMsg → T-22 warning
    width, height int
    renderer MessageRenderer  // §5/§9 pluggable-renderer seam
}

// MessageRenderer is the §9 hook: v2 widgets/pipe blocks implement this.
type MessageRenderer interface {
    Render(m Message, width int) string
}

type Ring struct{ ... }        // fixed-cap slice, evict-oldest; no disk, ever
func NewRing(cap int) *Ring    // default 200 (§5); --ring-size (§6)
```

`cmd/pchat/main.go` order: parse flags (`--room`, `--ring-size`, `--relay-only`,
`--debug`, `--version` — the §6 set + version) → resolve passphrase (T-14 chain;
`--room -` uses `term.ReadPassword`; none ⇒ commons per P-01) → banner to stderr
*before* alt-screen ("pchat — ephemeral p2p chat · nothing is stored · IPs may be
visible to peers: see threat model" — one line, P-03) → `identity.GenerateKeypair`
→ `p2p.NewHost` → `JoinRoom` → `NewDiscovery(mdns, [dht], [rendezvous]).Run` (sources
present as their packages land — this main wires whatever exists) → bridge goroutine:
`for ev := range room.Events() { prog.Send(ev) }` with small-batch coalescing →
`tea.NewProgram(model, tea.WithFilter(leaveFirstFilter))`.

**Implementation notes (verified):**
- `Program.Send` blocks before `Run()` starts and no-ops after quit — start the
  bridge goroutine right before `prog.Run()`, and route the leave broadcast through
  `Update`/filter, never through `Send` at teardown (T-15).
- Ctrl+C arrives as `tea.KeyPressMsg` (`"ctrl+c"`) — handle → `room.Leave(ctx500ms)`
  → `tea.Quit`. The `tea.WithFilter` catches external SIGINT/SIGTERM
  (`tea.InterruptMsg`/`QuitMsg` never reach `Update` otherwise — verified).
- Onboarding states (§11/§14): `Connecting` shows per-source status from
  `SourceStatus` ("mdns: listening · dht: bootstrapping…"); `Alone` shows
  "room is empty — messages wait for someone to arrive" (+ commons hint if in
  commons); transition to `Chatting` on first `PeerEvent`. Never a blank screen.
- Send-state (T-18): local echo renders immediately from the self-delivery event;
  marker `·` (pending, `PeerCount()==0`) / `✓` (sent) / `✗` (failed) in the gutter.
- Resize: every `tea.WindowSizeMsg` re-lays-out (initial one is guaranteed on TTYs —
  verified); `TooNarrow` below 40×8 (T-24; roster rules land with WP-09).
- Mouse: nothing enabled (`View.MouseMode` zero value) — T-23; alt-screen on.
- `--version`: print ldflags version, falling back to
  `runtime/debug.ReadBuildInfo().Main.Version` (go-install builds have no ldflags —
  verified).

**Acceptance criteria:**
1. Two terminals on one machine (mDNS): full chat both ways; glyphs+colors render;
   own messages echo exactly once.
2. Onboarding: launching alone shows the discovery status line, never a blank pane.
3. Ctrl+C in one terminal → the other logs an explicit leave (T-16 path) — verify
   with WP-04's headless peer if WP-09's roster UI isn't there yet.
4. Resize during chat reflows (SoftWrap) without garbling; 39-col terminal shows the
   TooNarrow notice and recovers on widen.
5. `pchat` with no room joins the commons (P-01); `--room -` prompts with echo off;
   `PCHAT_ROOM` works; passphrase never appears in the UI.
6. Zero disk writes (`Ring` is the only history; no config/cache files created).

**Test plan:** `Ring` + model-update unit tests (pure; feed `tea.Msg`s, assert view
strings); wiring smoke-tested manually per acceptance (recorded in PR); `-race` on
the bridge (headless: model against a fake RoomHandle).

---

### WP-09 — roster, presence, typing, mute, churn

**Status:** TODO · **Depends on:** WP-08 · **Gated by:** T-16 T-17 T-19 P-02 ·
**Spec:** §5 (roster/typing), §11 (churn), §12 (mute).

**Goal:** the social layer: who's here, who's typing, who left vs dropped, and the
local mute mechanism.

**Files:** `internal/ui/roster.go` (replace placeholder), edits to `model.go` /
`room_view.go`; `internal/p2p/room.go` gains nothing new (all events exist since
WP-04 — this is UI work plus the mute filter hookup).

**Key behavior:**
- Roster sidebar (right, width 12, hidden <70 cols per T-24): glyphs of present
  peers, self first, muted peers greyed + ⊘, colliding identities auto-suffixed
  (T-30 `Short()`).
- System lines in the room pane: "◆ joined", "◆ left", "◆ lost connection" (T-16),
  "⟡'s messages use an incompatible protocol version — hidden" (T-19, once/peer).
- Typing: `TypingEvent` renders a pulsing glyph strip above the input; auto-expire
  4 s (T-17); sending own typing: `started` at most every 4 s while editing,
  `stopped` on send/clear.
- Mute (P-02 mechanism): `room.Mute(pub, true)` — chat + typing from that pubkey
  filtered at the model boundary; roster keeps the peer visible (muted ≠ invisible:
  users should see who they've muted). Command surface arrives in WP-10.

**Acceptance criteria:**
1. 3-instance LAN session: roster consistent on all three within 20 s of any change
   (join, leave, kill -9).
2. kill -9 shows "lost connection" on peers; clean quit shows "left".
3. Typing indicator appears/expires; no self-typing echo.
4. Muted peer's messages stop rendering immediately; unmute resumes (new messages
   only — no retroactive reveal/unreveal of ring content is required in v1).

**Test plan:** model-level tests with synthetic event sequences (roster state
machine, mute filter, typing expiry with fake clock); manual 3-instance pass
recorded.

---

### WP-10 — command system + UX polish

**Status:** TODO · **Depends on:** WP-09 · **Gated by:** T-24 P-01 P-04 P-07 ·
**Spec:** §5, §11 (empty input), §14 (discoverability), §15 (prod command surface).

**Goal:** the complete v1 command surface and the last UX gaps: `/help`, `/who`,
`/mute`, `/unmute`, `/quit`; placeholder hint; empty-input trim; commons banner+AUP;
min-width behavior finalized.

**Files:** `internal/ui/commands.go` (new — T-28), edits to `input.go`/`model.go`.

**Key shapes:**

```go
// Command dispatch: input starting with "/" is parsed, everything else is chat.
type Command struct{ Name string; Args []string }
func ParseCommand(line string) (Command, bool)
// v1 surface (§15 prod list — exactly this, nothing more in prod builds):
//   /help          — command list + one-line privacy reminder
//   /who           — roster with indices (the /mute handles)
//   /mute <n>, /unmute <n>
//   /quit          — same path as ctrl+c (T-15)
// Dev builds add /debug ... via a build-tagged registration hook (WP-11).
```

- Input placeholder: `type a message — /help for commands` (P-07).
- Empty/whitespace-only input: trimmed, never published (§11) — unit-tested.
- Unknown command: local error line, not sent as chat (prevents the classic
  "/mute leaked into the room" embarrassment). A leading `//` escapes to send a
  literal `/`.
- Commons banner (P-01): persistent single line when in the commons, linking
  `docs/commons-room.md` (P-04) via `/help`.
- Chat text length: pre-publish check (T-04) surfaces "message too long (N/4000
  bytes)" inline instead of silently failing.

**Acceptance criteria:** each command behaves per above in a live 2-instance session;
`/mute` indices match `/who`; unknown `/foo` never publishes; placeholder visible on
empty input; `//literal` sends `/literal`.

**Test plan:** `ParseCommand` table tests; command→model behavior tests; manual pass.

---

### WP-11 — dev/prod build split (§15)

**Status:** TODO · **Depends on:** WP-08 · **Gated by:** T-25 T-26 · **Spec:** §15
(entire section).

**Goal:** complete §15: full dev instrumentation behind `//go:build dev`, prod-purity
enforcement in CI, Makefile targets. (The logging shim exists since WP-03 — this
package finishes the job.)

**Files:** `internal/devtools/metrics_dev.go`, `pprof_dev.go`, `commands_dev.go`
(+ `_prod.go` no-op counterparts), `ci/check_prod.sh`, Makefile + `.github/workflows/ci.yml`
edits.

**Key content:**
- Dev metrics: Prometheus client on `127.0.0.1:6060` (`/metrics`, `/debug/pprof`) —
  localhost-bound, dev builds only; §15's metric list (msgs/sec, peers, conns, mesh,
  discovery latency, codec timings).
- Dev debug commands (§15): `/debug peers|mesh|addrs|discovery|drop` registered via
  the WP-10 hook — absent from prod command surface.
- Prod `--debug` (T-25): fmt-based one-line events to stderr; no framework.
- `ci/check_prod.sh` — exactly the verified design (T-26): `go version -m` tag
  detector + `PCHAT_DEV_BUILD_CANARY` strings detector + dev-build self-test that
  proves both detectors have teeth. Wired into CI (on the local prod build) and later
  into the release workflow (WP-12, on all artifacts).
- Makefile: `build-dev` (`go build -tags dev`), `build-prod` (alias of `build`),
  `check-prod`; CI builds + vets + tests **both tag sets**, `-race` on tests.

**Acceptance criteria:**
1. `make build-prod` binary: no metrics/pprof listener, no `/debug` in `/help`, no
   slog; `make build-dev`: all present, localhost-only.
2. `ci/check_prod.sh` passes on the prod binary, **fails** on the dev binary, and its
   self-test fails if the canary is renamed (mutation-check once, manually, recorded).
3. Core behavior identical across tag sets: the full test suite passes under both
   (§15's "dev-testing stays representative").

**Test plan:** CI matrix (2 tag sets × build/vet/test); the check script's self-test;
manual `curl 127.0.0.1:6060/metrics` in dev, connection-refused in prod.

---

### WP-12 — release pipeline (goreleaser + signing)

**Status:** TODO · **Depends on:** WP-07 WP-11 · **Gated by:** T-27 · **Spec:** §8
(distribution), §13 (supply chain), §15 (release = prod).

**Goal:** tagged pushes produce signed, verifiable, reproducible-by-design release
artifacts for both binaries; supply-chain hygiene automated.

**Files:** `.goreleaser.yaml`, `.github/workflows/release.yml`,
`.github/dependabot.yml`, README install/verify section stub (final prose in WP-13).

**Key content (versions verified 2026-07-09):**
- `.goreleaser.yaml` (`version: 2`, goreleaser v2.17.0): two builds —
  `pchat` (darwin/linux × amd64/arm64 + windows/amd64) and `pchat-point`
  (linux × amd64/arm64); `env: [CGO_ENABLED=0]`, `flags: [-trimpath]`,
  `ldflags: -s -w -X main.version={{.Version}} -X <pkg>.defaultPointAddr={{.Env.PCHAT_POINT_ADDR}}`
  (T-11), `mod_timestamp: '{{ .CommitTimestamp }}'` (reproducibility — verified
  bit-for-bit across runs), zip for windows / tar.gz otherwise, `checksums.txt`.
  **No build tags** on release builds — prod is the absence of `dev` (verified
  `builds[].tags` semantics).
- `release.yml`: trigger `push: tags: [v*]`; permissions `contents: write`,
  `id-token: write`, `attestations: write`; `actions/checkout@v7` with
  `fetch-depth: 0`; `actions/setup-go@v6` (`go-version-file: go.mod`);
  `sigstore/cosign-installer@v4` **before** goreleaser;
  `goreleaser/goreleaser-action@v7` (`version: "~> v2"`); then `ci/check_prod.sh
  dist/...` (purity gate on the real artifacts); then
  `actions/attest-build-provenance@v4` with `subject-checksums: dist/checksums.txt`.
- Signing (T-27): goreleaser `signs` block → `cosign sign-blob --bundle
  checksums.txt.sigstore.json` (keyless; the modern bundle format — verified
  required in cosign v3).
- `dependabot.yml`: gomod + github-actions, weekly (supply-chain hygiene, §13).
- `go install github.com/Mediacom99/pchat/cmd/pchat@latest` must keep working:
  **no replace directives ever** (verified: any replace breaks go-install; the
  rendezvous fork needs none) — add a CI grep asserting go.mod contains none.

**Acceptance criteria:**
1. `goreleaser check` + `goreleaser release --snapshot --clean` green in CI (dry-run
   job on PRs touching release files).
2. A real pre-release tag (e.g. `v0.0.1-rc1`) produces: 7 archives, checksums,
   `.sigstore.json` bundle, attestation — and the README verify commands (cosign +
   `gh attestation verify` + `sha256sum -c`) all pass against the downloaded
   artifacts, executed and recorded.
3. `check_prod.sh` gates the workflow (inject a canary into a scratch branch build
   once to watch it fail — recorded).
4. `go install …@latest` on a clean container builds a working prod binary;
   `--version` reports the module version (ReadBuildInfo fallback).

**Test plan:** the rc-tag dry run *is* the test; keep it as a repeatable checklist in
the PR description.

---

### WP-13 — README, threat model, positioning, AUP

**Status:** TODO · **Depends on:** — (docs; needs decision verdicts, not code) ·
**Gated by:** T-01 P-01 P-02 P-03 P-04 P-06 P-08 · **Spec:** §7, §13, §14, §16,
launch checklist ("Before you post anywhere").

**Goal:** the honesty layer: README restructured per the launch checklist, a
user-language threat model, the commons AUP, and consistency fixes (SECURITY.md
still describes the pre-T-01 signing design).

**Files:** `README.md` (restructure), `docs/threat-model.md` (new),
`docs/commons-room.md` (new), `SECURITY.md` (update), `CLAUDE.md` (touch-ups if
drifted).

**README order (launch checklist, verbatim requirement):** (1) one-sentence hook —
proposed: *"A chat room that stops existing when the last person leaves."*; (2)
terminal recording placeholder (asciinema — recorded in WP-14 when the real thing
works); (3) install: release binaries + verify commands (from WP-12) + `go install`
one-liner; (4) usage with the commons passphrase called out (`pchat` → the commons;
`pchat --room "…"`), **immediately followed by** the IP-exposure caveat in user
language (P-03 — including the "up to ~48 h after you leave" sharpening and
"treat passphrases like invite links, not passwords"); then sharing-a-room note
(§11), positioning paragraph (P-03: what pchat is *not* — no history, no accounts,
no way to find the same person tomorrow), links.

**threat-model.md contents (§7 + §13 in user language, each with "what this means
for you"):** protected: impersonation-within-room (via transport signing, T-01
wording), casual room discovery. Not protected: participants who log/screenshot;
traffic analysis/no Tor; weak-passphrase brute force ⇒ room discovery **and**
participant IP harvesting via the public DHT (mechanics: provider records; linger
≤48 h; the ~20 closest DHT servers see participant IPs without any passphrase;
relayed peers expose the relay's IP instead — the one free partial mitigation);
rendezvous operator sees registrations + IPs (§3 trust tradeoff, stated as
first-party: "we run this server; here is exactly what it can see"); Sybil (P-08);
replay bounded to ±10 min (T-05); passphrase in `ps` when using `--room` (T-14
alternatives listed); mDNS advertises "a pchat user is on this LAN" (T-13). Plus the
abuse stance (P-02): no bans possible, mute is local, commons AUP link.

**SECURITY.md:** replace the "Ed25519 signatures over every wire message" framing
with the T-01 reality (transport-layer signing via libp2p pubsub; validator-enforced
freshness/caps); reporting flow unchanged.

**Acceptance criteria:** README top-to-bottom follows the checklist order; every
threat-model claim traces to a decision or a verified research fact (no aspirational
security prose); a cold reader can execute install + verify + join-commons from the
README alone; SECURITY.md no longer references envelope signatures.

**Test plan:** docs — review-only; run a link checker; have a fresh session read
README and list every claim not yet true of the build (that list must be empty at
WP-14).

---

### WP-14 — platform matrix + launch verification

**Status:** TODO · **Depends on:** WP-05 WP-06 WP-10 WP-12 WP-13 + WP-07 deployed ·
**Gated by:** T-22; **launch additionally gated by P-05 (legal consult) being done** ·
**Spec:** §11 (terminal compat), §13 (2–3 peer testing), §16 (Windows), launch
checklist.

**Goal:** the recorded, executed verification pass that gates launch — this package
produces *evidence*, not code (fixes it uncovers are follow-up punch-list items,
each a small PR).

**Files:** `docs/testing-matrix.md` (protocol + results).

**Matrix (execute and record each cell):**

| Dimension | Cells |
|---|---|
| Terminals | iTerm2, Terminal.app, Windows Terminal, conhost (cmd + PowerShell — §16 insists all three Windows shells), gnome-terminal, raw Linux tty |
| Per terminal | glyph set renders (no tofu), 256-color vs degraded profile warning (T-22), resize reflow, mouse-off text selection works, alt-screen restore on exit |
| Network scenarios | LAN/mDNS only; cross-network via deployed point (rendezvous); point stopped → DHT fallback finds peers (measure latency); `--relay-only` end-to-end through the VPS; both peers NAT'd (hole-punch attempt + relay fallback) |
| Scale | 2-peer and 3-peer real-machine sessions (the §13 requirement, on real networks not just the in-process tests) |
| Install | `go install …@latest` on a clean container; release binary + all three verify paths on macOS/Linux/Windows |
| Longevity | 1-hour idle session survives (keepalives, rendezvous refresh, no fd creep — watch dev metrics) |

**Also in this package:** record the asciinema/GIF for the README (launch checklist
asset); final pass of the launch checklist's "Before you post anywhere" engineering
items with links to evidence.

**Acceptance criteria:** every matrix cell has a recorded result; every failure has
a filed punch-list item (and blockers are fixed before this WP is DONE); the launch
checklist engineering section is checkable with evidence links; P-05 confirmed done
by the maintainer before this package is marked DONE.

---

## 7. Learnings log

Append-only. Every entry: date, what was learned, which packages/decisions it
changed. Seeded with what the 2026-07-09 research pass corrected:

- **2026-07-09 — CLAUDE.md's rendezvous-fork cgo claim was wrong.** Verified via
  `go list -deps`: the fork's *root* package pulls neither sqlite nor `database/sql`;
  the cgo drivers live in `db/sqlite`/`db/sqlcipher` subpackages. There is no
  "narrow client import" to find — the root *is* the client package (and also the
  server, cleanly, via a storage interface). CLAUDE.md corrected in the plan's
  commit. → T-12, WP-06, WP-07.
- **2026-07-09 — The Charm stack went v2-stable (2026-02-24) at new `charm.land/*`
  import paths;** CLAUDE.md's bubbletea v1.3.10 / lipgloss v1.1.0 pins are the
  frozen legacy line, and `bubbles` was missing from the table entirely. Windows
  resize regression in v2 fixed at v2.0.7 → pin v2.0.8. → T-31, WP-08, §3 matrix.
- **2026-07-09 — go-libp2p-pubsub v0.17.0 released this very day** (protobuf
  migration); staying on v0.16.0 (proven against go-libp2p v0.48.0 by compile+run).
  → T-32.
- **2026-07-09 — Circuit relay v2 default limits are 2 min / 128 KiB per circuit**
  (verified in source): unusable for relay-fallback chat; the point must raise them.
  Also: stale doc comment there (`MaxReservationsPerPeer` "4" — code says 1);
  trust code, not comments. → T-29, WP-07.
- **2026-07-09 — `go tool nm` is unusable for the §15 prod-purity check** (stripped
  binaries have no symbol table; inlining erases small dev funcs even unstripped) —
  verified empirically; `go version -m` build-tag inspection + used-canary-string
  scan both survive `-s -w` on all five target platforms. §15's own suggested
  mechanism corrected. → T-26, WP-11, WP-12.
- **2026-07-09 — cosign v3 made `--bundle` mandatory for `sign-blob`**: one
  `.sigstore.json` per artifact; the `.sig`+`.pem` two-file layout in older docs is
  legacy. `gh attestation verify` requires GitHub auth even for public repos
  (verified logged-out) — disqualifies attestations as *primary* verification for
  this audience. → T-27, WP-12.
- **2026-07-09 — DHT reality sharpened:** provider records linger ≤48 h after leave
  (nothing un-provides); the ~20 closest Amino servers see participant IPs without
  any passphrase; advertising before reachability is confirmed publishes undialable
  junk (or errors); custom-prefix DHT would need our own bootstrap network
  (maintainer-confirmed upstream). → T-09, WP-05, WP-13 threat model.
- **2026-07-09 — gossipsub verified at small scale:** mesh below Dlo = full clique
  (graceful); flood-publish is OFF by default (must enable — closes the ~1 s
  fresh-join miss window); peer scoring entirely off by default; `Publish` returns
  nil at zero peers (silent); self-delivery is real and `Message.Local` does *not*
  mean "mine". → T-08, T-18, WP-04, WP-08.

---

## 8. Risk register

Risks that could genuinely sink v1 — each with its mitigation *packaged*, not
advised:

- **R1 — Real-world NAT traversal fails** (hole-punch success is probabilistic;
  relay misconfig turns "p2p chat" into "doesn't connect"). *Packaged:* WP-07
  scheduled immediately after the MVP with chat-viable relay limits (T-29) and a
  real deployment; `--relay-only` escape hatch exists from WP-03; WP-14's network
  matrix exercises hole-punch, relay fallback, and double-NAT before anyone
  launches; onboarding UI states (WP-08) make failures legible instead of silent.
- **R2 — The rendezvous fork rots out from under us** (no tags, last commit
  2024-01). *Packaged:* WP-06 opens with a ≤1 h compile-and-run spike; the T-12
  contingency (DHT+mDNS-only v1) is **pre-approved** so no session stalls waiting
  for a maintainer decision; research already proved today's pin works against
  go-libp2p v0.48.0.
- **R3 — Gossipsub misbehaves exactly at our dominant scale (2–3 peers).**
  *Packaged:* verified-by-source analysis says it degrades gracefully (T-08), and
  WP-04's acceptance criteria *require* 2- and 3-peer convergence tests plus
  flood-publish, so regression is caught in CI, not in the launch thread.
- **R4 — Terminal/glyph rendering breaks on real setups** (tofu glyphs, 16-color
  collapse, Windows raw-mode quirks). *Packaged:* conservative curated sets with a
  font-pass acceptance gate (WP-01/T-30); capability detection + honest warning
  (T-22); WP-14's terminal matrix including all three Windows shells; Charm v2
  pinned past the known Windows resize regression (T-31).
- **R5 — Operator legal exposure** (§16: anonymous relay run personally by the
  maintainer). *Packaged as a gate, not code:* P-05 makes the lawyer consult a
  WP-14/launch precondition; WP-07's deploy doc + WP-13's AUP and first-party
  operator-visibility statement build the posture that consult needs. Not
  engineering-closable; deliberately not pretended otherwise.
- **R6 — Dropping the custom signing layer (T-01) silently weakens identity
  guarantees.** *Packaged:* WP-04 acceptance includes a forged-`From` integration
  test proving StrictSign drops impersonation attempts before delivery; SECURITY.md
  and the threat model are updated in WP-13 so the documented model matches the
  implemented one.
- **R7 — Single-VPS centralization becomes the story** (cost, bottleneck, trust —
  §16 "scaling reality"). *Packaged:* discovery works with the VPS down (Amino DHT
  + mDNS, T-09/T-10 — tested in WP-14's "point stopped" cell); `PCHAT_RENDEZVOUS`
  + MIT license (P-06) let anyone run their own point; WP-07's deploy doc carries
  the scale-out sketch so success doesn't catch us flat-footed.

---

*Plan authored 2026-07-09 against `docs/pchat-architecture.md` (as committed) and
live-verified library sources. Maintainer verdicts pending on all decisions.*
