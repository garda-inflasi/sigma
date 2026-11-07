# 03 — Audit trail

**Prioritas:** P0 (PRD §5.2) · **Status:** UI ✅ · Backend ✅ · Terhubung ✅ · Skema & seed ✅ · Tes ✅
**Cara kerja & DoD:** [00-cara-kerja.md](00-cara-kerja.md) · **Konteks data:** [19](19-migration-seeder.md)

## Yang sudah ada

- **UI:** [audit-page.tsx](../../apps/web/src/components/audit/audit-page.tsx) di `/audit`, memanggil `GET /v1/audit` dengan filter server dan paginasi cursor lewat [audit.ts](../../apps/web/src/lib/api/audit.ts).
- **API Go:** `GET /v1/audit` (izin `audit.lihat`). Paket `internal/audit` menyediakan `Writer` (disuntikkan ke auth dan cold-storage) dan daftar dengan filter server + cursor.
- **Skema:** `audit_log` (000002): `id`, `waktu`, `aktor_id` FK, `aksi`, `entitas`, `entitas_id`, `hasil`, `ip`, `metadata` JSONB. Struct `AuditLog` kini satu (`internal/audit/domain`); seeder `audit.demo` mengisi contoh.

### Kunci primer (000115, ✅)

- `audit_log.id` kini **BIGINT `GENERATED ALWAYS AS IDENTITY`** (dulu VARCHAR berisi UUID teks buatan aplikasi; `uuid.NewString()` di `usecase.buildRow` dihapus, basis data yang memberi ID). ID lama disimpan di `kode varchar(64)` (UNIQUE, NULL untuk baris baru): alias rute dan kunci idempotensi seeder demo (`demo-audit-NNNN`, `ON CONFLICT (kode) DO NOTHING`). ID baru berurut menurut `waktu`, lalu ID lama.
- **Rute `GET /v1/audit/:id`** memakai `idx.ParamIDAtauKode` (maks 64 karakter): ID BIGINT atau kode lama; angka murni selalu dibaca sebagai ID; ID salah bentuk (`0`, `-5`, `007`, kode > 64 karakter) dijawab **404**. JSON `id` berupa string desimal (`"4301"`) dan baris lama memuat `kode`.
- **Paginasi dan ekspor** memakai cursor `(waktu, id)` dengan `id` numerik (urutan `waktu DESC, id DESC`, indeks `idx_audit_log_waktu_id`), jadi urutan antar-baris berwaktu sama mengikuti urutan penyisipan, bukan leksikografis. Cursor lama (berisi UUID) dijawab 400; klien memulai dari halaman pertama. CSV tidak berubah.
- `entitas_id` tetap **rujukan polimorfik berupa teks, bukan FK**: berisi ID baru entitasnya (UUID atau angka). Baris lama dipetakan oleh migrasi per entitas (000110 `pengguna`, 000115 `sumber_data`, `langganan_push`, `preferensi_notifikasi`); nilai yang bukan ID (`ekspor`, `3 sumber`, rentang periode KPI) tidak disentuh.
- `aktor_id` uuid → `pengguna` **SET NULL** (000110). Indeks: **BRIN `waktu`**, `(waktu DESC, id DESC)` untuk cursor, `(entitas, entitas_id, waktu DESC)` untuk riwayat per entitas, `(aktor_id, waktu DESC)`, `(aksi, waktu DESC)`; `(entitas, entitas_id)` dan `(waktu)` dibuang karena redundan.
- Pemicu `trg_audit_log_append_only` dan `trg_audit_log_no_truncate` dimatikan hanya selama konversi (satu transaksi, Up dan Down) lalu dinyalakan lagi. Saat 000115 belum ada rantai hash; sejak 000116 migrasi tidak boleh lagi mengubah isi baris audit (lihat 03-C).

---

## 03-A — Satu penulis audit + jejak yang tak bisa diubah _(Fase P, urutan #3)_

**Tujuan:** setiap aksi sensitif menulis tepat satu baris audit lewat satu komponen, dan baris itu tidak bisa diubah/dihapus. **Aturan baru untuk semua slice berikutnya:** slice yang menambah aksi yang mengubah data wajib memanggil penulis audit ini (masuk DoD).
**Bergantung pada:** 02-A.

1. **MIG**
   - [x] `audit_log`: tambah `aktor_peran`, `user_agent`, `correlation_id`, `sebelum` JSONB, `sesudah` JSONB.
   - [x] **Append-only:** trigger `BEFORE UPDATE OR DELETE` yang melempar galat.
   - [x] Index `(aktor_id, waktu DESC)`, `(entitas, entitas_id, waktu DESC)`, `(aksi, waktu DESC)`.
   - [x] Keputusan FK `aktor_id`: `ON DELETE SET NULL` + snapshot `aktor_nama/aktor_email/aktor_peran`. Trigger append-only tetap mengizinkan pelepasan `aktor_id` oleh aksi FK itu sendiri (`pg_trigger_depth() > 1`), bukan oleh UPDATE langsung.
2. **SEED**
   - [x] Lapis `demo`: contoh log deterministik (login, validasi harga, aksi massal, perubahan peran), aktor = akun demo ([01-A](01-auth.md)), ditandai demo. Produksi: tidak ada seed.
3. **API**
   - [x] Hapus dua struct `AuditLog` duplikat; satu paket `internal/audit` dengan antarmuka `Writer` yang disuntikkan ke modul lain.
   - [x] Pasang pada: login/logout/gagal login, perubahan pengguna dan peran, aksi massal cold-storage. Modul lain menambah panggilan di slice masing-masing.
   - [x] `GET /v1/audit`: filter sisi server (`dari`, `sampai`, `aktor`, `aksi`, `entitas`, `hasil`) + paginasi cursor; batas default tidak lagi 500 sekaligus.
4. **FE**
   - [x] `audit-page` memakai filter server (bukan hanya filter klien atas 500 baris); sesuaikan `toAuditRow` bila bentuk respons berubah.
5. **TES**
   - [x] Aksi sensitif menulis tepat satu baris; `UPDATE`/`DELETE` pada `audit_log` ditolak; peran tanpa `audit.lihat` mendapat 403.
6. **DoD** — checklist [00 §4](00-cara-kerja.md).

---

## 03-B — Lengkapi tampilan dan ekspor _(Fase F, gelombang F-2)_

1. **API**
   - [x] `GET /v1/audit/export`: CSV (titik koma, BOM) mengikuti filter yang sama dengan daftar, maksimum 20.000 baris (`X-Export-Truncated`), sel anti-injeksi rumus, dan dicatat sebagai `EKSPOR_DATA`.
   - [x] `GET /v1/audit/:id`: entri lengkap + `perubahan` (diff `sebelum`/`sesudah` per bidang, dihitung di `domain.DiffState`).
   - [x] Filter lanjutan: `entitas_id`, `peran`, `correlation_id`, dan pencarian bebas `q`. Tanpa migrasi (index `audit_log` yang ada cukup untuk volume sekarang).
2. **FE**
   - [x] Pencarian `q`, rentang tanggal (sudah ada), panel detail dengan diff, filter "jejak objek ini" / "satu permintaan", tombol "Ekspor CSV sesuai filter".
   - [x] Tautan ke halaman terkait objek (halaman daftar; belum ada deep-link ke satu baris karena halaman tujuan belum mendukungnya).
   - [x] Filter awal `/audit` dari kueri URL (`entitas`, `entitas_id`, `aksi`, `aktor`, `hasil`, `q`, `dari`, `sampai`, `correlation_id`), dipakai tautan "Lihat di audit trail" pada riwayat peran pengguna ([02](02-pengguna-rbac.md)). Aksi `RESET_MFA` (reset MFA oleh admin) ditambahkan ke pilihan aksi.
3. **TES**
   - [x] Ekspor sesuai filter, tercatat sebagai `EKSPOR_DATA`, anti-injeksi rumus, 401/403/400; detail + diff; filter lanjutan; unit `DiffState` dan paginasi ekspor.

---

## 03-C — Integritas dan retensi _(Fase F, kebutuhan kepatuhan)_

1. **MIG**
   - [x] Hash berantai (`hash_sebelumnya`, `hash`) dan job verifikasi berkala (000116). Rantai dihitung atas `id` BIGINT berurut (bukan `kode`): urutan identity adalah urutan rantai.
     - **Format garda-audit-v1:** SHA-256 atas `hash_sebelumnya` ditambah isi baris. Kolom `aktor_id` tidak ikut karena FK `SET NULL` boleh mengosongkannya; pelaku tetap terlindungi lewat snapshot `aktor_nama/email/peran`.
     - **Dua implementasi:** pemicu SQL `audit_log_hitung_hash` dan Go `domain.HitungHash`. Keduanya dikunci vektor uji yang sama dan dicocokkan di tes integrasi.
     - **Urutan rantai:** pemicu `BEFORE INSERT FOR EACH STATEMENT` mengambil advisory lock sebelum identity dibagikan, jadi urutan id = urutan commit. Baris yang dilewati `ON CONFLICT` tidak memutus rantai. Penulisan dalam transaksi REPEATABLE READ atau SERIALIZABLE ditolak.
     - **Baris lama:** diberi hash urut id saat migrasi.
     - **Verifikasi:** job worker `audit_verifikasi` (24 jam) menghitung ulang seluruh rantai di Go dalam satu snapshot dan menyimpan hasilnya di `audit_log_verifikasi`. Jenis masalah: `ISI_BERUBAH`, `RANTAI_PUTUS`, `ARSIP_TIDAK_COCOK`. Hasil RUSAK dicatat sebagai `VERIFIKASI_AUDIT` dan membuat job tampil gagal di `/healthz`.
     - **API dan UI:** `GET /v1/audit/integritas` (izin `audit.lihat`) ditampilkan sebagai kartu "Integritas" di `/audit`. Detail entri kini memuat `hash` dan `hash_sebelumnya`.
   - [x] Partisi bulanan (`PARTITION BY RANGE (waktu)`) dan job pembuat partisi (000117).
     - **Kunci:** PK `(waktu, id)`, kategori `PKBigintPartisi` di `kunci_test.go`.
     - **Keputusan rute `:id`:** pencarian lintas partisi lewat `idx_audit_log_id`, tanpa `waktu` di URL. Alias `kode` lewat indeks parsial `idx_audit_log_kode`.
     - **Keputusan `kode`:** bukan `UNIQUE (waktu, kode)`. Keunikan dijaga global oleh pemicu `trg_audit_log_kode_unik` yang berjalan di bawah kunci rantai, jadi bebas balapan. Seeder demo menyaring kode yang sudah ada, bukan `ON CONFLICT`.
     - **Indeks:** `idx_audit_log_waktu_id` dibuang karena digantikan PK yang dipindai mundur. BRIN `waktu` tetap per partisi.
     - **Tanpa partisi DEFAULT.** Migrasi menyiapkan bulan lalu sampai +3. Job `audit_partisi` menjaga jendela ke depan (`AUDIT_PARTISI_KE_DEPAN`). Penulis audit membuat partisi bulannya sendiri bila belum ada, termasuk baris bertanggal mundur, kecuali bulan yang sudah diarsipkan.
     - **Partisi baru:** dibuat terpisah lalu di-ATTACH, sehingga baca dan tulis audit tidak berhenti. Tiap partisi punya pemicu tolak-TRUNCATE sendiri dan tidak bisa diakses langsung.
     - **Sisipan SQL mentah** ke bulan tanpa partisi butuh `audit_log_siapkan_partisi` lebih dulu (fixture tes diperbarui).
   - [x] `REVOKE UPDATE, DELETE` dari pengguna DB aplikasi (000119, [19 §7](19-migration-seeder.md)).
     - **Peran grup `garda_aplikasi` (NOLOGIN):** DML pada data, sequence, dan fungsi pemeliharaan; tanpa DDL dan tanpa TRUNCATE. Tabel jejak hanya SELECT/INSERT: `audit_log` dan partisinya, `audit_log_verifikasi`, `audit_log_arsip*`, `entri_harga_versi`. `goose_db_version` tertutup.
     - **Login aplikasi:** `cmd/migrate` membuat atau memperbarui login `DB_APP_USER`/`DB_APP_PASSWORD`. Kata sandi dikirim sebagai verifier SCRAM yang dihitung di Go. Compose menghubungkan api dan worker dengan login itu; migrate dan seed tetap memakai pemilik skema; api tidak lagi menerima `DATABASE_URL` pemilik.
     - **REFRESH dashboard** lewat `garda_segarkan_mv` (SECURITY DEFINER).
     - **Tanpa `DB_APP_*`:** perilaku lama tetap (pemicu append-only melindungi) dan API mencatat peringatan saat start.
     - **Bukti:** `internal/platform/migrate/pengguna_aplikasi_test.go` login sebagai pengguna aplikasi. Ubah atau hapus jejak, DDL, partisi langsung, dan tabel migrasi ditolak `42501`; tulis audit, refresh dashboard, verifikasi, dan retensi tetap jalan.
2. **API/Ops**
   - [x] Retensi: lepas partisi lama ke penyimpanan dingin sesuai kebijakan masa simpan (PDP).
     - **Kebijakan:** `AUDIT_RETENSI_BULAN` (12–1200, kosong = mati; usulan kebijakan 60).
     - **Ekspor:** job `audit_partisi` mengekspor partisi yang seluruhnya lebih tua ke `AUDIT_ARSIP_DIR` sebagai `jsonl.gz` dan manifest. Hash tiap baris diperiksa ulang, dan baris yang rusak tidak diarsipkan.
     - **Pelepasan:** `audit_log_lepas_partisi` (SECURITY DEFINER, minimal 12 bulan) memeriksa ulang jumlah baris dan ringkasan hash di bawah kunci. Fungsi itu mencatat `audit_log_arsip` dan segmen rantainya, lalu melakukan DETACH dan DROP. Kepala rantai tidak pernah diarsipkan.
     - **Sambungan rantai:** verifikasi berikutnya menyambung rantai lewat segmen arsip, termasuk id yang menyelang di batas bulan.
     - **Pemeriksaan luring:** `worker verifikasi-arsip <manifest>` memeriksa berkas tanpa basis data. Manifest yang belum sempat ditulis dilengkapi pada putaran berikutnya. Setiap pelepasan dicatat sebagai `ARSIP_AUDIT`.
     - **Batas:** `goose down` 000117 ditolak bila sudah ada arsip. Ops tercatat di [DEPLOYMENT §10b](../DEPLOYMENT.md).
   - [x] Data lineage per record harga: siapa input, validasi, publikasi, versi (PRD §1.4). Lihat [06](06-harga-validasi.md).
     - **Tabel `entri_harga_versi` (000118):** append-only, ditulis pemicu pada setiap perubahan isi `entri_harga`. Pelaku diambil dari GUC transaksi `garda.aktor_id` atau kolom pelakunya. Mengulang seeder tidak menambah versi.
     - **API:** `GET /v1/harga/:id/versi` dan `GET /v1/harga/:id/lineage` (asal input, validasi otomatis, validasi/publikasi/penolakan, versi, riwayat status, rujukan audit).
     - **UI:** panel "Lineage data" di detail harga.

**Belum / di luar 03-C:**

- Jangkar eksternal hash kepala rantai (mis. dikirim ke log terpisah atau penyimpanan WORM). Saat ini rantai mendeteksi perubahan oleh siapa pun yang tidak menghitung ulang seluruh rantai sesudahnya, dan setiap verifikasi menyimpan hash kepala.
- Salinan otomatis volume arsip ke luar VM masih langkah ops, sama seperti backup.
