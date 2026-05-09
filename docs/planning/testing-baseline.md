# Baseline тестов API (Go)

Дата фиксации: 2026-05-08

Метрики считаются по `go test -cover` с профилем, из которого исключены строки пакета `apps/api/internal/db` (сгенерированный sqlc). Запуск: `just check-coverage`.

Тесты с тегом `integration` в обычный прогон не входят (как `photos`); они запускаются через `just check-integration`. Один интеграционный тест в `internal/handler/auth` переведён на `//go:build integration`, чтобы `just check` и покрытие по умолчанию не требовали поднятого Postgres из `.env`.

## Line coverage (statements)

Общее (после фильтра `internal/db`): **10.8%**

По пакетам (как в выводе `go test -cover ./...`):

| Пакет | Покрытие |
| --- | --- |
| `internal/handler/auth` | 0.0% |
| `internal/handler/sync` | 0.0% |
| `internal/handler/photos` | 100.0% |
| `internal/jwtutil` | 82.4% |
| `internal/middleware` | 46.7% |
| `internal/storage` | 55.6% |
| `internal/config` | 0.0% |
| `cmd/api` | 0.0% |
| `internal/sync` | нет тестовых файлов |
| `internal/db` | исключён из отчёта по покрытию бизнес-кода |

## Branch coverage

В Go стандартный инструмент даёт покрытие по модели на основе базовых блоков (statement/block coverage через `-coverprofile`), а не отдельный отчёт branch coverage. Для API baseline ориентир — line/statement coverage из `go tool cover -func`.

## Endpoint coverage (OpenAPI)

Источник путей: `packages/api-contracts/openapi.yaml`

Всего операций на верхнем уровне: **6**

- `/health`
- `/api/v1/auth/sign-in`
- `/api/v1/auth/refresh`
- `/api/v1/sync/pull`
- `/api/v1/sync/push`
- `/api/v1/photos/upload-url`

Оценка по наличию хотя бы одного теста, дергающего маршрут: **1 из 6 (~16.7%)** — покрыт `POST /api/v1/photos/upload-url` (`internal/handler/photos`).

Остальные маршруты на момент baseline без таких тестов (в т.ч. `/health` объявлен в OpenAPI, но реализован в `cmd/api/main.go` без отдельного теста хендлера).

## Целевые ориентиры (следующий шаг)

*Примечание: мы не стремимся к увеличению покрытия ради самого покрытия. Это всего лишь одна из метрик для анализа ситуации и защиты от регрессий, а не самоцель.*

- Поднять statement coverage (без `internal/db`) до **≥ 30%**.
- Поднять endpoint coverage до **≥ 50%** за счёт тестов `auth` и `sync`.

## Порог в CI / `just check`

Минимальное заявленное покрытие statements (после того же фильтра): **10%** — см. `just check` и рецепт с порогом в `justfile`.
