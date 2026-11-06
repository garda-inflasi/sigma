# Skema database: kunci primer, kunci asing, dan indeks

Dokumen ini adalah kontrak skema PostgreSQL Garda Inflasi: cara memilih kunci primer (PK), aturan kunci asing
(FK), strategi indeks, dan cara kode Go, API, BFF, serta seeder memperlakukan ID. Kamus data per tabel ada di
bagian [Matriks tabel](#matriks-tabel).

Sumber kebenaran yang dijalankan mesin:

- **Matriks kategori PK**: `kategoriPK` di [`apps/api/internal/platform/schematest/kunci_test.go`](../apps/api/internal/platform/schematest/kunci_test.go). Tes `TestInvarianKunci` memeriksa invarian di bawah terhadap skema hasil migrasi.
- **Bentuk ID di Go dan JSON**: paket [`apps/api/pkg/idx`](../apps/api/pkg/idx/idx.go).
- **UUID deterministik untuk seed dan migrasi data**: `idx.UUIDStabil` di Go dan `garda_uuid_stabil` di SQL (migrasi `000109_pk_fondasi.sql`), yang diuji sama persis oleh `TestUUIDStabil_SamaDenganSQL`.

## 1. Kategori kunci primer

| Kategori     | Bentuk kolom                                                                              | Dipakai untuk                                                                                                                                                                                                                                                                                                         | Alasan                                                                                                                                                                     |
| ------------ | ----------------------------------------------------------------------------------------- | --------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- | -------------------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| **UUID**     | `id uuid PRIMARY KEY DEFAULT gen_random_uuid()`                                           | Entitas bisnis utama dan identitas yang terlihat klien: `pengguna`, `intervensi`, `policy_brief`, `sumber_data`, `cold_storage`                                                                                                                                                                                       | Tidak bisa ditebak atau dienumerasi dari URL. Bisa dibuat di mana saja tanpa koordinasi. Tidak membocorkan volume atau urutan pembuatan.                                   |
| **BIGINT**   | `id bigint GENERATED ALWAYS AS IDENTITY PRIMARY KEY` (tabel lama boleh tetap `BIGSERIAL`) | Tabel transaksional, time-series, log, dan operasional internal                                                                                                                                                                                                                                                       | Ringkas (8 byte) dan cepat untuk join. Indeks B-tree tumbuh di ujung kanan karena nilainya menaik, jadi insert murah. `ALWAYS` membuat basis data satu-satunya pemberi ID. |
| **KODE**     | Satu kolom kode alami (`varchar`, enum, atau `integer` tahun)                             | Referensi nasional dan master berkode: `provinsi` dan `kabupaten` (kode BPS), `komoditas`, `pasar`, `permissions`, `roles`, `kpi_definisi`, `alur_persetujuan`, `target_inflasi`, `wilayah_geometri` (1:1 dengan kabupaten)                                                                                           | Kode itu sendiri identitasnya, stabil lintas sistem (BPS, PIHPS), dan dipakai langsung sebagai nilai FK yang mudah dibaca.                                                 |
| **HASH**     | Satu kolom hash atau token id                                                             | Tabel keamanan: `jwt_blacklist` (jti), `kunci_login` (hash email), `sso_login_state` (hash state)                                                                                                                                                                                                                     | Kuncinya memang nilai yang dicari. Tidak ada entitas bisnis.                                                                                                               |
| **KOMPOSIT** | Gabungan kolom FK atau induk + urutan                                                     | Tabel penghubung (`role_permissions`, `user_permissions`, `intervensi_pj`, `tugas_entri`, `preferensi_notifikasi`, `pemetaan_sumber`) dan entitas lemah (`alur_persetujuan_langkah`, `intervensi_persetujuan_langkah`), serta definisi atau matriks referensi berkode (`tahap_intervensi_def`, `jarak_antar_wilayah`) | Satu baris per pasangan. Kunci gabungan mencegah duplikat tanpa indeks tambahan.                                                                                           |

Tabel time-series dan agregat yang dulu hanya punya kunci alami gabungan, misalnya `andil_inflasi(periode,
wilayah_kode, komoditas_kode)`, kini memakai PK BIGINT. Kunci alaminya tetap dijaga sebagai `UNIQUE` dan
dipakai sebagai target `ON CONFLICT` untuk upsert.

### Kolom `kode`: kode tampilan dan ID lama

Tabel yang PK-nya diubah dari `VARCHAR` menyimpan ID lamanya di kolom `kode varchar(64)` beserta indeks `UNIQUE`.
Kolom ini punya tiga fungsi:

1. **Kode tampilan** untuk manusia, misalnya `INT-1000`, `PB-2026-10`, `EWS-0901`, `CS-01`, `SRC-BAPANAS`, `TGS-…`.
   Baris baru tetap diberi kode bila modulnya memang menampilkan kode.
2. **Alias tautan lama**: rute entitas UUID yang punya kode menerima UUID maupun kode (`idx.ParamUUIDAtauKode`),
   jadi URL lama seperti `/policy-brief/PB-2026-10` tetap berlaku.
3. **Kunci idempoten seeder** (`ON CONFLICT (kode)`) dan pemulihan ID lama saat migrasi di-rollback.

Baris yang dibuat aplikasi tanpa makna kode boleh berisi `kode NULL`, karena indeks unik mengizinkan banyak NULL.

## 2. Aturan kunci asing

- **Tipe FK sama persis dengan tipe kolom induknya.** FK ke tabel UUID bertipe `uuid` dan FK ke tabel BIGINT bertipe `bigint`. Teks bebas tidak boleh menunjuk surrogate key.
- **Setiap FK punya aturan `ON DELETE` eksplisit.** `NO ACTION` tidak dipakai.

  | Hubungan                                                                                                | Aturan     | Contoh                                                       |
  | ------------------------------------------------------------------------------------------------------- | ---------- | ------------------------------------------------------------ |
  | Anak milik agregat (riwayat, langkah, lampiran, stok milik fasilitas, sesi milik pengguna)              | `CASCADE`  | `intervensi_riwayat.intervensi_id`, `refresh_tokens.user_id` |
  | Jejak pelaku opsional (kolom nullable "oleh siapa") yang tidak boleh menghalangi penghapusan akun       | `SET NULL` | `audit_log.aktor_id`, `ews_riwayat.oleh`                     |
  | Pelaku atau pemilik yang menjadi bukti asal data (penginput, validator, penerbit, penyusun)             | `RESTRICT` | `entri_harga.petugas_id`, `entri_harga.divalidasi_oleh`      |
  | Rujukan ke master atau referensi (`kabupaten`, `komoditas`, `pasar`, `sumber_data`) dan antar-transaksi | `RESTRICT` | `entri_harga.komoditas_kode`, `transaksi_stok.lot_id`        |

  Akun yang masih dirujuk sebagai bukti asal data tidak bisa dihapus. API menjawab `409` dan menyarankan
  menonaktifkan akun.

- **Setiap FK punya indeks** yang kolom awalnya sama dengan kolom FK. Indeks ini mencegah table scan saat join
  dan saat baris induk dihapus atau diubah. Indeks yang sudah diawali kolom FK, misalnya indeks gabungan
  `(kabupaten_kode, tanggal)`, sudah mencukupi.

## 3. Strategi indeks

- **Penamaan**: `idx_<tabel>_<kolom…>` untuk indeks biasa dan `uq_<tabel>_<kolom…>` untuk unik. Indeks parsial diberi akhiran keadaan, misalnya `idx_entri_harga_menunggu`.
- **Indeks FK**: wajib, lihat bagian 2.
- **Gabungan filter + urutan**: mengikuti pola kueri nyata di repositori. Kolom kesetaraan diletakkan dulu, lalu kolom urutan dengan arah yang sama dengan `ORDER BY`. Contoh: `(komoditas_kode, pasar_id, dicatat_pada DESC)`, `(status, dibuat_pada DESC)`.
- **Parsial untuk antrean**: baris yang menunggu tindakan, misalnya entri `DIAJUKAN`, tugas `DITUGASKAN`, alert `AKTIF`, persetujuan `MENUNGGU`, perubahan master `MENUNGGU`. Indeksnya kecil dan selalu panas.
- **BRIN untuk time-series append-only bervolume besar**: `entri_harga(dicatat_pada)`, `telemetri_suhu(waktu)`, `audit_log(waktu)`, `data_mentah(diambil_pada)`, `harga_eksternal(tanggal)`. Ukurannya sangat kecil dan efektif karena baris masuk kira-kira urut waktu. B-tree untuk kueri titik dan urutan halaman tetap dipertahankan.

## 4. Kode Go

| Kategori     | Tipe Go                        | Tag GORM                                                          |
| ------------ | ------------------------------ | ----------------------------------------------------------------- |
| UUID         | `string` (kanonik huruf kecil) | `gorm:"column:id;type:uuid;primaryKey;default:gen_random_uuid()"` |
| BIGINT       | `idx.ID`                       | `gorm:"column:id;primaryKey;autoIncrement"`                       |
| FK ke UUID   | `string` / `*string`           | `gorm:"column:x_id;type:uuid"`                                    |
| FK ke BIGINT | `idx.ID` / `*idx.ID`           | `gorm:"column:x_id"`                                              |

- **Kolom UUID nullable** dipetakan ke `*string`, dan "tidak ada pelaku" ditulis sebagai `NULL`, bukan string kosong. PostgreSQL menolak `''` untuk kolom `uuid`.
- **Repositori** membaca, mengubah, dan menghapus berdasarkan PK (`FindByID`, `Update`, `Delete`). Daftar yang menampilkan relasi memakai `JOIN` atau `Preload` yang terindeks, bukan kueri per baris (N+1).
- **Handler** membaca `:id` lewat `idx.ParamID`, `idx.ParamUUID`, atau `idx.ParamUUIDAtauKode`. ID yang salah bentuk dijawab `404`, sama dengan ID yang tidak ada, sehingga teks bebas tidak pernah sampai ke kolom `uuid`. Pelanggaran FK saat menghapus (`23503`) dijawab `409`.
- **Seed `GENERATED ALWAYS`**: kolom identity tidak boleh diisi. Tes dan seeder tidak menulis `id` BIGINT sendiri, kecuali memakai `OVERRIDING SYSTEM VALUE` di fixture SQL yang memang membutuhkannya.

## 5. API, BFF, dan web

- **Semua ID di JSON adalah string**: UUID kanonik, atau angka desimal untuk BIGINT (`"42"`, mengikuti pemetaan JSON proto3 untuk int64). Klien memperlakukan ID sebagai teks tak bermakna, sehingga tipe `id: string` di web tidak berubah dan angka di atas 2^53 tetap utuh. Saat membaca, API juga menerima angka JSON.
- **Rute** memakai bentuk `/v1/<sumber>/:id`. Entitas UUID yang punya kode tampilan juga menerima kode di posisi yang sama sebagai alias.
- **Kode tampilan** dikirim di field `kode`. UI menampilkan `kode`, bukan `id`.
- **Pemetaan BFF** meneruskan ID apa adanya. BFF tidak mengurai atau mengubah format ID.

## 6. Seeder

- **Idempoten** terhadap kunci alami yang unik, seperti `email` untuk pengguna, `kode` untuk master dan entitas berkode, atau kunci alami tabel agregat. Seeder memakai `ON CONFLICT (<kunci alami>) DO UPDATE` atau `DO NOTHING`, tidak pernah `ON CONFLICT (id)` untuk identity.
- **UUID baris seed** memakai `idx.UUIDStabil("<tabel>", "<kode lama>")`, misalnya `idx.UUIDStabil("pengguna", "superadmin")`. Basis data baru dan basis data yang dimigrasi memakai UUID yang sama, sehingga rujukan antar-seed langsung memakai nilainya.
- **BIGINT baris seed** diberi basis data. Seeder anak mencari ID induknya lewat `kode`, misalnya `(SELECT id FROM entri_harga WHERE kode = ?)`.
- **Menjalankan seeder dua kali** (`make db/seed/reference`, `make db/seed/demo`) tidak menambah baris.

## 7. Migrasi data dari ID lama

1. **VARCHAR → UUID**:
   - `id_baru = garda_uuid_stabil('<tabel>', id_lama)`.
   - Semua kolom FK anak diperbarui lewat pemetaan yang sama di dalam satu transaksi.
   - ID lama disimpan di `kode` untuk entitas berkode.
   - Untuk `pengguna`, ID lama disimpan di tabel pemetaan `pengguna_id_lama` supaya Down bisa memulihkannya.
2. **VARCHAR → BIGINT**:
   - ID baru diberikan berurutan menurut waktu pembuatan, lalu ID lama sebagai pemutus seri.
   - Kolom dijadikan `GENERATED ALWAYS AS IDENTITY` dan urutannya diset ke `max(id) + 1`.
   - ID lama disimpan di `kode` (UNIQUE), dan FK anak diperbarui lewat kode itu.
3. **Kunci alami gabungan → BIGINT + UNIQUE**: kolom `id` identity ditambahkan sebagai PK, dan kunci gabungan lama menjadi constraint `UNIQUE`.
4. **Down** mengembalikan tipe dan nilai lama, memakai `kode` atau tabel pemetaan. `TestMigrationsApplyUpDownUp` memastikan siklus naik-turun-naik berjalan.
5. **Tes migrasi data**: setiap migrasi konversi punya tes integrasi. Tes itu menjalankan migrasi sampai versi sebelumnya, mengisi baris berbentuk data lama, menjalankan migrasi, lalu memeriksa ID baru, kode lama, FK anak, dan rollback-nya.

## Matriks tabel

Kolom "FK utama" dan "Indeks utama" diisi oleh migrasi konversi masing-masing modul (`000110`–`000115`). Daftar
lengkap FK dan indeks selalu bisa diturunkan dari katalog (`\d+ <tabel>` atau kueri di `kunci_test.go`).

| Tabel                                       | PK  | Alasan | FK utama | Indeks utama |
| ------------------------------------------- | --- | ------ | -------- | ------------ |
| _diisi bertahap oleh migrasi 000110–000115_ |     |        |          |              |
