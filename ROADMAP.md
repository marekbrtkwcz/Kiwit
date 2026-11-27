# Kiwit TCMS — Roadmapa

Lista funkcji i etapów rozwoju systemu Kiwit TCMS.

## Etap 5 — Bezpieczeństwo i domykanie Core
- [x] **Middleware autoryzacji & RBAC:** wymuszanie `Authorization: Bearer <JWT | kiwit_token>` na `/api/v1/*`, egzekwowanie ról (`admin`, `tester`, `viewer`) oraz izolacja dostępu per produkt (`product_members`).
- [x] **Administracja kontami i tokenami:** listowanie/unieważnianie tokenów API, zmiana hasła z weryfikacją obecnego, panel zarządzania użytkownikami, `KIWIT_ALLOW_REGISTRATION`, rate limiting na logowaniu/rejestracji.
- [x] **Przypisywanie `TestCase` do `TestPlan`:** `POST` i `GET /api/v1/testplans/{id}/cases`, hierarchia planów (`parent_id`) + pełne UI w `TestPlansView`.
- [x] **Management (`components`, `tags`, `environments`, `templates`, słowniki):** endpointy OpenAPI, relacje M2M oraz widoki w `ProductsView` i `SettingsView`.
- [x] **Wersjonowanie `TestCase`:** historia rewizji przypadków testowych przy tworzeniu i edycji (zarówno w SQLite, jak i w PostgreSQL).

## Etap 6 — CI/CD, załączniki i eksport
- [x] **Import JUnit XML:** `POST /api/v1/testruns/import/junit` + import z poziomu UI (`TestRunsView`).
- [x] **Eksport raportów:** eksport wyników `TestRun` do CSV i HTML z uwierzytelnionym pobieraniem w UI.
- [x] **Załączniki (`attachments`):** upload, bezpieczne pobieranie (`nosniff`, `CSP: sandbox`, `mime.FormatMediaType`), usuwanie oraz automatyczne sprzątanie sierot po usunięciu `TestCase`/`TestRun`.

## Etap 7 — Integracje, Telemetria i Skala
- [x] **Issue Trackery (`bugs` / `issuetracker`):** konfiguracja GitHub/GitLab/Jira, ochrona przed SSRF, ręczne linkowanie oraz bezpośrednie zgłaszanie bugów z `TestExecution`.
- [x] **Telemetria i raporty:** podsumowanie Pass/Fail, trendy dzienne (`/execution-trends`), macierz statusów (`/status-matrix`) i wykrywanie flaky testów (`/test-case-health`) w `DashboardView`.
- [x] **Adaptery PostgreSQL (`pgx`):** pełna zgodność repozytoriów i migracji dla SQLite i PostgreSQL, paginacja i filtrowanie w SQL (bez problemu N+1).
- [x] **Powiadomienia:** webhooki Slack/Teams oraz SMTP z ustawieniami CC list uruchamianymi przy edycji `TestCase`/`TestPlan`.

## Etap 8 — Kolejne kroki (opcjonalne)
- [x] **Audit log (historia zmian):** tabela `audit_log` (migracja 00026) zapisująca kto/co zmienił dla `test_case`/`test_plan`/`test_run` (create/update/delete/stop), endpointy `GET /api/v1/audit` i `/api/v1/audit/all`, unified diff pól tekstowych oraz e-mail z diffem przy edycji.
- [x] **Bugzilla i Azure Boards:** providery `bugzilla` i `azure` w `issuetracker` (report/status/comment) obok GitHub/GitLab/Jira.
- [x] **Dashboard użytkownika:** `GET /api/v1/dashboard` z widgetami „Moje uruchomienia” i „Ostatnia aktywność” na stronie głównej.
- [x] **Testy kontraktowe PostgreSQL w CI:** dodanie serwisu `postgres:16` w `.github/workflows/ci.yml` i uruchamianie wspólnej suity testów repozytoriów na obu silnikach.
- [x] **Import danych (CSV/JSON):** `POST /api/v1/testcases/import` (multipart: `file` + `product_id`) — CSV z nagłówkiem (kroki jako `akcja => wynik ;; akcja => wynik`) albo tablica obiektów JSON jak w API; zgodny z własnym eksportem JSON (round-trip), walidacja per wiersz, limity 10 MiB / 1000 rekordów, przycisk Import w TestCasesView.
- [x] **Pełne tłumaczenia i18n (PL/EN):** wszystkie etykiety UI w słowniku `frontend/src/i18n/index.tsx` (brak hardkodowanych tekstów).
- [x] **Storage obiektowy (S3 / MinIO):** opcjonalny backend załączników obok lokalnego dysku (`KIWIT_STORAGE_TYPE=s3`).
- [x] **SSO (OIDC / OAuth2):** logowanie przez zewnętrznych dostawców tożsamości (`KIWIT_OIDC_ISSUER` + client ID/secret; state+nonce, JWKS verification).
- [x] **Pełne XML-RPC/JSON-RPC API zgodne z tcms-api:** oba endpointy (`/xml-rpc/`, `/json-rpc/`) z kompletem ~90 metod — Auth, KiwiTCMS, Markdown, wszystkie encje (TestCase/TestPlan/TestRun/TestExecution) z CRUD + tagi + komentarze + załączniki + linki + właściwości, słowniki (Product/Version/Build/Category/Component/Tag/Priority/Status/PlanType/Classification/Template/Environment), Bug/BugTracker, User/Group, Telemetry. Zgodność z oficjalnym klientem `tcms-api` (Python).
- [x] **Zaawansowane field lookups w `.filter()`:** Django-style lookups (`__startswith`, `__contains`, `__icontains`, `__gte`, `__lte`, `__gt`, `__lt`, `__in`, `__isnull`, `__range`, `__endswith`) + relacje przez `__` (np. `product__name__startswith`) na wszystkich metodach filter.
- [x] **InitDB endpoint i reset bazy:** `GET/POST /init-db/` (zgodny z kiwitcms — wykrywa niezainicjalizowaną bazę, tworzy admina z env `KIWIT_ADMIN_*` lub defaultów, seeduje słowniki) oraz `POST /api/v1/admin/reset-db` (admin-only, wipe wszystkich danych + re-seed).
- [x] **Konfigurowalne menu (MENU_ITEMS / HELP_MENU_ITEMS):** `GET /api/v1/menu` zwraca strukturę menu z `KIWIT_MENU_JSON` env (JSON) lub domyślną; frontend `App.tsx` pobiera menu z API zamiast hardkodować — fallback do i18n przy braku konfiguracji.
- [x] **bug.created webhook (EXTERNAL_ISSUE_POST_PROCESSORS):** event `bug.created` emitowany przy `POST /api/v1/bugs` i XML-RPC `Bug.report`, z polem `data` zawierającym `tracker`, `url`, `external_key`, `reporter`, `execution_id` — odpowiednik post-processorów kiwitcms dla Jira/GitHub/Redmine.
- [x] **Self-hosted captcha przy rejestracji:** `GET /api/v1/auth/captcha` zwraca math challenge (TTL 10 min, single-use), rejestracja wymaga `captcha_id` + `captcha_answer` + honeypot (`website` musi być puste) + time-trap (min 2s). Włączone gdy `KIWIT_ALLOW_REGISTRATION=true`, wyłączane przez `KIWIT_CAPTCHA_ENABLED=false`. Bez zewnętrznych dostawców (nie reCAPTCHA).
- [x] **Anonymous read-only access:** `KIWIT_ANONYMOUS_ACCESS=true` — niezalogowane GET-y dostają `AnonymousUser` (rola `anonymous`) w kontekście; `CheckWriteRole` blokuje zapisy; `AccessGuard` traktuje anonymous jak viewer (tylko publiczne produkty). Odpowiednik kiwitcms `AnonymousViewBackend`.
- [x] **Refresh permissions:** `POST /api/v1/admin/refresh-permissions` (admin-only) — tworzy domyślne grupy `Administrator` + `Tester`, przypisuje Tester rolę `tester` na wszystkich produktach. Idempotentne. Odpowiednik `manage.py refresh_permissions`.
- [x] **Set domain (runtime):** tabela `instance_settings` + `GET /api/v1/settings/domain` (public) i `GET/PUT /api/v1/admin/domain` (admin-only). PUT normalizuje domain (dodaje `https://`, usuwa trailing slash), zapisuje w DB i aktualizuje `auth.Service.baseURL` na żywo — linki e-mail od razu używają nowego adresu. Przy starcie domain z DB nadpisuje `KIWIT_BASE_URL`. Odpowiednik `manage.py set_domain`.
- [x] **Wszystkie ustawienia instance w runtime przez admin API:** `GET /api/v1/settings` (publiczne), `GET /api/v1/admin/settings` (wszystkie, hasło maskowane), `PUT /api/v1/admin/settings/{key}`, `POST /api/v1/admin/settings` (bulk). Klucze: `domain`, `smtp_host/port/user/pass/from`, `allow_registration`, `captcha_enabled`, `anonymous_access`. Każda zmiana strzela callback `OnChange` który na żywo aktualizuje `auth.Service` (baseURL, rejestracja, captcha, anonymous) i `EmailClient.SetConfig` (SMTP). Przy starcie wartości z DB nadpisują env.
