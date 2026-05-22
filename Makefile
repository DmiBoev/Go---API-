.PHONY: up down test test-local clean

up:
	docker-compose up --build -d

down:
	docker-compose down -v

test:
	docker-compose --profile test run --rm test

test-local:
	DATABASE_URL="host=localhost user=postgres password=secretpassword dbname=department_management_test port=5433 sslmode=disable" go test -v ./...

clean:
	docker-compose down -v
	rm -rf tmp/