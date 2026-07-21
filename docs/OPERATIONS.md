# Operations

## 1. Deployment model

- Application repository: GitLab, default branch `master`.
- Container registry: GitLab Container Registry.
- GitOps repository: existing `/Users/antonfilatov/www/my/k3s/devops-time-host`.
- Production cluster: existing k3s + Flux.
- Namespace/deployment/service: `health-diary`.
- PostgreSQL deployment: `health-diary-postgres` with RWO PVC.
- Production domain: TBD; do not hardcode placeholder as approved.

CI builds images; Flux deploys them. CI must not hold kubeconfig or run direct `kubectl apply`.

## 2. Local commands to implement

```bash
cp .env.example .env
make up          # app + postgres, build as needed
make migrate     # forward migrations
make logs        # follow app/postgres logs
make test        # unit + fast repository tests
make check       # CI-equivalent checks
make down        # stop without deleting data
make reset-db    # explicit dev-only destructive reset with guard
```

Recommended additional commands:

```bash
make web-dev
make test-postgres
make test-e2e
make lint
make build
make docker-build
```

`make reset-db` must target only the explicit Compose project/volume and print the target before deletion.

## 3. Compose design

Services:

- `postgres`: PostgreSQL 16, named volume, healthcheck, published locally on `15432` by default.
- `app`: application image, depends on healthy DB, long polling, published locally on `18080` by default (container port `8080`).

Optional profiles:

- `fake-llm` for deterministic manual tests;
- `obs` only if local Prometheus/Grafana is later useful.

No Redis in initial Compose.

## 4. Environment variables

`.env.example` is the canonical inventory. Configuration loader must:

- validate required values at startup;
- distinguish secret and non-secret values;
- reject invalid durations/timezones/URLs;
- never print secret values;
- require webhook URL/secret in webhook mode;
- require allowlist in production;
- require data encryption key in production;
- allow LLM disabled/fake mode for tests.

## 5. GitLab CI

Planned `.gitlab-ci.yml` stages:

1. `validate`
   - `go mod tidy` diff check;
   - formatting/lint;
   - frontend lockfile install, typecheck, lint;
   - migration file naming/checksum check.
2. `test`
   - Go unit/race tests;
   - PostgreSQL integration and migration smoke tests;
   - frontend Vitest;
   - API/LLM fixtures.
3. `build`
   - frontend production build;
   - Go binary build;
   - multi-stage container build.
4. `security`
   - Go/Node dependency audit;
   - secret scanning;
   - container/config scan.
5. `publish`
   - default branch only;
   - push `$CI_REGISTRY_IMAGE:$CI_COMMIT_SHA`;
   - update/push `latest` pointing to the same content;
   - emit image digest artifact.

Merge request pipelines never publish `latest`. Production publish requires protected branch and protected variables.

## 6. Docker image

One multi-stage Dockerfile:

1. Node stage: `npm ci`, test/typecheck in CI, `npm run build`.
2. Go stage: embed/copy web dist, build static Linux binary and migrate command.
3. Runtime stage: non-root user, CA certificates, timezone data, binary and migrations only.

Image exposes application and optional metrics ports, contains no source `.env`, Git metadata or build credentials.

## 7. GitOps files to add later

In `devops-time-host`:

```text
apps/health-diary/base/
├── namespace.yaml
├── configmap.yaml
├── deployment.yaml
├── service.yaml
├── ingress.yaml
├── postgres-deployment.yaml
├── postgres-service.yaml
├── postgres-pvc.yaml
├── kustomization.yaml
├── secret-placeholder.md
└── RELEASE_K3S.md
apps/health-diary/prod/kustomization.yaml
clusters/prod/infra/image-automation/health-diary-image.yaml
```

Also update:

- `clusters/prod/kustomization.yaml`;
- image-automation kustomization;
- `apps/k3s-backup/base/configmap.yaml`;
- observability/Alloy namespace filter if namespaces are explicitly listed.

## 8. Kubernetes workload

Application Deployment:

- replicas `1` initially;
- `revisionHistoryLimit: 2`;
- rolling update;
- non-root security context, read-only root filesystem if compatible;
- resource requests/limits based on measurement, initial small defaults;
- readiness `/readyz`, liveness `/healthz`;
- initContainer using the same app image: `health-diary migrate up`;
- ConfigMap for non-secret runtime configuration;
- Secret for credentials/keys.

PostgreSQL with RWO PVC follows existing workspace rule:

- rolling update with `maxSurge: 0`, `maxUnavailable: 1`;
- `pg_isready` probes;
- fixed user/database secret;
- restore procedure documented before production data.

Do not commit a Kubernetes Secret manifest with values.

## 9. Flux image automation

Follow existing `latest` digest pattern:

- `ImageRepository` points to GitLab registry image;
- registry credential is available to image-reflector-controller and namespace image pull;
- `ImagePolicy` filters `^latest$` with `digestReflectionPolicy: Always`;
- deployment image carries Flux setter comment;
- existing shared `ImageUpdateAutomation` commits digest to `devops-time-host main`;
- Flux applies the commit and rollout.

Verify GitLab registry auth from Flux before first release. Do not assume public registry behavior.

## 10. Secrets inventory

Secret `health-diary-secrets`:

- `DATABASE_URL` or DB password components;
- `TELEGRAM_BOT_TOKEN`;
- `TELEGRAM_WEBHOOK_SECRET`;
- `SESSION_SECRET`;
- `DATA_ENCRYPTION_KEY` and version;
- `LLM_API_KEY`;
- optional provider-specific credentials.

Non-secret ConfigMap:

- public URL, timezone, log level;
- bot mode/username/webhook URL;
- auth/session TTLs;
- LLM base URL/model/timeouts/provider-scoped proxy;
- retention flags;
- metrics/job configuration.

## 11. Backup and restore

Before production:

1. Add `pg_dump`/`pg_dumpall` capture to existing k3s backup CronJob.
2. Verify resulting archive encryption and remote retention.
3. Validate gzip/archive integrity in the backup job.
4. Perform a restore into a separate test namespace/database.
5. Run schema/application smoke checks against restored data.
6. Document RPO/RTO and deletion-vs-backup expiry.

Backup success is not established by a CronJob `Complete` status alone; dump size/integrity and restore must be checked.

## 12. Observability

Metrics without user/health labels:

- HTTP request count/duration by route template/status;
- Telegram updates accepted/duplicate/ignored;
- job queue depth, age, attempts and terminal failures;
- extraction latency/status/provider/model/token count;
- confirmation/correction counts;
- open episode count aggregate;
- DB pool metrics;
- outbox retry age.

Alerts:

- readiness unavailable;
- extraction terminal-failure spike;
- oldest queued job above threshold;
- webhook error spike;
- backup failure/missing expected artifact;
- PostgreSQL PVC/disk pressure.

Logs go to existing Loki/Alloy after verifying redaction with seeded markers.

## 13. Release verification

After Flux rollout:

```bash
kubectl -n health-diary get deploy,pods,svc,ingress,pvc
kubectl -n health-diary rollout status deploy/health-diary --timeout=180s
kubectl -n health-diary logs deploy/health-diary --tail=200
kubectl -n health-diary exec deploy/health-diary -- /app/health-diary migrate status
curl -fsS https://REPLACE_DOMAIN/healthz
```

Then perform a synthetic non-sensitive flow with a dedicated test phrase and verify it is absent from logs.

## 14. Rollback

- Revert Flux-managed image digest to previous known image commit.
- Forward-compatible migrations must allow previous application version during the rollout window.
- Never run migration down automatically in production.
- If schema is incompatible, follow the explicit migration rollback/roll-forward plan written with that migration.
- Telegram capture degradation should prefer storing raw entries over losing messages; disable LLM via config if provider causes incidents.
