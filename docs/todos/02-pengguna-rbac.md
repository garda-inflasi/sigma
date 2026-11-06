# 02 — Pengguna, peran, dan RBAC

**Prioritas:** P0 · **Status:** UI ✅ · Backend ✅ · Terhubung ✅ · Skema & seed ✅ · Tes 🟡
**Cara kerja & DoD:** [00-cara-kerja.md](00-cara-kerja.md) · **Konteks data:** [19 §1.4](19-migration-seeder.md)

## Yang sudah ada

- **UI:** [pengguna-page.tsx](../../apps/web/src/components/pengguna/pengguna-page.tsx) (daftar akun, aksi massal, matriks peran × izin) di `/pengguna`.
- **BFF:** `app/api/pengguna` (list, create, `[id]`, `[id]/roles`, `[id]/reset-password`, `bulk`) dan `app/api/rbac/{roles,permissions}`.
- **API Go:** `POST/GET /v1/users`, `GET/PATCH/DELETE /v1/users/:id`, `POST /v1/users/bulk`, `POST /v1/users/:id/reset-password`, `PUT /v1/users/:id/roles`, `GET /v1/roles`, `GET /v1/permissions` (semua butuh `pengguna.kelola`).
- **Aturan di web:** 14 peran dan 35 izin di [roles.ts](../../apps/web/src/lib/rbac/roles.ts) / [permissions.ts](../../apps/web/src/lib/rbac/permissions.ts), dipakai sidebar, palet perintah, middleware, aksi massal.
- **Temuan penting (terverifikasi dari kode):**
  - Izin yang berlaku di backend berasal dari peta `rolePermissions` di [auth/usecase/usecase.go](../../apps/api/internal/auth/usecase/usecase.go), dikunci pada enum `peran` (14 nilai) dengan 35 izin `domain.aksi`.
  - Tabel `roles` (15 baris), `permissions` (29 baris format `harga:read`), `role_permissions`, `user_permissions` diisi migrasi 000001/000006 tetapi **tidak dibaca kode apa pun**. Nama izinnya tidak ada yang sama dengan yang dipakai kode; `roles` berisi "Superadmin" yang tidak ada di enum `peran`.
  - Route yang meminta `lapangan.lihat`, `lapangan.baca`, `lapangan.validasi`, `prediksi.baca` **tidak dimiliki peran manapun**, jadi selalu 403 (termasuk semua `/v1/lapangan/*`).
- **Detail pengguna (✅, terhubung):** dialog "Ubah pengguna" memuat `GET /v1/users/:id` lewat BFF (`penggunaApi.get`, hook `usePenggunaDetail`, tanpa cache agar formulir selalu segar) sebelum formulir tampil, dan menampilkan rincian akun (ID, cakupan, status, MFA, terakhir masuk, dibuat). Bila gagal dimuat, formulir memakai baris daftar. Perbaikan backend: `GetUser` tidak lagi menjawab 404 untuk galat basis data (hanya "tidak ada" yang 404, selain itu 500); tes usecase dan integrasi `get_user_test.go` (200 tanpa rahasia, 404, 403 tanpa `pengguna.kelola`, 401).
- **Undang, impor, dan ekspor (02-D, ✅):** `POST /v1/users/undang`, `POST /v1/users/:id/undang-ulang`, publik `POST /v1/auth/aktivasi` (halaman `/aktivasi`), `POST /v1/users/impor`, `GET /v1/users/ekspor`; rancangannya di [02-D](#02-d--undang-pengguna-lewat-email-impor-dan-ekspor-csv).

---

## 02-A — Satu sumber izin + perbaiki route yang selalu 403 _(Fase P, urutan #2)_

**Tujuan:** tidak ada izin yatim, tidak ada route yang mustahil diakses, dan tabel RBAC tidak lagi menyesatkan.
**Bergantung pada:** 01-A. **Sentuh juga:** F1 menghapus penugasan izin dari 000006.

**Keputusan D3 (default yang dipakai di slice ini: Opsi A).**

- **Opsi A (default):** peta di kode adalah sumber tunggal; tabel RBAC dibuang. Termurah dan sesuai kondisi nyata.
- **Opsi B:** tabel DB menjadi sumber. Pilih ini hanya bila "izin langsung per pengguna" dan matriks yang bisa diedit di UI memang dibutuhkan; langkahnya ada di 02-C.

1. **MIG** _(Opsi A)_
   - [x] Migration yang `DROP` `user_permissions`, `role_permissions`, `permissions`, `roles`. `Down` membuat ulang tabel kosong. Ini tidak menyentuh enum `peran`.
2. **SEED**
   - [x] Tidak ada seed RBAC (peta di kode). Catat di F1 bahwa `INSERT` roles/permissions (000001) dan `role_permissions` + `DELETE FROM role_permissions` (000006) dihapus.
3. **API**
   - [x] Perbaiki nama izin pada route yang yatim, mis. `lapangan.lihat`/`lapangan.baca` → izin yang memang dimiliki peran lapangan (`tugas.lihat_sendiri`, `dashboard.operasional`), `lapangan.validasi` → `harga.validasi`, `prediksi.baca` → `prediksi.lihat`. Tentukan bersama D4 (modul `analytics`).
   - [x] Satu berkas kontrak `packages/contracts/permissions.json`, dihasilkan dari peta Go (`go test ./internal/auth/usecase -run TestPermissionsContractFile -update-permissions`). Konstanta web tidak dihasilkan, tetapi dijaga sama oleh tes `permissions.test.ts`.
   - [x] **Tes konsistensi:** setiap izin dalam `RequirePermission(...)` ada di daftar; setiap izin di daftar dimiliki ≥ 1 peran; daftar web sama dengan daftar Go.
4. **FE**
   - [x] Matriks akses di `/pengguna` memakai `GET /v1/roles` dan `GET /v1/permissions` (BFF sudah ada); pastikan sama dengan `lib/rbac`.
5. **TES**
   - [x] Integrasi: tiap peran → rute yang boleh/tidak boleh, dibandingkan dengan matriks di [ARCHITECTURE.md](../ARCHITECTURE.md).
6. **DoD** — checklist [00 §4](00-cara-kerja.md). Khusus: `GET /v1/lapangan/tugas/saya` tidak lagi 403 untuk peran lapangan.

---

## 02-B — Pemisahan tugas dan cakupan baris _(Fase F)_

**Tujuan:** aturan RBAC yang dijanjikan dokumen ditegakkan di backend, bukan hanya UI.

- [ ] **Row scope:** Koordinator dan Agregator hanya kabupatennya, Mahasiswa hanya entrinya sendiri. Terapkan di `harga`, `lapangan`, `dashboard`, `ews` (kolom `pengguna.kabupaten_kode` sudah ada). Dikerjakan bersama slice tiap modul.
- [ ] **Pemisahan tugas:** validator tidak boleh memvalidasi entrinya sendiri; penginput ≠ validator ≠ publisher; Admin Sistem tanpa hak persetujuan bisnis (kaitan: CHECK di 06-B).
- [ ] **Persetujuan berjenjang** intervensi ditegakkan di `UpdateTahap` (kaitan: 14-B).
- [ ] Tes integrasi RBAC per peran untuk tiap aturan di atas.

---

## 02-C — Izin langsung per pengguna _(opsional; hanya bila Opsi B dipilih)_

1. **MIG**
   - [ ] Buat ulang `roles` (kunci = nilai enum `peran`, 14 baris), `permissions` (35 izin `domain.aksi`, `deskripsi`, `kelompok`), `role_permissions`, `user_permissions` (+ `berlaku_sampai`, `alasan`, `diberikan_oleh`, `diberikan_pada`).
2. **SEED** (lapis `reference`)
   - [ ] 14 peran, 35 izin, dan `role_permissions` sesuai matriks; `ON CONFLICT DO NOTHING` agar tidak menimpa perubahan admin. **Jangan** ada `DELETE`.
3. **API**
   - [ ] Token membawa gabungan izin peran + izin langsung, dimuat dari DB dengan cache; UI matriks bisa mengubah.
4. **FE**: UI penugasan izin langsung dengan masa berlaku dan alasan.
5. **TES**: izin langsung berlaku dan kedaluwarsa; perubahan tercatat di audit.

---

## 02-D — Undang pengguna lewat email, impor dan ekspor CSV _(backlog lanjutan, selesai)_

**Tujuan:** admin tidak lagi mengarang dan menyampaikan kata sandi sementara; pemilik email menetapkan kata sandi pertamanya sendiri. Impor massal dari CSV dan ekspor yang tercatat di audit menggantikan ekspor sisi klien yang tak teraudit. Migration `000108_02d_undangan_pengguna.sql`.

### Model undangan

- **Tabel sendiri `undangan_pengguna`, bukan `password_reset_tokens` + kolom jenis.** Siklus hidupnya berbeda (72 jam vs 30 menit; dicabut saat kirim ulang; hanya untuk akun `MENUNGGU`), dan `ResetPassword`/`ConsumeResetToken` membaca tabel reset tanpa penyaring jenis: satu filter yang terlewat membuat token undangan bisa dipakai di jalur atur ulang. Dipisah, jalur atur ulang tidak mungkin mengaktifkan akun (diuji di integrasi).
- Kolom: `token_hash` (SHA-256 dari token acak 256-bit; token mentah hanya ada di email), `email` (alamat tujuan saat undangan dikirim), `diundang_oleh` (FK `ON DELETE SET NULL`, tidak menghalangi penghapusan admin), `expires_at`, `used_at`, `dicabut_pada`. Indeks unik parsial `uq_undangan_pengguna_aktif`: paling banyak satu undangan hidup per akun.
- **TTL 72 jam** (`usecase.Config.UndanganTTL`, bawaan `UndanganTTLBawaan`); email menyebut masa berlakunya.
- **Aktivasi atomik** (`repo.Aktifkan`, satu transaksi): `UPDATE undangan … SET used_at WHERE token_hash AND belum dipakai/dicabut/kedaluwarsa AND akun MENUNGGU AND lower(email akun) = lower(undangan.email) RETURNING`, lalu akun diisi kata sandi, `status = AKTIF`, `wajib_ganti_password = false`. Permintaan serentak: tepat satu berhasil (teruji unit dan integrasi). Karena syarat "email masih sama" ada di pernyataan itu, **mengubah email akun MENUNGGU otomatis menghanguskan tautan yang terlanjur dikirim ke alamat lama** (pemilik alamat lama tidak bisa mengambil alih akun); admin mengirim ulang ke alamat baru.
- **Urutan pemeriksaan:** token (ada, belum dipakai/dicabut, belum kedaluwarsa) → kebijakan kata sandi (yang sama dengan atur ulang, plus maksimal 72 karakter karena batas bcrypt) → transaksi aktivasi. Kata sandi lemah **tidak** menghanguskan tautan.
- **Tanpa auto-login:** aktivasi hanya menetapkan kata sandi; halaman menautkan ke `/masuk`. Alasannya: lockout, MFA (peran wajib MFA), dan penerbitan sesi tinggal di satu tempat (`Login`), dan pengguna langsung membuktikan kata sandinya bekerja.

### Status `MENUNGGU` dan anti-enumerasi

- Akun `MENUNGGU` tidak punya kata sandi. Di `Login` ia lewat cabang "tanpa kata sandi" yang sudah ada: bcrypt palsu, `401 INVALID_CREDENTIALS`, dan hitungan lockout per email yang **sama persis** dengan email tak terdaftar (teruji: urutan jawaban 401 ×5 lalu 423 identik, dan badan jawaban identik). `ForgotPassword` tidak mengirim apa pun untuknya (status bukan `AKTIF`). Tidak ada jawaban publik yang membedakan akun menunggu dari yang tidak ada. Membedakan "kedaluwarsa" dari "tidak valid" di `/v1/auth/aktivasi` hanya memberi tahu pemegang token yang sah (256 bit, tidak bisa ditebak).
- **Status hanya dikelola alur undangan:** `PATCH /v1/users/:id` menolak mengubah status akun menunggu (409) dan menolak memasang `MENUNGGU` pada akun lain (400); `reset-password` oleh admin untuk akun menunggu ditolak (409, gunakan kirim ulang undangan); aksi massal `aktifkan`/`nonaktifkan` melewatinya. Membatalkan undangan = menghapus akun (undangannya ikut terhapus lewat `ON DELETE CASCADE`).
- SSO Google tetap menolak akun yang bukan `AKTIF` (jadi `MENUNGGU` ditolak); aktivasi tidak dilakukan lewat SSO.

### API

| Endpoint | Izin | Isi |
|---|---|---|
| `POST /v1/users/undang` | `pengguna.kelola` | Buat akun `MENUNGGU` + undangan dalam satu transaksi, kirim email. `id` dibuat server (`USR-` + 16 heksadesimal), email dirapikan ke huruf kecil. 201 `{user, berlaku_sampai, email_terkirim}`; 400 (semua galat isian digabung), 409 email terdaftar. |
| `POST /v1/users/:id/undang-ulang` | `pengguna.kelola` | Cabut semua undangan hidup, buat yang baru (kunci baris akun), kirim ke email saat ini. 404 / 409 (bukan `MENUNGGU`). |
| `POST /v1/auth/aktivasi` | publik, batas laju per IP satu ember dengan login | `{token, password}`; 200 / 400 (kebijakan) / 401 (tidak valid, terpakai, dicabut, kedaluwarsa) / 429. |
| `POST /v1/users/impor` | `pengguna.kelola` | Lihat di bawah. |
| `GET /v1/users/ekspor` | `pengguna.kelola` | Lihat di bawah. |

- **Email.** `pkg/email` (`AktivasiLink`, `SendUndangan`; nama dari admin di-escape HTML). Dikirim **sinkron dengan batas 10 detik** untuk undangan tunggal dan kirim ulang (admin langsung tahu bila gagal: `email_terkirim: false`, akun dan undangan tetap ada). `SMTP_HOST` kosong = `email_terkirim: false`. Karena SMTP bisa tidak tersedia, "buat dengan kata sandi sementara" (`POST /v1/users`) **tetap ada sebagai opsi kedua** di dialog undangan (untuk lingkungan tanpa email atau pengguna tanpa akses email); `/pengguna` memakai undangan sebagai cara utama.
- **Pembersihan.** Langkah `undangan_pengguna` di job `auth_pembersihan` menghapus undangan yang kedaluwarsa, terpakai, atau dicabut lebih dari `RetensiResetToken` (bawaan 7 hari) lalu; undangan hidup tidak pernah dihapus. Akun `MENUNGGU` yang undangannya habis tetap ada (kirim ulang).

### Impor CSV

- Server yang mengurai dan memeriksa (`usecase/impor_pengguna.go`); klien hanya mengirim teks (`{csv, dry_run}`, `dry_run` wajib eksplisit). Header `nama,email,peran,lembaga,kabupaten_kode` (`kabupaten_kode` boleh tanpa kolom; urutan bebas; kolom lain ditolak supaya salah ketik tidak menghilangkan data), UTF-8 dengan atau tanpa BOM, koma atau titik koma (dari header), CRLF/LF, sel berkutip boleh memuat pemisah dan baris baru. Berkas bukan UTF-8 (termasuk karakter pengganti U+FFFD dan byte NUL) ditolak dengan petunjuk "CSV UTF-8". Batas **500 baris data dan 512 KB** (baris kosong tidak dihitung); lebih dari itu ditolak seluruhnya (400 / 413).
- Per baris: nama/lembaga wajib dan ≤ 255, email valid (bentuk sempit yang aman untuk header email dan CSV), peran dikenal, kabupaten 4 digit dan ada di master wilayah, email unik dalam berkas (baris pertama tetap sah, salinannya "Email sama dengan baris N") dan terhadap basis data (tak peka huruf besar-kecil). Semua galat satu baris dilaporkan sekaligus; nomor baris = nomor di spreadsheet (header 1).
- **Transaksi:** mode komit memeriksa ulang dengan data terkini, lalu menulis **semua baris sah dalam satu transaksi** (akun + undangan; semua atau tidak sama sekali). Baris tidak sah tidak pernah dibuat dan dilaporkan per baris tanpa menggagalkan yang lain. Bila email sebuah baris keburu didaftarkan pihak lain di antara pemeriksaan dan penulisan, seluruh penulisan dibatalkan (409, teruji dengan komit serentak: tepat satu menang, tak ada akun setengah jadi).
- **Email massal** dikirim sesudah komit **di latar belakang**, paling banyak 4 serempak, masing-masing dibatasi 10 detik (ratusan email serempak lewat SMTP akan menahan permintaan berpuluh detik). Kegagalan tercatat di log dan audit (`KIRIM_UNDANGAN`, `GAGAL`); akunnya tetap ada dan admin memakai "Kirim ulang undangan". Kompromi yang disadari: proses yang mati di tengah pengiriman kehilangan sisa email (akun tetap `MENUNGGU`, kirim ulang); `UseCase.TungguPengirimanEmail` tersedia untuk penutupan layanan dan tes.
- **Injeksi rumus:** nama, email, dan lembaga yang diawali `=`, `+`, `-`, atau `@` ditolak di impor.
- **Audit:** `UNDANG_PENGGUNA` per akun (`metadata.sumber = impor`, nomor baris) dan satu `IMPOR_PENGGUNA` (`total`, `dibuat`, `ditolak`). Mode uji tidak menulis dan tidak diaudit.

### Ekspor CSV

- Menggantikan ekspor sisi klien (yang membangun CSV dari baris tabel tanpa audit dan tanpa penetral rumus). Sumber tunggal filter: `terapkanFilterPengguna` ([filter_pengguna.go](../../apps/api/internal/auth/repository/postgres/filter_pengguna.go)) dipakai bersama `GET /v1/users` dan ekspor; parameter di `dto.EksporPenggunaRequest` (`search`, `ids`). **Menambah kriteria daftar (peran, status, dst.) berarti menambahkannya di dua tempat itu.**
- Kolom daftar putih tetap `id,nama,email,peran,lapis,lembaga,kabupaten_kode,status,mfa_aktif,terakhir_masuk,dibuat_pada`; tidak pernah kata sandi, rahasia MFA, kode cadangan, atau token (teruji). Titik koma + BOM UTF-8 (seperti ekspor audit); sel diawali `= + - @ tab CR` diberi awalan apostrof (sama dengan ekspor audit); sel berisi pemisah/kutip/baris baru dikutip.
- **Audit sebelum serah:** `EKSPOR_DATA` (`entitas = pengguna`, `entitas_id = ekspor`, metadata `baris`, `terpotong`, `search`, `jumlah_id`) ditulis sebelum berkas dikirim; bila audit gagal, jawabannya 500 tanpa data (teruji dengan tabel audit yang dirusak). Maksimal 20.000 baris (`X-Export-Truncated`), `ids` maksimal 500; `ids` yang hanya berisi pemisah ditolak (bukan berubah menjadi semua pengguna). Header `Cache-Control: no-store`.
- Web: unduhan lewat BFF `/api/pengguna/ekspor` (berkas diteruskan apa adanya, sama dengan pola `unduhEksporAudit`). Toolbar punya "Ekspor semua (CSV)"; aksi massal "Ekspor CSV" mengirim id terpilih ke server.

### Web

- **Halaman publik `/aktivasi`** (`app/(auth)/aktivasi`): form kata sandi baru + konfirmasi dengan aturan kebijakan langsung (`useFormBff` + `FormBff`), pesan galat tautan hangus mengarahkan ke admin. `/aktivasi` dan `/api/auth/aktivasi` masuk `PUBLIC_ROUTES` edge guard dan daftar tanpa-pemulihan sesi. BFF `POST /api/auth/aktivasi` dibatasi 5/menit/IP (jendela yang sama dengan login) selain batas Go.
- **`/pengguna`:** toolbar "Undang pengguna" (dialog nama, email, peran, lembaga, kabupaten/kota bila peran berlingkup wilayah), "Impor CSV" (pilih berkas → pratinjau dengan galat per baris → "Impor N pengguna valid" → ringkasan; templat CSV berisi header saja agar templat yang tak diisi tidak membuat akun contoh), "Ekspor semua (CSV)". Kolom status menampilkan "Menunggu aktivasi"; menu baris akun menunggu menggantikan "Reset kata sandi" dengan "Kirim ulang undangan". Dialog undangan menawarkan "Buat dengan kata sandi sementara" (dialog lama). Logika baru di berkas baru (`aksi-undangan.tsx`, `undang-dialog.tsx`, `impor-csv-dialog.tsx`, `lib/pengguna/*`, `lib/api/pengguna-undangan.ts`, `lib/query/undangan-hooks.ts`); `pengguna-page.tsx` hanya disambungkan.

### Tes

- **Go unit** (`usecase/undangan_test.go`, `impor_pengguna_test.go`, `ekspor_pengguna_test.go`, `pkg/email/undangan_test.go`): undang (akun MENUNGGU tanpa kata sandi, hanya hash token, email terkirim ke SMTP palsu `pkg/email/emailtest`, audit tanpa token), semua kelas galat isian, email kembar, SMTP gagal/mati/macet, kirim ulang mencabut tautan lama, aktivasi (sekali pakai, serentak, kedaluwarsa tepat di batas, kata sandi lemah tidak menghabiskan tautan, akun/ email berubah, galat penyimpanan), login/lupa kata sandi akun menunggu identik dengan email tak terdaftar, PATCH/reset; impor (BOM, titik koma, CRLF, sel berkutip, nomor baris, semua galat berkas dan baris, 500/501 baris, batas ukuran, dry run tanpa tulis, komit, bentrok, konkurensi email); ekspor (format, tanpa rahasia, injeksi rumus, filter, terpotong, audit gagal).
- **Go integrasi** (dockertest: `integration/undangan_test.go`, `impor_ekspor_test.go`, `undangan_pembersihan_test.go`): alur penuh via HTTP, 403/401, validasi/409, kirim ulang, kedaluwarsa, serentak, ganti email, jalur lain untuk akun menunggu, batas laju, penulisan atomik, komit serentak, ekspor (isi, header, filter sama dengan daftar, tanpa rahasia, injeksi, audit gagal), pembersihan tepat di batas.
- **Web (node:test):** skema/templat/pemetaan (`lib/pengguna/*.test.ts`), klien API dan unduhan, route BFF (`pengguna-undangan-route.test.ts`, `aktivasi-route.test.ts`: sesi, CSRF, validasi, penerusan tanpa isian liar, header unduhan, batas laju), skema/galat aktivasi, daftar rute publik.
- **E2E** [undangan.spec.ts](../../apps/web/e2e/undangan.spec.ts): undang dari UI → email Mailpit → `/aktivasi` → masuk form → beranda; tautan bekas ditolak; akun menunggu belum bisa masuk; kirim ulang (email baru, tautan lama mati); impor CSV (satu sah dua salah → pratinjau → konfirmasi → "Menunggu aktivasi"); ekspor semua dan terpilih, tercatat di audit. Pemakaian anggaran BFF: aktivasi 3 + masuk 1.
- Catatan: stack demo harus dibangun ulang (`make docker/demo/up`) untuk migration 000108 dan kode baru; email undangan membutuhkan `APP_PUBLIC_URL` dan SMTP (Mailpit di demo).

---

## Backlog lanjutan

- [x] Buka-kunci akun dari UI (lihat [01-F](01-auth.md)); undang pengguna lewat email (set password pertama kali); impor/ekspor CSV pengguna. _(selesai: lihat [02-D](#02-d--undang-pengguna-lewat-email-impor-dan-ekspor-csv))_
- [ ] Filter, sorting, dan paginasi sisi server untuk daftar pengguna (sekarang `list({ search })`).
- [ ] Tampilkan status MFA per pengguna dan tombol reset MFA oleh admin.
- [ ] Riwayat perubahan peran per pengguna (terhubung ke [03](03-audit.md)).
- [ ] Naikkan coverage handler/usecase pengguna; tes komponen `pengguna-page`.
