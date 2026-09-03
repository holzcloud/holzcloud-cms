# Roadmap: Holzcloud CMS

## Milestones

- ✅ **v1.0 — Core CMS** — Phases 1–5 (shipped 2026-04-14)
- 🚧 **v1.5 — Inhaltsmodell** — Phases 6–8 (in progress)

Phase numbering continues from v1.0. It never restarts. The five v1.0 phase
directories are archived under `.planning/milestones/v1.0-phases/`.

---

## Phases

**Phase Numbering:**
- Integer phases (6, 7, 8): planned milestone work
- Decimal phases (6.1, 6.2): urgent insertions, executed between their integers

<details>
<summary>✅ v1.0 — Core CMS (Phases 1–5) — SHIPPED 2026-04-14</summary>

Requirements FND, AUTH, SITE, PAGE, PUB, TPL, MENU, MEDIA, UI, USR and DEP —
all mapped, all complete, 13 plans. Goals, success criteria and plan records are
archived with the phase directories under `.planning/milestones/v1.0-phases/`.

- [x] **Phase 1: Foundation** — Binary skeleton, SQLite dual-pool + WAL, goose migrations, embed.FS, structured logging, graceful shutdown
- [x] **Phase 2: Auth + Admin Shell** — Login/logout, Argon2id, SCS sessions, CSRF + htmx wiring, admin layout and design tokens
- [x] **Phase 3: Multi-Site + Pages + Public Rendering** — Website/domain CRUD, Host-based routing, page authoring, slug routing, public template engine
- [x] **Phase 4: Templates + Menus + Media** — Template zip upload, activation, hierarchical menu builder, media upload and serving
- [x] **Phase 5: Admin Polish + Users + Deployment** — htmx progressive enhancement, view transitions, flash messages, dashboard, user management, systemd + Caddy configs

</details>

### 🚧 v1.5 — Inhaltsmodell (In Progress)

**Milestone Goal:** A website describes its content model completely — every
field kind an author needs, in every carrier that holds fields, and content can
also arrive as a table.

- [ ] **Phase 6: Field Kinds** — The palette an author reaches for: choice as a button row, a genuine multiple choice, terms as a field, plus `zeit`, `bereich` and `code`
- [ ] **Phase 7: Snippets Carry Fields** — A text snippet stops being one Markdown box and holds any field kind, reusing the field table that already exists
- [ ] **Phase 8: CSV Import** — Content arrives as a table: upload, map column to field, create pages the ordinary way, report every row

---

## Standing Gates (QUAL-01, QUAL-02)

Two of this milestone's fourteen requirements are not deliverables. They are
gates that apply to **every** phase that adds a user-visible string or a field
kind — which is all three.

- **QUAL-01** — every new string in all five languages; `go run ./tools/i18n`
  reports `0 offen, 0 verwaist`.
- **QUAL-02** — every new field kind exercised in the running application in a
  browser, not only in tests. Per `docs/offene-punkte.md`: the defects this
  project actually shipped were found by a browser pass, never by the suite.

**Decision on how they are mapped.** Mapping a recurring gate to the first phase
would tell Phases 7 and 8 that the gate is already satisfied — the exact failure
mode to avoid. So they are handled two ways at once:

1. Each of Phases 6, 7 and 8 carries the gate **verbatim as its own final
   success criterion**, so no phase can be called done while its strings are
   untranslated or its kinds unclicked.
2. For traceability's exactly-one-phase rule, QUAL-01 and QUAL-02 are formally
   assigned to **Phase 8**, the last phase — where they close milestone-wide and
   can no longer be deferred. Assigning them to the last rather than the first
   phase means they are never prematurely marked satisfied.

The Traceability table in REQUIREMENTS.md records this as `Phase 8 (gate on
6, 7, 8)`.

---

## Phase Details

### Phase 6: Field Kinds
**Goal**: A website's content model reaches every field kind this milestone names — the choice control an author actually wants, a value that can hold more than one string, tags as a field, and the three small kinds still missing — and each one survives save, reload and public render.
**Depends on**: Phase 5 (v1.0 complete; `internal/field` and `internal/term` in place)
**Requirements**: FIELD-01, FIELD-02, FIELD-03, FIELD-04, FIELD-05, FIELD-06
**Success Criteria** (what must be TRUE):
  1. A Choice field configured as a button row renders as a row of buttons in the page editor instead of a dropdown; picking one and saving keeps the picked value, and the control still works with JavaScript switched off.
  2. A Multiple-Choice field accepts several values in one save; after reload the same values are still selected, and a theme can loop over them and print each one — the first field value that is not a single string round-trips through storage intact.
  3. A Term field offers the tags the website already carries as a chooser rather than a free-text box, and the public page prints the term's **name**; renaming the term afterwards changes what the page shows, without touching the page.
  4. `zeit` accepts a time of day, `bereich` a number between its configured bounds entered as a slider, and `code` plain fixed-width text that never passes through Markdown — HTML typed into a Code field appears verbatim on the public page and does not execute.
  5. Gate for this phase: `go run ./tools/i18n` reports `0 offen, 0 verwaist`, and each new kind has been created, filled, saved and viewed in the running application in a browser.
**Plans**: TBD
**UI hint**: yes
**Planning notes**:
- FIELD-02 is the structural one; everything else here is small. `field.Data` needs either a second storage path or an encoding it commits to. `docs/offene-punkte.md` proposes one line per value inside the same string, the way `SplitChoices` already reads the options. Decide this **first** — Phase 8's importer has to write the same encoding.
- FIELD-03 has a complete template to copy: the reference chooser (`refPages` in `internal/admin/page_fields.go`) plus a `TermLookup` beside `Links.Page` in `internal/field/render.go`.
- FIELD-04/05/06 are an hour each, all inside `internal/field` + `templates/admin/field_input.html`.
- **No migration is needed for the kinds themselves.** `page_field_defs.art` in `00029` is `TEXT NOT NULL` with no CHECK constraint, so a new kind is a Go-side change only.
- No JavaScript beyond htmx: the button row is radio inputs, the multiple choice is checkboxes or a `<select multiple>`, the slider is `<input type=range>`. All must submit as a plain form; htmx is enhancement only.

### Phase 7: Snippets Carry Fields
**Goal**: A text snippet stops being one Markdown box and becomes a small content model of its own — any field kind, defined in the field table that already exists, rendered through the pipeline and the sanitisation that already exist.
**Depends on**: Phase 6 (a snippet field is worth little until the palette is complete, and the multi-value encoding must be settled before snippets store one)
**Requirements**: SNIP-01, SNIP-02, SNIP-03
**Success Criteria** (what must be TRUE):
  1. An admin can give a snippet fields of any kind — a phone number with a check on it, an image from the website's library, a number, any kind Phase 6 added — fill them on the snippet screen, and see the same values after reload.
  2. Snippets that already exist keep working untouched: their Markdown body still renders wherever a theme calls them, with no migration step the admin has to perform and no snippet key changing.
  3. The schema shows **one** field-definition table, not a third: a snippet's fields live in `page_field_defs` with a `snippet_id` beside `website_id`, and the same field key can exist once on a page and once on a snippet of the same website without colliding.
  4. A theme reads a snippet's field values the same way it reads a page's, and a `<script>` put into a snippet's long-text field is sanitised away on the public page exactly as it is on a page — same goldmark → bluemonday chain, no second one.
  5. Gate for this phase: `go run ./tools/i18n` reports `0 offen, 0 verwaist`, and a snippet carrying at least one field of each kind has been filled and viewed on a public page in a browser.
**Plans**: TBD
**UI hint**: yes
**Planning notes**:
- The honest route named in `docs/offene-punkte.md`: reuse `page_field_defs` with a `snippet_id` column — explicitly **not** a third field table. Roughly two days, half of it screen work.
- Migration warning: adding the column is an `ALTER TABLE ADD COLUMN` (cheap, the shape `00037` used). The cost sits in the two partial unique indexes from `00029` — `idx_page_field_defs_kennung_oben` on `(website_id, kennung) WHERE parent_id IS NULL` would make a snippet field collide with a page field of the same key. Swapping an index is two lines; loosening a table-level CHECK would have been a full rebuild. Read `00029` and `00031` before writing migration `00045`.
- `internal/snippet` currently stores exactly `content_markdown` + `content_html`. The body stays; fields are added beside it.

### Phase 8: CSV Import
**Goal**: Content can arrive as a table — an admin uploads a CSV beside the existing WXR and bundle importers, says which column is which field, and gets pages created through the ordinary path plus an honest per-row account of what happened.
**Depends on**: Phase 6 only, and only for the multi-value encoding decided there — a column mapped to a Multiple-Choice field has to write what `internal/field` reads. Independent of Phase 7; it may be pulled forward if Phase 6's encoding is settled.
**Requirements**: IMP-01, IMP-02, IMP-03, QUAL-01, QUAL-02
**Success Criteria** (what must be TRUE):
  1. Uploading a `.csv` on the website screen leads to a mapping screen that lists the file's columns with a sample row, and each column can be pointed at Title, Body, or any custom field of that website — or explicitly left out.
  2. Confirming the mapping creates pages indistinguishable from hand-made ones — slug generated and unique per website, Markdown rendered and sanitised, draft/published honoured — because the importer calls `page.CreatePage` rather than its own INSERT.
  3. The import ends on a report naming every row: created (with its slug) or skipped (with the reason — missing title, duplicate slug, a value the field cannot hold). A file mixing good and bad rows imports the good ones, skips the rest, and leaves nothing half-written.
  4. Milestone close-out (QUAL-01, QUAL-02): `go run ./tools/i18n` reports `0 offen, 0 verwaist` across everything v1.5 added, and every field kind from Phase 6, the snippet fields from Phase 7 and this importer have each been driven once through the running application in a browser.
**Plans**: TBD
**UI hint**: yes
**Planning notes**:
- `internal/csv` beside `internal/wxr`, hung on the same screen (`templates/admin/website_list.html`). About a day: the column → field mapping is the whole job, everything after it is `page.CreatePage`.
- The mapping screen is the only real UI work; htmx for the preview swap, full-page fallback mandatory.
- The upload is user data: size cap, row cap and a check that the file is text before it is parsed, same posture as the template and media uploads.

---

## Progress

**Execution Order:** 6 → 7 → 8

| Phase | Milestone | Plans Complete | Status | Completed |
|-------|-----------|----------------|--------|-----------|
| 1. Foundation | v1.0 | 2/2 | Complete | 2026-04-14 |
| 2. Auth + Admin Shell | v1.0 | 2/2 | Complete | 2026-04-14 |
| 3. Multi-Site + Pages + Public Rendering | v1.0 | 3/3 | Complete | 2026-04-14 |
| 4. Templates + Menus + Media | v1.0 | 3/3 | Complete | 2026-04-14 |
| 5. Admin Polish + Users + Deployment | v1.0 | 3/3 | Complete | 2026-04-14 |
| 6. Field Kinds | v1.5 | 0/TBD | Not started | - |
| 7. Snippets Carry Fields | v1.5 | 0/TBD | Not started | - |
| 8. CSV Import | v1.5 | 0/TBD | Not started | - |

---

## Coverage

All 14 v1.5 requirements are mapped to exactly one phase.

| Phase | Requirements | Count |
|-------|--------------|-------|
| 6. Field Kinds | FIELD-01, FIELD-02, FIELD-03, FIELD-04, FIELD-05, FIELD-06 | 6 |
| 7. Snippets Carry Fields | SNIP-01, SNIP-02, SNIP-03 | 3 |
| 8. CSV Import | IMP-01, IMP-02, IMP-03, QUAL-01, QUAL-02 | 5 |

**Mapped: 14 / 14. Orphans: 0. Duplicates: 0.**

QUAL-01 and QUAL-02 are counted once, in Phase 8, and additionally enforced as
the final success criterion of Phases 6 and 7 — see *Standing Gates* above.

---
*Roadmap for v1.5 created 2026-09-03. v1.0 section retained for phase-number continuity.*
