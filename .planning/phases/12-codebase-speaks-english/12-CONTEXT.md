# Phase 12 — The Codebase Speaks English

Opened 2026-09-11. Milestone **v2.0**. This document is the phase's measurement
and its two standing decisions. Written in English, as every planning artifact
from this phase on; `.planning/`'s older documents stay German by deliberate
rule (they are the project's record, and rewriting a record's language is how it
stops being one).

---

## 1. Measurement, taken 2026-09-11 against `ef4873a`

The roadmap's numbers are from 2026-09-06 and the tree has grown by ~26 000
lines since. Re-measured, because the roadmap says the phase's first task is to
measure rather than assume.

| | 2026-09-06 (roadmap) | 2026-09-11 (now) |
|---|---:|---:|
| Go files | 325 | **395** |
| Lines of Go | 84 753 | **110 468** |
| Files carrying umlauts (outside catalogues) | 191 | **215** |
| Umlaut lines | 2 797 | **2 922** |
| Catalogue keys | 1 158 | **1 328** |
| Colliding English values | 9 | **10** |
| Test functions | 868 | **1 271** |
| …of them plainly German | 37 | **91** |

### Baseline health

Measured before any change: `go build ./...` clean, `go vet ./...` clean, the
whole test suite green, `go run ./tools/i18n` reports 1328 strings and
`0 offen, 0 verwaist` for en/es/fr/it. Everything this phase breaks, it broke.

### German identifiers, counted

`Seite` 634, `Feld` 579, `Album` 279, `Baustein` 206, `Zeile` 206, `Kennung`
153, `Galerie` 19.

### The template contract is exactly seven names

Measured by walking every exported struct field in `internal/template/loader.go`
— 117 in all, of which **seven** are German:

| German | English | Glossary |
|---|---|---|
| `.Page.Felder` | `.Page.Fields` | listed |
| `.Page.Feldliste` | `.Page.FieldList` | listed |
| `.Page.Art` | `.Page.Kind` | listed |
| `.Page.Uebersetzungen` | `.Page.Translations` | listed |
| `.Site.Bausteinfelder` | `.Site.SnippetFields` | listed |
| `.Site.Bausteinliste` | `.Site.SnippetList` | listed |
| `.Site.Sprachen` | `.Site.Languages` | **added here** |

All eight shipped themes read the contract; all eight come with it.

### The ten catalogue collisions

Nine were known; `Import finished` ← {`Einlesen abgeschlossen`,
`Import abgeschlossen`} is new since the glossary was written and is added to
it. Three are pre-existing translation *defects* and are repaired, not merged:
`Label` ← {`Beschriftung`, `Schlagwort`}, `Time` ← {`Uhrzeit`, `Zeitpunkt`},
`.` ← {`an.`, `fest.`}.

### German SQL columns

`kennung` 26, `art` 14, `auswahl` 7, `pflicht` 6, `hinweis` 5, `gilt_fuer` 5,
`beschriftung` 4, `bedingung` 3, `modus` 2, `min_wert` 2, `max_wert` 2,
`max_werte` 2, `kollision` 2, `erstellt_am` 2, `darstellung` 2, `daten` 1,
`dateiname` 1. Migrations run to `00053`; new ones start at `00054`.

---

## 2. Decision on LANG-08 — the stored German vocabularies

**They stay German as stored values, and every Go mention of one becomes a named
English constant.**

The question is criterion 6's: about 25 German strings in Go are *data* rather
than identifiers — every field kind (`text`, `langtext`, `mehrfachauswahl`,
`gruppe`, `schlagwort`, …), `gilt_fuer`'s three values, `knopfreihe`, and the
seven block kinds.

**Why they stay.** Three independent reasons, any one of which would be enough:

1. **They are in every database that exists.** Turning them is a data migration
   over rows this project does not own — every operator's content — and a
   migration that rewrites content is a different risk class from one that
   renames a column.
2. **They travel in every bundle ever exported.** A bundle is a file an operator
   keeps. Changing the vocabulary breaks every archive made before the change,
   or forces a compatibility shim that has to live forever. v1.10's own
   experience is the argument: the album reference was deliberately made to
   survive the bundle round trip *by name*.
3. **They are CSS class names in all eight themes** — `hc-aufruf` 20×,
   `hc-zitat` 13×, `hc-video` 13×, `hc-bildtext` 11×, `hc-karten` 7×,
   `hc-galerie` 6×, `hc-trenner` 4×. This milestone already spends its one
   deliberate theme break on the data contract (LANG-04). A second break in the
   same release, for a rename that no reader benefits from, buys nothing.

**What a stranger loses, and how it is paid back.** The objection to leaving
them is real: a reader meets `"langtext"` in `internal/field` and has to know
German to read it. That is paid back without touching a single stored byte —
**no German string literal stands loose in Go any more; each is bound once to an
English constant** (`KindLongText = "langtext"`), and the code reads
`KindLongText` everywhere. The German survives in exactly one place per value,
next to a comment saying it is a stored value and why it is not translated.
`.planning/GLOSSARY.md` already carries the rule this rests on, and the runtime
bug that taught it (`kollision IN ('uebergehen', 'aktualisieren')` compiles
cleanly when translated and is refused by SQLite at runtime).

This also gives criterion 7's gate something it can enforce: German inside a
`Kind*`/`Block*` constant declaration is allowed and nowhere else is.

## 3. Decision on the 403 strings outside the collector's reach

The reach audit (`.planning/audits/v1.6-I18N-REICHWEITE.md`) left two product
questions open. Both are answered here; the roadmap sanctions answering the
third bundle with "no, and here is why".

### 3a. `plugins/` — **yes, a channel, and it is the host's catalogue**

274 strings. The five WASM plugins in this repository are *ours*; they ship in
the binary and an operator cannot tell them from the admin proper. A string an
operator reads should be reported by the gate whoever minted it.

**Decision:** `tools/i18n` grows `plugins/` as a third root, the SDK gets the
marker function `i18n.N`'s equivalent so a plugin can mark a string without
linking the host, and the host translates `AdminOut.Title`/`AdminOut.Flash` and
the `plugin.json` text on the way out. A third-party plugin that marks nothing
simply has nothing collected — the gate reports what exists, which is the whole
point of criterion 9.

**Sized, not hidden:** this is real work and it is scheduled as its own wave. If
it cannot land, it becomes a named carry-over rather than a silence.

### 3b. The public themes — **no, and here is why**

116 strings across 104 theme files in eight themes. `t`, `th` and `tf` are
absent from the public FuncMap by construction, and CLAUDE.md records this as a
standing exception and a known open decision.

**Decision: a theme is single-language, and this is now written down rather than
merely true.** The reasoning:

- A theme is **uploadable third-party content**. Its sentences cannot live in
  this program's catalogue, because this program does not ship them. Putting `t`
  in the public FuncMap without answering *where a stranger's theme's
  translations come from* would create a function that silently returns the key
  for every theme but the eight we ship — the worst of the shapes this audit
  exists to find, a gate that reads green while a visitor reads German.
- The content model already answers the multilingual case, and answered it in
  v1.10. A theme's chrome words belong in **snippets**, which carry fields
  (`.Site.SnippetFields`), are per-website and per-language, and are edited by
  the operator who knows what the page should say. "Warenkorb" is content, not
  chrome, on a site that sells in two languages.
- Deciding otherwise is a phase, not a fix: eight themes, `TEMPLATE-SPEC.md`,
  and a translation-provenance mechanism for archives.

**What this obliges:** `TEMPLATE-SPEC.md` gains a section saying a theme is
single-language and how a multilingual site is built instead, so a theme author
(often an AI agent, which is why the spec is written literally) is not left to
infer it. Without that section the decision is not a decision, only a silence
with a paragraph about it.

---

## 3c. Decision taken 2026-09-12: the MCP surface — **English, and it breaks**

Not foreseen when this phase was measured, and found only because
`tools/english` flags string literals as well as comments. The whole `/ai`
surface was German: nine tool names (`seite_anlegen`), every argument
(`"titel"`, `"zustand"`), every answer key (`"geaendert"`), every enum value
(`"entwurf"`, `"veroeffentlicht"`), every description and every refusal.

It is translated, in full, and 2.0 says so as a breaking change.

**Why it is in scope.** It is not a stored value and it is not the catalogue.
Nothing persists a tool name: the tool list is built per request, and the tables
hold none of these words. What it is, is *vocabulary a machine reads* — and the
measurement that names this phase ("the codebase speaks English") is about
exactly that. A field kind stored as `mehrfachauswahl` is data an operator
created; `seite_anlegen` is a name this repository chose.

**Why it may break.** The break is loud, which is what makes it acceptable: an
assistant that names a German tool gets `there is no tool "seite_anlegen"` and
stops. Nothing is silently misread, and nothing halfway-renamed is left behind.
An assistant that asks for the tool list — the way MCP is meant to be used —
notices nothing at all. Carrying both spellings would mean two names for every
tool for years, and would put the German half of the codebase back on the wire
after this phase had taken it out of the source.

**What was deliberately left alone.** Everything an operator typed: field keys
(`preis`), content type keys (`produkt`), field kinds (`mehrfachauswahl`) and
block kinds (`zitat`). `list_fields` still reports those verbatim, because they
are rows, and §2's stored-value rule holds.

**One criterion 9 find in passing.** `ai.Store.Issue` refused a nameless key with
`errors.New("der Schlüssel braucht einen Namen")`, and `internal/admin/ai.go`
put that straight into a flash message. A German sentence an operator reads, in
a package the collector reaches, invisible to it because it was minted by
`errors.New`. It is now the sentinel `ai.ErrNameMissing` and one catalogue
sentence at the handler.

---

## 4. Order of work, and why it is this order

1. **The template contract** (LANG-04) — independent of everything, lands in its
   own commit with the eight themes, because it is the one thing a user notices.
2. **The SQL columns** (LANG-05) — before the comment sweep touches
   `internal/field/store.go`, or that file is rewritten twice.
3. **Make the strings collectable** (QUAL-01, criterion 9) — the 140 mechanical
   ones, then the 154 shop strings, then `plugins/`. This must precede the
   catalogue flip: the flip is a bijection through `en.json`, and a string with
   no key has nothing to invert.
4. **Identifiers, then comments** (LANG-02, LANG-03, then LANG-01) — rename in
   one commit and translate in the next, per file, so `git log --follow` keeps
   the lineage at any similarity threshold.
5. **The catalogue flip** (LANG-06) — last of the string work, once no German
   user-visible string is still moving.
6. **The gates** (LANG-07) — both halves: no German in Go outside the catalogue
   files and the named constants, and no operator-facing string minted in a
   shape the collector cannot see.
7. **Verification** (QUAL-02) — the browser pass, larger here than in any phase
   that adds screens, because LANG-04 renames a field every screen reads.
