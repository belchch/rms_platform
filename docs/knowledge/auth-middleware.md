# Auth Middleware (BearerWorkspace)

В RMS Platform используется `huma` (v2) поверх `chi` для построения API.
Источником истины для контракта API является файл `packages/api-contracts/openapi.yaml`.

## Проблема chi-middleware
Ранее валидация JWT-токена выполнялась на уровне `chi` middleware (`router.Use(...)`). Это приводило к двум архитектурным проблемам:
1. **Хардкод публичных путей:** В middleware приходилось вручную проверять строковые пути (через `strings.HasPrefix` и т.д.), чтобы пропустить публичные роуты (например, `/health`, `/api/v1/auth/`). Это дублировало роутинг и ломало связь со спецификацией OpenAPI.
2. **Некорректный формат ошибок:** При ошибке авторизации (отсутствует токен, невалидный токен) middleware использовал `http.Error`, возвращающий `text/plain`. Клиенты же, согласно спецификации OpenAPI (`ErrorResponse`), ожидают `application/json`.

## Решение: Huma-middleware
Для централизации логики и соблюдения контракта OpenAPI валидация токена перенесена на уровень Huma (`api.UseMiddleware(...)`). 

Используется подход **Открыто по умолчанию (Public by default)**, что соответствует структуре нашего `openapi.yaml` (глобального блока `security` на верхнем уровне нет, он указывается только для конкретных путей).

### Как это работает

1. **Определение необходимости авторизации (без хардкода путей):**
   Huma-middleware имеет доступ к контексту уже распознанной операции (`ctx.Operation()`). Middleware просто проверяет поле `Security` у текущей операции:
   - Если `len(op.Security) == 0` — операция публичная, пропускаем запрос без проверок (вызываем `next(ctx)`).
   - Если в `Security` есть требование `"bearerAuth"` — извлекаем заголовок `Authorization`, парсим JWT токен и помещаем `workspaceId` в контекст.

2. **Ошибки в правильном формате:**
   В случае проблем с токеном middleware использует функцию `huma.WriteErr(api, ctx, http.StatusUnauthorized, "Unauthorized")`, что гарантирует автоматическое формирование корректного JSON (`ErrorResponse`), ожидаемого клиентами.

### Правила регистрации роутов

1. **Публичные ручки** (например, `/health`, `/api/v1/auth/sign-in`):
   При регистрации через `huma.Register` поле `Security` оставляется пустым.
   
2. **Защищённые ручки** (всё остальное API):
   При регистрации **обязательно** указывать схему `bearerAuth`, чтобы Huma-middleware понял, что здесь нужен токен, и чтобы генератор OpenAPI добавил замочек в документацию:
   ```go
   huma.Register(api, huma.Operation{
       OperationID: "sync-push",
       Path:        "/api/v1/sync/push",
       // ...
       Security: []map[string][]string{
           {"bearerAuth": {}}, // Явное требование авторизации
       },
   }, push)
   ```

### Чек-лист реализации (инструкция для агентов)
При внедрении или изменении Auth Middleware:
1. Изменить сигнатуру middleware: `func BearerWorkspace(api huma.API, secret string) func(ctx huma.Context, next func(huma.Context))`
2. Подключить middleware к API: `api.UseMiddleware(middleware.BearerWorkspace(api, cfg.JWTSecret))` в `cmd/api/main.go`.
3. **Удалить** старую функцию `bypassBearerAuth` (хардкод путей больше не нужен).
4. Запрашивать операцию в middleware: `op := ctx.Operation()`. Проверять необходимость авторизации через `len(op.Security) == 0`.
5. Возвращать ошибки **только** через `huma.WriteErr(...)`.
6. Сохранять `workspaceId` в контекст через `ctx = huma.WithValue(ctx, workspaceIDCtxKey{}, claims.WorkspaceID)`.
7. Во всех защищённых обработчиках (`/sync/push`, `/sync/pull`, `/photos/upload-url`) добавить `Security: []map[string][]string{{"bearerAuth": {}}}` в параметры `huma.Register`.
8. Убедиться, что в публичных обработчиках (`sign-in`, `refresh`, `health`) поле `Security` в `huma.Register` не заполняется.
