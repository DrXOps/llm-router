# Rules: How Model Selection Works

## Обзор

Правила определяют, какую модель использовать для каждого запроса.
LLM-router оценивает правила по порядку приоритета и выбирает модель
по **первому сработавшему правилу** (stop-on-first-match).

Правила живут в `rules.d/` как отдельные файлы. Можно менять их без
перекомпиляции бинарника. Два формата:
- **YAML** — декларативный, для типовых сценариев (ключевые слова,
  длина контекста, tool calls)
- **Starlark** — скриптовый, для сложной логики (Python-подобный
  синтаксис, полный sandbox)

## Порядок применения

1. Встроенные правила из конфига (`routing.rules`), если
   `include_builtin_rules: true` — priority 0..N
2. Файлы из `rules.d/` — сортируются по (`priority`, имя файла)
3. Первое сработавшее правило определяет модель
4. Если ни одно правило не сработало — используется fallback Haiku

## Декларативные YAML-правила

### always

Всегда срабатывает. Используется как fallback.

```yaml
name: "always_haiku"
priority: 9999
enabled: true
type: "always"
target: "haiku"
description: "Fallback: all requests to Haiku"
```

Параметры: нет.

### has_tool_calls

Срабатывает, если в истории есть tool_use или tool_result.

```yaml
name: "tools_to_deepseek"
priority: 10
enabled: true
type: "has_tool_calls"
target: "deepseek"
description: "Requests with tool calls → DeepSeek"
```

Параметры: нет.

### context_length

Срабатывает, если примерное количество токенов превышает порог.

```yaml
name: "long_context"
priority: 20
enabled: true
type: "context_length"
target: "deepseek"
params:
  threshold_tokens: 2000
description: "Long conversations → DeepSeek"
```

Параметры:
- `threshold_tokens` (int, обязательный) — порог токенов

### keyword_match

Срабатывает, если любое из сообщений пользователя содержит фразу
из заданного набора.

```yaml
name: "engineering_deepseek"
priority: 30
enabled: true
type: "keyword_match"
target: "deepseek"
params:
  phrase_set: "complex_engineering"
description: "Engineering keywords → DeepSeek"
```

Набор фраз определяется в config.yaml:
```yaml
phrases:
  - name: "complex_engineering"
    phrases:
      - "refactor"
      - "architecture"
      - "technical debt"
```

Параметры:
- `phrase_set` (string, обязательный) — имя набора из config.yaml

### short_question

Срабатывает, если последнее сообщение пользователя короткое.

```yaml
name: "short_to_haiku"
priority: 40
enabled: true
type: "short_question"
target: "haiku"
params:
  max_tokens: 10
description: "Short questions → Haiku (faster)"
```

Параметры:
- `max_tokens` (int, обязательный) — максимальное количество токенов

### regex_match

Срабатывает, если найдено совпадение с регулярным выражением.

```yaml
name: "code_references"
priority: 50
enabled: true
type: "regex_match"
target: "deepseek"
params:
  pattern: "file:\\w+\\.go|TODO|FIXME"
  field: "any_user_message"
description: "Code references → DeepSeek"
```

Параметры:
- `pattern` (string, обязательный) — регулярное выражение (RE2 syntax)
- `field` (string, опциональный) — `"last_user_message"` или
  `"any_user_message"` (по умолчанию)

## Starlark-правила

Starlark — Python-подобный язык, выполняющийся в песочнице.
Нет доступа к IO, сети, времени, random, os, exec.

### Структура файла

```python
# rules.d/50-custom.star

def rule(payload, ctx):
    """Возвращает dict: {match: bool, target: str, reason: str}"""
    # payload — словарь с телом запроса
    # ctx — контекст с информацией о запросе
    return {"match": False}
```

### Поля ctx

| Поле | Тип | Описание |
|------|-----|----------|
| `last_user_message` | string | Текст последнего сообщения пользователя |
| `all_user_messages` | list of string | Все сообщения пользователя |
| `message_count` | int | Количество сообщений в истории |
| `total_chars` | int | Общее количество символов |
| `approx_tokens` | int | Примерное количество токенов |
| `has_tool_calls` | bool | Есть ли tool_use/tool_result в истории |
| `model_ids` | dict | Словарь {target: actual_model_id} |
| `log(msg)` | function | Пишет сообщение в debug-лог |

### Примеры Starlark-правил

**Пример 1: длинная беседа с TODO**

```python
def rule(payload, ctx):
    msgs = ctx.get("all_user_messages", [])
    if len(msgs) > 5 and "TODO" in ctx.get("last_user_message", ""):
        return {
            "match": True,
            "target": "deepseek",
            "reason": "long conversation with TODO marker",
        }
    return {"match": False}
```

**Пример 2: файловые пути в сообщении**

```python
def rule(payload, ctx):
    import re
    msg = ctx.get("last_user_message", "")
    if re.search(r"[\w]+\.go:\d+", msg):
        return {
            "match": True,
            "target": "deepseek",
            "reason": "Go file reference detected",
        }
    return {"match": False}
```

**Пример 3: балансировка по длине контекста**

```python
def rule(payload, ctx):
    tokens = ctx.get("approx_tokens", 0)
    count = ctx.get("message_count", 0)
    if tokens > 3000:
        return {
            "match": True,
            "target": "deepseek",
            "reason": f"very long context: {tokens} tokens",
        }
    if tokens < 20 and count < 3:
        return {
            "match": True,
            "target": "haiku",
            "reason": "simple short conversation",
        }
    return {"match": False}
```

## Отладка

Включите в config.yaml:
```yaml
logging:
  level: debug
  log_model_choice: true   # Логировать выбранную модель
  log_request_body: true   # Логировать тело запроса (осторожно: может быть большим)
```

Формат логов:
```
INFO model selected target=deepseek model_id=deepseek-v4-pro
     reason="conversation contains tool calls"
```

## Рецепты

### Все запросы с "bug" в DeepSeek

```yaml
name: "bug_to_deepseek"
priority: 10
enabled: true
type: "keyword_match"
target: "deepseek"
params:
  phrase_set: "bug_patterns"
description: "Bug reports → DeepSeek"
```

```yaml
# в config.yaml
phrases:
  - name: "bug_patterns"
    phrases:
      - "bug"
      - "error"
      - "crash"
      - "exception"
```

### Регулярка на пути к файлам

```yaml
name: "file_paths"
priority: 15
enabled: true
type: "regex_match"
target: "deepseek"
params:
  pattern: "[a-zA-Z]:\\\\(?:[^\\\\]+\\\\)*[a-zA-Z0-9_]+\\.[a-z]+"
  field: "last_user_message"
```

### Учет длины ответа ассистента

Этот сценарий требует Starlark (т.к. нужно анализировать
предыдущие ответы):

```python
def rule(payload, ctx):
    # payload["messages"] содержит всю историю
    msgs = payload.get("messages", [])
    for msg in msgs:
        if msg.get("role") == "assistant":
            content = str(msg.get("content", ""))
            if len(content) > 500:
                return {
                    "match": True,
                    "target": "deepseek",
                    "reason": "previous assistant response very long",
                }
    return {"match": False}
```

## FAQ

**Почему правило не сработало?**
- Проверьте приоритет: другое правило могло сработать раньше
- Проверьте `enabled: true`
- Для keyword_match: убедитесь, что phrase_set есть в config.yaml
- Для Starlark: включите `logging.level: debug` и проверьте сообщения

**Ошибка Starlark — что делать?**
- Включите `strict_validation: false` в конфиге — сервер пропустит
  сломанное правило и продолжит работу
- При `strict_validation: true` сервер завершится с ошибкой,
  показывая файл, строку и сообщение

**Как временно отключить правило?**
- YAML: поставьте `enabled: false`
- Starlark: переименуйте файл в `*.disabled`

## Версионирование

Изменения в predeclared окружении Starlark (поля ctx, новые функции)
считаются **breaking change** и требуют обновления документации.