# HTTP API specification

Prefix: `/api/v1`.

Encoding: JSON UTF-8.

Authentication: server-side session cookie except auth and infrastructure endpoints.

## 1. Common conventions

- Dates: `YYYY-MM-DD` in the user's configured day. Its half-open interval
  starts at `settings.day_start_time` (default `00:00`) in the user timezone.
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

### `POST /entries`

Creates an encrypted web diary entry and queues extraction:

```json
{"text":"Около 15:00 заболела голова","date":"2026-07-22"}
```

Requires an `Idempotency-Key` header. Response `201`:
Optional `date` supplies the selected calendar day as extraction context; it
does not fabricate a missing event time.

```json
{"entry_id":"uuid","status":"queued"}
```

The response and subsequent reads are `no-store`. Extracted events remain pending until confirmation and are excluded from calendar aggregates and analytics.

### `GET /calendar?month=2026-07`

Returns all day-layer aggregates for the month. Optional `layers` is informational for clients; the server always returns every available layer. Legacy `mode` is accepted as an alias and echoed but does not filter aggregates.

Response:

```json
{
  "month": "2026-07",
  "timezone": "Europe/Moscow",
  "layers_available": ["pain","medication","activity","sleep","wellbeing","context","weather"],
  "days": [{
    "date": "2026-07-21",
    "has_data": true,
    "has_pending": false,
    "pain": {"max_intensity": 6, "open": false},
    "medication": {"intakes": 2},
    "activity": {"minutes": 45},
    "sleep": {"minutes": 420, "quality": 6},
    "wellbeing": {"score": 7, "motivation": 5},
    "context": {"period_type": "trip", "place_label": "Новосибирск", "segment": "middle"},
    "weather": {"temp_mean_c": 18, "weather_code": 3, "pressure_delta_24h_hpa": -8, "is_complete": true},
    "pending_count": 0
  }]
}
```

Compact UI rules: pain shows max intensity without `/10` or episode count; medication shows intake count with pill icon only; context is a continuous ribbon; weather shows one temperature plus icon. No raw text in calendar response.

### `GET /places/search?q=Липецк`

Authenticated city search via configured geocoder (Open-Meteo). Returns candidate cities with label, region, country, timezone and provider IDs. Does not store a place until the user selects one.

### `GET /me` / `PATCH /me`

Editable settings also include `home_place_id`. Selecting a home place may queue weather enrichment for completed local days.

### `GET /context-periods` / `POST /context-periods` / `PATCH /context-periods/{id}`

CRUD for confirmed context periods with revision concurrency. Creating/updating a period may enqueue weather enrichment for the covered dates.

### `GET /days/{date}`

Returns ordered timeline, episode summaries and pending batches for local date. Raw source text is excluded unless explicitly requested through the protected source endpoint.

### `GET /entries/{entry_id}`

Returns decrypted source only for its owner, with `Cache-Control: no-store`. Access is audited.

## 4. Events

### `GET /events`

Filters: `from`, `to`, repeated `kind`, `status`, `cursor`, `limit<=100`.

This endpoint returns only current confirmed, non-deleted events. `status` is
accepted only as `confirmed`; pending candidates are available through
`GET /batches?status=pending`.

### `GET /events/{id}`

Returns common envelope, typed data, provenance metadata and current revision:

```json
{
  "id": "uuid",
  "entry_id": "uuid",
  "episode_id": "uuid-or-null",
  "kind": "pain_observation",
  "occurred_at": "2026-07-21T12:00:00Z",
  "ended_at": null,
  "time_precision": "approximate",
  "data": {
    "symptom_type": "headache",
    "phase": "update",
    "intensity": 5,
    "locations": ["occiput_neck"]
  },
  "revision": 2
}
```

`entry_id` is always present for source disclosure. `episode_id` is present when the event is linked through `pain_observations` or `medication_intakes`.

### `PATCH /events/{id}`

```json
{
  "revision": 2,
  "occurred_at": "2026-07-21T12:00:00Z",
  "time_precision": "approximate",
  "data": {
    "intensity": 5,
    "comment": "после кофе стало хуже"
  }
}
```

Activity example:

```json
{
  "revision": 1,
  "data": {
    "activity_type": "бег",
    "duration_minutes": 40,
    "intensity": "moderate"
  }
}
```

Only supplied top-level fields change. Nested `data` is merged into existing attributes: omitted keys are preserved, explicit `null` clears a field. Same domain validation as LLM output. `data.comment` is user-authored (max 1000 runes); empty string from clients should be sent as `null` to clear. Activity `intensity` is `low|moderate|high`, not the pain 0..10 scale. Response `409 revision_conflict` for stale edit. After a successful pain or medication edit the episode projection is recalculated.

### `DELETE /events/{id}`

Soft delete, requires current `revision`; returns `204`. Analytics refresh is immediate.

### `POST /events/{id}/restore`

Restores within configurable undo window if related episode remains consistent.

## 5. Batches and parsing

### `GET /inbox`

Single poll endpoint for the web «Входящие» screen:

```json
{
  "processing": [
    {
      "id": "uuid",
      "source_type": "web",
      "source_sent_at": "2026-07-22T19:30:00Z",
      "processing_status": "queued"
    }
  ],
  "batches": [],
  "processing_count": 1,
  "batch_count": 0,
  "count": 1
}
```

`processing` lists `journal_entries` with `processing_status` in `queued`, `processing` or `failed` (newest first). Source plaintext is absent. `batches` matches pending confirmation batches (same shape as `GET /batches?status=pending`). `count` is `processing_count + batch_count` for nav badges. Clients may poll about every 2s while the inbox is visible.

### `GET /batches?status=pending`

Pending confirmation inbox. Each batch includes source-entry ID, source type,
message timestamp, batch version and candidate event time precision. Source
text is deliberately absent. Prefer `GET /inbox` when the UI also needs in-flight entries.

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
`settings.day_start_time` accepts `HH:MM` (`00:00` by default). `GET /me`
also returns `current_local_date` calculated with this boundary so calendar
navigation and the Today screen use the same date as the backend.

### `GET /exports?format=json|csv` (MVP)

Returns a synchronous authenticated download of current confirmed,
non-deleted events. The response is `Cache-Control: no-store`. This is the
single explicit MVP export contract; the asynchronous lifecycle is deferred
until export size or report generation requires stored artifacts.

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

## 12. Compatibility

`/api/v1` is the canonical public prefix. Existing root routes remain as
temporary compatibility aliases for the initial web shell. New clients must
use `/api/v1`; aliases may be removed only after the shell migration is
complete.
