# 00 — Cara kerja & urutan task

Baca berkas ini lebih dulu. Ia menjawab: **dalam urutan apa task dikerjakan, apa isi satu task, dan kapan sebuah task dianggap selesai.**

## 1. Prinsip: satu task = satu _vertical slice_

Setiap task menembus semua lapisan sampai halaman web menampilkan data dari database, dengan urutan tetap:

```
MIG → SEED → API → FE → TES → DoD
```

| Langkah  | Isi                                                                                                                                     | Keluaran yang bisa dicek                                               |
| -------- | --------------------------------------------------------------------------------------------------------------------------------------- | ---------------------------------------------------------------------- |
| **MIG**  | Migration SQL baru untuk task ini. **Dibuat saat task dikerjakan**, bukan direncanakan di muka.                                         | `make db/migrate` dan `down`/`up` berhasil pada DB kosong              |
| **SEED** | Seeder referensi/demo untuk tabel yang disentuh                                                                                         | `make db/seed/demo` dijalankan dua kali: jumlah baris sama             |
| **API**  | Entity, repository, usecase, handler, route Go; izin yang benar; OpenAPI                                                                | `go test` hijau; endpoint 200 untuk peran berhak, 403 untuk yang tidak |
| **FE**   | Sambungkan `lib/api/endpoints.ts` ke API, cocokkan bentuk respons dengan `lib/data/types.ts`, buang import mock/data statis di komponen | Halaman tampil dari DB (bukan mock)                                    |
| **TES**  | Tes yang mengunci perilaku slice                                                                                                        | Tes baru + tes kontrak skema                                           |
| **DoD**  | Checklist §4 di bawah                                                                                                                   | Semua tercentang → commit                                              |

Mengapa migration di depan: bentuk tabel menentukan entity, DTO, dan tipe di frontend. Mengubahnya belakangan berarti mengerjakan ulang tiga lapisan.

## 2. Dua fase

- **Fase P (Paritas):** setiap halaman yang sekarang memakai mock dibuat tampil **dari API + database dengan data seed demo**, setara dengan mode mock. Semua task berakhiran **-A**. Hasil akhirnya: `NEXT_PUBLIC_API_BASE_URL=/api` bisa dinyalakan tanpa halaman yang error atau kosong. Ini setara target Tahap 1 PRD.
- **Fase F (Fitur nyata):** logika yang membuat data itu nyata, bukan sekadar tampil: state machine harga, validasi otomatis, rule engine EWS, ingestion, model prediksi, LLM, workflow engine. Task berakhiran **-B, -C, -D**. Ini Tahap 2 dan 3 PRD.

Kerjakan seluruh Fase P dulu. Alasannya: setiap fitur Fase F membutuhkan tabel, seed, dan halaman yang sudah hidup.

## 3. Aturan migration dan seeder per task

**Migration**

- [ ] Nomor **diambil saat dibuat**, berurutan dari nomor terbesar yang ada: `goose -dir db/migrations create <nama> sql -s` (sekarang terakhir `000007`, jadi berikutnya `000008`). Jangan memberi nomor di dokumen ini; nomor di dokumen akan basi.
- [ ] Nama: `NNNNNN_<id-task>_<perubahan>.sql`, mis. `000008_06a_entri_harga_kolom_waktu.sql`. Satu task boleh punya lebih dari satu migration, tetapi satu migration tidak melayani dua task.
- [ ] Wajib `Up` dan `Down`; aman diulang (`IF NOT EXISTS`); **tanpa data demo dan tanpa `DELETE`/`TRUNCATE`**.
- [ ] Jangan mengubah migration yang sudah dipakai. Koreksi = migration baru. (Pengecualian satu-satunya: task `F1`, lihat §5.)
- [ ] Bila bentrok nomor (dua cabang), renumber saat merge.
- [ ] Aturan lengkap (pola tabel besar, `ALTER TYPE`, lint) ada di [19 §3](19-migration-seeder.md).

**Seeder**

- [ ] Letakkan di `apps/api/internal/<modul>/seeder/` (pola `cold-storage`), daftarkan ke registry seeder (task F0-4) dengan **lapis** (`reference` | `demo` | `admin`) dan **dependensi**.
- [ ] Idempoten (upsert) dan deterministik (PRNG seed tetap; tanggal relatif `now`).
- [ ] Bentuk dan jumlah data demo meniru `lib/api/mock.ts` supaya UI tampil sama. Tambahkan tes yang membandingkan jumlah/ID kunci.
- [ ] Lapis `demo` menolak berjalan bila `APP_ENV=production`.

## 4. Definition of Done (berlaku untuk setiap task)

- [ ] `make db/migrate` pada DB **kosong** berhasil; `down` lalu `up` berhasil.
- [ ] **Tes kontrak skema** (F0-3) lulus dan modul task ini sudah **dihapus dari daftar `belumSelaras`**. Ini penanda resmi bahwa entity cocok dengan migrasi.
- [ ] `make db/seed/reference db/seed/demo` dijalankan **dua kali**: tidak ada galat, jumlah baris tidak bertambah.
- [ ] Endpoint utama task: **200 dan data tidak kosong** untuk peran yang berhak, **403** untuk peran yang tidak berhak (tes integrasi dengan token nyata).
- [ ] Web dijalankan dengan `NEXT_PUBLIC_API_BASE_URL=/api` dan halaman task tampil dari DB. Tidak ada lagi `mockXxx`/`lib/data/*` statis dipakai komponen untuk data fitur ini.
- [ ] `pnpm typecheck` dan `pnpm test` (web) serta `go vet ./...` dan `go test ./...` (API) hijau.
- [ ] Setiap aksi yang **mengubah data** memanggil penulis audit ([03-A](03-audit.md)); berlaku untuk slice setelah 03-A.
- [ ] OpenAPI diperbarui untuk endpoint yang ditambah/diubah.
- [ ] Item yang selesai dicentang di berkas fitur, dan baris status di [README](README.md) diperbarui.
- [ ] Satu commit per task, format `feat(<fitur>): <ringkas>` (mengikuti gaya riwayat commit).

**Cara memverifikasi di lokal (setelah F0-1):**

```
make docker/infra/up          # Postgres, Redis
make db/migrate
make db/seed/reference && make db/seed/demo
make dev/api                  # terminal 1
make dev/web                  # terminal 2 (NEXT_PUBLIC_API_BASE_URL=/api di apps/web/.env.local)
```

Login dengan akun demo (peran sesuai halaman yang diuji).

## 5. Pola integrasi frontend ↔ API (keputusan, bisa Anda ubah)

Kode sekarang punya dua pola:

| Pola         | Jalur                                                                                                                   | Dipakai oleh                                      |
| ------------ | ----------------------------------------------------------------------------------------------------------------------- | ------------------------------------------------- |
| **BFF**      | Browser → `/api/<fitur>` (Next Route Handler) → Go `/v1/*`, dengan CSRF double-submit                                   | auth, pengguna, rbac, cold-storage                |
| **Langsung** | Browser → `/api/v1/*` (Caddy meneruskan ke Go) dengan cookie `garda_access` (Go membaca cookie sebagai cadangan Bearer) | sisanya lewat `lib/api/endpoints.ts` (`apiFetch`) |

**Default untuk task baru: pola Langsung** (cukup `endpoints.ts`), karena tanpa menulis route handler per fitur. Syaratnya dipenuhi di task **F0-1** (proxy dev) dan **F0-7** (proteksi CSRF/Origin di Go untuk request ber-cookie yang mengubah data; saat ini Go tidak memeriksa CSRF dan hanya mengandalkan `SameSite=Lax`).

Pakai **BFF** bila: perlu agregasi di sisi server, ada rahasia yang tidak boleh ke browser, atau unggah/ekspor berkas.

Selaraskan bentuk respons di satu tempat (mapper `toXxxRow` seperti `toAuditRow`, atau tipe `*Wire` seperti cold-storage), bukan di komponen.

## 6. Daftar task berurutan

### Fondasi (F0) — kerjakan pertama, kecil semuanya

| ID       | Task                                                                                                                                                                                                         | MIG?  | Berkas                                                 |
| -------- | ------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------ | ----- | ------------------------------------------------------ |
| **F0-1** | Dev proxy: Next `rewrites` `/api/v1/:path*` → Go agar `make dev/web` + `make dev/api` jalan tanpa Caddy; target Makefile `db/seed/reference`, `db/seed/demo`, `db/seed/admin`; README akar berisi alur lokal | tidak | [18](18-platform-lintas-fitur.md)                      |
| **F0-2** | Seragamkan domain email akun demo; 4 tes cold-storage hijau                                                                                                                                                  | tidak | [13](13-cold-storage.md), [19](19-migration-seeder.md) |
| **F0-3** | **Tes kontrak skema** (entity ↔ migrasi) dengan daftar `belumSelaras` berisi semua modul yang kini tidak cocok; ganti `splitGoose` dengan library goose                                                      | tidak | [19](19-migration-seeder.md)                           |
| **F0-4** | **Seeder kit:** registry + lapis + dependensi + guard `APP_ENV`; `cmd/seed reference\|demo\|admin`; pindahkan seeder cold-storage ke registry                                                                | tidak | [19](19-migration-seeder.md)                           |
| **F0-5** | Tes asap: setelah reference+demo, tiap endpoint GET utama web mengembalikan data (daftar modul bertambah seiring slice)                                                                                      | tidak | [19](19-migration-seeder.md)                           |
| **F0-6** | Migrasi nyata saat deploy (service/image `migrate`, gagal keras) — **gerbang produksi**                                                                                                                      | tidak | [19](19-migration-seeder.md)                           |
| **F0-7** | Proteksi CSRF/Origin di Go untuk request ber-cookie; perbaiki cookie `Secure` (kode sekarang `isDev = true` permanen)                                                                                        | tidak | [18](18-platform-lintas-fitur.md)                      |

### Fase P — paritas mock (urutan eksekusi)

Urutan mengikuti dependensi data (pengguna → master → harga → turunannya).

| #   | ID       | Task                                                                        | MIG?       | Bergantung pada        |
| --- | -------- | --------------------------------------------------------------------------- | ---------- | ---------------------- |
| 1   | **01-A** | Akun lewat seeder (admin awal + demo), `wajib_ganti_password`               | ya         | F0-4                   |
| 2   | **02-A** | Putuskan sumber izin (D3), perbaiki nama izin route                         | ya         | 01-A                   |
| 3   | **03-A** | Satukan `AuditLog`, penulis audit, append-only                              | ya         | 02-A                   |
| 4   | **04-A** | Master data: seed referensi, halaman dari API                               | ya (kecil) | 03-A                   |
| 5   | **F1**   | Netralkan seed di migrasi 000006 (gerbang produksi)                         | ya         | 01-A, 02-A, 04-A       |
| 6   | **06-A** | Harga: selaraskan entity, seed demo, halaman dari API                       | ya         | 04-A                   |
| 7   | **07-A** | Lapangan: tugas (selaraskan, seed, halaman)                                 | ya         | 06-A                   |
| 8   | **08-A** | Dashboard operasional: siklus, agregat harga, MV KPI/heatmap                | ya         | 06-A                   |
| 9   | **10-A** | EWS: selaraskan, aturan, seed, halaman                                      | ya         | 04-A                   |
| 10  | **14-A** | Intervensi: selaraskan, definisi tahap, seed, halaman                       | ya         | 10-A                   |
| 11  | **13-A** | Cold storage: pindahkan ke registry seeder (sudah hampir selesai)           | tidak      | F0-4                   |
| 12  | **12-A** | Peta: `wilayah`, neraca, rute/kandidat demo                                 | ya         | 06-A, 13-A             |
| 13  | **05-A** | Sumber data: selaraskan, katalog, halaman                                   | ya         | 04-A                   |
| 14  | **09-A** | Dashboard eksekutif: tabel inflasi/neraca/fiskal + seed berlabel demo       | ya         | 08-A                   |
| 15  | **15-A** | Policy brief: selaraskan, seed, `Generate` yang valid                       | ya         | 02-A                   |
| 16  | **16-A** | KPI: definisi/target/nilai + job hitung                                     | ya         | 06-A, 10-A, 13-A, 14-A |
| 17  | **11-A** | Prediksi: arahkan ke `prediksi_harga`, registry, seed demo, buang `mockXai` | ya         | 06-A                   |

Setelah #17: **tahap kelengkapan Fase P** — `NEXT_PUBLIC_API_BASE_URL=/api` bisa dinyalakan; semua halaman tampil dari DB.

### Fase F — fitur nyata (urutan disarankan; sesuaikan prioritas bisnis)

| Gelombang                             | Task                                                                                                                            |
| ------------------------------------- | ------------------------------------------------------------------------------------------------------------------------------- |
| **F-1 Alur data lapangan yang benar** | 07-B (POST harga idempoten + 409) → 06-B (state machine + riwayat) → 06-C (validasi otomatis) → 01-B/01-C (reset password, MFA) |
| **F-2 Peringatan dan tindakan**       | 10-B (rule engine) → 10-C (SSE) → 14-B (izin per tahap + riwayat) → 03-B (filter audit server)                                  |
| **F-3 Data eksternal**                | 05-B (framework ingestion + PIHPS) → 05-C (BMKG, BPS, bronze/silver/gold) → 08-B → 09-B                                         |
| **F-4 AI**                            | 11-B (baseline model + layanan ML) → 11-C (MLOps, simulasi) → 10-D (notifikasi) → 15-B (generator brief + LLM)                  |
| **F-5 Fisik dan optimasi**            | 14-C (workflow engine) → 13-B (telemetri, transaksi) → 12-B/12-C (geometri, optimasi rute) → 16-B                               |

## 7. Gerbang produksi (blokir deploy pertama)

Belum boleh deploy ke produksi sebelum semuanya tercentang:

- [x] **F0-6** migrasi nyata saat deploy.
- [ ] **F1** seed di migrasi dinetralkan (tidak ada akun demo/`DELETE role_permissions` di DB produksi).
- [ ] **01-A** admin awal dari env dengan `wajib_ganti_password`.
- [x] **F0-7** cookie `Secure` dan proteksi CSRF.
- [x] Seeder `demo` terbukti menolak `APP_ENV=production`.
- [ ] Backup dan uji restore ([19 §7](19-migration-seeder.md)).
