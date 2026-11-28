# Kiwit TCMS

Nowoczesny system zarządzania testami (TCMS) w **Go + React/TypeScript**, inspirowany Kiwi TCMS. Dostarczany jako pojedyncza binarka z wbudowanym interfejsem webowym (`embed.FS`) oraz pełnym wsparciem dla **SQLite** (bez CGO) i **PostgreSQL**.

## Funkcjonalności

* **Autoryzacja, RBAC i bezpieczeństwo:** JWT, tokeny API CI/CD (`kiwit_*`) z listowaniem i unieważnianiem, role (`admin`, `tester`, `viewer`), grupy użytkowników (`user_groups`) z per-product uprawnieniami (`group_product_permissions`), izolacja dostępu per produkt (`product_members`), panel administracji użytkownikami, rate limiting na logowaniu/rejestracji, walidacja siły hasła (min 8 znaków, lista popularnych haseł, zakaz zawierania e-maila/nazwy użytkownika), blokada SSRF dla Issue Trackerów, strukturalne logowanie HTTP (`slog`).
* **Management:** Produkty, Wersje, Buildy, Komponenty, Tagi (M2M), Kategorie, Priorytety, Statusy wykonań, Środowiska testowe (`environments`), Szablony (`templates`), słownik typów planów (`test_plan_types`), klasyfikacje produktów (`classifications`), właściwości encji (`entity_properties` key=value), lista CC per `TestRun` i linki zewnętrzne (`link_references`) per run/wykonanie.
* **Scenariusze testowe (`TestCases`):** kroki testowe i **Gherkin**, pola automatyzacji (`is_automated`, `script`, `arguments`), historia rewizji (SQLite i PostgreSQL), klonowanie, edycja zbiorcza (`bulk`), filtrowanie i paginacja w SQL, załączniki i komentarze.
* **Plany testowe (`TestPlans`):** hierarchia planów (`parent_id`), podpinanie i podgląd przypisanych scenariuszy, klonowanie, tagi.
* **Uruchomienia (`TestRuns` & `TestExecutions`):** snapshot kroków przy starcie uruchomienia, import raportów **JUnit XML**, eksport **CSV** i **HTML**, podpinanie i bezpośrednie zgłaszanie bugów (**GitHub / GitLab / Jira**), załączniki i komentarze.
* **Telemetria i powiadomienia:** podsumowanie Pass/Fail, trendy dzienne, macierz statusów, wykrywanie flaky/failing testów, powiadomienia SMTP (z listą CC per encja) i webhooki (Slack / Teams).
* **Wyszukiwarka:** `GET /api/v1/search?q=...` — pełnotekstowe przeszukiwanie scenariuszy (także kroków), planów i uruchomień (SQLite LIKE, PostgreSQL `to_tsvector`).
* **OpenAPI:** automatyczna dokumentacja API generowana przez `huma/v2` (`/docs`, `/openapi.json`).
* **XML-RPC:** kompatybilny z Kiwi TCMS endpoint `POST /xml-rpc/` z sesją cookie — pełny zestaw metod: `Auth.login/logout`, `KiwiTCMS.version`, `Markdown.render`, filter dla wszystkich encji (TestCase/TestPlan/TestRun/Build/Product/Version/Category/Component/Tag/Priority/Status/User/Group/Environment), CRUD + tagi + komentarze + historia dla TestCase, CRUD + add/remove/get_case + sortkeys dla TestPlan, CRUD + case + CC dla TestRun, `Group.users`, `User.join_group`, `Telemetry.breakdown/status_matrix/execution_trends/test_case_health`, linki (`TestExecution/TestRun.add_link/get_links/remove_link`), `TestExecution.*` (update, komentarze, historia, tagi, właściwości), `TestPlan.tree/add_tag/remove_tag`, załączniki base64 (`TestCase/TestRun/TestExecution.add_attachment/list_attachments`, `Attachment.remove`), `Bug.report/filter`, `Search.query`, `User.update/deactivate`, create dla słowników.
* **Markdown:** `POST /api/v1/markdown/render` — render Markdown do HTML (goldmark, raw HTML escapowany).

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
| `KIWIT_UPLOAD_DIR` | `./uploads` | Katalog przechowywania załączników (gdy `KIWIT_STORAGE_TYPE=local`). |
| `KIWIT_STORAGE_TYPE` | `local` | Backend załączników: `local` lub `s3`. |
| `KIWIT_S3_ENDPOINT` | *(AWS)* | Endpoint S3 — ustaw np. `http://localhost:9000` dla MinIO. |
| `KIWIT_S3_REGION` | `us-east-1` | Region S3. |
| `KIWIT_S3_BUCKET` | *(wymagany dla s3)* | Nazwa bucketa. |
| `KIWIT_S3_ACCESS_KEY` / `KIWIT_S3_SECRET_KEY` | *(wymagane dla s3)* | Dane uwierzytelniające S3. |
| `KIWIT_S3_USE_PATH_STYLE` | `true` | Styl adresowania path-style (wymagany przez MinIO). |
| `KIWIT_WEBHOOK_URL` | *(brak)* | URL webhooka (Slack / Teams) powiadamianego przy zatrzymaniu `TestRun`. |
| `KIWIT_SMTP_HOST` / `_PORT` / `_USER` / `_PASS` / `_FROM` | *(brak)* | Konfiguracja serwera SMTP dla powiadomień e-mail. |
| `KIWIT_TRACKER_TOKEN` | *(brak)* | Domyślny token API dla integracji z GitHub/GitLab/Jira (opcjonalny fallback). |
| `KIWIT_OIDC_ISSUER` | *(brak)* | Issuer URL dostawcy OIDC (np. `https://accounts.google.com`). Puste = SSO wyłączone. |
| `KIWIT_OIDC_CLIENT_ID` / `KIWIT_OIDC_CLIENT_SECRET` | *(wymagane dla SSO)* | Dane klienta OAuth2 u dostawcy. |
| `KIWIT_OIDC_REDIRECT_URL` | `{KIWIT_BASE_URL}/api/v1/auth/sso/callback` | Redirect URI zarejestrowany u dostawcy. |
| `KIWIT_OIDC_PROVIDER_NAME` | `SSO` | Nazwa wyświetlana na przycisku logowania. |
| `KIWIT_LDAP_SERVER` | *(brak)* | `host:port` serwera LDAP. Puste = LDAP wyłączone. |
| `KIWIT_LDAP_BIND_DN` / `KIWIT_LDAP_BIND_PASSWORD` | *(wymagane)* | Konto serwisowe do wyszukiwania użytkowników. |
| `KIWIT_LDAP_BASE_DN` | *(wymagane)* | Base DN, np. `dc=example,dc=com`. |
| `KIWIT_LDAP_USER_FILTER` | `(uid=%s)` | Filtr wyszukiwania użytkownika (`%s` = username). |
| `KIWIT_LDAP_USERNAME_ATTR` / `KIWIT_LDAP_EMAIL_ATTR` | `uid` / `mail` | Atrybuty LDAP z nazwą użytkownika i e-mailem. |
| `KIWIT_LDAP_STARTTLS` | `false` | Gdy `true`, połączenie `ldap://` jest podnoszone do TLS przez StartTLS (certyfikat weryfikowany). |
| `KIWIT_LDAP_INSECURE_SKIP_VERIFY` | `false` | Gdy `true`, wyłącza weryfikację certyfikatu TLS LDAP/ldaps (tylko dla wewnętrznego CA). `KIWIT_LDAP_SERVER` może też zawierać pełny URL `ldaps://host:port`. |
| `KIWIT_BASE_URL` | `http://localhost:8080` | Publiczny adres instancji (linki e-mail, redirect SSO). |
| `KIWIT_ANONYMOUS_ACCESS` | `false` | Gdy `true`, niezalogowani użytkownicy mają dostęp read-only (rola `anonymous`). |
| `KIWIT_CAPTCHA_ENABLED` | `false` | Gdy `true`, rejestracja wymaga captchy (`GET /api/v1/auth/captcha`). |
| `KIWIT_MENU_JSON` | *(brak)* | Opcjonalny JSON nadpisujący strukturę menu (`GET /api/v1/menu`). |
| `KIWIT_TRUSTED_PROXIES` | *(puste)* | Lista CIDR proxy (oddzielona przecinkami), których nagłówek `X-Forwarded-For` jest zaufany przy rate-limitingu. Loopback/link-local są zawsze zaufane. |
| `KIWIT_LOG_LEVEL` | `info` | Poziom logów (`debug` / `info` / `warn` / `error`). |
| `KIWIT_LOG_FORMAT` | `json` | Format logów: `json` lub `text`. |
| `KIWIT_LOG_FILE` | *(brak)* | Plik logów (pusty = sam stdout). |
| `KIWIT_LOG_MAX_SIZE` / `KIWIT_LOG_MAX_FILES` | `10` / `3` | Rotacja pliku logów (MB / liczba plików). |
| `KIWIT_LLM_API_KEY` | *(brak)* | Klucz API dostawcy LLM dla funkcji AI (puste = funkcje AI wyłączone). |

## Testy i budowanie

```bash
# Backend
cd backend
golangci-lint run ./...   # jak w CI (konfig w .golangci.yml); lokalnie: go install github.com/golangci/golangci-lint/v2/cmd/golangci-lint@v2.15.0
go vet ./...
go test -count=1 ./...

# Frontend
cd frontend
npm run check:ci          # biome ci + tsc --noEmit (jak w CI); lokalnie z poprawkami: npm run check
npm run check:i18n
npm test
npx vite build --outDir ../backend/internal/web/dist --emptyOutDir

# Kompilacja pojedynczej binarki Windows
cd ../backend
CGO_ENABLED=0 GOOS=windows GOARCH=amd64 go build -buildvcs=false -ldflags="-s -w" -o ../dist/kiwit.exe ./cmd/server
```

Budowanie obrazu kontenerowego:
```bash
docker build -t kiwit:latest .
```
