---
phase: quick-260903-hkh
plan: 01
type: execute
wave: 1
depends_on: []
files_modified:
  # Authorised scope. Every line number in this plan comes from the planner's
  # reading on 2026-09-03 and is a hint only — the executor opens the file and
  # works from what is actually there. If a section has moved, follow the
  # heading, not the number.
  - README.md
  - CONTRIBUTING.md
  - CHANGELOG.md
autonomous: true
# DOC-01..DOC-04 are local to this quick task. They are not IDs in
# .planning/REQUIREMENTS.md; they exist so each deliverable below is traceable.
#   DOC-01  README `## Versionen` states only facts observable in the new repository
#   DOC-02  CONTRIBUTING authorship claim stands without the commit-count command
#   DOC-03  CHANGELOG.md exists and begins at 1.4 as the first public release
#   DOC-04  README licence statement matches LICENSE
requirements: [DOC-01, DOC-02, DOC-03, DOC-04]

estimate:
  tokens: 35000
  raw_tokens: 35000
  tasks: 3
  confidence: low

must_haves:
  truths:
    - "Every claim in README's `## Versionen` can be checked inside the new repository by someone who has only that repository: no branch, tag or commit it names is absent (DOC-01)"
    - "README still explains why a tag has to exist at all — the build bakes the version in through `-ldflags` and `git describe`, and without a tag the binary reports a bare hash (DOC-01)"
    - "README still tells a reader that `holzcloud version` is how you ask a running binary what it is (DOC-01)"
    - "README answers, in plain words, why this project appears fully formed in a single commit: development happened in a private repository before this one, and the public record begins at v1.4 (DOC-01)"
    - "CONTRIBUTING still states that a large part of the project was written by an AI agent under one person's direction and review — the claim survives, only its now-absent evidence goes (DOC-02)"
    - "CONTRIBUTING's following paragraphs survive intact: the standard does not change, comments say why, a test that misses a deliberately introduced fault proves nothing, and contributors may use a model themselves (DOC-02)"
    - "CHANGELOG.md exists at the repository root, is written in German, and its first and only release entry is 1.4 (DOC-03)"
    - "CHANGELOG.md invents no version history for the private period, and says outright that 1.4 is where the public record begins (DOC-03)"
    - "The CHANGELOG 1.4 entry covers the four changes a user of the software would care about — the Holzcloud theme, x86-only builds, the AGPL §13 notice in the admin, and the Go raise that closed eight reachable stdlib vulnerabilities — and does not read as a transcript of planning work (DOC-03)"
    - "README's licence statement names the licence that is actually in LICENSE (DOC-04)"
    - "Nothing in the codebase moved: `go build ./...`, `go vet ./...` and `go test ./...` are green with 0 FAIL across 39 test packages, and `go run ./tools/i18n` still reports en/es/fr at 0 offen, 0 verwaist"
  artifacts:
    - "README.md — `## Versionen` rewritten; the licence statement at the foot corrected"
    - "CONTRIBUTING.md — `## Wer diesen Code geschrieben hat` rewritten"
    - "CHANGELOG.md — new file, German, first entry 1.4"
  key_links:
    - "README `## Versionen` -> the build that actually exists: `-ldflags=\"-s -w -X main.Version=$(git describe --tags --always) ...\"` at .github/workflows/ci.yml:62 and .github/workflows/release.yml:48. The prose must describe that line, not an imagined one."
    - "README `## Versionen` -> the `version` subcommand at cmd/holzcloud/cli.go:63, which accepts `version`, `-version` and `--version`. Confirmed present; the claim is safe to keep."
    - "CHANGELOG 1.4 entry -> the `### Quick Tasks Completed` table in .planning/STATE.md, rows dated 2026-09-03. That table is source material, not an outline — most of its rows are planning work and stay out."
    - "README licence statement -> LICENSE (GNU Affero General Public License v3) and CONTRIBUTING.md's closing `## Lizenz` section, which already says AGPL-3.0. Three places, one answer."
---

<objective>
Three documents in this repository describe a history that the repository being
published will not have. The project is moving to a fresh remote that begins
with one commit and one tag. README's `## Versionen` section explains a dead
branch, three superseded tags and an ancestry test — none of which will be
observable there. CONTRIBUTING's authorship section rests its claim on a command
that will return a single line. And there is no CHANGELOG at all.

Rewrite the first two so that every sentence in them can be checked by a reader
who has only the new repository, and add a CHANGELOG that starts at 1.4 without
pretending there were public releases before it.

Purpose: a document that names a branch a reader cannot find does not merely go
stale — it teaches the reader that this project's documentation is not to be
trusted. That lesson is expensive and it is the first thing a fresh repository
teaches. The fix is not to delete these sections but to say the true thing
plainly: development happened privately first, this is where the public record
starts.

Output: `README.md` and `CONTRIBUTING.md` amended in place, a new
`CHANGELOG.md`. No code changes — the whole test suite is a guard here, not a
target.

**Flagged for the developer, not silently applied without notice:** while
grounding this plan the licence statement at the foot of README.md was found to
read `MIT`, while `LICENSE` is the GNU Affero General Public License v3 and
CONTRIBUTING.md's `## Lizenz` section says AGPL-3.0. That is the same class of
defect as the other two — a document asserting something the repository
contradicts — in a file already in scope, and it is about to be published. It is
included as a one-line correction in Task 1 and called out in the summary so it
can be reverted if the intent was actually a licence change.
</objective>

<execution_context>
@/Users/holz/.claude/gsd-core/workflows/execute-plan.md
@/Users/holz/.claude/gsd-core/templates/summary.md
</execution_context>

<context>
@CLAUDE.md
@README.md
@CONTRIBUTING.md
@SECURITY.md
@.planning/STATE.md

Voice and language, read before writing a word:

- `SECURITY.md` is the closest model for what to aim at. German, full sentences,
  addresses the reader directly, states limits without apology ("Ich kann keine
  Fristen zusichern und zahle keine Prämien"), and prefers a concrete fact to a
  reassuring one. Match that register.
- `CONTRIBUTING.md` and `SECURITY.md` are German throughout. `README.md` opens in
  English, and several of its feature bullets are English, but its `## Versionen`
  section is German. **The rewritten `## Versionen` stays German. The new
  `CHANGELOG.md` is German.** Do not translate anything into English and do not
  translate the surrounding English README prose into German.
- Swiss orthography: this project writes `ss`, never `ß` (`grosser`, `blosser`,
  `ausserhalb`). Follow it.
- No em-dash-free rule here — the existing prose uses `—` freely; keep doing so.

Grounding checked by the planner on 2026-09-03, so the executor does not have to
re-derive it:

- `cmd/holzcloud/cli.go:63` — `case "version", "-version", "--version":`. The
  subcommand is real.
- `.github/workflows/ci.yml:62` and `.github/workflows/release.yml:48` both build
  with `-ldflags="-s -w -X main.Version=$(git describe --tags --always) -X main.Commit=$(git rev-parse --short HEAD)"`.
  Both workflows carry a comment saying the shallow default clone has no history
  for `git describe`. So the `--always` fallback story is accurate; only its
  *cause* changes, from "no tag was reachable on main" to "a tag has to exist".
- `LICENSE` line 1 is `GNU AFFERO GENERAL PUBLIC LICENSE`, version 3.
- `go list -f '{{if or .TestGoFiles .XTestGoFiles}}...'` over `./...` yields 39
  packages carrying tests, out of 41 total.
- `go build ./...` and `go vet ./...` are clean at HEAD.
- `go run ./tools/i18n` at HEAD prints `1128 übersetzt, 0 offen, 0 verwaist` for
  each of en, es, fr, and `0 ohne Gegenstück` for the three CH variants.

Do not read `.planning/` beyond `STATE.md`. The Quick Tasks table there is the
only planning artefact this task needs.
</context>

<tasks>

<task type="tracer">
  <name>Task 1: README `## Versionen` — say the true thing about a repository with one commit</name>
  <files>README.md</files>
  <read_first>
    Open `README.md` and locate the section by its heading `## Versionen` — at
    the time of planning it began at line 719 and ran to just before `## Build`,
    but work from the heading, not the number. Read the whole section before
    changing anything; it is roughly thirty lines and every paragraph is load-
    bearing in a different way.

    Also read the closing licence line at the very foot of the file (heading
    `## License`), and read `LICENSE` line 1 and `CONTRIBUTING.md`'s closing
    `## Lizenz` section, so the correction in step 3 is made against what those
    files actually say rather than against this plan's summary of them.
  </read_first>
  <action>
    This is the tracer for the set: it fixes the canonical wording of the
    fresh-start explanation, and the other two documents must not contradict what
    it settles here. Write it first and read it back before starting Task 2 or 3.

    Three separate edits to one file.

    **1. Replace the body of `## Versionen`.** Keep the heading. Everything that
    depends on the old remote's contents goes, because a reader of the new
    repository can check none of it: the claim that `main` carried no tag until a
    date in September, the superseded tag range and the branch it pointed at, the
    ancestry test invoked to prove they share no common ancestor, the sentence
    about those tags being left standing because deleting a tag helps nobody, and
    the closing paragraph reconciling planning-milestone numbers against that
    lineage. All of it described a repository that no longer exists in public.

    What replaces it must carry these facts, which remain true and are worth a
    reader's time:

    - This repository begins at **`v1.4`**, and that is the first tag it has.
    - The reason it appears fully formed in a single commit: the project was
      developed in a private repository before this one, and the public record
      begins here. Say it directly. A reader who notices the single commit and
      gets no explanation will assume something is being hidden; a reader who is
      told plainly will move on. Do not embroider it into a story about
      craftsmanship, and do not apologise for it either.
    - Why a tag has to exist at all: the build bakes the version in through
      `-ldflags` with `git describe --tags --always`, so without a reachable tag
      the `--always` fallback takes over and the binary reports a bare commit
      hash instead of a version. Keep the existing fenced `-ldflags` snippet —
      it matches ci.yml and release.yml verbatim today and is the concrete
      evidence for the paragraph.
    - `holzcloud version` is how you ask a running binary what it is. The
      subcommand also answers to `-version` and `--version`.

    You may keep a short sentence noting that the milestones under `.planning/`
    continue the same numbering — the tag `v1.4`, then the milestone `v1.5` — but
    only if it can be written without naming any earlier milestone number. If it
    cannot, drop it; it is the least valuable sentence in the section.

    Do not name the private repository's new archive name or its visibility.
    That is an operational detail of the maintainer's account, not something a
    public README owes anyone, and it dates badly.

    Target roughly the same length as the section it replaces, give or take a
    third. This is a README section, not an essay.

    **2. Sanity-check the rest of the file.** Grep the finished README for any
    other sentence that assumes a long visible history or a second branch. The
    planner found none outside `## Versionen`, but the check is one command and
    the cost of missing one is a reader's trust.

    **3. Correct the licence statement at the foot of the file.** It currently
    names a permissive licence that is not the one in `LICENSE`. Replace it so it
    names the licence `LICENSE` actually contains, in the same form CONTRIBUTING
    already uses for it, and link to `LICENSE`. This is one line. Mention it
    explicitly in the summary — it was found while grounding this plan and was
    not part of the original request, so the developer must see it stated rather
    than discover it in a diff.

    Leave the surrounding English README prose in English. The `## Versionen`
    section stays German.
  </action>
  <verify>
    <automated>cd /Users/holz/Projects/holzcloud-cms &amp;&amp; test "$(grep -cE 'archive/gsd-v1\.1-dead' README.md)" = 0 &amp;&amp; test "$(grep -cE 'v1\.0' README.md)" = 0 &amp;&amp; test "$(grep -cE 'v1\.2' README.md)" = 0 &amp;&amp; test "$(grep -cE 'v1\.3' README.md)" = 0 &amp;&amp; test "$(grep -cE 'merge-base' README.md)" = 0 &amp;&amp; test "$(grep -cE 'v1\.4' README.md)" -ge 1 &amp;&amp; test "$(grep -cE '^## Versionen$' README.md)" = 1 &amp;&amp; test "$(grep -cE 'git describe' README.md)" -ge 1 &amp;&amp; test "$(grep -cE 'holzcloud version' README.md)" -ge 1 &amp;&amp; test "$(grep -ciE 'AGPL|Affero' README.md)" -ge 1 &amp;&amp; test "$(grep -cE '(^|[^A-Za-z])MIT([^A-Za-z]|$)' README.md)" = 0 &amp;&amp; echo TASK1_OK</automated>
  </verify>
  <done>
    `## Versionen` still exists and is German. `README.md` contains zero
    occurrences of the dead-lineage branch name, of `v1.0`, `v1.2`, `v1.3`, and
    of `merge-base` (counts before this task: 1, 2, 0, 3, 1 — so the `v1.2` gate
    is a do-not-introduce gate, the other four are removals). It still contains
    `v1.4`, still contains `git describe`, still contains `holzcloud version`,
    now names the Affero licence and no longer names MIT. A reader of the new
    repository can verify every remaining claim in the section without leaving it.
  </done>
</task>

<task type="auto">
  <name>Task 2: CONTRIBUTING — let the authorship claim stand on its own</name>
  <files>CONTRIBUTING.md</files>
  <read_first>
    Open `CONTRIBUTING.md` and find the section by its heading
    `## Wer diesen Code geschrieben hat` — around line 69 at planning time, ending
    where `## Vorlagen` begins. Read the whole section including the three
    paragraphs after the fenced command, because those paragraphs stay and the
    rewrite has to hand off to them cleanly.
  </read_first>
  <action>
    The claim in this section is true and stays: a large part of this project was
    written by an AI agent, under the direction and review of a single person, and
    the reader should learn it here rather than work it out.

    What has to go is the sentence that offers the repository's commit history as
    the proof — that it "steht im Verlauf und fällt jedem auf, der das Repository
    klont", that many commits carry the agent as author and others a
    `Co-Authored-By` line — together with the fenced shell command that invites
    the reader to count authors. In the new repository the history is a single
    commit and that command returns one line, so the invitation now disproves the
    claim it was offered to support.

    Rewrite so the claim rests on the maintainer saying it, which is the only
    place it ever really rested. Something in the shape of: this is stated here
    because you would otherwise have to guess, the evidence lives in the private
    repository this one was published from, and you are being told rather than
    left to find out. Keep the existing sentence that the reader should read it
    here and not have to discover it themselves — that sentence is the point of
    the section and it survives untouched.

    Delete the fenced code block entirely. Do not replace it with a different
    command; there is no command in the new repository that demonstrates this, and
    offering a weaker one would be the same mistake again.

    **Everything from the paragraph beginning "Am Massstab ändert das nichts"
    onward is unchanged.** That paragraph, and the one after it about
    contributors being free to use a model and standing behind what they submit,
    are still entirely valid. Check after editing that the handoff still reads —
    that paragraph opens by referring back to what precedes it, so make sure what
    now precedes it is something it can refer back to.

    German, Swiss orthography, same register as the rest of the file.
  </action>
  <verify>
    <automated>cd /Users/holz/Projects/holzcloud-cms &amp;&amp; test "$(grep -cE 'git log --format' CONTRIBUTING.md)" = 0 &amp;&amp; test "$(grep -cE 'uniq -c' CONTRIBUTING.md)" = 0 &amp;&amp; test "$(grep -cE '^## Wer diesen Code geschrieben hat$' CONTRIBUTING.md)" = 1 &amp;&amp; test "$(grep -cE 'KI-Agenten' CONTRIBUTING.md)" -ge 1 &amp;&amp; test "$(grep -cE 'Am Massstab ändert das nichts' CONTRIBUTING.md)" = 1 &amp;&amp; test "$(grep -cE 'absichtlich eingebauten Fehler' CONTRIBUTING.md)" = 1 &amp;&amp; test "$(grep -cE 'Ihre Beiträge dürfen ebenso mit einem Modell entstehen' CONTRIBUTING.md)" = 1 &amp;&amp; echo TASK2_OK</automated>
  </verify>
  <done>
    The heading and the authorship claim survive. The author-count command and
    its framing sentence are gone (both were present once before this task). The
    three sentences the following paragraphs are built on — the one about the
    standard not changing, the one about a test that misses a deliberately
    introduced fault, and the one inviting contributors to use a model — are all
    still present exactly once each, and the section reads continuously from the
    rewritten opening into them.
  </done>
</task>

<task type="auto">
  <name>Task 3: CHANGELOG.md — a first entry that admits it is the first</name>
  <files>CHANGELOG.md</files>
  <read_first>
    Read the `### Quick Tasks Completed` table in `.planning/STATE.md` — around
    line 90, rows dated 2026-09-03. Read all of it, then read the `## Features`
    list near the top of `README.md`, which is the best existing inventory of
    what the software does.
  </read_first>
  <action>
    Create `CHANGELOG.md` at the repository root. It does not exist today.

    **Format decision, and the reasoning behind it.** Use the *skeleton* of Keep a
    Changelog — a top-of-file note saying what the file is and what convention it
    follows, newest release first, one `##` heading per release carrying the
    version and the date, grouped `###` subheadings — but write the entries as
    sentences rather than terse fragments. The skeleton is worth adopting because
    it is what a reader arriving at a public repository expects to find, it scans
    without being read, and it pairs with the semver-shaped tags this project
    already uses. The prose is worth keeping because every other document here
    (`SECURITY.md`, `CONTRIBUTING.md`, the German half of `README.md`) explains
    *why* in full sentences, and a file of clipped bullets would read as though a
    different person wrote it. State this choice in one line at the top of the
    file so the next person to add an entry knows which convention they are
    joining and does not quietly revert it to bullets.

    German, Swiss orthography, subheadings in German.

    **Content.** One release entry: `1.4`, dated 2026-09-03. Nothing below it.

    Open the entry with a short paragraph saying plainly that this is the first
    public release and therefore the first entry in this file; that the project
    was developed in a private repository beforehand; and that no entries are
    given for that period because there were no public releases in it and
    inventing some would be worth less than nothing. Do not hedge this into
    vagueness — it is the same honesty the README rewrite in Task 1 commits to,
    and the two must not contradict each other. Read the finished `## Versionen`
    section before writing this paragraph and keep them consistent in fact and in
    tone.

    Then describe what the software is at 1.4 — a self-hosted CMS as a single Go
    binary, several websites with several domains from one instance, pages in
    Markdown or blocks with per-site custom fields and content types, uploadable
    templates, a multilingual public site and a multilingual admin, media, menus,
    SEO, two-factor authentication for administrators, export and import of a
    whole site, SQLite storage with no CGO. Draw this from README's `## Features`
    list, but *compress* it: a reader wants a paragraph or a short grouped list
    telling them what they are getting, not the feature list transcribed. If a
    reader would have to open the README anyway, say so and point at it.

    Then the four changes from 2026-09-03 that a *user of the software* would
    care about. Take them from the STATE.md table, but write them for someone who
    runs this software, not for someone who watched it being built:

    - The eighth built-in template, "Holzcloud" — the design of holzcloud.ch
      available as a theme, with its own embedded fonts.
    - Builds are linux/amd64 only. arm64 and the Raspberry Pi were dropped from
      the build plan and the documentation, and a release workflow now publishes
      a binary and a checksum on a `v*` tag. Anyone who expected an arm build
      needs to read this line, so make it unmissable.
    - The AGPL §13 notice now appears in the admin, with the running version and
      a pointer to the source, in all five interface languages.
    - Go raised to 1.26.6, which closed eight reachable standard-library
      vulnerabilities; `govulncheck` now runs as a step in the security workflow.
      This one belongs under a `### Sicherheit` subheading — it is the reason to
      upgrade.

    **Everything else in that table stays out.** Planning artefacts, the history
    rewrite, moving customer sites out to a private repository, test fixtures, a
    flaky test being stabilised, i18n catalogue tidying, dependency bumps that
    changed no behaviour — a changelog is for people who use the software, and
    none of that changes what it does for them. The temptation to include the
    history rewrite because it was the most work today is exactly the temptation
    this instruction exists to refuse.

    Do not add an `## [Unreleased]` section with nothing under it. Add one only if
    it will hold something; an empty one is furniture.
  </action>
  <verify>
    <automated>cd /Users/holz/Projects/holzcloud-cms &amp;&amp; test -f CHANGELOG.md &amp;&amp; test "$(grep -cE '^## .*1\.4' CHANGELOG.md)" = 1 &amp;&amp; test "$(grep -cE '^## .*1\.[0-3]([^0-9]|$)' CHANGELOG.md)" = 0 &amp;&amp; test "$(grep -cE '^## .*0\.[0-9]' CHANGELOG.md)" = 0 &amp;&amp; test "$(grep -ciE 'erste öffentliche' CHANGELOG.md)" -ge 1 &amp;&amp; test "$(grep -cE 'ß' CHANGELOG.md)" = 0 &amp;&amp; test "$(grep -ciE 'amd64' CHANGELOG.md)" -ge 1 &amp;&amp; test "$(grep -ciE 'govulncheck|1\.26\.6' CHANGELOG.md)" -ge 1 &amp;&amp; test "$(grep -ciE 'holzcloud' CHANGELOG.md)" -ge 1 &amp;&amp; test "$(grep -ciE 'AGPL' CHANGELOG.md)" -ge 1 &amp;&amp; test "$(grep -ciE 'filter-repo|Historienbereinigung|<die Kundennamen>' CHANGELOG.md)" = 0 &amp;&amp; echo TASK3_OK</automated>
  </verify>
  <done>
    `CHANGELOG.md` exists, is German, contains no `ß`, and carries exactly one
    release heading naming 1.4 and none naming any earlier version. It says in so
    many words that this is the first public release. All four user-facing
    changes are present — the theme, amd64-only builds, the AGPL notice, the Go
    raise with govulncheck. None of the history-surgery or customer-data-removal
    work appears. The file states which changelog convention it follows and why
    prose was chosen over bullets.
  </done>
</task>

</tasks>

<threat_model>
## Trust Boundaries

| Boundary | Description |
|----------|-------------|
| repository -> public reader | Documentation crosses from the maintainer to an audience that cannot verify any claim except against the repository itself. This is the only boundary this task touches; no code, no runtime input, no dependency is added. |

## STRIDE Threat Register

| Threat ID | Category | Component | Severity | Disposition | Mitigation Plan |
|-----------|----------|-----------|----------|-------------|-----------------|
| T-hkh-01 | Repudiation | CONTRIBUTING.md authorship section | medium | mitigate | The disclosure that a large part of the code was model-written must survive the rewrite verbatim in substance. Task 2 pins it with a positive grep for `KI-Agenten` so a rewrite cannot quietly drop the disclosure along with its evidence. |
| T-hkh-02 | Information disclosure | README.md `## Versionen` | low | mitigate | Task 1 forbids naming the private archive repository or its visibility. A public README that points at a private repository by name invites requests for access and leaks an account-level detail that is nobody's business. |
| T-hkh-03 | Tampering | README.md licence statement | high | mitigate | README naming a permissive licence while LICENSE is AGPL-3.0 misstates the terms under which the software is offered. Task 1 step 3 corrects it and gates on it; the summary states the change so it is reviewed rather than absorbed. |
| T-hkh-04 | Spoofing | CHANGELOG.md | medium | mitigate | A fabricated pre-1.4 release history would misrepresent the project's provenance to anyone assessing its maturity. Task 3 gates on zero release headings below 1.4. |
| T-hkh-SC | Tampering | npm/pip/cargo installs | n/a | accept | No package manager runs in this task. No dependency is added, removed or upgraded; `go.mod` and `go.sum` are outside the authorised file scope. The package-legitimacy gate does not apply. |
</threat_model>

<verification>
Run all of it from `/Users/holz/Projects/holzcloud-cms`.

**1. Nothing in the codebase moved.** These are documentation-only changes, so
the entire suite is a guard: the gate is that it is exactly as green as it was
before.

```bash
go build ./...            # exit 0
go vet ./...              # exit 0
go test ./... 2>&1 | tee /tmp/hkh-test.log
test "$(grep -c '^FAIL' /tmp/hkh-test.log)" = 0
test "$(grep -cE '^(ok|---)' /tmp/hkh-test.log)" -ge 39
```

39 is the number of packages carrying tests (`go list` over 41 packages found 39
with `TestGoFiles` or `XTestGoFiles`), measured at HEAD before this task. If the
count comes back lower, something was skipped, not something was deleted — look
at the log rather than lowering the number.

**2. No stale reference survives.** Every one of these must print `0`:

```bash
grep -cE 'archive/gsd-v1\.1-dead' README.md
grep -cE 'v1\.0' README.md
grep -cE 'v1\.2' README.md
grep -cE 'v1\.3' README.md
grep -cE 'merge-base' README.md
grep -cE 'git log --format' CONTRIBUTING.md
grep -cE 'uniq -c' CONTRIBUTING.md
grep -cE '(^|[^A-Za-z])MIT([^A-Za-z]|$)' README.md
```

Counts at HEAD before the task, so a passing gate can be told apart from a gate
that was already passing: README `archive/gsd-v1.1-dead` 1, `v1.0` 2, `v1.2` 0,
`v1.3` 3, `merge-base` 1, `MIT` 1; CONTRIBUTING `git log --format` 1, `uniq -c`
1. Only the `v1.2` gate is trivially satisfied today — it guards against the
rewrite introducing the reference, not against leaving it.

And these must be non-zero, so the rewrite is a rewrite and not a deletion:

```bash
grep -cE '^## Versionen$' README.md                            # 1
grep -cE 'v1\.4' README.md                                     # >= 1
grep -cE 'git describe' README.md                              # >= 1
grep -cE 'holzcloud version' README.md                         # >= 1
grep -ciE 'AGPL|Affero' README.md                              # >= 1
grep -cE '^## Wer diesen Code geschrieben hat$' CONTRIBUTING.md # 1
grep -cE 'KI-Agenten' CONTRIBUTING.md                          # >= 1
```

**3. Translations untouched.** No translatable string should move — this task
edits no Go source and no template — but the check costs a second and catches an
accidental edit outside scope:

```bash
go run ./tools/i18n
```

`en.json`, `es.json` and `fr.json` must each report `0 offen, 0 verwaist`, and
`de-CH`, `fr-CH`, `it-CH` must each report `0 ohne Gegenstück`. At HEAD that is
1128 strings and 51 / 4 / 9 regional divergences respectively; the divergence
counts may not change either.

**4. Read it as a stranger would.** Automation cannot check the thing this task
is actually for. Before committing, read the finished `## Versionen`, the
finished authorship section and the finished CHANGELOG end to end and ask of
every sentence: could someone holding only the new repository check this? If the
answer for any sentence is no, that sentence is the same defect this task exists
to remove.
</verification>

<success_criteria>
- `go build ./...`, `go vet ./...`, `go test ./...` green; 0 FAIL; at least 39
  test packages reporting.
- All eight negative greps in Verification step 2 return 0; all seven positive
  greps return their stated minimum.
- `go run ./tools/i18n` reports en/es/fr at `0 offen, 0 verwaist` and the three
  CH variants at `0 ohne Gegenstück`.
- `CHANGELOG.md` exists, is German, carries exactly one release heading naming
  1.4 and none naming anything earlier.
- The three documents agree with each other about why this repository begins at
  v1.4, and none of them names a branch, tag or commit that the new repository
  will not contain.
- The summary states the README licence correction explicitly, as a change the
  developer was not asked for and may revert.
</success_criteria>

<output>
Create `.planning/quick/260903-hkh-dokumente-fuer-den-umzug-ins-frische-rep/260903-hkh-SUMMARY.md` when done.

Commit the three documentation files together — they are one change and a
partial commit would leave the repository asserting two different stories about
its own history.
</output>
