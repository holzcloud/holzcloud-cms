---
phase: "08"
slug: "snippets-carry-fields"
status: audited
# threats_open = count of OPEN threats at or above workflow.security_block_on (= high)
threats_open: 0
threats_total: 31
threats_closed: 31
asvs_level: 1
block_on: high
created: "2026-09-06"
carried_forward_closed: ["T-07-02"]
---

# Phase 8 — Security

> Bedrohungsregister, akzeptierte Risiken und Prüfspur für die Schnipselfelder.

Das Register wurde zur Planungszeit über fünf Pläne geschrieben
(`register_authored_at_plan_time: true`) und nach der Ausführung gegen den Code
geprüft. Die Abkürzung, die der Workflow bei `threats_open: 0` und ASVS 1
erlaubt, wurde **bewusst nicht genommen** — wie in Phase 7, und aus demselben
Grund: nach den Plänen fand ein Code-Review einen kritischen und acht weitere
Befunde, mehrere davon in genau den Mechanismen, deren Massnahmen „mitigate"
behaupteten. Alle neun sind behoben; die Massnahmen*texte* waren aber vor diesen
Befunden geschrieben.

---

## Trust Boundaries

| Grenze | Beschreibung | Was sie kreuzt |
|---|---|---|
| Browser → Schnipsel-Wertformular | Feldwerte eines wiederverwendbaren Textblocks, frei wählbar in Anzahl und Inhalt | Redaktorinhalt |
| Browser → Feldbildschirm im vierten Modus | Welchem Schnipsel ein Feld gehört, als `?textbaustein=<id>` | Trägerzuordnung — entscheidet, wessen Formular ein Feld erscheint |
| Gespeicherter Schnipselwert → jedes Theme | Ein Schnipsel steht auf *allen* Seiten, nicht nur auf einer | Redaktorinhalt in HTML-Kontext, website-weit |
| Archivdatei → Datenbank | Manifest eines fremden Rechners, Felddefinitionen **und** Werte des vierten Trägers | Nicht vertrauenswürdige Struktur und Inhalt |
| `page_field_defs` → vier Namensräume | Seitenfelder, Gruppen-Unterfelder, Bausteinart-Felder, Schnipselfelder in **einer** Tabelle | Trägergrenze: ein Leck zeigt fremde Felder auf jedem Formular |

---

## Threat Register

**31 eigenständige Bedrohungen** über fünf Pläne (35 Registerzeilen; `T-08-SC`
wiederholt sich je Plan und zählt einmal). **Alle geschlossen, jede mit belegter
Fundstelle.** Die vollständige Belegtabelle steht im Prüfbericht dieser Sitzung.

| Schweregrad | Gesamt | Geschlossen | Offen |
|---|---|---|---|
| high | 13 | 13 | 0 |
| medium | 8 | 8 | 0 |
| low | 10 | 10 | 0 |

### Aus Phase 7 übernommen und hier geschlossen

**T-07-02 · Tampering · medium — `Def.Key` wurde nie auf seine Form geprüft.**
Phase 7s Prüfung liess ihn offen; die Behebung kam im Phase-7-Abschluss und
wurde hier gegen den **vierten** Träger nachgeprüft: `validKey(d.Key)` steht in
`validate` (`internal/field/store.go:694`), das als erste Anweisung von `Create`
(`:377`) und `Update` (`:528`) läuft. Der Archivweg schreibt `Def`s
ausschliesslich über `Create` (`internal/bundle/import.go:854`, `:870`), also
wird auch ein wörtlich aus einem Manifest übernommener Schlüssel formgeprüft.
Belegt durch `TestFeldschluesselWirdAufSeineFormGeprueft` und
`TestArchivSchluesselMitKlammernWirdAbgelehnt`.

*Lückennotiz des Prüfers, kein offener Punkt:* der Formablehnungstest läuft auf
dem Seitenträger, nicht zusätzlich auf dem Schnipselträger. Der Codeweg ist
byteweise derselbe, deshalb bleibt T-07-02 geschlossen — ein Schnipselfall wäre
billige Doppelsicherung.

---

## Accepted Risks Log

> Der Prüfer hat die **Substanz** jeder Annahme im Code belegt, konnte sie aber
> gegen kein Protokoll prüfen, weil es keines gab. Sein Satz dazu, und der Grund
> für diesen Abschnitt: *„eine Annahme, die nie aufgeschrieben wird, ist eine
> Annahme, die niemand getroffen hat."* Hier sind sie, ausdrücklich getroffen.

| Risiko | Bezug | Begründung, im Code belegt | Angenommen | Datum |
|---|---|---|---|---|
| Eine Abfrage je Website statt je Schnipsel | T-08-06 · low | `internal/snippet/store.go:205` — die bestehende Abfrage trägt zwei Spalten mehr, kein zusätzlicher Rundgang | Plan 08-01 | 2026-09-06 |
| `MaxFields = 60` je Träger statt je Website | T-08-09 · medium | Vier ausdrückliche Zählzweige in `Create` (`internal/field/store.go:435-459`). D-05 hat den Zähler bewusst auf den Träger verengt: das Budget existiert, damit **ein Formular** bedienbar bleibt, und ein Formular zeigt genau einen Träger | D-05, Plan 08-02 | 2026-09-06 |
| Keine neue Route, also keine neue CSRF-Fläche | T-08-15, T-08-23 · low | `cmd/holzcloud/main.go:871` und `:802-803` hängen unter `csrfMiddleware` (`:993`), hinter `requireAuth`, `requireSecondFactor`, `requireWebsite` | Pläne 08-03, 08-04 | 2026-09-06 |
| Ein bedingt verstecktes Schnipselfeld | T-08-21 · medium | Der Fall kann auf diesem Träger nicht entstehen: `Condition` wird auf jedem Schnipselfeld geleert (`internal/field/store.go:780-782`). Die Bytegrenze gilt für versteckte Werte ohnehin weiter (`internal/field/field.go:1014-1028`, Phase 7 W-1) | Plan 08-04 | 2026-09-06 |
| Die Spezifikation nennt die Form der Theme-Oberfläche | T-08-28 · low | `internal/tmplspec/TEMPLATE-SPEC.md:213-214`, `:737-763` — Gestalt, kein Geheimnis, keine Adresse, keine Kennung | Plan 08-05 | 2026-09-06 |
| Keine Paketinstallation in dieser Phase | T-08-SC (×5) · low | `git log --name-only -- go.mod go.sum` über `1d16720…a8c6742` liefert nichts. Das Legitimitätstor feuert korrekt nicht | Prüflauf | 2026-09-06 |

---

## Was das Review fand, nachdem die Massnahmen geschrieben waren

Vier der neun Befunde berührten registrierte Bedrohungen. Alle behoben, alle
nachgeprüft:

- **CR-01 · Migration `00048`.** `00047`s Schnipsel-Index fehlte `AND parent_id
  IS NULL`, wodurch die Unterfelder einer Gruppe auf einem Schnipsel in dessen
  Namensraum oberster Ebene fielen — eine Seite darf zwei Gruppen mit gleichem
  Unterfeldnamen tragen, ein Schnipsel konnte es nicht. **Der Fehler war eine
  Folge des Fixes aus Welle 5**, der Gruppen-Unterfeldern die Trägerkennung gab.
  Kein registrierter Massnahmentext beschrieb diesen Index, CR-01 hat also keine
  Behauptung falsifiziert. `00048`s `Down` stellt bewusst `00047`s weitere Form
  wieder her und darf fehlschlagen, wenn inzwischen zwei Gruppen mit derselben
  Unterkennung existieren — die Rücknahme weigert sich dann, Zeilen zu
  vernichten, die es vor `00048` nicht geben konnte.
- **WR-01 · der Archiv-Schreibweg.** `Clean` und `CheckAll` steckten in
  `if len(defs) > 0`; ein Manifest mit Werten und ohne Definitionen erreichte
  `field.Encode` ungeprüft. Beide laufen jetzt unbedingt
  (`internal/bundle/import.go:1007-1008`). Der Prüfer hat bestätigt, dass **kein
  dritter Träger** dieselbe Form hat: alle vier Schreibwege nach
  `page_field_defs` wurden aufgezählt und geprüft.
- **WR-07 · `Create` prüfte seinen Träger nicht gegen die Website.** Jetzt
  `gehoertZurWebsite` → `ErrNoSnippet` (`internal/field/store.go:405-409`).
  Nicht umgehbar: `INSERT`/`UPDATE`/`DELETE` auf `page_field_defs` gibt es
  ausschliesslich in `internal/field/store.go`.
- **WR-04 · drei seitenlose öffentliche Routen** erreichten `fillSnippets`
  nicht. Jetzt schon. Die Eigenschaft „einziger Schreiber" wurde per grep über
  `internal/` belegt, **nicht** über eine Zahl — die drei Quellen im Projekt
  nannten 18, 17 und zwölf, und keine davon war die Sicherheitsaussage.

### Zwei Befunde, die ausdrücklich keine Sicherheitsbefunde sind

- **WR-03 — Bild-, Verweis- und Schlagwortwerte gehen auf der Archivreise
  verloren.** Ein Korrektheitsfehler, kein Sicherheitsfehler: alle drei Auflöser
  sind websitegebunden und weisen eine fremde Kennung ab. `fieldRefs`
  (`internal/public/pagedata.go:167-169`) verlangt zusätzlich
  `PubliclyVisible()` und `!Protected()` — **ein Entwurf kann also nicht über
  einen veralteten Verweis austreten.** Der Betreiber wird von
  `ortsgebundeneWerte` (`import.go:906-911`) darauf hingewiesen. Die
  eigentliche Behebung — `translateOut`/`translateIn` aus dem Seitenpfad
  herausheben — steht als Vorhaben in `deferred-items.md`.
- **`&#8592;` auf drei Rücklinks** ist Über-Maskierung eines Katalogschlüssels,
  die sichere Richtung, ohne Benutzerdaten. `.planning/WINDOWS.md` Eintrag 5.

---

## Dokumentationsfehler in den Massnahmentexten

Kein Verhaltensfehler, aber sie lesen sich falsch und gehören korrigiert, wenn
diese Pläne je wieder als Quelle dienen:

| Bedrohung | Was der Text sagt | Was der Code tut |
|---|---|---|
| T-08-22 | `MaxRows` werde von `field.CheckAll` durchgesetzt | Es lebt in `field.Clean` (`internal/field/field.go:613`). Beide laufen auf dem Schnipselweg, die Substanz hält, die Fundstelle ist eine Funktion daneben |
| T-08-25 | verspricht `field.Clean` | Der Code liefert `Clean` **und** `CheckAll` — stärker als erklärt |
| T-08-26 | „`Create`s eigene Websitebindung ist die zweite Schicht" | **War beim Schreiben falsch**, ist seit WR-07 wahr. `internal/bundle/import.go:834-837` hält fest, seit wann |

Das ist derselbe Fehlertyp wie T-07-26 aus Phase 7 — eine Massnahme, die einen
Mechanismus nennt, den es so nicht gibt. Dort wurde die Behauptung korrigiert
statt der Mechanismus gebaut; hier ist der Mechanismus inzwischen da und nur die
Reihenfolge stimmte nicht.

---

## Audit Trail

### Security Audit 2026-09-06

| Metrik | Anzahl |
|---|---|
| Bedrohungen im Register | 31 |
| Geschlossen | 31 |
| Offen (blockierend, ≥ `high`) | **0** |
| Aus Phase 7 übernommen und geschlossen | 1 (T-07-02) |
| Dokumentationsfehler ohne Verhaltensfolge | 3 |

**Verfahren:** alle fünf `<threat_model>`-Blöcke, alle fünf `## Threat Flags`,
das Code-Review, der Behebungsbericht und die Phase-7-Prüfung gelesen, dann jede
Registerzeile gegen den Code geprüft. Keine Implementierungsdatei verändert.
`go test -count=1` grün über `internal/{field,db,bundle,admin,public,snippet,template}`.

**Ein Hinweis, der kein Befund ist:** `08-05-SUMMARY.md` führt einen Punkt als
„neu und ausserhalb des Registers" — der Welle-2-Fix, der eine Gruppenkennung
ohne zugehörige Gruppe dieser Website mit 404 beantwortet statt ein Unterfeld
unter einem fremden Elternteil anzulegen (`internal/admin/field.go:194-202`).
Das **verengt** eine Grenze, die T-08-13 bereits deckt, und öffnet nichts.
