.PHONY: dev db-up db-down test build-linux build-windows

db-up:
	podman compose up -d

db-down:
	podman compose down

dev:
	cd backend && go run ./cmd/server

test:
	cd backend && go test ./...

build-linux:
	cd backend && CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -ldflags="-s -w" -o ../dist/kiwit-linux-amd64 ./cmd/server

build-windows:
	cd backend && CGO_ENABLED=0 GOOS=windows GOARCH=amd64 go build -ldflags="-s -w" -o ../dist/kiwit.exe ./cmd/server
