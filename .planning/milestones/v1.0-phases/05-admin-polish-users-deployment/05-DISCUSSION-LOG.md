# Phase 05: Admin Polish + Users + Deployment - Discussion Log

> **Audit trail only.** Do not use as input to planning, research, or execution agents.

**Date:** 2026-04-14
**Phase:** 05-admin-polish-users-deployment
**Mode:** --auto (all decisions auto-selected as recommended defaults)
**Areas discussed:** Progressive Enhancement, Flash Messages, Dashboard, View Transitions, Loading Indicators, User Management, Deployment

---

## All Areas (Auto Mode)

All gray areas auto-selected with recommended defaults:
- Audit existing handlers for no-JS fallback, ensure full-page path works
- CSS-animated auto-dismiss flash banners (success/error/warning)
- Dashboard with website cards and quick actions
- CSS-only view transitions (progressive enhancement)
- Per-element hx-indicator + global GitHub-style progress bar
- Standard user CRUD with safety guards (can't delete self, can't demote last admin)
- Production systemd + Caddy + documented backup in deploy/ directory

## Claude's Discretion

- Spinner design, dashboard layout, view transition timing, first-login password prompt

## Deferred Ideas

- Dark mode, activity log, password reset, 2FA, Docker, maintenance mode
