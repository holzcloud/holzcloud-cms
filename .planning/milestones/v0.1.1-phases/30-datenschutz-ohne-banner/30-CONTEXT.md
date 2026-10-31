# Phase 30: Datenschutz ohne Banner - Context

**Gathered:** 2026-10-29
**Status:** Ready for planning

<domain>
## Phase Boundary

Das CMS ist DSGVO-konform, ohne dass ein Cookie-Banner nötig ist. Die öffentliche Seite setzt ohne Zutun des Besuchers kein Cookie; was gesetzt wird, ist notwendig oder vom Besucher ausgelöst und steht in der Erklärung. Keine Rechtsberatung: Texte sind als Vorlage gekennzeichnet.

Keine neuen CMS-Funktionen über die fünf Audit-Punkte der ROADMAP hinaus.

</domain>

<decisions>
## Implementation Decisions

### IP im Request-Log (`internal/web/logging.go`)
- **D-01:** Die Client-IP wird vor dem Schreiben gekürzt: IPv4 auf /24, IPv6 auf /48. Das Feld `ip` bleibt, trägt aber nur die gekürzte Adresse.
- **D-02:** Die Kürzung gilt nur für das Request-Log. Rate-Limit hält die volle IP weiter nur im Speicher. `internal/activity` wird nicht angefasst.
- **D-03:** Die Aufbewahrungsfrist wird durchgesetzt, nicht nur beschrieben: `deploy/` liefert ein journald-Drop-in mit `MaxRetentionSec=14d` mit. Die Frist steht im Datenschutztext und ein Test hält Drop-in und Text zusammen.

### Datenschutz-Vorlage pro Website
- **D-04:** Die Starter-Seite `datenschutz` (`internal/admin/starter.go`) wird aus Bausteinen zusammengesetzt, je nach aktiven Funktionen der Website (Shop/Warenkorb-Cookie, Seitenschutz, Zahlungsanbieter, Formulare). Der Operator ergänzt den Verantwortlichen. Der Text bleibt als Vorlage gekennzeichnet (Entwurf, "ersetzt keine Rechtsberatung").
- **D-05:** Cookie- und Auftragsverarbeiter-Liste sind ein Abschnitt der Datenschutzseite: Tabelle Name, Zweck, Laufzeit, Rechtsgrundlage; Zahlungsanbieter mit Hinweis auf den AV-Vertrag. Keine eigene Unterseite.
- **D-06:** Bestehende Datenschutzseiten werden nicht verändert. Der Admin zeigt einen Hinweis und bietet die neue Vorlage als Entwurf an.
- **D-07:** Impressum- und Datenschutz-Starterseiten gibt es bereits; Audit-Punkt 4 der ROADMAP ist insofern teilweise überholt. Es geht um Inhalt der Datenschutzvorlage und die Liste, nicht um das Anlegen der Seitentypen.

### Cookies
- **D-08:** Der Ansicht-Wahl-Cookie (Audience, `internal/shop/pricing.go`) läuft 30 statt 365 Tage und wird als vom Besucher ausgelöst in der Cookie-Liste geführt.
- **D-09:** Ein Test erzwingt, dass jeder öffentliche GET ohne Interaktion kein `Set-Cookie` sendet. Er läuft über die Beispieldaten aller acht Themes (Startseite, Archiv, Suche, 404, Shop-Listen), nicht nur das Standard-Theme.

### Metadaten beim Upload
- **D-10:** Die EXIF-Entfernung (`internal/media/intake.go`) wird nur durch einen Regressionstest gesichert: Fixture mit GPS-/Kamera-EXIF, Ergebnis ohne diese Tags, Orientierung bleibt erhalten (Quick-Tasks 261026-rrh, 261026-sxy).

### Claude's Discretion
- Genaue Gliederung und Wortlaut der Textbausteine, solange sie als Vorlage gekennzeichnet sind und nur auf Deutsch über den Katalog (`{{t}}`, `go run ./tools/i18n` grün) laufen.
- Form des Hinweises im Admin (D-06).
- Aufbau der Kürzungsfunktion und des Tests dafür.

</decisions>

<canonical_refs>
## Canonical References

**Downstream agents MUST read these before planning or implementing.**

### Projekt und Roadmap
- `.planning/ROADMAP.md` §Phase 30 — Ziel und die fünf Audit-Punkte (2026-10-29)
- `.planning/REQUIREMENTS.md` — Anforderungen von 0.1.1; Phase 30 hat dort noch keine Einträge (TBD), der Planer legt sie an
- `CLAUDE.md` §Nothing Loads at Runtime, §Translation, §Security — CSP, Katalog, `tools/i18n`-Gate

### Code
- `internal/web/logging.go` — `AccessLog`, Feld `ip`
- `internal/web/clientip.go` — `ClientIPResolver`
- `internal/shop/pricing.go` — `SetAudienceCookie` (365 Tage)
- `internal/shop/cart.go` — Warenkorb-Cookie
- `internal/public/access.go` — Seitenschutz-Unlock-Cookie
- `internal/admin/starter.go` — Starterseiten Impressum und Datenschutz
- `internal/media/intake.go` — EXIF-Entfernung beim Upload
- `deploy/` — Systemd-Einheiten, Ort für das journald-Drop-in

</canonical_refs>

<code_context>
## Existing Code Insights

### Reusable Assets
- `starterPages` in `internal/admin/starter.go`: Muster für Entwurfsseiten mit offensichtlichen Platzhaltern.
- `internal/template/sample.go` (`SampleData`/`MinimalData`): Beispieldaten je Theme für den Cookie-Test.

### Established Patterns
- Entwurf zuerst, Platzhalter deutlich sichtbar: halb ausgefüllter Rechtstext, der fertig aussieht, ist der gefährliche Zustand.
- Betreiber-Texte laufen durch den Katalog; ein mit `fmt.Sprintf` gebauter Satz ist für `tools/i18n` unsichtbar.

### Integration Points
- `AccessLog` ist die einzige Stelle, die die IP ins Log schreibt.
- Das Drop-in gehört zu den ausgelieferten `deploy/`-Dateien und damit unter die bestehenden Prüfungen für diese Dateien.

</code_context>

<specifics>
## Specific Ideas

- Kürzung /24 bzw. /48, damit Fehlersuche nach Netz möglich bleibt.
- Frist 14 Tage im journald-Drop-in, im Datenschutztext genannt und per Test gekoppelt.

</specifics>

<deferred>
## Deferred Ideas

- Eigene Admin-Seite "Datenschutz" mit Formular pro Website (Verantwortlicher, Anbieter ankreuzen) — mögliche spätere Phase.
- Kürzung oder Prüfung von `internal/activity` — nicht Teil dieser Phase.

</deferred>

---

*Phase: 30-datenschutz-ohne-banner*
*Context gathered: 2026-10-29*
