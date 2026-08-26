# Multi-stage build, dev/local-try use only (see docker-compose.yml) — the real deploy path is
# Ansible fetching a prebuilt release binary onto a plain VPS (see docs/deploy.md and
# .github/workflows/release.yml, which builds the same way this stage does). Ubuntu 24.04,
# matching that actual deploy target: libvips-dev (and its transitive deps, notably
# libmagickcore/libmagickwand) installs cleanly from the plain archive on 24.04, no Ubuntu Pro/ESM
# needed — verified directly against a stock `ubuntu:24.04` image, no extra sources required.
# apt's golang-go package (1.22) is older than go.mod's requirement; Go's automatic toolchain
# switching (GOTOOLCHAIN=auto, the default since Go 1.21) downloads the pinned 1.25 toolchain on
# first use, same as it would on any dev machine with an older local Go.
FROM ubuntu:24.04 AS builder

RUN apt-get update && apt-get install -y --no-install-recommends golang-go libvips-dev pkg-config ca-certificates \
    && rm -rf /var/lib/apt/lists/*

# Installed as its own binary (not `go run` against our go.mod) so the CLI's own, much larger
# dependency tree doesn't need to be vendored into this project's go.sum.
RUN go install github.com/a-h/templ/cmd/templ@v0.3.1020
ENV PATH="/root/go/bin:${PATH}"

WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download

COPY . .
RUN templ generate
RUN CGO_ENABLED=1 go build -ldflags="-s -w" -o /out/server ./cmd/server

FROM ubuntu:24.04

# Runtime only needs the shared library, not -dev headers — same package docs/deploy.md's Ansible
# playbook installs directly on the VPS.
RUN apt-get update && apt-get install -y --no-install-recommends libvips42 ca-certificates \
    && rm -rf /var/lib/apt/lists/*

WORKDIR /app
# Migrations and static assets are embedded in the binary (db/migrations/embed.go,
# static/embed.go) — nothing else needs to be copied in.
COPY --from=builder /out/server ./server

ENV PORT=3000
ENV DB_PATH=/data/studio.db
ENV MEDIA_STORAGE_DIR=/data/media-storage
EXPOSE 3000
CMD ["./server"]
