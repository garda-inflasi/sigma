# 01 — Autentikasi, MFA, dan sesi

**Prioritas:** P0 · **Status:** UI ✅ · Backend ✅ · Terhubung ✅ · Skema & seed ✅ · Tes ✅
**Cara kerja & DoD:** [00-cara-kerja.md](00-cara-kerja.md) · **Konteks data:** [19](19-migration-seeder.md)

## Yang sudah ada

- **UI:** [ganti-password](<../../apps/web/src/app/(auth)/ganti-password/ganti-password-form.tsx>) (wajib saat `wajib_ganti_password`), [masuk](<../../apps/web/src/app/(auth)/masuk/masuk-form.tsx>) (email + kata sandi, ditambah "Masuk dengan Google" bila SSO dikonfigurasi: lihat 01-G), [lupa-password](<../../apps/web/src/app/(auth)/lupa-password/lupa-password-form.tsx>), halaman [akses-ditolak](../../apps/web/src/app/akses-ditolak/page.tsx).
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
3. **TES** (selesai): logika form (`lib/auth/reset-password.test.ts`) dan alur reset → login di integrasi Go. Form `lupa-password` diperbaiki agar memanggil BFF `/api/auth/forgot-password` (sebelumnya endpoint salah). E2E browser: lihat 01-D.

---

## 01-C — MFA: keamanan penyimpanan + UI _(Fase F, gelombang F-1)_

**Tujuan:** MFA benar-benar dipakai (wajib untuk lapis strategis, manajerial, koordinator, admin) dan rahasianya tidak tersimpan polos.
**Bergantung pada:** 01-A.

1. **MIG**
   - [x] Enkripsi `mfa_secret` memakai `pkg/crypto` (`Keyring`, versi kunci disimpan sebagai awalan `v1:`) beserta **migrasi data** Go versi 64 yang mengenkripsi nilai lama; Down memulihkannya (memerlukan `AES_KEY` yang sama).
   - [x] Tabel `mfa_kode_cadangan (user_id, kode_hash, dipakai_pada)` (migrasi 000063); kode polos di `pengguna.mfa_backup_codes` dipindahkan sebagai hash SHA-256 `"<id>:<kode>"` lalu kolomnya dibuang (Down tidak dapat memulihkan kode polos). Tambahan: `mfa_langkah_terakhir` agar satu kode TOTP tidak berlaku dua kali.
2. **SEED** (selesai): `user_mfa` dibuat ulang dengan rahasia bersegel dan kode cadangan ter-hash; rahasia dan kode demo (`DemoMFASecret`, `DemoKodeCadangan`) hanya ada di seeder demo.
3. **API**
   - [x] Verifikasi alur: login yang memerlukan MFA harus menandai butuh kode dan baru menerbitkan token penuh setelah `mfa/validate`.
   - [x] Kode cadangan: buat, pakai sekali, regenerasi.
   - [x] Paksa MFA untuk peran wajib (strategis, manajerial, koordinator, admin, dan akun bertanda `mfa_wajib`): sebelum setup, hanya endpoint setup MFA yang boleh diakses (403 `MFA_SETUP_REQUIRED`). Dikendalikan `MFA_ENFORCE` (kosong = hanya di `APP_ENV=production`), sehingga lokal/demo tetap bisa masuk; akun yang sudah ber-MFA selalu ditantang OTP.
4. **FE**
   - [x] Route BFF untuk `/v1/auth/mfa/*` (belum ada).
   - [x] Langkah kode OTP di login; halaman setup MFA (QR, verifikasi kode pertama, tampilkan kode cadangan sekali, nonaktifkan).
   - [x] Arahkan peran wajib yang belum setup ke halaman setup.
5. **TES** (selesai; e2e browser login+OTP: lihat 01-D): usecase MFA (setup, validasi, kode cadangan, pemaksaan peran); e2e login+OTP.

---

## 01-D — Email dev/demo (Mailpit), logout nyata, dan e2e halaman auth _(integrasi endpoint)_

**Tujuan:** setiap endpoint auth dipakai UI dan teruji ujung ke ujung, termasuk email atur ulang kata sandi. Tanpa migration.

1. **API/infra**
   - [x] `pkg/email`: `SMTP_USERNAME` kosong = tanpa `AUTH` (sebelumnya selalu `smtp.PlainAuth`, yang ditolak Go untuk server non-localhost tanpa TLS). Dipakai bersama notifikasi EWS (`SendHTML`); tes memakai server SMTP palsu (tanpa AUTH, dengan AUTH, dan kredensial ke server tanpa AUTH harus galat).
   - [x] `compose.yaml`: service `mailpit` (`axllent/mailpit:v1.27.0`) di profil `mailpit`, jaringan `garda_inflasi_net`, UI/API hanya di `127.0.0.1:${MAILPIT_PORT:-8025}`, port SMTP tidak dipublikasikan. `SMTP_*` bawaan tetap kosong. `make docker/demo/up` menyalakan profil dan memasang `SMTP_HOST=mailpit SMTP_PORT=1025` (`DEMO_MAILPIT_PORT` mengubah port host). Produksi (`compose.prod.yaml` + skrip deploy) tidak menyebut profil itu sehingga Mailpit tidak pernah berjalan; dibuktikan dengan `docker compose -f compose.yaml -f compose.prod.yaml config --services`.
2. **FE**
   - [x] Tombol "Masuk dengan Google", pemisah "atau", dan seluruh pipa `googleSso` dihapus dari `/masuk`.
   - [x] Menu "Keluar" (topbar dan menu profil lapangan) sebelumnya hanya `<Link href="/masuk">` sehingga sesi tidak pernah dicabut. Kini memanggil `POST /api/auth/logout` (cookie dihapus, token masuk daftar cabut) lalu memuat ulang `/masuk` (`lib/auth/logout.ts`, teruji unit).
3. **TES**
   - [x] E2E [auth.spec.ts](../../apps/web/e2e/auth.spec.ts) lewat form sungguhan: masuk ke beranda peran, kata sandi salah, keluar (cookie hilang, rute terlindungi dialihkan), MFA dengan TOTP RFC 6238 (retry satu kali bila langkah waktu sudah terpakai), lupa kata sandi → email di Mailpit → atur ulang → login sandi baru, tautan bekas ditolak, ganti kata sandi wajib untuk pengguna baru (+ salah kata sandi saat ini), serta cek API: jawaban forgot-password generik, token reset palsu, change-password salah. Pengguna uji `e2e.<timestamp>.<n>@gardainflasi.id` dibuat dan dihapus lewat API; akun seed tidak diubah kata sandinya.
   - [x] Anggaran laju BFF (login 5/menit, forgot-password 3/menit pada jendela yang sama) dikelola `anggaranBff` di [auth-helpers.ts](../../apps/web/e2e/auth-helpers.ts): menunggu deterministik, tanpa retry acak.
   - [x] Email ditangkap Mailpit; e2e membacanya lewat `E2E_MAILPIT_URL` (bawaan `http://localhost:8025`) dan gagal dengan pesan jelas bila tidak terjangkau.
   - Catatan: stack demo yang sedang berjalan harus dibangun ulang (`make docker/demo/up`) agar API memakai Mailpit dan menu Keluar baru.

---

## 01-G — SSO Google (OIDC)

**Tujuan:** masuk dengan akun Google yang dipetakan ke akun Garda Inflasi yang **sudah ada** (tidak ada pendaftaran
mandiri), tanpa melemahkan aturan login kata sandi. **Migration:** `000106_01g_sso_google.sql`.

**Alur** (BFF Next.js menjembatani cookie; API Go memegang semua keputusan keamanan):

1. `GET /api/auth/sso/google?lanjut=…` (Next) → `POST /v1/auth/sso/google/authorize` (Go) membangkitkan `state`, `nonce`,
   dan `code_verifier`, menyimpannya, dan mengembalikan URL otorisasi (PKCE S256) serta `binding`. BFF menaruh
   `binding` (+ tujuan `lanjut`) di cookie `garda_sso` lalu mengalihkan browser ke Google.
2. Google kembali ke `GET /api/auth/sso/google/callback` (Next) → `POST /v1/auth/sso/google/callback` (Go) menukar kode,
   memverifikasi ID token, memetakan akun, lalu BFF memasang cookie persis seperti login kata sandi dan mengalihkan ke
   beranda peran / `lanjut` aman, ke `/masuk?mfa=1` (langkah OTP) untuk akun ber-MFA, atau ke
   `/masuk?error=sso_<kode>` dengan pesan terbaca. Rute `/api/auth/sso` publik di edge guard.
3. Tombol hanya tampil bila `GET /v1/auth/sso/status` (dipanggil server component `/masuk` pada tiap render, bukan flag
   build) melaporkan SSO terkonfigurasi; galat atau waktu habis berarti disembunyikan.

**Keputusan keamanan**

| Hal | Keputusan | Alasan |
|---|---|---|
| Tempat `state`/`nonce`/`code_verifier` | Tabel `sso_login_state` di Postgres; `state` dan `binding` hanya sebagai hash SHA-256; nonce dan verifier tidak pernah ke browser | Tidak membocorkan rahasia PKCE/nonce ke cookie atau URL; dapat dipakai sekali secara atomik di banyak instance API (`DELETE … RETURNING`); umur 10 menit; baris kedaluwarsa dibersihkan saat alur baru dimulai |
| Login CSRF | `binding` acak di cookie httpOnly, SameSite=**Lax**, 10 menit (`__Host-garda_sso` saat Secure); dikirim BFF ke Go dan dicocokkan dengan hash | State saja tidak cukup: penyerang dapat memulai alur sendiri lalu memaksa korban membuka callback-nya. Tanpa cookie browser pemulai, callback ditolak. Lax wajib karena callback adalah navigasi lintas situs dari Google (cookie Strict tidak terkirim) |
| Replay | State dihapus pada percobaan pertama apa pun hasilnya (termasuk binding salah); cookie alur dihapus pada setiap jawaban callback; kode otorisasi sekali pakai di Google | Callback yang diputar ulang atau dicuri tidak bisa dipakai lagi |
| Session fixation | Sesi baru diterbitkan lewat `completeLogin` (keluarga refresh token baru); tidak ada ID sesi dari luar yang dipakai ulang; cookie `garda_access`/`garda_refresh`/`garda_csrf` ditimpa | Sama dengan login kata sandi |
| ID token | Tanda tangan lewat JWKS (`go-oidc`), `iss`, `aud`, `exp`, `nonce` (pembanding waktu-konstan), `azp` bila audiens ganda, `email_verified` wajib benar, `sub` wajib, `hd` bila `SSO_GOOGLE_HOSTED_DOMAIN` diisi; `alg=none` ditolak | OIDC Core §3.1.3.7 |
| Pemetaan akun | Cocokkan menurut `sub` lebih dulu (stabil walau email berubah), lalu email (tak peka huruf besar-kecil). Akun harus ada dan `AKTIF`. Masuk pertama menautkan `sso_provider`/`sso_subject` secara atomik (`WHERE sso_subject IS NULL`); UNIQUE gabungan mencegah satu identitas menaut dua akun. Email milik akun yang bertaut ke `sub` lain ditolak (409) dan diaudit `LOGIN` ditolak, alasan `sso_subjek_berbeda` | Email bisa berpindah tangan; `sub` tidak |
| Tidak aktif / terkunci | Ditolak dengan aturan yang sama dengan login kata sandi (`accountLocked`, status), sebelum penautan | Satu mekanisme kunci, tidak digandakan |
| MFA | Akun dengan MFA aktif menerima token tantangan yang sama (`CreateMFAChallengeToken`) dan melanjutkan ke langkah OTP yang ada; akun wajib MFA tanpa setup menerima klaim `wajib_setup_mfa` yang sama | Google tidak menggantikan faktor kedua |
| Audit | `LOGIN` berhasil memuat `metadata.metode = sso_google` (lewat konteks ke `completeLogin`); `SSO_LINK` saat penautan; `SSO_LOGIN` (menunggu_mfa) bagi akun ber-MFA; kegagalan memuat `alasan` | Jejak lengkap tanpa menyimpan rahasia |
| Open redirect | `lanjut` disaring `validateRedirectUrl` + `safeInternalPath` (menolak `//host`, `/\host`, URL absolut, rute `/api/`); pengalihan BFF memakai `Location` relatif | Tidak mempercayai header Host |
| Konfigurasi | `SSO_GOOGLE_CLIENT_ID`, `_CLIENT_SECRET`, `_ISSUER` (bawaan Google), `_REDIRECT_URL`, `_HOSTED_DOMAIN` (opsional). Client id kosong = SSO mati (503 `SSO_NOT_CONFIGURED`). Penemuan penerbit malas dengan jeda ulang 5 dtk: API tetap start bila Google tidak terjangkau. `prodcheck` memvalidasi di produksi (https, path callback tepat, origin ada di `APP_ALLOWED_ORIGINS`, rahasia terisi) | |

**Dev/demo/e2e:** mock OIDC `ghcr.io/navikt/mock-oauth2-server:6.1.4` (service `sso-mock`, profil compose `sso-mock`,
dinyalakan `make docker/demo/up`, tidak pernah di produksi). Penerbit `http://sso.localhost:8089/default` identik bagi browser
(loopback host) dan container API (alias jaringan `sso.localhost`, port sama). Panduan masuk dengan mock di README (bagian demo);
Google sungguhan di [DEPLOYMENT §3.7](../DEPLOYMENT.md).

**Tes:** `pkg/sso` (IdP palsu dalam proses: PKCE, nonce, audiens, penerbit, kedaluwarsa, tanda tangan, alg none, hd, penemuan malas),
`usecase/sso_test.go` (state/binding/TTL, pemetaan, penautan, MFA, penolakan), `integration/sso_test.go` (container mock OIDC +
Postgres: tukar kode dan verifikasi ID token sungguhan), `prodcheck/sso_test.go`, BFF (`lib/bff/sso.test.ts`, `route.test.ts`
rute SSO), dan e2e `apps/web/e2e/sso.spec.ts`.

---

## Backlog lanjutan (belum dijadwalkan)

- [x] **SSO Google:** OIDC (Authorization Code + PKCE S256) dipetakan ke akun yang sudah ada, migration `sso_provider`/`sso_subject` dengan UNIQUE gabungan, tombol "Masuk dengan Google" kembali di `/masuk` beserta e2e-nya. Rancangan dan keputusan keamanan: [01-G](#01-g--sso-google-oidc).
- [ ] Mekanisme lockout satu saja: hitung dari `login_attempts` (sudah ber-index) atau kolom `terkunci_sampai`; buka-kunci oleh admin.
- [ ] Daftar dan pencabutan sesi aktif (`refresh_tokens`: `last_used_at`, `user_agent`; index `(user_id, revoked_at)`).
- [ ] Rotasi refresh token dengan deteksi pemakaian ulang (verifikasi sudah ada).
- [ ] Job pembersihan baris kedaluwarsa `jwt_blacklist`, `refresh_tokens`, `password_reset_tokens`.
- [ ] Pesan error login konsisten (akun terkunci, password kedaluwarsa, 429).
- [ ] Logout membersihkan store offline lapangan (IndexedDB). Cache TanStack Query sudah bersih karena logout memuat ulang halaman penuh (lihat 01-D).
- [ ] Coverage usecase auth dari 63% ke ≥ 80% (MFA, reset, lockout).
- [ ] Tes `lib/bff/*` (cookies, csrf, rate-limit, safe-redirect), tes middleware. (E2E halaman auth sudah ada: 01-D.)
- [x] Satukan tiga salinan OpenAPI (lihat [18 §B](18-platform-lintas-fitur.md)).
