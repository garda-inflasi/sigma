# Garda Design System

Soft & smooth govtech: presisi, tenang, institusional, namun mudah didekati.

## Prinsip

1. **Satu keberanian per layar.** Dashboard eksekutif: inflasi diletakkan pada *koridor sasaran* (bukan angka besar biasa). Konsol: latar kontur topografi Gayo–pesisir. Sisanya tenang.
2. **Warna = makna, bukan dekorasi.** Navy (kepercayaan), teal *data*, emerald (normal), amber (waspada), rose (defisit), blue (surplus). Warna ambient seluruh UI mengikuti siklus ekonomi aktif lewat `data-siklus` di `<html>`.
3. **Gerak menjawab aksi.** Gerak otomatis hanya untuk menarik perhatian (EWS, data baru, satu orkestrasi KPI saat halaman dibuka). Semua menghormati `prefers-reduced-motion`.
4. **Angka selalu JetBrains Mono tabular** (`.num`); heading Plus Jakarta Sans; UI Inter. Tanpa label ALL CAPS, tanpa titik tengah sebagai pemisah meta.
5. **Semua tabel dapat dicentang.** Proses massal adalah pola utama, bukan fitur tambahan.

## Token (sumber: `app/globals.css`, cermin JS: `lib/design/tokens.ts`)

| Kelompok | Token | Contoh kelas |
|---|---|---|
| Navy 50–950 | `--color-navy-*` | `bg-navy-800`, `text-navy-900` |
| Data (teal) 50–950 | `--color-data-*` | `bg-data-500`, `shadow-glow-data` |
| Kabut (latar) | `--color-mist-50…400` | `bg-mist-100` |
| Status | `normal · waspada · defisit · surplus · kritis` (+`-soft`, `-ink`) | `bg-defisit-soft text-defisit-ink` |
| Siklus aktif | `--siklus`, `--siklus-soft`, `--siklus-ink` | `bg-siklus`, `ring-siklus` |
| Radius | kontrol `xl` 16px · kartu `2xl` 20px · panel `3xl` 28px · shell `4xl` 36px | `rounded-xl` |
| Bayangan | `ambient-xs/sm/ambient/lg`, `lift`, `glow-data/defisit/waspada` (bernuansa navy) | `shadow-ambient`, `hover:shadow-lift` |
| Kaca | `glass-subtle` · `glass` · `glass-strong` (backdrop-blur 16–28px + inset highlight) | `glass rounded-xl` |
| Tipe | `text-display-sm/md/lg/xl/2xl` (rasio 1.25, tracking negatif terkalibrasi) | `text-display-md font-semibold` |
| Easing | `--ease-garda` cubic-bezier(.22,1,.36,1) | `ease-garda` |

Dark mode tersedia (`.dark`, via next-themes) dengan token kaca terpisah.

## Gerak (`lib/motion/presets.ts`)

| Preset | Dipakai di |
|---|---|
| `spring.soft / snappy / bouncy / layout` | kartu · tombol & chip · pop-in & marker · indikator layoutId |
| `fadeUp` + `stagger()` | entrance satu kali per halaman (KPI, kartu fasilitas, KPI page) |
| `popIn` | badge hitungan, ikon sukses, chip validasi harga |
| `pulseRing()` | EWS (toast, feed, bell, pin wilayah defisit, siklus tekanan) |
| `hoverLift`, `buttonSpring` | `GlassCard interactive`, `SpringButton`/`SpringLink` |
| `drawPath` | `Sparkline` |
| `markerBounce` (custom=i) | pin peta Mapbox & skema SVG |
| `heatCell` (custom=baris+kolom) | gelombang diagonal heatmap |
| `feedItem`, `feedItemFlat` | feed EWS, riwayat lapangan |
| `bulkBar` | bilah aksi massal |
| `countUp` | `<CountUp>` — lanjut dari nilai sebelumnya saat refetch |

### Peta micro-interaction wajib

| Interaksi | Implementasi |
|---|---|
| Hover lift | `components/garda/glass-card.tsx` (`interactive`), kartu kanban |
| Button spring | `components/garda/spring-button.tsx` |
| CountUp KPI | `components/motion/count-up.tsx` |
| Chart draw | Recharts `animationDuration` + `Sparkline` pathLength |
| Pulse ring EWS | `components/motion/pulse-ring.tsx`, `components/ews/ews-toast.tsx` |
| Sidebar layoutId | `components/layout/sidebar.tsx` (`sidebar-active`), nav bawah `lapangan-nav` |
| Skeleton shimmer | utilitas `shimmer` + `components/garda/shimmer.tsx` |
| Sync rotating icon | `components/lapangan/offline-indicator.tsx`, riwayat (`animate-spin-slow`) |
| Map marker bounce-in stagger | `components/map/marker-pin.tsx` |

## Komponen inti

| Komponen | Catatan |
|---|---|
| `DataTable` | centang header (indeterminate), "Pilih seluruh N", Shift+klik rentang, Esc batal, facet, paginasi, `rowTone`, `hiddenColumns` |
| `BulkActionBar` | aksi difilter izin, `enabled()` per pilihan, konfirmasi destruktif, overflow menu |
| `Segmented` | pill aktif meluncur (layoutId) — tab, filter, tipe harga |
| `GlassCard`, `PageHeader`, `PanelTitle`, `StatusBadge`, `KomoditasIcon` | primitif tampilan |
| `CommandPalette` | ⌘K / Ctrl+K / `/`; aksi cepat, navigasi, komoditas, wilayah — difilter izin |
| `notifyEws()` | toast kustom Sonner; Kritis menetap, lainnya berbilah waktu |
| `DiagramAlur` | diagram alur beranimasi (garis tergambar + paket data) |
| `RegionMap` | Mapbox (react-map-gl v8) dengan fallback skema SVG tanpa token |

## Penulisan (Bahasa Indonesia profesional)

- Kalimat biasa (sentence case), kata kerja aktif: **Validasi**, **Publikasikan**, **Simpan entri** — nama aksi sama dengan toast hasilnya ("Entri divalidasi").
- Galat menjelaskan apa yang terjadi dan cara memperbaikinya, tanpa permintaan maaf: "Butuh persetujuan Kanwil DJPb".
- Layar kosong adalah ajakan bertindak: "Antrean validasi kosong … entri baru masuk otomatis".
- Angka Indonesia: `Rp52.000`, `2,84%`, `−3,4 poin`.

## Aksesibilitas

Fokus terlihat (`outline` teal), tautan "Lewati ke konten", `aria-live` untuk status sinkron & readout heatmap, target sentuh ≥44px di PWA, kontras teks ≥4.5:1 pada token `-ink`, gerak dimatikan oleh `MotionConfig reducedMotion="user"` + CSS `prefers-reduced-motion`.
