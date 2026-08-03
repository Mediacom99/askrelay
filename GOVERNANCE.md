# Governance

askrelay is an early, maintainer-led open-source project. This document states
what you can rely on. It reflects decisions recorded in the
[decision log](docs/phase2-decision-log.md) (referenced as D-xx).

## Licensing promise

- **The relay core stays Apache-2.0.** Everything in this repository is licensed
  under [Apache-2.0](LICENSE) (D-13). The core relay — the thing you self-host —
  will not be relicensed to a source-available or proprietary license. If the
  project ever offers paid or hosted options, they are built *around* the core,
  never by closing it (D-20).
- **No CLA — ever.** Contributions are accepted under the
  [Developer Certificate of Origin](https://developercertificate.org/) with a
  `Signed-off-by:` line (D-13). No contributor is asked to assign copyright or
  grant relicensing rights. This is what makes the promise above credible: no
  single party holds the rights needed to close the core.

## Decision-making

- The project is **maintainer-led** today. The maintainer reviews and merges
  changes; substantive product and technical decisions are written down, with
  rationale and a verdict, in the [decision log](docs/phase2-decision-log.md)
  (D-xx) and the [implementation plan](docs/askrelay-implementation-plan.md)
  (technical decisions T-xx, learnings §7).
- Design happens in the open — issues and discussion are the place to disagree
  before code. When a decision changes, the log is updated rather than quietly
  overwritten.

## Stewardship

- The repository lives on a personal account for now and will move to a neutral
  `askrelay` organization once the project has real adoption (D-14). The
  licensing and no-CLA promises above travel with it.
- askrelay's first users are the team at [Kosmoy](https://www.kosmoy.com); the
  project is built in the open from day one, not opened up after the fact.

## Security

Security response is a top operating commitment. To report a vulnerability, see
[SECURITY.md](SECURITY.md) — please do not open a public issue for undisclosed
vulnerabilities.
