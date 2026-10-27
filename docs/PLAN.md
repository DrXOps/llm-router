# LLM-Router — Implementation Plan

## Overview

`llm-router` is an HTTP reverse-proxy that sits between Claude Code and an Anthropic-compatible gateway (routerapi.ru). It intercepts `/v1/messages` requests, **automatically selects the best model** (Haiku 5.5 or DeepSeek V4 Pro) based on configurable rules, and forwards the request. Rules can be declarative YAML files or Starlark scripts — both loadable at runtime without recompilation.

---

## Этапы реализации

### 0. Инициализация репозитория и скелет модуля

**Файлы:**
- `.gitignore` — стандартный Go
- `go.mod` — `module github.com/user/llm-router`, Go 1.22+
- `cmd/router/main.go` — минимальная точка входа (hello world с флагом `-version`)
- `Makefile` — цели: `build`, `build-linux`, `build-win`, `test`, `vet`, `clean`

**Критерии готовности:**
- `go mod tidy` проходит
- `go vet ./...` чистый
- `go build ./cmd/router` даёт бинарник
- Коммит: `chore: scaffold module structure`

---

### 1. Пакет config: структуры, загрузка, встраивание дефолта, валидация

**Файлы:** `internal/config/`
- `config.go` — структуры `Config`, `Models`, `Routing`, `Logging`, `RuleDef`, `PhraseSet`
- `default.go` — `//go:embed` YAML с дефолтным конфигом
- `load.go` — функция `Load(path string) (*Config, error)`:
  - Пробует прочитать `config.yaml` рядом с бинарником
  - Если нет — использует embedded default, пишет `"using embedded default config"`
- `validate.go` — валидация:
  - `models` не пусты
  - `target` ссылаются на объявленные модели
  - `phrase_set` ссылаются на существующие наборы
  - `strict_validation` влияет на обработку Starlark-ошибок
- `config_test.go` — тесты загрузки, fallback, валидации

**Критерии готовности:**
- Без `config.yaml` сервер поднимается на дефолтах
- Ошибки валидации возвращаются пользователю
- `go test ./internal/config/ -v -race` зелёный
- Коммит: `feat(config): load config with embedded defaults and validation`

---

### 2. Пакет payload: разбор/сборка тела без потери полей

**Файлы:** `internal/payload/`
- `payload.go`:
  - `Parse(body []byte) (*Payload, error)` — минимальный разбор: читает `model`, `messages`
  - `Payload` хранит `Messages []Message`, `Model string`, `Raw map[string]any` (все остальные поля)
  - `Rebuild(p *Payload, newModel string) []byte` — собирает тело запроса, заменяя `model`
  - `Message` — `struct { Role string; Content any }` (Content остаётся сырым)
- `tokens.go` — `ApproxTokens(text string) int` (4 символа ≈ 1 токен, грубая оценка)
- `payload_test.go` — тесты roundtrip: разобрать → заменить модель → собрать → JSON совпадает

**Критерии готовности:**
- Все поля запроса (system, tools, stop_sequences, stream, temperature, и т.д.) сохраняются
- Поле `model` меняется, остальное — без изменений
- `go test -race` зелёный
- Коммит: `feat(payload): parse and rebuild Anthropic messages without field loss`

---

### 3. Пакет router: интерфейс Rule, встроенные типы правил

**Файлы:** `internal/router/`
- `router.go`:
  - `Rule` interface: `Eval(payload, ctx) Result`
  - `Result` = `{Match bool, Target string, Reason string}`
  - `Router` — агрегатор: `AddRule`, `Eval(payload) Result`
- `builtin.go` — реализации всех 6 декларативных типов:
  1. `has_tool_calls` — ищет `tool_use`/`tool_result` в `messages[].content`
  2. `context_length` — `params.threshold_tokens`
  3. `keyword_match` — `params.phrase_set`
  4. `short_question` — `params.max_tokens`
  5. `regex_match` — `params.pattern`, `field: last_user_message|any_user_message`
  6. `always` — тривиальный match
- `context.go` — `Ctx` struct с информацией о запросе (обёртка над payload)
- `router_test.go` — тесты каждого типа правила

**Критерии готовности:**
- Каждый тип правила протестирован отдельно
- `Eval` возвращает первое сработавшее правило по priority
- Коммит: `feat(router): add built-in rule types`

---

### 4. Загрузчик декларативных YAML-правил из rules_dir

**Файлы:** `internal/router/loader.go`
- `LoadRulesDir(dir string, cfg *Config) ([]Rule, error)`:
  - Сканирует `*.yaml`, `*.yml` в директории
  - Парсит YAML в структуру `RuleDef`
  - Создаёт соответствующий `Rule` по `type`
  - Сортирует по `priority`
- Встроенные правила из конфига (`routing.rules`) подмешиваются с priority 0..N
- Валидация: target проверяется, phrase_set проверяется
- Если `rules_dir` не существует — warning, работа на встроенных

**Критерии готовности:**
- Загрузка из примеров `rules.d/`
- Правильный порядок сортировки
- Некорректные файлы логируются, не валят сервер
- Коммит: `feat(router): load declarative YAML rules from rules_dir`

---

### 5. Starlark-движок: predeclared окружение, таймаут, обработка ошибок

**Файлы:** `internal/router/`
- `starlark.go` — загрузка `.star` файлов, вызов `rule(payload, ctx)`:
  - `NewStarlarkRule(filename string, src string) Rule`
  - Компиляция при старте (или hot-reload)
  - Таймаут 100 мс на выполнение
- `builtins_env.go` — predeclared окружение:
  - `payload` — dict из JSON тела
  - `ctx` — Starlark struct со всеми полями из спецификации
  - `ctx.log(msg)` — пишет в slog.Debug
- `starlark_test.go`:
  - Рабочий скрипт
  - Скрипт с ошибкой компиляции
  - Скрипт с бесконечным циклом (таймаут)
  - Проверка predeclared окружения

**Критерии готовности:**
- Starlark-правило может принять решение
- Таймаут 100 мс работает
- Ошибки не валят сервер (без strict_validation)
- Коммит: `feat(router): add Starlark engine with sandboxed environment`

---

### 6. Пакет sanitizer: очистка thinking-блоков для DeepSeek

**Файлы:** `internal/sanitizer/`
- `sanitizer.go` — `Sanitize(payload) Payload`:
  - Удаляет `<thinking>...</thinking>` блоки из `content`
  - DeepSeek может не поддерживать thinking блоки от Claude
  - Работает на `[]byte` уровне (через regexp или простой парсинг)
- `sanitizer_test.go`:
  - Разные варианты thinking-блоков
  - Вложенные скобки (если возможны)

**Критерии готовности:**
- Thinking-блоки удаляются только если target = DeepSeek
- Остальной контент не повреждается
- Коммит: `feat(sanitizer): strip thinking blocks for DeepSeek`

---

### 7. Пакет proxy: ReverseProxy с подменой модели

**Файлы:** `internal/proxy/`
- `proxy.go`:
  - `Handler(cfg *Config) http.Handler`
  - Разбирает запрос (`/v1/messages`)
  - Собирает тело через `payload.Parse`
  - Прогоняет через `router.Eval(payload)` → получает модель
  - Применяет `sanitizer.Sanitize` если нужно
  - Пересобирает тело с новой моделью через `payload.Rebuild`
  - Проксирует запрос в `cfg.Upstream.BaseURL`
  - Проксирует ответ обратно
  - Логирует выбранную модель
- `proxy_test.go`:
  - `httptest.NewServer` — полный end-to-end тест
  - Запрос приходит → модель выбрана → ответ возвращён

**Критерии готовности:**
- Запрос к `/v1/messages` корректно проксируется
- Модель логируется в structured log
- Другие пути (не `/v1/messages`) возвращают 404
- Коммит: `feat(proxy): add reverse proxy with model rewriting`

---

### 8. Опциональный SLM: gomlx-обвязка с graceful degradation

**Файлы:** `internal/slm/`
- `classifier.go`:
  - Загрузка ONNX-модели через gomlx
  - `Classify(payload) string` — возвращает "haiku", "deepseek" или "" (no opinion)
  - Всё опционально: флаг `routing.enable_slm_classifier`
  - Если модель не загрузилась — тихий fallback на правила
- `classifier_test.go` — тесты с mock

**Примечание:** Этот этап помечен как опциональный. Если gomlx/onnx-gomlx не собирается с CGO_ENABLED=0 или вызывает проблемы, SLM-пакет существует как заглушка с graceful degradation.

**Критерии готовности:**
- Сервер работает без SLM
- Если SLM включён, но модель не найдена — warning, работа продолжается
- Коммит: `feat(slm): optional local ONNX classifier with graceful degradation`

---

### 9. Hot-reload правил по SIGHUP / watcher

**Файлы:** `internal/router/watcher.go`, `cmd/router/main.go` (изменение)
- `Watcher` — горутина, которая:
  - По умолчанию: ничего не делает (правила загружены при старте)
  - Если `routing.watch_rules: true`:
    - На Linux/macOS: слушает SIGHUP, перезагружает правила
    - На Windows: per-request перезагрузка если файлы изменились (fsnotify или простой stat-based check)
  - На Windows нет SIGHUP, поэтому можно polling (min 5s)

**Критерии готовности:**
- После SIGHUP новые правила применяются без рестарта
- На Windows: polling-based hot-reload с интервалом ≥ 5s
- Коммит: `feat(router): hot-reload rules on SIGHUP`

---

### 10. Makefile, .gitignore, CI

**Файлы:**
- `Makefile`:
  - `build-linux`: `GOOS=linux GOARCH=amd64 CGO_ENABLED=0 go build -o llm-router ./cmd/router`
  - `build-win`: `GOOS=windows GOARCH=amd64 CGO_ENABLED=0 go build -o llm-router.exe ./cmd/router`
  - `test`: `go test ./... -race -count=1`
  - `vet`: `go vet ./...`
  - `clean`: удаление бинарников
- `.github/workflows/ci.yml`:
  - Триггер: push, PR
  - Матрица: ubuntu-latest, windows-latest
  - Steps: checkout, setup Go, go vet, go test, make build-linux (или build-win)

**Критерии готовности:**
- Обе сборки дают статические бинарники
- CI зелёный на обеих ОС
- Коммит: `ci: add GitHub Actions for vet, test, cross-build`

---

### 11. Документация и примеры

**Файлы:**
- `docs/RULES.md` — как писать правила (все типы, Starlark, примеры)
- `docs/CONFIG.md` — полный справочник по конфигу
- `docs/USAGE.md` — как собрать, настроить Claude Code, проверить
- `docs/ARCHITECTURE.md` — диаграмма потока, описание слоёв
- `rules.d/` — 5 примеров правил (4 YAML + 1 Starlark)
- `config.yaml.example` — пример конфига со всеми комментариями
- `config.default.yaml` — embedded default
- `README.md` — быстрый старт

**Критерии готовности:**
- Документация покрывает все разделы из спецификации
- Примеры в `rules.d/` рабочие (проходят тесты)
- Коммит: `docs: add RULES, CONFIG, USAGE, ARCHITECTURE`

---

### 12. Финальная проверка и релиз

**Чек-лист:**
- [ ] `go vet ./...` — чистый
- [ ] `go test ./... -race -count=1` — все тесты проходят
- [ ] `make build-linux` — статический бинарник Linux amd64
- [ ] `make build-win` — статический бинарник Windows amd64
- [ ] Без `config.yaml` — сервер стартует на дефолтах
- [ ] Без `rules.d/` — сервер работает на встроенных правилах
- [ ] Ключ API только из env
- [ ] `git log --oneline` — осмысленные этапы
- [ ] Тег `v0.1.0`

**Коммит:** `chore: v0.1.0 release`
**Тэг:** `git tag -a v0.1.0 -m "Initial release: llm-router v0.1.0"`

---

## Последовательность коммитов

```
docs: add detailed implementation plan
chore: scaffold module structure
feat(config): load config with embedded defaults and validation
feat(payload): parse and rebuild Anthropic messages without field loss
feat(router): add built-in rule types
feat(router): load declarative YAML rules from rules_dir
feat(router): add Starlark engine with sandboxed environment
feat(sanitizer): strip thinking blocks for DeepSeek
feat(proxy): add reverse proxy with model rewriting
feat(slm): optional local ONNX classifier with graceful degradation
feat(router): hot-reload rules on SIGHUP
ci: add GitHub Actions for vet, test, cross-build
docs: add RULES, CONFIG, USAGE, ARCHITECTURE
chore: v0.1.0 release
```