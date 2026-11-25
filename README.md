# Kiwit

Nowoczesny system zarządzania testami (TCMS) w Go + React/TypeScript, inspirowany Kiwi TCMS.

## Wymagania
- Go 1.23+
- Node.js 20+
- Podman / Docker (dla lokalnego PostgreSQL)

## Szybki start
1. Uruchom bazę danych:
   ```bash
   podman compose up -d
   ```
2. Uruchom backend:
   ```bash
   cd backend
   go run ./cmd/server
   ```
3. Sprawdź healthcheck: `http://localhost:8080/healthz`
