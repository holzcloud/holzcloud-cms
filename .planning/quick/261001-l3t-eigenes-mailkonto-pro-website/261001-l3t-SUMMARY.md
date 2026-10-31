---
phase: quick-261001-l3t
plan: 01
subsystem: mail
tags: [mail, smtp, encryption, admin, settings]
status: complete
requires: []
provides: ["per-website SMTP account", "HOLZCLOUD_SECRET_KEY"]
affects: [internal/mail, internal/outbox, internal/admin, internal/config, cmd/holzcloud]
tech-stack:
  added: []
  patterns: ["AES-GCM with row id as AAD, key derived from an environment variable", "sender chosen per message by website id in one place (mail.Accounts)"]
key-files:
  created:
    - internal/db/migrations/00059_website_mail.sql
    - internal/mail/accounts.go
    - internal/mail/accounts_test.go
    - internal/admin/website_mail.go
    - cmd/holzcloud/templates/admin/website_mail.html
  modified:
    - internal/mail/queue.go
    - internal/outbox/outbox.go
    - internal/config/config.go
    - cmd/holzcloud/main.go
decisions:
  - "Full SMTP account per website (user's choice), not only a From address: a From of another domain through the installation's server fails DMARC."
  - "Password in the database only as ciphertext; key from HOLZCLOUD_SECRET_KEY, consistent with the rule that secrets live in the environment and not in what goes into a backup."
  - "ErrNotConfigured at send time leaves a message waiting instead of counting an attempt, in both the mail queue and the shop outbox."
  - "Removing the account is a plain POST without hx-confirm, so the asserted count of 22 confirmations stays unchanged; the account can be entered again."
metrics:
  completed: 2026-10-01
  tasks: 3
deviations:
  - "Code was written before the GSD records; PLAN and SUMMARY were written afterwards on the user's instruction to run everything through GSD."
  - "Fixed a German letter in a comment of cmd/holzcloud/stylesheet_version_test.go (from 0.0.10) that made tools/english fail on main."
---

# Quick 261001-l3t: eigenes Mailkonto pro Website — Summary

A website can now send through an SMTP account of its own, entered by an admin
under the website's settings, "Versand". Its notifications, sender copies and
order mails go out through that account and under its address; everything else
keeps using the installation's account.

## Verified
- `go test ./internal/mail ./internal/outbox ./internal/config ./cmd/holzcloud` green,
  full `go test ./...` run.
- CI gates: gofmt, vet, tools/english, wasm, cites, assembled, themewords;
  i18n 1981 strings, 0 open, 0 orphaned.
- In the browser against a fake SMTP server: account saved, password stored
  encrypted, the test message and ten pending order mails left through the
  website's account (AUTH PLAIN, From "Ashcroft Cycle Works" <info@velo.test>),
  none through the installation's.
