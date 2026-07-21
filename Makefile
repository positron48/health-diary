.PHONY: up down logs migrate test check reset-db web-build

up:
	docker compose up --build -d

down:
	docker compose down

logs:
	docker compose logs -f

migrate:
	docker compose run --rm app /app/health-diary-migrate

test:
	go test ./...

web-build:
	npm --prefix web ci
	npm --prefix web run build

check: test web-build

reset-db:
	@echo "Removing explicit Compose volume health-diary_health_diary_postgres"
	docker compose down -v
