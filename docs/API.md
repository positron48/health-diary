# HTTP API specification

Prefix: `/api/v1`.

Encoding: JSON UTF-8.

Authentication: server-side session cookie except auth and infrastructure endpoints.

## 1. Common conventions

- Dates: `YYYY-MM-DD` in user timezone.
- Timestamps: RFC3339 with offset or `Z`.
- IDs: opaque UUID strings; never Telegram IDs.
- Pagination: cursor-based, newest first unless documented.
- Mutations accept `Idempotency-Key` where retries are expected.
- Mutations use event `revision` for optimistic concurrency.

Error envelope:

```json
{
  "error": {
    "code": "validation_failed",
    "message": "Проверьте введённые данные",
    "fields": {"dose_value": "must be positive"},
    "request_id": "..."
  }
}
```

The user message is localized; `code` is stable. Never return provider error bodies or SQL details.

## 2. Authentication

### `POST /auth/challenges`

Creates an unbound challenge.

Response `201`:

```json
{
  "challenge_id": "uuid",
  "telegram_deep_link": "https://t.me/example_bot?start=login_OPAQUE",
  "expires_at": "2026-07-21T12:05:00Z"
}
```

Rate-limited by IP hash. Plain deep-link token is returned once and not stored.

### Bot deep-link binding (internal)

`/start login_OPAQUE` binds the challenge to the allowlisted Telegram user and sends a 6-digit code. It is not a public web endpoint.

### `POST /auth/challenges/{challenge_id}/verify`

```json
{"code":"123456"}
```

Response `204` with `Set-Cookie`. Errors: `challenge_expired`, `invalid_code`, `challenge_locked`; wording must not disclose account details.

### `GET /auth/session`

Returns current user display settings and session expiry.

### `DELETE /auth/session`

Revokes current session and clears cookie.

### `DELETE /auth/sessions`

Revokes all sessions for current user except optionally current session.

## 3. Calendar and timeline

### `GET /calendar?month=2026-07&mode=overview`

Modes: `overview`, `pain`, `medication`, `activity`, `sleep`, `wellbeing`.

Response contains daily aggregates only:

```json
{
  "month": "2026-07",
  "timezone": "Europe/Moscow",
  "days": [{
    "date": "2026-07-21",
    "has_data": true,
    "pain": {"episodes": 1, "max_intensity": 6, "open": false},
    "medication": {"intakes": 1},
    "activity": {"minutes": 30},
    "sleep": {"minutes": 420, "quality": 6},
    "wellbeing": {"score": 7},
    "pending_count": 0
  }]
}
```

No raw text in calendar response.

### `GET /days/{date}`

Returns ordered timeline, episode summaries and pending batches for local date. Raw source text is excluded unless explicitly requested through the protected source endpoint.

### `GET /entries/{entry_id}`

Returns decrypted source only for its owner, with `Cache-Control: no-store`. Access is audited.

## 4. Events

### `GET /events`

Filters: `from`, `to`, repeated `kind`, `status`, `cursor`, `limit<=100`.

### `GET /events/{id}`

Returns common envelope, typed data, provenance metadata and current revision.

### `PATCH /events/{id}`

```json
{
  "revision": 2,
  "occurred_at": "2026-07-21T12:00:00Z",
  "time_precision": "approximate",
  "data": {"intensity": 5}
}
```

Only supplied fields change. Same domain validation as LLM output. Response `409 revision_conflict` for stale edit.

### `DELETE /events/{id}`

Soft delete, requires current `revision`; returns `204`. Analytics refresh is immediate.

### `POST /events/{id}/restore`

Restores within configurable undo window if related episode remains consistent.

## 5. Batches and parsing

### `GET /batches?status=pending`

Pending confirmation inbox.

### `POST /batches/{id}/confirm`

Body includes `version`; confirms all candidate events atomically.

### `POST /batches/{id}/reject`

Rejects candidate events but retains source entry according to retention policy.

### `POST /entries/{id}/retry`

Queues parsing after retryable failure. Rate-limited/idempotent.

### `POST /batches/{id}/corrections`

Accepts correction text from web; creates a new extraction batch. Manual field edits should use `PATCH /events/{id}` instead.

## 6. Episodes

### `GET /episodes`

Filters: `from`, `to`, `status`, `symptom_type`, cursor.

### `GET /episodes/{id}`

Returns episode interval, observations, related medications and calculated duration/max intensity.

### `POST /episodes/{id}/close`

Manual close with `ended_at`, precision and optional final intensity.

### `POST /episodes/{id}/reopen`

Reopens a mistakenly closed episode; creates audit revision.

## 7. Analytics

### `GET /analytics/summary?from=...&to=...`

Returns coverage, pain, medication, activity, sleep and wellbeing metrics with numerator/denominator.

### `GET /analytics/associations?from=...&to=...`

Returns only associations passing gates from `ANALYTICS.md`; otherwise includes `insufficient_data` reasons.

### `GET /analytics/medications?from=...&to=...`

Returns intake days, linked episodes and recorded response. It does not make treatment recommendations.

## 8. User/settings/export/delete

### `GET /me` / `PATCH /me`

Editable: timezone, locale, reminder preferences, raw retention preference and optional tracking fields.

### `POST /exports`

Body: `format` (`json`, `csv` initially), date range and included categories. Creates background export.

### `GET /exports/{id}`

Status and short-lived authenticated download link. Files expire automatically.

### `DELETE /exports/{id}`

Deletes artifact.

### `POST /me/deletion-request`

Requires re-authentication/code confirmation and returns a deletion job status. Final destructive semantics are defined in `SECURITY.md` and `DATA_MODEL.md`.

## 9. Telegram webhook

### `POST /telegram/webhook/{secret_path}`

- Validate `X-Telegram-Bot-Api-Secret-Token`.
- Limit body size and content type.
- Return quickly after durable transaction.
- Duplicate `update_id` returns `200` without repeated effects.
- Not exposed in OpenAPI public docs.

## 10. Infrastructure endpoints

- `GET /healthz`: process alive; no dependency details.
- `GET /readyz`: DB reachable, migrations compatible, worker initialized.
- `GET /metrics`: separate port or network-restricted route; no health labels or user IDs.

## 11. HTTP status mapping

| Status | Use |
|---|---|
| 200/201/204 | success |
| 400 | malformed request |
| 401 | absent/invalid session |
| 403 | authenticated but not allowed |
| 404 | resource absent or belongs to another user |
| 409 | revision/state conflict |
| 422 | domain validation failure |
| 429 | rate limit |
| 503 | temporary dependency unavailable |

Cross-user resource access returns `404`, not `403`.
