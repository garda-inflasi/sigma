SHELL := /bin/bash

.PHONY: help setup/env dev/api dev/web build/api build/web build/gardactl test/api test/ml test/web test/e2e lint/api lint/web gen-module db/migrate db/migrate/down db/migrate/version db/seed/reference db/seed/admin db/seed/demo db/impor/kecamatan ctl/ingest ctl/kpi openapi/sync openapi/check db/backup db/restore db/backup/verify docker/backup/up docker/up docker/down docker/infra/up docker/infra/down docker/app/up docker/app/down docker/demo/up docker/demo/down clean deploy deploy/build deploy/save deploy/transfer deploy/init-env deploy/remote deploy/prune rollback status logs

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

build/gardactl: ## Build gardactl CLI binary
	cd apps/api && go build -o bin/gardactl ./cmd/gardactl

build/web: ## Build Next.js for production
	cd apps/web && pnpm build

test/api: ## Run all Go tests
	cd apps/api && go test ./... -v -count=1

test/api/short: ## Run short Go tests (no integration)
	cd apps/api && go test ./... -v -count=1 -short

# Paket integrasi: tiap yang memakai Docker (dockertest). Tambahkan paket baru di sini.
INTEGRATION_PACKAGES = ./internal/auth/integration/... ./internal/cold-storage/integration/... ./internal/policy-brief/integration/... ./internal/prediksi/integration/... ./internal/ews/integration/... ./internal/intervensi/integration/... ./internal/spasial/integration/... ./internal/platform/... ./cmd/migrate/... ./internal/harga/integration/... ./internal/lapangan/integration/... ./internal/dashboard/integration/... ./internal/sumber-data/integration/... ./internal/ingestion/integration/... ./pkg/gardactl/... ./pkg/seed/integration/... ./internal/master/integration/... ./internal/audit/integration/...

test/api/integration: ## Run Go integration tests (requires Docker)
	cd apps/api && go test $(INTEGRATION_PACKAGES) -v -count=1

test/ml: ## Jalankan tes unit layanan ML (Python, tanpa dependensi)
	cd apps/ml && python3 -m unittest discover -s tests -t .

test/web: ## Run web unit tests (node:test + tsx)
	cd apps/web && pnpm test

# Stack harus dinyalakan lewat `make docker/demo/up`: e2e login ke API Go langsung (/api/v1/auth/login) ratusan kali
# per putaran, sedangkan bawaan API membatasi 10 percobaan auth per menit per IP (AUTH_RATE_LIMIT_PER_MENIT).
# Demo menyetelnya ke $(DEMO_AUTH_RATE_LIMIT_PER_MENIT); stack lain harus menyetelnya setinggi itu juga.
test/e2e: ## Uji e2e Playwright terhadap stack Docker yang berjalan (E2E_BASE_URL, bawaan http://localhost:8090)
	cd apps/web && pnpm exec playwright install chromium && pnpm exec playwright test

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
	cd apps/api && SEEDS_DIR=../../db/seeds go run ./cmd/seed reference

db/seed/admin: ## Seed akun admin awal (SEED_ADMIN_EMAIL, SEED_ADMIN_PASSWORD)
	cd apps/api && SEEDS_DIR=../../db/seeds go run ./cmd/seed admin

db/seed/demo: ## Seed data demo (ditolak bila APP_ENV=production)
	cd apps/api && SEEDS_DIR=../../db/seeds go run ./cmd/seed demo

# Menyegarkan kecamatan_aceh.csv dari API bridging wilayah BPS (perlu internet). Hasilnya di-commit; seeder
# master.reference menanamnya. Sumber dan lisensi: apps/api/internal/master/SUMBER.md.
db/impor/kecamatan: ## Impor kecamatan se-Aceh dari BPS ke internal/master/kecamatan_aceh.csv
	cd apps/api && go run ./cmd/gardactl master impor-kecamatan

ctl/ingest: ## Jalankan utilitas CLI gardactl ingest (mis. make ctl/ingest ARGS="daftar")
	cd apps/api && go run ./cmd/gardactl ingest $(ARGS)

ctl/kpi: ## Jalankan utilitas CLI gardactl kpi hitung (mis. make ctl/kpi ARGS="-bulan 2")
	cd apps/api && go run ./cmd/gardactl kpi hitung $(ARGS)

# ── OpenAPI: sumber tunggal packages/contracts/openapi.yaml ─────────────
# apps/api/pkg/swagger/openapi.yaml adalah salinan hasil generasi (go:embed tidak bisa
# membaca di luar paket). Sunting HANYA packages/contracts/openapi.yaml, lalu sinkronkan.
openapi/sync: ## Salin packages/contracts/openapi.yaml ke salinan embed Swagger (apps/api/pkg/swagger)
	cp packages/contracts/openapi.yaml apps/api/pkg/swagger/openapi.yaml

openapi/check: ## Gagal bila salinan embed Swagger berbeda dari sumber tunggal
	@cmp -s packages/contracts/openapi.yaml apps/api/pkg/swagger/openapi.yaml || { echo "Beda: jalankan make openapi/sync"; exit 1; }
	@echo "✓ OpenAPI sinkron"

# ── Backup & restore (scripts/backup, lihat docs/DEPLOYMENT.md §10) ─────
# Berjalan lewat `docker exec` ke container Postgres (PG_CONTAINER, bawaan garda-postgres).
# Ganti PG_CONTAINER/PGUSER/PGDATABASE/BACKUP_DIR lewat environment bila perlu.
db/backup: ## Backup database (pg_dump -Fc + checksum + manifest) ke ./backups
	@sh scripts/backup/backup.sh

db/backup/verify: ## Buktikan backup bisa dipulihkan (restore ke DB sementara, bandingkan, hapus)
	@sh scripts/backup/verify-restore.sh $(if $(FILE),--file $(FILE),)

db/restore: ## Restore dump ke database BARU: make db/restore FILE=backups/x.dump TARGET=garda_pulih
	@if [ -z "$(FILE)" ] || [ -z "$(TARGET)" ]; then echo "Usage: make db/restore FILE=<dump> TARGET=<database baru>"; exit 1; fi
	@sh scripts/backup/restore.sh "$(FILE)" --target-db "$(TARGET)"

docker/backup/up: ## Nyalakan backup terjadwal (profile backup di compose.infra.yaml)
	docker compose -f compose.infra.yaml --profile backup up -d backup

# ── Docker Compose targets ──────────────────────────────────────────────

docker/infra/up: ## Start infrastructure services (Postgres, Redis, MongoDB, pgAdmin)
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
	@echo "   pgAdmin  : http://localhost:5050 (admin@gardainflasi.id / admin)"
	@echo "   MongoDB  : localhost:27017"

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
DEMO_MAILPIT_PORT ?= 8025
DEMO_PGADMIN_PORT ?= 5050
DEMO_MONGO_PORT ?= 27017
DEMO_PGADMIN_VOLUME ?= garda_demo_pgadmin_data
DEMO_MONGO_VOLUME ?= garda_demo_mongo_data
# Mock OIDC (profil sso-mock) untuk "Masuk dengan Google" di demo/e2e. Penerbit http://sso.localhost:<port>/default
# harus sama bagi browser (loopback host) dan container api (alias jaringan); lihat compose.yaml.
DEMO_SSO_MOCK_PORT ?= 8089
# Batas percobaan auth per menit per IP di API Go (login, OTP, lupa/atur ulang kata sandi, SSO; bawaan produksi 10).
# e2e (make test/e2e) login ke /api/v1/auth/login langsung ratusan kali per putaran dari satu IP: demo wajib
# memakai nilai tinggi ini, atau e2e kena 429. Jangan dipakai di produksi.
DEMO_AUTH_RATE_LIMIT_PER_MENIT ?= 100000
# Profil mailpit dan sso-mock ditambahkan ke profil yang sudah disetel (mis. backup), bukan menggantinya.
comma := ,
DEMO_PROFILES = $(if $(COMPOSE_PROFILES),$(COMPOSE_PROFILES)$(comma))mailpit$(comma)sso-mock
DEMO_ENV = POSTGRES_VOLUME=$(DEMO_POSTGRES_VOLUME) REDIS_VOLUME=$(DEMO_REDIS_VOLUME) POSTGRES_PORT=$(DEMO_POSTGRES_PORT) REDIS_PORT=$(DEMO_REDIS_PORT) \
	PGADMIN_PORT=$(DEMO_PGADMIN_PORT) MONGO_PORT=$(DEMO_MONGO_PORT) \
	PGADMIN_VOLUME=$(DEMO_PGADMIN_VOLUME) MONGO_VOLUME=$(DEMO_MONGO_VOLUME) \
	CADDY_HTTP_PORT=$(DEMO_CADDY_HTTP_PORT) CADDY_HTTPS_PORT=$(DEMO_CADDY_HTTPS_PORT) \
	APP_ALLOWED_ORIGINS=http://localhost:$(DEMO_CADDY_HTTP_PORT),https://localhost:$(DEMO_CADDY_HTTPS_PORT) \
	APP_PUBLIC_URL=http://localhost:$(DEMO_CADDY_HTTP_PORT) \
	COMPOSE_PROFILES=$(DEMO_PROFILES) MAILPIT_PORT=$(DEMO_MAILPIT_PORT) \
	SMTP_HOST=mailpit SMTP_PORT=1025 SMTP_USERNAME= SMTP_PASSWORD= SMTP_FROM=noreply@gardainflasi.id \
	SSO_MOCK_PORT=$(DEMO_SSO_MOCK_PORT) SSO_GOOGLE_CLIENT_ID=garda-demo SSO_GOOGLE_CLIENT_SECRET=garda-demo-secret \
	SSO_GOOGLE_ISSUER=http://sso.localhost:$(DEMO_SSO_MOCK_PORT)/default SSO_GOOGLE_HOSTED_DOMAIN= \
	SSO_GOOGLE_REDIRECT_URL=http://localhost:$(DEMO_CADDY_HTTP_PORT)/api/auth/sso/google/callback \
	COOKIE_SECURE=false \
	AUTH_RATE_LIMIT_PER_MENIT=$(DEMO_AUTH_RATE_LIMIT_PER_MENIT)

docker/demo/up: ## Nyalakan demo: infra, migrasi, seed (reference+demo), lalu aplikasi
	$(DEMO_ENV) docker compose -f compose.infra.yaml up -d --wait
	$(DEMO_ENV) docker compose build
	$(DEMO_ENV) docker compose run --rm migrate
	$(DEMO_ENV) docker compose run --rm seed
	$(DEMO_ENV) docker compose up -d api worker ml web caddy mailpit sso-mock
	@echo ""
	@echo "✓ Demo berjalan: http://localhost:$(DEMO_CADDY_HTTP_PORT)  (login: superadmin@gardainflasi.id / Garda2024!)"
	@echo "  Kotak masuk email (Mailpit): http://localhost:$(DEMO_MAILPIT_PORT)"
	@echo "  Masuk dengan Google (mock OIDC): tombol di halaman masuk; isi username apa saja dan klaim {\"email\":\"superadmin@gardainflasi.id\",\"email_verified\":true}"
	@echo "  pgAdmin (PostgreSQL Web UI): http://localhost:$(DEMO_PGADMIN_PORT)  (admin@gardainflasi.id / admin)"
	@echo "  MongoDB (Local): localhost:$(DEMO_MONGO_PORT)"

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

logs: ## Tail logs dari VM (SERVICE=api|worker|ml|web|caddy TAIL=200)
	@SERVICE=$(SERVICE) TAIL=$(TAIL) bash scripts/deploy/logs.sh $(ARGS)

