# Kiwit TCMS — Roadmapa (na później)

Lista funkcji z Kiwi TCMS oraz rozszerzeń zaplanowanych do wdrożenia w kolejnych etapach:

## Etap 5 — Bezpieczeństwo i domykanie Core
- [ ] **Middleware autoryzacji & RBAC**: wymuszanie `Authorization: Bearer <JWT | kiwit_token>` na `/api/v1/*` oraz sprawdzanie ról (`admin`, `tester`).
- [ ] **Przypisywanie `TestCase` do `TestPlan`**: endpoint `POST /api/v1/testplans/{id}/cases` + UI w `TestPlansView`.
- [ ] **Management (`components` & `tags`)**: endpointy OpenAPI i widok zarządzania komponentami oraz tagami.
- [ ] **Wersjonowanie `TestCase`**: historia rewizji przypadków testowych przy edycji.

## Etap 6 — CI/CD i załączniki
- [ ] **Import JUnit XML**: `POST /api/v1/testruns/import/junit` parsujący raporty z pytest / Go / JUnit i tworzący `TestRun` + `TestExecution`.
- [ ] **Załączniki (`kiwi_attachments`)**: upload plików (zrzuty ekranu, logi) do `TestCase` i `TestExecution` (abstrakcja: lokalny dysk / S3).

## Etap 7 — Integracje, Telemetria i Skala
- [ ] **Issue Trackery (`bugs` / `issuetracker`)**: integracja z GitHub/GitLab Issues oraz Jira (tworzenie i podpinanie bugów z `TestExecution`).
- [ ] **Telemetria i raporty**: wykresy Pass/Fail w czasie, pokrycie planów i eksport raportów.
- [ ] **Adaptery PostgreSQL (`sqlc` / `pgx`)**: pełna implementacja repozytoriów dla PostgreSQL (obok obecnego SQLite).
