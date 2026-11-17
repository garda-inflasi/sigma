# Garda Inflasi

Platform pengendalian inflasi daerah (Aceh): data harga pangan dari lapangan dan sumber resmi dikumpulkan,
divalidasi, dianalisis (peringatan dini, prediksi dengan penjelasan faktor, peta surplus–defisit), lalu
diterjemahkan menjadi intervensi dan policy brief. Arsitektur lengkap: [docs/ARCHITECTURE.md](docs/ARCHITECTURE.md).

## Ringkasan platform

| Lapis | Isi | Modul |
| --- | --- | --- |
| 1 Data | Aplikasi lapangan (PWA offline-first), validasi harga bertahap, master data, sumber data eksternal | `lapangan`, `harga`, `master`, `sumber-data`, `ingestion` |
| 2 Analitik | Peringatan dini (rule engine + SSE), prediksi dan XAI (retraining terjadwal), peta spasial 23 kabupaten/kota, cold storage dan telemetri suhu | `ews`, `prediksi`, `spasial`, `cold-storage` |
| 3 Keputusan | Dashboard operasional dan eksekutif, policy brief, KPI | `dashboard`, `eksekutif`, `policy-brief`, `kpi`, `siklus` |
| 4 Tindakan | Intervensi (kanban 3 siklus) dengan persetujuan berjenjang | `intervensi` |
| Tata kelola | Autentikasi (MFA, SSO Google, sesi), RBAC berbasis basis data, audit trail berantai hash, profil dan riwayat aktivitas akun | `auth`, `audit` |

Prinsip keamanan yang ditegakkan di API Go (bukan hanya UI): izin granular `domain.aksi` dari basis data,
**cakupan baris** per peran (Koordinator dan Agregator hanya kabupatennya, Mahasiswa hanya entrinya sendiri),
**pemisahan tugas** (pencatat, validator, dan penerbit harga tiga orang berbeda; pengaju persetujuan intervensi tidak boleh
memutusnya; Admin Sistem tanpa hak persetujuan bisnis), MFA, dan jejak audit append-only.

## Isi repositori

| Lokasi | Isi |
| --- | --- |
| `apps/api` | API Go (Gin + GORM, hexagonal per modul) dengan empat biner di `cmd/`: `api`, `worker`, `migrate`, `seed` (serta `gardactl`). Lihat [apps/api/README.md](apps/api/README.md) |
| `apps/web` | Web Next.js 16: konsol (`app/(console)`), aplikasi lapangan PWA (`app/lapangan`), dan BFF (`app/api`) |
| `apps/ml` | Layanan ML Python (pustaka standar saja): peramalan harga, backtest, faktor XAI. HTTP internal `/health`, `/train`, `/predict` |
| `db/migrations` | Migrasi goose (PostgreSQL), satu-satunya sumber kebenaran skema |
| `db/seeds` | Seeder SQL-first: `reference/` (aman di produksi), `admin/`, `demo/` (ditolak bila `APP_ENV=production`) |
| `packages/contracts` | Kontrak OpenAPI (`openapi.yaml`, sumber tunggal) dan kontrak bersama API-web |
| `docker`, `compose*.yaml` | Image (api, web, ml, caddy), stack infra, aplikasi, dan produksi |
| `scripts` | Build, deploy, rollback, backup |
| `docs/todos` | Rencana kerja per fitur; mulai dari [00-cara-kerja.md](docs/todos/00-cara-kerja.md) |

## Prasyarat

| Kebutuhan | Versi / catatan |
| --- | --- |
| Go | Sesuai `apps/api/go.mod` (juga `go.work` di akar) |
| Node.js + pnpm | Node LTS terkini; pnpm lewat `corepack enable` |
| Docker | Untuk PostgreSQL, Redis, demo penuh, dan **tes integrasi Go** (dockertest) |
| Python | 3.11+ untuk layanan ML lokal dan `make test/ml` (tanpa dependensi pihak ketiga) |
| `make` | Semua perintah di bawah memakai Makefile |

## Konfigurasi environment

`make setup/env` menyalin [env.example](env.example) ke `.env` (akar) dan `apps/web/.env.local`. Variabel yang paling
sering disentuh (daftar lengkap beserta penjelasan ada di `env.example`):

| Variabel | Fungsi |
| --- | --- |
| `APP_ENV` | `production` mengaktifkan cookie `Secure`, penegakan MFA, dan menolak seed demo |
| `DB_HOST` `DB_PORT` `DB_USER` `DB_PASSWORD` `DB_NAME` | Koneksi PostgreSQL (atau `DATABASE_URL`) |
| `REDIS_URL` | Cache, batas laju, pub/sub SSE EWS |
| `JWT_SECRET`, `AES_KEY` | Penandatanganan token dan enkripsi rahasia MFA (wajib kuat di produksi) |
| `SEED_ADMIN_EMAIL`, `SEED_ADMIN_PASSWORD` | Akun admin awal (`make db/seed/admin`, wajib ganti sandi saat masuk pertama) |
| `ML_SERVICE_URL` | Alamat layanan ML; kosong = pelatihan model menjawab 503, data demo tetap tampil |
| `INGEST_SCHEDULER`, `BPS_API_KEY` | Penjadwal ingestion sumber data (PIHPS, BMKG, BPS, Bapanas, OpenWeather) |
| `EWS_EVAL_INTERVAL`, `INTERVENSI_*` | Evaluasi EWS, kartu intervensi otomatis, eskalasi SLA |
| `JOB_*_INTERVAL` | Interval job worker (lihat bagian Worker) |
| `SMTP_*`, `VAPID_*` | Email dan Web Push |
| `ANTHROPIC_API_KEY`, `LLM_MODEL` | Generator policy brief; tanpa kunci memakai templat |
| `SSO_GOOGLE_*`, `MFA_ENFORCE` | SSO Google dan penegakan MFA |
| `NEXT_PUBLIC_API_BASE_URL`, `GO_API_BASE_URL` | Web: `/api` agar tampil dari API (kosong = mode mock); tujuan proxy dev |

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

Komponen tambahan (opsional, perlu bila fiturnya diuji):

```bash
cd apps/api && go run ./cmd/worker   # job terjadwal: KPI, dashboard, siklus, audit, pembersihan auth, retraining
cd apps/ml && python3 -m garda_ml    # layanan ML di :8000, lalu set ML_SERVICE_URL=http://localhost:8000
make ctl/ingest ARGS="daftar"        # utilitas CLI ingestion sumber data
```

**Worker** menjalankan job berinterval: `dashboard_refresh`, `kpi_hitung`, `siklus_evaluasi`, `audit_partisi`,
`audit_verifikasi` (rantai hash), `auth_pembersihan`, `prediksi_retraining` (retraining model dengan gerbang MAPE
terhadap baseline), serta penjadwal ingestion bila `INGEST_SCHEDULER` aktif. Interval diatur variabel `JOB_*_INTERVAL`.

Login dengan akun demo, mis. `superadmin@gardainflasi.id`, kata sandi `Garda2024!`.

MFA: akun yang sudah mengaktifkan MFA (di demo: `mfa@gardainflasi.id`, rahasia TOTP `JBSWY3DPEHPK3PXP`, kode
cadangan `BACKUP01`-`BACKUP03`) diminta kode OTP setelah kata sandi. Penegakan MFA bagi peran strategis,
manajerial, koordinator, dan admin diatur `MFA_ENFORCE` (kosong = hanya di `APP_ENV=production`), jadi lokal
dan demo tetap bisa masuk tanpa setup. Dengan `MFA_ENFORCE=true`, akun wajib-MFA yang belum setup diarahkan
ke `/mfa` dan sesinya hanya membuka endpoint setup.

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
Login lewat web dibatasi 5 percobaan per menit per IP (BFF). API Go yang dijangkau langsung lewat `/api/v1/*`
membatasi percobaan auth (login, OTP, lupa/atur ulang kata sandi, SSO) per IP juga: `AUTH_RATE_LIMIT_PER_MENIT`, bawaan 10
di produksi dan `100000` di demo (`DEMO_AUTH_RATE_LIMIT_PER_MENIT`), karena e2e login ke API Go langsung ratusan kali per putaran.

Email (reset kata sandi, notifikasi EWS) di demo tidak keluar ke internet: `make docker/demo/up` ikut
menyalakan [Mailpit](https://mailpit.axllent.org) (service `mailpit`, profil compose `mailpit`) dan
mengarahkan `SMTP_HOST=mailpit` `SMTP_PORT=1025` ke sana tanpa autentikasi (`SMTP_USERNAME` kosong = tanpa
AUTH). Kotak masuknya di `http://localhost:8025` (hanya loopback host; ubah dengan `DEMO_MAILPIT_PORT`),
API-nya di `http://localhost:8025/api/v1/messages` (dipakai e2e `auth.spec.ts`, variabel `E2E_MAILPIT_URL`).
Tautan reset memakai `APP_PUBLIC_URL` (`http://localhost:8090`). Produksi tidak menjalankan Mailpit:
profilnya tidak pernah diaktifkan oleh `compose.prod.yaml` maupun skrip deploy.

**Masuk dengan Google (mock OIDC).** `make docker/demo/up` juga menyalakan penyedia OIDC palsu
([mock-oauth2-server](https://github.com/navikt/mock-oauth2-server), service `sso-mock`, profil compose
`sso-mock`) dan mengisi `SSO_GOOGLE_*` API untuknya, sehingga tombol "Masuk dengan Google" tampil di
`/masuk`. Penerbitnya `http://sso.localhost:8089/default` (ubah port dengan `DEMO_SSO_MOCK_PORT`) dan harus
identik bagi browser dan API: browser memetakan `*.localhost` ke loopback lalu port host `127.0.0.1:8089`
yang dipublikasikan, sedangkan container API menjangkaunya lewat alias jaringan `sso.localhost` pada port
yang sama. Cara masuk:

1. Buka `http://localhost:8090/masuk`, klik **Masuk dengan Google**.
2. Di halaman login mock isi *username* dengan nilai bebas (menjadi identitas Google / klaim `sub`) dan, pada
   kolom klaim, JSON dengan email akun demo yang sudah ada, mis.
   `{"email": "superadmin@gardainflasi.id", "email_verified": true}`, lalu **Sign-in**.
3. Masuk pertama menautkan akun itu ke `sub` tadi; selanjutnya gunakan *username* yang sama. Email yang
   tidak punya akun, `email_verified` salah/hilang, atau akun yang sudah tertaut ke `sub` lain ditolak
   dengan pesan di `/masuk`. Akun ber-MFA (`mfa@gardainflasi.id`) lanjut ke langkah kode OTP.

E2E-nya `apps/web/e2e/sso.spec.ts` (melepas tautan akun seed lewat `jalankanSql` sesudahnya). Produksi tidak
pernah menjalankan mock ini: profil `sso-mock` tidak diaktifkan `compose.prod.yaml` maupun skrip deploy
(`docker compose -f compose.yaml -f compose.prod.yaml config --services` tidak memuatnya). Google sungguhan:
[docs/DEPLOYMENT.md §3.7](docs/DEPLOYMENT.md).

`docker compose up` biasa (port bawaan, setelah `make docker/infra/up`) juga menjalankan service
`migrate` lalu `seed`; API baru naik setelah migrasi sukses.

## Tes

```bash
go -C apps/api vet ./...
go -C apps/api test ./... -count=1     # butuh Docker (dockertest); tambah -short untuk melewati integrasi
make test/api/integration              # hanya paket integrasi
pnpm -C apps/web typecheck
pnpm -C apps/web test
make test/ml                           # unit Python layanan ML
make test/e2e                          # Playwright terhadap stack demo yang sedang berjalan
```

E2E (`apps/web/e2e`) berjalan terhadap stack Docker yang hidup, bukan server sementara: nyalakan
`make docker/demo/up` dulu (termasuk Mailpit, mock OIDC `sso-mock`, dan layanan ML). Stack itu wajib: e2e login ke API Go
langsung (`/api/v1/auth/login`) ratusan kali per putaran, dan hanya demo yang menyetel `AUTH_RATE_LIMIT_PER_MENIT` tinggi;
stack dengan bawaan (10/menit/IP) membuat e2e gagal dengan 429 `RATE_LIMITED`. Alamat web diatur `E2E_BASE_URL`
(bawaan `http://localhost:8090`), Mailpit `E2E_MAILPIT_URL` (bawaan `http://localhost:8025`), container
Postgres `E2E_PG_CONTAINER` (bawaan `garda-postgres`). Tes berjalan berurutan, memulihkan data yang
diubahnya, dan aman diulang. Data uji yang tidak punya endpoint hapus dibersihkan lewat SQL.

## Deploy

Lihat [docs/DEPLOYMENT.md](docs/DEPLOYMENT.md). Deploy menjalankan migrasi sungguhan (gagal keras)
dan seed `reference`/`admin` saja, tidak pernah `demo`.
