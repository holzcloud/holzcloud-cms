---
phase: quick-260903-ceo
plan: 01
type: execute
wave: 1
depends_on: []
files_modified:
  # Stable scope — authorised, fixed, and known to exist (or known to be new).
  - sites/README.md
  - sites/beispiel/holzcloud.json
  - sites/beispiel/media/werkstatt-01.jpg
  - sites/beispiel/media/velo-01.jpg
  - tools/mkbundle/main.go
  - tools/mkbundle/pack_test.go
  - .gitignore
  # Deleted in task 2 — the two customer website directories under sites/,
  # with all of their media. `ls sites/` names them; this plan does not.
  # Expected scope for the identifier sweep, CONDITIONAL on the executor's own
  # grep at execution time (see <mutable_scope>, Finding C). The planner found
  # these eleven on 2026-09-03; the grep decides, not this list.
  - internal/mail/mail.go
  - internal/mail/mail_test.go
  - internal/web/layoutdata.go
  - internal/i18n/i18n_test.go
  - internal/plugin/store_test.go
  - internal/plugin/manager_test.go
  - internal/plugin/sdk_e2e_test.go
  - internal/public/suche_e2e_test.go
  - internal/public/formular_e2e_test.go
  - internal/admin/page_blocks_test.go
  - cmd/holzcloud/templates/public/weide/layout.html
autonomous: true
# OSS-01 is local to this quick task: item 1 of the open-source readiness audit
# run on 2026-09-03. It is not an ID in .planning/REQUIREMENTS.md.
requirements: [OSS-01]

estimate:
  tokens: 55000
  raw_tokens: 55000
  tasks: 3
  confidence: low

must_haves:
  truths:
    - "No file in the working tree outside .planning/ carries any of the five customer tokens — the case-insensitive grep in <verification> returns nothing"
    - "`go run ./tools/mkbundle sites/beispiel` exits 0 and writes sites/beispiel.zip"
    - "sites/beispiel.zip is ignored by git and never appears as an untracked file"
    - "The example bundle exercises each part of the format at least once — a page, a menu with a nested item, a snippet, a media entry, a featured image, and an inline /media/0/ path in a page body"
    - "Both example pictures are real decodable images whose bytes match the mime_type declared in the manifest — asserted by a test, not by eye"
    - "The example bundle stays valid as the format moves, because a committed test packs it on every `go test ./...`"
    - "sites/README.md explains what a bundle is, points at the example, and records that the two real websites moved to a private repository — naming neither"
    - "`go build ./...`, `go vet ./...` and `go test ./...` are green, 0 FAIL"
  artifacts:
    - "sites/beispiel/holzcloud.json — a hand-written manifest whose every key exists in internal/bundle/format.go"
    - "sites/beispiel/media/werkstatt-01.jpg and sites/beispiel/media/velo-01.jpg — generated placeholder JPEGs, no EXIF"
    - "tools/mkbundle/pack_test.go — packs the example into t.TempDir() and decodes both pictures"
    - "sites/README.md, rewritten around the example"
    - "The two customer directories under sites/ gone from HEAD, together with their 33 MB of photographs"
  key_links:
    - "json tags in internal/bundle/format.go -> DisallowUnknownFields in mkbundle readManifest -> every key that may appear in holzcloud.json"
    - "media/<file> on disk -> manifest media[].filename -> checkMedia in both directions -> the zip entry media/<file>"
    - "/media/0/<file> in a page body -> mkbundle mediaPathPattern -> bundle.rewriteMediaPaths on import -> the picture a visitor actually sees"
    - "manifest media[].mime_type -> image.Decode format in pack_test.go -> the type the browser is told to trust"
    - "alt text in the manifest and in the Markdown -> mkbundle altAllowed -> the alt attribute bluemonday keeps instead of dropping whole"
---

<objective>
Take the two live customer websites out of `sites/` and leave an invented example bundle
in their place — small, readable, and accepted by `tools/mkbundle`.

Purpose: the open-source readiness audit run today found one blocking issue. `sites/`
does not hold sample data. It holds two real businesses: 33 MB of their photographs,
their editorial text, a private email address and a postal address in both manifests.
None of it is this project's to publish under AGPL-3.0. The content already lives in the
private repository `holzcloud/holzcloud-sites`, pushed and verified, so nothing is lost
by removing it here.

But `sites/` cannot simply become empty. The directory is how somebody learns the bundle
format, and `tools/mkbundle` is a strict validator with nothing left to validate. So the
removal comes with a replacement: an ordinary, harmless, invented website that passes
every check and shows each part of the format once.

Output: `sites/beispiel` with a hand-written manifest and two generated pictures; a
rewritten `sites/README.md`; a regression test that keeps the example honest as the
format moves; and a working tree with no trace of either customer.
</objective>

<execution_context>
@/Users/holz/.claude/gsd-core/workflows/execute-plan.md
@/Users/holz/.claude/gsd-core/templates/summary.md
</execution_context>

<context>
@/Users/holz/Projects/holzcloud-cms/CLAUDE.md
@/Users/holz/Projects/holzcloud-cms/internal/bundle/format.go
@/Users/holz/Projects/holzcloud-cms/tools/mkbundle/main.go
@/Users/holz/Projects/holzcloud-cms/sites/README.md
</context>

<mutable_scope>
The planner read the repository on 2026-09-03 before writing this plan. Four findings
are recorded here as findings, not as instructions to trust blindly. The executor
confirms each by reading the named file and widens scope if reality has moved.

**Finding A — the accepted key set is external state and is NOT frozen into this plan.**
`readManifest` in `tools/mkbundle/main.go` decodes with `dec.DisallowUnknownFields()`
into `bundle.Manifest`. That means the set of keys a hand-written manifest may use is
exactly the set of json tags on the structs in `internal/bundle/format.go` — nothing
else, at execution time, not at planning time. **Do not copy a key list out of this
plan or out of any existing manifest.** Read `internal/bundle/format.go` and take the
tags from there. The planner saw one field the task briefing did not mention at all
(`terms`, checked by `checkReferences`), which is the whole reason for this rule.

Two hard constraints the planner did confirm in that file and in `main.go`:

- `readManifest` refuses a manifest whose `site.name` is empty or whose `pages` array
  is empty. Everything else is optional as far as decoding goes.
- `mkbundle` overwrites `version`, `exported_at` and `generated_by` at pack time, and
  computes every `sha256` itself. Writing a checksum into the file by hand is the one
  thing the README explicitly warns against.

**Finding B — the alt-text charset is narrower than German prose.** `altAllowed` in
`tools/mkbundle/main.go` is `^[\p{L}\p{N}\s\-_',\[\]!\./\\()]*$`. Letters, digits,
spaces, hyphen, underscore, apostrophe, comma, brackets, exclamation mark, full stop,
slashes and parentheses — and nothing else. A colon, a question mark, an en dash, an
ampersand, a percent sign or a guillemet in an image description makes `mkbundle`
refuse the build, because bluemonday would drop the whole attribute rather than the one
character. This applies to `![...](...)` in page bodies and to `<img alt="...">`. The
plan extends the same discipline to `media[].alt_text` in the manifest, which mkbundle
only checks for emptiness but which ends up in the same attribute after import. Page
prose outside an alt attribute is unaffected — guillemets and dashes are fine there.

**Finding C — the customer names reach eleven files outside `sites/`.** The planner's
grep on 2026-09-03 found them in `internal/mail/`, `internal/web/layoutdata.go`,
`internal/i18n/i18n_test.go`, three files under `internal/plugin/`, two under
`internal/public/`, `internal/admin/page_blocks_test.go` and one comment in
`cmd/holzcloud/templates/public/weide/layout.html`. Almost all are test fixture names
or doc-comment examples. **The executor's own grep decides the list, not this one.**

**Finding D — `.gitignore` already covers the built archive.** It carries
`/sites/*.zip` under a German comment about built website bundles. The executor confirms
this with `git check-ignore` rather than assuming it, and only edits `.gitignore` if the
check fails.
</mutable_scope>

<decisions>
Three questions the briefing left open. Each is answered here so the executor does not
have to reopen them, and each answer goes into the summary.

**D-01 — the pictures are generated by a throwaway script that is not committed.**
The alternative was a committed generator under `tools/`. Rejected: it would be a third
Go program in a repository that has two, and its entire job would be to produce two
placeholder pictures that never change. Pictures are content, not a build artefact —
nobody regenerates a photograph either, and a committed generator invites a rerun that
silently rewrites the bytes and therefore every checksum in every archive built since.
What matters is that the knowledge survives, so the recipe goes into `sites/README.md`
as prose: two flat-coloured JPEGs, `image` and `image/jpeg` from the standard library,
no third-party code, no EXIF. Anyone can redo that in ten minutes.

**D-02 — the identifier sweep goes all the way, including the doc comment in
`internal/mail/mail.go`.** A business name in a doc comment is not personal data, and on
its own it would be defensible. But it is a real business named in a public file for no
reason, the fix is a one-word replacement, and the acceptance bar for this task is a
grep that returns nothing. A sweep that stops at ten of eleven files leaves the next
auditor to rediscover the eleventh. It goes.

**D-03 — the example is verified through `mkbundle`, not through a live import.**
A test that actually imports the example would need a database, the full `Stores` set,
and either a duplicate of mkbundle's packing logic inside `internal/bundle` or a
duplicate of `newStores` inside `package main`. That is not a quick task. It is also
close to redundant: `mkbundle` decodes into the very same `bundle.Manifest` the importer
decodes into, with `DisallowUnknownFields` on top, and checks the media and the
references the importer would silently skip. The only untested residue is that both
sides use that same struct — which the compiler guarantees. Recorded as a known limit
in the summary rather than papered over.
</decisions>

<tasks>

<task type="tracer">
  <name>Task 1: An invented bundle at sites/beispiel that mkbundle accepts</name>
  <files>sites/beispiel/holzcloud.json, sites/beispiel/media/werkstatt-01.jpg, sites/beispiel/media/velo-01.jpg</files>
  <read_first>
    internal/bundle/format.go — the json tags are the only source of truth for which
    keys the manifest may contain (see <mutable_scope> Finding A). Read the whole file.
    tools/mkbundle/main.go — checkMedia, checkReferences, altAllowed, mediaPathPattern.
  </read_first>
  <action>
    Build the thin end-to-end path first and prove it before writing any prose. Create
    the directory, generate the two pictures, write a minimal manifest with one page and
    one media entry, and run mkbundle. Once that exits 0, expand it to the full example.
    That ordering means a wrong key name surfaces after five minutes, not after an hour
    of invented German text.

    Generate the pictures with a throwaway Go program in the scratchpad directory, per
    D-01. Do not commit the program and do not add it to tools/. Use only `image`,
    `image/color`, `image/draw` and `image/jpeg` from the standard library — this repo
    forbids reaching for a new dependency, and stdlib jpeg writes JFIF with no EXIF
    block, which is what is wanted. Two images, roughly 1200x800, flat background with
    a couple of simple rectangles so the two are visibly different, quality around 80,
    well under 100 KB each. Write them straight into sites/beispiel/media/ as
    werkstatt-01.jpg and velo-01.jpg. Confirm the bytes with `file` before going on.

    Then write sites/beispiel/holzcloud.json by hand. The invented business is a small
    bicycle workshop, deliberately unlike either removed website and obviously
    fictional. Fix these identifiers exactly so the executor does not have to invent
    twice, and so the identifier sweep in task 2 has a name to move to:

      site.name           Velowerkstatt Beispiel
      site.contact_email  kontakt@example.com     (RFC 2606 reserved, unreachable)
      site.street         Musterweg 1
      site.postal_code    0000                    (not a valid Swiss code, on purpose)
      site.city           Musterhausen
      site.country        CH
      site.locale         de
      site.timezone       Europe/Zurich
      site.org_type       LocalBusiness

    Give it a brand colour and a text measure between 40 and 110 — design.Sanitize
    bounds the measure and drops anything outside it, so a value inside the range proves
    the tokens travel. Leave version, exported_at, generated_by and every sha256 out or
    at their defaults; mkbundle writes all four itself.

    Four published pages, kind "page" (the constants live in internal/page/models.go):
    slug home titled Willkommen, slug werkstatt, slug service, slug kontakt. Give home a
    featured image of werkstatt-01.jpg and an inline /media/0/werkstatt-01.jpg in its
    body; give werkstatt an inline /media/0/velo-01.jpg. The zero is the placeholder the
    importer rewrites to the real website id, and having one in the example is the point
    — it is the only way a reader sees that rewriting exists. Add an excerpt and a
    meta_description to at least one page so those fields are demonstrated.

    One menu named Hauptmenü at location main, with a nested child — service sitting
    under werkstatt — plus one item of type url pointing at "/" and one of type page.
    One snippet, key oeffnungszeiten, with opening hours as its body.

    Write the German prose in this repository's voice: short, concrete, unhurried,
    ordinary. This file is a teaching object, and a lorem-ipsum bundle teaches nothing
    about what a page looks like. Invent everything. Carry over no sentence, no filename,
    no address and no image from either removed website — read neither of their manifests
    while writing this one.

    Keep every image description inside the charset in <mutable_scope> Finding B: no
    colon, no question mark, no en dash, no ampersand, no guillemets, in the Markdown
    alt text and in media[].alt_text alike. Commas and full stops are allowed and are
    enough. The page prose itself is unrestricted.

    Then run mkbundle and fix what it names. Its messages are precise; each one is a real
    defect in the manifest, not a formality to work around.
  </action>
  <verify>
    <automated>go run ./tools/mkbundle sites/beispiel &amp;&amp; test -f sites/beispiel.zip &amp;&amp; file sites/beispiel/media/*.jpg | grep -c 'JPEG image data'</automated>
  </verify>
  <done>
    mkbundle exits 0 and reports the written archive. Both files under
    sites/beispiel/media/ are real JPEG data according to `file`. The manifest contains a
    nested menu item, a snippet, a featured image and at least one /media/0/ path in a
    page body.
  </done>
</task>

<task type="auto">
  <name>Task 2: Remove the customer data and rewrite sites/README.md</name>
  <precondition>
    Both customer websites are already pushed to the private repository
    holzcloud/holzcloud-sites and verified there — the developer confirmed this on
    2026-09-03. If that is in any doubt, stop and ask: after this task the working tree
    is no longer a copy.
  </precondition>
  <reversibility rating="reversible">
    `git rm` removes the files from HEAD only. The blobs stay reachable in history, so a
    revert restores everything. This task deliberately does not rewrite history.
  </reversibility>
  <files>sites/README.md, tools/mkbundle/main.go, plus whatever the grep in &lt;verification&gt; names</files>
  <action>
    Delete the two website directories that sit beside the new example in sites/. They
    are the only other entries there — `ls sites/` names both. Use `git rm -r` so the
    removal is staged as a deletion rather than left as an untracked absence, and remove
    their media with them.

    Then sweep the identifiers. Run the grep from <verification> to get the current list
    of files, and replace every hit with the invented business from task 1. The mapping:
    the two-word display name becomes "Velowerkstatt Beispiel"; the single-word fixture
    name becomes "Velowerkstatt"; a test host becomes velowerkstatt.test. Two hits need
    care rather than a blind substitution:

    - internal/mail/mail_test.go has a case whose entire point is that a display name
      containing a comma gets quoted. Keep a comma in the replacement, and update the
      expected header string in the same file to match, or the test is asserting on a
      name nobody passes any more.
    - internal/i18n/i18n_test.go asserts on a formatted string that contains an en dash
      as a separator. Swap the name, keep the dash and keep the German and English
      expectations consistent with each other.

    Everywhere else the change is mechanical. Swap only the tokens; do not rewrite the
    surrounding test content, and keep each assertion consistent with the fixture in its
    own file. The comment in the weide layout template refers to a sister website by
    name — reword it so it describes the situation generally instead. The usage example
    in the tools/mkbundle package comment points at a directory that no longer exists;
    point it at sites/beispiel.

    Then rewrite sites/README.md. Keep what is still true and genuinely useful: what a
    bundle is, why it is checked in unpacked rather than as a zip, what mkbundle refuses
    to build and why, how the /media/0/ placeholder gets rewritten on import, that the
    theme and the domain are set by hand because neither travels in the manifest, and
    that after the first import the admin is the source of truth and the file is the
    archive to start over from. Drop the table naming the two websites and the section
    about where they came from. In their place: the example lives here and is what to
    read and copy, and the two real websites moved to the private repository
    holzcloud/holzcloud-sites because their content is not this project's to publish
    under AGPL-3.0. Name neither of them — the acceptance bar for this task is a grep
    that comes back empty, and this file is inside it. Add the recipe for the two
    pictures from D-01 as one short paragraph: generated placeholders, standard library
    only, redo them the same way if they ever need to change.
  </action>
  <verify>
    <automated>grep -rniE '<die Kundennamen>' --exclude-dir=.planning --exclude-dir=.git . ; test $? -eq 1</automated>
  </verify>
  <done>
    The two customer directories are gone from sites/ and staged as deletions. The grep
    returns nothing outside .planning/ and .git/. sites/README.md describes the example
    and records where the real websites went without naming them.
  </done>
</task>

<task type="auto" tdd="true">
  <name>Task 3: A test that keeps the example valid, and the full verification pass</name>
  <files>tools/mkbundle/pack_test.go</files>
  <behavior>
    - Packing sites/beispiel into a temporary directory succeeds, and the resulting
      archive contains holzcloud.json and one media/ entry per manifest entry.
    - Every file listed in the manifest decodes as an image whose format matches the
      declared mime_type — image/jpeg must decode as jpeg, image/png as png.
    - A manifest key that internal/bundle no longer knows makes this test fail rather
      than surviving until somebody next runs mkbundle by hand.
  </behavior>
  <action>
    Write tools/mkbundle/pack_test.go in package main, so it can call pack and
    readManifest directly. Point it at the example with filepath.Join("..", "..",
    "sites", "beispiel") and pack into t.TempDir(), so the test writes nothing into the
    repository and leaves no archive behind. Read the packed archive back with
    archive/zip and check the entries.

    For the picture check, read each manifest entry's file from sites/beispiel/media/ and
    run it through image.Decode with image/jpeg and image/png registered by blank import.
    Compare the format string it returns against the mime_type in the manifest. This is
    the check that closes the one gap mkbundle leaves open — it trusts the declared type
    and never looks at the bytes, and a manifest that claims image/jpeg over PNG bytes
    would import cleanly and render as a broken picture on a machine where nobody can see
    it.

    This test is the reason the example does not quietly rot: it fails the moment
    internal/bundle changes in a way the hand-written manifest does not follow.

    Then run the full verification pass and record the numbers in the summary. Also
    confirm the built archive stays out of git — `.gitignore` appears to cover it
    already (Finding D), so check rather than edit, and only add a line if the check
    fails. Delete any sites/beispiel.zip left over from task 1 before the final
    `git status`, so the tree is clean.

    In the summary, say plainly that this is a removal from HEAD and not a history
    rewrite: the blobs remain reachable in git history, and the developer has decided
    against a filter-repo pass for now. Anyone reading the summary later should not come
    away believing the data is gone from the repository. Record the D-03 limit as well —
    the example is proven through mkbundle, not through a live import.
  </action>
  <verify>
    <automated>go build ./... &amp;&amp; go vet ./... &amp;&amp; go test ./... &amp;&amp; git check-ignore -q sites/beispiel.zip</automated>
  </verify>
  <done>
    go build, go vet and go test are all green with 0 FAIL. The new test passes and fails
    if a manifest key is removed from internal/bundle. `git check-ignore` confirms the
    built archive is ignored, and `git status --porcelain sites/` shows no zip.
  </done>
</task>

</tasks>

<threat_model>
## Trust Boundaries

| Boundary | Description |
|----------|-------------|
| repository -> the public | Everything in HEAD is published under AGPL-3.0 to anyone who clones |
| bundle file -> importer | A manifest is a file anyone can edit; the importer treats it as untrusted |

## STRIDE Threat Register

| Threat ID | Category | Component | Severity | Disposition | Mitigation Plan |
|-----------|----------|-----------|----------|-------------|-----------------|
| T-ceo-01 | Information Disclosure | the two customer site bundles under `sites/` — customer photographs, editorial text, a private email address and a postal address in HEAD | high | mitigate | Task 2 removes both directories from HEAD; task 2's grep proves nothing is left in the working tree |
| T-ceo-02 | Information Disclosure | Customer names surviving in doc comments and test fixtures after the directories go | medium | mitigate | D-02: the sweep covers every file the grep finds, not just `sites/` |
| T-ceo-03 | Information Disclosure | The same data remains reachable in git history after `git rm` | medium | accept | Deliberate: the developer has decided against a filter-repo pass for now. Task 3 requires the summary to state this plainly rather than implying the data is gone |
| T-ceo-04 | Tampering | The example manifest declaring a mime_type its bytes do not match, so a future reader copies a lie about the format | low | mitigate | Task 3 decodes every picture and compares the format against the declared type |
| T-ceo-05 | Information Disclosure | A generated picture carrying EXIF from the machine that made it | low | mitigate | D-01: `image/jpeg` from the standard library writes JFIF with no EXIF block; no third-party encoder is used |
| T-ceo-SC | Tampering | Supply chain — package-manager installs | n/a | n/a | No install of any kind. Image generation uses the Go standard library; `go.mod` is untouched |
</threat_model>

<verification>
Run from the repository root.

```sh
# The example builds, and the archive it produces is not tracked
go run ./tools/mkbundle sites/beispiel
git check-ignore -q sites/beispiel.zip && echo "zip ignored"
rm -f sites/beispiel.zip

# The baseline stays green
go build ./... && go vet ./... && go test ./...

# Nothing of either customer is left in the working tree.
# .planning/ is excluded because this plan and its summary discuss the removal;
# .git/ is excluded because the history is deliberately not rewritten (T-ceo-03).
grep -rniE '<die Kundennamen>' --exclude-dir=.planning --exclude-dir=.git .

# The tree is clean apart from the intended changes
git status --porcelain
```

The grep is the acceptance bar for task 2 and must return no lines at all. It is the one
check here that cannot be satisfied by editing only `sites/`.
</verification>

<success_criteria>
- `go run ./tools/mkbundle sites/beispiel` exits 0 and writes `sites/beispiel.zip`
- The archive is git-ignored and absent from `git status`
- `go build ./...`, `go vet ./...`, `go test ./...` — 0 FAIL
- The customer grep returns nothing outside `.planning/` and `.git/`
- `sites/beispiel/holzcloud.json` uses only keys that exist as json tags in
  `internal/bundle/format.go`, and exercises a page, a nested menu item, a snippet, a
  media entry, a featured image and an inline `/media/0/` path
- `tools/mkbundle/pack_test.go` packs the example and decodes both pictures
- `sites/README.md` teaches the format from the example and names neither real website
- The summary states that history was not rewritten, and records D-01, D-02 and D-03
</success_criteria>

<output>
Create `.planning/quick/260903-ceo-beispiel-bundle-statt-kundendaten/260903-ceo-SUMMARY.md` when done
</output>
