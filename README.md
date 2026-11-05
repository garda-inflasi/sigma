# Garda Inflasi

Platform pengendalian inflasi daerah. Monorepo:

| Lokasi          | Isi                                                                                      |
| --------------- | ---------------------------------------------------------------------------------------- |
| `apps/api`      | API Go (Gin + GORM, hexagonal per modul). Lihat [apps/api/README.md](apps/api/README.md) |
| `apps/web`      | Web Next.js 16 (BFF + UI)                                                                |
| `db/migrations` | Migrasi goose (PostgreSQL), satu-satunya sumber kebenaran skema                          |
| `docs/todos`    | Rencana kerja per fitur; mulai dari [00-cara-kerja.md](docs/todos/00-cara-kerja.md)      |

Prasyarat: Go (versi di `apps/api/go.mod`), Node + pnpm, Docker.

## Alur lokal (pengembangan)

```bash
make setup/env            # salin env.example ke .env dan apps/web/.env.local
pnpm install              # sekali saja
make docker/infra/up      # PostgreSQL + Redis
make db/migrate           # terapkan db/migrations
make db/seed/reference    # data referensi
make db/seed/demo         # data demo (akun, histori, dst.)
make dev/api              # terminal 1: API di :8080
make dev/web              # terminal 2: web di :3000
```

Login dengan akun demo, mis. `superadmin@gardainflasi.id`, kata sandi `Garda2024!`.

- `NEXT_PUBLIC_API_BASE_URL=/api` di `apps/web/.env.local`. Panggilan `/api/v1/*` diteruskan Next ke Go
  (`GO_API_BASE_URL`) hanya saat `make dev/web`; di Docker/produksi Caddy yang meneruskannya.
- `make db/seed/reference db/seed/demo` aman dijalankan berulang (idempoten).
- `make db/seed/demo` menolak berjalan bila `APP_ENV=production`.
- `make db/migrate/down` membatalkan satu migrasi; `make db/migrate/version` mencetak versi skema.

## Demo lengkap dengan Docker

```bash
make docker/demo/up       # infra + migrasi + seed + api + web + caddy
make docker/demo/down
```

Memakai port host alternatif agar tidak bentrok: web `http://localhost:8090` (HTTPS `8453`),
Postgres `5433`, Redis `6380`. Ubah lewat `DEMO_POSTGRES_PORT`, `DEMO_REDIS_PORT`,
`DEMO_CADDY_HTTP_PORT`, `DEMO_CADDY_HTTPS_PORT`.

Demo memakai volume sendiri (`garda_demo_postgres_data`, `garda_demo_redis_data`) sehingga tidak
menyentuh database dev. Untuk mengulang dari data awal: `make docker/demo/down` lalu
`docker volume rm garda_demo_postgres_data garda_demo_redis_data` dan `make docker/demo/up`.
Login: `superadmin@gardainflasi.id` / `Garda2024!` (semua akun demo memakai kata sandi itu).
Login lewat web dibatasi 5 percobaan per menit per IP.

`docker compose up` biasa (port bawaan, setelah `make docker/infra/up`) juga menjalankan service
`migrate` lalu `seed`; API baru naik setelah migrasi sukses.

## Tes

```bash
go -C apps/api vet ./...
go -C apps/api test ./... -count=1     # butuh Docker (dockertest); tambah -short untuk melewati integrasi
make test/api/integration              # hanya paket integrasi
pnpm -C apps/web typecheck
pnpm -C apps/web test
```

## Deploy

Lihat [docs/DEPLOYMENT.md](docs/DEPLOYMENT.md). Deploy menjalankan migrasi sungguhan (gagal keras)
dan seed `reference`/`admin` saja, tidak pernah `demo`.
