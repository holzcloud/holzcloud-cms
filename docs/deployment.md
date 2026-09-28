# Deployment

Target platform is a small `linux/amd64` server. Step-by-step instructions are in
[`deploy/DEPLOY.md`](../deploy/DEPLOY.md); this page is the summary and the
reasoning.

## Building

```bash
# Local development
go build ./cmd/holzcloud

# Production
CGO_ENABLED=0 GOOS=linux GOARCH=amd64 \
  go build -trimpath -ldflags="-s -w" -o holzcloud ./cmd/holzcloud
```

The binary is about 23 MB and fully self-contained: templates, assets, fonts and
migrations are embedded with `embed.FS`, and SQLite is the pure-Go
`modernc.org/sqlite`, so no C toolchain and no shared library are involved. There
are no external files to ship.

## Installing

```bash
# 1. Copy the binary to the server
scp holzcloud user@your-server:/opt/holzcloud/

# 2. Install the systemd service
sudo cp deploy/holzcloud.service /etc/systemd/system/
sudo systemctl enable --now holzcloud

# 3. Caddy for HTTPS (optional)
sudo cp deploy/Caddyfile.example /etc/caddy/Caddyfile
# edit the domains, then: sudo systemctl reload caddy

# 4. Backups
sudo cp deploy/backup.sh /opt/holzcloud/
crontab -e
# 0 3 * * * /opt/holzcloud/backup.sh
```

| File in `deploy/` | Purpose |
|---|---|
| `holzcloud.service` | systemd unit with hardening (`ProtectSystem`, `NoNewPrivileges`, …) |
| `Caddyfile.example` | Caddy reverse proxy with automatic HTTPS |
| `backup.sh` | WAL-safe SQLite backup plus an rsync of media and templates |
| `DEPLOY.md` | step-by-step setup for a Debian/Ubuntu amd64 server |

Behind TLS, set `HOLZCLOUD_SECURE=true` so cookies are marked Secure. See
[configuration](configuration.md).

## The command line

The same binary is the maintenance tool, and every subcommand honours the same
`HOLZCLOUD_*` environment variables as the server:

```
holzcloud [serve]                 run the HTTP server (default)
holzcloud version                 print version and commit
holzcloud user list|create|passwd manage accounts
holzcloud user 2fa status|disable the way back in when a phone is lost
holzcloud backup <dir>            write a verified database snapshot
holzcloud migrate status|up       show or apply pending migrations
holzcloud compact                 rebuild the database file (VACUUM)
holzcloud rerender                re-render every page from its Markdown
holzcloud thumbnails [-force]     generate the scaled copies of older images; -force rebuilds them for every image
holzcloud check                   run an integrity check
holzcloud template check <path>   check a template directory or .zip
holzcloud template spec           print the template authoring specification
holzcloud ai key create           create a key for an AI assistant (-name, -level read|content|admin)
holzcloud ai key list|revoke <id> show or withdraw the keys
holzcloud export -website <id|domain> <dir>
                                  write a website as plain HTML files
```

`holzcloud export` asks the server's own router for every address the website
links to — pages, lists, feed, sitemap, theme files, media — and writes the
answers into an empty directory, ready for any static host. A list's second
page, `/blog?seite=2`, becomes `/blog/seite/2/`, a redirect becomes a page that
forwards without a script, and the site's 404 page is `404.html`. What only a
running server can do is left out and named in the report: search, forms,
protected pages, the shop's cart and checkout.

Passwords are read from stdin, so they never reach the shell history:

```bash
echo -n 'new password' | holzcloud user passwd -email admin@example.com
```

## Backups

Back up the whole data directory; `deploy/backup.sh` does it WAL-safely. Beyond
that, each website can be downloaded as a readable archive — see
[import and export](import-export.md). The two are not the same thing: the
database file is a backup of the installation, a bundle is one website in a form
that survives the installation.

## Docker

A `Dockerfile` is in the repository. The image is a static binary on a minimal
base; mount the data directory as a volume and set the environment as above.

## Signing in through an identity provider

Optional, off unless asked for, and reversible by deleting an environment block.
`deploy/DEPLOY.md` has the whole procedure and the shipped `Caddyfile.example`;
this is the shape of it and the two things worth knowing before you start.

Authentik sits in front as a **forward-auth** outpost: the reverse proxy asks it
about every admin request, and on a yes it copies the person's identity into
request headers. Holzcloud believes those headers only from a peer it already
trusts and only alongside a shared secret it compares in constant time — and it
deletes any such header a *visitor* sent, before anything downstream can read
one. That deletion happens in two places on purpose, here and in the proxy, and
the reason is that neither one is enough alone.

Two things are the operator's to check once against their own installation, and
`DEPLOY.md` says so rather than claiming them:

- **Caddy must be 2.11.2 or newer.** 2.10.0 through 2.11.1 do not remove the
  client's own identity headers on the forward-auth path (CVE-2026-30851), and
  the stable apt package may well be one of them.
- **The shipped example deletes the underscore spellings itself** —
  `X_authentik_username` and its three siblings — because Caddy's own fix
  removes only the canonical hyphenated names. Measured against 2.11.4, not
  inferred from the advisory.

And one consequence that is easy to miss: with single sign-on on, this
installation's **second factor is the one the identity provider enforces**. That
is a deliberate decision, the admin says so on two screens, and it means an
Authentik without a second factor is a Holzcloud without one.

## Versioning

Holzcloud CMS is **alpha**, and the tags count from **`v0.0.1`**. Until 1.0 a
release may break what the one before it did: back up before every update and
read the changelog first. Every 0.x release is published as a pre-release.

The code is older than the count. It was released as `v1.4` to `v2.9.1` first;
those releases and tags were deleted when the count started again, and their
entries remain in `CHANGELOG.md`, below 0.0.1. The repository's own history
begins further back still: it was developed in a private repository before
`v1.4`, and the single commit at the bottom is the beginning of the public
record, not of the project.

That a tag has to be there at all has a practical reason. The version is written
into the binary at build time:

```
-ldflags "-X main.Version=$(git describe --tags --always) ..."
```

`git describe` only finds tags reachable from `HEAD`. With none there, `--always`
takes over and a bare commit hash ends up in the binary instead of a version. With
`v0.0.1` there is one.

What a running binary thinks of itself, ask it directly:

```bash
./holzcloud version        # also -version and --version
```

The milestones under `.planning/` share the same numbering, so a milestone and
the release that closes it carry the same number.
