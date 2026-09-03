---
phase: quick-260903-bsk
plan: 01
type: execute
wave: 1
depends_on: []
files_modified:
  # Stable scope — these paths are fixed and authorised.
  - go.mod
  - .github/workflows/security.yml
  - .gitignore
  - plugins/bestellung/bestellung
  - plugins/kontaktformular/kontaktformular.zip
  # Expected scope for the AGPL notice, CONDITIONAL on the executor's own
  # reading of internal/web/layoutdata.go and base.html at execution time.
  # See <mutable_scope> — the plumbing appears to be finished already, so the
  # expected change is markup + catalogue only.
  - cmd/holzcloud/templates/admin/base.html
  - cmd/holzcloud/assets/admin.css
  - internal/web/render_test.go
  - internal/i18n/locales/en.json
  - internal/i18n/locales/es.json
  - internal/i18n/locales/fr.json
  - internal/i18n/locales/it.json
  - internal/i18n/locales/de-CH.json
autonomous: true
requirements: [DEP-01, QUAL-01]

estimate:
  tokens: 50000
  raw_tokens: 50000
  tasks: 3
  confidence: low

must_haves:
  truths:
    - "govulncheck ./... reports no called standard-library vulnerability — the eight reachable ones found on 2026-09-03 are gone"
    - "The weekly security workflow scans for vulnerabilities on its own and goes red when it finds one that the code calls"
    - "An operator looking at any admin screen sees which build is running, that it is AGPL-3.0, and a link to the source"
    - "That footer shows a real version string, not an empty gap — asserted by a test, not by eye"
    - "en, es, fr and it each report 0 offen, 0 verwaist after the new strings land"
    - "Neither the compiled Linux binary nor the kontaktformular archive is tracked or reappears untracked"
  artifacts:
    - "go.mod with a go directive of 1.26.6"
    - ".github/workflows/security.yml with a govulncheck step that can fail the job"
    - "cmd/holzcloud/templates/admin/base.html with a sidebar footer carrying version, licence and source link"
    - "internal/web/render_test.go with a test that fails when the footer renders empty"
    - ".gitignore entries covering both removed artefacts"
  key_links:
    - "go.mod go directive -> actions/setup-go go-version-file -> the Go version CI builds and scans with"
    - "ldflags -X main.Version -> main.Version -> web.SetBuild -> web.buildVersion -> NewLayoutData -> LayoutData.Version -> base.html -> rendered HTML"
    - "{{t}} call in base.html -> tools/i18n callsInTemplates regex -> catalogue key -> five locale files"
    - "govulncheck exit status 3 -> the security job fails -> somebody reads the advisory"
---

<objective>
Close the four hardening items from today's open-source readiness audit: raise the Go
floor past eight reachable standard-library vulnerabilities, teach CI to find the next
batch by itself, meet the AGPL §13 obligation the admin UI currently ignores entirely,
and get two committed build artefacts out of the working tree.

Purpose: the eight vulnerabilities are on paths this code actually calls — TLS
handshakes on the listener, `html/template`, the XML importer, the outbound Payrexx
client. They were found by hand; nothing in the repository would have found them. And a
CMS published under the AGPL that names neither its licence nor its source anywhere in
its own interface is not offering what the licence obliges it to offer.

Output: a raised toolchain floor proven by a clean scan, a weekly job that keeps it
clean, a licence footer that renders a real version, and a repository that stops
carrying a 3.7 MB opaque ELF forward.
</objective>

<execution_context>
@/Users/holz/.claude/gsd-core/workflows/execute-plan.md
@/Users/holz/.claude/gsd-core/templates/summary.md
</execution_context>

<context>
@/Users/holz/Projects/holzcloud-cms/CLAUDE.md
@/Users/holz/Projects/holzcloud-cms/go.mod
@/Users/holz/Projects/holzcloud-cms/.github/workflows/security.yml
@/Users/holz/Projects/holzcloud-cms/internal/web/layoutdata.go
@/Users/holz/Projects/holzcloud-cms/cmd/holzcloud/templates/admin/base.html
</context>

<mutable_scope>
The planner read the repository before writing this plan and found three things the
audit did not know. They are recorded as findings, not as instructions to trust blindly
— the executor confirms each by reading the named file, and widens scope if reality has
moved.

**Finding A — the version plumbing already exists, end to end.** The audit assumed the
plumbing was "the substantive part" of item 3. It is already built:

- `cmd/holzcloud/main.go:96` calls `web.SetBuild(Version, os.Getenv("HOLZCLOUD_SOURCE_URL"))`
- `internal/web/layoutdata.go:57-62` declares `LayoutData.Version` and `LayoutData.SourceURL`
- `internal/web/layoutdata.go:65-80` holds `buildVersion` / `buildSource` package vars and `SetBuild`; `buildSource` already defaults to `https://github.com/holzcloud/holzcloud-cms`
- `internal/web/layoutdata.go:140-141` assigns both into every `LayoutData`

So `LayoutData.Version` and `LayoutData.SourceURL` reach every admin page already. If
that still holds at execution time, `internal/web/layoutdata.go` and
`cmd/holzcloud/main.go` need **no change** and drop out of scope. If it does not hold,
the executor adds whatever the chain is missing and says so in the summary.

**Finding B — the CSS already exists and is wrong.** `cmd/holzcloud/assets/admin.css`
lines 302-315 already carry a `.sidebar-build` rule with a German comment naming the
AGPL. It was written for markup that was never added, so it has never been rendered. Its
`flex-basis: 100%` is the wrap-row idiom for forcing an item onto its own line; `.sidebar`
is `display: flex; flex-direction: column`, where that declaration sets the **height** to
100% and would collapse `.sidebar-nav` to nothing. Treat this rule as unproven, not as
finished work.

**Finding C — item 4 is narrower than the tracked tree suggests.** `git ls-files plugins/`
shows five `plugin.wasm` files (out of scope by instruction) and **five** `.zip` archives,
of which this task removes exactly one. `plugins/bestellung/bestellung.zip`,
`plugins/jahreszahl/jahreszahl.zip`, `plugins/nicht-gefunden/nicht-gefunden.zip` and
`plugins/suche/suche.zip` stay tracked. That is a deliberate scope limit, not an
oversight, and the summary records it so the next reader is not surprised.
</mutable_scope>

<decisions>
Four calls this plan makes explicitly rather than by accident.

**D-01 — no `toolchain` line beside the `go` directive.** `go 1.26.6` alone is already
binding in both places that matter: CI resolves its Go from `go-version-file: go.mod`,
and locally `GOTOOLCHAIN` is `auto`, so any `go` command in this module re-execs into a
1.26.6 toolchain and downloads it if absent. A `toolchain` line earns its place only when
the wanted toolchain is *newer* than the language floor. Here the two numbers are the
same, so a second line is a second thing to forget to bump and a second place for them to
disagree.

**D-02 — govulncheck via `go install …@latest`, run as a binary.** This is the invocation
the Go team documents, it keeps installation and scanning as two separately readable lines
in the job log, and `@latest` outside the module never touches `go.mod` or `go.sum`. It is
a CI tool, never an import: nothing about this reaches the shipped binary, so the
"no runtime third party" rule in CLAUDE.md is untouched.

**D-03 — the govulncheck step may fail the job, and runs last.** No `continue-on-error`.
govulncheck exits 3 when the code calls a vulnerable symbol, and a weekly job going red
the moment an advisory lands is the entire point of adding it — a scan whose failure is
swallowed reports to nobody. It goes *after* `go vet`, as the final step, so that an
upstream advisory cannot mask a genuine test regression: one run still gives the full
picture.

**D-04 — the AGPL notice goes in the admin only, not in the eight shipped themes.** §13
obliges an offer of Corresponding Source to network users. The person who can act on that
offer — obtain the source, comply on redistribution, answer for the modification — is the
operator, and the operator lives in the admin. The themes are user-replaceable: a licence
line in `templates/public/*` is lost the moment somebody uploads their own template, so it
buys weaker compliance for a much larger blast radius (eight templates plus a change to
the `internal/tmplspec/TEMPLATE-SPEC.md` contract that template authors follow literally).
Recorded as a scoping decision that can be revisited, not as an omission.
</decisions>

<tasks>

<task type="tracer" tdd="true">
  <name>Task 1: The AGPL §13 notice, wired from the build stamp to the rendered page</name>
  <files>internal/web/render_test.go, cmd/holzcloud/templates/admin/base.html, cmd/holzcloud/assets/admin.css, internal/i18n/locales/en.json, internal/i18n/locales/es.json, internal/i18n/locales/fr.json, internal/i18n/locales/it.json, internal/i18n/locales/de-CH.json</files>
  <read_first>internal/web/layoutdata.go (confirm Finding A: `Version`/`SourceURL` fields, `SetBuild`, and the two assignments in `NewLayoutData`), cmd/holzcloud/templates/admin/base.html (the `<aside class="sidebar">` block and its `<nav class="sidebar-nav">` child), internal/web/render_test.go (`adminTemplateFS`, `testDashboard`, `TestAdminRendersInTheRequestsLanguage` — the existing pattern for rendering the real on-disk templates), cmd/holzcloud/assets/admin.css lines 281-320 (Finding B), tools/i18n/main.go lines 34-55 (`callsInTemplates` — the regex that decides which strings become catalogue keys)</read_first>
  <behavior>
    Start red. Add `TestSidebarFooterShowsBuildAndLicence` to `internal/web/render_test.go`
    before touching the template, and watch it fail against the current base.html.

    - Test 1 — the version reaches the page: render `dashboard` through `RenderAdmin` with
      `testDashboard{LayoutData: LayoutData{Title: "Übersicht", Version: "v9.9.9-test", SourceURL: "https://example.invalid/quelle"}}`
      and assert the body contains both sentinel strings. This is the whole chain from
      `LayoutData` to HTML, and it is the named failure mode: a footer that renders as an
      empty gap because a field was never read.
    - Test 2 — the licence is named: the same body contains `AGPL`.
    - Test 3 — the offer is a link: the same body contains `href="https://example.invalid/quelle"`,
      so the source URL is an anchor target and not merely printed text.
    - Test 4 — the defaults are not empty: assert package vars `buildVersion` and
      `buildSource` are both non-empty, and that after `SetBuild("", "")` they are still
      non-empty. The test lives in package `web`, so both are reachable. This covers the
      one hole Test 1 leaves: a binary built without ldflags must still render `dev` and
      the default GitHub URL rather than nothing.
  </behavior>
  <action>
    Confirm Finding A first. If `LayoutData.Version`, `LayoutData.SourceURL` and their two
    assignments in `NewLayoutData` are all present, change no Go source outside the test
    file. If any link is missing, add it and record the deviation in the summary.

    Write the four assertions above into `internal/web/render_test.go`, following the
    existing `TestAdminRendersInTheRequestsLanguage` shape (`adminTemplateFS(t)`, a
    `httptest.NewRecorder`, `RenderAdmin`). Run them and confirm they fail on the current
    template before writing any markup.

    Then add the footer to `cmd/holzcloud/templates/admin/base.html` as the last child of
    `<aside class="sidebar">`, immediately after `</nav>` and before `</aside>`, carrying
    the class `sidebar-build` that the stylesheet already defines. It holds three things:

    - The program and its build, untranslated: the literal words `Holzcloud CMS` followed
      by `{{.Version}}`. No `{{t}}` wrapper — the line contains no translatable word, and
      an empty key in five catalogues for a product name is noise. Use the literal
      `Holzcloud CMS` and **not** `{{.Brand.Name}}`: `Brand` is the operator's own label
      for their installation, and §13 requires naming the program, which does not change
      when somebody white-labels the interface.
    - The licence, translated: `{{t "Freie Software unter der AGPL-3.0."}}`.
    - The offer, translated, as an outbound anchor: `<a href="{{.SourceURL}}" target="_blank" rel="noopener">{{t "Quelltext"}}</a>`,
      matching the `target`/`rel` pairing already used in `page_form.html`. This is an
      outbound hyperlink, which CLAUDE.md classifies as content and permits; it is not a
      subresource, so nothing new is fetched at runtime and the `default-src 'self'` policy
      in `internal/web/headers.go` is unaffected. Add no stylesheet, no icon, no font.

    Then fix the CSS per Finding B. In `cmd/holzcloud/assets/admin.css`, inside the
    existing `@layer layout` block, the `.sidebar-build` rule declares `flex-basis: 100%`,
    which in the column-flex `.sidebar` sets the height rather than forcing a line break
    and would starve `.sidebar-nav`. Replace that declaration with `margin-block-start: auto`
    and give the rule enough inline padding to line up with the nav items above it. Keep
    the existing German comment and the `.sidebar-build a { color: inherit }` rule. Render
    the admin and look at it before calling this done — this rule has never been on screen.

    Finally the catalogues. Run `go run ./tools/i18n -write` to add the two new keys, then
    fill in `en.json`, `es.json`, `fr.json` and `it.json` by hand — the tool writes empty
    values, and an empty value is a blank line in the interface. Suggested wording, which
    the executor may improve: en `Free software under the AGPL-3.0.` / `Source code`;
    es `Software libre bajo la AGPL-3.0.` / `Código fuente`; fr `Logiciel libre sous licence AGPL-3.0.` /
    `Code source`; it `Software libero con licenza AGPL-3.0.` / `Codice sorgente`. Then run
    `go run ./tools/i18n -schweiz` to rebuild `de-CH.json`. Baseline before this task is
    1126 strings with en/es/fr/it all clean and de-CH at 51 deviations; the binding
    acceptance is the clean report, not a particular count.

    Out of scope here, and worth a line in the summary: the `standalonePages` templates
    (`login`, `setup`, `set_password`, `two_factor_verify`, `order_print`) do not use
    `base.html` and so carry no footer.
  </action>
  <verify>
    <automated>cd /Users/holz/Projects/holzcloud-cms && go test ./internal/web/ -run 'TestSidebarFooterShowsBuildAndLicence|TestAdminRendersInTheRequestsLanguage|TestUnknownLanguageFallsBackToGerman|TestParseAdminTemplatesSucceeds' -count=1 -v 2>&1 | tail -30 && go vet ./internal/web/ && go run ./tools/i18n | python3 -c "
import re, sys
t = sys.stdin.read()
print(t)
bad = [l for l in ('en', 'es', 'fr', 'it') if not re.search(r'^%s\.json\s+\d+ übersetzt, 0 offen, 0 verwaist' % l, t, re.M)]
assert not bad, 'catalogue not clean: ' + ', '.join(bad)
print('TRACER OK — footer renders and the catalogues are clean')
"</automated>
    <human-check>Start the binary against a scratch data dir, sign in, and look at the bottom of the sidebar on a wide window and on a narrow one: the version reads as a real string rather than a gap, the nav above it is not squashed, and the source link opens the repository.</human-check>
  </verify>
  <done>The four assertions pass against the real on-disk templates; the sidebar of every layout page carries the program name, the running version, the AGPL-3.0 and a working source link; en, es, fr and it each report `0 offen, 0 verwaist`; `de-CH.json` has been rebuilt.</done>
</task>

<task type="auto">
  <name>Task 2: Raise the Go floor to 1.26.6 and make CI find the next batch itself</name>
  <files>go.mod, .github/workflows/security.yml</files>
  <read_first>go.mod (the `go` directive on line 3), .github/workflows/security.yml (the whole `audit` job — note `CGO_ENABLED: "0"` at file level, `"1"` only on the race step, and the long comment defending the 30-minute timeout), .github/workflows/ci.yml lines 44-48 (the `go mod tidy` diff check this change must survive)</read_first>
  <action>
    Change the `go` directive in `go.mod` from `1.26.2` to `1.26.6`. Add no `toolchain`
    line, per D-01. Leave every `require` block untouched — this is a language-floor
    change, not a dependency bump.

    The local toolchain is 1.26.4 and `GOTOOLCHAIN` is `auto`, so the first `go` command
    after the edit downloads 1.26.6 and re-execs into it. Do not assume it is already
    present, and do not assume it is instant. Confirm with `go version` run from the
    repository root, which must report `go1.26.6` once the directive is in place. If it
    reports 1.26.4, the toolchain switch did not happen: try the scan with an explicit
    `GOTOOLCHAIN=go1.26.6` prefix, and if that still fails, fetch it deliberately with
    `go install golang.org/dl/go1.26.6@latest && go1.26.6 download` before re-running.
    Getting this right matters more than the diff: govulncheck reports the standard
    library version of the toolchain it resolves, so a scan run under 1.26.4 would still
    show all eight findings and look like the change had failed.

    Then add a vulnerability scan to `.github/workflows/security.yml`, as the last step of
    the `audit` job, after `Vet`. Two lines in one `run` block: `go install
    golang.org/x/vuln/cmd/govulncheck@latest`, then `govulncheck ./...`. Give it a step
    name that says what it does. Do not set `continue-on-error` on it or on any other step
    in the file, and do not add a `CGO_ENABLED` override — the file-level `"0"` is correct
    and the scanner has no use for cgo.

    Write a comment above the step in the same register as the timeout comment already in
    this file: say that the scan may fail the job by design, that the eight standard
    library vulnerabilities it was added for were found by hand on 2026-09-03 with none of
    this repository's automation noticing, and that it sits after the tests so an upstream
    advisory cannot hide a test regression. If it goes red for a dependency rather than the
    standard library, that is the step working and earns its own fix, not a suppression.

    Then confirm `go mod tidy` leaves the file alone. Raising a patch-level `go` directive
    changes no module graph pruning, so `go.mod` and `go.sum` should both be byte-identical
    after a tidy — the verify gate below asserts exactly that, because the `Verify go.mod is
    tidy` step in `ci.yml` will otherwise fail on the next push.
  </action>
  <verify>
    <automated>cd /Users/holz/Projects/holzcloud-cms && grep -Eq '^go 1\.26\.6$' go.mod && go version && before=$(shasum go.mod go.sum) && go mod tidy && test "$before" = "$(shasum go.mod go.sum)" && go build ./... && go vet ./... && go test ./... 2>&1 | tail -5 && python3 -c "
import yaml
w = yaml.safe_load(open('.github/workflows/security.yml'))
steps = w['jobs']['audit']['steps']
runs = [s.get('run') or '' for s in steps]
assert any('govulncheck' in r for r in runs), 'no step actually runs govulncheck'
assert not any(s.get('continue-on-error') for s in steps), 'a step is allowed to fail silently'
for f in ('ci.yml', 'release.yml', 'security.yml'):
    yaml.safe_load(open('.github/workflows/' + f))
print('workflows parse; the scan step is real and may fail the job')
" && govulncheck ./... 2>&1 | tee /dev/stderr | python3 -c "
import re, sys
t = sys.stdin.read()
std = sorted(set(re.findall(r'Found in: (\S+@go1\.\S+)', t)))
assert not std, 'called standard library vulnerabilities remain: ' + ', '.join(std)
print('OK — no called standard library vulnerability')
"</automated>
  </verify>
  <done>`go.mod` declares `go 1.26.6` with no `toolchain` line; `go mod tidy` is a no-op; build, vet and the full test suite are green under 1.26.6; `govulncheck ./...` reports no reachable standard-library vulnerability, where it reported eight before; `security.yml` has a govulncheck step that can fail the job, and all three workflow files still parse.</done>
</task>

<task type="auto">
  <name>Task 3: Get the two committed build artefacts out of the tree</name>
  <files>plugins/bestellung/bestellung, plugins/kontaktformular/kontaktformular.zip, .gitignore</files>
  <read_first>.gitignore (the German-commented block near the end listing `plugins/*/kontaktformular`, `plugins/*/jahreszahl`, `plugins/*/suche`, `plugins/*/nicht-gefunden` — the block that already exists for exactly this mistake), plugins/README.md (how a plugin is built, so the ignore comment is accurate)</read_first>
  <action>
    Remove both artefacts from the index and from disk with `git rm`:
    `plugins/bestellung/bestellung` (3.7 MB, a statically linked x86-64 ELF sitting beside
    its own source in a repository developed on macOS) and
    `plugins/kontaktformular/kontaktformular.zip` (1.3 MB). Nothing references either path
    — a repository-wide search across `*.go`, `*.md`, `*.yml` and `*.json` outside
    `.planning/` returns no hit — so no code or documentation needs adjusting.

    Do not touch the five `plugins/*/plugin.wasm` files. Do not touch the four other
    tracked archives (`plugins/bestellung/bestellung.zip`,
    `plugins/jahreszahl/jahreszahl.zip`, `plugins/nicht-gefunden/nicht-gefunden.zip`,
    `plugins/suche/suche.zip`). Both exclusions are deliberate: the wasm files are an open
    shipping decision, and the four archives were outside the audit's finding.

    Then extend `.gitignore`, keeping the German comment register of the surrounding file.
    The existing native-binary block names four plugins and simply forgot the fifth: add
    the missing `plugins/*/bestellung` pattern to it, which is precisely why this ELF was
    committed while its four siblings never were. Then add the archive as its own named
    entry, and note in its comment that only this one archive is ignored today while the
    others remain tracked, so a later reader does not mistake the asymmetry for a bug.

    In the summary, state plainly and without hedging that this removes the blobs going
    forward only. Both remain reachable in history — the ELF in every commit that carried
    it — and a clone still pays for them. That is the accepted consequence of the separate
    decision not to run `filter-repo`, and anyone counting on the repository being free of
    them needs to know it is not.
  </action>
  <verify>
    <automated>cd /Users/holz/Projects/holzcloud-cms && test -z "$(git ls-files plugins/bestellung/bestellung plugins/kontaktformular/kontaktformular.zip)" && test ! -e plugins/bestellung/bestellung && test ! -e plugins/kontaktformular/kontaktformular.zip && test "$(git ls-files 'plugins/*/plugin.wasm' | wc -l | tr -d ' ')" = "5" && test "$(git ls-files 'plugins/*/*.zip' | wc -l | tr -d ' ')" = "4" && mkdir -p plugins/bestellung && touch plugins/bestellung/bestellung plugins/kontaktformular/kontaktformular.zip && git check-ignore -q plugins/bestellung/bestellung && git check-ignore -q plugins/kontaktformular/kontaktformular.zip && rm -f plugins/bestellung/bestellung plugins/kontaktformular/kontaktformular.zip && test -z "$(git status --porcelain --ignored=no plugins/ | grep '^??' || true)" && go build ./... && echo "OK — both artefacts gone, ignored on return, wasm and the other four archives untouched"</automated>
  </verify>
  <done>Neither path is tracked or present on disk; recreating either leaves it ignored rather than untracked; the five `plugin.wasm` files and the four remaining archives are still tracked; the build is green.</done>
</task>

</tasks>

<threat_model>
## Trust Boundaries

| Boundary | Description |
|----------|-------------|
| internet → HTTP listener | TLS handshake and request parsing on every public and admin request |
| uploaded content → renderer | WXR import (`encoding/xml`), Markdown and template rendering (`html/template`) |
| operator → licence obligation | a network user of a modified build has a statutory claim to the source |
| CI runner → third-party tooling | `go install` of a scanner into the build environment |

## STRIDE Threat Register

| Threat ID | Category | Component | Severity | Disposition | Mitigation Plan |
|-----------|----------|-----------|----------|-------------|-----------------|
| T-bsk-01 | Information Disclosure / Denial of Service | `crypto/tls`, `net/http`, `html/template`, `encoding/xml`, `net/url`, `encoding/asn1` at go1.26.4 | high | mitigate | Task 2 raises the `go` directive to 1.26.6; acceptance is `govulncheck ./...` finding no `@go1.*` entry, not the diff |
| T-bsk-02 | Tampering | `.github/workflows/security.yml` — no vulnerability scan, so the next advisory is found by hand or not at all | medium | mitigate | Task 2 adds a govulncheck step that may fail the job (D-03) |
| T-bsk-03 | Repudiation | admin UI names neither licence nor source, so AGPL §13's offer is never made | medium | mitigate | Task 1 puts program, version, licence and source link in the sidebar of every layout page, asserted by a test |
| T-bsk-04 | Tampering | `plugins/bestellung/bestellung` — an opaque 3.7 MB binary carried in a source repository, unreviewable and unverifiable against its neighbouring source | medium | mitigate | Task 3 removes and ignores it; residual risk accepted, see T-bsk-05 |
| T-bsk-05 | Tampering | the same blobs stay reachable in git history after Task 3 | low | accept | history rewrite was separately declined; the summary states the residue plainly so nobody assumes otherwise |
| T-bsk-SC | Tampering | `go install golang.org/x/vuln/cmd/govulncheck@latest` in CI | low | accept | first-party Go team module under `golang.org/x`, resolved through the checksum database; it is a CI tool that never enters `go.mod` or the shipped binary. No npm, pip or cargo install is introduced, so the package legitimacy gate does not apply |
</threat_model>

<verification>
Run from `/Users/holz/Projects/holzcloud-cms` after all three tasks:

1. `go build ./... && go vet ./... && go test ./...` — green, 38 packages, 0 FAIL, under go1.26.6.
2. `go version` — reports `go1.26.6`, proving the directive is binding locally as well as in CI.
3. `govulncheck ./...` — no `Found in: …@go1.*` line anywhere in the output. Eight such findings existed on 2026-09-03; the count that matters is zero, and the exit status should now be 0 rather than 3.
4. `go run ./tools/i18n` — `en.json`, `es.json`, `fr.json` and `it.json` each report `0 offen, 0 verwaist`.
5. `python3 -c "import yaml; [yaml.safe_load(open('.github/workflows/'+f)) for f in ('ci.yml','release.yml','security.yml')]"` — every workflow still parses, and the parsed `security.yml` contains a step whose `run` mentions govulncheck with no `continue-on-error` anywhere in the job.
6. `go test ./internal/web/ -run TestSidebarFooterShowsBuildAndLicence -count=1` — passes, and fails if the footer is deleted from `base.html`. Verify that second half once by hand: a test that would pass against an empty footer is worth nothing here.
7. `git ls-files plugins/bestellung/bestellung plugins/kontaktformular/kontaktformular.zip` — empty; `git ls-files 'plugins/*/plugin.wasm'` — still five.
8. Human pass: sign in and read the sidebar footer on a wide and a narrow window.
</verification>

<success_criteria>
- `go.mod` declares `go 1.26.6`, no `toolchain` line, `go mod tidy` a no-op.
- `govulncheck ./...` finds no reachable standard-library vulnerability, down from eight.
- `security.yml` scans for vulnerabilities weekly and can fail on what it finds.
- Every admin layout page shows the program, its real version, `AGPL-3.0` and a working source link, in whichever of the five languages the operator uses.
- en, es, fr and it are each `0 offen, 0 verwaist`; `de-CH.json` rebuilt.
- The ELF and the kontaktformular archive are untracked, absent and ignored; the wasm files and the four other archives are untouched.
- The summary states that the removed blobs remain in history, and records D-04's admin-only scope so the public-template question stays open rather than looking answered.
</success_criteria>

<output>
Create `.planning/quick/260903-bsk-haertung-nach-der-freigabepruefung/260903-bsk-SUMMARY.md` when done.
</output>
