# Architecture

## Поток запроса

```
┌─────────────┐     POST /v1/messages      ┌──────────────┐
│  Claude     │ ──────────────────────────▶ │  llm-router  │
│  Code       │                             │  :8080       │
│  (client)   │ ◀────────────────────────── │              │
└─────────────┘      ответ                  └──────┬───────┘
                                                   │
                                          ┌────────┴────────┐
                                          │   payload.Parse  │
                                          │   (читает model, │
                                          │    messages, raw)│
                                          └────────┬────────┘
                                                   │
                                          ┌────────┴────────┐
                                          │  router.Eval()  │
                                          │  ┌────────────┐ │
                                          │  │ rules.d/   │ │
                                          │  │ *.yaml     │ │
                                          │  │ *.star     │ │
                                          │  └────────────┘ │
                                          └────────┬────────┘
                                                   │
                                          ┌────────┴────────┐
                                          │ sanitizer       │
                                          │ (strip thinking │
                                          │  для DeepSeek)  │
                                          └────────┬────────┘
                                                   │
                                          ┌────────┴────────┐
                                          │ payload.Rebuild │
                                          │ (замена model)  │
                                          └────────┬────────┘
                                                   │
                                          ┌────────┴────────┐
                                          │  ReverseProxy   │
                                          │  → routerapi.ru │
                                          └─────────────────┘
```

## Слои

### 1. proxy (HTTP handler)

Точка входа. Принимает POST `/v1/messages`, читает тело,
передаёт через остальные слои, проксирует ответ.

### 2. payload (парсинг/сборка)

Разбирает JSON-тело запроса. Сохраняет все поля, которые не
меняет (system, tools, stream, max_tokens, temperature, и т.д.)
в `Raw` map.

### 3. router (правила)

Агрегатор правил. Получает контекст из payload и возвращает
результат маршрутизации. Правила загружаются из:
- Встроенных в конфиг
- `rules.d/*.yaml`
- `rules.d/*.star`

### 4. sanitizer (фильтрация)

При target=deepseek удаляет `<thinking>` блоки из сообщений.
DeepSeek может не поддерживать формат thinking-блоков Claude.

### 5. upstream (API gateway)

Конечная точка — routerapi.ru. Ключ API передаётся в заголовке
`X-Api-Key`.

## Где добавлять

### Новое декларативное правило

1. Добавить реализацию в `internal/router/builtins_env.go`
   (или `internal/router/router.go` на данный момент)
2. Добавить case в `BuildBuiltinRule`
3. Добавить тест
4. Задокументировать в `docs/RULES.md`

### Новый тип payload (чтение нового поля)

1. Добавить поле в `Payload` структуру
2. Добавить чтение в `Parse`
3. Добавить helper-метод (как `HasToolCalls`)
4. Добавить поле в `PayloadCtx`
5. Связать в proxy handler