# 15 — Policy brief generator

**Prioritas:** P0 (PRD §3.3) · **Status:** UI ✅ · Backend 🟡 · Terhubung ✅ · Skema & seed ✅ · Tes ✅ (15-A dan 15-B selesai; telaah, persetujuan, ekspor di 15-C)
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

## 15-B — Generator nyata (data + LLM) *(Fase F, gelombang F-4)* — selesai (LLM nyata belum teruji)

**Bergantung pada:** 15-A, 09-B, 10-B, 11-B, 14-A. Konteks yang dikumpulkan sekarang: inflasi, harga, peringatan dini, proyeksi; status intervensi, neraca pangan, dan ruang fiskal menyusul.

1. **MIG**
   - [x] `000092`: nilai status `DIPROSES` dan `GAGAL` (`ALTER TYPE … ADD VALUE`, `NO TRANSACTION`, terpisah dari migration yang memakainya).
   - [x] `000093`: `policy_brief.konten` (narasi per bagian + lampiran sitasi), `snapshot_data` (fakta yang dipakai), `sumber_narasi` (`LLM`|`TEMPLAT`|`DEMO`), `model_llm`, `prompt_versi`, `prompt_hash`, `galat`; `brief_job` (percobaan penyusunan); `policy_brief_versi` (versi isi, satu baris per penyusunan).
2. **SEED**: [x] `brief.demo` diperluas: brief contoh berlabel `DEMO` dengan `konten` (idempoten; brief contoh lama yang kontennya kosong dilengkapi, yang sudah berisi tidak ditimpa). Daftar bagian brief berupa konstanta kode, belum tabel referensi.
3. **API**
   - [x] **Pengumpul konteks** (`repository/postgres/konteks.go` + `BangunKonteks`, murni dan deterministik): inflasi dan sasaran, andil, perubahan harga bulanan, peringatan dini, proyeksi model produksi → daftar fakta ber-id dengan nilai terformat dan sumber (`inflasi_bulanan (BPS|DEMO)`, `entri_harga`, `ews_alert`, `prediksi_harga`); data demo menambah catatan keandalan.
   - [x] **`Generator`**: templat deterministik (cadangan, selalu ada) dan klien Claude Messages API (`net/http`, aktif hanya bila `ANTHROPIC_API_KEY` ada; model dari `LLM_MODEL`, bawaan `claude-sonnet-5-5`; jawaban berskema JSON lewat structured outputs; retry untuk 429/5xx/jaringan). Tanpa kunci perilakunya = templat.
   - [x] **Anti-halusinasi:** narasi menyebut angka/nama hanya lewat placeholder `{{ID}}` yang diganti server dengan nilai fakta; teks di luar placeholder tidak boleh memuat angka, placeholder harus dikenal; narasi LLM yang melanggar atau gagal digantikan templat dengan alasan dicatat (tanpa mengutip teks yang ditolak). Lampiran memuat sitasi sumber tiap fakta yang dipakai. Hasil LLM berlabel "Draf AI — wajib ditelaah manusia" dengan model, versi prompt, dan hash prompt.
   - [x] **Asinkron:** `POST /v1/policy-brief/generate` menjawab brief `DIPROSES`; `GET /v1/policy-brief/{id}` untuk memantau (isi, label, jejak model); brief `GAGAL` atau `DIPROSES` basi (> 5 menit) dapat diulang; audit `GENERATE_BRIEF`, `BRIEF_DRAF_SELESAI`, `BRIEF_DRAF_GAGAL`. Proses berjalan sebagai goroutine di instance API (belum antrean tahan restart).
4. **FE**: [x] dialog generator memantau status sampai draf siap, menampilkan label draf, bagian bernarasi, jejak model, dan sitasi sumber; galat dengan "Coba lagi"; daftar brief memuat ulang berkala selama ada brief `DIPROSES`. [ ] Pratinjau dokumen halaman penuh/ekspor (15-C).
5. **TES**: [x] pengumpul konteks dengan data tetap; render dan aturan anti-halusinasi (angka bebas, placeholder tak dikenal/rusak, batas butir); **angka di narasi final cocok dengan nilai fakta sumber**; templat pada berbagai kombinasi data; klien Claude dengan `httptest` (kontrak permintaan, retry, penolakan, galat, tanpa kebocoran kunci); integrasi pada DB hasil migrasi dengan Generator palsu (LLM berhasil, LLM gagal/mengarang angka, pengumpulan data gagal lalu diulang, duplikat/basi). **Tidak ada tes yang memanggil API Anthropic sungguhan.**

---

## 15-C — Telaah, persetujuan, ekspor *(Fase F)*

- [ ] Status DRAF → TELAAH → DISETUJUI → DIDISTRIBUSIKAN dengan izin `brief.susun` dan `brief.setujui` (ada di web, belum dipakai route); `GET /v1/policy-brief/:id`, ubah isi, dan `POST /v1/policy-brief/bulk` (UI memanggil `/v1/{sumber}/bulk`).
- [ ] Editor telaah (sunting teks, komentar, bandingkan versi); `disetujui_oleh`, `disetujui_pada`, `didistribusikan_pada`.
- [ ] Ekspor PDF/DOCX ke object storage (`policy_brief_lampiran`: `brief_id`, `format`, `lokasi_objek`, `checksum`) dan versi siap kirim lewat Unit Hubungan Pemerintah & Kebijakan.
- [ ] Jadwal otomatis bulanan; `UNIQUE (periode)` bila satu brief resmi per periode; audit pembuatan dan persetujuan.
