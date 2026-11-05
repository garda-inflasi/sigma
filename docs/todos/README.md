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
| 1   | Autentikasi, MFA, sesi                    | `/masuk`, `/lupa-password` | ✅  |   ✅    |    ✅     |      ✅      | ✅  | [01-auth.md](01-auth.md)                                   |
| 2   | Pengguna, peran, RBAC                     | `/pengguna`                | ✅  |   ✅    |    ✅     |      ✅      | 🟡  | [02-pengguna-rbac.md](02-pengguna-rbac.md)                 |
| 3   | Audit trail                               | `/audit`                   | ✅  |   ✅    |    ✅     |      ✅      | ✅  | [03-audit.md](03-audit.md)                                 |
| 4   | Master data                               | `/master/[entitas]`        | ✅  |   🟡    |    ✅     |      ✅      | ✅  | [04-master-data.md](04-master-data.md)                     |
| 5   | Sumber data & API                         | `/sumber-data`             | ✅  |   🟡    |    ✅     |      ✅      | ✅  | [05-sumber-data.md](05-sumber-data.md)                     |
| 6   | Validasi harga                            | `/data/harga`              | ✅  |   ✅    |    ✅     |      ✅      | ✅  | [06-harga-validasi.md](06-harga-validasi.md)               |
| 7   | Aplikasi lapangan (PWA)                   | `/lapangan/*`              | ✅  |   🟡    |    ✅     |      ✅      | ✅  | [07-lapangan-pwa.md](07-lapangan-pwa.md)                   |
| 8   | Dashboard operasional                     | `/operasional`             | ✅  |   ✅    |    ✅     |      ✅      | ✅  | [08-dashboard-operasional.md](08-dashboard-operasional.md) |
| 9   | Dashboard eksekutif                       | `/eksekutif`               | ✅  |   🟡    |    ✅     |      ✅      | ✅  | [09-dashboard-eksekutif.md](09-dashboard-eksekutif.md)     |
| 10  | Peringatan dini (EWS)                     | `/ews`                     | ✅  |   ✅    |    ✅     |      ✅      | ✅  | [10-ews.md](10-ews.md)                                     |
| 11  | Prediksi & XAI                            | `/prediksi`                | ✅  |   🟡    |    ✅     |      ✅      | ✅  | [11-prediksi-xai.md](11-prediksi-xai.md)                   |
| 12  | Peta surplus–defisit                      | `/peta`                    | ✅  |   🟡    |    ✅     |      ✅      | ✅  | [12-peta-spasial.md](12-peta-spasial.md)                   |
| 13  | Cold storage & food loss                  | `/cold-storage`            | ✅  |   ✅    |    ✅     |      ✅      | ✅  | [13-cold-storage.md](13-cold-storage.md)                   |
| 14  | Intervensi (kanban)                       | `/intervensi`              | ✅  |   ✅    |    ✅     |      ✅      | ✅  | [14-intervensi.md](14-intervensi.md)                       |
| 15  | Policy brief                              | `/policy-brief`            | ✅  |   🟡    |    ✅     |      ✅      | ✅  | [15-policy-brief.md](15-policy-brief.md)                   |
| 16  | Indikator kinerja (KPI)                   | `/kpi`                     | ✅  |   🟡    |    ✅     |      ✅      | ✅  | [16-kpi.md](16-kpi.md)                                     |
| 17  | Alur sistem & data                        | `/alur-sistem`             | ✅  |   n/a   |    n/a    |     n/a      | n/a | [17-alur-sistem.md](17-alur-sistem.md)                     |
| 18  | Lintas fitur (platform, CI, deploy, docs) | —                          |  —  |    —    |     —     |      —       |  —  | [18-platform-lintas-fitur.md](18-platform-lintas-fitur.md) |
| 19  | **Migration & seeder (fondasi data)**     | —                          |  —  |    —    |     —     |      —       |  —  | [19-migration-seeder.md](19-migration-seeder.md)           |

"Backend ⬜" berarti ada route/CRUD tipis tetapi logika inti (AI, scraping, generator) belum ada. "Backend 🟡" berarti route dan query dasar ada, tetapi aturan bisnis penuh belum. "Skema & seed ⬜" berarti, berdasarkan analisis kode, query modul itu akan gagal pada database hasil migrasi atau tabelnya kosong (detail di §19).

## Status setelah Fase F (diperbarui 5 Nov 2026)

Fondasi (F0), Fase P (semua slice `-A`), F1, Fase F gelombang F-1 sampai F-5, dan gerbang produksi
([00 §7](00-cara-kerja.md)) sudah dikerjakan dan lolos `go test ./...` (90 paket, termasuk tes Postgres
sungguhan), tes web, dan tes layanan ML. Stack Docker demo: `make docker/demo/up` ([README akar](../../README.md)).
"Backend 🟡" di tabel berarti slice Fase F-nya selesai tetapi masih ada keterbatasan di bawah. Yang perlu diketahui:

1. **Belum diuji terhadap layanan nyata** (tidak ada akses/kunci saat dikerjakan): konektor PIHPS, BMKG, dan BPS
   (parser dikunci fixture; id wilayah PIHPS dan id variabel BPS wajib diisi operator), panggilan Claude untuk
   brief (tanpa `ANTHROPIC_API_KEY` brief memakai templat), pengiriman SMTP dan Web Push, serta deploy ujung ke
   ujung (remote-deploy, rollback, sertifikat ACME).
2. **Belum dikerjakan (di luar daftar Fase F):** row scope dan pemisahan tugas per peran (02-B), hash berantai dan
   partisi audit (03-C), persetujuan perubahan master data (04-B, aksinya masih simulasi di klien), foto entri
   lapangan (07-C), ensemble dan fitur eksogen model prediksi serta retraining terjadwal (11), batas wilayah resmi
   dan matriks jarak jalan (12), MQTT cold storage (13), SSO, WhatsApp, dan konektor Bapanas/DJPb/OpenWeather.
3. **Data demo atau perkiraan** (berlabel di UI): geometri dan koordinat wilayah, target KPI dan target inflasi awal,
   skor ruang fiskal heuristik, koefisien simulasi skenario prediksi. Perlu konfirmasi pemilik program.
4. **Pengujian:** belum ada e2e Playwright otomatis (halaman diperiksa manual dengan browser headless saat demo);
   kinerja pada data ratusan ribu baris belum diukur.
5. **Operasional:** backup belum disalin ke luar VM dan belum ada WAL/PITR; pengguna DB aplikasi masih pemilik
   skema (least-privilege di [19 §7](19-migration-seeder.md) belum).

## Urutan pengerjaan

Daftar lengkap dengan dependensi ada di [00 §6](00-cara-kerja.md). Ringkasnya:

1. **Fondasi (F0-1 … F0-7):** dev proxy, tes cold-storage hijau, tes kontrak skema, seeder kit, tes asap, migrasi nyata saat deploy, CSRF/cookie.
2. **Fase P, paritas mock** (semua halaman tampil dari API + DB dengan data demo), dalam urutan dependensi:
   `01-A` akun → `02-A` izin → `03-A` audit → `04-A` master → **`F1`** (netralkan seed di migrasi 000006) → `06-A` harga → `07-A` tugas lapangan → `08-A` dashboard operasional → `10-A` EWS → `14-A` intervensi → `13-A` cold storage → `12-A` peta → `05-A` sumber data → `09-A` eksekutif → `15-A` policy brief → `16-A` KPI → `11-A` prediksi.
3. **Fase F, fitur nyata**, per gelombang: F-1 alur data lapangan yang benar → F-2 peringatan dan tindakan → F-3 data eksternal → F-4 AI → F-5 fisik dan optimasi.
4. **Gerbang produksi** ([00 §7](00-cara-kerja.md)): migrasi nyata saat deploy, seed bersih, admin awal, cookie/CSRF, backup. Belum boleh deploy sebelum semuanya tercentang.
