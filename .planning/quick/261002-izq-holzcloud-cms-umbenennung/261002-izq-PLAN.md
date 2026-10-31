---
phase: quick-261002-izq
plan: 01
type: execute
wave: 1
depends_on: []
files_modified:
  - README.md
  - docs/security.md
  - docs/deployment.md
  - docs/vergleich-statamic.md
  - deploy/DEPLOY.md
  - deploy/Caddyfile.example
  - deploy/backup.sh
  - cmd/holzcloud/templates/admin/dashboard.html
  - cmd/holzcloud/templates/admin/mail_status.html
  - cmd/holzcloud/templates/admin/branding.html
  - cmd/holzcloud/templates/public/default/style.css
  - internal/admin/account.go
  - internal/admin/mail.go
  - internal/admin/twofactor.go
  - internal/branding/branding.go
  - internal/branding/branding_test.go
  - internal/totp/totp_test.go
  - internal/i18n/locales/de.json
  - internal/i18n/locales/fr.json
  - internal/i18n/locales/it.json
  - internal/i18n/locales/es.json
  - internal/i18n/locales/de-CH.json
  - CHANGELOG.md
autonomous: true
requirements: [RENAME-01]
must_haves:
  truths:
    - "Every operator/user-facing mention of the program says holzcloud-CMS"
    - "Technical identifiers (HOLZCLOUD_*, X-Holzcloud-*, module path, binary, theme slug, cookie/unit/repo names, holzcloud.ch, history) are unchanged"
    - "i18n gate reports 0 open, 0 orphaned; all CI gates and listed tests pass"
  artifacts:
    - path: "CHANGELOG.md"
      provides: "## 0.0.17 — 2026-10-02 entry"
      contains: "## 0.0.17"
  key_links:
    - from: "templates / Go T() keys"
      to: "internal/i18n/locales/*.json"
      via: "German/English sentence key"
---

<objective>
Finish the rename to "holzcloud-CMS" (exact spelling) wherever the PROGRAM is named in user/operator-facing text. Release 0.0.16 did most of it.
Output: updated docs, templates, Go strings, locale catalogues, tests, CHANGELOG 0.0.17.
</objective>

<execution_context>
@$HOME/.claude/get-shit-done/workflows/execute-plan.md
@$HOME/.claude/get-shit-done/templates/summary.md
</execution_context>

<context>
@CLAUDE.md
@.planning/STATE.md
</context>

<tasks>

<task type="auto">
  <name>Task 1: Docs, deploy files, theme comment (prose only)</name>
  <files>README.md, docs/security.md, docs/deployment.md, docs/vergleich-statamic.md, deploy/DEPLOY.md, deploy/Caddyfile.example, deploy/backup.sh, cmd/holzcloud/templates/public/default/style.css</files>
  <action>
Run `git grep -n Holzcloud -- ':!.planning' ':!CHANGELOG.md'` and replace "Holzcloud" (as product name) with "holzcloud-CMS" in the listed files only: README.md (lines ~55, ~159 incl. image alt), docs/security.md (heading, TOC entry AND anchor: new heading "Why a holzcloud-CMS site needs no cookie banner" gives anchor `#why-a-holzcloud-cms-site-needs-no-cookie-banner`; update the TOC link and grep the whole repo incl. other docs/README/tests for the old anchor), docs/deployment.md, docs/vergleich-statamic.md (title "holzcloud-CMS beside Statamic" etc.; fix grammar of "a holzcloud-CMS" / "Holzcloud a page had" sentences, and the table header), deploy/DEPLOY.md, deploy/Caddyfile.example comments, deploy/backup.sh echo line ("=== holzcloud-CMS backup: ..."), default/style.css header comment "Holzcloud — Standardvorlage" -> "holzcloud-CMS — Standardvorlage" (NOT public/holzcloud/style.css: theme named holzcloud keeps its name; if style.css has a content-hash/version test, `cmd/holzcloud/stylesheet_version_test.go` — run it and update the expected version only if it is derived from content and fails).
DO NOT touch: X-Holzcloud-Proxy-Secret, HOLZCLOUD_*, paths, URLs, repo/container/unit names, holzcloud.ch, `holzcloud` command. Start of sentence: keep lowercase "holzcloud-CMS" (exact spelling) even at sentence start. Also check docs/brand/README (or similar) and list in the final summary: docs/brand/holzcloud-social.png still reads "Holzcloud CMS" — ORCHESTRATOR FOLLOW-UP to regenerate; executor must not touch it. Check any other image/README text referencing it and note in the summary.
  </action>
  <verify>
    <automated>cd /home/user/holzcloud-cms && ! git grep -n 'Holzcloud' -- README.md docs deploy ':!deploy/Caddyfile.example' | grep -v 'X-Holzcloud' ; git grep -n 'Holzcloud' -- deploy/Caddyfile.example | grep -v 'X-Holzcloud'; go test ./cmd/holzcloud/ -timeout 20m</automated>
  </verify>
  <done>No product-name "Holzcloud" remains in these files (only X-Holzcloud-* identifiers); anchor and TOC consistent; cmd/holzcloud tests pass.</done>
</task>

<task type="auto">
  <name>Task 2: Admin strings, Go code, catalogue keys, tests</name>
  <files>cmd/holzcloud/templates/admin/dashboard.html, cmd/holzcloud/templates/admin/mail_status.html, cmd/holzcloud/templates/admin/branding.html, internal/admin/account.go, internal/admin/mail.go, internal/admin/twofactor.go, internal/branding/branding.go, internal/branding/branding_test.go, internal/totp/totp_test.go, internal/i18n/locales/*.json</files>
  <action>
1. Templates: dashboard.html "Welcome to Holzcloud" -> "Welcome to holzcloud-CMS"; mail_status.html "Holzcloud sends email only…" -> "holzcloud-CMS sends email only…"; branding.html hint text mentioning Holzcloud and placeholder="Holzcloud" -> "holzcloud-CMS".
2. Go: account.go subject "Your access to holzcloud-CMS"; mail.go "Test message from holzcloud-CMS"; twofactor.go TOTP issuer fallback "holzcloud-CMS" and the "holzcloud-CMS (host)" form (line ~344-346; update comments); internal/branding/branding.go DefaultName = "holzcloud-CMS" (and its comments). Check how the brand initial/letter (the "H" mark, see branding_test.go ~73, ~426 and any SVG/logo letter code) is derived from DefaultName: keep the mark an "H" (upper-case first letter) so the logo does not change to "h"; adjust code or test deliberately if it derives from name[0]. Update tests asserting old strings: branding_test.go (lines ~153-156 expected fallback "holzcloud-CMS"), internal/totp/totp_test.go (issuer strings -> "holzcloud-CMS (example.de)" / `issuer=holzcloud-CMS` with proper URL-escaping as URI() does), and any admin tests grep'd via `git grep -n 'Holzcloud' -- '*_test.go'` that assert these strings (cmd/holzcloud/deploy_docs_test.go and stylesheet test name are identifiers: leave). Existing 2FA enrolments keep their stored label: do not migrate data.
   Leave unchanged (not program naming in operator text): template/loader.go theme entry {Name:"Holzcloud", Slug:"holzcloud"} (theme name), plugins/*/plugin.json "author" (publisher), ai/mcp.go Bearer realm (protocol identifier), code comments, migration comment.
3. i18n: keys are the English sentences, so renaming changes keys: "Welcome to Holzcloud", "Holzcloud sends email only when…", "Test message from Holzcloud", "Your access to Holzcloud", and the long branding hint key. Run `go run ./tools/i18n -write`, then in de/fr/it/es.json fill translations for the new keys using the old values with the name swapped to holzcloud-CMS (de uses du-form: e.g. "Willkommen bei holzcloud-CMS", "Dein Zugang zu holzcloud-CMS", "Testnachricht von holzcloud-CMS"); remove the orphaned old keys; edit hand-kept de-CH.json (and fr-CH/it-CH if they contain them: `git grep -n Holzcloud internal/i18n/locales`) so keys match the new sentences, Swiss spelling (ss). Then `go run ./tools/i18n -schweiz`. Gate: `go run ./tools/i18n` => 0 open, 0 orphaned. Also check internal/i18n/copied_test.go prose mentioning "Holzcloud" (comment only; update wording if trivial).
  </action>
  <verify>
    <automated>cd /home/user/holzcloud-cms && go run ./tools/i18n && go test ./internal/web/ ./internal/i18n/ ./internal/branding/ ./internal/totp/ -timeout 20m && go test ./internal/admin/ -run 'Mail|TwoFactor|Brand|Dashboard|Account' -timeout 20m</automated>
  </verify>
  <done>Final `git grep -n Holzcloud -- ':!.planning' ':!CHANGELOG.md'` shows only the allowed leftovers (identifiers, theme name/slug, plugin author, realm, comments); i18n gate clean; tests pass.</done>
</task>

<task type="auto">
  <name>Task 3: CHANGELOG 0.0.17 and CI gates</name>
  <files>CHANGELOG.md</files>
  <action>
Insert above "## 0.0.16 — 2026-10-02" a new section "## 0.0.17 — 2026-10-02" in German, Swiss spelling (ss, never ß), a bold lead sentence, whole sentences, in the style of the 0.0.16 entry (read it first, mirror its groups/headings). Content: the remaining mentions now say holzcloud-CMS — dashboard greeting, mail-status and branding screens, default brand name, the docs and deploy files, mail subjects ("Dein Zugang zu holzcloud-CMS", "Testnachricht von holzcloud-CMS"), and the name shown in authenticator apps for NEW enrolments; existing entries keep their old label. Mention that technical names (HOLZCLOUD_* variables, X-Holzcloud-* header, the `holzcloud` command, the theme "holzcloud") are unchanged. Do not edit older entries. Then run all gates and fix findings: gofmt -l . (must print nothing), go vet ./..., go run ./tools/english, go run ./tools/cites -check, go run ./tools/assembled -check, go run ./tools/themewords -check, go run ./tools/i18n. Do not commit docs/brand/holzcloud-social.png changes.
  </action>
  <verify>
    <automated>cd /home/user/holzcloud-cms && test -z "$(gofmt -l .)" && go vet ./... && go run ./tools/english && go run ./tools/cites -check && go run ./tools/assembled -check && go run ./tools/themewords -check && go run ./tools/i18n && grep -c '^## 0.0.17 — 2026-10-02' CHANGELOG.md</automated>
  </verify>
  <done>CHANGELOG has the 0.0.17 entry; all gates green.</done>
</task>

</tasks>

<verification>
Gates from Task 3 plus tests from Tasks 1-2. Orchestrator follow-up (not executor): regenerate docs/brand/holzcloud-social.png (text "Holzcloud CMS" -> "holzcloud-CMS") and review README images for embedded old name.
</verification>

<success_criteria>
All program-name mentions read holzcloud-CMS; no technical identifier changed; gates and tests green; CHANGELOG 0.0.17 present.
</success_criteria>

<output>
Create `.planning/quick/261002-izq-holzcloud-cms-umbenennung/261002-izq-SUMMARY.md` when done
</output>
