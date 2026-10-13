.PHONY: dev build run migrate-up docker-up docker-down test web-build web-dev

dev:
	go run ./cmd/aegisops

web-build:
	cd web && npm run build

web-dev:
	cd web && npm run dev

build: web-build
	go build -o bin/aegisops.exe ./cmd/aegisops
	go build -o bin/aegisops-mcp.exe ./cmd/aegisops-mcp

run: build
	./bin/aegisops.exe

docker-up:
	docker compose up -d postgres

docker-down:
	docker compose down

migrate-up:
	goose -dir migrations postgres "$(DATABASE_URL)" up

test: web-build
	go test ./...