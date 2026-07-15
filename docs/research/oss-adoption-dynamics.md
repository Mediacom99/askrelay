# How OSS developer/agent tools actually win adoption (distribution and launch evidence, 2025-2026)

> Decision brief — research sweep of 2026-07-14. Every key fact cites a live source
> fetched that day; an independent verifier re-checked each one (verdicts below).

## TL;DR

The documented growth channel for agent-infra OSS in 2025-26 is a working demo shown on X/social plus a one-command install — not Show HN (mcp_agent_mail's three Show HN posts scored 4, 1, and 14 points yet the project reached ~2,000 stars via the creator's X audience and Steve Yegge's amplification) and not registry listing alone (~9,652 registry entries, most invisible). Stars and adoption decouple both ways: Continue has ~32k stars but 3.65M VS Code installs; OpenClaw's 383k stars are backed by 12.9M npm downloads/month. Critically for this product, a hosted public HTTPS endpoint is not a marketing nicety but a hard technical requirement: claude.ai and ChatGPT web sessions can only connect to remote MCP servers. Recommended install path: hosted remote URL first, npx/uvx one-liner second, docker third; go install should not be the primary path — even GitHub's own Go-based MCP server doesn't list it.

## What it is

This brief maps how comparable MCP-server and agent-infrastructure OSS projects acquired users in 2025-2026, to ground three meeting decisions: install-path priority, whether to build a hosted demo relay, and launch sequencing. Evidence base: the official MCP Registry (preview launched 2025-09-08, ~2,000 servers by Nov 2025, ~9,652 entries by mid-2026), awesome-mcp-servers (90.8k stars), and case studies verified against live sources — mcp_agent_mail (Jeffrey Emanuel, Oct 2025; 2,000 stars; curl|bash + uv install; grew via X and a Steve Yegge fork/endorsement despite three failed Show HN attempts; its concepts were later absorbed into Claude's agent-teams feature), OpenClaw (Peter Steinberger; 9k stars in 24h at Nov 2025 launch, 383k stars and 12.9M monthly npm downloads by Jul 2026; npm install -g; grew via X and Discord, appearing on HN mostly as drama/security threads), Continue (YC S23; Launch HN Mar 2025 scored only 178 points; 3.65M VS Code marketplace installs), Aider (40k+ stars, pip/uv), GitHub's official MCP server (Go; Docker-recommended locally, remote hosted preview Jun 2025), and Cloudflare's remote-MCP templates with Deploy button (Mar 25, 2025).

## Maturity

The distribution infrastructure is young but functioning. Official MCP Registry: preview since 2025-09-08, still pre-GA with breaking changes possible; grew 407% to ~2,000 servers by the protocol's first anniversary (2025-11-25) and ~9,652 entries by mid-2026; it feeds subregistries (PulseMCP, Glama, mcpmarket) that auto-list servers — mcp_agent_mail appears on all three with no evident effort by its author. MCP itself is the de-facto standard: 97M+ monthly SDK downloads (Python+TS) and 41% of surveyed enterprises in limited-or-broad production (Stacklok survey via a secondary stats roundup, updated 2026-05-24); clients on both sides of the target persona support remote MCP (claude.ai custom connectors on all plans incl. Free with limits; ChatGPT developer mode on Pro/Plus/Business/Enterprise/Edu web since Oct 2025). Hosted-remote is the direction of travel: GitHub's remote MCP server public preview (2025-06-12) explicitly pitched "no local install or runtime needed" against its own Docker path; Cloudflare shipped remote-MCP scaffolding (McpAgent, workers-oauth-provider, mcp-remote, Deploy button) on 2025-03-25 and 13 hosted servers of its own. Case-study projects are all actively maintained as of July 2026 (Continue extension updated 2026-06-28; OpenClaw shipping continuously).

## What adopting it buys us

Adopting the evidence-backed playbook buys predictable distribution instead of a lottery. Concretely: (1) A hosted relay endpoint makes the product usable at all for claude.ai and ChatGPT web colleagues — both clients require a publicly reachable remote MCP URL — and doubles as the zero-install "try it in 60 seconds" demo, the asset GitHub and Cloudflare both found decisive enough to build hosted offerings around. (2) A one-command npx/uvx path matches the ecosystem's canonical quickstart (the official MCP docs configure servers via "command": "npx" with -y auto-install), which is what CLI-side users (Claude Code, Codex, Cursor) expect to paste into a config. (3) Registry + subregistry listings are free passive discovery: one publish to the official registry propagates to PulseMCP/Glama/mcpmarket. (4) A demo video/GIF on X targeting the agent-dev community is the only channel with documented causal wins in this exact niche (OpenClaw: 9k stars in 24h; mcp_agent_mail: 2k stars off a 44.8k-follower X account plus a Yegge fork, after Show HN failed three times). (5) Knowing stars ≠ adoption (Continue: 32k stars vs 3.65M installs) lets the team optimize for installs-per-week and named team deployments instead of vanity metrics.

## What it costs us

The hosted relay is the main cost: an operated service with uptime, abuse handling, OAuth, and security exposure — for a team-messaging tool this is the sensitive path (message content transits it). Cloudflare Workers makes the build cheap (templates, workers-oauth-provider, Durable Objects with hibernation "near zero" cost claims), but it commits the project to TypeScript-adjacent infra or a split stack, which pressures the Go default flagged in the transport brief. The npx-first install path similarly favors a TypeScript/Node implementation; keeping Go means shipping prebuilt binaries + an install script + optionally an npm wrapper, since go install is effectively absent as a distribution path even for GitHub's flagship Go MCP server (Docker recommended, go build relegated to "advanced users"). Launch-asset production is real work: a tight demo video/GIF, a README that sells in 30 seconds, registry publication, and — given that the biggest HN threads about OpenClaw are security backlash ("security nightmare" 397 points; privilege-escalation disclosure 514 points) and Microsoft publicly warned about poisoned MCP tool descriptions in June 2026 — a published threat model is cheap insurance for an agent-to-agent messaging tool, even though no quantitative evidence ties threat-model docs to adoption. Show HN costs little but should be budgeted as high-variance, not the plan.

## Recommendation

Adopt this ranked launch plan: (1) Hosted free relay + demo video first — build the public HTTPS MCP endpoint before launch; it is mandatory for claude.ai/ChatGPT participants and is the demo. (2) Launch on X with a 60-90s screen-capture of two colleagues' sessions talking (A's Claude Code asks, B's ChatGPT answers after approval) — the only channel with repeated documented wins in this niche; seed 3-5 practitioner-influencers directly (the Yegge effect is documented). (3) Publish to the official MCP Registry same day; subregistries auto-propagate. (4) PR to awesome-mcp-servers (90.8k-star list). (5) Show HN with the video in week 1, expectations low. (6) Stand up a Discord once there are >~50 users, not before. Install-path priority: hosted remote URL (paste into claude.ai/ChatGPT connectors) > one-command npx/uvx for CLI agents > docker for self-hosters > prebuilt binaries; do not ship go install as a promoted path. This feeds the stack decision: the evidence tilts the "single-binary vs Workers deploy" tiebreaker toward remote-endpoint-first (Workers or equivalent), with the local component thin and npx-installable — which weakens, though doesn't eliminate, the case for Go. Include a threat-model section in the README at launch: defensive necessity for a message-carrying agent tool, per the OpenClaw security backlash and Microsoft's MCP-poisoning warning.

## Key facts

1. **claude.ai custom connectors require the MCP server to be reachable over the public internet from Anthropic's IP ranges; available on Free (1 connector) through Enterprise plans — a hosted endpoint is a hard requirement for claude.ai participants.**
   - Source: <https://support.claude.com/en/articles/11175166-get-started-with-custom-connectors-using-remote-mcp>
   - Verified: Web search surfaced the Claude Help Center article stating the server must be publicly reachable and connections originate from Anthropic's servers, with plan availability listed.
2. **ChatGPT developer mode (custom MCP connectors) is available to Pro, Plus, Business, Enterprise, and Education accounts on the web, supports only remote SSE/streaming-HTTP servers, with a Secure MCP Tunnel option for private servers.**
   - Source: <https://developers.openai.com/api/docs/guides/developer-mode>
   - Verified: Fetched the OpenAI developer-mode guide; it quotes plan availability verbatim and lists supported remote protocols.
3. **mcp_agent_mail reached ~2,000 stars (215 forks) with a curl|bash + uv install, a 23-minute walkthrough video and animated GIF in the README, and no hosted instance.**
   - Source: <https://github.com/Dicklesworthstone/mcp_agent_mail>
   - Verified: Fetched the repo page: 2,000 stars, install.sh quickstart, showcase GIF and video link confirmed.
4. **All three of mcp_agent_mail's Show HN posts by its creator flopped: 4 points (2025-10-27), 1 point (2025-11-06), 14 points (2025-11-02) — its growth came from other channels (creator's ~44.8k-follower X account; Steve Yegge forked and promoted it alongside Beads).**
   - Source: <https://hn.algolia.com/api/v1/search?query=mcp_agent_mail&tags=story>
   - Verified: Queried the HN Algolia API directly; all three stories with points/dates returned. Yegge fork confirmed at github.com/steveyegge/mcp_agent_mail via search results.
5. **OpenClaw has 383k stars and 80.4k forks, installs via 'npm install -g openclaw@latest', and recorded 12,934,926 npm downloads in the month 2026-06-14 to 2026-07-13 — evidence that its stars reflect real installs, and that npm one-command install scaled.**
   - Source: <https://api.npmjs.org/downloads/point/last-month/openclaw>
   - Verified: Fetched npm downloads API JSON (12.93M/month) and the GitHub repo page (stars, forks, install command) separately.
6. **OpenClaw's launch growth was X/Discord-driven (9,000 stars in 24 hours at Nov 2025 launch as Clawdbot; ~200k by mid-Feb 2026); its largest HN threads are drama and security backlash, e.g. 'OpenClaw is a security nightmare' (397 points, 2026-03-22) and a privilege-escalation disclosure (514 points, 2026-04-03).**
   - Source: <https://hn.algolia.com/api/v1/search?query=openclaw&tags=story>
   - Verified: HN Algolia API listed top OpenClaw stories with points/dates; growth timeline cross-checked against n9o.xyz retrospective (community source, labeled as such).
7. **Continue's Launch HN (2025-03-27) scored only 178 points / 110 comments, yet the VS Code extension shows 3,650,978 installs (updated 2026-06-28) against ~32k GitHub stars — marketplace/registry placement, not HN or stars, drove its install base.**
   - Source: <https://marketplace.visualstudio.com/items?itemName=Continue.continue>
   - Verified: Fetched the VS Code Marketplace page (3.65M installs) and the HN thread item 43494427 (178 points) directly.
8. **The official MCP Registry launched in preview on 2025-09-08 and grew 407% to ~2,000 servers by 2025-11-25; it acts as upstream for subregistries (PulseMCP, Glama, mcpmarket) that auto-list servers, but publishes no install-driving metrics and is still pre-GA.**
   - Source: <https://blog.modelcontextprotocol.io/posts/2025-09-08-mcp-registry-preview/>
   - Verified: Fetched the registry preview announcement and the first-anniversary post (2025-11-25) on the official MCP blog; both dates and the 407%/2,000 figures confirmed.
9. **By mid-2026 the registry holds ~9,652 server entries and GitHub has ~15,926 mcp-server-topic repos, while MCP SDKs see 97M+ monthly downloads — listing alone is invisible; the median server gets no adoption.**
   - Source: <https://www.digitalapplied.com/blog/mcp-adoption-statistics-2026-model-context-protocol>
   - Verified: Fetched the stats roundup (published 2026-04-20, updated 2026-05-24); secondary source, but it cites Anthropic's Dec 2025 announcement and a Stacklok survey and self-corrects an earlier unsourced claim, increasing credibility.
10. **GitHub's official MCP server (written in Go) does not offer go install at all: docs recommend Docker locally ('most popular and recommended'), relegate 'go build' to advanced users, and GitHub launched a hosted remote version (public preview 2025-06-12) pitched as 'no local install or runtime needed. Just one-click install into VS Code.'**
   - Source: <https://github.com/github/github-mcp-server/blob/main/docs/installation-guides/README.md>
   - Verified: Fetched the installation-guides README (Docker recommended, no go install) and the GitHub changelog post of 2025-06-12 (remote preview, friction language quoted).
11. **The official MCP quickstart's canonical local-server install is npx ('command': 'npx', '-y', package) — one-command Node execution is the ecosystem's expected install shape for CLI-side clients.**
   - Source: <https://modelcontextprotocol.io/docs/develop/connect-local-servers>
   - Verified: Fetched the full quickstart page; the claude_desktop_config.json examples use npx exclusively, with Node.js as the stated prerequisite.
12. **Cloudflare shipped the remote-MCP build path on 2025-03-25: workers-oauth-provider, McpAgent (Durable Objects transport), the mcp-remote adapter, and a one-click Deploy button template — making a hosted MCP endpoint a minutes-not-weeks build.**
   - Source: <https://blog.cloudflare.com/remote-model-context-protocol-servers-mcp/>
   - Verified: Fetched the Cloudflare announcement; publish date and all four shipped components confirmed; noted it contains no adoption metrics.

## Open questions

- No public data isolates how many installs the official MCP Registry itself drives (it publishes listing counts, not referral/install analytics) — its rank in the launch plan is inferred from subregistry propagation, not measured conversion.
- No quantitative evidence found that threat-model transparency correlates with adoption; the recommendation to publish one rests on documented security backlash (OpenClaw HN threads, Microsoft's June 2026 poisoned-tool-descriptions warning) as downside insurance, not on a measured uplift.
- ChatGPT Free-tier users appear excluded from custom MCP connectors (Pro/Plus/Business/Enterprise/Edu only per OpenAI docs); confirm which plans the actual first-team colleagues are on before assuming ChatGPT-side reach.
- OpenAI's Secure MCP Tunnel could let a ChatGPT session reach a private/local server without public hosting — its capabilities, plan gating, and Claude-side equivalent were not verified in depth and could soften the hosted-relay requirement on one side.
- OpenClaw's 12.9M monthly npm downloads likely include CI and auto-update traffic; no per-team or DAU breakdown exists, so treat it as an order-of-magnitude usage signal only.
- mcp_agent_mail's exact star-growth curve (dates of inflection vs. Yegge's fork and the Claude agent-teams integration) was not archivally reconstructed; the channel attribution (X + influencer, not HN) is solid but the timing granularity is not.
- Whether an npm-wrapper distribution of a Go binary (npx-installable single binary) performs as well as a native Node package has no documented case study in this dataset — relevant if the meeting keeps the Go default.

## Independent verification

**Overall:** This brief is unusually reliable for its genre: all 12 key_facts are CONFIRMED against their primary sources, with exact numbers (HN points and dates, npm download counts to the digit, marketplace installs, registry growth figures, plan gating for both claude.ai and ChatGPT connectors) matching what the sources actually say, and the load-bearing architectural claim — that claude.ai and ChatGPT web sessions require a publicly reachable remote MCP endpoint — is verbatim in both vendors' docs. The brief also honestly labels its secondary sources and lists its own weaknesses in open_questions. Residual defects are minor: the Steve Yegge fork URL cited as verification now 404s (the fork appears deleted, though his promotion of the tool is independently documented), the 44.8k X-follower figure and Continue's 2026-06-28 update date are asserted without verifiable sources, Continue's GitHub stars are ~34.9k rather than '~32k' (which only strengthens the brief's decoupling argument), and several color details (awesome-mcp-servers and Aider star counts, the Microsoft June-2026 MCP-poisoning warning, Cloudflare's 13 hosted servers, subregistry auto-listing) ride without key_facts — of these, the ones I spot-checked (awesome-mcp-servers 90,785 stars, Aider 47,399 stars, Microsoft's 2026-06-30 guidance) all held up. The recommendations (hosted-remote-first install priority, X-demo launch, registry publication, threat-model section) are well supported by the verified evidence; nothing found would reverse any of them.

Verdicts: CONFIRMED 12

- ✅ **CONFIRMED** — claude.ai custom connectors require public reachability from Anthropic's IP ranges; available Free (1 connector) through Enterprise
  - Help Center article states verbatim that the server 'must be reachable over the public internet from Anthropic's IP ranges', connections originate from Anthropic's cloud not the user's device, and availability is 'Free, Pro, Max, Team, and Enterprise plans. Free users are limited to one custom connector.' Hard-requirement framing is accurate.
  - Evidence: <https://support.claude.com/en/articles/11175166-get-started-with-custom-connectors-using-remote-mcp>
- ✅ **CONFIRMED** — ChatGPT developer mode available Pro/Plus/Business/Enterprise/Edu on web; remote SSE/streaming-HTTP only; Secure MCP Tunnel option exists
  - Guide states availability on 'Pro, Plus, Business, Enterprise, and Education accounts on the web' and 'Supported MCP protocols: SSE and streaming HTTP.' A separate 'Secure MCP Tunnel' guide exists in the same docs navigation; its capabilities were not detailed on this page (the brief itself flags this as an open question).
  - Evidence: <https://developers.openai.com/api/docs/guides/developer-mode>
- ✅ **CONFIRMED** — mcp_agent_mail: ~2,000 stars, 215 forks, curl|bash + uv install, 23-min walkthrough video + animated GIF, no hosted instance
  - Repo shows 2k stars / 215 forks, one-line curl|bash install.sh quickstart, uv alternative, animated showcase and 23-minute walkthrough video referenced in README; self-hosted only (local FastMCP server on port 8765), no hosted instance.
  - Evidence: <https://github.com/Dicklesworthstone/mcp_agent_mail>
- ✅ **CONFIRMED** — All three Show HN posts flopped (4 pts 2025-10-27, 1 pt 2025-11-06, 14 pts 2025-11-02); growth via creator's ~44.8k-follower X account and Steve Yegge fork/promotion
  - HN Algolia API returns exactly the three stories with the stated points and dates, all by author 'eigenvalue' (Jeffrey Emanuel). Yegge promotion is documented (his LinkedIn 'beads + mcp_mail' post, Medium posts, and a Google-indexed steveyegge/mcp_agent_mail fork page) — but the fork URL itself now returns 404 on GitHub (deleted or renamed; it is absent from the upstream repo's live forks list). The 44.8k X-follower figure could not be independently verified (X unreachable). Core channel-attribution claim stands.
  - Evidence: <https://hn.algolia.com/api/v1/search?query=mcp_agent_mail&tags=story>
- ✅ **CONFIRMED** — OpenClaw: 383k stars / 80.4k forks, 'npm install -g openclaw@latest', 12,934,926 npm downloads 2026-06-14 to 2026-07-13
  - npm API returns exactly 12,934,926 downloads for June 14 – July 13, 2026; github.com/openclaw/openclaw shows 383k stars, 80.4k forks, and the exact install command, creator Peter Steinberger. The brief's own caveat that downloads include CI/auto-update traffic is appropriate.
  - Evidence: <https://api.npmjs.org/downloads/point/last-month/openclaw>
- ✅ **CONFIRMED** — OpenClaw launch: 9,000 stars in 24h as Clawdbot (Nov 2025), ~200k by mid-Feb 2026; largest HN threads are security backlash: 'security nightmare' 397 pts 2026-03-22, privilege-escalation 514 pts 2026-04-03
  - HN Algolia confirms both security stories at exactly the stated points/dates (plus a 1,349-pt drama thread), consistent with the 'HN presence is mostly drama/security' characterization. The 9k-stars-in-24h Clawdbot launch is corroborated by multiple independent retrospectives (taskade.com history, dev.to). The '~200k by mid-Feb 2026' point is directionally consistent with third-party histories (hundreds of thousands of stars by early 2026) but rests on the community retrospective the brief itself labels as such; exact mid-Feb figure not independently pinned.
  - Evidence: <https://hn.algolia.com/api/v1/search?query=openclaw&tags=story>
- ✅ **CONFIRMED** — Continue Launch HN (2025-03-27) 178 pts / 110 comments; VS Code extension 3,650,978 installs (updated 2026-06-28) vs ~32k GitHub stars
  - HN item 43494427 is 'Launch HN: Continue (YC S23)', 178 points, created 2025-03-27; comment count of 110 not directly countable from the API response but plausible. Marketplace page shows exactly 3,650,978 installs; the 2026-06-28 'last updated' date was not visible on the fetched page (UNVERIFIED sub-detail). GitHub API shows 34,880 stars — the brief's '~32k' slightly understates but the stars-vs-installs decoupling argument holds a fortiori.
  - Evidence: <https://hn.algolia.com/api/v1/items/43494427>
- ✅ **CONFIRMED** — Official MCP Registry preview launched 2025-09-08, grew 407% to ~2,000 servers by 2025-11-25, feeds subregistries, pre-GA with breaking changes possible
  - Preview announcement (2025-09-08) confirmed with explicit 'breaking changes may occur before general availability' language and subregistry architecture; first-anniversary post (2025-11-25) confirms 'close to two thousand entries... 407% growth'. The specific claim that PulseMCP/Glama/mcpmarket auto-listed mcp_agent_mail with no author effort was not independently spot-checked.
  - Evidence: <https://blog.modelcontextprotocol.io/posts/2025-11-25-first-mcp-anniversary/>
- ✅ **CONFIRMED** — Mid-2026: ~9,652 registry entries, ~15,926 mcp-server-topic GitHub repos, 97M+ monthly MCP SDK downloads, 41% enterprise limited/broad production (Stacklok)
  - Source page (published 2026-04-20, updated 2026-05-24) gives exactly 9,652 latest records, 15,926 mcp-server-topic repos, 97M+ monthly SDK downloads attributed to Anthropic's Dec 2025 update, and 41% (29% limited + 12% broad) from Stacklok's 2026 report — and does self-correct a prior unsourced 78% claim, as the brief says. Secondary source, correctly labeled as such in the brief; underlying registry-API and GitHub-API figures not re-derived by me.
  - Evidence: <https://www.digitalapplied.com/blog/mcp-adoption-statistics-2026-model-context-protocol>
- ✅ **CONFIRMED** — GitHub's official Go MCP server: no go install path, Docker 'most popular and recommended', go build for 'Advanced Users', remote hosted preview 2025-06-12 pitched as 'no local install or runtime needed'
  - Installation-guides README confirms 'Docker is the most popular and recommended approach', zero mentions of go install, and 'Build from Source (Advanced Users)'. GitHub changelog of 2025-06-12 confirms the remote public preview with the exact 'no local install or runtime needed' friction pitch and one-click VS Code install. (Remote server went GA 2025-09-04, which only strengthens the claim.)
  - Evidence: <https://github.blog/changelog/2025-06-12-remote-github-mcp-server-is-now-available-in-public-preview/>
- ✅ **CONFIRMED** — Official MCP quickstart's canonical local install is 'command': 'npx' with '-y' and package name; Node.js is the stated prerequisite
  - Fetched the full page: every claude_desktop_config.json example (macOS and Windows) uses 'command': 'npx' with '-y', and Node.js is an explicit prerequisite section. npx is used exclusively on this quickstart.
  - Evidence: <https://modelcontextprotocol.io/docs/develop/connect-local-servers>
- ✅ **CONFIRMED** — Cloudflare shipped remote-MCP build path 2025-03-25: workers-oauth-provider, McpAgent (Durable Objects), mcp-remote adapter, one-click Deploy button
  - Post dated March 25, 2025 announces all four named components, including the 'less than two minutes' Deploy-to-Cloudflare button. Brief's note that the post contains no adoption metrics is also accurate.
  - Evidence: <https://blog.cloudflare.com/remote-model-context-protocol-servers-mcp/>

Load-bearing claims in the prose **without** a sourced key fact:

- ⚠️ awesome-mcp-servers has 90.8k stars (used in what_it_is and as recommendation step 4) — no key_fact; I spot-checked it anyway: GitHub API shows 90,785 stars, so it is accurate but unbacked in the brief.
- ⚠️ Aider has 40k+ stars with pip/uv install — no key_fact; GitHub API shows 47,399 stars, so accurate but unbacked.
- ⚠️ mcp_agent_mail's concepts were 'later absorbed into Claude's agent-teams feature' — load-bearing color for the case study, no key_fact and not verified.
- ⚠️ Microsoft publicly warned about poisoned MCP tool descriptions in June 2026 (drives the threat-model recommendation) — no key_fact; I verified it independently: Microsoft Security Blog guidance of 2026-06-30 and Hacker News coverage confirm it, but the brief should have sourced it.
- ⚠️ Cloudflare runs '13 hosted servers of its own' — no key_fact, not verified.
- ⚠️ mcp_agent_mail 'appears on all three subregistries (PulseMCP, Glama, mcpmarket) with no evident effort by its author' — supports the free-passive-discovery argument (buys point 3), no key_fact and not verified.
- ⚠️ Durable Objects hibernation 'near zero' cost claims — quoted in what_it_costs_us with no key_fact or source.
- ⚠️ Creator's ~44.8k-follower X account (appears in tldr, buys, and key_fact 4's parenthetical) — the given source (HN Algolia) cannot support it and X follower counts were not verifiable; the star-attribution argument leans on this number.
- ⚠️ Continue extension 'updated 2026-06-28' — stated in key_fact 7 but the marketplace page as fetched displays no last-updated date; maintenance-status claim in maturity rests on it.
