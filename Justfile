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

_api-build-coverage-profile-quiet:
    #!/usr/bin/env bash
    set -euo pipefail
    cd "{{api_dir}}"
    cov_raw="$(mktemp)"
    test_output="$(mktemp)"
    trap 'rm -f "$cov_raw" "$test_output"' EXIT
    if ! go test -coverprofile="$cov_raw" ./... >"$test_output"; then
        cat "$test_output"
        exit 1
    fi
    grep -v '/internal/db/' "$cov_raw" > coverage_filtered.out || true

# Покрытие API (statements), без internal/db; полный отчёт go tool cover
check-coverage: _api-build-coverage-profile
    cd "{{api_dir}}" && go tool cover -func="coverage_filtered.out"

check-coverage-compact: _api-build-coverage-profile
    cd "{{api_dir}}" && go tool cover -func="coverage_filtered.out" | awk '$NF != "0.0%"'

check-coverage-missing-by-package: _api-build-coverage-profile-quiet
    #!/usr/bin/env bash
    set -euo pipefail
    cd "{{api_dir}}"
    rows="$(mktemp)"
    totals="$(mktemp)"
    trap 'rm -f "$rows" "$totals"' EXIT
    awk -v rows="$rows" -v totals="$totals" '
        NR == 1 { next }
        {
            path = $1
            statements = $2 + 0
            count = $3 + 0
            sub(/:.*/, "", path)
            pkg = path
            sub(/\/[^\/]+\.go$/, "", pkg)
            total += statements
            package_total[pkg] += statements
            if (count == 0) {
                uncovered += statements
                package_uncovered[pkg] += statements
            }
        }
        END {
            if (total == 0) {
                print "0\t0\t0" > totals
                exit
            }
            printf "%d\t%d\t%.1f\n", total, uncovered, uncovered / total * 100 > totals
            for (pkg in package_total) {
                if (package_uncovered[pkg] > 0) {
                    printf "%.6f\t%s\t%d\t%d\t%.6f\n",
                        package_uncovered[pkg] / uncovered * 100,
                        pkg,
                        package_uncovered[pkg],
                        package_total[pkg],
                        package_uncovered[pkg] / package_total[pkg] * 100 > rows
                }
            }
        }
    ' coverage_filtered.out
    awk -F'\t' '{ printf "total statements: %d\nuncovered statements: %d (%.1f%%)\n\n", $1, $2, $3 }' "$totals"
    printf "%10s  %10s  %10s  %10s  %s\n" "share" "pkg-miss" "missing" "total" "package"
    sort -nr -k1,1 "$rows" | awk -F'\t' '{ printf "%9.1f%%  %9.1f%%  %10d  %10d  %s\n", $1, $5, $3, $4, $2 }'

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
