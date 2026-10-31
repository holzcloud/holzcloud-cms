# Verification: Phase 13 — The public side speaks the visitor's language

Measured 2026-09-13 against `3d22e6a`, on a freshly built binary and a throwaway
database, by walking the running application rather than by reading it.

**417 Go files, 118 346 lines, 1332 test functions, 1824 catalogue entries in
each of de, es, fr and it.** The eight shipped themes carry four catalogues
each, 100 keys apiece, generated from one source of 138.

---

## PUB-01 — a visitor is refused in the language of the page

**Met, and the gate that missed it was widened.**

The regression was real and is measurable in the other direction now: a German
visitor who mistypes an address is told *"Bitte gib eine E-Mail-Adresse an,
damit wir antworten können."* and a French one *"Merci d'indiquer une adresse
e-mail pour que nous puissions répondre."*

Two mechanisms carry it, and they are different questions that were being asked
the wrong way round:

- `internal/public/locale.go` puts the **page's** language into the i18n
  context, so `sdk.T` called from a plugin during a public request answers in
  it. On an admin request the same call answers in the operator's language.
  Getting these two round the wrong way is how v2.0 came to answer German
  visitors in English.
- `plugins/kontaktformular` no longer builds a sentence. It produces a
  `refusal{Field, Code, Arg}`, and the sentence is made where the form is
  drawn — which is the only place that knows who is reading.

`tools/english` now reads `plugins/` and `sdk/` and fails the build on a
sentence-shaped literal that is not an argument of `T`, `Tf`, `Log` or `Logf`.
That is the half of the gate that was missing: its exception list named
`internal/` files and no plugin, so a visitor's text in `plugins/` was invisible
to exactly the gate built to see it.

`tools/i18n` gained `literalText`, which folds an all-literal `+` join. It found
**twelve** sentences that had been invisible to the collector, two of them a
year old in `internal/bundle`.

## PUB-02 — a theme carries its own catalogue

**Met.** `internal/template/lang.go` loads `lang/<tag>.json` from the theme being
rendered. The public FuncMap has `t`, `th` and `tf`, and they resolve against
that catalogue and against nothing else — never against this program's, which is
the point: a theme's vocabulary is the theme author's, and a `t` that reached
into the admin's catalogue would translate a word the author never wrote.

A key with no translation is **reported** rather than swallowed:
`CheckCatalogs` names it and `holzcloud template check` prints it. It refuses
nothing — a theme with a missing word still installs and still renders, showing
the key.

## PUB-03 — the operator says what their site calls things

**Met.** `internal/wording` plus migration `00055_theme_wording.sql`, per website
and per language, edited at `/admin/websites/<id>/wording`. The order is the
operator's word, then the theme's, then the key itself.

Driven in the browser: a website whose theme says "Weiterlesen" and whose
operator typed "Mehr davon" renders "Mehr davon", and the same page in French
renders the French catalogue's word until the operator overrides that too.

## PUB-04 — the eight themes, and they stay in step

**Met.** Measured before the work: **150 distinct strings in 955 places**, nearly
every one in all eight themes. They are generated from `tools/themewords/words.json`
by `go run ./tools/themewords`, and `-check` is a CI step:

```
every shipped theme's catalogue matches tools/themewords/words.json
```

Eight themes × four catalogues × 100 keys. Shipped in de, fr, it and es; the
source language is the key itself.

## PUB-05 — TEMPLATE-SPEC §2.5 is rewritten

**Met.** §2.5 said three days earlier that a theme is single-language and that
`t` is deliberately absent. It now says the opposite, and it quotes the old
argument and answers it rather than deleting it: a specification that quietly
changes its mind teaches a theme author nothing. §2.4 gained `.json` to the
allowed file types.

## PUB-06 — a missing translation is countable

**Met.** `holzcloud template check` names the keys a theme mints and does not
translate. `MintedByTheProgram` is the short list of keys the program itself
puts into a theme's hands (the not-found and maintenance pages), and it counts
only in the spare-key calculation — folding it into `KeysUsed` made a theme that
mints nothing demand a catalogue, which was wrong and was caught by its own test.

The upload path refuses nothing new: CSP, zip-slip and `template.Check` are
untouched.

## QUAL-01 — the gates

```
no German in the Go source outside the catalogues
1824 strings in the source
de.json      1824 translated, 0 open, 0 orphaned
es.json      1824 translated, 0 open, 0 orphaned
fr.json      1824 translated, 0 open, 0 orphaned
it.json      1824 translated, 0 open, 0 orphaned
every shipped theme's catalogue matches tools/themewords/words.json
```

## What this phase found that it did not plan for

- **Two gates that disagree productively.** `tools/english` accepts a sentence
  marked with `N` (it is collectable); `tools/i18n` then counts whether it is
  actually translated. Neither alone is the rule.
- **A refactor can orphan a catalogue entry.** Moving two sentences out of
  `web.Titlef` and into `fmt.Errorf` made them invisible to the collector while
  leaving their translations behind as orphans. Fixed with `media.Refusal` codes
  and a translator per reader — the same shape as the form's refusals, arrived
  at independently on the same day.
