# Department Management API

[Русский](README.md) | English

A REST API written in Go for managing a department hierarchy and its employees.
It supports creating, renaming, and moving departments with cycle prevention,
cascading deletion, and deletion with employee reassignment.

## Stack and architecture

- Go 1.25.7, `net/http`, PostgreSQL 15.
- `database/sql` + `sqlx`: explicit SQL, `$1` parameters, and context-aware queries.
- Goose: versioned SQL migrations embedded in the binary and applied at startup.
- `log/slog`: structured JSON request logs with status, duration, and response size.
- Panic recovery middleware and CORS with configurable allowed origins.
- OpenAPI 3.0.3 and Swagger UI with embedded local JavaScript and CSS assets.
- PostgreSQL integration tests in Docker and GitHub Actions with the race detector.

Layers: `handler` → `service` → `repository`. Only repositories execute SQL.
Department names are unique among siblings, including root departments.
The cycle check and move run in one transaction. A table lock serializes hierarchy
changes to prevent concurrent moves from creating a cycle.
Employee reassignment, child promotion, and department deletion also run in one
transaction: a name conflict rolls back every change.

## Run with Docker

Requires Docker with Compose v2.

```bash
git clone https://github.com/DmiBoev/Go---API-.git
cd Go---API-
docker compose up --build -d
docker compose logs -f app
```

API: [localhost:8080](http://localhost:8080/departments/1).
Swagger UI: [localhost:8080/docs/](http://localhost:8080/docs/).
OpenAPI contract: [localhost:8080/openapi.yaml](http://localhost:8080/openapi.yaml).
Source: [docs/openapi.yaml](docs/openapi.yaml).
Swagger UI works without a CDN; `Try it out` sends requests to the running application.

```bash
# Stop the application while keeping its database volume
docker compose down
```

## Run locally

Requires Go 1.25.7+ and PostgreSQL. To use the default configuration:

```bash
docker compose up -d db
go run ./cmd/api
```

| Variable | Default | Purpose |
| --- | --- | --- |
| `DATABASE_URL` | Local database `department_management`, user `postgres`, password `secretpassword`, port `5432`, SSL disabled | PostgreSQL DSN; the defaults are intended for local development |
| `PORT` | `:8080` | HTTP listen address, for example `:9000` |
| `CORS_ALLOWED_ORIGINS` | `*` | Comma-separated origins, for example `http://localhost:3000,https://example.com` |
| `TEST_DATABASE_URL` | Unset | Optional separate test database; when unset, integration tests start Docker containers automatically |

CORS allows `GET`, `HEAD`, `POST`, `PATCH`, `DELETE`, and `OPTIONS`, with
`Content-Type` and `Accept` headers. Credentials are disabled. Set
`CORS_ALLOWED_ORIGINS` to the specific origin of your frontend when needed.

## Checks

```bash
# Unit tests for middleware, configuration, validation, and OpenAPI; no Docker required
go test ./...

# Integration tests: start PostgreSQL in Docker automatically
go test -tags=integration -count=1 -timeout=5m ./...

# Static analysis
go vet -tags=integration ./...
```

Each integration test package starts its own temporary container on a random
localhost port. Every test receives a separate schema and applies the real
migrations. Schemas and containers are removed when the tests finish. If Docker
is unavailable, integration tests fail.

HTTP tests exercise the production router and middleware, covering department and
employee creation, hierarchy depth, uniqueness, validation, moves, cycle prevention,
cascading deletion, reassignment, transaction rollback, and concurrent moves.

To run all tests in Docker without installing Go, use a separate Compose project:

```bash
docker compose -p dept-api-tests -f docker-compose.test.yml up --build --abort-on-container-exit --exit-code-from test
docker compose -p dept-api-tests -f docker-compose.test.yml down --volumes
```

The test database uses `tmpfs`, has no published ports, and does not use the
application's database volume. On systems with `make`, use `make test`,
`make integration`, or `make test-docker`. The last command cleans up automatically
and preserves the test exit status. CI runs `gofmt`, `go vet`, and all tests with
`-race` on Linux.

To use your own **separate test database**, its user must have permission to create schemas:

```powershell
$env:TEST_DATABASE_URL = 'postgres://postgres:test@localhost:5433/department_test?sslmode=disable'
go test -tags=integration -count=1 ./...
```

## API

| Method | Path | Action |
| --- | --- | --- |
| `POST` | `/departments/` | Create a department |
| `POST` | `/departments/{id}/employees/` | Create an employee |
| `GET` | `/departments/{id}` | Get a department tree with `depth=1..5` and `include_employees=true/false` |
| `PATCH` | `/departments/{id}` | Rename a department or change its parent |
| `DELETE` | `/departments/{id}` | Delete with `mode=cascade` (default) or `mode=reassign` |

Errors use the JSON shape `{"error":"..."}`: `400` for invalid input, `404` when a
department is missing, `409` for a name conflict or cycle, and `500` for an
unexpected server error without database details. OpenAPI describes the full
request and response schemas.

`PATCH` updates only supplied, non-null fields. Setting `parent_id` to `null`
leaves the parent unchanged. In `reassign` mode, the deleted department's employees
move to `reassign_to_department_id`, and its immediate children move to its parent
or become root departments.

## Request examples

These commands use Bash syntax. In PowerShell, use `curl.exe` or Swagger UI.

```bash
curl -X POST http://localhost:8080/departments/ \
  -H 'Content-Type: application/json' \
  -d '{"name":"IT","parent_id":null}'

curl -X POST http://localhost:8080/departments/1/employees/ \
  -H 'Content-Type: application/json' \
  -d '{"full_name":"Ivan Petrov","position":"Developer","hired_at":"2023-01-15T00:00:00Z"}'

curl 'http://localhost:8080/departments/1?depth=2&include_employees=true'

curl -X PATCH http://localhost:8080/departments/2 \
  -H 'Content-Type: application/json' \
  -d '{"name":"Platform","parent_id":1}'

curl -X DELETE 'http://localhost:8080/departments/2?mode=reassign&reassign_to_department_id=3'

curl -X DELETE 'http://localhost:8080/departments/1?mode=cascade'
```

In JSON, `hired_at` accepts an RFC 3339 timestamp or `null`. PostgreSQL stores it
as a calendar date. For a date without a time, send midnight UTC as shown above.
