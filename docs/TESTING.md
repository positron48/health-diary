# Testing strategy

## 1. Required layers

### Unit

- natural/relative time normalization;
- domain constraints and unknown/null semantics;
- episode state transitions;
- medication normalization aliases;
- auth token/code hashing and expiry;
- analytics formulas/gates;
- redaction helpers;
- retry classification/backoff.

### Repository/PostgreSQL integration

- all migrations from empty DB;
- Telegram update deduplication under concurrency;
- job claim with `SKIP LOCKED`;
- atomic challenge consume/attempt lock;
- user-scoped queries;
- batch confirmation transaction;
- correction/revision persistence;
- soft/hard deletion cascade;
- timezone calendar queries.

Use Testcontainers or a CI PostgreSQL service. SQLite is not an acceptable substitute.

### LLM contract

- JSON Schema validation fixtures;
- missing fields stay null;
- multiple events from one sentence;
- approximate/relative time;
- open episode linking and ambiguity;
- prompt injection text is treated as diary data;
- fenced/non-JSON/prose response rejection;
- invalid range/dose/timestamp rejection;
- provider timeout/429/5xx retry classification;
- correction changes only stated fields.

Routine tests use fixtures/fake provider. Live provider tests are opt-in and never required for ordinary CI.

### Bot handler

- allowlist and private-chat enforcement;
- commands bypass LLM;
- duplicate update is harmless;
- callback token/version validation;
- stale confirmation/correction callback;
- send failure creates/retries outbox;
- no raw text appears in logs.

### API

- auth lifecycle and cookie flags;
- rate/attempt limits;
- unauthenticated and cross-user behavior;
- calendar modes/timezone boundaries;
- optimistic revision conflict;
- pending exclusion from analytics;
- export ownership/expiry;
- source endpoint `Cache-Control: no-store`.

### Frontend

- login challenge/code flow;
- calendar mode rendering and accessible labels;
- unknown vs zero/no-data states;
- day timeline ordering;
- edit/delete/restore conflict handling;
- pending inbox;
- analytics insufficient-data state;
- mobile layouts and keyboard navigation.

### End-to-end

Critical E2E path with fake Telegram and fake LLM:

1. submit Telegram update;
2. verify raw durable entry/job;
3. process fixture extraction;
4. confirm callback;
5. verify calendar/day/episode API;
6. authenticate and render web view;
7. correct event and verify analytics changes;
8. delete and verify exclusion/export behavior.

## 2. Test fixtures

Use synthetic, clearly fake phrases. Maintain a Russian fixture corpus covering:

- exact and approximate time;
- yesterday/today/overnight;
- pain start/update/end;
- medication with/without dose;
- several events in one message;
- negation (“голова не болела”);
- correction (“не 400, а 200”);
- ambiguous episode;
- non-health conversation/note;
- malicious prompt-like content.

Fixtures contain no real user health history.

## 3. Privacy regression

Seed a unique marker such as `SENSITIVE_TEST_MARKER_9f...`, run ingest/extraction/API, then assert it is absent from:

- captured application logs;
- metrics output;
- tracing attributes;
- job payload/error fields;
- auth/audit rows where not expected.

It may exist only in the encrypted raw column and deliberately decrypted API response.

## 4. Security tests

- brute-force challenge locks exactly at limit;
- code/challenge/session token cannot be replayed;
- session revocation is immediate;
- CSRF/CORS policy;
- body size and webhook secret rejection;
- SQL injection/path traversal/malformed UUID handling;
- ciphertext tamper detection;
- key rotation reads old/writes new version;
- cross-user matrix for every resource endpoint;
- export URL cannot be guessed/reused after expiry.

## 5. Analytics fixtures

Golden datasets must cover:

- episode crossing midnight;
- explicit no-headache vs missing day;
- open episode duration excluded;
- missing medication effect;
- local-date boundary;
- deleted/pending/superseded data;
- association gates just below/at threshold;
- stable result ordering and formula version.

## 6. Performance targets

Initial targets, measured in local/CI reference environment:

- Telegram durable acknowledgement path p95 < 300 ms excluding Telegram network;
- calendar month API p95 < 300 ms for 5 years of one-user data;
- day timeline p95 < 200 ms;
- login verify p95 < 300 ms;
- worker can process at least 10 queued entries/minute despite provider latency through concurrency/bounded queue.

Provider latency is separately measured and must not block HTTP readiness.

## 7. `make check`

Must include at least:

```text
go formatting/tidy diff
go vet/lint
go test -race ./...
PostgreSQL migration/integration tests
frontend npm ci + typecheck + lint + test + build
OpenAPI/schema validation
secret/config scan
git diff --check
```

Expensive live LLM and full browser tests may be scheduled/manual, but critical fake-provider E2E is required for merge.

## 8. Manual pre-production checklist

- Telegram onboarding/privacy visible.
- Message confirmation/correction/delete usable on phone.
- Web login code received only by the bound allowlisted account.
- Calendar modes and unknown states visually checked.
- Source text endpoint not cached.
- Logs inspected for seeded sensitive marker.
- Backup and restore tested.
- Flux rollout and rollback tested.
- Data export and deletion exercised.
