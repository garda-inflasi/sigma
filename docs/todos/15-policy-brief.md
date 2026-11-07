# 15 — Policy brief generator

**Prioritas:** P0 (PRD §3.3) · **Status:** UI ✅ · Backend 🟡 · Terhubung ✅ · Skema & seed ✅ · Tes ✅ (15-A dan 15-B selesai; telaah, persetujuan, ekspor di 15-C)
**Cara kerja & DoD:** [00-cara-kerja.md](00-cara-kerja.md) · **Konteks data:** [19](19-migration-seeder.md)

## Yang sudah ada

- **UI:** [brief-page.tsx](../../apps/web/src/components/brief/brief-page.tsx) di `/policy-brief`: generator draf beranimasi, telaah, persetujuan. Ringkasan brief juga tampil di [exec-side](../../apps/web/src/components/eksekutif/exec-side.tsx). Halaman memakai `mockXai` langsung untuk bagian tertentu.
- **API Go:** `GET /v1/policy-brief` dan `POST /v1/policy-brief/generate` (`brief.lihat`). `Generate` hanya menyimpan baris DRAFT dengan ringkasan tetap "Policy brief sedang diproses." Tidak ada isi, grafik, atau LLM.
- **Skema:** migrasi `policy_brief` (`id` VARCHAR mis. `PB-2026-10`, `periode`, `judul`, `status` enum `DRAF/TELAAH/DISETUJUI/DIDISTRIBUSIKAN`, `ringkasan`, `rekomendasi` JSONB, `halaman`, `disusun_oleh_id` NOT NULL FK, `diperbarui`). **`Generate` pasti gagal pada DB hasil migrasi:** `id` `int64`, status default `DRAFT` (enum hanya `DRAF`), kolom `dibuat_pada`/`diupdate_pada` tidak ada, dan `disusun_oleh_id` (NOT NULL) tidak diisi. Tabel kosong.

### Kunci primer (000114, ✅)

Rujukan [SKEMA-DATABASE.md](../SKEMA-DATABASE.md), subbagian 000114.

| Tabel | PK | `kode` | FK dan aturan ON DELETE |
| ----- | -- | ------ | ----------------------- |
| `policy_brief` | **UUID** `DEFAULT gen_random_uuid()`; ID lama `PB-YYYY-MM` → `garda_uuid_stabil('policy_brief', id_lama)` | **`PB-YYYY-MM`**, NOT NULL, **UNIQUE**, `CHECK` berbentuk `PB-YYYY-MM` (kode tampilan, alias URL, kunci idempoten) | `disusun_oleh_id` uuid → `pengguna` RESTRICT |
| `policy_brief_versi` | **BIGINT** `id` (= `versi_id`) + `UNIQUE (brief_id, versi)` | tanpa kode | `brief_id` uuid → `policy_brief` **CASCADE**; `oleh` uuid → `pengguna` RESTRICT |
| `brief_job` | **BIGINT** identity (urut `mulai`) | ID lama (UUID); NULL untuk job baru | `brief_id` uuid → `policy_brief` **CASCADE** (sebelumnya NO ACTION); indeks parsial `idx_brief_job_berjalan WHERE status = 'BERJALAN'` |

- **Rute:** `GET /v1/policy-brief/:id` dan semua anaknya membaca `:id` lewat `idx.ParamUUIDAtauKode` (maks 64 karakter): UUID (huruf besar dinormalkan) atau kode `PB-YYYY-MM`, jadi `/policy-brief/PB-2026-10` tetap berlaku. Bukan UUID sah dan bukan kode yang ada (kosong, > 64 karakter, angka, ejaan UUID non-kanonik) dijawab **404** tanpa menyentuh kolom `uuid`. `ids` pada `POST /v1/policy-brief/bulk` menerima UUID atau kode; elemen kosong atau > 64 karakter 400, rujukan yang tidak ada dihitung gagal. Respons memuat `id` (UUID) dan `kode`; audit memakai `entitas_id` = UUID brief (dan `metadata.kode`); `audit_log.entitas_id` lama dipetakan oleh migrasi.
- **Satu brief per periode.** Logika `Generate` sejak awal satu brief per periode (id = `PB-YYYY-MM`); kini keunikan itu dijaga `UNIQUE (kode)`: `INSERT … ON CONFLICT (kode) DO NOTHING` menjadikan permintaan bersamaan satu brief dan sisanya **409** (diuji paralel), brief `GAGAL` atau `DIPROSES` basi diulang pada brief yang sama (job baru). Kolom `periode` berisi label (`Oktober 2026`), bukan kunci, jadi `UNIQUE (periode)` pada 15-C tidak diperlukan.
- **`versi_id`.** Respons `GET /v1/policy-brief/:id` memuat `riwayat_versi: [{id, versi, oleh, waktu}]` (terbaru dulu; `id` = `versi_id` BIGINT sebagai string, `versi` = nomor urut per brief) agar telaah dan revisi dapat merujuk versi tertentu. `brief_job.id` tidak lagi dibuat aplikasi (`uuid.NewString()` dihapus).
- **Seeder** memakai `idx.UUIDStabil("policy_brief", "PB-2026-10")` dan `ON CONFLICT (kode)`; hasilnya sama dengan UUID hasil migrasi dan tidak menggandakan brief pada basis data yang sudah dimigrasi. Isi brief (`konten`, `snapshot_data`) memuat id fakta (`ANDIL_1_NILAI`), bukan ID basis data, jadi tidak perlu dipetakan.

---

## 15-A — Daftar brief dan `Generate` yang valid *(Fase P, urutan #15)*

**Tujuan:** `/policy-brief` menampilkan brief dari DB, dan tombol "susun" membuat draf yang valid (isinya masih placeholder).
**Bergantung pada:** 02-A, 11-A (untuk bagian XAI).

1. **MIG**
   - [x] `policy_brief`: tambah `dibuat_pada` (`DEFAULT now()`, migrasi `000050`). Entity diselaraskan: `id` string (`PB-YYYY-MM`; sejak 000114 UUID dengan `kode` `PB-YYYY-MM`), status `DRAF`, kolom waktu `dibuat_pada` dan `diperbarui`.
2. **SEED**
   - [x] `demo` (`brief.demo`, `DependsOn master.reference`, `After auth.demo`): 5 brief yang meniru `mockBriefs`, `disusun_oleh_id` = akun `manajer_kebijakan`. Idempoten (`DO NOTHING`, perubahan status tidak ditimpa). Produksi: tidak ada brief contoh.
3. **API**
   - [x] `Generate` (`brief.susun`): `periode` `YYYY-MM` -> kode `PB-YYYY-MM` (id UUID dari basis data), penyusun dari token, status `DRAF`, `rekomendasi` `[]`, isi dari templat deterministik berlabel "draf otomatis non-LLM"; periode duplikat 409, format salah 400; tercatat di audit (`GENERATE_BRIEF`). `GET` mengembalikan daftar (snake_case) dengan nama penyusun dan jumlah rekomendasi.
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
   - [x] `000093`: `policy_brief.konten` (narasi per bagian + lampiran sitasi), `snapshot_data` (fakta yang dipakai), `sumber_narasi` (`LLM`|`TEMPLAT`|`DEMO`), `model_llm`, `prompt_versi`, `prompt_hash`, `galat`; `brief_job` (percobaan penyusunan); `policy_brief_versi` (versi isi, satu baris per penyusunan; PK `(brief_id, versi)` menjadi `id` BIGINT + `UNIQUE (brief_id, versi)` di 000114).
2. **SEED**: [x] `brief.demo` diperluas: brief contoh berlabel `DEMO` dengan `konten` (idempoten; brief contoh lama yang kontennya kosong dilengkapi, yang sudah berisi tidak ditimpa). Daftar bagian brief berupa konstanta kode, belum tabel referensi.
3. **API**
   - [x] **Pengumpul konteks** (`repository/postgres/konteks.go` + `BangunKonteks`, murni dan deterministik): inflasi dan sasaran, andil, perubahan harga bulanan, peringatan dini, proyeksi model produksi → daftar fakta ber-id dengan nilai terformat dan sumber (`inflasi_bulanan (BPS|DEMO)`, `entri_harga`, `ews_alert`, `prediksi_harga`); data demo menambah catatan keandalan.
   - [x] **`Generator`**: templat deterministik (cadangan, selalu ada) dan klien Claude Messages API lewat SDK Go resmi `anthropic-sdk-go/v2` (aktif hanya bila `ANTHROPIC_API_KEY` ada; model dari `LLM_MODEL`, bawaan `claude-opus-5-5`; jawaban berskema JSON lewat structured outputs; fallback server-side `"default"` saat ditolak pengaman model; SDK mengulang 408/409/429/5xx/jaringan). Tanpa kunci perilakunya = templat.
   - [x] **Anti-halusinasi:** narasi menyebut angka/nama hanya lewat placeholder `{{ID}}` yang diganti server dengan nilai fakta; teks di luar placeholder tidak boleh memuat angka, placeholder harus dikenal; narasi LLM yang melanggar atau gagal digantikan templat dengan alasan dicatat (tanpa mengutip teks yang ditolak). Lampiran memuat sitasi sumber tiap fakta yang dipakai. Hasil LLM berlabel "Draf AI — wajib ditelaah manusia" dengan model, versi prompt, dan hash prompt.
   - [x] **Asinkron:** `POST /v1/policy-brief/generate` menjawab brief `DIPROSES`; `GET /v1/policy-brief/{id}` untuk memantau (isi, label, jejak model); brief `GAGAL` atau `DIPROSES` basi (> 5 menit) dapat diulang; audit `GENERATE_BRIEF`, `BRIEF_DRAF_SELESAI`, `BRIEF_DRAF_GAGAL`. Proses berjalan sebagai goroutine di instance API (belum antrean tahan restart).
4. **FE**: [x] dialog generator memantau status sampai draf siap, menampilkan label draf, bagian bernarasi, jejak model, dan sitasi sumber; galat dengan "Coba lagi"; daftar brief memuat ulang berkala selama ada brief `DIPROSES`. [ ] Pratinjau dokumen halaman penuh/ekspor (15-C).
5. **TES**: [x] pengumpul konteks dengan data tetap; render dan aturan anti-halusinasi (angka bebas, placeholder tak dikenal/rusak, batas butir); **angka di narasi final cocok dengan nilai fakta sumber**; templat pada berbagai kombinasi data; klien Claude dengan `httptest` (kontrak permintaan, retry, penolakan, galat, tanpa kebocoran kunci); integrasi pada DB hasil migrasi dengan Generator palsu (LLM berhasil, LLM gagal/mengarang angka, pengumpulan data gagal lalu diulang, duplikat/basi). **Tidak ada tes yang memanggil API Anthropic sungguhan.**

---

## 15-C — Telaah, persetujuan, ekspor *(Fase F)*

- [ ] Status DRAF → TELAAH → DISETUJUI → DIDISTRIBUSIKAN dengan izin `brief.susun` dan `brief.setujui` (ada di web, belum dipakai route); `POST /v1/policy-brief/bulk` untuk memajukan status (UI memanggil `/v1/{sumber}/bulk`; sudah ada) dan ubah isi lewat `PATCH /v1/policy-brief/:id` yang menulis **versi baru** (`policy_brief_versi`), bukan menimpa. `GET /v1/policy-brief/:id` sudah ada. Semua rute `:id` memakai `idx.ParamUUIDAtauKode` (UUID atau kode `PB-YYYY-MM`, 404 untuk rujukan salah bentuk).
- [ ] Editor telaah (sunting teks, komentar, bandingkan versi); `disetujui_oleh`, `disetujui_pada`, `didistribusikan_pada` (kolom `uuid` nullable → `pengguna` RESTRICT/SET NULL sesuai aturan SKEMA §2 untuk pelaku opsional, dengan indeks). Spesifikasi di bawah.
- [ ] Ekspor PDF/DOCX ke object storage (`policy_brief_lampiran`: `id` BIGINT identity, `brief_id` uuid → `policy_brief` CASCADE, `format`, `lokasi_objek`, `checksum`, indeks `idx_policy_brief_lampiran_brief_id`; rute `GET /v1/policy-brief/:id/lampiran`) dan versi siap kirim lewat Unit Hubungan Pemerintah & Kebijakan.
- [ ] Jadwal otomatis bulanan (memanggil jalur `Generate` yang sama; keunikan periode sudah dijaga `UNIQUE (kode)`, jadi tidak perlu `UNIQUE (periode)`); audit pembuatan dan persetujuan.

### Spesifikasi telaah editor dan revisi (rencana, belum dibuat)

Bagian ini hanya dokumen: tidak ada migrasi, kode, atau rute yang dibuat untuknya di 000114.

- **Tabel `policy_brief_telaah`** (satu baris per catatan telaah editor atas satu versi isi):

  | Kolom | Tipe | Keterangan |
  | ----- | ---- | ---------- |
  | `id` | `bigint GENERATED ALWAYS AS IDENTITY` | PK (BIGINT: transaksional) |
  | `brief_id` | `uuid` | FK → `policy_brief(id)` **ON DELETE CASCADE** |
  | `versi_id` | `bigint` | FK → `policy_brief_versi(id)` **ON DELETE CASCADE** |
  | `editor_id` | `uuid` | FK → `pengguna(id)` **ON DELETE RESTRICT** (bukti asal telaah; akun penelaah tidak bisa dihapus, disarankan dinonaktifkan) |
  | `komentar`, `status`, `dibuat_pada` | sesuai fitur | isi telaah, keputusan, waktu |

  Indeks: `idx_policy_brief_telaah_brief_versi (brief_id, versi_id)` (juga melayani FK `brief_id`), `idx_policy_brief_telaah_versi_id (versi_id)`, dan `idx_policy_brief_telaah_editor_id (editor_id)` (indeks FK ke `pengguna` wajib berawalan `editor_id`). Pasangan `(brief_id, versi_id)` harus konsisten dengan `policy_brief_versi.brief_id` (cek di use case atau FK komposit ke `UNIQUE (id, brief_id)`).
- **Rute:** `/v1/policy-brief/:id/versi/:versiId/telaah` (GET daftar telaah, POST menambah). `:id` memakai `idx.ParamUUIDAtauKode`; `:versiId` adalah **BIGINT** auto-increment (`policy_brief_versi.id` = `versi_id` di `riwayat_versi`) dan dibaca lewat `idx.ParamID` (tanpa alias kode; salah bentuk 404). Versi yang bukan milik `:id` dijawab 404.
- **Revisi** membuat baris `policy_brief_versi` **baru** (nomor `versi` berikutnya, `id` BIGINT baru) yang merujuk versi asalnya lewat kolom baru `versi_asal_id bigint NULL → policy_brief_versi(id) ON DELETE SET NULL` (NULL untuk versi hasil generator; indeks `idx_policy_brief_versi_versi_asal_id`). Versi lama tidak pernah diubah atau dihapus. Telaah merujuk `versi_id` yang ditelaah, sehingga komentar tetap terkait pada isi yang dibaca editor walau revisi menyusul.
- **Izin dan audit:** menulis telaah butuh `brief.susun` atau izin telaah khusus yang diputuskan di 15-C; setiap telaah dan revisi dicatat di audit dengan `entitas_id` = UUID brief dan `metadata.versi_id`.
