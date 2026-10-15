.PHONY: help dev/api dev/web build/api build/web test/api lint/api lint/web gen-module db/migrate db/seed docker/up docker/down clean

help: ## Show this help
	@grep -E '^[a-zA-Z/_-]+:.*?## .*$$' $(MAKEFILE_LIST) | sort | awk 'BEGIN {FS = ":.*?## "}; {printf "\033[36m%-20s\033[0m %s\n", $$1, $$2}'

dev/api: ## Run Go API in development mode
	cd apps/api && go run ./cmd/api

dev/web: ## Run Next.js in development mode
	cd apps/web && pnpm dev

build/api: ## Build Go API binary
	cd apps/api && go build -o bin/api ./cmd/api

build/web: ## Build Next.js for production
	cd apps/web && pnpm build

test/api: ## Run all Go tests
	cd apps/api && go test ./... -v -count=1

test/api/short: ## Run short Go tests (no integration)
	cd apps/api && go test ./... -v -count=1 -short

test/api/integration: ## Run Go integration tests (requires Docker)
	cd apps/api && go test ./tests/integration/... -v -count=1 -tags=integration

lint/api: ## Lint Go code
	cd apps/api && golangci-lint run ./...

lint/web: ## Lint Next.js code
	cd apps/web && pnpm lint

gen-module: ## Generate a new module scaffold: make gen-module name=<module>
	@if [ -z "$(name)" ]; then echo "Usage: make gen-module name=<module>"; exit 1; fi
	@bash scripts/gen-module.sh $(name)

db/migrate: ## Run database migrations (Goose)
	cd apps/api && goose -dir ../../db/migrations postgres "$$DATABASE_URL" up

db/migrate/down: ## Rollback last migration
	cd apps/api && goose -dir ../../db/migrations postgres "$$DATABASE_URL" down

db/seed: ## Run database seeder
	cd apps/api && go run ./cmd/seed

docker/up: ## Start all services with Docker Compose
	docker compose up -d

docker/down: ## Stop all services
	docker compose down

clean: ## Clean build artifacts
	rm -rf apps/api/bin
	rm -rf apps/web/.next
