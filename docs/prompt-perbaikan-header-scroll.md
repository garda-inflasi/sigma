# Prompt: Perbaikan Header & Smooth Scrolling — Garda Inflasi

## 1. Perbaiki Header (Topbar) — Lebih Smooth & Profesional

Header saat ini (`components/layout/topbar.tsx`) terasa kaku dan kurang profesional. Perbaiki dengan kriteria berikut:

### a. Gaya kekinian untuk header
- Header harus terasa lebih ringan, modern, dan profesional — cocok untuk platform data/institusi pemerintah.
- Gunakan glassmorphism yang lebih halus (transparansi lebih tinggi, blur lebih ringan).
- Hilangkan kesan "kotak kaku" — border-radius boleh dipertahankan tapi transisi hover/active harus lebih mulus.
- Pertimbangkan efek `backdrop-filter` yang lebih ringan agar tidak berat saat scroll.

### b. Title tidak double dengan konten halaman
- Saat ini title di header (dari `activeNav(pathname, NAV_FLAT)?.label`) sering tampil duplikat dengan judul yang sudah ada di konten halaman masing-masing.
- **Solusi**: Header tidak perlu menampilkan title halaman lagi. Cukup tampilkan:
  - Brand/logo (Garda Inflasi) — ringkas, tanpa tulisan panjang
  - Tanggal (boleh dipertimbangkan untuk di-takeout jika tidak perlu)
  - Elemen aksi: pencarian, notifikasi, avatar/user menu
- Atau alternatif: title di header dibuat lebih subtle (smaller, muted) agar tidak bersaing dengan konten.

### c. Tanggal — pertimbangkan untuk dihapus
- Tanggal di header (`{today}`) boleh dipertimbangkan untuk dihapus karena:
  - Tidak memberikan nilai fungsional yang signifikan
  - Menambah clutter visual
  - User sudah bisa melihat tanggal dari sistem/browser
- Jika dipertahankan, buat lebih minimal (format pendek, opacity rendah).

### d. Siklus badge
- Badge siklus (tekanan/normal/surplus) boleh tetap ada karena informatif.
- Tapi tampilkan lebih ringkas — mungkin sebagai dot dengan tooltip, atau chip yang lebih kecil.

### e. Konsistensi spacing & alignment
- Pastikan padding, gap, dan alignment vertikal konsisten.
- Icon dan teks harus center-align sempurna.
- Ukuran touch target minimal 44px untuk mobile.

## 2. Perbaiki Scroll — Smooth, Tidak Patah-patah

Saat halaman di-scroll, terasa berat dan patah-patah (janky). Ini kemungkinan karena:

### a. Backdrop blur太重
- `glass` utility menggunakan `blur(24px) saturate(1.5)` — ini sangat berat untuk GPU, terutama saat scroll.
- **Solusi**: Kurangi blur jadi `blur(12px)` atau `blur(16px)` untuk elemen yang di-scroll. Simpan blur tinggi hanya untuk elemen statis (sidebar, modal).

### b. Kontur backdrop SVG
- `KonturBackdrop` merender SVG kompleks dengan banyak path curves — ini bisa membebani compositing.
- **Solusi**:
  - Sederhanakan jumlah path (kurangi ring count atau gunakan gradient sebagai fallback)
  - Atau gunakan `will-change: transform` dengan `translateZ(0)` untuk memisahkan layer
  - Atau render sebagai CSS background pattern yang lebih ringan

### c. Animasi aurora
- `animate-aurora` dengan `blur-3xl` pada elemen besar (720px) bisa menyebabkan repaint berat.
- **Solusi**:
  - Gunakan `transform: translate3d()` untuk GPU acceleration
  - Kurangi ukuran atau opacity glow
  - Atau nonaktifkan saat scrolling (IntersectionObserver atau scroll event dengan throttle)

### d. CSS `will-change` & layer hints
- Tambahkan `will-change: transform` pada elemen yang sering berubah posisi.
- Pastikan konten utama menggunakan `transform: translateZ(0)` atau `contain: layout style paint` untuk memisahkan layer.
- Hindari animasi `filter` dan `backdrop-filter` pada elemen yang mengikuti scroll.

### e. Optimasi motion/react
- Pastikan animasi Framer Motion tidak berjalan pada elemen yang tidak terlihat (off-screen).
- Kurangi jumlah layout animations yang berjalan bersamaan.
- Gunakan `layoutScroll: true` pada motion components jika perlu.

## 3. Konten halaman tetap seperti sekarang
- Jangan ubah struktur atau styling konten utama di dalam `<main>` — hanya header dan scroll behavior yang diperbaiki.
- Jika ada title di konten halaman yang double dengan header, title di konten tetap dipertahankan (header yang menyesuaikan).

## 4. Target file yang perlu diubah
- `components/layout/topbar.tsx` — perbaiki struktur dan styling header
- `app/globals.css` — optimasi glass utility (kurangi blur weight), tambahkan scroll optimization
- `components/garda/kontur-backdrop.tsx` — optimasi performa SVG backdrop
- `components/layout/console-shell.tsx` — jika perlu tambahkan scroll container optimization

## 5. Non-functional requirements
- **Performa**: Lighthouse scroll performance tidak boleh turun. FPS saat scroll harus ≥55fps di device mid-range.
- **Aksesibilitas**: Skip-to-content link tetap berfungsi. Focus order tidak berubah.
- **Responsive**: Header harus tetap baik di mobile (sheet navigasi) dan desktop (sidebar).
- **Dark mode**: Semua perubahan harus kompatibel dengan class-based dark mode.
- **Reduced motion**: `prefers-reduced-motion` harus tetap dihormati.

## 6. Deliverables
1. File yang dimodifikasi dengan perubahan minimal dan presisi.
2. Tidak ada perubahan pada konten halaman di luar header dan scroll optimization.
3. Tidak ada penambahan dependency baru.
4. Perubahan harus bisa di-review dengan mudah (satu concern per commit).