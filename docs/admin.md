# Administration Guide

This guide covers installing, configuring and operating a Kiwit TCMS
instance: deployment, environment variables, runtime settings, users and
permissions, logging, webhooks, trackers, AI and backups. End-user topics
are in [user.md](user.md).

## Deployment

Kiwit is a single Go binary that embeds the React frontend, the OpenAPI
docs and this documentation — no runtime dependencies except the database.

Options:

- **Release binary** — `dist/kiwit.exe` (Windows) or
  `dist/kiwit-linux-amd64`. Set the environment variables below and run it.
- **Docker** — `docker compose up -d` (see `docker-compose.yml`; includes a
  PostgreSQL service).
- **From source** — `make build-windows` / `make build-linux`, then run the
  binary. Building requires Go and, for a fresh frontend build, Node.js.

The server listens on `:8080` by default (`KIWIT_ADDR`). Database schema
migrations run automatically at startup (goose) for both SQLite and
PostgreSQL; there is no manual migration step.

### Initializing the database

On a fresh database, initialize the instance with `GET/POST /init-db/`
(Kiwi TCMS compatible), for example:

```
curl -X POST http://localhost:8080/init-db/
```

This:

1. creates the admin account from `KIWIT_ADMIN_USERNAME`,
   `KIWIT_ADMIN_EMAIL` and `KIWIT_ADMIN_PASSWORD` (request body overrides
   env; final defaults: `admin` / `admin@kiwit.local` / `admin`),
2. seeds the dictionaries (priorities, categories, plan types,
   classifications, environments) and default templates.

`KIWIT_ADMIN_PASSWORD` should always be set in production. A full wipe +
re-seed is available to admins at `POST /api/v1/admin/reset-db` — it
deletes all data, so back up first.

## Environment variables

| Variable | Default | Purpose |
|---|---|---|
| `KIWIT_ADDR` | `:8080` | listen address |
| `KIWIT_DATABASE_URL` | `sqlite://./kiwit.db` | `sqlite://path` or `postgres://user:pass@host:5432/db` |
| `KIWIT_JWT_SECRET` | insecure dev value | **set in production**; signs session and API tokens |
| `KIWIT_BASE_URL` | `http://localhost:8080` | absolute URL used in e-mails and links |
| `KIWIT_ENV` | `development` | `production` hides verbose error details |
| `KIWIT_UPLOAD_DIR` | `./uploads` | local attachment storage |
| `KIWIT_STORAGE_TYPE` | `local` | `local` or `s3` |
| `KIWIT_S3_ENDPOINT` / `KIWIT_S3_REGION` / `KIWIT_S3_BUCKET` / `KIWIT_S3_ACCESS_KEY` / `KIWIT_S3_SECRET_KEY` / `KIWIT_S3_USE_PATH_STYLE` | — | S3 / MinIO attachment backend |
| `KIWIT_ALLOW_REGISTRATION` | `true` | self-service sign-up |
| `KIWIT_CAPTCHA_ENABLED` | `true` | math captcha + honeypot on registration (only when registration is on) |
| `KIWIT_ANONYMOUS_ACCESS` | `false` | read-only access for anonymous visitors (public products only) |
| `KIWIT_MENU_JSON` | built-in menu | custom sidebar menu structure (Kiwi TCMS `MENU_ITEMS` equivalent) |
| `KIWIT_ADMIN_USERNAME` / `KIWIT_ADMIN_EMAIL` / `KIWIT_ADMIN_PASSWORD` | `admin` / `admin@kiwit.local` / generated | admin created by `/init-db/` |
| `KIWIT_LOG_LEVEL` | `info` | `debug`, `info`, `warn`, `error` |
| `KIWIT_LOG_FORMAT` | `json` | `json` or `text` |
| `KIWIT_LOG_FILE` | — | rotating log file (size/keep from `KIWIT_LOG_MAX_SIZE` MB, `KIWIT_LOG_MAX_FILES`) |
| `KIWIT_SMTP_HOST` / `KIWIT_SMTP_PORT` / `KIWIT_SMTP_USER` / `KIWIT_SMTP_PASS` / `KIWIT_SMTP_FROM` | — | outgoing e-mail |
| `KIWIT_OIDC_ISSUER` / `KIWIT_OIDC_CLIENT_ID` / `KIWIT_OIDC_CLIENT_SECRET` / `KIWIT_OIDC_REDIRECT_URL` / `KIWIT_OIDC_PROVIDER_NAME` | — | SSO login (OIDC; issuer + client ID + secret enable it) |
| `KIWIT_LDAP_SERVER` / `KIWIT_LDAP_BIND_DN` / `KIWIT_LDAP_BIND_PASSWORD` / `KIWIT_LDAP_BASE_DN` / `KIWIT_LDAP_USER_FILTER` / `KIWIT_LDAP_USERNAME_ATTR` / `KIWIT_LDAP_EMAIL_ATTR` | — | LDAP login (server + base DN + filter enable it) |
| `KIWIT_LLM_BASE_URL` / `KIWIT_LLM_MODEL` / `KIWIT_LLM_API_KEY` | — | built-in LLM server for AI features; any OpenAI-compatible endpoint works (Ollama, llama.cpp, OpenAI, Mistral) |
| `KIWIT_WEBHOOK_URL` / `KIWIT_WEBHOOK_ALLOW_PRIVATE` | — | default outbound webhook and private-host policy |
| `KIWIT_TRACKER_TOKEN` / `KIWIT_TRACKER_USERNAME` / `KIWIT_TRACKER_ALLOW_PRIVATE_HOSTS` | — | default issue-tracker credentials and SSRF policy |

Values stored in the database (see below) override env vars at startup, so
the env is the bootstrap default and the admin UI is the live source.

## Runtime settings

Administrators manage live settings without restarting:

- `GET /api/v1/settings` — public settings (domain, registration, captcha,
  anonymous access).
- `GET /api/v1/admin/settings` and `PUT /api/v1/admin/settings/{key}` and
  bulk `POST /api/v1/admin/settings` — keys: `domain`, `smtp_host`,
  `smtp_port`, `smtp_user`, `smtp_pass`, `smtp_from`, `allow_registration`,
  `captcha_enabled`, `anonymous_access`.
- `GET /api/v1/admin/domain` / `PUT /api/v1/admin/domain` — convenience
  wrappers for the public base URL (the `manage.py set_domain` equivalent);
  links in e-mails pick it up immediately.

The Settings view in the UI exposes the same options.

## Users, groups and permissions

Roles: `admin`, `tester`, `viewer`. Product-level isolation comes from
product membership and per-group product permissions.

- **Users** — admin panel: create accounts, change roles, deactivate.
- **Groups** — group users and grant them `tester` or `viewer` rights per
  product (`group_product_perms`). `POST /api/v1/admin/refresh-permissions`
  re-creates the default `Administrator`/`Tester` groups and re-grants
  Tester access to all products (idempotent; Kiwi TCMS
  `refresh_permissions` equivalent).
- **API tokens** — users mint personal tokens; admins can revoke any token.
  Both `Authorization: Bearer <jwt|token>` work on `/api/v1/*` and on the
  XML-RPC/JSON-RPC endpoints.

Login and registration are rate-limited to slow down credential stuffing.

## Logging and audit

- **Application logs** — level/format/rotation via env (above). Secrets
  (JWT, SMTP, webhooks, LLM keys, reset tokens) are scrubbed from all
  sinks. Requests slower than 1 s are logged at WARN, and every request
  carries a request ID that also appears in error responses.
- **Admin log viewer** — Settings → Logs: browse recent in-memory log
  entries with text search, time-range filter and sortable columns; LLM
  calls are tagged so their latency and payload size are visible.
- **Audit log** — who changed what (create/update/delete/stop) for cases,
  plans and runs, with a unified text diff, `GET /api/v1/audit/all`
  (admin) and per-user `GET /api/v1/audit`; exportable to CSV/JSON/MD/XLSX.
  See [export.md](export.md).

## Webhooks

Outbound webhooks are configured in the UI (per-event URL + optional
secret). Kiwit signs deliveries with `X-Kiwit-Signature` (HMAC-SHA256);
delivery is one attempt with a 10-second timeout — there are no retries,
so point webhooks at a reliable receiver. Details and the payload
contract: [webhooks.md](webhooks.md).

## Issue trackers

Configure GitHub, GitLab, Jira, Bugzilla or Azure Boards per product in
Settings. SSRF protection blocks webhook/tracker calls to private network
addresses unless `KIWIT_WEBHOOK_ALLOW_PRIVATE` /
`KIWIT_TRACKER_ALLOW_PRIVATE_HOSTS` explicitly allows it.

## AI server

Set `KIWIT_LLM_BASE_URL`, `KIWIT_LLM_MODEL` and `KIWIT_LLM_API_KEY` to any
OpenAI-compatible endpoint. The API key never leaves the server (it is
registered in the secret vault and masked in logs). Once configured, the UI
offers: test-case and step generation, dataset proposals, requirement
analysis, exploratory charter drafts, note-to-findings conversion, run
summaries, bug triage, regression scoping, visual-regression verdicts and
the daily dashboard TL;DR.

## Data export and backups

- All entities export to CSV/JSON/Markdown/XLSX — see [export.md](export.md).
- **Backups**: SQLite — copy the `.db` file while the server is stopped;
  PostgreSQL — standard `pg_dump`. Attachments: back up `KIWIT_UPLOAD_DIR`
  or the S3 bucket.
- There is no built-in scheduled backup; use cron/systemd timers with the
  commands above.

## Building and embedded docs

The frontend is embedded from `backend/internal/web/dist` and the docs you
are reading from `backend/internal/docs/content` — both are committed, and
`make sync-docs` refreshes the embedded copy from the repo `docs/` folder
before `build-windows`/`build-linux`. A unit test fails the build if the
two copies drift apart.
