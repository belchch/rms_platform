---
title: Расположение DTO по контрактам — sync vs REST vs huma I/O
tags: [architecture, dto, sync, rest, huma, decisions]
created: 2026-04-30
---

# Расположение DTO по контрактам — sync vs REST vs huma I/O

Заметка фиксирует договорённости о том, **где живут типы данных в Go-бэкенде** и **почему одна доменная сущность представлена несколькими разными типами**. Возникла из обсуждения «почему `internal/sync/types.go` отдельная папка и не пора ли раскидать по фича-папкам в Spring-стиле».

---

## 1. Базовый принцип

**Одна доменная сущность ≠ один тип в Go.** У каждой сущности есть несколько представлений в коде, по одному на каждый сетевой контракт, в котором она участвует.

Пример: для `Project` к концу разработки будет минимум три типа в Go.

| Представление | Контракт | Где живёт | Что описывает |
|---|---|---|---|
| `synctypes.ProjectPayload` | sync-протокол | `internal/sync/types.go` | Что синхронизируется как одна LWW-операция |
| `projects.ProjectOutput` (huma) | REST `GET /api/v1/projects/:id` | `internal/handler/projects/handler.go` | Как проект показывается клиенту |
| `projects.CreateProjectInput` (huma) | REST `POST /api/v1/projects` | `internal/handler/projects/handler.go` | Что нужно для создания |

Каждое представление **эволюционирует независимо**: добавление поля в REST-view не должно ломать sync-контракт, и наоборот.

---

## 2. Почему `internal/sync/` — отдельная папка

`internal/sync/types.go` существует **только** ради sync-протокола, не как универсальный DTO-склад.

**Причина:** sync-протокол **полиморфный**. Один эндпоинт `POST /api/v1/sync/push` принимает в одном батче операции разных сущностей (`project`, `plan`, `room`, `wall`, `photo`). Хендлер `internal/handler/sync/push.go` должен уметь распарсить сырой JSON в нужную структуру по `entityType`. Чтобы избежать импорта пятью пакетами и потенциальных циклов, все sync-DTO собраны в одном месте.

Это разделение слоёв:

- `internal/sync/` — **DTO протокола синхронизации** (без HTTP, без SQL).
- `internal/handler/sync/` — **HTTP-обёртка** над протоколом (huma handlers, маршрутизация по `entityType`).

---

## 3. Что НЕ делать (антипаттерны)

### 3.1 Использовать `synctypes.*Payload` как REST response

**Плохо:**
```go
type GetPlanOutput struct {
    Body synctypes.PlanPayload   // ← антипаттерн
}
```

Это размывает границы пакета `internal/sync/`: он перестаёт значить «sync-протокол» и превращается в «общий DTO-склад». Когда позже понадобится добавить в REST-view поля, нужные только вебу (`createdAt`, `lastEstimateStatus`, computed-поля), их придётся либо засорять `PlanPayload` (тогда они полезут в sync-контракт мобилки, где не нужны), либо всё равно создавать отдельный тип.

**Хорошо:**
```go
type PlanOutput struct {
    Body struct {
        ID          string          `json:"id"`
        ProjectID   string          `json:"projectId"`
        Name        string          `json:"name"`
        UpdatedAt   int64           `json:"updatedAt"`
        PayloadJSON json.RawMessage `json:"payloadJson"`
    }
}
```

### 3.2 Объединять Input, Output и sync-DTO в один тип

В Spring-проектах часто соблазн сделать «один Entity на всё» с `@JsonView` / `@JsonIgnore` костылями. Здесь — нет. Каждый эндпоинт имеет собственные `Input` и `Output` структуры (см. правило `go-http-handlers.mdc`):

> Input and Output are always separate named structs — never anonymous or reused across endpoints.

Это касается и sync-DTO: переиспользовать `synctypes.ProjectPayload` как `Body` huma-input нельзя, даже если поля совпадают — у них разный жизненный цикл.

---

## 4. Когда выделять отдельный пакет под доменную сущность

**Сейчас:** не выделять. Структура работает:
- sync-DTO собраны в `internal/sync/` (общие для всех sync-хендлеров).
- REST-эндпоинтов для бизнес-сущностей пока нет.

**Когда появится первый не-sync REST-контракт** по конкретному домену (например, `GET /api/v1/projects` для эстиматора) — тогда возникнет развилка:

| Вариант | Структура | Когда выбрать |
|---|---|---|
| A. Локально в хендлере | `internal/handler/projects/handler.go` содержит `ProjectOutput` рядом с handler'ом | Если домен лёгкий, типы не шарятся между хендлерами |
| B. Vertical slice (фича-пакет) | `internal/projects/{dto.go, view.go, handler.go}` | Если у домена несколько хендлеров и общие view-модели |

Решение принимаем тогда, исходя из реальных типов и зависимостей. **Заранее (на пустом месте) пакеты не создаём** — `internal/projects/` с одним файлом на одну структуру = over-engineering.

---

## 5. Открытые вопросы

### 5.1 Общие данные между мобилкой и вебом

Sprint 1 синхронизирует `project`, `plan`, `room`, `wall`, `photo` через sync-протокол для мобилки. Когда веб дойдёт до эстиматора, ему понадобится список проектов (минимум — чтобы привязать смету к проекту).

Как отдавать эти данные вебу — **не зафиксировано**. Два варианта:

- **A.** Веб тоже использует `/sync/pull` с локальной копией в IndexedDB. Один протокол, один источник истины. Минус: веб становится stateful, нужен sync-движок на клиенте.
- **B.** Веб делает классический `GET /api/v1/projects`. Появляется второй контракт по тем же данным. Минус: дублирование, нужен отдельный read-only REST.

Решать в момент старта эстиматор-вертикали (волна 2 по `backend-stack-discussion.md` §5).

### 5.2 Связь с `architecture.mdc`

Если решение по 5.1 примем — обновить `architecture.mdc` §3 (слои `apps/web`) и §4 (инварианты sync-протокола), а также добавить ссылку на этот документ из §2 (слои `apps/api`).

---

## 6. TL;DR

1. У одной сущности — **несколько типов** в Go, по одному на сетевой контракт.
2. `internal/sync/` — **только** sync-DTO, не общий склад.
3. `synctypes.*` **нельзя** использовать как REST response — это антипаттерн.
4. REST Input/Output (huma) живут **рядом со своим хендлером** в `internal/handler/<domain>/`.
5. Vertical slice пакеты `internal/<domain>/` **создавать только когда появится реальная потребность**, не заранее.
