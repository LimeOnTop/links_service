APP_IMAGE ?= links-service:latest
COMPOSE ?= docker compose

.PHONY: docker-build compose-up compose-down compose-logs run test clean

docker-build:
	@docker build -t $(APP_IMAGE) .

compose-up:
	@$(COMPOSE) up --build -d

compose-down:
	@$(COMPOSE) down

compose-logs:
	@$(COMPOSE) logs -f links-service

run:
	@go run ./cmd/server

test:
	@go test ./...

clean:
	@$(COMPOSE) down -v || true
