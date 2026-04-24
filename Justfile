set dotenv-load := true

api_dir := "apps/api"
web_dir := "apps/web"

# Запустить весь стек локально
dev:
    docker compose up -d
    just _dev-parallel

_dev-parallel:
    #!/usr/bin/env bash
    set -e
    (cd {{api_dir}} && air) &
    (cd {{web_dir}} && pnpm dev) &
    wait

# Проверить всё: сборка + линт + тесты
check:
    cd {{api_dir}} && go build ./...
    cd {{api_dir}} && go vet ./...
    cd {{web_dir}} && pnpm vitest run

# Сгенерировать sqlc-код и TS-типы из OpenAPI
api-gen:
    cd {{api_dir}} && sqlc generate
    cd packages/api-contracts && pnpm generate

# Применить миграции (goose up)
migrate:
    cd {{api_dir}} && goose -dir migrations postgres "$DATABASE_URL" up

# Пересоздать dev-БД с нуля (до стабилизации схемы)
db-reset:
    cd {{api_dir}} && goose -dir migrations postgres "$DATABASE_URL" down-to 0
    cd {{api_dir}} && goose -dir migrations postgres "$DATABASE_URL" up

# Поднять только инфраструктуру (postgres + minio)
infra:
    docker compose up -d

# Остановить docker-compose
down:
    docker compose down
