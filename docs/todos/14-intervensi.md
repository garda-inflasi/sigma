# 14 — Intervensi (kanban 3 siklus) & workflow engine

**Prioritas:** P0 (Fase 3, PRD Layer 4) · **Status:** UI ✅ · Backend 🟡 (daftar, buat, pindah tahap; belum riwayat/konkurensi) · Terhubung ✅ · Skema & seed ✅ · Tes ✅
**Cara kerja & DoD:** [00-cara-kerja.md](00-cara-kerja.md) · **Konteks data:** [19](19-migration-seeder.md)

## Yang sudah ada

- **UI:** [kanban-board](../../apps/web/src/components/intervensi/kanban-board.tsx) (dnd-kit), kolom dan kartu, tampilan tabel, izin per tahap, [store kanban](../../apps/web/src/lib/stores/kanban-store.ts). Tiga siklus: Normal, Tekanan Harga/Defisit, Surplus.
- **API Go:** `GET /v1/intervensi` (filter `siklus`/`tahap`/`prioritas`; kartu beserta penanggung jawab), `GET /v1/intervensi/tahap` (definisi tahap), `POST /v1/intervensi` (`intervensi.kelola`; masuk tahap pertama siklusnya), `PATCH /v1/intervensi/:id/tahap` (menomori ulang kolom; izin tahap tujuan dari `tahap_intervensi_def.izin_dibutuhkan` ditegakkan di usecase; dicatat di audit). Endpoint `POST /v1/intervensi/bulk` (arsip/hapus) dicabut: tidak pernah berjalan (tabel `kartu_intervensi` tak ada), tidak dipakai web, dan `hapus` merusak data.
- **Web:** `api.intervensi.list` membaca API (`lib/api/intervensi.ts`) dan disegarkan tiap 30 detik; `SIKLUS_TAHAP` di `lib/data/siklus.ts` dipertahankan sebagai cache klien dan dijaga tes kesamaan terhadap `packages/contracts/intervensi-tahap.json` (cermin seed `intervensi.reference`).
- **Skema:** `intervensi` (+ `deskripsi`, `selesai_pada`), `intervensi_pj`, `tahap_intervensi_def` dengan FK komposit `(siklus, tahap)`; entity cocok (migrasi 000031). Seeder `intervensi.reference` (16 tahap) dan `intervensi.demo` (19 kartu meniru `mockKanban`, tertaut ke alert `ews.demo`).

---

## 14-A — Kanban dari DB *(Fase P, urutan #10)*

**Tujuan:** `/intervensi` menampilkan dan memindahkan kartu dari database; definisi tahap punya satu sumber.
**Bergantung pada:** 10-A (kartu merujuk alert), 02-A.

1. **MIG**
   - [x] `tahap_intervensi_def` (`siklus`, `kode_tahap`, `urutan`, `label`, `izin_dibutuhkan`, `sla_hari`); validasi `intervensi.tahap` terhadapnya.
   - [x] `intervensi`: tambah `deskripsi` dan `selesai_pada` (nullable). Entity diselaraskan ke `intervensi` + `intervensi_pj`: `id` string, `siklus` enum, `kabupaten_kode`; hapus `pic` (digantikan `intervensi_pj`) dan tipe `tahap_intervensi`.
2. **SEED**
   - [x] `reference`: definisi tahap untuk tiga siklus beserta izin per tahap (sumber: `lib/data/siklus.ts` dan PRD §4.1).
   - [x] `demo`: kartu dan penanggung jawab yang meniru `mockKanban`; `ews_alert_id` merujuk seed EWS ([10-A](10-ews.md)). Tidak ada kartu contoh di produksi.
3. **API**
   - [x] `GET /v1/intervensi` mengembalikan bentuk `KartuIntervensi` (dengan daftar penanggung jawab); `PATCH /:id/tahap` memvalidasi tahap terhadap definisi.
   - [x] Endpoint `GET /v1/intervensi/tahap` untuk definisi tahap.
4. **FE**
   - [x] `api.intervensi.*` sudah memanggil API; ganti sumber definisi tahap di `lib/data/siklus.ts` dengan data API (atau pertahankan sebagai cache dengan tes kesamaan).
   - [x] `useIntervensi` memakai `staleTime: Infinity`: kartu yang digeser pengguna lain tidak muncul tanpa refresh; tambahkan polling ringan.
5. **TES**: setiap `intervensi.tahap` ada di `tahap_intervensi_def` untuk siklusnya; hapus `intervensi` dari `belumSelaras`.
6. **DoD** — checklist [00 §4](00-cara-kerja.md).

---

## 14-B — Izin per tahap, riwayat, konkurensi *(Fase F, gelombang F-2)*

**Tujuan:** aturan persetujuan berjenjang ditegakkan di backend dan setiap perpindahan tercatat.
**Bergantung pada:** 14-A, 03-A.

1. **MIG**
   - [ ] `intervensi_riwayat` (`kartu_id`, `dari`, `ke`, `oleh`, `waktu`, `alasan`).
   - [ ] `intervensi`: `versi` INT untuk optimistic locking pada drag-and-drop bersamaan.
   - [ ] Urutan kartu: indeks pecahan (`urutan` NUMERIC / LexoRank) atau `UNIQUE (siklus, tahap, urutan) DEFERRABLE INITIALLY DEFERRED`.
   - [ ] `intervensi_komentar`, `intervensi_lampiran`.
2. **SEED** (`demo`): riwayat dan komentar untuk kartu demo.
3. **API**
   - [x] **Tegakkan** (sudah di 14-A, dari `tahap_intervensi_def.izin_dibutuhkan`): tahap "Petakan suplai" dan "Selesai" butuh `intervensi.setujui`; "Dukungan instrumen belanja" hanya `intervensi.setujui_fiskal`. Sekarang route hanya meminta `intervensi.kelola`, jadi aturan hanya hidup di UI.
   - [ ] Validasi transisi tahap; respons galat jelas; tolak bila `versi` usang; tulis audit.
   - [ ] Aksi `bulk` dengan hasil per kartu; row scope sesuai peran.
4. **FE**: dialog buat/ubah kartu (penanggung jawab, tenggat, prioritas); pesan jelas bila drop ditolak (izin atau aturan) alih-alih hanya animasi kembali.
5. **TES**: izin per tahap (peran × tahap); konkurensi pergeseran; `kanban-store` dan komponen drag-and-drop.

---

## 14-C — Workflow engine *(Fase F, gelombang F-5; PRD §4)*

**Bergantung pada:** 10-B, 08-B.

- [ ] **Pemicu otomatis siklus:** EWS tertentu → siklus Tekanan Harga; over-supply → Surplus; kembali Normal bila indikator pulih.
- [ ] **Kartu otomatis** dari alert/rekomendasi AI dengan tautan ke sumbernya.
- [ ] **Task management dan eskalasi:** penugasan otomatis ke unit, tenggat, pengingat, eskalasi bila lewat SLA (`tahap_intervensi_def.sla_hari`).
- [ ] Definisi alur sebagai data agar bisa diubah tanpa deploy.
- [ ] Simulasi what-if dan rekomendasi AI + validasi manusia + audit keputusan (PRD §4.3).
- [ ] Integrasi ke perencanaan APBD/TPID **melalui policy brief** ([15](15-policy-brief.md)), bukan akun TPID/Pemda (mereka bukan pengguna sistem).
- [ ] `checklist_item` bila checklist menjadi nyata (sekarang hanya dua counter).
