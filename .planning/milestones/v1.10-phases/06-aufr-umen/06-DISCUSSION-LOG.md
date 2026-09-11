# Phase 6: Aufräumen - Discussion Log

> **Audit trail only.** Do not use as input to planning, research, or execution agents.
> Decisions are captured in CONTEXT.md — this log preserves the alternatives considered.

**Date:** 2026-09-04
**Phase:** 6-Aufräumen
**Areas discussed:** Wasm-Vergleich & Toolchain, Umfang & Ort des Neubaus, i18n: Sperre & Selbstauskunft, Veraltete Notizen

The discussion was held in German; the options are reproduced in the language
they were presented in.

---

## Wasm-Vergleich & Toolchain

### Q1 — Was heisst „gegen die eingecheckte Datei vergleichen", wenn der Byte-Vergleich an der Go-Version hängt?

| Option | Beschreibung | Gewählt |
|--------|--------------|---------|
| Toolchain pinnen, Bytes vergleichen | `toolchain`-Zeile je Plugin, sechs Binaries einmal neu, danach `sha256` in CI | |
| Frisch bauen und damit testen | CI testet gegen den Neubau statt gegen die eingecheckte Datei; kein Hash flattert, weicht aber vom Kriterium ab | |
| Beides: Test hart, Hash weich | Frisch bauen und testen, zusätzlich ein Hash-Schritt als Alterungsmeldung | |

**Antwort:** „Entscheide du."
**Claudes Entscheidung:** Byte-Vergleich beibehalten, aber die Toolchain pinnen — mit der Verfeinerung, dass der Pin nicht per `toolchain`-Zeile in `go.mod` sitzt (siehe Q3). Die Tests lesen weiterhin die eingecheckte Datei, weil der Vergleich beweist, dass sie dem Neubau gleicht; MAINT-04 bleibt dadurch unangetastet.
**Grundlage:** Zwei Bauten mit derselben Go-Fassung ergaben identische Hashes; die eingecheckte Datei trägt `go1.24.7`, ein Neubau mit `go1.26.4` ist 215 KB grösser. Ohne Pin wäre der Vergleich am ersten Tag rot und danach bei jedem Dependabot-Sprung — ~21 MB erzwungene Commits ohne Aussage über den Code.

### Q2 — Auf welche Go-Version wird der Wasm-Bau gepinnt?

| Option | Beschreibung | Gewählt |
|--------|--------------|---------|
| `go1.24.7` — Ist-Stand festschreiben | Null Diff, kein Rebuild-Commit, aber die Gäste bleiben auf einer zwei Minor alten Fassung | |
| Aktuelles Go — jetzt einmal anheben | ~21 MB in einem Commit, dafür ein bewusst gesetzter Nullpunkt | ✓ |
| Entscheide du | — | |

**Notiz:** Der Neubau wird ein eigener Commit, der nichts als `.wasm`-Dateien anfasst — dieselbe Regel wie für die Kataloge (PITFALLS #25).

### Q3 — Wo lebt der Pin? *(abgeleitet, von Claude entschieden, nicht zur Wahl gestellt)*

Im Bauskript, als `GOTOOLCHAIN=go1.26.x` mit exaktem Patch-Stand — nicht in CIs `setup-go` und nicht in `go.mod`. Sonst wäre die Fragilität nur von `go.mod` nach CI verschoben und ein Mitwirkender erzeugte lokal andere Bytes als der Runner. Löst ausserdem `echo.wasm` mit, das im Wurzelmodul liegt und keine eigene `toolchain`-Zeile bekommen kann.

**Offen zur Verifikation:** ob `GOOS=wasip1 GOARCH=wasm` mit `-trimpath` auf darwin/arm64 und linux/amd64 byte-identisch baut. Nur auf einem Host gemessen. Trägt das nicht, ist die Rückfallebene „frisch bauen und damit testen".

---

## Umfang & Ort des Neubaus

### Q1 — Fällt `internal/plugin/testdata/echo.wasm` unter denselben Neubau-und-Vergleich?

| Option | Beschreibung | Gewählt |
|--------|--------------|---------|
| Ja — alle sechs gleich behandeln | Ein Werkzeug, sechs Module; schliesst den ABI-Fehlpass mit | ✓ |
| Nein — nur die fünf Plugins | Exakt der Wortlaut von MAINT-03, lässt echo als handgepflegtes Artefakt | |
| Entscheide du | — | |

**Notiz:** `echo` importiert das SDK nicht (rohe `//go:wasmimport`), kann also nicht durch eine SDK-Änderung veralten — wohl aber durch eine Änderung an `hc_call`/`hc_alloc`, und genau die zu bezeugen ist sein einziger Zweck. MAINT-03s Glob erreicht ihn nicht, MAINT-04s „fünf Tests" schon: die Anforderungen sind asymmetrisch.

### Q2 — Wo lebt der Neubau der sechs Module?

| Option | Beschreibung | Gewählt |
|--------|--------------|---------|
| `tools/wasm` als Go-Kommando | Folgt `tools/i18n`/`tools/mkbundle`; überquert Modulgrenzen, setzt `GOTOOLCHAIN`, vergleicht Hashes selbst | ✓ |
| `plugins/build.sh` wie notiert | Hält sich an die Roadmap-Notiz; Shell existiert im Repo aber nur unter `deploy/` | |
| Entscheide du | — | |

**Notiz:** Bewusste Abweichung von der Roadmap-Notiz, in CONTEXT.md als D-06 festgehalten und per D-01 in der Roadmap selbst korrigiert. Vorher gefunden: `internal/plugin/runtime_test.go:13` trägt bereits eine `//go:generate`-Zeile mit den richtigen Flags, aber `go generate ./...` steigt nicht in `plugins/*` hinab.

### Q3 — Wie wird der Vergleich in CI eingehängt?

| Option | Beschreibung | Gewählt |
|--------|--------------|---------|
| Eigener Job, parallel zu `test` | Verlängert die Gesamtdauer kaum, kostet ~30 s Sockel und einen zweiten Cache | |
| Schritt im bestehenden `test`-Job | Teilt Checkout, `setup-go` und Build-Cache; liegt auf dem kritischen Pfad | ✓ |
| Eigener Job, aber pfadgefiltert | Fast gratis, aber ein per `paths` übersprungener *required check* blockiert das Merge | |

### Q4 — In welchen Workflows wird `HOLZCLOUD_TEST_REQUIRE_WASM=1` gesetzt?

| Option | Beschreibung | Gewählt |
|--------|--------------|---------|
| Repo-weit in allen Workflows | Auch `security.yml`, wo die Plugin-Tests unter `-race` laufen | |
| Nur in `ci.yml` | Eine Stelle, aber `security.yml` bliebe nachsichtig | |
| Entscheide du | — | ✓ |

**Claudes Entscheidung:** Repo-weit, in den drei Workflows, die tatsächlich testen — `ci.yml`, `security.yml` und `release.yml:40`. `image.yml` testet nicht und bleibt unberührt.
**Begründung:** MAINT-04 will „laut in CI, nachsichtig auf dem Rechner eines Mitwirkenden". Alles auf einem GitHub-Runner *ist* CI; `security.yml` auszunehmen hiesse, das Loch dort offen zu lassen, wo die Plugin-Tests am gründlichsten laufen (297 s unter `-race`).

---

## i18n: Sperre & Selbstauskunft

**Vorbefund:** `quote()` (`main.go:277`) benutzt bereits `json.Encoder` mit `SetEscapeHTML(false)` — MAINT-01s „durch die Standardbibliothek" ist teilweise schon erfüllt. Drei Wege wurden im Scratchpad ausgeführt statt erinnert: `Encoder(SetEscapeHTML=false)` + `json.Indent("","")` trifft das heutige Format byte-genau; `MarshalIndent` escapt HTML zu `<`; `Encoder.SetIndent("","")` ist ein No-op.

### Q1 — Wie weit reicht der `git diff`-Beweis aus Erfolgskriterium 1?

| Option | Beschreibung | Gewählt |
|--------|--------------|---------|
| Dauerhafter CI-Schritt | Macht QUAL-01 teilweise mechanisch und trägt durch Phasen 7–10 | |
| Einmaliger Beweis in der Phase | Genau der Wortlaut; die dauerhafte Wache wäre allein der Go-Test | |
| Entscheide du | — | ✓ |

**Claudes Entscheidung:** Dauerhafter CI-Schritt, nach dem Muster des Zwillings in derselben Datei (`ci.yml:47–50`, `go mod tidy` + `git diff --exit-code`).
**Abgrenzung, bewusst gezogen:** Der Schritt beweist kanonisches Format und Gleichschritt mit dem Quelltext, **nicht** „0 offen" — `-write` legt einen fehlenden Schlüssel mit leerem Wert an, danach ist der Diff sauber und die Übersetzung fehlt trotzdem. Das ist QUAL-01s menschliche Hälfte und bleibt bei Phase 10. Als zurückgestellte Idee festgehalten.

### Q2 — `fr-CH.json` und `it-CH.json`: schreiben oder dokumentieren?

| Option | Beschreibung | Gewählt |
|--------|--------------|---------|
| Dokumentieren, Verhalten lassen | Doc-Kommentar plus Berichtszeile | ✓ (nach Revision) |
| `-schweiz` auf alle drei erweitern | Symmetrisch, kein Sonderfall | zunächst gewählt |
| Entscheide du | — | |

### Q2b — Was hiesse „erweitern" konkret? *(Nachfrage, weil zwei Lesarten zu verschiedener Arbeit führen)*

| Option | Beschreibung | Gewählt |
|--------|--------------|---------|
| Schreiben, nicht herleiten | Alle drei kanonisch zurückschreiben; der Sonderfall verschwindet aus dem Schreibpfad | |
| Auch eine Regel je Sprache | *septante/huitante/nonante* fürs Schweizer Französisch; fürs Italienische gibt es keine | |
| Doch nur dokumentieren | Kleinster Eingriff, löst MAINT-02 wortwörtlich | ✓ |

**Notiz:** Die Revision erfolgte nach Vorlage der Fakten. `writeSwiss` trägt für `de-CH` eine mechanische Regel (`main.go:221` — `ß→ss`, `„→«`, `“→»`), die die Masse der Einträge aus den deutschen Quellsätzen herleitet. Für `fr-CH` und `it-CH` gibt es keine: ihre Einträge sind reine Wortwahl (*natel*, *formulario*, *laptop*).

### Q3 — Lücke geschlossen *(abgeleitet, von Claude entschieden)*

Der Round-Trip-Test deckt trotzdem alle sieben Kataloge: er liest jede Datei und schreibt sie durch `writeCatalog` in einen Puffer zurück, byte-verglichen — auch die beiden, die das Werkzeug im Betrieb nie anfasst. Volle Format-Sperre ohne Sonderfall im Schreibpfad. Der klare Satz aus MAINT-02 steht im Doc-Kommentar (der dabei aufhört, eingerückte Ausgabe zu behaupten) und in der Berichtszeile bei `:113`.

---

## Veraltete Notizen

**Vorbefund, der über MAINT-05 hinausgeht:** Alle sieben Karten tragen `Analysis Date: 2026-08-22`. „38 goose migrations" steht in `ARCHITECTURE.md:112`, `CONCERNS.md:184`, `STACK.md:9` und `STRUCTURE.md:16` — tatsächlich sind es 45. „Go 1.26.2" steht in `CONCERNS.md:244`, `STACK.md:8` und `TESTING.md:8` — `go.mod` sagt 1.26.6.

### Q1 — Wie weit reicht die Korrektur der Karten?

| Option | Beschreibung | Gewählt |
|--------|--------------|---------|
| Alle sieben, chirurgisch | Zählbare Fakten überall, sonst nichts; zeilenweise belegbar | ✓ |
| Nur `ARCHITECTURE.md` | Wortlaut von MAINT-05; lässt dieselbe falsche Zahl in drei Karten stehen | |
| Karten neu erzeugen lassen | Frisch, aber grosser Diff und `CONCERNS.md`s Einschätzungen ändern sich unkontrolliert | |

### Q2 — Wie sieht „erledigt" in den drei `deferred-items.md` aus?

| Option | Beschreibung | Gewählt |
|--------|--------------|---------|
| Vermerk im Kopf, Text bleibt | Historie und Begründung bleiben lesbar | |
| Dateien löschen | Kleinster Fussabdruck, Begründung nur noch über `git log` | |
| Entscheide du | — | ✓ |

**Claudes Entscheidung:** Vermerk im Kopf, Text bleibt.
**Begründung:** MAINT-05 verlangt, dass sie „als erledigt zu lesen" sind — eine gelöschte Datei liest sich als gar nichts. Und die zurückgestellten Befunde tragen jeweils die Begründung, die eine spätere Phase braucht; die `page.terms`-Notiz entscheidet über spätere Bündel-Arbeit mit. Alle drei wurden während der Besprechung als tatsächlich geschlossen verifiziert.

### Q3 — Was geschieht mit dem fertigen Dependabot-Abschnitt?

| Option | Beschreibung | Gewählt |
|--------|--------------|---------|
| Vorgehen verschieben, Punkt streichen | Punkt fällt aus der Liste, Vorgehen zieht nach „Beim Weiterarbeiten" | ✓ |
| Abschnitt stehen lassen, nur umhängen | Nichts geht verloren, aber ein zweiter Ort für stehende Anweisungen entsteht | |
| Ganz löschen | Kürzer, aber die Erfahrung geht verloren | |

### Q4 — Werden `REQUIREMENTS.md` und `ROADMAP.md` angeglichen?

| Option | Beschreibung | Gewählt |
|--------|--------------|---------|
| Ja, als erste Aufgabe der Phase | Ein kleiner Doku-Commit vor der eigentlichen Arbeit | ✓ |
| Nein, CONTEXT.md genügt | Zwei Dokumente widersprächen sich, ausgerechnet in dieser Phase | |
| Entscheide du | — | |

---

## Claude's Discretion

Vier Fragen wurden mit „Entscheide du" beantwortet. Die getroffenen Wahlen und ihre Begründungen stehen oben bei der jeweiligen Frage und in CONTEXT.md unter „Claude's Discretion":

- Gestalt des Wasm-Vergleichs → D-02, D-03, D-09
- Ort von `HOLZCLOUD_TEST_REQUIRE_WASM=1` → D-10
- Reichweite des `git diff`-Beweises → D-15
- Form von „erledigt" in `deferred-items.md` → D-20

Zusätzlich zwei abgeleitete Entscheidungen, die nicht zur Wahl gestellt wurden, weil sie aus einer bereits getroffenen folgten: der Ort des Toolchain-Pins (D-03) und die Abdeckung aller sieben Kataloge durch den Round-Trip-Test (D-14).

## Deferred Ideas

- „0 offen, 0 verwaist" mechanisch in CI erzwingen — gehört zu QUAL-01, Phase 10.
- `plugins/*/go.mod` von `go 1.24` auf den gepinnten Compiler anheben — kosmetisch, für kein Kriterium nötig.
- Eine mechanische Schweizer-Französisch-Regel (*septante/huitante/nonante*) — für diese Phase verworfen.

## Scope Creep

Keiner. Die Besprechung blieb innerhalb der Phasengrenze; die einzige Ausweitung — sieben Karten statt einer bei MAINT-05 — ist dieselbe Art Arbeit an denselben Dateien und wird per D-01 in `REQUIREMENTS.md` nachgetragen, statt sie unausgesprochen zu lassen.
