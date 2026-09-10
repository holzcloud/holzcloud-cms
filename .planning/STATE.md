---
gsd_state_version: 1.0
milestone: v1.6
milestone_name: Inhaltsmodell und Zugang
current_phase: 10
current_phase_name: Authentik Forward-Auth
status: in_progress
stopped_at: >-
  Phase 10 abgeschlossen nach Fixrunde und zweitem Browserdurchgang:
  Verifizierung 5/6 (Kriterium 6 an Phase 12 uebergeben, WINDOWS 18),
  Sicherheit SECURED (58 geschlossen, 1 angenommen). Meilenstein bereit fuer
  den Abschluss.
last_updated: "2026-09-10T18:00:00.000Z"
last_activity: 2026-09-10
state_head: c2dc0b31c6f0ddd178924f9cfd97ab0ca7219c33
progress:
  total_phases: 6
  completed_phases: 6
  total_plans: 42
  completed_plans: 42
  percent: 100
---

## State: Holzcloud CMS

### Project Reference

- Core value: One Go binary runs several websites without dependency soup
- Current focus: **v1.6 ist inhaltlich fertig.** Phase 10 — Authentik Forward-Auth, 9 Wellen, 10/10 Plaene; Phase 11 — Galerie, 7/7 Plaene. Alle 13 Anforderungen der Phase 10 stehen auf Complete (SSO-03 und SSO-04 sind mit 10-10 nachgetragen worden — sie waren gefahren und belegt, aber im Register offen). Offen bleibt der Befund aus 10-10: fuenf deutsche Saetze auf einem englischen Verwaltungsbildschirm, WINDOWS.md 12–16.
- Constraints: Go + htmx + plain CSS + SQLite only — no deviations without explicit user approval
- Stack is a hard mandate: modernc.org/sqlite (pure-Go), html/template, log/slog, embed.FS, gorilla/csrf, alexedwards/scs, pressly/goose, goldmark, bluemonday
- Nothing loads at runtime: no CDN, no web fonts by URL, no third-party subresource of any kind

### Current Position

Phase: 10 — Authentik Forward-Auth (**10/10 Pläne ausgeführt**), zugleich die
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

Status: **Der Meilenstein v1.6 ist gebaut und verifiziert, mit einer benannten
Übergabe an Phase 12.** Bereit für den Meilenstein-Abschluss. Die frühere Zeile
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
Last activity: 2026-09-10

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

### Quick Tasks Completed

| Datum | Aufgabe | Ergebnis |
|---|---|---|
| 2026-08-30 | uebergabe-kundenwebsite-abschliessen | Eine Kundenwebsite in zwei Bundles aufgeteilt, Übergabe und Zeiger in CLAUDE.md entfernt |
| 2026-09-06 | pfeil-ruecklinks-t-zu-th | Die drei Rücklinks in `field_list.html` zeigen endlich einen Pfeil statt `&#8592;`. **Der Aufschub beruhte auf einer falschen Annahme über den eigenen Flick** — er setzte voraus, dass die Zeichenkette geändert werden müsste und drei Schlüssel in vier Katalogen verwaisen würden; der Flick ist `{{t}}` → `{{th}}`, die Zeichenkette bleibt Zeichen für Zeichen dieselbe, und `tools/i18n` sammelt `th` genauso wie `t`. Zähler unverändert bei 1158, `0 offen, 0 verwaist`. Alle drei Stellen im Browser gesehen, bei **englischer** Oberfläche — also durch den Übersetzungsweg hindurch. Fensterbuch Eintrag 5 und der Rückstandseintrag geschlossen, beide mit der berichtigten Begründung statt still gelöscht |
| 2026-09-06 | phase-7-abschluss-vier-befunde | Feldschlüssel wird auf seine Form geprüft (T-07-02), die Bytegrenze erreicht auch ein verstecktes Feld (W-1), `JoinValues` verteidigt sein Trennzeichen (W-3); Kriterium 1 sichtbar gesenkt statt die Datenstruktur umgebaut |
| 2026-09-02 | i18n-kataloge-sauber | en/es/fr/it auf 0 offen, 0 verwaist; 24 Sätze aus Laden und Bestellungen übersetzt, zwei unsichtbare Quelltext-Literale wieder extrahierbar, de-CH nachgezogen |
| 2026-09-02 | dependabot-13-14 | modernc.org/sqlite 1.56.0→1.57.0 und golang.org/x/net 0.57.0→0.58.0 gemergt; go.mod-Konflikt aufgelöst, 44 Wanderungen gegen echte DB-Datei geprüft (wal/5000/1/1, integrity ok), Punkt 7 der Arbeitsliste abgeschrieben |
| 2026-09-03 | theme-holzcloud | Achte eingebaute Vorlage „Holzcloud": das Design von holzcloud.ch als Theme, 15 Dateien plus vier woff2 (Manrope, JetBrains Mono, SIL OFL), Design-System byte-gleich übernommen, `template check` ohne Befund |
| 2026-09-03 | theme-holzcloud-sichtpruefung | Im Browser durchgeklickt (Start, 404, schmal): Design trifft, Menü ohne JS. Ein Fehler gefunden und behoben — home.html druckte .Page.Excerpt im Aufmacher, der Text stand dadurch zweimal auf jeder Seite |
| 2026-09-03 | x86-statt-arm | Raspberry Pi und arm64 aus Bauplan und Beschreibung entfernt, CI baut linux/amd64; neue release.yml veröffentlicht auf `v*`-Tag mit Binär und Prüfsumme; Race-Timeout im Security-Audit auf 30m; beide Workflows wieder aktiv, CI erstmals seit 22. August grün |
| 2026-09-03 | haertung-nach-der-freigabepruefung | Go auf 1.26.6 (govulncheck: 8 aufgerufene Lücken → 0), govulncheck als Schritt in security.yml, AGPL-§13-Hinweis mit Fassung und Quellverweis in der Verwaltung (in fünf Sprachen, im Browser geprüft), zwei versehentliche Bauergebnisse aus dem Arbeitsbaum |
| 2026-09-03 | beispiel-bundle-statt-kundendaten | Die zwei Kundenwebsites in ein eigenes privates Repository ausgezogen; an ihrer Stelle das erfundene Bundle sites/beispiel (Velowerkstatt, vier Seiten, zwei selbst erzeugte Bilder), sites/ von 33 MB auf 108 KB; Kundendaten in elf weiteren Dateien mitgeräumt; neuer Packtest. Historie unverändert — die Blobs bleiben erreichbar |
| 2026-09-03 | kleinigkeiten-aus-der-freigabepruefung | CODE_OF_CONDUCT, Issue- und PR-Vorlagen, Zeile zu den KI-verfassten Commits in CONTRIBUTING, alle drei Actions auf Commit-SHA gepinnt, fremde Domain holzbau.ch in neun Dateien durch example.* ersetzt. Dabei ein flatternder Bestandstest gefunden (1,25 %), abgelegt statt behoben |
| 2026-09-03 | zwei-abgelegte-fehler | mkbundle prüft die Schlagwörter jetzt gegen die Namen statt die Kennungen — der Import las die Kennung ohnehin nie; das Beispiel-Bundle trägt endlich Schlagwörter, darunter eines mit Name ≠ Kennung als stehende Absicherung. Der flatternde Formulartest prüft jetzt die Form statt zwei Zeichen im Rauschen, 400 Läufe grün. Vermerk: die Formular-Testreihe überspringt sich stumm ohne gebautes plugin.wasm |
| 2026-09-03 | historienbereinigung | `git filter-repo` hat die zwei Kundenwebsites unter sites/ aus allen Commits getilgt; ein Commit, der nur Fotos enthielt, fiel als leer weg (336→335). Repository 261 MB → 20 MB, E-Mail in 0 Blobs, Tests grün. force-Push auf main, v1.1 und v1.4 mitgeschrieben, die zwei claude/*-Zweige gelöscht. **Offen: GitHub liefert die alten Objekte weiter aus, bis es aufräumt** |
| 2026-09-03 | dokumente-fuer-den-umzug | README-Abschnitt „Versionen" und CONTRIBUTING-Abschnitt zur KI-Autorschaft auf das frische Repository umgeschrieben, CHANGELOG.md mit 1.4 als erster öffentlicher Fassung angelegt. Nebenbei berichtigt: README nannte unter „License" MIT, während LICENSE, CONTRIBUTING und VENDOR.md AGPL-3.0 sagen |
| 2026-09-03 | dokumente-fuer-den-umzug-ins-frische-rep | README „Versionen" erklärt jetzt den einen Commit statt des toten Zweigs archive/gsd-v1.1-dead und der Tags v1.0–v1.3; CONTRIBUTING trägt die KI-Aussage ohne den Zählbefehl, der in einem Ein-Commit-Repository eine Zeile ausgibt; neues CHANGELOG.md (deutsch, Keep-a-Changelog-Gerüst, Sätze statt Stichworte) mit 1.4 als erstem und einzigem Eintrag. Dabei korrigiert: README nannte MIT, während LICENSE die AGPL-3.0 trägt |
| 2026-09-03 | vorlage-weide-neu-gestalten | Die Vorlage `weide` ins Helle übersetzt: warmes Papier #FAF6EF, Erdbraun #6E4D32, Manrope aus dem Repository statt Systemschrift. style.css von 2332 auf 1657 Zeilen neu (vier @layer, dann die schichtlose CMS-Schicht, dann .Site.Design), alle zwölf Ansichten in der Bauteil-Sprache .hc-*, die neun Bausteinarten über die zehn Zeilen der Brücke. Kontraststufe auf hellem Papier neu gemessen: 64 % statt der 52 % aus holzcloud. Drei Fehler nebenbei behoben — fehlendes Druck-Stylesheet, durchgeisternde Kopfleiste, ein langer Betriebsname sprengte die Leiste auf dem Telefon. Nach der Sichtprüfung zwei Befunde nachgezogen: die Navigation unter 1000 px war ein Treppenmuster (oberste Liste blieb eine Flex-ZEILE, ein <li> mit Untermenü riss sie auf) — jetzt beide Ebenen als Spalte, dazu der Checkbox-Umschalter zurück, oberhalb der Schwelle auf display: none; und ein Bild allein im Absatz spannt bis breit-ende, weil eine Seite aus reinem Markdown sonst 62 % der Breite leer lässt. Nach dem Umstellen des Inhalts auf Bausteine zwei weitere Fehler: Überschriften in Bausteinen blieben klein (Kindselektor traf sie nicht — eine Überschrift über einem `bildtext` kann nur im Markdown des Bausteins stehen, weil es dort gar kein Titelfeld gibt), und `.hc-karten` fehlte in der Breit-Regel, weshalb drei Karten in der 705-px-Textspalte auf 2+1 umbrachen. Danach derselbe Befund ein drittes Mal für `.hc-bildtext` (Bild neben Text auf 2 x 340 statt 2 x 577 px); der Kommentar nennt seither die Regel statt der Liste — breit ist, was mehrspaltig ist oder ein Bild in natürlicher Grösse zeigt, schmal bleibt, was gelesen wird, und `.hc-aufruf` bleibt absichtlich schmal. Zuletzt die Bildgrösse selbst: kein Hochrechnen mehr (eine 235-px-Bildmarke wurde 2,4-fach gezogen) und eine Höhengrenze (ein Hochformat stand auf 569 x 1012), für die drei Bildarten, die nicht zugeschnitten werden — Zuschnitt scheidet aus, weil render.go das object-position nur für Karte und Galerie liefert. bausteine.css blieb unangetastet. Zum Schluss zwei Befunde von der Schwestervorlage: ein Aufruf mit vier Absätzen stand zentriert (jetzt linksbündig ab dem zweiten Absatz oder einer Liste, der kurze Zuruf bleibt mittig), und `.hc-bild--breit` mass 810 statt 1120 px, weil der Prozentüberzug des Kerns gegen das Raster kämpfte — für ein Bild hat breit jetzt drei benannte Stufen. Dabei ein eigener Fehler berichtigt: die Höhengrenze traf auch `--voll`. Zuletzt der schwerste Befund: das Untermenü des vorletzten Hauptpunktes ragte über den rechten Rand und liess damit JEDE Seite waagrecht scrollen, ohne dass jemand das Menü öffnete — ein absolut gesetztes Element zählt auch mit `visibility: hidden` zur Scrollbreite. Die Untermenüs der letzten drei Punkte hängen jetzt nach links; in headless Chrome gegen die echten Dateien gemessen, 1280/1100/1024 px je 0 px Überlauf, beide Enden mit einem langen und einem kurzen Prüfmenü nachgewiesen |
| 2026-09-03 | vorlage-rudel-neu-gestalten | Die Vorlage `rudel` als gruene Schwester der hellen `weide`: Papier #F7F6F0, Waldgruen #325737, Manrope, Rundung 10 px und Textmass 66 ch nach dem Manifest der Website statt nach dem Entwurf. Kontrast selbst nachgerechnet (Tinte 15.16, schwaechste Stufe 64 % = 4.80, Marke 7.59 auf Papier und 7.72 auf einer Karte). Alle vier Befunde aus der Sichtpruefung von `weide` sind hier im ersten Wurf drin statt nachtraeglich geerbt: Ueberschriften in Bausteinen, Breit-Regel fuer `.hc-karten` und `.hc-bildtext`, Hoehengrenze im Bildtext, kein Hochrechnen kleiner Bilder. Dazu eine Besonderheit dieser Website: ihr Markdown enthaelt handgeschriebenes HTML — `<div><section>` als Kartenreihe, `<aside>` als Aufruf —, das bluemonday durchlaesst; die Vorlage kleidet diese Uebergangsform mit, damit die Startseite nach der Neugestaltung nicht schlechter aussieht als vorher. Zwoelf der zwanzig Seiten sind Tierportraets, deshalb ist die Hoehengrenze hier die tragende Regel und nicht die Vorsichtsmassnahme |

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

**Last session:** 2026-09-08T07:34:21.604Z
**Stopped at:** Completed 10-10-PLAN.md — Phase 10 and milestone v1.6 complete
**Resume file:** None

## Decisions

- [Phase 6]: Phase 6: the i18n pointer tools/i18n/main.go:287 stands as written — D-17a is WITHDRAWN and a fresh grep confirms 287; what the note now says is that the claim lives in the writeCatalog doc comment, not the package doc comment at :1-15
- [Phase 6]: 06-03: writeCatalog is encoding/json end to end (SetEscapeHTML(false) + json.Indent with two empty strings); all seven committed catalogues are byte-unchanged, proven by test and by jq -S against HEAD
- [Phase 6]: 06-03: the round-trip test locks the FORMAT, not the key set — a cleanly deleted key still round-trips. The guard for a deleted key is the -write + git diff pair that 06-06 installs as CI (D-15)
- [Phase 6]: 06-03: a lock test on already-canonical behaviour cannot fail before the implementation exists, so the RED gate was proven by corrupting a real catalogue (reindent, stripped newline) instead of by absence
- [Phase 6]: Phase 6: MAINT-03 and success criterion 3 cover all six committed .wasm modules including internal/plugin/testdata/echo.wasm, and the four .zip archives are repacked by the same tool (D-07, D-23)
- [Phase 6]: 06-05: all four tools/wasm modes work from one in-memory artifact set, so -check compares byte for byte what the write mode would install — the modes cannot drift apart
- [Phase 6]: 06-05: the archives are Deflate with a fixed 1980-01-01 Modified stamp; two runs are byte-identical, and the residual dependency is compress/flate in the toolchain running the tool, not the pinned guest compiler
- [Phase 6]: 06-05: the go.mod floor that 06-02 flagged is now a startup guard (bodenPruefen) — goToolchain below go.mod's go directive refuses with one sentence naming both, instead of letting echo alone fail deep inside a build
- [Phase 6]: 06-05: `go run` collapses any non-zero exit to 1, so the tool's exit 2 for a usage error is only visible to a compiled binary. CI must treat non-zero as the signal, not the specific code
- [Phase 6]: Phase 6: MAINT-05 and success criterion 5 cover all seven codebase maps and point at 06-RESEARCH.md MAINT-05 Correction Inventory instead of naming a count that can itself go stale (D-18, D-19)
- [Phase 6]: Phase 6: the wasm build is tools/wasm, a Go command run as 'go run ./tools/wasm' (D-06); GOTOOLCHAIN is pinned inside the tool with a floor at the root go.mod go directive (D-03, D-03a); -buildvcs=false is mandatory in all four documented invocations (D-02a)
- [Phase 06]: Corrections to the codebase maps are derived by re-running each proof command against current HEAD, never copied from a research document measured at an older commit
- [Phase 06]: CONCERNS.md keeps every judgement; only its numbers moved. The Go-version-mismatch finding stays open because CLAUDE.md:7 still says "Go 1.22+" while go.mod says 1.26.6
- [Phase 06]: A resolved deferred finding is closed by a dated stamp above its text, not by deletion — the reasoning is what a later phase needs
- [Phase 06]: 06-02: D-05 is PASS — the six wasip1 guests hash identically on darwin/arm64 and ubuntu-latest (run 33866318077, 2026-09-04). The byte comparison can be made blocking; the D-05 fallback (-out plus HOLZCLOUD_WASM_DIR) is not needed and must not be planned for
- [Phase 06]: 06-02: tools/wasm forces GOTOOLCHAIN on every build subprocess, proven against ambient local/go1.26.7/go1.27.0 — the guest bytes depend on the goToolchain constant alone, not on setup-go or go.mod. The real trap is D-03a floor: bumping go.mod go directive above the pin breaks the echo build loudly, and only echo
- [Phase 07]: 07-01: D-05 als own-kind bestaetigt — mehrfachauswahl ist eine eigene Art neben auswahl, nicht auswahl mit max_werte>1. Gemessen: render.go Resolve hat keinen case KindChoice, eine Auswahl erreicht ein Theme also als String — Auswahl mehrwertig zu machen wuerde still umtypisieren, was jedes bestehende Theme auf jeder bestehenden Website liest — ohne Fehler an irgendeiner Stelle. Die Entscheidung wurde AUTOMATISCH gewaehlt (workflow.auto_advance: true) und ist einwegig; sie deckt sich mit 07-CONTEXT D-05, sollte aber einmal bestaetigt werden
- [Phase 07]: 07-01: SplitValues/JoinValues sind das eine Paar fuer mehrwertige Werte; SplitChoices/JoinChoices bleiben getrennt und keines ruft das andere auf — Zwei Namen fuer zwei Bedeutungen: eine spaetere Aenderung daran, wie eine Moeglichkeitenliste gelesen wird, darf nicht still aendern, wie ein Wert gespeichert wird. Phase 9 erbt die beiden ausgefuehrten Funktionen
- [Phase 07]: 07-01: Mehrwertigkeit steht im Namen des Formularfeldes (feld_<key>[]), gepraegt allein in Def.FieldName(); der versteckte Waechter haelt geleert von abwesend unterscheidbar — fieldsFromRequest liest bewusst per Praefix, bevor die Definitionen geladen sind — diese Eigenschaft musste erhalten bleiben. Eine Kennung ist slug-artig und kann keine Klammer enthalten, die Markierung kann also nicht kollidieren
- [Phase 07]: 07-02: D-14 als four-columns bestaetigt — darstellung, max_werte, min_wert und max_wert bekommen eigene Spalten auf page_field_defs, keine Pruefregel am Spaltenkopf, ein Down das alle vier zuruecknimmt. AUTOMATISCH gewaehlt (workflow.auto_advance: true) und einwegig: eine freigegebene Wanderung wird nie bearbeitet, eine Korrektur waere 00047. Deckt sich mit 07-CONTEXT D-14 und REQUIREMENTS.md:317, sollte aber einmal bestaetigt werden
- [Phase 07]: 07-02: die beiden Bereichsgrenzen sind Text und keine Zahlen — 'keine Grenze' und 'die Grenze ist null' sind zwei verschiedene Tatsachen, und eine Zahlenspalte mit Vorgabewert koennte sie nicht auseinanderhalten
- [Phase 07]: 07-02: die fuenf SELECT-Spaltenlisten in internal/field/store.go bleiben zeichengleiche Abschriften voneinander und werden als eine Ersetzung geaendert; die vier neuen Spalten stehen zwischen bedingung und COALESCE(block_type_id, 0). Das Tor darauf ist ein Lesetest ueber jeden Leseweg, nicht eine Zaehlung der Vorkommen — ein SELECT kann eine Spalte nennen und sie trotzdem nie in den Def schreiben
- [Phase 07]: 07-02: der Plan widersprach sich selbst — keine Art behaelt Display UND MaxValues, weil validate leert, was zur Art nicht passt. Die Leerregel gewinnt (sie entschaerft T-07-06); der Lesetest nimmt je Ebene ein Auswahl- und ein Mehrfachauswahl-Feld, zusammen decken die beiden alle vier Spalten auf jedem Leseweg
- [Phase 07]: 07-03: die Suchindex-Liste in internal/block/render.go PlainText nimmt KindCode und KindMulti auf und sonst nichts. Ausdruecklich festgehalten, weil weder Fahrplan noch Kontext diese Entscheidung getroffen hatten: ein Codefeld haelt Worte (Adresse, Einstellungszeile) und eine Seite aus Bausteinen waere sonst fuer ihre eigene Suche gerade dort unsichtbar, wo der Verfasser sich am meisten Muehe gab. Eine Mehrfachauswahl kommt mit Leerzeichen verbunden ueber field.SplitValues hinein, nicht als gespeicherte Zeilenspalte. zeit und bereich bleiben draussen, aus demselben Grund wie Bildnummer und Datum; schlagwort kann hier nie auftauchen
- [Phase 07]: 07-03: field.ParseNumber und field.ParseTimeOfDay sind die je eine Lesart. Check misst den eingegebenen Wert UND beide Grenzen damit, und validate in store.go wurde auf ParseNumber umgestellt: vorher las validate mit blankem strconv.ParseFloat, waehrend Check das Komma vorher ersetzt — "0,5"/"0,2" war damit ein verdrehtes Paar, das validate nicht sehen konnte und Check dann rueckwaerts durchgesetzt haette. Eine Uhrzeit wird ueber die Zeichenlaenge gelesen und nicht ueber time.Parse, dessen Stundenfeld auch einstellig annimmt: "9:30" kaeme sonst still als halb zehn durch
- [Phase 07]: 07-03: D-08 wurde hier bewusst NICHT entschieden — MayControl bleibt true fuer bereich, und der Platzhalterzweig im Zahlenfeld haelt die Praemisse ueberhaupt pruefbar. Der Browserdurchgang in 07-07 entscheidet, ob :placeholder-shown an einem Zahlenfeld greift, und traegt den geschriebenen Rueckweg. zeit dagegen ist raus, aus demselben Grund wie datum; MayControl wurde dafuer in zwei switch-Faelle geteilt, damit die gemeinsame Begruendung neben dem Paar steht, das sie erklaert
- [Phase 07]: 07-03: die dritte Leerregel in validate (beide Grenzen fuer jede Art ausser bereich) hat drei bestehende Testreihen zerbrochen — store_test, admin/field_defs_test und bundle_test hingen die Grenzen mangels Konstante an Auswahl, Mehrfachauswahl und Textfeld. Die Grenzen zogen in allen dreien an ein Bereichsfeld um; der Lesetest gewann dabei Deckung, weil jede Spalte jetzt an der Art sitzt, der sie gehoert. Am schlimmsten war die T-07-05-Pruefung auf gebundene Parameter: an einem Textfeld waere der boshafte Wert geleert worden und die Pruefung haette weiter gruen gemeldet, ohne noch irgendetwas zu beweisen
- [Phase 07]: 07-04: MaxValueBytes wird gemeldet und nicht mehr angewendet (D-13 erledigt). trimTo kuerzt nicht mehr, ein Bytewaechter steht vor dem Verteiler in Check und gilt damit jeder Art; gemessen wird der ganze verbundene Wert eines Feldes einschliesslich der Umbrueche, in Byte und nicht in Runen — Ein gekuerzter Wert sieht aus wie einer, den jemand so getippt hat. Seit das Budget allen Werten eines Feldes zusammen gehoert, haette das Kuerzen einen Wert halbiert und das Bruchstueck als echten Wert abgelegt. Die Datenbank zaehlt Byte, also zaehlt die Grenze Byte; die Begruendung sagt dazu, dass Umlaute doppelt zaehlen, statt 4000 Zeichen zu versprechen, die sie nicht halten kann. Eintrag 2 in .planning/WINDOWS.md ist fixed, offen sind null
- [Phase 07]: 07-04: max_werte gilt serverseitig in Check, gezaehlt ueber SplitValues; null heisst ohne Grenze — Eine Haekchengruppe laesst sich in HTML ohne JavaScript nicht begrenzen, und dieses Programm traegt keines ausser htmx (D-05). Null ist das, was jedes vor dieser Phase angelegte Feld traegt — eine erzwungene Null waere eine Verhaltensaenderung an jedem bestehenden Feld
- [Phase 07]: 07-04: parseRowName bekam eine vierte Wache — ein Unterfeldname, der nach dem Abschneiden der Markierung leer waere, wird abgelehnt — Der Plan nannte drei zu erhaltende Wachen; keine davon lehnt gruppe.<kennung>.<nummer>. oder gruppe.<kennung>.<nummer>.[] ab. Beide legten bisher einen Zeileneintrag unter dem leeren Schluessel an; cleanRow warf ihn spaeter weg, gespeichert wurde also nie etwas. Der Weg oben auf der Seite uebergeht feld_[] seit 07-01 — die beiden Namenswege stimmen jetzt ueberein, und die Ablehnungstabelle weist es nach
- [Phase 07]: 07-05: Check prueft ein Schlagwortkuerzel mit page.Slugify(value) != value und NICHT mit page.ValidateSlug — ValidateSlug traegt reservedSlugs, eine Reservierung der Router-Pfade; ein Schlagwort namens "admin" ist unter /tag/admin voellig erreichbar und waere abgelehnt worden. Slugify(v)==v ist genau die hier gestellte Frage: ist das schon die normalisierte Schreibweise. Dafuer importiert internal/field erstmals internal/page — zyklenfrei geprueft, weil internal/page nur auth und db zieht und keines von beiden field kennt
- [Phase 07]: 07-05: Der Wert eines Schlagwortfeldes reist im Archiv als NAME und nicht als Kuerzel, internal/bundle/format.go blieb unangetastet. Rename behaelt das Kuerzel absichtlich, damit Links nicht brechen — ein als Kuerzel reisender Wert haette auf der Zielmaschine still ins Leere gezeigt. format.go:152-159 und :274-284 sind als Entscheidung gelesen worden und nicht als Beschreibung: an der Schreibweise zu drehen wuerde die Bedeutung jedes bereits ausgehaendigten Archivs aendern. Folge und kein Fehler: die Adresse einer Beschriftung darf sich ueber eine Rundreise bewegen (moebel kommt als moebelbau an), das gemeinte Schlagwort nicht — der Test sagt das im Kommentar, damit es spaeter niemand als Fehler meldet
- [Phase 07]: 07-05: term.EnsureNames und importTerms legen jedes Schlagwort an, das ein Manifest nennt; report.Terms zaehlt Angelegtes statt Behauptetes, an genau einer Stelle gesetzt. Bisher entstand ein Schlagwort nur als Nebenwirkung einer Seite, die es traegt — eines, das nur ein Schlagwortfeld nennt, kam nirgends an. EnsureNames leitet das Kuerzel mit derselben page.Slugify ab wie SetForPage; liefen die beiden auseinander, waere ein Schlagwort zwei Zeilen
- [Phase 07]: 07-05: Der offene Blocker aus 07-04 ist ENTSCHIEDEN und behoben statt vertagt — importFieldValues laeuft jetzt durch field.Clean und field.CheckAll (Rule 2, fehlende Eingabepruefung an einer Vertrauensgrenze). Ein Archiv ist genauso unvertraut wie ein Formularfeld, alle anderen Schreibwege waren gedeckt, und seit 07-04 kuerzt trimTo nichts mehr — CheckAll ist damit die einzige verbliebene Stelle, an der das Bytebudget gilt. Die Definitionen werden ueber s.Fields.List GELESEN und nicht aus dem Manifest nachgebaut, damit gegen das geprueft wird, was diese Website hat. Ein beanstandeter Wert wird benannt und entfernt, nie die ganze Seite verworfen; gemeldet nur im ersten Durchgang, nicht noch einmal im Verweisdurchgang
- [Phase 07]: 07-05: Der beweisende Rundreisetest benennt das Schlagwort VOR dem Export um, und das ist nicht optional — im RED-Lauf bestand der Untertest ohne Umbenennung gegen den unreparierten Baum, waehrend der mit Umbenennung in drei Behauptungen fiel. Ein Schlagwort, dessen Name noch zu seinem Kuerzel passt, reist auch ohne die Uebersetzung heil und bewiese gar nichts
- [Phase 07]: 07-05: KEIN Kontrollpunkt in diesem Plan — alle vier Tasks type=auto, kein tracer, kein precondition. Es wurde nichts automatisch genehmigt und nichts automatisch gewaehlt; anders als 07-01 und 07-02 gibt es hier keine einwegige Auswahl nachzubestaetigen
- [Phase 07]: v1.6 Phase 7: switchOf nimmt die ganze field.Def statt einer blossen Art — „eine Auswahl als Knopfreihe" ist mit einer Zeichenkette nicht auszudruecken, weil die Darstellung Teil der Antwort ist. Eine einzige Aufrufstelle, viewOf — Ein zweiter Parameter oder eine blosse Darstellungszeichenkette waeren dieselbe Entscheidung an zwei Stellen geschrieben
- [Phase 07]: v1.6 Phase 7: .feld-schalter--knopfreihe kopiert die Form der auswahl-Regel (Vorgabe sichtbar, verbergen bei Treffer) und nicht die des kreuz — eine beantwortete Auswahl zeigt ihre Abhaengigen, eine offene verbirgt sie. Selektor: input[type="radio"][value=""]:checked
- [Phase 07]: v1.6 Phase 7: eine Mehrfachauswahl und ein Schlagwortfeld meldeten vor 07-06 den Schalter „text", dessen Regel einen Platzhalter sucht, den beide nicht tragen — jedes Feld daran blieb stumm sichtbar. Beide fielen in switchOfs default-Zweig; 07-06 hat sie auf „kreuz" und „auswahl" gestellt
- [Phase 07]: v1.6 Phase 7: MayControl() blieb in 07-06 unangetastet — KindTime ausgeschlossen, KindRange drin. D-08 gehoert dem Browserdurchgang in 07-07 und darf nicht aus dem Markup entschieden werden
- [Phase 07]: v1.6: D-08 ist durch Beobachtung geschlossen und die Umkehr NICHT angewendet — `MayControl()` bleibt unverändert, `KindRange` bleibt steuernd. Ein `<input type="number">` trifft `:placeholder-shown`, `.feld-schalter--text` greift daran, das abhängige Feld erschien und verschwand (Playwright, 2026-09-05) — Die Fahrplan-Notiz, `KindRange` neben `KindDate` auszuschliessen, ruhte auf der Schieber-Annahme, die D-07 verworfen hat. Auch der zweite Zweig des Auftrags ist beantwortet: `placeholder=" "` steht im gezeichneten Markup, es fehlte also nicht. Drei Kommentare, die auf den Durchgang vorauswiesen, sagen jetzt, was gesehen wurde.
- [Phase 07]: v1.6: eine `<precondition>`, die dem Executor fehlt, wird gehalten und die Arbeit geteilt — nicht in eine Checkliste zurückverwandelt. Schritt 3 von 07-07 verlangte ein Browserwerkzeug; der Executor hielt, der Orchestrator fuhr den Durchgang mit dem Playwright-MCP-Server — Eine Checkliste zurückzugeben ist genau das, was der Auftrag verbietet, und was die Browserhälfte in Phase 6 halb hat liegen lassen. Das Muster gilt für Phase 8, 9, 10 und 11 weiter.
- [Phase 07]: v1.6: der leere Fall einer neuen Feldart liegt in `MinimalData`s `Felder`-Karte, nicht in `Feldliste` — `field.List` lässt jeden leeren Eintrag aus, eine Liste mit leerem Eintrag gibt es also gar nicht — Ihn in `Feldliste` zu verlangen hiesse, eine Form zu fordern, die der Code nie erzeugt. Gilt für jede Art, die Phase 8 und 9 hinzufügen.
- [Phase 08]: 08-01: Wanderung 00047 traegt REFERENCES snippets(id) ON DELETE CASCADE — SQLite verweigert nur REFERENCES ZUSAMMEN MIT einem Vorgabewert ungleich NULL, was 00038:42 samt eigenem Kommentar bei :36-39 beweist; snippet_id hat keinen Vorgabewert, also steht die Beziehung in der Datenbank statt in Go
- [Phase 08]: 08-01: .Site.Bausteinfelder (map[string]map[string]any) und .Site.Bausteinliste (map[string][]field.Entry) stehen NEBEN .Site.Snippets, das seinen Typ map[string]template.HTML behaelt — loader.go:355-357 verbietet Umbenennen und Entfernen, und SNIP-05 gilt damit durch Bauart statt durch eine Pruefung
- [Phase 08]: 08-01: field.Store.Sub bleibt bewusst OHNE snippet_id-Klausel — eine Gruppe darf an einem Textbaustein stehen, ihre Unterfelder tragen dann parent_id UND snippet_id, und ein AND snippet_id IS NULL liesse jede solche Gruppe leer zurueckkommen; ihr Namensraum ist die Gruppennummer
- [Phase 08]: 08-01: die Zaehlgatter des Plans (6 gesamt / 5 Spaltenlisten) sind gegen fuenf Leser gerechnet, waehrend dieselbe Aufgabe OfSnippet als sechsten verlangt — gemessen 7/6, und die Gegenprobe fuer block_type_id misst in derselben Datei ebenfalls 7; das Tor auf den Namensraum ist ohnehin TestBausteinNamensraum, das zurueckliest statt zu zaehlen
- [Phase 08]: Der Feldvorrat (MaxFields = 60) wird auf den Träger gezählt statt auf die Website (D-05): vier Arme, jeder nennt seinen Namensraum ausdrücklich — Ein Formular zeichnet immer nur die Felder eines Trägers; ein geteilter Vorrat liesse einen Träger den anderen still verwehren, und ErrTooMany nennte einen Grund, der nicht wahr ist
- [Phase 08]: fillSnippets ist die einzige Zuweisungsstelle der drei Textbaustein-Mitglieder einer SiteData und liest über den Massenleser OfSnippets — Ein Mitglied, das an dreizehn von vierzehn Stellen gefüllt wird, ist auf der vierzehnten unsichtbar; ein grep beweist nur, dass keine Zuweisung überlebt — drei Routen im Test beweisen, dass die Funktion auch gerufen wird
- [Phase 08]: 08-03: der Feldbildschirm bekommt einen vierten Modus (?textbaustein=<id>) statt einer vierten Route — FieldListData.Snippet, ein vierter case in fieldListData mit field.Kinds in voller Breite, Simple() auf drei Traeger, fieldPath mit viertem Arm an allen vier Aufrufstellen — Es gibt keinen Handler und keine Vorlage je Bausteinart; ?baustein= ist bereits der dritte Modus desselben Bildschirms. Eine vierte Route waere eine zweite Wahrheit ueber denselben Bildschirm gewesen
- [Phase 08]: 08-03: snippetOf ist die eine Eigentumspruefung, aufgerufen vom GET und vom POST; h.snippets.Get steht in internal/admin/field.go genau einmal — blockTypes.Get nimmt die Websitenummer und findet eine Bausteinart einer anderen Seite nicht; snippets.Get nimmt nur eine Nummer. Zwei eingelassene Vergleiche koennten auseinanderlaufen — eine Funktion kann es nicht
- [Phase 08]: 08-03: die drei {{if not .BlockType}}-Verneinungen in field_list.html bleiben unverbreitert — sieben, nicht zehn, ist die Zahl der neuen Arme — Die drei Verneinungen sind die drei Orte der Pflicht-Spalte. Eine davon auf .Snippet auszuweiten haette dem Bediener die einzige Moeglichkeit genommen, ein Textbausteinfeld als Pflicht zu markieren — und waere der bequeme falsche Weg gewesen, eine Armzahl passend zu machen
- [Phase 11]: Ein Album reist im Archiv unter seinem Namen und ohne Kuerzel; die andere Maschine leitet die Adresse mit page.Slugify ab, dem einen Aufruf, den album.Store.Create schon macht
- [Phase 11]: importAlbums steht nach den Bildern und vor den Seiten, ausdruecklich nicht zuletzt wie der Menue-Import; Manifest.Version wird nicht erhoeht, weil albums omitempty ist
- [Phase 10]: Env-Variablennamen als Konstanten (envListen, envSSO*) — die einzige Form, in der die Zaehl-Schranke des Plans (7 Zeilen) und seine Verhaltensliste (jede Absage nennt ihre Variable) beide gelten
- [Phase 10]: Startabsagen bleiben schlichte Go-Literale in Englisch, nicht im Katalog — das ist, was die Payrexx- und SMTP-Absagen bereits tun; tools/i18n bleibt bei 1311/34 offen unveraendert
- [Phase 10]: handler.go's 'return assigned == 0 || mine > 0' steht auf Zeile 183, nicht 173 (ROADMAP) und nicht 178 (Plan). Neue Kommentare nennen die Funktion NewWebsiteAccessLookup statt einer Zeilennummer
- [Phase 10]: provisionSSOUser writes the website assignment in the same function that creates the account — NewWebsiteAccessLookup reads zero rows in user_websites as access to every website; between Create and the assignment there must be no request, no error path and no later plan
- [Phase 10]: The zero-rows property is proved through NewWebsiteAccessLookup, never by counting rows in user_websites — Mutation 7 writes exactly one row naming the wrong website: a row-counting test passes while the account reaches the site it must not have
- [Phase 10]: errSSOEmptyAddress is a sentinel so provisioning's empty-address guard can be asserted apart from the two other layers that also refuse it — Mutation 5 stayed green because step 4 and user.Store.Create both refuse; a guard whose removal nothing notices gets deleted by the next tidier
- [Phase 11]: album.Store.Create prueft den Namen jetzt in derselben Schreibtransaktion wie Rename — Im Browserdurchgang von 11-07 gefunden: die Adresse bewegt sich beim Umbenennen absichtlich nicht (GAL-04), also ist der alte Name unter einer anderen Adresse wieder frei und das INSERT laeuft an der UNIQUE-Bedingung vorbei. CR-02 hatte nur Rename geschlossen.
- [Phase 11]: ErrDuplicateSlug bekommt einen eigenen Satz, getrennt von ErrDuplicateName — Nach einer Umbenennung sind Adress- und Namenskollision verschiedene Ereignisse. 'Ein Album mit diesem Namen gibt es schon' schickt den Betreiber sonst in eine Liste, in der dieser Name nicht vorkommt.
- [Phase 11]: 10-05: an editor whose groups map to no configured website is refused the sign-in rather than written as an empty assignment — D-01's inversion reached by subtraction
- [Phase 11]: 10-05: with HOLZCLOUD_SSO_WEBSITE_GROUPS unset the website half of the sync does not run; the invariant is that it never WRITES an empty assignment, not that an empty result is always a refusal
- [Phase 10]: MustHaveSecondFactor gains a viaSSO parameter rather than a variant: the arity change is what made the compiler enumerate five call sites where ROADMAP.md and 10-CONTEXT.md both recorded one
- [Phase 10]: 10-07: Die Abmeldung einer über den Ausweisdienst begonnenen Sitzung leitet auf cfg.SSOSignOutPath um — ein Pfad auf diesem Server, nie aus r.Host zusammengesetzt. ROADMAP.md Zeile 481 verlangt das Gegenteil und widerspricht sich dabei selbst; Mutation 5 ist genau dieser Satz und ist an fünf Stellen rot.
- [Phase 10]: 10-07: web.AdminCSP / web.AdminHeadersWith (Bauschritt ⑦) wurden bewusst NICHT gebaut — das Ziel ist gleicher Ursprung, also genügt form-action 'self'. Zwei Tests halten die Voraussetzung; sie fallen zuerst, sobald ein eigener Outpost-Host unterstützt wird.
- [Phase 10]: 10-07: auth.SafeReturn wird nicht wiederverwendet, obwohl die Roadmap es verlangt — gemessen: SafeReturn("/outpost.goauthentik.io/sign_out") liefert "/admin/", die Abmeldung hätte den Menschen zurück in die Verwaltung geschickt.
- [Phase 11]: 10-08: der (holzcloud-sso)-Schnipsel steht ueber den Site-Bloecken — Caddy loest import beim Parsen in Dateireihenfolge auf, ein Schnipsel darunter scheitert mit 'File to import not found'
- [Phase 11]: 10-08: die Antwortkopfzeilen liegen in (holzcloud-headers), von beiden Wegen importiert; caddy adapt liefert vorher und nachher byteweise dasselbe JSON
- [Phase 11]: 10-08: gemessen auf Caddy 2.11.4 — die Loeschung, die die CVE-Behebung erzeugt, deckt nur die kanonische Bindestrich-Schreibweise; die Unterstrich-Zeilen tragen also auch auf einem behobenen Caddy
- [Phase 11]: 10-08: docs/configuration.md bekommt neun Variablen, nicht sieben — HOLZCLOUD_LISTEN und HOLZCLOUD_TRUSTED_PROXIES fehlten dort schon vor dieser Phase
- [Phase 11]: 10-09: Ein Zaehl-Tor, das eine Absolutzahl prueft, wird von jeder Phase bewegt, die sich denselben Baum teilt. Das ehrliche Tor vergleicht die Schluesselmenge gegen einen benannten Commit und nennt zu jedem neuen Schluessel den Commit: 10 hinzu seit cdcfbab, 8 davon Phase 11, 2 davon 10-06.
- [Phase 11]: 10-09: Die Anrede wurde am Katalog gemessen und nicht aus dem Deutschen uebernommen — es und it duzen, fr siezt, in 40 bestehenden Eintraegen. Das deutsche du nach Franzoesisch zu kopieren waere ein Fehler gewesen, den kein Tor dieses Projekts sieht.
- [Phase 11]: 10-09: Sechste und siebte Instanz von 'ein Tor misst etwas anderes als sein Name' in Phase 10 — completeLogin zaehlt eine Erwaehnung mitten in einem Kommentar (druckt 5, Aufrufstellen sind 4), MustHaveSecondFactor zaehlt Doku-Kommentar, Deklaration und einen neuen Prosa-Kommentar (druckt 8, Aufrufstellen 5 vorher wie nachher).

## Accumulated Context

### Roadmap Evolution

- Phase 11 added: Galerie — Vergroessern/Lightbox (:target, kein JS), Album als wiederverwendbares Ding, Diashow via CSS scroll-snap. Haengt an Phase 7 (Mehrwert-Kodierung FIELD-07), nicht an Phase 10. Passt nicht ins v1.6-Meilensteinziel; per Entwicklerentscheid trotzdem hier.

### Blockers

- ~~07-04 gemeldet, nicht behoben: internal/bundle/import.go importFieldValues schreibt Feldwerte mit field.Encode direkt, ohne CheckAll und ohne Clean.~~ **ERLEDIGT in 07-05 (7cb09f4).** Entschieden wurde gedeckt und nicht vertagt: ein Archiv ist eine Datei, die jeder bearbeiten kann, alle anderen Schreibwege sind gedeckt, und seit 07-04 kuerzt trimTo nichts mehr — CheckAll ist damit die einzige Stelle, an der das Bytebudget ueberhaupt noch gilt. importPages liest die tatsaechlich angelegten Definitionen ueber s.Fields.List und reicht sie in importFieldValues; dort laufen field.Clean und field.CheckAll, ein beanstandeter Wert wird entfernt und namentlich in den Bericht geschrieben, nie die ganze Seite verworfen. TestArchivwerteGehenDurchDieselbePruefung beweist es, Gegenprobe mit deaktivierter Wache gefuehrt (vier Behauptungen fallen). Offene Blocker: keine
- Plan 10-01: vier Zaehl-Schranken der Phase 10 sind gegen den Baum vom 2026-09-07 geeicht und seit Phase 11 veraltet (Migrationen 49 statt 50, Pakete 40 statt 41, Admin-Vorlagen 66 statt 68, Zeichenketten 1277 statt 1311). Die Plaene 10-02 bis 10-09 tragen dieselben Zahlen — gemessene Werte aus 10-01-SUMMARY uebernehmen
- Offen (WINDOWS.md Eintrag 8): eine Album-Diashow zeigt ihre Lichtkasten-Bedienelemente in der Sprache des Besuchers und den Namen ihres Schiebefelds auf Deutsch. render.go:212 uebersetzt mit s.text (nur beim Speichern gesetzt), die Bedienelemente ueber expand.go:133 mit set.t (bei der Anfrage). Die Behebung verschiebt die Grenze zwischen Speicherzeit und Anfragezeit — Architekturentscheid des Entwicklers.
- 10-08: das isTrustedProxy-Tor des Plans schliesst '^\./\.planning/' aus, grep gibt hier aber Pfade ohne './' aus — es liest 14 statt 0. Korrigierte Form: grep -v '^\(\./\)\?\.planning/'. Die Eigenschaft selbst gilt (0 ausserhalb .planning/). Fuenfter Fall in dieser Phase.
