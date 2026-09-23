# Go Starter Kit

Starter kit ini dibuat berdasarkan pola `gin-be-dashnet-app`, tetapi dibersihkan dari masalah utama yang ditemukan saat audit: secret hardcoded, JWT key init terlalu awal, public upload, dynamic SQL sort tanpa whitelist, global config mutable, weak password hash, dan server tanpa timeout.

## Fitur

- Gin HTTP server dengan graceful shutdown dan timeout.
- Config berbasis environment variable dan `.env`, tanpa secrets di source code.
- PostgreSQL via GORM dengan lifecycle cleanup.
- JWT access/refresh token memakai `github.com/golang-jwt/jwt/v5`.
- Password hashing memakai bcrypt.
- Response helper konsisten.
- Request ID, security headers, CORS explicit, dan rate limiter.
- Redis-ready limiter untuk multi-instance deployment.
- Prometheus metrics middleware di `/api/v1/metrics`.
- Refresh token rotation dengan hashed token di database.
- Audit log module dan middleware untuk mencatat akses resource sensitif.
- HTTP client, transaction helper, RBAC helper, storage abstraction, dan outbox skeleton.
- Module contoh: `auth`, `users`, `health`.
- Migration awal `users` dengan primary key, unique index, dan soft delete.
- Tests untuk token dan pagination whitelist.

## Struktur

```text
main.go                         entrypoint seperti gin-be-dashnet-app
boot/boot.go                    composition root / dependency injection
router/router.go                Gin route grouping dan global middleware
infrastructure/config/          env config + validation
infrastructure/database/        postgres client
infrastructure/httplib/         response dan pagination helpers
infrastructure/jwt/             JWT service
infrastructure/log/             logrus setup
infrastructure/middleware/      request id, auth, rate limit, security headers
infrastructure/validator/       validator wrapper
infrastructure/redis/           redis client
infrastructure/observability/   prometheus metrics
infrastructure/authz/           role/ownership helper
infrastructure/transaction/     transaction helper
infrastructure/storage/         storage abstraction
infrastructure/worker/          outbox skeleton
modules/                        feature modules handler/service/repository
modules/primitive/              shared request/model DTO
utils/                          shared utilities
migrations/                     SQL migrations
```

## Pola Module

Setiap module mengikuti pola `gin-be-dashnet-app`:

```text
modules/<feature>/
├── handler.go      HTTP adapter, bind request, call service, format response
├── service.go      business logic dan orchestration
└── repository.go   database access dengan GORM
```

Untuk module yang tidak membutuhkan database langsung, `repository.go` boleh tidak ada. Untuk starter ini, `health` tetap dibuat lengkap agar pola `repository -> service -> handler` jelas.

## Audit Log

Audit log aktif sebagai middleware global dan melewati endpoint health/metrics. Data yang dicatat:

- actor ID jika request authenticated
- action berdasarkan method HTTP
- resource type dari route
- resource ID dari `:id` jika ada
- method, path, status
- IP address dan user-agent
- request ID
- metadata non-sensitive seperti query string

Endpoint baca audit log tersedia di:

```text
GET /api/v1/audit-logs
```

Endpoint ini protected dengan JWT dan memakai cursor pagination.

## Quick Start

1. Copy env example:

```bash
cp .env.example .env
```

2. Ubah `JWT_SECRET` menjadi minimal 32 karakter acak.

3. Jalankan Postgres:

```bash
make docker-up
```

4. Jalankan migration dengan tool pilihan, contoh `golang-migrate`:

```bash
migrate -path migrations -database "postgres://starter:starter@localhost:5432/starter?sslmode=disable" up
```

5. Jalankan app:

```bash
make run
```

## Discord marketplace (ported from skillissue-discord)

Business rules live in Go — not in the Node ticket store:

- Exclusive tiers User → Buyer (Verify) → Seller (admin `/seller-approve`)
- 17 ticket types, 4 workflows; withdrawal `paid` only via `/withdraw-paid`
- Sensitive-input screen, opener recusal, 3 open-ticket cap
- Manual withdrawal ledger (`/mutasi`); bot never transfers funds

HTTP (JWT): `/api/v1/tickets`, `/api/v1/membership`.
Discord gateway: set `DISCORD_BOT_TOKEN` + `DISCORD_GUILD_ID`, enable **Server Members Intent**, then `make run`.
Apply `migrations/000002_discord_marketplace.up.sql`.

Production: see `DEPLOY.md`. CD is manual (`workflow_dispatch`), image `ghcr.io/<owner>/skill-issues-bot`.

Guild layout: `/rolesync` creates missing launch roles (never deletes). Channel apply stays a one-time Discord admin job; the live bot does not recreate channels.

## Endpoints

- `GET /api/v1/health/live`
- `GET /api/v1/health/ready`
- `POST /api/v1/auth/register`
- `POST /api/v1/auth/login`
- `GET /api/v1/users/me` with `Authorization: Bearer <access_token>`
- `POST /api/v1/tickets` create marketplace ticket
- `POST /api/v1/membership/verify` Buyer gate

## Prinsip yang Diambil dari Audit Dashnet

- Jangan cache JWT secret sebelum config loaded.
- Fail fast saat secret wajib kosong atau terlalu lemah.
- Jangan public serve file upload secara langsung.
- Dynamic sort wajib pakai allowlist.
- Password tidak boleh MD5/SHA1, gunakan bcrypt/argon2id.
- Migration harus executable dan schema harus selaras dengan model/repository.
- Route default harus private kecuali memang public.
- Server production wajib punya timeout dan graceful shutdown cukup panjang.

## Commands

```bash
make test
make tidy
make build
make docker-up
make docker-down
```
