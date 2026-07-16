# Market research — Market size: addressable population of teams for askrelay (cross-person, approval-gated AI-session messaging)

> Market go/no-go input, researched 2026-07-16 against live sources;
> every evidence item independently re-checked (verdicts below).

## TL;DR

The raw population is large and growing fast: ~35-47M developers worldwide, ~50-90% using AI tools (51% of professionals daily per Stack Overflow 2025), and millions on agentic session tools specifically (Claude Code ~2-4M weekly actives, Copilot 4.7M paid subscribers, 7M+ ChatGPT for Work seats). An order-of-magnitude ESTIMATE puts the theoretical TAM at ~10^6 small teams (2-50 devs, 2+ daily AI users), a serviceable niche of ~10^4-10^5 MCP-forward teams, and a realistic year-one obtainable of 10^2-10^3 teams for a solo-maintainer OSS tool. Market size is not the binding constraint — discovery and pain intensity are.

## Evidence

1. 🟡 **[secondary, Q1 2026 (SlashData); 2025 (Evans Data)]** Global developer population: SlashData counts 47.2M active developers (incl. hobbyists/students, code at least monthly) in its Q1 2026 report; Evans Data 2025 pegs paid professionals at 35.6M. Use ~35M professionals as the sizing base.
   - Source: <https://www.slashdata.co/research/developer-population>
2. 🟢 **[primary, 2025 survey, results published 2025-12-29]** Stack Overflow 2025 Developer Survey (published Dec 2025): 84% of respondents use or plan to use AI tools (up from 76%), 51% of professional developers use them daily, but only 29% trust output accuracy and ~3% highly trust AI-generated code; ~23% regularly use AI agents.
   - Source: <https://survey.stackoverflow.co/2025/ai>
3. 🟢 **[primary, 2025 (report published Sept 2025)]** Google DORA 2025 report (~5,000 tech professionals surveyed): 90% use AI in daily workflows, median ~2 hours/day working with AI. Confirms daily multi-person AI usage is now the norm inside teams, not the exception.
   - Source: <https://dora.dev/dora-report-2025/>
4. 🟡 **[secondary, Feb-May 2026]** Claude Code run-rate revenue passed $2.5B by Feb 2026 (Reuters-reported; fastest enterprise software product to $1B annualized, ~6 months after May 2025 GA). Aggregators claim ~$8B annualized and 4.2M weekly active developers by Q1-May 2026 and 1,400+ enterprise engineering org deployments — these larger figures could not be traced to a first-party or wire source and should be treated as unverified.
   - Source: <https://venturebeat.com/technology/anthropic-says-it-hit-a-30-billion-revenue-run-rate-after-crazy-80x-growth>
5. 🟡 **[secondary, April 2026]** Anthropic overall: ~$30B annual run-rate (April 2026, VentureBeat/Reuters); 300,000+ business customers; ~70% of Fortune 100 use Claude. A LinkedIn milestone post puts Claude Code past 2M weekly active users around Dec 2025 — so 2-4M WAU is the defensible H1-2026 range.
   - Source: <https://venturebeat.com/technology/anthropic-says-it-hit-a-30-billion-revenue-run-rate-after-crazy-80x-growth>
6. 🟡 **[secondary, late 2025 - Feb 2026]** OpenAI first-party: 1 million paying business customers announced on openai.com ('fastest-growing business platform in history'); later reporting cites 9M+ paying business users by Feb 2026 and 7M+ ChatGPT for Work seats. ChatGPT Enterprise total seat counts are not publicly disclosed (absence of signal). Direct fetch of the OpenAI page was blocked (403); figures corroborated via search snippets only.
   - Source: <https://openai.com/index/1-million-businesses-putting-ai-to-work/>
7. 🟡 **[secondary, July 2025 - Jan 2026]** GitHub Copilot: ~20M cumulative users by July 2025; ~4.7M paid subscribers by Jan 2026 (+~75% YoY); deployed at ~90% of Fortune 100; 50,000+ organizations use it. Confirms org-level (not just individual) AI-coding adoption at scale.
   - Source: <https://www.getpanto.ai/blog/github-copilot-statistics>
8. 🟡 **[secondary, May-July 2026]** MCP adoption trajectory: ~97-110M monthly SDK downloads by mid-2026; ~9,650 servers in the official MCP Registry (May 2026); ~15,900 GitHub repos tagged mcp-server; Stacklok 2026 survey: 41% of software orgs in limited-or-broad production with MCP; supported across ChatGPT, Gemini, Copilot, VS Code. askrelay's zero-install connect path rides a protocol that is now cross-vendor mainstream.
   - Source: <https://www.digitalapplied.com/blog/mcp-adoption-statistics-2026-model-context-protocol>
9. 🟡 **[secondary, 2025-2026]** Self-hosted OSS appetite (analogies): n8n reached $40M ARR (July 2025), $2.5B valuation (Oct 2025), ~127k GitHub stars and 1,400+ enterprise customers with ~55% of revenue from cloud vs ~30% self-host enterprise licenses; Gitea reported 40,000+ tracked production deployments (mid-2025); Plausible CE has ~27k stars with the cloud offering as the main business. Pattern: dev-infra OSS reaches 10^4-10^5 installs at maturity, and hosted/low-ops paths capture the majority of paying users.
   - Source: <https://sacra.com/c/n8n/>
10. 🟢 **[primary, 2025 survey]** Team-size fit: Stack Overflow 2025 reports 57% of employed developers work at companies with fewer than 500 employees — the small-company segment where 2-50-dev teams and informal knowledge-sharing (askrelay's ICP) dominate.
   - Source: <https://survey.stackoverflow.co/2025/work/>
11. ⚪ **[weak, as of 2026-07-16]** No public source quantifies the specific pain askrelay targets (cross-person AI-session Q&A / knowledge-silo friction between agent sessions). No survey measures 'teams where 2+ members run agentic coding sessions daily.' This layer of the funnel is pure extrapolation — the absence of a direct demand signal is itself a finding.
   - Source: <https://survey.stackoverflow.co/2025/ai>

## Assessment

ESTIMATE (order of magnitude, clearly extrapolated): take ~35M professional developers, ~51-90% using AI daily (SO 2025 / DORA 2025), organized into teams averaging ~5-10 devs — roughly 3-5M dev teams worldwide, of which teams of 2-50 devs with 2+ daily AI users plausibly number ~1M (10^6). That is the theoretical TAM and it is generous. The serviceable market is much narrower: teams already running agentic session tools (Claude Code 2-4M WAU, Codex, Cursor) AND comfortable adopting third-party MCP infrastructure — Stacklok's 41%-in-production figure and 10^4-10^5-install ceilings of n8n/Gitea-class tools suggest ~50k-200k teams (10^5). Realistic obtainable for a pre-code, solo-maintainer OSS project in year one is 10^2-10^3 teams, consistent with early trajectories of comparable dev-infra OSS. Confidence: high that the population is large and growing (multiple independent primary/secondary sources agree); low on the team-level cuts, which no survey measures directly. The critical unknown is not headcount but pain incidence — nobody publishes data on cross-person AI-session friction, so the ICP fraction feeling it acutely enough to deploy a relay could be anywhere from 1% to 20% of the serviceable pool. Aggregator-site figures (4.2M Claude Code WAU, $8B run-rate) were not first-party verifiable and are excluded from the base case.

## Implications for askrelay

Market size clears the go bar for an OSS project: even the pessimistic cut (10^4 serviceable teams, 1% activation) yields ~100 real teams — enough for a healthy niche OSS community, though not a venture-scale business, which matches askrelay's positioning. The zero-install path is the decisive multiplier: only one person per team must run the Go relay; everyone else connects via remote MCP from claude.ai/ChatGPT with no install. Analogy data (n8n ~55% cloud revenue; Plausible cloud-dominant) suggests low-ops paths reach roughly 3-10x more users than self-host-only, so preserving and marketing the "one teammate hosts, everyone else just connects" story is worth more than any feature. MCP's cross-vendor mainstreaming (97M+ monthly SDK downloads, 41% of orgs in production) means the connection substrate askrelay bets on is no longer a niche risk. Beachhead should be MCP-forward small companies (<500 employees, 57% of employed devs) already on Claude Code — the tool with the steepest 2026 growth curve. The main sizing caveat stands: no data quantifies the specific pain, so early validation with Kosmoy plus ~5-10 external teams matters more than any TAM figure.

## Open questions

- What fraction of teams actually feel cross-person AI-session knowledge-silo pain acutely? No survey measures this; needs primary validation with 5-10 external teams.
- Claude Code's true weekly-active count in mid-2026: the 4.2M WAU / $8B run-rate figures circulate only on aggregator sites — is there a first-party or wire-service confirmation?
- ChatGPT Enterprise total seat count is undisclosed; the 150-seat minimum means Enterprise skews above askrelay's 2-50-dev ICP — how much of the 7M+ Work seats sits in small Business-tier teams?
- What share of production MCP use is remote Streamable-HTTP (askrelay's zero-install path) vs local stdio servers?
- Will the Stack Overflow 2026 survey (in field July 2026) show regular AI-agent usage rising materially above 2025's ~23% — the leading indicator for askrelay's session-to-session use case?
- How fast do vendor first-party features (e.g., Anthropic/OpenAI team workspaces or agent-to-agent features) compress the niche before askrelay ships?

## Independent verification

Verdicts: CONFIRMED 9, FAILED 4, UNVERIFIABLE 1

**Overall:** The brief is directionally sound and unusually honest about its own extrapolations, and its biggest numbers mostly survive adversarial checking: Stack Overflow 2025 (84%/51%/29%/3%, Dec 29 2025 publication), DORA 2025 (~5,000 surveyed, 90%, 2h/day), Copilot (20M users, 4.7M paid +75% YoY per Microsoft's Jan 28 2026 earnings), OpenAI (1M business customers, 7M+ Work seats, 9M+ paying business users), Anthropic ($30B run-rate April 2026, Claude Code $2.5B by Feb 2026, 300k customers, ~70% F100), MCP registry/Stacklok figures, and the SO 57%-under-500-employees ICP datapoint all confirm against primary or credible secondary sources. However, four defects need correction before this feeds the viability write-up: (1) the developer-population attribution is garbled — 47.2M is SlashData's early-2025 figure (latest 48.4M, Q3 2025) and Evans Data never published 35.6M (its latest is ~27M total; 36.5M professionals is SlashData's number); (2) the Claude Code 2M-WAU milestone is spring 2026, not Dec 2025, so the tldr's '2-4M weekly actives' should read ~1-2M — a 2x inflation of the flagship serviceable-market datapoint; (3) the '~23% regularly use AI agents' figure does not exist in the SO survey (31% current use / 14.1% daily); (4) two n8n details are wrong in the conservative direction (3,000+ enterprise customers, ~197k stars). Because the sizing argument runs on orders of magnitude and generous rounding, none of these errors overturn the conclusion that market size clears the bar and discovery/pain-intensity is the binding constraint — but the assessment's team-level arithmetic (team-size averages, the 3-10x low-ops multiplier, year-one OSS trajectories) rests entirely on unsourced assumptions and should be labeled as such. Verdict: usable after the listed corrections; treat all team-count and obtainable-market figures as modeling assumptions, not researched facts.

- ❌ **FAILED** — SlashData counts 47.2M active developers in its Q1 2026 report; Evans Data 2025 pegs paid professionals at 35.6M
  - The ~35M professional sizing base survives numerically (SlashData: professionals grew to 36.5M), but both the report date (Q1 2026) and the Evans Data attribution are wrong. Cross-checked against evansdata.com press releases (27M in 2024; 26.4M prior) and slashdata.co/post/global-developer-population-trends-2025.
  - **Correction:** 47.2M is SlashData's early-2025 figure (post 'Global developer population trends 2025', published ~April 2025); the cited SlashData research page now shows 48.4M as of Q3 2025. Evans Data's latest published figure is ~27M total developers (2024 biannual report) — it never pegged professionals at 35.6M. The ~36.5M professional-developer count is SlashData's, not Evans Data's.
  - Evidence: <https://www.slashdata.co/research/developer-population>
- ✅ **CONFIRMED** — Stack Overflow 2025 survey (published 2025-12-29): 84% use/plan to use AI (up from 76%); 51% of professional devs use daily; only 29% trust accuracy; ~3% highly trust
  - All confirmed against survey.stackoverflow.co/2025/ai and the SO blog: 84% (from 76%), 51% daily among professionals, trust in accuracy 'fallen from 40% to just 29%' (survey page detail: 3.1% highly + 29.6% somewhat trust), publication date Dec 29, 2025 matches the blog URL and page.
  - Evidence: <https://stackoverflow.blog/2025/12/29/developers-remain-willing-but-reluctant-to-use-ai-the-2025-developer-survey-results-are-here/>
- ❌ **FAILED** — Stack Overflow 2025: ~23% of developers regularly use AI agents
  - This figure is also load-bearing in open_questions ('rising materially above 2025's ~23%'). Neither the survey AI page, the SO blog post, nor targeted searches surfaced 23%/22.7% anywhere; use 31% (any current use) or 14.1% (daily) instead.
  - **Correction:** The survey reports 31% of developers currently use AI agents, 14.1% use them daily, and ~38% have no plans to adopt them. No ~23% 'regular use' figure appears in the published results.
  - Evidence: <https://survey.stackoverflow.co/2025/ai>
- ✅ **CONFIRMED** — Google DORA 2025 (~5,000 tech professionals surveyed, published Sept 2025): 90% use AI in daily workflows, median ~2 hours/day working with AI
  - The dora.dev landing page itself carries no stats, but Google's announcement and multiple independent write-ups confirm: nearly 5,000 technology professionals surveyed, 90% report using AI at work, median two hours daily, report released September 2025. Minor nuance: DORA says 90% 'use AI at work', not strictly 'in daily workflows'.
  - Evidence: <https://cloud.google.com/blog/products/ai-machine-learning/announcing-the-2025-dora-report>
- ✅ **CONFIRMED** — Claude Code run-rate revenue passed $2.5B by Feb 2026; fastest enterprise software product to $1B annualized (~6 months after May 2025 GA); the 4.2M-WAU/$8B aggregator figures are unverifiable
  - Anthropic's own Series G announcement (Feb 2026) and wire-derived coverage confirm Claude Code >$2.5B run-rate, more than doubled since start of 2026, and $1B annualized within ~6 months of the mid-2025 GA. The VentureBeat source URL exists (title matched in search) but 403s automated fetch. The brief's hedge on 4.2M WAU/$8B is sound — my searches also found those only on aggregator sites.
  - Evidence: <https://www.anthropic.com/news/anthropic-raises-30-billion-series-g-funding-380-billion-post-money-valuation>
- ✅ **CONFIRMED** — Anthropic ~$30B annual run-rate (April 2026); 300,000+ business customers; ~70% of Fortune 100 use Claude
  - Multiple sources confirm run-rate exceeded $30B as of early April 2026 (nearly triple the ~$9B at end-2025), 300,000+ business customers, ~70% of Fortune 100 and 8 of Fortune 10 as Claude customers, 1,000+ accounts >$1M/yr.
  - Evidence: <https://venturebeat.com/technology/anthropic-says-it-hit-a-30-billion-revenue-run-rate-after-crazy-80x-growth>
- ❌ **FAILED** — A LinkedIn milestone post puts Claude Code past 2M weekly active users around Dec 2025, so 2-4M WAU is the defensible H1-2026 range
  - This halves the flagship serviceable-population datapoint used in the tldr ('Claude Code ~2-4M weekly actives'). The order-of-magnitude funnel (10^6 TAM / 10^4-10^5 serviceable) survives, but the tldr and assessment should say ~1-2M WAU.
  - **Correction:** The 2M-WAU milestone dates to roughly March-May 2026, not December 2025 (the LinkedIn activity ID decodes to ~March 2026; coverage pegs it at May 2026, with WAU reported as having doubled since January 1, 2026 — implying ~1M WAU in Jan 2026). The defensible H1-2026 range is therefore ~1-2M WAU, not 2-4M.
  - Evidence: <https://www.linkedin.com/posts/gptproto_exciting-milestone-alert-in-just-activity-7437069422558883840-fzyf>
- ✅ **CONFIRMED** — OpenAI: 1M paying business customers announced; 9M+ paying business users by Feb 2026; 7M+ ChatGPT for Work seats; ChatGPT Enterprise total seats undisclosed
  - 1M business customers announced Nov 2025 ('fastest-growing business platform in history'); 7M+ Work seats (up 40% in two months); Enterprise seats disclosed only as a 9x YoY multiple, no absolute count — consistent with 'undisclosed'. 9M+ paying business users by Feb 2026 (4x from Sept 2025) corroborated by digitalinformationworld.com and aggregators; that later figure remains secondary-sourced.
  - Evidence: <https://openai.com/index/1-million-businesses-putting-ai-to-work/>
- ✅ **CONFIRMED** — GitHub Copilot: ~20M cumulative users by July 2025; ~4.7M paid subscribers by Jan 2026 (+~75% YoY); ~90% of Fortune 100; 50,000+ organizations use it
  - 20M by July 2025, 4.7M paid subscribers (+75% YoY) disclosed in Microsoft's FY26 Q2 earnings on Jan 28, 2026 (independently confirmed via windowsforum/office365itpros), and ~90% Fortune 100 all check out. Caveat: the '50,000+ organizations' sub-claim is NOT on the cited page (which cites ~77,000 GitHub enterprise customers, FY2024) — 50k+ Copilot Business orgs is a stale Microsoft figure from Feb 2024 and should be dropped or re-dated.
  - Evidence: <https://www.getpanto.ai/blog/github-copilot-statistics>
- ✅ **CONFIRMED** — MCP: ~97-110M monthly SDK downloads by mid-2026; ~9,650 servers in official Registry (May 2026); ~15,900 GitHub mcp-server repos; Stacklok 2026: 41% of software orgs in limited-or-broad production; supported across ChatGPT/Gemini/Copilot/VS Code
  - Source page confirms 9,652 registry records (May 24, 2026), 15,926 mcp-server repos (May 24, 2026), Stacklok 41% (all-software group) in some form of production, and cross-vendor support per Anthropic's Dec 9, 2025 ecosystem update. Caveat: the page supports '97M+ monthly SDK downloads' as of Dec 2025 (Anthropic-cited); the 110M upper bound and 'mid-2026' dating for downloads are not in the source.
  - Evidence: <https://www.digitalapplied.com/blog/mcp-adoption-statistics-2026-model-context-protocol>
- ❌ **FAILED** — n8n: $40M ARR (July 2025), $2.5B valuation (Oct 2025), ~127k GitHub stars, 1,400+ enterprise customers, ~55% cloud / ~30% self-host enterprise revenue
  - Both errors understate n8n, so the 'dev-infra OSS reaches 10^4-10^5 installs' pattern conclusion still holds — if anything more strongly.
  - **Correction:** Sacra: $40M ARR (July 2025), $180M raise at $2.5B valuation (Oct 2025), and 55% cloud / 30% enterprise licenses / 15% embedded-OEM all confirm — but Sacra says 3,000+ enterprise customers, not 1,400+, and the n8n GitHub repo shows ~197k stars as of July 2026, not ~127k. (The 1,400+ figure appears to be cross-contaminated from the brief's own unverified Claude Code enterprise-deployment claim.)
  - Evidence: <https://sacra.com/c/n8n/>
- ✅ **CONFIRMED** — Gitea reported 40,000+ tracked production deployments (mid-2025); Plausible CE has ~27k stars with cloud as the main business
  - Secondary coverage confirms 40,000+ production deployments tracked via Gitea's opt-in instance-statistics endpoint by mid-2025 (true count likely higher; a May 2026 CVE analysis separately found 30,000+ internet-exposed instances via Shodan). Plausible at 27.7k stars confirmed; cloud-first business model confirmed (CE is community-supported self-host).
  - Evidence: <https://www.serverspan.com/en/blog/the-2026-guide-to-self-hosted-git-gitea-forgejo-and-the-future-of-code-hosting>
- ✅ **CONFIRMED** — Stack Overflow 2025: 57% of employed developers work at companies with fewer than 500 employees
  - The survey page states 57% directly; breakdown (freelancer 3.9%, <20: 17.2%, 20-99: 20.8%, 100-499: 18.5%) sums consistently (~56.5% excluding freelancers, ~60% including).
  - Evidence: <https://survey.stackoverflow.co/2025/work/>
- ❓ **UNVERIFIABLE** — No public source quantifies cross-person AI-session Q&A friction, and no survey measures 'teams where 2+ members run agentic coding sessions daily'
  - A claim of absence cannot be positively proven, but my searches are consistent with it: the closest data found (SO's May 2026 'Agents on a leash' post, DORA 2025, Stacklok 2026) measures individual agent usage and org-level MCP production, never per-team daily-agentic-user density or cross-person session friction. Reasonable to treat as a genuine gap.
  - Evidence: <https://stackoverflow.blog/2026/05/27/agents-on-a-leash-agentic-ai-remains-mostly-monitored-at-work/>

Unsourced load-bearing prose claims:

- ⚠️ Teams average ~5-10 devs, yielding 'roughly 3-5M dev teams worldwide' — no evidence item supports any average team size; the entire team-count denominator is unsourced.
- ⚠️ 'Teams of 2-50 devs with 2+ daily AI users plausibly number ~1M (10^6)' — flagged as extrapolation in the text but the 2-50-dev size-distribution cut has no source at all.
- ⚠️ 'Low-ops paths reach roughly 3-10x more users than self-host-only' — the 3-10x multiplier is invented; n8n's 55%-cloud revenue split measures revenue, not user reach, and Plausible's split is not quantified anywhere in the brief.
- ⚠️ 'Claude Code — the tool with the steepest 2026 growth curve' — comparative claim vs Cursor/Copilot/Codex with no evidence item (Copilot's paid base grew 75% YoY, so 'steepest' needs support).
- ⚠️ 'Realistic obtainable of 10^2-10^3 teams in year one, consistent with early trajectories of comparable dev-infra OSS' — no evidence item covers any comparable project's year-one adoption; the n8n/Gitea/Plausible data are all at-maturity figures.
- ⚠️ 'ChatGPT Enterprise's 150-seat minimum' (open_questions) — asserted with no evidence item and not verified anywhere in the brief.
- ⚠️ 'Stack Overflow 2026 survey (in field July 2026)' — no evidence item; the only related datapoint found is SO's April 2026 standalone AI-agents survey, which is not the annual survey.
- ⚠️ 'Stacklok's 41%' is applied to 'MCP-forward teams' sizing, but Stacklok surveyed organizations, not teams — the org-to-team conversion in the 50k-200k serviceable estimate is unstated and unsourced.
