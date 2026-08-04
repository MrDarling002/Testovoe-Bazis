# Task Team Service

REST API сервис управления задачами в командах: ролевая модель (owner / admin / member), история изменений задач, аналитические SQL-отчёты, кеширование в Redis и Prometheus-метрики.

Стек: **Go, MySQL 8, Redis 7, Docker Compose**.

## Быстрый старт

```bash
docker compose up --build
```

API поднимется на `http://localhost:8080`. Миграции применяются автоматически при старте приложения (golang-migrate, файлы в `migrations/`).

> Если у вас остался volume от старой версии схемы — пересоздайте его: `docker compose down -v && docker compose up --build`.

## Конфигурация

Базовые значения — в `config/config.yaml`, секреты и окружение — через переменные:

| Переменная | Назначение |
|---|---|
| `CONFIG_PATH` | путь к YAML-конфигу (по умолчанию `config/config.yaml`) |
| `DB_DSN` | DSN MySQL |
| `REDIS_ADDR`, `REDIS_PASSWORD`, `REDIS_DB` | подключение к Redis |
| `JWT_SECRET` | **обязателен**, минимум 32 символа — сервис не стартует без него |
| `EMAIL_BASE_URL` | адрес email-сервиса (в compose — мок `email-mock`) |
| `SERVER_ADDR` | адрес HTTP-сервера |

## API

Все защищённые эндпоинты требуют заголовок `Authorization: Bearer <token>`.

| Метод | Путь | Описание |
|---|---|---|
| POST | `/api/v1/register` | регистрация (`email`, `username`, `password`) |
| POST | `/api/v1/login` | вход, возвращает JWT |
| POST | `/api/v1/teams` | создать команду (создатель становится owner) |
| GET | `/api/v1/teams` | команды, где состоит пользователь |
| POST | `/api/v1/teams/{id}/invite` | пригласить (`user_id` или `email`, опционально `role`); только owner/admin |
| POST | `/api/v1/tasks` | создать задачу (только член команды) |
| GET | `/api/v1/tasks?team_id=1&status=todo&assignee_id=5&page=1&per_page=20` | список с фильтрами и пагинацией (кешируется в Redis на 5 минут) |
| PUT | `/api/v1/tasks/{id}` | частичное обновление с проверкой прав; `"assignee_id": null` снимает исполнителя |
| GET | `/api/v1/tasks/{id}/history` | история изменений задачи (пагинация `page`/`per_page`) |
| GET | `/api/v1/analytics/teams-summary` | по каждой команде пользователя: участники и done-задачи за 7 дней |
| GET | `/api/v1/analytics/top-creators` | топ-3 создателя задач за 30 дней в каждой команде пользователя |
| GET | `/api/v1/analytics/invalid-assignees` | задачи, где исполнитель не входит в команду |
| GET | `/health` | liveness |
| GET | `/metrics` | Prometheus |

Права на обновление задачи: owner/admin и автор — любые поля; исполнитель — только `status` и `description`.

## Тесты

```bash
# unit-тесты
go test ./...

# unit-тесты с покрытием бизнес-логики
go test ./internal/... -cover

# интеграционные тесты (нужен запущенный Docker; testcontainers поднимет MySQL)
go test -tags integration ./internal/store/mysql/ -v
```

## Архитектура

```
cmd/api            — точка входа, DI, graceful shutdown
internal/config    — YAML-конфиг + ENV-переопределения
internal/domain    — модели и доменные ошибки
internal/service   — бизнес-логика (авторизация, задачи, команды, аналитика)
internal/store     — mysql (репозитории, миграции), rediscache (кеш списков задач)
internal/httpapi   — HTTP-handlers, роутер, middleware
internal/auth      — JWT и auth middleware
internal/ratelimit — token bucket на Redis + Lua (100 req/мин на пользователя/IP)
internal/email     — HTTP-клиент email-сервиса с circuit breaker (gobreaker)
internal/metrics   — Prometheus: запросы, длительность, ошибки, кеш, пул БД
migrations         — SQL-миграции (golang-migrate, встраиваются в бинарник)
```
