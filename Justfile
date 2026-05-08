set dotenv-load := true

api_dir := "apps/api"
web_dir := "apps/web"
api_coverage_min_pct := "10"

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

_api-build-coverage-profile:
    #!/usr/bin/env bash
    set -euo pipefail
    cd "{{api_dir}}"
    cov_raw="$(mktemp)"
    trap 'rm -f "$cov_raw"' EXIT
    go test -coverprofile="$cov_raw" ./...
    grep -v '/internal/db/' "$cov_raw" > coverage_filtered.out || true

# Покрытие API (statements), без internal/db; полный отчёт go tool cover
check-coverage: _api-build-coverage-profile
    cd "{{api_dir}}" && go tool cover -func="coverage_filtered.out"

_api-coverage-threshold: _api-build-coverage-profile
    #!/usr/bin/env bash
    set -euo pipefail
    cd "{{api_dir}}"
    pct="$(go tool cover -func="coverage_filtered.out" | awk '/^total:/ { gsub(/%/, "", $NF); print $NF; exit }')"
    awk -v p="$pct" -v min="{{api_coverage_min_pct}}" 'BEGIN { exit !((p + 0) >= (min + 0)) }' || {
        echo "API coverage ${pct}% is below minimum {{api_coverage_min_pct}}% (filtered internal/db)" >&2
        exit 1
    }
    echo "API coverage OK: ${pct}% (min {{api_coverage_min_pct}}%, internal/db excluded)"

# Проверить всё: сборка + линт + тесты + порог покрытия API
check:
	cd {{api_dir}} && go build ./...
	cd {{api_dir}} && go vet ./...
	just _api-coverage-threshold
	cd {{web_dir}} && pnpm vitest run

# Интеграционные тесты API (Docker, -tags=integration)
check-integration:
	cd {{api_dir}} && go test -tags=integration -count=1 -timeout=15m ./...

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
