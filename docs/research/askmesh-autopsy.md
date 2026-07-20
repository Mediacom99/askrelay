# askmesh autopsy (spike S-05) — verdict & differentiation memo

> Ran 2026-07-20 (ahead of the 2026-08-01 gate), adversarially: three parallel
> investigators — distribution-footprint hunt, product teardown, and a
> both-sides demand read charged to resist a self-serving "proceed." This is the
> gate before any application code (plan §8 kill criterion 1).

## Verdict: PROCEED — kill criterion 1 does NOT fire

askmesh's near-zero adoption is explained by **invisibility, not rejection**, so
it is *not* evidence that the market saw teammate-to-teammate AI messaging and
declined. The pre-registered kill condition — "a competent, discoverable,
actually-distributed product that teammates simply declined to use" — is not
met: it was competent, but neither discoverable nor distributed. We proceed.

Two honest caveats recorded alongside the go:

1. **This removes a false negative; it does not supply a positive.** The demand
   question is now *unproven*, not *disproven*. The adversarial reader reached
   "distribution failure" at only 60% confidence and was right to hedge — what
   is established with confidence is the narrow claim (askmesh doesn't count
   against us), not that anyone wants askrelay.
2. **The retention/cold-start risk is real and askrelay inherits it undiminished
   (new risk R-11).** People who *did* reach askmesh tried it in bursts and
   almost none stuck. Even fully explained by distribution, the structural
   hazard remains: the product is worthless unless the *other* person is also on
   it and responsive; a solo installer churns. Our OSS/self-host/zero-install
   edges fix discovery and trust, not the network-effect cold start. This is why
   the Kosmoy whole-team dogfood is the right first test, and it sharpens the
   January checkpoint to measure **retention past novelty**, not trial.

## Evidence

**Essentially unlaunched (discoverability verdict: ESSENTIALLY_UNLAUNCHED).**
- Closed-source: the repository named in npm metadata, `github.com/mantoine/askmesh`,
  returns 404; the author's only 3 public repos are unrelated. No GitHub
  discovery surface at all — no stars, no README SEO, no awesome-MCP listing.
- Zero launch footprint on any channel: HN (`hn.algolia.com` exact-domain search
  = 0 hits), Reddit, Product Hunt, dev.to, Medium, YouTube — all nothing for
  askmesh specifically. The author's *own* portfolio (michael-antoine.fr) never
  mentions it.
- In no MCP registry: official registry (count 0), PulseMCP (18k+ servers, none),
  Glama (57k+, none). Not promoted and not auto-scraped — with a private repo and
  no registry entry, crawlers had nothing to index.
- Download curve (api.npmjs.org, 2026-01-01…07-20): ~7.7–8.3k total, but
  spike-on-publish-then-decay over a 1–10/day baseline that fades to single
  digits by July — non-organic (CI/npx/dogfooding) traffic, no rising baseline.
  (The earlier "~815 downloads / 62 releases" figure was off; ~82 versions,
  ~8k downloads — still tiny, correction noted.)
- Solo, unfunded author (Michael Antoine, Aix-en-Provence; reachable at
  hello@michael-antoine.fr), 52→0.22.0 releases in ~11 weeks then a hard stop at
  2026-06-17 — the builder-shipped-hard-saw-no-pull-and-stopped signature.

## Differentiation memo — askmesh solved a different problem

askmesh is functional and thoughtfully built, and it is genuinely **not the same
product**. This is the positioning gift: we keep the core mechanic
(session-to-session Q&A between people) and occupy a different center of gravity.

| Axis | askmesh | askrelay |
|---|---|---|
| Default behavior | **Automatic answering** — your agent replies for you | **Human approval both directions** — nobody's AI acts without a tap |
| Hosting | Closed hosted SaaS (`api.askmesh.dev`) | Self-hosted Go relay + SQLite; open source |
| Answering vendor | **Claude-only** (Sampling / Agent SDK / Anthropic API) | Cross-vendor (Claude Code, claude.ai, ChatGPT, Codex) |
| Inbound trust model | Teammate questions fed straight to an autonomous agent with Bash/Edit/WebFetch — **no signing, spotlighting, or redaction** | Inbound is untrusted data: Ed25519-signed envelopes, spotlighting, no tool triggering, secret redaction |
| Install | npx local MCP server + account + API token | zero-install remote-MCP (paste a URL) as a first-class path |

**The one-line framing:** *askmesh automates answering; askrelay makes consented,
safe asking across trust boundaries the product.* Their design optimizes for
agents responding autonomously (and thereby opens a wide prompt-injection surface
and locks to one vendor's cloud); ours optimizes for a human staying in the loop,
across any vendor, on infrastructure you control. Same functionality at the core,
opposite thesis about who is in control.

## What would actually settle the demand question

The autopsy cannot (the decisive data — retention among people who reached
askmesh — is private to its author). The real validation is direct and ours to
run: the Kosmoy whole-team dogfood plus 2–3 outside dev teams, measuring not
trial but **sustained use once the novelty fades** (plan §8 kill criterion 3).
Optional, cheap, high-information: email Michael Antoine and simply ask why it
stalled — did teams onboard and drop off, or did he never get it in front of
teams? He is reachable and the answer is worth an email.
