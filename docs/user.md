# User Guide

This guide covers day-to-day work in Kiwit TCMS: writing test cases, running
them, reporting bugs and tracking progress. Administration (installation,
configuration, user management) is covered in [admin.md](admin.md).

## First login

1. Open the application address (default when running locally:
   `http://localhost:8080`).
2. Log in with your account. New instances start with the administrator
   account configured at init time (see [admin.md](admin.md#initializing-the-database)).
3. If self-registration is enabled, use **Create an account** on the login
   screen. Registrations are protected by a simple math captcha and a
   honeypot field.

Forgot your password? See [password-reset.md](password-reset.md) — the whole
flow is self-service: a one-time link is e-mailed to you and stays valid for
one hour.

## Finding your way around

- **Sidebar** — the menu is configurable by the administrator (the same
  concept as `MENU_ITEMS` in Kiwi TCMS). Groups cover Testing, Search and
  Navigate.
- **Command palette** — press `Ctrl+K` (or the search button in the header)
  to jump to any view or product without touching the mouse.
- **Language & theme** — Polish/English switch and light/dark theme toggle
  live in the sidebar footer. The whole UI is translated; no mixed language.
- **Product switcher** — most views are scoped to the product selected in
  the header. Change the product once and every list follows.

## Test cases

Test cases are the core record. Each case belongs to a product and a
category, has a priority, status (`draft`, `confirmed`, `disabled`) and one
of four formats:

| Format | Best for | Where the content lives |
|---|---|---|
| `steps` | classic step-by-step scripts | numbered steps with Action + Expected result |
| `gherkin` | BDD scenarios (Given/When/Then) | a single Gherkin body; exports to `.feature` |
| `markdown` | free-form descriptions | a Markdown body rendered in the UI |
| `checklist` | quick pass/fail lists | compact checklist items |

Practical features:

- **Data-driven testing** — a case can carry any number of *datasets*
  (named parameter sets). When the case is executed inside a test run you
  pick which dataset applies, and the execution remembers it. Datasets can
  also be proposed by the AI assistant.
- **Automation** — mark a case as automated and fill in the `script`
  location and `arguments`; CI pipelines can look them up.
- **Revisions** — every edit snapshots the previous version. Open a case and
  browse its revision history to compare what changed.
- **Clone** — duplicate a case (with its steps and datasets) instead of
  re-typing.
- **Bulk edit** — select many cases in the list and change status or
  priority in one action.
- **Export** — any filtered list or a selection can be exported to CSV,
  JSON, Markdown, XLSX, or (for Gherkin cases) a `.feature` file. See
  [export.md](export.md).
- **Attachments, comments, tags** — attach files, discuss a case and tag it
  for filtering.

## Test plans

Plans group cases for a product version and can be nested (a plan may have
a parent plan). Add cases from the plan view, reorder them, then export the
plan (CSV/JSON/MD/XLSX).

## Test runs

A test run executes a plan against a concrete **build** of a version.

1. Create a run: pick a plan, version and build.
2. Execute cases one by one: set a status
   (`idle`, `passed`, `failed`, `blocked`), leave a comment, pick a dataset,
   and — when something breaks — file a bug straight from the execution.
3. Statuses propagate to the plan and dashboard statistics in real time.

Other run features:

- **JUnit import** — paste or upload a JUnit XML report and Kiwit creates
  executions automatically. This is the standard way to ingest CI results.
- **Reports & exports** — per-run exports in CSV (legacy layout), HTML,
  JUnit XML, JSON and Markdown; the HTML report is shareable with people
  who do not use the tool.
- **Stop/finish a run** — closing a run freezes its statistics.

## Bugs and issue trackers

Bugs live in Kiwit and can be linked to executions and cases. When an issue
tracker is configured (GitHub, GitLab, Jira, Bugzilla or Azure Boards), you
can report a bug directly to the external tracker from an execution and
Kiwit keeps the link and the external status in sync.

## Requirements, milestones, campaigns

- **Requirements** — write requirements in Markdown, link test cases to them
  for traceability and see coverage per requirement
  (`GET /api/v1/requirements/{id}/coverage`).
- **Milestones** — group test runs into a goal with a target date and
  status; milestone progress feeds the dashboard.
- **Campaigns** — bundle multiple runs (e.g. a regression campaign) and
  watch their aggregate status.

## Exploratory testing

Dedicated view for session-based exploratory testing: a charter (what to
explore, timebox), notes taken live during the session, and conversion of
notes into findings/bugs afterwards. Charters and note summaries can be
drafted with the AI assistant.

## Dashboard

The dashboard aggregates executions into:

- pass/fail summary and daily trends,
- a status matrix (case × status),
- test-case health (flaky detection),
- tester workload and UX metrics,
- milestone progress.

Any dataset can be exported (CSV/JSON/MD/XLSX) — the export button lives
next to the dataset selector.

## AI assistant

If the administrator configured an LLM server, several helpers become
available (see [admin.md](admin.md#ai-server)): generating test cases or
steps from a requirement, proposing data-driven datasets, analyzing
requirement quality, converting exploratory notes into findings, summarizing
runs, triaging bugs, estimating regression scope for a milestone and a daily
dashboard TL;DR.

## Notifications and webhooks

Comment on a case and its watchers get an e-mail / in-app notification
(depending on preferences). Administrators can additionally configure
outbound webhooks for events such as `testrun.updated` or `bug.created` —
see [webhooks.md](webhooks.md).

## Your account

- Change your password in **Account** (the current password is required).
- Personal API tokens for scripts and CI live under **Account → API
  tokens**; treat them like passwords and revoke them when they leak.
- The XML-RPC / JSON-RPC API is compatible with the `tcms-api` Python
  client — see [xmlrpc-api.md](xmlrpc-api.md).
