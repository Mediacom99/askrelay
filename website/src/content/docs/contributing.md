---
title: Contributing
description: Build, test, and contribute to askrelay — and where the internal design docs live.
---

askrelay is open source (Apache-2.0) and welcomes contributions. It's a Go
project: one module, standard library first, five pinned dependencies.

## Build & test

Requires **Go 1.26+**. The shipped binary builds with `CGO_ENABLED=0` (a single
static binary); the test target uses cgo because the race detector needs it.

```sh
make build      # build ./askrelay
make test       # race-enabled test suite
make lint       # golangci-lint
make tidy       # go mod tidy
make help       # list targets
```

CI runs the static build, a cross-compile smoke, `go vet`, the race tests, and
golangci-lint. Make sure `make test` and `make lint` are green before opening a
pull request.

## Sign your commits (DCO)

Contributions are under the Developer Certificate of Origin — sign off each
commit:

```sh
git commit -s -m "your message"
```

No copyright assignment (CLA) is asked or accepted.

## Conventions

- **Errors:** exported sentinel values matched with `errors.Is`; wrap to
  propagate (`fmt.Errorf("pkg: …: %w", err)`); return a zero value **or** an
  error, never both. The pure core returns errors and never logs; only
  boundaries log.
- **Logging:** `log/slog` only — never log message content, secrets, keys,
  tokens, or PII; ids and shapes only.

## Design docs

The source-of-truth design lives in the repository's `docs/` directory:

- **`askrelay-architecture.md`** — the full design
- **`askrelay-implementation-plan.md`** — work packages, technical decisions,
  and the risk register
- **`phase2-decision-log.md`** — every product decision with its rationale
- **`askrelay-system-map.md`** — the network & architecture picture
- **`GOVERNANCE.md`**, **`SECURITY.md`** — project governance and disclosure

These docs, not this site, are authoritative for *why* the code is shaped the
way it is; this site documents *how to use and run it*.
