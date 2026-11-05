# 15 — Policy brief generator

**Prioritas:** P0 (PRD §3.3) · **Status:** UI ✅ · Backend 🟡 · Terhubung ✅ · Skema & seed ✅ · Tes ✅ (15-A selesai; generator nyata di 15-B)
**Cara kerja & DoD:** [00-cara-kerja.md](00-cara-kerja.md) · **Konteks data:** [19](19-migration-seeder.md)

## Yang sudah ada

- **UI:** [brief-page.tsx](../../apps/web/src/components/brief/brief-page.tsx) di `/policy-brief`: generator draf beranimasi, telaah, persetujuan. Ringkasan brief juga tampil di [exec-side](../../apps/web/src/components/eksekutif/exec-side.tsx). Halaman memakai `mockXai` langsung untuk bagian tertentu.
- **API Go:** `GET /v1/policy-brief` dan `POST /v1/policy-brief/generate` (`brief.lihat`). `Generate` hanya menyimpan baris DRAFT dengan ringkasan tetap "Policy brief sedang diproses." Tidak ada isi, grafik, atau LLM.
- **Skema:** migrasi `policy_brief` (`id` VARCHAR mis. `PB-2026-10`, `periode`, `judul`, `status` enum `DRAF/TELAAH/DISETUJUI/DIDISTRIBUSIKAN`, `ringkasan`, `rekomendasi` JSONB, `halaman`, `disusun_oleh_id` NOT NULL FK, `diperbarui`). **`Generate` pasti gagal pada DB hasil migrasi:** `id` `int64`, status default `DRAFT` (enum hanya `DRAF`), kolom `dibuat_pada`/`diupdate_pada` tidak ada, dan `disusun_oleh_id` (NOT NULL) tidak diisi. Tabel kosong.

---

## 15-A — Daftar brief dan `Generate` yang valid *(Fase P, urutan #15)*

**Tujuan:** `/policy-brief` menampilkan brief dari DB, dan tombol "susun" membuat draf yang valid (isinya masih placeholder).
**Bergantung pada:** 02-A, 11-A (untuk bagian XAI).

1. **MIG**
   - [x] `policy_brief`: tambah `dibuat_pada` (`DEFAULT now()`, migrasi `000050`). Entity diselaraskan: `id` string (`PB-YYYY-MM`), status `DRAF`, kolom waktu `dibuat_pada` dan `diperbarui`.
2. **SEED**
   - [x] `demo` (`brief.demo`, `DependsOn master.reference`, `After auth.demo`): 5 brief yang meniru `mockBriefs`, `disusun_oleh_id` = akun `manajer_kebijakan`. Idempoten (`DO NOTHING`, perubahan status tidak ditimpa). Produksi: tidak ada brief contoh.
3. **API**
   - [x] `Generate` (`brief.susun`): `periode` `YYYY-MM` -> id `PB-YYYY-MM`, penyusun dari token, status `DRAF`, `rekomendasi` `[]`, isi dari templat deterministik berlabel "draf otomatis non-LLM"; periode duplikat 409, format salah 400; tercatat di audit (`GENERATE_BRIEF`). `GET` mengembalikan daftar (snake_case) dengan nama penyusun dan jumlah rekomendasi.
4. **FE**
   - [x] `useBriefs` ke API lewat mapper `toPolicyBrief` (`lib/api/brief.ts`); dialog generator memanggil `api.brief.susun` (konfirmasi -> simpan -> tampilkan draf tersimpan). Stepper animasi palsu dan `mockXai` di `brief-page.tsx` dibuang (bagian XAI memakai data nyata di [11-A](11-prediksi-xai.md)).
   - Catatan: aksi massal "Ajukan telaah", "Setujui", "Unduh PDF" masih menunggu 15-C (`POST /v1/policy-brief/bulk` belum ada).
5. **TES**
   - [x] `Generate` pada DB hasil migrasi menyimpan baris `DRAF` dengan penyusun dari token, audit, 403/409/400 (integrasi); seeder idempoten; `policy-brief` keluar dari `belumSelaras`; tes asap daftar brief.
6. **DoD** — checklist [00 §4](00-cara-kerja.md).

---

## 15-B — Generator nyata (data + LLM) *(Fase F, gelombang F-4)*

**Bergantung pada:** 15-A, 09-B, 10-B, 11-B, 14-A. Untuk LLM gunakan skill `claude-api` dan model Claude terbaru.

1. **MIG**
   - [ ] Nilai status `DIPROSES` dan `GAGAL`: `ALTER TYPE … ADD VALUE` di migration **terpisah** dengan `-- +goose NO TRANSACTION`, tidak dipakai di migration yang sama.
   - [ ] `policy_brief`: `konten` JSONB (narasi per bagian), `snapshot_data` JSONB (angka dasar yang dipakai), `model_llm`, `prompt_versi`.
   - [ ] `brief_job` (`id`, `brief_id`, `status`, `mulai`, `selesai`, `galat`), `policy_brief_versi` (`brief_id`, `versi`, `konten`, `oleh`, `waktu`).
2. **SEED** (`reference`): templat brief (daftar bagian dan urutannya).
3. **API**
   - [ ] **Pengumpul konteks** untuk periode terpilih: inflasi, harga komoditas utama, EWS, proyeksi + faktor XAI, status intervensi, neraca pangan, ruang fiskal.
   - [ ] **Penyusun narasi** dengan LLM memakai templat terstruktur (ringkasan, temuan, risiko, rekomendasi untuk Pemda & TPID, lampiran). **Anti-halusinasi:** semua angka disisipkan dari data terstruktur lewat placeholder; LLM hanya menulis narasi; sertakan sitasi sumber.
   - [ ] Proses asinkron: `Generate` mengembalikan id + status `DIPROSES`, klien polling/SSE sampai `DRAF` siap (UI sekarang berasumsi 2–3 detik).
4. **FE**: polling status, penanganan galat/timeout dengan opsi coba lagi, pratinjau dokumen.
5. **TES**: pengumpul konteks dengan data tetap; **uji otomatis bahwa angka di narasi cocok dengan data sumber**.

---

## 15-C — Telaah, persetujuan, ekspor *(Fase F)*

- [ ] Status DRAF → TELAAH → DISETUJUI → DIDISTRIBUSIKAN dengan izin `brief.susun` dan `brief.setujui` (ada di web, belum dipakai route); `GET /v1/policy-brief/:id`, ubah isi, dan `POST /v1/policy-brief/bulk` (UI memanggil `/v1/{sumber}/bulk`).
- [ ] Editor telaah (sunting teks, komentar, bandingkan versi); `disetujui_oleh`, `disetujui_pada`, `didistribusikan_pada`.
- [ ] Ekspor PDF/DOCX ke object storage (`policy_brief_lampiran`: `brief_id`, `format`, `lokasi_objek`, `checksum`) dan versi siap kirim lewat Unit Hubungan Pemerintah & Kebijakan.
- [ ] Jadwal otomatis bulanan; `UNIQUE (periode)` bila satu brief resmi per periode; audit pembuatan dan persetujuan.
