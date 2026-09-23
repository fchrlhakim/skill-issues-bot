# Production Checklist

- Set `JWT_SECRET` to at least 32 random characters.
- Use `DB_SSL_MODE=require` or `verify-full` outside local development.
- Run migrations before deploy.
- Enable Redis-backed rate limiting for multi-instance deployment.
- Production HTTP is loopback `127.0.0.1:18102` only (`DEPLOY.md`). Do not publish it. Discord is outbound.
- Keep `/api/v1/metrics` restricted at gateway/network layer.
- Use object storage for file uploads in production.
- Not in CI today: `gosec` and `govulncheck`. CI runs `gofmt`, `go vet`, `go test`, and `go build` (`.github/workflows/ci.yml`).
- Use PgBouncer or equivalent when scaling API instances.
- Load test with `k6 run loadtest/k6-smoke.js` and production-like payloads.
