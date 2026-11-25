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
- [ ] **Testy kontraktowe PostgreSQL w CI:** dodanie serwisu `postgres:16` w `.github/workflows/ci.yml` i uruchamianie wspólnej suity testów repozytoriów na obu silnikach.
- [ ] **Pełne tłumaczenia i18n (PL/EN):** przeniesienie wszystkich nowo dodanych etykiet UI do słownika `frontend/src/i18n/index.tsx`.
- [ ] **Storage obiektowy (S3 / MinIO):** opcjonalny backend załączników obok lokalnego dysku.
- [ ] **SSO (OIDC / OAuth2):** logowanie przez zewnętrznych dostawców tożsamości (GitHub, GitLab, Keycloak, Google).
