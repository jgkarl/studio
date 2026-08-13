.PHONY: generate build run test dev docker-up docker-down

generate:
	templ generate

build: generate
	go build -o bin/server ./cmd/server

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
