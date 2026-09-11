# Content model

A page has a title, an address and text. Everything beyond that — what a page of
*this* website is made of — is defined per website, in the admin, without a new
build of the program.

- [Fields](#fields)
- [Sections and conditions](#sections-and-conditions)
- [References to your own pages](#references-to-your-own-pages)
- [Repeatable groups](#repeatable-groups)
- [Content kinds](#content-kinds)
- [Block kinds](#block-kinds)
- [Snippets](#snippets)

## Fields

Under **Fields**, each website decides what else belongs on a page.

| Kind | For |
|---|---|
| Short text | a name, a quantity, a variety |
| Long text | several lines without formatting |
| Code | several lines, shown exactly as typed — never through the Markdown renderer |
| Number | a price, a weight, a vintage |
| Range | a number between two bounds, as `<input type="number">` with `min`, `max` and `step` |
| Date | a day, without a time |
| Time | a time, without a day — deliberately separate from Date, because "not filled in" and "midnight" are two different facts |
| Yes/no | available, sold out |
| Choice | a list of possibilities, one of them |
| Multiple choice | the same list, any number of them, as checkboxes |
| Image | an image from this website's media library |
| Link | one of your own pages or a foreign address |
| Reference | a page of this website, picked rather than typed — see below |
| Label | one of this website's labels, picked rather than typed; the page always shows the current name |
| Group | several fields, filled in more than once — see below |
| Section | no input, but a heading above the fields that follow |

A field applies to pages, to posts, to both — or to exactly one of your own
content kinds. A price belongs on a product, an author on a post; asking for
both everywhere makes the form longer without making it do more.

The **key** is derived from the label once and then fixed. It is the name the
theme addresses the field by and the name every stored value sits under; if it
changed with the label, everything filled in would silently disappear after each
rewording.

In a theme, fields are reachable two ways:

```gotemplate
{{/* by name — for a theme written for this one website */}}
{{ with .Page.Fields.preis_pro_kilo }}<p class="preis">CHF {{ . }}</p>{{ end }}
{{ if .Page.Fields.direkt_bestellbar }}<p>Direct from the farm</p>{{ end }}

{{/* as a list — for a bundled theme that cannot know the names */}}
{{ range .Page.FieldList }}<dt>{{ .Label }}</dt><dd>{{ .Text }}</dd>{{ end }}
```

Values arrive typed: a number as a number (and printed the way it was typed), a
date as `*time.Time` for `formatDate`, a yes/no as `bool`, an image as a struct
with address, alt text and focal point — or `nil` when the image was deleted. A
group arrives as a list of rows, each row with its own fields. Every bundled
theme prints the list below the text; a theme of your own decides for itself.

Validation happens on save, and identically everywhere: in the admin, on import,
and over the AI endpoint. A price that is not a number is refused; a choice that
is not in the list likewise, even if somebody bypassed the `<select>`. For a
group the row number is in the message: "Price tiers, row 2: price must be a
number."

A deleted field does not take its values with it. They stay on the pages until
those are next saved — if you deleted one by mistake, create it again.

## Sections and conditions

Twenty fields in a row are a wall. Two things turn that into a form somebody
fills in.

A **section** is a heading between fields — itself a field, only one without an
input. Everything below it belongs under that heading until the next section.

A **condition** reveals a field only once another one is filled in: *offer
price* only once *on offer* is ticked. While the field is not shown:

- it is **not required**, even if it is a required field,
- the theme does **not output it** — neither under its key nor in `.FieldList`,
- **its value stays.** A checkbox unticked by accident costs nobody their input;
  tick it again and everything is back.

Showing and hiding in the browser is pure CSS, like everything else here. That
decides what a condition can do: **filled in or not**, and nothing more. Testing
for a particular value would need a stylesheet per website — a second place where
the same rule lives, and which would drift. For the same reason a condition
cannot be hung on every field: the browser cannot tell whether a date is filled
in. The form only offers the fields where it works, so nobody has to know the
rule.

The server checks a second time anyway, with the same function for admin, import
and AI access. A browser without `:has()` therefore shows one field too many at
worst, and never a wrong result.

## References to your own pages

A link is a typed address. It goes nowhere the moment somebody renames the
target, and nobody notices. A **reference** is a choice from your own stock
instead:

```gotemplate
{{ with .Page.Fields.gehoert_zu }}
  Belongs to <a href="{{ .URL }}">{{ .Title }}</a>
{{ end }}
```

What is stored is the page, not its address. Everything else follows from that:

- **Renaming does no harm** — the reference still points at the same page, under
  its new address.
- **Title and address are always current**, because they are fetched at serving
  time rather than copied at picking time.
- **A draft stays a draft.** If a reference points at an unpublished, deleted or
  password-protected page, the theme sees *nothing* — a `{{ with }}` simply
  leaves the block out. The editor says so beforehand: "This page is not
  published."
- **In an export the address travels**, not the number. A reference to a page
  that only appears later in the archive still finds its target after import.

For several targets: a **group** with a reference in it — then the order is
fixed too, and the arrows change it.

## Repeatable groups

A single field is enough for one price. It is not enough for opening hours, team
members or a price tier. That is what the **group** kind is for: create it,
decide what a row is made of, then fill it in the editor as often as there are
rows.

Exactly one level — a group inside a group is refused. Nesting costs an editor
that calls itself and a form nobody can find their way around.

*Add row*, *Remove* and the arrows are ordinary submit buttons: the server
rebuilds the form. Like the block editor, it works without a line of JavaScript.

```gotemplate
{{ range .Page.Fields.preisstaffel }}
  from {{ .ab_menge }} {{ .einheit }}: CHF {{ .preis }}
{{ end }}
```

## Content kinds

A website comes with two kinds: the **page**, which stands on its own, and the
**post**, which sits in the archive by date. Anyone keeping products, events,
recipes or animals has a third thing, and otherwise makes do by keeping it as a
page and telling them apart in their head.

Under **Content kinds** each website creates its own:

| Setting | For |
|---|---|
| Name (singular) | appears on the buttons: "New product" |
| Plural | appears above the list and in the filter |
| Key | for the theme; derived from the name and then fixed |
| Listing page | the address under which all entries are listed — `/shop`. Leave empty for a kind without one |
| Sorting | newest first, or by title |

The kind is then everywhere it belongs: in the editor's choice, as a column and
filter in the list, as a target for a field ("applies to products only"), as a
public listing and in the sitemap.

```gotemplate
{{/* the listing page uses list.html, like the archive */}}
{{ range .Archive.Entries }}<h2><a href="{{ .URL }}">{{ .Title }}</a></h2>{{ end }}

{{/* on an entry: .Page.Kind is the key, empty for page and post */}}
{{ if eq .Page.Kind "produkt" }}<p>CHF {{ .Page.Fields.preis }}</p>{{ end }}
```

Two things a kind deliberately does **not** change:

- **The addresses.** A product lives at `/large-wool-pack`, not at
  `/products/large-wool-pack`. Giving a kind or taking it away therefore breaks
  no link.
- **The stock.** Remove a kind and its entries stay, still carrying its key. If
  you removed it by mistake, create it again.

Export, import and AI access all carry the kind.

## Block kinds

The block editor comes with nine kinds — text, image, image and text, gallery,
cards, quotation, call to action, video, rule. A tenth used to mean a new
version of the program. Under **Block kinds** each website creates its own:

1. **Create the kind** — a name ("Repair step") and a hint. The key comes from
   the name and is then fixed.
2. **Decide its fields** — the same kinds as on a page: text, long text, number,
   date, yes/no, choice, image, link.
3. **Use it** — the kind sits behind the built-in ones in the editor's menu.

In the theme it arrives as markup you address with CSS:

```html
<div class="hc-block hc-eigen hc-eigen--rezeptschritt hc-ja--hervorheben">
  <p class="hc-eigen__zeile hc-eigen__zeile--nummer">Step 1</p>
  <div class="hc-eigen__text hc-eigen__text--anleitung"><h3>Start the dough</h3>…</div>
  <figure class="hc-eigen__bild hc-eigen__bild--bild"><img …></figure>
</div>
```

Three rules explain the rest:

- **A long text is output as Markdown.** That is the way to a heading or a list
  inside a block — and the reason no hand-written template is needed.
- **A yes/no outputs nothing**, it sets `hc-ja--key` on the block. With that the
  theme makes the same block emphasised or ordinary.
- **There is no reference here.** A reference follows a rename; a block is turned
  into HTML once and for all when the page is saved and could not keep that
  promise. A link says what it is.

There is deliberately no per-kind HTML template: that would be a templating
language in a text box, and therefore a way to bring a `<script>` onto a page
through the front door. The whole program rests on the promise that no such way
exists.

Export and import carry the kinds with their fields — and the blocks on the
pages with them. An image inside one travels as a file name, not as a number, and
the page is set again on the other machine.

## Snippets

Snippets are reusable blocks — an address, opening hours. A
`[[snippet:key]]` in the page text is replaced at serving time, not at saving
time, so a change happens in exactly one place. A theme can place a snippet
itself with `{{index .Site.Snippets "key"}}`.
