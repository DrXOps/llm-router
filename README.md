# LLM-Router

Умный HTTP reverse-proxy для маршрутизации запросов Claude Code
между моделями Haiku 5.5 и DeepSeek V4 Pro через routerai.ru.

Автоматически выбирает модель на основе NLP-классификатора + правил.
**Не требует конфигурации — работает из коробки.**

## Быстрый старт

```bash
# Скачать готовый бинарник (Linux amd64)
curl -LO https://github.com/DrXOps/llm-router/releases/download/v0.1.0/llm-router-linux
chmod +x llm-router-linux

# Запустить
export LLM_ROUTER_API_KEY=ваш_ключ_routerai
./llm-router-linux
```

Claude Code:
```bash
export ANTHROPIC_BASE_URL=http://localhost:8080
export ANTHROPIC_AUTH_TOKEN=""
claude
```

## Проверка что работает

```bash
curl http://localhost:8080/health
# → {"status":"ok","version":"0.1.0","rules_count":1,"classifier":"disabled","upstream":"https://routerai.ru"}
```

## Документация

- [USAGE](docs/USAGE.md) — как установить, настроить Claude Code
- [RULES](docs/RULES.md) — как писать правила маршрутизации
- [CONFIG](docs/CONFIG.md) — справочник по конфигурации
- [ARCHITECTURE](docs/ARCHITECTURE.md) — как устроен поток запроса

## Особенности

- 🚀 **Готовый бинарник** — скачал и запустил, без Go, без сборки
- 📦 **Zero config** — работает из коробки с embedded дефолтами
- 🧠 **NLP-классификатор** — понимает смысл запроса (русский + английский)
- 🔧 **Гибкие правила** — YAML для простых случаев, Starlark для сложных
- 🔒 **Sandbox** — Starlark-скрипты в песочнице, без доступа к IO/сети
- ♻️ **Hot-reload** — меняй правила без перезапуска
- 📊 **Health endpoint** — `/health` для мониторинга
- ⚡ **CGO_ENABLED=0** — статический бинарник, работает везде