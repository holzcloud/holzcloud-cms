---
phase: 05-admin-polish-users-deployment
plan: 02
subsystem: user-management
tags: [user-crud, password-change, safety-guards, argon2id]
dependency_graph:
  requires: [05-01]
  provides: [user-crud, password-management, user-templates]
  affects: [admin-routes, users-table]
tech_stack:
  added: []
  patterns: [self-password-verification, last-admin-guard, requireadmin-middleware]
key_files:
  created:
    - internal/db/migrations/00006_user_name.sql
    - internal/admin/user.go
    - cmd/holzcloud/templates/admin/user_list.html
    - cmd/holzcloud/templates/admin/user_form.html
    - cmd/holzcloud/templates/admin/user_password.html
  modified:
    - internal/web/render.go
    - cmd/holzcloud/main.go
decisions:
  - User CRUD routes wrapped in RequireAdmin; password change allows self-access for non-admins
  - redirectBack helper method centralizes htmx HX-Redirect vs standard 303 redirect logic
  - Delete also checks last-admin count (not just self-deletion) for completeness
metrics:
  duration_seconds: 135
  completed: "2026-04-14T15:47:23Z"
---

# Phase 05 Plan 02: User Management CRUD Summary

User CRUD at /admin/users with Argon2id password hashing, self-change requires current password, admin override skips it, last-admin demotion/deletion guard, self-deletion prevention, RequireAdmin on all routes except self password change.

## Task Results

### Task 1: Migration + user CRUD handlers + password change
**Commit:** ab0cc9a

- Migration 00006 adds `name TEXT NOT NULL DEFAULT ''` to users table
- `internal/admin/user.go`: 7 data access helpers (listUsers, getUserByID, createUser, updateUser, updatePassword, deleteUser, countAdmins)
- 5 exported handlers: HandleUserList, HandleUserCreate, HandleUserEdit, HandleUserDelete, HandlePasswordChange
- Safety guards: self-deletion blocked, last-admin demotion blocked via countAdmins check, last-admin deletion blocked
- Self-password-change calls `auth.VerifyPassword` on current password; admin override skips it
- Role validation: only "admin" or "editor" accepted
- UNIQUE constraint errors caught with user-friendly flash messages
- Registered user_list, user_form, user_password in render.go layout pages

### Task 2: User templates + route wiring
**Commit:** 8002965

- `user_list.html`: table with Name/Email/Role columns, Edit/Password/Delete actions, current user row highlighted, delete hidden for self
- `user_form.html`: shared create/edit form; password field only shown in create mode; role select dropdown
- `user_password.html`: conditional current password field via `{{if .IsSelf}}`; new password + confirm fields
- All forms: CSRF hidden input, `hx-disabled-elt="this"` on submit buttons
- Routes wired in main.go: user CRUD under RequireAdmin middleware, password change accessible by authenticated users (handler enforces self-only for non-admins)

## Deviations from Plan

### Auto-fixed Issues

**1. [Rule 2 - Security] Non-admin password change access control**
- **Found during:** Task 2 (route wiring)
- **Issue:** Password change route registered without RequireAdmin to allow self-access, but lacked any authorization check for non-admin users accessing other users' passwords
- **Fix:** Added role check in HandlePasswordChange: non-admin users who aren't the target user get 403 Forbidden
- **Files modified:** `internal/admin/user.go`
- **Commit:** 8002965

**2. [Rule 2 - Security] Last-admin deletion guard**
- **Found during:** Task 1
- **Issue:** Plan specified self-deletion guard but didn't explicitly require last-admin deletion guard (separate from demotion guard)
- **Fix:** Added countAdmins check before deleting any admin user, preventing deletion of the last admin even by another route
- **Files modified:** `internal/admin/user.go`
- **Commit:** ab0cc9a

## Self-Check: PASSED
