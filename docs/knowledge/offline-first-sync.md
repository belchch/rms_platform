---
title: Offline-first синхронизация — индустриальный стандарт
tags: [offline-first, sync, distributed-systems, architecture]
created: 2026-04-25
---

# Offline-first синхронизация — индустриальный стандарт

Заметка-памятка по терминам и паттернам offline-first sync: **outbox**, **cursor**, **LWW rebase**, форматы ответов `/sync/push`. Собрано из контекста обсуждения `rms_platform` sprint 1, но по сути — общая индустриальная сводка, применимая к любому offline-first клиенту.

---

## 1. Outbox — очередь исходящих операций на клиенте

### Что это

Клиент, работающий оффлайн, не может просто вызвать REST и получить ответ — сети может не быть. Запись делится на две транзакции:

1. **Локальная транзакция** в SQLite: применить изменение к таблицам + **записать операцию в таблицу `outbox`**.
2. **Фоновая синхронизация**: читает `outbox`, отправляет на сервер, помечает как `synced` или `failed`.

Это версия **Transactional Outbox** — паттерн из распределённых систем (изначально — для микросервисов, чтобы не терять события при падении), адаптированная для offline-first клиента.

### Конкретная форма таблицы

```
outbox
  id                 UUID
  entityType         'project' | 'plan' | 'photo'
  entityId           UUID
  op                 'create' | 'update' | 'delete'
  payload            JSON
  clientUpdatedAt    int64
  status             'pending' | 'inFlight' | 'failed'
  attempts           int
  lastError          string
```

Когда пользователь нажимает «сохранить», в одной локальной транзакции происходит `INSERT INTO <entity>` **и** `INSERT INTO outbox`. Если приложение крашится после этого, операция не теряется — она лежит в outbox и будет отправлена в следующий запуск.

### Кто так делает

- **WatermelonDB** (React Native): есть `_changes` таблица, по смыслу тот же outbox ([docs/Sync](https://watermelondb.dev/docs/Sync/Intro)).
- **Replicache**: mutations хранятся в `pending` очереди ([doc.replicache.dev](https://doc.replicache.dev/concepts/how-it-works)).
- **RxDB**: concept `revision log` ([rxdb.info/replication](https://rxdb.info/replication.html)).
- **PouchDB/CouchDB**: `_local/<sync_id>` document хранит last-seen + pending.
- **Backend-версия паттерна**: Chris Richardson [microservices.io/patterns/data/transactional-outbox](https://microservices.io/patterns/data/transactional-outbox.html).

### Что из этого следует для сервера

Клиентский outbox определяет **контракт `/sync/push`**: сервер должен принимать **батч операций** и возвращать **per-operation статус**, чтобы клиент мог пометить каждую строку outbox как `synced` / `failed`.

---

## 2. Cursor — монотонный маркер «что я уже видел»

### Проблема

Классический способ «дай мне всё, что изменилось с момента T» — это `WHERE updated_at > T`. Работает плохо по трём причинам:

1. **Часы клиента и сервера рассинхронизированы** — клиентский `T` не соответствует серверному `updated_at`.
2. **Точность timestamp'а** — если два обновления в одну миллисекунду, одно потеряется.
3. **Длительные транзакции** — коммит транзакции в `now() = T1` виден read-after-commit после `T2 > T1`, но в таблице уже лежит `updated_at = T1` → read «с T2» это пропустит.

### Решение

Сервер держит один **глобальный счётчик** (в Postgres — `SEQUENCE`). На каждую мутацию (`INSERT` / `UPDATE` / soft-delete) строка получает `sync_cursor = nextval('sync_cursor_seq')`. Счётчик только растёт, не зависит от часов, уникальный.

```sql
-- schema
CREATE SEQUENCE sync_cursor_seq;

CREATE TABLE projects (
    id          TEXT PRIMARY KEY,
    updated_at  TIMESTAMPTZ NOT NULL DEFAULT now(),
    sync_cursor BIGINT NOT NULL DEFAULT nextval('sync_cursor_seq'),
    ...
);

-- trigger bumps sync_cursor на UPDATE
CREATE FUNCTION bump_sync_cursor() RETURNS trigger AS $$
BEGIN
  NEW.sync_cursor := nextval('sync_cursor_seq');
  RETURN NEW;
END;
$$ LANGUAGE plpgsql;

CREATE TRIGGER projects_bump_cursor
  BEFORE UPDATE ON projects
  FOR EACH ROW EXECUTE FUNCTION bump_sync_cursor();
```

Клиент хранит `lastSeenCursor` (одно число). Запрос: `GET /sync/pull?since=<lastSeenCursor>`. Ответ: все строки с `sync_cursor > since`, плюс новый `lastSeenCursor = max(sync_cursor)` среди возвращённого.

### Почему это правильно

- Счётчик монотонен — ничего не потеряется.
- Нет рассинхрона часов — сервер единственный источник.
- Одно число на клиенте — копеечный overhead.

### Кто так делает

- **Postgres Logical Replication**: позиция WAL (`pg_lsn`) — ровно такой же cursor.
- **CouchDB `_changes` feed**: параметр `since=<seq>`, см. [docs.couchdb.org](https://docs.couchdb.org/en/stable/api/database/changes.html). Это **оригинальный источник** паттерна в прикладной синхронизации.
- **Firebase Firestore**: `SnapshotMetadata` — тот же принцип под капотом.
- **Kafka consumer offsets** — то же самое для event streams.
- **Git `HEAD`** — тоже cursor, только в DAG.

Книжный референс: Martin Kleppmann, «Designing Data-Intensive Applications», главы 5 (Replication) и 11 (Stream Processing).

---

## 3. LWW (Last-Writer-Wins) — стратегия разрешения конфликтов

### Что такое конфликт

Ситуация: клиент A и клиент B оба редактировали один проект оффлайн. Оба потом пушат. B пушит позже. Что делать?

- **Применить последнее** (B перезаписал A).
- **Применить по timestamp'у** (кто позже начал редактировать — тот и прав → LWW).
- **Автоматический мёрж** (CRDT / OT) — структура данных гарантирует сходимость.
- **Показать пользователю** (git-merge-conflict UX).

### LWW

LWW = **Last Writer Wins by timestamp**. На каждой строке — `updated_at`. При входящем push-операции:

```
if incoming.clientUpdatedAt > db.updated_at:
    apply(incoming)
else:
    conflict  -- клиентская версия устарела
```

Ключевой момент: сравниваемый timestamp — **клиентский `updated_at` на момент операции**, не момент прихода на сервер. Это именно то, как был помечен объект, когда пользователь его редактировал.

### LWW vs CRDT vs OT — когда что

| Стратегия | Что даёт | Когда уместно |
|-----------|----------|---------------|
| **LWW** | Простая реализация, одно поле `updated_at` | Редко редактируется параллельно: CRM, файловая система, метаданные |
| **Field-level LWW** | Каждое поле — отдельный timestamp | Параллельное редактирование разных полей |
| **CRDT** (Automerge, Yjs) | Автоматический мёрж без потерь | Совместное редактирование документов в реальном времени (Notion, Linear offline-ops) |
| **OT** (Operational Transform) | То же, но через трансформацию операций | Google Docs, Figma multiplayer |
| **Server-authoritative** | Сервер — источник истины, клиент ретраит | Финансы, корзины в e-commerce |

### Недостаток LWW

**Теряет данные.** Одна из двух параллельных правок полностью исчезает. Поэтому LWW годится там, где параллельность редкая. Если начинает ломать UX — нужно переходить к полевому LWW или CRDT.

---

## 4. Rebase + коды ответа — что делать при конфликте

### Аналогия с git

Git rebase: локальные коммиты, на remote появились другие. Push не пройдёт — нужно сначала pull, перебазировать свои поверх, потом push. Синхронизация работает **так же**:

- Клиент пушит операцию для row X с базой `updated_at = T1`.
- Сервер видит, что у X сейчас `updated_at = T2 > T1`.
- Значит, между pull'ом клиента и его push'ем кто-то другой изменил X.
- Клиент должен сначала подтянуть (pull), смёржить, потом пушить заново.

### Три индустриальных подхода к ответу сервера

### A. Fail-fast 409 (git style)

- Первый конфликт → ответ `409 Conflict`, вся транзакция откатывается.
- Тело: `{ conflict: { clientOp, serverVersion } }`.
- Клиент: получает 409 → делает pull → мёржит локально → пушит заново.

**Плюсы**: простая реализация, атомарно, семантически честно.
**Минусы**: если в батче 50 операций и одна конфликтная — клиент ретраит все 50 после resolve. Chatty.

**Примеры**: стандартные REST API ресурсов с ETag / If-Match, старый Azure Mobile Services, PostgREST.

### B. Partial success 200 (DynamoDB / Couch style)

- Сервер применяет всё, что можно, конфликтные — откладывает.
- Ответ `200 OK` с телом:

```json
{
  "cursor": 12345,
  "applied": ["op-id-1", "op-id-2"],
  "conflicts": [
    { "clientOpId": "op-id-3", "reason": "stale", "serverVersion": { "...": "..." } }
  ],
  "errors": [
    { "clientOpId": "op-id-4", "reason": "validation", "message": "..." }
  ]
}
```

- Клиент: applied → пометить synced, conflicts → пометить failed, поднять UI для resolve.

**Плюсы**: один round-trip, клиент видит полный фронт конфликтов, 80% операций типично применяются сразу.
**Минусы**: не атомарно — в одном батче часть операций может быть conflict'ной, остальные пройдут. Нужно аккуратно с инвариантами между сущностями.

**Примеры**:
- **DynamoDB `BatchWriteItem`** возвращает `UnprocessedItems`, клиент ретраит — тот же паттерн в retry-форме.
- **CouchDB `_bulk_docs`** — массив per-doc результатов, конфликты маркируются отдельно ([docs.couchdb.org/_bulk_docs](https://docs.couchdb.org/en/stable/api/database/bulk-api.html)).
- **Google Calendar Batch API** — multipart ответ с per-item статусом.
- **Stripe bulk** — тот же принцип.
- **WatermelonDB sync protocol** — сервер применяет и возвращает `conflicts` отдельным полем.

### C. All-or-nothing 422 (финансовая транзакционность)

- Любой конфликт → вся транзакция откатывается, `422 Unprocessable Entity` со списком конфликтов.
- Клиент: resolve все → ретраит батч целиком.

**Плюсы**: строгая атомарность.
**Минусы**: почти не нужна в offline-sync (inter-entity инвариантов в батче обычно нет), увеличивает хвост ретраев.

**Примеры**: банковские транзакции, резервирование товаров — «либо всё, либо ничего».

---

## 5. Как делают конкретные продукты

### CouchDB / PouchDB

- Каждый документ — `_rev` (ревизия, инкрементируется).
- Push: если `_rev` в запросе ≠ `_rev` в БД → конфликт по этому документу.
- `_bulk_docs` возвращает per-doc массив — **partial success**.
- Conflict-resolution — ручная: pull → merge → push с новым `_rev`.

### WatermelonDB (React Native)

- Явно описывает протокол pull/push: `pullChanges({ lastPulledAt })` → возвращает дельту → `pushChanges({ changes, lastPulledAt })` → применяет.
- Partial success: `changes` в ответе push'а может содержать и applied, и conflicts.
- Документация — одна из самых внятных в индустрии:
  - [Sync Intro](https://watermelondb.dev/docs/Sync/Intro)
  - [Sync Backend](https://watermelondb.dev/docs/Sync/Backend)

### Replicache (mutation-based вместо state-based)

- Клиент хранит не «новое состояние row», а **mutations** (функции типа `createTodo(id, title)`).
- Сервер выполняет mutations по очереди, они идемпотентные — классические конфликты исчезают.
- Совсем другой ментальный фрейм, но ультимативно элегантный: [doc.replicache.dev/concepts/how-it-works](https://doc.replicache.dev/concepts/how-it-works).

### Linear

- Mutation-based sync engine, очень близко к Replicache.
- Технический talk Tuomas Artman: [linear.app/blog/scaling-the-linear-sync-engine](https://linear.app/blog/scaling-the-linear-sync-engine).

### RxDB

- Универсальная клиент-side sync для любого бэкенда.
- Явно формализует паттерн: `pullHandler` + `pushHandler` + `checkpoint` (их слово для cursor).
- [rxdb.info/replication](https://rxdb.info/replication.html) — компактное и ясное объяснение концептов.

### Figma

- OT + CRDT для совместного редактирования. Не наш случай, но для общей эрудиции: [figma.com/blog/how-figmas-multiplayer-technology-works](https://www.figma.com/blog/how-figmas-multiplayer-technology-works/).

### Local-First движение (академическая сторона)

- Манифест: [inkandswitch.com/local-first](https://www.inkandswitch.com/local-first/).
- Комбинирует offline-sync, CRDT, user ownership.

---

## 6. Короткий reading list (сверху — самое полезное)

1. **Replicache «How it works»** — [doc.replicache.dev/concepts/how-it-works](https://doc.replicache.dev/concepts/how-it-works) — 10 минут, лучшее объяснение sync в индустрии.
2. **WatermelonDB Sync Backend** — [watermelondb.dev/docs/Sync/Backend](https://watermelondb.dev/docs/Sync/Backend) — ближе всего к тому, как строится протокол; формальный pull/push.
3. **RxDB Replication** — [rxdb.info/replication](https://rxdb.info/replication.html) — введение в checkpoint / pull / push.
4. **Kleppmann, DDIA, глава 5** — фундамент по репликации и конфликтам.
5. **Local-First Software (Ink & Switch)** — [inkandswitch.com/local-first](https://www.inkandswitch.com/local-first/) — философия.
6. **CouchDB `_changes` feed** — [docs.couchdb.org](https://docs.couchdb.org/en/stable/api/database/changes.html) — оригинал cursor-паттерна.
7. **Transactional Outbox (backend-side)** — [microservices.io/patterns/data/transactional-outbox](https://microservices.io/patterns/data/transactional-outbox.html).
8. **Linear Sync Engine talk** — [linear.app/blog/scaling-the-linear-sync-engine](https://linear.app/blog/scaling-the-linear-sync-engine).

Если читать **только одно** — Replicache или WatermelonDB Sync Backend.

---

## 7. Сводная памятка-cheatsheet

| Термин | Что делает | Где живёт |
|--------|-----------|-----------|
| **Outbox** | Очередь исходящих операций клиента, «операция не потеряется при падении» | Клиентская SQLite-таблица |
| **Cursor** | Монотонный серверный счётчик «что я уже видел» | Postgres SEQUENCE + колонка `sync_cursor` |
| **LWW** | Конфликт → побеждает запись с бóльшим `updated_at` | Логика в serverside push-обработчике |
| **Rebase** | Клиентская версия устарела → pull → retry | Клиентский retry-loop |
| **409 Conflict** | Fail-fast на первый конфликт, откат батча | Code path для кон-сервативных API |
| **200 + `conflicts[]`** | Partial success: applied + conflicts + errors | Современный стандарт для sync API |
| **422 all-or-nothing** | Любой конфликт → полный откат | Финансы, bulk reservations |

### Связки

- **Outbox + Cursor** — минимальный дуэт для offline-first. Всё остальное — стратегии конфликтов поверх.
- **LWW + Partial 200** — типичный современный выбор для CRM-подобных приложений (редкие параллельные правки + per-op статусы для клиентского outbox).
- **CRDT + WebSocket** — для реального real-time co-editing (Figma, Linear issues). Overkill для 90% бизнес-приложений.

---

## 8. Мапинг на rms_platform

Для справки: в проекте `rms_platform` в sprint 1 делается именно **Outbox (на клиенте) + Cursor (через Postgres SEQUENCE) + LWW (по `updated_at`) + Partial 200 на `/sync/push`**. См. [[sprint-1]] (`docs/planning/sprint-1.md`) §§3.4, 4 и §5 строки «Формат `PushOperation`» и «Инкремент `sync_cursor`».
