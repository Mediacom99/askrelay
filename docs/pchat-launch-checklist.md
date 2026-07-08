# pchat — Launch Checklist

## Before you post anywhere

- [ ] One-liner install works cold, on a clean machine (`go install github.com/you/pchat@latest` or a curl-to-binary script). Test this yourself on a fresh VM/container — don't assume it works because it works on your machine.
- [ ] Cross-compiled binaries built for macOS (amd64+arm64), Linux (amd64+arm64), Windows (amd64) and attached to a GitHub release.
- [ ] README has, near the top, in this order:
  1. One-sentence hook (the feel, not the tech — e.g. "A chat room that doesn't exist when nobody's in it.")
  2. A terminal recording (asciinema link or GIF) showing two people talking, glyphs visible.
  3. Install command.
  4. `pchat --room <passphrase>` usage, with the commons-room passphrase called out explicitly.
- [ ] README states plainly (not buried): passphrase = access, weak passphrases are effectively public with visible IPs via DHT lookups (per §13 of the architecture doc). Don't let someone else discover this and post it as a "gotcha."
- [ ] Public "commons" room decided and named (e.g. `--room launch-day` or similar) — this is your live demo mechanism, not an afterthought.
- [ ] Basic abuse handling in place: at minimum, local mute works, and graceful Ctrl+C sends a `left` presence so departures don't look like crashes.
- [ ] Test the 2-peer and 3-peer case specifically (per §13) — this is exactly the scale your first users will hit.

## Assets to have ready

- [ ] Terminal recording / GIF (primary asset — this carries more weight than any written description).
- [ ] Short Show HN–style title + one paragraph description drafted in advance, not written live under launch pressure.
- [ ] A couple of screenshots as backup for platforms that don't render GIFs/embeds well (Reddit, some Discords).
- [ ] Your own glyph/signature ready to reference ("look for the [glyph] — that's me") so early visitors have a friendly face to find.

## Launch sequence

- [ ] **Seed first.** Be sitting in the commons room yourself before you post anywhere. Nobody should land in an empty room.
- [ ] Post order — smallest/friendliest audience first, so you can fix rough edges before the bigger swings:
  1. A CLI/terminal-tool Discord or small subreddit (r/commandline) — low-stakes test of the install flow and first impressions.
  2. r/golang — technical crowd, will read the architecture doc if you link it.
  3. lobste.rs — good fit for the ephemeral/privacy framing.
  4. Show HN — biggest potential spike, do this once the above have shaken out obvious bugs.
  5. Twitter/X — thread version of the hook + the recording, tag a few CLI-tool/indiehacker accounts who might reshare.
- [ ] Space these out (don't post everywhere in the same hour) — you want to be present and responsive in each thread, and a second wave a few days later reads better than one big simultaneous dump.
- [ ] In every post: lead with the one-sentence hook, not the tech stack. Save libp2p/Noise/Ed25519 details for people who ask or for the linked architecture doc.

## During launch (first 24–48 hours)

- [ ] Stay in the commons room as much as possible — you *are* the onboarding experience right now.
- [ ] Respond to every comment/thread, especially bug reports — this is a zero-users-to-some-users product, every single early tester matters disproportionately.
- [ ] Watch for the passphrase/IP-exposure caveat coming up as a question or criticism — you already have the honest answer ready in the README, so point to it rather than getting defensive.
- [ ] Note every piece of confusion (install friction, unclear commands, silent failures) — these are your v1.1 punch list, not just launch-day noise.

## After the first wave

- [ ] Collect the 3–5 most common questions/confusions and fold fixes into the README or CLI help text directly.
- [ ] Decide, now that real strangers have used it: does the commons room stay as a permanent front door, or was it launch-only? (Ties back to the cold-start question in §12.)
- [ ] Revisit the "Open Questions / Known Gaps" and "Product-Level Considerations" sections of the architecture doc against what actually happened in practice — real usage will answer some of those faster than more planning would.
