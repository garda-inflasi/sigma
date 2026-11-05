# 01 — Autentikasi, MFA, dan sesi

**Prioritas:** P0 · **Status:** UI 🟡 · Backend ✅ · Terhubung ✅ · Skema & seed ✅ · Tes ✅
**Cara kerja & DoD:** [00-cara-kerja.md](00-cara-kerja.md) · **Konteks data:** [19](19-migration-seeder.md)

## Yang sudah ada

- **UI:** [ganti-password](<../../apps/web/src/app/(auth)/ganti-password/ganti-password-form.tsx>) (wajib saat `wajib_ganti_password`), [masuk](<../../apps/web/src/app/(auth)/masuk/masuk-form.tsx>) (email + kata sandi, tombol SSO Google), [lupa-password](<../../apps/web/src/app/(auth)/lupa-password/lupa-password-form.tsx>), halaman [akses-ditolak](../../apps/web/src/app/akses-ditolak/page.tsx).
- **BFF Next.js** (`app/api/auth/*`): login, logout, refresh, me, change-password, forgot-password, reset-password. Cookie `garda_access`/`garda_refresh`, CSRF, rate limit, redirect aman di `lib/bff/*`.
- **Middleware edge** ([middleware.ts](../../apps/web/src/middleware.ts)): proteksi rute, refresh token otomatis, header keamanan, RBAC tingkat rute.
- **API Go:** `POST /v1/auth/{login,refresh,forgot-password,reset-password,logout,change-password}`, `GET /v1/auth/me`, MFA `/v1/auth/mfa/{setup,verify,validate,disable}`.
- **Skema:** entity auth cocok dengan migrasi (dicek skrip + integration test lolos). Tabel: `pengguna` (+ `mfa_secret`, `mfa_backup_codes`, `mfa_wajib` dari 000004), `refresh_tokens`, `login_attempts`, `password_reset_tokens`, `jwt_blacklist`, `password_history` (dibuang di 01-A karena tidak dirujuk kode).
- **Tes:** [integration_test.go](../../apps/api/internal/auth/integration/integration_test.go) dan [usecase_test.go](../../apps/api/internal/auth/usecase/usecase_test.go) (coverage 63%).

---

## 01-A — Akun lewat seeder + wajib ganti password _(Fase P, urutan #1)_

**Tujuan:** akun tidak lagi lahir dari migrasi. Admin awal dibuat dari env, akun demo dari seeder, dan akun yang password-nya ditentukan pihak lain wajib diganti saat masuk pertama.
**Bergantung pada:** F0-4 (seeder kit). **Sentuh juga:** F1 menghapus akun dari 000006 setelah slice ini.

1. **MIG**
   - [x] `pengguna`: tambah `wajib_ganti_password BOOLEAN NOT NULL DEFAULT false` dan `password_diubah_pada TIMESTAMPTZ`.
   - [x] Email unik tak peka huruf besar-kecil: `CREATE UNIQUE INDEX … ON pengguna (lower(email))`.
2. **SEED**
   - [x] Lapis `admin`: satu akun dari `SEED_ADMIN_EMAIL` + `SEED_ADMIN_PASSWORD` (atau password acak dicetak sekali), `wajib_ganti_password = true`, MFA wajib; hanya dibuat bila belum ada admin.
   - [x] Lapis `demo`: pindahkan 25 akun dari 000006 (semua 14 peran terwakili, 10 mahasiswa, 1 `NONAKTIF`, 1 MFA aktif). Password demo tetap `Garda2024!` **hanya di seeder demo**.
   - [x] Domain email: ikuti F0-2.
3. **API**
   - [x] Respons login dan `GET /v1/auth/me` memuat `wajib_ganti_password`.
   - [x] Selama flag aktif, semua endpoint selain `change-password`, `logout`, dan `me` menjawab 403 dengan kode `PASSWORD_CHANGE_REQUIRED`. `change-password` mematikan flag dan mengisi `password_diubah_pada`.
   - [x] Pastikan `password_history` benar-benar dipakai untuk aturan "tidak boleh memakai ulang password lama", atau hapus tabelnya lewat migration lain.
4. **FE**
   - [x] Halaman ganti password wajib (menampilkan aturan password policy) dan pengalihan dari login/middleware bila flag aktif. BFF `change-password` sudah ada.
   - [x] Tempat memanggil ganti password sukarela (profil/topbar).
5. **TES**
   - [x] Integration: admin seed → login → 403 `PASSWORD_CHANGE_REQUIRED` pada endpoint lain → ganti password → akses normal.
   - [x] Tes: setelah `reference + admin` tidak ada akun ber-password default.
6. **DoD** — checklist [00 §4](00-cara-kerja.md). Khusus: login semua akun demo berhasil dengan peran yang benar.

---

## 01-B — Halaman reset password _(Fase F, gelombang F-1)_

**Tujuan:** tautan di email reset tidak lagi berujung 404. Tanpa migration.

1. **API** (selesai): token dipakai atomik (`ConsumeResetToken`), kedaluwarsa dan terpakai dibedakan di pesan, token lain akun yang sama hangus, sesi dicabut; tautan email mengarah ke `/reset-password` (`APP_PUBLIC_URL`).
2. **FE**
   - [x] Halaman `/reset-password` (token dari query, form password baru + konfirmasi, aturan password policy). BFF `/api/auth/reset-password` sudah ada; rute sudah ada di `PUBLIC_ROUTES`.
   - [x] Pesan jelas untuk token kedaluwarsa/terpakai, lalu arahkan ke `/masuk`.
   - [x] Hapus `/daftar` dari `PUBLIC_ROUTES` bila tidak ada halamannya.
3. **TES** (selesai): logika form (`lib/auth/reset-password.test.ts`) dan alur reset → login di integrasi Go. Form `lupa-password` diperbaiki agar memanggil BFF `/api/auth/forgot-password` (sebelumnya endpoint salah). E2E browser belum ada.

---

## 01-C — MFA: keamanan penyimpanan + UI _(Fase F, gelombang F-1)_

**Tujuan:** MFA benar-benar dipakai (wajib untuk lapis strategis, manajerial, koordinator, admin) dan rahasianya tidak tersimpan polos.
**Bergantung pada:** 01-A.

1. **MIG**
   - [ ] Enkripsi `mfa_secret` memakai `pkg/crypto` (simpan versi kunci) beserta **migrasi data** yang mengenkripsi nilai lama. Down harus dapat memulihkan.
   - [ ] Tabel `mfa_backup_codes (user_id, kode_hash, dipakai_pada)`; pindahkan `mfa_backup_codes` JSONB (polos) menjadi hash.
2. **SEED**: akun demo MFA dibuat ulang dengan secret terenkripsi dan kode cadangan ter-hash; secret dan kode demo ditandai demo.
3. **API**
   - [ ] Verifikasi alur: login yang memerlukan MFA harus menandai butuh kode dan baru menerbitkan token penuh setelah `mfa/validate`.
   - [ ] Kode cadangan: buat, pakai sekali, regenerasi.
   - [ ] Paksa MFA untuk peran wajib: sebelum setup, hanya endpoint setup MFA yang boleh diakses.
4. **FE**
   - [ ] Route BFF untuk `/v1/auth/mfa/*` (belum ada).
   - [ ] Langkah kode OTP di login; halaman setup MFA (QR, verifikasi kode pertama, tampilkan kode cadangan sekali, nonaktifkan).
   - [ ] Arahkan peran wajib yang belum setup ke halaman setup.
5. **TES**: usecase MFA (setup, validasi, kode cadangan, pemaksaan peran); e2e login+OTP.

---

## Backlog lanjutan (belum dijadwalkan)

- [ ] **SSO Google:** tombol di login mengarah ke `/v1/auth/sso/google/*` yang tidak ada di backend. Implementasikan (OIDC + pemetaan akun ke peran; migration `sso_provider`, `sso_subject` dengan UNIQUE gabungan; `password_hash` sudah nullable) atau sembunyikan tombolnya. Putuskan dulu apakah SSO masuk scope (PRD §5.1 menyebut SSO).
- [ ] Mekanisme lockout satu saja: hitung dari `login_attempts` (sudah ber-index) atau kolom `terkunci_sampai`; buka-kunci oleh admin.
- [ ] Daftar dan pencabutan sesi aktif (`refresh_tokens`: `last_used_at`, `user_agent`; index `(user_id, revoked_at)`).
- [ ] Rotasi refresh token dengan deteksi pemakaian ulang (verifikasi sudah ada).
- [ ] Job pembersihan baris kedaluwarsa `jwt_blacklist`, `refresh_tokens`, `password_reset_tokens`.
- [ ] Pesan error login konsisten (akun terkunci, password kedaluwarsa, 429).
- [ ] Logout membersihkan cache TanStack Query dan store offline lapangan.
- [ ] Coverage usecase auth dari 63% ke ≥ 80% (MFA, reset, lockout).
- [ ] Tes `lib/bff/*` (cookies, csrf, rate-limit, safe-redirect), tes middleware, e2e Playwright penuh.
- [ ] Satukan tiga salinan OpenAPI (`apps/api/docs`, `apps/api/pkg/swagger`, `packages/contracts`).
