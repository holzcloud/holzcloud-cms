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
cd plugins/jahreszahl
GOOS=wasip1 GOARCH=wasm go build -buildmode=c-shared -trimpath -buildvcs=false -ldflags="-s -w" -o plugin.wasm .
zip -j jahreszahl.zip plugin.json plugin.wasm
```

`-buildvcs=false` is not a nicety but a condition. Without it, `go build` writes
the git state of the moment into the module; a second build at a later commit then
produces different bytes although not a line of source has changed, and the
comparison in the check pipeline can never come out right again.

If you would rather not keep the flag in your head: `go run ./tools/wasm` builds
all six bundled modules with the pinned Go version and packs the four archives
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

Use `notfound` rather than `request` whenever you can: `request` runs on every
page view, `notfound` only on a miss. A redirect table belongs on the second hook,
not the first.

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
