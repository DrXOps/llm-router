# Configuration Reference

LLM-router использует один YAML-файл конфигурации. По умолчанию
ищет `config.yaml` рядом с бинарником. Если файла нет —
используется встроенный дефолт (embedded).

## Полная схема

```yaml
models:
  haiku:
    id: "claude-haiku-5-5"
    name: "Haiku 5.5"
  deepseek:
    id: "deepseek-v4-pro"
    name: "DeepSeek V4 Pro"

phrases:
  - name: "complex_engineering"
    phrases:
      - "refactor"
      - "architecture"

routing:
  rules_dir: "./rules.d"
  include_builtin_rules: true
  stop_on_first_match: true
  strict_validation: false
  watch_rules: false
  enable_slm_classifier: false
  rules:
    - name: "always_haiku_fallback"
      priority: 9999
      enabled: true
      type: "always"
      target: "haiku"
      description: "Fallback to Haiku"

upstream:
  base_url: "https://routerapi.ru"

logging:
  level: info          # debug | info | warn | error
  log_model_choice: true
  log_request_body: false
```

## Секции

### models

Тип: `map[string]object` — обязательный

Определяет модели, между которыми роутер выбирает.
Ключ — короткое имя (target), значение — объект с полями:

- `id` (string, обязательный) — идентификатор модели в API
- `name` (string, опциональный) — человекочитаемое имя

### phrases

Тип: `[]object` — опциональный

Наборы ключевых фраз для правила `keyword_match`.

- `name` (string, обязательный) — уникальное имя набора
- `phrases` ([]string, обязательный) — список фраз (регистронезависимый поиск)

### routing

Тип: `object` — опциональный

Управление маршрутизацией.

| Поле | Тип | Дефолт | Описание |
|------|-----|--------|----------|
| `rules_dir` | string | `"./rules.d"` | Директория с файлами правил |
| `include_builtin_rules` | bool | `true` | Подмешивать правила из `routing.rules` |
| `stop_on_first_match` | bool | `true` | Всегда true в v1 |
| `strict_validation` | bool | `false` | При true — ошибка Starlark валит сервер |
| `watch_rules` | bool | `false` | Hot-reload правил по polling (≥5s) |
| `enable_slm_classifier` | bool | `false` | Включить ONNX-классификатор |
| `rules` | []RuleDef | — | Встроенные правила (см. RULES.md) |

### upstream

Тип: `object` — опциональный

- `base_url` (string, дефолт: `"https://routerapi.ru"`) — базовый URL

### logging

Тип: `object` — опциональный

| Поле | Тип | Дефолт | Описание |
|------|-----|--------|----------|
| `level` | string | `"info"` | `debug`, `info`, `warn`, `error` |
| `log_model_choice` | bool | `true` | Логировать выбранную модель |
| `log_request_body` | bool | `false` | Логировать тело запроса |