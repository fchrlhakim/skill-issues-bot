# Infrastructure Agent Guide

## Infrastructure Identity
Infrastructure packages are reusable technical capabilities. They must not depend on business modules except middleware audit interfaces and primitive audit model where already established.

## Current Packages
- `authz/`: role and ownership helpers.
- `cache/`: cache interface, no-op cache, Redis cache.
- `client/http/`: context-aware HTTP client with retry and circuit breaker.
- `config/`: env-based config loading and validation.
- `database/`: PostgreSQL/GORM client and SQL DB lifecycle.
- `httplib/`: response helpers and pagination helpers.
- `jwt/`: JWT v5 token service.
- `limiter/`: key-based local limiter and Redis limiter.
- `log/`: logrus setup.
- `middleware/`: request ID, recovery, access log, auth, audit, body limit, limiter.
- `observability/`: Prometheus HTTP and DB metrics.
- `redis/`: Redis client lifecycle.
- `storage/`: storage interface and local implementation.
- `transaction/`: GORM transaction helper.
- `validator/`: validator wrapper.
- `worker/`: outbox event and worker skeleton.

## Rules
- Keep infra generic. Do not put patient, product, order, or other domain logic here.
- Config changes must be reflected in `.env.example`.
- Dependencies created in `boot/boot.go` must have cleanup when they hold connections.
- Middleware should return sanitized client errors and log detailed errors server-side.
- Metrics must not expose secrets, tokens, patient data, or raw payloads.
- HTTP clients must use context and timeout.
- Redis-backed features must degrade intentionally or fail fast; do not silently ignore critical Redis errors.

## Examples
- Add new middleware by following `infrastructure/middleware/middleware.go`.
- Add new config field in `infrastructure/config/config.go` and `.env.example`.
- Add new reusable external service client under `infrastructure/client/<service>/`.
- Add cache usage through `infrastructure/cache.Cache` rather than direct Redis calls in modules.

## Anti-Patterns
- Do not import `modules/user`, `modules/patient`, or feature services into infrastructure packages.
- Do not create global mutable clients.
- Do not add hardcoded DSNs, JWT secrets, Redis addresses, or API keys.
- Do not change `jwt` signing behavior without updating tests in `infrastructure/jwt/jwt_test.go`.

## Pre-PR Checks
```bash
gofmt -w infrastructure/**/*.go && go test ./... && go vet ./...
```
