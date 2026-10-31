# Phase 21: Deploy, update, rollback - Discussion Log

> **Audit trail only.** Do not use as input to planning, research, or execution agents.
> Decisions are captured in CONTEXT.md — this log preserves the alternatives considered.

**Date:** 2026-10-12
**Phase:** 21-deploy-update-rollback
**Areas discussed:** none interactively — the user delegated all four gray areas to Claude

## Gray areas offered
- Fresh-install harness (verbatim script vs. step by step; Caddy/scp in container)
- Update test: which migration (real tags vs. synthetic; meaning of "no data loss")
- Rollback and migration limit (snapshot copy vs. goose down)
- Doc changes & automation (fix in place vs. new section; scripts vs. findings only)

**User's choice:** "du kannst alles selber entscheiden"

## Claude's Discretion
All areas; decisions D-01..D-13 in CONTEXT.md. Snapshot-based rollback chosen over goose down; two real tagged builds for the update; scripts committed under deploy/test/.

## Deferred Ideas
- goose down as operator command; automatic rollback on failed health check.
