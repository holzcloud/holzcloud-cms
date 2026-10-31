# Verification: Phase 12 — The Codebase Speaks English

Measured 2026-09-12/13 against `b43c72b`, on a freshly built binary and a
throwaway database, by walking the running application rather than by reading.

**402 Go files, 112 410 lines, 1279 test functions, 1610 catalogue entries in
each of de, es, fr and it.**

---

## Criterion 1 — no German in Go, and the reasons survive

**Met, and the criterion's own wording had to be corrected to state it.**

The criterion says `grep -rn '[äöüÄÖÜß]' --include='*.go' .` prints nothing. That
gate cannot be written as stated and this phase found out why, twice:

- **It miscounts.** With no locale set, grep matches bytes, and the UTF-8
  encoding of `Ü` (C3 9C) shares its second byte with the typographic quotation
  mark `“` (E2 80 9C). Measured in this repository's own container: three hits
  reported in a file that had two.
- **It forbids a sentence this repository needs.** `internal/field/field.go`
  explains that `SlugifyKey` used to drop every accented letter but ä, ö, ü and
  ß. That sentence cannot be written under a literal reading of the rule.

So the gate is a program, `tools/english`, that parses rather than scans — it
knows a comment from a string literal from an identifier, which a line does not.
It reports:

```
no German in the Go source outside the catalogues
```

Its exceptions are four named lists, each carrying its reason at its own site:

| List | What it holds | Why |
|---|---|---|
| the catalogue files | de/es/fr/it.json | dictionaries; German is their content |
| `fixtures` | `internal/template/sample.go` | the page an uploaded template is rendered against, printed verbatim to a theme author by TEMPLATE-SPEC |
| `germanVoice` | 17 files | the sentences a **customer** reads — checkout, cart, order e-mails, VAT wording, month names — plus the language names and the starter pages. §3b and §3d. |
| `aboutGerman` | 10 files | files whose **comments** are about German and cannot be written without naming it |

Plus `//nolint:german` with a reason, on 52 individual lines across 30 files.
Almost all of them are an English comment QUOTING a German word the sentence is
about — "Möbel" next to "Moebel" on the term screen, `Müller@example.com`
against `müller@example.com` in the forward-auth note, the broken CSV report the
test exists to have fixed.

**The reasons survived.** The sweep was done block by block, translating the
argument rather than the words. Spot-checked at the longest blocks —
`internal/bundle/format.go`'s snippet contract, `internal/album/store.go`'s
transaction note, `internal/csvimport/mapping.go`'s NFC/NFD fold: each is the
same length and makes the same argument in English.

## Criterion 2 — identifiers, one word each

**Met.** `go run ./tools/rename -inventory -root .` reports 97 distinct
"German-looking" identifiers, and every one of them is a false positive of that
tool's heuristic: `archive`, `Archive`, `ArchiveURL`, `marker`, `marked`,
`markEnd`, `UpdateItem`, `HasArchive`, `AlbumMarker` and the like — English
words the heuristic cannot tell from German ones.

870 identifiers were renamed, in one `go/scanner` pass that rewrites only
`token.IDENT`. That is what kept catalogue keys, form field names and stored
values untouched — a `sed` over the same map would have rewritten all three.

The map was written by hand. An earlier attempt in this phase to generate it
from word stems produced `Archivee`, `Groupn` and `AlbumTokenr`; the roadmap
predicted exactly that, and it is why `.planning/GLOSSARY.md` exists.

## Criterion 3 — the template contract

**Met, and proven in the browser.**

All seven names were read from a running server through a theme written for the
purpose and uploaded through the admin:

```
<p id="snippetfield">07721 123456</p>      .Site.SnippetFields
<p id="kind">produkt</p>                    .Page.Kind
<p id="fieldcount">2</p>                    .Page.FieldList
<p id="langs">de fr </p>                    .Site.Languages
<p id="trans">de fr </p>                    .Page.Translations
<p id="snippetlist">footer-kontakt </p>     .Site.SnippetList
<p id="fields">Preis=1290.50; Sorten=Eiche, Buche; </p>
```

All eight shipped themes render the same page with both field values.

The break is loud. A theme carrying `.Page.Felder` is refused by
`holzcloud template check`, and the refusal counts the English fields out:

```
can't evaluate field Felder in type template.PageContent (at .Page.Felder.preis)
— PageContent has: Title, ContentHTML, Slug, PublishedAt, UpdatedAt, Excerpt,
HasOwnHeading, IsPost, Kind, Prev, Next, ArchiveURL, Terms, Fields, FieldList,
Translations
```

## Criterion 4 — the SQL columns

**Met.** Migration `00054` renames 24 columns and rebuilds 6 indexes. No
released migration was edited. Verified against the live database of the browser
pass, which was migrated from empty through all 54:

```
pages.content_kind exists: true
pages.art exists: false
```

and no German column name in any table.

The `Down` half is driven by a test rather than assumed —
`TestMigration00054DownAndUp` rolls it back and forward again. Three older
rollback tests broke on it and were given a shared helper, because goose applies
a rollback newest-first and 00054 renames columns their `Down` halves name in
German.

`pages.art` → `pages.content_kind` and not `kind`: `kind` is the word for the
block kind, the field kind and the content kind alike, and a column called
`kind` on `pages` would be the fourth meaning. The GLOSSARY carries the
exception.

## Criterion 5 — the catalogue's source language

**Met.** `i18n.Source` is `"en"`. `de.json` is a translation like the others,
`de-CH.json` derives from `de.json` (it could no longer derive from the source:
applying the ß rule to an English sentence yields something that is not German
at all), and es/fr/it were re-keyed through the old mapping.

The collisions were resolved one by one. The three that collided because the
existing translation was wrong were fixed rather than merged.

Proven in the browser: the same screen in five languages.

```
de     Seiten – Schreinerei Muster — Holzcloud
fr     Pages – Schreinerei Muster — Holzcloud
es     Páginas – Schreinerei Muster — Holzcloud
it     Pagine – Schreinerei Muster — Holzcloud
de-CH  Seiten – Schreinerei Muster — Holzcloud
```

## Criterion 6 — the stored German vocabularies

**Met.** §2 of `12-CONTEXT.md` decides it: they stay German as stored values,
and every Go mention of one is a named English constant. Three independent
reasons, any one sufficient — they are in every database, they travel in every
bundle, and the block kinds are CSS classes in all eight themes.

The binding half is done. The last two loose literals were found by grep at the
close of the phase and bound: the four switch names in
`internal/admin/page_fields.go` (the second half of a class name in `admin.css`)
and the four reserved words in `internal/kind`.

`tools/english` enforces the rule from the other side: German inside a
declaration it knows by name is allowed and nowhere else is.

## Criterion 7 — German cannot come back unnoticed

**Met.** `tools/english` runs in `ci.yml` as a **blocking** step, deliberately
unlike the catalogue step below it: a German comment is not a translation in
progress, it is a regression.

Its second half — criterion 9's — is the `tr`/`trs` entry in `tools/i18n`'s
`goFuncs` and the `plugins/` root, both added this phase.

## Criterion 8 — the standing gates

**Met.**

`go run ./tools/i18n`: `1610 translated, 0 open, 0 orphaned` on de, es, fr and
it; de-CH and the two other regional editions carry only their deviations.

The browser pass drove: account creation, sign-in, the second factor, a website,
two fields, a page with field values, a snippet with a field, an own content
kind, a domain, the public page under all eight themes, the 404, the sitemap,
the feed, a bundle out and back in, the whole MCP surface, and a plugin screen
in four languages. 45 admin addresses, every one 200 with an English title.

Four findings, all fixed in the same pass — see the CHANGELOG's *Behoben*.

## Criterion 9 — every operator-facing string is collectable

**Met, and the measurement it rests on was wrong in a useful way.**

The audit counted 828, of which 274 were `plugins/`. Walking them showed most of
that 274 to be a **visitor's** text — the contact form, the order form, the farm
shop — which §3b covers and this milestone does not touch. What an operator
reads in `plugins/` is two screens.

So §3a landed in full rather than as a carry-over: a host operation `translate`
(the first that needs no permission), `T` and `Tf` in the SDK, and `plugins/` as
`tools/i18n`'s third root. Proven in the browser, four languages:

```
en  Write <code>[[jahr]]    Replaced so far
de  Schreibe <code>[[jahr]] Bisher ersetzt
fr  Écris <code>[[jahr]]    Remplacé jusqu
it  Scrivi <code>[[jahr]]   Sostituito finora
```

The 425 inside the roots were the real work: ~140 mechanical sentences plus the
shop's admin screens. Every one now goes through `web.T`, `web.Titlef`,
`SetFlash*`, `i18n.N` or `{{t}}`. The catalogue grew from 1360 to 1610.

**One lesson worth carrying.** A format string assembled with `+` across lines
is invisible to the collector even inside `i18n.N(...)` — it reads a string, not
an expression. Two sentences were caught by the gate reporting them as
*orphaned* after being added; they are now single literals with a comment
saying why.

---

## What is NOT met

**Nothing in the nine criteria.** Two things are deliberately outside them and
are written down rather than left silent:

1. **The public side has no translation channel at all.** A French-language
   website sells in German. §3b decided this is a feature and not a cleanup —
   it needs the theme and the handler decided together — and §3d made the
   decision enforceable as `germanVoice` so that the next reader meets it in the
   gate rather than by accident.
2. **`plugin.json`'s own name and description are not translated.** They are the
   plugin's identity in a list, written once by whoever built it, and a
   third-party plugin cannot be expected to have entries in this installation's
   catalogue. Visible in the browser pass as the one German sentence left on
   `/admin/plugins`.
