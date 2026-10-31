# STATE history

This file holds the parts of `.planning/STATE.md` that describe closed milestones and old positions. They were moved out on 2026-10-26 (phase 27, plan 27-03, requirement TIDY-02) so that `STATE.md` has exactly one current position and the workflow tools can read it without refusing (`state.advance-plan` answered `ambiguous_position_phase` while three `Phase:` lines stood in the slice of the one section named `Current Position`).

The old `STATE.md` is the one at commit `58b2546786`; the blocks below were moved at that commit. They are moved verbatim: copied unchanged with `sed -n` line ranges, headings included, nothing reworded, nothing dropped. Their headings therefore repeat names that `STATE.md` also uses (`Current Position`, `Operator Next Steps`, `Performance Metrics`, `Session Continuity`, `Project Reference`, `Blockers`). No tool reads this file, so the repetition does no harm; the live ones are in `STATE.md`.

One fact the reader needs: the workflow tools overwrote the first `Phase:` line of the old file, the one in the block `Current Position (v1.10, ...)`, with "Phase: 27 — Repository tidy" and left the prose after it standing. That line is therefore not what v1.10 wrote; the prose that follows it is.

## Where each block of the old STATE.md went

| Block of the old STATE.md | Where it went |
|---------------------------|---------------|
| frontmatter | STATE.md (kept verbatim, apart from the progress counters) |
| `## Where we are (corrected 2026-10-12)` | STATE-HISTORY.md |
| `## State: Holzcloud CMS` (the heading only) | STATE.md (kept) |
| `### Project Reference` | STATE-HISTORY.md; STATE.md (renewed) |
| `### Current Position (v1.10, ...)` | STATE-HISTORY.md |
| `### Milestone Map` | STATE-HISTORY.md |
| `### Performance / Quality Notes` | STATE-HISTORY.md |
| `### Accumulated Context` with its four `####` sub-blocks | STATE-HISTORY.md |
| `### Quick Tasks Completed` | STATE.md (kept verbatim) |
| `### Performance Metrics` | STATE-HISTORY.md; STATE.md (renewed) |
| `### Session Continuity` | STATE-HISTORY.md; STATE.md (renewed) |
| `## Decisions` | STATE.md (kept verbatim) |
| `## Accumulated Context` (Roadmap Evolution and the old Blockers) | STATE-HISTORY.md; STATE.md (renewed `### Blockers`) |
| `## Current Position` (the old one) with both `### The milestone before this one (v2.1 ...)` and `(v2.0 ...)` blocks | STATE-HISTORY.md; STATE.md (renewed) |
| `## Operator Next Steps` (the old one: stale tag instructions for v2.0 and v2.1) | STATE-HISTORY.md; STATE.md (renewed) |
| `## Deferred Items` | STATE.md (kept verbatim) |

The blocks follow in the order they had in the old file.

---

## Where we are (corrected 2026-10-12)

**This block is the current one.** The blocks further down ("Current Position (v1.10)",
"Current Position" for v2.2, "Current focus: v2.4") were written at their own milestone
close and are history, not status.

- **Release:** 0.1.0 (tag `v0.1.0`, release page published 2026-10-18, first beta); before it 0.0.25 (2026-10-11). The count started again at 0.0.1 on
  2026-09-26; the software calls itself alpha.
- **Milestone:** v0.1.0 is released and its phases 20–23 are complete and verified; it is not yet archived under `.planning/milestones/` (the CLI finds no phases for it in ROADMAP.md). v2.4 and v2.5 are closed (2026-09-17). Since the restart, work
  runs as quick tasks (`.planning/quick/`, 31 directories), not as phases.
- **Latest work:** 0.0.20 SEO check/report, `llms.txt` · 0.0.21 security audit in four groups
  (order token, plugin/SMTP hardening, Go 1.26.8, pinned actions, hardened systemd unit) ·
  0.0.22/0.0.23 theme holzcloud (phone tables, holzWave colours) · 0.0.24/0.0.25 menus
  (own rail entry, rename, edit entries, phone list).
- **Open, needs a human:**
  - The hardened unit ran under systemd 252 in a container (phase 20, release notes of 0.1.0); not on a real server, and the plugins were measured on native arm64 only, not amd64.
  - Orphan remote branches (`backfill-releases`, `claude/*`, `feat/holzwave-farben`,
    `fix/tables-mobile`, `holzbar-colours`, `redirects-keep-language`, `tagfix`) can only be
    deleted in the GitHub UI; the proxy answers 403 to ref deletion.
  - `docs/offene-punkte.md`: all six items are built; nothing queued.
- **Next:** `/gsd-new-milestone` when the owner has the next goal.

### Project Reference

- Core value: One Go binary runs several websites without dependency soup
- Current focus: **v2.4 „What Has Accumulated“ (Phasen 17–18), eröffnet 2026-09-15.** Kein Funktionsmeilenstein und mit Absicht keiner: drei Meilensteine hintereinander haben geändert, was das Programm sagt — dieser ändert nichts, was ein Betreiber sieht. Er gibt es, weil das Fensterbuch zum ersten Mal leer ist, und ein leeres Fensterbuch ist kein sauberer Baum, sondern einer, in dem nichts mehr steht, das jemand aufgeschrieben hat. Der Bestand ist gemessen: `.planning/phases/17-what-has-accumulated/17-INVENTORY.md`. Einstieg: `.planning/REQUIREMENTS.md`, `.planning/ROADMAP.md`, `17-CONTEXT.md`. Was v2.3 war, steht in `milestones/v2.3-MILESTONE-AUDIT.md`.
- Constraints: Go + htmx + plain CSS + SQLite only — no deviations without explicit user approval
- Stack is a hard mandate: modernc.org/sqlite (pure-Go), html/template, log/slog, embed.FS, gorilla/csrf, alexedwards/scs, pressly/goose, goldmark, bluemonday
- Nothing loads at runtime: no CDN, no web fonts by URL, no third-party subresource of any kind

### Current Position (v1.10, geschrieben beim Abschluss von Phase 10 — die geltende steht unter `## Current Position`)

Phase: 27 — Repository tidy
letzte offene Phase des Meilensteins. Alle sechs Phasen von v1.6 (6–11) haben
ihre Pläne gefahren: 42 von 42.

Plan: keiner offen. **Phase 10 ist abgeschlossen, mit einer ausdrücklichen
Übergabe.** Am 2026-09-10 gemessen (`10-VERIFICATION.md`, `10-SECURITY.md`,
nachgeholter Code-Durchgang `10-REVIEW.md`), dann in einer Fixrunde geschlossen,
dann nach der Fixrunde erneut im Browser gefahren (Binär aus `64b4b92`):

- Verifizierung **5 von 6** (vorher 1 von 6). Kriterium 6 bleibt teilweise: die
  fünfzehn v1.6-Sätze aus `field.Check` und `internal/field/store.go`, die am
  i18n-Tor vorbei auf den Bildschirm kommen, sind bewusst an **Phase 12**
  übergeben (Fensterbuch 18).
- Sicherheit **58 von 59 geschlossen, 1 angenommen, 0 offen** (vorher 43/15/1),
  `SECURED`.
- Code-Durchgang: alle drei Critical und sieben der acht Warnungen geschlossen;
  WR-07 und IN-06 sind als bekannte Grenzen in `deploy/DEPLOY.md` beschrieben.

Die Fixrunde hat zwei Migrationen hinzugefügt (`00052 users.websites_limited`,
`00053 users.sso_username`), einen CLI-Befehl (`holzcloud user sso`) und strengere
Startprüfungen. Der CHANGELOG sagt Betreibern, was sie nach dem Aktualisieren
tun müssen — insbesondere: Redakteure, die vor dem Update ihre einzige Website
verloren haben, stehen danach bei „alle Websites" und müssen von Hand neu begrenzt
werden, weil kein Programm sie von nie begrenzten unterscheiden kann.

**Wie gemessen wurde, hatte eigene Fehler, und alle stehen in den Berichten
und Commits:** der erste Prüflauf teilte sich einen Arbeitsbaum (zwei Mutationen
blieben stehen, als 66 Agenten am Wochenlimit starben); der isolierte Nachlauf
stand auf `d4ca500`; zwei erfundene Commit-IDs (vor bzw. per `--amend`
berichtigt); ein zsh-Probenskript, das nichts sicherte und „byte-gleich" über zwei
leere Prüfsummen meldete; ein Commit ohne i18n-Tor; eine Probe, die einen
Buildfehler als rot zählte.

Status: Ready to plan
mit einer benannten Übergabe an Phase 12 (v2.0), und archiviert. Die frühere Zeile
„gebaut, aber nicht abgeschlossen" galt vom 2026-09-09 bis zur Fixrunde.

**Was 10-10 auf dem Bildschirm fand und kein Tor sah:** fünf deutsche Sätze auf
einer englischen Verwaltung, während `go run ./tools/i18n` auf allen vier
Katalogen `0 offen, 0 verwaist` meldete. Einer davon war v1.6s eigener
(`internal/admin/field.go`, aus Phase 8, `48e5b1d`) und ist am 2026-09-09
geschlossen (`46e0722`, Fensterbuch 13); die vier übrigen sind vorbestehend und
gehören Phase 12 als deren neuntes Kriterium (Fensterbuch 12, 14, 15, 16). Der
Grund, warum kein Tor sie sehen konnte, ist in beiden Hälften lehrreich: die
Sätze sind verkettet, tragen an der Stelle, die der Sammler liest, also gar
kein Literal — und `"Felder – "` trägt weder Umlaut noch Eszett noch deutsche
Anführungszeichen, wäre also auch einem Tor entgangen, das nach deutsch
*aussehenden* Literalen sucht. Gefunden allein durch Umschalten und Hinsehen.

Offen aus Phase 8: `V2-18` — Kennungen in einem Textbausteinwert reisen beim
Archivweg **nicht** übersetzt (Bild, Verweis, Schlagwort). ~~Der
`&#8592;`-Befund in `field_list.html`~~ — **geschlossen am 2026-09-06 im
Schnellauftrag 260906-m9z**, und die Begründung des Aufschubs, die hier stand
(„die Zeichenkette **ist** der Katalogschlüssel, ein Flick verwaist drei
Schlüssel in vier Katalogen"), war falsch: der gefahrene Flick wechselte nur
`t` auf `th`, die Zeichenkette blieb byte-gleich, kein Schlüssel verwaiste.
Fensterbuch 5 trägt die Berichtigung ausgeschrieben statt still gelöscht
~~Offen aus dem stehenden Tor (Phase 6)~~ — **erledigt, und dieser Eintrag war
seit dem 4. September falsch.** `06-VERIFICATION.md:212-260` trägt den
vollständigen Nachdurchgang: alle fünf Archive über die Verwaltung
hochgeladen, `kontaktformular`s eigene Migration beim Einspielen angewandt und
mit ihrer sha256 vermerkt, `/suche?q=Willkommen` mit einem echten Treffer,
`[[jahr]]`, `[[formular]]`, `[[bestellung]]` auf der Startseite und
`nicht-gefunden` am 404-Haken. Kriterium 6 steht dort als **MET**, mit dem
alten PARTIAL-Urteil darunter erhalten. Nur STATE.md hat das nie nachgezogen

~~Offen aus dem stehenden Tor (Phase 7)~~ — **ebenfalls erledigt.**
`07-VERIFICATION.md:212-219` zeigt `code` innerhalb eines Bausteins auf der
öffentlichen Seite gefahren, mit dem gerenderten HTML im Bericht: eine eigene
Bausteinart `Ausstattungskasten`, ein `<script>` im Wert, und die Ausgabe
maskiert in `<pre><code>`. Auch hier hat nur STATE.md nicht nachgezogen

**Wirklich offen aus Phase 7** ist etwas anderes, und es steht in der
Kopfzeile des Berichts: Kriterium 1, zweiter Satz — „geleert" ist von „dieses
Formular trug das Feld nie" **nicht** unterscheidbar, weil `field.Clean` jeden
leer trimmenden Wert verwirft. Phase 9 war angewiesen, diese Unterscheidung zu
erben. Sie hat es nicht getan, sondern sie **ausdrücklich abgelehnt**, und das
ist die richtige Antwort: `internal/csvimport/row.go`s `update()` schreibt
hin, dass eine CSV-Datei den Unterschied gar nicht tragen kann, und leitet
daraus ab, dass Leeren aus einem CSV-Update nicht ausdrückbar ist. Die Lücke
ist damit nicht geschlossen, sondern eingegrenzt: sie besteht im Formularpfad
und nirgends sonst

Dazu **Fenster Nr. 3**: die Ablehnungsgründe aus `internal/field/field.go`
erschienen bei englischer Oberfläche auf Deutsch — vorbestehend, gegen
`60ff5b2` geprüft, in `.planning/WINDOWS.md` eingetragen. Der Umfang dieses
Fensters ist inzwischen gemessen und ist grösser als drei Sätze: siehe
`.planning/audits/v1.6-I18N-828.md`
Last activity: 2026-10-26 — Phase 26 complete, transitioned to Phase 27

### Milestone Map

**v1.6 — Inhaltsmodell und Zugang.** Phases 6–11. Numbering continues from v1.0
and never restarts; the five v1.0 phase directories are archived under
`.planning/milestones/v1.0-phases/`. There is no v1.5 milestone shell — its three
phases were renumbered into this one as 7, 8 and 9.

| Phase | Name | Requirements | Count | Status |
|-------|------|--------------|-------|--------|
| 6 | Aufräumen | MAINT-01…05 | 5 | Plans 7/7 — Abnahme offen (Browserhälfte des Tors) |
| 7 | Field Kinds | FIELD-01…08 | 8 | Plans 7/7 — Abnahme offen (eine Browserzeile, Fenster Nr. 3) |
| 8 | Snippets Carry Fields | SNIP-01…05 | 5 | **Abgeschlossen** — 5/5 Pläne, verifiziert, Sicherheitsprüfung abgelegt |
| 9 | CSV Import | IMP-01…10 | 10 | **Abgeschlossen** — 6/6 Pläne, 1 kritischer + 8 Warnungen behoben, 4 Browserfunde, 3 Verifikationslücken + 1 Sicherheitsbefund geschlossen |
| 10 | Authentik Forward-Auth | SSO-01…11, QUAL-01, QUAL-02 | 13 | **Geplant** - 10 Plaene in 9 Wellen; der Planer fand 13 Irrtuemer, darunter D-01s zweiten Weg: Rechte bei jeder Anmeldung neu zu setzen erreicht „null heisst jede Website“ durch **Subtraktion** |
| 11 | Galerie | GAL-01…07 | 7 | **Wellen 1-2 von 5 ausgefuehrt** - Lichtkasten, Album-Speicher, Verwaltungsbereich, Anzeigemodus |
| 12 | The Codebase Speaks English | LANG-01…08 | 8 | Not started — **die Freigabe damit ist 2.0** (brechender Vorlagen-Vertrag) |

**Execution order: 6 → 7 → (8 ∥ 9 ∥ 11) → 10 → 12.** The one real dependency inside the
milestone is that Phase 9 needs Phase 7's multi-value encoding (Phase 7's build
step ①). Once that lands, Phases 8 and 9 are independent of each other and may
run in parallel. Phase 10's file set is disjoint from every other phase's and
can move anywhere; it is scheduled last because it carries the milestone's
close-out gate.

**UI phases:** 7, 8 and 9 all involve admin screen work. Phase 9's mapping and
dry-run screens are the largest UI surface in the milestone. Phases 6 and 10 are
not UI phases.

**Research flags:** Phase 7 wants a UI-SPEC for `bereich` alone (the
invisible-slider problem has three candidate answers and the choice is a design
question). Phase 9's dry-run report screen is a real information-design problem.
Phase 10 wants `/gsd-discuss-phase` for the whole phase — not for lack of
research but because every remaining question there is a policy decision.
Phases 6 and 8 follow standard patterns.

**Migration numbers claimed:** Phase 7 took `00046`, Phase 8 took `00047` and
`00048` (the latter from the review-fix round). **Phase 9 takes `00049`** — the
staging table for the uploaded file (D-01). Phases 6 and 10 need none; Phase 11
needs one for albums.

Coverage: 56 / 56 requirements mapped. Orphans 0, duplicates 0.

### Performance / Quality Notes

- Hard stack: Go 1.26 + htmx 2.x + plain CSS + SQLite (modernc.org/sqlite) — no exceptions
- UI aesthetic: schlicht/modern (Linear/Ghost/Vercel feel), OKLCH color tokens, @layer cascade, 8px spacing scale, system font stack, view transitions
- Target: linux/amd64 single binary (retargeted from arm64/Pi on 2026-09-03)
- Go patterns: 1.22+ stdlib ServeMux, slog structured logging, embed.FS for all assets/templates/migrations
- SQLite: dual-pool (write pool MaxOpenConns=1, read pool higher), WAL + busy_timeout=5000 + foreign_keys=ON on every connection
- Migrations stand at 00048 (`00048_snippet_group_namespace.sql`); 00046 was Phase 7 (`darstellung`, `max_werte`, `bereich` bounds), 00047 and 00048 were Phase 8 (`snippet_id` + index swap + `snippets.fields`, then the group namespace). **Phase 9 takes 00049** (the staging table for an uploaded CSV). Phases 6 and 10 need none

### Accumulated Context

#### Decisions

- Stack pivot from PHP + SQLite to Go + htmx + SQLite; PHP implementation preserved on legacy/php-stack branch, not ported file-by-file
- modernc.org/sqlite chosen over mattn/go-sqlite3 for pure-Go cross-compilation (no CGO)
- alexedwards/scs chosen for server-side sessions stored in SQLite (no Redis dependency)
- gorilla/csrf chosen for CSRF; htmx integration requires hx-headers on <body> element — hidden form fields are NOT sent by htmx AJAX requests
- goldmark + bluemonday pipeline for Markdown: goldmark renders to raw HTML, bluemonday sanitizes before template.HTML cast (never skip sanitization)
- goose for migrations: SQL files embedded via embed.FS, run automatically at startup
- v1.0: 5-phase coarse roadmap, Foundation → Auth+Shell → Multi-Site+Pages+Public → Templates+Menus+Media → Polish+Users+Deploy
- v1.6: 5-phase coarse roadmap, Aufräumen → Field Kinds → Snippets Carry Fields → CSV Import → Authentik Forward-Auth. The middle three follow the milestone goal as PROJECT.md states it: every field kind, in every carrier, then content as a table
- v1.6: QUAL-01 (five languages) and QUAL-02 (browser pass) are standing gates, not deliverables. Counted once in the last phase for traceability; repeated verbatim as the final success criterion of every other phase so no phase can close without them
- v1.6: snippet fields reuse `page_field_defs` with a `snippet_id` column — explicitly not a third field table
- v1.6: `darstellung`, `max_werte` and the `bereich` bounds get their own columns rather than riding in `auswahl` — a line that is not an option is exactly the ambiguity the one-value-per-line encoding was careful to avoid. Phase 7 therefore ships a migration
- v1.6: a multi-valued field is stored one value per line in the existing string slot, and crosses a CSV cell as a pipe. The delimiter is already illegal inside a value because `SplitChoices` reads options one per line — correct for closed vocabularies, which is all v1.6 has
- v1.6: an Authentik session satisfies the second factor unconditionally (operator's decision, 2026-09-03). One home: `auth.MustHaveSecondFactor` at `internal/auth/twofactor.go:44`. The dependency must be stated in DEPLOY.md and shown in the admin
- **2026-09-06: the code speaks English.** The project is open source, so identifiers, comments, test names, the template data contract and the SQL columns together with the catalogue keys all become English — full scope, chosen by the developer. The template contract is a **hard break with a version bump**, so the release carrying Phase 12 is **2.0**. One word list, `.planning/GLOSSARY.md`, is the single place a German term's English word is fixed; a term translated without an entry gets one in the same commit. `.planning/` itself is **not** translated — it is the project's own record, and rewriting a record's language is how it stops being one
- v1.6: the CSV import offers both an existing website and a new one, chosen on screen 1 (operator's decision, 2026-09-03) — a deliberate departure from `wordpress.go`'s always-a-new-website rule, answered by the update-or-skip choice plus the dry run
- README's `## License` said MIT while LICENSE carries the full GNU AGPL-3.0; corrected to AGPL-3.0 with a link to LICENSE. A documentation-defect fix, not a relicensing — revert commit d089e3d if MIT was ever the intent
- CHANGELOG.md follows the Keep a Changelog skeleton but writes entries as full sentences, matching the register of SECURITY.md and CONTRIBUTING.md; the choice is stated at the top of the file so the next entry does not revert to bullets
- The public record begins at v1.4 and no pre-1.4 releases are invented. README, CONTRIBUTING and CHANGELOG all say development happened in a private repository first; none of them names that repository or its visibility

#### Known Risks

- **Die Website-Isolation ist die tragende Schwachstellenfamilie dieses Projekts, und sie ist dreimal aufgetreten.** Am 2026-09-06 vier Menüeintrags-Handler (`de4a1ce` beweist, `5e453a9` behebt), am 2026-09-07 das Produkt-Speichern (`071bead`/`2e43bc1`) und die Bestellungs-Anzeige (`fb9c76a`). Die Ursache ist jedes Mal dieselbe: `auth.RequireWebsiteAccess` liest die Website **aus dem Pfad**, der Handler nimmt eine zweite Kennung ebenfalls aus dem Pfad, und niemand verbindet die beiden. **Wo der Speicher die Zuordnung selbst erzwingt — `term`, `kind`, `block`, `field` — ist es nie passiert.** Wo sie im Aufrufer liegt, ist es passiert. Jede neue Ressource mit einer eigenen Kennung gehört deshalb nach `term/store.go:284-311` gebaut, nicht nach `menu/store.go`

- FIELD-02 (multiple choice) is the first field value that is not a single string. The encoding chosen in Phase 7 is load-bearing for Phase 9's importer — decide it before either phase writes code
- A CHECK constraint at the table head cannot be loosened in SQLite without a full table rebuild, and `pages` has foreign-key children. `page_field_defs.art` carries no CHECK, so a new field *kind* needs no migration — but Phase 7 ships 00046 anyway, because `darstellung`, `max_werte` and the `bereich` bounds were given their own columns rather than being squeezed into `auswahl`. `users.role` DOES carry a table-level CHECK at `00001:7`: Phase 10 must not invent a third role
- Phase 8's `snippet_id` column collides with the partial unique index `idx_page_field_defs_kennung_oben` — an index swap, not a rebuild. **That index was already replaced by `00038:52–56`, not left as `00029` wrote it.** Read 00029 AND 00038 before writing 00047; 00038 is a line-for-line template and its own comment explains the operation
- No JavaScript beyond htmx: the button row, the multiple choice and the slider must all work as plain form controls with a full-page fallback
- Every new user-visible string must land in de/en/es/fr/it; `go run ./tools/i18n` must say `0 offen, 0 verwaist`
- Draft page leakage: always include AND status='published' in every public page query — applies to a Term or Ref field resolving on the public site too
- Vary: HX-Request header required on any handler returning different content based on HX-Request
- **The upgrade path from an old installation is broken and no test can see it (found 2026-09-04, own task pending).** Migration `00036_content_types.sql:60-62` builds a partial index on `pages.deleted_at`; the column is added by `00008_revisions_locking_trash.sql:15`. A fresh database migrates 0 → 45 cleanly, but an installation whose recorded version 8 predates an edit to `00008` lacks the column, dies at 36 with `no such column: deleted_at`, and the server refuses to start. goose never re-runs a recorded version and no migration rebuilds `pages`. The fix needs a test that migrates from an old snapshot — a suite that only ever migrates from zero is structurally blind to this. Evidence in `.planning/phases/06-aufr-umen/06-07-SUMMARY.md`
- The defects this project actually shipped were found in the browser, not by the suite — the QUAL-02 pass is not optional
- **Die Ablehnungsgründe in `internal/field/field.go` sind nie übersetzt und der QUAL-01-Zähler sieht sie nicht** (beobachtet 2026-09-05, Fenster Nr. 3). `rangeReason` (674–682) und die Längen- und Mehrwertmeldungen (710, 778–781) werden durch blosse Zeichenkettenverkettung ohne `i18n.N` gebaut; bei englischer Oberfläche erschien `Ausstattung: höchstens 3 Werte, ausgewählt sind 4.` auf Deutsch. Vorbestehend, gegen `60ff5b2` geprüft — Phase 7 ist der Konvention gefolgt und hat die Fläche verbreitert. `go run ./tools/i18n` kann deshalb `0 offen, 0 verwaist` melden, während eine Person deutsche Ablehnungen liest. Jede Validierungsrückgabe in `field.go` umzuschreiben ist eigene Arbeit

#### Todos

- **Offener Widerspruch, benannt am 2026-09-07 und bewusst nicht still entschieden: `featured_media_id` wird nirgends gegen die Website geprueft** — weder im Produktformular noch im Seitenformular. Das ist **kein** Fehler derselben Familie wie die drei Isolationsluecken, sondern ein Konflikt zwischen zwei Regeln dieses Projekts. `HandleMediaServe` haelt die websiteuebergreifende Wiederverwendung von Medien ausdruecklich fest („es wuerde die Vorschau in der Verwaltung zerbrechen"), und `/media/{websiteID}/…` ist fuer jede aktive Website ohnehin abrufbar. Die Projektregel dagegen lautet: jede Ressource gehoert zu genau einer Website, nichts wird geteilt. Eine Pruefung einzubauen waere also nicht die Behebung eines Fehlers, sondern die stille Umkehrung eines dokumentierten Entscheids — und sie beruehrt beide Formulare. **Zu entscheiden, nicht zu erben.** Belege in `.planning/quick/260907-product-scope/REPORT.md` §6

- Phase 6 is done and none of it must be re-discovered. The i18n catalogues already matched the tool's output (quick task `260903-bsk`, `.planning/WINDOWS.md`); all three former defects are closed — `06-03` deleted the indentation claim from the `writeCatalog` doc comment (do **not** go looking for `tools/i18n/main.go:287`; the line and the claim are both gone, and `06-07` retired the ROADMAP note that pointed at them) and made the tool state which regional catalogues it only reads, and `06-06` made CI rebuild and compare all ten artifacts before any test runs
- Phase 6 ordering: rebuild-and-hash-compare in CI first, promote the test skips second. Any catalogue reformat is its own commit, proven with a `jq -S` semantic diff
- Vor dem Ausliefern zu entscheiden: die eine ungefahrene Browserzeile aus Phase 7 (`code` innerhalb eines Blocks auf der öffentlichen Seite — im Test gedeckt, im Browser nicht gesehen) und Fenster Nr. 3 (unübersetzte Ablehnungsgründe). Beide stehen als offen, nicht als bestanden
- Phase 8 must edit `internal/field/store.go:53` first — the missing `AND snippet_id IS NULL` puts every snippet field on every page's edit form, silently
- Phase 10 must gate on the existing `web.ClientIPResolver.IsTrustedPeer` (`internal/web/clientip.go:49–52`), not a second peer check; the middleware goes in at `cmd/holzcloud/main.go:968` between `setupGuard` and `requireAuth`

#### Blockers

- (none)

### Performance Metrics

**Velocity:**

- v1.0: 13 plans across 5 phases, all complete 2026-04-14
- v1.6: 19 plans complete across Phases 6, 7 and 8

| Plan | Duration | Tasks | Files |
|------|----------|-------|-------|
| Phase quick-260903-ato P01 | 35m | 3 tasks | 35 files |
| Phase quick-260903-hkh P01 | 7m | 3 tasks | 3 files |
| Phase quick-260903-rqq P01 | 121m | 3 tasks | 18 files |
| Phase quick-260903-t0s P01 | 26m | 3 tasks | 18 files |
| Phase 6 P01 | 6 min | 1 tasks | 2 files |
| Phase 06 P03 | 8 min | 3 tasks | 2 files |
| Phase 06 P04 | 27 min | 3 tasks | 11 files |
| Phase 06 P02 | 13 min | 2 tasks | 2 files |
| Phase 06 P05 | 21 min | 3 tasks | 4 files |
| Phase 06 P07 | 18 min | 4 tasks | 13 files |
| Phase 07 P01 | 22 min | 3 tasks | 10 files |
| Phase 07 P02 | 21 min | 3 tasks | 11 files |
| Phase 07 P03 | 17 min | 3 tasks | 12 files |
| Phase 07 P04 | 22 min | 3 tasks | 7 files |
| Phase 07 P05 | 19 min | 4 tasks | 13 files |
| Phase 07 P06 | 9 min | 3 tasks | 4 files |
| Phase 07 P07 | 37 min | 3 tasks | 13 files |
| Phase 08 P01 | 10 min | 3 tasks | 12 files |
| Phase 08 P02 | 12 min | 2 tasks | 14 files |
| Phase 08 P03 | 9 min | 2 tasks | 9 files |
| Phase 08 P04 | 10 min | 3 tasks | 8 files |
| Phase 08 P05 | 32 min | 3 tasks | 9 files |
| Phase 11 P06 | 21 min | 3 tasks | 7 files |
| Phase 10 P01 | 42 min | 3 tasks | 4 files |
| Phase 10 P03 | 23 min | 3 tasks | 5 files |
| Phase 10 P04 | 15 min | 2 tasks | 2 files |
| Phase 11 P07 | 40 min | 2 tasks | 9 files |
| Phase 10 P05 | 40 min | 2 tasks | 2 files |
| Phase 10 P06 | 25 min | 3 tasks | 7 files |
| Phase 10 P07 | 28 min | 2 tasks | 2 files |
| Phase 10 P08 | 41min | 3 tasks | 5 files |
| Phase 10 P09 | 15 min | 2 tasks | 5 files |
| Phase 10 P10 | 55 min | 3 tasks | 2 files |
| Phase 20 P01 | 55min | 2 tasks | 6 files |
| Phase 20 P02 | 25min | 3 tasks | 2 files |
| Phase 20 P03 | 45min | 3 tasks | 3 files |
| Phase 21 P01 | 8 min | 2 tasks | 1 files |
| Phase 21 P02 | 10 min | 2 tasks | 3 files |
| Phase 21 P03 | 12 min | 2 tasks | 3 files |
| Phase 21 P04 | 25 min | 2 tasks | 3 files |
| Phase 21 P05 | 35min | 2 tasks | 4 files |
| Phase 21 P06 | 8 min | 2 tasks | 3 files |
| Phase 22 P01 | 22 min | 2 tasks | 1 files |
| Phase 22 P02 | 13 min | 3 tasks | 2 files |
| Phase 22 P03 | 14 min | 2 tasks | 1 files |
| Phase 22 P04 | 17 min | 3 tasks | 7 files |
| Phase 22 P05 | 8 min | 2 tasks | 2 files |
| Phase 23 P01 | 35min | 2 tasks | 2 files |
| Phase 23 P02 | 15min | 2 tasks | 3 files |
| Phase 23 P03 | 15min | 2 tasks | 1 files |
| Phase 23 P04 | 25min | 2 tasks | 1 files |
| Phase 26 P03 | 4min | 2 tasks | 3 files |

### Session Continuity

To resume: read `.planning/ROADMAP.md` for the v1.6 phase structure (Phases 6–10)
and its *Standing Gates* section. Requirement IDs are in
`.planning/REQUIREMENTS.md`; the working list most of them came from, with the
size and location of each item, is `docs/offene-punkte.md`.

Next command: `/gsd-plan-phase 9 --skip-research` — `09-CONTEXT.md` steht mit
30 Entscheiden, der Kantentest ist mit 34 von 34 geschlossen abgelegt
(`09-EDGES-*.json`), und der Musterabgleich läuft. Danach `gsd-planner`,
`gsd-plan-checker` mit der Überarbeitungsschleife, Ausführung in Wellen,
Code-Review, Behebungsrunde, **dann** der Browserdurchgang (in Phase 7 und 8
hat er sonst beide Male Änderungen verpasst, die nach dem Durchgang landeten),
dann Verifikation und Sicherheitsprüfung.

**Zwei Lehren aus Phase 8, die für Phase 9 gelten:** Zähl-Tore zeilenweise
gegen den **nach**-Zustand abzählen statt schätzen — Welle 3 war die einzige
ohne Abweichung, und zwar genau deshalb, weil ihr Plan die Zeilennummern
tabelliert hatte. Und der Browserlauf gehört **hinter** die Behebungsrunde.

Danach: Phase 11 (Galerie) kann parallel laufen. Phase 10 (Authentik) wird
geplant und **vorgelegt, nicht ausgeführt** — jede verbleibende Frage dort ist
eine Richtlinienentscheidung über die eigene Authentik-/Caddy-Anlage des
Entwicklers. Weiterhin offen und unabhängig davon: `/gsd-verify-work 6` für die
Browserhälfte des stehenden Tors und `/gsd-verify-work 7` für die eine
ungefahrene Zeile (`code` im Block, öffentlich)

**Last session:** 2026-10-26T15:41:20.623Z
**Stopped at:** Completed 26-03-PLAN.md
**Resume file:** None

## Accumulated Context

### Roadmap Evolution

- Phase 11 added: Galerie — Vergroessern/Lightbox (:target, kein JS), Album als wiederverwendbares Ding, Diashow via CSS scroll-snap. Haengt an Phase 7 (Mehrwert-Kodierung FIELD-07), nicht an Phase 10. Passt nicht ins v1.6-Meilensteinziel; per Entwicklerentscheid trotzdem hier.

### Blockers

- ~~07-04 gemeldet, nicht behoben: internal/bundle/import.go importFieldValues schreibt Feldwerte mit field.Encode direkt, ohne CheckAll und ohne Clean.~~ **ERLEDIGT in 07-05 (7cb09f4).** Entschieden wurde gedeckt und nicht vertagt: ein Archiv ist eine Datei, die jeder bearbeiten kann, alle anderen Schreibwege sind gedeckt, und seit 07-04 kuerzt trimTo nichts mehr — CheckAll ist damit die einzige Stelle, an der das Bytebudget ueberhaupt noch gilt. importPages liest die tatsaechlich angelegten Definitionen ueber s.Fields.List und reicht sie in importFieldValues; dort laufen field.Clean und field.CheckAll, ein beanstandeter Wert wird entfernt und namentlich in den Bericht geschrieben, nie die ganze Seite verworfen. TestArchivwerteGehenDurchDieselbePruefung beweist es, Gegenprobe mit deaktivierter Wache gefuehrt (vier Behauptungen fallen). Offene Blocker: keine
- Plan 10-01: vier Zaehl-Schranken der Phase 10 sind gegen den Baum vom 2026-09-07 geeicht und seit Phase 11 veraltet (Migrationen 49 statt 50, Pakete 40 statt 41, Admin-Vorlagen 66 statt 68, Zeichenketten 1277 statt 1311). Die Plaene 10-02 bis 10-09 tragen dieselben Zahlen — gemessene Werte aus 10-01-SUMMARY uebernehmen
- Offen (WINDOWS.md Eintrag 8): eine Album-Diashow zeigt ihre Lichtkasten-Bedienelemente in der Sprache des Besuchers und den Namen ihres Schiebefelds auf Deutsch. render.go:212 uebersetzt mit s.text (nur beim Speichern gesetzt), die Bedienelemente ueber expand.go:133 mit set.t (bei der Anfrage). Die Behebung verschiebt die Grenze zwischen Speicherzeit und Anfragezeit — Architekturentscheid des Entwicklers.
- 10-08: das isTrustedProxy-Tor des Plans schliesst '^\./\.planning/' aus, grep gibt hier aber Pfade ohne './' aus — es liest 14 statt 0. Korrigierte Form: grep -v '^\(\./\)\?\.planning/'. Die Eigenschaft selbst gilt (0 ausserhalb .planning/). Fuenfter Fall in dieser Phase.

## Current Position

Phase: Not started (defining requirements)
Plan: Not started
Status: Defining requirements
Last activity: 2026-10-20 — Milestone v0.1.1 started

### The milestone before this one (v2.1, closed 2026-09-13)

Phase: none open. **Milestone v2.1 is closed** (2026-09-13), 14 of 14
requirements satisfied, audit in `.planning/milestones/v2.1-MILESTONE-AUDIT.md`.
Phases 13 and 14 both verified: `13-VERIFICATION.md`, `14-VERIFICATION.md`.
Status: Complete
Last activity: 2026-09-28 - Completed quick task 260928-f2n: MCP-Upload-Link für grosse Mediendateien (create_upload_link)
running server; CHANGELOG `## 2.1` written

The GSD plugin is still not installed in this execution container (no `/gsd-*`
commands, no `gsd-tools` binary) — the workflow was carried by hand in the same
shape, with the same artifacts in the same order.

**What v2.1 did.** Phase 13 built the channel: the page's language reaches a
plugin's `sdk.T`, a theme carries its own `lang/<tag>.json`, the operator can
override any key per website and language, the eight shipped themes are
generated from one vocabulary of 138 keys, and TEMPLATE-SPEC §2.5 now says the
opposite of what it said three days earlier with the old argument quoted and
answered. Phase 14 rebuilt the contact form on top of it: per-field refusals in
all eight themes, a receipt to the sender behind a second permission *and* the
operator's switch, replies from the admin, consent stored with its wording and
its moment, a quarantine for what the spam traps refuse, attachments the plugin
never sees the bytes of, and conditional fields in a second step without
JavaScript.

**Three bugs that were nobody's feature** were found by building it, not by a
test. The largest: every answer to a form submission was built from the page's
slug, so a form on the **start page** redirected to `/home`, which the public
side redirects to `/` without its query — no visitor of a start page had ever
seen a thank-you or a reason. `ContentIn.Path` and a hidden field fix it.

**One thing is deliberately open**, carried unchanged from v2.0: a plugin's
`plugin.json` name and description are not translated.

---

### The milestone before this one (v2.0, closed 2026-09-13)

Phase: 12 — The Codebase Speaks English (milestone v2.0)
Plan: none open. **Phase 12 is finished and verified.**
Status: Complete
Last activity: 2026-09-28 - Completed quick task 260928-f2n: MCP-Upload-Link für grosse Mediendateien (create_upload_link)
`12-VERIFICATION.md` written

The GSD plugin is not installed in the execution container that carried this
milestone (no `/gsd-*` commands, no `gsd-tools` binary) — only the artifacts it
wrote. The workflow was therefore carried by hand in the same shape: the same
artifacts, the same order, the same gates.

**What the phase did**, in the order the roadmap asked for: the template
contract with the eight themes (LANG-04), migration 00054 (LANG-05), the
collectable strings (QUAL-01 / criterion 9), identifiers and comments (LANG-02,
LANG-03, LANG-01), the catalogue flip (LANG-06), the gates (LANG-07) and the
browser pass (QUAL-02).

**Three decisions were taken during the phase and written into
`12-CONTEXT.md`** rather than into a commit message: §3a landed in full (the
plugin translation channel, smaller than the measurement suggested because most
of the 274 was a visitor's text), §3c is new (the MCP surface speaks English —
not foreseen, found because `tools/english` reads string literals as well as
comments), §3d makes §3b enforceable as the gate's `germanVoice` list.

**Two things are deliberately open** and named in `12-VERIFICATION.md`: the
public side still has no translation channel at all, and a plugin's
`plugin.json` name and description are not translated.

## Operator Next Steps

- **Die Release-Tags `v2.0` und `v2.1` legt der Betreiber an, und zwar aus einem
  Grund, der zweimal falsch aufgeschrieben war.**

  Erste Fassung: "der Zugang darf Branch-Refs schreiben, Tag-Refs nicht." Das
  war eine Vermutung aus dem blossen Statuscode. Die Antwort steht im Lauf, der
  es von der anderen Seite versuchte:

  ```
  ! [remote rejected] v2.0 -> v2.0 (refusing to allow a GitHub App to create or
    update workflow `.github/workflows/ci.yml` without `workflows` permission)
  ```

  Ein neuer Tag-Ref hat keine Grundlage, gegen die er verglichen wuerde, also
  zaehlt jede Workflow-Datei im Baum als neu hinzugefuegt — und kein
  `permissions:`-Schluessel kann GITHUB_TOKEN den Bereich geben, der das
  erlaubte. Es ging nie um die Art des Refs.

  Zweite Fassung, auch unvollstaendig: "`release.yml` kann den Tag jetzt per
  `workflow_dispatch` selbst praegen." Kann es — aber nicht von hier aus. Der
  Lauf baut und testet sieben Minuten lang und scheitert am letzten Schritt mit
  `HTTP 403: Resource not accessible by integration`, weil ein Lauf, den eine
  App ausgeloest hat, deren Rechte traegt und nicht die des Repositorys.

  **Der ganze Grund, gemessen am 2026-09-14:** bis zum 11.09. wurde in dieser
  Arbeitsumgebung unter dem KONTO DES BETREIBERS gearbeitet — die Commits bis
  `ef4873a` tragen `holz41289 <6815936+holz41289@users.noreply.github.com>`, und
  die Tagger von `v1.8`, `v1.9` und `v1.10` ebenso. Seit `5a46c18`, der
  Eroeffnung von v2.0 am selben Abend, ist es eine App-Integration, und die
  Commits tragen `Claude <noreply@anthropic.com>`. v2.0 ist genau der erste
  Meilenstein, dessen Tag sich nicht mehr pushen liess.

  Es ist also nichts kaputtgegangen und keine Regel dazugekommen: ein Mensch
  darf einen Tag-Ref mit Workflow-Dateien darin anlegen, eine App nicht.

  **Zu tun, unter dem Konto des Betreibers, einer von beiden Wegen:**

  - *Actions → Release → Run workflow*, `tag: v2.0`, `commit: 9fe3279`; danach
    `tag: v2.1`, `commit: 32d545d`. Dass ein von Hand ausgeloester Lauf das
    normale Token des Repositorys bekommt, ist das uebliche Verhalten von
    GitHub Actions — hier nicht nachgemessen, weil diese Sitzung den Knopf
    nicht als Mensch druecken kann.
  - Oder aus einer Arbeitskopie:

    ```bash
    git tag -a v2.0 9fe3279 -m "2.0 — The Codebase Speaks English"
    git tag -a v2.1 32d545d -m "2.1 — The Public Side Speaks the Visitor's Language"
    git push origin v2.0 v2.1
    ```

  Bis dahin stempelt `image.yml` (`git describe --tags`) jedes Abbild als
  `v1.10-…` statt als `v2.0` oder `v2.1`.
- Nothing open in phase 12. The milestone is closed: `12-CONTEXT.md` and
  `12-VERIFICATION.md` are archived under `.planning/milestones/v2.0-phases/`,
  `REQUIREMENTS.md` and `ROADMAP.md` are copied beside them as
  `v2.0-REQUIREMENTS.md` / `v2.0-ROADMAP.md`, the audit is
  `v2.0-MILESTONE-AUDIT.md`, and the active requirements file is removed as it
  is at every close. Ready to tag as **2.0**.
- The open decision worth the next milestone's attention: the public side has no
  translation channel, so a French-language website sells in German. It needs
  the theme and the handler decided together — a feature, not a cleanup.

