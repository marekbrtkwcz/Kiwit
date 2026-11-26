# Password Reset

Kiwit lets users recover a forgotten account through a one-time reset link
emailed to the registered address, and lets admins force a reset for any
account. Links are single-use and valid for one hour; the token is never
stored in plain form.

## Quick start: reset your password

For end users, the whole flow lives in the web UI and takes about a minute:

1. On the login screen choose **Forgot your password?** (Polish UI:
   *Nie pamiętasz hasła?*).
2. Enter your account e-mail and submit. The app shows a neutral message
   ("If the account exists, we have sent a password reset link.") whether
   or not the account exists.
3. Check your mailbox for "[Kiwit] Password reset" and open the link. It
   points at `https://<your-kiwit-host>/reset-password?token=...`, where the
   token is 64 hex characters, and works even if you are still logged in
   elsewhere.
4. Enter the new password twice (minimum 8 characters) and submit.
5. Sign in with the new password.

If a link is shown as invalid, it either expired, was already used, or was
superseded by a newer admin reset — request a new one.

## Endpoints

The reset flow shares the API surface with the other auth endpoints:

| Method | Path                                    | Auth       | Purpose                                             |
|--------|-----------------------------------------|------------|------------------------------------------------------|
| `POST` | `/api/v1/auth/password-reset`           | public     | Request a reset link for an e-mail                   |
| `POST` | `/api/v1/auth/password-reset/confirm`   | public     | Set a new password with the one-time token           |
| `POST` | `/api/v1/users/{id}/password-reset`     | `admin`    | Force a fresh reset link for a user (admin fallback) |

Users who still know their current password do not need this flow — they
call `POST /api/v1/auth/change-password` with `current_password` and
`new_password` instead.

## Requesting a reset

`POST /api/v1/auth/password-reset` is public and unauthenticated.

```json
{"email": "alice@example.com"}
```

The response is always `{"ok": true}` with HTTP 200. Unknown e-mail
addresses are a silent no-op, so the endpoint cannot be used to enumerate
accounts.

For scripted checks (health of the endpoint, not for real resets):

```bash
curl -s -X POST "$KIWIT/api/v1/auth/password-reset" \
  -H "Content-Type: application/json" \
  -d '{"email": "alice@example.com"}'
# {"ok":true}
```

Password-reset requests share a sliding per-IP rate limit with login and
registration: **20 requests per minute per IP** by default (login,
register, reset request, and reset confirmation all count against the same
budget). When the budget is exhausted, Kiwit answers with HTTP 429, a
`Retry-After` header, and the JSON body
`{"title": "Too Many Requests", "status": 429, "detail": "rate limit exceeded"}`.
The client IP is taken from the first `X-Forwarded-For` entry or from the
connection remote address.

## Confirming a reset

`POST /api/v1/auth/password-reset/confirm` completes the flow with the
token from the emailed link.

| Field          | Type   | Notes                                                    |
|----------------|--------|----------------------------------------------------------|
| `token`        | string | Required, `minLength: 1`; the 64-hex token from the link |
| `new_password` | string | Required, `minLength: 8`                                  |

| Response           | When it happens                                                            |
|--------------------|----------------------------------------------------------------------------|
| `200 {"ok": true}` | Password changed; sign in with the new password                            |
| `400`              | Unknown, expired, or already used token (single generic detail, indistinguishable) |
| `422`              | Schema validation failed (e.g. password shorter than 8, empty token, invalid JSON) |
| `403`              | Bearer is a `kiwit_*` API token, not a user session                        |
| `429`              | Per-IP rate limit exhausted                                                |

The change is atomic: the server validates the token, stores the bcrypt
hash of the new password, and marks the token used in one database
transaction. A used token is therefore spent even if you retry with a
stronger password — request a fresh link.

An important subtlety: the API cannot tell you *which* case the 400 covers
(unknown vs expired vs used token). The UI shows a generic error and points
back to the reset-request form.

## Admin-triggered resets

Admins can force a reset for any account without knowing the old password:

| Method | Path                                | Role    | Description                                            |
|--------|-------------------------------------|---------|---------------------------------------------------------|
| `POST` | `/api/v1/users/{id}/password-reset` | `admin` | Create a fresh one-time token for the user (`404` if the user does not exist) |

```bash
curl -s -X POST "$KIWIT/api/v1/users/42/password-reset" \
  -H "Authorization: Bearer $ADMIN_JWT"
# {"emailed":false,"reset_url":"http://localhost:8080/reset-password?token=<64 hex>"}
```

The response depends on e-mail configuration:

```json
{"emailed": true}
```

- **With SMTP configured** (`emailed: true`): the link is delivered to the
  user's address in the background. `emailed` means accepted for the
  background send; delivery errors are logged server-side only.
- **Without SMTP** (`emailed: false` + `reset_url`): a self-hosted
  fallback — the raw link is returned in the response **and written to the
  server log**. Hand the link to the user via a trusted channel; it grants
  full account access.

Creating an admin reset **invalidates all of the user's earlier unused
reset tokens**, so after this call only the newest link works. Note that
the public self-service request does *not* invalidate earlier links; each
request simply adds one more token valid until expiry or use.

### Configuration

The flow reads these environment variables at server start:

| Variable             | Purpose                                            | Default                 |
|----------------------|----------------------------------------------------|--------------------------|
| `KIWIT_BASE_URL`     | Base for the link to the reset page                | `http://localhost:8080` |
| `KIWIT_SMTP_HOST`    | SMTP host; unset/empty disables e-mail delivery    | (empty)                 |
| `KIWIT_SMTP_PORT`    | SMTP port                                          | (empty)                 |
| `KIWIT_SMTP_FROM`    | From address for reset e-mails                     | (empty)                 |
| `KIWIT_SMTP_USER`    | SMTP user name                                     | (empty)                 |
| `KIWIT_SMTP_PASS`    | SMTP password                                      | (empty)                 |

The reset link always points at `KIWIT_BASE_URL` + `/reset-password?token=...`;
without a configured base URL it falls back to `http://localhost:8080`, so
self-hosted deployments behind a public hostname **must** set `KIWIT_BASE_URL`
or emailed links will open the wrong host.

## Security

- **No plain tokens.** The reset token is 32 bytes from `crypto/rand`
  (rendered as 64 hex characters); only its SHA-256 hash reaches the
  database, so a leak of stored data cannot be replayed into the flow.
- **One-time and short-lived.** Tokens are single-use (`used_at`) and
  expire after 1 hour.
- **No account enumeration.** The public request answers a fixed
  `{"ok": true}` for both existing and unknown accounts; the confirm
  endpoint uses one generic error for unknown/expired/used tokens.
- **Rate limiting.** Both public reset routes sit behind the shared
  per-IP auth rate limiter (20/min), limiting brute-force use of tokens.
- **API tokens cannot reset passwords.** A `kiwit_*` API token bearer
  receives `403` on the confirm endpoint.
- **UI does not leak state.** The reset pages ignore any stored JWT, so a
  stale logged-in session never hides the reset forms; opening
  `/reset-password` without a token shows an error card with a way back to
  the login screen.
- **Success responses carry no secrets.** The public endpoints never
  return a reset link; only the admin-only route can return `reset_url`.

## Limitations

- **E-mail contents are English only.** The UI is translated (PL/EN), but
  the reset e-mail subject and body are not localized.
- **No token management UX.** Issued tokens are not listable or revocable
  over REST; only relevant for admins via the invalidation rule above.
- **Shared rate-limit budget.** Reset traffic consumes the same per-IP
  window as login/register, so CI systems hammering login can soak the
  budget users need for resets.
- **Links in logs without SMTP.** Without SMTP every reset link (public
  and admin flow) is written to the server log; treat those log lines as
  credentials and ensure log retention is at least as short as the 1-hour
  token TTL.
- **E-mail delivery is fire-and-forget.** A failed SMTP send is logged but
  not retried; the user must request a new link.
