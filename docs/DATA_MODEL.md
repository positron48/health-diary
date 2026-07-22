# Data model

## 1. General rules

- PostgreSQL 16 is the source of truth.
- Primary keys are UUIDv7 (or UUIDv4 if the chosen library lacks stable UUIDv7 support).
- All timestamps are `TIMESTAMPTZ` stored in UTC.
- User timezone is an IANA name such as `Europe/Moscow`.
- Health records are always scoped by `user_id`.
- Soft deletion supports normal undo/audit; user-requested hard deletion is a separate privacy operation.
- Flexible attributes use JSONB only behind a versioned Go schema; query-critical fields are typed columns.
- Unknown is `NULL`, not an empty string, zero or guessed value.

## 2. Core tables

### `users`

| Column | Type | Notes |
|---|---|---|
| `id` | uuid PK | Internal identity |
| `telegram_user_id` | bigint unique | Never expose as application ID |
| `telegram_username` | text nullable | Mutable display metadata |
| `locale` | text | Default `ru` |
| `timezone` | text | IANA timezone |
| `status` | text | `active`, `disabled`, `deletion_pending` |
| `settings` | jsonb | Versioned non-secret preferences; `day_start_time` is `HH:MM`, default `00:00` |
| `created_at`, `updated_at` | timestamptz | |

### `telegram_updates`

Idempotency ledger.

| Column | Type | Notes |
|---|---|---|
| `update_id` | bigint PK | Telegram update ID |
| `user_id` | uuid nullable | Resolved user |
| `update_kind` | text | message, callback, command |
| `received_at` | timestamptz | |
| `processed_at` | timestamptz nullable | |
| `result` | text | accepted, duplicate, ignored, failed |

Do not store the whole Telegram update by default.

### `journal_entries`

Immutable source capture.

| Column | Type | Notes |
|---|---|---|
| `id` | uuid PK | |
| `user_id` | uuid FK | |
| `source_type` | text | `telegram_text`, future `telegram_voice`, `web`, `import` |
| `source_message_id` | bigint nullable | Unique with user/source |
| `source_sent_at` | timestamptz | Telegram/user timestamp |
| `received_at` | timestamptz | Server timestamp |
| `raw_text_ciphertext` | bytea | Application-level encrypted text |
| `encryption_key_version` | int | Rotation support |
| `content_sha256` | bytea | Idempotency/debug without plaintext |
| `language` | text nullable | |
| `processing_status` | text | `queued`, `processing`, `parsed`, `needs_input`, `failed`, `deleted` |
| `deleted_at` | timestamptz nullable | |
| `created_at` | timestamptz | |

Unique partial index: `(user_id, source_type, source_message_id)` where message ID is not null.

### `extraction_runs`

| Column | Type | Notes |
|---|---|---|
| `id` | uuid PK | |
| `entry_id` | uuid FK | |
| `attempt` | int | |
| `provider` | text | Config name, not secret |
| `model` | text | |
| `prompt_version` | text | |
| `schema_version` | text | |
| `context_fingerprint` | text | Hash of supplied context |
| `status` | text | running/succeeded/retryable_failed/terminal_failed |
| `validated_result` | jsonb nullable | Normalized validated output |
| `raw_response_ciphertext` | bytea nullable | Off by default |
| `latency_ms`, `input_tokens`, `output_tokens` | int nullable | |
| `error_code` | text nullable | No raw health content |
| `started_at`, `finished_at` | timestamptz | |

Unique `(entry_id, attempt)`.

### `event_batches`

Groups candidate events produced from one extraction/correction for one confirmation action.

| Column | Type | Notes |
|---|---|---|
| `id` | uuid PK | |
| `user_id` | uuid FK | |
| `entry_id` | uuid FK | |
| `extraction_run_id` | uuid FK | |
| `status` | text | `pending`, `confirmed`, `rejected`, `superseded` |
| `version` | int | Optimistic callback lock |
| `confirmed_at`, `rejected_at` | timestamptz nullable | |

### `health_events`

Common event envelope.

| Column | Type | Notes |
|---|---|---|
| `id` | uuid PK | |
| `user_id` | uuid FK | |
| `batch_id` | uuid FK | |
| `entry_id` | uuid FK | Provenance |
| `kind` | text | Enumerated below |
| `occurred_at` | timestamptz | Required |
| `ended_at` | timestamptz nullable | Must be >= occurred_at |
| `time_precision` | text | `exact`, `approximate`, `date_only`, `inferred_from_message` |
| `confidence` | numeric(4,3) nullable | Parser confidence, never medical confidence |
| `status` | text | `pending`, `confirmed`, `superseded`, `deleted` |
| `client_ref` | text | Stable within extraction result |
| `attributes` | jsonb | Versioned secondary fields |
| `revision` | int | |
| `created_at`, `updated_at`, `deleted_at` | timestamptz | |

Unique `(batch_id, client_ref)`. Analytics filter: `status='confirmed' AND deleted_at IS NULL`.

Kinds:

- `pain_observation`
- `medication_intake`
- `wellbeing`
- `activity`
- `sleep`
- `food_drink`
- `measurement`
- `note`

### `event_revisions`

| Column | Type | Notes |
|---|---|---|
| `id` | uuid PK | |
| `event_id` | uuid FK | |
| `revision` | int | |
| `changed_by` | text | bot_correction, web_user, reprocess, system |
| `before_data`, `after_data` | jsonb | Encrypted if containing raw free text |
| `reason` | text nullable | Must not contain health plaintext in logs |
| `created_at` | timestamptz | |

Unique `(event_id, revision)`.

## 3. Typed event tables

Each typed table has `event_id uuid PRIMARY KEY REFERENCES health_events(id) ON DELETE CASCADE`.

### `symptom_episodes`

Represents the lifecycle, not an individual message.

- `id`, `user_id`
- `symptom_type` (`headache` initially)
- `started_at`, `ended_at`
- `start_precision`, `end_precision`
- `status` (`open`, `closed`, `cancelled`)
- `max_intensity` nullable, check `0..10`
- `created_from_event_id`
- `created_at`, `updated_at`

An initial implementation allows multiple open episodes but the bot asks for clarification if more than one could match.

### `pain_observations`

- `event_id`
- `episode_id` FK
- `phase`: `start`, `update`, `end`
- `intensity` smallint nullable, `0..10`
- `locations` text[]
- `laterality` nullable: left/right/bilateral/center/unknown
- `qualities` text[]: throbbing/pressure/stabbing/burning/other
- `associated_symptoms` text[]
- `functional_impact` smallint nullable, `0..3`
- `reported_relief` nullable boolean

### `medication_intakes`

- `event_id`
- `episode_id` nullable FK
- `medication_name_raw` encrypted text or protected column
- `medication_name_normalized` nullable text
- `medication_class` nullable text
- `dose_value` numeric nullable, positive
- `dose_unit` nullable text
- `route` nullable text
- `reason` nullable text
- `effect_rating` nullable smallint, `-2..2`
- `effect_observed_at` nullable timestamptz
- `side_effects` text[]

Medication normalization is a user-editable catalog mapping, not an LLM-only truth.

### `activities`

- `event_id`
- `activity_type`
- `duration_minutes` nullable positive int
- `intensity` nullable: low/moderate/high
- `distance_meters` nullable positive int

### `daily_checkins`

- `event_id`
- `wellbeing_score`, `energy_score`, `mood_score`, `stress_score`, `sleep_quality`: nullable `0..10`
- `explicit_no_headache` nullable boolean

### `sleep_records`

- `event_id`
- `sleep_started_at`, `sleep_ended_at` nullable
- `duration_minutes` nullable positive int
- `quality_score` nullable `0..10`
- `interruptions` nullable non-negative int

### `food_drink_records`

- `event_id`
- `category`: meal/water/caffeine/alcohol/other
- `quantity_value`, `quantity_unit` nullable
- `labels` text[]

### `measurements`

- `event_id`
- `measurement_type`: blood_pressure/heart_rate/temperature/weight/other
- typed `value_primary`, `value_secondary`, `unit`
- constraints per type enforced in domain validation; obviously impossible values rejected or require confirmation.

## 4. Authentication and jobs

### `auth_challenges`

- `id` uuid PK
- `public_token_hash` bytea unique
- `user_id` nullable until Telegram deep-link binding
- `code_hash` bytea nullable
- `expires_at`
- `attempt_count`, `max_attempts`
- `bound_at`, `consumed_at`, `locked_at` nullable
- `request_ip_hash`, `user_agent_hash` nullable
- `created_at`

Never store or log plain challenge token/code.

### `web_sessions`

- `id` uuid PK
- `user_id` FK
- `token_hash` bytea unique
- `expires_at`, `last_seen_at`, `revoked_at`
- `ip_hash`, `user_agent_hash` nullable
- `created_at`

### `jobs`

- `id` uuid PK
- `kind`
- `payload` jsonb containing IDs, never raw health text
- `status`: queued/running/succeeded/retryable_failed/terminal_failed
- `available_at`, `locked_at`, `locked_by`
- `attempts`, `max_attempts`
- `last_error_code`
- timestamps

### `outbox_messages`

Stores retryable Telegram notifications/callback updates as structured message templates and referenced entity IDs.

## 5. Indexes

At minimum:

- `health_events (user_id, occurred_at DESC)` partial confirmed/non-deleted;
- `health_events (user_id, kind, occurred_at DESC)`;
- `symptom_episodes (user_id, started_at DESC)`;
- partial `symptom_episodes (user_id) WHERE status='open'`;
- `medication_intakes (medication_name_normalized)` joined through event/user;
- `jobs (status, available_at)` for queue claim;
- `auth_challenges (expires_at)` and `web_sessions (expires_at)` for cleanup.

Add GIN indexes to JSONB only after a real query requires them.

## 6. Migration strategy

- Numbered up/down SQL files for local development; prod uses only up.
- Application image includes migrations and a `migrate` command.
- Kubernetes initContainer runs the same immutable image with `migrate up`.
- Schema changes follow expand → backfill → switch read/write → contract.
- Backfills are explicit commands with dry-run/default limit, idempotency and progress metrics.
- Migration smoke test starts an empty PostgreSQL, runs all migrations, inserts a minimal user/event and rolls back only in test.

## 7. Retention and hard deletion

User hard deletion transactionally schedules:

1. revoke sessions/challenges;
2. delete exports;
3. delete health/events/revisions/extractions/raw entries;
4. remove Telegram identifiers or delete user;
5. write only a non-identifying deletion completion audit;
6. document backup expiry rather than claiming immediate removal from immutable backups.

Automatic raw-text retention cleanup is disabled at MVP launch. If enabled later, it must never delete normalized events or their minimal provenance timestamp/source type.
