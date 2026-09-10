---
quick_id: 260910-o6d
slug: block-feldpruefung-und-feed-album
date: 2026-09-10
status: complete
commits:
  red: bda9151
  fix: 9ab7beb
closes:
  - "v1.6-MILESTONE-AUDIT gaps.integration: Feldarten in allen Trägern (broken)"
  - "v1.6-MILESTONE-AUDIT gaps.integration: Alben auf jedem Auslieferungsweg (partial)"
  - "FIELD-02, FIELD-04, FIELD-05, FIELD-07, GAL-03 (partial → satisfied)"
requirements-completed: [FIELD-02, FIELD-04, FIELD-05, FIELD-07, GAL-03]
---

# Quick 260910-o6d — Zusammenfassung

## Was war

Der Meilenstein-Audit v1.6 hat mit einer Sonde gemessen: `block.Set.Clean`
behielt in einer eigenen Bausteinart `janein="vielleicht"`/`"nein"`,
`stufe="99"` (1–5), `zeit="25:99"`, eine Wahl und eine Mehrfachauswahl ausserhalb
der Liste; `renderOwn` gab ein gespeichertes Nein als `hc-ja--…` aus und druckte
Mehrwerte roh. Der Feed bildete `<updated>`/`Last-Modified` ohne
`album.Set.Latest()`.

## Rotbeweis — `bda9151`

Fünf Tests, alle `--- FAIL` ohne Build-Fehler, jeder aus seinem Grund:
`TestCleanRefusesWhatCheckRefusesInAnOwnKind`,
`TestCleanNormalisesAnOwnKindLikeAPage`, `TestAStoredNoIsNotRenderedAsYes`,
`TestAMultiValueInABlockIsReadValueByValue`,
`TestChangingAnAlbumMovesTheFeedsDates`.

## Flick — `9ab7beb`

- `internal/block/block.go` `keepFields`: `field.Clean` für die Schreibweise,
  `field.Check` als Tor; ein abgelehnter Wert wird beim nächsten Speichern
  verworfen. **Ausnahme Verweis**: `safeURL` rendert `#fragment`, `checkLink`
  lehnt es ab — Wächtertest `TestCleanKeepsALinkTheBlockRendererAccepts`.
- `internal/block/render.go`: Ja/Nein über `field.NormalizeBool` (Altbestand
  „nein“ wird sofort richtig gelesen); `KindMulti` über `field.SplitValues` als
  `<ul class="hc-eigen__liste hc-eigen__liste--KEY">`.
- `internal/public/feed.go`: `newest` nimmt `albums.Latest()` auf.
- `cmd/holzcloud/assets/bausteine.css`: `.hc-eigen__liste`.
- `CHANGELOG.md`: zwei Einträge unter „Behoben“.

## Abweichung

`TestMehrfachauswahlInEigenerBausteinartUeberlebtDasSpeichern` wurde rot: er las
mit einem handgebauten `Set` ohne `Choices` zurück, das unter dem neuen Tor
„Eiche“ ablehnt, weil `Decode` auch auf dem Rückweg `Clean` ruft. Gespeichert war
richtig. Der Test liest jetzt mit `block.NewStore(…).Set`, dem Set jedes
produktiven Lesers — stärker als vorher. `block.Store.Set` ist das einzige Set,
das Produktionscode baut (gesucht, nicht angenommen).

**Folge, benannt:** Entfernt ein Betreiber eine Auswahl aus dem Feld einer
Bausteinart, verschwindet ein gespeicherter Wert mit dieser Auswahl beim nächsten
Öffnen aus dem Editor und beim nächsten Speichern aus der Seite — wie bisher schon
ein Wert, dessen Feld entfernt wurde. Ein Seitenfeld meldet in diesem Fall einen
Fehler; der Bausteinpfad hat keine Fehlerfläche.

## Mutationsproben

Unter `bash` 3.2, Sicherungen und Prüfsummen fail-closed, Baum danach
byte-identisch. Jede Flickzeile einzeln zurückgenommen:

| Probe | Ergebnis |
|---|---|
| M1 Check-Ablehnung aus | RED |
| M2 Normalisierung aus | RED |
| M3 Link-Ausnahme aus | RED |
| M4 Ja/Nein wieder `value != "0"` | RED |
| M5 Mehrwert-Zweig aus | RED (erster Lauf brach sicher ab: Anker zweimal vorhanden; mit eindeutigem Anker wiederholt) |
| M6 Feed-Alben aus | RED |

## Tore

gofmt 0 · `go vet ./...` sauber · `go test ./...` ohne Fehlschlag (44 Pakete) ·
`go run ./tools/i18n` 1328 / 0 offen / 0 verwaist auf en, es, fr, it ·
`go run ./tools/wasm -check` aktuell.

## Nicht im Umfang, weiter offen

- Fehlermeldung im Bausteinformular für abgelehnte Werte.
- `<updated>` pro Feedeintrag nach Albumänderung (Album-Set ist feedweit).
- `checkLink` (Seite) und `safeURL` (Baustein) lesen einen Verweis verschieden
  (`#fragment`) — zwei Lesarten, im Code benannt.
