# Milestones

## v2.1 The Public Side Speaks the Visitor's Language (Shipped: 2026-09-13)

**Delivered:** A website published in French reads French — its chrome, its
forms and everything a plugin says to a visitor, not only its pages. And the
contact form, which was the plainest thing on the public side, became one worth
writing into: per-field refusals in all eight themes, a receipt, replies from
the admin, consent with its wording and its moment, a quarantine for what the
spam traps refuse, attachments, and a question that is asked only when it
applies — without a line of JavaScript.

**Phases completed:** 2 phases (13, 14), 10 plans
**Timeline:** 2026-09-13, one day, `0337fb1` → close, 11 commits
**Code:** 233 files changed, +12 869 / −1 456 lines since the `v2.0` release;
118 346 lines of Go in 417 files at close, 1332 test functions, 1824 catalogue
entries in each of de, es, fr and it
**Verification:** `milestones/v2.1-phases/13-public-translation/13-VERIFICATION.md` and
`milestones/v2.1-phases/14-contact-form/14-VERIFICATION.md` — 14 of 14 requirements met, the
eight themes driven in two languages against a running server
**Closeout:** clean — nothing carried forward, one thing deliberately open and
named (`plugin.json`'s name and description, inherited from v2.0)

**Key accomplishments:**

1. **The regression that opened the milestone is closed, and the gate that
   missed it was widened.** v2.0 translated two plugins' visitor-facing refusals
   into English while the public side had no channel to turn them back. The
   channel is the page's language reaching `sdk.T` through the request context;
   the plugin now produces a reason code and the sentence is made where the form
   is drawn, which is the only place that knows who is reading. `tools/english`
   reads `plugins/` and `sdk/` so the next one cannot be invisible.

2. **A theme carries its own words.** `lang/<tag>.json` beside the templates,
   `t`/`th`/`tf` resolving against it and against nothing else, and above it the
   operator's own wording per website and language. The eight shipped themes are
   generated from one vocabulary of 138 keys so they cannot drift; `-check` is a
   CI step. TEMPLATE-SPEC §2.5 now says the opposite of what it said three days
   earlier, with the old argument quoted and answered rather than deleted.

3. **The contact form got the six things it was missing** and each of them had
   to get past a rule that existed for a reason. The receipt had to answer
   `PermNotify`'s own "a plugin must never choose a recipient" — it does, with
   one copy to the address the message already carries, behind a second
   permission and the operator's switch. Attachments had to get past the 256 KB
   body bound — they do, because the host holds the files and the plugin only
   ever sees a name, a kind and a size, which is also what lets the spam traps
   run before anything reaches the disk.

4. **Three bugs that were nobody's feature, all found by building.** The largest
   had been there for every release: an answer to a form submission was built
   from the page's slug, so a form on the **start page** redirected to `/home`,
   which the public side redirects to `/` without its query. Nobody who used a
   form on a start page had ever seen a thank-you or a reason. The other two:
   the same redirect lost a second language's prefix, and the consent sentence
   was stored once per website, so a French visitor was asked in German.

5. **The browser pass earned its place in the gate.** All eight themes in two
   languages, and it is what found the consent hole — no test caught it, and one
   language of one page showed it at once.

**Open:** the release tag. `v2.1` and `v2.0` are both still missing from the
remote; this environment's credential writes branch refs and not tag refs (403),
and the remedy is a working copy with full access. See `STATE.md` →
*Operator Next Steps*.

---

## v2.0 The Codebase Speaks English (Shipped: 2026-09-13)

**Delivered:** A stranger can read this repository. Every comment, every
identifier, every test name, every SQL column and every catalogue key is
English — and so are the two contracts a machine reads: the template data
contract a theme author types, and the MCP surface an assistant calls. The
product still speaks German to the person using it; what changed is the language
of the source, and the catalogue turned round to serve it.

**Phases completed:** 1 phase (12), 10 waves
**Timeline:** 2026-09-11 → 2026-09-13, `e23ed79` → close, 32 commits
**Code:** 491 files changed, +18 650 / −14 770 lines since the `v1.10` release;
112 410 lines of Go in 402 files at close, 1279 test functions
**Verification:** `phases/12-codebase-speaks-english/12-VERIFICATION.md` — all
nine criteria met, browser pass driven, four findings fixed in the same pass
**Closeout:** clean — nothing carried forward, two things deliberately open and
named

**Key accomplishments:**

1. **Two breaking changes, announced, in one release.** The template data
   contract (`.Page.Felder` → `.Page.Fields` and six siblings) and the MCP tool
   surface (`seite_anlegen` → `create_page`, every argument and every answer key
   with it). Both break loudly: `holzcloud template check` names the English
   fields a theme should have used, and the server answers
   `there is no tool "seite_anlegen"` rather than doing something unexpected.
   All eight shipped themes came along.

2. **The catalogue turned round.** The English sentence is now the key and
   `de.json` is a translation like `fr.json`. `de-CH` derives from `de.json`
   rather than from source — it could no longer derive from source, because
   applying the ß rule to an English sentence yields something that is not
   German. Nothing changed for an operator: the admin still speaks German the
   moment the browser asks for it.

3. **Migration 00054.** 24 German column names renamed and 6 indexes rebuilt,
   through a new migration rather than by editing a released one. Its `Down`
   half is driven by a test, which is how three older rollback tests were found
   to break on it.

4. **Criterion 9, which is what makes the translation gate mean anything.** 828
   strings a person reads were in no catalogue at all — not untranslated,
   *unreported*, because the collector could not see a sentence built with
   `fmt.Sprintf` or joined with `+`. The shop's whole admin surface was among
   them. The catalogue grew from 1360 to 1610 entries in four languages.

5. **A plugin can translate.** The host grew an operation `translate` — the
   first that needs no permission — the SDK grew `T` and `Tf`, and `tools/i18n`
   grew `plugins/` as a third root. A plugin writes English and the operator
   reads their own language.

6. **A gate, not a convention.** `tools/english` parses rather than greps,
   because a grep with no locale confuses `Ü` with a typographic quotation mark
   and because a comment explaining how umlauts are transliterated has to be
   able to name them. It blocks in CI. Its four exception lists each carry their
   reason at their own site.

**What this milestone deliberately did not do:** the stored German vocabularies
stay German — they are in every database, they travel in every bundle, and the
block kinds are CSS classes in all eight themes. Every Go mention of one is now
a named English constant instead. And the public side still has no translation
channel at all, which is the open decision handed to whatever comes next.

### Known Gaps

- `WINDOWS.md`: 15 open of 31 at close, none of them this milestone's — every
  one is inherited from phase 08, 10 or 11. Phase 12 opened no window and
  closed three it was handed (3, 18, 29).
- Two things deliberately open, named in `12-VERIFICATION.md`: the public side
  has no translation channel at all, so a French-language website sells in
  German; and a plugin's `plugin.json` name and description are not translated.
- The stored German vocabularies stay German by decision, not by omission —
  criterion 6 asked for the decision and `12-CONTEXT.md` §3b carries it.

### Tag

`v2.0` — the release built from this milestone (2026-09-13), on `9fe3279`, the
first commit of `main` that carries the whole phase. **The tag is written but
not yet on the remote**: this milestone was closed from an execution
environment whose GitHub access writes branch refs and not tag refs, and the
push is refused with 403. `STATE.md` carries it under *Operator Next Steps*. It is the first close under the
rule the v1.10 entry states: from here a milestone close creates its release
tag, and milestone and release carry one number. The major number is not a
courtesy — two public contracts break in this release, the template data
contract and the MCP tool surface.

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
