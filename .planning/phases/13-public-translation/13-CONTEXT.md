# Phase 13 — The public side speaks the visitor's language

Opened 2026-09-13. Measured against `5c1e6bf` before any change.

## 1. What was measured

**The eight shipped themes, 104 `.html` files.** Translation calls in them:
zero — and not by accident of the collector but of the program. `funcMap`
(`internal/template/loader.go:574`) supplies `safeHTML`, `formatDate`,
`formatDateShort`, `formatDateISO`, `formatWeekday`, `menu`, `menuFor`, and no
`t`. Counted on 2026-09-13 over text nodes and the translatable attributes
(`aria-label`, `placeholder`, `title`, `alt`, `value`):

| | |
|---|---|
| distinct strings | **150** |
| occurrences | **955** |
| appearing in all eight themes | nearly all of them |

The most frequent are `Kategorien` (25), `Zwischensumme` (24), `Versand` (24),
`kostenlos` (24), `Gesamt` (24), `Suchen` (21), `Schlagwörter` (21). The shop
vocabulary dominates: a website that sells is where the German shows most.

**That the eight mint the same 150 words is the finding that shapes the work.**
Eight independent catalogues would be eight copies drifting apart from their
first correction onward. They are generated from one shared source by a
committed tool instead, the same way `tools/i18n` is committed.

**`plugins/` is worse than untranslated, it is wrong.** `check()` in
`plugins/kontaktformular/main.go:549` answers a German visitor:

```go
case n.Name == "":        return "Bitte trage deinen Namen ein."
case n.Email == "":       return "Please enter an e-mail address so that we can answer."
case !plausibleAddress:   return "The e-mail address does not look right."
case len(n.Email) > max:  return "Die E-Mail-Adresse ist zu lang."
```

Six of eight English, two German, the labels beside them German.
`plugins/bestellung` counts 21 English against 4 German. Introduced by
`df4ff4c` during v2.0 — its message reasons about *"die Texte, die ein Betreiber
liest"*, and that is the error: these are a **visitor's** texts, and the v2.0
decision §3b put a visitor's German on the `germanVoice` exception list
precisely so it would not be translated. The list names `internal/` files. No
plugin is on it, so the gate that exists to catch this could not see it.

## 2. The decision, and what it overturns

**A theme carries its own catalogue, and the operator can override any key.**
Taken 2026-09-13 by the developer, out of three offered.

It overturns `TEMPLATE-SPEC.md` §2.5, written three days earlier, which states
that a theme is single-language and that `t` is *deliberately* absent. That
section's argument is good and must be answered rather than deleted:

> a theme is uploadable content, so its sentences cannot live in this program's
> catalogue — this program does not ship them. A `t` that worked for the eight
> themes we ship and silently returned the key for yours would be worse than
> none, because nothing would report it and a visitor would read the wrong
> language on a page every gate called green.

The answer is in two halves. The sentences do not live in this program's
catalogue — they live in **the theme's**, shipped inside the theme. And nothing
is silent: a key a theme mints without translating is named by
`holzcloud template check`, which is the same place a missing template field is
named today.

§2.5 also offers snippet fields as the multilingual route, and that stays true
and stays documented. It is the right answer for a theme written for one site.
It is the wrong answer for eight themes we ship to strangers, because it asks
every operator to type fifteen fields per language before the chrome is right.

## 3. Where it lands in the code

`funcMap(locale, timezone)` already takes the locale and already exists because
`formatDate` needed it — the comment above it says so. `cacheKey`
(`loader.go:636`) is `{websiteID, view, locale, timezone}`, so a per-website,
per-language override is already cacheable and `InvalidateTemplateCache`
already exists for when one changes. The shape of this change was anticipated by
the shape of the last one; there is no new plumbing, only a third argument.

## 4. Order of work

1. **PUB-01 first, alone, and small.** It is a live regression on `main` and it
   is the only item here a visitor is being harmed by today. It does not depend
   on anything else in this phase: `sdk.T` shipped in v2.0.
2. **The gate, in the same commit.** A fix without it invites the next sweep to
   do it again — that is the lesson v2.0's criterion 7 wrote down and this
   phase is the proof it was right to.
3. Loader, FuncMap and the theme catalogue format (PUB-02, PUB-06).
4. The operator override (PUB-03).
5. The eight themes and their generator (PUB-04), then §2.5 (PUB-05).

The themes come last on purpose: 955 replacements against a mechanism that is
still moving is the one way to have to do them twice.
