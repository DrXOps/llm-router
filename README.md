# LLM-Router

Умный HTTP reverse-proxy для маршрутизации запросов Claude Code
между моделями Haiku 5.5 и DeepSeek V4 Pro через routerapi.ru.

Автоматически выбирает модель на основе правил (YAML + Starlark),
без участия пользователя.

## Быстрый старт

```bash
# Собрать
make build-linux

# Запустить (с дефолтным конфигом)
export LLM_ROUTER_API_KEY=ваш_ключ
./llm-router

# Настроить Claude Code
export ANTHROPIC_BASE_URL=http://localhost:8080
export ANTHROPIC_AUTH_TOKEN=""   # обязательно пустой
```

## Документация

- [USAGE.md](docs/USAGE.md) — как установить и запустить
- [CONFIG.md](docs/CONFIG.md) — справочник по конфигурации
- [RULES.md](docs/RULES.md) — как писать правила маршрутизации
- [ARCHITECTURE.md](docs/ARCHITECTURE.md) — как всё устроено
- [PLAN.md](docs/PLAN.md) — план реализации

## Особенности

- 🚀 **Один бинарник** — статическая сборка, CGO_ENABLED=0
- 📦 **Zero config** — работает из коробки с embedded дефолтами
- 🔧 **Гибкие правила** — YAML для простых случаев, Starlark для сложных
- 🔒 **Sandbox** — Starlark-скрипты не имеют доступа к IO/сети
- ♻️ **Hot-reload** — меняйте правила без перезапуска
- 📊 **Структурированные логи** — log/slog в stderr

## Команды

```bash
make build-linux    # Сборка для Linux amd64
make build-win      # Сборка для Windows amd64
make test           # Запуск тестов
make vet            # Статический анализ
```