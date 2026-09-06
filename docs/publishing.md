# Publishing

How a website is put together day to day: sites and domains, pages, menus,
media, templates, design, SEO — and the plugins that sit alongside them.

## The admin

A dark bar along the top, the sections down the left, the content on the right on
a grey ground in white cards. The bar carries **where** you are, the sidebar
**what** you can do there — that is the split you find your way around by without
having it explained.

- **A website switcher** in the top bar. Anyone looking after several websites
  switches in one place instead of going the long way round through the list.
- **The sections stay put.** The sidebar shows pages, media, menus and the rest
  even where the address names no website: what was last worked on is remembered.
- **Plugin screens are in the menu**, among the rest. Somebody reading contact-form
  enquiries looks for them next to the content, not under "Plugins".
- **One visible action per row**, the rest behind three dots. Five equally loud
  buttons times ten rows are fifty, and then you see none of them.
- **Light and dark**, following the operating system. Only the colour values are
  swapped; there is no second set of rules that could drift apart.
- **Everything works without JavaScript.** htmx is progressive enhancement, not a
  requirement — every action is a form, and the fold-outs are
  `<details>`/`<summary>`, which the keyboard operates by itself. What they cannot
  do is close when you click beside them. That is the price of not having a line
  of JavaScript for menus.

## Websites and domains

1. **Websites** in the sidebar → create a website (name and description)
2. Add one or more domains
3. Point DNS for those domains at your server
4. Requests whose `Host` header matches a domain are served by that website

Domains are normalised on save: a pasted URL, an explicit port, a trailing dot or
capital letters are all reduced to the bare host name, and an internationalised
name is converted to punycode (`möbel.de` is stored as `xn--mbel-5qa.de`, which
is what a browser actually sends). The admin shows the readable form. Unticking
**Active** takes the site offline immediately.

## Pages

Create a page with a title, an address and Markdown; the Markdown is rendered to
HTML and sanitised. Set the status to **Published** to make it visible — a draft
returns 404 publicly and never appears in a listing, the archive, the search, the
feed or the sitemap.

A page can also be built from **blocks** instead of Markdown; see
[content model](content-model.md). Both kinds of page carry the website's own
fields, its labels, a preview image, an excerpt and a meta description.

A page can have a *visible from* and a *visible until*. Both are conditions
applied when reading, not a background job: there is no moment that can be
missed, and a machine that was switched off over the date comes up correct
already. The feed is at `/feed.xml`.

When a page is renamed, a redirect from the old address is created
automatically. Under *Redirects* you can additionally enter the addresses of an
old website; the hit counters show which of them still bring traffic.

Under **Not found** are the addresses visitors asked for that do not exist: the
broken inbound links the redirect list cannot tell you about, because it only
records what somebody already made a redirect for. Any line can be turned into a
redirect from a picker. Obvious scanner noise (`.php`, `/wp-`, `/.env`) is not
counted, and hits are collected in memory and written once a minute — one write
per 404 would go through the single-writer pool and block real page views. The
same screen lists the links in your own text that lead nowhere, with the page
they are on.

## Search

Full-text search runs on SQLite FTS5, already compiled into the pure-Go driver —
no extra service, no CGO. Diacritics are folded, so `mobel` finds "Möbel".
Publicly at `/suche`, in the admin through the search box on the page list.

## Menus

1. Create a menu with a location key (`main`, `footer`)
2. Add items: links to published pages, external URLs, or plain text
3. Nest items up to three levels deep, reorder with the up and down buttons

Menus render as nested `<ul>` lists in the public template. With more than one
language, each language has its own menus; a missing one falls back to the main
language, per location.

## Media

Upload images (JPEG, PNG, GIF, WebP, SVG), PDFs or MP4 video. Files are stored
under `data/media/{website_id}/` with unique names and served with immutable
cache headers. Insert them from the editor, or write the Markdown yourself:
`![Description](/media/{website_id}/{filename})`. The library is paginated at 24
files per page and can be filtered by name, description, kind and "unused".

**Camera data is stripped on upload.** A phone photograph carries the GPS
coordinates of where it was taken in its EXIF block; the file is served publicly
and unchanged, so a product photo taken at home would publish a private address.
EXIF, XMP, IPTC and comments are removed — by byte surgery, not by re-encoding:
the image stays bit-for-bit the same. Colour profile (ICC), pixel density (JFIF)
and colour transform (Adobe) are kept, because losing them would visibly change
the image.

The same applies to video. An MP4 refers to itself by absolute byte positions, so
nothing is cut out — the box holding the recording location is renamed `free` and
filled with zeroes. Same length, same offsets, no coordinates.

An image description (alt text) can be stored with every file and is carried over
automatically on insert. The library shows how many images do not have one yet.
When a file that is still used on a page is deleted, the error names the affected
pages rather than quietly breaking the images there.

### Video from your own library

A **video** block shows an uploaded MP4 in a `<video>` element — with `controls`,
without `autoplay`, with `preload="metadata"`, so the film itself is not fetched
before somebody presses play. Optionally with a poster image and a caption.

No YouTube and no Vimeo: an embedded frame is exactly what the rule "nothing from
third parties at runtime" forbids — and the reason these sites need no cookie
banner. A file of your own needs neither.

## Templates

1. **Templates** in the sidebar → upload a `.zip` containing at least
   `layout.html` and `page.html`
2. Optionally `home.html`, `404.html`, `list.html` and an `assets/` directory
3. Activate the template for a website under **Design**
4. The public site uses it immediately, without a restart

`layout.html` is the document and is what gets rendered. Each view file supplies
the `{{define "content"}}` block that `layout.html` pulls in, so a view is parsed
together with the layout into its own template set. Eight templates are built in
and always available; anything an archive leaves out falls back to them.

An uploaded template must follow the same rule as the rest of the program: bundle
CSS, images and fonts into the archive and reference them with relative or
root-relative paths (`/t/style.css`, `/t/fonts/inter.woff2`). An archive that
references another origin is refused, with the offending URL named in the error.
Ordinary outbound hyperlinks are content, not subresources, and remain allowed —
as do `rel="canonical"` and `rel="alternate"`.

> The bundled templates' own wording is German throughout; they contain no
> translation calls. A site in another language shows its own content in that
> language and the template's fixed words in German. A template of your own is
> the way around it.

## Per-site design

Between "pick one of eight templates" — too coarse, because everyone wants their
own colour — and "upload your own template" — too much, because it means
maintaining a copy of a theme through every future change — sits a handful of
values that every bundled template picks up: text colour, background, accent,
typeface, text width, corner rounding.

Each is validated to a shape the CSS can read only one way: a hex colour, a
typeface from a fixed list, or an integer in range. A value that does not fit is
dropped and the template answers for it, rather than the whole form being refused
over one mistyped colour. The typeface list is fixed for a second reason: a name
typed by hand is either not installed — silently falling back to something else —
or a URL, which would load a font from another server.

## SEO and structured data

Every website automatically serves, under each of its domains:

- `/sitemap.xml` — the homepage plus every published page with a `lastmod` date.
  Drafts are excluded by the query, so an unpublished page is not disclosed here
  either.
- `/robots.txt` — allows the public site, disallows `/admin/`, points at the
  sitemap.

Absolute URLs use the request's `Host` and the scheme implied by
`HOLZCLOUD_SECURE`. A forwarded scheme header is deliberately ignored: these URLs
go to search engines, so a client must not be able to influence them.

Every page carries a schema.org graph as JSON-LD: the organisation behind the
site, the page or article itself, and a breadcrumb trail. That is the difference
between a search result with an address, opening hours and a telephone number and
one with a blue link. The fields are the ones that belong in the imprint anyway;
the settings screen only asks for them in a form a search engine understands
rather than guesses at. Leaving the business type empty emits no organisation
block at all, because a personal blog should not claim to be a shop.

It is built in Go rather than left to the template. A template author who gets one
field wrong produces a result that looks perfectly fine and is silently ignored,
and there is nothing on the site itself to show it.

## Plugins

A plugin is its own Go module and need not live in this repository — that is the
point: if you need something that does not exist, you write it without forking
the CMS. Five are bundled as examples:

| Directory | What it does | Hooks |
|---|---|---|
| `jahreszahl/` | replaces `[[jahr]]` in the text with the current year | `content`, `admin` |
| `nicht-gefunden/` | keeps the record of addresses asked for and not found | `event`, `admin` |
| `suche/` | the full-text search at `/suche` | `route` |
| `kontaktformular/` | `[[formular]]` becomes a form, `/formular` receives it | `content`, `route`, `admin` |
| `bestellung/` | the farm shop: `[[bestellung]]` becomes a product list with quantity fields | `content`, `route`, `admin` |

The last three used to be in the core. They moved out because a website does not
need them in order to be a website — and what you do not need you should not have
to carry. Install them and they are there; leave them out and they are nowhere:
no route, no table, no screen, no call.

Redirects, labels and export/import look like add-ons and deliberately stayed in
the core; `plugins/README.md` says why.

### The farm shop

An order form over your own products, as a plugin. It brings no product model
with it: **a product is a page that has a price field filled in.** Which field
that is is set on the plugin's own screen — pre-filled with `preis`, `einheit`
and `verfuegbarkeit`.

`[[bestellung]]` on a page becomes the product list with quantity fields, plus
fields for name, e-mail, telephone, address and a remark. The order goes to
`/bestellung`, lands in the plugin's own storage and triggers an e-mail to the
notification address — with the orderer's address as the reply-to, so a click on
*Reply* is enough.

Three decisions that differ from an ordinary shop:

- **No basket.** Somebody ordering from a farm picks once and sends. A basket
  would need a session per visitor, a cookie and a second page — and all of it
  would have to be right before the first order arrived.
- **No payment.** A payment provider would be an outbound call at runtime, which
  is exactly what lets this CMS do without a cookie banner. You pay on collection
  or by invoice; the note about it is in the plugin's settings.
- **Prices come from the page, not from the form.** The form supplies the
  quantity and nothing else — otherwise the visitor would decide what something
  costs. Today's price is stored, so a change tomorrow does not rewrite an old
  order.

Against spam the same as the contact form: a honeypot, a signed timestamp (faster
than two seconds was not a human; older than twelve hours is a page with possibly
stale prices) and an hourly limit.
