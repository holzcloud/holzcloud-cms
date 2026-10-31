# Context: Phase 14 — A contact form worth writing into

Written as the phase ran, 2026-09-13. Every section here is a decision taken
*during* the work, put with the other decisions rather than left in a commit
message where the next reader will not look for it.

---

## §1 The receipt, and the rule it had to get past

`PermNotify` carries an explicit rule in its own documentation: **the recipient
is never the plugin's to choose**, because a plugin that could name one would be
a mail relay with a public web interface behind somebody else's domain.

FORM-02 asks for a copy to the sender. That is a plugin naming a recipient, so
the rule had to be *answered* and not waved past. The answer has four parts and
all four are needed:

1. It is **one** copy, not a list.
2. Of the **message just sent**, not of text the plugin composes.
3. To the **single address the message already carries** as its ReplyTo — the
   plugin does not name it, it is the address the sender typed.
4. Behind a **second permission** (`PermConfirm`, declared in `plugin.json` and
   checked at install) **and** the operator's per-website switch
   (`websites.confirm_senders`, migration `00056`, off by default).

The host enforces 1–3; 4 is two independent switches, one the plugin author's
and one the operator's. Neither alone opens it.

## §2 Attachments: held, not stored

Two constraints met here and both hold.

`MaxPluginBodyBytes` is 256 KB and stays. A plugin that could pull a
five-megabyte upload into its linear memory is a way to exhaust a small node with
one request. So the **host** takes the files out of the multipart submission and
leaves the fields; a plugin written before attachments existed reads exactly
what it read before, and `RequestIn.Files` carries a name, a kind and a size —
never bytes.

And the files are **held in memory, not written**, until the plugin answers. That
is what lets the spam traps run first: a robot that trips the honeypot leaves
nothing on the disk at all. Proven live — a honeypot submission carrying a file
leaves the media count unchanged.

`OpKeepFiles` is the plugin saying "these ones, now", and it needs `PermAttach`.

## §3 A refusal is a code, never a sentence

This shape was arrived at three times in this phase from three directions —
`field.Reason`, the form's `refusal{Field, Code, Arg}`, and `media.Refusal{Code,
Arg}` — and the rule behind all three is the same: **never build a finished
sentence where the reader is not yet known.**

It buys three things:

- The message can stand **at the field it is about**, which one string handed
  back through the address bar can never do.
- The set of things the address bar may say is **closed**: an unknown code gives
  the general sentence, a field name that is not this form's is dropped.
- The sentence is made in the **reader's** language, which for a public request
  is the page's and for an admin request is the operator's.

A sentence built with `fmt.Sprintf` or joined with `+` is invisible to the
collector, so the whole sentence is a format string in the catalogue and the
operator's own label travels as `%s`. A form that asks for "Lieblingsfarbe" says
"Lieblingsfarbe" back, untranslated, because that is the operator's word.

## §4 Conditional fields: one split, and the query is the visitor's own

**One split and not a chain.** A form that needs three steps needs three forms,
and a visitor who has to press Continue twice has already left. So `steps()`
splits at the *first* conditional field: everything before it is the first step,
the rest is the second.

**Not drawn, not hidden.** A hidden field is still in the document, still
submitted, and still read out by a screen reader that ignores the stylesheet —
and un-hiding is exactly what needs JavaScript.

**Not a wizard that locks the first step.** Every field stays visible and
changeable, so somebody who picked the wrong answer can pick another and press
Continue again.

**And the query.** This program still puts nothing a visitor typed into an
address of its own accord; that is why a refused submission says what is missing
and no more. But `formmethod="get"` means the visitor's own browser writes the
answers into the address, and reading back what is already there is not the same
act as writing it. `answersFromQuery` reads it only for the form the Continue
belonged to, only for that form's own fields, bounded and escaped. The cost is
named where it is paid: a two-step form's first answers stand in the server log
like the query string of every GET, which is why the split is at the first
conditional field — a form that asks something which should not be logged should
ask it after Continue.

## §5 The consent, per language — decided after the browser pass

Not planned. FORM-04 as written is satisfied by one sentence per website, and
the phase shipped that.

Then QUAL-02's pass through the eight themes in two languages showed the French
page asking, in German, for agreement to store the visitor's details. The
milestone's own headline claim was false on the most visible element of its own
second phase, so this was fixed rather than recorded.

The decision that makes it work is the smaller one underneath: **a plugin has to
be told which language *this* page is in.** `Site().Locale` is the website's
first language and answering from it is the same mistake, one level down, that
PUB-01 exists to repair. So `ContentIn.Lang` and `RequestIn.Lang`.

And a third thing, which is the part that is easy to get wrong: the **submission**
goes to `/formular`, which carries no language. The form therefore carries the
language it was *drawn* in, checked on receipt against the languages this website
publishes in. What must be stored with a message is the sentence the sender
actually read.
