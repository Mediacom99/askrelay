# CLAUDE.md — pchat

## Project overview

pchat is a terminal-only, fully peer-to-peer, ephemeral chat tool for developers and
"internet weirdos." There are no servers, no accounts, and no logs — you exist in a
room only while you're in it, and nothing touches disk. Rooms are defined purely by a
shared passphrase (`topic_id = SHA-256(room_passphrase)`), identity is an anonymous but
distinguishable per-session Ed25519 keypair rendered as a glyph+color signature (no
nicknames), and messaging fans out over libp2p gossipsub. The single hosted component
is a lightweight rendezvous/relay VPS that never stores anything. v1 is core chat only.

**The source of truth for all design decisions is
[`docs/pchat-architecture.md`](docs/pchat-architecture.md).** Read the relevant section
before implementing — this file is a map, not a replacement.

**The source of truth for execution is
[`docs/pchat-implementation-plan.md`](docs/pchat-implementation-plan.md).** It contains
the work packages, dependency graph, decision log (with maintainer verdicts), risk
register, and the session protocol. Where an APPROVED decision there amends the
architecture doc, the plan wins.

## Current status

**Scaffolded + planned; no application code yet.** Every file under `internal/` and
`cmd/` is a placeholder (package declaration + TODO citing the architecture section).
`go.mod` intentionally lists **no dependencies** — they are added as each work package
first imports them, at the exact versions pinned in the plan's §3 dependency matrix.
`go.sum` is empty for the same reason.

**Next step:** follow the implementation plan's §2 Execution protocol — pick the next
eligible work package from its status board (dependencies DONE, gating decisions
APPROVED), implement it, run its test plan, update its status. Do not implement
against decisions still marked PROPOSED when the package lists them as gates.

## Repo layout (architecture §8)

```
pchat/
  cmd/pchat/
    main.go              # CLI entry, flag parsing, wires everything together
  internal/
    p2p/
      host.go            # libp2p host setup, transports, NAT config
      discovery.go       # mDNS + DHT peer discovery
      room.go            # topic derivation, gossipsub join/leave
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

## Build order

The architecture's §10 ordering (identity → protocol → networking → UI) is refined
into PR-sized work packages **WP-01 … WP-14** in the implementation plan, which also
covers what §10 omits: the rendezvous/relay VPS daemon, the dev/prod build-tag split,
the release pipeline, and the README/threat-model work. **Use the plan's status board
and dependency graph — not this list — to pick work.** Recommended sequential order:
WP-01, 02, 03, 04, 08 *(LAN-chat milestone)*, 07, 06, 05, 09, 10, 11, 12, 13, 14.

## Key architectural constraints

- **Ephemeral, no disk.** No history, logs, or saved identity — ever. Fresh Ed25519
  keypair per launch (§1, §2).
- **Identity = glyph + 256-color**, derived deterministically from the pubkey via a
  curated, colorblind-safe set. Color is never the only differentiator. No nicknames (§2, §5).
- **Rooms via passphrase only.** `topic_id = SHA-256(passphrase)` is both the gossipsub
  topic and DHT key; it is never sent in plaintext. One room per process in v1 (§3, §11).
- **Transports** TCP + QUIC, Noise security, NAT traversal via AutoNAT + DCUtR hole
  punching + circuit relay v2 fallback. Discovery order: rendezvous → DHT → mDNS (§3).
- **Wire format = CBOR** (`fxamacker/cbor`). Shared signed envelope
  `{v,type,sender,ts,sig,payload}`; every message signed and verified before
  render/relay (§4). Enforce a payload cap (~4KB) and CBOR decode depth/size guards (§4, §13).
- **UI** = Bubbletea + Lipgloss; in-memory ring buffer (default 200), pluggable message
  renderer, explicit SIGWINCH/narrow-terminal handling, send-state feedback (§5).
- **Security is honest about limits** (§7, §13): no protection against a peer logging
  the room, metadata/traffic analysis, weak-passphrase brute force, Sybil flooding, or
  IP exposure via DHT for weak passphrases. Surface these, don't hide them.
- **Dev vs. prod builds are separate artifacts via build tags** (§15): dev-only
  logging/metrics/pprof/debug commands live behind `//go:build dev` and must not be
  compiled into prod. Keep the core path identical between the two.
- **v2+ is out of scope** (§9) but leave hooks open (PoW as a pubsub validator, Tor as
  an alternate transport, pluggable renderer for widgets).

## Planned dependencies

`go.mod` starts empty on purpose (see "Current status"). Add each dependency with
`go get` at the moment the first real code imports it — do not pre-add them. The
authoritative pin list is the **implementation plan's §3 dependency matrix** (every
entry there was verified against live sources on 2026-07-09, including compile-and-run
proofs). Summary:

| Library | Module | Pin (verified 2026-07-09) | Enters at |
| --- | --- | --- | --- |
| libp2p host | `github.com/libp2p/go-libp2p` | v0.48.0 | WP-03 (`p2p/host.go`) |
| gossipsub | `github.com/libp2p/go-libp2p-pubsub` | v0.16.0 (v0.17.0 exists but is hours old — see plan T-32) | WP-04 (`p2p/room.go`) |
| Kademlia DHT | `github.com/libp2p/go-libp2p-kad-dht` | v0.41.0 (requires exactly go-libp2p v0.48.0) | WP-05 |
| rendezvous | `github.com/waku-org/go-libp2p-rendezvous` | v0.0.0-20240110193335-a67d1cc760a0 (no tags; pin the pseudo-version) | WP-06 client / WP-07 server |
| CBOR | `github.com/fxamacker/cbor/v2` | v2.9.2 | WP-02 (`protocol/message.go`) |
| Bubbletea | `charm.land/bubbletea/v2` | v2.0.8 | WP-08 (`ui/`) |
| Bubbles (viewport, textinput) | `charm.land/bubbles/v2` | v2.1.1 | WP-08 (`ui/`) |
| Lipgloss | `charm.land/lipgloss/v2` | v2.0.5 | WP-08 (`ui/`) |
| terminal (hidden passphrase prompt) | `golang.org/x/term` | latest at import | WP-08 (`cmd/pchat`) |
| Prometheus client (**dev builds only**) | `github.com/prometheus/client_golang` | latest at import | WP-11 (`internal/devtools`) |

Notes:

- **Charm stack is the v2 line at `charm.land/*` vanity paths** (plan decision T-31).
  The Charm libraries went v2-stable 2026-02-24 under new import paths — do **not**
  use `github.com/charmbracelet/bubbletea` (frozen v1 line, previously pinned here) or
  `github.com/charmbracelet/*/v2` (stale path — mixing it with `charm.land` compiles
  two incompatible copies of bubbletea). Bubbletea ≥ v2.0.7 is required (Windows
  resize regression fixed there).
- **Rendezvous uses a maintained fork, not the spec's named module.** §3 names
  `github.com/libp2p/go-libp2p-rendezvous`, but that repo is **dead** — its `master`
  branch was stripped of all Go code (only CI/stale-bot commits remain), so `@latest`
  has no importable package; the only real code is a 2021-era `implement-spec` branch
  needing the deprecated `go-libp2p-core`. go-libp2p **itself is very actively
  maintained** (monthly releases, v0.48.0); the core protocols were consolidated into
  that monorepo (`p2p/protocol/{identify,ping,holepunch,circuitv2,autonatv2}`), but
  rendezvous — whose spec is still only a 2021 "Working Draft" — was never migrated and
  its satellite repo was orphaned. The actively maintained continuation is the **Waku
  fork** `github.com/waku-org/go-libp2p-rendezvous` (used in production by Waku/Status).
  NB: `go-waku-rendezvous` is a *different*, spec-incompatible variant (20s TTL) — do
  not use that one.
- **The fork's root package is cgo-free — import it directly.** An earlier note here
  claimed the root package pulls in `mattn/go-sqlite3` (cgo); that was **verified
  wrong** on 2026-07-09 (`go list -deps` shows no sqlite, no `database/sql`; the whole
  client+server built and ran with `CGO_ENABLED=0` against go-libp2p v0.48.0). The cgo
  sqlite/sqlcipher drivers live in the `db/sqlite` / `db/sqlcipher` subpackages —
  simply never import those. The VPS daemon uses a hand-written in-memory
  implementation of the fork's storage interface (plan WP-07).
- **Fork risk is pre-decided.** If the fork ever fails the WP-06 opening spike, the
  pre-approved contingency in plan decision T-12 applies (v1 ships DHT+mDNS discovery;
  VPS stays relay-only) — no session should stall on this.
- **Never add `replace` directives to go.mod** — they break
  `go install github.com/Mediacom99/pchat/cmd/pchat@latest` (verified; the launch
  checklist depends on that path). The fork resolves under its own module path and
  needs none.

## Common commands

Run via the Makefile (`make help` lists targets):

- `make build` — compile the `pchat` binary (`go build -o pchat ./cmd/pchat`).
- `make run ARGS="--room foo"` — build and run.
- `make test` — `go test ./...`.
- `make tidy` — `go mod tidy`.
- `make lint` — `golangci-lint run` (config in `.golangci.yml`).

CI (`.github/workflows/ci.yml`) runs build, vet, test, and golangci-lint on push and PR.

## Documentation

- [`docs/pchat-architecture.md`](docs/pchat-architecture.md) — **source of truth for
  design intent**: architecture, wire protocol, security model.
- [`docs/pchat-implementation-plan.md`](docs/pchat-implementation-plan.md) — **source
  of truth for execution**: work packages, dependency graph, decision log (maintainer
  verdicts live here), risk register, session protocol + kickoff prompt.
- [`docs/pchat-launch-checklist.md`](docs/pchat-launch-checklist.md) — launch checklist
  (reference).
- [`docs/pchat-overview.html`](docs/pchat-overview.html) — the maintainer's
  single-page, self-contained reading copy of all the docs above (open locally in a
  browser). **Generated — never edit by hand.** After editing ANY markdown doc in
  this list (or README.md / this file), run `make overview` and commit the
  regenerated HTML in the same commit. Generator: `docs/tools/build-overview.py`
  (stdlib-only; renders via the vendored `docs/tools/marked.min.js`).
