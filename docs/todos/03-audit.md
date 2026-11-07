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
- Pemicu `trg_audit_log_append_only` dan `trg_audit_log_no_truncate` dimatikan hanya selama konversi (satu transaksi, Up dan Down) lalu dinyalakan lagi. Tidak ada rantai hash atau kolom turunan dari `id`, jadi konversi tidak merusak verifikasi apa pun; hash berantai (03-C) masih belum ada.

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
   - [ ] Hash berantai (`hash_sebelumnya`, `hash`) dan job verifikasi berkala. Rantai dihitung atas `id` BIGINT berurut (bukan `kode`), jadi urutan identity adalah urutan rantai; migrasi konversi lebih dulu menetapkan ID menurut `waktu`.
   - [ ] Partisi bulanan (`PARTITION BY RANGE (waktu)`) + job pembuat partisi. PK tabel berpartisi harus memuat kolom partisi (`PRIMARY KEY (waktu, id)`), `uq_audit_log_kode` ikut menjadi `UNIQUE (waktu, kode)`, dan rute `:id` perlu `waktu` atau pencarian lintas partisi; putuskan sebelum mulai. BRIN `waktu` tetap dipakai per partisi.
   - [ ] `REVOKE UPDATE, DELETE` dari pengguna DB aplikasi (lihat [19 §7](19-migration-seeder.md)).
2. **API/Ops**
   - [ ] Retensi: lepas partisi lama ke penyimpanan dingin sesuai kebijakan masa simpan (PDP).
   - [ ] Data lineage per record harga: siapa input, validasi, publikasi, versi (PRD §1.4).
