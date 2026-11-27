# Unified data export

Kiwit exposes a unified export pipeline: every entity maps its rows to a
shared table model and one renderer serializes it to CSV, JSON, Markdown or
XLSX. Test run executions can also be exported as JUnit XML, and Gherkin
test cases as `.feature` files.

## API endpoints

All export endpoints require a bearer token and reuse the same filters as the
corresponding list endpoints. Anonymous users get the same visibility as in the
list APIs. `format` accepts `csv` (default), `json`, `md` (alias `markdown`),
`xlsx`; test case routes additionally accept `feature`.

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
- **XLSX**: single-sheet Excel workbook with a bold header row
  (`application/vnd.openxmlformats-officedocument.spreadsheetml.sheet`).
- **Gherkin `.feature`**: the Gherkin bodies of `format=gherkin` cases in one
  `.feature` file (several Feature blocks are valid for cucumber-style tools);
  non-Gherkin cases are skipped and a selection without any returns 404.

## Frontend

The `ExportButton` component (`frontend/src/components/ExportButton.tsx`)
renders a download menu (CSV / JSON / Markdown / XLSX; test cases also offer
Gherkin `.feature`) wherever lists are shown:
TestCases, TestRuns, TestPlans, Requirements, Milestones, Bugs and the
Telemetry dashboard (with a dataset selector). When rows are selected, the
button switches to the selection export (POST) for test cases and milestones.
A single test case can be downloaded as Markdown from the case drawer
(FileDown icon in the drawer header).

## Import (test cases)

`POST /api/v1/testcases/import` — multipart form data:

- `file` — the import file (`.csv` or `.json`; the format is detected from
  the file extension or can be forced with a `format` form field),
- `product_id` — the target product (write access required, role `tester`).

Response: `{"imported": n, "failed": m, "errors": [{"row": i, "error": "..."}]}`
— valid rows are created even when other rows fail, so fix the listed rows
and re-upload them. Limits: 10 MiB and 1000 records per file. Created cases
are audit-logged as `create` with an `imported` flag.

### JSON format

An array of case objects using the same keys as the `POST /api/v1/testcases`
body:

```json
[
  {
    "summary": "User can log in",
    "preconditions": "DB seeded",
    "format": "steps",
    "priority": "high",
    "status": "confirmed",
    "is_automated": true,
    "script": "tests/login.spec.ts",
    "arguments": "--headed",
    "steps": [
      {"action": "Open /login", "expected_result": "Login form visible"},
      {"action": "Submit credentials", "expected": "Dashboard loads"}
    ]
  }
]
```

`expected_result` and `expected` are both accepted for step results. The
flat JSON produced by the export pipeline is also accepted: unknown keys
(including the `steps` count) are ignored, so an exported file can be
re-imported after adding the missing detail fields.

### CSV format

A header row is required; column names are case-insensitive and unknown
columns are ignored. Supported columns: `summary` (required),
`preconditions`, `requirement`, `extra_link`, `notes`, `format`,
`gherkin_body`, `priority`, `status`, `is_automated`, `script`,
`arguments`, `steps`. The `steps` cell separates steps with ` ;; ` and the
expected result with ` => ` (the result part is optional):

```csv
summary,format,steps
Login works,steps,"Open /login => form visible ;; Submit credentials => dashboard"
Gherkin case,gherkin,
```

(Gherkin cases carry their body in the `gherkin_body` column.)

Defaults: `format=steps`, `priority=medium`, `status=confirmed`,
`is_automated=false`. Invalid enum values, empty summaries and empty step
actions reject the whole file with a 400 and a `record N` position hint.

### Frontend

TestCasesView has an **Import** button (Upload icon) next to the export
menu: pick a `.csv`/`.json` file and the cases are created in the currently
selected product, with per-row errors listed in the form.
