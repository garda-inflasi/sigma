SHELL := /bin/bash

.PHONY: help setup/env dev/api dev/web build/api build/web test/api test/web lint/api lint/web gen-module db/migrate db/migrate/down db/migrate/version db/seed/reference db/seed/admin db/seed/demo docker/up docker/down docker/infra/up docker/infra/down docker/app/up docker/app/down docker/demo/up docker/demo/down clean deploy deploy/build deploy/save deploy/transfer deploy/init-env deploy/remote deploy/prune rollback status logs

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

# Paket integrasi: tiap yang memakai Docker (dockertest). Tambahkan paket baru di sini.
INTEGRATION_PACKAGES = ./internal/auth/integration/... ./internal/cold-storage/integration/... ./internal/policy-brief/integration/... ./internal/prediksi/integration/... ./internal/ews/integration/... ./internal/intervensi/integration/... ./internal/spasial/integration/... ./internal/platform/... ./cmd/migrate/... ./internal/harga/integration/... ./internal/harga/seeder/... ./internal/lapangan/integration/... ./internal/lapangan/seeder/... ./internal/dashboard/integration/... ./internal/dashboard/seeder/... ./internal/sumber-data/integration/... ./internal/ingestion/integration/... ./internal/ingestion/seeder/... ./cmd/ingest/...

test/api/integration: ## Run Go integration tests (requires Docker)
	cd apps/api && go test $(INTEGRATION_PACKAGES) -v -count=1

test/web: ## Run web unit tests (node:test + tsx)
	cd apps/web && pnpm test

lint/api: ## Lint Go code
	cd apps/api && golangci-lint run ./...

lint/web: ## Lint Next.js code
	cd apps/web && pnpm lint

gen-module: ## Generate a new module scaffold: make gen-module name=<module>
	@if [ -z "$(name)" ]; then echo "Usage: make gen-module name=<module>"; exit 1; fi
	@bash scripts/gen-module.sh $(name)

# Migrasi dijalankan cmd/migrate (library goose), program yang sama dengan service
# `migrate` di Docker; tidak perlu memasang CLI goose.
db/migrate: ## Terapkan migrasi database
	cd apps/api && MIGRATIONS_DIR=../../db/migrations go run ./cmd/migrate up

db/migrate/down: ## Batalkan satu migrasi terakhir
	cd apps/api && MIGRATIONS_DIR=../../db/migrations go run ./cmd/migrate down

db/migrate/version: ## Cetak versi skema saat ini
	cd apps/api && MIGRATIONS_DIR=../../db/migrations go run ./cmd/migrate version

db/seed/reference: ## Seed data referensi (aman di produksi)
	cd apps/api && go run ./cmd/seed reference

db/seed/admin: ## Seed akun admin awal (SEED_ADMIN_EMAIL, SEED_ADMIN_PASSWORD)
	cd apps/api && go run ./cmd/seed admin

db/seed/demo: ## Seed data demo (ditolak bila APP_ENV=production)
	cd apps/api && go run ./cmd/seed demo

# ── Docker Compose targets ──────────────────────────────────────────────

docker/infra/up: ## Start infrastructure services (Postgres, Redis)
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

# Demo lengkap (infra + migrasi + seed + aplikasi) di port host alternatif supaya
# tidak bentrok dengan proyek lain. Ganti dengan mis. `make docker/demo/up DEMO_CADDY_HTTP_PORT=9000`.
DEMO_POSTGRES_PORT ?= 5433
DEMO_REDIS_PORT ?= 6380
DEMO_CADDY_HTTP_PORT ?= 8090
DEMO_CADDY_HTTPS_PORT ?= 8453
DEMO_POSTGRES_VOLUME ?= garda_demo_postgres_data
DEMO_REDIS_VOLUME ?= garda_demo_redis_data
DEMO_ENV = POSTGRES_VOLUME=$(DEMO_POSTGRES_VOLUME) REDIS_VOLUME=$(DEMO_REDIS_VOLUME) POSTGRES_PORT=$(DEMO_POSTGRES_PORT) REDIS_PORT=$(DEMO_REDIS_PORT) \
	CADDY_HTTP_PORT=$(DEMO_CADDY_HTTP_PORT) CADDY_HTTPS_PORT=$(DEMO_CADDY_HTTPS_PORT) \
	APP_ALLOWED_ORIGINS=http://localhost:$(DEMO_CADDY_HTTP_PORT),https://localhost:$(DEMO_CADDY_HTTPS_PORT) \
	COOKIE_SECURE=false

docker/demo/up: ## Nyalakan demo: infra, migrasi, seed (reference+demo), lalu aplikasi
	$(DEMO_ENV) docker compose -f compose.infra.yaml up -d --wait
	$(DEMO_ENV) docker compose build
	$(DEMO_ENV) docker compose run --rm migrate
	$(DEMO_ENV) docker compose run --rm seed
	$(DEMO_ENV) docker compose up -d api web caddy
	@echo ""
	@echo "✓ Demo berjalan: http://localhost:$(DEMO_CADDY_HTTP_PORT)  (login: superadmin@gardainflasi.id / Garda2024!)"

docker/demo/down: ## Matikan demo (data tetap di volume)
	$(DEMO_ENV) docker compose down
	$(DEMO_ENV) docker compose -f compose.infra.yaml down

clean: ## Clean build artifacts
	rm -rf apps/api/bin
	rm -rf apps/web/.next

# ── Deploy targets (build di Mac → transfer → jalankan di VM) ───────────
# Semua target memakai scripts/deploy/*.sh. Lihat docs/DEPLOYMENT.md.

deploy: ## Full deploy: build → save → transfer → up (ARGS=--skip-build dsb)
	@bash scripts/deploy/deploy.sh $(ARGS)

deploy/build: ## Build image lokal (ARGS=--only=api,web,caddy --no-cache)
	@bash scripts/deploy/build-local.sh $(ARGS)

deploy/save: ## Save image ke dist/garda-images-<sha>.tar.gz
	@bash scripts/deploy/save-images.sh $(ARGS)

deploy/transfer: ## Transfer artefak ke VM via rsync/SSH
	@bash scripts/deploy/transfer.sh $(ARGS)

deploy/init-env: ## Buat .env.deploy dari template
	@test -f .env.deploy || cp .env.deploy.example .env.deploy
	@echo "✓ .env.deploy siap — edit nilainya lalu jalankan: make deploy"

deploy/remote: ## Jalankan deploy di VM saja (tanpa build/transfer)
	@bash scripts/deploy/deploy.sh --skip-build --skip-transfer $(ARGS)

deploy/prune: ## Prune image & tarball lama (keep N terakhir)
	@bash scripts/rollback/keep-last-n-images.sh $(ARGS)

rollback: ## Rollback ke deploy sebelumnya (ARGS=--to=<sha>)
	@bash scripts/deploy/rollback.sh $(ARGS)

status: ## Status service di VM + resource usage
	@bash scripts/deploy/status.sh $(ARGS)

logs: ## Tail logs dari VM (SERVICE=api|web|caddy TAIL=200)
	@SERVICE=$(SERVICE) TAIL=$(TAIL) bash scripts/deploy/logs.sh $(ARGS)

