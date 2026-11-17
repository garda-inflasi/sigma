# Impersonasi peran dan akun (alih akun sementara)

Fitur bagi `superadmin` (peran `ADMIN_SISTEM`, lihat `accounts.md`) dan akun lain yang diizinkan kebijakan untuk
sementara bekerja sebagai akun lain guna memeriksa tampilan, izin, dan alur kerja suatu peran. Seluruh aksi tetap
dipertanggungjawabkan kepada akun asli (audit hash berantai, Permenkes dan UU PDP).

## Keputusan desain

1. **Impersonasi selalu atas AKUN, bukan sekadar menimpa peran di token.** Modul lain (dashboard, harga, lapangan)
   menentukan peran dan cakupan kabupaten dari basis data berdasarkan `user_id`, bukan dari klaim `role`. Token yang
   hanya mengubah `role` membuat izin dan cakupan data tidak selaras. Maka `sub`/`user_id` token = akun sasaran, dan
   klaim `imp` menunjuk pelaku asli.
2. **Impersonasi ke "peran" (`target_peran`)** diselesaikan server menjadi satu akun AKTIF berperan itu yang
   diizinkan: tepat satu kandidat -> dipakai; lebih dari satu -> 409 `TARGET_AMBIGU` (klien meminta memilih akun lewat
   `target_pengguna_id`); nol -> 404 `TARGET_TIDAK_ADA`. UI pemilih cepat menampilkan peran, dan akun di bawahnya
   bila lebih dari satu.
3. **Superadmin = `ADMIN_SISTEM`.** Selalu boleh meng-impersonate akun mana pun KECUALI akun `ADMIN_SISTEM`
   (tidak pernah boleh menjadi sasaran, oleh siapa pun) dan dirinya sendiri, selama kebijakan global `aktif`.
4. **Izin `impersonasi.kelola`** (kelompok "Pengguna dan audit") mengatur kebijakan dan riwayat. Eksklusif milik
   `ADMIN_SISTEM`: ditegakkan (a) rute Go: izin dan peran `ADMIN_SISTEM`, (b) usecase memuat ulang akun dan memeriksa
   peran, (c) editor matriks peran menolak memberikannya ke peran lain atau mencabutnya dari `ADMIN_SISTEM`, (d) izin
   langsung per pengguna menolaknya. Peran lain (mis. QA) yang di-allowlist hanya mendapat hak _memulai_ impersonasi,
   bukan mengubah kebijakan.
5. **Sesi.** Memulai impersonasi mencabut sesi asal pelaku dan membuka sesi baru (family refresh token sendiri,
   `refresh_tokens.impersonasi_id` terisi, tidak tampil di daftar sesi milik sasaran). Berhenti mencabut sesi
   impersonasi dan menerbitkan sesi baru bagi pelaku (tanpa kata sandi/MFA ulang: bukti ada pada catatan
   `impersonasi_sesi` di server, bukan di body/header klien). Tidak ada impersonasi bersarang: memulai impersonasi
   saat sedang impersonasi = _ganti_, otorisasi dievaluasi terhadap pelaku asli, catatan lama berakhir `BERGANTI`.
   Mencabut semua sesi pelaku (keluar dari semua perangkat, pencabutan sesi lain, paksa keluar oleh admin, ganti atau
   reset kata sandi, penonaktifan) juga mencabut sesi impersonasi yang ia mulai, walau sesi itu tercatat atas `user_id`
   sasaran. Logout dari sesi impersonasi menutup catatannya (`LOGOUT`).
6. **Penegakan per request** (`pkg/middleware.JWTAuth`): token ber-`imp` hanya sah selama catatan `impersonasi_sesi`
   belum berakhir/kedaluwarsa, kebijakan masih `aktif`, pelaku masih AKTIF dan masih berhak, sasaran masih AKTIF dan
   masih diizinkan. Gagal -> 401 `IMPERSONATION_ENDED` (klien web mencoba refresh, refresh ditolak, masuk ulang).
   Izin efektif = izin akun sasaran dari basis data (bukan izin pelaku).
7. **Dilarang selama impersonasi** (403 `IMPERSONATION_RESTRICTED`): ganti kata sandi, semua endpoint MFA,
   `PATCH /auth/me`, `logout-all`, pencabutan sesi, mutasi `/users*` dan `/roles*`, serta pengaturan impersonasi
   (kecuali `stop`). Mencegah pelaku mengubah kredensial atau hak sasaran.
8. **Audit.** (a) `audit.Writer` mengganti `aktor_id` dengan pelaku asli dan menambah `metadata.impersonasi`
   (`sesi_id`, `sebagai_pengguna_id`, `peran_efektif`) pada SETIAP baris yang ditulis selama impersonasi; `metadata`
   masuk hash berantai sehingga tamper-evident. (b) `IMPERSONASI_MULAI`, `IMPERSONASI_GANTI`, `IMPERSONASI_BERHENTI`,
   `IMPERSONASI_DIAKHIRI` (paksa oleh admin), `IMPERSONASI_BERAKHIR` (otomatis: kedaluwarsa, kebijakan nonaktif atau tidak
   lagi mengizinkan, akun tidak aktif), `IMPERSONASI_KEBIJAKAN_UBAH`. Penolakan memulai juga dicatat (`DITOLAK`). (c) Jaring pengaman: setiap request mutasi
   (POST/PUT/PATCH/DELETE) selama impersonasi menulis `IMPERSONASI_AKSI` (metode, rute, status).
9. Skema tidak menyisipkan data (tes `TestMigrationsLeaveEveryTableEmpty`): tanpa baris di `impersonasi_kebijakan`,
   kebijakan bawaan berlaku (`aktif=true`, `wajib_alasan=true`, `durasi_maks_menit=60`, `versi=0`) sampai superadmin
   menyimpannya pertama kali. Seeder reference hanya menambah izin `impersonasi.kelola` ke katalog dan matriks
   `ADMIN_SISTEM`; di stack yang sudah berjalan jalankan `seed reference` sekali sesudah migrasi 000129 agar izin
   itu tampil di katalog dan matriks (penegakannya tidak bergantung pada seed: resolver izin selalu menambahkannya
   ke `ADMIN_SISTEM` dan membuangnya dari peran lain).
10. `permissionsVersion` pada brief tidak ada di basis kode ini; padanannya adalah klaim `permissions` + penolakan
    `PERMISSIONS_CHANGED` bila berbeda dari izin efektif DB. Token impersonasi membawa izin sasaran.

## Kontrak API Go (`/v1`, amplop `{code,message,data}`)

Nama field mengikuti konvensi API ini (snake_case Indonesia). Peran = nilai enum (`MANAJER_DATA`, dst.).

### Kebijakan (khusus `ADMIN_SISTEM` + izin `impersonasi.kelola`; selain itu 403)

`GET /admin/impersonation/policy` -> `Kebijakan`; `PUT /admin/impersonation/policy` body `KebijakanInput` -> `Kebijakan`.
PUT mengganti seluruh kebijakan (pengaturan + daftar pelaku + target) dalam satu transaksi dengan kunci optimistis `versi`.
Versi usang -> 409 `VERSI_KEBIJAKAN_BERUBAH` dengan `data` = kebijakan terkini.

```jsonc
// Kebijakan
{
  "aktif": true,
  "wajib_alasan": true,
  "durasi_maks_menit": 60, // null = tanpa batas; selain itu 5..1440
  "versi": 3, // 0 bila belum pernah disimpan (bawaan)
  "diperbarui_pada": "2026-11-17T10:00:00Z", // null bila belum pernah disimpan
  "diperbarui_oleh": "uuid|null",
  "diperbarui_oleh_nama": "string|null",
  "pelaku": [
    {
      "tipe": "PERAN", // "PERAN" | "PENGGUNA"
      "peran": "MANAJER_MUTU", // bila tipe PERAN
      "pengguna_id": null, // bila tipe PENGGUNA
      "pengguna_nama": null,
      "pengguna_email": null,
      "pengguna_peran": null, // hanya di respons
      "aktif": true,
      "target": [
        { "tipe": "PERAN", "peran": "OPS_MAHASISWA" },
        {
          "tipe": "PENGGUNA",
          "pengguna_id": "uuid",
          "pengguna_nama": "...",
          "pengguna_email": "...",
          "pengguna_peran": "OPS_KOORDINATOR",
        },
      ],
    },
  ],
}
// KebijakanInput = { versi, aktif, wajib_alasan, durasi_maks_menit, pelaku: [{tipe, peran|pengguna_id, aktif, target:[{tipe, peran|pengguna_id}]}] }
```

Validasi (400 dengan pesan Indonesia): `ADMIN_SISTEM` tidak boleh jadi pelaku (selalu boleh) maupun target; peran tak
dikenal; pengguna tak ada; duplikat; maks 100 pelaku dan 200 target per pelaku.

### Riwayat dan pengakhiran paksa (sama: khusus `ADMIN_SISTEM` + `impersonasi.kelola`)

`GET /admin/impersonation/riwayat?status=semua|aktif&limit=&cursor=` -> `{ items: Riwayat[], next_cursor }`

```jsonc
// Riwayat
{
  "id": "uuid",
  "pelaku": {
    "id": "uuid|null",
    "nama": "...",
    "email": "...",
    "peran": "ADMIN_SISTEM",
  }, // snapshot
  "sasaran": {
    "id": "uuid|null",
    "nama": "...",
    "email": "...",
    "peran": "OPS_KOORDINATOR",
  },
  "alasan": "string|null",
  "mulai_pada": "...",
  "kedaluwarsa_pada": "...|null",
  "berakhir_pada": "...|null",
  "alasan_berakhir": "STOP|BERGANTI|KEDALUWARSA|KEBIJAKAN|DIAKHIRI|LOGOUT|DICABUT|null",
  "aktif": true,
  "ip": "...|null",
  "perangkat": "Chrome di macOS",
}
```

`POST /admin/impersonation/riwayat/:id/akhiri` -> `{ "berakhir": true }` (404 bila tidak ada atau sudah berakhir).

### Pemilih sasaran (siapa pun yang login; bukan 403 bila tidak berhak)

`GET /auth/impersonation/targets` ->

```jsonc
{
  "boleh": true, // pemanggil (atau pelaku asli bila sedang impersonasi) berhak memulai
  "aktif": true, // kebijakan global aktif
  "wajib_alasan": true,
  "durasi_maks_menit": 60,
  "peran": [
    {
      "peran": "OPS_KOORDINATOR",
      "label": "Koordinator Lapangan",
      "lapis": "OPERASIONAL",
      "akun": [
        {
          "id": "uuid",
          "nama": "...",
          "email": "...",
          "lembaga": "...",
          "kabupaten_kode": "1101|null",
        },
      ],
    },
  ],
}
```

Hanya peran yang punya >= 1 akun AKTIF layak (tidak termasuk pemanggil sendiri, tidak termasuk `ADMIN_SISTEM`).
`boleh:false` -> `peran: []`.

### Mulai dan berhenti

`POST /auth/impersonation/start` body `{ "target_pengguna_id"?: uuid, "target_peran"?: enum, "alasan"?: string<=500 }`
(salah satu target wajib; `target_pengguna_id` didahulukan). Sukses 200:

```jsonc
{ "access_token": "...", "refresh_token": "...", "user": UserResponse(sasaran), "impersonasi": ImpersonasiMeta }
```

Galat: 400 `ALASAN_WAJIB`; 403 `IMPERSONASI_NONAKTIF` | `IMPERSONASI_DITOLAK` (pelaku tak berhak atau sasaran tak
diizinkan/`ADMIN_SISTEM`/diri sendiri); 404 `TARGET_TIDAK_ADA`; 409 `TARGET_AMBIGU`; 403 `IMPERSONATION_RESTRICTED`.

`POST /auth/impersonation/stop` (tanpa body; sah bagi sesi impersonasi apa pun, tanpa izin khusus) -> 200
`{ "access_token": "...", "refresh_token": "...", "user": UserResponse(pelaku) }`. 400 `TIDAK_ADA_IMPERSONASI` bila
token bukan token impersonasi.

### `GET /auth/me`

Profil akun efektif (sasaran selama impersonasi) + `impersonasi?: ImpersonasiMeta` (ada hanya saat aktif):

```jsonc
// ImpersonasiMeta
{
  "aktif": true,
  "sesi_id": "uuid",
  "pelaku": {
    "id": "uuid",
    "nama": "...",
    "email": "...",
    "peran": "ADMIN_SISTEM",
  },
  "sasaran": { "id": "uuid", "nama": "...", "peran": "OPS_KOORDINATOR" },
  "mulai_pada": "...",
  "kedaluwarsa_pada": "...|null",
  "alasan": "...|null",
}
```

### Klaim JWT access token (dibaca web hanya untuk tampilan; otorisasi tetap di Go)

`sub`, `role`, `permissions`, `name`, `email`, `org`, `kab` = milik SASARAN. Tambahan saat impersonasi:

```jsonc
"imp": { "id": "<impersonasi_sesi.id>", "by": { "user_id": "...", "name": "...", "email": "...", "role": "ADMIN_SISTEM" }, "started_at": 1763370000 }
```

## Pemetaan istilah brief -> kode nyata

| Brief                                                                                     | Repo ini                                                                                                                     |
| ----------------------------------------------------------------------------------------- | ---------------------------------------------------------------------------------------------------------------------------- |
| `apps/api/internal/domain/identity`, `application/auth`, `adapters/inbound/http/identity` | `apps/api/internal/auth/{domain,usecase,delivery/http,routes}`                                                               |
| `wiring/rbac.go`, `rbac-roles.json`, `api-routes.json`                                    | `internal/auth/usecase/rbac*.go`, `packages/contracts/permissions.json`, `packages/contracts/openapi.yaml`                   |
| `(dashboard)/layout.tsx`, `AppTopbar`, `UserMenu`, `NotificationBellDrawer`               | `app/(console)/layout.tsx`, `components/layout/{console-shell,topbar}.tsx` (menu akun dan lonceng ada di dalam `topbar.tsx`) |
| `nav-sections.ts`, `buildNavSections`, `isRouteAllowed`                                   | `lib/nav.ts`, `lib/rbac/routes.ts` (`canAccessRoute`)                                                                        |
| `superadmin`                                                                              | peran `ADMIN_SISTEM`                                                                                                         |
| `impersonation:policy:manage`                                                             | izin `impersonasi.kelola` (format `domain.aksi`)                                                                             |
