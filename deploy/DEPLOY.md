# Deploying Holzcloud CMS on a linux/amd64 server

This guide covers building, installing, and running Holzcloud CMS on a fresh Debian or Ubuntu server (x86-64). Nothing here assumes a particular host: a small VPS, a bare-metal box, or a node in a cluster all work the same way.

## Prerequisites

On the **server**, you need:

- Debian or Ubuntu (x86-64); any systemd-based Linux works
- `sqlite3` CLI (for backups): `sudo apt install sqlite3`
- A domain name pointed at the server's public IP

On your **development machine**, you need:

- Go 1.22 or later
- The Holzcloud source code

**Note:** Go is NOT needed on the server. The binary is built on the development machine and is fully self-contained — all templates, assets, and migrations are embedded.

## Storage

SQLite in WAL mode wants a disk whose fsync is honest. Any ordinary SSD-backed volume qualifies. What matters more than the device is that the data directory lives on a filesystem you can snapshot and back up, and that it is not the same disk your backups land on.

## Build

On your development machine:

```bash
CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -trimpath -ldflags="-s -w" ./cmd/holzcloud
```

This produces a single `holzcloud` binary (typically 15-25 MB) for linux/amd64.

## The container image

If you run Kubernetes or Docker rather than systemd, there is an image:

```
ghcr.io/holzcloud/holzcloud-cms:<tag>
```

It is the same binary in a two-stage build on `distroless/static`, so the image
holds the program, the root certificates and nothing else. What it needs from
you:

- **A writable `/data`.** That is where the SQLite file, the media and the
  uploaded templates live. The container runs as uid **65532** (`nonroot`), so
  the mounted volume has to belong to it — in Kubernetes through `fsGroup`.
- **One replica, and `Recreate` rather than `RollingUpdate`.** SQLite tolerates
  many readers and one writer; two pods on one volume are two writers.
- **`HOLZCLOUD_SECURE=true`** behind TLS, so the session cookie carries the
  Secure flag.
- **`HOLZCLOUD_LISTEN=0.0.0.0`**, because the process binds `127.0.0.1` by
  default and a published port reaches a process that bound loopback *inside its
  own network namespace* and finds nobody there. Without this the container
  starts, logs nothing wrong, and answers no request.
- **`/healthz`** for both probes. It answers 200 as soon as the database is
  open.

The image is built for linux/amd64 only. `docker run --rm -v holzcloud:/data -p
8080:8080 -e HOLZCLOUD_LISTEN=0.0.0.0 ghcr.io/holzcloud/holzcloud-cms:<tag>` is
enough to try it.

## Transfer to the server

```bash
scp holzcloud user@your-server:/tmp/holzcloud
scp -r deploy/ user@your-server:/tmp/deploy/
```

## Install

SSH into the server and run:

```bash
# Create system user (no login shell, no home directory)
sudo useradd -r -s /usr/sbin/nologin holzcloud

# Create directory structure
sudo mkdir -p /opt/holzcloud/data
sudo mkdir -p /opt/holzcloud/backups

# Copy binary
sudo cp /tmp/holzcloud /opt/holzcloud/holzcloud
sudo chmod 755 /opt/holzcloud/holzcloud

# Copy deploy files
sudo cp /tmp/deploy/backup.sh /opt/holzcloud/backup.sh
sudo chmod 755 /opt/holzcloud/backup.sh

# Set ownership
sudo chown -R holzcloud:holzcloud /opt/holzcloud/data
```

## systemd Service

```bash
# Install service file
sudo cp /tmp/deploy/holzcloud.service /etc/systemd/system/holzcloud.service

# Reload systemd, enable and start
sudo systemctl daemon-reload
sudo systemctl enable holzcloud
sudo systemctl start holzcloud

# Check status
sudo systemctl status holzcloud
```

The service is hardened with security flags (see `holzcloud.service`): the process cannot escalate privileges, cannot write outside its data directory, and runs with restricted system call access.

## Environment Variables

Configure by editing the service file (`sudo systemctl edit holzcloud`) or adding a drop-in:

Every variable is prefixed `HOLZCLOUD_`. A name without the prefix is ignored
without a word — set `DATA_DIR` instead of `HOLZCLOUD_DATA_DIR` and the service
keeps writing to the default directory, which means the root filesystem fills up
while the volume you mounted for data stays empty.

| Variable | Default | Description |
|----------|---------|-------------|
| `HOLZCLOUD_PORT` | `8080` | HTTP listen port |
| `HOLZCLOUD_LISTEN` | `127.0.0.1` | The address the server binds. The process used to listen on every interface; it no longer does. A proxy on the same host — the documented deployment — is covered by the default. A proxy on another machine, or a container whose port is published, needs an explicit address here (`0.0.0.0`, or `::` for both families) |
| `HOLZCLOUD_DATA_DIR` | `data` (relative!) | Database, media, and template storage — set this explicitly |
| `HOLZCLOUD_LOG_LEVEL` | `INFO` | Logging level: `DEBUG`, `INFO`, `WARN`, `ERROR` |
| `HOLZCLOUD_SECURE` | `false` | Secure cookie flag — set `true` behind HTTPS |
| `HOLZCLOUD_TRUSTED_PROXIES` | `127.0.0.1/32,::1/128` | CIDRs whose `X-Forwarded-For` is believed. Caddy on the same host is covered by the default; a proxy on another machine must be listed here or the login throttle will treat every visitor as one client |
| `HOLZCLOUD_MAX_TEMPLATE_SIZE` | `10485760` | Max template archive size in bytes |
| `HOLZCLOUD_MAX_MEDIA_SIZE` | `5242880` | Max media file size in bytes |
| `HOLZCLOUD_MAX_MEGAPIXELS` | `24` | Pixel budget for the variant pipeline; a byte limit is not a pixel limit |
| `HOLZCLOUD_ARGON2_MEMORY` | `65536` | Argon2id memory cost in KB |
| `HOLZCLOUD_ARGON2_ITERATIONS` | `1` | Argon2id time cost |
| `HOLZCLOUD_ARGON2_PARALLELISM` | `2` | Argon2id parallelism |

An invalid value is no longer silently replaced by the default: the process
reports every bad setting at once and refuses to start. The effective
configuration is written to the log on every start, so `journalctl -u holzcloud`
answers what the service actually resolved.

After changing environment variables:

```bash
sudo systemctl daemon-reload
sudo systemctl restart holzcloud
```

## Caddy (Reverse Proxy + HTTPS)

Caddy automatically obtains and renews TLS certificates via Let's Encrypt.

```bash
# Install Caddy
sudo apt install -y debian-keyring debian-archive-keyring apt-transport-https
curl -1sLf 'https://dl.cloudsmith.io/public/caddy/stable/gpg.key' | sudo gpg --dearmor -o /usr/share/keyrings/caddy-stable-archive-keyring.gpg
curl -1sLf 'https://dl.cloudsmith.io/public/caddy/stable/debian.deb.txt' | sudo tee /etc/apt/sources.list.d/caddy-stable.list
sudo apt update && sudo apt install caddy

# Configure
sudo cp /tmp/deploy/Caddyfile.example /etc/caddy/Caddyfile
# Edit /etc/caddy/Caddyfile — replace example.com with your domain

sudo systemctl reload caddy
```

For multiple websites, add one domain block per site in the Caddyfile. Holzcloud resolves the correct website by the incoming `Host` header.

**If you intend to use single sign-on, Caddy must be 2.11.2 or newer.**
`caddy version` says which you have, and
[Single sign-on (Authentik forward auth)](#single-sign-on-authentik-forward-auth)
below says what is wrong with the versions before it. Without single
sign-on the version does not matter; any Caddy that still gets security
updates serves this file.

## Single sign-on (Authentik forward auth)

Off unless you switch it on. With `HOLZCLOUD_SSO_ENABLED` unset — the default —
not one line of this path executes and the password login is exactly what it was.

### What it does, and what it does not

Whoever your Authentik has already signed in reaches `/admin` without a second
sign-in. Caddy asks the Authentik outpost who the visitor is, copies the answer
onto the forwarded request as a handful of `X-authentik-…` headers, and adds a
shared secret that says the request really came through your own proxy.
Holzcloud believes that claim only from a trusted peer, only with the right
secret, and it deletes every inbound identity header itself — on every path,
whoever the peer was — before any handler runs. A visitor who writes those
headers by hand is refused, and gets the ordinary login form.

**The password path is untouched, and it is the way back in when the identity
provider is down.** The login form, the throttle, two-factor, the recovery codes
and `holzcloud user 2fa disable` all behave exactly as they did. If the identity
provider is unreachable and nobody can sign in, see [Locked out of an
administrator account](#locked-out-of-an-administrator-account) further down;
that route needs shell access on this machine and nothing else.

### The settings

All seven are read only when `HOLZCLOUD_SSO_ENABLED` is true, and an invalid
combination stops the process at start-up rather than at the first sign-in.

| Variable | Default | Description |
|----------|---------|-------------|
| `HOLZCLOUD_SSO_ENABLED` | `false` | The master switch. False means the whole path is dead code |
| `HOLZCLOUD_SSO_SECRET` | — | The shared secret the proxy sends in `X-Holzcloud-Proxy-Secret`. Environment only, never in the database and never in the log. Switched on without one, the service refuses to start |
| `HOLZCLOUD_SSO_ADMIN_GROUP` | — | The identity provider group that grants administration. See below |
| `HOLZCLOUD_SSO_WEBSITE_GROUPS` | — | Comma-separated `group=websiteID` pairs. A group not listed here grants no website. Ids and not names, because `websites` has no slug column and a rename would silently unassign everybody |
| `HOLZCLOUD_SSO_PROVISION` | `false` | Create an account for an identity the provider vouches for but this installation has never seen. See below |
| `HOLZCLOUD_SSO_DEFAULT_WEBSITE` | — | The website a newly created account is assigned to. Required whenever provisioning is on, and checked against the database at start-up |
| `HOLZCLOUD_SSO_SIGN_OUT_PATH` | `/outpost.goauthentik.io/sign_out` | Where the sign-out button sends the browser. A path on this server, never a URL — validated at start-up as exactly one leading slash |

**`HOLZCLOUD_SSO_ADMIN_GROUP` has no default, deliberately.** An empty value is a
legal configuration and means that no group grants administration. A default
group name would be a group name somebody at the identity provider can create.

**`HOLZCLOUD_SSO_PROVISION` and `HOLZCLOUD_SSO_DEFAULT_WEBSITE` belong together,
and the service refuses to start with the first set and the second missing.**
This is the most important paragraph in this section. Holzcloud expresses "this
person may enter every website" as *no website assignment at all* — see
[Rights per person](../docs/security.md#rights-per-person). That reading is
correct for an account you created by hand, and it inverts under provisioning: a
freshly created account has no assignment *by construction*, so without a
default website the first stranger who authenticates at your identity provider
would get editor access to every website in the installation. The refusal is
what stops that, and it is loud at start-up rather than quiet at the first
sign-in. For the same reason, when `HOLZCLOUD_SSO_WEBSITE_GROUPS` **is** set
and an existing account's groups match none of the pairs in it, that sign-in is
refused rather than written back as an empty assignment — removing somebody's
last website group must take access away, not grant everything. With
`HOLZCLOUD_SSO_WEBSITE_GROUPS` unset the website half does not run at all, which
is the difference between a setting that grants to nobody and a setting that
does not decide.

**An account is linked to one identity, and an address is never enough.** A
sign-in reaches the account linked to its `X-authentik-username`. It does not
reach an account because the identity arrives with that account's address: an
address is something your identity provider emits, and in many setups something
a person can change about themselves. Until this was fixed, whoever could make
the identity provider emit an administrator's address signed in as that
administrator.

An account created by provisioning is linked to the identity it was created for.
An account you create by hand is not reachable through single sign-on until you
link it:

```bash
holzcloud user sso -email ada@example.com -username ada
```

`-unlink` removes the link. An identity whose username is linked to nothing but
whose address belongs to an account is refused — with provisioning on as well —
rather than linked on first sight, because otherwise whoever signs in first on
the day you switch this on decides which account is whose.

Group membership is re-read on **every** sign-in. Taking a group away at the
identity provider takes the access away here at the next sign-in, not at the
next session expiry.

### The minimum Caddy version is 2.11.2

`caddy version` tells you what you have. Anything from **2.10.0 through 2.11.1**
is affected by **CVE-2026-30851** (GHSA-7r4p-vjf4-gxv4), and it is worth
understanding rather than upgrading past: `forward_auth … { copy_headers X-Foo }`
generated a *conditional set* of `X-Foo` and **no delete** of the client's own
inbound `X-Foo`. Whenever the outpost answers 200 without emitting that header —
an anonymous route, an account with no e-mail address, any header Authentik chose
not to send — the visitor's own value was forwarded to the backend verbatim. The
defect had been latent since November 2024, so it is in the Caddy a lot of
servers currently have from the stable apt repository.

The shipped `deploy/Caddyfile.example` **deletes each copied header explicitly**,
in both the hyphenated and the underscored spelling, before `forward_auth` is
asked anything. A correct Caddyfile is therefore not something you have to
derive. The underscored spelling matters on a *fixed* Caddy too: Go treats
`X-authentik-email` and `X_authentik_email` as two different headers, and the
fix's own delete covers only the canonical hyphenated name (measured against
Caddy 2.11.4 with `caddy adapt`). That is the companion advisory
GHSA-f59h-q822-g45g / CVE-2026-52845.

Holzcloud strips all of these again itself regardless — and it matters to be
exact about what that does and does not cover. It keeps every identity header
away from a visitor who reaches this service directly, and away from any
spelling Caddy did not canonicalise. It does **not** protect against a Caddy that
forwards the visitor's own `X-authentik-email` under the canonical name: by the
time the request arrives here it comes from the trusted proxy with the right
secret, and a forwarded value looks exactly like one the outpost sent. A header
that arrives with **two** values is refused outright, because that is the shape a
proxy leaves when it appends instead of replacing. A single forwarded value
cannot be told apart. **Against that case the Caddy version and the delete lines
are the only defence**, which is why checking them below is a requirement and not
background.

### Two things you verify once, against your own instance

Neither can be looked up from here, and neither is a claim this project makes
about your setup. Both are checks you run once, when you first wire it up.

1. **That your Authentik really emits `X-authentik-username`, with the value you
   expect.** The identity is pinned to the username and deliberately *not* to
   `X-authentik-uid`: `-uid` carries the OIDC subject, whose shape depends on the
   provider's **Subject mode** and defaults to a hashed identifier. Read the
   header your outpost actually sends — Authentik's own provider preview, or a
   temporary `respond {header.X-authentik-username}` route in Caddy, will show
   it. Renaming a user at the identity provider **unlinks them here**: the new
   username is linked to no account, so that person cannot sign in until you
   link it again with `holzcloud user sso` (below). With provisioning on they are
   refused too, because their address already belongs to the old account.

2. **That your Caddy is 2.11.2 or newer, and that the configuration it generates
   really carries a delete per copied header.** The version alone is not the
   check; the generated configuration is:

   ```bash
   caddy version
   caddy adapt --config /etc/caddy/Caddyfile 2>/dev/null \
     | grep -o '"delete":\[[^]]*\]'
   ```

   You should see one entry per copied header in the hyphenated spelling from
   the shipped example, one in the underscored spelling, and — on 2.11.2 and
   newer — Caddy's own canonical delete beside them.

### The acceptance test, and the answer that is not a pass

One command, run **from a second machine**, over IPv4 **and** over IPv6:

```bash
curl -sS -o /dev/null -w '%{http_code} %{redirect_url}\n' \
  -H 'X-authentik-email: stranger@example.com' \
  http://<server>:8080/admin/
```

The expected answer is `303` with a redirect to `…/admin/login` — **a login
form, not a dashboard.** A stranger who writes the header themselves is nobody.

**`Connection refused` is not a pass.** It means the port is not reachable from
where you are standing, which is the default and is good, but it proves the
*listener* and says nothing whatever about whether a header is believed. To
actually test the header the port has to answer: set `HOLZCLOUD_LISTEN=0.0.0.0`
temporarily, restart, run the command from the other machine over both address
families, and then put the setting back. A test that cannot fail is not a test.

### E-mail addresses above plain ASCII are refused

An identity whose e-mail address contains any character above plain ASCII is
refused at sign-in, and the person falls through to the password form. The
reason is in the schema: `users.email` is unique under SQLite's
`COLLATE NOCASE`, which folds ASCII and nothing else. `Müller@example.com` and
`müller@example.com` are therefore two rows and would become two accounts — and
if one of them is an administrator, two administrators. Refusing is the only
answer that does not guess which one was meant. Give such a person an ASCII
address at the identity provider, or create the account here by hand.

### The second factor is whatever your Authentik enforces

An Authentik session satisfies this installation's two-step requirement
unconditionally. **So this installation's second factor is the one your identity
provider asks for.** If your Authentik is satisfied by a password alone, then
every administrator who signs in through it has a single factor, and the
compulsory TOTP that Holzcloud requires of administrators on the password path
does not apply to them.

That is a deliberate decision and not an oversight — asking for a second factor
twice is how people are trained to click past them — but it moves the guarantee
onto your identity provider, and it is your job to put an authenticator or a
passkey stage in front of the Holzcloud application in Authentik. The same
dependency is stated inside the admin, on *My account* and on the user list, so
whoever administers the installation reads it there without opening this file.

## First Run

1. Ensure the service is running: `sudo systemctl status holzcloud`
2. Visit `https://your-domain/admin` in your browser
3. Create your initial admin account through the setup form
4. Start adding websites, pages, and content

## Backups

`backup.sh` writes a timestamped directory containing the database, the CSRF
key, media and user templates, plus a `MANIFEST` naming the version it came
from. The database snapshot is taken by the binary itself via `VACUUM INTO` and
**verified with an integrity check** before it counts as a backup — an
unverified copy is how a corrupted database quietly replaces every good
snapshot you have.

Enable the timer instead of a cron entry, so `systemctl list-timers` shows
whether backups are actually running:

```bash
sudo cp deploy/holzcloud-backup.{service,timer} /etc/systemd/system/
sudo systemctl daemon-reload
sudo systemctl enable --now holzcloud-backup.timer
systemctl list-timers holzcloud-backup.timer

# Manual run
sudo -u holzcloud /opt/holzcloud/deploy/backup.sh /opt/holzcloud/backups
```

Retention defaults to 30 days (`RETENTION_DAYS`). Set `REMOTE_TARGET` in
`holzcloud-backup.service` to rsync the whole backup root off the box — a backup
on the same disk does not survive the disk failure it exists for, and a backup
that has never left the machine is not yet a backup.

Keep the journal bounded while you are here:

```
# /etc/systemd/journald.conf
SystemMaxUse=200M
```

**Security note:** Backup directories are created with `700` permissions because
they contain the database, which includes password hashes, and the CSRF key.

## Updating

```bash
# Build new binary on dev machine
CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -trimpath -ldflags="-s -w" ./cmd/holzcloud

# Transfer to the server
scp holzcloud user@your-server:/tmp/holzcloud

# On the server: stop, replace, start
sudo systemctl stop holzcloud
sudo cp /tmp/holzcloud /opt/holzcloud/holzcloud
sudo systemctl start holzcloud
```

Migrations run automatically on startup — the new binary applies any pending schema changes.

## Troubleshooting

**View logs:**

```bash
sudo journalctl -u holzcloud -f          # Follow live logs
sudo journalctl -u holzcloud --since today # Today's logs
```

**Service won't start:**

- Check logs: `sudo journalctl -u holzcloud -n 50`
- Verify binary permissions: `ls -la /opt/holzcloud/holzcloud`
- Verify data directory ownership: `ls -la /opt/holzcloud/data/`
- Test manually: `sudo -u holzcloud /opt/holzcloud/holzcloud`

**Database locked errors:**

- Holzcloud uses WAL mode and a single-writer pool — this should be rare
- Check if another process has the database open: `sudo fuser /opt/holzcloud/data/holzcloud.sqlite`

**Caddy not serving HTTPS:**

- Ensure your domain's DNS A record points to the server's public IP
- Check Caddy logs: `sudo journalctl -u caddy -f`
- Verify port 80 and 443 are open (router/firewall)

**High memory usage:**

- Holzcloud is designed for a modest server and should use well under 100 MB at rest; 1 GB of RAM is enough for a small site
- If memory grows, check for connection leaks in logs


## Recovery

### Locked out of the admin interface

The login throttle is per client address; ten failures from one address block it
for 15 minutes. There is no reset link and `/admin/setup` returns 404 once a user
exists, so recovery goes through the binary:

```bash
sudo -u holzcloud HOLZCLOUD_DATA_DIR=/opt/holzcloud/data \
  /opt/holzcloud/holzcloud user list

echo -n 'a new password' | sudo -u holzcloud HOLZCLOUD_DATA_DIR=/opt/holzcloud/data \
  /opt/holzcloud/holzcloud user passwd -email admin@example.com
```

The password is read from stdin so it never reaches the shell history. If no
admin account exists at all, create one with `holzcloud user create -email … -role admin`.

Restarting the service clears the throttle, since it is held in memory.

### Restore from a backup

```bash
sudo /opt/holzcloud/deploy/restore.sh /opt/holzcloud/backups/20260802-031500
```

The script verifies the snapshot's integrity **before** touching live data,
moves the current data directory aside instead of deleting it, restores
database, CSRF key, media and templates, and waits for `/readyz`. If the wrong
snapshot was picked, the previous state is still in
`/opt/holzcloud/data.before-restore-<timestamp>`.

Restore is only proven if you have run it. Do it once on a test directory
before you need it.

### A failed migration

The service takes a verified snapshot before applying migrations and logs the
exact restore command if they fail:

```
systemctl stop holzcloud && cp /opt/holzcloud/data/pre-upgrade-<version>-<ts>.sqlite \
  /opt/holzcloud/data/holzcloud.sqlite && systemctl start holzcloud
```

### Health checks

| Endpoint | Meaning |
|----------|---------|
| `/healthz` | Liveness — a constant 200 while the process answers. Use for systemd. |
| `/readyz` | Readiness — database reachable, integrity verdict, free disk space, data directory writable. 503 when any of these fails, with a `problems` array naming the cause. |

```bash
curl -s http://127.0.0.1:8080/readyz | jq
```


## Locked out of an administrator account

Two-factor is compulsory for administrators, so a lost phone with no recovery
codes left needs shell access to the machine:

```
sudo -u holzcloud holzcloud user 2fa status
sudo -u holzcloud holzcloud user 2fa disable -email admin@example.de
```

The account can then sign in with its password alone and will be asked to enrol
a new authenticator on the next request. Nothing else about the account changes;
its password is untouched.
