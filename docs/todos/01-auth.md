# 01 — Autentikasi, MFA, dan sesi

**Prioritas:** P0 · **Status:** UI ✅ · Backend ✅ · Terhubung ✅ · Skema & seed ✅ · Tes ✅
**Cara kerja & DoD:** [00-cara-kerja.md](00-cara-kerja.md) · **Konteks data:** [19](19-migration-seeder.md)

## Yang sudah ada

- **UI:** [ganti-password](<../../apps/web/src/app/(auth)/ganti-password/ganti-password-form.tsx>) (wajib saat `wajib_ganti_password`), [masuk](<../../apps/web/src/app/(auth)/masuk/masuk-form.tsx>) (email + kata sandi; tombol SSO Google dihapus karena API Go tidak punya rute SSO), [lupa-password](<../../apps/web/src/app/(auth)/lupa-password/lupa-password-form.tsx>), halaman [akses-ditolak](../../apps/web/src/app/akses-ditolak/page.tsx).
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

## Backlog lanjutan (belum dijadwalkan)

- [ ] **SSO Google:** tombolnya sudah dihapus dari login (rute `/v1/auth/sso/*` tidak pernah ada, tombol berujung 404). Bila SSO masuk scope (PRD §5.1 menyebut SSO): OIDC + pemetaan akun ke peran; migration `sso_provider`, `sso_subject` dengan UNIQUE gabungan (`password_hash` sudah nullable); lalu pasang kembali tombolnya bersama e2e-nya.
- [ ] Mekanisme lockout satu saja: hitung dari `login_attempts` (sudah ber-index) atau kolom `terkunci_sampai`; buka-kunci oleh admin.
- [ ] Daftar dan pencabutan sesi aktif (`refresh_tokens`: `last_used_at`, `user_agent`; index `(user_id, revoked_at)`).
- [ ] Rotasi refresh token dengan deteksi pemakaian ulang (verifikasi sudah ada).
- [x] Job pembersihan baris kedaluwarsa `jwt_blacklist`, `refresh_tokens`, `password_reset_tokens` (+ `login_attempts`). Lihat "Pembersihan tabel auth" di bawah.
- [ ] Pesan error login konsisten (akun terkunci, password kedaluwarsa, 429).
- [x] Logout membersihkan store offline lapangan (IndexedDB). Lihat "Logout dan data offline lapangan" di bawah. Cache TanStack Query sudah bersih karena logout memuat ulang halaman penuh (lihat 01-D).
- [ ] Coverage usecase auth dari 63% ke ≥ 80% (MFA, reset, lockout).
- [ ] Tes `lib/bff/*` (cookies, csrf, rate-limit, safe-redirect), tes middleware. (E2E halaman auth sudah ada: 01-D.)
- [x] Satukan tiga salinan OpenAPI (lihat [18 §B](18-platform-lintas-fitur.md)).

---

## Pembersihan tabel auth (job `auth_pembersihan`)

Job worker (`cmd/worker/register_auth.go`, usecase `internal/auth/usecase/pembersihan.go`, SQL `internal/auth/repository/postgres/repo_pembersihan.go`) menghapus baris mati agar tabel auth tidak tumbuh tanpa batas. Tanpa migration.

| Tabel | Dihapus bila | Dipertahankan |
|---|---|---|
| `jwt_blacklist` | `expires_at` sudah lewat | yang masih berlaku (pembacaan blacklist sudah mengabaikan yang kedaluwarsa, jadi perilaku tidak berubah) |
| `refresh_tokens` | `expires_at` sudah lewat | token **dicabut tetapi belum kedaluwarsa**: `Refresh` memakainya untuk mendeteksi pemakaian ulang dan mencabut seluruh keluarga |
| `password_reset_tokens` | kedaluwarsa atau terpakai lebih dari **7 hari** lalu | token hidup; token yang baru mati disimpan 7 hari karena `ResetPassword` membedakan "sudah kedaluwarsa" dari "tidak valid atau sudah dipakai" bagi tautan yang tidak ditemukan, dan barisnya berguna untuk debug keluhan pengguna |
| `login_attempts` | lebih tua dari **90 hari** | yang lebih muda; **lantai mutlak 1 hari** (`MinRetensiLoginAttempt`) berapa pun konfigurasinya, karena perhitungan penguncian akun membaca jendela waktu pendek dari tabel ini |

- **Batch:** tiap `DELETE` memilih paling banyak 1000 id (`ORDER BY id LIMIT … FOR UPDATE SKIP LOCKED`), berjalan dalam transaksi dengan `statement_timeout` 30 detik; baris yang sedang dikunci transaksi lain dilewati dan disapu putaran berikutnya. Satu putaran memproses paling banyak 500 batch per tabel (`terpotong` tercatat di log) dan dibatasi `Timeout` job 5 menit. Gagal di satu tabel tidak menghentikan tabel lain; galatnya digabung dan tercatat di `/healthz`.
- **Jadwal dan env:** `JOB_AUTH_PEMBERSIHAN_INTERVAL` (bawaan `1h`, `off` menonaktifkan), `JOB_AUTH_PEMBERSIHAN_LOGIN_HARI` (bawaan `90`, minimal `1`, `off` = `login_attempts` tidak dihapus), `JOB_AUTH_PEMBERSIHAN_RESET_HARI` (bawaan `7`, `0` = langsung), `JOB_AUTH_PEMBERSIHAN_BATCH` (bawaan `1000`). Nilai di luar rentang membuat worker gagal start. Terdaftar di `env.example`, `compose.yaml` (service `worker`), dan `docs/DEPLOYMENT.md`. Manual: `worker run auth_pembersihan`.
- **Menambah tabel (mis. sesi):** tambahkan satu `Langkah` di `langkahBawaan` (atau lewat argumen `tambahan` `NewPembersihan`) beserta metode di `domain.PembersihanRepository` dan repositori; orkestrasi tidak perlu diubah.
- **Indeks:** `refresh_tokens.expires_at`, `password_reset_tokens.expires_at`, dan `login_attempts.attempted_at` tidak berindeks. Kueri memakai urutan kunci primer (id berkorelasi dengan waktu) sehingga memadai pada volume saat ini; bila tabel membesar, tambahkan indeks lewat migration terpisah.
- **Tes:** `usecase/pembersihan_test.go` (orkestrasi dengan repositori palsu), `integration/pembersihan_test.go` (dockertest: baris mana yang terhapus tepat di batas, token dicabut-tapi-berlaku dipertahankan, lantai retensi, batch, kunci baris), `cmd/worker/register_auth_test.go` (env dan pendaftaran job).

## Logout dan data offline lapangan

Menu "Keluar" (topbar dan menu profil lapangan, lewat `keluarDariSesi()` di `lib/auth/logout-client.ts`) sekarang:

1. Membaca antrean entri harga dari IndexedDB. Bila ada entri yang belum sampai di server (antre, gagal, konflik, draf), dialog (`components/auth/dialog-keluar.tsx`) menyebut jumlahnya dan menawarkan **Kirim dulu** (sinkronkan sekarang bila online; keluar otomatis bila semuanya terkirim, bila tidak dialog tetap terbuka dengan jumlah sisa), **Tetap keluar**, dan **Batal**. Tanpa entri tertahan, keluar langsung.
2. Memanggil BFF logout (dibatasi 8 detik; gagal atau offline tidak menahan pengguna), lalu membersihkan data perangkat (`lib/offline/bersihkan.ts`, dipasang di `bersihkan-klien.ts`), lalu memuat ulang `/masuk`.

Yang **dihapus**: seluruh isi IndexedDB aplikasi (antrean, cache data acuan, penanda pemilik) dan isi Cache Storage `garda-*` yang bukan aset statis (HTML navigasi `/lapangan/*`, `/masuk`: dirender server dengan nama dan wilayah pengguna, jadi pengguna berikutnya yang membuka aplikasi offline akan melihat halaman pengguna sebelumnya). Penulisan ke IndexedDB ditutup (`penutup.ts`) dan antrean di memori dikosongkan sebelum penghapusan, agar sinkronisasi yang masih berjalan tidak menghidupkan lagi datanya.
Yang **dipertahankan**: `/_next/static/*`, `/icons/*`, berkas statis (woff2/png/svg/css/js), `/offline`, `/manifest.webmanifest` (sama untuk semua pengguna, dan `/offline` dibutuhkan service worker saat peramban luring; layout akar menyisipkan sesi ke semua HTML, jadi `public/sw.js` kini mengambil halaman publik itu dengan `credentials: "omit"` agar salinannya anonim, dan tidak menyimpan hasil navigasi ke `/offline`), serta cache pihak ketiga (mis. tile Mapbox). Service worker tetap terdaftar. Cache TanStack Query dan state in-memory sudah bersih karena pemuatan ulang penuh.

**Sesi berakhir tanpa Keluar** (token habis, pengguna lain masuk di perangkat yang sama): IndexedDB mencatat id pemilik data (`garda-pemilik-offline`). `PenjagaDataOffline` (dirender di `app/providers.tsx`, semua halaman berpengguna) membandingkannya dengan pengguna sesi saat halaman dimuat; bila berbeda, data pengguna sebelumnya dihapus dan toast menyebut jumlah entri yang dibuang. `FieldShell` menunggu hasil yang sama **sebelum** memulihkan antrean dan mengunduh data acuan, supaya antrean milik A tidak terkirim atas nama B. Pengguna yang sama masuk lagi tetap memegang antreannya; data lama tanpa penanda diadopsi oleh pengguna pertama yang memuat aplikasi sesudah fitur ini. Logika murni dan teruji (`lib/offline/pemilik.ts`, `bersihkan.ts`, `lib/auth/konfirmasi-keluar.ts`, `lib/auth/logout.ts`); e2e: `apps/web/e2e/logout-offline.spec.ts` (butuh build produksi karena service worker hanya terdaftar di produksi).

Keterbatasan: keluar saat **offline** tidak bisa mencabut sesi di server (BFF tak terjangkau); data perangkat tetap dibersihkan dan token server kedaluwarsa sendiri. Dua tab yang terbuka sekaligus dengan pengguna berbeda tidak disinkronkan lewat BroadcastChannel; tab lama yang masih memegang antrean di memori dapat menulisnya kembali sampai dimuat ulang.
