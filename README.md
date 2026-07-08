# pchat

A terminal-only, fully peer-to-peer, **ephemeral** chat tool for developers and internet
weirdos. No servers, no accounts, no logs — you exist in a room only while you're in it,
and nothing ever touches disk.

- **Ephemeral by default** — no history, no logs, no saved identity. Quitting loses
  everything, on purpose.
- **No central server** — rooms live only as long as a peer holding them is online.
  Messaging fans out over [libp2p](https://libp2p.io/) gossipsub.
- **Anonymous but distinguishable identity** — a fresh Ed25519 keypair per launch,
  rendered as a deterministic **glyph + color** signature. No nicknames — recognition is
  purely visual.
- **Rooms are just passphrases** — `topic_id = SHA-256(room_passphrase)`. No passphrase,
  no way to find or join a room.

> **Status: scaffolded only.** This repository currently contains the project structure,
> tooling, and placeholder packages — **no functionality is implemented yet.** See
> [`CLAUDE.md`](CLAUDE.md) for the build plan; `internal/identity` is the next step.

## Repository layout

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
  docs/                  # architecture spec + launch checklist
  go.mod
  README.md
```

## Build & run

Requires **Go 1.23+** (developed against Go 1.26). Common tasks go through the
[`Makefile`](Makefile) — run `make help` to list targets:

```sh
make build                 # compile the ./pchat binary
make run ARGS="--room foo"  # build and run (see planned flags below)
make test                  # go test ./...
make tidy                  # go mod tidy
make lint                  # golangci-lint run
```

Or directly:

```sh
go build -o pchat ./cmd/pchat
./pchat --room "some passphrase"
```

### Planned CLI (v1, not yet implemented)

```
pchat --room "some passphrase"   # required: derives the room topic
      [--ring-size 200]          # in-memory scrollback size
      [--relay-only]             # force relay, skip hole punching (debug)
      [--debug]                  # verbose logging to stderr only, never to disk
```

Identity is always fresh per launch — there are no identity or nickname flags by design.

### Sharing a room

The passphrase **is** the entire join mechanism, and sharing it is out-of-band by
design — send it however you'd send a Discord invite link (DM, pasted in chat, QR for a
local demo). Two people in the same room simply ran `pchat` with the same `--room` value.

> ⚠️ **Privacy note:** treat the passphrase like an invite link, not a password. Weak or
> common passphrases are effectively public rooms, and peer **IP addresses can be exposed**
> to anyone who knows or guesses the passphrase (via DHT lookups). Message *content* stays
> end-to-end encrypted, but network metadata does not. See the threat model in the docs
> below before using pchat for anything sensitive.

## Documentation

- [`docs/pchat-architecture.md`](docs/pchat-architecture.md) — the full architecture and
  design specification (identity, networking, wire protocol, UI, security & threat model,
  build order). **This is the source of truth for the project.**
- [`docs/pchat-launch-checklist.md`](docs/pchat-launch-checklist.md) — launch checklist
  and reference material.

## Contributing & security

- Development conventions and the build order live in [`CLAUDE.md`](CLAUDE.md).
- Security policy and how to report vulnerabilities: [`SECURITY.md`](SECURITY.md).

## License

[MIT](LICENSE) © 2026 Mediacom99
