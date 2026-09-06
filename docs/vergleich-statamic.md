# Holzcloud beside Statamic

Statamic is a full-grown CMS with over ten years of development, a team behind it
and a market for extensions. Holzcloud is a single Go file on a small server. The
comparison is useful all the same — not in order to catch up, but to see which
gaps **hurt** and which are deliberate decisions.

As of August 2026, Statamic v6. The figures below are that snapshot and have not
been carried forward.

---

## The one structural gap — since closed

When this paper was written, the difference in construction stood here:

> **Statamic lets the operator determine their own content model. Holzcloud has a
> fixed one.**

In Statamic you create *blueprints*: "A product has a name, a price, an image, an
availability and three variants." Over 40 field types are available for that. In
Holzcloud a page had a title, an address, content, an excerpt, a preview image,
labels and an access setting — and anyone who needed a field "price" needed a new
Go version.

That was the reason we first built the farm shop as Markdown text and not as a
product list.

That gap is closed: **fields of your own** per website (point 1), **repeatable
groups** (point 2), **content kinds of your own** (point 6) and **sections and
conditions** in the form (point 7). Anyone keeping products creates the kind
*Product*, hangs its fields on it, divides them up with headings, has the offer
price appear only once the offer is ticked — and gets an admin that speaks of
products. With point 8 the same holds one level down: the editor's blocks are no
longer a fixed list either.

---

## Field types: 47 against 15

Statamic has 47 field types. We now have eleven a website can assign itself — nine
kinds of input, the repeatable group and the heading (see the README, *Fields of
your own*) — plus the fixed inputs of the remaining screens:

| Where | Inputs |
|---|---|
| Page editor | text, address, Markdown, long text, image picker, yes/no, choice, date+time, labels, password |
| Block editor | 9 block kinds with their own fields each (Markdown, image, alt, caption, width/variant, title, text, source, link text, link target, repeatable sub-items) |
| Form plugin | short answer, long answer, e-mail, telephone, number, date, choice, checkbox |
| Website settings | colour, typeface, number, text, e-mail, choice, yes/no |

The difference is not the number. It is that our inputs are **not combinable**:
the form plugin can do eight kinds, but only for forms; the block editor can do
images, but only inside blocks.

### Which individual field types we lack

Ordered by usefulness to this project, not by Statamic's order:

| Statamic | Do we have it? | Note |
|---|---|---|
| `replicator`, `grid`, `group`, `array`, `list`, `table` | **yes, one kind** | the group: repeatable rows of fields, one level deep |
| `entries` | **yes** | the kind "reference": a page of this website, picked rather than typed |
| `terms`, `taxonomies`, `collections` | **partly** | labels exist; they cannot be picked as a *field* |
| `link` | **yes** | the kind "link": an internal address, a foreign address or a mail address |
| `code`, `html`, `yaml` | **no** | rarely for editors, sometimes for developers |
| `integer`, `float`, `range`, `time` | **partly** | numbers only in settings, time only in scheduling |
| `radio`, `checkboxes`, `button_group` | **no** | choice exists, but only as a dropdown |
| `video` | **as a block** | your own MP4 in a `<video>`; `icon` and `dictionary` are missing |
| `assets` | **yes** | media picker with preview |
| `markdown`, `textarea`, `text`, `slug`, `toggle`, `select`, `date`, `color` | **yes** | |
| `bard` (block editor) | **yes, our own** | 9 built-in kinds plus the ones a website creates itself |
| `users`, `user_groups`, `user_roles`, `sites`, `structures`, `navs`, `form`, `template` | **no** | need a field system first to make sense |
| `section`, `revealer` | **yes** | the kind "section": a heading between the fields, and the condition "only show when …" |
| `hidden`, `spacer`, `width` | **no** | cosmetics of form layout |

---

## Features: side by side

### What we have

| Statamic | Holzcloud |
|---|---|
| Revisions & content history | ✅ versions with restore |
| Asset manager | ✅ media per website, with a required alt-text check |
| Focal point editor | ✅ image crop **and** focal point |
| Dynamic image manipulation | ✅ crop, rotation, responsive sizes |
| Block-based editor (Bard) | ✅ block editor, 9 built-in kinds plus a website's own |
| Live preview | ✅ preview in the editor, plus a full preview in the theme |
| Filter & search | ✅ search, status, kind and sort filters |
| Collections | ✅ a website's own content kinds, with a listing page and their own fields |
| Drag & drop nav builder | ⚠️ menus yes, reordering with arrow buttons (no JS) |
| Forms | ✅ as a plugin, with a form builder of its own |
| Globals | ⚠️ snippets — text yes, other types no |
| Content protection | ✅ page behind a password, whole website offline |
| Two-factor authentication | ✅ TOTP, **compulsory** for administrators |
| User management | ✅ two roles plus rights per person (websites, publishing); no groups |
| Dark mode | ✅ as of today |
| Importer | ✅ our own format and WordPress (WXR); CSV no |
| Content API | ✅ as MCP at `/ai` — read and write |
| Command line tools | ✅ users, backup, migration, integrity |
| Helpful utilities | ✅ e-mail test, integrity check, wastebasket |
| Simple Commerce (addon) | ✅ as a plugin: products from your own fields, an order form, orders |
| Static site generator | ❌ |
| Multi-site | ✅ several websites with several domains, plus translations of one website (main language without a prefix) |

### What Statamic has and we do not

| | What it is | Worth it for us? |
|---|---|---|
| **Blueprints** | your own content model with field types | **built** — own fields, groups, content kinds, sections, conditions and block kinds |
| **Multilingual** | the same page in several languages | **built** — main language without a prefix, further ones under `/fr/…` |
| **Translated admin** | ~30 languages | **built** — five languages bundled plus the Swiss variant of the three national languages, per person; further ones as a file while running |
| **Roles and rights** | roles, groups, individual permissions | **built** — two roles plus two rights per person: which websites, and publish yes/no. No groups |
| **Custom columns in lists** | choose the visible columns | **built** — per person, six columns to tick |
| **Saved filters** | save and reuse views | **built** — per person and website, as chips above the list |
| **Elevated sessions** | ask for the password again before sensitive actions | **built** — in front of four buttons, valid for 15 minutes |
| **Importer (WordPress, CSV)** | moving in from elsewhere | **built** — WXR with pages, posts and labels; images deliberately stay behind. Our own format carries everything: languages, fields, content and block kinds, blocks, media |
| **Static export** | a website as plain HTML files | medium — on a small node quite appealing |
| **Whitelabel** | your own logo in the admin | **built** — name, mark and logo under *Brand* |
| **Full-screen mode** | writing without distraction | **built** — `?vollbild=1`, pure CSS |
| **Command palette (⌘K)** | keyboard access to everything | **does not fit** — needs JavaScript |
| **Passkeys** | signing in without a password | **does not fit** — needs JavaScript |
| **Video embeds** | embedding YouTube/Vimeo by address | **does not fit** — precisely what the rule "nothing from third parties at runtime" forbids |
| **GraphQL** | headless interface | does not fit — the rule is a server that serves pages |
| **OAuth** | signing in through Google, GitHub … | does not fit — every sign-in would be a call outwards |
| **Git automation** | every change as a commit | does not fit — we are a database, not files |
| **Laravel ecosystem** | a whole framework underneath | does not fit — and does not want to |

### What we have and Statamic does not

For completeness, because a comparison is otherwise lopsided:

- **Plugins run in a sandbox.** A Statamic addon is PHP with all the server's
  rights. Our plugins are WebAssembly: no file system, no network, only the host
  functions named in the manifest — and an endless loop is aborted after two
  seconds.
- **Nothing is fetched from third parties at runtime.** Not as a recommendation
  but enforced: a content security policy on every response, and theme archives
  are refused at upload if they load a foreign address.
- **One file.** No PHP, no Composer, no Node, no web server needed in front.
  `scp` and `systemctl restart`.
- **AI connection built in.** MCP with its own key management and read and write
  rights per website.
- **Two-factor is compulsory**, not optional.
- **Design values per website** without changing theme.

---

## Proposal: the next steps

Ordered by effect, with what I would actually build.

### 1. Fields of your own per website ("field sets") — **built**

The foundation. Not Statamic's full blueprints — a smaller version that does 90 %
of it:

- Per website a list of additional fields for pages: key, label, kind, required
  yes/no, hint.
- Eight kinds to begin with: **text, long text, number, date, yes/no, choice,
  image, link**. Exactly the ones the form builder can already do — the code for
  them is written and has proved itself.
- Stored as JSON on the page, like the blocks. No schema rebuild per field.
- Reachable in the theme as `{{ .Felder.preis }}`.
- Optionally bound to the kind: fields for posts only, for pages only, or for a
  third kind of your own.

**Why first:** without it, every new content form stays a change to the core. With
it, the shop becomes an application rather than an extension.

*Done.* Eight kinds, definable per website, in the editor below the content, in
the theme as `.Page.Felder.key` and as `.Page.Feldliste`, validated in the admin,
on import and over AI access, and included in export and import. See the README,
section *Fields of your own*.

### 2. Repeatable field groups — **built**

The second part of the same thought: a group of fields you fill in more than once.
Opening hours, team members, product variants, price tiers.

*Done.* The kind "group", one level deep, with rows to add, remove and move —
all ordinary submit buttons, without JavaScript. In the theme as
`{{range .Page.Felder.preisstaffel}}`; included in export, import and AI access.
With that our field system covers what Statamic has `replicator`, `grid`, `group`
and `table` for.

### 3. A shop on that foundation — **built**

A product is a page with fields: price, unit, availability, image.

*Done* as the plugin `plugins/bestellung`. It managed without a basket: somebody
ordering from a farm picks once and sends, and a basket would have needed a
session per visitor, a cookie and a second page. Payment stays outside — a payment
provider would be a call outwards at runtime. See the README, section *Farm shop*.

### 4. Multilingual — **built**

A page in several languages, connected by a shared key, with `hreflang` in the
head and a language switcher in the theme.

*Done.* The load-bearing decision: the main language keeps its addresses, and
every further one lives under its code (`/fr/contact`). So no existing link breaks
when somebody switches on a second language, and a website with one language
notices nothing of the whole business.

The versions form a star rather than a chain — all of them point at the same page
in the main language — so that the order of creation is not built in permanently
and deleting a middle language does not tear the group apart. *Create version*
copies the page including blocks and own fields as a draft into the other
language. See the README, section *Multilingual*.

Unlike in Statamic, addresses are unique per website across all languages: the
constraint has been at the head of the `pages` table since the first version, and
relaxing it would mean rebuilding a table with foreign-key children in a
migration. The price for it is small — a French page is called something different
from the German one anyway.

### 5. A translated admin — **built**

Our admin was fixed German — every text stood in the template. For a CMS other
people are meant to host, that was a hard limit.

*Done.* The load-bearing decision is a different one from Statamic's (and from
most): **the German sentence is the key.** A template writes
`{{t "Seite gespeichert"}}`, not `{{t "page.saved"}}`. Two things follow, and both
are worth more than a tidy key scheme:

- A missing translation falls back to German — to a sentence somebody can read,
  rather than to an identifier, which is an error on screen.
- The templates stay readable. You can see what a screen says without looking
  anything up. That is the difference between a translation that is maintained and
  one that rots.

1011 strings, complete in **five languages**: German, English, French, Italian,
Spanish — plus the **Swiss variant** of the three national languages (`de-CH`,
`fr-CH`, `it-CH`), which carries only its deviations and takes everything else
from the base language. A Swiss browser gets it by itself. The language belongs to
the person, not to the website: under *My account* everyone picks their own;
without a choice `Accept-Language` decides — on the login screen too, where an
unreadable language would be worst.

And further than Statamic in one place: **a language is a file you drop in while
the thing is running.** `data/sprachen/nl.json` — upload it or copy it in, press
*Reload*, done; no rebuild, no restart. The same mechanism as with templates: the
disk wins over what is built in, so a `de.json` with ten lines corrects ten of our
own wordings too. It is checked before being stored — placeholders have to match,
markup is filtered, and what is missing appears in German. See the README,
*Language of the admin*.

### 6. Content kinds of your own — **built**

The last part of the thought this list began with. Your own fields say *what* is
on a page; your own content kind says *what sort of thing* the page is. Statamic
calls that *collections*.

*Done.* Under *Content kinds* a website creates its own — product, event, recipe,
animal — each with a singular, a plural, a key for the theme, a listing address
and a sort order. After that:

- The editor offers the kind for selection, beside *page* and *post*.
- A field applies either to all, only to pages, only to posts, or **only to one
  kind** — the price field is on the product and nowhere else.
- The list filters by it and shows the kind as a column.
- The listing address publicly serves a page listing all entries of the kind
  (`/hofladen`), and it is in the sitemap.
- Export, import and AI access carry the kind.

Two decisions are worth mentioning. The kind is a **column `art` of its own** and
not a third value in `kind`: `kind` carries a CHECK constraint in a STRICT table,
and `pages` has foreign-key children — the value could only be extended by
rebuilding the table completely, and that is exactly what nearly cost us the menus
in migration 00031. And the addresses stay as they are: a product lives at
`/wollpaket-gross`, not at `/produkte/wollpaket-gross`. Assigning a kind or
removing one therefore breaks not a single link.

### 7. Sections and conditions in the form — **built**

Statamic's blueprints divide a form into *sections* and let a field appear and
disappear through *conditions*. Both were missing, and both are what turns twenty
fields in a row into a form somebody fills in.

*Done.* A **section** is a field kind without an input — a heading, under which
everything stands until the next section. A **condition** hangs one field on
another: "offer price" appears only once "on offer" is ticked. While it does not
appear it is not required (not even as a required field), the theme does not
output it — and its value stays stored anyway, so a checkbox unticked by accident
costs nobody their input.

The interesting part is what the rule "no JavaScript" determined here. Showing and
hiding in the browser is a handful of CSS rules, and those need two things: the
dependent fields stand in the HTML **beside** the field they hang on — a
stylesheet reaches there and nowhere else — and the condition only checks what a
browser can read without a script, namely **filled in or not**.

Checking for a particular value — Statamic can do that — would mean generating a
stylesheet per website: a second place where the same rule lives, and the two
would drift apart sooner or later. Doing without costs little: almost every
condition in an editorial form is a yes/no.

### 8. Block kinds of your own — **built**

The last sentence from this paper's heading, one level down: the block editor
could do nine kinds, and those stood in the Go code. A tenth — a recipe step, an
opening-hours box, a price line — was a new version of the program.

*Done.* Under *Block kinds* a website creates its own; what one is made of it
decides with **the same fields as a page**. That is exactly why it needed little
new code: the image picker, the dropdown, the date field and their validation have
existed since point 1, and the fields of a block kind live in the same table as
those of a page — with one reference more and two swapped indexes.

Two decisions explain the rest:

**The core writes the markup, the theme does the looks.** A block of your own
becomes `<div class="hc-block hc-eigen hc-eigen--rezeptschritt">` with one class
per filled-in field. Statamic lets the operator write a template alongside; here
that would be a templating language in a text box, and therefore a way to bring a
`<script>` onto a page through the front door — and this program rests on the
promise that no such way exists. Where a class is not enough there is the **long
text field**: it goes through the Markdown renderer and the same sanitising as any
other text, so an editor writes `### Step 3` and gets a heading.

**No reference inside a block.** A reference exists in order to survive a rename;
a block is turned into HTML once and for all when the page is saved and could not
keep that promise. Better not to offer it at all than to make an undertaking that
breaks silently — the link does the same work and says what it is.

### 9. Small things that give a lot

- **Custom columns** in the page list (what you see while paging through).
- **Saved filters** — "my drafts", "for review".
- **Asking for the password again** before deleting a website or issuing an AI
  key.
- **Full-screen mode** in the editor — a checkbox and three CSS rules.
- **WordPress import** — WXR is XML; that is an afternoon's work for pages and
  posts, longer for media.
- **A video block for your own files** — *built*: an uploaded MP4 in `<video>`,
  without YouTube and without metadata in the file.

### 10. What I would deliberately not build

- **A command palette and passkeys** — both need JavaScript, which we do not want.
  The gain does not outweigh the exception.
- **YouTube embedding** — the rule "nothing from third parties at runtime" is the
  reason this CMS does without a cookie banner. An embed code costs exactly that.
- **GraphQL, OAuth, git automation, static export** — each of them is a second mode
  of operation beside the one that works.

---

## In summary

We are closer than the number 47 suggests: versions, media, image cropping, the
block editor, preview, filters, protection, two-factor, forms, import/export and a
write interface are all there. The one thing that was missing at the start — that
the operator can decide for themselves what their content is made of — is built:
own fields, repeatable groups, references between pages, own content kinds,
sections and conditions. The farm shop is therefore no longer special treatment in
the core but an application of what is already there.

With that the gap this paper began with is closed on both levels: at the page and
at the block. What remains is no longer a matter of construction but a list of
individual things — choice as a button row rather than only a dropdown, labels as
a field kind, snippets with types other than text, CSV import, static export. Each
of them is an afternoon; none of them changes how the program is built.
