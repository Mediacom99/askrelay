# Open-source licensing and governance options for the AI-session-to-AI-session messaging tool (permissive vs. copyleft vs. source-available, CLA vs. DCO, trademark, governance signals)

> Decision brief — research sweep of 2026-07-14. Every key fact cites a live source
> fetched that day; an independent verifier re-checked each one (verdicts below).

## TL;DR

The agentic-AI ecosystem the tool must live in is uniformly permissive: MCP SDKs are MIT/Apache-2.0 under the Linux Foundation's Agentic AI Foundation (Dec 2025), A2A is Apache-2.0 under the LF, iroh is dual MIT/Apache-2.0, go-libp2p is MIT. The hosted-relay clone risk is real in principle but the 2023-2026 precedents (HashiCorp BSL, Sentry FSL, Element AGPL) show that restrictive licenses cost more in community trust and enterprise adoption than clone protection is worth for a pre-traction project — and trademark, not copyright, is the tool that actually stops a clone from stealing the name. Recommended: Apache-2.0 everywhere, DCO (no CLA), register the trademark before launch, org-owned repo with a published governance ladder; a split AGPL-relay variant is the credible fallback if the meeting insists on clone protection.

## What it is

This brief covers four coupled choices the meeting must make: (1) outbound license — permissive (MIT/Apache-2.0), copyleft on the server (AGPLv3), or source-available (BSL/FSL); (2) inbound contribution mechanism — CLA (a signed grant that preserves the maintainer's right to relicense or dual-license later) vs. DCO (a per-commit `Signed-off-by` certification, Linux-kernel style, that keeps copyright with contributors and makes future relicensing effectively impossible once outside contributions land); (3) trademark — whether to register the project name and publish a usage policy, which is legally independent of the code license (Linux Foundation guidance is explicit that an open-source copyright license grants no trademark rights); and (4) governance shape — personal repo vs. org-owned repo with public maintainer/decision docs vs. eventual foundation home (LF/AAIF, CNCF). The comparables split cleanly: protocol/SDK projects seeking ecosystem adoption are permissive and foundation-hosted (MCP, A2A); infrastructure companies defending a hosted business went copyleft or source-available and paid for it in fallout (Element/AGPL+CLA, HashiCorp/BSL→OpenTofu fork, Sentry/FSL which is explicitly "Fair Source", not open source); Tailscale runs a mixed model — open client and DERP relay, proprietary coordination server, with the independent Headscale reimplementation tolerated and even quietly supported. mcp_agent_mail is the anti-pattern: single maintainer, MIT-with-a-rider banning OpenAI/Anthropic use, making it non-open-source and unusable inside the ecosystems it targets.

## Maturity

The legal instruments themselves are all mature and battle-tested as of mid-2026. Apache-2.0 (2004) is the AAIF/LF default; the MCP TypeScript SDK now takes new contributions under Apache-2.0 atop its MIT base (v1.29.0, 2026-03-30), and A2A shipped v1.0.1 under Apache-2.0 on 2026-05-28. DCO v1.1 dates to the Linux kernel era and is the visible 2025 trend direction: Spring dropped its CLA for DCO on 2025-01-06, and OpenInfra/OpenStack followed mid-2025. AGPLv3 (2007) got its highest-profile recent use in Element's Synapse relicense (announced 2023-11-06, effective 2023-12-13) — three years on, Synapse development continues at element-hq but the Matrix.org Foundation publicly wrestled with what "core project" means afterward. BSL 1.1 (HashiCorp, 2023-08-10) produced the OpenTofu fork, which reached CNCF Sandbox on 2025-04-23 with enterprise migrations — the clearest evidence that restrictive relicensing of community infrastructure backfires. FSL (Sentry, 2023-11-17; 2-year conversion to Apache-2.0/MIT) is stable but self-described Fair Source, not open source. The AAIF (2025-12-09) is brand new but LF-backed with Anthropic, OpenAI, Block, Google, Microsoft, and AWS involvement — a plausible eventual home if the project gets traction.

## What adopting it buys us

Choosing Apache-2.0 + DCO + registered trademark now buys four concrete things. First, ecosystem compatibility both directions: every dependency named in the transport and MCP briefs is permissive (go-libp2p MIT v0.48.0; iroh dual MIT/Apache-2.0 v1.0.2, 2026-07-06; MCP SDKs MIT/Apache-2.0; A2A Apache-2.0), so any of our license options can consume them — but only a permissive outbound license lets our code flow back into MCP/A2A-adjacent projects and be embedded by vendors like Cursor or Codex-side tooling, which is exactly the adoption path this product needs. Second, frictionless employer adoption: the first users are colleagues at one company; Apache-2.0 with its explicit patent grant is the license enterprise OSS policies whitelist by default, and DCO means no legal-department CLA review before a first PR (Spring's stated reason for dropping its CLA). Third, credibility that is hard to fake: DCO without a CLA makes a future HashiCorp/Element-style relicense practically impossible once outside contributions exist — a verifiable commitment, not a promise. Fourth, real clone defense where it matters: LF guidance and the WP Engine litigation (preliminary injunction against Automattic, 2025-12-10) both show the name, not the code, is the defensible asset — a registered trademark plus published policy stops a hosted clone from marketing itself as the project, which is the damage that actually hurts.

## What it costs us

Apache-2.0 permits exactly the scenario the agenda worries about: any vendor or SaaS can run a hosted relay commercially without contributing back — Tailscale-style "open client, proprietary control plane" businesses could even build on our protocol. Mitigations are trademark (they can't use the name), network effects, and the fact that a relay for a small-team tool has near-zero standalone monetization value today; but if a commercial hosted service later becomes our own business plan, we will have permanently given up the Element/Sentry defensive options — DCO means no relicensing without contacting every contributor. The alternatives cost more: AGPL on the relay chills enterprise contribution (many corporate OSS policies ban AGPL outright), and pairing it with a CLA — required if we ever want dual-licensing — is precisely the combination the Matrix community flagged as bad faith in 2023. FSL/BSL forfeits the "open source from day one" positioning entirely (Sentry itself labels FSL "Fair Source") and invites an OpenTofu-style hostile fork if the project matters. Direct costs of the recommended package: a US trademark registration (roughly $350-750/class plus any attorney fees — exact current USPTO fees not verified here), the discipline of org-owned governance docs, and accepting that "we can always tighten the license later" is off the table — later loosening is easy, later tightening is the reputation-destroyer every 2023-2026 precedent demonstrates.

## Recommendation

Pick Package A: Apache-2.0 for everything (clients, SDK, protocol docs via CC-BY, and the relay), DCO enforced by the GitHub DCO app from the first external PR, no CLA ever, repo under a neutral GitHub org (not a personal account), a MAINTAINERS/GOVERNANCE.md stating the decision process, and register the chosen name as a trademark (US at minimum) before public launch with a published trademark policy. This matches MCP/A2A norms, maximizes employer adoptability for the first users, and makes the no-rug-pull promise structurally verifiable. Fallback Package B, only if the meeting weights hosted-relay cloning heavily: Apache-2.0 clients/SDK/protocol + AGPLv3 relay, still DCO-only — explicitly rejecting Element's CLA so dual-licensing is impossible and the copyleft reads as protective rather than monetization-in-waiting; accept that some enterprises will self-host but not contribute to the relay. Explicitly reject Package C (FSL/BSL source-available relay): it contradicts "ships as open source from day one," and HashiCorp→OpenTofu shows the fork risk lands on us, not the cloner. Whichever package is chosen, decide it in this meeting — with DCO the license becomes effectively immutable at first outside contribution — and put "donate to LF/AAIF or CNCF Sandbox at N maintainers/M orgs" on the roadmap as the enterprise-governance signal, since single-maintainer projects (cf. mcp_agent_mail) read as adoption risk regardless of license.

## Key facts

1. **The MCP TypeScript SDK is licensed 'Apache License 2.0 for new contributions, with existing code under MIT'; latest release v1.29.0 on 2026-03-30, governed by the modelcontextprotocol org.**
   - Source: <https://github.com/modelcontextprotocol/typescript-sdk>
   - Verified: WebFetched the repo page; license statement and v1.29.0 release date read directly from it.
2. **MCP joined the Agentic AI Foundation (a directed fund under the Linux Foundation co-founded by Anthropic, Block, OpenAI) on 2025-12-09, with maintainers retaining full technical autonomy.**
   - Source: <https://blog.modelcontextprotocol.io/posts/2025-12-09-mcp-joins-agentic-ai-foundation/>
   - Verified: WebFetched the official MCP blog post; date, LF structure, and governance-autonomy language confirmed.
3. **A2A is Apache-2.0 under the Linux Foundation (contributed by Google); latest release v1.0.1 on 2026-05-28.**
   - Source: <https://github.com/a2aproject/A2A>
   - Verified: WebFetched the repo; Apache-2.0 badge, LF affiliation statement, and release date shown.
4. **mcp_agent_mail is licensed 'MIT License (with OpenAI/Anthropic Rider) Copyright (c) 2026 Jeffrey Emanuel' — a non-OSI license banning use by OpenAI/Anthropic and affiliates; single-maintainer project.**
   - Source: <https://raw.githubusercontent.com/Dicklesworthstone/mcp_agent_mail/main/LICENSE>
   - Verified: WebFetched the raw LICENSE file; rider text and copyright line quoted directly.
5. **Element relicensed Synapse and friends from Apache-2.0 to AGPLv3 (announced 2023-11-06, Synapse effective 2023-12-13) with an ASF-style CLA granting Element dual-licensing rights to sell exceptions; the Matrix.org Foundation said it 'would prefer these projects be unencumbered by CLA'.**
   - Source: <https://element.io/blog/element-to-adopt-agplv3/>
   - Verified: WebFetched Element's announcement for dates/CLA terms; foundation reaction from The Register/matrix.org coverage surfaced via WebSearch.
6. **iroh is dual-licensed MIT OR Apache-2.0, maintained by N0, Inc.; latest release v1.0.2 on 2026-07-06.**
   - Source: <https://github.com/n0-computer/iroh>
   - Verified: WebFetched the repo; dual-license statement and release version/date shown.
7. **Tailscale's model: client daemon and DERP relay servers are open source, the coordination/control server is proprietary, and Headscale is an 'independent, community-maintained' open-source coordination server whose lead maintainer Tailscale employs without directing the project.**
   - Source: <https://tailscale.com/opensource>
   - Verified: WebFetched Tailscale's official open-source page; all three points stated there.
8. **go-libp2p (the Go transport candidate) is MIT-licensed; latest release v0.48.0 on 2026-03-17 — all named Go/TS dependencies are permissive, so any candidate outbound license can consume them.**
   - Source: <https://github.com/libp2p/go-libp2p>
   - Verified: WebFetched the repo; MIT license and v0.48.0 release date shown.
9. **Apache-2.0 compatibility with GPLv3/AGPLv3 is one-way: Apache-2.0 code can be included in GPLv3 works, but GPLv3 code cannot be included in Apache projects — so an AGPL relay can use every listed dependency, but an Apache-2.0 codebase must never import AGPL libraries.**
   - Source: <https://www.apache.org/licenses/GPL-compatibility.html>
   - Verified: WebFetched the ASF's GPL-compatibility page; directionality quoted directly.
10. **HashiCorp moved Terraform to BSL 1.1 on 2023-08-10; the OpenTofu fork (kept MPL-2.0) was accepted as a CNCF Sandbox project on 2025-04-23, demonstrating that restrictive relicensing of adopted infrastructure triggers viable hostile forks.**
   - Source: <https://www.cncf.io/projects/opentofu/>
   - Verified: WebFetched HashiCorp's license FAQ (BSL 1.1, Aug 2023 announcement) and the CNCF project page (Sandbox, accepted 2025-04-23).
11. **Sentry introduced the FSL on 2023-11-17 (replacing its BSL) — non-compete restriction converting to Apache-2.0/MIT after 2 years — and frames it as Fair Source, i.e., explicitly not open source.**
   - Source: <https://blog.sentry.io/introducing-the-functional-source-license-freedom-without-free-riding/>
   - Verified: WebFetched Sentry's announcement and fsl.software; date, terms, and Fair Source framing confirmed.
12. **The 2025 direction of travel is CLA→DCO: Spring replaced its CLA with DCO on 2025-01-06 citing legal-review friction and employer-approval burden, with OpenInfra/OpenStack following mid-2025; DCO v1.1 is the Linux Foundation standard requiring only 'git commit -s'.**
   - Source: <https://spring.io/blog/2025/01/06/hello-dco-goodbye-cla-simplifying-contributions-to-spring>
   - Verified: WebFetched Spring's announcement and developercertificate.org (DCO v1.1 text); OpenInfra/OpenStack 2025 moves surfaced via WebSearch of their governance pages.
13. **Linux Foundation guidance: an open-source copyright license carries no implied trademark rights, and registering the mark 'enables us to better protect the community against misrepresentation, misuse, and confusion' — neutral trademark ownership (e.g., Kubernetes in LF) is the model; the WP Engine case (preliminary injunction ordering Automattic to restore access, 2025-12-10) shows the fallout when trademark control is ambiguous and centrally weaponized.**
   - Source: <https://www.linuxfoundation.org/blog/blog/open-source-communities-and-trademarks-a-reprise>
   - Verified: LF trademark guidance located and quoted via WebSearch of linuxfoundation.org; WP Engine injunction date from TechCrunch/court coverage in WebSearch results (community/press sources, labeled as such).

## Open questions

- Does the team ever intend a commercial hosted relay? If yes, Package B (AGPL relay) vs. A changes materially — this is a business decision the meeting must make before the license locks in at first outside contribution.
- Exact current USPTO trademark filing fees and whether EU registration is warranted given the team is Italy-based (kosmoy.com) — not verified in this pass; a fee schedule check and a knockout search on the chosen name (separate naming brief) are needed before filing.
- The first users' employer's actual OSS policy: does it whitelist Apache-2.0 contribution by default, and does it ban AGPL use or only AGPL contribution? Asking their legal/OSPO directly beats assuming.
- Whether A2A/LF projects require a CLA in practice (A2A's CONTRIBUTING.md was not fetched; LF projects vary between DCO and corporate CLA) — worth checking if we model governance docs on A2A.
- Whether the Agentic AI Foundation accepts small external projects (its intake criteria are not yet published anywhere I could verify) — the 'donate later' plank of the recommendation currently assumes it or CNCF Sandbox remains open to this class of project.
- If protocol docs/spec are split from code, whether to use CC-BY-4.0 (A2A/MCP pattern) or keep everything Apache-2.0 — minor, but should be fixed in the repo scaffold.

## Independent verification

**Overall:** This brief is highly reliable on its sourced facts: 12 of 13 key_facts were confirmed directly against their cited sources, usually verbatim (licenses, release versions and dates, foundation affiliations, the Element CLA terms, the ASF one-way-compatibility language, the Spring and OpenStack DCO dates, the CNCF OpenTofu acceptance). The one FAILED item is a real error, though contained: the WP Engine preliminary injunction against Automattic is dated 2025-12-10 in the brief but was actually issued 2024-12-10 — a one-year slip repeated in the prose, likely picked up from a mis-dated secondary blog; the argumentative use of the case survives the correction. Two soft spots inside otherwise-confirmed facts: the "Fair Source" framing comes from fsl.software rather than Sentry's 2023 announcement itself (the brief cited both, so this is a nuance, not an error), and mcp_agent_mail's "single-maintainer" status was asserted but not directly countable from the fetched page. The larger reliability risk is in the prose, not the key_facts: several load-bearing generalizations that drive the recommendation — enterprise policies banning AGPL, Apache-2.0 being whitelisted by default, "relicensing practically impossible" under DCO, the CC-BY "A2A/MCP pattern" for spec docs, and OpenTofu "enterprise migrations" — carry no key_fact at all and should be treated as informed argument rather than verified fact. Net: safe to use for the meeting, with the injunction date corrected to December 10, 2024 and the unsourced enterprise-policy and CC-BY claims either verified or downgraded to stated assumptions.

Verdicts: CONFIRMED 12, FAILED 1

- ✅ **CONFIRMED** — MCP TypeScript SDK licensed 'Apache 2.0 for new contributions, existing code under MIT'; latest v1.29.0 on 2026-03-30; modelcontextprotocol org
  - Repo states verbatim 'licensed under the Apache License 2.0 for new contributions, with existing code under MIT'; latest release v1.29.0 dated March 30, 2026; governed by the modelcontextprotocol org. Note the brief's tldr generalizes to 'MCP SDKs' plural — only the TS SDK was verified.
  - Evidence: <https://github.com/modelcontextprotocol/typescript-sdk>
- ✅ **CONFIRMED** — MCP joined the Agentic AI Foundation (directed fund under Linux Foundation, co-founded by Anthropic, Block, OpenAI) on 2025-12-09 with maintainers retaining full technical autonomy
  - Official MCP blog post dated Dec 9, 2025 confirms all elements: 'a directed fund under the Linux Foundation, co-founded by Anthropic, Block and OpenAI, with support from Google, Microsoft, AWS, Cloudflare and Bloomberg'; projects 'maintain full autonomy over their technical direction'.
  - Evidence: <https://blog.modelcontextprotocol.io/posts/2025-12-09-mcp-joins-agentic-ai-foundation/>
- ✅ **CONFIRMED** — A2A is Apache-2.0 under the Linux Foundation (contributed by Google); latest release v1.0.1 on 2026-05-28
  - Repo states 'licensed under the Apache License 2.0' and 'an open source project under the Linux Foundation, contributed by Google'; releases show v1.0.1 on May 28, 2026.
  - Evidence: <https://github.com/a2aproject/A2A>
- ✅ **CONFIRMED** — mcp_agent_mail licensed 'MIT License (with OpenAI/Anthropic Rider) Copyright (c) 2026 Jeffrey Emanuel', rider banning OpenAI/Anthropic and affiliates; single-maintainer
  - Raw LICENSE confirms the exact title, copyright line, and a rider denying all rights to 'Restricted Parties' (OpenAI, Anthropic, affiliates, agents), including ML-training use. The 'single-maintainer' descriptor was not directly countable from the repo page fetch (owner Dicklesworthstone = Jeffrey Emanuel is consistent with it), so that sub-claim rests on weaker evidence, but the load-bearing license facts are verbatim correct.
  - Evidence: <https://raw.githubusercontent.com/Dicklesworthstone/mcp_agent_mail/main/LICENSE>
- ✅ **CONFIRMED** — Element relicensed Synapse etc. Apache-2.0 to AGPLv3 (announced 2023-11-06, Synapse effective 2023-12-13) with ASF-style CLA enabling dual-licensing; Matrix.org Foundation would prefer projects 'unencumbered by CLA'
  - Element's post (Nov 6, 2023) confirms Apache→AGPLv3, Synapse AGPL as of Dec 13, 2023, a CLA 'based on the Apache Software Foundation's CLA' whose stated purpose is to let Element dual-license/sell commercial exceptions. The Register's coverage (theregister.com/2023/11/06/element_moves_to_agplv3/) confirms the Foundation's preference for projects 'open source, and unencumbered by a CLA' and that it finds DCOs sufficient.
  - Evidence: <https://element.io/blog/element-to-adopt-agplv3/>
- ✅ **CONFIRMED** — iroh is dual MIT OR Apache-2.0, maintained by N0, Inc.; latest release v1.0.2 on 2026-07-06
  - Repo shows dual Apache-2.0/MIT licensing, copyright 'N0, INC', and release v1.0.2 dated July 6, 2026.
  - Evidence: <https://github.com/n0-computer/iroh>
- ✅ **CONFIRMED** — Tailscale: open-source client daemon and DERP relays, proprietary coordination server; Headscale is an independent community-maintained coordination server whose lead maintainer Tailscale employs without directing the project
  - Tailscale's own page states all three points, including 'independent, community-maintained, open-source coordination server' and that Tailscale employs the lead maintainer but 'does not direct or steer the project'.
  - Evidence: <https://tailscale.com/opensource>
- ✅ **CONFIRMED** — go-libp2p is MIT-licensed; latest release v0.48.0 on 2026-03-17
  - Repo states 'go-libp2p is MIT-licensed open source software'; v0.48.0 released March 17, 2026. Also matches the pin recorded in this project's own dependency matrix (verified 2026-07-09).
  - Evidence: <https://github.com/libp2p/go-libp2p>
- ✅ **CONFIRMED** — Apache-2.0/GPLv3 compatibility is one-way: Apache-2.0 code can go into GPLv3 works but not vice versa
  - ASF page states verbatim: 'Apache 2 software can therefore be included in GPLv3 projects' and 'GPLv3 software cannot be included in Apache projects. The licenses are incompatible in one direction only.' The extension to AGPLv3 is the brief's (standard, correct) inference — the ASF page discusses GPLv3.
  - Evidence: <https://www.apache.org/licenses/GPL-compatibility.html>
- ✅ **CONFIRMED** — HashiCorp moved Terraform to BSL 1.1 on 2023-08-10; OpenTofu fork (kept MPL-2.0) accepted as CNCF Sandbox on 2025-04-23
  - CNCF page: 'OpenTofu was accepted to CNCF on April 23, 2025 at the Sandbox maturity level.' HashiCorp's BSL announcement date of Aug 10, 2023 confirmed via HashiCorp's own blog and press coverage (theregister.com/2023/08/11/hashicorp_bsl_licence/, infoq.com); OpenTofu repo confirms MPL-2.0.
  - Evidence: <https://www.cncf.io/projects/opentofu/>
- ✅ **CONFIRMED** — Sentry introduced FSL on 2023-11-17 (replacing BSL), 2-year conversion to Apache-2.0/MIT, framed as Fair Source i.e. not open source
  - Sentry blog (Nov 17, 2023) confirms FSL replacing BSL and the 2-year conversion to Apache-2.0 or MIT. Caveat: the 2023 announcement itself does not use the term 'Fair Source' (that branding came later); the 'Fair Source license... converts to Apache 2.0 or MIT after two years' framing and the 'Why not Open Source?' FAQ are on fsl.software, which the brief also cited. Substance correct.
  - Evidence: <https://blog.sentry.io/introducing-the-functional-source-license-freedom-without-free-riding/>
- ✅ **CONFIRMED** — 2025 CLA→DCO trend: Spring replaced CLA with DCO on 2025-01-06 citing legal-review/employer-approval friction; OpenInfra/OpenStack followed mid-2025; DCO v1.1 requires only 'git commit -s'
  - Spring post dated Jan 6, 2025 confirms DCO replacing CLA, cites CLAs as lengthy custom legal documents requiring employer approval, and references 'git commit -s'. OpenStack TC resolution 2025-05-20 set DCO effective July 1, 2025 (governance.openstack.org/tc/resolutions/20250520-replace-the-cla-with-dco-for-all-contributions.html; openinfra.org/dco/) — 'mid-2025' is accurate. Minor nit: DCO is stewarded via developercertificate.org/Linux Foundation; calling it 'the Linux Foundation standard' is a fair simplification.
  - Evidence: <https://spring.io/blog/2025/01/06/hello-dco-goodbye-cla-simplifying-contributions-to-spring>
- ❌ **FAILED** — LF trademark guidance (no implied trademark rights; 'enables us to better protect the community against misrepresentation, misuse, and confusion'; Kubernetes neutral-ownership model) plus WP Engine preliminary injunction against Automattic dated 2025-12-10
  - LF page verified directly (exact quote present; Kubernetes example present, including Google insisting LF own the mark). Injunction date cross-checked against TechCrunch (Dec 10, 2024) and The Register (theregister.com/2024/12/11/wp_engine_wins_injunction_against/). One mis-dated secondary blog (365i.co.uk, '2025/12/11') may be the source of the error. The substantive lesson drawn from the case is unaffected, but the date as stated is wrong by a year.
  - **Correction:** The LF guidance half is accurate — the blog contains the quoted protection language and the Kubernetes neutral-trademark-ownership discussion — but the WP Engine preliminary injunction (Judge Araceli Martínez-Olguín, N.D. Cal., ordering Automattic/Mullenweg to restore WP Engine's access to WordPress.org within 72 hours) was issued 2024-12-10, not 2025-12-10. The brief repeats the wrong year in 'what_adopting_buys_us' as well.
  - Evidence: <https://techcrunch.com/2024/12/10/court-orders-mullenweg-and-automattic-to-restore-wp-engines-access-to-wordpress-org/>

Load-bearing claims in the prose **without** a sourced key fact:

- ⚠️ "Many corporate OSS policies ban AGPL outright" (what_it_costs_us) — load-bearing for rejecting Package B, but no key_fact or source (Google's AGPL ban is the usual citation; not referenced).
- ⚠️ "Apache-2.0 ... is the license enterprise OSS policies whitelist by default" (what_adopting_buys_us) — asserted without any key_fact or survey source.
- ⚠️ OpenTofu reached Sandbox "with enterprise migrations" (maturity) — the enterprise-migration part has no key_fact behind it; only the CNCF acceptance is sourced.
- ⚠️ "The Matrix.org Foundation publicly wrestled with what 'core project' means afterward" (maturity) — no source; the sourced Foundation reaction is only the CLA-preference quote.
- ⚠️ "Three years on, Synapse development continues at element-hq" (maturity) — current-state claim with no key_fact.
- ⚠️ US trademark registration "roughly $350-750/class" (what_it_costs_us) — numeric fee range with no key_fact; the brief itself flags USPTO fees as unverified, but the range is still presented as an anchor. (Current USPTO base filing fee is $350/class as of the 2025 fee schedule, so the range is plausible but unsourced.)
- ⚠️ Protocol docs "via CC-BY" described as the "A2A/MCP pattern" (recommendation/open_questions) — that A2A/MCP license their specs CC-BY-4.0 is never sourced; the MCP spec repo is not CC-BY as far as public records show, so this pattern claim needs checking before the repo scaffold copies it.
- ⚠️ "DCO ... makes a future relicense practically impossible" (tldr/what_adopting_buys_us) — a legal characterization stated as fact with no source; relicensing is hard (requires consent of all copyright holders) but not literally impossible, and no authority is cited.
- ⚠️ tldr's "MCP SDKs are MIT/Apache-2.0" (plural) — only the TypeScript SDK is backed by a key_fact; the Python/other SDK licenses are never sourced.
