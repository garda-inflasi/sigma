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

## 01-E — Sesi aktif, rotasi refresh token, dan pencabutan access token

**Tujuan:** sesi dapat dilihat dan dicabut per perangkat, pencabutan berlaku seketika, dan web memperbarui sesi tanpa mengeluarkan pengguna.
**Migration:** `000104_01e_sesi_refresh_token.sql`.

### Dua bug yang ditemukan dan diperbaiki

1. **Access token yang dicabut tidak pernah ditegakkan.** `Logout` menulis `jti` ke `jwt_blacklist`, tetapi tidak ada yang memanggil `IsJWTBlacklisted`; `JWTAuth` hanya memeriksa tanda tangan dan klaim. Token yang sudah logout tetap sah di API Go sampai 15 menit. Kini `JWTAuth` menolak token yang `jti`-nya masuk daftar cabut **atau** sesinya (`sid`) sudah tidak hidup, seketika.
2. **Web tidak pernah memperbarui sesi.** Route BFF `/api/auth/refresh` ada tetapi tidak dipanggil siapa pun: setelah 15 menit navigasi halaman dilempar ke `/masuk` dan panggilan data menerima 401, padahal cookie refresh (7 hari) masih sah. Kini 401 dipulihkan senyap dan navigasi halaman dipulihkan lewat redirect (lihat di bawah).

Temuan terkait yang ikut diperbaiki: rotasi tidak atomik (dua refresh serentak dengan token yang sama sama-sama berhasil) dan tab kedua yang menyegarkan bersamaan terbaca sebagai "pemakaian ulang" sehingga mematikan sesi di semua tab; logout mencabut semua refresh token pengguna (keluar di satu perangkat mengeluarkan semua perangkat); penonaktifan massal tidak mencabut sesi; kegagalan mencabut sesi saat penonaktifan, reset admin, dan ganti peran ditelan tanpa suara; dan aliran SSE EWS hidup terus setelah sesinya dicabut.

### Model

- **Sesi = satu `family_id` pada `refresh_tokens` = klaim `sid` access token.** Satu baris tetap satu token. Kolom baru: `user_agent` (kolom `device` lama tidak dipakai; ringkasan "Chrome di macOS" dihitung saat dibaca), `last_used_at` (login atau rotasi terakhir), `session_started_at` (diwariskan ke token pengganti), `rotated_at` (diisi bersama `revoked_at` hanya bila token dicabut karena **diganti**, bukan karena keluar/dicabut). Index `(user_id, revoked_at)` dan index parsial `idx_rt_family_aktif (family_id) WHERE revoked_at IS NULL`.
- **Penegakan.** `token.Manager.SetRevocationChecker` dipasang oleh `internal/auth/routes.Register` pada `Manager` yang dibagi semua modul, jadi tanda tangan `JWTAuth(tokenMgr)` tidak berubah dan setiap router yang memasang modul auth (cmd/api, harness smoketest, tes integrasi) otomatis menegakkannya; router tanpa modul auth tidak menegakkan apa pun. Satu pernyataan SQL per request: lookup PK `jwt_blacklist` + satu entri indeks parsial. **Tanpa cache**: pencabutan harus seketika di semua replika dan biayanya satu lookup terindeks. Token tanpa `sid` (terbitan sebelum fitur ini, maks. 15 menit) hanya diperiksa lewat `jti`.
- **Gagal-tutup.** Bila status pencabutan tidak dapat diperiksa (galat basis data) request dijawab **503**, bukan diloloskan: fail-open menghidupkan token yang sudah dicabut tepat saat basis data bermasalah, sedangkan hampir semua handler juga butuh basis data sehingga kehilangan ketersediaannya kecil. 503 (bukan 401) agar klien tidak menyangka sesinya berakhir.
- **Rotasi atomik.** `RotateRefreshToken` mencabut token lama dengan `UPDATE … WHERE revoked_at IS NULL AND expires_at > now` lalu menyimpan penggantinya dalam satu transaksi dan memeriksa `RowsAffected`; transaksi kedua menunggu kunci baris, melihat token sudah dicabut, dan tidak menyimpan apa pun.
- **Token yang sudah dirotasi dipakai lagi:**
  - dalam **jendela toleransi** (`Config.RefreshGraceWindow`, bawaan 10 detik; 0 = mati) dan sesinya masih hidup → dianggap tab/permintaan serentak yang sah: hanya **access token baru** yang dikembalikan, tanpa `refresh_token`, karena pemenang rotasi sudah menerbitkan satu-satunya token pengganti dan memasangnya di cookie bersama;
  - lewat jendela itu → pemakaian ulang: seluruh family dicabut (access token-nya ikut mati lewat `sid`) dan `TOKEN_REUSE` dicatat di audit;
  - token yang dicabut tanpa digantikan (keluar, "Cabut", paksa keluar, penonaktifan) hanya dijawab 401, tanpa audit pencurian.

  *Trade-off:* pemegang token lama yang mengulang dalam ≤ 10 detik setelah rotasi mendapat access token (maks. 15 menit) tanpa bisa memutar lagi; menolak keras akan mengeluarkan pengguna dari semua tab setiap kali dua tab menyegarkan bersamaan. Jendela pendek dan hanya untuk sesi yang masih hidup menekan risikonya.
- **Konsistensi pencabutan.** Keluar: hanya sesi ini (`sid`; cadangan: family dari `refresh_token` di badan; tanpa keduanya: semua, seperti dulu). Ganti kata sandi dan aktivasi MFA: sesi ini tetap (family sama, token baru) dan sesi lain dicabut. Reset kata sandi (lupa), reset admin, ganti peran, penonaktifan (satu per satu maupun massal), aksi massal "akhiri sesi", dan penghapusan pengguna: semua sesi dicabut. Refresh juga menolak akun yang sudah tidak aktif.
- **SSE EWS** ditutup saat access token kedaluwarsa (`middleware.TokenLifetimeContext`); `EventSource` menyambung ulang lewat pemeriksaan penuh.

### API

| Endpoint | Keterangan |
|---|---|
| `GET /v1/auth/sessions` | Sesi aktif sendiri: `id`, `perangkat`, `user_agent`, `ip`, `dibuat_pada`, `terakhir_dipakai_pada`, `kedaluwarsa_pada`, `saat_ini`. Semua peran. |
| `DELETE /v1/auth/sessions/{id}` | Cabut satu sesi sendiri (sesi ini ditolak 400; sesi orang lain/tidak ada 404). Audit `REVOKE_SESSION`. |
| `POST /v1/auth/sessions/revoke-others` | Cabut semua sesi lain. Audit `REVOKE_OTHER_SESSIONS`. |
| `POST /v1/auth/logout` | Hanya sesi ini; badan opsional `{refresh_token}`. |
| `POST /v1/auth/logout-all` | Semua perangkat, termasuk ini. Audit `LOGOUT_ALL`. |
| `POST /v1/users/{id}/revoke-sessions` | Paksa keluar (`pengguna.kelola`). Audit `FORCE_LOGOUT` (aktor admin, sasaran pengguna). |
| `POST /v1/auth/refresh` | `refresh_token` pada respons bisa kosong (pemutaran ulang yang ditoleransi). |

Login mencatat IP dan user agent; refresh memperbarui `last_used_at`, IP, dan user agent.

### Web

- **Refresh senyap.** `apiFetch`/`bffFetch` (dan tiga unduhan CSV) memakai `fetchSesi`: 401 karena token (bukan karena kata sandi salah, bukan alur masuk) → `POST /api/auth/refresh` **single-flight**, dikunci antar-tab dengan Web Locks dan catatan di `localStorage` (tab yang menunggu melewati refresh bila tab lain baru berhasil sesudah permintaannya dimulai) → permintaan diulang sekali (header CSRF dihitung ulang). Hanya bila refresh ditolak (401) pengguna diarahkan ke `/masuk?lanjut=<halaman>`; galat sementara (5xx, jaringan) tidak mengeluarkan siapa pun.
- **Navigasi halaman.** Cookie refresh ber-path `/api/auth`, sehingga edge tidak pernah melihatnya. Halaman terlindungi tanpa access token dialihkan ke `GET /api/auth/refresh?lanjut=<path+query>` yang menukar token lalu kembali ke tujuan semula (tujuan divalidasi: hanya path internal) atau ke `/masuk` bila tidak ada/ditolak. Penanda `garda_pulih` (20 detik) memutus loop. Batasan: navigasi yang dimulai dari situs lain (tautan email/chat) tidak membawa cookie `SameSite=Strict`, sehingga tetap berakhir di `/masuk`.
- **Keluar.** `/api/auth/logout` kini publik di edge dan diperiksa `Origin`-nya; bila access token sudah habis ia menukar refresh token dulu supaya sesi di server benar-benar dicabut. Ditambah `logout-all`, `sessions`, `sessions/[id]`, `sessions/revoke-others`, dan `pengguna/[id]/revoke-sessions`.
- **IP dan user agent peramban** diteruskan BFF ke Go (`User-Agent`, `X-Forwarded-For`); Go memercayai `X-Forwarded-For` hanya dari proxy tepercaya (`TRUSTED_PROXIES`, bawaan jaringan privat tempat web berada).
- **SSE EWS** (`lib/api/ews-sambung.ts`): koneksi yang ditolak (CLOSED) memulihkan sesi lalu membuat `EventSource` baru dan menyegarkan daftar alert; berhenti setelah beberapa kali pemulihan yang langsung ditolak lagi.
- **UI.** Halaman `/keamanan` ("Keamanan akun", semua peran; tautan di menu profil konsol dan lapangan): daftar sesi dengan penanda "Perangkat ini", "Cabut" per sesi, "Keluar dari perangkat lain", "Keluar dari semua perangkat". `/pengguna`: aksi baris "Paksa keluar".

### Tes

- Go: `usecase/sesi_test.go` (rotasi, jendela toleransi, reuse, logout, daftar/cabut/paksa keluar, ganti kata sandi), `pkg/middleware/revocation_test.go`, `pkg/token/session_test.go`, `dto/sesi_test.go`, dan integrasi dockertest `auth/integration/sesi_test.go` (penegakan jti/sid di semua modul, gagal-tutup 503, rotasi 16 goroutine → satu pemenang, tanpa toleransi → satu sukses, reuse mencabut family + audit, daftar/cabut/revoke-others/logout/logout-all, paksa keluar dan izinnya, ganti kata sandi, aksi admin). Perintah: `cd apps/api && go test ./internal/auth/... ./pkg/... ./internal/platform/...`.
- Web: `sesi-pulih.test.ts`, `ews-sambung.test.ts`, `sesi.test.ts`, `edge-guard.test.ts`, `refresh-session.test.ts`, `logout-server.test.ts`, `session-cookies.test.ts`, `api-client-sesi.test.ts` (`pnpm test`).
- E2E: [sesi.spec.ts](../../apps/web/e2e/sesi.spec.ts) (pengguna uji sementara; sesi "perangkat lain" dibuat lewat API dengan User-Agent berbeda). Stack Docker harus dibangun ulang agar memuat migrasi 000104 dan build web baru.

---

## Backlog lanjutan (belum dijadwalkan)

- [ ] **SSO Google:** tombolnya sudah dihapus dari login (rute `/v1/auth/sso/*` tidak pernah ada, tombol berujung 404). Bila SSO masuk scope (PRD §5.1 menyebut SSO): OIDC + pemetaan akun ke peran; migration `sso_provider`, `sso_subject` dengan UNIQUE gabungan (`password_hash` sudah nullable); lalu pasang kembali tombolnya bersama e2e-nya.
- [ ] Mekanisme lockout satu saja: hitung dari `login_attempts` (sudah ber-index) atau kolom `terkunci_sampai`; buka-kunci oleh admin.
- [x] Daftar dan pencabutan sesi aktif (`refresh_tokens`: `last_used_at`, `user_agent`; index `(user_id, revoked_at)`): lihat [01-E](#01-e--sesi-aktif-rotasi-refresh-token-dan-pencabutan-access-token).
- [x] Rotasi refresh token dengan deteksi pemakaian ulang (verifikasi sudah ada): atomik, dengan jendela toleransi tab serentak; lihat [01-E](#01-e--sesi-aktif-rotasi-refresh-token-dan-pencabutan-access-token).
- [ ] Job pembersihan baris kedaluwarsa `jwt_blacklist`, `refresh_tokens`, `password_reset_tokens`.
- [ ] Pesan error login konsisten (akun terkunci, password kedaluwarsa, 429).
- [ ] Logout membersihkan store offline lapangan (IndexedDB). Cache TanStack Query sudah bersih karena logout memuat ulang halaman penuh (lihat 01-D).
- [ ] Coverage usecase auth dari 63% ke ≥ 80% (MFA, reset, lockout).
- [ ] Tes `lib/bff/*` (cookies, csrf, rate-limit, safe-redirect), tes middleware. (E2E halaman auth sudah ada: 01-D.)
- [x] Satukan tiga salinan OpenAPI (lihat [18 §B](18-platform-lintas-fitur.md)).
