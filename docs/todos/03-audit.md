# 03 — Audit trail

**Prioritas:** P0 (PRD §5.2) · **Status:** UI ✅ · Backend 🟡 · Terhubung ✅ · Skema & seed 🟡 · Tes ⬜
**Cara kerja & DoD:** [00-cara-kerja.md](00-cara-kerja.md) · **Konteks data:** [19](19-migration-seeder.md)

## Yang sudah ada

- **UI:** [audit-page.tsx](../../apps/web/src/components/audit/audit-page.tsx) di `/audit`, memanggil `GET /v1/audit?limit=500` lewat [audit.ts](../../apps/web/src/lib/api/audit.ts).
- **API Go:** `GET /v1/audit` (izin `audit.lihat`). Modul `audit` hanya 2 fungsi usecase dan repo 24 baris.
- **Skema:** `audit_log` (000002): `id`, `waktu`, `aktor_id` FK, `aksi`, `entitas`, `entitas_id`, `hasil`, `ip`, `metadata` JSONB. Kolom entity cocok, tetapi ada **tiga struct `AuditLog`** (modul `auth`, `audit`, `cold-storage`) untuk tabel yang sama. Tabel kosong setelah seed.

---

## 03-A — Satu penulis audit + jejak yang tak bisa diubah _(Fase P, urutan #3)_

**Tujuan:** setiap aksi sensitif menulis tepat satu baris audit lewat satu komponen, dan baris itu tidak bisa diubah/dihapus. **Aturan baru untuk semua slice berikutnya:** slice yang menambah aksi yang mengubah data wajib memanggil penulis audit ini (masuk DoD).
**Bergantung pada:** 02-A.

1. **MIG**
   - [ ] `audit_log`: tambah `aktor_peran`, `user_agent`, `correlation_id`, `sebelum` JSONB, `sesudah` JSONB.
   - [ ] **Append-only:** trigger `BEFORE UPDATE OR DELETE` yang melempar galat.
   - [ ] Index `(aktor_id, waktu DESC)`, `(entitas, entitas_id, waktu DESC)`, `(aksi, waktu DESC)`.
   - [ ] Keputusan FK `aktor_id`: sekarang `REFERENCES pengguna(id)` tanpa `ON DELETE`, sehingga menghapus pengguna yang punya jejak ditolak. Pilih: pengguna hanya dinonaktifkan, **atau** simpan snapshot nama/email aktor dan pakai `ON DELETE SET NULL`.
2. **SEED**
   - [ ] Lapis `demo`: contoh log deterministik (login, validasi harga, aksi massal, perubahan peran), aktor = akun demo ([01-A](01-auth.md)), ditandai demo. Produksi: tidak ada seed.
3. **API**
   - [ ] Hapus dua struct `AuditLog` duplikat; satu paket `internal/audit` dengan antarmuka `Writer` yang disuntikkan ke modul lain.
   - [ ] Pasang pada: login/logout/gagal login, perubahan pengguna dan peran, aksi massal cold-storage. Modul lain menambah panggilan di slice masing-masing.
   - [ ] `GET /v1/audit`: filter sisi server (`dari`, `sampai`, `aktor`, `aksi`, `entitas`, `hasil`) + paginasi cursor; batas default tidak lagi 500 sekaligus.
4. **FE**
   - [ ] `audit-page` memakai filter server (bukan hanya filter klien atas 500 baris); sesuaikan `toAuditRow` bila bentuk respons berubah.
5. **TES**
   - [ ] Aksi sensitif menulis tepat satu baris; `UPDATE`/`DELETE` pada `audit_log` ditolak; peran tanpa `audit.lihat` mendapat 403.
6. **DoD** — checklist [00 §4](00-cara-kerja.md).

---

## 03-B — Lengkapi tampilan dan ekspor _(Fase F, gelombang F-2)_

1. **API**: ekspor CSV; endpoint detail entri.
2. **FE**: rentang tanggal, pencarian, tampilan detail dengan diff `sebelum`/`sesudah`, tautan ke objek terkait (detail harga/pengguna), tombol ekspor (utility `lib/csv.ts` sudah ada).
3. **TES**: ekspor sesuai filter.

---

## 03-C — Integritas dan retensi _(Fase F, kebutuhan kepatuhan)_

1. **MIG**
   - [ ] Hash berantai (`hash_sebelumnya`, `hash`) dan job verifikasi berkala.
   - [ ] Partisi bulanan (`PARTITION BY RANGE (waktu)`) + job pembuat partisi.
   - [ ] `REVOKE UPDATE, DELETE` dari pengguna DB aplikasi (lihat [19 §7](19-migration-seeder.md)).
2. **API/Ops**
   - [ ] Retensi: lepas partisi lama ke penyimpanan dingin sesuai kebijakan masa simpan (PDP).
   - [ ] Data lineage per record harga: siapa input, validasi, publikasi, versi (PRD §1.4).
