# Outbound Webhooks

Kiwit pushes signed HTTP callbacks (webhooks) to external systems when test
runs and executions change state. Webhooks are instance-level, admin-managed
resources delivered as signed JSON `POST` requests.

## Quick start: your first webhook in 5 minutes

Prerequisites: an admin JWT (all webhook endpoints are admin-only) and a
publicly reachable `http(s)` endpoint that accepts `POST`.

1. Create a webhook and subscribe it to events (returns the stored resource):

   ```bash
   curl -X POST "$KIWIT/api/v1/webhooks" \
     -H "Authorization: Bearer $ADMIN_JWT" -H "Content-Type: application/json" \
     -d '{"name":"ci-hook","url":"https://ci.example.com/hooks/kiwit","secret":"topsecret",
          "events":["run.stopped","execution.failed","execution.assigned"]}'
   ```

2. Send a signed test delivery to it (replace `1` with the created ID):

   ```bash
   curl -X POST "$KIWIT/api/v1/webhooks/1/test" \
     -H "Authorization: Bearer $ADMIN_JWT"
   ```

   The target receives a `webhook.test` payload with the same headers and
   signature scheme as real events. Success returns `{"delivered": true,
   "status_code": 200}`; failures return a generic `502`.

3. Trigger a real event (for example, stop a test run) and implement
   [signature verification](#verifying-signatures) before trusting payloads.

## Managing webhooks

All endpoints require the `admin` role. The resource is `/api/v1/webhooks`;
list returns rows ordered by `id` ascending.

| Method   | Path                            | Description                                   |
|----------|---------------------------------|-----------------------------------------------|
| `GET`    | `/api/v1/webhooks`              | List all webhooks                             |
| `POST`   | `/api/v1/webhooks`              | Create a webhook                              |
| `PUT`    | `/api/v1/webhooks/{id}`         | Update a webhook (full replace of body)       |
| `DELETE` | `/api/v1/webhooks/{id}`         | Delete; returns `{"deleted": true}`          |
| `POST`   | `/api/v1/webhooks/{id}/test`    | Send a signed `webhook.test` delivery         |

Resource fields accepted by create and update (`secret` is never echoed back):

| Field       | Type     | Notes                                                                  |
|-------------|----------|------------------------------------------------------------------------|
| `name`      | string   | Required, trimmed, non-empty                                           |
| `url`       | string   | Required, must parse as `http:` or `https:` with a host                |
| `secret`    | string   | Optional, write-only; see below                                        |
| `events`    | []string | Subscriptions; at least one required on create, deduplicated, validated against the catalog |
| `is_active` | bool     | Optional, defaults to `true`                                           |

### Secret semantics

The secret is write-only: never included in any response. An omitted or
empty `secret` on `PUT` **keeps the stored secret**, so updates never
silently disable signing. It cannot be cleared; delete and re-create the
webhook to stop signing.

## Delivery contract

Kiwit delivers `POST` requests with a JSON body and these headers:

| Header              | Value                                            |
|---------------------|--------------------------------------------------|
| `Content-Type`      | `application/json`                               |
| `User-Agent`        | `Kiwit-Webhook/1.0`                              |
| `X-Kiwit-Event`     | Event name, e.g. `run.stopped`                   |
| `X-Kiwit-Signature` | `t=<unix>,v1=<hex>` — only when a secret is set |

The signature is a GitHub-style timestamped HMAC: `v1` is the hex-encoded
HMAC-SHA256 of `"<t>.<body>"` (unix timestamp, literal `.`, raw body) keyed
with the secret; `t` enables replay protection (see below).

Deliveries are fire-and-forget: each runs in a background goroutine with a
10-second timeout and one attempt only (no retries). A status of 400 or
higher counts as a failure, logged server-side and dropped.

## Payload

| Field         | Type   | Always present   | Description                                            |
|---------------|--------|------------------|--------------------------------------------------------|
| `event`       | string | yes              | Event name from the catalog                            |
| `entity_type` | string | no (`omitempty`) | `test_run`, `test_execution`, or `webhook`             |
| `entity_id`   | number | no (`omitempty`) | ID of the entity the event refers to                   |
| `title`       | string | yes              | Human-readable one-liner describing the event          |
| `body`        | string | no (`omitempty`) | Free-form detail (run summary or execution comment)   |
| `timestamp`   | string | yes              | Delivery time, RFC 3339 in UTC                          |

Example payload:

```json
{
  "event": "execution.failed",
  "entity_type": "test_execution",
  "entity_id": 42,
  "title": "Execution #42 failed (run #17)",
  "body": "Regression after API change",
  "timestamp": "2025-01-15T12:30:05Z"
}
```

## Event catalog

| Event                | Emitted when                                                 | `entity_type`                                |
|----------------------|--------------------------------------------------------------|----------------------------------------------|
| `run.stopped`        | A test run is stopped for the first time                     | `test_run`                                   |
| `execution.failed`   | An execution transitions into `failed` status                | `test_execution` (single), `test_run` (bulk) |
| `execution.assigned` | An execution is assigned to a tester without a status change  | `test_execution` (single), `test_run` (bulk) |
| `webhook.test`       | `POST /api/v1/webhooks/{id}/test` is called                  | `webhook`                                    |

`webhook.test` cannot be subscribed to; the catalog accepted in `events`
is `run.stopped`, `execution.failed`, `execution.assigned` (unknown values
are rejected with `400`).

### Event shapes

- `run.stopped`: title `Test run #X stopped`; `entity_id` = run ID; `body` = run summary.
- `execution.failed` (single): title `Execution #X failed (run #Y)`; `body` = update comment if provided, else run summary.
- `execution.failed` (bulk, one per run): title `N executions failed (run #X)`; `body` = run summary.
- `execution.assigned` (single): title `Execution #X assigned to you`; no `body`.
- `execution.assigned` (bulk, one per run): title `N executions assigned to you (run #X)`; `body` = run summary.
- `webhook.test`: title `Test delivery from Kiwit TCMS`; `entity_id` = webhook ID; no `body`.

### Emission rules

- **Transition guards (idempotency):** re-stopping a stopped run does not
  emit `run.stopped`; re-failing an already `failed` execution (single or
  bulk) does not emit `execution.failed`; assignments emit only with an
  unchanged status.
- **Audience:** in-app and email channels exclude the acting user, and when
  the stakeholder list is then empty no event is emitted at all, so
  webhooks are skipped too. Subscriptions are not per-user.
- An active webhook only receives events listed in its `events` array;
  deactivated webhooks (`is_active: false`) receive nothing.

## Verifying signatures

Verify every delivery before trusting it. Example in Go:

```go
func verify(secret, sigHeader string, body []byte) bool {
	parts := strings.SplitN(sigHeader, ",", 2)
	if len(parts) != 2 {
		return false
	}
	ts, err := strconv.ParseInt(strings.TrimPrefix(parts[0], "t="), 10, 64)
	if err != nil {
		return false
	}
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write([]byte(fmt.Sprintf("%d.%s", ts, body)))
	if "v1="+hex.EncodeToString(mac.Sum(nil)) != parts[1] {
		return false
	}
	return time.Since(time.Unix(ts, 0)) < 5*time.Minute // reject replays
}
```

Use a constant-time comparison (for example `hmac.Equal`) in production code, and reject stale `t` timestamps to prevent replays.

## Security

- **SSRF guard.** Targets resolving to loopback, private, link-local,
  multicast, or unspecified IPs are refused at dial time; only the first
  vetted IP is dialed, and every connection — including redirects — goes
  through the guarded dialer. Local development can set
  `KIWIT_WEBHOOK_ALLOW_PRIVATE=1` (or `true`/`yes`); production keeps the
  guard on.
- **Test deliveries** that fail return a generic `502` without target
  details; **secrets** are stored and never returned.

## Limitations

- No retries and no dead-letter queue: a failed delivery is logged and lost.
- No delivery log or retention over REST; failures are server log lines only.
- A webhook secret cannot be removed; recreate the webhook to disable signing.
- No HTTP proxy support: the delivery client ignores `HTTP_PROXY`-style
  environment variables.
