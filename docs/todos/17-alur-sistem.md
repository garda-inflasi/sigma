# 17 — Alur sistem & data

**Prioritas:** P2 (dokumentasi interaktif) · **Status:** UI ✅ · Backend n/a · Migration & seeder: **tidak ada**
**Cara kerja & DoD:** [00-cara-kerja.md](00-cara-kerja.md)

## Yang sudah ada

- **UI:** [alur-page.tsx](../../apps/web/src/components/alur/alur-page.tsx) dan [flow.tsx](../../apps/web/src/components/flow/flow.tsx) di `/alur-sistem`: diagram beranimasi alur sistem, data, dan siklus. Dapat diakses semua peran.
- Sepenuhnya statis (memakai `lib/data/siklus.ts`). Tidak ada API dan memang tidak diperlukan. Satu-satunya keterkaitan data: bila definisi tahap siklus pindah ke database ([14-A](14-intervensi.md)), diagram ini sebaiknya ikut membacanya agar tidak menyimpang.

---

## 17-A — Sinkronkan dengan kenyataan *(setelah Fase P)*

Tidak ada migration, seeder, maupun API.

- [ ] Perbarui node dan alur agar sesuai arsitektur nyata (layanan ingestion, layanan ML, Redis, object storage).
- [ ] Beri penanda "rencana" pada node yang belum diimplementasikan supaya pengguna tidak salah paham.
- [ ] Aksesibilitas: alternatif teks untuk diagram; patuhi `prefers-reduced-motion` (MotionConfig sudah ada di providers).
- [ ] Tautan dari node ke halaman fitur terkait.
- [ ] Sinkronkan dengan [FLOW.md](../FLOW.md) dan [ARCHITECTURE.md](../ARCHITECTURE.md), atau hasilkan semuanya dari satu sumber data.
