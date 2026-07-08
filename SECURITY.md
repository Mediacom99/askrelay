# Security Policy

## Supported Versions

pchat is pre-release and scaffolded only; there is no stable release yet. Security
fixes will be applied to the latest `main` and, once releases begin, to the most
recent minor version.

| Version | Supported |
| ------- | --------- |
| `main` (unreleased) | ✅ |
| tagged releases | latest minor only, once they exist |

## Reporting a Vulnerability

**Please do not open a public issue for security vulnerabilities.**

Report privately using GitHub's
[private vulnerability reporting](https://github.com/Mediacom99/pchat/security/advisories/new)
("Report a vulnerability" under the repository's **Security** tab). If that is
unavailable, contact the maintainer through the address listed on the
[Mediacom99 GitHub profile](https://github.com/Mediacom99).

When reporting, please include:

- a description of the issue and its impact,
- steps to reproduce (or a proof of concept), and
- affected version / commit.

You can expect an initial acknowledgement within a few days. Please allow a
reasonable period for a fix before any public disclosure (coordinated disclosure).

## Why security reports matter here

pchat is a peer-to-peer chat tool that handles **cryptographic identity and message
signing**: per-session Ed25519 keypairs, libp2p peer identity, and Ed25519 signatures
over every wire message (see [`docs/pchat-architecture.md`](docs/pchat-architecture.md)
§2 Identity, §4 Wire Protocol, and §7 Security & Threat Model). Bugs in these paths —
identity derivation, signing/verification, replay handling, or the CBOR decode limits —
can have real privacy and impersonation consequences for users, so such reports are
taken seriously.

Note that some limitations are **known and documented, not vulnerabilities**, in
`docs/pchat-architecture.md` §7 and §13 — for example: IP-address exposure via DHT
provider records for weak passphrases, the absence of proof-of-work Sybil resistance
in v1, and no protection against a participant who locally logs a room they are in.
Please review those sections before reporting.
