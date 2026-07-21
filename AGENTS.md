# AGENTS.md

## Scope

Инструкции применяются ко всему репозиторию `health-diary`.

## Source of truth

До начала реализации прочитать:

1. `README.md`
2. `docs/DECISIONS.md`
3. `docs/PRODUCT.md`
4. `docs/ARCHITECTURE.md`
5. документ конкретной подсистемы
6. `docs/IMPLEMENTATION_PLAN.md`

Если реализация меняет архитектурное решение, API, схему, env, deploy или privacy boundary, сначала обновить соответствующий документ и `docs/DECISIONS.md`.

## Non-negotiable rules

- LLM используется для extraction/summarization, но не является source of truth для метрик, диагнозов или рекомендаций.
- В аналитику попадают только подтверждённые и не удалённые события.
- Исходное сообщение сохраняется отдельно от извлечённых фактов; один message может породить несколько events.
- Не заполнять отсутствующие время, дозу, интенсивность или эффект догадками. Неизвестное значение хранится как `NULL`.
- Все timestamps в БД — `TIMESTAMPTZ` в UTC; пользовательский timezone хранится отдельно.
- Telegram update/message обрабатываются идемпотентно.
- Содержимое health entries, LLM prompts/responses и OTP никогда не пишется в application logs.
- Auth code хранится только в виде hash, одноразовый, с TTL и лимитом попыток.
- Web session — server-side, cookie `Secure`, `HttpOnly`, `SameSite=Lax` или строже.
- Секреты не находятся в Git, ConfigMap или Docker image.
- В prod deploy идёт только через GitLab image build и Flux GitOps; CI не выполняет прямой `kubectl apply`.
- Миграции forward-only для prod; destructive migration требует отдельного backfill/rollout plan.
- Любая функция удаления должна учитывать raw entries, derived events, revisions, exports и backups/retention.

## Intended stack

- Go application: Telegram bot, REST API, background jobs, embedded Vue build.
- PostgreSQL 16, `pgx`, `sqlc`, `golang-migrate`.
- Vue 3, Vite, TypeScript, Vue Router, Chart.js, Lucide.
- OpenAI-compatible LLM adapter with strict JSON Schema and provider isolation.
- Docker Compose locally; GitLab CI + GitLab Container Registry; Flux + k3s in production.

Do not introduce Redis, Kafka, gRPC, Envoy, Kubernetes operators or extra deployable services without evidence that PostgreSQL-backed jobs and the monolith are insufficient.

## Required command surface

The final repository must provide:

```bash
make up
make down
make logs
make migrate
make test
make check
make reset-db
```

`make check` must be the local equivalent of required CI checks.

## Completion discipline

- Implement phases in `docs/IMPLEMENTATION_PLAN.md` order unless the plan is updated first.
- Every phase ends with automated tests and a runnable verification path.
- Keep `README.md` status and `docs/OPERATIONS.md` exact as commands/env change.
- Significant operational knowledge must be written here or linked from here.
