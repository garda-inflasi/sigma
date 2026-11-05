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

## 01-F — Lockout akun satu mekanisme dan pesan galat login konsisten _(backlog lanjutan)_

**Tujuan:** satu cara menghitung percobaan gagal yang tidak membocorkan email terdaftar, admin bisa membuka kunci, dan setiap kegagalan login (Go → BFF → form) memakai kode mesin dan pesan Indonesia yang sama. Migration `000105_01f_lockout_akun.sql` (tabel `kunci_login`).

1. **Desain lockout** ([lockout.go](../../apps/api/internal/auth/usecase/lockout.go), [kunci.go](../../apps/api/internal/auth/domain/kunci.go), [repo_lockout.go](../../apps/api/internal/auth/repository/postgres/repo_lockout.go))
   - **Dihitung per email ternormalisasi** (`lower(trim)`), bukan per `user_id`: kuncinya SHA-256 email di `kunci_login (email_hash PK, gagal, jendela_mulai, terkunci_sampai, diperbarui_pada)`. Email tak terdaftar hanya tersimpan sebagai hash. `login_attempts` tetap catatan peristiwa (IP, alasan); `CountRecentFailedAttempts` dan `accountLocked` dihapus supaya mekanismenya satu.
   - **Aturan:** `MaxFailedAttempts` (5) kegagalan dalam jendela `LockoutDuration` (15 menit, dihitung sejak kegagalan pertama) memasang `terkunci_sampai = waktu kegagalan ke-5 + 15 menit`. Selama terkunci, semua percobaan (juga dengan kata sandi benar) ditolak **tanpa diperiksa dan tanpa mengubah apa pun**, jadi percobaan baru tidak pernah memperpanjang kunci. Sesudah kunci berakhir atau jendela habis, hitungan mulai dari nol. Nilai konfigurasi 0/negatif jatuh ke bawaan (lockout tidak bisa mati tanpa sengaja).
   - **Atomik:** setiap percobaan "dipesan" sebelum rahasianya diperiksa (`SELECT … FOR UPDATE` pada baris email), jadi ledakan permintaan serentak tidak bisa melewati batas (teruji: 24 tebakan serentak, tepat 5 diperiksa). Kata sandi benar yang masih menunggu OTP, akun nonaktif dengan kata sandi benar, dan galat infrastruktur mengembalikan percobaannya (bukan tebakan salah).
   - **Reset hanya oleh:** login lengkap (`completeLogin`: kata sandi benar, dan OTP lolos bila ada) dan buka-kunci admin. Kata sandi benar yang menunggu OTP tidak menghapus hitungan (kalau iya, kata sandi bocor memberi tebakan OTP tanpa batas). Atur ulang kata sandi lewat email dan reset oleh admin **tidak** membuka kunci (admin punya aksi buka-kunci).
   - **Satu kunci untuk:** login (kata sandi), validasi OTP login, kode pertama setup MFA, dan kata sandi lama pada ganti kata sandi. Konfirmasi kata sandi di nonaktifkan-MFA/kode-cadangan butuh sesi sah dan memang tidak pernah dihitung; tidak diubah.
   - **Anti-enumerasi:** kunci diperiksa sebelum akun dicari; email tak terdaftar, akun tanpa kata sandi (SSO), dan akun terdaftar mendapat urutan jawaban identik (401 ×5, lalu 423 dengan akhir kunci yang sama), dan email tak terdaftar tetap membandingkan bcrypt dengan hash tak bertuan (`hashPalsu`) supaya waktu jawab sama. `ACCOUNT_INACTIVE` (403) sebelumnya dijawab **sebelum** kata sandi diperiksa (membocorkan akun nonaktif); kini hanya setelah kata sandi benar, kata sandi salah tetap 401 dan dihitung. Galat basis data saat mencari akun atau memesan percobaan ditolak (500) dan tidak dihitung.
   - **Kompromi yang disadari:** pihak ketiga yang tahu sebuah email bisa mengunci akun itu 15 menit per 5 tebakan (standar lockout per akun, sama dengan PRD §6.1 #5); penawarnya kunci otomatis berakhir, buka-kunci admin, dan batas laju per IP (BFF 5/menit, API 600/menit). Hash email tanpa garam (bukan rahasia: email ada polos di `pengguna`).
   - **Pembersihan:** `UseCase.BersihkanKunciKedaluwarsa` memangkas baris yang tak diperbarui lebih dari 2× durasi; dipanggil acak (1%) saat login dan bisa dipasang ke job pembersihan.
2. **Admin:** `POST /v1/users/:id/buka-kunci` (`pengguna.kelola`, audit `UNLOCK_USER` dengan `terkunci_sebelumnya`; idempoten, 404 bila pengguna tak ada). Daftar dan detail pengguna memuat `terkunci_sampai` selama terkunci. `/pengguna` menampilkan lencana "Terkunci · sisa waktu" (berdetak) dan aksi "Buka kunci" di menu baris. OpenAPI disinkronkan.
3. **Kontrak galat (Go → BFF → UI).** Kode di kolom `code` amplop, diteruskan BFF tanpa diubah; satu tabel pesan di [pesan-login.ts](../../apps/web/src/lib/auth/pesan-login.ts).

   | Kode | HTTP | Pesan (web) | Data |
   |---|---|---|---|
   | `INVALID_CREDENTIALS` | 401 | Email atau password salah. | (sama untuk email tak terdaftar) |
   | `ACCOUNT_LOCKED` | **423** | Akun terkunci karena terlalu banyak percobaan gagal. Coba lagi dalam _N_. | `terkunci_sampai`, `sisa_detik`, header `Retry-After` |
   | `ACCOUNT_INACTIVE` | 403 | Akun Anda tidak aktif. Hubungi administrator. | hanya setelah kata sandi benar |
   | `RATE_LIMITED` (lahir di BFF) | 429 | Terlalu banyak percobaan dari perangkat ini. Coba lagi dalam _N_. | `sisa_detik`, `Retry-After` |
   | `MFA_CODE_INVALID` | 401 (400 di setup) | Kode verifikasi tidak valid. … | |
   | `MFA_CHALLENGE_EXPIRED` | 401 | Sesi verifikasi berakhir. Masuk ulang dengan kata sandi Anda. | form menawarkan kembali ke /masuk |
   | `PASSWORD_CHANGE_REQUIRED` | 403 | Anda harus mengganti kata sandi sebelum melanjutkan. | (sudah ada) |

   423 dipilih untuk kunci akun supaya tidak tertukar dengan 429 batas laju BFF per IP (dua hal berbeda yang dulu sama-sama 429). Kiriman form login dialihkan ke `/masuk?error=…&kode=…&sampai=<epoch detik>`: pesan selalu dari tabel (kode dikenal mengalahkan teks `error`), `sampai` absolut sehingga muat ulang tidak mengulang hitungan. Form masuk, OTP, lupa/atur ulang/ganti kata sandi menampilkan hitung mundur (`role="timer"`, dirender setelah hidrasi) dan menahan tombol kirim selama menunggu.
4. **"Password kedaluwarsa" — keputusan.** Tidak ada kebijakan umur kata sandi yang aktif: PRD §6.1 #11 hanya menyebut `PASSWORD_EXPIRY_DAYS` dengan bawaan 0 (nonaktif), dan tidak ada kolom/aturan di kode. Tidak dibuat kebijakan baru. Kata sandi yang wajib diganti, apa pun sebabnya, lewat alur wajib-ganti yang ada (`wajib_ganti_password` → 403 `PASSWORD_CHANGE_REQUIRED` → `/ganti-password`) dan pesannya ada di tabel di atas. Bila `PASSWORD_EXPIRY_DAYS` kelak diaktifkan, tempatnya di `completeLogin`: nyalakan `wajib_ganti_password` bila `password_diubah_pada` lebih tua dari batas, tanpa kode galat baru.
5. **TES**
   - [x] Go unit: aturan domain (`kunci_test.go`: ambang, akhir kunci, tidak diperpanjang, jendela, kembalikan), usecase (`lockout_test.go`: ambang + akhir kunci, kedaluwarsa, reset setelah berhasil, percobaan ke-5 benar berhasil, email terdaftar vs tak terdaftar identik, nonaktif, SSO, normalisasi email, fail-closed, MFA berbagi kunci, setup MFA, ganti kata sandi, audit, buka-kunci).
   - [x] Go integrasi (dockertest, `integration/lockout_test.go`): status/kode/`Retry-After`, kunci tidak bergeser, serentak, urutan jawaban identik untuk email tak terdaftar, hanya hash tersimpan, nonaktif, MFA, ganti kata sandi, buka-kunci (izin, 404, audit, daftar/detail), pangkas.
   - [x] Web (node:test): `pesan-login.test.ts`, `galat-auth.test.ts`, `rate-limit.test.ts`, `login-route.test.ts` (429 body/header, alihan form), `pengguna.test.ts`.
   - [x] E2E [lockout.spec.ts](../../apps/web/e2e/lockout.spec.ts): kunci lewat API langsung, pesan + hitung mundur di form, kata sandi benar tetap ditolak, buka-kunci dari /pengguna, masuk berhasil, email tak terdaftar identik, pesan 429 di form. Pemakaian anggaran BFF: 2 kiriman form + sampai 5 permintaan langsung (dicatat lewat `anggaranBff.catat`) + 1 kiriman form ditolak 429.
   - Catatan: stack demo harus dibangun ulang (`make docker/demo/up`) agar API Go dan BFF memakai kode baru dan migration 000105.

---

## Backlog lanjutan (belum dijadwalkan)

- [ ] **SSO Google:** tombolnya sudah dihapus dari login (rute `/v1/auth/sso/*` tidak pernah ada, tombol berujung 404). Bila SSO masuk scope (PRD §5.1 menyebut SSO): OIDC + pemetaan akun ke peran; migration `sso_provider`, `sso_subject` dengan UNIQUE gabungan (`password_hash` sudah nullable); lalu pasang kembali tombolnya bersama e2e-nya.
- [x] Mekanisme lockout satu saja: hitung dari `login_attempts` (sudah ber-index) atau kolom `terkunci_sampai`; buka-kunci oleh admin. _(selesai: lihat 01-F; dihitung per email di tabel `kunci_login`)_
- [ ] Daftar dan pencabutan sesi aktif (`refresh_tokens`: `last_used_at`, `user_agent`; index `(user_id, revoked_at)`).
- [ ] Rotasi refresh token dengan deteksi pemakaian ulang (verifikasi sudah ada).
- [ ] Job pembersihan baris kedaluwarsa `jwt_blacklist`, `refresh_tokens`, `password_reset_tokens`.
- [x] Pesan error login konsisten (akun terkunci, password kedaluwarsa, 429). _(selesai: lihat 01-F)_
- [ ] Logout membersihkan store offline lapangan (IndexedDB). Cache TanStack Query sudah bersih karena logout memuat ulang halaman penuh (lihat 01-D).
- [ ] Coverage usecase auth dari 63% ke ≥ 80% (MFA, reset, lockout).
- [ ] Tes `lib/bff/*` (cookies, csrf, rate-limit, safe-redirect), tes middleware. (E2E halaman auth sudah ada: 01-D.)
- [x] Satukan tiga salinan OpenAPI (lihat [18 §B](18-platform-lintas-fitur.md)).
