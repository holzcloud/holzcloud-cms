# Phase 20: Hardened unit under real systemd - Discussion Log

> **Audit trail only.** Do not use as input to planning, research, or execution agents.
> Decisions are captured in CONTEXT.md — this log preserves the alternatives considered.

**Date:** 2026-10-12
**Phase:** 20-hardened-unit-under-real-systemd
**Areas discussed:** Container-Architektur, Container-Rezept & Übergabe, MDWE-vs-wazero Beweis, Security-Score festhalten

---

## Container-Architektur
Options: Beides (amd64 massgeblich) / Nur amd64 (Emulation) ✓ / Nur arm64 nativ
**User's choice:** Nur amd64 (Emulation)

## Container-Rezept
Options: deploy/test/ Dockerfile + run.sh ✓ / docker compose / Nur Doku-Rezept
**User's choice:** deploy/test/ mit Dockerfile + run.sh

## MDWE vs wazero
Options: Beide Varianten messen ✓ / Gleich Interpreter / MDWE vorsorglich entfernen
**User's choice:** Beide Varianten messen, dann entscheiden

## Security-Score
Options: Rohausgabe in deploy/SECURITY-SCORE.md ✓ / Mit Zielwert / Nur Verifikation
**User's choice:** Rohausgabe ohne Zielwert

## Claude's Discretion
Base image, Docker flags, readiness polling, plugin exercise method, score file name.

## Deferred Ideas
None.
