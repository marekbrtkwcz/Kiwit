# Unified data export

Kiwit exposes a unified export pipeline: every entity maps its rows to a
shared table model and one renderer serializes it to CSV, JSON or Markdown.
A JUnit XML renderer is available for test run executions.

## API endpoints

All export endpoints require a bearer token and reuse the same filters as the
corresponding list endpoints. Anonymous users get the same visibility as in the
list APIs. `format` accepts `csv` (default), `json`, `md` (alias `markdown`).

| Endpoint | Scope |
|---|---|
| `GET /api/v1/testcases/export` | filtered case list; `md` renders full documents |
| `GET /api/v1/testcases/{id}/export` | single case; `md` renders a full document |
| `POST /api/v1/testcases/export` | body `{ids, format}` — selected cases |
| `GET /api/v1/testruns/export` | filtered run list |
| `POST /api/v1/testruns/export` | body `{ids, format}` — selected runs |
| `GET /api/v1/testruns/{id}/export` | one run's executions; extra formats `html`, `junit` |
| `GET /api/v1/testplans/export` | filtered plan list |
| `GET /api/v1/requirements/export` | filtered requirement list |
| `GET /api/v1/milestones/export` | milestones, optional `version_id` filter |
| `POST /api/v1/milestones/export` | body `{ids, format}` — selected milestones |
| `GET /api/v1/bugs/export` | filtered bug list |
| `GET /api/v1/telemetry/export?dataset=…` | `summary`, `trends`, `matrix`, `health`, `breakdown`, `ux`, `dashboard`; same filters as the telemetry API |
| `GET /api/v1/audit/export?limit=N` | admin only; recent audit entries |

## Response conventions

- **CSV**: `text/csv; charset=utf-8` with a UTF-8 BOM (opens correctly in
  Excel), `Content-Disposition: attachment`.
- **JSON**: array of objects keyed by column name.
- **Markdown**: a pipe table, except test cases which render as full documents
  (`# summary`, metadata, `## Preconditions`, numbered steps with expected
  results, fenced ```gherkin``` block or verbatim markdown body, `## Notes`).
  Multiple cases in one file are separated by `---`.
- **JUnit XML**: run executions mapped as `passed`/`failed`/`error`
  (`blocked`)/`skipped` (`idle`), suitable for CI/CD result visualization.

## Frontend

The `ExportButton` component (`frontend/src/components/ExportButton.tsx`)
renders a download menu (CSV / JSON / Markdown) wherever lists are shown:
TestCases, TestRuns, TestPlans, Requirements, Milestones, Bugs and the
Telemetry dashboard (with a dataset selector). When rows are selected, the
button switches to the selection export (POST) for test cases and milestones.
A single test case can be downloaded as Markdown from the case drawer
(FileDown icon in the drawer header).
