# Writing a template for Holzcloud CMS

This document is the complete contract for a public site template. It is
written to be handed to an AI agent as-is: everything needed to produce a
working template is here, and nothing here needs to be looked up elsewhere.

Print the current copy from the running binary at any time:

    holzcloud template spec

Check your work before uploading, as often as you like:

    holzcloud template check ./my-theme
    holzcloud template check my-theme.zip

The same checks run on upload, so anything `check` accepts will install, and
anything it rejects would have been rejected anyway — with the same message.

---

## 1. What you are building

A template is a directory of files, zipped. The zip is uploaded in the admin UI
under **Templates**, and activated per website. One CMS can host many websites
and each picks its own template.

    my-theme/
      layout.html      required — the whole HTML document
      page.html        required — a single page
      home.html        optional
      list.html        optional
      search.html      optional
      gate.html        optional
      404.html         optional
      maintenance.html optional
      shop.html        optional — the catalogue, on a website that sells
      product.html     optional — one product
      cart.html        optional — the basket
      checkout.html    optional — the order form
      order.html       optional — the confirmation of a placed order
      style.css        optional, but every real template has one
      fonts/…          optional, bundled font files
      *.svg *.png …    optional, bundled images

Zip the **contents**, not the folder: `layout.html` must be at the root of the
archive, not inside `my-theme/`.

Every file you omit is served from the built-in default theme instead, wrapped
in **your** `layout.html`. This is a real, supported configuration — an archive
containing only `layout.html` and `page.html` is complete — but it means your
layout has to work with markup you did not write. The checker renders that
combination for you.

---

## 2. The rules

These are enforced. Breaking one means the upload is refused with a message
naming the file.

### 2.1 Nothing is loaded from a third party. Ever.

Every byte the browser fetches must come from this server. No CDN, no Google
Fonts, no remote images, no analytics, no embedded maps, no off-site iframes.

This is enforced twice: the upload rejects an archive that references an
external subresource, and a `Content-Security-Policy` of `default-src 'self'`
blocks it in the browser if it ever got that far.

Want a font? Put the file in the archive and reference it relatively:

```css
@font-face {
  font-family: "Inter";
  src: url("/t/fonts/inter.woff2") format("woff2");
  font-display: swap;
}
```

An outbound hyperlink — `<a href="https://example.com">` — is content, not a
subresource, and is fine.

### 2.2 No JavaScript.

Not a `.js` file, not an inline `<script>`, not an `onclick=` attribute, not a
`javascript:` URL. The five shipped themes contain none.

The things people reach for a script for are all reachable without one in 2026:

| Want | Use |
|---|---|
| A mobile menu that opens | A hidden `<input type="checkbox">` and a `<label>` |
| An accordion / FAQ | `<details>` and `<summary>` |
| A lightbox | `:target` and an anchor, or `<dialog>` with an anchor |
| A carousel | `scroll-snap-type` on an overflow container |
| Reveal on scroll | `animation-timeline: view()` |
| Sticky header shadow | `position: sticky` and `animation-timeline: scroll()` |
| A tooltip | `:hover` / `:focus-visible` plus `aria-describedby` |

One exception, because it is not code: a `<script type="application/ld+json">`
data block. The browser never executes it. Use it for `.Meta.StructuredData`,
which the CMS builds for you — see §6.

### 2.3 Sanitising is already done. Do not undo it.

`.Page.ContentHTML`, `.Search…Snippet` and the values in `.Site.Snippets` are
already HTML-sanitised by the server and are typed so the template engine emits
them unescaped. Write `{{.Page.ContentHTML}}` — nothing else is needed.

The `safeHTML` helper exists for legacy templates. You do not need it. Calling
it on anything a visitor can influence is how a template introduces a
cross-site-scripting hole into an application that did not have one.

### 2.4 File types

Allowed: `.html` `.css` `.svg` `.png` `.jpg` `.jpeg` `.gif` `.webp` `.ico`
`.woff` `.woff2` `.ttf`. At most 500 files, 10 MB uncompressed by default.

---

## 3. How rendering works

Exactly two files are parsed together for any given request:

1. `layout.html` — always yours, and it is what gets executed.
2. one view file — `home.html`, `page.html`, `list.html`, `search.html`,
   `gate.html`, `shop.html`, `product.html`, `cart.html`, `checkout.html`,
   `order.html`, `404.html`, or `maintenance.html`.

The view supplies a template named `content`; the layout pulls it in.

**layout.html**

```html
<!DOCTYPE html>
<html lang="{{.Site.Locale}}">
<head>
  <meta charset="UTF-8">
  <meta name="viewport" content="width=device-width, initial-scale=1.0">
  <title>{{.Page.Title}}{{if .Site.Name}} — {{.Site.Name}}{{end}}</title>
  <link rel="stylesheet" href="/t/style.css">
</head>
<body>
  {{template "content" .}}
</body>
</html>
```

**page.html**

```html
{{define "content"}}
  <article>
    <h1>{{.Page.Title}}</h1>
    {{.Page.ContentHTML}}
  </article>
{{end}}
```

Both spellings of the inclusion work: `{{template "content" .}}` and
`{{block "content" .}}{{end}}`.

Three consequences worth stating plainly:

- **There are no partials across files.** Only the layout and one view are
  parsed together. A `{{define "header"}}` in a third file is never seen. Define
  any sub-template inside `layout.html` itself, where every view can reach it.
- **Every view must define `content`,** and a view file that renders markup
  outside that block renders nothing at all.
- **`layout.html` is always yours,** even for the views you did not supply.

The templating language is Go's `html/template`. It escapes by context, which
means it knows the difference between HTML text, an attribute, a URL and a
`<style>` block, and escapes accordingly.

---

## 4. Asset URLs

| You write | The browser gets |
|---|---|
| `/t/style.css` | `style.css` from your archive |
| `/t/fonts/inter.woff2` | `fonts/inter.woff2` from your archive |
| `{{.Site.LogoURL}}` | an uploaded image, e.g. `/media/1/logo.png` |

`/t/` maps to your template directory, subfolders included. Media the operator
uploaded is under `/media/` and always reaches you as a ready-made path — never
build one yourself.

Fixed public routes you may link to: `/` `/suche` `/feed.xml` `/sitemap.xml`.

---

## 5. The data

One value is passed to every view. `{{.Site.Name}}` reads it. Fields are only
ever added to this structure, never renamed or removed, so a template written
today keeps working.

### `.Site` — the website

| Field | Type | Notes |
|---|---|---|
| `.Site.Name` | string | Always present |
| `.Site.Description` | string | A tagline; may be empty |
| `.Site.MetaDescription` | string | Site-wide fallback description |
| `.Site.Locale` | string | `de`, `en`, … — put it in `<html lang="">` |
| `.Site.TimeZone` | string | IANA name; dates are already converted |
| `.Site.FaviconURL` | string | `/media/…` or empty |
| `.Site.LogoURL` | string | `/media/…` or empty |
| `.Site.URL` | string | Canonical base, e.g. `https://example.de` |
| `.Site.Snippets` | map | Reusable HTML blocks by key (§7) |
| `.Site.Terms` | list of `TermLink` | Labels in use, most used first |
| `.Site.Design` | CSS | The operator's colour and font settings (§7) |
| `.Site.HasSearch` | bool | Whether this site answers `/suche` at all — hide the search form when false |
| `.Site.FeedURL` | string | The Atom feed in the language being served, or empty |
| `.Site.Sprachen` | list of `LanguageLink` | The site's languages, main one first. Empty on a one-language site |

### `.Page` — the page

| Field | Type | Notes |
|---|---|---|
| `.Page.Title` | string | Always present |
| `.Page.ContentHTML` | HTML | The body. Already sanitised |
| `.Page.Slug` | string | e.g. `about`; the home page is `home` |
| `.Page.PublishedAt` | date or **nil** | The date helpers accept nil (§8) |
| `.Page.UpdatedAt` | date or **nil** | The date helpers accept nil (§8) |
| `.Page.Excerpt` | string | Short plain-text summary |
| `.Page.HasOwnHeading` | bool | **See below** |
| `.Page.IsPost` | bool | True for a blog entry, false for a page |
| `.Page.Prev` | `PageLink` or **nil** | Older neighbouring post |
| `.Page.Next` | `PageLink` or **nil** | Newer neighbouring post |
| `.Page.ArchiveURL` | string | The blog index, or empty |
| `.Page.Terms` | list of `TermLink` | This page's labels |
| `.Page.Art` | string | The key of the website's own content kind — `produkt`, `termin`. Empty for an ordinary page or post |
| `.Page.Felder` | map | The website's own fields by key, resolved to the type they mean |
| `.Page.Feldliste` | list of `FieldEntry` | The same fields in their defined order, with labels |
| `.Page.Uebersetzungen` | list of `LanguageLink` | The languages this page really exists in. Empty on a one-language site |

`.Page.HasOwnHeading` is true when the editor's text already starts with a
heading. Print the title yourself only when it is false, or the page shows its
heading twice — the single most visible defect a new template ships with:

```html
{{if not .Page.HasOwnHeading}}<h1>{{.Page.Title}}</h1>{{end}}
```

### `.Meta` — for `<head>`

| Field | Type | Notes |
|---|---|---|
| `.Meta.CanonicalURL` | string | Absolute |
| `.Meta.Description` | string | Page value, else excerpt, else site default |
| `.Meta.OGImage` | string | Absolute URL, or empty |
| `.Meta.NoIndex` | bool | Emit `<meta name="robots" content="noindex, follow">` |
| `.Meta.Message` | string | The operator's text on `maintenance.html` |
| `.Meta.StructuredData` | JSON | schema.org JSON-LD, ready to place (§6) |

### `.Menus` — navigation

A map from a location key to a menu. The operator names the keys; `main` and
`footer` are the convention every shipped theme uses.

```html
{{menuFor .Menus "main" .Page.Slug}}
```

That emits nested `<ul><li><a>` with no classes, up to three levels, and marks
the current entry with `aria-current="page"`. Style it by descent:

```css
.site-nav ul { list-style: none; display: flex; gap: 1rem; }
.site-nav a[aria-current="page"] { text-decoration: underline; }
```

Wrap it in a `<nav>` with an `aria-label` yourself — the helper does not, and a
bare `<ul>` is not a landmark.

### `.Archive` — only in `list.html`

| Field | Type | Notes |
|---|---|---|
| `.Archive.Entries` | list of `ArchiveEntry` | May be empty — say so |
| `.Archive.Page` | int | Current page number |
| `.Archive.TotalPages` | int | 1 when everything fits |
| `.Archive.Total` | int | Entries in the whole archive |
| `.Archive.PrevURL` | string | Empty at the start |
| `.Archive.NextURL` | string | Empty at the end |
| `.Archive.Term` | string | The label being filtered by, or empty |
| `.Archive.Terms` | list of `TermLink` | For a label cloud |

`ArchiveEntry`: `.Title` `.URL` `.Excerpt` `.PublishedAt` (may be nil)
`.ImageURL` (may be empty) `.Terms`

When `.Archive.Term` is set, **name it on the page**. An archive that silently
shows a subset leaves the reader guessing why these entries and no others.

### `.Search` — only in `search.html`

| Field | Type | Notes |
|---|---|---|
| `.Search.Query` | string | What was typed |
| `.Search.Submitted` | bool | Separates "nothing typed yet" from "no results" |
| `.Search.Results` | list of `SearchHit` | |

`SearchHit`: `.Title` `.URL` `.Snippet` (HTML with `<mark>` around the terms —
style `mark`).

### `.Gate` — only in `gate.html`

| Field | Type | Notes |
|---|---|---|
| `.Gate.Hint` | string | The operator's note above the field, may be empty |
| `.Gate.Path` | string | The page being unlocked |
| `.Gate.Wrong` | bool | True after a failed attempt — say so, with `role="alert"` |

### `.Preview` — a page reached through a share link

| Field | Type | Notes |
|---|---|---|
| `.Preview.Active` | bool | Show a banner when true |
| `.Preview.Status` | string | `draft`, `review`, … |

A reader who cannot tell a preview from the live site will report the page as
published, and nobody will believe them.


### `.Shop` — on every page of a website that sells

Unlike `.Catalogue` and `.Product`, this one is filled on **every** page, so a
layout can put a basket link or a price switch in the header. On a website with
no shop `.Shop.Enabled` is false and everything else is empty — which most
websites in a multi-site installation are, so guard it.

| Field | Type | Notes |
|---|---|---|
| `.Shop.Enabled` | bool | False when this website sells nothing |
| `.Shop.URL` | string | The catalogue's address, e.g. `/shop` |
| `.Shop.Audience` | string | `private` or `business` — whose prices are shown |
| `.Shop.CanSwitchAudience` | bool | **See below** |
| `.Shop.TaxNote` | string | "inkl. 8.1 % MWST", worded for the audience |
| `.Shop.ShippingNote` | string | "Versandkostenfrei ab CHF 200.00", or empty |
| `.Shop.Categories` | list of `TermLink` | For a shop menu |

`.Shop.CanSwitchAudience` is true only where the operator serves both consumers
and trade. **Do not offer the switch when it is false.** The price a consumer is
shown is regulated — under the Preisbekanntgabeverordnung it must be the amount
actually payable — so a consumer shop must not offer to show net prices.

### `.Catalogue` — only in `shop.html`

| Field | Type | Notes |
|---|---|---|
| `.Catalogue.Products` | list of `ProductEntry` | May be empty — say so |
| `.Catalogue.Page` | int | Current page |
| `.Catalogue.TotalPages` | int | 1 when everything fits |
| `.Catalogue.Total` | int | Products in the whole catalogue |
| `.Catalogue.PrevURL` | string | Empty at the start |
| `.Catalogue.NextURL` | string | Empty at the end |
| `.Catalogue.Term` | string | The category being filtered by, or empty |

`ProductEntry`: `.Title` `.Subtitle` `.URL` `.Excerpt` `.ImageURL` (may be
empty) `.Price` `.PriceNote` `.Available` `.SoldOutLabel` `.Terms`

### `.Product` — only in `product.html`

| Field | Type | Notes |
|---|---|---|
| `.Product.Title` | string | |
| `.Product.Subtitle` | string | "Eiche massiv, geölt" — may be empty |
| `.Product.Slug` | string | Goes into the buy form |
| `.Product.DescriptionHTML` | HTML | Already sanitised |
| `.Product.SKU` | string | Artikelnummer, may be empty |
| `.Product.Price` | string | |
| `.Product.PriceNote` | string | |
| `.Product.PriceOther` | string | The other audience's price, or empty |
| `.Product.ImageURL` | string | Main picture, may be empty |
| `.Product.Gallery` | list of string | Further pictures |
| `.Product.Available` | bool | False when tracked and sold out |
| `.Product.StockNote` | string | "Noch 2 an Lager", or empty when untracked |
| `.Product.DeliveryNote` | string | "Lieferzeit 3–4 Wochen", or empty |
| `.Product.Terms` | list of `TermLink` | |
| `.Product.AddURL` | string | Where the buy form posts |

### Prices are strings, deliberately

`.Price` arrives as `CHF 1’234.55` — currency, grouping and decimal separator
already applied, with a **non-breaking space** so it cannot wrap after "CHF".
There is no numeric price field and no formatting helper, on purpose: money
arithmetic in a template is money arithmetic nobody tests, and the difference
between a consumer total and a trade total is a rounding rule, not a
multiplication.

Never build a price yourself, and never put `.Price` into a form field.


### `.Cart` — the basket

Like `.Shop`, this is filled on **every** page of a selling website, so a layout
can put a basket link in its header. Only `.Cart.Count`, `.Cart.URL` and
`.Cart.Total` are filled there — reading the lines on every request would be a
join to render a number. Everything else is filled only in `cart.html`.

| Field | Type | Notes |
|---|---|---|
| `.Cart.Count` | int | Articles, summed over the lines. 0 means empty |
| `.Cart.URL` | string | `/warenkorb` |
| `.Cart.Total` | string | Printed amount |
| `.Cart.Lines` | list of `CartLine` | Only in `cart.html` |
| `.Cart.Totals` | `CartTotals` | Only in `cart.html` |
| `.Cart.CheckoutURL` | string | Empty when the basket cannot be ordered |
| `.Cart.Blocked` | string | Why checkout is impossible, or empty |
| `.Cart.UpdateURL` | string | Where the quantity form posts |
| `.Cart.RemoveURL` | string | Where the remove form posts |

`CartLine`: `.Title` `.Subtitle` `.URL` `.Slug` `.ImageURL` `.Quantity`
`.UnitPrice` `.LinePrice` `.Available`

`CartTotals`: `.Items` `.Shipping` `.ShippingFree` (bool — say "kostenlos"
rather than "CHF 0.00") `.Total` `.TaxLines` `.TaxNote`

`CartTaxLine`: `.Label` `.Net` `.Tax`

`.Cart.Count` is the number of **articles**, not of lines: a badge that counts
lines says "1" for ten of the same stool.

An article that ran out while the basket sat there stays in the list with
`.Available` false, and `.Cart.Blocked` explains why checkout is closed. Do not
hide such a line — quietly shortening someone's basket and charging for the rest
is worse than telling them.


### `.Checkout` — only in `checkout.html`

| Field | Type | Notes |
|---|---|---|
| `.Checkout.Action` | string | Where the form posts — always back to itself |
| `.Checkout.Values` | map | What was typed, by field name |
| `.Checkout.Errors` | map | Messages by field name — print beside the field |
| `.Checkout.Notice` | string | A problem with the order as a whole, or empty |
| `.Checkout.Business` | bool | True when ordering as a company |
| `.Checkout.Methods` | list of `PaymentMethod` | `.Value` `.Label` `.Note` |
| `.Checkout.ReturnPolicy` | string | May be empty — **see below** |
| `.Checkout.Accepted` | bool | State of the confirmation box |

Read the values with `index`: `{{index .Checkout.Values "email"}}`. A rejected
form must come back filled in — retyping an address because one field was wrong
is how a shop loses an order.

**`.Checkout.ReturnPolicy` is often empty, and that is lawful.** Switzerland
grants no statutory right of withdrawal for orders placed online — OR Art. 40a
covers doorstep and telephone sales, not a web shop. Whatever stands here is a
voluntary promise by the shop. When it is empty, print nothing: inventing a
"14-day right of return" because other shops have one states a promise the
operator never made.

The form fields are fixed, like the other form contracts in §6: `email` `name`
`firma` `uid` `telefon` `strasse` `plz` `ort` `land` `bemerkung` `zahlungsart`
`bestaetigt`.

### `.Order` — only in `order.html`

The confirmation of a placed order.

| Field | Type | Notes |
|---|---|---|
| `.Order.Number` | string | "2026-0007" |
| `.Order.Email` `.Order.Name` `.Order.Company` | string | |
| `.Order.Address` | string | The delivery address on one line |
| `.Order.Note` | string | What the customer wrote, or empty |
| `.Order.Status` | string | `new`, `paid`, `shipped`, `cancelled` |
| `.Order.PaymentLabel` | string | The chosen method in words |
| `.Order.PaymentNote` | string | Where the payment stands, in a sentence |
| `.Order.PaymentPending` | bool | True while the money has not arrived |
| `.Order.ReturnPolicy` | string | As promised **at the time of the order** |
| `.Order.Lines` | list of `CartLine` | Frozen: the title and price as sold |
| `.Order.Totals` | `CartTotals` | |

Print `.Order.PaymentNote` wherever the payment is mentioned. It is the only
thing on the page that tells the customer whether they still owe something,
and it is worded for the method they chose — with an invoice it says the bill
comes with the goods, with a failed card payment it says to get in touch.
`.Order.PaymentPending` carries the same fact as a flag, for a theme that wants
to mark an unsettled order visually; ignoring it is fine.

The page is reachable by its number with no login, and the number is
sequential. Nothing on it may be worth guessing — show what was ordered and
where it goes, which the person who placed it already knows.

### `TermLink`

`.Name` `.URL` `.Count` — used for labels throughout.

### `LanguageLink`

`.Code` `.Name` `.URL` `.Active` — used by `.Site.Sprachen` and
`.Page.Uebersetzungen`.

`.Code` is the tag for `lang` and `hreflang` and is **always filled**, including
for the main language. `.Name` is the language as it calls itself — "Français",
"Italiano" — because a switcher is read by somebody who does not speak the page
they are standing on. `.Active` marks the language being shown right now.

```html
{{if .Site.Sprachen}}
<nav aria-label="Sprache">
  <ul>
    {{range .Site.Sprachen}}
    <li><a href="{{.URL}}" hreflang="{{.Code}}"{{if .Active}} aria-current="true"{{end}}>{{.Name}}</a></li>
    {{end}}
  </ul>
</nav>
{{end}}
```

Use `.Page.Uebersetzungen` for `<link rel="alternate">` in `<head>`: it lists
only the languages this page actually exists in, so it will not promise a
translation that answers with a 404.

### `FieldEntry`

`.Key` `.Label` `.Kind` `.Value` `.Text` `.Image` `.Ref` `.Rows` — one entry of
`.Page.Feldliste`.

The website's own fields are defined by the operator, so a template cannot know
their names. Print `.Label` and the value and let the order decide the layout:

```html
{{if .Page.Feldliste}}
<dl>
  {{range .Page.Feldliste}}
  <dt>{{.Label}}</dt>
  <dd>
    {{if eq .Kind "bild"}}{{with .Image}}<img src="{{.URL}}" alt="{{.Alt}}" loading="lazy">{{end}}
    {{else if eq .Kind "datum"}}<time datetime="{{formatDateISO .Value}}">{{formatDate .Value}}</time>
    {{else if eq .Kind "verweis"}}{{with .Ref}}<a href="{{.URL}}">{{.Title}}</a>{{end}}
    {{else}}{{.Text}}{{end}}
  </dd>
  {{end}}
</dl>
{{end}}
```

`.Page.Felder` is the same data as a map, keyed by field name. It is for a
theme written for one particular site, which knows the names it wants.

---

## 6. Forms and fixed contracts

Two views submit to the server. The endpoints and the **field names** are
fixed, are not guessable, and are checked on upload. A form that renames a
field renders perfectly and submits nothing the server can use.

**gate.html** — unlocking a password-protected page:

```html
<form method="POST" action="/freischalten">
  <input type="hidden" name="seite" value="{{.Page.Slug}}">
  <label for="pw">Passwort</label>
  <input type="password" id="pw" name="passwort" required autocomplete="current-password">
  <button type="submit">Weiter</button>
</form>
```

**product.html** — the buy form:

```html
<form method="POST" action="{{.Product.AddURL}}">
  <input type="hidden" name="artikel" value="{{.Product.Slug}}">
  <label for="menge">Menge</label>
  <input type="number" id="menge" name="menge" value="1" min="1" max="99">
  <button type="submit">In den Warenkorb</button>
</form>
```

**cart.html** — the quantity and remove forms:

```html
<form method="POST" action="{{$.Cart.UpdateURL}}">
  <input type="hidden" name="artikel" value="{{.Slug}}">
  <input type="number" name="menge" value="{{.Quantity}}" min="0" max="99">
  <button type="submit">Ändern</button>
</form>
```

A quantity of 0 removes the line. The article is named by its slug, never by a
number — a numeric id from a form would let a guessed value reach another
website's catalogue.

**shop.html** — the price switch, only when `.Shop.CanSwitchAudience`:

```html
<form method="POST" action="/preise">
  <input type="hidden" name="zurueck" value="{{.Shop.URL}}">
  <button type="submit" name="ansicht" value="private">inkl. MWST</button>
  <button type="submit" name="ansicht" value="business">exkl. MWST</button>
</form>
```

**search.html** — the search box:

```html
<form method="GET" action="/suche" role="search">
  <label for="q">Suche</label>
  <input type="search" id="q" name="q" value="{{.Search.Query}}">
  <button type="submit">Suchen</button>
</form>
```

Put the search form in `layout.html` too if the design calls for it; it is a
plain GET and works from anywhere.

**Structured data**, in `layout.html`:

```html
{{if .Meta.StructuredData}}
<script type="application/ld+json">{{.Meta.StructuredData}}</script>
{{end}}
```

---

## 7. The operator's own settings

Two fields let the person running the site change things without editing your
template. Honouring them is what makes a template reusable.

**`.Site.Design`** is a `:root` rule the operator produced in the admin UI. Emit
it *after* your stylesheet so it wins:

```html
<link rel="stylesheet" href="/t/style.css">
{{if .Site.Design}}<style>{{.Site.Design}}</style>{{end}}
```

It may define `--hc-ink`, `--hc-paper`, `--hc-brand`, `--hc-font`,
`--hc-measure`, `--hc-radius`. Build your own tokens on top with a fallback, so
the template stands alone and still bends when told to:

```css
:root {
  --ink:   var(--hc-ink,   oklch(22% 0.02 60));
  --brand: var(--hc-brand, oklch(55% 0.12 45));
  --font:  var(--hc-font,  ui-serif, Georgia, serif);
}
```

**`.Site.Snippets`** are reusable blocks the operator maintains, addressed by
key. Useful for putting an address in the footer without hard-coding it:

```html
{{with index .Site.Snippets "footer-kontakt"}}<div>{{.}}</div>{{end}}
```

---

## 8. Helper functions

| Helper | Use |
|---|---|
| `formatDate` | `{{formatDate .Page.PublishedAt}}` → `14. März 2026`, in the site's language |
| `formatDateShort` | A compact date |
| `formatDateISO` | `2026-03-14` — for `<time datetime="…">`, never for display |
| `formatWeekday` | The weekday name |
| `menu` | `{{menu .Menus "main"}}` — no current-page marking |
| `menuFor` | `{{menuFor .Menus "main" .Page.Slug}}` — marks the current entry |
| `safeHTML` | Legacy. Do not use — see §2.3 |

There are no others. Go's built-ins (`if`, `range`, `with`, `index`, `len`,
`printf`, `eq`, `and`, `or`, `not`) are all available.

Dates are already converted to the site's time zone. Pass the value straight in.

**The four date helpers accept a missing date** and return an empty string, so
`{{formatDate .Page.PublishedAt}}` never fails. Wrapping it in `{{with}}` is
still worth doing where it would otherwise leave an empty `<time>` element or a
stray separator behind — but that is a matter of tidiness, not of correctness.
The things in §9 are the ones that genuinely break.

---

## 9. Everything optional can be absent

This is the mistake that passes review and breaks in production. `.Page.Next`
is nil on the newest post, `.Page.Prev` on the oldest. `.Site.LogoURL` is empty
until someone uploads a logo. Menus are empty on a new site, `.Archive.Entries`
before the first post, `.Search.Results` when nothing matched.

Reaching *through* a nil value is what errors — `.Page.Next.URL` when there is
no next post. Printing an empty string or ranging over an empty list is
harmless.

Wrong — works until a visitor opens the newest post, then errors:

```html
<a href="{{.Page.Next.URL}}">{{.Page.Next.Title}}</a>
```

Right:

```html
{{with .Page.Next}}<a href="{{.URL}}">{{.Title}}</a>{{end}}
```

`{{with}}` binds `.` to the value and skips the block when it is absent, which
is why it is the right tool here and `{{if}}` usually is not.

The checker renders every view twice — once with every field filled in, once
with every optional field empty — precisely to catch this.

---

## 10. What a good template does anyway

Not enforced, but every shipped theme does all of it, and a template that skips
them is worse than the default:

- **A skip link** — `<a href="#main" class="skip-link">` as the first element in
  `<body>`, pointing at `<main id="main" tabindex="-1">`. Without it a keyboard
  user tabs through the whole navigation on every page.
- **Landmarks** — `<header>`, `<main>`, `<footer role="contentinfo">`, and each
  menu wrapped in a `<nav>` with its own `aria-label`.
- **Never remove the focus ring** without replacing it. `outline: none` with
  nothing in its place leaves a keyboard user with no idea where they are.
- **A print stylesheet** — `@media print`. People print recipes, menus, opening
  hours and invoices.
- **Dark mode** — `light-dark()` with `color-scheme: light dark`, or a
  `prefers-color-scheme` block.
- **Reduced motion** — wrap animation in
  `@media (prefers-reduced-motion: no-preference)`.
- **Responsive images** — `loading="lazy"` and `decoding="async"` on anything
  below the fold.
- **Real empty states.** An empty archive, a search with no results, a site with
  no menu yet. Say what is going on rather than rendering a blank area.

CSS is unrestricted and unprocessed — there is no build step, and nothing is
transpiled or prefixed. Write for current browsers: `@layer`, OKLCH,
`light-dark()`, container queries, `:has()`, `text-wrap: balance`, subgrid,
`@view-transition`, scroll-driven animations.

---

## 11. A complete minimal template

Two files. It installs, and it renders every view.

**layout.html**

```html
<!DOCTYPE html>
<html lang="{{.Site.Locale}}">
<head>
  <meta charset="UTF-8">
  <meta name="viewport" content="width=device-width, initial-scale=1.0">
  <title>{{.Page.Title}}{{if .Site.Name}} — {{.Site.Name}}{{end}}</title>
  {{if .Meta.Description}}<meta name="description" content="{{.Meta.Description}}">{{end}}
  {{if .Meta.NoIndex}}<meta name="robots" content="noindex, follow">{{end}}
  {{if .Meta.CanonicalURL}}<link rel="canonical" href="{{.Meta.CanonicalURL}}">{{end}}
  <link rel="stylesheet" href="/t/style.css">
  {{if .Site.Design}}<style>{{.Site.Design}}</style>{{end}}
</head>
<body>
  <a href="#main" class="skip-link">Zum Inhalt springen</a>

  <header>
    <a href="/">{{.Site.Name}}</a>
    <nav aria-label="Hauptnavigation">{{menuFor .Menus "main" .Page.Slug}}</nav>
  </header>

  <main id="main" tabindex="-1">
    {{template "content" .}}
  </main>

  <footer role="contentinfo">
    <nav aria-label="Rechtliches">{{menuFor .Menus "footer" .Page.Slug}}</nav>
    <p>&copy; {{.Site.Name}}</p>
  </footer>
</body>
</html>
```

**page.html**

```html
{{define "content"}}
<article>
  {{if not .Page.HasOwnHeading}}<h1>{{.Page.Title}}</h1>{{end}}

  {{if .Preview.Active}}
    <p class="preview-banner" role="status">Vorschau — Status: {{.Preview.Status}}</p>
  {{end}}

  {{with .Page.PublishedAt}}
    <time datetime="{{formatDateISO .}}">{{formatDate .}}</time>
  {{end}}

  {{.Page.ContentHTML}}

  {{with .Page.Terms}}
    <ul class="terms">
      {{range .}}<li><a href="{{.URL}}">{{.Name}}</a></li>{{end}}
    </ul>
  {{end}}

  {{if or .Page.Prev .Page.Next}}
    <nav aria-label="Weitere Beiträge">
      {{with .Page.Prev}}<a href="{{.URL}}">← {{.Title}}</a>{{end}}
      {{with .Page.Next}}<a href="{{.URL}}">{{.Title}} →</a>{{end}}
    </nav>
  {{end}}
</article>
{{end}}
```

---

## 12. Before you ship

Run `holzcloud template check` — it catches everything below that can be caught
mechanically, and its messages name the file, the line, and the fix.

- [ ] `layout.html` is a whole document and includes `{{template "content" .}}`
- [ ] Every view file is wrapped in `{{define "content"}} … {{end}}`
- [ ] Nothing is fetched from another origin
- [ ] No JavaScript anywhere
- [ ] Every optional field is guarded with `{{with}}` or `{{if}}`
- [ ] `.Page.HasOwnHeading` is respected
- [ ] `gate.html` and `search.html` keep their field names
- [ ] A skip link, landmarks, a visible focus ring
- [ ] `@media print` exists
- [ ] Empty archive, empty search, no menu — all render sensibly
- [ ] Zipped with `layout.html` at the root
