# Requirements: v2.0 — The Codebase Speaks English

Milestone opened 2026-09-11. Requirements carried in from
`.planning/milestones/v1.10-REQUIREMENTS.md` (section *Language*), verbatim
except where a measurement has since moved — those are marked.

**Milestone goal:** A stranger can read this repository. Every identifier, every
comment, every test name, every SQL column and every catalogue key is English —
and the template data contract is too, which is why the release carrying this
milestone is **2.0** and not 1.11.

---

## Language

- [ ] **LANG-01**: Every comment in Go source is English, and a comment that
      carried a *reason* still carries it at the same length. The reasoning in
      this repository's comments is its most valuable content; a sweep that
      shortens it has done damage, not work.
- [ ] **LANG-02**: Every identifier is English, and each German term maps to
      exactly one English word, fixed in `.planning/GLOSSARY.md`. A term
      translated without an entry gets one in the same commit.
- [ ] **LANG-03**: Every test function name is English and still says what it
      asserts. The German names are unusually good at this; the English ones
      must be no worse.
- [ ] **LANG-04**: The template data contract is English — `.Page.Fields`,
      `.Site.SnippetFields`, `.Page.Translations`, `.Page.Kind`. All eight
      shipped themes come with it, `TEMPLATE-SPEC.md` names only the English
      fields, and `CHANGELOG.md` records it as a **breaking change** under
      `## 2.0`. A theme written against 1.x stops working, deliberately and in
      one release.
- [ ] **LANG-05**: The German SQL column names are English, through **new**
      migrations. No released migration is edited. Every hand-written SQL
      statement follows, including the carrier discriminator in
      `internal/field/store.go`.
- [ ] **LANG-06**: The catalogue's source language is English: the German keys
      become English keys, a new `de.json` carries German as a translation,
      `de-CH.json` derives from it, and `es/fr/it.json` are re-keyed through the
      old German→English mapping. The colliding keys are resolved one by one,
      and the three that collide because the existing translation is **wrong**
      are fixed rather than merged.
      *(The archive says 1158 keys / nine collisions, measured 2026-09-06. The
      gate reads 1328 keys today; 12-01 re-measures the collisions.)*
- [ ] **LANG-07**: German cannot return unnoticed. A mechanical gate fails the
      build if German enters Go source outside the catalogue files — a gate, not
      a review convention.
- [ ] **LANG-08**: The stored German vocabularies are decided explicitly. About
      25 German strings in Go are **data**, not identifiers — the field kinds,
      `gilt_fuer`'s three values, `knopfreihe`, and the seven block kinds, which
      also appear as CSS classes in all eight themes. They sit in every existing
      database and travel in every exported bundle. Turning them is a data
      migration plus a bundle-compatibility question plus a second theme break.
      **Leaving them German is a defensible answer; leaving them undiscussed is
      not.**

## Quality (standing gates, carried from v1.10)

- [ ] **QUAL-01**: `go run ./tools/i18n` reports `0 offen, 0 verwaist` on every
      catalogue — **and** every operator-facing string is *collectable*, so that
      the report means what it says. 828 strings were measured outside the
      collector's sight on 2026-09-08
      (`.planning/audits/v1.6-I18N-828.md`); 12-01 re-measures them.
- [ ] **QUAL-02**: Every screen a person can see has been driven once through
      the running application in a browser. Larger here than in any phase that
      adds screens, because LANG-04 renames a contract field that every screen
      reads.

---

## Coverage

| Requirement | Phase | Status |
|---|---|---|
| LANG-01 | 12 | Pending |
| LANG-02 | 12 | Pending |
| LANG-03 | 12 | Pending |
| LANG-04 | 12 | Pending |
| LANG-05 | 12 | Pending |
| LANG-06 | 12 | Pending |
| LANG-07 | 12 | Pending |
| LANG-08 | 12 | Pending |
| QUAL-01 | 12 | Pending |
| QUAL-02 | 12 | Pending |

## Handed in from v1.10

- **GAL-07** — met in substance, not in its wording. Closed here by restating
  the requirement against the mechanism that exists.
- **QUAL-01** — the sentences that reach a screen past the gate
  (`WINDOWS.md` 3, 6, 18, 29) plus the 828 uncollectable strings.
- **Phase 10, criterion 6** — carried into LANG-01/LANG-02's sweep.
