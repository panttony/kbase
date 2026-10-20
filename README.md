# Сервис личной базы знаний (kbase)

Сервис для ведения личной базы знаний: приём Markdown-документов, анализ и
нормализация содержимого, полнотекстовый поиск с релевантностью (BM25 + свой
слой TF-IDF), фильтрация, подсветка совпадений, версионирование документов и
кэширование поисковых запросов.

Стек: Go 1.25, Bleve (поиск), PostgreSQL (хранение + версии), Redis (кэш),
REST API (стандартная библиотека `net/http`).

## Запуск

### Через Docker (проще всего)

```bash
docker compose up --build
```

Поднимает PostgreSQL 16, Redis 7 и само приложение. API доступен на
`http://localhost:8080`; схема базы данных создаётся автоматически при старте
(миграции встраиваются в бинарник через `go:embed`).

### Локальная разработка

```bash
docker compose up -d postgres redis          # только инфраструктура
export PATH=/usr/local/go/bin:$PATH
export PG_DSN="postgres://kbase:kbase@localhost:5432/kbase?sslmode=disable"
go run ./cmd/server
```

### Конфигурация (переменные окружения)

| Переменная | Значение по умолчанию | Описание |
|---|---|---|
| `HTTP_ADDR` | `:8080` | адрес HTTP |
| `PG_DSN` | — (обязательна) | строка подключения PostgreSQL |
| `REDIS_ADDR` | `localhost:6379` | адрес Redis |
| `REDIS_PASSWORD` | пусто | пароль Redis |
| `INDEX_PATH` | `./data/index` | путь к Bleve-индексу на диске |
| `CACHE_TTL` | `60s` | TTL кэша поисковых запросов |
| `LOG_LEVEL` | `info` | уровень логов |


## Использование API

Базовый URL: `http://localhost:8080`. Интерактивная документация (Swagger UI) —
`http://localhost:8080/docs/`; машинная спецификация OpenAPI 3.1 отдаётся по
адресу `/api/v1/openapi.yaml` (исходник — `internal/api/openapi.yaml`).

### Создание документа

Формат — Markdown с необязательным YAML-front matter (title, tags, category,
authors):

```bash
curl -X POST http://localhost:8080/api/v1/documents \
  -H 'Content-Type: application/json' \
  -d '{"content":"---\ntitle: Go Networking\ntags: [go, backend]\ncategory: dev\n---\n# Go\n\nThe net package provides portable network interfaces."}'
```

Ответ — JSON-документ с `id`, `created_at`, `version: 1`.

### Получение и обновление

```bash
curl http://localhost:8080/api/v1/documents/<id>

# Обновление создаёт новую версию, история сохраняется
curl -X PUT http://localhost:8080/api/v1/documents/<id> \
  -H 'Content-Type: application/json' \
  -d '{"content":"---\ntitle: Go Networking v2\ntags: [go, backend]\ncategory: dev\n---\n..."}'
```

### Версии документа

```bash
curl http://localhost:8080/api/v1/documents/<id>/versions        # список
curl http://localhost:8080/api/v1/documents/<id>/versions/2      # конкретная версия
```

### Удаление

Мягкое удаление — метка `deleted_at`, история версий остаётся в базе:

```bash
curl -X DELETE http://localhost:8080/api/v1/documents/<id>
```

### Поиск

Полнотекстовый поиск с релевантностью, фильтрами, сортировкой и пагинацией:

```bash
# базовый поиск (скоринг BM25 по умолчанию)
curl "http://localhost:8080/api/v1/search?q=go+net"

# TF-IDF + фильтры по тегам и категории + сортировка по дате обновления
curl "http://localhost:8080/api/v1/search?q=networking&method=tfidf&tags=go&tags=backend&category=dev&sort=updated_at"

# диапазон дат обновления (RFC3339) + пагинация
curl "http://localhost:8080/api/v1/search?q=go&from=2026-01-01T00:00:00Z&to=2026-12-31T23:59:59Z&page=1&page_size=10"
```

Параметры запроса:

| Параметр | Описание |
|---|---|
| `q` | текст запроса (пустой — выдача всех документов с учётом фильтров) |
| `method` | метод скоринга: `bm25` (по умолчанию) или `tfidf` |
| `tags` | фильтр по тегам, параметр повторяемый (OR) |
| `category` | фильтр по категории |
| `from`, `to` | диапазон даты обновления документа (RFC3339) |
| `sort` | `relevance` (по умолчанию), `title`, `created_at`, `updated_at` |
| `page`, `page_size` | пагинация (по умолчанию 1 и 20) |

Каждый результат содержит `score` (релевантность) и `snippet` — фрагмент
текста с подсветкой совпадения тегом `<mark>`.

### Статистика базы

```bash
curl http://localhost:8080/api/v1/stats
# -> {"documents":3,"total_size":1234}
```

## Поведение

- **Валидация**: документ проходит цепочку «парсинг → нормализация (NFKC,
  дедуп тегов) → валидация»; ошибки возвращаются кодом `400` в едином формате
  `{"error":{"code","message"}}`.
- **Кэш**: одинаковые поисковые запросы отдаются из Redis в течение `CACHE_TTL`
  (ключ `search:v1:` + sha256 канонического ключа), при изменении или удалении
  документа кэш инвалидируется. Если Redis недоступен — сервис продолжает
  работать без кэша (fail-open).
- **Версионирование**: каждое изменение документа создаёт новую версию;
  тело и метаданные предыдущих версий доступны через `/versions`.

## Документация

- `docs/system-design.md` — архитектура, схема БД, поток поискового запроса, стратегия кэширования
- `internal/api/openapi.yaml` — спецификация REST API (Swagger UI сервера — `/docs/`)

## Разработка

```bash
export PATH=/usr/local/go/bin:$PATH
go test ./...         # тесты (интеграционные с PostgreSQL/Redis пропускаются без Docker)
go test -bench . -benchmem ./internal/indexing/   # бенчмарки индексации и поиска
go vet ./...
gofmt -l .
```