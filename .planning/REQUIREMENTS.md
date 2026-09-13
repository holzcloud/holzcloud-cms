# Requirements: v2.1 — The Public Side Speaks the Visitor's Language

Milestone opened 2026-09-13, immediately after v2.0 closed. It exists because
v2.0 named two things deliberately open and one of them turned out to be
actively broken rather than merely absent — see PUB-01's note.

**Milestone goal:** A website published in French reads French — its chrome, its
plugin refusals and its forms, not only its pages. And the form a visitor
actually writes into is worth looking at and can do the things a contact form
has to do.

---

## The public translation channel

- [ ] **PUB-01**: **The visitor-facing strings in `plugins/` speak the visitor's
      language again.** Measured 2026-09-13 on `main`: `check()` in
      `plugins/kontaktformular` answers a German visitor with *"Please enter an
      e-mail address so that we can answer."* — six of its eight refusals are
      English, two are still German, and the labels beside them are German.
      `plugins/bestellung` carries 21 English strings against 4 German ones.
      This is a **regression from v2.0** (`df4ff4c`), whose commit message
      justifies the change with *"die Texte, die ein Betreiber liest"* — the
      error being that these are a *visitor's* texts. `sdk.T` was added in the
      same milestone and is the channel; it is simply not used here.
- [ ] **PUB-02**: **A theme carries its own catalogue.** A theme directory may
      hold `lang/<tag>.json`. The public FuncMap gains `t`, `th` and `tf`, and
      they resolve **only** against the catalogue of the theme being rendered —
      never against the program's. This is what answers the objection recorded
      in `TEMPLATE-SPEC.md` §2.5: a `t` that silently returned the key for a
      third-party theme would be worse than none, so a missing key is
      **reported**, not swallowed.
- [ ] **PUB-03**: **The operator can override any key**, per website and per
      language, from the admin. Precedence is override → theme catalogue →
      the key itself. `cacheKey` already carries `websiteID` and `locale`, so a
      changed override invalidates exactly the sets it must.
- [ ] **PUB-04**: **The eight shipped themes are converted and stay in step.**
      Measured 2026-09-13: **150 distinct strings, 955 occurrences**, and nearly
      every one appears in all eight themes — they mint the same vocabulary.
      They are generated from one shared source by a committed tool so they
      cannot drift, and shipped in de, en, fr, it and es.
- [ ] **PUB-05**: **`TEMPLATE-SPEC.md` §2.5 is rewritten.** It says today that a
      theme is single-language and that `t` is deliberately absent. After this
      milestone it says the opposite. The old reasoning is kept and the section
      says why it no longer holds — a specification that quietly changes its
      mind teaches a theme author nothing.
- [ ] **PUB-06**: **A missing translation is countable, not silent.**
      `holzcloud template check` names the keys a theme mints but does not
      translate, and the upload path refuses nothing new — CSP, zip-slip and
      `template.Check` are untouched.

## The contact form

- [ ] **FORM-01**: **It is worth looking at, in all eight themes.** Measured
      2026-09-13: `holzcloud`, `rudel` and `weide` carry **zero** CSS rules for
      `.contact-form`; the other five carry 11–15. Each theme styles it in its
      own hand. A refusal appears **at the field it concerns** rather than as
      one sentence above the form, required fields are marked, focus is visible.
      No JavaScript, as everywhere.
- [ ] **FORM-02**: **The sender learns that it arrived.** Today `notify()` mails
      the operator and nobody mails the sender. And the operator can answer from
      the admin instead of switching to a mail client; the answer stays with the
      message.
- [ ] **FORM-03**: **No enquiry is thrown away silently.** `sweep()` deletes the
      oldest messages past 500 with nothing but a log line. This plugin's own
      migration says *"eine Anfrage, die jemand gestellt hat und die niemand
      liest, ist eine verlorene Anfrage"* — an unread message never falls
      automatically.
- [ ] **FORM-04**: **Consent is asked for and recorded.** A required tick box
      with a linked privacy text; the wording shown and the moment it was
      ticked are stored with the message, because a consent nobody can
      reconstruct is not one.
- [ ] **FORM-05**: **A refused submission is visible.** Honeypot, time trap and
      hourly limit currently drop a submission into nothing. They land in a
      quarantine carrying the reason, so a false positive can be found.
- [ ] **FORM-06**: **Attachments and conditional fields.** Attachments go
      through the existing media store with a size and type bound and must not
      breach website isolation. A field that only appears when an earlier answer
      calls for it, without JavaScript — so in a second step, not a hidden div.

## Standing gates

- [ ] **QUAL-01**: `go run ./tools/i18n` reports `0 open, 0 orphaned` on every
      catalogue, and `go run ./tools/english` stays green — **with the gate
      extended**, because it did not catch PUB-01. Its `germanVoice` list names
      `internal/` files and no plugin, so a visitor's text in `plugins/` was
      invisible to exactly the gate built to see it.
- [ ] **QUAL-02**: Every screen a person can see is driven once through the
      running application in a browser. The blast radius here is every public
      page of all eight themes, in at least two languages.
