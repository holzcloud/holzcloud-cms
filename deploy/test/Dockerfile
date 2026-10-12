# Test recipe: the shipped deploy/holzcloud.service under a real systemd.
#
# The image is a Debian with systemd as PID 1, the holzcloud binary built from
# this tree, the unit copied in UNCHANGED, and the five shipped plugin archives.
# deploy/test/run.sh builds it, starts a privileged container from it and drives
# the checks. Phases 21 and 22 reuse the same image with other steps, so the
# stages are separable: `--target systemd-base` gives a bare systemd Debian.
#
# Build from the repository root (the context is "."):
#   docker build --platform linux/amd64 -f deploy/test/Dockerfile -t IMAGE .
# The root .dockerignore excludes deploy/ and *.md, which this build needs;
# deploy/test/Dockerfile.dockerignore takes precedence over it for this file.

ARG GO_VERSION=1.26.8
ARG BASE_IMAGE=debian:bookworm-slim

# Stage build: cross-compiled on the native builder, so an amd64 image on an
# arm64 host does not compile under emulation.
FROM --platform=$BUILDPLATFORM golang:${GO_VERSION} AS build

# Filled in by BuildKit from --platform. They must NOT carry a default in the ARG
# line: a default here wins over --platform, and `--platform linux/arm64` then
# produced an amd64 binary (seen as "rosetta error" in the native arm64 run).
# The :- fallbacks below only matter for a builder that sets neither.
ARG TARGETOS
ARG TARGETARCH
ARG VERSION=dev
ARG COMMIT=unknown

WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .

RUN CGO_ENABLED=0 GOOS=${TARGETOS:-linux} GOARCH=${TARGETARCH:-amd64} go build -trimpath \
    -ldflags="-s -w -X main.Version=${VERSION} -X main.Commit=${COMMIT}" \
    -o /holzcloud ./cmd/holzcloud

# Only the archives travel on (about 5 MB), not the 24 MB plugin tree.
RUN mkdir /plugin-zips && cp plugins/*/*.zip /plugin-zips/

# Stage canary: the enforcement probe (deploy/test/canary, stdlib only, its own
# module). A static binary per architecture; run.sh starts it under the unit's
# sandbox properties to learn whether this platform enforces seccomp at all.
FROM --platform=$BUILDPLATFORM golang:${GO_VERSION} AS canary

# Filled in by BuildKit from --platform, without defaults (see stage build).
ARG TARGETOS
ARG TARGETARCH

WORKDIR /canary
COPY deploy/test/canary/ ./
RUN CGO_ENABLED=0 GOOS=${TARGETOS:-linux} GOARCH=${TARGETARCH:-amd64} go build -trimpath \
    -ldflags="-s -w" -o /hc-canary .

# Stage systemd-base: Debian with systemd as PID 1 and the tools the checks use.
# sudo is there so the commands in deploy/DEPLOY.md can be pasted unchanged.
FROM ${BASE_IMAGE} AS systemd-base

ENV container=docker DEBIAN_FRONTEND=noninteractive

RUN apt-get update \
 && apt-get install -y --no-install-recommends \
      systemd systemd-sysv dbus curl ca-certificates jq procps sqlite3 sudo \
 && rm -rf /var/lib/apt/lists/* \
 # Fails in a container and turns the whole system "degraded" on trixie.
 && systemctl mask systemd-modules-load.service

STOPSIGNAL SIGRTMIN+3
CMD ["/lib/systemd/systemd"]

# Stage unit-test: must stay the last stage, so it is the default build target.
FROM systemd-base AS unit-test

# The line from deploy/DEPLOY.md, verbatim.
RUN useradd -r -s /usr/sbin/nologin holzcloud \
 && mkdir -p /opt/holzcloud/data \
 && chown holzcloud:holzcloud /opt/holzcloud/data

COPY --from=build --chmod=755 /holzcloud /opt/holzcloud/holzcloud
COPY deploy/holzcloud.service /etc/systemd/system/holzcloud.service
COPY --from=build /plugin-zips /opt/holzcloud/plugin-zips

# Under /usr/local/bin, not /tmp: PrivateTmp=true hides /tmp from the transient
# unit that runs the probe (status 203/EXEC).
COPY --from=canary --chmod=755 /hc-canary /usr/local/bin/hc-canary

# The unit is deliberately NOT enabled here: run.sh starts it, so the first
# journal lines are captured. The unit sets no HOLZCLOUD_LISTEN, so the service
# binds 127.0.0.1 inside the container and no port is ever published; every
# request goes through `docker exec ... curl http://127.0.0.1:8080/...`.
