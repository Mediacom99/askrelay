# Transport, delivery, and identity building blocks for cross-person AI-session messaging

> Decision brief — research sweep of 2026-07-14. Every key fact cites a live source
> fetched that day; an independent verifier re-checked each one (verdicts below).

## TL;DR

For a small team behind NATs with sleeping laptops, offline/queued delivery is the binding constraint, which rules out pure P2P (Tailscale-only, WebRTC, iroh) as the core. The strongest shape is a tiny purpose-built relay with a message queue — either a Go binary on a cheap VPS or a TypeScript Durable Object on Cloudflare (whose free tier and WebSocket hibernation make it effectively free at team scale) — with magic invite links binding device keypairs as v1 identity. Slack Socket Mode is the best zero-infra adapter since the company already runs Slack, and Matrix is the fallback only if E2EE becomes a hard requirement. Off-the-shelf brokers (NATS, MQTT, Redis) and email solve less than they cost here.

## What it is

This brief surveys how a message gets from person A's AI session to person B's across NAT, sleeping laptops, and mixed tooling. Four families: (a) a self-written relay — a small WebSocket/HTTP server holding per-recipient queues, hostable on a VPS (Go, coder/websocket) or on Cloudflare Durable Objects (TS/JS; other languages only via WASM), where hibernating WebSockets incur no idle-duration billing; (b) off-the-shelf messaging infra — NATS JetStream (persistent streams, self-hosted single binary), Redis Streams, MQTT, ntfy.sh (push notifications, no E2EE); (c) existing chat networks as transport — Slack Socket Mode bots (outbound-only WebSocket, no public endpoint needed, company already has Slack), Matrix bots with real E2EE via mautrix-go or matrix-bot-sdk, XMPP, Discord, email/AgentMail; (d) P2P overlays — Tailscale tsnet (embed a node in a Go binary; Funnel for public TLS listeners), WebRTC data channels (Pion v4 in Go), and iroh (Rust, 1.0 in June 2026, no Go bindings). Identity options for "colleagues at a company" range from magic invite links and device keys (SSH/age-style) up to GitHub org membership, OIDC/SSO, and mTLS.

## Maturity

All checked against live sources July 2026. Self-relay building blocks: coder/websocket is the maintained Go WebSocket library (gorilla/websocket archived, per websocket.org's 2026 guide); Cloudflare DO free tier is 100k requests/day, 13k GB-s/day, SQLite backend, with WebSocket Hibernation GA. Brokers: nats-server v2.14.3 (2026-06-29, 6-month cadence); Go client current, but the TypeScript client nats.js last tagged v3.4.0 on 2025-05-08 — a year quiet. Chat networks: Slack Socket Mode is a stable documented API (10 connections/app cap; Bolt JS last release v4.7.3, 2025-05-27 — slow but stable); Matrix Synapse v1.156.0 (2026-07-07, Element-maintained), mautrix-go v0.28.1 (2026-06-16) with E2EE incl. cross-signing and key backup; ntfy v2.26.0 (2026-07-09) but E2EE has been an open issue since Dec 2021. P2P: iroh hit 1.0.0 on 2026-06-15 (already at 1.0.2) but is Rust-only — FFI bindings cover Swift/Kotlin/Python/JS, Go absent and FFI releases paused; Tailscale tsnet/Funnel is production-grade Go; Pion WebRTC v4 active (published 2026-06-28). AgentMail is a hosted, seed-stage startup ($6M, TechCrunch 2026-03-10) — too early to depend on.

## What adopting it buys us

Shape 1 (thin self-written relay with per-user queues): exact fit — queued delivery for sleeping laptops by design, trivial NAT story (clients dial out), self-host on a $5 VPS or free at team scale on Durable Objects with hibernation, full control of message schema, delivery receipts, and approval flows, and a clean path to E2EE (clients encrypt to recipient device keys; relay stays dumb). Open-sources cleanly: users run one binary or one `wrangler deploy`. Shape 2 (Slack Socket Mode adapter): zero new infrastructure, identity and IT approval inherited from the existing workspace, offline delivery free (Slack retains messages for fetch-on-reconnect), and human-approval UX (B taps a Slack button) for free; Socket Mode explicitly targets behind-corporate-firewall apps with no public endpoint. Shape 3 (Matrix): native store-and-forward, federation across companies later, real bot E2EE proven in both Go (mautrix-go) and TS (matrix-bot-sdk via the Rust crypto SDK), and an open-standard story that suits an OSS project. Identity via magic invite links binding a per-device keypair buys onboarding in seconds with no IdP integration, upgradeable to GitHub-org or OIDC checks without changing the wire protocol.

## What it costs us

Shape 1: we own uptime, auth, and abuse handling of a network service; queued-delivery semantics (acks, dedupe, expiry) are subtle to get right; choosing Durable Objects locks hosting to Cloudflare and effectively forces TypeScript for the relay (Go only via WASM — awkward for DO classes), which challenges the Go default. Shape 2: caps at 10 concurrent Socket Mode connections per app (a real ceiling if every teammate's machine holds its own socket), no E2EE (Slack sees everything), Marketplace distribution disallowed for Socket Mode apps, Bolt's slow release cadence, and the tool's core becomes coupled to one proprietary network — acceptable for an adapter, wrong for the core. Shape 3: running Synapse + Postgres is the heaviest self-host burden surveyed, and headless E2EE bots mean key backup/cross-signing state management — meaningful complexity for a v1. Rejected options cost more than they give: NATS adds a server plus client deps yet still needs our identity/E2EE layer on top (and its TS client lags); ntfy has no E2EE; Tailscale tsnet is Go-only and has no offline queue (both peers must be up); iroh has no Go or mature TS path; email/AgentMail is high-latency, spam-filtered, and hosted-only.

## Recommendation

Build shape 1 as the core: a deliberately thin relay with per-recipient durable queues, clients connecting outbound over WebSocket with HTTP fallback. Decide hosting first, because it decides language: if we want zero-ops and free, write it in TypeScript on Cloudflare Durable Objects (SQLite storage + WebSocket hibernation); if we want self-host-anywhere as the flagship OSS story, write it in Go (coder/websocket) as a single binary with SQLite — I lean Go-single-binary as primary with a DO port later, since "docker run one container" is the cleaner open-source pitch. Ship Slack Socket Mode as the first transport adapter, not the core: it gets the pilot team messaging on day one with inherited identity and approval buttons, while the relay matures. Identity v1: admin-generated magic invite links that enroll a per-device keypair (age/SSH-style); verify GitHub-org or OIDC membership at enrollment as a v1.5, not v1 — do not build mTLS PKI. Explicitly do not adopt NATS, Matrix, or any P2P overlay for v1; revisit Matrix only if E2EE becomes a hard requirement, and Tailscale only as an optional direct-path optimization. This preserves the lesson from the predecessor project: decentralization is not the requirement here — delivery-while-offline and a 10-minute setup are.

## Key facts

1. **Cloudflare Durable Objects free tier includes 100,000 requests/day and 13,000 GB-s/day, SQLite storage backend only; hibernatable idle WebSockets are not billed for duration and outgoing WebSocket messages are free.**
   - Source: <https://developers.cloudflare.com/durable-objects/platform/pricing/>
   - Verified: Fetched the official pricing page; it lists the free-tier daily limits, SQLite-only restriction, and hibernation/WebSocket billing rules.
2. **Slack Socket Mode apps can hold at most 10 open WebSocket connections at the same time and are not allowed in the public Slack Marketplace; Slack positions Socket Mode for apps behind corporate firewalls.**
   - Source: <https://docs.slack.dev/apis/events-api/using-socket-mode/>
   - Verified: Fetched Slack's official Socket Mode docs; they state the 10-connection cap, Marketplace exclusion, and firewall use case verbatim.
3. **Matrix homeserver Synapse is actively maintained by Element: v1.156.0 released 2026-07-07, with releases roughly every 2-4 weeks (v1.155.0 on 2026-06-16, v1.154.0 on 2026-06-04).**
   - Source: <https://github.com/element-hq/synapse/releases>
   - Verified: Fetched the element-hq/synapse GitHub releases page and read the last three release tags and dates.
4. **mautrix-go (maunium.net/go/mautrix) v0.28.1 was published 2026-06-16 and advertises end-to-end encryption support including key backup, cross-signing, and interactive verification.**
   - Source: <https://pkg.go.dev/maunium.net/go/mautrix>
   - Verified: Fetched the pkg.go.dev module page; it shows the version, publish date, and the README's E2EE feature list.
5. **nats-server v2.14.3 was released 2026-06-29 and the project follows a ~6-month release cycle, but the official TypeScript client nats.js has had no tagged release since v3.4.0 on 2025-05-08.**
   - Source: <https://github.com/nats-io/nats.js/releases>
   - Verified: Fetched nats-io/nats-server and nats-io/nats.js GitHub releases pages; server shows v2.14.3 (June 2026), JS client's newest tag is v3.4.0 (May 2025).
6. **iroh reached 1.0.0 on 2026-06-15 (1.0.2 on 2026-07-06) but is Rust-native; its FFI bindings cover Swift, Kotlin, Python, and JS — no Go bindings exist and the team has paused FFI releases.**
   - Source: <https://github.com/n0-computer/iroh/releases>
   - Verified: Fetched the iroh GitHub releases page for versions/dates; iroh's language-bindings docs and FFI blog post (via search of docs.iroh.computer and iroh.computer/blog/ffi-updates) confirm the missing Go support and paused FFI releases.
7. **Tailscale's tsnet package embeds a Tailscale node directly in a Go program with no separate daemon, and Server.ListenFunnel exposes a publicly reachable TLS listener (TCP 443/8443/10000).**
   - Source: <https://tailscale.com/docs/features/tsnet>
   - Verified: WebSearch surfaced and quoted Tailscale's official tsnet docs and tsnet-server API reference describing embedding and ListenFunnel port constraints.
8. **Tailscale authenticates via your existing identity provider: native SSO integrations for Google, Microsoft Entra, Okta, GitHub, and OneLogin, plus any custom OIDC-compliant provider.**
   - Source: <https://tailscale.com/docs/integrations/identity>
   - Verified: WebSearch results quoting Tailscale's official identity-integration docs list the native IdPs and the custom-OIDC option.
9. **ntfy is actively maintained (v2.26.0 released 2026-07-09) but has no end-to-end encryption; the E2EE feature request (issue #69) has been open since December 2021.**
   - Source: <https://github.com/binwiederhier/ntfy/issues/69>
   - Verified: Fetched ntfy's GitHub releases page for the version/date; search of the ntfy repo and FAQ confirmed E2EE remains an open, unimplemented issue.
10. **For Go WebSocket servers in 2026, coder/websocket is the maintained choice while gorilla/websocket is archived; Coder took over maintenance from nhooyr (who maintained it 2019-2024).**
   - Source: <https://websocket.org/guides/languages/go/>
   - Verified: WebSearch surfaced websocket.org's Go guide and the coder/websocket GitHub README stating Coder's maintenance takeover and recommending it over the archived gorilla library.
11. **Slack's Bolt for JavaScript last shipped v4.7.3 on 2025-05-27 (a security fix); the framework is stable but on a slow release cadence.**
   - Source: <https://github.com/slackapi/bolt-js/releases>
   - Verified: Fetched the bolt-js GitHub releases page; newest tags are v4.7.3 (May 2025) and v4.7.2 (April 2025), both security-focused patches.
12. **Cloudflare Workers natively supports JavaScript/TypeScript; Go runs only when compiled to WebAssembly, making a Workers/Durable Objects relay effectively a TypeScript project.**
   - Source: <https://developers.cloudflare.com/workers/languages/>
   - Verified: WebSearch quoting Cloudflare's official languages docs: JS/TS are first-class, Go is listed only under WebAssembly-compiled languages.
13. **AgentMail (email API for AI agents) is a hosted, seed-stage service: it raised a $6M seed led by General Catalyst, announced 2026-03-10.**
   - Source: <https://techcrunch.com/2026/03/10/agentmail-raises-6m-to-build-an-email-service-for-ai-agents/>
   - Verified: WebSearch surfaced the TechCrunch funding story and AgentMail's own site describing it as a hosted inbox API — no self-host option.

## Open questions

- Slack Socket Mode's 10-connection cap is per app: does our design need one socket per teammate machine (hard ceiling ~10 users) or can a single shared connection/central daemon route for the whole team? Needs a prototype and a check of per-app rate limits.
- Will company IT approve a custom internal Slack app with the message/interactivity scopes we need, and are there workspace admin policies blocking Socket Mode apps?
- Is E2EE actually a requirement for this team tool, or is TLS-to-a-self-hosted-relay acceptable to company IT? The answer decides whether Matrix (or client-side encryption on the relay) is needed at all.
- Can a Durable Object class practically be implemented in Go-compiled-to-WASM, or is the Workers path strictly TypeScript for us? (Cloudflare docs list Go only generically under WASM; no DO-specific guidance found.)
- nats.js showed no release since May 2025 while nats-server advanced to 2.14 (June 2026) — is the TS client in maintenance mode or moving to new packages (JSR @nats-io/*)? Verify before ever depending on it.
- Tailscale free-plan seat/device limits for a small team, and whether Funnel bandwidth limits matter, were not verified in this pass.
- Headless E2EE Matrix bots: how much operational pain is key backup/cross-signing state across restarts for a CLI-installed agent (especially in TypeScript via matrix-sdk-crypto-nodejs)? Needs a spike if Matrix is shortlisted.
- Delivery semantics target: do we need exactly-once with acks and replay (favors purpose-built relay or JetStream) or is at-least-once with client-side dedupe fine? Should be decided before the relay schema is designed.

## Independent verification

**Overall:** The brief is substantially reliable: 11 of 13 key_facts checked out exactly against their cited sources, including every version number and date I could independently confirm (Cloudflare DO pricing, Slack Socket Mode caps, Synapse, mautrix-go, nats-server/nats.js, ntfy, bolt-js, tsnet/Funnel, Tailscale IdPs, Workers languages, AgentMail funding). The two failures share a pattern of staleness that flatters the brief's chosen narrative: gorilla/websocket is described as 'archived' (it was revived in 2023 and is merely low-activity — the cited websocket.org guide is itself wrong), and iroh's FFI story is reported pre-1.0 ('paused', 'no Go bindings') when 1.0 actually shipped official Swift/Kotlin/Python/JS bindings plus community-status Go bindings — which also quietly undercuts the prose claim that iroh has 'no mature TS path'. Neither failure overturns the core recommendation (thin self-written relay + Slack adapter + magic-link identity), since the decisive arguments (offline queueing, Go-first, 10-connection Slack cap) rest on confirmed facts. The bigger caution is in the unbacked prose: the assertion that Slack 'retains messages for fetch-on-reconnect' is doing real work in selling the Slack adapter and is not straightforwardly true for Socket Mode event delivery — it should be verified with a prototype before the adapter is scoped, as should the Pion release date and the matrix-bot-sdk E2EE maturity claim if those options are ever revisited.

Verdicts: CONFIRMED 11, FAILED 2

- ✅ **CONFIRMED** — Cloudflare DO free tier: 100k requests/day, 13k GB-s/day, SQLite-only; hibernating WebSockets not billed for duration; outgoing WS messages free
  - Pricing page confirms all four sub-claims verbatim: 100,000 req/day and 13,000 GB-s/day free tier, 'Workers Free plan: Only Durable Objects with SQLite storage backend are available', idle hibernating DOs incur no duration charges, and 'There is no charge for outgoing WebSocket messages'. Worth knowing: incoming WS messages bill at a 20:1 ratio against the request quota.
  - Evidence: <https://developers.cloudflare.com/durable-objects/platform/pricing/>
- ✅ **CONFIRMED** — Slack Socket Mode: max 10 open WebSocket connections per app, not allowed in the public Slack Marketplace, positioned for behind-corporate-firewall apps
  - Official docs state all three: 'up to 10 open WebSocket connections at the same time', 'Apps using Socket Mode are not currently allowed in the public Slack Marketplace', and the corporate-firewall use case.
  - Evidence: <https://docs.slack.dev/apis/events-api/using-socket-mode/>
- ✅ **CONFIRMED** — Synapse actively maintained by Element: v1.156.0 on 2026-07-07, v1.155.0 on 2026-06-16, ~2-4 week cadence
  - Releases page (element-hq org) shows v1.156.0 released Jul 7 2026 and v1.155.0 Jun 16 2026, with an rc in between — cadence consistent with 2-4 weeks. The claimed v1.154.0 date (2026-06-04) was below the fold and not individually verified, but the two dates checked match exactly.
  - Evidence: <https://github.com/element-hq/synapse/releases>
- ✅ **CONFIRMED** — mautrix-go v0.28.1 published 2026-06-16 with E2EE incl. key backup, cross-signing, interactive verification
  - pkg.go.dev shows v0.28.1 published Jun 16 2026 and the README's 'End-to-end encryption support (incl. key backup, cross-signing, interactive verification, etc)'. Note pkg.go.dev flags that an even newer version now exists, which only strengthens the 'actively maintained' point.
  - Evidence: <https://pkg.go.dev/maunium.net/go/mautrix>
- ✅ **CONFIRMED** — nats-server v2.14.3 released 2026-06-29, ~6-month release cycle; nats.js has had no tagged release since v3.4.0 on 2025-05-08
  - nats-server releases page shows v2.14.3 on Jun 29 2026; the official NATS 2.12/2.14 blog posts confirm the deliberate 6-month major cycle (2.14 slipped slightly past six months). nats.js releases page confirms newest tag v3.4.0, May 8 2025. Caveat: v3.4.0 already supports server 2.14 JetStream features, so 'a year quiet' does not necessarily mean incompatible or dead — the brief correctly flags the JSR @nats-io packaging question in open_questions.
  - Evidence: <https://github.com/nats-io/nats-server/releases>
- ❌ **FAILED** — iroh 1.0.0 on 2026-06-15 (1.0.2 on 2026-07-06), Rust-native; FFI bindings cover Swift/Kotlin/Python/JS — no Go bindings exist and the team has paused FFI releases
  - Cross-checked GitHub releases page, docs.iroh.computer/languages, and the iroh 'language support' / 'ffi-updates' blog posts. The load-bearing conclusion (don't build a Go project on iroh) still holds, but the supporting facts describe the pre-1.0 state.
  - **Correction:** Versions/dates are correct (1.0.0 on 2026-06-15, 1.0.1 on 2026-06-29, 1.0.2 on 2026-07-06) and there are no OFFICIAL Go bindings. But the 'FFI releases paused' part is stale: the pause was pre-1.0, and with the 1.0 release the team shipped first-party Swift, Kotlin, Python, and JavaScript bindings mirroring the 1.0 API. Also, docs.iroh.computer/languages lists Go with 'Community' status via community-maintained FFI bindings, so 'no Go bindings exist' is too strong — 'no official/supported Go bindings' is accurate. Note the official JS bindings also weaken the brief's separate 'no mature TS path' assertion.
  - Evidence: <https://docs.iroh.computer/languages>
- ✅ **CONFIRMED** — Tailscale tsnet embeds a node in a Go program with no separate daemon; Server.ListenFunnel exposes a public TLS listener (TCP 443/8443/10000)
  - pkg.go.dev docs confirm verbatim: tsnet embeds a node 'without running a separate tailscaled daemon', and ListenFunnel exposes a TLS listener with Funnel supporting 'TCP on ports 443, 8443, and 10000'. The brief's source_url path (tailscale.com/docs/features/tsnet) resolves to the tsnet KB page which confirms the embedding claim.
  - Evidence: <https://pkg.go.dev/tailscale.com/tsnet>
- ✅ **CONFIRMED** — Tailscale authenticates via existing IdPs: native SSO for Google, Microsoft Entra, Okta, GitHub, OneLogin, plus custom OIDC providers
  - Official SSO-providers doc lists Google, Microsoft (incl. Entra ID), Okta, GitHub, and OneLogin as natively supported (Apple too), and explicitly supports custom OIDC-compliant providers. No email/password signup exists.
  - Evidence: <https://tailscale.com/kb/1013/sso-providers>
- ✅ **CONFIRMED** — ntfy actively maintained (v2.26.0 on 2026-07-09) but no E2EE; feature request issue #69 open since December 2021
  - Releases page shows v2.26.0 released Jul 9 2026; issue #69 ('Add e2e encryption') opened Dec 29 2021 and still open, marked as a hot-priority unimplemented request.
  - Evidence: <https://github.com/binwiederhier/ntfy/issues/69>
- ❌ **FAILED** — coder/websocket is the maintained Go choice while gorilla/websocket is archived; Coder took over from nhooyr (2019-2024)
  - This is a case where the source_url genuinely says what the brief claims, but an independent check of the gorilla repo itself contradicts it. The recommendation (prefer coder/websocket) survives; the stated justification does not.
  - **Correction:** The Coder takeover is real (coder/websocket README: nhooyr authored/maintained 2019-2024, Coder now maintains) and coder/websocket is a sound recommendation. But gorilla/websocket is NOT currently archived: it was archived in late 2022, the Gorilla toolkit was revived under new maintainers in 2023, and the repo today is unarchived with open issues/PRs and a v1.5.3 release (June 2024). 'Archived' should read 'low-activity maintenance mode after a 2022 archiving and 2023 revival'. The cited websocket.org guide does contain the archived claim — the source is what's outdated.
  - Evidence: <https://github.com/gorilla/websocket>
- ✅ **CONFIRMED** — Bolt for JavaScript last shipped v4.7.3 on 2025-05-27, a security fix; slow but stable cadence
  - Releases page confirms v4.7.3 (May 27 2025, rejects empty signingSecret to prevent HMAC forgery) and v4.7.2 (Apr 30 2025, SSL-check security fix) as the newest tags — over a year without a release as of July 2026, matching 'slow cadence'.
  - Evidence: <https://github.com/slackapi/bolt-js/releases>
- ✅ **CONFIRMED** — Cloudflare Workers natively supports JS/TS; Go runs only when compiled to WebAssembly, making a Workers/DO relay effectively a TypeScript project
  - Docs confirm JS/TS are first-class and Go appears only under WebAssembly-compiled languages. Caveat on the inference: the same page lists Python and Rust as first-class too (and workers-rs supports Durable Objects), so 'effectively a TypeScript project' overstates the constraint — 'effectively not a Go project' is the defensible form.
  - Evidence: <https://developers.cloudflare.com/workers/languages/>
- ✅ **CONFIRMED** — AgentMail is a hosted, seed-stage service: $6M seed led by General Catalyst, announced 2026-03-10 (TechCrunch)
  - TechCrunch article dated Mar 10 2026 confirms the $6M seed led by General Catalyst (YC, Phosphor Capital, and angels incl. Paul Graham participating) and describes a hosted API platform giving agents email inboxes. The article itself does not explicitly say 'no self-host option' — that sub-claim rests on AgentMail's site per the brief's how_verified and is plausible but not settled by this source.
  - Evidence: <https://techcrunch.com/2026/03/10/agentmail-raises-6m-to-build-an-email-service-for-ai-agents/>

Load-bearing claims in the prose **without** a sourced key fact:

- ⚠️ Pion WebRTC v4 is active with a release published 2026-06-28 (maturity section) — no key_fact or source backs this version/date.
- ⚠️ Slack offline delivery is 'free' because 'Slack retains messages for fetch-on-reconnect' (what_adopting_buys_us) — unbacked and technically shaky: Socket Mode does not queue events for disconnected apps; recovering missed messages requires polling conversations.history on reconnect, which needs extra scopes and code. This is load-bearing for the 'Slack adapter solves offline delivery' argument.
- ⚠️ matrix-bot-sdk provides 'real E2EE' in TypeScript via the Rust crypto SDK (matrix-sdk-crypto-nodejs) — asserted in what_it_is/buys but has no key_fact; the maintenance status of matrix-bot-sdk and its crypto bindings was never checked.
- ⚠️ Tailscale tsnet 'has no offline queue (both peers must be up)' (what_it_costs_us) — a correct-sounding but unsourced architectural assertion used to reject the option.
- ⚠️ Cloudflare DO 'WebSocket Hibernation GA' (maturity) — the pricing page verifies billing treatment, not GA status of the Hibernation API.
- ⚠️ iroh has 'no mature TS path' (what_it_costs_us) — contradicted by the checks: official first-party JavaScript bindings shipped with iroh 1.0.
- ⚠️ AgentMail is 'hosted-only, no self-host option' — inferred from the vendor site per how_verified, not established by the cited TechCrunch source.
- ⚠️ Rejections of Redis Streams and MQTT ('solve less than they cost') carry no key_facts at all — acceptable as judgment calls, but nothing was verified about either.
