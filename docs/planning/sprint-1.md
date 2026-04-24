# Sprint 1 — план первого спринта бэкенда

> **Это sprint 1, а не весь MVP.** Документ описывает **минимально необходимый slice бэкенда**, который нужен мобильному клиенту (`rms_mobile`) для прохождения своих этапов 4 и 5 из [`rms_mobile/docs/planning/sprint-1.md`](../../../AndroidStudioProjects/rms_mobile/docs/planning/sprint-1.md). Estimator (price lists, cost rules, Typst-экспорт), web-клиент (Quasar), RBAC, multi-tenant, фоновая синхронизация, метрики — **за рамками sprint 1**. Полный объём MVP и причины выбора стека — в [`backend-stack-discussion.md`](backend-stack-discussion.md).
>
> Документ — опора для постановки бэкенд-тикетов параллельно со sprint 1 мобилки, не ТЗ на одну задачу.

---

## 1. Контекст

**Что делаем.** Живой REST-бэкенд, который стыкуется с клиентским `ApiClient`, принимает push-операции из outbox, отдаёт дельту через pull-by-cursor, принимает multipart-загрузку фото и выдаёт хотя бы «демо»-JWT, чтобы interceptor мобилки работал на реальных токенах.

**Что уже есть (baseline — «этап 0» бэкенда).**

| Компонент | Состояние |
|---|---|
| `docker-compose.yml` | Postgres 16 + MinIO поднимаются локально |
| `Justfile` | `just dev`, `just migrate`, `just db-reset` работают |
| `apps/api/cmd/api/main.go` | chi + huma/v2 поднимают сервер, регистрируют роуты auth/sync/photos, `/health` отвечает |
| `apps/api/migrations/001_init.sql` | Схема для `users`, `refresh_tokens`, `workspaces`, `projects`, `plans`, `rooms`, `walls`, `photoables`, `photos`; колонки `updated_at`, `deleted_at`, `sync_cursor` (эквивалент `serverCursor` из sprint-1 мобилки §7.3) |
| `apps/api/sqlc.yaml` | Настроен, но `queries/` пустая и `internal/db/` пустая — кодогенерация не запускалась |
| `packages/api-contracts/openapi.yaml` | Есть пути `/auth/sign-in`, `/auth/refresh`, `/sync/pull`, `/sync/push`, `/photos`; но `changes` и `operations` описаны как `items: {}` |
| `apps/api/internal/handler/{auth,sync,photos}/handler.go` | Все три пакета возвращают `"todo"` или `cursor: 0, changes: []` — бизнес-логики нет |
| Auth middleware | Отсутствует, хотя маршруты в OpenAPI помечены `bearerAuth` |
| S3-клиент | Нет в `go.mod`, MinIO бакет не создаётся |
| Seed (демо-пользователь + workspace) | Нет |

Вывод: скелет полноценно компилируется и поднимается, но клиенту против него нельзя ни войти, ни запушить, ни скачать — всё зеркало заглушек.

---

## 2. Что требует sprint 1 мобилки

Ссылки — на разделы [`rms_mobile/docs/planning/sprint-1.md`](../../../AndroidStudioProjects/rms_mobile/docs/planning/sprint-1.md).

| Этап мобилки | Что клиент делает | Что от этого нужно серверу |
|---|---|---|
| **4 — REST API + sync** | `ApiClient` (dio), outbox, `SyncService`, LWW-merge, pull-to-refresh | Рабочие `/sync/pull` + `/sync/push` с согласованной схемой; зафиксированные DTO операций и дельт; `sync_cursor`, отдаваемый сервером и монотонно растущий |
| **4 — photos upload** | Пайплайн `dirty → upload → synced`, multipart POST | Рабочий `/photos`: парсит multipart, кладёт бинарник в MinIO, пишет `photos` в БД, возвращает `id` + `remoteUrl` |
| **5 — авторизация** | JWT interceptor, refresh-race, sign-in экран | Рабочий `/auth/sign-in` + `/auth/refresh` с реальными подписанными JWT; middleware, достающий `workspaceId` из токена |
| **4 (даже до этапа 5)** | `dio` interceptor всё равно подписывает запросы | Хотя бы «демо»-endpoint, который по `demo/demo` выдаёт настоящий JWT — иначе нельзя тестировать sync |

---

## 3. Пробелы — сгруппировано по блокеру

### 3.1 Контракт (блокирует клиентский codegen)

В `openapi.yaml` `PullResponse.changes: array items: {}` и `PushRequest.operations: array items: {}` — схема не зафиксирована. Пока это так, клиентский DTO нельзя ни сгенерировать, ни написать руками без произвольного угадывания.

**Что нужно закрыть:**

- `PushOperation` — как минимум `op: "create" | "update" | "delete"`, `entityType: "project" | "plan" | "photo"`, `id` (UUID v7 с клиента), `clientUpdatedAt` (unix-millis), `payload` (частичный объект сущности).
- `PullChange` — аналогичная структура + серверный `cursor` на элемент и `deletedAt?` для tombstone.
- Ответ `/sync/push`: новый `cursor` + маппинг `{ clientId → serverId }` (если мы планируем серверные ID, иначе — эхо клиентских) + список конфликтов (409) с серверной версией для каждого.

Решение формата операций — первое, что делаем в sprint 1 бэкенда. Без него ни клиент, ни сервер не могут двигаться параллельно.

### 3.2 Данные (блокирует первый успешный pull/push)

- `apps/api/queries/*.sql` пустая → `internal/db/*.go` не сгенерирован. Нужны `.sql` для `projects`, `plans`, `photos`, `workspaces`, `users`, `refresh_tokens`: select by cursor/workspace, upsert, soft-delete, lookup refresh-токена по хэшу.
- `sync_cursor`: сейчас колонка есть, но не инкрементируется. Нужно решить **как** — через Postgres `SEQUENCE` + trigger на update, через `NOW()` в epoch-millis, или через прикладной счётчик. Первый вариант проще всего разогнать с sqlc. Выбор зафиксировать в Этапе 4 (см. §4).
- Seed: демо-`user` + демо-`workspace`, чтобы клиент из sprint 1 (где `demoUserId` захардкожен) получил непустой `workspaceId` при первом же pull.

### 3.3 Auth (блокирует interceptor мобилки)

- `sign-in`/`refresh` возвращают `"todo"`. Нужно: проверка email+пароль (bcrypt), генерация JWT (`golang-jwt/jwt/v5`), выписывание refresh-токена в `refresh_tokens` (хэш — `argon2`/`bcrypt`), возврат пары `access + refresh`.
- Нет middleware, который достаёт `workspaceId` из access-токена и кладёт в `context.Context`. Без этого нельзя фильтровать pull по workspace.
- Вопрос **middleware: свой vs huma‑built-in** — решаем в момент реализации (Этап 3 бэкенда).

### 3.4 Sync push/pull (блокирует outbox flow)

- `/sync/pull?since=<cursor>`: должен читать `projects`/`plans`/`photos` (плюс `workspaces`, если решим их синкать), где `sync_cursor > since AND workspace_id = <from token>`, включая tombstone-ы (`deleted_at IS NOT NULL`), возвращать max-cursor как новый.
- `/sync/push`: транзакция на весь батч; LWW по `updated_at` (см. sprint-1 мобилки §7.2); 409 при rebase — ответом отдаём серверную версию конфликтного объекта; инкремент `sync_cursor` на каждую применённую запись. **Валидация**: перед сохранением частичного `payload` сервер обязан наложить его на текущую строку из БД и провалидировать инварианты (через бизнес-логику или `CHECK` constraints), чтобы избежать сломанных состояний при слиянии; при ошибке операция возвращается в списке конфликтов/ошибок.
- FK `projects.cover_photo_id → photos.id` сделан как `ON DELETE SET NULL` — это поведение ок для мобилки (soft-delete фото обнулит обложку проекта), **не** забыть то же на prun pull delta.

### 3.5 Фото (блокирует этап 3→4 мобилки)

- `/photos` сейчас — пустой ответ. Нужно:
  1. Парсинг multipart (в huma это не плоско — либо прокидываем `http.Handler` через chi напрямую, либо используем `huma.RawBody` + руками парсим `multipart.Reader`).
  2. Загрузка в MinIO — зависимость `minio-go/v7` (или `aws-sdk-go-v2`); бакет `S3_BUCKET` из `.env.example` создаётся при старте сервера, если отсутствует.
  3. Запись в `photos` с `remote_url` = presigned GET URL (или плейн URL, если бакет публичный — решить в момент реализации; sprint-1 мобилки §10 пока не закрыл этот вопрос, а [`backend-stack-discussion.md`](backend-stack-discussion.md) §6 склоняется к pre-signed).
  4. Ответ `{ id, remoteUrl }` — как уже описано в `openapi.yaml`.
- Thumbnail генерируем **на клиенте** (sprint-1 мобилки §6), сервер просто хранит — никаких `imagemagick` на сервере в sprint 1.

---

## 4. Этапы sprint 1 бэкенда

Порядок выстроен так, чтобы **рано пройти стек насквозь** — от контракта до first-successful-roundtrip — и только потом добирать аут/фото.

| # | Этап | Результат | Зависимость от клиента |
|---|------|-----------|------------------------|
| 0 | [done] Фундамент | chi + huma + pgx, миграция v1, docker-compose, handler-скелеты | — |
| 1 | **Контракт** | `PushOperation`, `PullChange`, `PullResponse.cursor`, формат ответа `/sync/push` зафиксированы в `openapi.yaml` и сгенерены в `packages/api-contracts/` | **Блокер** для мобилки этап 4 |
| 2 | sqlc-слой | `queries/*.sql` для всех таблиц; `sqlc generate` даёт `internal/db/*.go` | — |
| 3 | Demo auth | `/auth/sign-in` выдаёт настоящий JWT с `workspaceId`; middleware валидирует и кладёт `workspaceId` в context; seed демо-пользователя | Клиент этап 4 уже может отправлять подписанные запросы, даже если экран `SignInScreen` (этап 5) ещё не готов |
| 4 | Sync pull/push | LWW по `updated_at`, 409 при rebase, `sync_cursor` инкрементируется через sequence; тесты на create/update/delete round-trip и на 409 | Клиент этап 4 — полноценный pull-to-refresh работает |
| 5 | Photos | Multipart → MinIO (бакет автосоздаётся), запись в `photos`, presigned GET URL | Клиент этап 4 (photo upload pipeline) завершается |
| 6 | Seed + smoke | Интеграционный тест на docker-compose: поднять api + postgres + minio, прогнать `/auth/sign-in → /sync/push → /sync/pull → /photos → /sync/pull` и убедиться, что на втором pull приходит только что загруженное фото | Клиент может писать e2e-тесты |

Этапы 5 и 3 можно менять местами — зависит от того, что срочнее: начать отлаживать фото или снять `"todo"`-JWT у interceptor'а.

**Внутри каждого этапа** порядок — как и в sprint-1 мобилки §9: сначала контракт (если меняется), потом SQL, потом handler, потом тест.

---

## 5. Решения, которые надо принять в ходе sprint 1

| Вопрос | Когда закрываем | Варианты |
|--------|----------------|----------|
| Формат `PushOperation` | Этап 1 | (a) плоский `{ op, entityType, id, payload, clientUpdatedAt }` — проще; (b) tagged union по `entityType` — строже типизация у клиента. Предварительно — (a), закрываем до начала этапа 2 |
| Серверный ID vs эхо клиентского UUID | Этап 1 | Эхо клиентского UUID v7 — единственный способ избежать маппинга в `outbox`; рекомендация — так и делать, `id` в таблицах Postgres — `TEXT`, не `UUID`, как раз под это |
| Инкремент `sync_cursor` | Этап 4 | Postgres `SEQUENCE` + trigger `BEFORE UPDATE OR INSERT` на каждой синк-таблице vs `EXTRACT(EPOCH FROM NOW()) * 1000` при каждом upsert. Sequence надёжнее (не зависит от часов), trigger нетривиален; epoch-millis проще, но возможен clash на быстрых апдейтах. Выбор — sequence |
| Middleware auth: свой vs huma-built-in | Этап 3 | huma даёт декларацию `security` в операции, но валидацию всё равно пишем сами поверх chi — проще сделать chi-мидлвар, huma-middleware оставить на потом |
| `/photos`: multipart vs pre-signed PUT | Этап 5 | sprint-1 мобилки §10 пока не закрыл. Предварительно — multipart на первой итерации (проще на клиенте), pre-signed PUT — если упрёмся в размер запроса/таймауты |
| `workspace_id` в JWT claim vs lookup по `user_id` | Этап 3 | В JWT — быстрее (middleware не ходит в БД); при смене workspace нужно перевыпускать access. Предварительно — в JWT, потому что в sprint 1 один пользователь = один workspace |

---

## 6. Что **за рамками** sprint 1 (и почему)

| Фича | Почему не в sprint 1 |
|------|---------------------|
| Estimator (price lists, cost rules, Typst-экспорт) | Мобилка в sprint 1 не использует, это отдельная вертикаль; см. [`backend-stack-discussion.md`](backend-stack-discussion.md) §2 (волна 2) |
| Web-клиент (Quasar) | Мобилка в sprint 1 — единственный клиент; см. [`backend-stack-discussion.md`](backend-stack-discussion.md) §2 (волна 3) |
| Фоновая синхронизация (workmanager / background_fetch) | Это этап 6 sprint 1 **мобилки**, но он помечен как optional и после 4+5; бэкенду от него ничего дополнительно не нужно |
| RBAC, multi-tenant | В sprint 1 — один пользователь на один workspace, без ролей |
| Rate limit, метрики (Prometheus), структурные алерты | Для dev-инфры избыточно, добавим в sprint-N когда появится staging |
| Schema-first codegen из OpenAPI (`oapi-codegen`) | `huma` — code-first; переход на schema-first — вопрос в [`backend-stack-discussion.md`](backend-stack-discussion.md) §«Открытые вопросы» |
| Полноценный registration flow (регистрация, восстановление пароля) | В sprint 1 — только демо-юзер из seed'а; `POST /auth/register` добавим в sprint-N |
| Realtime / co-editing чертежа | См. sprint-1 мобилки §4.2: геометрия — один JSON, LWW, без CRDT |

---

## 7. Риски

| Риск | Митигация |
|------|-----------|
| Контракт операций меняется уже после того, как клиент написал DTO | Закрыть §5 строку «Формат `PushOperation`» **до** старта этапа 2 мобилки; изменения после этого момента — через PR в `packages/api-contracts/` с миграцией клиентского кода |
| `sync_cursor` через sequence требует миграции | Закладываем это в миграцию `002_sync_cursor_seq.sql` в этапе 4; политика dev-БД в мобилке (см. `rms_mobile/.cursor/skills/drift-dev-migrations/SKILL.md`) не применима к Postgres — на сервере миграции аддитивные, `goose down` только на `just db-reset` |
| Multipart в huma неожиданно окажется неудобен | Откатываемся на голый `http.Handler` в chi для `/photos`, остальные роуты остаются на huma. Не блокер, но возможная потеря 0.5 дня |
| MinIO presigned URL работают на localhost, но ломаются при смене хоста | Закладываем `S3_ENDPOINT` + `S3_PUBLIC_ENDPOINT` (второй — чтобы подписать URL, который клиент сможет резолвить) сразу, даже если в dev они совпадают |

---

## 8. Что это **не** меняет в `rms_mobile`

- Клиентская архитектура (MVVM/Riverpod) — sprint-1 мобилки §2.
- Drift dev-migrations стратегия (`.cursor/skills/drift-dev-migrations/SKILL.md`) — остаётся как есть, это про локальную БД клиента, не про Postgres.
- Правила изоляции слоёв (`lib/data/` не знает про Flutter) — не пересекаются с сервером.

---

## 9. Связанные документы

- [`rms_mobile/docs/planning/sprint-1.md`](../../../AndroidStudioProjects/rms_mobile/docs/planning/sprint-1.md) — клиентская часть того же sprint 1; этапы 4 и 5 мобилки определяют скоуп этого документа.
- [`backend-stack-discussion.md`](backend-stack-discussion.md) — история выбора Go + chi + huma + pgx; полный объём MVP (estimator, web, PDF).
- [`rms_mobile/docs/planning/data-model.md`](../../../AndroidStudioProjects/rms_mobile/docs/planning/data-model.md) — каноническая ER-диаграмма, по которой уже построена миграция `001_init.sql`.
- `apps/api/migrations/001_init.sql` — текущая схема.
- `packages/api-contracts/openapi.yaml` — текущий (неполный) контракт; sprint 1 закрывает его белые пятна.
