---
quick_id: 260907-product-scope
slug: product-scope
date: 2026-09-07
status: complete
record: REPORT.md
commits:
  red: 071bead
  fix: 2e43bc1
  red_second: fb9c76a
  fix_second: 25c134e
debug_session: .planning/debug/product-cross-website-write.md
summary_written: 2026-09-10
---

# Quick 260907-product-scope — Zusammenfassung

Nachgetragen beim Meilenstein-Abschluss v1.6 (2026-09-10). Der Auftrag war am
2026-09-07 erledigt und hat seinen vollständigen Bericht in `REPORT.md`
hinterlassen, aber keine Datei, die das Abschlusswerkzeug als Abschluss liest.
Diese Datei ist der Zeiger, kein neuer Befund. Die Debug-Sitzung stand aus
demselben Grund noch auf `verifying` und ist mit denselben Belegen auf
`resolved` gesetzt.

**Was war:** `handleProductSave` reichte eine Produktkennung aus der Adresse an
`shop.Store.Update`, dessen `WHERE` nur die Kennung trug. Ein auf Website A
beschränkter Redakteur konnte ein Produkt von Website B umbenennen, umpreisen
und seine Kategorien mitnehmen. Beim Nachprüfen des Ladens kam ein zweiter
Befund derselben Familie dazu: die Bestellansicht schickte die Kundenmail einer
fremden Website erneut (`mail_id` aus dem Formular, `outbox.Store.Retry` ohne
`website_id`).

**Rotbeweise** `071bead` und `fb9c76a`, **Flicke** `2e43bc1` (Update/Delete mit
`AND website_id`, `ErrNotFound` → 404) und `25c134e` (Retry mit `AND
website_id`, eine Antwort für „schon gesendet“, „gibt es nicht“ und „nicht
deine“).

**Beim Abschluss nachgefahren (2026-09-10):** die sieben Tests aus
`internal/admin/product_scope_test.go` und die zwei aus `order_scope_test.go`
PASS, zusammen mit den sechs aus `menu_scope_test.go`.

**Offen geblieben, wie im Bericht benannt:** bereits über die Grenze
geschriebene `product_terms`-Zeilen in einer laufenden Datenbank repariert kein
Code; `SetTerms`, `SetGallery`, `AdjustStock` bleiben kennungsgebunden und sind
nur sauber, solange ihre Aufrufer das Produkt zuerst beweisen.
