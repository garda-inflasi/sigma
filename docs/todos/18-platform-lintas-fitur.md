# 18 — Platform & pekerjaan lintas fitur

Hal-hal yang tidak dimiliki satu halaman, tetapi menghambat banyak fitur sekaligus. **Urutan dan aturan kerja ada di [00-cara-kerja.md](00-cara-kerja.md)**; bagian A di bawah hanya memetakan item lama ke task fondasi (F0/F1) di sana.

## A. Fondasi

| Item | Task |
|---|---|
| Perbaiki 4 tes cold-storage (domain email `@garda-inflasi.id` di tes vs `@gardainflasi.id` di seed) | **F0-2** |
| Dev proxy `/api/v1` agar `make dev/web` + `make dev/api` jalan tanpa Caddy; target `db/seed/*`; README akar | **F0-1** |
| Tes kontrak skema entity ↔ migrasi; seeder kit; tes asap seed | **F0-3, F0-4, F0-5** ([19](19-migration-seeder.md)) |
| Migrasi nyata saat deploy (image API tanpa goose, langkah dilewati diam-diam) | **F0-6** |
| Proteksi CSRF di Go untuk request ber-cookie; cookie `Secure` (`isDev = true` permanen di `cookies.ts`) | **F0-7** |
| Netralkan seed di migrasi 000006 | **F1** |
| Satukan sumber izin dan perbaiki route yang selalu 403 | **02-A** ([02](02-pengguna-rbac.md)) |

Keputusan arsitektur yang masih terbuka:

- [ ] **Pilih satu modul dashboard/prediksi (D4):** `dashboard` + `prediksi` (Postgres) atau `analytics` (MongoDB). Web tidak memanggil satupun route `analytics`. Putuskan sebelum 08-A.
- [ ] **Pola akses data web:** default **langsung** (`/api/v1/*` lewat `endpoints.ts`, Go membaca cookie `garda_access`), BFF hanya bila perlu agregasi/rahasia/unggah berkas. Alasan dan syarat di [00 §5](00-cara-kerja.md).
- [ ] **Endpoint yang diminta web tetapi belum ada:** `POST /v1/lapangan/harga` (07-B), `GET /v1/ews/stream` (10-C), `/v1/auth/sso/*` (01 backlog), endpoint KPI (16-A), bulk policy brief (15-C) dan model (11-C).
- [ ] Hapus binary `apps/api/seed-analytics` dari git dan tambahkan ke `.gitignore`.
- [ ] Hapus `apps/web/package-lock.json`, karena monorepo memakai pnpm (`pnpm-lock.yaml`).

## B. Kontrak API

- [x] Satukan tiga salinan OpenAPI. Sumber tunggal: `packages/contracts/openapi.yaml`; `apps/api/docs/openapi.yaml` dihapus; `apps/api/pkg/swagger/openapi.yaml` salinan hasil generasi (`make openapi/sync`, `make openapi/check`) dan `pkg/swagger/swagger_test.go` gagal bila berbeda atau bila path analytics lama muncul. `/swagger/openapi.yaml` menyajikan spesifikasi terkini (aktif selain `GIN_MODE=release`, atau `SWAGGER_ENABLED=true`).
- [ ] Hasilkan tipe TypeScript klien dari OpenAPI (ganti tipe manual di `lib/data/types.ts` yang mudah menyimpang).
- [ ] Tes kontrak di CI: respons backend cocok dengan skema.
- [ ] Konvensi: format error, paginasi (page/limit vs cursor), envelope respons `{ code, message, data }`, dan `Idempotency-Key` untuk semua POST yang mengubah data.
- [x] **Konvensi ID (7 Nov 2026, [SKEMA-DATABASE.md §4–5](../SKEMA-DATABASE.md)):** rute `/v1/<sumber>/:id` dengan `:id` UUID atau BIGINT (string desimal di JSON; skema OpenAPI `IdUuid`, `IdBigint`, `KodeTampilan`). Kode lama diterima sebagai alias. ID salah bentuk dijawab 404, hapus yang masih dirujuk dijawab 409. Handler memakai helper `apps/api/pkg/idx` (`ParamID`, `ParamUUID`, `ParamIDAtauKode`, `ParamUUIDAtauKode`); UI menampilkan `kode ?? id`.

## C. Data dan infrastruktur

- [ ] **Redis** untuk cache, rate limit terdistribusi, dan pub/sub SSE (lihat [10](10-ews.md)); periksa ada atau tidaknya di `compose.yaml` dan `compose.prod.yaml`.
- [ ] **Penyimpanan objek** (MinIO/S3) untuk foto lapangan dan lampiran brief.
- [x] **Job queue/scheduler** untuk ingestion, generator brief, dan perhitungan KPI. Penjadwal berkala selesai: `cmd/worker` + `internal/platform/jobs` (refresh dashboard, KPI; advisory lock; lihat [API README](../../apps/api/README.md) §6). Antrean job sesuai permintaan (queue) belum ada.
- [ ] **Layanan ML** terpisah (lihat [11](11-prediksi-xai.md)) dan pipelinenya.
- [ ] Strategi time-series (partisi tabel/TimescaleDB) dan retensi.
- [ ] **Migration dan seeder:** temuan dan tugas fondasi di [19-migration-seeder.md](19-migration-seeder.md); migration/seeder spesifik ada di dalam slice tiap fitur (langkah `MIG` dan `SEED`).

## D. Keamanan dan kepatuhan (PRD §5.2)

- [ ] Enkripsi data at-rest (disk/DB) dan in-transit (TLS di Caddy sudah ada; verifikasi koneksi ke DB/Mongo/Redis).
- [x] Manajemen rahasia: `.env` tidak boleh berisi rahasia produksi; pakai secret manager atau berkas rahasia pada server. `.env` lokal ter-ignore dengan benar. Penolakan start pada rahasia bawaan/lemah (API, worker, remote-deploy) selesai; secret manager belum.
- [ ] Pemindaian dependensi (`govulncheck`, `pnpm audit`) dan SAST di CI.
- [ ] Kepatuhan UU PDP dan SPBE: inventaris data pribadi (pengguna, petugas, lokasi GPS), dasar pemrosesan, masa simpan, hak subjek data.
  - `pengguna.telepon` (20-A, migrasi `000123`): telepon/WhatsApp pemilik akun, diubah hanya oleh pemiliknya lewat `PATCH /v1/auth/me`.
    - **Tampil di:** profil sendiri dan respons pengguna untuk pemegang `pengguna.kelola`. Tidak masuk JWT dan tidak masuk ekspor CSV.
    - **Tercatat di:** snapshot `audit_log` (`UPDATE_PROFIL` dan audit pengguna lain). Audit itu append-only, jadi nilainya tidak bisa dihapus kemudian; samarkan di `userSnapshot` bila kebijakan PDP menuntutnya.
- [x] Backup, restore yang pernah diuji, dan rencana pemulihan bencana (RPO/RTO tertulis). Selesai sebagian: `scripts/backup` + [DEPLOYMENT.md §10](../DEPLOYMENT.md) (RPO harian, RTO terukur 1 detik pada data demo). Pemulihan bencana lintas-VM belum.
- [x] Tinjauan header keamanan, CSP, dan CORS pada konfigurasi produksi. Ditinjau: lihat [DEPLOYMENT.md §12b](../DEPLOYMENT.md); CSP halaman Next.js masih terbuka.

## E. Observabilitas

- [ ] Log terstruktur dengan correlation id dari Next.js BFF ke Go.
- [ ] Metrik (Prometheus) dan dasbor: latensi, error rate, antrean, kegagalan ingestion.
- [ ] Health/readiness endpoint untuk API, web, dan dependensi.
- [ ] Pelacakan galat (mis. Sentry) di web dan API.

## F. Kualitas dan CI/CD

- [ ] **Pipeline CI** (GitHub Actions): lint, typecheck, `go vet`, `go test` (termasuk integrasi dengan Docker), `pnpm test`, build image. Belum ada berkas CI di repo.
- [ ] Coverage minimum per modul Go; saat ini hanya `auth` (63%) dan `cold-storage` yang terukur baik, banyak modul 0 tes (`analytics`, `audit`, `dashboard`, `prediksi`, `policy-brief`, `siklus`, `spasial`).
- [ ] Tes komponen web (Testing Library) dan tes e2e (Playwright) untuk alur kritis: login, input lapangan offline → sinkron, validasi harga, EWS.
- [ ] Uji beban untuk endpoint agregasi dan SSE.
- [ ] Ubah `apps/web` agar `lint` dijalankan di CI (`eslint .` sudah ada di skrip).
- [ ] Alur rilis: versi, changelog, dan rollback (skrip rollback sudah ada di `scripts/deploy`).

## G. Deployment

- [ ] Uji `make` deploy ujung ke ujung pada server staging; dokumentasikan hasil di [DEPLOYMENT.md](../DEPLOYMENT.md).
- [ ] Pastikan `NEXT_PUBLIC_API_BASE_URL` terisi saat build image web. Bila kosong, aplikasi diam-diam jatuh ke mode mock (lihat komentar di `docker/web.Dockerfile`). Tambahkan pemeriksaan build yang gagal bila kosong di produksi.
- [ ] Pemantauan pasca-deploy (smoke test otomatis).
- [ ] **Langkah migrasi di `remote-deploy.sh` saat ini dilewati diam-diam** (image API tidak berisi goose maupun `/migrations`, dan `|| echo` menelan kegagalan). Perbaikan ada di task F0-3 dan F0-6 ([19 §5](19-migration-seeder.md)).
- [ ] Rencana migrasi data dari lingkungan pilot ke produksi.

## H. Dokumentasi

- [ ] Perbarui atau hapus [session-summary.md](../session-summary.md), yang masih membahas masalah hydration/scroll awal Oktober.
- [ ] Buat `README.md` di akar repo (langkah menjalankan lokal, variabel env, perintah penting). Saat ini hanya `start.md` yang menjelaskan seeder analytics dan Swagger.
- [ ] Perbarui [ARCHITECTURE.md](../ARCHITECTURE.md) agar kontrak API sesuai route nyata (mis. `/v1/auth/sso/start` tidak ada).
- [ ] Catat keputusan arsitektur penting (ADR): pola BFF, pilihan Postgres+Mongo, layanan ML terpisah.
- [ ] Pembaruan berkala daftar todo ini setiap selesai satu fase.

## I. Peta jalan terhadap fase PRD

| Fase PRD                            | Isi utama                                                             | Berkas terkait                                                                                                                                                                              |
| ----------------------------------- | --------------------------------------------------------------------- | ------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| **Tahap 1** (Fondasi & MVP)         | Auth, RBAC, dashboard dasar, input lapangan                           | [01](01-auth.md), [02](02-pengguna-rbac.md), [06](06-harga-validasi.md), [07](07-lapangan-pwa.md), [08](08-dashboard-operasional.md) — hampir selesai, sisa penyambungan ke backend dan MFA |
| **Tahap 2** (Agregasi & Distribusi) | Dashboard eksekutif, prediksi harga, EWS, policy brief, akuisisi data | [05](05-sumber-data.md), [09](09-dashboard-eksekutif.md), [10](10-ews.md), [11](11-prediksi-xai.md), [15](15-policy-brief.md) — belum dimulai di sisi AI/data                               |
| **Tahap 3** (Eskalasi & Fisik)      | Workflow engine, cold storage, optimasi rute, CV QC                   | [12](12-peta-spasial.md), [13](13-cold-storage.md), [14](14-intervensi.md), [16](16-kpi.md)                                                                                                 |
