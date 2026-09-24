# syntax=docker/dockerfile:1
# Build context is the repo root.

FROM golang:1.27.1-bookworm AS builder
WORKDIR /app
COPY go.mod go.sum ./
ENV GOPROXY=https://proxy.golang.org,direct
RUN go mod download
COPY . ./
RUN CGO_ENABLED=0 go build -trimpath -ldflags="-s -w" -o /out/api .
RUN CGO_ENABLED=0 go install -tags 'pgx5' github.com/golang-migrate/migrate/v4/cmd/migrate@v4.20.1

FROM debian:bookworm-slim
RUN apt-get update && apt-get install -y --no-install-recommends \
      ca-certificates tzdata wget \
    && rm -rf /var/lib/apt/lists/*
ENV TZ=Asia/Jakarta
WORKDIR /app
COPY --from=builder /out/api /app/api
COPY --from=builder /go/bin/migrate /usr/local/bin/migrate
COPY --from=builder /app/migrations /app/migrations
COPY deploy/be-entrypoint.sh /app/entrypoint.sh
RUN chmod +x /app/entrypoint.sh \
    && useradd -r -u 10001 -m -d /home/app app \
    && mkdir -p /app/storage \
    && chown -R app:app /app /home/app
USER app
EXPOSE 8080
ENTRYPOINT ["/app/entrypoint.sh"]
