APP_NAME := skill-issues-bot

.PHONY: run test tidy build docker-up docker-down migrate-up

run:
	go run .

test:
	go test ./...

tidy:
	go mod tidy

build:
	go build -o bin/$(APP_NAME) .

docker-up:
	docker compose up -d

docker-down:
	docker compose down

docker-up-mysql:
	docker compose --profile mysql up -d

migrate-up:
	@echo "Run migrations with your preferred tool:"
	@echo "  Postgres: migrate -path migrations/postgres -database $$DATABASE_URL up"
	@echo "  MySQL:    migrate -path migrations/mysql -database $$DATABASE_URL up"
