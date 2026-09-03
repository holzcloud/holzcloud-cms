---
phase: quick-260903-bsk
plan: 01
subsystem: infra
tags: [govulncheck, go-toolchain, agpl, licensing, i18n, github-actions, gitignore]

requires:
  - phase: 260903-ato
    provides: the x86 retarget and the 30-minute audit timeout this workflow file already carried
provides:
  - "Go language floor raised to 1.26.6 — the eight called standard-library vulnerabilities of 2026-09-03 are gone"
  - "A weekly govulncheck step in security.yml that may fail the job"
  - "An AGPL §13 notice in the admin sidebar: program, running version, licence, source link"
  - "A test that fails when that footer renders empty"
  - "Two committed build artefacts removed from the tree and covered by .gitignore"
affects: [release, ci, admin-ui, plugins]

actuals:
  tokens: 207000
  tasks: 3
  commits: 4

tech-stack:
  added: [govulncheck]
  patterns:
    - "CI scans for vulnerabilities on its own schedule and is allowed to go red"
    - "The licence obligation is asserted by a rendering test, not by eye"

key-files:
  created: []
  modified:
    - go.mod
    - .github/workflows/security.yml
    - .gitignore
    - cmd/holzcloud/templates/admin/base.html
    - cmd/holzcloud/assets/admin.css
    - internal/web/render_test.go
    - internal/i18n/locales/en.json
    - internal/i18n/locales/es.json
    - internal/i18n/locales/fr.json
    - internal/i18n/locales/it.json

key-decisions:
  - "D-01 — no toolchain line beside the go directive: both numbers would be 1.26.6, and two places can disagree"
  - "D-02/D-03 — govulncheck installed at @latest outside the module, run last, with no continue-on-error"
  - "D-04 — the AGPL notice goes in the admin only; the eight shipped public templates are user-replaceable and would lose it on the first upload"
  - "The product name in the footer is the literal Holzcloud CMS, not .Brand.Name: the licence names the program, not the label the operator puts on their installation"
  - "Only plugins/kontaktformular/kontaktformular.zip is ignored; the other four archives and all five plugin.wasm stay tracked, pending a separate decision"

patterns-established:
  - "A licence/compliance obligation gets a failing test before it gets markup — the named failure mode is an empty gap, which no smoke test would catch"
  - "The i18n catalogues are written by tools/i18n and nothing else; hand-formatting drifts from the tool"

requirements-completed: [DEP-01, QUAL-01]

coverage:
  - id: D1
    description: "govulncheck ./... reports no called standard-library vulnerability, down from eight"
    requirement: DEP-01
    verification:
      - kind: other
        ref: "govulncheck ./... — exit 0, 'Your code is affected by 0 vulnerabilities.'"
        status: pass
      - kind: other
        ref: "go version — go1.26.6 darwin/arm64, proving the raised directive is binding locally"
        status: pass
    human_judgment: false
  - id: D2
    description: "The weekly security workflow scans for vulnerabilities itself and may fail the job"
    verification:
      - kind: other
        ref: "python3 yaml.safe_load over .github/workflows/{ci,release,security}.yml — all parse; last audit step runs govulncheck; no continue-on-error on any step"
        status: pass
    human_judgment: true
    rationale: "Only a real scheduled run on a hosted runner proves that govulncheck lands on PATH there and that the job actually goes red; local YAML parsing cannot show that."
  - id: D3
    description: "Every admin layout page names the program, its running version, the AGPL-3.0 and links the source"
    verification:
      - kind: unit
        ref: "internal/web/render_test.go#TestSidebarFooterShowsBuildAndLicence"
        status: pass
      - kind: unit
        ref: "internal/web/render_test.go#TestBuildStampNeverEmpty"
        status: pass
      - kind: other
        ref: "negative check — footer deleted from base.html by hand, all four assertions failed, template restored"
        status: pass
    human_judgment: false
  - id: D4
    description: "The footer sits at the bottom of the sidebar without squashing the navigation, on a wide and on a narrow window"
    verification: []
    human_judgment: true
    rationale: "The .sidebar-build rule had never been rendered before this change. Its layout behaviour in the column flex container is a visual judgement no assertion in this repository makes; no browser tooling was available to this executor."
  - id: D5
    description: "en, es, fr and it each report 0 offen, 0 verwaist with the two new strings translated"
    requirement: QUAL-01
    verification:
      - kind: other
        ref: "go run ./tools/i18n — 1128 strings, en/es/fr/it each '0 offen, 0 verwaist'; de-CH rebuilt via -schweiz"
        status: pass
    human_judgment: false
  - id: D6
    description: "The x86-64 ELF and the kontaktformular archive are untracked, absent and ignored; the wasm files and four archives untouched"
    verification:
      - kind: other
        ref: "git ls-files on both paths is empty; both re-created and confirmed by git check-ignore; git ls-files 'plugins/*/plugin.wasm' still 5, 'plugins/*/*.zip' still 4; go build ./... green"
        status: pass
    human_judgment: false

duration: 20min
completed: 2026-09-03
status: complete
---

# Quick 260903-bsk: Härtung nach der Freigabeprüfung — Summary

**Eight called standard-library vulnerabilities closed by a toolchain floor of 1.26.6, a weekly govulncheck that is allowed to go red, an AGPL §13 notice the admin now actually makes, and 5 MB of build artefacts out of the working tree.**

## Performance

- **Duration:** ~20 min
- **Started:** 2026-09-03T06:28Z
- **Completed:** 2026-09-03T06:47Z
- **Tasks:** 3 of 3
- **Files modified:** 12 (10 modified, 2 deleted)

## Accomplishments

- **The vulnerabilities are gone, and proven gone.** `go.mod` now declares `go 1.26.6`. The local toolchain re-execed into 1.26.6 (`go version` confirms it — this mattered, because govulncheck reports the *resolved* toolchain's standard library, so a scan run under 1.26.4 would have shown all eight findings and looked exactly like a failed change). `govulncheck ./...` exits 0 with "Your code is affected by 0 vulnerabilities." No `Found in: …@go1.*` line remains.
- **CI will find the next batch without a human.** `security.yml` gained a final `Scan for known vulnerabilities` step. No `continue-on-error` anywhere in the job. It runs after `Vet` so an upstream advisory cannot mask a test regression.
- **The AGPL §13 offer is now made.** The sidebar of every layout page carries `Holzcloud CMS {{.Version}}`, `{{t "Freie Software unter der AGPL-3.0."}}` and an anchor to `{{.SourceURL}}`. Written test-first: the test was red on all four assertions before any markup existed, and was checked once more against a hand-deleted footer to prove it is not vacuous.
- **A CSS rule that had never been on screen was fixed before it shipped.** `.sidebar-build` already existed with `flex-basis: 100%`. `.sidebar` is `flex-direction: column`, where that declaration sets the **height**, not a line break — it would have starved `.sidebar-nav` to nothing on the first render. Replaced with `margin-block-start: auto`.
- **Both build artefacts are gone and cannot come back silently.** The 3.7 MB statically linked x86-64 ELF (`file` confirms: `ELF 64-bit LSB executable, x86-64, statically linked, with debug_info, not stripped`) and the 1.3 MB `kontaktformular.zip`. The existing `.gitignore` block for exactly this mistake listed four plugins and had forgotten the fifth — which is precisely why this one binary was committed and its four siblings never were. It now lists all five.

## Task Commits

1. **Task 1 (RED): the failing footer test** — `adf3361` (test)
2. **Task 1 (GREEN): footer markup, CSS fix, five catalogues** — `1ff10c5` (feat)
3. **Task 2: Go floor 1.26.6 + govulncheck in CI** — `6ff0b8b` (chore)
4. **Task 3: the two artefacts out of the tree** — `b71e95b` (chore)

Task 1 needed no REFACTOR commit. Plan metadata commit is the orchestrator's.

## Files Created/Modified

- `go.mod` — `go 1.26.2` → `go 1.26.6`. No `toolchain` line (D-01). No `require` touched. `go mod tidy` verified byte-identical afterwards, so `ci.yml`'s tidy check will not trip.
- `.github/workflows/security.yml` — final `Scan for known vulnerabilities` step, plus a comment in the register of the existing timeout comment explaining why it may fail the job.
- `internal/web/render_test.go` — `TestSidebarFooterShowsBuildAndLicence` (program name, version, `AGPL`, `href=` to the source URL) and `TestBuildStampNeverEmpty` (defaults survive `SetBuild("", "")`).
- `cmd/holzcloud/templates/admin/base.html` — `<p class="sidebar-build">` as the last child of `<aside class="sidebar">`.
- `cmd/holzcloud/assets/admin.css` — `.sidebar-build`: `flex-basis: 100%` → `margin-block-start: auto`, plus `padding: var(--space-2)` (which lines the text up with the nav items above: `.sidebar-nav` contributes `--space-1` and `.nav-item` another, so 1rem) and `line-height: 1.5`. The German comment and `.sidebar-build a { color: inherit }` kept.
- `internal/i18n/locales/{en,es,fr,it}.json` — two new keys, translated by hand; see the deviation below about their formatting.
- `plugins/bestellung/bestellung`, `plugins/kontaktformular/kontaktformular.zip` — removed.
- `.gitignore` — `plugins/*/bestellung` added to the existing native-binary block; the archive as its own entry with a comment recording that the asymmetry is deliberate.

## Decisions Made

All four plan decisions (D-01 … D-04) were followed as written. Two calls made during execution:

- **The literal `Holzcloud CMS`, not `{{.Brand.Name}}`.** The plan asked for this and it is worth restating: `Brand` is what the operator calls their installation, and it is editable in the admin. §13 requires naming the program, which does not change when somebody white-labels the interface.
- **The catalogue reformat was kept rather than reverted** — see the deviation below.

## Deviations from Plan

### Auto-fixed / accepted

**1. [Rule 3 — collateral] `tools/i18n -write` re-indented all four full catalogues**

- **Found during:** Task 1
- **Issue:** The diff for `en/es/fr/it.json` is ~2250 changed lines each, not the two the change actually adds. `writeCatalog` in `tools/i18n/main.go` emits JSON flush-left, while the four committed files carried a two-space indent.
- **Investigation:** `de-CH.json`, `fr-CH.json` and `it-CH.json` were *already* flush-left before this change. So the tool's format is the repository's canonical one, and the indentation in the four full catalogues was drift — almost certainly an editor's JSON formatter during a hand-translation pass (`20bf357`).
- **Decision:** kept the tool's output. The reformat moves all seven catalogues onto one format rather than away from it. Reverting it would have meant hand-formatting files the tool owns, and the next `-write` would undo the revert anyway.
- **Consequence for the reader:** commit `1ff10c5` looks far larger than it is. The substantive diff outside the catalogues is 7,057 characters; the whole diff is 829,633. That gap is entirely whitespace.
- **Verification:** `go run ./tools/i18n` — en/es/fr/it each `0 offen, 0 verwaist`, 1128 strings; the full test suite (which includes `internal/i18n`) green.

**Total deviations:** 1 accepted, 0 fixes needed. No scope creep. Finding A held exactly as the plan recorded it, so **no Go source outside the test file changed** — `internal/web/layoutdata.go` and `cmd/holzcloud/main.go` dropped out of scope as predicted.

## Issues Encountered

- **The toolchain switch had to be confirmed, not assumed.** Before the edit, `go version` reported go1.26.4. After it, go1.26.6 — `GOTOOLCHAIN=auto` re-execed and fetched it without any explicit `golang.org/dl` fallback. Had this silently not happened, the scan would still have shown eight findings.
- **Nothing else.** Full suite: 38 packages `ok`, 0 FAIL, matching the baseline.

## Residual Risk — stated plainly

**This was not a history rewrite.** Both removed blobs remain reachable through every commit that carried them. A fresh clone still pays for the 3.7 MB ELF and the 1.3 MB archive, and both can still be checked out from history by anyone. Nothing in this change reduces the repository's size on the wire or removes an unreviewable binary from the record. That is the accepted consequence of the separate decision not to run `filter-repo`; anyone counting on the repository being free of them needs to know it is not.

**govulncheck still reports uncalled findings.** 1 vulnerability in imported packages and 3 in required modules — `GO-2025-3884`, `GO-2026-6355`, `GO-2026-6354`, `GO-2026-5932`. None is called, so the scan exits 0 and CI stays green. They are worth a look on their own schedule; a dependency bump would clear them.

## Open Questions Deliberately Left Open

- **The four remaining archives and the five `plugin.wasm` files stay tracked.** `plugins/{bestellung,jahreszahl,nicht-gefunden,suche}/*.zip` and all five wasm modules were outside this task by instruction. Whether built artefacts should ship in the repository at all is still undecided, and the `.gitignore` comment says so.
- **D-04: the eight public templates carry no licence line.** The notice is admin-only. A line in `templates/public/*` is lost the moment an operator uploads their own template, and it would drag `internal/tmplspec/TEMPLATE-SPEC.md` — which template authors follow literally — into the change. Recorded as a scoping decision that can be revisited.
- **The five `standalonePages` templates carry no footer:** `login`, `setup`, `set_password`, `two_factor_verify`, `order_print` do not use `base.html`. §13 addresses users of the running service, who are past the login screen; noted rather than fixed.
- **`tools/i18n`'s `writeCatalog` does not indent**, though its own doc comment says "sorted and indented". Cosmetic, out of scope here, worth a one-line fix someday.

## Known Stubs

None. No placeholder text, no empty defaults reaching the UI, no skipped tests. The two new translation values are real translations in each language, not copies of the German and not empty.

## Threat Flags

No new surface. Two notes for the record, neither a finding:

- The footer's `href="{{.SourceURL}}"` is an **outbound hyperlink**, which `CLAUDE.md` classifies as content, not a subresource. Nothing new is fetched at runtime; `default-src 'self'` in `internal/web/headers.go` is unaffected. No stylesheet, icon or font was added.
- `SourceURL` originates from `HOLZCLOUD_SOURCE_URL`, set by the operator, and reaches the page through `html/template`'s URL context, which neutralises a `javascript:` scheme. It is not user input and does not cross a trust boundary.

Threat register dispositions T-bsk-01 through T-bsk-04 are all mitigated. T-bsk-05 (blobs in history) is accepted and stated above. T-bsk-SC (govulncheck via `go install`) is accepted: a first-party `golang.org/x` module resolved through the checksum database, a CI tool that never enters `go.mod` and never reaches the shipped binary.

## Final Gate

| # | Check | Result |
|---|-------|--------|
| 1 | `go build ./...` | green |
| 2 | `go vet ./...` | green |
| 3 | `go test ./...` | 38 ok, 0 FAIL (baseline) |
| 4 | `govulncheck ./...` | exit 0, 0 called vulnerabilities, 0 stdlib findings |
| 5 | `go run ./tools/i18n` | en/es/fr/it each `0 offen, 0 verwaist` |
| 6 | YAML parse of `.github/workflows/*` | ci.yml, release.yml, security.yml all parse |
| 7 | Admin footer renders a non-empty version and source link | asserted by `TestSidebarFooterShowsBuildAndLicence`; fails when the footer is deleted |
| — | `go version` | go1.26.6 darwin/arm64 |
| — | `git ls-files` on the two artefacts | empty; wasm still 5, archives still 4 |

## Next Readiness

Nothing blocks the next piece of work. One thing is outstanding and belongs to a human:

**Visual pass on the sidebar footer (D4).** No browser tooling was reachable from this executor, so the wide/narrow-window look of a CSS rule that had never rendered before has not been seen by anybody. Sign in, look at the bottom of the sidebar: the version should read as a real string rather than a gap, the navigation above it should not be squashed, and the source link should open the repository.

---
*Quick task: 260903-bsk-haertung-nach-der-freigabepruefung*
*Completed: 2026-09-03*

## Self-Check: PASSED

All ten claimed modified files exist on disk. Both claimed-removed artefacts are absent from disk and from the index. All four claimed commits (`adf3361`, `1ff10c5`, `6ff0b8b`, `b71e95b`) are present in `git log`. The working tree is clean apart from this summary.
