# Requirements: v0.1.0 — Betrieb härten

**Defined:** 2026-10-12
**Core Value:** One small binary runs several websites without dependency soup.

Ziel: holzcloud-cms läuft nachweislich sicher im Betrieb und schließt als Release 0.1.0, die erste Beta.

**Testumgebung (entschieden 2026-10-12):** kein echter Server, sondern ein Linux-Container mit systemd (Docker). Jede Abweichung von einem echten Server (Kernel, cgroups, Sandbox-Direktiven, die ein Container nicht erzwingt) wird bei der Messung ausdrücklich genannt, nie verschwiegen.

## Gehärtete systemd-Unit

- [ ] **UNIT-01**: Die Unit `deploy/holzcloud.service` läuft unter echtem systemd, der Dienst startet und antwortet auf HTTP.
- [ ] **UNIT-02**: Alle Plugins (wazero) laden unter der gehärteten Unit. Der Verdacht `MemoryDenyWriteExecute=true` ist belegt entschieden, die Unit bei Bedarf angepasst.
- [ ] **UNIT-03**: Der Wert von `systemd-analyze security` ist gemessen und im Repo festgehalten.

## Deploy, Update, Rollback

- [ ] **DEP-01**: Eine frische Installation nach `deploy/DEPLOY.md` gelingt Schritt für Schritt; gefundene Abweichungen sind in der Doku korrigiert.
- [ ] **DEP-02**: Ein Update auf eine neue Version samt Migrationen gelingt ohne Datenverlust.
- [ ] **DEP-03**: Ein Rollback auf die Vorversion (Binär und Datenbank) ist gespielt und dokumentiert, einschließlich der Grenze bei Migrationen.

## Backup und Restore

- [ ] **BAK-01**: `backup.sh` erzeugt eine verifizierte Sicherung von SQLite und Medien.
- [ ] **BAK-02**: `restore.sh` stellt auf einem leeren System her; Seiten, Medien und Login funktionieren. Dauer und Größe sind gemessen.
- [ ] **BAK-03**: Ein Lauf über `holzcloud-backup.timer` ist einmal real durchgespielt.

## Repository und Beta

- [ ] **REPO-01**: Die Beta-Kriterien stehen schriftlich: was „0.1.0" verspricht und was nicht.
- [ ] **REPO-02**: CHANGELOG-Eintrag und Release Notes für 0.1.0, Tag `v0.1.0`.
- [ ] **REPO-03**: Die verwaisten Remote-Branches sind aufgelistet, mit Anleitung zum Löschen in der GitHub-Oberfläche. Gelöscht wird dort von Hand (der Proxy antwortet 403).

## Standing Gate

- [ ] **QUAL-01**: `go run ./tools/i18n` meldet `0 open, 0 orphaned`; jede neue Operator-Zeile läuft durch den Katalog.

## Out of Scope

| Feature | Reason |
|---------|--------|
| Neue Funktionen in Verwaltung oder Themes | Dieser Meilenstein härtet den Betrieb, er baut nichts, was ein Betreiber im Menü sieht |
| Test auf einem echten Server | Keiner vorhanden; Container-Ersatz mit benannten Abweichungen |
| Automatisches Löschen der Orphan-Branches | Proxy lehnt Ref-Löschung ab, nur GitHub-UI |
| Hochverfügbarkeit, Replikation, Off-Site-Backup-Dienst | Single-Binary-Ziel; `backup.sh` nennt rsync als Betreiberentscheid |

## Traceability

| Requirement | Phase | Status |
|-------------|-------|--------|
| UNIT-01 | Phase 20 | Pending |
| UNIT-02 | Phase 20 | Pending |
| UNIT-03 | Phase 20 | Pending |
| DEP-01 | Phase 21 | Pending |
| DEP-02 | Phase 21 | Pending |
| DEP-03 | Phase 21 | Pending |
| BAK-01 | Phase 22 | Pending |
| BAK-02 | Phase 22 | Pending |
| BAK-03 | Phase 22 | Pending |
| REPO-01 | Phase 23 | Pending |
| REPO-02 | Phase 23 | Pending |
| REPO-03 | Phase 23 | Pending |
| QUAL-01 | Phase 23 | Pending |

**Coverage:** 13 total, 13 mapped, 0 unmapped.

---
*Requirements defined: 2026-10-12*
