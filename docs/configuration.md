# Configuration

Everything is set through environment variables, prefixed `HOLZCLOUD_`. There is
no configuration file: a process with the right environment is the whole setup.

## Variables

| Variable | Default | Description |
|---|---|---|
| `HOLZCLOUD_PORT` | `8080` | HTTP listen port |
| `HOLZCLOUD_DATA_DIR` | `data` | Directory for the SQLite database, media and uploaded templates |
| `HOLZCLOUD_LOG_LEVEL` | `INFO` | `DEBUG`, `INFO`, `WARN`, `ERROR` |
| `HOLZCLOUD_SECURE` | `false` | Set `true` behind TLS — enables Secure cookies |
| `HOLZCLOUD_MAX_TEMPLATE_SIZE` | `10485760` | Largest template archive, in bytes (10 MB) |
| `HOLZCLOUD_MAX_MEDIA_SIZE` | `5242880` | Largest media file, in bytes (5 MB) |
| `HOLZCLOUD_MAX_VIDEO_SIZE` | `67108864` | Largest video file, in bytes (64 MB) |
| `HOLZCLOUD_MAX_MEGAPIXELS` | `24` | Largest image area decoded for scaled-down versions |
| `HOLZCLOUD_ARGON2_MEMORY` | `65536` | Argon2id memory cost in KB (64 MB) |
| `HOLZCLOUD_ARGON2_ITERATIONS` | `1` | Argon2id time cost |
| `HOLZCLOUD_ARGON2_PARALLELISM` | `2` | Argon2id parallelism |
| `HOLZCLOUD_SMTP_HOST` | — | Mail server. Empty means no mail is sent at all |
| `HOLZCLOUD_SMTP_PORT` | `587` | Port of the mail server |
| `HOLZCLOUD_SMTP_USER` | — | Login name; empty for a relay without authentication |
| `HOLZCLOUD_SMTP_PASSWORD` | — | The password for it |
| `HOLZCLOUD_SMTP_FROM` | — | Sender address. To be set together with `_HOST` |
| `HOLZCLOUD_SMTP_FROM_NAME` | — | Display name of the sender |
| `HOLZCLOUD_SMTP_TLS` | `starttls` | `starttls`, `tls` or `none` |

## E-mail

Sending is off as long as `HOLZCLOUD_SMTP_HOST` and `HOLZCLOUD_SMTP_FROM` are
not both set. Then everything behaves as it did before: an invitation or
password link is shown on screen once and passed on by hand.

With sending on, that link additionally goes out by mail — it is still shown on
screen, because e-mail can be delayed or filtered. If a website has a
notification address under *Settings*, the contact-form plugin reports every new
enquiry there.

This is the only place where this server dials outwards of its own accord, and it
does so to exactly the one server entered here. For a visitor's browser nothing
changes: every resource still comes from this server, and the content security
policy is unchanged. What is sent is plain text, never HTML — tracking pixels and
remotely loaded images have as little business in a mailbox as on a page.

The state is shown in the admin under *E-mail*, together with a test send to your
own address.

## The data directory

All runtime data lives in one directory (`data` by default):

```
data/
  holzcloud.sqlite          the database
  holzcloud.sqlite-wal      the write-ahead log
  csrf.key                  CSRF encryption key (32 bytes, generated on first run)
  media/{website_id}/       uploaded media
  templates/{slug}/         uploaded template archives, extracted
  sprachen/                 admin translations added at runtime (optional)
```

Back up this whole directory and you have everything. `deploy/backup.sh` does it
WAL-safely; see [deployment](deployment.md).

Two directories follow the same rule: **what is on disk wins over what is in the
binary.** A template extracted under `templates/` replaces the built-in one of
the same name, and a language file under `sprachen/` replaces the built-in
translation. Neither needs a restart.

## The database

SQLite in WAL mode, with two connection pools:

- **Write pool** — `MaxOpenConns=1`, so `SQLITE_BUSY` cannot happen
- **Read pool** — higher concurrency for queries
- **Pragmas** per connection: `journal_mode=WAL`, `busy_timeout=5000`,
  `foreign_keys=ON`
- **Migrations** run automatically at startup, through goose, from embedded SQL
