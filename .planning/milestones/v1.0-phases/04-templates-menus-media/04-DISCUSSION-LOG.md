# Phase 04: Templates + Menus + Media - Discussion Log

> **Audit trail only.** Do not use as input to planning, research, or execution agents.

**Date:** 2026-04-14
**Phase:** 04-templates-menus-media
**Mode:** --auto (all decisions auto-selected as recommended defaults)
**Areas discussed:** Template Upload, Hierarchical Menus, Media Upload

---

## All Areas (Auto Mode)

All gray areas auto-selected with recommended defaults:
- Zip extraction with security validation (zip-slip, extension allow-list, size cap)
- Parent_id tree with sort_order for hierarchical menus, up/down reordering
- UUID-prefixed filenames with magic byte MIME validation for media
- Per-website template activation via join table
- Nested `<ul>` output for menus in public templates
- Media serving with immutable cache headers

## Claude's Discretion

- No thumbnail generation in v1, no menu icons, no template HTML validation, media pagination threshold

## Deferred Ideas

- Drag-and-drop menu (V2-10), template preview (V2-12), image resizing, media insertion in editor
