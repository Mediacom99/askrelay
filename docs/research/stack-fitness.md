# Stack fitness: Go vs TypeScript (vs Python) for the local attach component and the shared relay

> Decision brief — research sweep of 2026-07-14. Every key fact cites a live source
> fetched that day; an independent verifier re-checked each one (verdicts below).

## TL;DR

The Go default survives for the local attach component: the official MCP Go SDK is currently the healthiest of the three (stable line, Google co-maintained, already carrying next-spec support with backward compatibility), while the TypeScript and Python SDKs are both mid-v2 rewrites whose stable lines will not get the 2026-07-28 spec. Go also wins distribution (trivial cross-compile, GoReleaser-automated brew/curl installs vs Node-runtime-dependent npx or immature single-binary tooling) and is the only language with an embedded Tailscale option (tsnet). The one hard exception: if the relay lands on Cloudflare Durable Objects as the transport brief favors, that component must be TypeScript — Go runs there only via Wasm. TypeScript's real edge is contributor pool (roughly 3x Go's usage share), which is a tradeoff, not a blocker.

## What it is

A language/stack decision for two deliverables: (1) a local attach component each colleague runs — most likely an MCP server or small daemon where single-binary install friction dominates — and (2) a shared relay. The comparison rests on live-verified maturity of the official MCP SDKs (modelcontextprotocol org: go-sdk, typescript-sdk, python-sdk), A2A SDKs (a2aproject: a2a-go, a2a-js, a2a-python), the transport libraries the transport brief surfaced (Tailscale tsnet, Matrix, NATS, Slack, Cloudflare Workers), plus cross-compilation/distribution mechanics and OSS contributor accessibility. Python is considered only where relevant: its MCP SDK is also mid-rewrite (stable v1.28.0 of 2026-06-16, v2.0.0b2 of 2026-07-14, with release notes admitting the new spec "means replacing the SDK's core") and its distribution story for a run-everywhere daemon is the weakest of the three, so it is out of contention for the shipped binaries though fine for future agent examples and scripts.

## Maturity

MCP SDKs (all verified 2026-07-15): Go SDK — stable v1.6.1 (2026-05-22), v1.7.0-pre.2 (2026-07-09) with full 2026-07-28 spec support and "backward compatibility with 2025-11-25 and earlier preserved on every endpoint"; repo pushed 2026-07-14, 4.8k stars, described as "maintained in collaboration with Google". TypeScript SDK — last stable v1.29.0 (2026-03-30); everything since is the v2.0.0-beta rewrite (beta.4, 2026-07-13) split into new packages (@modelcontextprotocol/core, /server, /node, /express, …); README says v1.x "remains the supported release for production" and recommends new projects stay on v1 until v2 stabilizes (target 2026-07-28). Python SDK — stable v1.28.0 (2026-06-16), v2.0.0b2 (2026-07-14). A2A: a2a-go stable v2.3.1 (2026-05-13, A2A v1.0-compliant); a2a-js still v0.3.14 stable (2026-07-09) with v1.0 only at beta.0 (2026-07-01). Transport clients are healthy in both languages: nats.go v1.52.0 (2026-05-07) / nats.js v3.4.0 (2026-05-08); slack-go v0.27.0 (2026-06-27) / bolt-js v4.7.3 (2026-05-27); mautrix-go v0.28.1 (2026-06-16) / matrix-js-sdk v41.9.0 (2026-07-07). tsnet is Go-only; libtailscale has no JS/TS binding and zero releases.

## What adopting it buys us

Keeping Go for the attach component buys us: the strongest official MCP SDK right now — the only one whose stable release line already tracks the incoming 2026-07-28 spec with explicit backward compatibility, versus TypeScript/Python where next-spec support requires adopting a beta core rewrite during our v1 build window; first-class A2A support (a2a-go v2.3.1 stable and v1.0-compliant while a2a-js v1.0 is still beta) if we later bridge to the winning agent-interop protocol; the only embedded-Tailscale option (tsnet) among the transport brief's candidates; and a materially better distribution story for mixed-tooling colleagues — GOOS/GOARCH cross-compilation is native, GoReleaser auto-publishes Homebrew casks and curl-able archives, and users need no runtime. By contrast npx requires a working Node install, Node's own single-executable feature is Stability 1.1 "Active development" with macOS x64 untested, and Bun's --compile cross-compiles but produces binaries the Bun team itself calls "way too big." Choosing TypeScript instead would buy a larger contributor funnel (TS 43.6% vs Go 16.4% usage in the SO 2025 survey; TS became GitHub's #1 language in Aug 2025) and native fit with Cloudflare Workers/Durable Objects.

## What it costs us

Go's costs are real but bounded. Contributor accessibility: Go's user base is roughly a third of TypeScript's (16.4% vs 43.6% of all SO 2025 respondents; 17.4% vs 48.8% of professionals), and the AI-agent community skews TS/Python, so an all-Go repo narrows the drive-by-contributor pool for an open-source project whose audience is exactly that community. Cloudflare exclusion: Workers' first-class languages are JS/TS, Python, and Rust; Go runs only compiled to Wasm — so the transport brief's cheapest relay option (a Durable Object with WebSocket hibernation, effectively free at team scale) cannot reasonably be Go, forcing either a two-language repo (Go daemon + small TS relay) or a Go relay binary on a paid VPS. Ecosystem breadth: the MCP TS SDK has the largest install base and framework adapters (express/fastify/hono in v2), and most MCP server examples users copy are npx-based, so a Go server is slightly off the beaten path for docs and issue reports. One sibling-brief claim we must retract: nats.js and bolt-js are NOT stale (both released May 2026, verified) — TypeScript's transport-client ecosystem is fine, so that argument for Go is off the table.

## Recommendation

Confirm Go for the local attach component; make the relay's language follow the hosting decision rather than the other way around. The attach component is where install friction, runtime-free distribution, and MCP SDK stability actually bite, and Go wins all three today on live evidence: the go-sdk is the only official SDK whose stable line spans current and next spec (both TS and Python force a beta v2 rewrite adoption to track 2026-07-28), cross-compiled static binaries plus GoReleaser give the brew/curl one-liner we want across colleagues' mixed machines, and tsnet keeps the Tailscale option open. For the relay: if the meeting picks Cloudflare Durable Objects (the transport brief's cost favorite), accept a small TypeScript relay in the same monorepo — this is a ~hundreds-of-lines component and bilingual repos of this shape are common; if the meeting picks a VPS, keep it Go and stay single-language. Do not choose TypeScript for the daemon primarily to widen the contributor pool: that benefit is speculative for a young project, while the TS SDK's v2 transition risk and Node-runtime install requirement are concrete costs during exactly our build window. Revisit only if the TS SDK v2 ships stable on schedule (2026-07-28) and we find ourselves rejecting TS contributors.

## Key facts

1. **The official MCP Go SDK is described by the project as 'The official Go SDK for Model Context Protocol servers and clients. Maintained in collaboration with Google', with 4.8k stars and last push 2026-07-14.**
   - Source: <https://api.github.com/repos/modelcontextprotocol/go-sdk>
   - Verified: Fetched the GitHub API repo object; description, stargazers_count (4,798), and pushed_at (2026-07-14) read directly.
2. **MCP Go SDK: latest stable v1.6.1 (2026-05-22); v1.7.0-pre.2 (2026-07-09) adds full support for the next spec revision 2026-07-28 while 'backward compatibility with 2025-11-25 and earlier is preserved on every endpoint' — next-spec support lands in the same v1 line, not a rewrite.**
   - Source: <https://api.github.com/repos/modelcontextprotocol/go-sdk/releases>
   - Verified: Fetched the releases API: tags, dates, prerelease flags, and the v1.7.0-pre.1 backward-compatibility statement quoted from release notes.
3. **MCP TypeScript SDK: last stable release is v1.29.0 (2026-03-30); all releases since are the v2.0.0-beta package-split rewrite (beta.4 on 2026-07-13 across @modelcontextprotocol/core, /server, /node, /express, /fastify, /hono), and the README says v1.x 'remains the supported release for production', recommending new projects stay on v1 until v2 stabilizes (~2026-07-28).**
   - Source: <https://github.com/modelcontextprotocol/typescript-sdk>
   - Verified: Fetched releases/latest (v1.29.0, 2026-03-30), the releases list (only 2.0.0-beta entries since), and the raw README (v1-for-production and v2-beta statements).
4. **MCP Python SDK is likewise mid-rewrite: stable v1.28.0 (2026-06-16), v2.0.0b2 (2026-07-14), with release notes stating 'The v1 SDK is built around long-lived sessions, so supporting the new spec means replacing the SDK's core.'**
   - Source: <https://api.github.com/repos/modelcontextprotocol/python-sdk/releases>
   - Verified: Fetched the releases API; versions, dates, prerelease flags, and the quoted rationale from the v2 beta notes.
5. **A2A SDKs: a2a-go stable v2.3.1 (2026-05-13, A2A v1.0-compliant); a2a-js latest stable is still v0.3.14 (2026-07-09) with v1.0 only at v1.0.0-beta.0 (2026-07-01) — Go leads TypeScript on A2A readiness.**
   - Source: <https://api.github.com/repos/a2aproject/a2a-js/releases>
   - Verified: Fetched both a2aproject/a2a-go and a2aproject/a2a-js releases APIs; tags, dates, and prerelease flags read directly.
6. **tsnet, which embeds a Tailscale node in-process with no separate daemon, is Go-only; Tailscale's libtailscale C library shows bindings for Swift/Python/Ruby but none for JS/TS and has zero published releases.**
   - Source: <https://tailscale.com/kb/1244/tsnet>
   - Verified: Fetched the official tsnet KB page ('lets you embed Tailscale inside a Go program', no other languages mentioned) and the github.com/tailscale/libtailscale repo page (language breakdown, no releases, no JS/TS binding).
7. **Cloudflare Workers' first-class languages are JavaScript, TypeScript, Python Workers, and Rust; Go is supported only by compiling to WebAssembly — a Durable Objects relay is effectively a TypeScript component.**
   - Source: <https://developers.cloudflare.com/workers/languages/>
   - Verified: Fetched the official Workers languages page; first-class list and the Wasm-only path for Go quoted directly.
8. **Node.js Single Executable Applications remain Stability 1.1 'Active development', with CI testing only on Windows, macOS arm64 (x64 unsupported), and most Linux; cross-platform SEA generation requires disabling code cache/snapshots.**
   - Source: <https://nodejs.org/api/single-executable-applications.html>
   - Verified: Fetched the official Node.js API doc; stability index, platform-testing list, and cross-compilation caveat quoted.
9. **Bun's `bun build --compile` does support cross-compilation to Linux/macOS/Windows x64+arm64 (incl. musl), but the docs concede 'Bun's binary is still way too big and we need to make it smaller'.**
   - Source: <https://bun.com/docs/bundler/executables>
   - Verified: Fetched Bun's official executables docs; target list and binary-size admission quoted.
10. **GoReleaser automates publishing Homebrew casks to a tap after a GitHub release, including binaries, completions, and signing/notarization support — giving Go the brew/curl one-liner distribution path.**
   - Source: <https://goreleaser.com/customization/homebrew_casks/>
   - Verified: Fetched the GoReleaser docs page; quoted its statement that it can 'generate and publish a Homebrew Cask into a repository (Tap)' post-release.
11. **Contributor pool skews TypeScript: SO 2025 survey shows TS used by 43.6% of all respondents (48.8% of professionals) vs Go 16.4% (17.4%), and GitHub Octoverse reports TypeScript became the most-used language on GitHub in August 2025.**
   - Source: <https://survey.stackoverflow.co/2025/technology>
   - Verified: Fetched the SO 2025 technology page for the exact percentages; Octoverse claim confirmed via the GitHub blog post 'Octoverse: A new developer joins GitHub every second as AI leads TypeScript to #1' surfaced in search.
12. **Correction to the transport sibling brief: nats.js and bolt-js are not stale — nats.js v3.4.0 shipped 2026-05-08 (JetStream 2.14 features) and bolt-js v4.7.3 shipped 2026-05-27; the sibling's dates were off by a year, so both ecosystems have maintained NATS and Slack clients.**
   - Source: <https://api.github.com/repos/nats-io/nats.js/releases>
   - Verified: Fetched nats-io/nats.js and slackapi/bolt-js releases APIs on 2026-07-15; latest tags and dates read directly, contradicting the sibling brief's 2025 dates.
13. **Transport libraries are otherwise healthy in both languages: nats.go v1.52.0 (2026-05-07), slack-go v0.27.0 (2026-06-27), mautrix-go v0.28.1 with E2EE (2026-06-16), and matrix-js-sdk v41.9.0 (2026-07-07) on a biweekly cadence.**
   - Source: <https://api.github.com/repos/matrix-org/matrix-js-sdk/releases>
   - Verified: Fetched releases APIs for nats-io/nats.go, slack-go/slack, mautrix/go (releases/latest), and matrix-org/matrix-js-sdk; all tags and dates read directly.

## Open questions

- Does the relay land on Cloudflare Durable Objects (forcing a TypeScript relay component) or a VPS (allowing all-Go)? This is the transport meeting's decision and the only thing that changes the language mix.
- Will the MCP TypeScript SDK v2 actually ship stable on its 2026-07-28 target? If it does and proves solid within a month, the strongest technical argument against TS narrows to distribution only.
- Does the local attach component even need to run on machines of colleagues using only claude.ai/ChatGPT web (who connect to a remote MCP server instead)? If most users never install anything locally, single-binary distribution matters less than assumed.
- The go-sdk's GitHub API license field reads 'Other (NOASSERTION)' — the actual license file (expected MIT) was not read; confirm before depending on it in an OSS project.
- If the Matrix fallback is ever chosen with E2EE, does mautrix-go's encryption path still require cgo/libolm or is the pure-Go implementation (goolm) production-ready? Not verified in this pass.
- Claude Code's proprietary channels extension is stdio-based and language-agnostic per the MCP sibling brief, but no Go-vs-TS implementation friction for it was tested here.

## Independent verification

**Overall:** This brief is unusually reliable at the fact level: all 13 key_facts CONFIRMED against their primary sources, with exact version numbers, dates, prerelease flags, and verbatim quotes checking out (go-sdk description/stars/push date, all six MCP/A2A SDK release timelines, the Python 'replacing the SDK's core' quote, the TS README v1-for-production statement, tsnet/libtailscale, Cloudflare Workers language tiers, Node SEA stability/platform caveats, Bun's binary-size admission, GoReleaser casks, SO 2025 percentages, Octoverse Aug-2025 TypeScript #1, and the nats.js/bolt-js staleness correction). The weaknesses are in the prose glue, not the facts: the recommendation's strongest line — that Go's *stable* line already spans both spec versions — overstates its own evidence (2026-07-28 support is only in v1.7.0 prereleases; stable v1.6.1 lacks it, and the new protocol over HTTP additionally requires stateless mode), the claim that TS/Python stable lines 'will not get' the new spec is inference rather than sourced fact, and several supporting assertions (largest TS install base, npx-based example culture, agent-community language skew, Durable Objects being effectively free, Python's weak distribution) carry no key_fact. None of these flip the recommendation — Go still wins on rewrite-avoidance, distribution, and tsnet — but the framing should be softened from 'stable line already tracks the next spec' to 'same v1 line via prerelease, no rewrite required', and the Cloudflare cost premise should be re-verified in the transport meeting since it is the single fact that forces the bilingual-repo concession.

Verdicts: CONFIRMED 13

- ✅ **CONFIRMED** — MCP Go SDK described as 'The official Go SDK for MCP servers and clients. Maintained in collaboration with Google', 4.8k stars, last push 2026-07-14
  - API returns that exact description, stargazers_count 4798, pushed_at 2026-07-14T16:22:07Z. Also confirms the open question's license concern: license field is 'Other / NOASSERTION'.
  - Evidence: <https://api.github.com/repos/modelcontextprotocol/go-sdk>
- ✅ **CONFIRMED** — Go SDK: stable v1.6.1 (2026-05-22); v1.7.0-pre.2 (2026-07-09) with full 2026-07-28 spec support, 'backward compatibility with 2025-11-25 and earlier is preserved on every endpoint', next-spec in same v1 line
  - Releases API matches all tags/dates. The backward-compat sentence appears verbatim in v1.7.0-pre.1 notes (2026-06-24), which is where full 2026-07-28 support first landed; pre.2 rounds it out. Two nuances the claim omits: v1.7.0 is still a PRERELEASE (latest stable v1.6.1 does not have 2026-07-28), and 2026-07-28 over streamable HTTP requires StreamableHTTPOptions.Stateless=true (stateful sessions negotiate down to 2025-11-25).
  - Evidence: <https://api.github.com/repos/modelcontextprotocol/go-sdk/releases>
- ✅ **CONFIRMED** — MCP TypeScript SDK: last stable v1.29.0 (2026-03-30); everything since is v2.0.0-beta package split (beta.4 2026-07-13 across core/server/node/express/fastify/hono); README says v1.x 'remains the supported release for production', v2 stable expected 2026-07-28
  - releases/latest returns v1.29.0 (2026-03-30); the releases list since then is exclusively 2.0.0-beta packages incl. all six named plus /client, /codemod, /server-legacy. README (main) contains verbatim: 'We expect a stable release alongside the full release of the 2026-07-28 spec on July 28, 2026. Until then, v1.x remains the supported release for production'. The 'recommends new projects stay on v1' is a fair paraphrase but not a literal quote.
  - Evidence: <https://github.com/modelcontextprotocol/typescript-sdk>
- ✅ **CONFIRMED** — MCP Python SDK: stable v1.28.0 (2026-06-16), v2.0.0b2 (2026-07-14), notes stating 'The v1 SDK is built around long-lived sessions, so supporting the new spec means replacing the SDK's core.'
  - Tags/dates match exactly; the quoted sentence appears verbatim in the v2.0.0a1 release notes. b1 (2026-06-30) was the first release with full 2026-07-28 support; stable v2 targeted 2026-07-28.
  - Evidence: <https://api.github.com/repos/modelcontextprotocol/python-sdk/releases>
- ✅ **CONFIRMED** — a2a-go stable v2.3.1 (2026-05-13, A2A v1.0-compliant); a2a-js latest stable v0.3.14 (2026-07-09), v1.0 only at v1.0.0-beta.0 (2026-07-01)
  - Both releases APIs match all tags/dates/prerelease flags. a2a-go README states compliance with the 'Agent2Agent (A2A) v1.0 Protocol Specification' (v2.x SDK line began 2026-03-17). a2a-js v1.0.0-beta.0 is flagged prerelease; v0.3.14 is the newest stable.
  - Evidence: <https://api.github.com/repos/a2aproject/a2a-js/releases>
- ✅ **CONFIRMED** — tsnet embeds Tailscale in-process, Go-only; libtailscale has Swift/Python/Ruby bindings, no JS/TS, zero releases
  - KB page: 'tsnet is a library that lets you embed Tailscale inside a Go program' — no other languages mentioned. github.com/tailscale/libtailscale API: top-level dirs swift/, python/, ruby/ (no JS/TS), releases count = 0.
  - Evidence: <https://tailscale.com/kb/1244/tsnet>
- ✅ **CONFIRMED** — Cloudflare Workers first-class languages are JavaScript, TypeScript, Python Workers, Rust; Go only via compiling to WebAssembly
  - Official page lists exactly those four as first-class and names Go only as an example language compilable to Wasm. The inference that a Durable Objects relay is 'effectively a TypeScript component' is reasonable given the team's stack, though Python/Rust Workers technically exist.
  - Evidence: <https://developers.cloudflare.com/workers/languages/>
- ✅ **CONFIRMED** — Node.js SEA is Stability 1.1 'Active development'; tested on Windows, macOS arm64 (x64 skipped), most Linux; cross-platform SEA requires disabling code cache/snapshots
  - Doc confirms Stability 1.1 Active development; macOS x64 'is skipped' in CI; Linux all distros except Alpine, all arch except s390x; cross-platform builds must set useCodeCache/useSnapshot false.
  - Evidence: <https://nodejs.org/api/single-executable-applications.html>
- ✅ **CONFIRMED** — bun build --compile cross-compiles to Linux/macOS/Windows x64+arm64 incl. musl, but docs say 'Bun's binary is still way too big and we need to make it smaller'
  - Target matrix confirmed (linux x64/arm64 incl. musl, macOS x64/arm64, Windows x64/arm64); the size admission is quoted verbatim from the Minification section.
  - Evidence: <https://bun.com/docs/bundler/executables>
- ✅ **CONFIRMED** — GoReleaser automates publishing Homebrew Casks to a tap after a GitHub release, incl. binaries, completions, signing/notarization
  - Page states GoReleaser 'can generate and publish a Homebrew Cask into a repository (Tap)' after releasing; binaries and completions fields documented; signing/notarization discussed (with quarantine-flag workaround caveat).
  - Evidence: <https://goreleaser.com/customization/homebrew_casks/>
- ✅ **CONFIRMED** — SO 2025: TypeScript 43.6% of all respondents (48.8% professionals) vs Go 16.4% (17.4%); Octoverse: TypeScript became GitHub's most-used language in August 2025
  - Survey page shows exactly those four percentages. Octoverse cross-checked: github.blog post 'Octoverse: A new developer joins GitHub every second as AI leads TypeScript to #1' — TypeScript overtook Python and JavaScript in August 2025 (report published Oct 2025).
  - Evidence: <https://survey.stackoverflow.co/2025/technology>
- ✅ **CONFIRMED** — nats.js v3.4.0 shipped 2026-05-08 (JetStream 2.14 features) and bolt-js v4.7.3 shipped 2026-05-27 — neither is stale
  - nats.js v3.4.0 published 2026-05-08; its notes explicitly say it 'adds support for exciting nats-server 2.14 JetStream features'. bolt-js v4.7.3 published 2026-05-27, with a steady monthly cadence before it. The sibling brief's alleged wrong dates could not be inspected, but the corrected facts themselves are accurate.
  - Evidence: <https://api.github.com/repos/nats-io/nats.js/releases>
- ✅ **CONFIRMED** — nats.go v1.52.0 (2026-05-07), slack-go v0.27.0 (2026-06-27), mautrix-go v0.28.1 (2026-06-16), matrix-js-sdk v41.9.0 (2026-07-07) on a biweekly cadence
  - All four tags/dates match the respective releases APIs; matrix-js-sdk shows the claimed biweekly rc→stable cadence (41.7.0 → 41.8.0 → 41.9.0 → 42.0.0-rc.0 at ~1-2 week intervals). The 'with E2EE' attribute of mautrix-go was not separately verified in this pass (its crypto subpackage is well established, but the release note wasn't checked for it).
  - Evidence: <https://api.github.com/repos/matrix-org/matrix-js-sdk/releases>

Load-bearing claims in the prose **without** a sourced key fact:

- ⚠️ 'The go-sdk is the only official SDK whose stable line spans current and next spec' (recommendation) and 'stable line... already carrying next-spec support' (tldr) — overstated relative to its own key_fact: 2026-07-28 support exists only in v1.7.0-pre.1/pre.2 PRERELEASES; the latest stable Go release (v1.6.1) does not have it. The defensible claim is 'same major line, no rewrite required', not 'stable line already has it'.
- ⚠️ 'TypeScript and Python SDKs... stable lines will not get the 2026-07-28 spec' (tldr) — an inference; the TS README says v1.x stays supported for production but nowhere states v1 will never receive 2026-07-28 support. Strongly implied for Python (core-replacement quote), unproven for TS.
- ⚠️ 'the MCP TS SDK has the largest install base' — no key_fact (no npm/pkg download comparison was fetched).
- ⚠️ 'most MCP server examples users copy are npx-based' — no key_fact, unverified.
- ⚠️ 'the AI-agent community skews TS/Python' — no key_fact behind it (SO/Octoverse data is general developer population, not agent-community-specific).
- ⚠️ 'a Durable Object with WebSocket hibernation, effectively free at team scale' — Cloudflare pricing/plan-gating claim inherited from the sibling transport brief with no key_fact here (DO WebSocket hibernation pricing and free-plan availability were not verified in this brief).
- ⚠️ 'Python... distribution story for a run-everywhere daemon is the weakest of the three' — asserted without any key_fact (no PyInstaller/uv/shiv-style evidence examined).
- ⚠️ 'this is a ~hundreds-of-lines component and bilingual repos of this shape are common' — size estimate and prevalence claim with no supporting fact.
- ⚠️ 'GoReleaser auto-publishes... curl-able archives' — the key_fact and source only cover Homebrew Casks; archive/curl publishing is plausible GoReleaser behavior but not in the cited page's verified content.
- ⚠️ Node SEA 'macOS x64 untested' is sourced, but the brief's framing that npx 'requires a working Node install' plus the overall distribution-friction comparison (install one-liner vs npx) rests on no measured evidence of colleague friction — a judgment call presented as fact.
