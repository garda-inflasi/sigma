# 01 — Autentikasi, MFA, dan sesi

**Prioritas:** P0 · **Status:** UI ✅ · Backend ✅ · Terhubung ✅ · Skema & seed ✅ · Tes ✅
**Cara kerja & DoD:** [00-cara-kerja.md](00-cara-kerja.md) · **Konteks data:** [19](19-migration-seeder.md)

## Yang sudah ada

- **UI:** [ganti-password](<../../apps/web/src/app/(auth)/ganti-password/ganti-password-form.tsx>) (wajib saat `wajib_ganti_password`), [masuk](<../../apps/web/src/app/(auth)/masuk/masuk-form.tsx>) (email + kata sandi, ditambah "Masuk dengan Google" bila SSO dikonfigurasi: lihat 01-G), [lupa-password](<../../apps/web/src/app/(auth)/lupa-password/lupa-password-form.tsx>), halaman [akses-ditolak](../../apps/web/src/app/akses-ditolak/page.tsx).
- **BFF Next.js** (`app/api/auth/*`): login, logout, refresh, me, change-password, forgot-password, reset-password. Cookie `garda_access`/`garda_refresh`, CSRF, rate limit, redirect aman di `lib/bff/*`.
- **Middleware edge** ([middleware.ts](../../apps/web/src/middleware.ts)): proteksi rute, refresh token otomatis, header keamanan, RBAC tingkat rute.
- **API Go:** `POST /v1/auth/{login,refresh,forgot-password,reset-password,logout,change-password}`, `GET /v1/auth/me`, MFA `/v1/auth/mfa/{setup,verify,validate,disable}`.
- **Skema:** entity auth cocok dengan migrasi (dicek skrip + integration test lolos). Tabel: `pengguna` (+ `mfa_secret`, `mfa_backup_codes`, `mfa_wajib` dari 000004), `refresh_tokens`, `login_attempts`, `password_reset_tokens`, `jwt_blacklist`, `password_history` (dibuang di 01-A karena tidak dirujuk kode).
- **Tes:** [integration_test.go](../../apps/api/internal/auth/integration/integration_test.go) dan [usecase_test.go](../../apps/api/internal/auth/usecase/usecase_test.go) (coverage 96,9% sejak penutupan backlog; lihat "Penutupan backlog auth").

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

## 01-F — Lockout akun satu mekanisme dan pesan galat login konsisten _(backlog lanjutan)_

**Tujuan:** satu cara menghitung percobaan gagal yang tidak membocorkan email terdaftar, admin bisa membuka kunci, dan setiap kegagalan login (Go → BFF → form) memakai kode mesin dan pesan Indonesia yang sama. Migration `000105_01f_lockout_akun.sql` (tabel `kunci_login`).

1. **Desain lockout** ([lockout.go](../../apps/api/internal/auth/usecase/lockout.go), [kunci.go](../../apps/api/internal/auth/domain/kunci.go), [repo_lockout.go](../../apps/api/internal/auth/repository/postgres/repo_lockout.go))
   - **Dihitung per email ternormalisasi** (`lower(trim)`), bukan per `user_id`: kuncinya SHA-256 email di `kunci_login (email_hash PK, gagal, jendela_mulai, terkunci_sampai, diperbarui_pada)`. Email tak terdaftar hanya tersimpan sebagai hash. `login_attempts` tetap catatan peristiwa (IP, alasan); `CountRecentFailedAttempts` dan `accountLocked` dihapus supaya mekanismenya satu.
   - **Aturan:** `MaxFailedAttempts` (5) kegagalan dalam jendela `LockoutDuration` (15 menit, dihitung sejak kegagalan pertama) memasang `terkunci_sampai = waktu kegagalan ke-5 + 15 menit`. Selama terkunci, semua percobaan (juga dengan kata sandi benar) ditolak **tanpa diperiksa dan tanpa mengubah apa pun**, jadi percobaan baru tidak pernah memperpanjang kunci. Sesudah kunci berakhir atau jendela habis, hitungan mulai dari nol. Nilai konfigurasi 0/negatif jatuh ke bawaan (lockout tidak bisa mati tanpa sengaja).
   - **Atomik:** setiap percobaan "dipesan" sebelum rahasianya diperiksa (`SELECT … FOR UPDATE` pada baris email), jadi ledakan permintaan serentak tidak bisa melewati batas (teruji: 24 tebakan serentak, tepat 5 diperiksa). Kata sandi benar yang masih menunggu OTP, akun nonaktif dengan kata sandi benar, dan galat infrastruktur mengembalikan percobaannya (bukan tebakan salah).
   - **Reset hanya oleh:** login lengkap (`completeLogin`: kata sandi benar, dan OTP lolos bila ada) dan buka-kunci admin. Kata sandi benar yang menunggu OTP tidak menghapus hitungan (kalau iya, kata sandi bocor memberi tebakan OTP tanpa batas). Atur ulang kata sandi lewat email dan reset oleh admin **tidak** membuka kunci (admin punya aksi buka-kunci).
   - **Satu kunci untuk:** login (kata sandi), validasi OTP login, kode pertama setup MFA, kata sandi lama pada ganti kata sandi, dan konfirmasi kata sandi di nonaktifkan-MFA / kode-cadangan-baru (semula tidak dihitung; ditutup sesudah tinjauan, lihat "Tindak lanjut tinjauan").
   - **Anti-enumerasi:** kunci diperiksa sebelum akun dicari; email tak terdaftar, akun tanpa kata sandi (SSO), dan akun terdaftar mendapat urutan jawaban identik (401 ×5, lalu 423 dengan akhir kunci yang sama), dan email tak terdaftar tetap membandingkan bcrypt dengan hash tak bertuan (`hashPalsu`) supaya waktu jawab sama. `ACCOUNT_INACTIVE` (403) sebelumnya dijawab **sebelum** kata sandi diperiksa (membocorkan akun nonaktif); kini hanya setelah kata sandi benar, kata sandi salah tetap 401 dan dihitung. Galat basis data saat mencari akun atau memesan percobaan ditolak (500) dan tidak dihitung.
   - **Kompromi yang disadari:** pihak ketiga yang tahu sebuah email bisa mengunci akun itu 15 menit per 5 tebakan (standar lockout per akun, sama dengan PRD §6.1 #5); penawarnya kunci otomatis berakhir, buka-kunci admin, dan batas laju per IP (BFF 5/menit; API Go 10/menit untuk endpoint auth publik, `AUTH_RATE_LIMIT_PER_MENIT`, ditambah batas global 600/menit). Hash email tanpa garam (bukan rahasia: email ada polos di `pengguna`).
   - **Pembersihan:** baris yang tak diperbarui lebih dari 2× durasi dan tidak sedang terkunci dipangkas oleh langkah `kunci_login` job `auth_pembersihan` (lihat "Pembersihan tabel auth"); tidak ada pemangkasan acak saat login.
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
| Audit | `LOGIN` berhasil memuat `metadata.metode = sso_google` (lewat konteks ke `completeLogin`); `SSO_LINK` saat penautan; `SSO_LOGIN` (menunggu_mfa) bagi akun ber-MFA, lalu `LOGIN` sesudah OTP memuat `metode` dan `mfa` (metode dibawa klaim token tantangan); kegagalan memuat `alasan` | Jejak lengkap tanpa menyimpan rahasia |
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
- [x] Mekanisme lockout satu saja: hitung dari `login_attempts` (sudah ber-index) atau kolom `terkunci_sampai`; buka-kunci oleh admin. _(selesai: lihat 01-F; dihitung per email di tabel `kunci_login`)_
- [x] Daftar dan pencabutan sesi aktif (`refresh_tokens`: `last_used_at`, `user_agent`; index `(user_id, revoked_at)`): lihat [01-E](#01-e--sesi-aktif-rotasi-refresh-token-dan-pencabutan-access-token).
- [x] Rotasi refresh token dengan deteksi pemakaian ulang (verifikasi sudah ada): atomik, dengan jendela toleransi tab serentak; lihat [01-E](#01-e--sesi-aktif-rotasi-refresh-token-dan-pencabutan-access-token).
- [x] Job pembersihan baris kedaluwarsa `jwt_blacklist`, `refresh_tokens`, `password_reset_tokens` (+ `login_attempts`). Lihat "Pembersihan tabel auth" di bawah.
- [x] Pesan error login konsisten (akun terkunci, password kedaluwarsa, 429). _(selesai: lihat 01-F)_
- [x] Logout membersihkan store offline lapangan (IndexedDB). Lihat "Logout dan data offline lapangan" di bawah. Cache TanStack Query sudah bersih karena logout memuat ulang halaman penuh (lihat 01-D).
- [x] Coverage usecase auth dari 63% ke ≥ 80% (MFA, reset, lockout). _(selesai: 96,9%; lihat "Penutupan backlog auth")_
- [x] Tes `lib/bff/*` (cookies, csrf, rate-limit, safe-redirect), tes middleware. (E2E halaman auth sudah ada: 01-D.) _(selesai: tes web `lib/bff/*` dan `middleware.ts`, tes Go `pkg/middleware`; lihat "Penutupan backlog auth")_
- [x] Satukan tiga salinan OpenAPI (lihat [18 §B](18-platform-lintas-fitur.md)).

---

## Penutupan backlog auth (coverage, tes middleware, audit akhir)

### Coverage usecase auth

`cd apps/api && go test ./internal/auth/usecase/ -coverprofile=/tmp/c.out && go tool cover -func=/tmp/c.out`

| Berkas | Sebelum (setelah merge 01-E/F/G) | Sesudah |
|---|---|---|
| `lockout.go` | 95,9% | 100,0% |
| `mfa.go` | 90,3% | 96,3% |
| `pembersihan.go` | 98,7% | 98,7% |
| `sesi.go` | 85,7% | 96,4% |
| `sso.go` | 94,3% | 96,5% |
| `usecase.go` | 74,9% | 96,4% |
| **Total paket** | **86,2%** (dari 63% sebelum 01-E) | **96,9%** |

Tes baru memeriksa akibat, bukan sekadar bahwa fungsinya terpanggil: [admin_test.go](../../apps/api/internal/auth/usecase/admin_test.go) (ubah/hapus/reset/aksi massal oleh admin, katalog peran, login yang tidak menimpa perubahan admin, jalur galat reset/ganti kata sandi, `ForgotPassword` yang tidak menunggu SMTP) dan [galat_infrastruktur_test.go](../../apps/api/internal/auth/usecase/galat_infrastruktur_test.go) (kegagalan basis data/kunci pada MFA, pergantian sesi, refresh, logout, lockout, SSO: tidak dihitung sebagai tebakan salah, tidak dijawab "token tidak sah", gagal-tutup, tidak jatuh ke jalur yang lebih lemah).

### Tes middleware dan `lib/bff/*`

- **Web** (`pnpm test`, node:test): `cookies.test.ts` (atribut tiap cookie: HttpOnly/SameSite/Path/umur/Secure, cookie penghapus memakai path yang sama), `csrf.test.ts` (double-submit, Origin/Referer, daftar izin, host penyerupa), `handler.test.ts` (urutan CSRF → sesi, pemetaan galat tanpa membocorkan pesan internal), `safe-redirect.test.ts`, `session-response.test.ts`, `security-headers.test.ts`, tambahan di `cookie-policy.test.ts` dan `rate-limit.test.ts`, serta `src/middleware.test.ts` (terjemahan keputusan edge menjadi respons: Location, cookie dihapus, JSON galat, header keamanan, `config.matcher`). `lib/bff/request-scope-testing.ts` memasang request scope Next agar `cookies()` dapat dipakai di tes.
- **Go** (`pkg/middleware`, 94,4%): `auth_sumber_token_test.go` menambah sumber token (header vs cookie BFF, tanpa jatuh ke cookie bila bearer rusak), pencabutan diperiksa sebelum kewajiban ganti kata sandi/MFA, rute sesi baru tertutup bagi sesi setengah-jadi, pengecualian dicocokkan dengan pola rute (bukan URL mentah), serta batas laju serentak dan IP klien di belakang proxy (X-Forwarded-For palsu dari klien langsung tidak membuka ember baru).

### Cacat yang ditemukan audit akhir dan sudah diperbaiki

| # | Tingkat | Lokasi | Cacat | Perbaikan | Tes regresi |
|---|---|---|---|---|---|
| 1 | Sedang | `apps/web/src/lib/bff/safe-redirect.ts` `validateRedirectUrl` | Kondisi `includes("//") && !startsWith("//")` justru **meloloskan** `//evil.com` (juga `/\evil.com`, `/<TAB>/evil.com`). `/mfa?lanjut=//evil.com` dan `/ganti-password?lanjut=//evil.com` mengalihkan pengguna keluar situs lewat `window.location.assign` begitu formulir selesai. Rute SSO tidak terdampak karena menyaringnya lagi lewat `safeInternalPath`. | Hanya path dengan satu garis miring depan tanpa `//`, `\`, atau karakter kontrol yang lolos. | `safe-redirect.test.ts` (32 kasus; 12 gagal pada kode lama) |
| 2 | Sedang | `usecase.go` `completeLogin` | Login menyimpan ulang **seluruh baris** pengguna (`UpdateUser` = GORM `Save`) dari salinan yang dibaca sebelum bcrypt. Penonaktifan, reset kata sandi (hash lama kembali), atau ganti peran oleh admin yang terjadi selama login berjalan tertimpa nilai lama; penyerang yang terus-menerus login dapat membuat penonaktifan akun tidak pernah berlaku. | `domain.Repository.TouchLastLogin` menulis hanya `terakhir_masuk`. | `TestLogin_PerubahanAdminSelamaLoginTidakTertimpa`, `TestLogin_MencatatTerakhirMasukTanpaMenyentuhKolomLain` (integrasi) |
| 3 | Sedang | `usecase.go` `UpdateUser` | `PATCH /v1/users/:id` dengan `peran` baru tidak mencabut sesi (hanya `PUT /roles` yang mencabut, padahal 01-E menyatakan "ganti peran: semua sesi dicabut"): access token lama membawa izin peran sebelumnya sampai 15 menit. | Sesi dicabut bila peran berubah atau status bukan AKTIF. | `TestUpdateUser_*`, `TestAdmin_PatchGantiPeranMencabutSesiSeketika` |
| 4 | Sedang | `usecase.go` `ForgotPassword` | Email dikirim sinkron lewat `smtp.SendMail` (tanpa batas waktu). Hanya email terdaftar yang sampai ke langkah itu, jadi lama jawaban menandai email terdaftar (enumerasi) dan SMTP yang macet menggantung permintaan. | Pengiriman di latar belakang (goroutine); kegagalan hanya dilog. | `TestForgotPassword_PengirimanEmailTidakMenahanPermintaan` (menggantung pada kode lama) |
| 5 | Rendah | `usecase.go` `DeleteUser` | Sesi dicabut **sebelum** penghapusan; bila penghapusan ditolak FK ("masih memiliki data terkait") pengguna yang tetap ada ikut dikeluarkan dari semua perangkat. | Pencabutan dipindah ke sesudah penghapusan (kaskade `refresh_tokens` sudah mematikan sesi). | `TestDeleteUser_PenghapusanDitolakTidakMencabutSesi` |
| 6 | Rendah | `delivery/http/handler.go` `CreateUser`, `BulkUser` | Kata sandi lemah saat membuat pengguna dijawab **500**; galat basis data pada aksi massal dijawab **400** berisi teks galat mentah (`bulk user action: …`). | `errors.Is(ErrWeakPassword)` → 400; hanya `ErrInvalidAction` → 400, selain itu 500 generik. | `handler_test.go` (2 gagal pada kode lama), `TestAdmin_BuatPenggunaKataSandiLemahDijawab400` |
| 7 | Rendah | `apps/web/src/lib/bff/sso-api.ts` | Rute SSO hanya meneruskan `X-Forwarded-For`; sesi hasil masuk Google tercatat dengan user agent `node` sehingga `/keamanan` menampilkan perangkat yang salah. | `User-Agent` peramban ikut diteruskan di `authorize` dan `callback`. | `sso-api.test.ts`, `route.test.ts` rute SSO |

Tidak ada migration yang berubah; tidak ada perubahan kontrak API (`make openapi/check` tidak perlu dijalankan ulang).

### Kompromi yang dibiarkan

- **Penguncian akun juga menolak masuk SSO** (disengaja, 01-G): pihak ketiga yang menebak kata sandi sebuah email dapat menutup jalur SSO akun itu selama kunci berlaku.

### Tindak lanjut tinjauan: lima celah yang ditutup

Lima catatan "kompromi" di tinjauan sebelumnya ditutup (semuanya selesai):

| # | Celah | Perbaikan | Tes regresi |
|---|---|---|---|
| 1 | **Konfirmasi kata sandi tidak dihitung lockout.** `confirmPassword` (nonaktifkan MFA, kode cadangan baru) memeriksa bcrypt tanpa `mulaiPercobaan`, jadi access token curian bisa menebak kata sandi tanpa batas. | `periksaKataSandiPemilik` ([lockout.go](../../apps/api/internal/auth/usecase/lockout.go)) dipakai bersama `ChangePassword`, `DisableMFA`, `RegenerateBackupCodes`: terkunci → 423 `ACCOUNT_LOCKED`, salah → terhitung (alasan `wrong_password_confirm`), benar → percobaan dikembalikan, galat infrastruktur → ditolak dan tidak dihitung. Satu kunci dengan login. | `konfirmasi_kata_sandi_test.go` (usecase, gagal pada kode lama), `TestIntegration_Lockout_KonfirmasiKataSandiMFA` |
| 2 | **Login langsung ke Go (`/api/v1/*`) melewati batas laju BFF.** Hanya batas global 600/menit; lockout per email tidak menahan credential stuffing lintas email. | `middleware.AuthRateLimit` per IP klien (`c.ClientIP()`: `X-Forwarded-For` hanya dari `TRUSTED_PROXIES`) pada login, `mfa/validate`, `forgot-password`, `reset-password` (satu ember) dan SSO `authorize`/`callback` (ember sendiri). `AUTH_RATE_LIMIT_PER_MENIT`, bawaan 10, diisi `cmd/api` (`Dependencies.AuthRateLimitPerMenit`; 0 = mati, nilai tes). 429 `RATE_LIMITED` + `Retry-After` + `data.sisa_detik`, kontrak sama dengan BFF sehingga diteruskan tanpa diubah; ditolak sebelum handler, jadi tidak dihitung lockout. `LoginRateLimit()` yang tidak terpasang dihapus. Stack demo menyetel `100000` (`DEMO_AUTH_RATE_LIMIT_PER_MENIT` di Makefile) karena e2e login ke API Go langsung ratusan kali per putaran; `make test/e2e` butuh stack itu (README, komentar Makefile, pesan galat `sesiApi`). | `pkg/middleware/ratelimit_auth_test.go`, `integration/batas_laju_test.go` (per IP, XFF palsu, ember bersama/terpisah, kontrak 429, penolakan tidak dihitung lockout) |
| 3 | **Lost update dari `Save` seluruh baris.** `setPassword` (ganti/reset/reset admin), `SetupMFA`, `UpdateUser`, `AssignRole` menyimpan salinan basi: penonaktifan, ganti peran, atau reset oleh admin yang terjadi di sela-sela tertimpa (akun nonaktif kembali AKTIF). | `UpdateUser` dihapus dari `domain.Repository`; penggantinya menulis hanya kolom jalurnya: `PatchUser` (kolom patch; baris sebelum/sesudah dibaca dalam satu transaksi `FOR UPDATE` sehingga snapshot audit akurat), `SetPassword` (tiga kolom; ganti kata sandi memakai compare-and-swap pada hash yang baru diverifikasi → `ErrPenggunaBerubah`, 409), `SetMFASecret` (hanya bila `mfa_aktif=false`), `ActivateMFA` (hanya bila belum aktif dan rahasia masih yang diverifikasi → `ErrMFASetupBerubah`, 409; aktivasi serentak tidak lagi menerbitkan dua set kode cadangan). Email kembar serentak kini 409 (bukan 500). | `penulisan_kolom_test.go` (gaya `TestLogin_PerubahanAdminSelamaLoginTidakTertimpa`; 13 dari 14 gagal pada perilaku lama), `integration/kolom_pengguna_test.go` (SQL nyata: kolom, CAS, serentak) |
| 4 | **Edge guard mencocokkan awalan tanpa batas segmen.** `/ganti-passwordX` dan `/api/authX` ikut terbuka. | `diBawahPath` ([path-segmen.ts](../../apps/web/src/lib/auth/path-segmen.ts)): path tepat sama atau `path + "/"`, dipakai `isPasswordChangeExempt` dan `isMfaSetupExempt`. | `password-change.test.ts`, `mfa.test.ts`, `edge-guard.test.ts` |
| 5 | **Audit `LOGIN` SSO + OTP tanpa `metode: sso_google`.** | Metode login dibawa klaim `metode_login` pada token tantangan MFA (`token.WithLoginMethod`, `Manager.VerifyMFAChallenge`) dan dipasang kembali ke konteks `ValidateMFALogin`: entri `LOGIN` akhir memuat `mfa` dan `metode`. | `sso_mfa_test.go`, `token_test.go`, `integration/sso_test.go` |

Kontrak OpenAPI disesuaikan (`make openapi/sync` + `make openapi/check`): 429 `RATE_LIMITED` pada login, `mfa/validate`, `forgot-password`, `reset-password`, SSO `authorize`/`callback`; 423 pada `mfa/disable` dan `mfa/backup-codes`; 409 pada `change-password`; dan koreksi dokumen SSO `callback` (`SSO_ACCOUNT_LOCKED` dijawab 423, bukan 429). Tanpa migration.

Yang tidak disentuh: token sesi baru (ganti kata sandi, aktivasi MFA) masih dicetak dari baris yang dibaca di awal aksi; penonaktifan yang jatuh tepat di antara pembacaan dan pencetakan itu bisa menyisakan access token sampai 15 menit (refresh menolak akun nonaktif). Jendelanya milidetik dan bukan lost update, jadi dibiarkan.

---

## Pembersihan tabel auth (job `auth_pembersihan`)

Job worker (`cmd/worker/register_auth.go`, usecase `internal/auth/usecase/pembersihan.go`, SQL `internal/auth/repository/postgres/repo_pembersihan.go`) menghapus baris mati agar tabel auth tidak tumbuh tanpa batas. Tanpa migration.

| Tabel | Dihapus bila | Dipertahankan |
|---|---|---|
| `jwt_blacklist` | `expires_at` sudah lewat | yang masih berlaku (pembacaan blacklist sudah mengabaikan yang kedaluwarsa, jadi perilaku tidak berubah) |
| `refresh_tokens` | `expires_at` sudah lewat | token **dicabut tetapi belum kedaluwarsa**: `Refresh` memakainya untuk mendeteksi pemakaian ulang dan mencabut seluruh keluarga |
| `password_reset_tokens` | kedaluwarsa atau terpakai lebih dari **7 hari** lalu | token hidup; token yang baru mati disimpan 7 hari karena `ResetPassword` membedakan "sudah kedaluwarsa" dari "tidak valid atau sudah dipakai" bagi tautan yang tidak ditemukan, dan barisnya berguna untuk debug keluhan pengguna |
| `undangan_pengguna` | kedaluwarsa, terpakai, atau dicabut lebih dari **7 hari** lalu (`RetensiResetToken`) | undangan hidup; alasan penyimpanan 7 hari sama dengan token reset ([02-D](02-pengguna-rbac.md#02-d--undang-pengguna-lewat-email-impor-dan-ekspor-csv)) |
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
