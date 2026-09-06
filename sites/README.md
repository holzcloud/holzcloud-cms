# Websites as source

Here lies a finished website in the form you can read and change: the manifest as
JSON, the images as ordinary files. `tools/mkbundle` packs them into the zip
archive the admin's import expects.

A bundle is not checked in as a zip, although that is the form it is finally
needed in. A zip cannot be read in a diff, and one corrected line of text
rewrites thirty megabytes of binary. Unpacked, a typo is one line in the diff.

## What is here

`beispiel/` — the Velowerkstatt Beispiel in Musterhausen. It does not exist. The
address, the prices, the opening hours and both images are invented, and
`example.com` is a domain that by RFC 2606 never belongs to anybody.

It is here so that you can see the format in something complete rather than in a
list of fields. Every piece occurs exactly once: four published pages, a menu with
one sub-item, a snippet, two images with descriptions, a preview image and two
paths of the form `/media/0/`. To make a website of your own, copy the directory
and rewrite it.

Which fields exist at all is not documented here but in
`internal/bundle/format.go`. The JSON names on the structs are the only truth
about it, and `mkbundle` rejects every name that does not appear there. A list in
this place would be a second truth, and it would go stale the moment somebody
adds a field.

## Building and importing

```sh
go run ./tools/mkbundle sites/beispiel
```

That writes `sites/beispiel.zip`. The archive does not belong in the repository
and is in `.gitignore`. Then upload it in the admin under **Websites → Import
website**. The import creates a **new** website every time; it merges nothing. To
replace a version, import again, compare, and delete the old one.

After the import the domain is still missing: an imported website has none, and
without a domain it cannot be reached. Enter one under **Domains**. The domains
are deliberately not in the manifest — a bundle should be able to land elsewhere
without claiming a host name that does not belong to that machine.

The template has to be set by hand too. It is not in the manifest, because a
bundle can land on a machine that does not have that template at all.

## What `mkbundle` checks

It refuses to build an archive that would silently lose content on import:

- **Unknown fields.** The import is deliberately forgiving — it takes what it
  knows, so an old bundle still lands. Here the opposite applies: `meta_desc`
  instead of `meta_description` would cost the website every description, without
  a single error message.
- **Images in both directions.** Every file in `media/` is in the manifest, and
  every entry in the manifest lies in `media/`. An image that is not listed does
  not travel; the page that shows it stays empty on the target machine.
- **Missing alt texts.** An image description can be added later, but nobody does
  it. Build time is the moment when there is still somebody who has the image in
  front of them.
- **Characters that lose an image description entirely.** A colon, a question
  mark, an em dash, an ampersand — with each of those the sanitiser removes not
  the character but the whole attribute. The image still appears, the page looks
  finished, and to a screen reader it is mute. Comma and full stop are allowed and
  are enough.
- **References into nothing.** Menu items pointing at pages that do not exist.
  Preview images that are in no media list. `/media/…` paths in the text with no
  file behind them.

It writes the checksums itself while packing. A checked-in checksum would be a
second copy of the truth, going stale the moment somebody re-crops a photograph —
and then the import fails with a message about a corruption that does not exist.
The same goes for `version`, `exported_at` and `generated_by`: they are not
written by hand.

## Images in the text

Images are written as `/media/0/filename.jpg`. The zero is a placeholder: which
number the website gets is decided only at import, and the import rewrites the
paths to the right one. That applies to any number in that position, including
that of a real website — which is exactly what makes an export from one server
work again on another.

Only files that are inside the bundle are rewritten. A reference to anything else
is left alone.

## The two images in the example

They show nothing. They are two flat JPEGs, 1200 by 800, with a few rectangles on
them so you can tell them apart. A throwaway program made them out of `image`,
`image/color`, `image/draw` and `image/jpeg` — standard library only, which is
why they are JFIF without an EXIF block and give nothing away about the machine
they came from.

The program is not checked in. It would have exactly one task that never repeats,
and a second run would silently rewrite the bytes and with them every checksum.
An image is content, not a build output — nobody regenerates a photograph either.
Anyone who does have to replace the two writes the program again in ten minutes.

## Changing things

Text and structure are in `holzcloud.json`. Change something there and you build
again and import again — there is deliberately no way to pull a running website
back into line with this file. That would be a second source of truth beside the
admin, and the editorial changes of the last few weeks would be silently
overwritten.

After the first import the admin is the source. These files are the starting
point, and the archive you can start again from.

## Where the real websites are

Until the source was released, two customer websites stood here, with their
photographs, their texts, an e-mail address and a postal address. That is not
this project's material, and this repository is under the AGPL-3.0 — so
everything in it is published along with it.

Both have therefore moved to the private repository
`holzcloud/holzcloud-sites`. That is where they belong and where they go on being
maintained. What stays here is the format, and one example of it.
