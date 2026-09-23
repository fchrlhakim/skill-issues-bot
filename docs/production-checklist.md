# Production Checklist

- Set `JWT_SECRET` to at least 32 random characters.
- Use `POSTGRES_SSL_MODE=require` or `verify-full` outside local development.
- Run migrations before deploy.
- Enable Redis-backed rate limiting for multi-instance deployment.
- Put the service behind a load balancer or API gateway.
- Keep `/api/v1/metrics` restricted at gateway/network layer.
- Use object storage for file uploads in production.
- Run `go test ./...`, `go vet ./...`, `gosec`, and `govulncheck` in CI.
- Use PgBouncer or equivalent when scaling API instances.
- Load test with `k6 run loadtest/k6-smoke.js` and production-like payloads.
