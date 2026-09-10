---
quick_id: 260906-menu-scope
slug: menu-scope
date: 2026-09-06
status: complete
record: REPORT.md
commits:
  red: de4a1ce
  fix: 5e453a9
  docs: 8559e6e
debug_session: .planning/debug/menu-item-cross-website-write.md
summary_written: 2026-09-10
---

# Quick 260906-menu-scope — Zusammenfassung

Nachgetragen beim Meilenstein-Abschluss v1.6 (2026-09-10). Der Auftrag war am
2026-09-06 erledigt und hat seinen vollständigen Bericht in `REPORT.md`
hinterlassen, aber keine Datei, die das Abschlusswerkzeug als Abschluss liest —
darum führte der Abschluss-Audit ihn als „missing“. Diese Datei ist der Zeiger,
kein neuer Befund.

**Was war:** Vier Menüeintrags-Handler prüften die Website aus der Adresse, aber
nie, dass das Menü aus der zweiten Kennung zu dieser Website gehört. Ein auf
Website A beschränkter Redakteur konnte die Navigation von Website B
umschreiben (auch umsortieren — der vierte Handler war im ursprünglichen
Bericht nicht genannt).

**Rotbeweis** `de4a1ce`, **Flick** `5e453a9` (`menuOfWebsite`/`itemOfWebsite` in
`internal/admin/menu.go`, alle sieben Menü-Handler darüber), Unterlagen
`8559e6e`. Muster im Wissensbestand `.planning/debug/knowledge-base.md`.

**Beim Abschluss nachgefahren (2026-09-10):** alle sechs Tests aus
`internal/admin/menu_scope_test.go` PASS, zusammen mit den neun aus
`product_scope_test.go` und `order_scope_test.go`.
