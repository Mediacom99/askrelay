# Security policy

## Reporting a vulnerability

Use GitHub's private vulnerability reporting on this repository
(**Security → Report a vulnerability**). Please do not open public issues or
discussions for security reports. You'll get an acknowledgment within 72 hours
and a coordinated-disclosure timeline agreed with you (90 days by default).

For a tool whose product *is* the approval gate, a consent-bypass report is a
drop-everything event: we pre-commit to fast, non-dismissive handling —
validation over defensiveness, and credit to the reporter.

## Supported versions

Pre-release: no supported versions yet. From `v0.1.0` onward, the latest minor
release receives security fixes.

## Scope and posture

askrelay moves model-authored text between different people's AI sessions.
Prompt injection and data leakage are its central design constraints, not edge
cases. The architectural invariants (see
[architecture §8](docs/askrelay-architecture.md#8-security--threat-model)):

- Inbound messages are untrusted data: quarantine rendering, no tool
  triggering, human approval gates in both directions, no auto-fetching of
  message-referenced content — regardless of sender.
- Per-device Ed25519 signatures prove origin and never imply safety; devices
  are individually revocable.
- Client-side secret redaction with visible markers; the relay flags but never
  silently rewrites.
- The relay stores plaintext ephemerally (deleted after delivery-ack + grace);
  self-hosting inside your perimeter is the recommended deployment.

Reports that gate approvals, break the quarantine rendering, leak message
bodies past retention, or confuse device identity are exactly what we want to
hear about — the [security research brief](docs/research/security.md) documents
the incident class this project is designed against.
