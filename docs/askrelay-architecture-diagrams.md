# askrelay — Architecture diagrams (overview set)

**Status:** the parent diagram set for the whole system. Every node and edge
traces to [`askrelay-architecture.md`](askrelay-architecture.md) (§ refs),
[`askrelay-implementation-plan.md`](askrelay-implementation-plan.md) (WP ids,
which win over the architecture doc), [`phase2-decision-log.md`](phase2-decision-log.md)
(D-xx), and the code on disk. Built-vs-planned is derived from the code, not
from the status board. Verified against the tree on **2026-08-03** (branch
`wp-08-delivery`).

Anything the sources imply but never specify is **not drawn** — it is listed in
[Open / unspecified](#open--unspecified) instead. Deeper per-subsystem charts
hang off the stable anchors below.

| View | Anchor |
|---|---|
| 1 — Component & deployment topology | [`#view-1--component--deployment-topology`](#view-1--component--deployment-topology) |
| 2 — End-to-end message lifecycle | [`#view-2--end-to-end-message-lifecycle`](#view-2--end-to-end-message-lifecycle) |
| 3 — Approval-gate state machine | [`#view-3--approval-gate-state-machine`](#view-3--approval-gate-state-machine) |
| 4 — Trust boundaries & data classification | [`#view-4--trust-boundaries--data-classification`](#view-4--trust-boundaries--data-classification) |
| Legend | [`#legend`](#legend) |

---

## View 1 — Component & deployment topology

Four hosts, one binary. The relay is a single Go process over one SQLite file;
the daemon is optional by hard rule (D-06) and not written yet, so every
push-into-the-session path is dashed. Solid boxes are code in this tree today.
The build frontier is visible on the right-hand side of the relay: the MCP
surface, the resource server, the WS hub and the sweeper are built; the embedded
authorization server that browser clients need is not, which is what keeps
claude.ai / ChatGPT dashed (D-23 put them off the critical path).

```mermaid
flowchart LR
  classDef built fill:#dbeafe,stroke:#1d4ed8,stroke-width:2px,color:#0f172a
  classDef planned fill:#f1f5f9,stroke:#64748b,stroke-width:1.5px,stroke-dasharray:5 4,color:#334155

  subgraph host_a["developer A machine"]
    sess_a["Claude Code session A<br/>MCP client, live local context<br/><i>§2 · §9 · client-side</i>"]
    daemon_a["askrelay daemon A<br/>device key · sign · redact · stdio MCP · WS client<br/><i>§6 · WP-09/10/11 · planned</i>"]
  end

  subgraph vendorhost["vendor-hosted clients"]
    browser["claude.ai · ChatGPT<br/>remote MCP connector, zero install<br/><i>§4.4 · §9 · WP-06 · planned</i>"]
  end

  subgraph relayhost["relay host — one binary, one SQLite file"]
    proxy["reverse proxy<br/>TLS termination<br/><i>§4.5 · §11 · operator-supplied</i>"]
    http_surface["internal/relay server.go<br/>ServeMux · healthz · enroll · graceful drain<br/><i>§4.5 · WP-04 · built</i>"]
    oauth_rs["internal/relay/oauth<br/>RFC 9728 PRM · EdDSA bearer · device creds<br/><i>§4.4 · WP-05 · T-06 · built</i>"]
    oauth_as["embedded authorization server<br/>auth code + PKCE · DCR · CIMD · JWKS<br/><i>§4.4 · WP-06 · planned, not mounted</i>"]
    mcp_surface["internal/relay/mcp<br/>9 tools + spotlighting, stateless<br/><i>§5.1–5.4 · WP-07 · T-07 · built</i>"]
    ws_hub["internal/relay ws.go<br/>connection hub · push · ack · submit<br/><i>§4.5 · §6 · WP-08 · built, WP in progress</i>"]
    sweeper["runSweeper, hourly<br/>ack+grace · hard TTL · tombstone prune<br/><i>§4.2 · WP-08 · T-09 · built</i>"]
    gate["internal/gate<br/>approval state machine, pure and I/O-free<br/><i>§5.3 · WP-02 · built</i>"]
    envelope["internal/envelope<br/>JCS canonical · Ed25519 · size caps<br/><i>§3 · WP-01 · T-16 · built</i>"]
    relay_store["internal/relay/store<br/>persons · devices · threads · messages · drafts · grants<br/><i>§4.1 · WP-03 · T-03 · built</i>"]
    sqlite[("SQLite file<br/>WAL, CGO_ENABLED=0<br/><i>§4.1 · D-09 · built</i>")]
    cli["cmd/askrelay<br/>serve · invite · version<br/><i>§7 · WP-04 · T-05 · built</i>"]
  end

  subgraph host_b["developer B machine"]
    daemon_b["askrelay daemon B<br/>WS client · outbound queue · push into session<br/><i>§6 · WP-09/10 · planned</i>"]
    sess_b["Claude Code session B<br/>answers from live local context<br/><i>§2 · D-21 · client-side</i>"]
  end

  sess_a -.->|"stdio MCP"| daemon_a
  daemon_a ==>|"POST /enroll/{token} once, then WSS GET /ws with device credential"| proxy
  sess_a ==>|"POST /mcp · Bearer · Streamable HTTP"| proxy
  browser ==>|"GET /.well-known/oauth-protected-resource then POST /mcp"| proxy
  browser -.->|"OAuth 2.1 /oauth/* — WP-06, no route mounted"| oauth_as
  sess_b ==>|"POST /mcp · Bearer"| proxy
  daemon_b ==>|"WSS GET /ws · device credential"| proxy

  proxy -->|"plain HTTP on localhost"| http_surface
  http_surface -->|"bearer + PRM"| oauth_rs
  http_surface --> mcp_surface
  http_surface --> ws_hub
  mcp_surface --> relay_store
  ws_hub --> relay_store
  ws_hub -->|"Decode + Verify submitted envelopes"| envelope
  sweeper --> relay_store
  relay_store -->|"every transition"| gate
  relay_store -->|"canonical form, never raw wire"| envelope
  relay_store --> sqlite
  cli -->|"invite opens the DB directly"| sqlite
  ws_hub -.->|"push: new mail"| daemon_b
  daemon_b -.->|"channels notification or hooks line"| sess_b

  class sess_a,sess_b,http_surface,oauth_rs,mcp_surface,ws_hub,sweeper,gate,envelope,relay_store,sqlite,cli built
  class daemon_a,daemon_b,browser,oauth_as,proxy planned
```

17 nodes. Detail deferred to the per-subsystem charts for `internal/relay/mcp`,
`internal/relay/oauth`, and `internal/relay/store`.

Two facts the boxes cannot show. **Nothing in the binary mints an access
token** — `oauth.Issuer.Mint` has no non-test caller, and `/mcp` refuses a
device credential (`use` claim mismatch, `oauth/token.go`), so every `POST /mcp`
edge above is exercised only by tests until WP-06 lands. The device-credential
path (`/enroll` → `/ws`) is complete end to end. And `internal/envelope` is
drawn inside the relay host because that is where it runs today; the same
package links into the daemon binary (§10).

---

## View 2 — End-to-end message lifecycle

One question and its answer. Read it for two things: where a human has to tap,
and where the design and the code diverge. The code gates **four** crossings per
round trip, not two — the docs foreground B's inbound gate and B's outbound
review (D-03, D-11), but `send_message` puts A's own ask in `pending_review`
too, and the arriving reply lands on A's side at `input-required`. Same two
gates, seen from both sides. Delivery on the released path is **relay-attested,
not signed**: `store.DeliverDraft` carries no device signature (its own
`ponytail:` note says so), because the author was authenticated when the draft
was created and released.

```mermaid
sequenceDiagram
    participant sess_a as Claude Code A<br/>§2 · §9 · client-side
    participant dmn_a as daemon A<br/>§6 · WP-09 · planned
    participant relay as relay HTTP surface<br/>§4.5 · WP-04/05/07/08 · built
    participant store as store + gate<br/>§4.1 · §5.3 · WP-02/03 · built
    participant dmn_b as daemon B<br/>§6 · WP-09/10 · planned
    participant sess_b as Claude Code B<br/>§2 · §9 · client-side
    participant hb as B, the human<br/>D-03 · D-11

    Note over sess_a,dmn_a: secret redaction runs client-side before signing<br/>D-12 · WP-11 · planned — no redaction on the built path
    sess_a->>relay: POST /mcp send_message(to, text) — WP-07 built
    relay->>store: StartThread + CreateDraft, state pending_review
    relay-->>sess_a: draft_id, state=pending_review
    Note over sess_a,relay: GATE 1 — A's outbound review (D-11)
    sess_a->>relay: POST /mcp approve_reply(draft_id)
    relay->>store: ReleaseDraft — gate.ApplyOutbound mints a Release, draft to sent
    relay->>store: DeliverDraft — relay-attested, no device signature
    store->>store: canonical blob + replay tombstone + thread to input-required + one delivery row per active device
    relay-->>dmn_b: /ws push frame, type=message — WP-08 built
    dmn_b-->>relay: /ws ack frame — sets acked_at, feeds retention
    dmn_b-->>sess_b: channels notification or hooks line — §6 · WP-10 · planned
    sess_b->>relay: POST /mcp check_inbox()
    relay-->>sess_b: spotlighted DATA block, fresh nonce, URLs de-fanged — §5.2 built
    Note over hb,sess_b: GATE 2 — B's inbound gate (D-03)
    hb->>sess_b: taps approve
    sess_b->>relay: approve_message(id) — thread input-required to working
    sess_b->>relay: send_message(text, thread) — B's AI answers from its own context, draft pending_review
    Note over hb,sess_b: GATE 3 — B's outbound review (D-11)
    hb->>sess_b: taps approve, optionally editing first
    sess_b->>relay: approve_reply(id, edited_text) — ReleaseDraft then DeliverDraft
    relay-->>dmn_a: /ws push frame, then ack — A's thread now input-required
    dmn_a-->>sess_a: nudge into the session — WP-10 · planned
    Note over sess_a,relay: GATE 4 — A's inbound gate (D-03), check_inbox then approve_message
    store->>store: sweeper deletes bodies at all-acked plus 72h grace, or hard TTL 30d — threads, grants and tombstones survive
```

7 participants. The alternative ask path is the daemon's `/ws` submit frame
(`handleSubmit`, built): a signed envelope goes straight to `IngestMessage`
under the socket's authenticated identity, with the outbound gate held locally
by the daemon instead of by the relay — see [Open / unspecified](#open--unspecified).

---

## View 3 — Approval-gate state machine

The two gate states are the mandatory waypoints, so the invariant is structural
rather than decorative: `pending_review` is the outbound gate and
`input-required` is the inbound gate, and there is no edge into a recipient's
inbox that does not leave one of them. `sent` exists only where
`gate.ApplyOutbound` minted a `*gate.Release` — the type has unexported fields
and no other constructor — and `DeliverDraft` refuses any draft not already in
`sent`. The one path that skips the relay's outbound gate is the daemon's `/ws`
submit, drawn as its own entry into the inbound gate.

```mermaid
stateDiagram-v2
    direction LR
    classDef built fill:#dbeafe,stroke:#1d4ed8,stroke-width:2px,color:#0f172a
    classDef planned fill:#f1f5f9,stroke:#64748b,stroke-width:1.5px,stroke-dasharray:5 4,color:#334155

    state "pending_review<br/>OUTBOUND GATE — every draft waits here<br/>gate/states.go · §5.3 · WP-02 · built" as pending_review
    state "sent<br/>a gate.Release was minted<br/>gate.ApplyOutbound · WP-02 · built" as sent
    state "discarded<br/>discard_reply, mints nothing<br/>§5.1 · WP-07 · built" as discarded
    state "input_required<br/>INBOUND GATE — delivered, awaiting the tap<br/>a2a · §3 · WP-03/08 · built" as input_required
    state "working<br/>the recipient's AI may now act<br/>approve_message · WP-07 · built" as working
    state "rejected<br/>decline_message<br/>§5.3 · WP-07 · built" as rejected
    state "acked<br/>every active device acknowledged<br/>store.Ack over /ws · WP-08 · built" as acked
    state "swept<br/>body deleted; thread, grant and tombstone survive<br/>§4.2 · T-09 · WP-08 · built" as swept

    [*] --> pending_review : send_message creates a draft
    pending_review --> sent : approve_reply, a human tap
    pending_review --> sent : outbound grant covers the thread, via_grant=1
    pending_review --> discarded : discard_reply
    sent --> input_required : DeliverDraft, the only path into an inbox
    [*] --> input_required : daemon /ws submit, outbound gate held locally by the daemon
    input_required --> working : approve_message, a human tap
    input_required --> rejected : decline_message
    input_required --> working : inbound grant — PLANNED, store method has no caller
    input_required --> acked : device ack over /ws
    working --> acked : device ack over /ws
    acked --> swept : all acked plus 72h grace
    input_required --> swept : hard TTL 30d, fetched or not
    working --> swept : hard TTL 30d
    discarded --> swept : hard TTL 30d after the decision
    rejected --> swept : hard TTL 30d
    swept --> [*]

    note right of pending_review
      No draft leaves this state without approve_reply
      or an active outbound grant (D-11). gate.Release
      has unexported fields and one constructor, so
      delivery cannot be reached by any other route.
    end note

    note right of input_required
      Delivered is not approved. A message here is
      readable only as spotlighted DATA (§5.2); the
      recipient's AI may act on it only after
      approve_message (D-03).
    end note

    class pending_review,sent,discarded,input_required,working,rejected,acked,swept built
```

8 states. Transition status is carried in the label text, not the edge style:
`stateDiagram-v2` has no portable dashed-edge form. Three A2A states — `completed`,
`failed`, `canceled` — are defined in `internal/a2a` and reachable by no code
path today; `submitted` is written by `StartThread` and by `envelope.New`, but a
recipient-side thread row is created directly at `input-required`.

---

## View 4 — Trust boundaries & data classification

Four zones. The machine-local zone holds everything the relay must never see:
the live session context, the device private key, and the redaction pass that
runs *before* signing (D-12 — the relay is not involved). The vendor zone holds
the human's own login, which is the boundary the whole vendor-ToS posture rests
on: **the relay never sees vendor credentials** (D-06, §8) — all AI work happens
in the participant's own client under their own account. The relay zone is
honest about what it *can* see: message bodies are plaintext in SQLite (E2EE
deferred, D-05), and ephemeral retention is the mitigation, not encryption.
Message content itself is its own zone — authenticated but never benign (D-10).

```mermaid
flowchart TB
  classDef built fill:#dbeafe,stroke:#1d4ed8,stroke-width:2px,color:#0f172a
  classDef planned fill:#f1f5f9,stroke:#64748b,stroke-width:1.5px,stroke-dasharray:5 4,color:#334155

  subgraph zone_local["developer machine — local trust zone"]
    ctx_a["live local session context<br/>uncommitted code · terminal · private repos<br/><i>D-21 · §2 · client-side</i>"]
    redact_a["internal/redact<br/>visible markers, runs before signing<br/><i>D-12 · T-12 · WP-11 · planned</i>"]
    key_a["device Ed25519 private key<br/>0600 beside the config, never leaves the device<br/><i>§4.3 · §7 · WP-09 · planned</i>"]
  end

  subgraph zone_vendor["vendor-hosted client zone — the vendor holds the human's login"]
    vend["claude.ai · ChatGPT<br/>connects OUT to the relay; never driven by it<br/><i>§4.4 · D-23 · WP-06 · planned</i>"]
    vcred["vendor credentials and subscription session<br/>NEVER cross into the relay<br/><i>§8 · D-06 · invariant</i>"]
  end

  subgraph zone_untrusted["untrusted data zone — authenticated but never benign"]
    content["message content<br/>a signature proves origin, never intent or safety<br/><i>§3 · §8 · D-10 · built</i>"]
  end

  subgraph zone_relay["relay host — operator trust zone, self-hosted"]
    tls["reverse proxy — TLS terminates here<br/><i>§4.5 · §11 · operator-supplied</i>"]
    authz["bearer and device-credential auth<br/>identifies a PERSON; holds no vendor secret<br/><i>§4.4 · WP-05 · built</i>"]
    gatestore["gate + store<br/>the only writer of approval state<br/><i>§5.3 · WP-02/03 · built</i>"]
    plaintext[("SQLite bodies — PLAINTEXT<br/>the relay can read message text; E2EE deferred<br/><i>§4.2 · D-05 · D-10 · built</i>")]
    ret["retention sweeper<br/>bodies deleted at ack+72h or hard TTL 30d<br/>threads, grants, tombstones survive<br/><i>§4.2 · T-09 · WP-08 · built</i>"]
    spot["mcp.Spotlight<br/>nonce-tagged DATA block, URLs de-fanged, unknown parts inert<br/><i>§5.2 · T-08 · WP-07 · built</i>"]
  end

  subgraph zone_recipient["recipient machine — local trust zone"]
    sess_b4["recipient session<br/>renders DATA only: no tool triggering, no auto-fetch<br/><i>§5.2 · §8 · WP-07 · built</i>"]
    human_b4["the human — the tap<br/>both gates require a person<br/><i>D-03 · D-11 · built</i>"]
  end

  ctx_a ==>|"only what the human's AI puts in the text"| redact_a
  key_a -->|"signs the canonical form; the key never crosses"| content
  redact_a ==>|"redacted text, then signed — relay not involved"| content
  vcred x--x|"vendor credentials NEVER cross — D-06 · §8"| tls
  vend ==>|"POST /mcp with an OAuth 2.1 bearer — WP-06"| tls
  content ==>|"HTTPS: signed envelope, 64 KiB cap; logs carry ids and shapes only"| tls
  tls --> authz
  authz --> gatestore
  gatestore --> plaintext
  plaintext --> ret
  gatestore ==>|"released only after a gate — D-03 / D-11"| spot
  spot ==>|"spotlighted DATA block crosses; never a fetchable resource"| sess_b4
  sess_b4 --> human_b4
  human_b4 ==>|"approve_message / approve_reply — the only way out of a gate"| gatestore

  class ctx_a,content,tls,authz,gatestore,plaintext,ret,spot,sess_b4,human_b4 built
  class redact_a,key_a,vend,vcred planned

  style zone_local fill:#f8fafc,stroke:#334155
  style zone_vendor fill:#fdf4f5,stroke:#9f1239
  style zone_untrusted fill:#fff7ed,stroke:#c2410c
  style zone_relay fill:#eff6ff,stroke:#1d4ed8
  style zone_recipient fill:#f8fafc,stroke:#334155
```

14 nodes. `ctx_a` and `human_b4` are actors rather than packages; they carry the
decision that binds them instead of a package path. Detail deferred to the
threat-model document (WP-15, arch §8).

---

## Legend

**Status styling** — derived from the code on disk, cross-checked against the
plan's §1 status board.

| Styling | Meaning |
|---|---|
| Solid border, filled | Built: the code exists in this tree today. |
| Dashed border, muted fill | Not built here: a planned WP, or operator-supplied infrastructure (the reverse proxy). |

For nodes that are not packages (actors, credentials, content), the class
reflects whether the *mechanism* exists in the tree today.

**Edge styles**

| Style | Meaning |
|---|---|
| `-->` solid thin | Synchronous request or in-process call. |
| `==>` thick | Trust-boundary crossing — host to host, or untrusted content entering a session. |
| `-.->` dashed | Asynchronous push, or a hop whose implementation is planned. |
| `x--x` crossed | A crossing that must never happen (View 4 only). |
| `->>` / `-->>` | Sequence view: request / push-or-response. |

`stateDiagram-v2` carries edge status in the label text, since dashed
transitions are not portable there.

**Citation format** — every node label ends with an italic line:
`§<architecture section> · <WP id or decision> · <built | planned>`. A `T-xx` is
a technical decision from plan §5; a `D-xx` is a product verdict from the
decision log. Section numbers are `docs/askrelay-architecture.md`.

**Node budget** — roughly 20 nodes per view, hard ceiling 25. For sequence
diagrams the budget counts participants. Anything larger collapses to one node
with a "detail deferred" line beneath the diagram.

---

## Open / unspecified

Everything below is implied by the sources but not decided or not wired. None of
it is drawn as a box.

1. **Inbound grants are recorded but never consulted.**
   `store.ApproveInboundViaGrant` has no non-test caller, and no ingest,
   delivery, or read path reads an inbound grant row.
   `set_thread_grant(direction="inbound")` therefore writes an immortal audit
   row that changes nothing today; only the outbound grant fires
   (`ReleaseReplyViaGrant`, called from `send_message`). Drawn in View 3 as a
   labelled PLANNED transition. The thread-level `ApproveInbound` /
   `DeclineInbound` are likewise callerless — the tools use the message-level
   pair. §5.3 and D-03 describe the inbound grant as live.

2. **The `/ws` submit path has no relay-side outbound gate.** `handleSubmit`
   verifies the signature and the device→person binding, then calls
   `IngestMessage` directly: no draft, no `pending_review`, no `gate.Release`.
   The architecture puts that gate in the daemon (§6: "the daemon never
   auto-approves anything"), but the daemon is WP-09 and does not exist, so the
   relay-side property is currently "unexercised", not "enforced". The
   recipient's inbound gate still holds for a new thread.

3. **`/ws` submit onto an *existing* thread does not re-arm the inbound gate.**
   `ensureThread` deliberately never transitions an existing thread's state, and
   `handleSubmit` does not force `input-required` the way `DeliverDraft` does.
   A submitted message onto a thread already at `working` is stored and visible
   through `get_thread` (spotlighted), but never appears in `check_inbox` and
   gets no fresh per-message verdict. WP-08's own delivery obligation names this
   requirement; it is satisfied on the `DeliverDraft` path only.

4. **A revoked device keeps its live socket.** The open blocking finding:
   `Notify`/`pushInbox` never re-check `ActiveDeviceByID` and `RevokeDevice`
   never calls `hub.remove`/`CloseNow`, so a device revoked while connected is
   still pushed mail. Reproduced on 2026-08-03 —
   `TestWSRevokedDeviceStopsReceivingPush` fails. Not a gate bypass (the mail was
   already released), so View 3's invariant is unaffected; View 4's `/ws`
   boundary is drawn as it is *specified* (§4.3), not as it currently behaves.

5. **Delivery signing is contradicted between the plan and the code.** WP-08's
   "Delivery obligations" bullet says delivery signs the payload; `DeliverDraft`
   is relay-attested with no signature, and the same entry's "Current state"
   paragraph agrees with the code. The diagrams follow the code. Whether the
   released path should ever carry a device signature is undecided — it is
   entangled with the deferred browser-client signing model (D-23).

6. **Browser-client signing is an open wire-format decision** (D-23, WP-08
   entry): a browser MCP client has no local device key, and the choice between
   a relay-held browser-device key and read/approve-only browser clients is not
   made. Taken with WP-06 when the browser-connector path is wanted.

7. **No access token can be minted** until WP-06's authorization server exists,
   so the MCP surface has no live authentication path in the binary. The
   local-first loop (D-23) is unaffected: it runs over `/enroll` → `/ws`.

8. **Three A2A thread states are unreachable.** `completed`, `failed`, and
   `canceled` are defined in `internal/a2a` and validated by the gate, but no
   code path transitions into them. What closes a thread is unspecified.

9. **Not drawn from §6's push paths:** which of the channels bridge and the
   hooks fallback a given session gets is a WP-10 runtime decision gated on
   spike S-01. View 1 and View 2 show one dashed "nudge" edge rather than
   guessing the split.

10. **Client profiles** (§5.4) select a `wait_for_activity` cap from the token's
    `client_type` claim, which only WP-06's authorization server will stamp.
    Today `profileCap` falls through to the 45 s ChatGPT-safe default for every
    caller. Not drawn — it is a per-request property, not a component.
