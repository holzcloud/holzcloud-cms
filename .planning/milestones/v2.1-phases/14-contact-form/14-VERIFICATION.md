# Verification: Phase 14 — A contact form worth writing into

Measured 2026-09-13 against `1b44e70` and the work that follows it, on a running
binary at `127.0.0.1:8080` with a throwaway database, driven through HTTP rather
than read.

---

## FORM-01 — worth looking at, in all eight themes

**Met.** Before: `holzcloud`, `rudel` and `weide` carried **zero** CSS rules for
`.contact-form` — on three of eight themes the form rendered in browser
defaults. Now each of the eight styles it in its own hand, and all eight gained
`__why`, `__required`, `__limit`, `__consent`, `__more` and `[aria-invalid]`.

A refusal stands **at the field it concerns**. That is what the reason-code shape
buys: `refusal{Field, Code, Arg}` travels back through the address, and the form
puts the sentence under the right `<label>` and marks the input
`aria-invalid="true"`. One sentence above the form cannot do that.

Required fields are marked in words and not only by colour, and the character
limit is stated rather than discovered by being truncated.

## FORM-02 — the sender learns it arrived, and can be answered

**Met, and the permission rule it had to get past was answered rather than waved
past.** `PermNotify`'s own documentation says the recipient is never the
plugin's to choose, because a plugin that could name one would be a mail relay
with a public web interface. The answer:

- **one** copy, of the message just sent,
- to the single address it already carries as ReplyTo,
- gated by a **second** permission (`PermConfirm`) **and** the operator's
  per-website switch (`websites.confirm_senders`, migration `00056`).

Answering from the admin writes the reply back under the same key as the
message. `speichern()` mints a fresh key, so the first attempt would have
duplicated the enquiry; a test holds the fix.

## FORM-03 — no enquiry is thrown away silently

**Met.** `sweep()` no longer deletes the oldest past 500 regardless. It skips
anything unread and says in its result what it kept, so an operator can see why
the store is full rather than discovering that an enquiry is gone.

`TestSweepKeepsWhatNobodyHasRead` holds it.

## FORM-04 — consent asked for and recorded

**Met, and then extended after the theme pass found it half-done.**

The wording shown and the moment it was ticked are stored with the message. The
tick box exists only when the operator writes a sentence — a form that grew one
because the plugin was updated would be the wrong kind of surprise.

The theme pass (QUAL-02) found the hole: the sentence was stored **once** per
website, so a French page asked, in German, for agreement to store the visitor's
details. A consent nobody can read is not one. So:

- `SettingsResult.Locales` — every language the website publishes in, each with
  its native name. Built from `Website.AllLocales()`, which is new because the
  trap behind it had now been fallen into twice: `Locales()` is the **extra**
  languages and leaves out the one most of the site is written in.
- `ContentIn.Lang` and `RequestIn.Lang` — the language *this* page or request is
  answered in, which is not the website's first.
- The consent screen has one box per language, and says beside an empty one what
  a visitor reading that language will actually be shown.
- The form carries the language it was **drawn** in, in a hidden field, checked
  on receipt against the languages this website publishes in. The submission
  goes to `/formular`, which has no language of its own — without this the
  sentence recorded with a French enquiry was the German one.

Proven live: the French page carries *"J'accepte que mes données soient
enregistrées."*, the German page its own sentence, and the message stored from
the French form records the French words.

## FORM-05 — a refused submission is visible

**Met.** Honeypot, time trap and hourly limit land in a quarantine carrying the
reason, bounded at 50, visible at `?ansicht=abgewiesen` and releasable from
there. A false positive can now be found; before, it was gone.

## FORM-06 — attachments and conditional fields

**Met, in two halves.**

**Attachments.** The host takes the files and the plugin never sees bytes —
`MaxPluginBodyBytes` stays at 256 KB and `RequestIn.Files` carries name, kind
and size only. Files are *held*, not stored, until the spam traps have run.
Proven live:

```
honeypot -> /?formular=gesendet | media files: 0 (was 0)
real     -> /?formular=gesendet | media files: 1
```

and in the admin the enquiry carries
`<a href="/media/1/bb0bd5da…png">pixel.png</a>`.

**Conditional fields.** A field is asked only when an earlier field has been
answered in a particular way. Without JavaScript that means a second go: a
Continue button submits the form as a GET to the page it stands on, and the
fields whose condition is now met appear. A field whose condition is not met is
**not drawn** rather than hidden — a hidden field is still submitted and still
read out.

Two things had to be true for that path to work, and neither was:

1. The first step's answers had to be read back out of the address.
   `answersFromQuery` does it only for the form the Continue belonged to, only
   for that form's own fields, bounded and escaped. This program still puts
   nothing a visitor typed into an address of its own accord; a query the
   visitor's own browser wrote is not that.
2. A form has to know the address of the page it stands on. `ContentIn.Path`,
   filled by the host from the request. **This was a bug well beyond the
   feature**: every answer to a submission was built from the slug, so a form on
   the start page redirected to `/home`, which the public side redirects to `/`
   *without its query* — and the visitor saw the form again and never the
   thank-you or the reason.

Proven live, on the start page:

```
step 1            fields: [wobei-koennen-wir-helfen]          continue -> /
Continue "Wolle"  fields: [wobei…, wie-viel-wolle-brauchen-sie]  selected: Wolle
Continue "Hoff…"  fields: [wobei…]                              selected: Hoffuehrung
submit            303 -> /?formular=gesendet&welches=anfrage
admin             Wobei koennen wir helfen? Wolle · Wie viel Wolle brauchen Sie? drei Kilo
```

and the thank-you and the refusal both now arrive on the start page:

```
/?formular=gesendet              -> "Danke, die Nachricht ist angekommen."
/?formular=fehler&grund=consent… -> "Bitte stimme vor dem Absenden zu."
```

## QUAL-02 — driven in a browser

**Met.** All eight shipped themes, in two languages, against the running server:

```
default      | de 200 form clean | fr 200 form clean
holzcloud    | de 200 form clean | fr 200 form clean
journal      | de 200 form clean | fr 200 form clean
magazine     | de 200 form clean | fr 200 form clean
midnight     | de 200 form clean | fr 200 form clean
rudel        | de 200 form clean | fr 200 form clean
schlicht     | de 200 form clean | fr 200 form clean
weide        | de 200 form clean | fr 200 form clean
```

*clean* means no English left standing in the page text. The French page reads
French throughout — chrome, form, labels and hints:

> Accueil — Hofladen · Aller au contenu · Nom (obligatoire) · E-mail
> (obligatoire) · Objet · Message (obligatoire) · au maximum 8000 caractères ·
> Pièce jointe · au maximum 3 fichiers · Envoyer le message · Mentions légales

This pass is what found the consent hole under FORM-04. It is the argument for
the gate: no test caught it, and one language of one page showed it at once.

## What this phase found that it did not plan for

- **The reason-code shape, three times.** `field.Reason`, the form's
  `refusal{Field, Code, Arg}` and `media.Refusal{Code, Arg}` are the same idea
  arrived at from three directions: never a finished sentence where the reader
  is not yet known.
- **A test that did its job.** `TestEveryCodeThisProgramProducesHasASentence`
  failed at 22 sentences for 20 codes when `consent-missing` and
  `attach-refused` were added without updating its list.
- **`pageName` was never enough.** It allows `[a-z0-9-]` and no slash, so a page
  in a second language would have lost its prefix too. `pagePath` replaces it
  for the redirect target and refuses anything that is not an address on this
  website; `pageName` stays for the record of which page the enquiry came from.
