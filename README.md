# Health Diary

Личный дневник здоровья с вводом через Telegram, структурированием записей через LLM и отдельной web/PWA-панелью для календаря, эпизодов головной боли, лекарств, активности и аналитики.

Реализован работающий MVP-контур: PostgreSQL schema/migrations, encrypted Telegram
ingest, Polza-compatible extraction, confirmation, web login/review, export and
deterministic basic analytics. Документы ниже остаются source of truth; до
production rollout ещё нужны infrastructure gates из `docs/DECISIONS.md`.

## Цель

Сделать ввод данных почти незаметным: пользователь пишет боту о самочувствии своими словами, подтверждает распознанные факты, а приложение сохраняет их в структуре, пригодной для календаря и поиска повторяющихся закономерностей.

Ключевой принцип: **LLM извлекает факты, но не считает статистику, не диагностирует и не назначает лечение**. Метрики и возможные ассоциации вычисляются детерминированно по подтверждённым данным.

## Документация

- [Product scope](docs/PRODUCT.md) — цели, пользовательские сценарии, MVP и acceptance criteria.
- [Architecture](docs/ARCHITECTURE.md) — компоненты, стек, потоки данных и структура репозитория.
- [Data model](docs/DATA_MODEL.md) — таблицы, типы событий, ограничения и миграционная стратегия.
- [Bot and LLM](docs/BOT_AND_LLM.md) — UX бота, JSON Schema, подтверждения и исправления.
- [API](docs/API.md) — HTTP-контракты, авторизация, ответы и ошибки.
- [Analytics](docs/ANALYTICS.md) — метрики, правила поиска ассоциаций и ограничения интерпретации.
- [Frontend and UX](docs/FRONTEND.md) — экраны, навигация, визуальная система, структура Vue-кода и порядок редизайна.
- [Security and privacy](docs/SECURITY.md) — чувствительные данные, Telegram/LLM boundaries и threat model.
- [Operations](docs/OPERATIONS.md) — local dev, env, GitLab CI, Flux, k3s, backup и monitoring.
- [Testing](docs/TESTING.md) — тестовая стратегия и обязательные проверки.
- [Implementation plan](docs/IMPLEMENTATION_PLAN.md) — этапы, файлы, зависимости и Definition of Done.
- [Decisions](docs/DECISIONS.md) — принятые решения, допущения и открытые вопросы.

## Планируемый developer experience

После реализации основной интерфейс проекта должен быть таким:

```bash
cp .env.example .env
make up
make migrate
make check
make down
```

`make up` должен поднимать приложение и PostgreSQL, применять безопасные forward migrations и показывать локальные URL. Секреты в Git не коммитятся.

## Статус

- [x] Проект и Git-репозиторий инициализированы.
- [x] Product/architecture/API/data/security/operations specification подготовлена.
- [x] Application scaffold, Docker Compose и required Make targets.
- [x] Forward PostgreSQL migrations, encryption, jobs and outbox.
- [x] Сквозной поток Telegram → LLM → durable confirmation → web review/export.
- [x] Unified calendar layers, city-level context periods and Open-Meteo weather enrichment.
- [x] GitLab checks/container build/publish configuration.
- [ ] Flux manifest, domain, registry access and encrypted-backup restore verification.

## Важное ограничение

Приложение является инструментом личного наблюдения и подготовки данных для обсуждения с врачом. Оно не должно позиционироваться как медицинское устройство, ставить диагноз или давать инструкции по началу/прекращению лечения.
