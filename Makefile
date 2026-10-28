.PHONY: up down test integration test-docker clean

up:
	docker compose up --build -d

down:
	docker compose down

test:
	go test ./...

integration:
	go test -tags=integration -count=1 -timeout=5m ./...

# The trap preserves the test exit status and cleans up only the test project.
test-docker:
	@trap 'docker compose -p dept-api-tests -f docker-compose.test.yml down --volumes' EXIT; \
	docker compose -p dept-api-tests -f docker-compose.test.yml up --build --abort-on-container-exit --exit-code-from test

clean:
	docker compose -p dept-api-tests -f docker-compose.test.yml down --volumes
