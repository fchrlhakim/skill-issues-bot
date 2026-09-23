# Skill Issues Bot Agent Guide

## Project Snapshot

- This repo is the Skill Issues Discord marketplace bot: Gin HTTP plus a discordgo gateway. Domain code is `modules/ticket`, `modules/membership`, and `modules/discordbot`. `infrastructure/` stays free of those domain rules.
- The bot calls skill-issues-saas in exactly one place: an optional, read-only `GET` of the SaaS admin finance overview, used by `/revenue` (`SAAS_REVENUE_URL`, `SAAS_REVENUE_TOKEN` in `modules/discordbot/bot.go`). Keep it read-only and optional: with either variable empty, `/revenue` must report `SaaS revenue: unavailable` rather than fail. Never share a database with the SaaS. Do not add a client to skill-issues-proxy, and do not widen this into writes, shared secrets, or a shared store.
- Two other Discord features exist in the SaaS repo (`sellerkyc`, `identity`) and do not share `guild_members`. Do not treat them as this bot's store.
- Gateway: `modules/discordbot`. Business rules for tickets and withdrawals: `modules/ticket` (`catalog.go`, `workflow.go`, `withdrawal.go`, `safety.go`). Tiers: `modules/membership`. HTTP wiring: `boot/boot.go`, `router/router.go`. Product overview for humans: `README.md`. Deploy: `DEPLOY.md`.

## Non-Negotiable Rules
- Preserve the existing architecture: `main.go`, `boot/`, `router/`, `infrastructure/`, `modules/`, `modules/primitive/`, `utils/`, `migrations/`.
- Do not invent a new structure such as `cmd/`, `internal/`, `pkg/`, or framework-specific folders unless explicitly requested.
- Do not add domain logic into `infrastructure/`. Infrastructure must stay reusable.
- Do not put database calls in handlers. Use `handler -> service -> repository`.
- Do not return DB models directly when data can be sensitive. Use response DTOs from `modules/primitive/response.go`.
- Do not commit secrets. Use `.env.example` for placeholders only.
- Do not add public static file serving for uploads.
- Do not use raw dynamic SQL identifiers. Sort/filter identifiers must be allowlisted.
- Do not weaken security defaults to make local development easier.
- You may change code only when it follows the established pattern or is a clear production best-practice improvement.
- Do not let the bot transfer funds, mint a balance, or mark a withdrawal `paid` except through `RecordWithdrawalPayment` / `/withdraw-paid`.
- Discord command visibility is not authorization. Admin slash commands recheck `IsTicketAdmin`.
- `/rolesync` and `/guildsync` create missing launch objects only. Do not make them delete or overwrite.

## Root Commands
```bash
make run
make test
make tidy
make build
make docker-up
make docker-down
go test ./...
go vet ./...
```

## Definition Of Done
- `gofmt` applied to every changed `.go` file.
- `go mod tidy` run after dependency changes.
- `go test ./...` passes.
- `go vet ./...` passes for production-impacting changes.
- New DB fields/tables have migrations.
- New routes are wired through `boot/boot.go` and `router/router.go`.
- Security-sensitive endpoints are protected with auth and limiter middleware.

## Security & Privacy
- Never log passwords, tokens, NIK, medical records, patient data, payment secrets, or raw request bodies.
- Audit logs should record metadata only: actor, action, resource, IP, user-agent, request ID, status.
- Use bcrypt via `utils/password.go` for passwords.
- Use JWT service from `infrastructure/jwt/`; do not create ad-hoc JWT parsing.
- Uploads must go through `modules/upload` and `infrastructure/storage`; never serve local files directly.

## JIT Index
- Entrypoint: `main.go`
- Dependency wiring: `boot/boot.go`
- Route registration: `router/router.go`
- Shared response/pagination: `infrastructure/httplib/`
- Middleware: `infrastructure/middleware/`
- JWT: `infrastructure/jwt/`
- Limiter: `infrastructure/limiter/`
- Metrics: `infrastructure/observability/`
- Cache: `infrastructure/cache/`
- Outbox: `infrastructure/worker/`
- Module contracts: `modules/*/handler.go`, `modules/*/service.go`, `modules/*/repository.go`
- Shared DTO/model/constants: `modules/primitive/`
- Migrations: `migrations/*.sql`
- Discord gateway: `modules/discordbot/`
- Ticket rules: `modules/ticket/`
- Membership tiers: `modules/membership/`

## Quick Find
```bash
rg -n "type .*Interface" modules infrastructure
rg -n "func New.*" modules infrastructure boot router
rg -n "Group.*\(" modules router
rg -n "WithContext\(ctx\)" modules
rg -n "GetCursorPaginationFromCtx|GetPaginationFromCtx" .
rg -n "AuditLog|audit_logs" .
```

## Sub-Guides
- Module development: `modules/AGENTS.md`
- Infrastructure development: `infrastructure/AGENTS.md`
- Database migrations: `migrations/AGENTS.md`
