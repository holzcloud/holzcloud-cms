# Milestones

## v1.10 Inhaltsmodell und Zugang (Shipped: 2026-09-11)

*Planned and worked as v1.6, closed on 2026-09-10, renumbered v1.10 when it was released
so that milestone and release carry one number (decided 2026-09-11).*

**Delivered:** A website describes its content model completely — every field kind
an author needs, in every carrier that holds fields, content that can arrive as a
table, and albums that live once and appear on many pages — and whoever enters the
admin may arrive through the sign-in the operator already runs.

**Phases completed:** 6 phases (6–11), 42 plans, 95 tasks
**Timeline:** 2026-09-03 → 2026-09-10 (8 days), `30f2064` → close, 449 commits
**Code:** 472 files changed, +119 622 / −9 700 lines since the `v1.5` release tag;
110 399 lines of Go in 394 files at close
**Audit:** `tech_debt`, 46/48 requirements satisfied — `milestones/v1.10-MILESTONE-AUDIT.md`
(first verdict `gaps_found`; the one broken cross-phase seam was closed before this entry)
**Closeout:** override_closeout — Known verification overrides: 5 newly acknowledged,
0 carried forward from a prior close (see STATE.md Deferred Items)

**Key accomplishments:**

1. **Housekeeping that makes the rest measurable (Phase 6).** The i18n catalogue
   format is locked by a test, CI rebuilds the six wasm guests and four plugin
   archives byte-reproducibly and compares them (`go run ./tools/wasm -check`), and
   the tests that used to skip themselves fail on a runner.
2. **The field palette (Phase 7).** Choice as a button row with an explicit empty
   choice, a real multiple choice with a server-side maximum, a term field that
   stores the slug and prints the current name, and `zeit`, `bereich` and `code` —
   with one multi-value encoding (`field.SplitValues`/`JoinValues`) shared by form,
   renderer, bundle and CSV importer.
3. **Snippets carry fields (Phase 8).** A text snippet holds any field kind through
   the existing field table and the same sanitisation pipeline as a page, and
   themes read them as `.Site.Bausteinfelder`.
4. **CSV import (Phase 9).** Upload, map column to field, dry run, create pages the
   ordinary way into a new or existing website, and a report for every row.
5. **Single sign-on through Authentik forward-auth (Phase 10).** Peer address checked
   before any header is read, identity headers stripped unconditionally, a
   constant-time shared secret; roles and websites re-derived from groups on every
   request of an SSO session; an identity bound to an account by username, never by
   e-mail address; the password path unchanged beside it.
6. **Gallery (Phase 11).** A lightbox with no JavaScript (`:target`), previous/next,
   albums assembled once per website and placed on many pages, a scroll-snap
   slideshow, and an album reference that survives the bundle round trip by name.

At the close, the milestone audit found that own block kinds never checked their
field kinds — a stored "nein" rendered as yes — and that the feed's dates ignored
album changes. Both were proven red and fixed (`bda9151`, `9ab7beb`).

### Known Gaps

- **GAL-07** — met in substance (one mechanism for the picture list), not in its
  wording, which names FIELD-07's pair. Handed to Phase 12 (v2.0).
- **QUAL-01** — the gate reads 1328 strings, 0 offen, 0 verwaist, but sentences
  built with `fmt.Sprintf` or concatenation reach the screen past it
  (`WINDOWS.md` 6, 18, 29), plus the 828 uncollectable strings measured in
  `.planning/audits/v1.6-I18N-828.md`. Handed to Phase 12 criterion 9 (v2.0).
- Verification overrides acknowledged: 07 (criterion 1, second sentence — a named
  limit), 10 (criterion 6 → Phase 12), 11 (criterion 6 → Phase 12).
- `WINDOWS.md`: 22 open of 29 at close.

### Tag

`v1.10` — the release built from this milestone (2026-09-11). The tag `v1.6` already
named the published release "Fassung 1.6" (2026-09-04, `ad6a793`) and releases had
reached `v1.9`, so the milestone took the next free release number rather than moving a
published tag. From here a milestone close creates its release tag.

---
