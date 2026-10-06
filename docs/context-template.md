## KONTEKS
Saya memerlukan desain UI/UX lengkap dan detail untuk platform "SIGMA" — platform terintegrasi pengendalian inflasi daerah Provinsi Aceh berbasis data & rantai pasok, bagian dari penelitian magister Kecerdasan Buatan FMIPA Universitas Syiah Kuala (USK).

Referensi utama: PRD (Product Requirements Document) "SIGMA" yang saya lampirkan (file: Blueprint_AI_Garda_Inflasi.pdf dan Uraian Blueprint SIGMA.docx). Seluruh kebutuhan fungsional, non-fungsional, arsitektur, role, dan fitur WAJIB mengacu pada PRD tersebut.

## STACK TEKNOLOGI (WAJIB)

### Framework & Library Utama
- **NextJS (latest)**
- **Tailwind CSS 3.x** (via CDN: `https://cdn.tailwindcss.com`)
- **shadcn/ui** — komponen di-replikasi manual dengan pattern API shadcn (menggunakan **Radix UI primitives** via ESM CDN + **class-variance-authority (CVA)** + **tailwind-merge** + **clsx**)
- **Lucide React** (via ESM CDN: `https://esm.sh/lucide-react`)
- **Recharts** (via ESM CDN: `https://esm.sh/recharts`)
- **Leaflet + React-Leaflet** (via ESM CDN)
- **SweetAlert2** (via CDN: `https://cdn.jsdelivr.net/npm/sweetalert2@11`)
- **Framer Motion** (via ESM CDN: `https://esm.sh/framer-motion`) — untuk micro-interactions smooth
- **Google Fonts: Inter** + **JetBrains Mono**

### Implementasi shadcn/ui dalam Single-File HTML
Karena shadcn/ui biasanya memerlukan build step, gunakan pendekatan berikut:
1. **Tailwind Config** didefinisikan inline via `tailwind.config = {...}` dengan design tokens dari PRD
2. **shadcn/ui components** direplikasi sebagai React components dalam `<script type="text/babel">` menggunakan Radix UI primitives dari ESM CDN
3. **CVA (class-variance-authority)** untuk variant management
4. **tailwind-merge + clsx** untuk className merging (fungsi `cn()`)
5. Semua komponen mengikuti **struktur & API shadcn/ui** (props, variant, size, asChild, dsb.)
6. **Dark mode** (opsional, default light) menggunakan CSS variables shadcn pattern

### Babel Standalone
Gunakan `https://unpkg.com/@babel/standalone/babel.min.js` untuk transform JSX in-browser.

## DELIVERABLE

**Format output:** HTML bundle single-file (`ai-garda-inflasi.html`, siap render di browser, tanpa build step), berisi:
- Multi-halaman dengan **Next Route** (HashRouter) ATAU state-based routing
- Semua komponen shadcn/ui yang relevan (Button, Card, Input, Table, Dialog, Sheet, DropdownMenu, Tabs, Badge, Alert, Toast, Checkbox, Select, Command, Popover, Tooltip, Switch, Slider, Progress, Avatar, Separator, Skeleton, Accordion, Collapsible, ScrollArea, Calendar, DatePicker, Form, Label, Textarea, RadioGroup, Toggle, ToggleGroup, NavigationMenu, Breadcrumb, Pagination, AlertDialog, ContextMenu, Menubar, HoverCard, Resizable, Sidebar, dsb.)
- Dark mode toggle (opsional, tetap sediakan)
- Responsive (mobile-first)

**Fidelity:** High-fidelity, production-ready visual (bukan wireframe).

**Interaktivitas:** Functional prototype — sidebar klik berfungsi, role switcher berfungsi, form submit, bulk action, sweet alert muncul, chart render, peta render, modal buka/tutup, toast notification.

**Bahasa:** Indonesia (utama). Format angka: Rp1.234.567, tanggal: 02/10/2026, zona waktu WIB.

**Mock data:** Realistis dan konsisten (lihat §G).

## SCOPE: SELURUH FITUR PRD (TANPA TERTINGGAL)

### A. HALAMAN GLOBAL (WAJIB)
1. **Login Page** — email + password, mock auth, ilustrasi SIGMA, branding FMIPA USK. Gunakan shadcn `Card`, `Input`, `Label`, `Button`, `Form`.
2. **Role Switcher** — dropdown di top-bar (shadcn `DropdownMenu`), 11 role (PRD §4.2) + "Public".
3. **404 / Empty State** — ilustrasi, pesan, CTA kembali. Gunakan shadcn `Empty` pattern (custom).
4. **Loading State** — shadcn `Skeleton` di semua halaman.
5. **Error State** — retry button, error message. Gunakan shadcn `Alert` variant destructive.
6. **Toast Notification** — shadcn `Toast` (Sonner-style) untuk notifikasi global.

### B. HALAMAN PER ROLE (SIDEBAR BERBEDA — WAJIB SEMUA 11 + PUBLIC)

Gunakan **shadcn/ui Sidebar component** (sidebar-07 pattern) dengan collapsible group, icon Lucide, dan active state. Setiap role punya konfigurasi sidebar unik:

#### ROLE 01 — DEWAN PENGARAH
Sidebar (shadcn SidebarGroup):
- 📊 Ringkasan Eksekutif → Executive Dashboard, Peta Neraca Pangan, Ruang Fiskal & Inflasi
- 📄 Kebijakan → Policy Brief Generator, Persetujuan Intervensi, Kalender Kebijakan
- 📈 Analitik Strategis → Proyeksi Inflasi Regional, Peta Surplus-Defisit, Evaluasi Triwulanan
- ⚙️ Tata Kelola → Risk Register, Audit Trail, Pengaturan Kelembagaan

#### ROLE 02 — MANAJER DATA & STATISTIK
- 🗄️ Data Pipeline → Monitoring Ingesti, Kualitas Data, Anomali & Pembersihan, Sumber Data (BPS/BI/Logistik/Fiskal)
- 🧠 AI Model Management → Model Registry, Training Pipeline, Evaluasi Model (RMSE/MAE/MAPE), Explainability (SHAP/LIME)
- ⚠️ Early Warning System → Konfigurasi Threshold, Sinyal Aktif, Log Alarm
- 📊 Dashboard Pokok → Ringkasan Statistik

#### ROLE 03 — MANAJER HUBUNGAN PEMERINTAH & KEBIJAKAN
- 📝 Policy Brief → Generator Otomatis (LLM), Draft & Reviu, Arsip Brief, Distribusi ke TPID/Pemda
- 🔗 Konversi Kebijakan → Temuan AI → Rekomendasi APBD, Tracking Tindak Lanjut, Matriks Sinergi Lembaga
- 📅 Koordinasi → Kalender Rapat, Notulen & Tindak Lanjut, Korespondensi Resmi

#### ROLE 04 — MANAJER PEMBIAYAAN & INVESTASI
- 💰 Studi Kelayakan → Kalkulator Kelayakan, Proyeksi ROI Infrastruktur, Simulasi Tarif
- 🏦 Pipeline Pembiayaan → Identifikasi Sumber Dana (CSR/Perbankan), Status Pengajuan, Monitoring Anggaran
- 📊 Analitik Fiskal → Dampak Ekonomi Daerah, Integrasi DJPb

#### ROLE 05 — MANAJER MONITORING & RISIKO
- 🔍 Audit & Verifikasi → Audit Silang Data Lapangan, Verifikasi BPS/BI, Log Audit
- ⚠️ Manajemen Risiko → Risk Register, Mitigasi Operasional, Eskalasi Risiko
- 🏭 Monitoring Gudang → Utilisasi Cold Storage, Food Loss Tracking, Stok Real-Time

#### ROLE 06 — MANAJER OPERASIONAL & LOGISTIK
- 🌾 Agregasi Produsen → Konsolidasi Pasokan, Transparansi Harga Dasar, Peta Sentra Produksi
- ❄️ Cold Storage → Peta Lokasi Sentra, Stok Real-Time, Manajemen Distribusi, Shelf-Life Monitoring
- 🚚 Logistik & Distribusi → Optimasi Rute (VRP), Network Flow, Estimasi Biaya
- 🔬 Mutu & Pascapanen → QC Computer Vision, Standar Kemasan, Reduksi Food Loss

#### ROLE 07 — KOORDINATOR LAPANGAN
- 👥 Manajemen Tim → Daftar Mahasiswa Agen, Jadwal Kunjungan, KPI Tim
- ✅ Verifikasi Input → Antrian Verifikasi, Validasi Telemetri, Cross-check BPS
- 📍 Peta Wilayah → Sebaran Kelompok Tani, Heatmap Aktivitas

#### ROLE 08 — MAHASISWA AGEN LAPANGAN (MOBILE ANDROID)
Drawer Navigation (shadcn `Sheet` — offline-first):
- 🏠 Beranda → Tugas Hari Ini, Ringkasan Input, Notifikasi
- 📝 Input Telemetri → Data Panen (Volume/Kalender), Harga Produsen, Geospasial Kelompok Tani, Unggah Foto
- 📚 Edukasi → Modul Literasi Digital, SOP Kode Etik, FAQ Produsen
- 👤 Profil → Data Diri, Riwayat Input, Sinkronisasi Offline

#### ROLE 09 — OPERATOR COLD STORAGE (MOBILE + TABLET)
- 📦 Manajemen Stok → Stok In/Out, Kapasitas Real-Time, Batch Tracking
- 🌡️ IoT Monitoring → Suhu & Kelembapan, Gas Sensor, Alert Anomali
- ⏳ Shelf-Life → Prediksi Masa Simpan, Prioritas Distribusi, Rekomendasi Intervensi

#### ROLE 10 — UNIT MUTU & PASCAPANEN (FMIPA)
- 🔬 Lab Analitik → Input Hasil Uji, Standar Mutu, Sertifikasi
- 📷 Computer Vision QC → Deteksi Cacat, Klasifikasi Mutu, Riwayat QC
- 📉 Reduksi Food Loss → Analitik Kehilangan, Rekomendasi Perbaikan

#### ROLE 11 — UNIT AGREGATOR PRODUSEN
- 📥 Agregasi → Input Volume, Verifikasi Produsen, Daftar Order
- 💵 Transparansi Harga → Harga Dasar Produsen, Riwayat Harga, Perbandingan Pasar

#### ROLE 12 — PUBLIC / OPEN DATA (TAMBAHAN DEMO)
- 🌐 Dashboard Publik → Ringkasan Inflasi, Peta Harga
- 📊 Transparansi → Data Terbuka, Unduh Dataset
- 📰 Berita & Pengumuman → Rilis Resmi, Edukasi Publik

**Catatan:** Role switcher di top-bar untuk demo (shadcn `DropdownMenu` + `Command` untuk search role).

### C. HALAMAN KHUSUS (WAJIB — DETAIL MATANG)

#### 1. COMMAND CENTER (Halaman Utama Dewan Pengarah)
**Layout (shadcn pattern):**
- Header: shadcn `Card` dengan title "Command Center — Pengendalian Inflasi Aceh" + live clock + `Badge` status sistem (hijau)
- Grid 3×3 menggunakan Tailwind Grid:
  - **Baris 1:** 4 `Card` KPI besar (Akurasi Prediksi 98%, Volatilitas Harga 4.1%, Food Loss 3.2%, Tindak Lanjut AI 78%) — masing-masing dengan `Badge` trend indicator (▲/▼) dan Recharts `Sparkline`
  - **Baris 2:** `Card` besar dengan Leaflet Map interaktif — overlay sentra produksi (hijau), defisit (merah), heatmap harga; layer toggles via shadcn `ToggleGroup`
  - **Baris 2 (kanan):** `Card` Feed EWS Real-Time — `ScrollArea` + list alarm dengan `Badge` (hijau/kuning/merah), timestamp, komoditas, wilayah, aksi cepat via `DropdownMenu`
  - **Baris 3:** `Card` Chart Proyeksi Harga 30 hari (Recharts `LineChart` + `Area` confidence interval) untuk 3 komoditas
  - **Baris 3 (kanan):** `Card` Quick Actions — 4 `Button` besar: "Approve Intervensi", "Generate Policy Brief", "Lihat Peta Neraca", "Ekspor Laporan"
- Footer: `Card` Status integrasi (BPS ●, BI ●, DJPb ●, IoT ●) dengan shadcn `Badge`

**Fitur:**
- Auto-refresh setiap 60 detik (interval JS)
- Toast notification untuk EWS baru (shadcn `Toast`)
- Klik EWS → shadcn `Dialog` detail dengan rekomendasi AI + tombol aksi
- Ekspor Command Center ke PDF (mock)

#### 2. EXECUTIVE DASHBOARD PER INSTITUSI (Tab-Based)
**Tab Navigation:** shadcn `Tabs` — FMIPA USK | Kanwil DJPb | BI | BPS | Public

**Tab FMIPA USK (Analitik Ekonomi):**
- KPI: 4 `Card` (Akurasi Model, Produsen Terverifikasi, Volume Agregasi, Riset Aktif)
- Chart: Recharts `BarChart` Performa Model AI (RMSE/MAE/MAPE)
- Tabel: shadcn `Table` Daftar Model Aktif + versi + metrik
- Peta: Leaflet Sebaran Kelompok Tani Binaan
- Panel: `Card` Publikasi & Riset Terbaru

**Tab Kanwil DJPb (Analitik Fiskal):**
- KPI: 4 `Card` (Realisasi Anggaran, Ruang Fiskal, Dampak Inflasi, Proyeksi APBD)
- Chart: Recharts `ComposedChart` Belanja Pemerintah vs Inflasi (dual axis)
- Tabel: `Table` Program Pembiayaan Aktif
- Panel: `Card` Rekomendasi Kebijakan Fiskal

**Tab BI (Indikator Harga):**
- KPI: 4 `Card` (Inflasi Regional, Indeks Harga, Suku Bunga, Cadangan Devisa)
- Chart: `LineChart` Tren Inflasi 12 bulan
- Tabel: `Table` Indikator Harga per Komoditas
- Panel: `Card` Analisis Kebijakan Moneter

**Tab BPS (Statistik Resmi):**
- KPI: 4 `Card` (Produksi Pangan, Inflasi YoY, Konsumsi, Ekspor)
- Chart: `BarChart` Produksi per Komoditas
- Tabel: `Table` Data Statistik Resmi
- Panel: `Card` Rilis Berita Resmi

**Tab Public (Open Data):**
- KPI: 4 `Card` (Harga Hari Ini, Inflasi Bulan Ini, Peta Harga, Jumlah Dataset)
- Chart: `LineChart` Tren Harga Publik
- Peta: Leaflet Harga per Wilayah
- `Button` Unduh Dataset (CSV/JSON)

#### 3. USER MANAGEMENT (Best Practices RBAC)
**Layout:**
- Header: `Card` title "User Management" + `Button` "+ Tambah User" + `Button` "Ekspor"
- Filter bar: shadcn `Input` (search), `Select` (role, institusi, status), `Button` Reset
- Tabel: shadcn `Table` dengan:
  - `Checkbox` (select all + per row)
  - `Avatar` + Nama
  - Email
  - `Badge` Role
  - `Badge` Institusi
  - `Switch` Status (Aktif/Nonaktif)
  - Last Login
  - `DropdownMenu` Aksi (Edit, Hapus, Reset Password, Lihat Detail)
- **Bulk Action Bar** (muncul saat ada checkbox terpilih) — shadcn `Card` floating di bottom:
  - "X item terpilih"
  - `Button` Aktifkan, Nonaktifkan, Hapus, Ekspor, Assign Role
  - `Button` "Batal Pilih"
- Pagination: shadcn `Pagination` (10/25/50/100 per halaman)
- Modal Create/Edit User: shadcn `Dialog` + `Form` + `Input` + `Label` + `Select` (multi) + `Switch` + `Avatar` upload
  - Password strength meter (custom)
- **SweetAlert2:**
  - Konfirmasi hapus (double): `Swal.fire({ icon: 'warning', title: 'Yakin hapus X user?', text: 'Tindakan ini tidak dapat dibatalkan.', input: 'text', inputPlaceholder: 'Ketik HAPUS untuk konfirmasi', ... })`
  - Sukses: `Swal.fire({ icon: 'success', title: 'User berhasil ditambahkan', timer: 2000 })`
  - Error: `Swal.fire({ icon: 'error', title: 'Gagal menyimpan', text: 'Silakan coba lagi.' })`

#### 4. ROLES MANAGEMENT
**Layout:**
- Header: `Card` title "Roles Management" + `Button` "+ Tambah Role"
- Tabel: shadcn `Table` dengan `Checkbox`, Nama Role, Deskripsi, Jumlah User, Jumlah Permission, `Badge` Status, `DropdownMenu` Aksi (Edit, Hapus, Duplikat, Lihat Permission)
- Modal Create/Edit Role: shadcn `Dialog` + `Form`
  - Nama Role, Deskripsi
  - **Permission Matrix:** shadcn `Table` dengan `Checkbox` per modul × aksi
  - Modul: Dashboard, Data, AI Model, EWS, Policy Brief, Intervensi, Logistik, Cold Storage, Mutu, User Mgmt, Audit
  - Aksi: Read, Write, Delete, Approve, Export
- Bulk Action Bar: Aktifkan, Nonaktifkan, Hapus, Duplikat
- SweetAlert2 untuk konfirmasi

#### 5. PERMISSIONS MANAGEMENT
**Layout:**
- Header: `Card` title "Permissions Management"
- **Matrix Resource × Action:** shadcn `Table` dengan `Checkbox` per sel
  - Baris: Resource (Dashboard, Telemetri, Harga, Logistik, Fiskal, Model AI, Policy Brief, Intervensi, User, Audit)
  - Kolom: Read, Write, Delete, Approve, Export
  - Group by Module: shadcn `Accordion` (collapsible)
- Bulk toggle per baris/kolom: `Checkbox` di header row/column
- Simpan otomatis dengan shadcn `Toast` notification
- Audit log perubahan permission: shadcn `Table` + `ScrollArea`

#### 6. HALAMAN FITUR LAINNYA (WAJIB DETAIL)

**Predictive Supply & Price Modeling:**
- Form: shadcn `Form` + `Select` (Komoditas, Wilayah), `Textarea` (Historical Prices CSV), `Input` (Historical Volumes), `Input` (Exogenous Features: weather, holiday, BBM)
- `Button` "Prediksi"
- Output: `Card` Predicted price, `Card` Confidence interval, `Card` Feature importance (Recharts `BarChart` SHAP), `Card` Explanation text
- Chart: Recharts `ComposedChart` Historical + Predicted line + CI band
- Tabel: shadcn `Table` Prediksi 7/30/90 hari

**AI Early Warning System:**
- Konfigurasi threshold: shadcn `Slider` untuk hijau/kuning/merah per komoditas
- Sinyal aktif: shadcn `Card` list dengan `Badge`, timestamp, komoditas, wilayah, probabilitas
- Log alarm: shadcn `Table` historis dengan `Select` filter
- Chart: Recharts `ScatterChart` Anomaly detection timeline

**Spatial & Supply Chain Optimization:**
- Peta: Leaflet + `Marker` (sentra produksi hijau, defisit merah) + `Polyline` (rute distribusi)
- Panel: shadcn `Card` dengan `Select` Komoditas, `Select` Wilayah
- Output: `Card` Peta surplus-defisit, `Card` Rekomendasi rute, `Card` Estimasi biaya
- Tabel: shadcn `Table` Rute optimal (biaya, waktu, jarak)

**Quality & Food Loss Analytics:**
- Form: shadcn `Form` + `Select` (Komoditas) + `Input` (Suhu, Kelembapan, Waktu simpan)
- Output: `Card` Prediksi shelf-life (hari), `Card` Rekomendasi waktu simpan
- Chart: Recharts `AreaChart` Shelf-life curve
- Tabel: shadcn `Table` Prioritas distribusi berdasarkan umur simpan

**Cold Storage Management:**
- Peta: Leaflet lokasi sentra
- Tabel: shadcn `Table` stok real-time (Batch ID, Komoditas, Volume, Suhu, Kelembapan, Shelf-life, Status)
- IoT Dashboard: shadcn `Card` dengan Recharts `RadialBarChart` (gauge suhu, kelembapan, gas)
- Alert anomali: `Card` list dengan `Badge`

**Logistik & Distribusi:**
- Form: shadcn `Form` + `Select` (Origin, Destination, Komoditas, Kendaraan) + `Input` (Volume)
- `Button` "Optimasi Rute"
- Output: `Card` Rute optimal (peta + list), `Card` Biaya, `Card` Waktu, `Card` Utilisasi kendaraan
- Tabel: shadcn `Table` Riwayat pengiriman

**Mutu & Pascapanen:**
- Upload foto: shadcn `Input type="file"` + preview
- Computer Vision QC: shadcn `Card` Deteksi cacat + `Badge` Klasifikasi mutu (A/B/C)
- Input hasil uji lab: shadcn `Form`
- Standar mutu: shadcn `Accordion` per komoditas
- Chart: Recharts `LineChart` Food loss reduction over time

**Agregasi Produsen:**
- Form: shadcn `Form` + `Select` Produsen + `Input` Volume
- Verifikasi produsen: shadcn `Dialog` (KTP, lokasi)
- Daftar order: shadcn `Table`
- Tabel: shadcn `Table` Agregasi per wilayah

**Transparansi Harga:**
- Tabel: shadcn `Table` harga dasar produsen real-time
- Chart: Recharts `LineChart` Riwayat harga
- Perbandingan: shadcn `Card` Perbandingan harga pasar
- Peta: Leaflet Harga per wilayah

**Input Telemetri (Mobile Agen Lapangan):**
- Form offline-first (shadcn `Form`):
  - `Select` Produsen (search)
  - `Input` Volume Panen (number + unit)
  - `Calendar` Kalender Panen (date range)
  - `Input` Harga Produsen (number)
  - Geospasial: auto-capture GPS + Leaflet Map picker
  - `Input type="file"` Foto (camera + gallery)
  - `Textarea` Catatan
- `Button` "Simpan" (offline) + `Button` "Sinkronisasi" (online)
- Indikator status koneksi: shadcn `Badge` + `Switch`
- Riwayat input: shadcn `Table` dengan `Badge` status verifikasi

**Policy Brief Generator:**
- Form: shadcn `Form` + `Select` (Periode, Komoditas, Wilayah, Fokus Eksekutif/Operasional)
- `Button` "Generate"
- Output: shadcn `Card` dengan rich text editor (contentEditable atau Textarea besar) berisi:
  - Ringkasan Eksekutif
  - Analisis Situasi
  - Rekomendasi Kebijakan
  - Lampiran Data
- `Button` Simpan Draft, Kirim ke TPID, Ekspor PDF
- Riwayat brief: shadcn `Table`

**Workflow Engine (Smart Intervention):**
- Visualisasi 3 siklus (Normal, Tekanan Harga, Surplus) dengan **shadcn Stepper** (custom) atau **Framer Motion** animated stepper
- Status siklus saat ini: `Badge` + `Progress`
- Log aktivitas per siklus: shadcn `Timeline` (custom) atau `Table`
- `Button` trigger manual
- Konfigurasi workflow: shadcn `Form`

**Audit Trail:**
- Tabel: shadcn `Table` log (Timestamp, User, Role, Action, Entity, IP, Status)
- Filter: shadcn `Calendar` (date range), `Select` (User, Action)
- Export CSV/PDF: `Button`
- Immutable (read-only)

**Risk Register:**
- Tabel: shadcn `Table` risiko (ID, Deskripsi, Kategori, Probabilitas, Dampak, Skor, Mitigasi, Owner, Status)
- Heatmap risiko: Recharts `ScatterChart` (probabilitas × dampak)
- Form: shadcn `Dialog` + `Form`
- Bulk action

**Feasibility Calculator:**
- Form: shadcn `Form` (Investasi awal, Arus kas, Discount rate, Periode)
- Output: shadcn `Card` (NPV, IRR, BCR, Payback Period)
- Chart: Recharts `BarChart` Cash flow projection
- Skenario: shadcn `Tabs` (Optimis, Moderat, Pesimis)

**Funding Pipeline:**
- Kanban board: **shadcn Card + Drag & Drop** (react-beautiful-dnd atau @dnd-kit via ESM CDN)
- Kolom: Identifikasi → Pengajuan → Review → Disetujui → Cair
- Card: Sumber dana, Jumlah, Status, PIC
- Tabel detail: shadcn `Table`

### D. FITUR WAJIB DI SEMUA TABLE (TANPA TERTINGGAL)

Setiap table di seluruh halaman WAJIB menggunakan **shadcn `Table` + `DataTable` pattern** (tanstack-table) atau custom, dengan fitur:

1. ✅ **Checkbox per row** + **Select All** — shadcn `Checkbox`
2. ✅ **Bulk Insert** — shadcn `Dialog` form multi-row ATAU CSV upload (drag & drop dengan `Input type="file"`)
3. ✅ **Bulk Update** — shadcn `Dialog` dengan `Select` field + `Input` value
4. ✅ **Bulk Delete** — shadcn `AlertDialog` + SweetAlert2 double confirmation (modal + ketik "HAPUS")
5. ✅ **SweetAlert2** untuk semua notifikasi:
   - Sukses (icon: success, auto-close 2s, timer: 2000)
   - Error (icon: error, dengan detail)
   - Warning (icon: warning, dengan konfirmasi)
   - Info (icon: info)
   - Konfirmasi (icon: question, tombol Ya/Tidak)
6. ✅ **Search** — shadcn `Input` dengan icon Search (real-time filter)
7. ✅ **Filter** — shadcn `Popover` + `Command` untuk multi-criteria filter
8. ✅ **Sort** — klik header kolom (asc/desc) dengan Lucide `ArrowUpDown`
9. ✅ **Pagination** — shadcn `Pagination` (10/25/50/100 per halaman)
10. ✅ **Empty State** — shadcn `Empty` pattern (custom) dengan ilustrasi + CTA
11. ✅ **Loading Skeleton** — shadcn `Skeleton` dengan shimmer effect
12. ✅ **Error State** — shadcn `Alert` variant destructive + retry button
13. ✅ **Export** — `DropdownMenu` CSV, Excel, PDF
14. ✅ **Column Visibility Toggle** — shadcn `DropdownMenu` + `CheckboxMenu`
15. ✅ **Row Density Toggle** — shadcn `ToggleGroup` (compact/comfortable)
16. ✅ **Keyboard Navigation** — arrow keys, Enter, Escape (via Radix)

**Bulk Action Bar** (shadcn pattern — floating di bottom, muncul saat ada checkbox terpilih):
- shadcn `Card` dengan shadow-lg, border, radius
- "X item terpilih"
- `Button` Edit Massal, Hapus, Ekspor, Aktifkan, Nonaktifkan, Assign
- `Button` "Batal Pilih"
- Animasi Framer Motion (slide up)

## E. DESIGN STYLE (TAILWIND + SHADCN CONFIG — WAJIB DIIKUTI)

### Tailwind Config (inline)
```javascript
tailwind.config = {
  darkMode: ["class"],
  theme: {
    extend: {
      colors: {
        border: "#E5E4E0",
        input: "#E5E4E0",
        ring: "#2E86AB",
        background: "#F5F4EF",
        foreground: "#1F1E1D",
        primary: {
          DEFAULT: "#1E3A5F",
          50: "#F0F4F8",
          100: "#D9E2EC",
          200: "#BCCCDC",
          500: "#1E3A5F",
          700: "#142A45",
          900: "#0A1622",
          foreground: "#FFFFFF",
        },
        secondary: {
          DEFAULT: "#2E86AB",
          50: "#EBF5FB",
          100: "#D6EAF8",
          500: "#2E86AB",
          700: "#1F5F7A",
          foreground: "#FFFFFF",
        },
        success: {
          DEFAULT: "#27AE60",
          50: "#E8F8F0",
          100: "#D1F2E0",
          500: "#27AE60",
          700: "#1E8449",
        },
        warning: {
          DEFAULT: "#F39C12",
          50: "#FEF5E7",
          100: "#FDEBD0",
          500: "#F39C12",
          700: "#B9770E",
        },
        danger: {
          DEFAULT: "#E74C3C",
          50: "#FDEDEC",
          100: "#FADBD8",
          500: "#E74C3C",
          700: "#B03A2E",
        },
        muted: {
          DEFAULT: "#6B6A68",
          foreground: "#6B6A68",
        },
        surface: "#FFFFFF",
        card: {
          DEFAULT: "#FFFFFF",
          foreground: "#1F1E1D",
        },
      },
      borderRadius: {
        sm: "6px",
        md: "10px",
        lg: "14px",
        xl: "20px",
      },
      boxShadow: {
        xs: "0 1px 2px rgba(30, 58, 95, 0.04)",
        sm: "0 1px 3px rgba(30, 58, 95, 0.06), 0 1px 2px rgba(30, 58, 95, 0.04)",
        md: "0 4px 6px rgba(30, 58, 95, 0.05), 0 2px 4px rgba(30, 58, 95, 0.04)",
        lg: "0 10px 15px rgba(30, 58, 95, 0.06), 0 4px 6px rgba(30, 58, 95, 0.04)",
        xl: "0 20px 25px rgba(30, 58, 95, 0.08), 0 10px 10px rgba(30, 58, 95, 0.04)",
      },
      fontFamily: {
        sans: ["Inter", "system-ui", "-apple-system", "Segoe UI", "sans-serif"],
        mono: ["JetBrains Mono", "monospace"],
      },
      spacing: {
        xs: "4px",
        sm: "8px",
        md: "16px",
        lg: "24px",
        xl: "32px",
        "2xl": "48px",
        "3xl": "64px",
      },
      keyframes: {
        shimmer: {
          "0%": { backgroundPosition: "-1000px 0" },
          "100%": { backgroundPosition: "1000px 0" },
        },
      },
      animation: {
        shimmer: "shimmer 1.5s infinite linear",
      },
    },
  },
};
```

### CSS Variables (shadcn pattern)
```css
:root {
  --background: 40 20% 95%;
  --foreground: 30 5% 12%;
  --card: 0 0% 100%;
  --card-foreground: 30 5% 12%;
  --popover: 0 0% 100%;
  --popover-foreground: 30 5% 12%;
  --primary: 210 51% 24%;
  --primary-foreground: 0 0% 100%;
  --secondary: 200 60% 42%;
  --secondary-foreground: 0 0% 100%;
  --muted: 40 10% 92%;
  --muted-foreground: 30 3% 42%;
  --accent: 200 60% 42%;
  --accent-foreground: 0 0% 100%;
  --destructive: 6 78% 57%;
  --destructive-foreground: 0 0% 100%;
  --border: 40 10% 90%;
  --input: 40 10% 90%;
  --ring: 200 60% 42%;
  --radius: 10px;
}
```

### Komponen shadcn/ui yang Wajib Diimplementasikan
Minimal implementasi komponen berikut dengan pattern shadcn:
1. `Button` (variant: default, secondary, destructive, outline, ghost, link; size: sm, default, lg, icon)
2. `Card` (Header, Title, Description, Content, Footer)
3. `Input`
4. `Textarea`
5. `Label`
6. `Select` (Radix)
7. `Checkbox` (Radix)
8. `RadioGroup` (Radix)
9. `Switch` (Radix)
10. `Slider` (Radix)
11. `Badge` (variant: default, secondary, destructive, outline, success, warning)
12. `Avatar` (Radix)
13. `Table` (Header, Body, Row, Head, Cell, Caption)
14. `Dialog` (Radix) — Trigger, Content, Header, Title, Description, Footer, Close
15. `AlertDialog` (Radix) — untuk konfirmasi
16. `Sheet` (Radix) — untuk drawer mobile
17. `DropdownMenu` (Radix) — Trigger, Content, Item, Separator, Label
18. `ContextMenu` (Radix)
19. `Popover` (Radix)
20. `Tooltip` (Radix)
21. `Tabs` (Radix) — List, Trigger, Content
22. `Accordion` (Radix)
23. `Collapsible` (Radix)
24. `ScrollArea` (Radix)
25. `Separator` (Radix)
26. `Skeleton`
27. `Progress` (Radix)
28. `Toast` (Radix) — Provider, Viewport, Toast, Title, Description, Close, Action
29. `Alert`
30. `Breadcrumb`
31. `Pagination`
32. `Command` (cmdk) — untuk global search
33. `Calendar` (react-day-picker)
34. `DatePicker`
35. `Form` (react-hook-form pattern)
36. `NavigationMenu` (Radix)
37. `Sidebar` (shadcn sidebar-07 pattern) — Sidebar, SidebarHeader, SidebarContent, SidebarGroup, SidebarGroupLabel, SidebarGroupContent, SidebarMenu, SidebarMenuItem, SidebarMenuButton, SidebarMenuSub, SidebarTrigger, SidebarInset, SidebarProvider
38. `Sonner` (untuk toast) — alternatif

### Design Language
- **Style:** Soft, smooth, modern, calm, professional
- **Referensi:** Linear.app, Notion, Vercel Dashboard, Muji, shadcn/ui examples
- **Feel:** Clean, minimal, data-dense tapi tidak overwhelming
- **Micro-interactions:** Hover halus (200ms ease), transisi smooth, ripple effect pada tombol (Framer Motion)
- **Spacing:** Konsisten dengan skala 4-8-16-24-32-48-64
- **Radius:** Soft (md=10px, lg=14px, xl=20px)
- **Shadow:** Soft, low opacity, tidak ada shadow keras

### Tipografi
- **Font Family:** Inter (Google Fonts) + fallback system-ui
- **Monospace:** JetBrains Mono
- **H1:** 32px / 700 / -0.02em
- **H2:** 24px / 600 / -0.01em
- **H3:** 18px / 600 / 0
- **H4:** 16px / 600 / 0
- **Body:** 14px / 400 / 0
- **Body Small:** 13px / 400 / 0
- **Caption:** 12px / 400 / 0
- **Overline:** 11px / 600 / 0.05em / uppercase

## F. RESPONSIVE & AKSESIBILITAS

### Breakpoints (Tailwind default)
```
sm: 640px   md: 768px   lg: 1024px   xl: 1280px   2xl: 1536px
```

### Responsive Behavior
- **Mobile (<640px):** Sidebar → shadcn `Sheet` (drawer), table → card view, chart → simplified, peta → full width
- **Tablet (640-1023px):** Sidebar → collapsible, table → horizontal scroll, chart → medium
- **Desktop (1024-1439px):** Sidebar → expanded, table → full, chart → detailed
- **Wide (1440px+):** Multi-column layout, chart → side-by-side

### Aksesibilitas (WCAG 2.1 AA)
- Kontras minimal 4.5:1 untuk teks normal, 3:1 untuk teks besar
- Fokus state jelas (ring 3px `ring-secondary-100`)
- ARIA labels untuk semua komponen interaktif (Radix sudah menyediakan)
- Keyboard navigation penuh (Tab, Shift+Tab, Enter, Escape, Arrow keys) — Radix default
- Skip to content link
- Alt text untuk semua gambar
- Form label terhubung dengan input (`Label` + `htmlFor`)
- Error message dengan `role="alert"`
- Screen reader friendly
- `prefers-reduced-motion` dihormati

## G. KONTEN & MOCK DATA

### Data Komoditas
```json
[
  { "id": "cabai-merah", "name": "Cabai Merah", "unit": "kg", "harga": 58000, "icon": "🌶️" },
  { "id": "bawang-merah", "name": "Bawang Merah", "unit": "kg", "harga": 38000, "icon": "🧅" },
  { "id": "beras", "name": "Beras", "unit": "kg", "harga": 13500, "icon": "🌾" }
]
```

### Data Wilayah (20 kabupaten/kota Aceh)
```json
[
  { "id": "banda-aceh", "name": "Banda Aceh", "lat": 5.5483, "lng": 95.3238, "status": "normal" },
  { "id": "aceh-besar", "name": "Aceh Besar", "lat": 5.3833, "lng": 95.5167, "status": "warning" },
  { "id": "pidie", "name": "Pidie", "lat": 5.3833, "lng": 95.9333, "status": "critical" },
  { "id": "pidie-jaya", "name": "Pidie Jaya", "lat": 5.3167, "lng": 96.1833, "status": "normal" },
  { "id": "bireuen", "name": "Bireuen", "lat": 5.2000, "lng": 96.7000, "status": "normal" },
  { "id": "lhokseumawe", "name": "Lhokseumawe", "lat": 5.1800, "lng": 97.1500, "status": "normal" },
  { "id": "aceh-utara", "name": "Aceh Utara", "lat": 5.0167, "lng": 97.1333, "status": "warning" },
  { "id": "aceh-timur", "name": "Aceh Timur", "lat": 4.6167, "lng": 97.7667, "status": "normal" },
  { "id": "langsa", "name": "Langsa", "lat": 4.4667, "lng": 97.9667, "status": "normal" },
  { "id": "aceh-tamiang", "name": "Aceh Tamiang", "lat": 4.3167, "lng": 98.0833, "status": "normal" },
  { "id": "gayo-lues", "name": "Gayo Lues", "lat": 3.9333, "lng": 97.5000, "status": "normal" },
  { "id": "aceh-tengah", "name": "Aceh Tengah", "lat": 4.5167, "lng": 96.8500, "status": "normal" },
  { "id": "bener-meriah", "name": "Bener Meriah", "lat": 4.7500, "lng": 96.8667, "status": "normal" },
  { "id": "aceh-barat", "name": "Aceh Barat", "lat": 4.4500, "lng": 96.1833, "status": "warning" },
  { "id": "nagan-raya", "name": "Nagan Raya", "lat": 4.1667, "lng": 96.3167, "status": "normal" },
  { "id": "aceh-jaya", "name": "Aceh Jaya", "lat": 4.8000, "lng": 95.6667, "status": "normal" },
  { "id": "aceh-selatan", "name": "Aceh Selatan", "lat": 3.1667, "lng": 97.4167, "status": "normal" },
  { "id": "aceh-singkil", "name": "Aceh Singkil", "lat": 2.2833, "lng": 97.7833, "status": "normal" },
  { "id": "simeulue", "name": "Simeulue", "lat": 2.6167, "lng": 96.0833, "status": "normal" },
  { "id": "sabang", "name": "Sabang", "lat": 5.8933, "lng": 95.3214, "status": "normal" }
]
```

### Data Produsen (30+ dummy)
Contoh pola: `Kelompok Tani Makmur`, `Gapoktan Sejahtera`, `Kelompok Tani Subur`, `Gapoktan Bina Usaha`, `Kelompok Tani Tani Jaya`, `Gapoktan Sumber Rezeki`, `Kelompok Tani Mekar`, `Gapoktan Harapan Baru`, `Kelompok Tani Bersama`, `Gapoktan Karya Tani`, dst. dengan atribut: id, name, komoditas, wilayah, volume, verified.

### Data User (20+ dummy)
Contoh: Dr. Ir. Ahmad Fauzi, M.Sc. (Dewan Pengarah, FMIPA USK); Siti Nurhaliza, S.Kom. (Manajer Data, FMIPA USK); Budi Santoso, S.E. (Manajer Kebijakan, TPID); Dewi Lestari, M.M. (Manajer Pembiayaan, DJPb); Rudi Hartono, S.T. (Manajer Operasional, FMIPA USK); dst.

### Data EWS
```json
[
  { "id": "EWS001", "level": "merah", "komoditas": "Cabai Merah", "wilayah": "Pidie", "probabilitas": 0.92, "timestamp": "2026-10-02 10:23", "pesan": "Potensi lonjakan harga >15% dalam 7 hari" },
  { "id": "EWS002", "level": "kuning", "komoditas": "Bawang Merah", "wilayah": "Aceh Besar", "probabilitas": 0.68, "timestamp": "2026-10-02 09:45", "pesan": "Stok menurun 20% dari rata-rata" },
  { "id": "EWS003", "level": "hijau", "komoditas": "Beras", "wilayah": "Bireuen", "probabilitas": 0.21, "timestamp": "2026-10-02 08:12", "pesan": "Stok stabil" }
]
```

### Data KPI
```json
{
  "akurasiPrediksi": 98,
  "volatilitasHarga": 4.1,
  "foodLoss": 3.2,
  "tindakLanjutAI": 78,
  "utilisasiColdStorage": 75,
  "inflasiRegional": 4.2,
  "produksiPangan": 12500,
  "marginHarga": 32
}
```

## H. OUTPUT YANG DIHARAPKAN

1. **1 file HTML bundle** (`ai-garda-inflasi.html`) berisi:
   - **NextJS (latest) + Babel Standalone** via CDN
   - **Tailwind CSS CDN** dengan config inline (design tokens PRD)
   - **shadcn/ui components** direplikasi manual (lihat §E) menggunakan Radix UI + CVA + tailwind-merge + clsx
   - **Semua halaman** (login, dashboard per role, command center, executive dashboard per institusi, user management, roles, permissions, semua fitur lainnya)
   - **Routing** via state-based (React useState) atau HashRouter
   - **Semua fitur bulk action + SweetAlert2**
   - **Chart & peta berfungsi** (Recharts + Leaflet)
   - **Role switcher berfungsi** (11 role + Public)
   - **Responsive & aksesibel**
   - **Dark mode toggle** (opsional)

2. **Dokumentasi singkat** (di dalam HTML atau markdown terpisah):
   - Struktur halaman
   - Mapping role → menu
   - Cara switch role
   - Design decision
   - Catatan penggunaan
   - Daftar komponen shadcn yang diimplementasikan

3. **Peta Menu Semua Role** — halaman khusus yang menampilkan matrix role × menu (seperti PRD §5.4)

## I. YANG TIDAK PERLU

- ❌ Backend / API nyata
- ❌ Autentikasi nyata (mock saja)
- ❌ Database nyata (hardcoded JSON di dalam script)
- ❌ Deployment
- ❌ Build step (langsung buka di browser)

## J. CATATAN PENTING

1. **Semua fitur di PRD WAJIB ada** — jangan ada yang tertinggal.
2. **Semua role WAJIB punya sidebar berbeda** — jangan disamakan.
3. **Bulk action + SweetAlert2 WAJIB di semua table** — tanpa kecuali.
4. **Design soft & smooth** — hindari warna mencolok, gunakan shadow halus, radius besar, transisi smooth.
5. **Detail setiap halaman harus matang** — bukan hanya placeholder, tapi konten realistis.
6. **Role switcher untuk demo** — mudah diakses di top-bar.
7. **Command Center & Executive Dashboard** — halaman prioritas, buat sangat detail.
8. **User Management, Roles, Permissions** — ikuti best practices RBAC.
9. **Mock data realistis** — nama Indonesia, harga realistis, tanggal 2026.
10. **Responsive & aksesibel** — WCAG 2.1 AA.
11. **Gunakan shadcn/ui** — jangan buat komponen dari nol jika shadcn sudah punya pattern-nya. Replikasi API shadcn (variant, size, asChild) agar konsisten.
12. **Tailwind config inline** — jangan gunakan config default, gunakan design tokens dari PRD.
13. **Framer Motion** — untuk micro-interactions (hover, modal, toast, sidebar).
14. **Lucide React** — untuk semua icon (konsisten, tidak campur dengan icon library lain).
15. **Dark mode** — sediakan toggle, default light.

## K. REFERENSI VISUAL

- **shadcn/ui** — https://ui.shadcn.com (komponen, pattern, contoh dashboard)
- **Linear.app** — sidebar, command palette, micro-interactions
- **Notion** — clean layout, typography
- **Vercel Dashboard** — data density, chart
- **Muji** — soft, minimal, calm
- **Stripe Dashboard** — table, bulk action
- **SweetAlert2** — alert style
- **Radix UI** — primitives documentation

## L. STRUKTUR FILE YANG DIHARAPKAN

```
ai-garda-inflasi.html
├── <head>
│   ├── Meta tags
│   ├── Google Fonts (Inter, JetBrains Mono)
│   ├── Tailwind CDN + inline config
│   ├── shadcn CSS variables
│   ├── Leaflet CSS
│   └── SweetAlert2 CSS
├── <body>
│   └── <div id="root"></div>
├── <script type="importmap">
│   └── React, ReactDOM, Radix, CVA, clsx, tailwind-merge, Lucide, Recharts, Leaflet, Framer Motion
├── <script type="text/babel">
│   ├── // === UTILS ===
│   │   ├── cn() function (clsx + tailwind-merge)
│   │   └── formatRupiah(), formatDate(), dll.
│   ├── // === SHADCN COMPONENTS ===
│   │   ├── Button, Card, Input, Label, Select, Checkbox, Switch, Slider, Badge, Avatar
│   │   ├── Table, Dialog, AlertDialog, Sheet, DropdownMenu, Popover, Tooltip
│   │   ├── Tabs, Accordion, Collapsible, ScrollArea, Separator, Skeleton, Progress
│   │   ├── Toast, Alert, Breadcrumb, Pagination, Command, Calendar, DatePicker
│   │   └── Sidebar (sidebar-07 pattern)
│   ├── // === MOCK DATA ===
│   │   ├── commodities, regions, producers, users, ews, kpi
│   │   └── models, roles, permissions, risks, funding, dll.
│   ├── // === SHARED COMPONENTS ===
│   │   ├── AppSidebar (dynamic per role)
│   │   ├── TopBar (role switcher, search, notif, profile)
│   │   ├── DataTable (dengan bulk action + sweet alert)
│   │   ├── KpiCard, ChartCard, MapCard
│   │   └── EmptyState, LoadingSkeleton, ErrorState
│   ├── // === PAGES PER ROLE ===
│   │   ├── LoginPage
│   │   ├── CommandCenter (Dewan Pengarah)
│   │   ├── ExecutiveDashboard (tab per institusi)
│   │   ├── UserManagement
│   │   ├── RolesManagement
│   │   ├── PermissionsManagement
│   │   ├── DataPipeline, AIModelMgmt, EWSConfig
│   │   ├── PolicyBriefGenerator, KonversiKebijakan
│   │   ├── StudiKelayakan, PipelinePembiayaan
│   │   ├── AuditTrail, RiskRegister, MonitoringGudang
│   │   ├── AgregasiProdusen, ColdStorage, Logistik, Mutu
│   │   ├── ManajemenTim, VerifikasiInput, PetaWilayah
│   │   ├── InputTelemetri (mobile)
│   │   ├── LabAnalitik, ComputerVisionQC, ReduksiFoodLoss
│   │   ├── InputVolume, TransparansiHarga
│   │   ├── DashboardPublik, Transparansi, Berita
│   │   └── PetaMenuSemuaRole
│   └── // === APP ===
│       ├── App (router + role state)
│       └── ReactDOM.createRoot(...).render(<App />)
```

**MULAI GENERASI SEKARANG. BUAT SEMUA HALAMAN, SEMUA FITUR, SEMUA FLOW TANPA TERTINGGAL. GUNAKAN TAILWIND + SHADCN/UI DENGAN DESIGN TOKENS DARI PRD. JIKA ADA YANG TIDAK JELAS, ASUMSIKAN YANG PALING PROFESIONAL DAN BEST PRACTICE.**
```
