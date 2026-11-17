# 21 — Impersonasi peran dan akun (alih akun sementara)

**Prioritas:** belum ditetapkan · **Status:** UI ✅ · Backend ✅ · Terhubung ✅ · Skema & seed ✅ (migrasi `000129`) · Tes ✅ (17 Nov 2026)
**Cara kerja & DoD:** [00-cara-kerja.md](00-cara-kerja.md) · **Spesifikasi dan keputusan desain:** [../impersonasi.md](../impersonasi.md) · **Terkait:** [01-auth.md](01-auth.md), [02-pengguna-rbac.md](02-pengguna-rbac.md), [03-audit.md](03-audit.md)

## Konteks & tujuan

`superadmin` (peran `ADMIN_SISTEM`) dan akun yang diizinkan kebijakan dapat sementara bekerja sebagai akun lain untuk memeriksa tampilan, izin, dan alur kerja sebuah peran, tanpa kata sandi akun itu, dengan seluruh aksi tetap dipertanggungjawabkan kepada akun asli (audit hash berantai).

## Keputusan yang menyimpang dari brief awal (alasannya di [impersonasi.md](../impersonasi.md))

- Brief menyebut path `domain/identity`, `wiring/rbac.go`, `rbac-roles.json`, `api-routes.json`, `(dashboard)/layout.tsx`, `AppTopbar`, `UserMenu`, `nav-sections.ts`: tidak ada di repo ini. Padanannya ada di tabel pemetaan [impersonasi.md](../impersonasi.md#pemetaan-istilah-brief---kode-nyata).
- Impersonasi selalu atas **akun** (token milik sasaran + klaim `imp`), bukan menimpa `role` di token: modul lain menentukan peran dan cakupan kabupaten dari DB berdasarkan `user_id`. "Impersonasi ke peran" diselesaikan server menjadi tepat satu akun (409 `TARGET_AMBIGU` bila lebih).
- `permissionsVersion` tidak ada di basis kode ini; padanannya klaim `permissions` + penolakan `PERMISSIONS_CHANGED`.
- Izin `impersonasi.kelola` (format `domain.aksi`, bukan `impersonation:policy:manage`) eksklusif `ADMIN_SISTEM`; ditegakkan juga di resolver izin efektif sehingga tidak bergantung pada seed.

## Slice 21-A (selesai)

- [x] **MIG** [`000129_21a_impersonasi.sql`](../../db/migrations/000129_21a_impersonasi.sql): `impersonasi_kebijakan`, `impersonasi_pelaku`, `impersonasi_target`, `impersonasi_sesi`, `refresh_tokens.impersonasi_id`. Tanpa data; `down` lalu `up` lulus; tabel terdaftar di matriks PK ([kunci_test.go](../../apps/api/internal/platform/schematest/kunci_test.go)) dan komparator model.
- [x] **SEED** [`03_auth_rbac.sql`](../../db/seeds/reference/03_auth_rbac.sql): izin `impersonasi.kelola` (36 izin) dan matriks `ADMIN_SISTEM`. Stack yang sudah berjalan: `seed reference` sekali sesudah migrasi.
- [x] **API** token (`pkg/token/impersonation.go`, klaim `imp`, `ImpersonationGuard`), middleware (`JWTAuth`: validasi per request, rute terlarang, pencatatan mutasi, `RequireRole`), audit (`aktor_id` = pelaku asli + `metadata.impersonasi`, masuk hash berantai), usecase/repositori/handler/rute (`/v1/auth/impersonation/{targets,start,stop}`, `/v1/admin/impersonation/{policy,riwayat,riwayat/:id/akhiri}`), `GET /v1/auth/me` memuat `impersonasi`, OpenAPI, `permissions.json`.
- [x] **FE** topbar/banner/dialog/halaman `/impersonasi`, BFF `/api/impersonation/*`: lihat [impersonasi.md](../impersonasi.md).
- [x] **TES** Go: tes integrasi `auth/integration/impersonasi_test.go` (alur penuh terhadap Postgres), tes unit token, middleware, audit, usecase. Web: tes unit/komponen, typecheck, lint. Spesifikasi e2e Playwright (`halaman.spec.ts`, `ui-audit.spec.ts`) sudah diperbarui tetapi **belum dijalankan** (butuh stack penuh); tampilan banner/topbar belum diperiksa di browser sungguhan.

## Backlog lanjutan

- [ ] Konfirmasi ulang (kata sandi atau OTP) sebelum memulai impersonasi (step-up); belum diminta.
- [ ] Notifikasi kepada akun sasaran (email atau lonceng) bahwa akunnya diimpersonasi.
- [ ] Filter dan ekspor riwayat impersonasi (pelaku, sasaran, rentang waktu); kini hanya aktif/semua dengan cursor.
- [ ] Batas jumlah impersonasi aktif serentak per pelaku.
