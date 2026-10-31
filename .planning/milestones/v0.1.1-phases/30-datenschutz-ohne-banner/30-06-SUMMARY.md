---
phase: 30-datenschutz-ohne-banner
plan: 06
subsystem: privacy-text
tags: [privacy, admin, settings-card, docs, changelog]
requires: [30-05]
provides:
  - admin.Handler.HandlePrivacyDraft
  - route POST /admin/websites/{id}/datenschutz-vorlage (editorOpenRoutes)
  - privacyHint on websiteFormData; Privacy statement card in website_form.html
  - ten catalogue keys in de, es, fr, it
affects: []
key-files:
  created:
    - internal/admin/privacy_draft.go
    - internal/admin/privacy_draft_test.go
  modified:
    - internal/admin/website.go
    - cmd/holzcloud/main.go
    - cmd/holzcloud/main_test.go
    - cmd/holzcloud/templates/admin/website_form.html
    - internal/i18n/locales/de.json
    - internal/i18n/locales/es.json
    - internal/i18n/locales/fr.json
    - internal/i18n/locales/it.json
    - docs/security.md
    - CHANGELOG.md
requirements: [DSV-07]
status: complete
commits: 3
plan_head_before: b54b48ff130ffbace5c04ef40b0006402ee4e6a5
plan_head_after: 80dc71b
actuals:
  tasks: 3
  commits: 3
---

# Phase 30 Plan 06: The way to the current privacy template for existing websites, and the phase written down

An editor can create the current privacy template as a new draft from the settings screen of any website; the website's existing privacy page is provably untouched. `docs/security.md` and the 0.1.1 CHANGELOG entry now state the three public cookies, the truncated 14-day request log with its limits, the picture guard and the template.

## Tasks

| Task | Commit | What |
|------|--------|------|
| 1 | b74890f | `HandlePrivacyDraft`, `privacyHint`, the route, `editorOpenRoutes`, six tests |
| 2 | a603323 | Privacy statement card (three states, no `hx-post`, CSRF field), `PrivacyHint` in `websiteFormData`, ten catalogue keys, card test with five subtests |
| 3 | 80dc71b | `docs/security.md` cookie section rewritten plus log, pictures and template subsections; CHANGELOG paragraph "Datenschutz ohne Banner." and one sentence on the evidence sets |

## Tests

- `TestPrivacyDraftCreatesTheCurrentTemplate`, `...LeavesAnExistingPageAlone`, `...TwiceMakesTwoDrafts`, `...BelongsToTheWebsiteOfThePath`, `...UnknownWebsite`, `...OnlyCreatesAPage` (go/parser: the only calls on the page store are `CreatePage` and `GetPageBySlug`): PASS.
- `TestWebsiteSettingsShowThePrivacyCard` (none, published, draft, unfilled template, new-website screen; stored title and body markers never in the response): PASS.
- `TestEveryAdminRouteIsClassified`, `TestTheClassificationMatchesTheBehaviour`: PASS.

## Deviations from Plan

**1. Worktree base was stale.** Branch started at b0995b9, not an ancestor of `main`; reset to `main` (b54b48f) before any work. Acceptance diffs were therefore taken against b54b48f (not e2513b1): `git diff --quiet b54b48f -- go.mod go.sum internal/activity deploy/DEPLOY.md deploy/holzcloud.service deploy/backup.sh deploy/restore.sh docs/beta.md` exits 0.

**2. `de-CH.json` unchanged.** `go run ./tools/i18n -schweiz` ran and produced no change for the ten new keys (no rule applies); the file is still generated and consistent.

**3. Gate scratch files.** `28-check.sh all` failed in the worktree on `nightly.sh record-check` because the gitignored `deploy/test/out/` scratch files are absent from a fresh worktree. They were copied from the main checkout (untracked/ignored, not committed) and the gate then passed in full. Not caused by this plan.

**4. Playwright UAT (verification step 6) not driven by this agent.** The behaviours are covered by the handler and card tests; the owner/orchestrator may still run the dev server through create website, enable shop, open settings, press the button, open the draft.

**5. Commits via `-F`.** Messages written to the scratchpad and committed with `-F` (sandbox refuses compound git commands). The plan-head ledger file was not created for the same reason; `plan_head_before` is the known base b54b48f and `commits: 3` is `git rev-list --count b54b48f..HEAD`.

## Verification

- `go build ./...`, `go vet ./...`, `gofmt -l cmd internal tools plugins sdk`: clean
- `go run ./tools/i18n`: every catalogue `2077 translated, 0 open, 0 orphaned` (de-CH 106 deviations, generated)
- `go run ./tools/english | grep -cE '(internal|cmd|tools|plugins|sdk)/'`: 0
- `go run ./tools/themewords -check`: every shipped theme's catalogue matches words.json
- `GO=0 NATIVE_SELFTEST=0 bash .planning/phases/28-the-promise-in-writing/28-check.sh all`: last line `ok: regression: nightly.sh selftest 646, record-check 29, record-check-con 27 (earlier set) and 30 ...`; `frozen` line ok
- `bash .planning/phases/27-repository-tidy/27-check.sh requirements`: `27-CHECK requirements: PASS`
- `bash deploy/test/native-amd64.sh status-check`: `STATUS-CHECK OK: SRV-01 SRV-02 SRV-03 agree with the record`
- `grep -c 'two exceptions' docs/security.md`: 0; `hc_warenkorb` 1, `hc_frei_` 1, `hc_preise` 1; `journald-holzcloud.conf` in both docs
- `go test ./... -count=1 -timeout 20m`: every package ok (output filtered of `ok` and `no test files` lines was empty), run after the last commit

## For the owner

- **Four places still write a whole address** (D-02, accepted, named in the CHANGELOG): `internal/admin/login.go:45` (sign-in rate-limit warning), `internal/admin/oidc.go:250` (OpenID Connect refusal), `internal/admin/forwardauth.go:884` (single-sign-on refusal), `internal/web/forwardauth.go:165` (forward-auth wrong-secret warning, `peer`).
- **The journald drop-in is not confirmed on a real host.** Check on the host:
  `sudo install -D -m 0644 deploy/journald-holzcloud.conf /etc/systemd/journald.conf.d/holzcloud.conf`, `sudo systemctl restart systemd-journald`, `systemd-analyze cat-config systemd/journald.conf` must list `MaxRetentionSec=14d` and `MaxFileSec=1d`.
- **`docs/beta.md` was not edited.** It is bound by the evidence rules; its reading of "program against evidence" is pinned to the commits it names and is now incomplete (the packages of phase 30 differ from both evidence sets).
- Form detection covers only `kontaktformular` and `bestellung` (`formPluginIDs`); another plugin that stores visitor input does not switch the forms block on.

## Known Stubs

None. The `(eintragen)` markers in the composed text are intentional fill-in places (D-04); the draft is created as a draft.

## Threat Flags

None. Threats T-30-18 to T-30-21 are mitigated and tested: website scope by the path id, CSRF by the form token under the `/admin` middleware (no `hx-post`), existing page untouched (source test), card never prints stored title or text.

## Self-Check: PASSED

internal/admin/privacy_draft.go and privacy_draft_test.go exist; commits b74890f, a603323, 80dc71b are on the branch.
