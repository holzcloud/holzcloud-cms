# Plugins

What lies here are the bundled examples. A plugin is its own Go module and need
not live in this repository — that is exactly the point: if you need a feature
that does not exist, you write it without forking the CMS.

| Directory | What it does | Hooks |
|---|---|---|
| `jahreszahl/` | replaces `[[jahr]]` in the text with the current year | `content`, `admin` |
| `nicht-gefunden/` | keeps the record of addresses asked for and not found | `event`, `admin` |
| `suche/` | the full-text search at `/suche` | `route` |
| `kontaktformular/` | `[[formular]]` becomes a form, `/formular` receives it; further forms with `[[formular:key]]` | `content`, `route`, `admin` |
| `bestellung/` | the farm shop: `[[bestellung]]` becomes a product list with quantity fields, `/bestellung` receives it | `content`, `route`, `admin` |

The last three used to be in the core. They moved out because a website does not
need them in order to be a website — and what you do not need you should not have
to carry around. Install them and they are there; leave them out and they are
nowhere: no route, no table, no screen, no call.

Three things look like add-ons and deliberately stayed in the core:

- **Redirects** are written when a page is renamed, in the same transaction as
  the rename itself. A plugin only hears about it afterwards and could miss the
  message. That is not a feature but the protection against the CMS breaking its
  own addresses.
- **Labels** are part of how a page is written: the field is in the editor, the
  values reach every theme, the archive groups by them, the export carries them.
  A plugin reaches none of those places.
- **Export and import** would need write access to pages, media, labels, menus,
  snippets and settings — that is, to everything. A sandbox that hands out
  everything is not one.

The rule behind it: what gets moved out is what a website does not need in order
to be a website. Not what can technically be moved out.

The **farm shop** is the example of how far that carries: it brings no product
model with it but reads the website's own fields. A product is a page that has a
price field filled in — it has a picture, a description and an address anyway. The
plugin only contributes what nobody needs without orders: the form, the orders,
and the screen they sit on.

## Building a plugin

```sh
go run ./tools/wasm jahreszahl
```

That is the whole build, and it is the one to use. Written out by hand it is:

```sh
cd plugins/jahreszahl
GOOS=wasip1 GOARCH=wasm go build -buildmode=c-shared -trimpath -buildvcs=false -ldflags="-s -w" -o plugin.wasm .
```

**Do not pack the archive with `zip -j`.** It stamps the wall-clock time into
every entry, so two runs never produce the same bytes and the check below can
never come out right — and `-j` flattens, which silently drops a `migrations/`
directory. `kontaktformular` carries one, and losing it is the defect the
packer was rewritten to prevent.

`-buildvcs=false` is not a nicety but a condition. Without it, `go build` writes
the git state of the moment into the module; a second build at a later commit then
produces different bytes although not a line of source has changed, and the
comparison in the check pipeline can never come out right again.

If you would rather not keep the flag in your head: `go run ./tools/wasm` builds
all six bundled modules with the pinned Go version and packs the five archives
along with them; `go run ./tools/wasm -check` says whether what is in the
repository still matches the source beside it.

The zip is uploaded in the admin under *Plugins*. It contains:

| File | Required | For |
|---|---|---|
| `plugin.json` | yes | key, hooks, addresses, permissions |
| `plugin.wasm` | yes | the module |
| `assets/…` | no | stylesheets, images, fonts, JSON and text served under `/plugin-assets/<key>/` (see [Assets](#assets)) |
| `migrations/*.sql` | no | tables of its own, applied once (see [Migrations](#migrations)) |

## Handing over a file

An admin screen can return a file instead of HTML:

```go
return plugin.AdminOut{Download: &plugin.Download{
    Filename:    "nachrichten-2026-08-30.csv",
    ContentType: "text/csv",
    Body:        string(tabelle),
}}, nil
```

The host passes it on as an attachment and not for display. Allowed are
`text/csv`, `text/plain`, `text/markdown` and `application/json`; everything else
becomes `application/octet-stream`. No `text/html` — a plugin able to serve a page
under the admin's address could put a page there that looks like the admin and is
not. The file name is cut back to a plain name, and the size is capped at 8 MB.

## The admin screen and the publishing right

A plugin's screen is declared in the manifest's `admin` entry:

```json
"admin": {
  "label": "Orders",
  "per_website": true,
  "admin_only": false,
  "respects_read_only": true
}
```

`label` is what the sidebar says, `per_website` puts the screen under a website,
`admin_only` keeps editors out. `respects_read_only` is about the editors who
are let in but may not publish.

Without the publishing right, nobody changes what visitors see at once. The
host cannot tell which of a plugin's POSTs does that: the farm shop's settings
shape the public order form, ticking off an order changes nothing a visitor
sees, and both arrive at the same address. Only the plugin knows, so the plugin
decides — if it says it will:

- **With `"respects_read_only": true`** the host passes `read_only` in
  `AdminIn` (`in.ReadOnly` in the SDK) on every call, GET and POST, whenever the
  person on the screen may not publish. The plugin then refuses every write that
  changes the public site — answering with a sentence through `plugin.T` and
  `FlashError`, before anything is stored — and draws no control for it. Private
  work stays open: the shipped plugins still let such a person tick off and
  delete orders, release and discard messages, and clear the not-found log.
- **Without it** the plugin is never told. The host refuses every POST of such a
  person to the screen with its own sentence, before the plugin is called, and
  draws the screen with every control disabled. A GET still reaches the plugin,
  because reading stays open — but the call runs read-only: `plugin.Set`,
  `plugin.Delete` and keeping uploaded files answer with an error instead of
  writing. A GET that tried to write and carried a query (a "delete" link,
  `?aktion=…`) is answered with the host's sentence and leads back to the plain
  screen; the plain screen itself is still shown, with nothing written. A
  plugin that has never heard of the rule cannot be the way around it, neither
  by a form nor by a link.

The declaration is a promise, and the administrator who installs a plugin
decides whether to trust it. Both fields are optional and leave the interface
version at 1: a plugin built before them installs and runs as before.

## What a plugin may do

Only what is in the manifest. A hook that is not named is never called; a
permission that is not there is refused. That is not documentation but a check on
every call.

### Hooks

| Hook | When | SDK |
|---|---|---|
| `content` | before a page is served; may change the text | `OnContent` |
| `request` | before **every** public request is routed | `OnRequest` |
| `route` | for the addresses the manifest claims | `OnRoute` |
| `notfound` | only once the core has found nothing, before the 404 page | `OnNotFound` |
| `admin` | for its own screen in the admin | `OnAdmin` |
| `event` | after something has happened; changes nothing any more | `OnEvent` |
| `privacy` | when the operator searches, erases or corrects a person's data, and when the retention job runs | `OnPrivacy` |

Use `notfound` rather than `request` whenever you can: `request` runs on every
page view, `notfound` only on a miss. A redirect table belongs on the second hook,
not the first.

### The privacy hook

A plugin that stores what a visitor typed — a message, an order, a sign-up —
keeps personal data, and only the plugin knows the shape it stored it in. So the
host asks, through `privacy`, with one of four operations in `PrivacyIn.Op`:

| Operation | The host gives | The plugin answers |
|---|---|---|
| `find` | `Email` | `Records`: every record that carries the address, in any letter case, with its fields, the fields `correct` accepts and the media ids of its files; an erased record says `Erased` and the `RetainUntil` it was given |
| `erase` | `Keys` (from its own `find`), `RetainUntil` | `Deleted` and `MediaIDs`; a record the law says to keep is reduced instead and listed in `Kept` with its fields and `Until` |
| `correct` | `Key`, `Field`, `Value` | the record as it now stands |
| `expire` | `Kind`, `Before`, `Limit` | `Deleted`, `MediaIDs`, and `More` when it stopped at the limit |

`find` is read-only: the host refuses every write during it and reports a plugin
that tried one as not searched. A record's `Kind` is `form_submission` or
`order`, the two kinds the retention job asks plugins to expire; a `find` that
answers with any other kind is reported as not searched too, because such a
record would never expire. `erase` refuses a key that is not one of the
plugin's own records before it deletes anything. Media ids are a claim: the
host deletes only a file that `KeepFiles` stored for this plugin on this
website, and refuses and reports any other — another website's, the operator's
own, another plugin's. Each call has the same two
seconds as every hook, which is why `expire` works in batches.

Unlike the other hooks, an error your handler returns reaches the host: the SDK
puts it into the answer, and the host reports the plugin as not searched rather
than as having found nothing. Return what you already did together with the
error: an `erase` or `expire` that deleted records and then failed hands back
their `Deleted` and `MediaIDs`, and the host still counts them and deletes
their files — once a record is gone, nothing else would find its attachments
again. The `Records` of a failed call are dropped. A plugin that declares
`privacy` and registers no handler answers nothing, and that is reported the
same way.

A plugin that keeps data for a website and does not declare `privacy` is named on
the Privacy screen as not searched, with how many entries it holds — the
operator then has to look through it by hand. Both shipped plugins that keep
visitors' data answer it: `kontaktformular` deletes a message with its
attachments, and `bestellung` reduces an order to its invoice basis — name,
address, items, sum and the customer's note — until the date it was given.

### Permissions

| Permission | Allows | SDK |
|---|---|---|
| `store` | its own key/value store, separate per website | `Get`, `Set`, `Delete`, `List` |
| `pages:read` | read and search the **published** pages, optionally with the website's own fields | `Pages`, `Posts`, `GetPage`, `SearchPages`, `PagesWithFields` |
| `settings` | read the website's settings | `Site` |
| `render` | output a public page in the website's theme | `Render` |
| `notify` | send a notification to the operator — never to an address of its own choosing | `Notify` |
| `log` | write to the server log | `Log`, `Logf` |

A plugin never sees drafts — not with `pages:read` either, and not on its own
admin screen. That line is drawn by the host, not by the plugin.

`render` is the permission that puts a plugin on the public website: it supplies
the title and the body, while the header, menu, fonts and footer come from the
theme. What it supplies goes through the same filter as an editor's text — a
script does not survive it.

`notify` sends to the address in the website's settings and to no other. A plugin
allowed to name the recipient would be a mail relay with a web interface.

An admin screen may have several views: the plugin receives the query string and
can link to itself. Its HTML is filtered before it is shown — forms, tables and
select boxes survive that, anything executable does not.

A form on that screen posts back to the same address; the host inserts the session
token into every form after filtering. A plugin never gets to see it and does not
need it — it simply writes `<form method="POST">`.

`PagesWithFields` additionally returns a website's own fields, as they are stored:
an image as an id, a number as the text somebody typed. It has to be asked for
explicitly and is not always included — a list of a hundred pages with all their
extra values would be a second payload through a sandbox that has sixteen
megabytes in total.

A plugin runs in a sandbox. It cannot read the disk, cannot open a socket, and
reaches the database only through the host functions the SDK offers. An endless
loop is aborted after two seconds and a crash is caught — in both cases the
request carries on as though the plugin had said nothing, and the reason is shown
in the admin next to the plugin.

### What an answer may carry

A route, request or notfound hook answers a visitor on a website's own address,
and that address shares its session with the admin. So the host decides what
leaves, not the plugin.

- **Media type.** `text/html`, `text/plain`, `text/csv`, `text/calendar`,
  `text/xml`, `application/xml`, `application/json`, `application/rss+xml` and
  `application/atom+xml`, with their charset as given. An empty type means
  `text/html; charset=utf-8`. Anything else, `text/javascript`, `text/css`,
  `image/svg+xml` or a type that does not parse, is sent as
  `text/plain; charset=utf-8`. The type is compared as parsed, not as a prefix.
- **Headers.** `Cache-Control`, `Content-Disposition`, `Content-Language`,
  `ETag`, `Expires`, `Last-Modified`, `Retry-After`, `Vary` and `X-Robots-Tag`.
  Every other header is dropped, whatever its case: `Refresh` (it redirects
  without the `Location` check), `Link` (it preloads a script), `Set-Cookie`, the
  security policies and anything a browser may learn to obey later.
- **Status.** 200 to 599. Anything below 200 (a 103 would carry headers to the
  browser ahead of the answer) or above 599 becomes 200.
- **Location.** A path on the same site, as before; a redirect without a
  3xx status becomes 303.
- **Script.** Every answer carries a content policy with `script-src 'none'`
  on top of the one the site already has. A plugin's page cannot use
  JavaScript, neither inline, nor in an event attribute, nor from a file.
  Forms and links work as usual, a form still posts to the site itself.

### Assets

Files under `assets/` are served under `/plugin-assets/<key>/` on every host of
the installation, including the one the admin session lives on, so only files a
browser never executes are allowed: `.css`, `.png`, `.jpg`, `.jpeg`, `.gif`,
`.webp`, `.avif`, `.ico`, `.woff2`, `.woff`, `.json`, `.txt` and `.svg`. Each is
served with the content type of that extension and never sniffed; an `.svg` is
served as a download only. `.js`, `.mjs`, `.cjs`, `.wasm`, HTML (`.html`,
`.htm`, `.xhtml`), `.xml` and files without an extension are refused when the
archive is installed, with a message that names the file, and are never served,
even when an older installation still has one on disk.

### Migrations

A file under `migrations/` is applied once, in name order, inside a transaction.
SQLite itself checks what it does, at install, while it runs:

- **May create and change:** tables and indexes whose name starts with
  `plugin_<id>_` (the id with `-` written as `_`), and add rows to
  `plugin_store` with its own plugin id.
- **May read:** its own tables; `form_messages` through `id`, `website_id`,
  `created_at`, `name`, `email`, `subject`, `body`, `read_at` and `page_id`;
  `pages.id` and `pages.slug`. The schema tables (`sqlite_master`,
  `sqlite_sequence`) can be read, because SQLite itself reads them when it alters
  a table: the structure of the host's tables is public in the source and the
  counters are all that is learned there. That is a known and accepted limit.
  Changing them is checked after every statement: a new entry outside its prefix,
  a missing foreign entry or a changed foreign counter refuses the migration.
- **Refused:** every other table and column (`users`, `sessions`, `websites`,
  `pages.access_password` and so on), reading `plugin_store`, updating or
  deleting `plugin_store` rows, triggers, views, renaming a table out of the
  prefix, `PRAGMA`, `ATTACH`, `BEGIN`/`COMMIT`/`SAVEPOINT`, and a foreign key
  (`REFERENCES`) from one of its tables to a table that is not its own: a row
  pointing at `plugins`, `pages` or `form_messages` would keep the host from
  deleting the row it points at. Keys between its own tables are fine.
- **Names:** a plugin whose id would share a table prefix with an installed
  plugin's (`nicht` beside `nicht-gefunden`) is refused at install.
- **Removal:** removing a plugin drops the tables in its prefix together with
  its store, its website assignments and the record of its migrations, so a
  plugin installed later under the same id starts empty and runs its
  migrations again. An update (installing a new version over the old one)
  keeps all of it.
- **Time:** a migration has ten seconds. A refused, failed or timed-out
  migration changes nothing and does not stop the host from working.

### When a plugin is busy or failing

- A call runs to its own two-second bound even when the visitor has left, so a
  form that was submitted is not cut off by a closed tab. The bound starts when
  the call gets to run, not while it waits for another.
- Callers of one plugin wait at most two seconds, and at most 32 of them wait.
  Beyond that a route answers `503` with `Retry-After`; so does a visitor who
  asks while the plugin is held back.
- After three failed calls in a row (a trap, a timeout, an error) the plugin is
  held back for 30 seconds: its routes answer `503` with `Retry-After`, and the
  admin shows that it is held back and until when. The first call after that
  decides; a success ends it, a failure holds again for 30 seconds. A plugin
  that declines a request is not failing.
- **The host may start a plugin's module afresh at any time**: after a timeout,
  a trap, a restart or an update. A plugin therefore keeps nothing it needs in
  memory between calls and uses its store instead, as the five bundled ones do.

## The size

A Go plugin weighs about three megabytes, almost entirely Go runtime. That is the
price of an author not needing a second toolchain. With TinyGo it would be about
150 KB; the SDK works there just as well but requires an extra installation.
