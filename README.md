# Kiwit TCMS

Nowoczesny system zarządzania testami (TCMS) w **Go + React/TypeScript**, inspirowany Kiwi TCMS. Dostarczany jako pojedyncza binarka z wbudowanym interfejsem webowym (`embed.FS`) oraz pełnym wsparciem dla **SQLite** (bez CGO) i **PostgreSQL**.

## Funkcjonalności

* **Autoryzacja, RBAC i bezpieczeństwo:** JWT, tokeny API CI/CD (`kiwit_*`) z listowaniem i unieważnianiem, role (`admin`, `tester`, `viewer`), izolacja dostępu per produkt (`product_members`), panel administracji użytkownikami, rate limiting na logowaniu/rejestracji, blokada SSRF dla Issue Trackerów, strukturalne logowanie HTTP (`slog`).
* **Management:** Produkty, Wersje, Buildy, Komponenty, Tagi (M2M), Kategorie, Priorytety, Statusy wykonań, Środowiska testowe (`environments`) i Szablony (`templates`).
* **Scenariusze testowe (`TestCases`):** kroki testowe i **Gherkin**, pola automatyzacji (`is_automated`, `script`, `arguments`), historia rewizji (SQLite i PostgreSQL), klonowanie, edycja zbiorcza (`bulk`), filtrowanie i paginacja w SQL, załączniki i komentarze.
* **Plany testowe (`TestPlans`):** hierarchia planów (`parent_id`), podpinanie i podgląd przypisanych scenariuszy, klonowanie, tagi.
* **Uruchomienia (`TestRuns` & `TestExecutions`):** snapshot kroków przy starcie uruchomienia, import raportów **JUnit XML**, eksport **CSV** i **HTML**, podpinanie i bezpośrednie zgłaszanie bugów (**GitHub / GitLab / Jira**), załączniki i komentarze.
* **Telemetria i powiadomienia:** podsumowanie Pass/Fail, trendy dzienne, macierz statusów, wykrywanie flaky/failing testów, powiadomienia SMTP (z listą CC per encja) i webhooki (Slack / Teams).
* **OpenAPI:** automatyczna dokumentacja API generowana przez `huma/v2` (`/docs`, `/openapi.json`).

## Wymagania

* **Uruchomienie binarki:** brak zewnętrznych zależności (domyślnie SQLite).
* **Development / budowanie:**
  * Go 1.26+
  * Node.js 20+
  * Docker / Podman (opcjonalnie, dla lokalnego PostgreSQL)

## Szybki start

### 1. Uruchomienie gotowej binarki (Windows / SQLite)
```powershell
.\dist\kiwit.exe
```
Aplikacja wystartuje pod `http://localhost:8080` (pierwszy zarejestrowany użytkownik automatycznie otrzymuje rolę `admin`).

### 2. Tryb developerski (SQLite lub PostgreSQL)
Opcjonalnie uruchom PostgreSQL:
```bash
docker compose up -d
```
Uruchom backend:
```bash
cd backend
go run ./cmd/server
```
Uruchom frontend (dev server z proxy na `:8080`):
```bash
cd frontend
npm ci
npm run dev
```

## Konfiguracja (zmienne środowiskowe)

| Zmienna | Domyślnie | Opis |
| :--- | :--- | :--- |
| `KIWIT_ADDR` | `:8080` | Adres nasłuchiwania serwera (bez tej zmiennej fallback na wolny port `8080–8180`). |
| `KIWIT_ENV` | `development` | Środowisko (`development` / `production`). W `production` wymagany jest silny `KIWIT_JWT_SECRET`. |
| `KIWIT_DATABASE_URL` | `sqlite://./kiwit.db` | DSN bazy danych: `sqlite://...` lub `postgres://user:pass@host:5432/kiwit`. |
| `KIWIT_JWT_SECRET` | *(dev only)* | Sekret podpisu JWT (w `production` min. 16 znaków, inaczej serwer odmawia startu). |
| `KIWIT_ALLOW_REGISTRATION` | `true` | Gdy `false`, blokuje publiczną rejestrację po utworzeniu pierwszego administratora. |
| `KIWIT_UPLOAD_DIR` | `./uploads` | Katalog przechowywania załączników. |
| `KIWIT_WEBHOOK_URL` | *(brak)* | URL webhooka (Slack / Teams) powiadamianego przy zatrzymaniu `TestRun`. |
| `KIWIT_SMTP_HOST` / `_PORT` / `_USER` / `_PASS` / `_FROM` | *(brak)* | Konfiguracja serwera SMTP dla powiadomień e-mail. |
| `KIWIT_TRACKER_TOKEN` | *(brak)* | Domyślny token API dla integracji z GitHub/GitLab/Jira (opcjonalny fallback). |

## Testy i budowanie

```bash
# Backend
cd backend
go vet ./...
go test -count=1 ./...

# Frontend
cd frontend
npm test
npx tsc --noEmit
npx vite build --outDir ../backend/internal/web/dist --emptyOutDir

# Kompilacja pojedynczej binarki Windows
cd ../backend
CGO_ENABLED=0 GOOS=windows GOARCH=amd64 go build -buildvcs=false -ldflags="-s -w" -o ../dist/kiwit.exe ./cmd/server
```

Budowanie obrazu kontenerowego:
```bash
docker build -t kiwit:latest .
```
