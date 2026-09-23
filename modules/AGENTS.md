# Modules Agent Guide

## Module Identity
Feature modules implement application/domain behavior. Every real module should follow the `gin-be-dashnet-app` pattern: `handler.go`, `service.go`, `repository.go`.
Ticket and membership rules stay in those packages; `discordbot` is the gateway adapter and calls the ticket and membership services.

## Required Pattern
```text
modules/<feature>/
├── handler.go      HTTP adapter: bind, validate, call service, response
├── service.go      business logic, authorization, orchestration, transaction
└── repository.go   database access with GORM only
```

Examples to copy:
- Auth: `modules/auth/handler.go`, `modules/auth/service.go`, `modules/auth/repository.go`
- User CRUD/list pattern: `modules/user/handler.go`, `modules/user/service.go`, `modules/user/repository.go`
- Audit log pattern: `modules/audit-log/`
- Upload pattern: `modules/upload/`

## Primitive Rules
- Add DB models to `modules/primitive/model.go`.
- Add request DTOs to `modules/primitive/request.go`.
- Add response DTOs and mapper functions to `modules/primitive/response.go`.
- Add shared messages/status constants to `modules/primitive/constant.go`.
- Do not create random DTO files inside feature modules unless there is a strong reason.

## Handler Rules
- Bind request using Gin in `handler.go` only.
- Validate request using `infrastructure/validator`.
- Use `httplib.SetSuccessResponse`, `SetCreatedResponse`, or `SetErrorResponse`.
- Do not call GORM directly from handlers.
- Protected routes must be registered in `router/router.go` with `AuthMiddleware` and route-specific limiter.

## Service Rules
- Put business rules, authorization, and orchestration in `service.go`.
- Use `infrastructure/authz` for ownership/role checks.
- Use `infrastructure/transaction.WithTransaction` for multi-table writes.
- Use `infrastructure/cache` for cache-aside patterns.
- Do not leak raw repository/database errors to handlers if they expose internals.

## Repository Rules
- Use `db.WithContext(ctx)` for every query.
- Keep repository methods focused on DB access only.
- Use parameterized conditions.
- For list endpoints, prefer keyset pagination. See `modules/user/repository.go`.
- Never concatenate user-controlled column names. Use `httplib.GetPaginationFromCtx` allowlists when offset pagination is needed.

## Adding A New CRUD Module
1. Create `modules/<feature>/handler.go`, `service.go`, `repository.go`.
2. Add model/request/response in `modules/primitive/`.
3. Add SQL migration in `migrations/`.
4. Wire repository/service/http in `boot/boot.go`.
5. Register route group in `router/router.go`.
6. Add auth, route limiter, audit awareness, and tests as needed.
7. Run `gofmt`, `go test ./...`, `go vet ./...`.

## Common Gotchas
- `modules/auth` owns token login/refresh/logout flow; do not duplicate auth logic in other modules.
- Audit logging is global middleware; do not manually log sensitive payloads.
- Uploads must use `modules/upload`; do not add public file serving.
- Use response DTOs for patient/medical/government data.

## Pre-PR Checks
```bash
gofmt -w modules/**/*.go && go test ./... && go vet ./...
```
