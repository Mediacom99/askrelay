# Research briefs — pivot research sweep (2026-07-14)

Decision briefs feeding the Phase 2 deep dive. Method: one research agent per
topic, required to verify every claim against a live source fetched during the
sweep; an independent adversarial verifier then re-fetched and re-checked every
key fact (per-fact verdicts at the bottom of each brief). The ChatGPT question
was researched twice, independently (official docs vs. practitioner reports).

| Brief | Verification |
|---|---|
| [Model Context Protocol (MCP) and its ecosystem](mcp.md) | 13 confirmed |
| [A2A (Agent2Agent) protocol](a2a.md) | 11 confirmed |
| [OpenAI developer-platform interop offerings (Agents SDK, Responses API, AgentKit, Codex)](openai-interop.md) | 12 confirmed, 1 failed |
| [ChatGPT session extension points today (official docs)](chatgpt-extension.md) | 8 confirmed, 2 failed |
| [ChatGPT extension points - practitioner field reports](chatgpt-field.md) | 14 confirmed |
| [Existing tools connecting AI sessions or agents across machines and people](prior-art.md) | 11 confirmed |
| [Claude-side extension points (Claude Code, claude.ai, Agent SDK, Slack)](claude-extension.md) | 7 confirmed, 5 failed, 1 unverifiable |
| [Transport, delivery, and identity building blocks for cross-person messaging](transport.md) | 11 confirmed, 2 failed |
| [Security prior art: prompt injection and data leakage in inter-agent messaging](security.md) | 10 confirmed, 2 failed |
| [Stack fitness - Go vs TypeScript for this tool](stack-fitness.md) | 13 confirmed |
| [Open-source licensing and governance options](oss-licensing.md) | 12 confirmed, 1 failed |
| [Vendor terms-of-service and acceptable-use constraints on automated/unattended session participation](vendor-tos.md) | 14 confirmed |
| [How OSS developer/agent tools actually win adoption (distribution and launch evidence)](oss-adoption-dynamics.md) | 12 confirmed |

## Completeness critic

Coverage of the integration surfaces (MCP, A2A, OpenAI, ChatGPT official+field, Claude-side), transport/identity, prompt-injection security, and competitive prior art is strong and mutually consistent — no gaps flagged there beyond their own open questions. The three gaps above map to agenda items that currently have zero or thin research behind them: (1) licensing/governance is an explicit meeting decision with no brief at all; (2) vendor ToS/acceptable-use is a cross-cutting assumption every integration brief silently makes (that unattended participation via consumer sessions is allowed) and could invalidate the auto-reply scope decision; (3) the marketing agenda item has positioning covered but no distribution/adoption evidence. Deliberately not flagged: naming and Go-vs-TS (handled separately per the task), E2EE/IT-approval and delivery-semantics questions (already open questions in the transport brief), and approval-UX design (a design task, not a research gap).

Gap topics flagged and researched in a second round:

- **Open-source licensing and governance options** — Licensing and governance is an explicit agenda item at the decision meeting, and none of the nine briefs touches it. The choice (permissive vs. copyleft vs. source-available, CLA vs. DCO, trademark policy) is load-bearing for three other decisions: whether a vendor or SaaS can clone the hosted relay (architecture has a hosted-relay shape), whether target users' employers can legally adopt and contribute (first users are at one company), and OSS positioning credibility. Deciding this in the meeting without research means deciding on vibes.
- **Vendor terms-of-service and acceptable-use constraints on automated/unattended session participation** — The product's core loop has one person's AI answering another person's AI, possibly unattended (auto-reply mode). Several briefs brush against this (Codex plan-auth automation, Claude Code channels allowlist, ChatGPT Tasks) but nobody has read the actual Anthropic and OpenAI usage policies and consumer/commercial terms. If unattended operation of a consumer-plan ChatGPT or claude.ai session, or relaying model output between accounts, violates ToS, that constrains product scope, the auto-reply security decision, and which plans/personas we can promise support for — silently invalidating parts of every integration brief.
- **How OSS developer/agent tools actually win adoption (distribution and launch evidence)** — "OSS positioning and marketing" is on the meeting agenda, but the prior-art brief only covers competitive positioning (vs. Claude Tag, mcp_agent_mail) — not how comparable projects actually acquired users. Decisions like install-path priority (npx vs. docker vs. go install), whether a hosted demo relay is worth building, and launch sequencing would otherwise rest on assumption. Distribution choices also feed back into the tech-stack and architecture decisions (single-binary story vs. Workers deploy was flagged as a tiebreaker in the transport brief).

## Market research (2026-07-16) — demand & viability go/no-go

Second sweep, after the design docs landed: six evidence dimensions, same
live-source + adversarial-verification method, then a three-lens judge panel
(bull / bear / base-rate) and a synthesis. **The verdict document is
[market-verdict.md](market-verdict.md): GO_WITH_CHANGES**, with mandatory
pre-code homework, kill criteria, and a watch list.

| Brief | Verification |
|---|---|
| [Direct demand: who is asking for this?](market-demand-direct.md) | see per-item verdicts |
| [Indirect demand: adjacent-tool traction](market-demand-indirect.md) | see per-item verdicts |
| [The vendor clock](market-vendor-clock.md) | see per-item verdicts |
| [Market size](market-market-size.md) | see per-item verdicts |
| [Flagship fit: solo-maintainer base rates](market-flagship-fit.md) | see per-item verdicts |
| [Futures: adjacencies](market-futures.md) | see per-item verdicts |
