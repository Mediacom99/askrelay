# Security policy

askrelay is pre-release: there are no supported versions and nothing runnable
yet. This policy will be replaced by a full one (with a published threat model)
before the first release.

## Reporting a vulnerability

Please use GitHub's private vulnerability reporting on this repository
(**Security → Report a vulnerability**). Do not open public issues for
security reports.

## Why security is the product's center

askrelay moves model-authored text between different people's AI sessions.
Prompt injection and data leakage are therefore its central design constraints,
not edge cases: inbound messages are treated as untrusted data behind a human
approval gate, outgoing AI-drafted replies are reviewed before sending, clients
never auto-fetch content referenced in messages, and the relay's retention is
ephemeral. The research this posture is built on — including the documented
2025–26 incidents in MCP-connected systems — is in
[`docs/research/security.md`](docs/research/security.md).
