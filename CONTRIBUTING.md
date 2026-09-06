# Contributing

Holzcloud CMS is a CMS for a small server on a shelf or in a cluster. What is
built here should run smoothly on the cheapest node and keep going for years
without attention. That is not a limitation mentioned in passing — it is the
decision most of the others follow from.

## What you need

Go 1.26 or newer. Nothing else.

```bash
go build ./cmd/holzcloud
go test ./...
./holzcloud
# open http://localhost:8080/admin — the first account is created there
```

No npm, no bundler, no database to set up: SQLite comes into being on the first
start, and the migrations run by themselves.

## The hard limits

These four rules are not negotiable. A proposal that breaks one of them is
refused however good it is otherwise.

**No JavaScript except htmx.** htmx 2.0 is in the repository and served from our
own server. Nothing else. Every page must be fully operable without JavaScript —
htmx makes it pleasanter, not usable.

**Nothing is loaded at runtime.** No script, no stylesheet, no font, no image
from a foreign server. Everything the browser asks for comes from our own origin.
At build time two things may be fetched: Go modules and fonts — and a font is put
into the repository and shipped through `embed.FS`, never referenced by address.

Enforced in three places, all of which have to work: `internal/web/headers.go`
(the content security policy), `internal/tmplmgr/external.go` (foreign sources in
uploaded templates) and `internal/tmplmgr/script.go` (scripts inside them).

**One program, no side dishes.** Templates, assets and migrations sit inside the
program through `embed.FS`. A file that has to lie beside it at runtime is a file
that gets forgotten on the next update.

**No CGO.** SQLite comes through `modernc.org/sqlite`, so a static program can be
built without a C toolchain.

## What code looks like here

**Comments say why, not what.** The code already says what it does. What it
cannot say is which bug made this line necessary. Comments like that are all over
the project, and they are the reason you still understand half a year later why
something is the way it is and not otherwise.

**Tests that bite.** A test that stays green when you break the line it checks
checks nothing. Break it and look. Several tests in this project only came about
because the first draft failed to notice a deliberately introduced fault.

**Migrations are never changed once they have been applied.** New column, new
file. A changed migration does not run again on an existing installation — the
column is simply missing there.

**Errors are shown, not swallowed.** An empty selection, a silently skipped
template, a discarded return value: those are the faults that make it into
production. Better a start that aborts.

## Who wrote this code

A large part of this project was written by an AI agent, under the direction and
review of a single person. You should read that here rather than have to work it
out.

It cannot be counted here. The commits carrying the agent as author are in the
private repository this one was published from; in public the record begins at
`v1.4` — the "Versioning" section in the README says why. So what stands here is
the statement and no evidence for it. That is exactly why it stands here at all.

It changes nothing about the standard. What is written two sections above holds:
comments say why, and a test that does not notice a deliberately introduced fault
checks nothing. A model does not keep to that by itself — that is the part the
review does.

Your contributions may equally be made with a model. Whoever opens a pull request
vouches that it does what it claims, and that the rights to it can be transferred
as described under "Licence". A contribution nobody read before sending shows.

## Templates

Building a template needs no contribution to the code. The complete description
of the data contract is in `internal/tmplspec/TEMPLATE-SPEC.md`, and the program
prints it itself:

```bash
./holzcloud template spec                    # the description
./holzcloud template check ./my-template     # check, without installing anything
```

The description is explicitly meant for AI agents too: it is written so a model
can follow it to the letter. Tests bind it to the code — a field that is in the
contract but not in the description makes the test suite fail.

## Before you open a pull request

```bash
gofmt -l internal/ cmd/     # must be empty
go vet ./...
go test ./...
for t in default holzcloud journal magazine midnight rudel schlicht weide; do
    go run ./cmd/holzcloud template check cmd/holzcloud/templates/public/$t
done
```

For a larger change: please open an issue first. It is more pleasant for
everybody to agree on the direction before somebody has spent an evening on it.

## Language

**German is the source language of everything a user sees.** The admin is
translated from it into English, French, Italian and Spanish, plus Swiss variants
of the three national languages — see
[`docs/multilingual.md`](docs/multilingual.md). The German sentence is the
translation key, so a new string is written in German and picked up by
`go run ./tools/i18n`. The bundled public templates carry their own fixed wording,
and that is German.

**English is the language of everything a developer sees**: this file, the README,
the documents under `docs/`, and the template specification — the last because it
addresses an international audience and language models.

Comments in the code: both occur. Write in whichever language lets you be more
precise.

## Licence

By contributing you place your contribution under the
[GNU AGPL-3.0](LICENSE), the licence the rest of the project is under.
