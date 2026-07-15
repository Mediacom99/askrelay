# A2A (Agent2Agent) protocol — governance, spec status, adoption, and fit for cross-person AI-session messaging

> Decision brief — research sweep of 2026-07-14. Every key fact cites a live source
> fetched that day; an independent verifier re-checked each one (verdicts below).

## TL;DR

A2A is now a Linux Foundation project (contributed by Google) with a stable v1.0 spec (v1.0.0 March 12, 2026; v1.0.1 May 28, 2026), active SDKs in 6 languages including a solid Go SDK, and it absorbed IBM's ACP in September 2025 — it has won the agent-interop protocol race. But its architecture assumes agents are long-lived HTTP services with reachable endpoints (Agent Card at a URL, client→server request/response, webhooks for push), which is the opposite of our ephemeral laptop sessions behind NAT owned by different people. Recommendation: do not adopt A2A as our transport/discovery layer, but strongly consider borrowing its task/message/artifact data model and task lifecycle (especially input-required/auth-required/rejected states, which map exactly onto "B approves before answering"), and keep a future A2A-bridge in mind since every major agent platform is standardizing on it.

## What it is

An open protocol for communication between opaque agentic applications, originally announced by Google (April 2025), now governed as an open-source project under the Linux Foundation (Apache 2.0, github.com/a2aproject/A2A, 24.8k stars, 163 contributors). Core model: an agent publishes an Agent Card (JSON metadata: identity, skills, endpoints, auth requirements) for discovery; clients call SendMessage/SendStreamingMessage to create Tasks; Tasks move through a defined lifecycle (submitted → working → completed/failed/canceled/rejected, plus interrupted states input-required and auth-required); Messages carry role + Parts (text, bytes, URLs, structured data); outputs are Artifacts composed of Parts. Three official protocol bindings, all HTTP-family: JSON-RPC 2.0 over HTTP, gRPC (with server streaming), and REST (HTTP+JSON), with SSE streaming and webhook push notifications as update channels alongside polling. Auth is standard web-service fare declared in the Agent Card: API keys, HTTP Basic/Bearer, OAuth 2.0 (v1.0 modernized to device-code/PKCE, dropped implicit/password), OIDC, mutual TLS; v1.0 added multi-tenancy and Agent Card signature validation. Its relationship to MCP is explicitly complementary (spec Appendix B): MCP is model-to-tool, A2A is agent-to-agent.

## Maturity

Mature and consolidating. Spec: v1.0.0 released March 12, 2026 (first stable, with breaking changes and a v0.3→v1.0 migration path); v1.0.1 May 28, 2026 is latest. Governance: Linux Foundation (LF AI & Data), contributed by Google. SDKs (all under a2aproject org): Python a2a-sdk v1.1.0 (May 29, 2026, steady release cadence since v1.0.0 April 20, 2026); Go github.com/a2aproject/a2a-go/v2 v2.3.1 (May 13, 2026; 26 releases, requires Go 1.25+, client+server, all three transports); JS v1.0.0-beta.0 (July 1, 2026) with v0.3.x still patched (v0.3.14 July 9, 2026) — JS is the laggard, still pre-GA on v1.0; Java v1.1.0.Final shipped with full 1.0 support; .NET and Rust also listed. Ecosystem consolidation: IBM's ACP/BeeAI protocol formally merged into A2A under LF (announced Aug–Sep 2025); BeeAI now uses A2A natively, so the 2025 rival is dead as an independent protocol. Adoption: LF's April 9, 2026 one-year press release claims 150+ supporting orgs and production deployments; the concrete, verifiable integrations are platform-level — Azure AI Foundry and Copilot Studio (Microsoft), Amazon Bedrock AgentCore Runtime (AWS), Google Cloud. No named end-customer production case studies appear in the press release itself; independent commentary flags exactly this support-vs-usage gap.

## What adopting it buys us

A ready-made, well-reviewed vocabulary for exactly our interaction shape: an asynchronous task with streaming updates, whose lifecycle already includes the states we need — input-required (agent paused awaiting a human), auth-required, and rejected (agent/owner declines) — which is precisely the "B's session answers, possibly after B approves" flow. The Message/Part/Artifact structure handles text, files, and structured data without us inventing a envelope format, and it is protobuf-defined, so the data model can be reused over any transport we choose, not just HTTP. Adopting the semantics (even partially) buys future interoperability: Azure AI Foundry, Copilot Studio, Bedrock AgentCore, and Google Cloud all speak A2A, and ACP's merger means there is no credible rival vocabulary to bet on instead. If our tool ever grows a bridge mode ("expose this session as an A2A endpoint" or "call an enterprise A2A agent"), having isomorphic internal semantics makes that a thin adapter rather than a rewrite. The Go SDK (v2.3.1, active) also means Go — our challengeable default — is a first-class A2A language if we go that route.

## What it costs us

A2A's deployment model fundamentally mismatches ours. It assumes the responding agent is an addressable, long-lived HTTP(S) service: discovery is fetching an Agent Card from a URL, all three transports are client→server over HTTP, and even push notifications require the CLIENT to expose a webhook endpoint — so in a laptop-to-laptop scenario both sides need public reachability. It has no rendezvous, relay, NAT traversal, or offline-delivery story whatsoever; we would have to build all of that anyway (tunnels, a relay server, or P2P), at which point A2A's transport bindings give us nothing. Its auth model (OAuth/API keys/mTLS declared in Agent Cards) presumes an org-managed service identity, not "two colleagues' ephemeral sessions with per-person consent." Full spec compliance also drags in machinery we don't need: multi-tenancy, extended agent cards, push-notification config CRUD, three parallel transport bindings. Neither Claude Code, claude.ai, ChatGPT, nor Codex ships as an A2A agent today — every session must be wrapped by our tool regardless, so A2A compliance buys zero out-of-the-box interop with our actual first users' tools. Finally, verified named production usage beyond cloud-platform integrations is thin; the protocol is real but the "150 organizations" figure is largely logo-tier.

## Recommendation

Do not build on A2A as our protocol stack — its endpoint/discovery/auth model assumes exactly what we don't have (long-lived, publicly addressable, org-owned services), and none of our users' AI tools natively speak it, so compliance buys no immediate interop while forcing us to solve NAT/rendezvous ourselves anyway. Instead, steal its semantics deliberately: model our cross-session exchanges as A2A-shaped Tasks with Messages (role + Parts) and Artifacts, and adopt its task lifecycle verbatim — submitted/working/completed/failed/canceled/rejected plus input-required and auth-required — because "input-required" and "rejected" are precisely the human-approval gate at the heart of our product. Keep field names and state names aligned with A2A v1.0 so a future bridge (exposing a session as an A2A endpoint for enterprise agents, or calling A2A services from a session) is a thin adapter. This is low-cost insurance: A2A has clearly won the interop-protocol consolidation (ACP merged into it; Microsoft/AWS/Google platforms integrated it), so alignment protects us if the ecosystem matures, while our own transport (relay/P2P, our own identity and consent model) does the work A2A cannot. Revisit as a first-class transport only if Claude/ChatGPT-class products ever expose A2A endpoints for user sessions — no sign of that today.

## Key facts

1. **A2A is an open-source project under the Linux Foundation, contributed by Google; Apache 2.0; repo has 24.8k stars, 163 contributors, 588 commits on main.**
   - Source: <https://github.com/a2aproject/A2A>
   - Verified: Fetched the GitHub repo README/page; it states LF governance, license, and shows the community metrics.
2. **Current spec is v1.0.1 (May 28, 2026); v1.0.0 (first stable) was released March 12, 2026 with breaking changes from v0.3 including OAuth modernization (device code/PKCE, implicit/password removed) and multi-tenancy.**
   - Source: <https://github.com/a2aproject/A2A/releases>
   - Verified: Fetched the releases page; listed v1.0.1 (2026-05-28, latest), v1.0.0 (2026-03-12), and v1.0.0 changelog highlights.
3. **Core model: Agent Cards for discovery/auth declaration; Tasks with lifecycle states submitted/working/completed/failed/canceled/rejected plus interrupted states input-required and auth-required; Messages of Parts; Artifacts as outputs; operations SendMessage, SendStreamingMessage, GetTask, ListTasks, CancelTask, SubscribeToTask.**
   - Source: <https://a2a-protocol.org/latest/specification/>
   - Verified: Fetched the official spec; extracted the enumerated task states, core objects, and method list.
4. **All three official transports are HTTP-family (JSON-RPC 2.0/HTTP, gRPC, REST+JSON with SSE), and push notifications require the client to register a webhook endpoint the server POSTs to — i.e., both directions assume reachable HTTP endpoints.**
   - Source: <https://a2a-protocol.org/latest/specification/>
   - Verified: Fetched the spec's Protocol Bindings and Task Update Delivery sections; polling/streaming/webhook are the only update mechanisms, all HTTP-based.
5. **Auth model is web-service style declared in the Agent Card: API key, HTTP Basic/Bearer, OAuth 2.0 (authorization code, client credentials, device code), OIDC, mutual TLS; in-task auth via TASK_STATE_AUTH_REQUIRED.**
   - Source: <https://a2a-protocol.org/latest/specification/>
   - Verified: Fetched the spec's Authentication & Authorization section listing these schemes.
6. **Go SDK github.com/a2aproject/a2a-go/v2 is active and v1.0-compliant: v2.3.1 released May 13, 2026, 26 releases, client+server across gRPC/REST/JSON-RPC, requires Go 1.25+.**
   - Source: <https://github.com/a2aproject/a2a-go>
   - Verified: Fetched the repo page; README states A2A v1.0 compliance and metrics; latest release v2.3.1 dated 2026-05-13.
7. **Python SDK is stable and actively released: a2a-sdk v1.0.0 on April 20, 2026 (with v0.3→v1.0 migration guide), latest v1.1.0 on May 29, 2026.**
   - Source: <https://github.com/a2aproject/a2a-python/releases>
   - Verified: Fetched the releases page; versions and dates listed there.
8. **JS SDK lags: v1.0 is still pre-GA (v1.0.0-beta.0, July 1, 2026) while the v0.3.x line is still being patched (v0.3.14, July 9, 2026).**
   - Source: <https://github.com/a2aproject/a2a-js/releases>
   - Verified: Fetched the releases page; beta/alpha v1.0 releases and current v0.3.14 listed with dates.
9. **IBM's ACP (BeeAI) formally merged into A2A under the Linux Foundation (announced Aug 29, 2025 on the LF AI & Data blog); BeeAI now uses A2A natively — the main 2025 rival protocol is dead as an independent effort.**
   - Source: <https://lfaidata.foundation/communityblog/2025/08/29/acp-joins-forces-with-a2a-under-the-linux-foundations-lf-ai-data/>
   - Verified: Web search surfaced the LF AI & Data blog post and IBM/i-am-bee discussion confirming the merger and BeeAI's switch to A2A.
10. **LF's one-year press release (April 9, 2026) claims 150+ supporting organizations and 'production deployments across multiple industries', but names zero end-customer deployments; the only concrete integrations are platform-level: Azure AI Foundry, Copilot Studio, Amazon Bedrock AgentCore Runtime, Google Cloud.**
   - Source: <https://www.linuxfoundation.org/press/a2a-protocol-surpasses-150-organizations-lands-in-major-cloud-platforms-and-sees-enterprise-production-use-in-first-year>
   - Verified: Fetched the press release and specifically checked for named production users vs. supporter logos; none named, platform integrations were.
11. **A2A and MCP are positioned as complementary: MCP handles model-to-tool interactions, A2A handles agent-to-agent communication (spec Appendix B).**
   - Source: <https://a2a-protocol.org/latest/specification/>
   - Verified: Fetched the spec, which contains 'Appendix B: Relationship to MCP' framing them as complementary layers.

## Open questions

- No named end-customer production deployments could be verified — LF cites industries, not companies; whether anyone runs A2A across real organizational boundaries (vs. inside one platform) is unconfirmed.
- Java SDK release dates could not be reliably verified (the fetched page rendered inconsistent dates); version line (v1.1.0.Final, full 1.0 support) is confirmed but its cadence is not.
- Whether any coding-agent product (Claude Code, Codex, Cursor) has an A2A bridge on its roadmap — nothing found today, but this would change the bridge calculus.
- How stable the v1.x data model will be (v1.0 broke v0.3 significantly); if we mirror A2A field/state names, we inherit their deprecation churn.
- Whether the A2A community has any draft work on non-HTTP transports or peer-to-peer profiles that would change the fit assessment (none found in the v1.0.1 spec).

## Independent verification

**Overall:** This brief is unusually reliable: all 11 key_facts were CONFIRMED against their cited primary sources, with exact matches on every version number and date I could check (spec v1.0.0 2026-03-12 and v1.0.1 2026-05-28; Go v2.3.1 2026-05-13 with 26 releases and Go 1.25+; Python v1.0.0 2026-04-20 / v1.1.0 2026-05-29; JS v1.0.0-beta.0 2026-07-01 and v0.3.14 2026-07-09; ACP merger blog 2025-08-29; LF press release 2026-04-09), and the load-bearing ones survived independent cross-checks (GitHub API, i-am-bee/IBM confirmations of the ACP merger, the v1.0 announcement page). The only factual blemishes are trivial repo-metric drift (GitHub API reports ~151 contributors vs the claimed 163; 589 vs 588 commits), and the brief correctly resists third-party blog noise (one blog wrongly dates v1.0 to April 2026 and invents a 'v1.2'; the brief matches the authoritative releases page). It is also commendably honest about its own gaps (JS SDK lag, no named end customers, unverified Java cadence). The residual risk is concentrated in the missed_claims: the recommendation's pivotal negative ('none of our users' AI tools speak A2A today') and the transport-independence-of-the-protobuf-model premise are unsourced, and 'won the protocol race' extrapolates from the ACP merger alone without surveying other rivals. Treat the descriptive facts as solid and the strategic framing as well-reasoned but resting on a few uncited assertions worth a spot-check before Phase 2 decisions.

Verdicts: CONFIRMED 11

- ✅ **CONFIRMED** — A2A is an LF open-source project contributed by Google; Apache 2.0; 24.8k stars, 163 contributors, 588 commits on main
  - Repo page confirms Linux Foundation governance, Apache-2.0, 24.8k stars (GitHub API: 24,790) and 589 commits on main (drift of 1 since the brief). Only wrinkle: the contributors API (including anonymous) reports 151, not 163; the repo page fetch did not display a contributor count. Non-load-bearing metric drift/discrepancy, everything substantive checks out.
  - Evidence: <https://github.com/a2aproject/A2A>
- ✅ **CONFIRMED** — Spec v1.0.1 (May 28, 2026) is latest; v1.0.0 (first stable) released March 12, 2026 with breaking changes incl. OAuth modernization (device code/PKCE added, implicit/password removed) and multi-tenancy
  - Releases page shows exactly v1.0.1 (2026-05-28, latest) and v1.0.0 (2026-03-12); v1.0.0 changelog literally says 'modernize oauth 2.0 flows - remove implicit/password, add device code / PKCE' and 'Natively Support Multi-tenancy on gRPC'. One third-party blog surfaced by search claims v1.0 shipped in April 2026 and a 'v1.2' exists; the authoritative GitHub releases page contradicts that — the brief matches the primary source.
  - Evidence: <https://github.com/a2aproject/A2A/releases>
- ✅ **CONFIRMED** — Core model: Agent Cards; Task lifecycle submitted/working/completed/failed/canceled/rejected + interrupted input-required and auth-required; Messages of Parts; Artifacts; operations SendMessage, SendStreamingMessage, GetTask, ListTasks, CancelTask, SubscribeToTask
  - Spec enumerates exactly TASK_STATE_SUBMITTED/WORKING/COMPLETED/FAILED/CANCELED/REJECTED plus interrupted TASK_STATE_INPUT_REQUIRED and TASK_STATE_AUTH_REQUIRED (plus UNSPECIFIED); AgentCard/Task/Message/Part/Artifact and all six listed operations confirmed (spec additionally has push-notification-config CRUD and Get Extended Agent Card, which the brief's cost section acknowledges).
  - Evidence: <https://a2a-protocol.org/latest/specification/>
- ✅ **CONFIRMED** — All three official transports are HTTP-family (JSON-RPC 2.0/HTTP, gRPC, REST+JSON with SSE); push notifications require the client to expose a webhook endpoint the server POSTs to
  - Spec defines exactly three bindings — JSON-RPC 2.0 (§9), gRPC (§10), HTTP+JSON/REST (§11) — with SSE and webhook push as the only update channels beyond polling. gRPC runs over HTTP/2, so 'all HTTP-family' is accurate; the webhook mechanism does require a client-reachable HTTP endpoint. The architectural inference (both directions assume reachable HTTP endpoints) follows from the spec.
  - Evidence: <https://a2a-protocol.org/latest/specification/>
- ✅ **CONFIRMED** — Auth is web-service style declared in the Agent Card: API key, HTTP Basic/Bearer, OAuth 2.0 (authorization code, client credentials, device code), OIDC, mutual TLS; in-task auth via TASK_STATE_AUTH_REQUIRED
  - Spec's auth section lists exactly these schemes and flows, and TASK_STATE_AUTH_REQUIRED is confirmed as an interrupted state.
  - Evidence: <https://a2a-protocol.org/latest/specification/>
- ✅ **CONFIRMED** — Go SDK github.com/a2aproject/a2a-go/v2 is active and v1.0-compliant: v2.3.1 (May 13, 2026), 26 releases, client+server across gRPC/REST/JSON-RPC, Go 1.25+
  - Repo confirms module path /v2, v2.3.1 dated 2026-05-13, 26 releases, Go 1.25.0+, a2asrv (server) + a2aclient (client), all three protocol bindings, and an explicit A2A v1.0 spec compliance claim in the README.
  - Evidence: <https://github.com/a2aproject/a2a-go>
- ✅ **CONFIRMED** — Python SDK: a2a-sdk v1.0.0 on April 20, 2026 with v0.3→v1.0 migration guide; latest v1.1.0 on May 29, 2026
  - Releases page shows v1.0.0 released 20 Apr 2026 with an explicit 'v0.3 → v1.0 migration guide' link, and v1.1.0 (29 May 2026) as latest with nothing newer.
  - Evidence: <https://github.com/a2aproject/a2a-python/releases>
- ✅ **CONFIRMED** — JS SDK lags: v1.0.0-beta.0 (July 1, 2026), no v1.0 GA; v0.3.x still patched (v0.3.14, July 9, 2026)
  - Releases page confirms v1.0.0-beta.0 dated 2026-07-01, no GA v1.0.0, and v0.3.14 dated 2026-07-09 still maintained — exactly as the brief characterizes.
  - Evidence: <https://github.com/a2aproject/a2a-js/releases>
- ✅ **CONFIRMED** — IBM's ACP (BeeAI) formally merged into A2A under the Linux Foundation, announced Aug 29, 2025 on the LF AI & Data blog; BeeAI now uses A2A natively
  - LF AI & Data blog post exists, dated Aug 29, 2025, announcing the ACP→A2A merger; BeeAI migrated to A2A (A2AServer adapter / A2AAgent client, migration guide provided). Independently cross-checked via the i-am-bee GitHub org discussion and IBM's own ACP page confirming ACP wound down active development and contributed to A2A.
  - Evidence: <https://lfaidata.foundation/communityblog/2025/08/29/acp-joins-forces-with-a2a-under-the-linux-foundations-lf-ai-data/>
- ✅ **CONFIRMED** — LF's April 9, 2026 one-year press release claims 150+ supporting orgs and production deployments across industries, names zero end-customer deployments; concrete integrations are Azure AI Foundry, Copilot Studio, Amazon Bedrock AgentCore Runtime, Google Cloud
  - Press release dated April 9, 2026; states growth from 50+ to 150+ orgs and 'active production deployments across multiple industries' (supply chain, financial services, insurance, IT ops) without naming a single end customer; all four platform integrations named. Vendors named are AWS, Cisco, Google, IBM, Microsoft, Salesforce, SAP, ServiceNow plus LangGraph/CrewAI — vendors, not customers, exactly the support-vs-usage gap the brief flags.
  - Evidence: <https://www.linuxfoundation.org/press/a2a-protocol-surpasses-150-organizations-lands-in-major-cloud-platforms-and-sees-enterprise-production-use-in-first-year>
- ✅ **CONFIRMED** — A2A and MCP positioned as complementary (spec Appendix B): MCP is model-to-tool, A2A is agent-to-agent
  - Spec contains 'Appendix B. Relationship to MCP (Model Context Protocol)' framing the two as complementary layers, matching the brief.
  - Evidence: <https://a2a-protocol.org/latest/specification/>

Load-bearing claims in the prose **without** a sourced key fact:

- ⚠️ 'Neither Claude Code, claude.ai, ChatGPT, nor Codex ships as an A2A agent today' — a load-bearing negative underpinning the whole recommendation (compliance buys zero interop with first users), with no key_fact or source; negatives like this are hard to verify but should at least cite roadmap/docs searches per product.
- ⚠️ 'Active SDKs in 6 languages' and 'Java v1.1.0.Final shipped with full 1.0 support; .NET and Rust also listed' — no key_fact; the open_questions section itself admits Java dates could not be verified, and the .NET/Rust status (maturity, v1.0 support) is asserted without any source.
- ⚠️ 'The data model is protobuf-defined, so it can be reused over any transport we choose' — a load-bearing premise of the steal-the-semantics recommendation, not backed by any key_fact (the TASK_STATE_* naming is consistent with proto enums, but the transport-independence claim is uncited).
- ⚠️ 'It has won the agent-interop protocol race' / 'no credible rival vocabulary to bet on instead' — rests only on the ACP-merger key_fact; other agent-interop efforts (e.g., AGNTCY/Cisco's stack, ANP) are neither mentioned nor ruled out by any sourced fact.
- ⚠️ 'It has no rendezvous, relay, NAT traversal, or offline-delivery story whatsoever' — consistent with the verified transport bindings and the open question about non-HTTP profiles, but the specific absence claim (especially offline delivery) has no key_fact anchoring a spec-wide check.
- ⚠️ 'Originally announced by Google (April 2025)' — true (Google Cloud Next, April 2025) but carried by no key_fact/source in the brief.
- ⚠️ 'Independent commentary flags exactly this support-vs-usage gap' — no source given for the commentary itself.
- ⚠️ 'v1.0 added Agent Card signature validation' (what_it_is) — not covered by any key_fact as written, though I verified it incidentally: the v1.0.0 release notes do add a signatures field to AgentCard.
