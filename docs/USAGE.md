# Usage: Как установить и запустить

## 🚀 Быстрый старт (1 минута)

### Linux / macOS

```bash
# 1. Скачать готовый бинарник
curl -LO https://github.com/DrXOps/llm-router/releases/download/v0.1.0/llm-router-linux
chmod +x llm-router-linux

# 2. Запустить
export LLM_ROUTER_API_KEY=ваш_ключ_routerai
./llm-router-linux

# 3. Настроить Claude Code (в другом терминале)
export ANTHROPIC_BASE_URL=http://localhost:8080
export ANTHROPIC_AUTH_TOKEN=""
claude
```

### Windows (PowerShell)

```powershell
# 1. Скачать готовый бинарник
curl.exe -LO https://github.com/DrXOps/llm-router/releases/download/v0.1.0/llm-router-win.exe

# 2. Запустить
$env:LLM_ROUTER_API_KEY = "ваш_ключ_routerai"
.\llm-router-win.exe

# 3. Настроить Claude Code (в другом окне)
$env:ANTHROPIC_BASE_URL = "http://localhost:8080"
$env:ANTHROPIC_AUTH_TOKEN = ""
claude
```

## 🧪 Проверка что работает

```bash
# Статус роутера
curl http://localhost:8080/health
# → {"status":"ok","version":"0.1.0","rules_count":5,"classifier":"disabled"}

# Тестовый запрос (без Claude Code)
curl -X POST http://localhost:8080/v1/messages \
  -H "Content-Type: application/json" \
  -H "X-Api-Key: $LLM_ROUTER_API_KEY" \
  -d '{"messages":[{"role":"user","content":"Say hello in Russian"}],"max_tokens":50}'
```

## 📋 Где брать ключ

Зарегистрируйтесь на **[routerai.ru](https://routerai.ru)** и получите API-ключ.

## ⚙️ Сборка из исходников (если нужна)

```bash
git clone https://github.com/DrXOps/llm-router
cd llm-router
make build-linux    # Linux amd64
make build-win      # Windows amd64
```

## 🧠 Как понять какую модель выбрал роутер

В логах:

```
INFO model selected target=haiku model_id=claude-haiku-5-5 reason="always rule matched"
INFO model selected target=deepseek model_id=deepseek-v4-pro reason="NLP classifier: 0.92 confidence"
```

## 🔧 Аргументы командной строки

| Флаг | Дефолт | Описание |
|------|--------|----------|
| `--addr` | `:8080` | Адрес для прослушивания |
| `--config` | `config.yaml` | Путь к конфигу |
| `--version` | — | Показать версию |

## 🌐 Переменные окружения

| Переменная | Назначение |
|-----------|-----------|
| `LLM_ROUTER_API_KEY` | API-ключ routerai.ru (передаётся в X-Api-Key) |

## 📂 Файлы рядом с бинарником

```
config.yaml          # опционально, конфигурация
rules.d/             # опционально, правила маршрутизации
model.bin            # опционально, NLP-модель (~22 MB)
vocab.txt            # опционально, словарь NLP-модели
```

Без всех файлов сервер запускается на встроенных дефолтах.