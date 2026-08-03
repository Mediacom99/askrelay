# Contributing to askrelay

Thanks for your interest. askrelay is Apache-2.0, built in the open, and takes
**no CLA** — see [GOVERNANCE.md](GOVERNANCE.md).

## Ground rules

- **DCO sign-off, not a CLA.** Every commit must carry a
  [Developer Certificate of Origin](https://developercertificate.org/) sign-off:
  ```sh
  git commit -s -m "your message"
  ```
  The `Signed-off-by:` line certifies you wrote the patch (or have the right to
  submit it). No copyright assignment is asked or accepted.
- **Be excellent to each other.** Assume good faith; keep discussion technical.

## Development

Requires **Go 1.26+**. `CGO_ENABLED=0` everywhere — the build stays a single
static binary with no cgo.

```sh
make build      # build ./askrelay
make test       # go test -race ./...
make lint       # golangci-lint
make tidy       # go mod tidy
make overview   # regenerate docs/askrelay-overview.html (see below)
make help       # list targets
```

CI (`.github/workflows/ci.yml`) runs build, `go vet`, tests, and
golangci-lint. Please make sure `make test` and `make lint` are green before
opening a PR.

### Conventions

- **Errors** — exported sentinel values matched with `errors.Is`; wrap to
  propagate (`fmt.Errorf("pkg: …: %w", err)`); return a zero value **or** an
  error, never both. The pure core (`internal/a2a`, `internal/envelope`,
  `internal/gate`) returns errors and never logs; only boundaries log.
- **Logging** — `log/slog` only. Never log message content, secrets, keys,
  tokens, or PII — ids and shapes only. Security refusals log at `Warn` without
  the offending content.
- **Inbound is untrusted data** — never let received content trigger tools or
  auto-fetch URLs. The quarantine/spotlighting invariant is load-bearing.
- Match the style of the surrounding code; keep diffs small and focused.

## How the project is built

askrelay is **plan-first**. The design is settled and documented before code:

- [`docs/askrelay-architecture.md`](docs/askrelay-architecture.md) — design.
- [`docs/askrelay-implementation-plan.md`](docs/askrelay-implementation-plan.md)
  — work packages (WP-xx), verified dependency pins (§3), technical decisions
  (T-xx), risks, and the learnings log (§7). **The plan wins over the
  architecture doc where they differ.**
- [`docs/phase2-decision-log.md`](docs/phase2-decision-log.md) — product
  decisions (D-xx) with verdicts, binding both.

Dependencies enter only at the plan's §3 pins, and only when a work package
first imports them — `go.mod` is kept minimal on purpose.

## Where to start

- **Design discussion & issues** are welcome any time.
- **Code**: the core through WP-08 has landed. Good entry points are the open
  work packages (WP-06 OAuth authorization server, WP-09 daemon, WP-11
  redaction) — check the implementation plan's status board for what's eligible
  (a WP is eligible when everything it depends on is DONE).

## Documentation upkeep

`docs/askrelay-overview.html` is generated — **never edit it by hand**. After
editing any tracked markdown doc (or `README.md`), run `make overview` and
commit the regenerated HTML in the same commit.
