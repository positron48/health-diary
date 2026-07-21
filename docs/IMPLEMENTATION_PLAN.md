# Implementation plan

Status: Phases 0–3 have implemented vertical slices; Phase 5 has implemented
Telegram-code session basics; Phase 6 has a minimal responsive review UI;
Phase 7 has conservative confirmed-event summary/export. The remaining detailed
items in these phases and all production external gates remain open until their
tests and operational verification are completed.

Scope: first production-capable MVP described in `PRODUCT.md`.

Estimate: 10–14 focused engineering days plus review/owner decisions.

## 0. Rules for the implementer

Before coding:

1. Read `AGENTS.md`, `DECISIONS.md`, `PRODUCT.md`, `ARCHITECTURE.md`.
2. Treat this document as ordered work; update docs before changing scope.
3. Do not add production secrets, create external repositories/bots/domains or modify `devops-time-host` until the owner approves those external changes.
4. Keep each phase runnable and tested.
5. Use fake Telegram/LLM adapters for deterministic development until credentials are supplied.

## Phase 0 — application scaffold and local runtime

Goal: empty application starts locally and CI-equivalent checks have a stable command surface.

Create:

```text
go.mod
cmd/server/main.go
cmd/migrate/main.go
internal/app/
internal/config/
internal/database/
internal/httpapi/
web/
Dockerfile
docker-compose.yml
Makefile
sqlc.yaml
.gitlab-ci.yml
```

Tasks:

- Select/pin current Go and Node versions compatible with runners.
- Implement typed config loading/validation from `.env.example`.
- Add structured logging and request IDs with redaction policy.
- Add `/healthz`, `/readyz`, `/metrics`.
- Scaffold Vue/Vite/TypeScript and embed production assets in Go.
- Compose app + PostgreSQL 16 with healthchecks and named volume.
- Implement required Make targets.
- Add initial GitLab validate/test/build jobs without publish credentials.

Tests/verification:

```bash
cp .env.example .env
make up
curl -fsS http://localhost:18080/healthz
make check
make down
```

Definition of Done:

- App starts with LLM/Telegram fake or disabled mode.
- Web shell loads.
- Readiness fails when DB is unavailable and recovers.
- CI jobs match `make check` components.

Estimate: 1–1.5 days.

## Phase 1 — schema, repositories, encryption and job queue

Goal: durable, encrypted, user-scoped storage and PostgreSQL-backed jobs.

Create packages/files:

```text
migrations/000001_core.up.sql
migrations/000001_core.down.sql
migrations/000002_health_events.up.sql
migrations/000002_health_events.down.sql
queries/*.sql
internal/crypto/
internal/journal/
internal/jobs/
internal/testutil/
```

Tasks:

- Implement tables from `DATA_MODEL.md` in logical migrations.
- Generate typed queries with `sqlc`.
- Implement application-level authenticated encryption with key version.
- Add user/entry/update/batch/event repositories.
- Add job claim/retry/terminal failure lifecycle with `SKIP LOCKED`.
- Add outbox table/service.
- Add cleanup jobs for auth/session/export later without enabling them yet.

Tests:

- migrate empty DB;
- concurrent duplicate Telegram update insert;
- encryption round-trip/tamper/key-version tests;
- concurrent job claim exactly once;
- every repository query cross-user test;
- transaction rollback leaves no partial entry/job.

Definition of Done:

- A synthetic entry is encrypted at rest and can be decrypted only through service code.
- Duplicate update cannot create duplicate source entry.
- Worker safely handles multiple instances.

Estimate: 1.5–2 days.

## Phase 2 — Telegram capture and confirmation shell

Goal: accept text, retain it, create a fake parsed batch and confirm/reject it.

Create:

```text
internal/bot/client.go
internal/bot/handler.go
internal/bot/commands.go
internal/bot/callbacks.go
internal/ingest/service.go
internal/jobs/worker.go
```

Tasks:

- Long polling in local mode, webhook handler in production mode.
- Private chat + allowlist enforcement before storage.
- `/start`, `/help`, `/today`, `/privacy`.
- Durable message acknowledgement path.
- Signed/opaque callback token mapping.
- Confirmation/reject/correction state machine.
- Outbox retry for bot messages.
- Fake extractor that returns deterministic events for tests.

Tests:

- allowlisted/unknown/group chat cases;
- duplicate delivery;
- stale/forged callback;
- confirmation atomicity;
- bot send failure/outbox retry;
- privacy log marker scan.

Definition of Done:

- Synthetic Telegram update becomes an encrypted entry and pending batch.
- Confirmation marks it eligible for reads; reject excludes it.
- No real LLM dependency yet.

Estimate: 1–1.5 days.

## Phase 3 — LLM extraction adapter

Provider decision: Polza.ai via the OpenAI-compatible adapter, default model `openai/gpt-5.4-nano`; health text is sent only under the documented de-identified privacy boundary. Development may proceed with fake/live sandbox.

Goal: transform Russian free text into validated candidate events.

Create:

```text
internal/llm/provider.go
internal/llm/openai_compatible.go
internal/llm/schema/health-entry-v1.json
internal/llm/extractor.go
internal/llm/validator.go
internal/llm/testdata/*.json
prompts/health-entry-v1.txt
```

Tasks:

- Provider-neutral interface and isolated optional SOCKS5 transport.
- Strict recursive JSON Schema (`additionalProperties: false`).
- Prompt rules from `BOT_AND_LLM.md`.
- Minimal open-episode and medication-alias context.
- Schema + domain validation; atomic failure.
- Retry categorization with bounded repair attempt for invalid response.
- Persist validated result and usage metadata; raw provider response off by default.
- Bot renders exact vs approximate time and multiple events.

Tests:

- fixture corpus from `TESTING.md`;
- multi-event input;
- missing/null values;
- relative time around midnight;
- injection-like diary input;
- timeout/429/5xx/invalid JSON;
- no identity in provider request.

Definition of Done:

- Example pain + ibuprofen message creates two valid pending events.
- Parser cannot invent omitted dose/time through backend defaults.
- Provider outage preserves entry and exposes retry.

Estimate: 1.5–2 days.

## Phase 4 — episode lifecycle and corrections

Goal: represent how a headache develops across messages.

Create:

```text
internal/episode/service.go
internal/episode/matcher.go
internal/journal/correction.go
queries/episodes.sql
```

Tasks:

- Start/update/end episode projection.
- Link medication intake to explicit/unambiguous episode.
- Clarification when multiple/no open episodes make action ambiguous.
- Natural-language correction creates superseding batch.
- Web/manual correction uses same domain validator.
- Preserve revision history and optimistic version.
- Recalculate episode max intensity/duration after correction/delete.

Tests:

- full start → update → medication → end path;
- overnight episode;
- ambiguous multiple open episodes;
- correction of dose/time/intensity;
- delete/restore observation and episode projection;
- stale callback/revision.

Definition of Done:

- Episode detail is consistent after confirmation, correction and deletion.
- No negative or fabricated duration.

Estimate: 1 day.

## Phase 5 — Telegram code auth and REST API

Goal: authenticated API with exact contracts in `API.md`.

Create:

```text
internal/auth/challenge.go
internal/auth/session.go
internal/httpapi/auth.go
internal/httpapi/calendar.go
internal/httpapi/events.go
internal/httpapi/episodes.go
internal/httpapi/settings.go
openapi/health-diary.yaml
```

Tasks:

- Opaque web challenge and Telegram deep-link binding.
- Hashed 6-digit OTP, expiry, atomic attempts/consume.
- Server-side hashed sessions and secure cookie.
- CSRF/CORS/security headers.
- Auth/session, calendar, day, event, batch and episode endpoints.
- User scope at repository boundary.
- OpenAPI contract and validation.
- Source-text endpoint with `Cache-Control: no-store` and audit.

Tests:

- auth happy/expired/wrong/locked/replayed flows;
- cookie flags and session revocation;
- forged/expired deep link;
- cross-user matrix;
- API schema and error envelope;
- timezone calendar boundary and revision conflict.

Definition of Done:

- User opens bot deep link, receives code, logs into site and reads only their data.
- One-time credentials cannot be replayed.

Estimate: 1–1.5 days.

## Phase 6 — Vue PWA calendar and editing

Goal: usable mobile-first review interface.

Create views/components approximately:

```text
web/src/views/LoginView.vue
web/src/views/CalendarView.vue
web/src/views/DayView.vue
web/src/views/EpisodeView.vue
web/src/views/PendingView.vue
web/src/views/SettingsView.vue
web/src/components/calendar/
web/src/components/timeline/
web/src/components/events/
web/src/api/
```

Tasks:

- Login challenge/code UX.
- Month calendar and six modes.
- Accessible icons/color/unknown state.
- Day timeline and episode detail.
- Pending batch confirm/reject/correct.
- Manual edit/delete/restore with revision conflicts.
- Responsive layout and application shell PWA.
- `no-store` health fetches; service worker caches static shell only.

Tests:

- component tests for all states;
- mobile viewport visual/manual QA;
- keyboard/focus/accessible labels;
- conflict and expired-session UX;
- unknown/no-data/no-headache distinction.

Definition of Done:

- Calendar is usable on phone and all confirmed data can be reviewed/corrected.
- Switching mode preserves month/scroll where appropriate.

Estimate: 2–2.5 days.

## Phase 7 — deterministic analytics and export

Goal: useful but conservative summaries.

Create:

```text
internal/analytics/coverage.go
internal/analytics/headache.go
internal/analytics/medication.go
internal/analytics/associations.go
internal/export/
internal/httpapi/analytics.go
internal/httpapi/export.go
web/src/views/AnalyticsView.vue
```

Tasks:

- Coverage/headache/medication/sleep/activity/wellbeing metrics.
- 7/30/60/90-day range selector.
- Association gates and transparent counts.
- Formula/version metadata.
- CSV and JSON async export with expiry.
- Analytics UI with insufficient-data states and disclaimers.
- Optional LLM wording remains off unless aggregate-only summary is explicitly added/tested.

Tests:

- golden datasets from `ANALYTICS.md`/`TESTING.md`;
- exact range/timezone semantics;
- missingness and denominator display;
- export ownership/content/expiry;
- deleted/pending exclusion.

Definition of Done:

- All numbers reproduce from fixture SQL/Go without LLM.
- Possible associations never appear before gates pass.

Estimate: 1.5–2 days.

## Phase 8 — production CI, GitOps, backup and observability

External-change gate: owner supplies/approves GitLab project, registry, bot, domain and production secrets.

Application repo tasks:

- Complete protected GitLab publish stage.
- Build/push `$CI_COMMIT_SHA` and `latest` same digest.
- Container/security scan and SBOM/artifact as supported.
- Production Docker config and migration command.
- `RELEASE.md` or link to GitOps runbook.

`devops-time-host` tasks:

- Add files listed in `OPERATIONS.md`.
- Add private GitLab registry credentials procedure.
- Add Flux ImageRepository/Policy and setter.
- Add namespace/config/deploy/service/ingress/Postgres/PVC.
- Add secret placeholder/runbook only, no values.
- Add backup dump/integrity/restore documentation.
- Add namespace to log collection and relevant alerts.

Verification:

- app pipeline publishes image;
- Flux sees new digest, commits update and rolls deployment;
- webhook health and test capture work;
- logs contain no seeded sensitive marker;
- backup artifact validates and restores in test namespace;
- previous image rollback works without schema rollback.

Estimate: 1–1.5 days, excluding external provisioning/wait time.

## Phase 9 — privacy completion and production readiness

Owner gate: verify ADR-D04 backup encryption before production.

Tasks:

- Settings/privacy text with exact provider/retention.
- Export/delete UI and re-authenticated hard deletion job.
- Cleanup schedules for auth/session/export/raw retention.
- Session/data/LLM/bot key rotation runbook.
- Security headers/CSP and dependency scans.
- End-to-end privacy regression.
- Production readiness checklist and known limitations.

Definition of Done:

- Export/delete/retention behavior matches documentation.
- Restore and rotation are tested, not merely described.
- Owner approves privacy wording and provider boundary.

Estimate: 1 day.

## Post-MVP backlog, in recommended order

1. Reminder to close an open episode.
2. Follow-up after medication to record effect.
3. Morning/evening check-ins, including explicit no-headache days.
4. Voice transcription with separate privacy boundary.
5. Printable 8-week PDF report.
6. Fixed-location weather enrichment.
7. Temporary clinician read-only report link.
8. Apple Health/Health Connect bridge/native companion.
9. Reviewed medication-overuse informational indicator.
10. Reviewed safety-signal messaging.

Do not pull these into MVP without updating `PRODUCT.md`, estimates and privacy/security review.

## Final release acceptance checklist

- [ ] All MVP acceptance criteria in `PRODUCT.md` pass.
- [ ] `make check` equals required GitLab checks and passes cleanly.
- [ ] Empty DB migrations and restore migrations pass.
- [ ] Telegram → extraction → confirmation → calendar E2E passes.
- [ ] Correction/delete immediately changes analytics.
- [ ] Auth replay/brute-force/cross-user tests pass.
- [ ] Sensitive marker absent from logs/metrics/traces.
- [ ] Production provider/retention/domain decisions recorded.
- [ ] GitLab image digest deployed only through Flux.
- [ ] Backup integrity and restore tested.
- [ ] Rollback tested.
- [ ] Export and hard deletion tested.
- [ ] Privacy/medical limitations visible in bot and web.

## Expected delivery shape

Prefer small, reviewable commits by phase. A phase is not complete if only happy-path code exists without migrations/tests/docs. The first demonstrable vertical slice should be available after phase 3: text message → validated candidate events → confirmation → persisted confirmed data. The first end-user UI is complete after phase 6.
