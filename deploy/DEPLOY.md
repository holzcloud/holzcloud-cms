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
- **`/healthz`** for both probes. It answers 200 as soon as the database is
  open.

The image is built for linux/amd64 only. `docker run --rm -v holzcloud:/data -p
8080:8080 ghcr.io/holzcloud/holzcloud-cms:<tag>` is enough to try it.

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
