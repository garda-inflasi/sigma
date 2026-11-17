# Garda Design System

**v2 "Clean Clinical" (17 Nov 2026).** Bersih, lembut, dan sangat rapi: kanvas off-white, kartu putih dengan garis rambut, aksi utama near-black, aksen teal. Terinspirasi Linear / Notion / Attio. Sejak v2 tidak ada lagi kaca/blur; nama utilitas `glass*` dipertahankan tetapi kini permukaan datar.

## Prinsip

1. **Datar dan tenang.** Kanvas `#f8f9fa`, kartu `#fff` dengan garis `#eaecef` dan bayangan nyaris tak terlihat. Hanya elemen melayang (drawer, popover, toast, bilah aksi massal) yang terangkat.
2. **Warna = makna.** Aksi utama near-black `#18181b`; konfirmasi dan fokus teal `#0d9488`; status memakai pil pastel (latar lembut, teks pekat, titik kecil): normal/berhasil emerald, waspada amber, defisit/kritis merah, surplus/info biru. Warna ambient siklus ekonomi lewat `data-siklus` di `<html>` dipakai kecil-kecil (indikator, pil siklus), bukan latar.
3. **Gerak menjawab aksi.** Masuk halaman: fade + geser naik 10px, 300ms, easing `[0.22, 1, 0.36, 1]`, stagger 30–40ms pada KPI dan baris tabel. Semua menghormati `prefers-reduced-motion`.
4. **Angka tabular.** Seluruh angka memakai `.num` (digit tabular Inter). Heading Plus Jakarta Sans, UI Inter. Header tabel dan label grup sidebar memakai huruf kapital kecil (11px, tracking lebar, warna muted).
5. **Semua tabel dapat dicentang dan dikelompokkan.** Proses massal adalah pola utama; baris dapat dikelompokkan per status dengan header lipat.

## Token (sumber: `app/globals.css`, cermin JS: `lib/design/tokens.ts`)

| Kelompok | Token | Contoh kelas |
|---|---|---|
| "Navy" = rampa tinta netral 50–950 | `--color-navy-*` (900 teks `#111827`, 800 aksi utama `#18181b`) | `bg-navy-800`, `text-navy-900` |
| Data (deep teal) 50–950 | `--color-data-*` (600 `#0d9488`, 700 `#0f766e`) | `bg-data-600`, `text-data-700` |
| Kabut (kanvas, permukaan) | `--color-mist-50…400` | `bg-mist-100` |
| Status (pastel) | `normal · waspada · defisit · surplus · kritis` (+`-soft`, `-ink`) | `bg-defisit-soft text-defisit-ink` |
| Siklus aktif | `--siklus`, `--siklus-soft`, `--siklus-ink` | `bg-siklus` |
| Radius | tombol/badge 6px (`rounded-md`), kartu/input 8px (`rounded-lg`/`xl`), modal 12px (`rounded-2xl`) | `rounded-lg` |
| Bayangan | `ambient-xs/sm/ambient/lg`, `lift`, `glow-*` (cincin fokus 3px) | `shadow-ambient-xs` |
| Permukaan | `glass` (kartu), `glass-strong` (melayang), `glass-subtle` (tersier) | `glass rounded-lg` |
| Tipe | judul halaman `text-display-md` 20px, header seksi `text-display-sm` 14px, badan 13–14px, KPI `text-display-lg/xl` | `text-display-md font-semibold` |
| Easing | `--ease-garda` cubic-bezier(.22,1,.36,1) | `ease-garda` |

Dark mode tersedia (`.dark`, next-themes) dengan permukaan netral gelap (`#0c0d10` / `#131418`).

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
| `DataTable` | centang header (indeterminate), "Pilih seluruh N", Shift+klik rentang, Esc batal, facet, paginasi, `rowTone`, `hiddenColumns`, **`groupBy`** (grup berheader lipat dengan badge jumlah), baris masuk berstagger (`row-in`) |
| `BulkActionBar` | pil gelap mengambang di bawah layar ("N entri terpilih · aksi · ✕"); aksi difilter izin, `enabled()` per pilihan, konfirmasi destruktif, overflow menu |
| `Segmented` | kontrol segmen: trek abu, pil putih aktif meluncur (layoutId) — mode, filter, tipe harga |
| `ViewTabs` | tab tampilan bergaris bawah di bawah judul halaman (Papan/Tabel), ikon dan badge jumlah |
| `PageHeader` (`tabs`), `PanelTitle` | judul 20px, deskripsi 13px, aksi di kanan, tab tampilan di bawah; remah roti ada di topbar (`remahRoti`) |
| `Sidebar` + `WorkspaceSwitcher` | sidebar datar bergrup (label kapital kecil), pengalih ruang kerja (Provinsi Aceh + siklus), item aktif berupa kartu putih bergaris, ikon `strokeWidth` 1,75, dapat diciutkan ke 64px |
| `Sheet` + `InspectorSection`, `MetaGrid`/`MetaItem`, `ChecklistProgress`, `ActivityTimeline` | drawer inspector kanan (480px): metadata 2 kolom, checklist dengan bilah progres tipis, linimasa aktivitas |
| `OptionCards` | kartu pilihan tunggal (radio) dengan ikon dan indikator radio, untuk jenis/kategori di modal |
| `EmptyState`, `Shimmer`/`TableSkeleton`/`KpiRowSkeleton`/`ListSkeleton` | keadaan kosong dan kerangka pemuatan |
| `PageEnter`, `Stagger`/`StaggerItem` | animasi masuk halaman dan deretan kartu |
| `GlassCard`, `StatusBadge`, `KomoditasIcon` | primitif tampilan (`StatusBadge` = pil 6px berpenanda titik) |
| `PanelGalat` | galat query di dalam widget: pesan server berbahasa Indonesia (atau "Periksa sambungan internet" untuk galat jaringan) + tombol "Coba lagi"; data yang sudah tampil tidak dihapus bila polling berikutnya gagal |
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
