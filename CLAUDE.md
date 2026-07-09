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

## Current status

**Scaffolded only.** Every file under `internal/` and `cmd/` is a placeholder
(package declaration + TODO citing the architecture section). No business logic exists
yet. `cmd/pchat/tools.go` is a build-tagged dependency anchor (tag `tools`) that keeps
the core libraries in `go.mod` until real code imports them — delete it once they are.

**Next step:** implement `internal/identity` (build order step 1 — pure logic, no
networking, easy to test standalone). Do not start implementation until the human
confirms the scaffold.

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

## Suggested build order (architecture §10)

Work outward from pure logic to I/O — **identity → protocol → networking → UI**:

1. `internal/identity` — keypair gen + glyph/color derivation (pure, testable).
2. `internal/protocol` — Message struct, CBOR encode/decode, sign/verify.
3. `internal/p2p/host.go` — bare libp2p host that starts and gets a peer ID.
4. `internal/p2p/room.go` + `discovery.go` — topic derivation, mDNS, gossipsub;
   confirm two local processes can talk.
5. Add DHT discovery for cross-network joins.
6. `internal/ui/` — Bubbletea shell wired to p2p (input → publish; receive → append).
7. Polish: roster, typing indicators, ring-buffer eviction, `--debug` logging.

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

## Dependency notes

- **Rendezvous library is a maintained fork.** The spec (§3) names
  `github.com/libp2p/go-libp2p-rendezvous`, but that module is **dead upstream** — its
  `master` branch was stripped of all Go code, and `@latest` resolves to a commit with
  no importable package; the only real implementation left is 2019 pre-modules code on
  an `implement-spec` branch that needs the deprecated `go-libp2p-core`. We therefore
  depend on the actively maintained **Waku fork**
  `github.com/waku-org/go-libp2p-rendezvous` (tracks modern go-libp2p). Treat it as the
  drop-in replacement for the spec's named module.
- **Import the rendezvous *client* selectively.** The fork's root package also contains
  the server/registration side, which pulls in `mattn/go-sqlite3` (cgo). The pchat
  *client* only needs the rendezvous client; import the narrowest package that provides
  it so the client binary stays cgo-free and matches the "single static binary, no
  runtime deps" goal (§8). The VPS rendezvous/relay component is separate.

## Common commands

Run via the Makefile (`make help` lists targets):

- `make build` — compile the `pchat` binary (`go build -o pchat ./cmd/pchat`).
- `make run ARGS="--room foo"` — build and run.
- `make test` — `go test ./...`.
- `make tidy` — `go mod tidy`.
- `make lint` — `golangci-lint run` (config in `.golangci.yml`).

CI (`.github/workflows/ci.yml`) runs build, vet, test, and golangci-lint on push and PR.

## Documentation

- [`docs/pchat-architecture.md`](docs/pchat-architecture.md) — **source of truth** for
  architecture, wire protocol, security model, and build order.
- [`docs/pchat-launch-checklist.md`](docs/pchat-launch-checklist.md) — launch checklist
  (reference).
