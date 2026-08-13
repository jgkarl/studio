.PHONY: generate build release run test dev docker-up docker-down

CONTAINER_ENGINE ?= docker

generate:
	templ generate

# Local dev build — requires libvips-dev installed on this machine (`apt install libvips-dev` on
# Debian; see README). Not what dist/server (committed, deployed to the VPS) is built with.
build: generate
	go build -o bin/server ./cmd/server

# Rebuilds dist/server — the binary committed to the repo and deployed by `git pull` on the VPS
# (see docs/deploy.md). Built inside the same Debian image the Dockerfile's builder stage uses,
# not on this machine directly, so it doesn't matter whether you have libvips-dev installed
# locally: the container has it, and the result is portable to any Debian 12 host with the
# matching libvips42 runtime package installed (`apt install libvips42` — no build tools needed
# there). Run this after any code change before committing dist/server.
release:
	$(CONTAINER_ENGINE) build --target builder -t studio-go-builder:latest .
	$(CONTAINER_ENGINE) create --name studio-go-builder-extract studio-go-builder:latest
	$(CONTAINER_ENGINE) cp studio-go-builder-extract:/out/server ./dist/server
	$(CONTAINER_ENGINE) rm studio-go-builder-extract
	chmod +x dist/server

run: generate
	go run ./cmd/server

test: generate
	go test ./...

# Regenerates templ on every .templ save and restarts the server. Requires the templ CLI
# (`go install github.com/a-h/templ/cmd/templ@latest`) on PATH.
dev:
	templ generate --watch --proxy="http://localhost:3000" --cmd="go run ./cmd/server"

docker-up:
	docker compose up --build

docker-down:
	docker compose down
