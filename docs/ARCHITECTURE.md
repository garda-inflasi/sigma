# Arsitektur halaman & akses — AI Garda Inflasi

Dokumen ini memetakan **4 lapis blueprint** ke rute Next.js 16 App Router, komponen, sumber data, dan peran. Matriks akses di bawah dihasilkan langsung dari `lib/rbac` — sumber yang sama dengan `proxy.ts`, sidebar, palet perintah, dan aksi massal.

## 1. Pohon rute

```
app/
├─ layout.tsx                 font (Inter, Plus Jakarta Sans, JetBrains Mono), latar kontur, Providers, SessionProvider
├─ providers.tsx              TanStack Query · next-themes · MotionConfig(reducedMotion) · Tooltip · Sonner
├─ page.tsx                   redirect ke beranda peran
├─ manifest.ts                PWA "Garda Lapangan"
├─ offline/                   fallback service worker
├─ akses-ditolak/             403 kontekstual (peran + rute)
├─ (auth)/masuk/              SSO institusi + catatan MFA
├─ lapangan/                  ▸ LAYER 1 — PWA offline-first (FieldShell: indikator sinkron, nav bawah layoutId)
│  ├─ input/                  form harga per komoditas, validasi di perangkat, geofence, foto, kode etik
│  ├─ riwayat/                antrean perangkat + status server, mode pilih massal, penyelesaian konflik
│  └─ tugas/                  tugas survei, progres cincin, pengingat, unduh data acuan
└─ (console)/                 ConsoleShell: sidebar kaca, topbar, ⌘K, aliran EWS SSE, siklus ambient
   ├─ eksekutif/              ▸ LAYER 3 — hero koridor inflasi, neraca, ruang fiskal, XAI, ringkasan
   ├─ operasional/            ▸ LAYER 3 — KPI countUp, heatmap, feed EWS, peta, tren, antrean validasi
   ├─ data/harga/             ▸ LAYER 1 — validasi & publikasi massal, detail + timeline alur entri
   ├─ sumber-data/            ▸ LAYER 1 — pipeline medallion, mutu data, API & scraper
   ├─ master/[entitas]/       ▸ LAYER 1 — komoditas | wilayah | pasar, versioning & persetujuan
   ├─ ews/                    ▸ LAYER 2 — tabel alert massal, ambang & kanal notifikasi
   ├─ prediksi/               ▸ LAYER 2 — XAI, simulasi skenario, proyeksi semua komoditas, registri model
   ├─ peta/                   ▸ LAYER 2 — surplus–defisit, rute distribusi, kandidat cold storage
   ├─ cold-storage/           ▸ LAYER 2 — utilisasi & suhu fasilitas, stok + shelf-life + food loss
   ├─ intervensi/             ▸ LAYER 4 — kanban 3 siklus (dnd-kit, izin per tahap) + tampilan tabel
   ├─ policy-brief/           ▸ LAYER 3 — generator draf beranimasi, telaah, persetujuan
   ├─ kpi/                    ▸ KPI 4 kelompok blueprint
   ├─ pengguna/               ▸ Tata kelola — akun + matriks peran × izin
   ├─ audit/                  ▸ Tata kelola — audit trail
   └─ alur-sistem/            ▸ Peta alur sistem & data (diagram beranimasi)
```

Setiap `page.tsx` adalah Server Component tipis (metadata + `<Suspense>`); interaktivitas di komponen klien `components/<domain>/*-page.tsx`. Rute dinamis memakai `params: Promise<…>` (Next 15+/16).

## 2. Lapisan data di klien

| Lapisan | Berkas | Peran |
|---|---|---|
| Klien HTTP | `lib/api/client.ts` | `fetch` ke API Go (`NEXT_PUBLIC_API_BASE_URL`), cookie sesi, `Idempotency-Key` |
| Endpoint | `lib/api/endpoints.ts` | satu fungsi per sumber; otomatis ke mock bila base URL kosong |
| Mock | `lib/api/mock.ts` | data deterministik, identik dengan seeder |
| Query | `lib/query/keys.ts`, `hooks.ts` | key hierarkis, polling 30 dtk, `useBulkAction` optimistis + rollback |
| Real-time | `components/ews/use-ews-stream.ts` | SSE `/v1/ews/stream` (Redis pub/sub di Go) → cache + toast |
| Offline | `lib/stores/field-queue-store.ts`, `lib/offline/*` | Zustand + IndexedDB, sync engine idempoten, service worker |
| State UI | `lib/stores/*` | sidebar, palet ⌘K, kanban, status aliran EWS |

### Kontrak API yang diharapkan dari layanan Go

| Metode | Path | Dipakai oleh |
|---|---|---|
| GET | `/v1/siklus/aktif` | warna ambient seluruh UI |
| GET | `/v1/dashboard/operasional/{kpi,heatmap}` | dashboard operasional |
| GET | `/v1/dashboard/eksekutif` | dashboard eksekutif |
| GET | `/v1/spasial/wilayah`, `/v1/harga/tren` | peta & grafik tren |
| GET | `/v1/prediksi/{komoditas}` | panel XAI |
| GET/POST | `/v1/harga`, `/v1/lapangan/harga` (409 = konflik) | validasi & sinkronisasi lapangan |
| GET | `/v1/ews`, SSE `/v1/ews/stream` (event `alert`) | EWS |
| GET/PATCH | `/v1/intervensi`, `/v1/intervensi/{id}/tahap` | kanban |
| POST | `/v1/{sumber}/bulk` `{ aksi, ids }` → `{ berhasil, gagal }` | seluruh aksi massal |
| POST | `/v1/policy-brief/generate` | generator brief |
| GET | `/v1/auth/sso/start` | halaman masuk |

## 3. Peran (tanpa TPID/Pemda)

TPID dan Pemda **bukan pengguna sistem**. Keluaran untuk mereka (policy brief, rekomendasi APBD) disalurkan oleh Unit Hubungan Pemerintah & Kebijakan. Tahap kanban "Usulan intervensi logistik kilat" tidak lagi bergantung pada akun TPID.

Prinsip yang ditegakkan:
- **Least privilege** — izin granular `domain.aksi` (35 izin).
- **Pemisahan tugas** — penginput ≠ validator ≠ publisher; validator tidak dapat memvalidasi entrinya sendiri; Admin Sistem tanpa hak persetujuan bisnis.
- **Cakupan baris** — Koordinator & Agregator: kabupatennya; Mahasiswa: entrinya sendiri; lainnya: seluruh Aceh. Ditegakkan ulang di API Go.
- **Persetujuan berjenjang** — tahap "Petakan suplai" & "Selesai" butuh `intervensi.setujui` (Dewan Pengarah); "Dukungan instrumen belanja" hanya `intervensi.setujui_fiskal` (Kanwil DJPb).
- **MFA** wajib untuk lapis strategis, manajerial, koordinator, dan admin.

### Matriks rute × peran

| Rute | Izin | FMIPA | BI | BPS | DJPb | Prog | Data | Kebj | Biaya | Risk | Mutu | Koord | Agreg | Mhs | Admin |
|---|---|:-:|:-:|:-:|:-:|:-:|:-:|:-:|:-:|:-:|:-:|:-:|:-:|:-:|:-:|
| `/operasional` | `dashboard.operasional` | ✓ | ✓ | ✓ | ✓ | ✓ | ✓ | ✓ | ✓ | ✓ | ✓ | ✓ | ✓ | · | · |
| `/eksekutif` | `dashboard.eksekutif` | ✓ | ✓ | ✓ | ✓ | ✓ | · | ✓ | ✓ | · | · | · | · | · | · |
| `/data/harga` | `harga.lihat` | ✓ | ✓ | ✓ | ✓ | ✓ | ✓ | ✓ | ✓ | ✓ | ✓ | ✓ | ✓ | · | · |
| `/ews` | `ews.lihat` | ✓ | ✓ | ✓ | ✓ | ✓ | ✓ | ✓ | · | ✓ | ✓ | ✓ | ✓ | · | · |
| `/prediksi` | `prediksi.lihat` | ✓ | ✓ | ✓ | ✓ | ✓ | ✓ | ✓ | · | · | ✓ | · | · | · | · |
| `/peta` | `spasial.lihat` | ✓ | ✓ | ✓ | ✓ | ✓ | ✓ | ✓ | ✓ | · | · | ✓ | · | · | · |
| `/cold-storage` | `coldstorage.lihat` | ✓ | ✓ | ✓ | ✓ | ✓ | · | · | ✓ | ✓ | ✓ | · | ✓ | · | · |
| `/intervensi` | `intervensi.lihat` | ✓ | ✓ | ✓ | ✓ | ✓ | ✓ | ✓ | ✓ | ✓ | ✓ | ✓ | ✓ | · | · |
| `/policy-brief` | `brief.lihat` | ✓ | ✓ | ✓ | ✓ | ✓ | ✓ | ✓ | ✓ | · | · | · | · | · | · |
| `/kpi` | `kpi.lihat` | ✓ | ✓ | ✓ | ✓ | ✓ | ✓ | ✓ | ✓ | ✓ | ✓ | · | · | · | · |
| `/master` | `master.lihat` | ✓ | ✓ | ✓ | ✓ | ✓ | ✓ | · | · | ✓ | · | ✓ | · | · | ✓ |
| `/sumber-data` | `sumber.lihat` | ✓ | ✓ | ✓ | ✓ | ✓ | ✓ | · | · | · | · | · | · | · | ✓ |
| `/pengguna` | `pengguna.kelola` | · | · | · | · | · | · | · | · | · | · | · | · | · | ✓ |
| `/audit` | `audit.lihat` | ✓ | ✓ | ✓ | ✓ | ✓ | ✓ | · | · | ✓ | · | · | · | · | ✓ |
| `/lapangan` | `harga.input` | · | · | · | · | · | · | · | · | · | · | ✓ | ✓ | ✓ | · |
| `/alur-sistem` | — | ✓ | ✓ | ✓ | ✓ | ✓ | ✓ | ✓ | ✓ | ✓ | ✓ | ✓ | ✓ | ✓ | ✓ |

Keterangan singkatan: **FMIPA** = Pimpinan FMIPA USK; **BI** = Analis Bank Indonesia; **BPS** = Statistisi BPS; **DJPb** = Analis Kanwil DJPb; **Prog** = Manajer Program; **Data** = Unit Data & Statistik; **Kebj** = Unit Hubungan Pemerintah & Kebijakan; **Biaya** = Unit Pembiayaan & Investasi; **Risk** = Unit Monitoring & Risiko; **Mutu** = Unit Mutu & Pascapanen; **Koord** = Koordinator Lapangan; **Agreg** = Unit Agregator; **Mhs** = Agen Lapangan Mahasiswa; **Admin** = Administrator Sistem.

Aksi massal di tiap tabel juga difilter izin (mis. *Validasi* butuh `harga.validasi`, *Publikasikan* butuh `harga.publikasi`, *Promosikan model* butuh `model.kelola`). Matriks izin lengkap dapat dilihat di aplikasi: **Pengguna & peran → Matriks akses**.
