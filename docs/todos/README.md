# TODO — AI Garda Inflasi

Daftar pekerjaan per fitur, disusun dari **audit kode per 5 Nov 2026** (commit `301a715`).

> **Mulai dari [00-cara-kerja.md](00-cara-kerja.md).** Di sana ada urutan task, definisi satu task, dan Definition of Done.

**Bentuk pekerjaan:** setiap fitur dipecah menjadi *vertical slice* bernama `NN-A`, `NN-B`, … Satu slice menembus semua lapisan dengan urutan tetap **MIG → SEED → API → FE → TES → DoD**: migration dibuat saat slice dikerjakan (nomor diambil saat itu, tidak dipra-rencanakan), lalu seeder, API Go, integrasi frontend, dan verifikasi bahwa halaman web tampil dari database. Slice `-A` = **paritas mock** (Fase P); slice `-B` dan seterusnya = **fitur nyata** (Fase F).

Setiap berkas fitur berisi: status, apa yang sudah ada, slice berurutan, dan backlog lanjutan.

## Legenda

| Tanda         | Arti                                                                    |
| ------------- | ----------------------------------------------------------------------- |
| ✅            | Selesai dan terverifikasi (kode ada dan tes/typecheck lolos)            |
| 🟡            | Sebagian: ada kodenya, tetapi belum end-to-end atau belum terverifikasi |
| ⬜            | Belum dikerjakan, atau ada tetapi terbukti tidak cocok dengan skema     |
| `[x]` / `[ ]` | Item checklist selesai / belum                                          |
| P0 / P1 / P2  | Prioritas, mengacu ke §13.2 [prd-prp.md](../prd-prp.md)                 |

Kolom **Terhubung** berarti halaman web memanggil API Go (bukan `lib/api/mock.ts`); mock aktif otomatis bila `NEXT_PUBLIC_API_BASE_URL` kosong. **Terhubung belum berarti berfungsi:** lihat kolom **Skema & seed**, yang menilai apakah entity Go cocok dengan migrasi dan apakah ada data awalnya ([19](19-migration-seeder.md)).

## Ringkasan status

| #   | Fitur                                     | Rute UI                    | UI  | Backend | Terhubung | Skema & seed | Tes | Berkas                                                     |
| --- | ----------------------------------------- | -------------------------- | :-: | :-----: | :-------: | :----------: | :-: | ---------------------------------------------------------- |
| 1   | Autentikasi, MFA, sesi                    | `/masuk`, `/lupa-password` | 🟡  |   ✅    |    ✅     |      ✅      | ✅  | [01-auth.md](01-auth.md)                                   |
| 2   | Pengguna, peran, RBAC                     | `/pengguna`                | ✅  |   ✅    |    ✅     |      ✅      | 🟡  | [02-pengguna-rbac.md](02-pengguna-rbac.md)                 |
| 3   | Audit trail                               | `/audit`                   | ✅  |   ✅    |    ✅     |      ✅      | ✅  | [03-audit.md](03-audit.md)                                 |
| 4   | Master data                               | `/master/[entitas]`        | ✅  |   🟡    |    ✅     |      ✅      | ✅  | [04-master-data.md](04-master-data.md)                     |
| 5   | Sumber data & API                         | `/sumber-data`             | ✅  |   🟡    |    ✅     |      ✅      | ✅  | [05-sumber-data.md](05-sumber-data.md)                     |
| 6   | Validasi harga                            | `/data/harga`              | ✅  |   🟡    |    ✅     |      ✅      | ✅  | [06-harga-validasi.md](06-harga-validasi.md)               |
| 7   | Aplikasi lapangan (PWA)                   | `/lapangan/*`              | ✅  |   🟡    |    ✅     |      ✅      | ✅  | [07-lapangan-pwa.md](07-lapangan-pwa.md)                   |
| 8   | Dashboard operasional                     | `/operasional`             | ✅  |   🟡    |    ✅     |      ✅      | ✅  | [08-dashboard-operasional.md](08-dashboard-operasional.md) |
| 9   | Dashboard eksekutif                       | `/eksekutif`               | ✅  |   🟡    |    ✅     |      ✅      | ✅  | [09-dashboard-eksekutif.md](09-dashboard-eksekutif.md)     |
| 10  | Peringatan dini (EWS)                     | `/ews`                     | ✅  |   🟡    |    ✅     |      ✅      | ✅  | [10-ews.md](10-ews.md)                                     |
| 11  | Prediksi & XAI                            | `/prediksi`                | ✅  |   🟡    |    ✅     |      ✅      | ✅  | [11-prediksi-xai.md](11-prediksi-xai.md)                   |
| 12  | Peta surplus–defisit                      | `/peta`                    | ✅  |   🟡    |    ✅     |      ✅      | ✅  | [12-peta-spasial.md](12-peta-spasial.md)                   |
| 13  | Cold storage & food loss                  | `/cold-storage`            | ✅  |   ✅    |    ✅     |      ✅      | ✅  | [13-cold-storage.md](13-cold-storage.md)                   |
| 14  | Intervensi (kanban)                       | `/intervensi`              | ✅  |   🟡    |    ✅     |      ✅      | ✅  | [14-intervensi.md](14-intervensi.md)                       |
| 15  | Policy brief                              | `/policy-brief`            | ✅  |   🟡    |    ✅     |      ✅      | ✅  | [15-policy-brief.md](15-policy-brief.md)                   |
| 16  | Indikator kinerja (KPI)                   | `/kpi`                     | ✅  |   🟡    |    ✅     |      ✅      | ✅  | [16-kpi.md](16-kpi.md)                                     |
| 17  | Alur sistem & data                        | `/alur-sistem`             | ✅  |   n/a   |    n/a    |     n/a      | n/a | [17-alur-sistem.md](17-alur-sistem.md)                     |
| 18  | Lintas fitur (platform, CI, deploy, docs) | —                          |  —  |    —    |     —     |      —       |  —  | [18-platform-lintas-fitur.md](18-platform-lintas-fitur.md) |
| 19  | **Migration & seeder (fondasi data)**     | —                          |  —  |    —    |     —     |      —       |  —  | [19-migration-seeder.md](19-migration-seeder.md)           |

"Backend ⬜" berarti ada route/CRUD tipis tetapi logika inti (AI, scraping, generator) belum ada. "Backend 🟡" berarti route dan query dasar ada, tetapi aturan bisnis penuh belum. "Skema & seed ⬜" berarti, berdasarkan analisis kode, query modul itu akan gagal pada database hasil migrasi atau tabelnya kosong (detail di §19).

## Status setelah Fase P (diperbarui 5 Nov 2026)

Fondasi (F0), seluruh slice `-A` Fase P, dan F1 sudah dikerjakan dan lolos `go test ./...` serta typecheck/tes web. Semua halaman tampil dari API Go dan database; stack Docker demo ada di [README akar](../../README.md) (`make docker/demo/up`). Yang **belum** selesai dan perlu diketahui:

1. **Fase F (fitur nyata) belum dikerjakan:** layanan ML/prediksi nyata, penyusunan brief dengan LLM, ingestion/scraping sumber data, SSE `GET /v1/ews/stream`, PostGIS, workflow persetujuan, MFA UI, halaman reset password, SSO, sinkronisasi offline lapangan (`POST /v1/lapangan/harga`). Data pada halaman prediksi, eksekutif, peta, dan sebagian sumber data adalah **data demo** yang berlabel demo.
2. **Penegakan izin per halaman di edge belum ada:** `middleware.ts` hanya memeriksa keberadaan cookie. Izin ditegakkan di API Go (403) dan sidebar menyembunyikan menu; membuka URL halaman tanpa izin menampilkan halaman dengan pesan galat, bukan `/akses-ditolak`.
3. **Aksi yang masih simulasi di klien:** ajukan/setujui/arsip master data (04-B), "Tugaskan ke saya" dan eskalasi kanban (14-B), unduh data acuan lapangan (07-B).
4. **Belum diuji:** beban data besar (`EXPLAIN` pada ratusan ribu baris), e2e Playwright otomatis, dan penjadwal di luar proses API (refresh materialized view dan `cmd/kpi` belum dipasang ke cron/deploy).
5. **Asumsi yang perlu dikonfirmasi pemilik program:** target `target_kpi` awal, target inflasi 2025–2027 (1,5–3,5%), skor ruang fiskal heuristik, dan kode/koordinat wilayah-pasar perkiraan.
6. **Dua salinan OpenAPI lama** (`apps/api/docs`, `apps/api/pkg/swagger`) masih berisi spesifikasi analytics; yang diperbarui hanya `packages/contracts/openapi.yaml`.

## Urutan pengerjaan

Daftar lengkap dengan dependensi ada di [00 §6](00-cara-kerja.md). Ringkasnya:

1. **Fondasi (F0-1 … F0-7):** dev proxy, tes cold-storage hijau, tes kontrak skema, seeder kit, tes asap, migrasi nyata saat deploy, CSRF/cookie.
2. **Fase P, paritas mock** (semua halaman tampil dari API + DB dengan data demo), dalam urutan dependensi:
   `01-A` akun → `02-A` izin → `03-A` audit → `04-A` master → **`F1`** (netralkan seed di migrasi 000006) → `06-A` harga → `07-A` tugas lapangan → `08-A` dashboard operasional → `10-A` EWS → `14-A` intervensi → `13-A` cold storage → `12-A` peta → `05-A` sumber data → `09-A` eksekutif → `15-A` policy brief → `16-A` KPI → `11-A` prediksi.
3. **Fase F, fitur nyata**, per gelombang: F-1 alur data lapangan yang benar → F-2 peringatan dan tindakan → F-3 data eksternal → F-4 AI → F-5 fisik dan optimasi.
4. **Gerbang produksi** ([00 §7](00-cara-kerja.md)): migrasi nyata saat deploy, seed bersih, admin awal, cookie/CSRF, backup. Belum boleh deploy sebelum semuanya tercentang.
