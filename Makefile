.PHONY: help setup/env dev/api dev/web build/api build/web test/api lint/api lint/web gen-module db/migrate db/seed docker/up docker/down docker/infra/up docker/infra/down docker/app/up docker/app/down clean

# Muat konfigurasi root `.env` ke environment semua target (backend, db, dsb).
# File dibuat dari `make setup/env`. Abaikan error bila belum ada.
-include .env
export

help: ## Show this help
	@grep -E '^[a-zA-Z/_-]+:.*?## .*$$' $(MAKEFILE_LIST) | sort | awk 'BEGIN {FS = ":.*?## "}; {printf "\033[36m%-20s\033[0m %s\n", $$1, $$2}'

setup/env: ## Salin template env ke .env (root) & apps/web/.env.local
	@test -f .env || cp env.example .env
	@echo "✓ .env (root) siap — dipakai backend Go (make dev/api) & Docker Compose"
	@mkdir -p apps/web
	@test -f apps/web/.env.local || cp apps/web/env.local.example apps/web/.env.local
	@echo "✓ apps/web/.env.local siap — dipakai frontend Next.js (make dev/web)"

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

# ── Docker Compose targets ──────────────────────────────────────────────

docker/infra/up: ## Start infrastructure services (Postgres, MongoDB, Redis)
	docker compose -f compose.infra.yaml up -d

docker/infra/down: ## Stop infrastructure services
	docker compose -f compose.infra.yaml down

docker/infra/logs: ## View infrastructure logs
	docker compose -f compose.infra.yaml logs -f

docker/app/up: ## Start application services (API, Web, Caddy)
	docker compose up -d

docker/app/down: ## Stop application services
	docker compose down

docker/app/logs: ## View application logs
	docker compose logs -f

docker/up: ## Start all services (infra + app)
	@echo "=== Starting infrastructure services ==="
	docker compose -f compose.infra.yaml up -d
	@echo "=== Starting application services ==="
	docker compose up -d
	@echo ""
	@echo "✓ Semua layanan berjalan. Akses:"
	@echo "   Frontend : https://localhost"
	@echo "   API      : https://localhost/api"
	@echo "   Health   : https://localhost/healthz"

docker/down: ## Stop all services (app + infra)
	docker compose down
	docker compose -f compose.infra.yaml down

docker/rebuild: ## Rebuild and restart all services
	docker compose build --no-cache
	docker compose -f compose.infra.yaml up -d
	docker compose up -d

clean: ## Clean build artifacts
	rm -rf apps/api/bin
	rm -rf apps/web/.next

