# AI Garda Inflasi — Web & PWA

Design system + implementasi UI platform pengendalian inflasi daerah (FMIPA USK × Kanwil DJPb) dengan arsitektur 4 lapis: ingesti data, analitik AI, dashboard & DSS, dan smart intervention.

**Stack:** Next.js 16 (App Router, `proxy.ts`) · React 19 · Tailwind CSS v4 · shadcn/ui · Motion (penerus Framer Motion, `motion/react`) · Recharts 3 · Mapbox GL + react-map-gl 8 · TanStack Query 5 & Table 8 · Zustand 5 · Sonner · dnd-kit · cmdk · Prisma 6 (PostgreSQL).

## Mulai cepat

```bash
npm install
npm run ui:add            # memasang primitif shadcn (checkbox, command, dialog, sheet, select, …)
cp .env.example .env.local
npm run dev               # http://localhost:3000 — mode mock aktif bila NEXT_PUBLIC_API_BASE_URL kosong
```

Di mode pengembangan, gunakan **"Lihat sebagai"** di topbar untuk berpindah antar 14 peran dan meninjau RBAC di setiap halaman. Tekan **⌘K / Ctrl+K** untuk palet perintah; pilih *Simulasikan alert EWS kritis* untuk melihat toast berdenyut.

### Database & seeder

```bash
# DATABASE_URL di .env
npm run db:push           # atau db:migrate
npm run db:seed           # ±33 ribu baris; password awal dicetak (atau set SEED_PASSWORD)
```

Seeder (`prisma/seed.ts`) memakai generator deterministik yang sama dengan mode mock, sehingga UI mock dan database menampilkan skenario identik (per 3 Oktober 2026: cabai tertekan pascacurah hujan ekstrem Gayo, kentang & tomat surplus).

## Peta deliverable

| # | Deliverable | Berkas |
|---|---|---|
| 1 | Token Tailwind (warna garda, tipe, bayangan, radius) | `app/globals.css` (@theme v4), `lib/design/tokens.ts` |
| 2 | Preset motion | `lib/motion/presets.ts`, `components/motion/*` |
| 3 | PWA lapangan (input, riwayat, tugas, indikator offline) | `app/lapangan/*`, `components/lapangan/*`, `lib/offline/*`, `public/sw.js`, `app/manifest.ts` |
| 4 | Dashboard operasional | `app/(console)/operasional`, `components/dashboard/*`, `components/ews/ews-live-feed.tsx`, `components/map/*` |
| 5 | Dashboard eksekutif | `app/(console)/eksekutif`, `components/eksekutif/*` |
| 6 | Kanban intervensi 3 siklus | `app/(console)/intervensi`, `components/intervensi/*`, `lib/data/siklus.ts` |
| 7 | Command palette ⌘K | `components/command/command-palette.tsx` |
| 8 | Toast EWS + pulse ring | `components/ews/ews-toast.tsx`, `components/ews/use-ews-stream.ts` |
| 9 | Prisma schema & seeder | `prisma/schema.prisma`, `prisma/seed.ts`, `lib/data/reference.ts`, `lib/data/generators.ts` |
| 10 | Arsitektur halaman & alur | `docs/ARCHITECTURE.md`, `docs/FLOW.md`, `/alur-sistem` |
| — | Tabel massal di semua daftar | `components/data-table/*` |
| — | RBAC tanpa TPID/Pemda | `lib/rbac/*`, `proxy.ts`, `docs/ARCHITECTURE.md` |

Panduan visual & penulisan: `docs/DESIGN-SYSTEM.md`.

## Integrasi backend Go

Set `NEXT_PUBLIC_API_BASE_URL` ke layanan Go. Kontrak endpoint ada di `docs/ARCHITECTURE.md §2`. Ringkasnya: REST JSON + ISO 8601, sesi JWT di cookie httpOnly `garda_session` (diverifikasi `proxy.ts` dengan `GARDA_JWT_SECRET`), aksi massal `POST /v1/{sumber}/bulk`, aliran EWS via SSE (`event: alert`), sinkronisasi lapangan idempoten dengan `Idempotency-Key` dan respons 409 untuk konflik. Prisma di repo ini berfungsi sebagai sumber skema/migrasi + seeder; tabel memakai nama snake_case agar mudah dibaca sqlc/pgx.

## Catatan versi

- **Tailwind v4**: konfigurasi berada di CSS (`@theme`). Tidak ada `tailwind.config.ts`.
- **Prisma dipin ke 6.19** (API `prisma-client-js` klasik). Untuk Prisma 7+: pindahkan konfigurasi ke `prisma.config.ts` dan gunakan driver adapter `@prisma/adapter-pg`.
- **TanStack Table dipin ke v8** (API yang dipakai shadcn data-table).
- **Mapbox** opsional: tanpa `NEXT_PUBLIC_MAPBOX_TOKEN`, peta memakai skema SVG Aceh (tidak berskala).
- Service worker hanya didaftarkan di build produksi (`npm run build && npm start`).
- Koordinat pasar & data harga bersifat sintetis untuk desain/demo; ganti dengan data resmi sebelum operasional.
