# Department Management API

Русский | [English](README.en.md)

REST API на Go для управления деревом подразделений и сотрудниками. Поддерживает
создание, переименование, перемещение с защитой от циклов, каскадное удаление и
удаление с переназначением сотрудников.

## Стек и устройство

- Go 1.25.7, `net/http`, PostgreSQL 15.
- `database/sql` + `sqlx`: явный SQL, параметры `$1`, контекст в каждом запросе.
- Goose: версионированные SQL-миграции встроены в бинарник и выполняются при старте.
- `log/slog`: JSON-логи запросов со статусом, длительностью и размером ответа.
- Middleware восстановления после паники и CORS с настраиваемыми origin.
- OpenAPI 3.0.3 и Swagger UI со встроенными локальными JS/CSS.
- Интеграционные тесты PostgreSQL в Docker, GitHub Actions с race detector.

Слои: `handler` → `service` → `repository`. Только репозитории выполняют SQL.
Имена уникальны среди подразделений одного родителя, включая корневые.
Проверка цикла и перемещение выполняются в одной транзакции; блокировка таблицы
сериализует изменения дерева и предотвращает циклы при конкурентных запросах.
Переназначение сотрудников, подъём дочерних отделов и удаление также выполняются
одной транзакцией: конфликт имени откатывает все изменения.

## Запуск в Docker

Требуется Docker с Compose v2.

```bash
git clone https://github.com/DmiBoev/Go---API-.git
cd Go---API-
docker compose up --build -d
docker compose logs -f app
```

API: [localhost:8080](http://localhost:8080/departments/1).
Swagger UI: [localhost:8080/docs/](http://localhost:8080/docs/).
Контракт: [localhost:8080/openapi.yaml](http://localhost:8080/openapi.yaml),
исходник — [docs/openapi.yaml](docs/openapi.yaml).
Swagger UI работает без CDN; `Try it out` отправляет запросы в текущее приложение.

```bash
# Остановить приложение, сохранив данные
docker compose down
```

## Локальный запуск

Требуются Go 1.25.7+ и PostgreSQL. Для стандартных настроек достаточно:

```bash
docker compose up -d db
go run ./cmd/api
```

| Переменная | По умолчанию | Назначение |
| --- | --- | --- |
| `DATABASE_URL` | локальная БД `department_management`, пользователь `postgres`, пароль `secretpassword`, порт `5432`, SSL выключен | DSN PostgreSQL; значения по умолчанию предназначены для локальной разработки |
| `PORT` | `:8080` | Адрес прослушивания `net/http`, например `:9000` |
| `CORS_ALLOWED_ORIGINS` | `*` | Список origin через запятую, например `http://localhost:3000,https://example.com` |
| `TEST_DATABASE_URL` | не задана | Необязательная отдельная тестовая БД; без неё тесты сами запускают Docker |

CORS разрешает `GET`, `HEAD`, `POST`, `PATCH`, `DELETE`, `OPTIONS` и заголовки
`Content-Type`, `Accept`. Credentials не включены. Для конкретного фронтенда
задайте его origin через `CORS_ALLOWED_ORIGINS`.

## Проверки

```bash
# Модульные тесты middleware, конфигурации, валидации и OpenAPI; Docker не нужен
go test ./...

# Интеграционные тесты: автоматически запускают PostgreSQL в Docker
go test -tags=integration -count=1 -timeout=5m ./...

# Статический анализ
go vet -tags=integration ./...
```

Каждый тестовый пакет запускает отдельный временный контейнер на случайном порту
localhost. Каждый тест получает отдельную схему и реальные миграции. После
завершения схемы и контейнеры удаляются. Недоступный Docker завершает тесты ошибкой.
Проверяются HTTP-запросы через настоящий роутер и middleware: создание отделов и
сотрудников, дерево и глубина, уникальность, валидация, перемещение, запрет циклов,
каскадное удаление, переназначение, откат транзакции и конкурентные перемещения.

Если Go не установлен, можно выполнить все тесты в Docker в отдельном проекте:

```bash
docker compose -p dept-api-tests -f docker-compose.test.yml up --build --abort-on-container-exit --exit-code-from test
docker compose -p dept-api-tests -f docker-compose.test.yml down --volumes
```

Тестовая БД хранится в `tmpfs`, без опубликованных портов и без тома приложения.
На системах с `make` доступны `make test`, `make integration`, `make test-docker`.
Последняя команда автоматически выполняет очистку, сохраняя код завершения тестов.
CI выполняет `gofmt`, `go vet` и все тесты с `-race` на Linux.

Для собственной **отдельной тестовой БД** (пользователю нужны права на создание схем):

```powershell
$env:TEST_DATABASE_URL = 'postgres://postgres:test@localhost:5433/department_test?sslmode=disable'
go test -tags=integration -count=1 ./...
```

## API

| Метод | Путь | Действие |
| --- | --- | --- |
| `POST` | `/departments/` | Создать подразделение |
| `POST` | `/departments/{id}/employees/` | Создать сотрудника |
| `GET` | `/departments/{id}` | Получить дерево: `depth=1..5`, `include_employees=true/false` |
| `PATCH` | `/departments/{id}` | Переименовать или сменить родителя |
| `DELETE` | `/departments/{id}` | `mode=cascade` (по умолчанию) или `mode=reassign` |

Ошибки возвращаются как `{"error":"..."}`: `400` — некорректные данные,
`404` — подразделение не найдено, `409` — конфликт имени или цикл,
`500` — внутренняя ошибка без деталей БД. Полные схемы и ответы описаны в OpenAPI.

`PATCH` обновляет только ненулевые переданные поля; `parent_id: null` оставляет
родителя без изменений. В режиме `reassign` сотрудники удаляемого отдела переходят
в `reassign_to_department_id`, а его непосредственные дочерние отделы поднимаются
к его родителю (или становятся корневыми).

## Примеры запросов

Команды для Bash; в PowerShell используйте `curl.exe` или Swagger UI.

```bash
curl -X POST http://localhost:8080/departments/ \
  -H 'Content-Type: application/json' \
  -d '{"name":"IT","parent_id":null}'

curl -X POST http://localhost:8080/departments/1/employees/ \
  -H 'Content-Type: application/json' \
  -d '{"full_name":"Иван Петров","position":"Разработчик","hired_at":"2023-01-15T00:00:00Z"}'

curl 'http://localhost:8080/departments/1?depth=2&include_employees=true'

curl -X PATCH http://localhost:8080/departments/2 \
  -H 'Content-Type: application/json' \
  -d '{"name":"Platform","parent_id":1}'

curl -X DELETE 'http://localhost:8080/departments/2?mode=reassign&reassign_to_department_id=3'

curl -X DELETE 'http://localhost:8080/departments/1?mode=cascade'
```

`hired_at` в JSON — RFC 3339 timestamp или `null`; в PostgreSQL хранится календарная
дата. Для даты без времени передавайте полночь UTC, как в примере.
