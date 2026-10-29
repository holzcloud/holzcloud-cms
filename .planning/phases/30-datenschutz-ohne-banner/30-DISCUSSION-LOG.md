# Phase 30: Datenschutz ohne Banner - Discussion Log

> **Audit trail only.** Do not use as input to planning, research, or execution agents.
> Decisions are captured in CONTEXT.md — this log preserves the alternatives considered.

**Date:** 2026-10-29
**Phase:** 30-datenschutz-ohne-banner
**Areas discussed:** IP im Request-Log, Datenschutz-Vorlage pro Website, Cookie-Test erzwingen, Ansicht-Wahl 365 Tage

---

## IP im Request-Log

| Option | Description | Selected |
|--------|-------------|----------|
| Kürzen (/24, /48) | Fehlersuche nach Netz bleibt möglich | ✓ |
| Gar nicht loggen | Feld entfällt | |
| Ungekürzt, Frist dokumentieren | Berechtigtes Interesse | |

**Frist:** journald-Drop-in im Deploy (14 Tage) ✓; Alternativen: nur dokumentieren, eigene Log-Datei.
**Umfang:** nur Request-Log ✓; Alternative: auch `internal/activity` prüfen.

---

## Datenschutz-Vorlage pro Website

| Option | Description | Selected |
|--------|-------------|----------|
| Bausteine aus Konfiguration | Abschnitte je nach aktiven Funktionen | ✓ |
| Statische Vorlage | Liste von Hand | |
| Eigene Admin-Seite | Formular pro Website | |

**Liste:** Abschnitt der Datenschutzseite ✓; Alternative: Unterseite /cookies.
**Bestand:** nicht anfassen, Hinweis im Admin ✓; Alternative: nur neue Websites.

---

## Cookie-Test und Ansicht-Wahl

| Option | Description | Selected |
|--------|-------------|----------|
| Alle öffentlichen GETs, alle 8 Themes | Test über Beispieldaten | ✓ |
| Nur Standard-Theme | | |

**Laufzeit Ansicht-Wahl:** 30 Tage, als vom Besucher ausgelöst dokumentiert ✓; Alternativen: Sitzungs-Cookie, 365 Tage lassen.
**EXIF:** Regressionstest mit GPS-Bild ✓; Alternative: zusätzlich Eintrag in der Vorlage.

---

## Claude's Discretion

Wortlaut der Textbausteine, Form des Admin-Hinweises, Aufbau der Kürzungsfunktion und ihres Tests.

## Deferred Ideas

- Admin-Seite "Datenschutz" mit Formular pro Website.
- Prüfung/Kürzung von `internal/activity`.
