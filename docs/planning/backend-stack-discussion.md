# RMA — выбор backend/web‑стека: история дискуссии

> Запись обсуждения стека для backend и web‑клиента RMA. Фиксирует не только итог, но и эволюцию требований, рассмотренные варианты, и — главное — **почему** на каждом шаге приоритеты смещались. Нужна как опора при следующих пересмотрах решения.

---

## 1. Контекст

На момент обсуждения в проекте существует:
- Мобильный Flutter‑клиент (sprint 1 в разработке, этапы по `rms_mobile/docs/planning/sprint-1.md`).
- Flame‑редактор плана помещения (`lib/ui/plan_editor/`).
- План sprint 1 с архитектурой и этапами 0–6 (offline‑first, JWT, sync) — **это только первый спринт, не весь MVP**.
- Каноническая модель данных в `docs/planning/data-model.md`.

Планируется:
- Упрощённый **Estimator**‑модуль (по образцу Magicplan): прайс‑листы, строки сметы, cost rules, шаблоны, версии сметы, экспорт PDF/Excel.
- **Web‑клиент** для estimator’а (как у Magicplan — estimator online‑only, tablet/desktop).
- Backend с REST API под sync push/pull, JWT‑auth, multipart‑фото.

Разработку ведёт **один инженер**, главный со‑разработчик — **AI‑агент** (Cursor/Claude), у проекта культура `.cursor/rules/` + `.cursor/skills/`.

---

## 2. Эволюция требований

Требования уточнялись по ходу дискуссии. Это важно: финальная рекомендация учитывает все четыре волны, не только исходную постановку.

| Волна | Новое требование | Эффект на выбор |
|---|---|---|
| 1 | REST + sync + JWT + фото (исходное в sprint-1 мобилки §3, §7) | Определяет базовые потребности: реляционная БД, HTTP‑клиент, файловое хранилище |
| 2 | **Estimator‑модуль** с PDF/Excel‑экспортом | Добавляет нагрузку «серверной полиграфии» и денежной арифметики; усиливает Python (WeasyPrint) |
| 3 | **Web‑клиент** для estimator (как у Magicplan) | Вводит выбор между HTMX / SPA (React/Vue) / Flutter Web |
| 4 | Приоритет **статической типизации** + **надёжности** | Снимает FastAPI как основного кандидата; поднимает Go, Rust, TS |
| 5 | Основной разработчик — **AI‑агент** | Сдвигает веса в сторону быстрого compile‑loop, эксплицитности, codegen‑дружественных стеков |
| 6 | Появление альтернативы **Elysia + Bun** | Обновлён TS‑вариант: Eden Treaty даёт end‑to‑end типы backend↔Quasar без OpenAPI‑codegen, становится серьёзным претендентом на №1 |

---

## 3. Рассмотренные backend‑варианты

### 3.1 Сводная оценка

| Стек | Статическая типизация | Надёжность в проде | Скорость MVP соло | PDF/Excel | Уход от enterprise | Эксплуатация | AI‑ML‑будущее | Карьера (startup‑трек) | AI‑агент friendly | Типы до Quasar |
|---|---|---|---|---|---|---|---|---|---|---|
| **Go + chi + sqlc** | 9 | 9 | 8 | 6 (Typst спасает) | 9 | **10** | 6 | 9 | **10** | 8 (через OpenAPI‑codegen) |
| **TS + Elysia + Bun + Drizzle** | 7 | 7 | **10** | 7 | 9 | 9 (`bun build --compile`) | 7 | 8 | 8 | **10** (Eden Treaty) |
| **TS + Fastify + Node + Drizzle** | 7 | 7 | 9 | 7 | 8 | 7 | 7 | 7 | **9** | 8 (через OpenAPI‑codegen или tRPC) |
| **Rust + Axum + SQLx** | 10 | 10 | 5 | 8 (Typst‑native) | 9 | **10** | 7 | 8 | 4 | 6 (через OpenAPI‑codegen) |
| **C# + ASP.NET + QuestPDF** | 9 | 9 | 8 | **10** | 5 | 8 | 7 | 5 | 8 | 7 (через OpenAPI‑codegen) |
| **Kotlin + Ktor + Exposed** | 9 | 9 | 9 (родной) | 7 | 7 | 8 | 6 | 6 | 6 | 7 (через OpenAPI‑codegen) |
| **Kotlin + Spring Boot** | 9 | 9 | 9 (родной) | 7 | **3** | 7 | 6 | 5 | 5 | 7 (через OpenAPI‑codegen) |
| **FastAPI + SQLAlchemy** | 6 (с mypy — 8) | 7 | 8 | **9** (WeasyPrint) | 9 | 7 | **10** | 8 | 7 | 8 (через OpenAPI‑codegen) |

Оценки — по 10‑балльной шкале, субъективные, прикладываются к **конкретному кейсу RMA** (solo dev, AI‑agent‑driven, estimator + sync + Quasar‑web).

### 3.2 Разбор по кандидатам

#### FastAPI + Python 3.12 + SQLAlchemy 2.0 + Alembic

**За:** быстрый старт, авто‑OpenAPI, Pydantic‑валидация, **WeasyPrint — лучший HTML→PDF**, openpyxl для Excel, Decimal native, Python‑ML‑будущее, активный startup‑трек.

**Против:** runtime‑ошибки даже с mypy strict; рефакторинг большой кодовой базы слабее типизированных альтернатив; **AI‑агент не получает compile‑time сигнал** — тесты падают в runtime → длиннее итерации.

**Статус:** был основным кандидатом после волны 2 (estimator‑усиление); **снят с №1 на волне 4** (приоритет статической типизации).

#### Spring Boot + Kotlin

**За:** родной стек инженера, первый день — 100% продуктивность, Spring Security, Liquibase, проверено временем.

**Против:** JVM тяжёлая (200–400 МБ baseline, 5–15 сек cold start), аннотационная магия, DI‑контекст — **AI‑агент часто галлюцинирует bean‑resolution и конфиги**; консервирует enterprise‑паттерны, от которых инженер сознательно уходит.

**Статус:** **отклонён**. Даже как «speedrun‑вариант» проигрывает Ktor’у по startup‑feel и AI‑циклу.

#### Ktor + Kotlin + Exposed + Flyway

**За:** родной язык без Spring‑корпоративщины, Kotlin‑first DSL, быстрый fat JAR / GraalVM native (20 МБ), коротрoутины, нет аннотационной магии.

**Против:** Gradle cold‑start 5–15 сек (bad для AI‑loop); PDF‑экосистема слабее (OpenHtmlToPdf vs WeasyPrint); корпус Ktor/Exposed в обучении AI‑модели меньше → **агент чаще галлюцинирует API**.

**Статус:** **fallback‑вариант**. Не основной, но рекомендуется как «запасной план», если Python/Go не зайдёт. Конкретная точка переключения с FastAPI предлагалась: «после этапа 4 если Python‑рефакторинги стабильно занимают ×2 времени → переход на Ktor».

#### Go + chi + sqlc + pgx + goose

**За:**
- **sqlc — killer feature:** пишете SQL, sqlc генерирует типизированный Go. Compile‑time SQL‑safety сильнее любого ORM. Агент меняет схему → компилятор показывает все сломанные места.
- Быстрый compile‑loop (~1 сек) — идеальный AI‑цикл.
- Один бинарник 15–30 МБ, scratch‑Docker.
- «Скучный, предсказуемый» = reliable + AI‑friendly (агент пишет шаблонный Go правильно с первого раза).
- Startup‑культура: Docker, Kubernetes, Datadog, Cloudflare, HashiCorp, Tailscale, Fly.io — всё Go.

**Против:**
- **PDF — weak point.** Варианты: `chromedp` (тянет Chromium ~300 МБ), `gofpdf` (низкоуровневый), **Typst** (subprocess — красиво, но отдельный markup‑язык).
- Verbose error handling (`if err != nil`) — через 2 недели привычка.
- Generic’и молодые и ограниченные.

**Статус:** **основная рекомендация на волне 4+5**. Усилена AI‑агент‑контекстом.

#### TypeScript + Elysia + Bun + Drizzle (актуальный TS‑вариант)

**За:**
- **Eden Treaty — end‑to‑end типы без промежуточного OpenAPI‑codegen.** На backend описали endpoint → на Quasar `treaty<typeof app>(...)` даёт типизированный клиент. **Это лучше OpenAPI‑связки** по dev‑loop: меняешь контракт → TS в Vue падает сразу, без шага `just api-gen`.
- **Bun 1.2+** — быстрый рантайм, встроенные bundler/test runner/package manager, встроенный SQLite для локалдева.
- `bun build --compile` → **один бинарник ~40–60 МБ**, близко к Go по деплой‑простоте.
- Elysia — современный TS‑фреймворк с TypeBox‑валидацией, авто‑OpenAPI (для мобильного клиента на Dart), WebSocket‑primitives.
- **Монорепо с Quasar единым TS** — схемы валидации используются с обеих сторон.
- Drizzle ORM — ближе к SQL, предсказуем для AI‑агента, лучше Prisma в этом отношении.

**Против:**
- TypeScript ≠ полная статическая типизация: `any`, `as`, runtime‑JSON‑несоответствия.
- Bun‑экосистема к 2026 стабильна, но пограничные случаи остаются (native bindings в некоторых PDF/image‑библиотеках).
- Elysia моложе Fastify → в обучающем корпусе AI‑моделей реже → чуть выше вероятность галлюцинаций API (частично компенсируется отличной документацией + rules).
- Vendor‑lock к Bun: возврат на Node потребует правок при использовании `Bun.*` API напрямую.
- PDF: Puppeteer тянет Chromium (+300 МБ к образу).
- `any`/`as`‑пути всё равно есть — не уровень Go/Rust по жёсткости.

**Статус:** **сильная альтернатива №1 (Go) для случаев, где Eden‑типы с Quasar важнее жёсткости SQL/типизации.** Особенно если web‑часть занимает большую долю работы.

#### TypeScript + Fastify + Node + Drizzle (устаревшая формулировка TS‑варианта)

Был предложен как №2 до появления Elysia+Bun в обсуждении. **Elysia+Bun перекрывает его по всем параметрам** (Eden > OpenAPI‑codegen, Bun > Node, `bun build --compile` > Docker‑Node‑slim). Fastify остаётся разумным выбором только если нужна максимальная стабильность экосистемы и вы не готовы к Bun‑edge‑cases.

#### Rust + Axum + SQLx (+ опц. SeaORM)

**За:**
- Максимальная надёжность: `Option<T>`, `Result<T, E>`, borrow checker — целые классы ошибок невозможны.
- **SQLx compile‑time‑checked queries:** `cargo build` реально коннектится к dev‑БД и валидирует SQL — до запуска.
- Typst встраивается как Rust‑crate → самое изящное PDF‑решение из всех вариантов.
- Бинарник 5–15 МБ.

**Против:**
- **Кривая обучения 3–6 месяцев** для Kotlin‑инженера.
- **Плохо дружит с AI‑агентом:** длинные compile‑loops (10–60 сек инкрементально), сложные ошибки про lifetimes/Send/Sync, которые агент не всегда исправляет без помощи человека.
- Async‑экосистема всё ещё взрослеет.
- ORM‑экосистема беднее.

**Статус:** **отложить**. Для AI‑driven MVP — overkill. Хороший кандидат для отдельного системного проекта в будущем.

#### C# + ASP.NET Core 9 + EF Core + QuestPDF

**«Тёмная лошадка».**

**За:**
- Сильная типизация (nullable reference types, records, sum types на подходе).
- ASP.NET Core Minimal APIs — **не** enterprise по ощущению.
- **QuestPDF — лучшая PDF‑библиотека во всех рассмотренных стеках** (MIT, Fluent API, hot‑reload preview).
- NativeAOT → бинарь ~15 МБ, старт <100 мс.

**Против:**
- Microsoft‑экосистема ментально близка к Java‑enterprise, от которой инженер уходит.
- Корпус обучения AI‑моделей меньше TS/Python/Go.

**Статус:** **не рекомендован**, но осознанно — не из‑за слабости стека, а из‑за карьерного вектора инженера.

### 3.3 Отвергнутые сразу

- **Firebase / Supabase** — backend свой (явное требование sprint-1 мобилки §3.3). Supabase‑RLS плохо стыкуется с кастомным sync‑протоколом push/pull cursor.
- **MongoDB и любой NoSQL** — строго реляционные связи `Workspace → Project → Plan → Room → Wall`. JSONB в Postgres закрывает единственный нереляционный кейс (`Plan.payloadJson`).
- **Django / Django REST** — async вторичен, ORM и админка не нужны, FastAPI легче.
- **GraphQL / gRPC** — избыточно для 3–5 ресурсов.
- **Retrofit‑подобные генераторы** — для малого API не окупаются.

---

## 4. Рассмотренные web‑варианты

| Сценарий | Что на веб | Рекомендованный стек | Комментарий |
|---|---|---|---|
| **A.** Только estimator + просмотр проектов (как у Magicplan: online‑only) | CRUD смет, прайс‑листы, список проектов, чертёж = картинка | **HTMX + Jinja2 + Alpine.js + Tailwind** поверх того же FastAPI (если бы выбрали Python) | MVP за ~1 неделю, ноль дубликатов моделей. Жизнеспособно только при Python‑backend |
| **B.** Полноценный второй клиент (читать всё, админка) | + фото, просмотр плана, отчёты | **Next.js + React + shadcn/ui + TanStack Query** ИЛИ **Vue 3 + Quasar + Pinia** | См. ниже |
| **C.** Полный web‑редактор плана | + редактировать `Room`/`Wall` в браузере | **Flutter Web** (переиспользует Flame‑редактор и 100% модель/репозитории) | Отложить как можно дольше. Magicplan тоже редактор на web не даёт |

### Vue + Quasar vs React + Next.js (для сценария B)

| Аспект | Vue 3 + Quasar | Next.js + React + shadcn |
|---|---|---|
| Готовые компоненты для estimator (q‑table, q‑uploader, q‑form) | **Сильно** | Собираем из TanStack Table + shadcn Input‑ов |
| Тренировочный корпус AI‑модели | Средний | **Огромный** |
| Fluency AI‑агента в генерации | Средняя | **Высокая** |
| Зависимость от SSR (не нужен для админки) | **Нет, SPA из коробки** | Next.js — SSR‑first, полезные фичи уходят в SPA‑режиме |
| Монорепо с TS‑backend | Работает | Работает естественнее |
| Сообщество найма | Меньше | Больше |

**Решение инженера на волне 4:** Vue + Quasar. Осмысленное предпочтение, AI‑агент компенсируется готовыми компонентами (меньше писать с нуля) + правилами в `.cursor/rules/`.

**Важный архитектурный приём:** `quasar build` → статика → FastAPI/Go‑сервер отдаёт `dist/spa/` через `StaticFiles` с `/api/v1/...` для API. **Один контейнер, один деплой, нет CORS, cookie‑auth тривиально**.

---

## 5. Что меняется с estimator‑модулем (волна 2)

Добавляет серверную нагрузку, которой не было в sprint-1 мобилки §3:

| Подзадача | Требование | Влияние |
|---|---|---|
| PDF‑экспорт | HTML/CSS → PDF, кириллица, брендинг, разрывы страниц | Python (WeasyPrint) или .NET (QuestPDF) — лучшие; Go/Rust — через Typst (+сильный кандидат); TS — Puppeteer (heavy) |
| Excel‑экспорт | Стандартный `.xlsx`, формулы в итогах | Все стеки имеют зрелые библиотеки (openpyxl, excelize, exceljs, ClosedXML) |
| Decimal‑арифметика | Нельзя float для цены × количества | Native в Python/C#, через библиотеку в Go/JS |
| Фоновая генерация PDF | Не держим HTTP 3–5 сек | ARQ (Python), asynq/River (Go), BullMQ (TS) |
| Прайс‑листы | Импорт из Excel, категории | openpyxl первоклассный; остальные тоже работают |
| Шаблоны/версии смет | Clone, status machine (Draft/Sent/Approved/Rejected) | Чисто реляционное |
| Связь plan↔estimate | `quantitySource ∈ {manual, floor_area, wall_area, ...}` | Не автоматическая: ручная кнопка «обновить количества из плана», чтобы не нарушать sync LWW |

**Ключевое наблюдение:** у Magicplan сметы экспортируются **без чертежа**. Потенциальный дифференциатор RMA — включать thumbnail плана в PDF через `<img>` в шаблоне.

---

## 6. Что меняется с AI‑агентом как основным со‑разработчиком (волна 5)

Критерии переоценены:

| Критерий | Важность для человека | Важность для AI‑агента |
|---|---|---|
| Скорость compile/check‑loop | Средняя | **Критическая** (десятки итераций за задачу) |
| Качество сигнала компилятора | Высокая (через IDE) | **Критическая** (агент видит только stdout) |
| Плотность стека в обучающем корпусе | Низкая | **Высокая** (меньше галлюцинаций API) |
| Эксплицитность > краткость | Низкая | **Высокая** (магию агент часто путает) |
| Схема → codegen | Средняя | **Высокая** (расширяет систему типов на генерируемый код) |
| Стабильность API фреймворка | Средняя | **Высокая** (устаревшее знание → галлюцинации) |

### Практики, дающие AI‑cycle‑выигрыш (независимо от стека)

1. **Один `just check`** — форматирование + компиляция + линтинг + быстрые тесты, бинарный выход. Уже заложено в `global-check-commands.mdc`.
2. **Schema‑first + codegen** везде: sqlc (SQL→Go), OpenAPI (Go→TS‑клиент Quasar).
3. **Rules/skills под ваш стек:** `go-error-handling.mdc`, `sqlc-queries.mdc`, `http-handlers.mdc` и т. п. (по образцу существующих `.cursor/rules/`).
4. **Testcontainers, не моки** — агент хуже пишет корректные моки, чем использует реальные зависимости.
5. **Малые модули** — файлы до ~500 строк, один домен на файл.
6. **Документация `docs/planning/`** — агент читает перед генерацией, принимает согласованные решения. Не забрасывать эту практику при переходе на backend.
7. **OpenAPI как якорь синхронизации** между backend и Quasar‑frontend.

---

## 7. Итоговая рекомендация

### Финальный стек

| Роль | Выбор | Ключевые пакеты |
|---|---|---|
| **Backend** | **Go + chi + sqlc + pgx + goose + huma** | chi (роутинг), sqlc (SQL→типизированный Go), pgx (драйвер), goose (миграции), huma (typed OpenAPI), zerolog, golang-jwt/jwt/v5 |
| **Backend (Estimator — добавить позже)** | — | asynq/River (очередь фоновой генерации PDF), excelize (Excel-экспорт смет), Typst subprocess (PDF-генерация) |
| **БД** | **PostgreSQL 16+** | JSONB для `Plan.payloadJson`, UUID v7 генерируется на клиенте (sprint-1 мобилки §4.2) |
| **Файловое хранилище** | **S3‑совместимое** (MinIO локально, Cloudflare R2 / Backblaze B2 в проде) + **pre‑signed URL** (двухфазная загрузка фото) | Закрывает открытый вопрос sprint-1 мобилки §10: multipart vs pre‑signed |
| **Auth** | JWT access 15 мин + refresh 30 дней (sprint-1 мобилки §8), refresh‑таблица в БД, cookie‑session для web | `golang-jwt/jwt/v5` |
| **Web** | **Vue 3 + Quasar 2 + Pinia + openapi‑typescript + Vitest** | Оcознанный выбор инженера; AI‑fluency компенсируется готовыми компонентами + rules |
| **Монорепо** | pnpm workspace: `apps/api/` (Go), `apps/web/` (Quasar), `packages/api-contracts/` (OpenAPI + generated TS) | Атомарные рефакторинги backend+web |
| **Деплой** | Docker → Fly.io / Railway на MVP; Hetzner Cloud + Coolify при росте | Один бинарник Go (15–30 МБ) + `dist/spa/` от Quasar отдаётся тем же контейнером |
| **Очередь** | Redis + asynq или Postgres‑based **River** | Для фоновой генерации PDF |
| **Observability** | `zerolog` → JSON stdout; OpenTelemetry опционально потом | — |

### Почему именно так

1. **Go + sqlc** даёт самый чистый AI‑agent‑loop (быстрый compile + явные ошибки + compile‑time SQL‑safety) при сохранении строгой типизации и надёжности.
2. **Vue + Quasar** — сознательное предпочтение инженера; Quasar компенсирует меньшую AI‑fluency готовыми компонентами под estimator‑UI (q‑table, q‑uploader, q‑form).
3. **OpenAPI‑codegen** связывает два стека в одну типизированную систему — изменение контракта на backend мгновенно показывается на web как TS‑ошибка.
4. **PostgreSQL + S3** — стандарт отрасли, снимают класс вопросов про offline‑sync и бинарники.
5. **Typst для PDF** — современное элегантное решение, компенсирует единственный weak point Go‑стека.

### Серьёзная альтернатива №1 — **TS + Elysia + Bun + Drizzle**

Не победила, но **близко**. Выбрать её, если:
- **End‑to‑end типы backend↔Quasar через Eden Treaty важнее, чем compile‑time SQL‑safety** от sqlc.
- Вы ожидаете **много итераций на контракте API** между backend и web (быстрое меню полей, перестановки dto), и готовы платить шагом «OpenAPI регенерация» — Eden это устраняет.
- Хотите **один язык во всём проекте** (TS на backend, TS на Quasar‑web, можно даже shared‑пакеты с бизнес‑логикой валидации итогов сметы).

Стек в этом случае: **Bun 1.2+ + Elysia + Drizzle ORM + drizzle‑kit (миграции) + TypeBox (валидация+OpenAPI) + Puppeteer (PDF) + exceljs (Excel) + BullMQ/Bun‑queue (фон) + pg (драйвер Postgres)**. Деплой: `bun build --compile` → один бинарник.

### Точки пересмотра

Зафиксированы заранее, чтобы не метаться:

| Триггер | Действие |
|---|---|
| После этапа 4 (sync) Go‑разработка ощутимо медленнее ожидаемого | Переоценить переход на **Ktor + Exposed + Flyway** (не на Spring) |
| OpenAPI‑шаг «регенерировать TS‑клиент для Quasar» часто забывается/пропускается агентом | Серьёзно рассмотреть переход на **Elysia + Bun** — Eden Treaty снимает этот шаг |
| Typst‑шаблоны окажутся ограничивающими для дизайна смет | Переход на `chromedp` (HTML/CSS → PDF через headless Chrome) — ценой +300 МБ к образу |
| Появится потребность в тяжёлом ML (OCR сканов, классификация фото) | Вынести ML‑часть в отдельный Python/FastAPI‑микросервис рядом с Go‑монолитом |
| Shared‑types между backend и web станут регулярной болью | Перейти на **TS + Elysia + Bun** (не на Fastify) — монорепо с Vue + Eden |
| Команда вырастет и Vue‑найм станет узким местом | Миграция web на React + shadcn + Next.js в SPA‑режиме |

---

## 8. Открытые вопросы (к этому документу)

- **Формат хранения Typst‑шаблонов сметы** (`.typ`‑файлы в репо vs в БД для динамического редактирования) — решать на этапе PDF‑экспорта в будущем sprint-N (за рамками sprint 1).
- **Связь web‑деплоя с backend‑деплоем**: один контейнер (Go отдаёт статику Quasar) vs два (Quasar на Cloudflare Pages, Go на Fly.io). Решение — по готовности этапа 7 (Estimator v0 на web).
- **Monorepo‑инструмент**: pnpm workspace достаточно или нужен Turborepo для кеша билдов. Решить при первом появлении frontend‑пакета.
- **Schema‑first vs code‑first для OpenAPI**: huma предлагает code‑first (Go‑типы → spec); альтернатива — писать `openapi.yaml` руками и генерировать Go‑stubs через `oapi-codegen`. Решить при старте этапа 4 (REST API).
- **Kill‑switch для Python**: нужен ли изначально запасной Python‑микросервис под возможные ML‑фичи, или поднимать когда (и если) потребуется. По умолчанию — по факту.

---

## 9. Связанные документы

- `docs/planning/open-questions.md` — сквозные нерешённые вопросы по платформе (например, версионирование API).
- `docs/planning/sprint-1.md` — бэкенд-часть sprint 1: что именно должно быть готово на сервере к моменту, когда мобилка дойдёт до этапа 4.
- `rms_mobile/docs/planning/sprint-1.md` — клиентская часть sprint 1.
- `rms_mobile/docs/planning/data-model.md` — каноническая доменная схема.
- `rms_mobile/docs/planning/stage-3-photos.md` — пайплайн фото на этапе 3.
- `rms_mobile/docs/planning/plan-thumbnail.md` — генерация thumbnail плана (нужна и для PDF сметы).
- `.cursor/rules/global-check-commands.mdc` — стандарт `just check`, поддерживается этим решением.
