# Открытые вопросы (платформа)

Короткий реестр решений, которые ещё не зафиксированы в коде или в [`backend-stack-discussion.md`](backend-stack-discussion.md). Точечные вопросы, привязанные только к обсуждению стека, остаются в §8 того документа.

---

1. **Версионирование API** — как именно будем версионировать публичный REST (префикс пути `/v1`, заголовок `Accept-Version`, отдельный хост, политика сосуществования нескольких major при обновлении мобилки и web).

2. **`/photos`: multipart vs pre-signed PUT** — ~~открыт~~. **Решено в Этапе 1 sprint 1 (апрель 2026):** выбран pre-signed PUT. `POST /api/v1/photos` (multipart) удалён из контракта; вместо него — `POST /api/v1/photos/upload-url`, возвращающий `{ uploadUrl, method: "PUT", headers, expiresAt }`. Метаданные фото доставляются через `/sync/push` (`entityType=photo`). Fallback на multipart — рядом, не вместо — только если pre-signed окажется несовместимым с CORS или iOS (этап 5).

3. **Формат ответа `/sync/push`** — ~~открыт~~. **Решено в Этапе 1 sprint 1 (апрель 2026):** partial-success 200 (вариант B из `docs/knowledge/offline-first-sync.md §4`): `{ cursor, applied: [clientOpId], conflicts: [{clientOpId, reason, serverVersion}], errors: [{clientOpId, reason, message}] }`. HTTP 4xx не используется для конфликтов — всегда 200, клиент смотрит в `conflicts` / `errors`.
