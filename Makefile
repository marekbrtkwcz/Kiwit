.PHONY: dev db-up db-down test sync-docs build-linux build-windows

db-up:
	podman compose up -d

db-down:
	podman compose down

dev:
	cd backend && go run ./cmd/server

test:
	cd backend && go test ./...

# Refresh the embedded copy of docs/ served at /docs/<name>.md.
sync-docs:
	mkdir -p backend/internal/docs/content
	cp docs/*.md backend/internal/docs/content/

build-linux: sync-docs
	cd backend && CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -ldflags="-s -w" -o ../dist/kiwit-linux-amd64 ./cmd/server

build-windows: sync-docs
	cd backend && CGO_ENABLED=0 GOOS=windows GOARCH=amd64 go build -ldflags="-s -w" -o ../dist/kiwit.exe ./cmd/server
