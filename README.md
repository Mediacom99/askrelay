# askrelay

**Your AI can ask my AI.**

Async, approval-gated messaging between your team's AI sessions — Claude Code,
Claude, ChatGPT, Codex. Your Claude Code session sends a question to a
colleague; your colleague approves; their AI answers with its own context. No
human copy-paste in between.

> **Status: pre-implementation.** The design is fully decided and evidence-backed;
> the architecture and implementation-plan documents are being written now, and
> code follows them. Nothing is runnable yet.
>
> - [Decision log](docs/phase2-decision-log.md) — every design decision, with verdicts
> - [Research briefs](docs/research/README.md) — the live-verified evidence behind them

## What it will be

- **An async mailbox, not a chat room.** You address a *person*, not a machine.
  Messages land in a durable per-person inbox on a small relay and surface in
  whatever AI session that person attaches. Delivery is honest per client:
  instant push where the platform allows it (Claude Code), on next prompt where
  it doesn't (claude.ai, ChatGPT).
- **Human approval in both directions.** Inbound messages are untrusted data
  until the recipient approves them; outgoing AI-drafted replies are reviewed
  before they leave. Per-thread grants (revocable, logged) keep multi-turn
  exchanges fluid.
- **A thin, self-hostable Go relay** — single binary + SQLite — that speaks MCP
  directly, so claude.ai and ChatGPT connect with zero local install. An
  optional local daemon upgrades Claude Code with push and in-terminal
  approvals.
- **Security as a headline feature, not a caveat**: signed sender identity,
  spotlighting of inbound content, no auto-fetching of links or images from
  messages, built-in secret redaction, ephemeral relay retention. The threat
  model ships with the docs.

## License

Apache-2.0. Contributions will be accepted under DCO sign-off; there is no CLA.
