# Project Retrospective

*A living document updated after each milestone. Lessons feed forward into future planning.*

## Milestone: v1.10 — Inhaltsmodell und Zugang (planned as v1.6)

**Shipped:** 2026-09-10 (milestone close), released as 1.10 on 2026-09-11
**Phases:** 6 (6–11) | **Plans:** 42 | **Tasks:** 95 | **Sessions:** several, not counted

### What Was Built

- The field palette in every carrier: button-row choice, multiple choice, term field, `zeit`, `bereich`, `code`, with one multi-value encoding (Phases 7, 8)
- CSV import with mapping, dry run and a per-row report (Phase 9)
- Single sign-on through Authentik forward-auth, with the trust boundary closed before the first header is read (Phase 10)
- A gallery with albums, a script-free lightbox and a scroll-snap slideshow (Phase 11)
- The housekeeping gates the rest was measured against: catalogue format test, reproducible wasm rebuild in CI (Phase 6)

### What Worked

- **Red before green, in its own commit.** Every defect found late — the cross-website writes, the fifth unexpanded album path, the own-block-kind check at the audit — was proven by a failing test committed before the fix. The history shows the defect, not only the remedy.
- **Mutation probes after every fix.** Reverting each fix line one at a time and demanding red caught tests that passed for the wrong reason, and once caught a probe that counted a build failure as red.
- **Browser passes against the running binary.** They found what every green gate missed: German sentences printed to an English admin with the i18n gate reading `0 offen`, and a Critical in Phase 11 the code review had not seen.
- **An integration check at milestone level.** The one broken seam of v1.6 — own block kinds never checking the field kinds Phase 7 added — belonged to no single phase and was invisible to all six verifications.
- **Amend, never rewrite.** Verification and security reports kept their original verdicts beside the new ones (`*_at_audit`), so a later reader can see what changed and why.

### What Was Inefficient

- **Ledgers drifted behind the code.** At the close, four reports, one debug session and two quick tasks said "open" about work that was done and tested — each fixed in a commit that did not touch its report. The audit spent a noticeable share of its time re-proving closed items.
- **Counting gates measured something other than their names.** In Phase 10, seven of eighteen counting rows printed a number other than the one they claimed, while the property they were meant to protect held — a green gate that could not have gone red.
- **Shell and harness traps.** The Bash tool runs zsh: an unquoted list was never split and a mutation script restored nothing while "checksums matched" over empty strings. The system bash is 3.2 and has no associative arrays. Parallel workflow agents shared one tree and saw each other's mutations.
- **Scope decided late.** Phases 11 and 12 were added mid-milestone; Phase 12's move to v2.0 was decided three days later, and the roadmap kept it inside the v1.6 window until the close tooling refused to proceed.
- **Agent capacity.** Several multi-agent runs died on the usage limit and had to be retried.

### Patterns Established

- A second id taken from a path or a form is checked against the website in the same statement that uses it (`AND website_id`), not in the handler beside it — six occurrences taught this in four days.
- Every renderer of a stored thing is enumerated when the thing is added: page, share link, preview, plugin API, feed, conditional requests.
- Mutation and restore scripts run under `bash`, back up first, and fail closed on empty checksums.
- A full suite runs in a worktree on the exact commit, not in the working tree.
- Reports are amended with a *Nachtrag* and `*_at_*` keys; the original verdict stays.

### Key Lessons

1. **A gate is a claim about what it reads.** Before trusting green, check that the gate could have gone red on the defect it names — the i18n gate cannot see a `fmt.Sprintf` sentence, and a counting grep counts its own comment.
2. **Close the ledger in the commit that closes the defect.** A fix whose report still says "open" costs the next audit the full proof again.
3. **Milestone-level integration checks are not optional.** Cross-phase seams (a new field kind reaching an older carrier) are exactly what per-phase verification is structured not to see.
4. **Decide scope moves in the roadmap structure, not only in prose.** A note that says "moved to v2.0" under a v1.6 heading is still v1.6 to every tool.
5. **Keep milestone and release on one number.** The planning v1.6 closed while the release tags already stood at v1.9; it was released as 1.10 and renumbered to match (decided 2026-09-11). From here a milestone close creates its release tag.

### Cost Observations

- Model mix: not recorded
- Sessions: several, not counted
- Notable: most late cost went into re-proving closed work (ledger drift) and re-running agents after usage-limit failures, not into building

---

## Milestone: v2.0 — The Codebase Speaks English

**Shipped:** 2026-09-13
**Phases:** 1 (12) | **Waves:** 10 | **Commits:** 32

### What Was Built

- The template data contract in English, with the eight shipped themes, and a
  refusal that names the English fields when a 1.x theme is uploaded
- The MCP surface in English — not planned, found by the gate
- Migration 00054: 24 columns, 6 indexes, a `Down` half driven by a test
- The catalogue turned round: the English sentence is the key, `de.json` is a
  translation, `de-CH` derives from `de.json`
- ~250 new catalogue entries in four languages, most of them sentences that had
  never been in any catalogue at all
- A translation channel for plugins: a host operation, two SDK functions, a
  third collector root
- `tools/english`, a gate that parses, blocking in CI

### What Worked

**Writing the rename map by hand.** The phase tried generating it from word
stems first and produced `Archivee`, `Groupn`, `AlbumTokenr`. The roadmap had
predicted exactly that. 870 identifiers written out by hand cost an hour and
were right.

**`go/scanner` instead of `sed`.** The rename tool rewrites only `token.IDENT`,
which is the whole reason catalogue keys, form field names and stored values
came through a 870-identifier rename untouched. A textual replace over the same
map would have rewritten all three and the damage would have been silent.

**Letting the gate find the work.** `tools/english` was built to *hold* the
result, and it turned out to be the thing that found the work: the MCP surface,
the four column headings that had broken in the flip, the wrapper functions the
collector did not know. A gate written early reports on the phase that writes it.

**The browser pass caught four things no test did.** Four column headings
reading German on an English screen, two page titles assembled with `+`, a
dropdown with no catalogue path, and a bundle import announcing itself as a
WordPress import. None of them is the kind of thing a test asserts, and every
one of them is the kind of thing a person sees immediately.

### What Did Not

**The criterion's own gate could not be written as specified.** Criterion 1 asked
for `grep -rn '[äöüÄÖÜß]'` to print nothing. With no locale set grep matches
bytes, and `Ü` shares its second byte with `“` — the gate miscounted in this
repository's own container. And a comment explaining how umlauts are
transliterated cannot be written under a literal reading of the rule. The
criterion was right about the *goal* and wrong about the *instrument*, and that
was only discoverable by trying it.

**A format string assembled with `+` is invisible even inside `i18n.N`.**
CLAUDE.md says a sentence built with `fmt.Sprintf` cannot be collected. It does
not say that a *marked* sentence joined across three lines cannot be either —
the collector reads a string literal, not an expression. Two sentences were
added, reported as orphaned, and had to be rewritten as single literals. The
note is now in the code beside them.

### Lessons for Next Time

1. **A milestone whose product is readability needs a mechanical reader.** Every
   estimate in this phase was wrong in the same direction until `tools/english`
   existed; after it existed, the remaining work was a countdown.
2. **Measure the measurement.** The 828 that sized criterion 9 counted 274
   strings in `plugins/` that were mostly a visitor's text — outside the
   milestone entirely. Walking them turned a scheduled carry-over into an
   afternoon.
3. **A decision taken mid-phase belongs with the other decisions.** §3c and §3d
   were written into `12-CONTEXT.md` beside §3a and §3b, not into commit
   messages. The next reader looks in one place.
4. **Name the exception in the gate, not in prose.** The public side has no
   translation channel; that fact now lives in `germanVoice` in
   `tools/english/main.go`, where somebody meets it while working rather than
   while reading the archive.

## Cross-Milestone Trends

### Process Evolution

| Milestone | Sessions | Phases | Key Change |
|-----------|----------|--------|------------|
| v1.0 | not recorded | 5 | Initial build; no retrospective written |
| v1.10 (planned as v1.6) | several | 6 | Proof-first fixes, mutation probes, browser passes and a milestone integration check as standing practice |

### Cumulative Quality

| Milestone | Test packages | Coverage | Zero-Dep Additions |
|-----------|---------------|----------|--------------------|
| v1.0 | not recorded | not recorded | — |
| v1.10 | 44 green at close | not measured | SSO, CSV import and gallery added without a new module dependency |

### Top Lessons (Verified Across Milestones)

1. Planning artefacts that are not updated at boundaries fall behind the code — seen after v1.0 (five months of "v1.0 complete") and again inside v1.10 (ledgers behind fixes).
