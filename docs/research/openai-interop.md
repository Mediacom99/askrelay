# OpenAI developer-platform interop offerings (Agents SDK, Responses API, AgentKit, Codex)

> Decision brief — research sweep of 2026-07-14. Every key fact cites a live source
> fetched that day; an independent verifier re-checked each one (verdicts below).

## TL;DR

OpenAI's entire developer-platform interop story has consolidated on MCP: the Agents SDK is an MCP client, the Responses API hosts remote MCP servers, ChatGPT Apps SDK apps ARE MCP servers, and Codex is both an MCP client (stdio + Streamable HTTP) and an MCP server (`codex mcp-server`). OpenAI has not adopted A2A, and nothing in its stack does cross-session or cross-user agent messaging — the new Responses multi-agent beta (spawn/send_message/wait) is explicitly single-request-scoped. Codex is the durable, headlessly-drivable integration point (codex exec, TS/Python SDKs, releases daily as of 2026-07-14); AgentKit is a cautionary tale, with Agent Builder killed 13 months after launch (shutdown 2026-11-30). For our tool: speak MCP and you reach every OpenAI surface that matters; the cross-person transport gap is ours to fill.

## What it is

Four distinct offerings. (1) Agents SDK: open-source orchestration frameworks in Python (v0.18.2, 2026-07-11) and TypeScript (v0.13.3, 2026-07-13) with agents, handoffs (in-process delegation between agent objects), sessions (persistent conversation memory), guardrails, and tracing; MCP client only — stdio, Streamable HTTP, deprecated SSE, or hosted execution via the Responses API; supports MCP tools and prompts, not resources. (2) Responses API: OpenAI's primary API, with a remote-MCP tool (any Streamable HTTP/SSE server, approval flow, no extra fees), eight built-in connectors (Gmail, Drive, Teams, SharePoint, etc.), background mode with webhooks, and a new multi-agent beta (GPT-5.6) giving hosted subagents mailbox-style send_message/wait_agent primitives — but only inside one API request. (3) AgentKit (DevDay, 2025-10-06): Agent Builder visual canvas (deprecated 2026-06-03, dead 2026-11-30), ChatKit embeddable chat UI (survives, now self-hosted-backend), Connector Registry (enterprise admin, beta). The Apps SDK — apps running inside ChatGPT — is explicitly built on MCP: an app is an MCP server returning tools plus embedded UI components. (4) Codex: the coding agent as CLI, IDE extension, and cloud, releasing multiple times daily (0.144.4, 2026-07-14); full MCP client with config shared across CLI/IDE/ChatGPT desktop app; runs as an MCP server exposing codex/codex-reply tools with thread continuity; headless via `codex exec` (JSON-schema output) and TS/Python Codex SDKs.

## Maturity

Mixed, and the variance is the story. Mature and fast-moving: Codex (latest 0.144.4 on 2026-07-14, several releases per day, 75K+ GitHub stars, MCP client+server shipped, docs actively maintained) and the Agents SDK (Python v0.18.2 on 2026-07-11, TS v0.13.3 on 2026-07-13, near-weekly releases, both open source under github.com/openai). GA and stable: Responses API MCP tool and connectors, platform webhooks (signed, 72h retry). Beta: Responses multi-agent (GPT-5.6 only, beta header required), Codex Python SDK, Connector Registry. Dead or dying: Agent Builder — launched 2025-10-06, deprecated 2026-06-03, shutdown 2026-11-30, with the OpenAI-hosted ChatKit backend path going away too per the community deprecation thread; a 13-month product lifetime that community reaction calls a serious trust problem. Governance: everything is unilateral OpenAI; no standards-body participation on agent-to-agent protocols (no A2A membership as of the LF's 150-org milestone press release), but genuine, deep adoption of Anthropic-originated MCP across all four offerings. One operational wart observed live: developers.openai.com/codex URLs now 308-redirect to learn.chatgpt.com, i.e., docs are mid-migration and links rot.

## What adopting it buys us

If our tool speaks MCP, we get the entire OpenAI surface for free on the receiving and sending side. Codex CLI/IDE and the ChatGPT desktop app can connect to our tool as a Streamable HTTP or stdio MCP server with zero OpenAI cooperation — that is the single highest-leverage fact for us. In the other direction, colleague B's Codex session is programmatically reachable: `codex mcp-server` lets our daemon drive a Codex thread (codex/codex-reply with threadId), and `codex exec --output-schema` or the Codex SDKs (TS GA, Python beta) let us run headless OpenAI-side agents in CI-like fashion under either ChatGPT-plan auth or an API key. Codex is included in every ChatGPT tier including Free (token-based limits since 2026-04-02), so no enterprise-plan gate blocks our first users. The Agents SDK gives anyone building a custom OpenAI-side participant a maintained, open-source MCP client with sessions and human-in-the-loop approvals built in — matching our 'B approves before answering' flow. Responses API webhooks (response.completed on background mode) give a crude async completion signal if we ever host OpenAI-side agents ourselves. And OpenAI's non-adoption of A2A plus all-in MCP commitment removes a hard protocol dilemma: MCP is the only interop layer that spans OpenAI, Anthropic, and Cursor-class tools today.

## What it costs us

Nothing in OpenAI's stack does what our product does, so 'adoption' means integration points, not a foundation — and each has costs. MCP is client-server tool-calling, not peer messaging: to make B's session answer A, we must invert the model (our server holds a mailbox; B's agent polls or is prompted to check), because no OpenAI surface accepts inbound push — webhooks are outbound-only job notifications, and the multi-agent beta's send_message/wait_agent mailbox is sealed inside a single Responses API request on GPT-5.6 only. Driving Codex remotely via codex mcp-server means running on B's machine with B's auth; unattended use requires sandbox and approval-policy loosening, a real security decision. The Codex SDKs are TS/Python only, so a Go core shells out to `codex exec` or speaks MCP-over-stdio instead (acceptable, not elegant). Platform churn is the big tax: Agent Builder died 13 months after launch, docs are mid-migration to learn.chatgpt.com, and the Agents SDK's MCP support omits resources and elicitation — anything we build against a beta (multi-agent, Python Codex SDK, Connector Registry) may be resurfaced or killed. Finally, ChatGPT Enterprise admin controls (Connector Registry) could let IT block our MCP server for managed workspaces — unverifiable from outside.

## Recommendation

Build the tool as an MCP server (Streamable HTTP for remote, stdio optional) and treat that as the sole OpenAI-side contract — it is the only interface every relevant OpenAI surface (Codex CLI, Codex IDE, ChatGPT desktop, Agents SDK apps) consumes today, and OpenAI's Apps SDK doubling down on MCP makes it the safest bet on their roadmap. Do not build on AgentKit, Agent Builder, ChatKit, or the Responses multi-agent beta: the first is a graveyard (Agent Builder dead 2026-11-30 after 13 months) and the last is single-request-scoped and beta-gated, useless for cross-person messaging. Ignore A2A for OpenAI compatibility — they have not touched it. Accept that OpenAI provides no inbound push: our tool must own the transport/mailbox layer, and OpenAI-side sessions participate by polling our MCP server's tools (or via a long-lived tool call), which is exactly the gap that justifies the product. For headless OpenAI-side participants, standardize on `codex exec --output-schema` and codex mcp-server rather than the Python Codex SDK (beta). Go stays viable as the implementation language — MCP servers are language-agnostic and Codex is driven via CLI/stdio — but note the reference SDK gravity is TS/Python. Re-verify the multi-agent beta's scope and ChatGPT Enterprise MCP admin controls before the decision meeting; both moved within the last quarter.

## Key facts

1. **OpenAI Agents SDK (Python) is at v0.18.2, released 2026-07-11, requiring Python >=3.10; release cadence is near-weekly.**
   - Source: <https://pypi.org/project/openai-agents/>
   - Verified: Fetched PyPI project page; showed 0.18.2 latest with 2026-07-11 release date, cross-checked against the GitHub releases page listing v0.17.7 through v0.18.2 over ~3 weeks.
2. **The Agents SDK is an MCP client only (stdio, Streamable HTTP, deprecated SSE, and hosted-via-Responses-API modes); it supports MCP tools and prompts but not resources or elicitation, and does not implement the server role.**
   - Source: <https://openai.github.io/openai-agents-python/mcp/>
   - Verified: Fetched the official MCP docs page; it enumerates the five integration patterns and covers only tools/prompts, with the SDK acting exclusively as client.
3. **The Agents SDK also exists in TypeScript: @openai/agents v0.13.3, published 2026-07-13.**
   - Source: <https://registry.npmjs.org/@openai/agents>
   - Verified: Queried the npm registry API via curl; dist-tags.latest = 0.13.3 with publish time 2026-07-13T23:35Z.
4. **The Responses API has a remote MCP tool (Streamable HTTP or HTTP/SSE servers) plus eight OpenAI-maintained connectors (Dropbox, Gmail, Google Calendar/Drive, Teams, Outlook Email/Calendar, SharePoint), with a default-on approval flow and no per-tool-call fees beyond tokens.**
   - Source: <https://developers.openai.com/api/docs/guides/tools-connectors-mcp>
   - Verified: Fetched the official guide; it documents mcp_list_tools/mcp_call items, require_approval settings, the connector list, and pricing.
5. **The Responses API now has a hosted multi-agent beta (GPT-5.6 models, responses_multi_agent=v1) with spawn_agent, send_message, wait_agent, interrupt_agent primitives — but it is explicitly scoped to a single API request and cannot span requests or users.**
   - Source: <https://developers.openai.com/api/docs/guides/responses-multi-agent>
   - Verified: Fetched the official multi-agent guide; it lists the six collaboration actions, beta header requirement, and single-request scope.
6. **Agent Builder (AgentKit's visual canvas, launched at DevDay 2025-10-06) was deprecated on 2026-06-03 and shuts down 2026-11-30 — a ~13-month product lifetime; ChatKit and the Agents SDK remain available.**
   - Source: <https://developers.openai.com/api/docs/guides/agent-builder>
   - Verified: Fetched the official Agent Builder docs page carrying the deprecation notice with both dates; launch date corroborated by TechCrunch (2025-10-06) and the OpenAI community deprecation thread.
7. **The Apps SDK for building apps that run inside ChatGPT is built on MCP: a ChatGPT app is an MCP server exposing tools plus embedded UI resources.**
   - Source: <https://developers.openai.com/apps-sdk/concepts/mcp-server>
   - Verified: Web search surfaced the official Apps SDK docs, which state the SDK 'builds on the Model Context Protocol' and that an MCP server is the required component of every app.
8. **Codex (CLI, IDE extension, and ChatGPT desktop app) is a full MCP client supporting stdio and Streamable HTTP servers (bearer token and OAuth auth), with MCP configuration shared across all three surfaces via ~/.codex/config.toml or `codex mcp add`.**
   - Source: <https://learn.chatgpt.com/docs/extend/mcp?surface=cli>
   - Verified: Fetched the official Codex MCP docs (developers.openai.com/codex/mcp 308-redirects here); it documents both transports, auth options, and the shared config.
9. **Codex can itself run as an MCP server via `codex mcp-server` (stdio), exposing `codex` and `codex-reply` tools with threadId-based session continuity, so external agents can drive a Codex session programmatically.**
   - Source: <https://learn.chatgpt.com/docs/mcp-server>
   - Verified: Fetched the official docs page (redirect target of developers.openai.com/codex/guides/agents-sdk); it shows the command, both tools, and an Agents SDK MCPServerStdio integration example.
10. **Codex is headlessly drivable two ways: `codex exec` non-interactive mode (read-only sandbox by default, --output-schema for JSON-schema-conforming output, CODEX_API_KEY recommended for automation) and a Codex SDK in TypeScript (@openai/codex-sdk) and Python (openai-codex, beta) that can start and resume threads.**
   - Source: <https://learn.chatgpt.com/docs/codex-sdk>
   - Verified: Fetched the official Codex SDK page (both languages, thread start/resume, Python beta status); codex exec details from the official non-interactive docs surfaced via search (developers.openai.com/codex/noninteractive).
11. **Codex is very actively maintained: latest stable release 0.144.4 on 2026-07-14, with multiple releases (stable + alpha) per day.**
   - Source: <https://github.com/openai/codex/releases>
   - Verified: Fetched the GitHub releases page; last five releases all dated 2026-07-13/14.
12. **OpenAI has not adopted or endorsed A2A: the Linux Foundation's A2A project (contributed by Google, launched 2025-06-23, 150+ supporting organizations including Microsoft, AWS, Salesforce, SAP) does not list OpenAI among its backers.**
   - Source: <https://www.linuxfoundation.org/press/a2a-protocol-surpasses-150-organizations-lands-in-major-cloud-platforms-and-sees-enterprise-production-use-in-first-year>
   - Verified: Searched LF press releases and the a2aproject GitHub org; OpenAI absent from every participant list, while OpenAI's own docs consistently standardize on MCP instead.
13. **OpenAI platform webhooks are outbound job-completion notifications only (response.completed, batch.completed, etc., signed, retried up to 72h) — there is no general push/messaging bus in the platform.**
   - Source: <https://developers.openai.com/api/docs/guides/webhooks>
   - Verified: Searched and reviewed the official webhooks guide and webhook-events API reference; event catalog covers background responses, batches, and fine-tuning jobs, nothing session-to-session.

## Open questions

- Is Evals also being wound down alongside Agent Builder? Community posts and third-party blogs say yes (same 2026-11-30 date); the official Agent Builder docs page names only Agent Builder — needs confirmation from an official OpenAI notice.
- Can ChatGPT Business/Enterprise admins block or allowlist MCP servers for Codex via the Connector Registry (beta, admin-console-gated)? Unverifiable without an Enterprise tenant; matters for whether corporate IT can silently break our tool for managed users.
- Does codex mcp-server support any transport other than stdio (e.g., Streamable HTTP listener)? Docs show stdio only, which forces our daemon to co-locate with B's machine.
- Are there rate/ToS constraints on driving Codex headlessly under ChatGPT-plan auth (vs. CODEX_API_KEY at API rates)? Docs recommend API keys for automation but don't state plan-auth automation is disallowed.
- What exactly is the Agents SDK v0.18.2 'hosted multi-agent beta support' surface, and is there any hint the request-scoped mailbox will ever span requests or users?
- The developers.openai.com -> learn.chatgpt.com docs migration was observed mid-flight (308 redirects); confirm canonical doc URLs before citing them in our own docs.
- Does the ChatGPT desktop app's shared MCP config mean a consumer-plan colleague gets our MCP server in ChatGPT with no dev-mode toggle, or is Developer Mode still required there? (Sibling ChatGPT brief should own this.)

## Independent verification

**Overall:** This brief is largely reliable: 12 of 13 key_facts checked out against their cited sources, with exact version numbers, dates, transports, connector lists, and deprecation dates all accurate as of 2026-07-15 (one immaterial drift: Codex has since released 0.145.0 alphas dated 07-15). The single FAILED fact is unfortunately the most strategically load-bearing one — the claim that the Responses multi-agent beta is 'explicitly scoped to a single API request' is directly contradicted by the cited guide, which states a run may span multiple API requests; the brief repeats this wrong framing in the tldr ('sealed inside a single Responses API request'), costs, and recommendation. The correct limitation (run-scoped, no documented cross-run/cross-user persistence) still supports the brief's core thesis that OpenAI offers no cross-person agent messaging, so the recommendation survives, but the 'single request' language should be corrected before the decision meeting and the beta re-examined since a multi-request run weakens the 'useless for messaging' dismissal somewhat. Secondary weaknesses: several persuasive prose claims (Codex on the Free tier since 2026-04-02, TS SDK GA status, ChatKit hosted-backend wind-down, Connector Registry) have no key_fact behind them, and the '~13-month' Agent Builder lifetime is really closer to 14. The A2A non-adoption and MCP-everywhere findings were independently corroborated and are solid.

Verdicts: CONFIRMED 12, FAILED 1

- ✅ **CONFIRMED** — Agents SDK (Python) v0.18.2, released 2026-07-11, Python >=3.10, near-weekly cadence
  - PyPI shows 0.18.2 latest, released Jul 11 2026, requires Python >=3.10 (supports 3.10-3.14). Release history (0.17.7 Jun 24 → 0.17.8 Jul 6 → 0.18.0 Jul 7 → 0.18.1 Jul 9 → 0.18.2 Jul 11) supports 'near-weekly' — recently even faster.
  - Evidence: <https://pypi.org/project/openai-agents/>
- ✅ **CONFIRMED** — Agents SDK is MCP client only (stdio, Streamable HTTP, deprecated SSE, hosted-via-Responses-API); supports MCP tools and prompts, not resources/elicitation; no server role
  - Docs enumerate exactly these integration patterns (plus a server-manager helper), explicitly say 'The MCP project has deprecated the Server-Sent Events transport,' cover tools and prompts only, and describe no server role. Caveat: 'not resources or elicitation' is verified by absence from the docs, not by an explicit statement of non-support.
  - Evidence: <https://openai.github.io/openai-agents-python/mcp/>
- ✅ **CONFIRMED** — TypeScript @openai/agents v0.13.3, published 2026-07-13
  - Registry API: dist-tags.latest = 0.13.3, time['0.13.3'] = 2026-07-13T23:35:05Z. Verified directly via curl against the npm registry.
  - Evidence: <https://registry.npmjs.org/@openai/agents>
- ✅ **CONFIRMED** — Responses API remote MCP tool (Streamable HTTP or HTTP/SSE) plus eight OpenAI-maintained connectors, default-on approval, no per-tool-call fees beyond tokens
  - Guide documents both transports verbatim, lists exactly the 8 named connectors (Dropbox, Gmail, Google Calendar, Google Drive, Teams, Outlook Email, Outlook Calendar, SharePoint), states approval is requested by default (require_approval configurable), and 'no additional fees per tool call' beyond tokens.
  - Evidence: <https://developers.openai.com/api/docs/guides/tools-connectors-mcp>
- ❌ **FAILED** — Responses multi-agent beta (GPT-5.6, responses_multi_agent=v1) with spawn_agent/send_message/wait_agent/interrupt_agent — explicitly scoped to a single API request and cannot span requests or users
  - Models, beta flag, and primitives confirmed; the single-request-scope framing — repeated in the tldr, costs, and recommendation — is the opposite of what the guide says. Also note the brief's key_fact names four primitives while the guide (and the brief's own how_verified) has six.
  - **Correction:** The beta exists as described (GPT-5.6 models, responses_multi_agent=v1 header/betas flag, six actions: spawn_agent, send_message, followup_task, wait_agent, interrupt_agent, list_agents), but the scope claim is wrong: the guide explicitly states 'A single Multi-agent run may span multiple Responses API requests' (e.g., when developer-defined function outputs are submitted in new HTTP requests or via WebSocket). The correct limitation is that agents are scoped to a single multi-agent RUN — nothing documented lets agents persist across separate runs/conversations or reach other users. The cross-user gap the brief relies on still holds, but 'sealed inside a single API request' is contradicted by the cited source.
  - Evidence: <https://developers.openai.com/api/docs/guides/responses-multi-agent>
- ✅ **CONFIRMED** — Agent Builder (launched DevDay 2025-10-06) deprecated 2026-06-03, shuts down 2026-11-30 (~13-month lifetime); ChatKit and Agents SDK remain
  - The docs page carries the deprecation notice with the Nov 30 2026 shutdown and says ChatKit remains available; the Jun 3 2026 notification date and the Oct 6 2025 DevDay/AgentKit launch are corroborated by the OpenAI community deprecation thread, openai.com/index/introducing-agentkit, and third-party coverage. Minor nit: Oct 6 2025 → Nov 30 2026 is closer to 14 months than 13. Third-party coverage (mcp.directory, therouter.ai) also reports the Evals platform was deprecated the same day — partially answering the brief's first open question.
  - Evidence: <https://developers.openai.com/api/docs/guides/agent-builder>
- ✅ **CONFIRMED** — Apps SDK is built on MCP: a ChatGPT app is an MCP server exposing tools plus embedded UI resources
  - Page states 'MCP is the backbone that keeps server, model, and UI in sync' and describes the minimal app as an MCP server listing/calling tools and optionally pointing to an embedded resource rendered in ChatGPT.
  - Evidence: <https://developers.openai.com/apps-sdk/concepts/mcp-server>
- ✅ **CONFIRMED** — Codex (CLI, IDE, ChatGPT desktop) is a full MCP client: stdio + Streamable HTTP, bearer/OAuth auth, config shared via ~/.codex/config.toml or codex mcp add
  - Official docs confirm all elements: 'The ChatGPT desktop app, Codex CLI, and IDE extension support MCP servers and share MCP configuration,' STDIO and Streamable HTTP transports, bearer-token and OAuth auth (plus ChatGPT-session auth for first-party servers), [mcp_servers.<name>] tables in ~/.codex/config.toml, and codex mcp add. The developers.openai.com→learn.chatgpt.com 308 redirect was observed live during this check too.
  - Evidence: <https://learn.chatgpt.com/docs/extend/mcp?surface=cli>
- ✅ **CONFIRMED** — Codex runs as an MCP server via codex mcp-server (stdio), exposing codex and codex-reply tools with threadId continuity
  - Docs show the codex mcp-server command over stdio, both tools (codex to start, codex-reply to continue via threadId from structuredContent.threadId), and an Agents SDK MCPServerStdio integration example.
  - Evidence: <https://learn.chatgpt.com/docs/mcp-server>
- ✅ **CONFIRMED** — Codex headless: codex exec (read-only sandbox default, --output-schema, CODEX_API_KEY for automation) plus Codex SDKs in TS (@openai/codex-sdk) and Python (openai-codex, beta) with thread start/resume
  - SDK page confirms both languages, Python explicitly beta ('While the Python SDK is in beta, pip install --pre openai-codex'), and thread start/resume in both. Non-interactive docs (learn.chatgpt.com/docs/non-interactive-mode, redirect target of developers.openai.com/codex/noninteractive) confirm codex exec, read-only sandbox by default, --output-schema, and API keys as 'the right default for automation' — with a caveat the brief omits: docs warn against job-level CODEX_API_KEY env vars, recommending per-invocation use.
  - Evidence: <https://learn.chatgpt.com/docs/codex-sdk>
- ✅ **CONFIRMED** — Codex latest stable 0.144.4 on 2026-07-14, multiple releases (stable + alpha) per day
  - Releases page shows 0.144.4 (stable) dated 2026-07-14, 0.144.2/0.144.3 on 07-13, and four 0.145.0-alpha prereleases on 07-14 alone (plus alpha.12 on 07-15) — 'multiple per day' holds.
  - Evidence: <https://github.com/openai/codex/releases>
- ✅ **CONFIRMED** — OpenAI has not adopted A2A; LF A2A project (contributed by Google, launched 2025-06-23, 150+ orgs incl. Microsoft, AWS, Salesforce, SAP) does not list OpenAI
  - The 150-org press release names AWS, Cisco, Google, IBM, Microsoft, Salesforce, SAP, ServiceNow — OpenAI appears nowhere. LF launch date 2025-06-23 independently confirmed by the original LF launch press release (Open Source Summit NA, Denver). Nuance: the '150 orgs in first year' anniversary counts from Google's April 2025 protocol announcement, not the June LF donation; the claim's framing is still accurate. Absence of adoption is inherently a negative claim, but no evidence of OpenAI A2A involvement surfaced in any cross-check.
  - Evidence: <https://www.linuxfoundation.org/press/a2a-protocol-surpasses-150-organizations-lands-in-major-cloud-platforms-and-sees-enterprise-production-use-in-first-year>
- ✅ **CONFIRMED** — Platform webhooks are outbound job-completion notifications only (response.completed, batch.completed, etc.), signed, retried up to 72h; no general push/messaging bus
  - Guide confirms signed webhooks (Standard Webhooks compatible), exponential-backoff retries 'for up to 72 hours,' and an event catalog limited to backend job events (background responses, batches, fine-tuning). Nothing resembling inbound push or session-to-session messaging is documented.
  - Evidence: <https://developers.openai.com/api/docs/guides/webhooks>

Load-bearing claims in the prose **without** a sourced key fact:

- ⚠️ Codex is included in every ChatGPT tier including Free with token-based limits 'since 2026-04-02' — load-bearing for 'no enterprise-plan gate blocks our first users' in what_adopting_buys_us, but has no key_fact and the specific date and Free-tier inclusion were never sourced.
- ⚠️ Codex has '75K+ GitHub stars' (maturity) — no key_fact; not checked against the repo.
- ⚠️ ChatKit is 'now self-hosted-backend' and 'the OpenAI-hosted ChatKit backend path is going away too per the community deprecation thread' (maturity) — the official Agent Builder page only says ChatKit remains available; the hosted-backend wind-down rests on an uncited community thread.
- ⚠️ The Connector Registry exists as an enterprise-admin beta (what_it_is/maturity) — no key_fact behind it; also load-bearing for the 'IT could block our MCP server' cost.
- ⚠️ The TypeScript Codex SDK is 'GA' (what_adopting_buys_us/recommendation contrast it with the beta Python SDK) — docs confirm Python is beta but were not shown to declare the TS SDK GA.
- ⚠️ 'MCP is the only interop layer that spans OpenAI, Anthropic, and Cursor-class tools today' (what_adopting_buys_us) — sweeping cross-vendor claim with no key_fact.
- ⚠️ Community reaction to the Agent Builder shutdown 'calls a serious trust problem' (maturity) — characterization with no cited source.
- ⚠️ Agents SDK feature set of handoffs, sessions, guardrails, tracing, and 'human-in-the-loop approvals built in' matching the 'B approves before answering' flow — plausible from docs but not backed by any key_fact.
