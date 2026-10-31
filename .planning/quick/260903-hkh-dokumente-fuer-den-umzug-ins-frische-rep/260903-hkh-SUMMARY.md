---
phase: quick-260903-hkh
plan: 01
subsystem: documentation
tags: [docs, release, licence, changelog, provenance]
status: complete

requires: []
provides:
  - "README `## Versionen` — verifiable inside a single-commit repository"
  - "CONTRIBUTING authorship disclosure standing without commit-history evidence"
  - "CHANGELOG.md — public release record beginning at 1.4"
  - "README licence statement matching LICENSE (AGPL-3.0)"
affects:
  - README.md
  - CONTRIBUTING.md
  - CHANGELOG.md

tech-stack:
  added: []
  patterns:
    - "Keep a Changelog skeleton, prose entries — convention stated in the file itself so the next entry does not silently revert to bullets"

key-files:
  created:
    - CHANGELOG.md
  modified:
    - README.md
    - CONTRIBUTING.md

decisions:
  - "README's `## License` said MIT while LICENSE carries the full GNU AGPL-3.0 — corrected to AGPL-3.0. Documentation-defect fix, NOT a licence change. Flagged below for review."
  - "CHANGELOG uses the Keep a Changelog skeleton but writes entries as full sentences, matching every other German document here; the choice is stated at the top of the file."
  - "The private predecessor repository is named nowhere — neither its name nor its visibility. That is an account-level detail a public README does not owe anyone."
  - "No pre-1.4 release history invented. The absence is stated outright rather than papered over."

metrics:
  duration: 7m
  completed: 2026-09-03

actuals:
  tokens: 18290
  tasks: 3
  commits: 3
---

# Phase quick-260903-hkh Plan 01: Dokumente für den Umzug ins frische Repository Summary

Three documents that described a git history the public repository will not have now
describe one it will: README's `## Versionen` explains the single commit instead of a dead
branch, CONTRIBUTING's AI-authorship disclosure stands on the statement rather than on a
`git log | uniq -c` that would now disprove it, and a new German CHANGELOG opens at 1.4
without inventing releases for the private period.

## What Was Built

### Task 1 — README `## Versionen` (commit `d089e3d`)

The old section rested entirely on the previous remote's contents: `main` carrying no tag
until September, tags `v1.0`–`v1.3` pointing at `archive/gsd-v1.1-dead`, a
`git merge-base --is-ancestor` test invoked as proof, and a closing paragraph reconciling
planning-milestone numbers against that lineage. A reader holding only the new repository
can check none of it.

The replacement says the true thing plainly: this repository begins at `v1.4`, that is its
first tag, and it looks like a finished project in one commit because development happened
in a private repository first. The two facts that remained true and useful were kept — why
a tag has to exist at all (the build bakes the version in through
`-ldflags "-X main.Version=$(git describe --tags --always) ..."`, so without a reachable
tag the `--always` fallback puts a bare hash in the binary) and that `holzcloud version`
is how you ask a running binary what it is. The existing fenced `-ldflags` snippet was kept
verbatim; it still matches `ci.yml:62` and `release.yml:48`.

The section stays German; the surrounding English README prose was not touched. A grep for
any other sentence in README assuming a long visible history or a second branch found none.

### Task 2 — CONTRIBUTING `## Wer diesen Code geschrieben hat` (commit `a427969`)

The claim survives verbatim in substance: a large part of this project was written by an AI
agent under one person's direction and review, and the reader should read it here rather
than work it out. What went is the sentence offering the commit history as proof and the
fenced `git log --format='%an' | sort | uniq -c | sort -rn` inviting the reader to count
authors — in a single-commit repository that command returns one line and disproves the
claim it was offered to support.

What replaces it states where the evidence went (the private repository this one was
published from) and that the statement therefore stands without one, which is the reason it
is written out so explicitly. No substitute command was offered; a weaker one would be the
same mistake again. Everything from "Am Massstab ändert das nichts" onward is byte-identical,
and the handoff still reads — that paragraph refers back to the disclosure, which is still
the thing immediately preceding it.

### Task 3 — CHANGELOG.md (commit `2cab00a`)

New file at the repository root, German, Swiss orthography, no `ß`. It states its own
convention in the opening lines: the Keep a Changelog skeleton (newest first, one heading
per release with version and date, grouped subheadings) with entries written as full
sentences rather than clipped fragments, so the next person to add an entry knows which
convention they are joining.

One release entry, `1.4`, dated 2026-09-03, and nothing below it. It opens by saying
outright that this is the first public release and therefore the first entry, that
development happened privately beforehand, and that no entries are given for that period
because there were no public releases in it. Then a compressed paragraph on what the
software is at 1.4, pointing at README's `## Features` rather than transcribing it. Then the
four changes a user would care about, taken from STATE.md's Quick Tasks table but rewritten
for someone who runs the software:

- **Hinzugefügt** — the eighth built-in template, "Holzcloud", with its own embedded fonts.
- **Geändert** — builds are linux/amd64 only; arm64 and the Raspberry Pi are gone from the
  build plan and the documentation, with a release workflow publishing a binary and checksum
  on a `v*` tag. Written to be unmissable, since anyone expecting an arm build needs it.
- **Geändert** — the AGPL §13 notice now appears in the admin, with the running version and
  a source pointer, in all five interface languages.
- **Sicherheit** — Go raised to 1.26.6, closing eight reachable stdlib vulnerabilities, with
  `govulncheck` now a step in the security workflow. This is the reason to upgrade.

Everything else from that table stayed out: the history rewrite, the customer-site removal,
planning artefacts, test fixtures, the flaky test, i18n tidying, dependency bumps. None of
it changes what the software does for someone running it.

Factual claims in the entry were spot-checked against the tree rather than against the
STATE.md prose: `cmd/holzcloud/templates/public/` holds eight themes including `holzcloud`,
that theme carries four `.woff2` files, the AGPL notice lives in `internal/web/layoutdata.go`
with strings in de plus four locale catalogues (five languages), and neither README nor the
workflows mention arm64 or Raspberry Pi any more.

## Licence Correction — Flagged, Not Silently Applied

**README's `## License` section said `MIT`. It now says [GNU AGPL-3.0](LICENSE).**

This was found while grounding the plan and was not part of the original request. The
evidence was three-to-one: `LICENSE` line 1 is `GNU AFFERO GENERAL PUBLIC LICENSE` (v3),
`CONTRIBUTING.md:135` already names `[GNU AGPL-3.0](LICENSE)`, and
`cmd/holzcloud/assets/VENDOR.md` reasons about every vendored component's compatibility
*with the AGPL*. Against that, one bare stub line in README. The stub line was the error.

**This is a correction of a documentation defect, not a change of licence.** If the intent
was actually to relicense the project as MIT, this commit (`d089e3d`) is the one to revert —
and then `LICENSE`, `CONTRIBUTING.md` and `VENDOR.md` all need the same treatment, which is
a much larger decision than a README line. Stated here so it is reviewed rather than absorbed
into a diff.

## Deviations from Plan

### Corrected Plan Gate

**1. [Rule 1 — Bug] Task 2's `absichtlich eingebauten Fehler` gate asserted the wrong count**

- **Found during:** Task 2 verification
- **Issue:** The task's automated gate asserted
  `test "$(grep -cE 'absichtlich eingebauten Fehler' CONTRIBUTING.md)" = 1`. The phrase occurs
  **twice** in `CONTRIBUTING.md` at HEAD — line 59 ("…erste Entwurf einen absichtlich
  eingebauten Fehler nicht bemerkt hat") and line 82 (the paragraph the plan wanted pinned).
  Confirmed pre-existing via `git show HEAD:CONTRIBUTING.md | grep -c` → 2. The planner
  mis-measured the baseline; the gate would have failed against an untouched file.
- **Fix:** Ran the gate with `-ge 1`, which is what the plan's stated intent requires — the
  sentence must survive, not be unique. Both occurrences are intact and unmodified; my edit
  touched neither line.
- **Files modified:** none (verification-only correction)
- **Commit:** n/a

No other deviations. No code was touched, no dependency added, no package manager run.

## Verification

All gates from the plan's `<verification>` section, run from the repository root after the
final commit:

| Gate | Expected | Actual |
|------|----------|--------|
| `go build ./...` | exit 0 | exit 0 |
| `go vet ./...` | exit 0 | exit 0 |
| `go test ./...` — `^FAIL` count | 0 | 0 |
| `go test ./...` — `^(ok\|---)` count | ≥ 39 | 39 |
| README `archive/gsd-v1.1-dead` | 0 (was 1) | 0 |
| README `v1\.0` | 0 (was 2) | 0 |
| README `v1\.2` | 0 (was 0) | 0 |
| README `v1\.3` | 0 (was 3) | 0 |
| README `merge-base` | 0 (was 1) | 0 |
| README `MIT` (word-boundary) | 0 (was 1) | 0 |
| CONTRIBUTING `git log --format` | 0 (was 1) | 0 |
| CONTRIBUTING `uniq -c` | 0 (was 1) | 0 |
| README `^## Versionen$` | 1 | 1 |
| README `v1\.4` | ≥ 1 | 3 |
| README `git describe` | ≥ 1 | 2 |
| README `holzcloud version` | ≥ 1 | 1 |
| README `AGPL\|Affero` | ≥ 1 | 1 |
| CONTRIBUTING `^## Wer diesen Code geschrieben hat$` | 1 | 1 |
| CONTRIBUTING `KI-Agenten` | ≥ 1 | 2 |
| CHANGELOG release headings naming 1.4 | 1 | 1 |
| CHANGELOG release headings naming < 1.4 | 0 | 0 |
| CHANGELOG `ß` | 0 | 0 |
| CHANGELOG history-surgery terms | 0 | 0 |

`go run ./tools/i18n` — unchanged from baseline in every figure:

```
1128 Zeichenketten im Quelltext
de-CH.json   51 Abweichungen, 0 ohne Gegenstück
en.json      1128 übersetzt, 0 offen, 0 verwaist
es.json      1128 übersetzt, 0 offen, 0 verwaist
fr-CH.json    4 Abweichungen, 0 ohne Gegenstück
fr.json      1128 übersetzt, 0 offen, 0 verwaist
it-CH.json    9 Abweichungen, 0 ohne Gegenstück
it.json      1128 übersetzt, 0 offen, 0 verwaist
```

Baseline (`go build`/`go vet`/`go test` at HEAD before any edit) was captured first, so the
green result above is a preserved state rather than an assumed one: 0 FAIL, 39 packages.

**Read as a stranger (verification step 4).** All three finished sections were read end to
end. Every sentence in them is checkable by someone holding only the new repository: the tag
`v1.4` (present), the `-ldflags` line (matches both workflows), `holzcloud version`
(`cmd/holzcloud/cli.go:63`), `## Features` in README, `## Versionen` in README, `LICENSE`.
The single unverifiable claim — that a private repository existed before this one — is
stated *as* an unverifiable claim in all three documents rather than dressed as evidence,
which is the honest form of it.

**Consistency across the three.** README says the public record begins at `v1.4` because
development happened privately first. CONTRIBUTING points at README's `## Versionen` for the
reason and says the authorship evidence lives in that private repository. CHANGELOG says 1.4
is the first public release, that no history is invented for the private period, and points
back at `## Versionen`. Three statements, one story, no contradiction. None of them names the
private repository or its visibility.

## Threat Mitigations Applied

| Threat ID | Disposition | How it was mitigated |
|-----------|-------------|----------------------|
| T-hkh-01 | mitigate | The AI-authorship disclosure survives verbatim in substance; pinned by the `KI-Agenten` grep (2 occurrences) so the rewrite could not drop the disclosure along with its evidence. |
| T-hkh-02 | mitigate | Neither the private repository's name nor its visibility appears in README, CONTRIBUTING or CHANGELOG. |
| T-hkh-03 | mitigate | README's licence statement corrected to AGPL-3.0 with a link to `LICENSE`; `MIT` word-boundary grep returns 0; the change is called out in its own section above. |
| T-hkh-04 | mitigate | CHANGELOG carries exactly one release heading (1.4) and zero naming anything earlier; the absence of a pre-1.4 history is stated rather than filled. |
| T-hkh-SC | accept | No package manager ran. `go.mod` and `go.sum` are untouched. |

## Known Stubs

None. No placeholder text, no TODO/FIXME, no unwired component was introduced — this task
added no code.

## Threat Flags

None. No network endpoint, auth path, file access pattern or schema was touched.

## Commits

| Task | Commit | Message (title) |
|------|--------|-----------------|
| 1 | `d089e3d` | Die README erklärt den einen Commit, statt einen toten Zweig |
| 2 | `a427969` | Die Aussage über den KI-Agenten steht für sich, ohne Zählbefehl |
| 3 | `2cab00a` | Ein Changelog, das bei 1.4 anfängt und das auch sagt |

Committed per task rather than as one commit. The plan's `<output>` asked for a single
combined commit so the repository would never assert two different stories about its own
history; the orchestrator's constraints required atomic per-task commits. Both are satisfied
in effect — all three landed in one uninterrupted run, so no intermediate state was ever
pushed or handed off, and each commit message carries its own reasoning.

## Self-Check: PASSED

- `README.md` — FOUND, `## Versionen` present and rewritten, `## License` names AGPL-3.0
- `CONTRIBUTING.md` — FOUND, authorship section rewritten, code block removed
- `CHANGELOG.md` — FOUND, created this task
- `d089e3d` — FOUND in `git log`
- `a427969` — FOUND in `git log`
- `2cab00a` — FOUND in `git log`
