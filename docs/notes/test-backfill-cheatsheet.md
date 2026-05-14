# Test Backfill Cheatsheet

## Как выбирать тест

- Один тестовый кандидат должен проверять один инвариант.
- Инвариант должен формулироваться одним предложением.
- Приоритет задает цена бага, а не прирост coverage.
- Хороший кандидат лежит в hot path: auth, sync, photos, middleware, jwtutil.
- Тест должен ловить правдоподобную мутацию реализации.

## Workspace Isolation

- Это security invariant, а не обычная validation.
- Источник `workspaceId` — только access JWT через middleware.
- `push`: запрос с токеном workspace A не должен менять данные workspace B.
- `pull`: запрос с токеном workspace A не должен видеть данные workspace B.
- Для `push` проверяем `forbidden` и отсутствие записи.
- Для `pull` проверяем реальные SQL-фильтры интеграционно против Postgres.

## Почему ИИ может пропустить

- ИИ часто проверяет happy path, validation, `notFound` и LWW.
- Опасный кейс выглядит как обычный existing entity by ID.
- Без вопроса "а чей это workspace?" тесты могут быть зелеными при утечке данных.

## Следующие кандидаты

- `pushProject`: чужой workspace => `forbidden`.
- `pushPlan`: чужой parent project или existing plan => `forbidden`.
- `pushRoom`: чужой parent plan или existing room => `forbidden`.
- `pushWall`: чужой parent room или existing wall => `forbidden`.
- `pushPhoto`: чужой parent или existing photo => `forbidden`.
- `pull`: чужие project, plan, room, wall, photo не появляются в response.
