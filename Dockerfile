# An image for the cluster. Two stages, and the second one is nearly empty.
#
# The program is a single binary without CGO: templates, assets, migrations and
# fonts sit inside it through embed.FS. So the runtime stage needs no
# distribution, no libraries and no package manager — only the root
# certificates for sending mail and a user that is not root. distroless/static
# brings both.
#
# It is built for linux/amd64, like the rest of the project since 1.4.

FROM golang:1.26 AS build

WORKDIR /src

# The module list first, then the source: that way the layer with the
# dependencies stays in the cache as long as go.mod and go.sum do not change.
COPY go.mod go.sum ./
RUN go mod download

COPY . .

# The same particulars the release workflow writes into the binary. Without
# them `holzcloud version` reports "dev", and the admin shows a version in its
# footer that nobody can tie to a release — which is exactly the statement
# AGPL §13 asks for.
ARG VERSION=dev
ARG COMMIT=unknown

RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -trimpath \
    -ldflags="-s -w -X main.Version=${VERSION} -X main.Commit=${COMMIT}" \
    -o /holzcloud ./cmd/holzcloud

FROM gcr.io/distroless/static-debian12:nonroot

COPY --from=build /holzcloud /holzcloud

# The data directory is mounted at run time and deliberately not declared as a
# VOLUME here: a VOLUME with no mount point silently creates a nameless volume
# on every start, and the SQLite file inside it would be gone at the next start
# without anything anywhere having failed.
#
# HOLZCLOUD_LISTEN=0.0.0.0, because since 1.10 the service binds only 127.0.0.1
# by default. That is right for the setup with Caddy on the same machine and
# wrong in a container without exception: the container is its own network
# space, and a published port, a Kubernetes service and the kubelet's probes
# all arrive over the container's address — on the loopback they would find
# nobody. The container started, reported nothing and answered no request. Held
# by cmd/holzcloud/dockerfile_test.go.
ENV HOLZCLOUD_DATA_DIR=/data \
    HOLZCLOUD_PORT=8080 \
    HOLZCLOUD_LISTEN=0.0.0.0

EXPOSE 8080

# 65532, the user from distroless. The mounted directory has to belong to them
# — in the cluster through fsGroup.
USER nonroot:nonroot

ENTRYPOINT ["/holzcloud"]
CMD ["serve"]
