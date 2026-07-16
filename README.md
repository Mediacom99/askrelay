# askrelay

**Your AI can ask my AI.**

The self-hosted, cross-vendor messaging layer between AI sessions — where
human approval is the product, not a checkbox. askrelay works **across
vendors** (Claude Code ↔ ChatGPT ↔ Claude ↔ Codex), **across organizations**,
and **on your own infrastructure**. And it reaches what no shared channel bot
or org assistant can: the *live local context* of your colleague's session —
uncommitted code, terminal state, private repos — because their own AI
answers, in their own session, with their explicit approval.

Concretely: your Claude Code session hits a question only your colleague can
answer ("why does the auth service special-case tenant IDs?"). Instead of you
writing a Slack message, your session sends the question to your colleague
*as a person*. It waits in their inbox. They tap approve; their AI — with
their code, their context — drafts the answer; they review it; your session
gets it. Nobody copy-pastes anything.

> **Status: pre-implementation.** The design is fully decided, researched, and
> documented; implementation follows the plan below, work package by work
> package. Nothing is runnable yet — watch the repo if you want the first
> release.
>
> | Document | What it is |
> |---|---|
> | [Architecture](docs/askrelay-architecture.md) | design source of truth |
> | [Implementation plan](docs/askrelay-implementation-plan.md) | work packages, verified dependency pins, risks |
> | [Decision log](docs/phase2-decision-log.md) | every product decision, with verdicts |
> | [Research briefs](docs/research/README.md) | live-verified evidence (MCP, A2A, ChatGPT/Claude surfaces, security incidents, prior art) |

## How it works

```
 you (asker)                      relay                     colleague (answerer)
 Claude Code ── send_message ──► inbox ── push/pull ──► their session
                                   │                        │ approve ✓
 your session ◄── reply ────────── ◄──── review ✓ ────  their AI answers
```

- **It's a mailbox, not a chat room.** Messages address a *person* and wait
  durably; delivery is honest per client — instant push into Claude Code (via
  the optional daemon), "next time they prompt" on claude.ai/ChatGPT, because
  those platforms are pull-only for external systems. We document that instead
  of pretending otherwise.
- **Approval in both directions.** Inbound messages are untrusted data until
  the recipient approves them; outgoing AI-drafted replies are reviewed before
  they leave. For an ongoing exchange, either side can grant auto-approval on
  just that thread — revocable, and every grant-derived action is logged.
- **Also works solo, across your machines.** Your own agents on the laptop,
  the desktop, and the homelab can message each other through the same inbox —
  same primitives; sender and recipient just happen to be you.
- **One small relay, self-hosted.** A single Go binary with a SQLite file.
  It speaks MCP directly over HTTPS, so claude.ai and ChatGPT connect with
  zero local install; an optional daemon upgrades Claude Code with push and
  in-terminal approvals. Your messages live on your infrastructure — and only
  briefly: the relay deletes them after delivery.

## What connects, honestly

| Client | Ask | Answer | How fast they see your question |
|---|---|---|---|
| Claude Code (+ daemon) | ✅ | ✅ | seconds (push) or next prompt (hooks) |
| Claude Code (remote-only) | ✅ | ✅ | on inbox check / long-poll |
| Codex CLI (via the local daemon) | ✅ | ✅ | on next tool call |
| claude.ai / Claude Desktop | ✅ | ✅ | next time they prompt |
| ChatGPT (web, connector) | ✅ | ✅ | next time they prompt; plan-gating applies |
| ChatGPT mobile | ❌ | ❌ | no custom connectors |

The asymmetry is a platform fact (see the [research](docs/research/README.md)),
not a bug: askrelay is designed async-first so "answer whenever your colleague
is back" is the product, not a failure mode.

## Security is the headline feature

Cross-person AI messaging is a textbook prompt-injection and data-leakage
surface — the [security brief](docs/research/security.md) walks the 2025–26
incident history that shaped this design. askrelay's answers, by construction:

- Inbound content is **data**: rendered in tamper-evident quarantine blocks,
  never able to trigger tools, never auto-fetched (links stay plain text).
- **Ed25519-signed envelopes** from per-device keys — origin is verifiable,
  and verification never implies trust: the gates apply to everyone.
- **Built-in secret redaction** on outgoing messages (cloud keys, tokens, PEM
  blocks, `.env` lines) with visible markers, plus a hook for your patterns.
- **Ephemeral retention**: the relay deletes message bodies after delivery
  acknowledgment; approval/grant audit records outlive them.
- No ML "guardrail" classifiers giving false confidence — architectural
  controls and honest documentation instead.

The full threat model ships with the docs (see architecture §8).

## Self-hosting (the flagship path)

One binary + any HTTPS reverse proxy:

```
askrelay serve --db /data/askrelay.db --base-url https://relay.your.team
askrelay invite colleague@your.team      # send them the link
```

Colleagues on claude.ai/ChatGPT paste the relay URL as a custom connector;
Claude Code users optionally run `askrelay daemon` for push. (Commands are the
decided design — they go live with the first release.)

## Contributing

Apache-2.0, [DCO](https://developercertificate.org/) sign-off, **no CLA** —
and that's a promise, not a placeholder: the license doesn't move. The project
is built plan-first: work packages, dependency pins, and the session protocol
are in the [implementation plan](docs/askrelay-implementation-plan.md). Issues
and design discussion are welcome now; code PRs make sense once WP-01 (the
first work package — the signed envelope library) lands.

askrelay's first users are the team at [Kosmoy](https://www.kosmoy.com) —
built in the open from day one.
