# Migrations Agent Guide

## Migration Identity
Migrations define the production database contract for PostgreSQL. They must be reproducible, safe, and aligned with `modules/primitive/model.go` and repository queries.

## Rules
- Every table must have a primary key.
- Add foreign keys for important relationships.
- Add unique constraints for identity and idempotency fields.
- Add indexes for list filters, ownership checks, soft deletes, and keyset pagination.
- Prefer `TIMESTAMPTZ` for timestamps.
- Use `deleted_at` only when soft delete is needed.
- Keep `up.sql` and `down.sql` consistent.
- Do not drop production tables casually.
- Do not add columns used by code without migrations.

## Current Schema Areas
- `users`: base user identity.
- `refresh_tokens`: hashed refresh token rotation/revocation.
- `outbox_events`: async processing queue.
- `dead_letter_events`: failed async event storage.
- `uploaded_files`: upload metadata.
- `audit_logs`: access/resource audit trail.

## Required Checks Before Changing Schema
```bash
go test ./...
go vet ./...
test -f migrations/000001_create_users.up.sql
test -f migrations/000001_create_users.down.sql
```

## Common Gotchas
- If adding a model field in `modules/primitive/model.go`, add the matching migration.
- If adding a repository filter, add an index when the table can grow large.
- If adding a new generated business identifier, add a unique constraint.
- If adding a high-volume list endpoint, add keyset-compatible index such as `(created_at DESC, id DESC)`.
