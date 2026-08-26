.PHONY: generate build release run test dev docker-up docker-down install-hooks

CONTAINER_ENGINE ?= docker

generate:
	templ generate

# Local dev build — requires libvips-dev installed on this machine (`apt install libvips-dev`;
# see README). Not what the release binary Ansible deploys is built with.
build: generate
	go build -o bin/server ./cmd/server

# Builds dist/server the same way .github/workflows/release.yml does — inside an Ubuntu 24.04
# container (matching the actual deploy target, see docs/deploy.md), so it doesn't matter whether
# you have libvips-dev installed locally. Useful for a local smoke test of a release build;
# dist/server itself isn't committed (see .gitignore) — CI attaches it to a GitHub Release on
# every `vX.Y.Z` tag, and that's what Ansible actually deploys.
release:
	$(CONTAINER_ENGINE) build --target builder -t studio-builder:latest .
	$(CONTAINER_ENGINE) create --name studio-builder-extract studio-builder:latest
	$(CONTAINER_ENGINE) cp studio-builder-extract:/out/server ./dist/server
	$(CONTAINER_ENGINE) rm studio-builder-extract
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

# One-time, per-clone opt-in — git doesn't use .githooks/ on its own. Points this clone's hooks at
# the tracked .githooks/ dir (see .githooks/pre-commit) instead of the untracked .git/hooks/.
install-hooks:
	git config core.hooksPath .githooks
	chmod +x .githooks/*
