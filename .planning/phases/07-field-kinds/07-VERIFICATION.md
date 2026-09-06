---
phase: 07-field-kinds
verified: 2026-09-06
status: gaps_found
score: 4/6 roadmap success criteria verified
method: goal-backward — every verdict below rests on a file I read, a command I ran, or a named test I executed in this session
gaps:
  - truth: "Kriterium 1, zweiter Satz: geleert ist von „dieses Formular trug das Feld nie“ unterscheidbar"
    status: partial
    reason: >-
      Der Unterschied entsteht in fieldsFromRequest und ist zwei Aufrufe später
      wieder weg. field.Clean verwirft jeden Wert, der leer trimmt, also sind
      „geleert“ und „nie getragen“ nach Clean+Encode byteweise dasselbe. Selbst
      nachgemessen, nicht aus dem Review übernommen. Phase 9 ist angewiesen,
      genau diese Unterscheidung zu erben — heute gibt es sie nur in einer
      Funktion.
    artifacts:
      - path: "internal/field/field.go:575-581"
        issue: "Clean schreibt nur Werte in out.Values, die nach trimTo nicht leer sind — der present-but-empty-Schlüssel fällt weg"
      - path: "internal/admin/page_form.go:164-172"
        issue: "Der Kommentar sagt „das ist der Unterschied, den dieser Zweig bewahrt“ — bewahrt wird er nur bis Clean"
    missing:
      - "Entweder field.Values um Präsenz erweitern (Clean unterscheidet leer-vorhanden von abwesend), oder den Kommentar und die Phase-9-Erblast auf das zurückschreiben, was der Wächter wirklich garantiert: der Schlüssel ist immer da, und der Speicherpfad ist ein vollständiges Ersetzen"
      - "JoinValues verteidigt sein Trennzeichen nicht (selbst gemessen: [\"a\\nb\",\"c\"] kommt als drei Werte zurück) — für den CSV-Importer aus Phase 9 ist das dieselbe Baustelle"
  - truth: "Kriterium 6, Browserhälfte: alles Sichtbare wurde einmal durch die laufende Anwendung gefahren"
    status: partial
    reason: >-
      Die Übersetzungshälfte ist grün (selbst gefahren). Die Browserhälfte hat
      vier ungefahrene Zeilen: die eine, die 07-07 selbst als offen notiert, und
      drei, die nach dem Durchgang durch die Review-Fixes dazugekommen sind.
    artifacts:
      - path: "internal/block/render.go:325-334"
        issue: "code innerhalb eines Bausteins auf der öffentlichen Seite — im Test gedeckt (TestCodeImBausteinWirdMaskiert, hier gelaufen), nie im Browser gesehen; 07-07-SUMMARY.md:290 führt es selbst als nicht gefahren"
      - path: "cmd/holzcloud/templates/admin/field_list.html:136"
        issue: "Der Hinweistext unter „Möglichkeiten“ wurde in f63bc6e (05.09., 18:49) geändert — nach dem Browserdurchgang (dokumentiert in 96b1107, vor 35ab403 um 18:33). Nie im Browser gesehen."
      - path: "internal/field/store.go:449"
        issue: "Neue Ablehnung „eine Auswahl braucht mindestens eine Möglichkeit“, sichtbar auf dem Definitionsbildschirm, in f63bc6e dazugekommen. Nie im Browser gesehen."
      - path: "internal/admin/page_blocks.go:242-249"
        issue: "Der Bausteineditor prägt seine Feldnamen seit bf4abdd (18:40) anders — ein Bedienelement, dessen Verhalten sich nach dem Durchgang geändert hat, mit Regressionstest, ohne Browser."
    missing:
      - "Ein kurzer Nachdurchgang: code im Baustein auf der öffentlichen Seite, der geänderte Hinweis und die neue Ablehnung auf dem Feldbildschirm, eine Mehrfachauswahl in einer eigenen Bausteinart"
human_verification:
  - test: "Zwei gleichzeitige Speichervorgänge derselben Seite, beide mit Mehrfachauswahl"
    expected: "Der spätere Schreiber gewinnt vollständig; die Werte werden nicht vermischt"
    why_human: >-
      07-01-PLANs einzige backstop-Zusicherung. Der Code stützt sie (page/store.go:185
      schreibt die ganze fields-Spalte in einem UPDATE, der Schreibpool hat
      MaxOpenConns(1)), aber es gibt keinen Nebenläufigkeitstest und keine
      Beobachtung. Nach dem honest-verifier-Vertrag zählt Lesen hier nicht — insufficient_spec.
  - test: "Dieselbe Seite mit einem bereich-Feld zweimal hintereinander abrufen und die Reihenfolge in .Page.Feldliste vergleichen"
    expected: "Die Reihenfolge ist zwischen zwei Anfragen identisch"
    why_human: >-
      07-03-PLANs backstop-Zusicherung. field.Sort (field.go:975-982) ist ein
      stabiler Sort über (Position, ID) und damit dem Augenschein nach total —
      aber kein Test misst es und niemand hat es beobachtet. insufficient_spec.
warnings:
  - "WR-08 selbst bestätigt: renderOwn (block/render.go:264-334) hat keinen KindMulti-Zweig. Eine Mehrfachauswahl in einem Baustein landet im default-Zweig und erscheint als ein <p> mit Zeilenumbrüchen statt als Liste. PlainText hat den Zweig (render.go:441). Kriterium 3 spricht nur vom Theme, nicht vom Baustein — deshalb Warnung und kein Gap."
  - "WR-03 nachvollzogen: ParseNumber nimmt das Komma, <input type=\"number\"> nicht. Ein über die KI- oder Bündelschnittstelle geschriebenes 1,5 rendert in ein Steuerelement, das leer aussieht — Kriterium 5s „vor dem Speichern lesbar“ gilt dann nicht. Über den Adminweg unerreichbar, weil das Zahlenfeld kein Komma sendet."
  - "WR-07: TEMPLATE-SPEC.md verspricht bei bereich mehr, als das Programm hält (Grenzen gelten zum Speicherzeitpunkt, nachträglich verschärfte Grenzen schreiben gespeicherte Seiten nicht um). Der Vertrag wird von Themeautoren wörtlich gelesen."
  - "Die Ablehnungsgründe aus internal/field/field.go sind Zeichenkettenverkettungen ohne i18n.N — der QUAL-01-Zähler sieht sie nicht. Bereits als offenes Fenster Nr. 3 notiert; für Kriterium 6 wörtlich gelesen unschädlich, für dessen Sinn nicht."
---

# Phase 7: Field Kinds — Verifikationsbericht

**Ziel (ROADMAP):** Das Inhaltsmodell einer Website erreicht jede Feldart dieses Meilensteins, und jede übersteht Speichern, Neuladen, öffentliche Darstellung und die Archivreise.
**Geprüft:** 2026-09-06
**Status:** gaps_found — 4 von 6 Kriterien erfüllt, 2 teilweise
**Vorherige Verifikation:** keine (Erstlauf)

## Die sechs Kriterien

### 1. Eine ausgeführte Mehrwert-Mechanik — **teilweise (Gap)**

**Erster Satz: verifiziert.** `SplitValues`/`JoinValues` (`internal/field/field.go:892-926`) sind das eine Paar, und alle drei Wege gehen hindurch:

| Weg | Stelle | Beleg |
|---|---|---|
| Seitenformular schreibt | `internal/admin/page_form.go:171`, `:192` | `field.JoinValues(values)` an beiden Namensstellen |
| Bausteinformular schreibt | `internal/block/form.go:155` | `field.JoinValues(values)` |
| Renderer liest | `internal/field/render.go:221` | `SplitValues(raw)` im `case KindMulti` |
| Editor liest | `internal/admin/page_fields.go:284` | `field.SplitValues(value)` für `FieldView.Selected` |
| Archivreise | `internal/bundle/*` | trägt die verbundene Zeichenkette unverändert — keine eigene Schreibweise. `TestMehrfachauswahlUeberlebtDieArchivreise` — PASS (hier gelaufen) |

Die Markierung wird an genau einer Stelle geprägt (`Def.NameSuffix`, `field.go:449-457`) und an den drei Lesestellen wieder abgeschnitten. Der Review-Befund CR-01 war der dritte Prägeort, der sie nicht kannte; nach `bf4abdd` ruft auch `page_blocks.go:249` `d.NameSuffix()` auf. Regression: `TestMehrfachauswahlInEigenerBausteinartUeberlebtDasSpeichern` — PASS. **Drei Häkchen kommen als drei Werte an:** `TestMehrfachauswahlVomFormularBisZurAnzeige` — PASS (drei Zeilen gespeichert, dreimal `checked` nach dem Neuzeichnen, zweimal Speichern byteidentisch).

**Zweiter Satz: hält nur bis `Clean`.** Selbst gemessen, mit einem Wegwerftest über `go test -overlay` (kein Eingriff in den Baum):

```
cleared -> ""
absent  -> ""
BEFUND: geleert und nie getragen sind im Speicher identisch
```

`field.Clean` (`field.go:575-581`) schreibt nur nicht-leere Werte fort, also erzeugen der vorhandene leere Schlüssel und der fehlende Schlüssel dasselbe JSON. Der Wächter im Formular ist harmlos und heute folgenlos — der Speicherpfad ist ein vollständiges Ersetzen, geleert wirkt so oder so. Der Review-Befund WR-05 ist zutreffend, der Plankommentar ist es an dieser Stelle nicht.

Dazu, weil dasselbe Erbe betroffen ist: `JoinValues` verteidigt sein Trennzeichen nicht. Selbst gemessen: `["a\nb", "c"]` kommt als `["a" "b" "c"]` zurück. Für die Häkchengruppe unerheblich (Werte stammen aus einer zeilenweise gelesenen Liste), für den CSV-Importer aus Phase 9 nicht.

### 2. Auswahl als Knopfreihe — **verifiziert**

- Radioknöpfe statt Klappliste: `field_input.html:86-99`, `{{if .Buttons}}`. `TestKnopfreihe` — PASS: `farbe` trägt die Knöpfe `["", "hell", "mittel", "dunkel"]`, das Nachbarfeld ohne `darstellung` bleibt ein `<select>`.
- Ausdrückliche „keine Angabe“: `field_input.html:90-92` — ein Radioknopf mit `value=""`, beschriftet „– keine Angabe –“ (bzw. „bitte wählen“ bei Pflicht), ohne gespeicherten Wert `checked`. Damit ist ein Klick ohne JavaScript rücknehmbar.
- Die Bedingungsregel greift an der Knopfreihe: `switchOf` (`page_fields.go:246-258`) gibt für `IsButtonRow()` den Namen `knopfreihe` zurück, `admin.css:1115` ist `.feld-schalter--knopfreihe:has(> .form-group input[type="radio"][value=""]:checked)`. `TestSchalter` — PASS, und er misst das Paar: Klassenname am Kasten **und** das Element, an dem die zugehörige Regel greift, gegen `admin.css` gelesen; sechs steuernde Arten, dazu die Gegenprobe, dass an einer Uhrzeit nichts hängen kann.

### 3. Mehrfachauswahl — **verifiziert**

- Mehrere Werte in einem Speichern und dieselben nach dem Neuladen: `TestMehrfachauswahlVomFormularBisZurAnzeige` — PASS. Auch eine Ebene tiefer: `TestMehrfachauswahlInEinerGruppe` — PASS.
- Ein Theme kann darüber laufen: `Resolve` liefert immer ein `[]string`, nie nil (`render.go:216-225`); `TEMPLATE-SPEC.md:571` dokumentiert `{{range .Values}}`; `SampleData`/`MinimalData` tragen den gefüllten und den leeren Zwilling (`sample.go:107-111`). `TestSpecDocumentsEveryFieldKind`, `TestSampleFieldsAreShapedLikeTheRendererProducesThem` — PASS.
- Einwertiges Lesen ergibt einen lesbaren Satz: `Entry.Text = strings.Join(v, ", ")` (`render.go:301-303`). Das mitgelieferte Theme fällt für diese Art in den `{{else}} {{.Text}}`-Zweig (`public/default/page.html:53`) und druckt damit „Schublade, Kabelauslass, Verlängerung“ statt eines Rohwerts.

### 4. Schlagwortfeld — **verifiziert**

- Wähler statt Freitext, auf die eigene Website beschränkt: `siteTerms` (`page_fields.go:339-352`) über `terms.ListAll(ctx, websiteID)`; das `<select>` in `field_input.html:151-155`. `TestSchlagwortfeldImSeiteneditor`, `TestSchlagwortauswahlScheitertLeise` — PASS.
- Kürzel gespeichert, Name gedruckt, Umbenennen wirkt ohne Anfassen der Seite: `Resolve` `case KindTerm` schlägt mit dem rohen Kürzel nach (`render.go:194-213`), `Entry.Text = v.Name`. `TestSchlagwortfeldDrucktDenAktuellenNamen` — PASS, und der Test benennt tatsächlich um: vorher `<span class="thema">Möbel</span>`, nach `terms.Rename` `Möbelbau`, die Adresse bleibt `/tag/moebel`. `TestSchlagwortfeldErreichtKeineFremdeWebsite` — PASS.
- Aus dem Feldwähler einer Bausteinart ausgeschlossen, neben `KindRef`: `BlockKinds()` (`field.go:167-177`), verdrahtet in `internal/admin/field.go:241`. Test `field_test.go:1026`.

### 5. `zeit`, `bereich`, `code` — **verifiziert**

- **`zeit`** trägt keine Zeitzone und leer ist nicht Mitternacht: `ParseTimeOfDay` (`field.go:656-672`) liest ohne Datum; `Resolve` gibt `*time.Time`, nil bei leer (`render.go:140-149`). `TestZeitAufgeloest` — PASS, und er misst genau das: `00:00` ergibt einen Zeiger, leer ergibt nil, `tz.Zone()`-Versatz ist 0, `Location()` ist UTC.
- **`bereich`** nimmt eine Zahl zwischen seinen Grenzen, und die gewählte Zahl steht ohne JavaScript im Steuerelement: `<input type="number" step="any" min max>` (`field_input.html:54-56`) — die Zahl *ist* der sichtbare Inhalt. Serverseitig einschliessend geprüft (`field.go:724-737`), verdrehte Grenzen schon bei der Definition abgelehnt (`store.go:463-473`, `ErrRangeInverted`). `TestBereichPruefung`, `TestLeererBereichIstNichtNull`, `TestBereichDrucktDasGetippte` — PASS. Migration `00046_field_kinds.sql` legt `darstellung`, `max_werte`, `min_wert`, `max_wert` als eigene Spalten ohne CHECK an.
- **`code`** geht nie durch Markdown und erscheint wörtlich: im Theme über `html/template` (Spezifikation `TEMPLATE-SPEC.md:539`, `:613-618`), im Baustein in der Einfrierstelle selbst — `renderOwn` `case field.KindCode` schreibt `<pre><code>` mit `html.EscapeString` und ausdrücklich nicht über `prose()` (`block/render.go:325-334`). `TestCodeIstRoherText` und `TestCodeImBausteinWirdMaskiert` — PASS; letzterer misst auch, dass kein `<p>` entsteht, also der Markdownweg nicht genommen wurde. Feste Breite im Editor: `.form-code` mit `--font-mono` und `white-space: pre` (`admin.css:1274-1282`).

Der Browserbeleg für den Bausteinfall fehlt — er gehört zu Kriterium 6, dessen Wortlaut den Browser verlangt; Kriterium 5 tut es nicht.

### 6. Stehendes Tor (QUAL-01, QUAL-02) — **teilweise (Gap)**

**Übersetzungshälfte: verifiziert, selbst gefahren.**

```
$ go run ./tools/i18n
1151 Zeichenketten im Quelltext
en.json      1151 übersetzt, 0 offen, 0 verwaist
es.json      1151 übersetzt, 0 offen, 0 verwaist
fr.json      1151 übersetzt, 0 offen, 0 verwaist
it.json      1151 übersetzt, 0 offen, 0 verwaist
de-CH.json   54 Abweichungen, 0 ohne Gegenstück
```

Dazu `gofmt -l .` still, `go vet ./...` still, `go test ./...` ohne Fehlschlag (alle drei hier gelaufen). Keine Schuldmarke (`TODO`/`FIXME`/`XXX`/`HACK`) in den 37 Nicht-Planungsdateien, die diese Phase angefasst hat.

**Browserhälfte: nicht vollständig.** Der Durchgang vom 5. September ist echt und breit — was er abgedeckt hat, ist in `07-07-SUMMARY.md:240-287` protokolliert. Vier Zeilen fehlen:

1. `code` innerhalb eines Bausteins auf der öffentlichen Seite. Vom Plan selbst als „nicht gefahren“ notiert (`07-07-SUMMARY.md:290`), nur testgedeckt.
2. Der geänderte Hinweistext unter „Möglichkeiten“ (`field_list.html:136`, Commit `f63bc6e`, 18:49).
3. Die neue Ablehnung „eine Auswahl braucht mindestens eine Möglichkeit“ (`store.go:449`, derselbe Commit) — sie erscheint auf dem Definitionsbildschirm.
4. Die geänderte Namensprägung im Bausteineditor (`page_blocks.go:249`, Commit `bf4abdd`, 18:40) — ein sichtbares Bedienelement, dessen Verhalten sich nach dem Durchgang geändert hat.

Der Durchgang ist in `96b1107` dokumentiert, das Review liegt bei `35ab403` (18:33); alle drei Fix-Commits liegen danach. Das ist keine Nachlässigkeit des Durchgangs, sondern die gewöhnliche Folge davon, dass nach dem Klicken noch am Sichtbaren geändert wurde — und genau das ist, was das Tor verbietet.

## Anforderungen

| Anforderung | Status | Beleg |
|---|---|---|
| FIELD-01 Knopfreihe | erfüllt | Kriterium 2 |
| FIELD-02 Mehrfachauswahl | erfüllt | Kriterium 3 |
| FIELD-03 Schlagwortfeld | erfüllt | Kriterium 4 |
| FIELD-04 `zeit` | erfüllt | Kriterium 5 |
| FIELD-05 `bereich` | erfüllt, mit Warnung | Kriterium 5 + WR-03 |
| FIELD-06 `code` | erfüllt | Kriterium 5 |
| FIELD-07 eine Mechanik | teilweise | Kriterium 1 — das Paar ja, die Unterscheidung nur bis `Clean`, das Trennzeichen unverteidigt |
| FIELD-08 Bedingung an der Knopfreihe | erfüllt | Kriterium 2, `TestSchalter` |

## Was ich gefahren habe

| Kommando / Test | Ergebnis |
|---|---|
| `go run ./tools/i18n` | 0 offen, 0 verwaist (en/es/fr/it) |
| `gofmt -l .`, `go vet ./...` | still |
| `go test ./...` | 0 Fehlschläge |
| `TestKnopfreihe`, `TestSchalter` (admin) | PASS |
| `TestMehrfachauswahl{VomFormularBisZurAnzeige,GeleertOderAbwesend,InEinerGruppe,InEigenerBausteinartUeberlebtDasSpeichern}` | PASS |
| `TestSchlagwortfeld{ImSeiteneditor,DrucktDenAktuellenNamen,ErreichtKeineFremdeWebsite,OhneSchlagwortBleibtLeer,Rundreise}` | PASS |
| `TestZeit{Pruefung,Aufgeloest,StehtAlsTextInDerListe}`, `TestBereich{Pruefung,DrucktDasGetippte}`, `TestLeererBereichIstNichtNull`, `TestCodeIstRoherText` | PASS |
| `TestCodeImBausteinWirdMaskiert`, `TestPlainTextNimmtCodeUndMehrfachauswahl`, `TestMehrfachauswahlImBausteinBehaeltAlleHaken` | PASS |
| `TestMehrfachauswahlUeberlebtDieArchivreise` | PASS |
| tmplspec- und template-Fixturewächter | PASS |
| eigener Wegwerftest (`-overlay`): geleert vs. abwesend nach `Clean`+`Encode` | **identisch — Befund** |
| eigener Wegwerftest (`-overlay`): `JoinValues`/`SplitValues` über `"a\nb"` | **3 statt 2 Werte — Befund** |

Beide Wegwerftests liefen über `go test -overlay` gegen eine Datei ausserhalb des Baums; der Arbeitsbaum ist unverändert.

---

_Verifiziert: 2026-09-06_
_Verifizierer: Claude (gsd-verifier)_
