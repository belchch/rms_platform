---
title: Sync Protocol — зафиксированные решения
tags: [sync, api, protocol, decisions]
created: 2026-04-25
---

# Sync Protocol — зафиксированные решения

Документ — единственная точка правды по API синхронизации между `rms_platform` (бэкенд) и `rms_mobile` (клиент). Все решения из него **окончательны** для sprint 1; изменения — только через PR в `packages/api-contracts/openapi.yaml` с одновременным обновлением этого файла.

---

## 1. Сущности и типы

### EntityType

```
project | plan | room | wall | photo
```

`room` и `wall` — first-class sync-сущности, не вложенные в `plan.payloadJson`.

### Op

```
create | update | delete
```

`delete` — soft-delete: сервер выставляет `deleted_at`, строка остаётся в БД. Клиент получает tombstone в следующем pull.

---

## 2. Push — `/api/v1/sync/push`

**Метод:** `POST`  
**Auth:** Bearer JWT (обязателен)

### Запрос

```json
{
  "operations": [
    {
      "clientOpId": "01HZ...",
      "op": "create",
      "entityType": "project",
      "entityId": "01HZ...",
      "clientUpdatedAt": 1714046400000,
      "payload": { ... }
    }
  ]
}
```

| Поле | Тип | Описание |
|------|-----|----------|
| `clientOpId` | UUID v7 | Уникален **на операцию** (не на сущность). Используется клиентом для маркировки outbox-строки как synced/failed. |
| `op` | `create \| update \| delete` | Тип операции. |
| `entityType` | `EntityType` | Тип сущности. |
| `entityId` | UUID v7 | ID сущности — эхо клиентского. Сервер **не генерирует** свой ID. |
| `clientUpdatedAt` | int64 (epoch-ms) | Метка времени клиента на момент мутации. Используется для LWW-разрешения конфликтов. |
| `payload` | JSON-объект | Частичное состояние сущности (см. §4). При `op=delete` — пустой объект `{}`. |

### Ответ — Partial-success 200

**Все ответы push-endpoint возвращают `200 OK`**, включая случаи с конфликтами. Статус `4xx` используется только для ошибок транспортного уровня (невалидный JSON, отсутствующий auth).

```json
{
  "cursor": 12345,
  "applied": ["01HZ...", "01HZ..."],
  "conflicts": [
    {
      "clientOpId": "01HZ...",
      "reason": "stale",
      "serverVersion": {
        "entityType": "project",
        "entityId": "01HZ...",
        "payload": { ... }
      }
    }
  ],
  "errors": [
    {
      "clientOpId": "01HZ...",
      "reason": "validation",
      "message": "name is required"
    }
  ]
}
```

| Поле | Тип | Описание |
|------|-----|----------|
| `cursor` | int64 | Новый `sync_cursor` после применения батча. Клиент сохраняет как `lastPushedCursor`. |
| `applied` | `string[]` | `clientOpId` операций, успешно применённых. Outbox → `synced`. |
| `conflicts` | `ConflictItem[]` | Операции, отклонённые по LWW. Outbox → `failed`. |
| `errors` | `ErrorItem[]` | Операции, отклонённые по валидации или бизнес-правилу. Outbox → `failed`. |

#### ConflictItem

| Поле | Тип | Описание |
|------|-----|----------|
| `clientOpId` | string | ID отклонённой операции из запроса. |
| `reason` | `"stale"` | Единственная причина: LWW — серверная версия новее `clientUpdatedAt`. |
| `serverVersion.entityType` | EntityType | — |
| `serverVersion.entityId` | string | — |
| `serverVersion.payload` | JSON | Актуальное состояние сущности на сервере. Клиент использует для UI merge/conflict-resolution. |

#### ErrorItem

| Поле | Тип | Описание |
|------|-----|----------|
| `clientOpId` | string | ID операции с ошибкой. |
| `reason` | `"validation" \| "notFound" \| "forbidden" \| "unknown"` | Машиночитаемый код. `notFound` — сущность не существует; `forbidden` — нет прав; `unknown` — внутренняя ошибка. |
| `message` | string | Человекочитаемое описание для лога/debug. |

### LWW-правило на сервере

```
if incoming.clientUpdatedAt > db.updated_at:
    apply(incoming)           → добавить clientOpId в applied[]
else:
    reject(incoming)          → добавить в conflicts[]
```

Сравнение — по `clientUpdatedAt` (epoch-ms), не по времени получения запроса.

---

## 3. Pull — `/api/v1/sync/pull`

**Метод:** `GET`  
**Auth:** Bearer JWT (обязателен)  
**Query:** `?since=<cursor>` (int64, 0 при первом pull)

### Ответ

```json
{
  "cursor": 12345,
  "changes": [
    {
      "entityType": "project",
      "entityId": "01HZ...",
      "payload": { ... },
      "updatedAt": 1714046400000,
      "syncCursor": 12340,
      "deletedAt": null
    }
  ]
}
```

| Поле | Тип | Описание |
|------|-----|----------|
| `cursor` | int64 | `max(changes[].syncCursor)`. Клиент сохраняет как `lastSeenCursor` и использует в следующем `?since=`. |
| `changes` | `PullChange[]` | Дельта сущностей с `sync_cursor > since` в рамках `workspace_id` из JWT. |
| `changes[].syncCursor` | int64 | Серверный курсор конкретного изменения. |
| `changes[].deletedAt` | int64? (epoch-ms) | Tombstone: если не `null` — сущность удалена, клиент должен удалить локально. |

Фильтрация на сервере: `sync_cursor > since AND workspace_id = <из JWT> AND (deleted_at IS NULL OR deleted_at IS NOT NULL)` — tombstones включаются в дельту.

---

## 4. Payload-схемы по entityType

### ProjectPayload

```json
{
  "name": "ЖК Прогресс",
  "address": "ул. Ленина, 1",
  "description": null,
  "isArchived": false,
  "isFavourite": true
}
```

| Поле | Обязательность |
|------|---------------|
| `name` | required |
| `address` | optional |
| `description` | optional |
| `isArchived` | required |
| `isFavourite` | required |

`coverPhotoId` — **отсутствует** (убран из payload; связь фото с проектом — через `PhotoPayload.parentType = "project"`).

### PlanPayload

```json
{
  "projectId": "01HZ...",
  "name": "Этаж 1",
  "payloadJson": { ... }
}
```

| Поле | Обязательность |
|------|---------------|
| `projectId` | required |
| `name` | required |
| `payloadJson` | optional — геометрия плана в JSONB; в sprint 1 может отсутствовать |

### RoomPayload

```json
{
  "planId": "01HZ...",
  "name": "Гостиная"
}
```

| Поле | Обязательность |
|------|---------------|
| `planId` | required |
| `name` | optional |

### WallPayload

```json
{
  "roomId": "01HZ..."
}
```

| Поле | Обязательность |
|------|---------------|
| `roomId` | required |

### PhotoPayload

Polymorphic: фото может принадлежать `project`, `room` или `wall`.

```json
{
  "parentType": "room",
  "parentId": "01HZ...",
  "contentType": "image/jpeg",
  "name": "Северная стена",
  "caption": "Трещина над окном",
  "takenAt": 1714046400000
}
```

| Поле | Обязательность |
|------|---------------|
| `parentType` | required — `project \| room \| wall` |
| `parentId` | required |
| `contentType` | required |
| `name` | optional |
| `caption` | optional |
| `takenAt` | optional (epoch-ms) |

---

## 5. Загрузка фото

Трёхшаговая. `photoId` генерирует **клиент** (UUID v7) — консистентно с остальными сущностями.

**Шаг 1 — получить pre-signed URL:**
```
POST /api/v1/photos/upload-url
Content-Type: application/json

{ "photoId": "01HZ...", "contentType": "image/jpeg" }
```

Ответ:
```json
{
  "uploadUrl": "https://minio.local/...",
  "method": "PUT",
  "headers": { "Content-Type": "image/jpeg" },
  "expiresAt": 1714050000000
}
```

**Шаг 2 — загрузить бинарник:**
```
PUT <uploadUrl>
<headers из ответа шага 1>

<binary>
```

Заголовки из `headers` обязательны — S3/MinIO включает их в подпись URL.

**Шаг 3 — зафиксировать метаданные:**
Отправить `PushOperation` с `entityType=photo`, `entityId=<photoId из шага 1>`, `op=create`, `payload=PhotoPayload`.

Multipart **не используется**. Причина: pre-signed PUT не привязан к API-серверу, масштабируется независимо, таймаут крупных файлов не блокирует бизнес-логику.

---

## 6. Auth — JWT-контракт

### `/auth/sign-in`

```
POST /api/v1/auth/sign-in
{ "email": "demo@rms.local", "password": "demo" }
```

Ответ:
```json
{
  "accessToken": "eyJ...",
  "refreshToken": "..."
}
```

TTL токена декодируется из claims (`exp`). Отдельное поле `expiresIn` не передаётся.

### Access-token claims

```json
{
  "sub": "<userId>",
  "workspaceId": "<workspaceId>",
  "exp": 1714047300
}
```

`workspaceId` вшит в токен — middleware не ходит в БД на каждый запрос. При появлении shared workspaces — endpoint `/auth/switch-workspace` (вне sprint 1).

### `/auth/refresh`

```
POST /api/v1/auth/refresh
{ "refreshToken": "..." }
```

Ответ — та же структура, что и sign-in.

---

## 7. `sync_cursor` — механизм

- Postgres `SEQUENCE` (`sync_cursor_seq`).
- Каждая таблица (`projects`, `plans`, `rooms`, `walls`, `photos`) имеет колонку `sync_cursor BIGINT`.
- Триггер `BEFORE INSERT OR UPDATE` вызывает `nextval('sync_cursor_seq')` и пишет в `sync_cursor`.
- Клиент хранит одно число `lastSeenCursor`. Нет зависимости от часов клиента или сервера.

---

## 8. Клиентский outbox — маппинг статусов

| Событие | Поле в ответе сервера | Новый статус outbox-строки |
|---------|-----------------------|---------------------------|
| Операция применена | `applied[]` содержит `clientOpId` | `synced` |
| Конфликт LWW | `conflicts[]` содержит `clientOpId` | `failed`, показать UI resolve |
| Ошибка валидации | `errors[]` содержит `clientOpId` | `failed`, показать пользователю `message` |
| Сеть недоступна | HTTP-ошибка (нет ответа) | Без изменений, retry при следующей сессии |

---

## 9. Связанные артефакты

| Файл | Что там |
|------|---------|
| `packages/api-contracts/openapi.yaml` | Машиночитаемый контракт (источник истины для codegen) |
| `apps/api/internal/sync/types.go` | Go-реализация всех типов из §2–4 |
| `docs/knowledge/offline-first-sync.md` | Индустриальный контекст: outbox, cursor, LWW, partial-200 |
| `docs/planning/sprint-1.md` §10 | История решений, снятые риски |
| `apps/api/migrations/001_init.sql` | Текущая схема БД включая `sync_cursor` и триггеры |
