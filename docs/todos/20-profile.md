# 20 — Halaman profil terpadu & manajemen keamanan akun

**Prioritas:** belum ditetapkan · **Status:** UI ✅ · Backend ✅ · Terhubung ✅ · Skema & seed ✅ (migrasi `000123`) · Tes ✅ (selesai 8 Nov 2026)
**Cara kerja & DoD:** [00-cara-kerja.md](00-cara-kerja.md) · **Terkait:** [01-auth.md](01-auth.md), [02-pengguna-rbac.md](02-pengguna-rbac.md), [03-audit.md](03-audit.md)

## Konteks & tujuan

Menu akun di avatar dropdown topbar ([topbar.tsx](../../apps/web/src/components/layout/topbar.tsx)) mengarah ke tiga halaman auth terpisah di luar shell konsol: `/ganti-password`, `/mfa`, `/keamanan`. Belum ada halaman untuk melihat dan memperbarui profil sendiri (nama, telepon, lembaga, wilayah), dan belum ada endpoint `PATCH /v1/auth/me`.

Tujuan: halaman **`/profil`** di `apps/web/src/app/(console)/profil/page.tsx` (di dalam `ConsoleShell`) dengan empat tab, ditautkan dari avatar dropdown dengan deep link `?tab=`. Seluruh lapisan dikerjakan: migrasi, API Go, BFF, dan UI.

## Yang sudah ada (dicek 8 Nov 2026, commit `bb264f5`)

- **API Go** ([routes.go](../../apps/api/internal/auth/routes/routes.go), grup `protected`): `GET /auth/me` → `dto.MeUserResponse` (`UserResponse` + `mfa_setup_diwajibkan`, `mfa_wajib`, `kode_cadangan_tersisa`), `POST /auth/change-password`, `GET/DELETE /auth/sessions[/:id]`, `POST /auth/sessions/revoke-others`, `POST /auth/mfa/{setup,verify,backup-codes,disable}`. Setup MFA sudah mengembalikan `qr_svg`; `JumlahKodeCadangan = 10`.
- **Patch pengguna:** `domain.PenggunaPatch` + `repo.PatchUser(ctx, id, patch)` (mengembalikan sebelum/sesudah, menulis per kolom) dipakai `PATCH /v1/users/:id` milik admin. Profil mandiri cukup memakai jalur ini, tanpa method repo baru.
- **BFF:** `app/api/auth/me/route.ts` (GET saja), `change-password`, `sessions`, `mfa/*`. Pola mutasi: `validateCSRF` + `validateOrigin`, lalu `tanggapiGalatApi` ([change-password/route.ts](../../apps/web/src/app/api/auth/change-password/route.ts)).
- **UI auth** (sebelum 20-B; logikanya kini di [components/akun/](../../apps/web/src/components/akun/)): `ganti-password-form.tsx` (checklist kebijakan live dari `evaluatePassword`, hitung mundur 423), `mfa-panel.tsx` (QR, kode pertama, kode cadangan: salin, unduh `.txt`, buat ulang, nonaktifkan), `keamanan-panel.tsx` (ikon Laptop/Smartphone, penanda sesi saat ini, cabut, `AlertDialog` cabut semua).
- **Hook & komponen:** `useSesiAktif`, `useCabutSesi`, `useCabutSesiLain` ([sesi-hooks.ts](../../apps/web/src/lib/query/sesi-hooks.ts)), kunci `keys.sesi.aktif`; `SpringButton`, `KolomSandi`, `inisialNama`, `ROLES`; utilitas CSS `glass`, `glass-strong`, `glass-subtle`; preset `fadeIn`, `popIn` ([presets.ts](../../apps/web/src/lib/motion/presets.ts)).
- **Topbar:** dropdown sudah berisi tautan Ganti kata sandi, Verifikasi dua langkah, dan Keamanan akun ke rute auth lama; belum ada "Profil Saya".

## Penyesuaian spesifikasi awal terhadap kode

Spesifikasi awal ditulis sebelum kode dicek. Pada butir berikut, task di bawah mengikuti kolom **Kode / keputusan**.

| #   | Spesifikasi awal                                                             | Kode / keputusan                                                                                                                                                                                                                                                                                                                                             |
| --- | ---------------------------------------------------------------------------- | ------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------ |
| 1   | Tabel `pengguna_aplikasi`, kolom `updated_at`                                | Tabel `pengguna`, kolom `diupdate_pada`                                                                                                                                                                                                                                                                                                                      |
| 2   | Kolom `telepon` diperbarui                                                   | Kolom belum ada → **migrasi baru** (langkah MIG 20-A)                                                                                                                                                                                                                                                                                                        |
| 3   | DTO baru `ProfileResponse`                                                   | Sudah ada `dto.MeUserResponse` dengan field yang sama. Pakai ulang; tambahkan `telepon` ke `UserResponse`                                                                                                                                                                                                                                                    |
| 4   | Method repo baru `UpdateProfile` di `repo_profil.go`                         | Pakai `repo.PatchUser` + `PenggunaPatch` (tambah field `Telepon`)                                                                                                                                                                                                                                                                                            |
| 5   | `u.audit.Log(ctx, audit.Event{Aksi: "pengguna.update_profil_mandiri", ...})` | API audit di modul ini adalah `uc.writeAudit(ctx, audit.Entry{AktorID, Aksi, Entitas: "pengguna", EntitasID, Hasil, Sebelum, Sesudah})` dengan kode aksi huruf kapital (`UPDATE_USER`, `CHANGE_PASSWORD`). Pakai `Aksi: "UPDATE_PROFIL"` dan `userSnapshot` sebelum/sesudah                                                                                  |
| 6   | BFF "memperbarui cookie sesi bila nama berubah"                              | Nama di topbar berasal dari klaim JWT `name` (`lib/auth/resolve.ts`). Cookie hanya berubah bila ada **access token baru**: BFF menukar refresh token (`tukarRefreshToken`, [refresh-session.ts](../../apps/web/src/lib/bff/refresh-session.ts)) setelah PATCH berhasil, memasang cookie baru, lalu klien memanggil `router.refresh()`                        |
| 7   | Checkbox "Keluarkan dari semua perangkat lain" (default dicentang)           | `ChangePassword` di Go **selalu** mencabut sesi lain (`replaceCurrentSession`). Lihat Keputusan terbuka K1                                                                                                                                                                                                                                                   |
| 8   | Checklist "huruf besar **& kecil**"                                          | `ValidatePassword` (Go) dan cerminnya `evaluatePassword` menuntut: min. 8 karakter, kapital, angka, karakter khusus, bukan sandi umum. Huruf kecil tidak dituntut. Checklist UI tetap dari `evaluatePassword`; aturan baru hanya ditambahkan di Go dan web sekaligus                                                                                         |
| 9   | 8–10 kode cadangan                                                           | Tepat 10 (`JumlahKodeCadangan`)                                                                                                                                                                                                                                                                                                                              |
| 10  | Sesi: "lokasi perkiraan"                                                     | DTO sesi hanya punya `perangkat`, `user_agent`, `ip`, `dibuat_pada`, `terakhir_dipakai_pada`, `saat_ini`. Tidak ada sumber GeoIP → backlog                                                                                                                                                                                                                   |
| 11  | Email "badge verified"                                                       | Tidak ada status verifikasi email di `pengguna`. Tampilkan email read-only dengan ikon gembok, tanpa klaim "terverifikasi"                                                                                                                                                                                                                                   |
| 12  | "Cakupan wilayah: provinsi / kabupaten / kota"                               | Data yang ada: `lapis` dan `kabupaten_kode`. Nama kabupaten diambil dari master wilayah; `kabupaten_kode` kosong berarti cakupan provinsi                                                                                                                                                                                                                    |
| 13  | Hook memakai kunci `['auth', 'me']`                                          | Kunci query terpusat di [keys.ts](../../apps/web/src/lib/query/keys.ts). Tambahkan `keys.profil.saya()`                                                                                                                                                                                                                                                      |
| 14  | `npm --prefix apps/web run test`                                             | Monorepo memakai pnpm: `pnpm --dir apps/web test`                                                                                                                                                                                                                                                                                                            |
| 15  | Halaman auth lama digantikan                                                 | `/ganti-password` (`PASSWORD_CHANGE_PATH`) dan `/mfa` (`MFA_SETUP_PATH`) adalah tujuan pengalihan wajib di [edge-guard.ts](../../apps/web/src/lib/rbac/edge-guard.ts) untuk akun wajib ganti sandi/wajib MFA, yang belum boleh masuk konsol. **Keduanya tetap ada**; logikanya dipindah ke komponen bersama yang dipakai oleh halaman auth dan tab `/profil` |

## Keputusan (ditetapkan 8 Nov 2026)

- [x] **K1 — Pencabutan sesi saat ganti sandi:** perilaku sekarang dipertahankan, jadi sesi lain selalu dicabut. Checkbox diganti teks info "Perangkat lain akan dikeluarkan setelah kata sandi diganti." API tidak berubah.
- [x] **K2 — Nasib `/keamanan`:** dialihkan ke `/profil?tab=keamanan`, karena rute ini bukan tujuan pengalihan wajib. `/ganti-password` dan `/mfa` tetap hidup (butir 15).
- [x] **K3 — Format telepon:** E.164. Aturan yang sama dipakai di Go (usecase) dan web (`lib/profil/telepon.ts`):
  1. Trim; `""` berarti hapus (NULL).
  2. Buang spasi, `-`, `.`, `(`, `)`.
  3. Awalan `+62`, `62`, dan `0` dinormalisasi ke `+62`; awalan lain ditolak.
  4. Bagian nasional harus cocok `^[1-9][0-9]{7,12}$` (total 10–15 digit).
  5. Galat: "Nomor telepon tidak valid".
- [ ] **K4 — Admin ikut mengubah telepon** (`PATCH /v1/users/:id`, halaman `/pengguna`): **ditunda**, di luar cakupan 20-A/20-B.

**Kontrak `PATCH /v1/auth/me`:**

- **Body:** `{ "nama"?: string, "telepon"?: string }`. Field yang absen atau `null` tidak diubah; `"telepon": ""` menghapus nomor. Field lain diabaikan.
- **Galat 400:**
  - "Nama minimal 3 karakter" / "Nama maksimal 100 karakter" (setelah trim, hitung rune);
  - "Nomor telepon tidak valid";
  - "Tidak ada perubahan" bila body tidak memuat nama maupun telepon.
- **Nilai sama dengan yang tersimpan:** 200 tanpa menulis DB dan tanpa audit.
- **Sukses:** data berbentuk sama dengan `GET /v1/auth/me` ditambah `telepon`.

---

## 20-A — Profil mandiri: data diri + `PATCH /v1/auth/me` + navigasi topbar _(Fase F)_

**Tujuan:** pengguna dapat melihat profil dan memperbarui nama serta telepon dari `/profil`. Perubahan tercatat di audit dan langsung tampil di avatar topbar tanpa keluar-masuk.
**Bergantung pada:** 01, 02, 03-A (penulis audit).

1. **MIG**
   - [x] Migrasi baru (nomor diambil saat dibuat, sekarang terakhir `000122`): `NNNNNN_20a_pengguna_telepon.sql`, `ALTER TABLE pengguna ADD COLUMN IF NOT EXISTS telepon varchar(20)` (NULL boleh), dengan `Down` yang menghapus kolom.
   - [x] Periksa hak `garda_aplikasi` ([03-C](03-audit.md)): `UPDATE` pada kolom baru ikut ter-grant.
2. **SEED**
   - [x] `demo`: nomor telepon fiktif yang deterministik untuk akun demo (rentang `+62 812-0000-xxxx`). Lapis `reference`/`admin` tidak diubah.
3. **API** (`apps/api/internal/auth`)
   - [x] `domain.Pengguna`: field `Telepon *string` (`gorm:"column:telepon"`). `domain.PenggunaPatch`: field `Telepon`, ditangani di `Kolom()` dan `Terapkan()`.
   - [x] `dto/request.go`: `UpdateProfilRequest { Nama *string "binding:omitempty,min=3,max=100"; Telepon *string "binding:omitempty,max=20" }`. Hanya kolom yang dikirim yang diubah; `telepon: ""` menghapus nomor.
   - [x] `dto.UserResponse`: field `Telepon *string "json:telepon,omitempty"`, diisi di `ToUserResponse`.
   - [x] `usecase/profil.go`: `UpdateProfil(ctx, userID string, patch ProfilPatch) (*domain.Pengguna, error)`. Isinya:
     - trim nama dan normalisasi telepon (K3);
     - tolak body tanpa nama dan telepon (400) atau format telepon tidak valid; nilai yang sama dengan tersimpan menjawab 200 tanpa menulis;
     - panggil `repo.PatchUser`;
     - tulis `uc.writeAudit(ctx, audit.Entry{AktorID: userID, Aksi: "UPDATE_PROFIL", Entitas: "pengguna", EntitasID: userID, Hasil: audit.HasilBerhasil, Sebelum: userSnapshot(sebelum), Sesudah: userSnapshot(sesudah)})`. `userSnapshot` ikut memuat `telepon`.
   - [x] Batas kewenangan: email, peran, lapis, lembaga, kabupaten, dan status **tidak bisa** diubah lewat endpoint ini. Field di luar `nama`/`telepon` ditolak atau diabaikan dan dikunci dengan tes.
   - [x] Handler `UpdateMe` di [handler.go](../../apps/api/internal/auth/delivery/http/handler.go): `userID := c.GetString("user_id")`, `ShouldBindJSON`, panggil usecase, jawab `dto.MeUserResponse` (bentuk sama dengan `GET /auth/me`, termasuk status MFA). Error dipetakan: 400 untuk validasi, 404 bila pengguna tidak ada, 500 untuk galat lain.
   - [x] Rute: `protected.PATCH("/auth/me", handler.UpdateMe)` di [routes.go](../../apps/api/internal/auth/routes/routes.go).
   - [x] OpenAPI: `patch` pada `/v1/auth/me` di [openapi.yaml](../../packages/contracts/openapi.yaml), field `telepon` di skema pengguna; `make openapi/sync`.
4. **FE**
   - [x] [api-client.ts](../../apps/web/src/lib/bff/api-client.ts): tipe `UpdateProfilRequest { nama?: string; telepon?: string | null }`, `telepon?: string` di `UserResponse`, dan method `authApi.updateMe(accessToken, data)` (`PATCH /auth/me`, `Authorization: Bearer`).
   - [x] BFF `PATCH` di [me/route.ts](../../apps/web/src/app/api/auth/me/route.ts):
     - `validateCSRF` + `validateOrigin`, lalu `getAccessToken()` dan `authApi.updateMe`;
     - bila `nama` berubah, tukar refresh token dan pasang cookie access/refresh baru (butir 6);
     - galat lewat `tanggapiGalatApi`.
   - [x] `lib/query/profil-hooks.ts`:
     - `useProfil()` (`GET /api/auth/me`, kunci `keys.profil.saya()`);
     - `useUbahProfil()` (PATCH dengan pembaruan optimistis dan rollback saat galat). Setelah sukses: invalidasi `keys.profil.saya()` lalu `router.refresh()` agar `SessionProvider` menerima klaim baru.
   - [x] Halaman `app/(console)/profil/page.tsx`: tata letak 2 kolom. Di desktop, navigasi tab vertikal di kiri dan panel kartu di kanan; di mobile, pills horizontal (`no-scrollbar overflow-x-auto`). Tab aktif dibaca dari `?tab=` (`profil` bawaan, `ganti-password`, `mfa`, `keamanan`); nilai tak dikenal jatuh ke `profil`. Ganti tab memakai `router.replace` agar riwayat tidak menumpuk. Rute tanpa izin khusus: semua pengguna yang masuk boleh membukanya (`requiredPermission` tidak mendaftarkannya).
   - [x] **Tab Edit Profil:**
     - **Ringkasan akun:** avatar besar dari `inisialNama`, badge peran `ROLES[role].label`, badge status ("Aktif"), dan tanggal bergabung (`dibuat_pada`).
     - **Form data pribadi:** nama lengkap (min. 3 karakter) dan telepon/WhatsApp (format Indonesia, K3). Email read-only dengan ikon gembok dan catatan "Hubungi administrator untuk perubahan email instansi".
     - **Penugasan (read-only):** lembaga; cakupan wilayah (butir 12); peran utama; daftar izin efektif dari klaim `permissions` sesi dalam accordion/chip list.
     - **Aksi:** "Simpan Perubahan" (`SpringButton` + `Loader2`, nonaktif bila tidak ada perubahan) dan "Batalkan perubahan". Hasil ditampilkan dengan `toast.success` / `toast.error` berbahasa Indonesia.
   - [x] **Topbar** ([topbar.tsx](../../apps/web/src/components/layout/topbar.tsx)), urutan dropdown:
     1. Header profil (sudah ada).
     2. Profil Saya (`User`) → `/profil`.
     3. Ganti Kata Sandi (`KeyRound`) → `/profil?tab=ganti-password`.
     4. Verifikasi 2 Langkah (`ShieldCheck`) → `/profil?tab=mfa`.
     5. Perangkat & Sesi Aktif (`MonitorSmartphone`) → `/profil?tab=keamanan`.
     6. Separator.
     7. Toggle Terang/Gelap (sudah ada).
     8. Separator.
     9. Keluar (`keluarDariSesi()`, sudah ada).
5. **TES**
   - [x] Go usecase: ubah nama saja, ubah telepon saja, hapus telepon, normalisasi `08…`/`62…`/`+62…`, tolak telepon tidak valid, tolak nama < 3 karakter, tolak permintaan tanpa perubahan, audit `UPDATE_PROFIL` berisi sebelum/sesudah.
   - [x] Go handler/integrasi (Postgres sungguhan, token nyata): `PATCH /v1/auth/me` 200 untuk semua peran; 401 tanpa token; field `peran`/`email` di body tidak mengubah apa pun; `GET /auth/me` sesudahnya memuat nilai baru.
   - [x] Tes kontrak skema hijau dengan kolom `telepon`.
   - [x] Web unit: route BFF PATCH (CSRF ditolak 403, token baru dipasang bila nama berubah); `authApi.updateMe`; pembacaan `?tab=`.
   - [x] Web komponen: form edit (validasi, tombol nonaktif tanpa perubahan, batalkan perubahan); urutan dan `href` menu dropdown topbar.
   - [x] E2E: ubah nama dari `/profil`, nama baru tampil di avatar topbar tanpa login ulang; tautan dropdown "Verifikasi 2 Langkah" membuka `/profil?tab=mfa`.
6. **DoD**: checklist [00 §4](00-cara-kerja.md). Tambahkan `telepon` ke inventaris data pribadi UU PDP ([18 §D](18-platform-lintas-fitur.md)).

---

## 20-B — Tab keamanan di `/profil`: ganti sandi, MFA, sesi aktif _(Fase F)_

**Tujuan:** tiga fitur keamanan yang kini terpisah di rute auth tersedia sebagai tab di `/profil`, dengan perilaku yang sama. Halaman auth lama tetap berfungsi untuk alur wajib (butir 15).
**Bergantung pada:** 20-A (kerangka halaman dan tab).

1. **MIG / SEED**: tidak ada. Bila K1 memilih checkbox, tidak perlu migrasi (hanya field request).
2. **API**
   - [x] ~~Hanya bila K1 = checkbox~~ tidak berlaku (K1: teks info, API tetap).
3. **FE**
   - [x] **Refaktor ke komponen bersama**, tanpa salinan logika. Ekstrak inti `ganti-password-form`, `mfa-panel`, dan `keamanan-panel` ke `components/akun/` (mis. `FormGantiSandi`, `PanelMFA`, `DaftarSesi`). Halaman `(auth)/ganti-password`, `(auth)/mfa`, `(auth)/keamanan` dan tab `/profil` memakai komponen yang sama. Perbedaan perilaku sesudah sukses (alur wajib berpindah halaman, tab tetap di tempat) diatur lewat prop.
   - [x] **Tab Ganti Kata Sandi:**
     - **Kolom:** kata sandi saat ini, kata sandi baru, konfirmasi (`KolomSandi`).
     - **Meter kekuatan** baru (lemah/sedang/kuat), dihitung dari jumlah aturan `evaluatePassword` yang terpenuhi ditambah panjang. Checklist kebijakan live tetap dipakai (butir 8).
     - **Galat:** penanganan 423 (akun terkunci, hitung mundur) dan 429 (batas laju) seperti di form sekarang.
     - **Sesi lain:** checkbox atau teks info sesuai K1.
     - **Sesudah sukses:** `toast.success`, form direset, dan cookie sesi baru sudah dipasang oleh BFF.
   - [x] **Tab Verifikasi 2 Langkah:**
     - **Status:** aktif (perisai hijau) atau belum aktif (banner imbauan).
     - **Setup:** QR dari `qr_svg`, "Salin kunci manual", lalu input token 6 digit.
     - **Kode cadangan:** 10 kode tampil sekali dengan "Salin semua" dan "Unduh berkas (.txt)". "Buat ulang kode cadangan" meminta konfirmasi kata sandi. Sisa kode (`kode_cadangan_tersisa`) ditampilkan.
     - **Nonaktifkan:** dialog konfirmasi dengan kata sandi. Tombol disembunyikan atau dinonaktifkan dengan penjelasan bila `mfa_wajib`.
     - **Transisi:** antar-tahap memakai `motion/react` (`AnimatePresence` + `fadeIn`).
   - [x] **Tab Perangkat & Sesi Aktif:** daftar dari `useSesiAktif()`. Tiap kartu menampilkan ikon perangkat, browser/OS (`perangkat`), IP, waktu login (`dibuat_pada`), dan terakhir aktif (`terakhir_dipakai_pada`), dengan badge **"Sesi Ini"** untuk `saat_ini`. "Cabut Sesi" per perangkat lain memakai konfirmasi cepat. "Keluarkan Semua Perangkat Lain" memakai `AlertDialog`.
   - [x] `/keamanan` sesuai K2. Tautan internal ke rute lama (mis. pesan galat, email) diperiksa ulang.
   - [x] Pergantian tab tanpa layout shift: tinggi panel stabil dan skeleton saat memuat.
4. **TES**
   - [x] Komponen: meter kekuatan (batas lemah/sedang/kuat), checklist kebijakan, hitung mundur 423; MFA: tahap setup → kode → cadangan, salin/unduh, nonaktifkan tersembunyi bila `mfa_wajib`; sesi: badge "Sesi Ini", tidak ada tombol cabut pada sesi saat ini, dialog cabut semua.
   - [x] Regresi alur wajib: akun `wajib_ganti_password` tetap diarahkan ke `/ganti-password` dan akun wajib MFA ke `/mfa`; keduanya tidak bisa membuka `/profil` sebelum selesai.
   - [x] E2E: ganti sandi dari tab, sesi lain tercabut (sesuai K1); aktivasi MFA dari tab dengan TOTP; cabut satu sesi dari tab. Suite [sesi.spec.ts](../../apps/web/e2e/sesi.spec.ts) dan [auth.spec.ts](../../apps/web/e2e/auth.spec.ts) disesuaikan bila memakai rute lama.
5. **DoD**: checklist [00 §4](00-cara-kerja.md).

---

## Hasil implementasi (8 Nov 2026)

Selisih terhadap rencana di atas, beserta alasannya:

- **Sesi terbatas tidak bisa mengubah profil.** Bagi token wajib-ganti-sandi atau wajib-setup-MFA, `GET /v1/auth/me` tetap terbuka, tetapi `PATCH` dijawab 403 (`PASSWORD_CHANGE_REQUIRED` / `MFA_SETUP_REQUIRED`). Pengaturnya `terbukaBagiSesiTerbatas` di `pkg/middleware/auth.go`.
- **Jaring di DB.** `chk_pengguna_telepon` menolak telepon yang bukan E.164 Indonesia, sehingga penulisan SQL langsung juga harus ternormalisasi.
- **Seed demo.** Ke-25 akun demo mendapat nomor `+62812000000NN` sesuai urutan seeder.
- **BFF.** Galat diteruskan lewat `handleRouteError` (status dan pesan Go apa adanya), bukan `tanggapiGalatApi` yang khusus alur login. Nama berubah dideteksi dari klaim `name` pada access token; penukaran refresh token yang gagal tetap menjawab 200.
- **Kunci query dan tab.** Kunci query bernama `qk.profil.saya()`. Ganti tab memakai `history.replaceState` (Next menyelaraskan `useSearchParams`), bukan `router.replace`, sehingga tidak ada permintaan ke server.
- **Komponen bersama** ada di [components/akun/](../../apps/web/src/components/akun/): `FormGantiSandi`, `PanelMfa`, `DaftarSesi`, `MeterKekuatanSandi`, dengan prop `mode` `halaman` (alur wajib) atau `tab` (tetap di tempat). Konfirmasi sandi untuk buat ulang kode cadangan dan nonaktifkan MFA kini lewat dialog.
- **Navigasi.** Judul topbar di `/profil` adalah "Profil saya". Menu profil aplikasi lapangan juga menaut ke "Profil Saya" dan "Perangkat & Sesi Aktif". Filter aksi `/audit` mengenal `UPDATE_PROFIL`.
- **Tes.**
  - Go: 44 fungsi tes baru (usecase, handler, integrasi Postgres dengan token nyata, middleware, migrasi).
  - Web: lebih dari 300 tes unit dan komponen baru. Harness `src/test/render.tsx` kini menutup toast Sonner di setiap `afterEach`.
  - E2E: [profil.spec.ts](../../apps/web/e2e/profil.spec.ts) (13 tes), serta `sesi.spec.ts`, `halaman.spec.ts`, dan `auth.spec.ts` yang disesuaikan.
  - Hasil penuh: `go test ./...` hijau, web 1.376 unit dan 241 komponen, e2e 346/346.
- **Keterbatasan.**
  - Kode cadangan yang baru tampil hilang bila pengguna pindah tab sebelum menyimpannya. MFA tetap aktif dan kode bisa dibuat ulang dengan kata sandi; belum ada peringatan sebelum pindah tab.
  - Pengalihan akun wajib-MFA ke `/mfa` hanya diuji unit, karena demo berjalan tanpa `MFA_ENFORCE`.

**Perbaikan sisa 04-D (di luar cakupan 20, memblokir seed demo dan e2e).** Migrasi `000122` mengubah `pasar.id` menjadi BIGINT surrogate, tetapi kolom anak (`entri_harga.pasar_id`, `tugas_lapangan.pasar_id`, `pedagang.pasar_id`) tetap berisi kode pasar.

- **Seeder.** `harga`, `lapangan`, dan `master` (termasuk jalur skema sebelum 000122) disesuaikan, jadi `cmd/seed reference demo` kembali berhasil dan idempoten.
- **Query.** Join `(p.kode = e.pasar_id OR p.id::text = e.pasar_id)` diganti `p.kode = e.pasar_id`. Akibatnya `POST /v1/harga` dengan id surrogate kini menjawab 400, bukan 500.
- **Web.** Sheet detail pasar, filter pedagang, dan `entitas_id` usulan pasar memakai kode, bukan id angka.
- **Tes.** Sebelumnya 112 tes Go di 12 paket gagal di `main`; kini seluruh suite hijau.

## Standar UI/UX

- **Kartu:** utilitas `glass` / `glass-strong` dengan `rounded-2xl` atau `rounded-xl`. Latar panel `bg-white/80 dark:bg-navy-900/60 backdrop-blur-xl`.
- **Warna:** token `navy-900`, `navy-800`, `data-500`, `data-600`; status `emerald-500` (aman) dan `defisit` (bahaya). Hindari warna literal di luar token `globals.css`.
- **Animasi:** `motion.div` dengan preset `fadeIn` / `popIn` dari [presets.ts](../../apps/web/src/lib/motion/presets.ts) saat ganti tab dan antar-tahap MFA. Animasi dimatikan saat `prefers-reduced-motion`.
- **Responsif:** mobile (`< sm`), tablet (`md`), desktop (`lg`/`xl`). Navigasi tab mobile bisa di-scroll horizontal; target sentuh ≥ 44 px.
- **Umpan balik:** semua mutasi menampilkan toast Sonner berbahasa Indonesia. Galat dari API diteruskan apa adanya bila sudah informatif (pola `tanggapiGalatApi`).
- **Aksesibilitas:** tab memakai pola `role="tablist"` (atau komponen `ui/tabs` Radix), fokus keyboard terlihat, dan label pada setiap input dan tombol ikon.

## Kriteria penerimaan

1. **Navigasi topbar:** dropdown avatar menampilkan Profil Saya dan tiga sub-tab dengan urutan di 20-A; setiap tautan membuka tab yang benar (deep link `?tab=`).
2. **Edit profil:** nama dan telepon berhasil diubah; nilai baru tampil di avatar topbar tanpa login ulang, tersimpan di DB, dan tercatat di `/audit` sebagai `UPDATE_PROFIL`.
3. **Ganti kata sandi:** meter kekuatan dan checklist bekerja real-time; sandi berhasil diubah; sesi lain dicabut sesuai K1.
4. **MFA:** QR ter-render tajam (SVG); verifikasi TOTP 6 digit berhasil; 10 kode cadangan dapat disalin dan diunduh; nonaktifkan tidak tersedia bagi akun `mfa_wajib`.
5. **Sesi aktif:** daftar perangkat akurat dengan penanda "Sesi Ini"; cabut satu sesi dan cabut semua sesi lain bekerja.
6. **Alur wajib tidak rusak:** `/ganti-password` dan `/mfa` tetap menjadi tujuan pengalihan untuk akun yang wajib ganti sandi atau wajib MFA.
7. **Kualitas kode & tes:**
   - `go -C apps/api vet ./...` dan `go -C apps/api test ./internal/auth/... -v` hijau (lalu `go test ./...` penuh untuk DoD).
   - `pnpm --dir apps/web typecheck` dan `pnpm --dir apps/web test` hijau, tanpa regresi.
   - `make test/e2e` untuk suite auth, sesi, dan profil.

## Backlog lanjutan

- [ ] Perkiraan lokasi sesi dari IP (butuh sumber GeoIP lokal, mis. basis data MaxMind; jangan memanggil layanan pihak ketiga dengan IP pengguna tanpa dasar UU PDP).
- [ ] Unggah foto profil (butuh penyimpanan objek, [18 §C](18-platform-lintas-fitur.md)).
- [ ] Verifikasi nomor telepon via OTP (bergantung pada kanal WhatsApp/SMS yang belum ada).
- [ ] Riwayat aktivitas akun sendiri di `/profil` (login, ganti sandi, MFA) dari `audit_log` dengan filter aktor = diri sendiri.
- [ ] Preferensi notifikasi per pengguna (EWS, web push) di tab tersendiri.
- [ ] Peringatan sebelum meninggalkan tab MFA saat kode cadangan belum disimpan.
- [ ] Pertimbangkan menyamarkan `telepon` di snapshot `audit_log` (append-only) dan di respons pengguna untuk admin, sesuai kebijakan UU PDP ([18 §D](18-platform-lintas-fitur.md)).
- [ ] K4: admin mengubah telepon pengguna lain lewat `PATCH /v1/users/:id` dan halaman `/pengguna`.
