---
phase: 30-datenschutz-ohne-banner
status: deferred
audit_acknowledged:
  milestone: v0.1.1
  at: 2026-10-31
  gap_snapshot: "deferred::scenarios=0"
---

# Phase 30 - offene Pruefung auf einem echten Server

Bewusst ohne diese Pruefung abgeschlossen (Entscheidung 2026-10-31); der Dev-Rechner hat kein systemd.

## journald-Aufbewahrung (DSV-02)

Auf dem Server:

    sudo install -D -m 0644 deploy/journald-holzcloud.conf /etc/systemd/journald.conf.d/holzcloud.conf
    sudo systemctl restart systemd-journald
    systemd-analyze cat-config systemd/journald.conf

Erwartet: die Ausgabe nennt `MaxRetentionSec=14d` und `MaxFileSec=1d`.

## Bekannte, akzeptierte Reste (D-02)

Vier Logstellen schreiben noch die volle Adresse: `internal/admin/login.go:45`, `internal/admin/oidc.go:250`, `internal/admin/forwardauth.go:884`, `internal/web/forwardauth.go:165`. Optional mit `web.TruncateIP` zu kuerzen.

## Lokal

`go run ./tools/english` schlaegt auf dieser Maschine an wegen der unversionierten Datei `build/devseed/main.go` (gitignoriert); kein Befund der Phase.
