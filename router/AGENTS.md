# Router Agent Guide

## Router Identity
`router/` owns global middleware order, API version grouping, and route registration. It should not contain business logic or database access.

## Middleware Order
Keep global middleware intentional:
1. Recovery.
2. Access log.
3. Request ID.
4. Security headers.
5. Body limit.
6. Global rate limiter.
7. Metrics middleware when enabled.
8. Audit middleware.
9. CORS.

## Rules
- Register all API routes under `/api/v1` unless explicitly versioning a new API.
- Protected routes must use `middleware.AuthMiddleware` before feature handlers.
- Every protected route group should have a route-specific limiter prefix.
- Do not expose uploads through static file serving.
- Keep metrics gated by `Config.Metrics.Enable`.
- Use `httplib.SetErrorResponse` for `NoRoute` and `NoMethod` handlers.
- Do not add direct handler functions in `router/`; handlers belong in `modules/<feature>/handler.go`.

## Adding A Route Group
1. Add the module handler to `boot.HandlerSetup` and `MakeHandler`.
2. Create a `v1.Group("/<resource>")` in `RouterWithMiddleware`.
3. Apply auth middleware if the resource is not public.
4. Apply `RateLimiterMiddlewareWithPrefix` with a stable feature prefix.
5. Call the module's `Group<Feature>` method.

## Public Routes
- Health checks may be public.
- Auth register/login/refresh/logout may be public but must stay rate-limited.
- Metrics are enabled only by config and should be protected upstream in production deployments.
- `/tickets` and `/membership` are JWT groups with limiter prefixes `ticket` and `membership` (`router/router.go`). Do not list them as public.

## Gotchas
- Middleware order changes can affect audit logs, metrics labels, response headers, and auth behavior.
- Adding a public route is a security decision; document why it is public.
- Never return raw errors from router-level handlers.

## Pre-PR Checks
```bash
gofmt -w router/*.go && go test ./... && go vet ./...
```
