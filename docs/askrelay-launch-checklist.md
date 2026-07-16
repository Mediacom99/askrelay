# askrelay — Launch Checklist

Gates are ordered; nothing in a later gate starts before the earlier gate is
fully checked. Evidence expectations: check items off with a link or commit
ref, not from memory. Decision citations (D-xx) are inline. The implementation
plan's §8 kill criteria govern continue/stop decisions; this checklist
implements their launch-adjacent items.

## Gate 0 — dogfooding (before any public word)

- [ ] Relay self-hosted for the Kosmoy team; every colleague enrolled with
      their own device key (D-08).
- [ ] ≥ 25 real questions answered across ≥ 3 people, including at least one
      Claude Code ↔ ChatGPT exchange (D-02) — the demo flow, proven on real work.
- [ ] Team's actual ChatGPT plan mix verified against write-MCP gating
      (chatgpt-extension.md; Phase 2 action item) — S-04 spike done.
- [ ] Anthropic asked the "human-approved relay = ordinary individual usage?"
      question via the channel their legal page names (vendor-tos.md); answer
      (or non-answer) recorded in the decision log.
- [ ] OpenAI ToS/help pages that 403'd to fetchers pulled by a human and
      re-checked (vendor-tos.md).
- [ ] S-03 outcome folded in (MCP 2026-07-28 final vs our 2025-11-25 target).

## Assets ready before launch

- [ ] 60–90 s demo video: A's Claude Code asks → B approves in ChatGPT → B's
      AI answers (D-15, D-16).
- [ ] README front door + threat-model doc + per-client setup guides shipped
      (WP-15); honest client matrix from arch §9 (condensed is fine — the
      architecture table is authoritative).
- [ ] Kosmoy legal settled: owner-of-record for copyright/trademark named;
      employer-IP terms for contributors documented in CONTRIBUTING.md
      (oss-licensing.md; Phase 2 action item — must land before the first
      external PR, since DCO presupposes it).
- [ ] Hosted demo relay hardened and live (WP-16): rate limits, isolation
      audit, AUP, abuse contact (D-08).
- [ ] Release pipeline proven: tagged pre-release built by GoReleaser; brew
      tap, npm wrapper, docker image all install on a clean machine (D-09,
      WP-14).
- [ ] MCP Registry entry prepared; awesome-mcp-servers PR drafted (D-16).
- [ ] Repo hygiene: description = D-15 tagline, topics set, issue templates,
      DCO check enforced, CONTRIBUTING.md (explicit no-CLA), GOVERNANCE.md
      with no-relicense pledge (D-13).
- [ ] Security review of the OAuth surface (R-06) — at minimum a focused
      internal pass; external audit if budget allows.
- [ ] At least one co-maintainer or serious early contributor on board (a
      Kosmoy colleague counts) — the "one contributor = dead by default"
      heuristic (D-19/C7).
- [ ] Public release-cadence statement (≥ monthly for the first six months)
      in README/GOVERNANCE (D-19/C5, C7).

## Launch sequence (one day)

1. [ ] Tag `v0.1.0`, publish release + docker image.
2. [ ] MCP Registry publication (subregistries auto-propagate — D-16).
3. [ ] X post with the demo video; seed 3–5 practitioner accounts directly
       (oss-adoption-dynamics.md).
4. [ ] awesome-mcp-servers PR submitted.
5. [ ] Show HN within the week — expectations explicitly low (D-16).

## During the first 48 hours

- [ ] Watch relay logs: per-client tool-call success rates (R-01), OAuth
      failures, abuse signals on the hosted instance.
- [ ] Answer security questions with threat-model links, not improvisation.
- [ ] Triage-only mode on issues: label, thank, fix crashes; no feature
      commitments in the heat.

## After the first wave

- [ ] Discord/community channel only after ~50 real users (D-16).
- [ ] D-14 trigger check: colleagues + externals adopted? → neutral GitHub
      org migration (org name `askrelay` was free at naming time — re-verify).
- [ ] D-17 trigger check: traction shown? → EUIPO filing first, then USPTO.
- [ ] Revisit squatted `askrelay.com` (negotiate or ignore — D-18/R-08).
- [ ] v1.1 planning gate: unattended auto-reply only if the Anthropic answer
      permits it (D-03, R-03).
- [ ] Kosmoy paid-maintenance-hours conversation, once dogfooding has proven
      value (D-19/C8 — "ask later").
- [ ] Recruit 2–3 cross-org tester pairs (OSS co-maintainers, contractor/
      client) when stable enough not to burn goodwill — the moat-quadrant
      probe (D-19/C9).
