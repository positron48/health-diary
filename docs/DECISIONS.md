# Architecture decisions and open questions

This is a lightweight ADR registry. Update status and rationale before changing an accepted decision.

## Accepted

### ADR-001: Standalone repository

- Status: accepted
- Decision: project lives in `/Users/antonfilatov/www/my/k3s/health-diary` and has its own Git repository.
- Reason: independent release lifecycle and sensitive domain boundaries.

### ADR-002: Modular Go monolith

- Status: accepted
- Decision: one Go service contains Telegram ingress, REST, worker, analytics and embedded Vue assets.
- Reason: simplest reliable operational model for initial load.

### ADR-003: PostgreSQL is the only stateful dependency

- Status: accepted
- Decision: PostgreSQL stores domain data, sessions, jobs and outbox state.
- Reason: avoids Redis/broker operational cost while retaining transactions and safe worker claims.

### ADR-004: LLM extraction, deterministic analytics

- Status: accepted
- Decision: LLM turns natural language into validated candidate events. SQL/Go calculates all metrics and associations.
- Reason: reproducibility, testability and safety.

### ADR-005: Raw source and normalized facts are separate

- Status: accepted
- Decision: immutable source entry, versioned extraction runs and revisable normalized events are different records.
- Reason: reprocessing, auditing, parser upgrades and user correction.

### ADR-006: Server-side web sessions

- Status: accepted
- Decision: one-time Telegram code creates a revocable server-side session in an HttpOnly cookie; no browser-stored JWT.
- Reason: simple revocation and smaller token exposure surface.

### ADR-007: GitLab application CI, existing Flux GitOps

- Status: accepted
- Decision: GitLab CI tests/builds and pushes immutable image plus `latest`; Flux tracks the digest and updates `devops-time-host`.
- Reason: preserves the existing k3s deployment model without giving CI direct cluster credentials.

### ADR-008: Single-user-first, multi-user schema

- Status: accepted
- Decision: production initially allows configured Telegram IDs, while every domain record remains scoped by internal `user_id`.
- Reason: secure first release without a future data-model rewrite.

### ADR-009: Russian-first PWA

- Status: accepted
- Decision: Vue responsive web/PWA, Russian UI and `Europe/Moscow` default timezone.
- Reason: matches the initial user and avoids premature native application work.

### ADR-010: Confirmation before analytics

- Status: accepted
- Decision: extracted events are pending until user confirmation; only confirmed events affect analytics.
- Reason: LLM extraction can be wrong and health statistics must be correctable.

### ADR-D01: LLM provider

- Status: accepted
- Decision: use Polza.ai through the OpenAI-compatible adapter, default model `openai/gpt-4o-mini-2024-07-18`.
- Constraint: send only the diary text and the minimal de-identified extraction context; never send Telegram identity, username, chat/message IDs, IP or session data. The adapter remains replaceable.
- Reason: matches the neighbouring English project production model/provider choice while retaining provider isolation and strict schema validation.

## Deferred

### ADR-D02: Production domain

- Status: deferred
- Required before GitOps phase.
- Placeholder: `health.qantrix.ru` is not approved and must not be committed as final without confirmation.

### ADR-D03: Raw entry retention

- Status: accepted.
- Decision: do not automatically delete raw entries or normalized health data. Retention cleanup for raw entries is disabled by default; explicit user-requested deletion remains available.
- Reason: owner preference at MVP launch. Backup expiry is still disclosed honestly.

### ADR-D04: Backup encryption verification

- Status: deferred.
- Existing k3s backup integration is the target, but encryption of the remote/archive must be verified before adding health data.

### ADR-D05: Informational medication-overuse indicators

- Status: deferred beyond MVP.
- Must be based on medication class, observation window and clinician-reviewed wording. It must never instruct medication withdrawal.

### ADR-D06: Safety/red-flag messages

- Status: deferred beyond MVP.
- Any implementation requires a separately reviewed rule source, explicit limitations and tests for wording. Absence of a warning must never imply safety.

## Open product questions

1. Final project/product name and Telegram bot username.
2. Production domain.
3. Whether explicit “no headache today” check-ins should be prompted.
4. Preferred morning/evening reminder schedule, if any.
5. Whether menstrual-cycle tracking is relevant and should be opt-in.
6. Whether the first export must include a printable PDF or CSV/JSON is sufficient.

None of these questions blocks phases 0–2 if defaults remain configuration-driven.
