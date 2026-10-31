---
status: partial
phase: 30-datenschutz-ohne-banner
source: 30-06-SUMMARY.md, 30-HUMAN-UAT.md
started: 2026-10-31T11:05:00+01:00
updated: 2026-10-31T11:12:00+01:00
---

## Current Test

[testing paused — 1 item outstanding]

## Tests

Driven by Claude through the Playwright browser against a fresh dev instance (empty data dir, port 18030, stopped afterwards).

### 1. Privacy statement card on the website settings screen
expected: Settings of a website show a "Privacy statement" card that names the state of the existing privacy page (here: draft, unfilled template).
result: pass
evidence: card text read from `/admin/websites/1`: "Your privacy page is a draft and is left exactly as it is. That page still carries the notice that it is an unfilled template."

### 2. Create the current template as a new draft
expected: The button creates a new draft page and opens its editor with a flash message.
result: pass
evidence: redirect to `/admin/websites/1/pages/4/edit`, flash "Privacy template created as a draft", status Draft.

### 3. Existing privacy page stays untouched
expected: The page list shows the old page unchanged next to the new one.
result: pass
evidence: list holds "Datenschutzerklärung /datenschutz Draft" and "Datenschutzerklärung (neue Vorlage)"; card still shown afterwards.

### 4. Template text is marked as unfinished
expected: The notice that the text is an unfilled template is the first thing in the page, with fill-in markers.
result: pass
evidence: editor shows "Diese Seite ist eine Vorlage und noch nicht ausgefüllt. … (eintragen)" as the first paragraph.

### 5. Form posts the same way with and without script
expected: Form action is `/admin/websites/1/datenschutz-vorlage`, carries the CSRF token, has no `hx-post`.
result: pass
evidence: `action` read from the DOM, `hx-post` null, one `gorilla.csrf.Token` input.

### 6. journald retention on a real server
expected: `systemd-analyze cat-config systemd/journald.conf` names `MaxRetentionSec=14d` and `MaxFileSec=1d` after installing the drop-in.
result: blocked
blocked_by: server
reason: the dev machine has no systemd; deferred on 2026-10-31 (see 30-HUMAN-UAT.md)

## Summary

total: 6
passed: 5
issues: 0
pending: 0
skipped: 0
blocked: 1

## Gaps

[none]
