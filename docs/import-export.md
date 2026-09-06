# Import and export

- [A website as a bundle](#a-website-as-a-bundle)
- [What travels and what does not](#what-travels-and-what-does-not)
- [Websites as source in the repository](#websites-as-source-in-the-repository)
- [Moving from WordPress](#moving-from-wordpress)

## A website as a bundle

*Settings → Download website* writes one website as a zip: a JSON manifest plus
the media files. It exists so that a site is not a hostage to the machine it runs
on — a database file is a backup of the whole installation, this is one website,
in a form somebody can read, diff and repair in a text editor when everything
else has gone wrong.

Import always creates a **new** website rather than merging. Merging would need
an answer for every collision — same address, different text — and the honest
answer at this size is a second site the operator can compare and then delete.

Everything read out of the archive goes through the same validators as the forms:
a bundle is a file anyone can edit, so it is exactly as untrusted as a text field.

## What travels and what does not

Travelling: settings **including the site's further languages**, design tokens,
pages and posts as **Markdown**, pages built from **blocks** with the blocks
themselves, the website's own **fields**, **content kinds** and **block kinds**,
menus (nested, pointing at pages by slug, one per language), snippets, labels and
every media file with its description and a SHA-256.

Anything that names a picture or a page names it the way the other machine can
read: a **file name**, never an id — in the preview image, in a picture field,
inside a block, and inside a block's own fields. An id means nothing on the
machine a bundle lands on, and one that was silently kept would point at somebody
else's photo.

Deliberately not travelling:

- **Domains.** A bundle is meant to land somewhere else; carrying the host names
  would either collide with the site already serving them or quietly claim a
  domain the new machine does not own. An imported site therefore has none, and
  the report says so.
- **Passwords.** Neither account passwords nor page passwords. A hash in a file
  that gets e-mailed around is a hash somebody can attack offline. A page that was
  protected arrives unprotected — and the report names it, rather than leaving a
  price list publicly readable with nobody told.
- **Rendered HTML.** Only the Markdown and the blocks travel; the HTML is derived,
  it roughly doubles the archive, and a bundle written by an older renderer would
  otherwise import markup that no longer matches what this version produces. A
  page of blocks is therefore set again on the way in.

> A field is bound to a content kind through `applies_to`, which holds either
> `beides`, `seite`, `beitrag` or the key of one of the website's own kinds. The
> manifest struct also declares a `content_type` on a field; nothing writes or
> reads it, so a hand-written manifest that uses it loses the binding silently.

## Websites as source in the repository

`sites/` holds complete websites in the form you can read and change: the
manifest as JSON, the images as ordinary files. A bundle is not checked in as a
zip, because a zip cannot be read in a diff and one corrected line of text
rewrites thirty megabytes of binary. Unpacked, a typo is one line in the diff.

```bash
go run ./tools/mkbundle sites/beispiel
```

That writes `sites/beispiel.zip`, which the admin's **Websites → Import website**
accepts. The archive does not belong in the repository and is in `.gitignore`.

`mkbundle` also checks the things a hand-written manifest gets wrong, and refuses
rather than letting an import silently drop a mistyped field. Which fields exist
at all is not documented twice: the JSON tags on the structs in
`internal/bundle/format.go` are the only truth about it, and `mkbundle` rejects
every name that does not appear there.

## Moving from WordPress

Whether somebody arrives at all is decided before the first login. Under
*Websites → Move from WordPress* the CMS accepts a WXR file (in WordPress:
*Tools → Export → All content*):

- pages and posts with title, address, text, excerpt, status, date and labels
- shortcodes (`[gallery …]`) are removed rather than guessed at
- everything that was not published arrives as a **draft**
- attachments, menu items and the wastebasket are skipped and counted
- it always creates a **new** website, as with your own import

**The images stay behind** — deliberately. They are on the old server, and this
one fetches nothing from third parties of its own accord. The report lists the
addresses; the files come over under *Media*. Twenty files by hand are better
than one broken rule.

The content arrives as HTML and is stored as the page's source. That is not a
compromise: Markdown lets HTML through, and everything goes through the same
filter as any other text — a `<script>` that WordPress carried along does not
survive the first render.
