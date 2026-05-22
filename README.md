# Department Management API

REST API для управления организационной структурой: подразделения (дерево), сотрудники, перемещение, каскадное удаление, переназначение.

## Технологии

- Go 1.25.7, `net/http`
- GORM (ORM), PostgreSQL
- Goose (миграции)
- Docker, docker-compose

## Быстрый старт (Docker)

Убедитесь, что у вас установлены **Docker** и **docker-compose**.

```bash
# Клонировать репозиторий
git clone <your-repo-url>
cd <project-folder>

# Запустить приложение (фоновый режим)
docker-compose up --build -d

# Проверить логи
docker-compose logs -f app

# Остановка и очистка
docker-compose down -v   # удалить контейнеры и том с данными

# Примеры запросов

# Создать подразделение
curl -X POST http://localhost:8080/departments/ \
  -H "Content-Type: application/json" \
  -d '{"name":"IT","parent_id":null}'

# Создать сотрудника в подразделении
curl -X POST http://localhost:8080/departments/1/employees/ \
  -H "Content-Type: application/json" \
  -d '{"full_name":"Иван Петров","position":"Разработчик","hired_at":"2023-01-15"}'

# Получить подразделение с деревом (до глубины 2)
curl "http://localhost:8080/departments/1?depth=2&include_employees=true"

# Переместить подразделение (изменить родителя)
curl -X PATCH http://localhost:8080/departments/2 \
  -H "Content-Type: application/json" \
  -d '{"parent_id":1}'

# Переименовать подразделение
curl -X PATCH http://localhost:8080/departments/2 \
  -H "Content-Type: application/json" \
  -d '{"name":"Новое имя"}'

# Удалить подразделение каскадно
curl -X DELETE "http://localhost:8080/departments/1?mode=cascade"

# Удалить подразделение с переназначением сотрудников в другой отдел
curl -X DELETE "http://localhost:8080/departments/2?mode=reassign&reassign_to_department_id=3"
