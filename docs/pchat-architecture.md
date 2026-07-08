# pchat — Architecture Document (v1)

A terminal-only, fully P2P, ephemeral chat tool for developers and internet weirdos.
No servers, no accounts, no logs. You exist in a room only while you're in it.

---

## 1. Design Principles

- **Ephemeral by default.** Nothing touches disk. No history, no logs, no saved identity.
- **No central server.** Rooms exist only as long as at least one peer holding them is online.
- **Minimal wire footprint.** Every byte on the wire should earn its place.
- **Identity is anonymous but distinguishable.** You don't get a name — you get a signature.
- **v1 = core chat only.** Pipe support, code sandbox, widgets, drift mode, Tor — all deferred to v2+. Don't build them now; just don't architect anything that blocks adding them later.

---

## 2. Identity

- **Keypair:** Ed25519, generated fresh on every process launch. Never written to disk.
- **Peer ID:** derived the standard libp2p way (multihash of the public key).
- **Signature glyph:** deterministically derived from the pubkey bytes —
  - Take first N bytes of `SHA-256(pubkey)`.
  - Map to: one ASCII/Unicode glyph from a curated symbol set + one 256-color terminal color code.
  - Same pubkey always → same glyph+color for that session. Different launch → different keypair → different glyph.
- No nicknames in v1. Recognition in a room is purely visual (glyph + color), reinforcing the "faces without names" feel.

---

## 3. Networking

**Base layer:** go-libp2p

- **Transports:** TCP + QUIC (QUIC preferred when available — faster handshake, built-in encryption).
- **Security:** Noise protocol (libp2p default).
- **NAT traversal:**
  - AutoNAT for detecting reachability.
  - Circuit relay v2 as fallback for peers behind symmetric NATs / strict firewalls.
  - Hole punching (DCUtR) attempted before falling back to relay.
- **Peer discovery (v1 = LAN + internet):**
  - **Rendezvous point (primary, v1):** a lightweight VPS running `go-libp2p-rendezvous`. Peers `Register` under a namespace equal to `topic_id` (see below) with a short TTL, and `Discover` other peers registered under the same namespace, receiving back full multiaddr info (not just a raw IP) so direct dialing can follow. Use the existing library rather than a custom HTTP endpoint — it already returns NAT-aware address info correctly.
    - As a side effect, the VPS observes each peer's real public IP:port during registration, which can be used to coordinate a hole-punch attempt (DCUtR) between two NAT'd peers immediately after a `Discover` match — effectively a free lightweight STUN role, layered on top of its rendezvous job.
    - The same VPS doubles as a **Circuit Relay v2** fallback for peers behind symmetric NATs where hole punching won't succeed. Budget relay bandwidth separately from rendezvous traffic — registration/lookup is tiny (KBs), relay is proxying real chat traffic for the life of a connection.
    - **No persistent storage on the rendezvous VPS.** Registrations are TTL-based and should live fully in memory, matching the project's no-logs ethos — this component should not become the one piece of infrastructure that quietly writes anything to disk.
    - Firewall the VPS to only the rendezvous/relay port; nothing else exposed.
  - **mDNS** for same-LAN discovery (instant, zero-config), kept as a fallback path independent of the VPS.
  - **Kademlia DHT** kept as a second fallback — slower to bootstrap but fully decentralized, so discovery still works if the rendezvous VPS is ever down.
  - **Client discovery order:** rendezvous (fast) → DHT (slower, resilient) → mDNS (LAN-only), falling through automatically if a method fails or times out.
  - **Trust tradeoff to be explicit about:** the rendezvous VPS operator sees which peers register under which room-topic hashes, plus their real IPs — this is a more centralized point of metadata exposure than pure DHT discovery (where that knowledge is naturally sharded across many nodes). Message *content* still never touches the VPS and remains Noise-encrypted end-to-end, but this should be stated plainly to users, not left implicit.

**Rooms:**
- A room = a **topic string**, never transmitted in plaintext.
- `topic_id = SHA-256(room_passphrase)`
- Peers use `topic_id` both as the **gossipsub topic** and as the **DHT lookup key** to find other peers in the same room.
- No passphrase = no way to find or join a room. This is the entire access-control model for v1 — deliberately simple.
- Messaging fan-out via **gossipsub** (libp2p pubsub).

**Explicitly out of scope for v1:** proof-of-work join gating, Tor/onion transport. Both are clean additions later (PoW as a pubsub validator function, Tor as an alternate transport) — leave hooks but don't build them now.

---

## 4. Wire Protocol

**Format: CBOR** (Concise Binary Object Representation).

Why CBOR over raw custom binary or Protobuf:
- Binary and compact — smaller than JSON, comparable to Protobuf for small messages.
- Self-describing (no separate `.proto` compilation step), which keeps the project easy for outside devs to write bot/alt clients against — they just need the CBOR spec, not your build tooling.
- Mature Go library: `fxamacker/cbor`.

If you later want the absolute smallest possible payload (e.g. for very high-frequency presence/cursor updates), a hand-rolled fixed-width binary format is the next step down — but CBOR is the right tradeoff for v1 between tiny and maintainable.

**Message envelope (all messages share this shape):**

```
Message {
  v         uint8       // protocol version, starts at 1
  type      uint8       // 0=chat, 1=presence, 2=typing  (widget/file-chunk reserved: 3, 4...)
  sender    bytes       // sender pubkey (32 bytes, Ed25519)
  ts        uint64      // unix millis, sender-local clock, informational only
  sig       bytes       // Ed25519 signature over (v|type|sender|ts|payload)
  payload   bytes       // type-specific CBOR-encoded body
}
```

**Payload types (v1):**
- `chat`: `{ text: string }`
- `presence`: `{ state: uint8 }` (0=joined, 1=left)
- `typing`: `{ state: uint8 }` (0=started, 1=stopped)

**Versioning:** the `v` field lets future clients detect and reject/ignore messages from incompatible protocol versions rather than crashing on unknown fields.

**Signing:** every message is signed with the sender's session privkey. Peers verify `sig` before rendering/relaying. This doesn't prevent a malicious peer from lying about content, but it does prevent impersonation of another peer's glyph/identity within a session.

---

## 5. Terminal UI

**Framework:** Bubbletea (charmbracelet) + Lipgloss for styling.

**Layout (v1):**
- **Room pane:** scrolling message view. Each line prefixed with sender's glyph+color.
- **Input bar:** single-line input at the bottom.
- **Roster (optional, thin sidebar or top bar):** glyphs of currently present peers.
- **Typing indicator:** small ephemeral marker (e.g. a peer's glyph pulsing) — driven by `typing` messages.

**Ephemeral scrollback:**
- In-memory ring buffer, default cap **200 messages** per room, configurable via flag.
- Nothing is ever flushed to disk. Exiting the app == total loss of history, by design.

**Accessibility & legibility:**
- **Colorblind-safe palette:** glyph color must be derived from a curated, tested-distinct subset of the 256-color space — not the raw hash output — so peers remain distinguishable for colorblind users (~8% of men). Color should never be the *only* differentiator; pair it with a genuinely distinct glyph shape, not just color variants of the same symbol.
- **Glyph font compatibility:** the curated glyph/symbol set needs a legibility pass across common terminal fonts (Menlo, Consolas, JetBrains Mono, default Windows Terminal font) before being locked in, to avoid tofu/box rendering on some setups.

**Message delivery feedback:**
- Sent messages need a visible state distinction: pending (published locally, not yet confirmed) vs. confirmed vs. failed. On flaky connections, silent uncertainty reads as a broken app — don't let "did that even send?" be an open question for the user.

**Resize & narrow-terminal behavior:**
- Explicit handling for `SIGWINCH` (terminal resize) mid-session — scrollback reflow and roster truncation need real testing, not an assumption that Bubbletea degrades gracefully by default.
- Define a minimum-width behavior (e.g. below ~60 cols): does the roster hide, do glyphs abbreviate, or does layout break? Pick one on purpose.

**Mouse mode:**
- Decide explicitly whether Bubbletea's mouse capture is enabled. On: unlocks future widget interactions. Off: preserves native terminal text-selection/copy-paste, which users will expect by default. Don't leave this as an accidental default either way.

**Out of scope for v1:** ASCII art rendering, sixel/kitty image support, shared widgets, drift mode. Architect the UI with a pluggable "message renderer" so these can slot in later without a rewrite.

---

## 6. CLI / Config

**v1 flags (no config file needed yet — keep it simple):**

```
pchat --room "some passphrase"       # required: derives topic_id
      [--ring-size 200]              # scrollback buffer size
      [--relay-only]                 # force relay, skip hole punching (debug/testing)
      [--debug]                      # opt-in verbose logging to stderr only, never to disk
```

- No identity flags — identity is always fresh per launch.
- No nickname flag — glyph is the identity, on purpose.

---

## 7. Security & Threat Model

**What v1 protects against:**
- Impersonation within a room (message signing/verification).
- Casual discovery of a room by outsiders (passphrase-hashed topic).

**What v1 explicitly does NOT protect against (be upfront about this with users):**
- A participant who joins a room and locally logs/screenshots everything — no protocol can prevent this.
- Traffic analysis / metadata leakage at the network level (no Tor in v1).
- Brute-forcing weak/common passphrases to find a room — this is a low bar, not a security boundary. Treat the passphrase like a Discord invite link, not a password.
- DoS/spam from a malicious peer — no PoW or rate limiting in v1. A basic per-peer message rate cap is a good "v1.1" addition even before PoW.

---

## 8. Build & Repo Structure

```
pchat/
  cmd/pchat/
    main.go              # CLI entry, flag parsing, wires everything together
  internal/
    p2p/
      host.go            # libp2p host setup, transports, NAT config
      discovery.go       # mDNS + DHT peer discovery
      room.go             # topic derivation, gossipsub join/leave
    protocol/
      message.go         # Message struct, CBOR encode/decode
      sign.go            # sign/verify helpers
    identity/
      identity.go        # keypair gen, glyph+color derivation
    ui/
      model.go           # Bubbletea root model
      room_view.go       # scrollback rendering
      input.go           # input bar component
      roster.go          # presence sidebar
  go.mod
  README.md
```

**Distribution:** single static binary, cross-compiled (goreleaser) for macOS/Linux/Windows, amd64+arm64. No runtime dependencies — this matters for the "just run it" feel you want.

---

## 9. Deferred to v2+ (explicitly not in this doc's scope, but keep interfaces open for them)

- Pipe support (`cat file | pchat`) → shared syntax-highlighted blocks
- Sandboxed code execution + shared output
- Shared ASCII widgets (whiteboard, counter, tiny games)
- Drift mode (idle rooms merging)
- Tor/onion transport toggle
- Proof-of-work join gating
- Per-peer rate limiting

---

## 10. Suggested Build Order (for Claude Code sessions)

1. `identity` package — keypair gen + glyph derivation (pure logic, no networking, easy to test standalone).
2. `protocol` package — Message struct, CBOR encode/decode, sign/verify (also testable in isolation).
3. `p2p/host.go` — bring up a bare libp2p host, confirm it starts and gets a peer ID.
4. `p2p/room.go` + `discovery.go` — topic derivation, mDNS join, gossipsub pub/sub, confirm two local processes can talk.
5. Add DHT discovery for cross-network joins.
6. `ui/` — Bubbletea shell wired to the p2p layer: send input → publish message; receive message → append to scrollback.
7. Polish: roster, typing indicators, ring buffer eviction, `--debug` logging.

---

## 11. Open Questions / Known Gaps (decide before or during build — don't let these get silently assumed)

- **Multi-room support:** is one running instance locked to a single room, or can it join several at once? Affects the UI model (would need tabs/panes) and whether the p2p layer needs to manage multiple topic subscriptions concurrently. v1 default assumption unless decided otherwise: **one room per process.**
- **Message ordering:** gossipsub gives no cross-peer delivery-order guarantee. Bursts of messages may render slightly out of order. Decide whether this matters (likely not, for casual chat) or whether a simple per-sender sequence number should be added to the envelope later for stable local sorting.
- **Max payload size:** no cap defined yet. Need a hard limit (e.g. 4KB) on the `chat` payload to stop a single peer from flooding scrollback with a huge paste.
- **Connection churn:** need distinct handling/UI states for "peer timed out / connection dropped" vs. "peer sent an explicit `left` presence message" — otherwise timeouts will look like silent bugs rather than departures.
- **Terminal compatibility:** glyph + 256-color rendering needs manual verification across common terminals (iTerm, Windows Terminal, plain Linux tty) — don't assume Lipgloss output looks identical everywhere.
- **Testing strategy:** no test harness defined yet. Since there's no server, integration tests likely mean spinning up 2–3 libp2p hosts in-process (or via a test helper) and asserting they converge on shared room state, rather than manually testing across terminal windows.
- **Graceful shutdown:** Ctrl+C / SIGINT should trigger a `presence: left` broadcast before the process exits, not just terminate the connection silently.
- **Onboarding UX:** a room with no peers yet should show an explicit "listening for peers..." state — a blank terminal with no feedback will read as broken rather than working-as-intended.
- **Passphrase sharing:** the room passphrase is the entire join mechanism, but sharing it is out-of-band by design (link, QR for local demos, pasted in Discord, whatever). Not a code problem — just needs a line in the README so first-time users know how people actually get into the same room together.
- **Version mismatch UX:** the `v` field lets peers detect incompatible protocol versions, but silently dropping those messages will look like a bug rather than a mismatch. v1 should surface some minimal notice (e.g. "peer running incompatible version") rather than just discarding silently.
- **Empty/whitespace input:** trivial but easy to forget — trim input and ignore empty sends so blank lines don't get published as messages.

---

## 12. Product-Level Considerations (design, not just engineering)

- **The cold-start / discovery paradox:** the core pitch is connecting strangers who don't know each other yet, but the join mechanism requires a pre-shared passphrase — which only works once two people already have a way to coordinate. As-is, the architecture serves private/friend-group rooms well but doesn't solve "how do weird internet people find each other" at all. Worth deciding before launch: is there a **public "commons" room** — e.g. running `pchat` with no `--room` flag lands you in a well-known default topic — acting as a front door where strangers land, talk, and splinter off into private passphrase rooms from there? Without something like this, the product has no actual cold start.
- **Abuse & recourse:** the privacy model (no logs, no persistent identity, fully anonymous) is a deliberate strength, but it also means zero consequence for a harasser — reconnecting generates a brand-new keypair, so there's no ban that survives a reconnect and no record of what was said. This needs an explicit product stance, not a silent default. Minimum viable safety valve worth considering for v1: a **local mute** — client-side, session-scoped ignore of a peer's current pubkey — even without any persistent/global ban mechanism.

---

## 13. Architecture & Security Review

### Architecture

- **Redundant identity/signing layer:** libp2p already signs pubsub messages with the peer's own key and carries the PeerID per message. The custom envelope's `sender` + `sig` fields duplicate this. Either drop custom signing and derive the glyph directly from the libp2p PeerID/pubkey, or have a clear reason to keep both — as specified, it's two overlapping signature systems, which is unnecessary attack surface for no real gain.
- **Gossipsub mesh minimums at small scale:** gossipsub's default mesh parameters (`D_lo`/`D_hi`) are tuned for a reasonable peer count. Rooms of 2–3 peers (which will be common for this use case) may behave differently than gossipsub's design target. Don't assume it "just works" at any size — explicitly test 2-peer and 3-peer rooms.
- **Unbounded connections/resources:** no connection manager limits (watermarks) are specified yet. Without explicit low/high watermarks, a room — or a single hostile peer — could open enough connections to exhaust file descriptors or memory. This needs to be explicit config, not left at library defaults.
- **CBOR decode limits:** the 4KB payload cap bounds byte size but not structural complexity — a CBOR decoder can still be pointed at deeply nested structures as a DoS vector. Decoder needs explicit max-nesting-depth and max-size guards, not just a payload byte limit.

### Security

- **DHT provider records leak IP addresses by design:** using the passphrase hash as the DHT lookup key means anyone who knows (or brute-forces) a passphrase can query the DHT and obtain participants' IP addresses. Weak/common passphrases are effectively public rooms with exposed IPs. This should be stated plainly and prominently in the README, not buried only in the internal threat-model section — better to be upfront than have someone discover it after the fact.
- **No replay protection:** signed messages have no nonce or dedupe window. A malicious relay peer could re-broadcast an old signed message later, making it appear freshly sent by someone no longer present. Fix: a short-lived seen-message cache (hash of the signed envelope) plus a timestamp freshness window (reject messages with `ts` outside roughly ±N minutes of local clock).
- **Zero-cost Sybil attacks:** Ed25519 keypairs are free and instant to generate, so a single person can flood a room with dozens of apparent "peers." PoW gating is deferred to v2 (per §9) — reasonable, but this should be tracked as a known, currently-unmitigated Sybil vector rather than left implicit.
- **Passphrase exposure via process list:** passing `--room "passphrase"` as a CLI argument makes it visible to other users on a shared machine via `ps aux` and similar. Should support an environment variable or interactive stdin prompt as an alternative input method.
- **Supply chain:** go-libp2p pulls in a large dependency tree. Pin versions and commit `go.sum`; consider signing release binaries (e.g. cosign or minisign). There's no auto-update mechanism, so without signed releases users have no way to verify a downloaded binary matches what was actually built from source.

---

## 14. First-User Perspective (reality check)

Notes from walking through the experience as a brand-new user landing from a launch post, not as the builder. These are the moments that will actually determine whether someone stays past the first 30 seconds.

- **"Prove it's alive" is the single biggest churn risk in the product.** A new user typing `pchat --room X` and seeing only "listening for peers..." with nothing else will assume it's dead or broken within seconds and tab away — this matters more than almost any technical decision elsewhere in this doc. Ties directly to §5's onboarding-state requirement and §12's cold-start problem: the commons room needs to feel reliably alive, and the maker (or seed users) should be present in it during any launch window, not just architecturally possible to join.
- **In-app command discoverability is a real gap, not a nice-to-have.** A user who doesn't know `/help` or `/who` exist just sits in front of a bare input bar unsure what's possible. At minimum, placeholder hint text (e.g. `type a message, /help for commands`) should be considered part of v1, not a polish item deferred to later.
- **Honest positioning matters more than feature completeness.** This product is naturally suited to being a "drop-in, ephemeral, weird hangout" experience — not a daily-habit app, since resetting identity every launch works against building return visits or finding specific people again. Marketing and onboarding copy should set this expectation explicitly rather than imply persistence or community continuity the architecture doesn't actually provide. This isn't a flaw to fix — it's a positioning decision to make on purpose (see also the launch checklist's framing guidance).
- **The IP-exposure caveat (§13) is a trust cliff, not a footnote.** A first-time user who later learns their IP was exposed via an "obvious" passphrase, without having been told upfront, won't just be mildly annoyed — they'll feel misled and are likely to warn others off the tool. This reinforces §13's recommendation to state it plainly and prominently, not bury it.

---

## 15. Dev vs. Prod Build Architecture

Two genuinely separate build artifacts, not one binary with a hidden flag. The goal: it should be **structurally impossible** for dev-only instrumentation to end up in what gets distributed to real users — not just disabled by default, but literally not compiled in.

### Why two artifacts, not a runtime flag

A single binary with `--verbose` or `--debug` is one accidental flag away from leaking logs/metrics in a prod deployment, and it means dev tooling still ships inside every user's download even if switched off. Instead, use **Go build tags** to physically exclude dev-only code from the prod build:

```go
//go:build dev

// logging.go, metrics.go, debug commands — only compiled into dev builds
```

```
make build-dev    # go build -tags dev ./cmd/pchat
make build-prod   # go build ./cmd/pchat   (no tags — dev files aren't even in the binary)
```

### Dev build — what it includes

- **Structured logging** (e.g. `log/slog` or `zerolog`) to stderr and/or a local file in a temp dir — never inside the prod code path at all.
- **Metrics endpoint** (Prometheus client, `/metrics` bound to `localhost` only) tracking things like: messages/sec, active peer count, connection count, gossipsub mesh health/score, DHT and rendezvous lookup latency, CBOR encode/decode timing.
- **pprof endpoint** for CPU/memory profiling during development.
- **Debug commands** — e.g. dump current peer table, force a DHT lookup, simulate N local peers for mesh-size testing, inject artificial latency/packet loss to test NAT traversal and hole-punching paths without needing real-world flaky networks.
- **Verbose error messages** — full error chains, stack traces, internal state dumps. Fine in dev, actively bad in prod (see below).

### Prod build — what it strips down to

- **No logging code path at all** — not "logging disabled," but the logging package isn't linked into the binary.
- **No metrics/pprof endpoints** — nothing listening that wasn't explicitly part of the chat protocol itself. Smaller attack surface, and nothing to accidentally expose if someone runs it on a box with an open port.
- **No debug commands** — the command set a prod user sees is exactly the v1 command surface (chat, `/help`, `/who`, mute), nothing internal.
- **Minimal, generic error messages** — a connection failure should say "couldn't connect, retrying" to the user, not leak internal state, stack traces, or peer implementation details that could aid an attacker profiling the network.
- **Hardened defaults locked in, not just recommended** — connection manager watermarks (§13), CBOR decode depth/size guards (§13), timestamp freshness window for replay protection (§13) all on by default with no flag to loosen them in the prod build.

### CI/build pipeline

- Verify the prod build genuinely excludes dev code — e.g. a CI step that greps the compiled binary for known dev-only strings/symbols (like the metrics route path or a log format string) and fails the build if found, rather than trusting the build tag alone.
- Cross-compile prod releases the same way as described in §8 (goreleaser, signed binaries per §13); dev builds only need to run locally and never need cross-compilation or signing.
- Keep dev and prod builds in the same repo/module — no code duplication — differentiated purely by build tags on a small number of files (logging, metrics, debug commands), so the core protocol/UI logic is identical between both and dev-testing stays representative of what prod actually runs.

---

## 16. Remaining Gaps (real-world, not engineering)

These weren't covered by any of the technical passes above and shouldn't be treated as solved just because the rest of the doc is thorough.

- **Operator legal exposure — flagged, not resolved here.** You'll be personally running the rendezvous/relay VPS, for a product explicitly pitched as anonymous, no-logs, "talk freely about anything." Relay traffic is encrypted end-to-end, so you can't see what's proxied through your server — but that same fact is exactly what a liability argument could turn around on you as the operator. This is a real legal question (platform liability, relay/common-carrier-style exposure varies significantly by jurisdiction) and needs an actual conversation with a lawyer before launch, not a policy written into this doc. Don't treat this as closed just because it's now written down.
- **Open-source license:** none chosen yet. Matters more than usual here since decentralization is part of the pitch — the license affects how freely others can fork the project and stand up their own independent rendezvous points rather than everyone depending on yours.
- **Windows terminal quirks:** beyond the font/color rendering already noted in §5, `cmd.exe`, PowerShell, and Windows Terminal can differ in raw-mode input handling and ANSI escape support under Bubbletea. Needs explicit testing across all three, not just "Windows" as one bucket.
- **Scaling reality if it takes off:** the v1 architecture assumes a single VPS for rendezvous/relay. If usage grows, relay bandwidth becomes a real cost and that VPS becomes both a bottleneck and a bigger single point of trust/failure. Not a launch blocker, but worth having a rough next-step plan (more relay nodes, letting the community run their own) rather than being caught flat-footed by success.
- **Acceptable-use stance for the commons room:** since you're the named operator of the default public room, even a no-logs/anonymous design usually benefits from a short, plainly stated policy — if only so you have something to point to if the room's content or use ever gets questioned.
