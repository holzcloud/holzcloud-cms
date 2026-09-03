# Phase 03: Multi-Site + Pages + Public Rendering - Discussion Log

> **Audit trail only.** Do not use as input to planning, research, or execution agents.
> Decisions are captured in CONTEXT.md — this log preserves the alternatives considered.

**Date:** 2026-04-14
**Phase:** 03-multi-site-pages-public-rendering
**Mode:** --auto (all decisions auto-selected as recommended defaults)
**Areas discussed:** Website CRUD, Host-Based Routing, Page Authoring, Draft/Published Workflow, Admin Page List, Public Template Engine, Caching Headers, Public 404

---

## All Areas (Auto Mode)

All gray areas were auto-selected with recommended defaults. No interactive discussion occurred.

Key auto-decisions:
- Standard CRUD with domain association table for multi-site
- Middleware with cached domain lookup for host routing
- Store both markdown and rendered HTML for pages
- Strict published-only public queries (no draft preview)
- htmx partial pagination + click-to-edit for page list
- Disk-first with embedded fallback for templates
- Standard HTTP caching with ETag/Last-Modified
- Site-themed 404 page for unknown slugs

## Claude's Discretion

- goldmark extensions, page editor UX, pagination style, default template design, homepage concept

## Deferred Ideas

- SEO meta fields, scheduled publishing, content versioning, FTS, draft preview, sitemap/robots.txt
