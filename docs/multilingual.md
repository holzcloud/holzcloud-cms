# Multilingual

Two separate things share the word "language" here: the languages a **website**
appears in, and the language a **person** administers it in. They have nothing to
do with each other.

- [Websites in several languages](#websites-in-several-languages)
- [The language of the admin](#the-language-of-the-admin)
- [Adding a language without a rebuild](#adding-a-language-without-a-rebuild)
- [Regional variants](#regional-variants)

## Websites in several languages

In the website's settings you set the **main language** and, below it, **further
languages** — codes separated by commas, `fr, it` or `fr-CH`, at most eight.

The one decision everything else hangs on:

> **The main language keeps its addresses.** `/kontakt` stays `/kontakt` when
> somebody switches French on. Every further language lives under its code:
> `/fr/contact`.

No existing link breaks, no redirect becomes necessary, and a website with one
language notices nothing of the whole business — no prefix, no language picker,
no extra field in the form.

**Creating a version.** The page form has a section *Versions in other
languages* showing which languages this page already exists in and which are
missing. *Create version* copies the page as a **draft** into the other language —
with text, blocks, own fields and preview image. You then translate the title,
the text and the address; the address starts as `kontakt-fr` and becomes
`contact`. A draft, because a half-translated page does not belong on the web.

The versions form a **star**: all of them point at the same page in the main
language. Not a chain — that would build the order of creation in permanently,
and deleting a middle language would tear the group apart. Deleting the German
page does not delete the French one; that one then stands on its own.

**What exists per language:** pages, menus (the same main menu again in French),
the archive, the label pages and the feed at `/fr/feed.xml`. If a menu is missing
in a language, the main language's is used — per location, so that "main menu
first, footer menu later" works. Labels themselves are shared: a topic is the
same topic in French, only the content under it differs.

**What a missing translation does:** `/fr/kontakt` answers 404, it does not show
the German page. Honest rather than confusing — and the language picker only
offers what really exists. A picker that offers French and then answers 404 is
worse than no picker at all.

**For search engines** every page carries its own `canonical` with prefix, plus
`hreflang` for each version that really exists. The sitemap lists all languages
with their prefixes and additionally every home page.

**For a template** there are two fields: `.Site.Sprachen` is the language picker —
the versions of this page where there are any, otherwise the home pages of the
languages. `.Page.Uebersetzungen` are only the real versions, and `hreflang` is
built from those. The names are given in the language itself ("Français"),
because the picker is read by somebody who does not understand the page they are
standing on.

One limitation worth knowing: **addresses are unique per website, across all
languages.** So there can be `/kontakt` and `/fr/contact`, but not the same slug
twice in two languages. In practice this is rarely in the way — a French page is
called something different from the German one anyway.

## The language of the admin

Five languages are bundled: **German, English, French, Italian and Spanish**, all
complete, plus the **Swiss variants** of the three national languages. Under **My
account → Language of the admin** each person picks their own; without a choice
the browser decides (`Accept-Language`), and that applies to the login screen too
— the place where an unreadable language would be worst, because from there no
path leads to a setting.

The language belongs to the person, not to the website: a German editor and an
English-speaking developer work on the same website and each sees their own
admin.

**How it is built.** The German sentence is the key:

```html
<label>{{t "Titel"}}</label>
<p>{{tf "Seite %d von %d" .Page .TotalPages}}</p>
<p>{{th "Ohne Favicon fragt jeder Browser <code>/favicon.ico</code> an."}}</p>
```

`t` translates, `tf` translates the pattern and then fills it (so a language can
reorder the parts), `th` is for a sentence that carries its own markup — otherwise
it would fall apart into three fragments no translator can do anything with. All
three only ever see string literals from the source, never user data.

Two things follow, and both are worth more than a tidy key scheme:

- **A missing translation falls back to German** — to a sentence somebody can
  read, rather than to `page.saved`, which would be an error on screen.
- **The templates stay readable.** You can see what a screen says without looking
  anything up — the difference between a translation that is maintained and one
  that rots.

Messages from Go need nothing: `SetFlashError` translates when storing,
`NewLayoutData` translates the title, and form errors are translated when drawn.
Only a composed sentence needs `web.Titlef(r, "Seiten – %s", name)`, so the frame
stays findable. Labels built at startup (block kinds, field kinds, readiness) are
marked with `i18n.N` and translated by the template with `{{t .Name}}`.

## Adding a language without a rebuild

Languages are files, like templates: **what is on disk wins over what is in the
binary.** The directory is `data/sprachen/`, one file per language, named after
the code (`nl.json`, `fr-CH.json`).

In the admin, under **Languages**:

1. **Download the template** — a JSON file with all German sentences as keys and
   empty values (currently 1158 of them).
2. Fill in the values. What stays empty appears in German; half a translation is
   usable.
3. **Upload** — or copy the file into the directory by hand and press *Reload*. A
   restart is not needed.

Deleting works the same way: remove the file, or press *Remove* in the admin.
Anyone who had that language set falls back to their browser's.

The same mechanism corrects a bundled language: a `de.json` with ten lines in it
changes ten wordings, everything else still comes from the binary.

Files are checked before they land in the directory:

- The file must consist of text pairs — German sentence to translation.
- The **placeholders must match**. A missing `%s` would otherwise silently
  swallow a name; such lines are skipped and the German sentence stays.
- **Markup is filtered.** `<code>`, `<strong>` and a few others are allowed for
  the text itself; everything else is removed. A language file is therefore not a
  way to get foreign markup into the admin.

A language that should ship with the program goes to `internal/i18n/locales/`
instead and is built in:

```bash
cp internal/i18n/locales/en.json internal/i18n/locales/nl.json
go run ./tools/i18n              # reports what is missing and what is orphaned
go run ./tools/i18n -write       # adds new strings, empty
```

No Go code, no key list. The tool reads the `{{t}}` calls in the templates and
the messages in the Go source; `go test ./internal/i18n/` checks that no bundled
translation is empty and that the placeholders match the German version.

## Regional variants

A language with a region is a language file like any other, with **one rule on
top: what it does not say, its base language says.** `de-CH.json` contains the
few dozen sentences written differently in Switzerland; the rest come from
German. That is what keeps a regional variant maintainable: a new sentence in the
program appears in it as soon as it is translated once, and nobody keeps two
nearly identical files in step by hand.

The three national languages ship in their Swiss form:

| File | What is in it |
|---|---|
| `de-CH.json` | no ß (`gross`, `heisst`, `Strasse`), «angled quotation marks» instead of „…", *Natel* instead of *Handy* |
| `fr-CH.json` | *natel* instead of *téléphone*, *laptop* instead of *portable* |
| `it-CH.json` | *formulario* instead of *modulo*, *natel* instead of *cellulare* |

Romansh is the fourth national language and has no second form to differ from. An
`rm.json` in the directory is enough, and the admin lists it as *Rumantsch* —
written, it is not.

**The browser finds them on its own.** A Swiss browser sends
`Accept-Language: de-CH,de;q=0.9`; the whole code is looked up first, then the
language without the region. So `de-CH` hits the Swiss variant, `de-DE` hits
German, and a code with nothing behind it ends at German rather than at nothing.

Swiss German spelling is a rule, not a judgement: no ß, quotation marks pointing
outwards. It is therefore computed, not typed:

```bash
go run ./tools/i18n -schweiz     # builds de-CH.json from the German source
```

What the tool produces by rule it rewrites; lines added by hand — a word like
*Natel*, which is not a spelling question — stay and get the rule applied on top.
A `go test ./internal/i18n/` checks that no Swiss variant contains a ß and that
each of its keys is a sentence that really exists in the program.
