# Usage: How to Set Up and Run

## Быстрый старт

```bash
# 1. Собрать бинарник
make build-linux
# или
make build-win

# 2. Запустить
export ANTHROPIC_BASE_URL=http://localhost:8080
export ANTHROPIC_AUTH_TOKEN=""    # пустой — ключ передаётся через X-Api-Key
./llm-router

# 3. В другом терминале — Claude Code
claude
```

## Пошаговая инструкция

### 1. Получить ключ routerapi.ru

Зарегистрируйтесь на https://routerapi.ru и получите API-ключ.

### 2. Собрать

```bash
git clone https://github.com/DrXOps/llm-router
cd llm-router
make build-linux      # Linux amd64
# или
make build-win        # Windows amd64
```

Бинарник появится в текущей директории:
- Linux: `llm-router`
- Windows: `llm-router.exe`

### 3. Настроить окружение Claude Code

**Linux/macOS** (`~/.bashrc` или `~/.zshrc`):

```bash
export ANTHROPIC_BASE_URL=http://localhost:8080
export ANTHROPIC_AUTH_TOKEN=""   # обязательно пустой
```

**Windows PowerShell** (`$PROFILE`):

```powershell
$env:ANTHROPIC_BASE_URL = "http://localhost:8080"
$env:ANTHROPIC_AUTH_TOKEN = ""
```

### 4. Запустить роутер

```bash
export LLM_ROUTER_API_KEY=ваш_ключ_routerapi_ru
./llm-router --addr :8080
```

### 5. Проверить

```bash
curl -v http://localhost:8080/v1/messages \
  -H "Content-Type: application/json" \
  -H "X-Api-Key: $LLM_ROUTER_API_KEY" \
  -d '{
    "messages": [{"role": "user", "content": "Say hello"}],
    "max_tokens": 50
  }'
```

### 6. Увидеть, какая модель выбрана

В логах роутера:
```
INFO model selected target=haiku model_id=claude-haiku-5-5
     reason="always rule \"always_haiku_fallback\" matched"
```

## Аргументы командной строки

```
--config  string   путь к config.yaml (по умолчанию: config.yaml в текущей директории)
--addr    string   адрес для прослушивания (по умолчанию: :8080)
--version          показать версию и выйти
```

## Переменные окружения

- `LLM_ROUTER_API_KEY` — API-ключ для routerapi.ru
  (передаётся в заголовке X-Api-Key)

## Пример конфига

Создайте `config.yaml` рядом с бинарником:

```yaml
models:
  haiku:
    id: "claude-haiku-5-5"
  deepseek:
    id: "deepseek-v4-pro"

upstream:
  base_url: "https://routerapi.ru"

logging:
  level: info
  log_model_choice: true
```

Без этого файла сервер запустится на встроенных дефолтах.