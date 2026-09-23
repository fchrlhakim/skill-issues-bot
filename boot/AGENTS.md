# Boot Agent Guide

## Boot Identity
`boot/` is the dependency wiring layer. It creates infrastructure clients, repositories, services, handlers, and cleanup callbacks for the HTTP application.

## Rules
- Keep `boot/boot.go` focused on construction and lifecycle only.
- Do not add business rules, request validation, or route definitions here.
- Wire modules in this order: infrastructure dependency, repository, service, HTTP handler.
- Any dependency that opens a connection or file-like resource must be closed in the returned cleanup function.
- If a new config field is required, update `infrastructure/config/config.go` and `.env.example` in the same change.
- Do not silently downgrade critical infrastructure failures. Database startup failures must return errors.
- Redis-backed optional features may fall back only when the config explicitly allows that behavior.

## Adding A Module
1. Create `modules/<feature>/handler.go`, `service.go`, and `repository.go`.
2. Add the handler field to `HandlerSetup`.
3. Instantiate repository, service, and handler in `MakeHandler`.
4. Return the handler in `HandlerSetup`.
5. Register routes in `router/router.go`.

## Current Wiring Map
- `user`: `user.NewRepository` -> `user.NewService` -> `user.NewHttp`.
- `auth`: `auth.NewRepository` -> `auth.NewService` -> `auth.NewHttp`.
- `health`: `health.NewRepository` -> `health.NewService` -> `health.NewHttp`.
- `audit-log`: `auditLog.NewRepository` -> `auditLog.NewService` -> `auditLog.NewHttp`.
- `upload`: `upload.NewRepository` + `storage.NewLocalStorage` -> `upload.NewService` -> `upload.NewHttp`.

## Gotchas
- Do not create global singletons for DB, Redis, token service, validator, or logger.
- Keep `HandlerSetup` explicit; avoid map-based dependency containers.
- If adding workers, ensure they receive context-aware dependencies and have a shutdown path.
- Avoid importing router packages into `boot/`; routing owns HTTP group registration.

## Pre-PR Checks
```bash
gofmt -w boot/*.go && go test ./... && go vet ./...
```
