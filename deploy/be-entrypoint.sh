#!/bin/sh
# Apply DB migrations, then start the API + Discord gateway.
set -eu

: "${DB_HOST:=postgres}"
: "${DB_PORT:=5432}"
: "${DB_SSL_MODE:=disable}"
: "${DB_USER:?DB_USER is required}"
: "${DB_PASSWORD:?DB_PASSWORD is required}"
: "${DB_NAME:?DB_NAME is required}"

DSN="pgx5://${DB_USER}:${DB_PASSWORD}@${DB_HOST}:${DB_PORT}/${DB_NAME}?sslmode=${DB_SSL_MODE}"

if [ "${SKIP_MIGRATIONS:-}" = "true" ] || [ "${SKIP_MIGRATIONS:-}" = "1" ]; then
  echo "[entrypoint] SKIP_MIGRATIONS set — not applying migrations."
  exec /app/api
fi

echo "[entrypoint] applying migrations to ${DB_HOST}:${DB_PORT}/${DB_NAME} ..."
i=0
until migrate -path /app/migrations/postgres -database "$DSN" up; do
  i=$((i + 1))
  if [ "$i" -ge 30 ]; then
    echo "[entrypoint] migrations still failing after ${i} tries — aborting." >&2
    exit 1
  fi
  echo "[entrypoint] db not ready / migrate failed (try ${i}), retrying in 3s ..."
  sleep 3
done
echo "[entrypoint] migrations applied. starting api."

exec /app/api
