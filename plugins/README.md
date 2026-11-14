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
| `assets/…` | no | files served under `/plugin-assets/<key>/` |
| `migrations/*.sql` | no | tables of its own, applied once |

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
| `find` | `Email` | `Records`: every record that carries the address, in any letter case, with its fields, the fields `correct` accepts and the media ids of its files |
| `erase` | `Keys` (from its own `find`), `RetainUntil` | `Deleted` and `MediaIDs`; a record the law says to keep is reduced instead and listed in `Kept` with its fields and `Until` |
| `correct` | `Key`, `Field`, `Value` | the record as it now stands |
| `expire` | `Kind`, `Before`, `Limit` | `Deleted`, `MediaIDs`, and `More` when it stopped at the limit |

`find` is read-only: the host refuses every write during it and reports a plugin
that tried one as not searched. `erase` refuses a key that is not one of the
plugin's own records before it deletes anything. Media ids are a claim the host
checks against the website before it deletes a file. Each call has the same two
seconds as every hook, which is why `expire` works in batches.

A plugin that keeps data for a website and does not declare `privacy` is named on
the Privacy screen as not searched, with how many entries it holds — the
operator then has to look through it by hand. `kontaktformular` answers it.

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

## The size

A Go plugin weighs about three megabytes, almost entirely Go runtime. That is the
price of an author not needing a second toolchain. With TinyGo it would be about
150 KB; the SDK works there just as well but requires an extra installation.
