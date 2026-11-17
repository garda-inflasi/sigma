# Arsitektur halaman & akses — AI Garda Inflasi

Dokumen ini memetakan **4 lapis blueprint** ke rute Next.js 16 App Router, komponen, sumber data, dan peran, lalu menjelaskan backend (API Go, worker, layanan ML, basis data) yang melayaninya. Matriks akses di bawah dihasilkan langsung dari `lib/rbac` — sumber yang sama dengan `proxy.ts`, sidebar, palet perintah, dan aksi massal. Penegakan akhirnya ada di API Go, bukan di UI.

Disinkronkan dengan kode per 17 Nov 2026 (migrasi terakhir `000128`). Cara menjalankan ada di [README](../README.md); rencana dan status per fitur di [docs/todos](todos/README.md).

## 0. Topologi sistem

```
Browser ─► Caddy ─┬─► Next.js (apps/web): UI + BFF (/api/auth/*, /api/pengguna, ...)  ─┐
 (PWA lapangan)   └─► /api/v1/* ─────────────────────────────────────────────────────┐ │
                                                                                      ▼ ▼
                                              API Go (apps/api/cmd/api, Gin + GORM, hexagonal per modul)
                                               │            │             │
                                  PostgreSQL ◄─┘   Redis ◄──┘             └─► ML service (apps/ml, HTTP internal)
                                  (migrasi goose,  (cache, batas laju,        /train /predict /health
                                   seed SQL-first)  pub/sub SSE EWS)
                                       ▲
                          Worker Go (cmd/worker) ── job terjadwal ── konektor ingestion (PIHPS, BMKG, BPS, Bapanas, OpenWeather)
```

| Komponen | Lokasi | Tanggung jawab |
|---|---|---|
| Web | `apps/web` | Konsol, PWA lapangan, BFF dengan CSRF double-submit untuk auth/pengguna/profil |
| API | `apps/api/cmd/api` | Semua aturan bisnis dan penegakan izin, cakupan baris, pemisahan tugas, audit |
| Worker | `apps/api/cmd/worker` | `dashboard_refresh`, `kpi_hitung`, `siklus_evaluasi`, evaluasi EWS dan notifikasi, eskalasi SLA intervensi, `audit_partisi`, `audit_verifikasi`, `auth_pembersihan`, `prediksi_retraining`, penjadwal ingestion |
| ML | `apps/ml` | Peramalan harga: baseline naif/musiman/Holt/Holt-Winters dipilih per kabupaten lewat backtest, fitur eksogen cuaca dan hari libur, interval 80%, faktor XAI. Hanya pustaka standar Python |
| Migrasi dan seed | `db/migrations`, `db/seeds` | Skema (goose, nomor berurutan) dan data benih `reference` / `admin` / `demo` |
| Kontrak | `packages/contracts/openapi.yaml` | Sumber tunggal OpenAPI; `make openapi/sync` menyalin ke embed Swagger |

Alur data utama: **input lapangan** (`POST /v1/lapangan/harga`, idempoten) → `entri_harga` berstatus DRAF/DIAJUKAN → **validasi otomatis** (deviasi, geofence, anomali) → **validasi manusia** → **publikasi** → agregat dashboard, EWS, prediksi, dan KPI. Setiap langkah mengubah status lewat state machine yang ditegakkan di domain dan dicatat di `riwayat_status_entri` serta audit.

## 1. Pohon rute

```
app/
├─ layout.tsx                 font (Inter, Plus Jakarta Sans, JetBrains Mono), latar kontur, Providers, SessionProvider
├─ providers.tsx              TanStack Query · next-themes · MotionConfig(reducedMotion) · Tooltip · Sonner
├─ page.tsx                   redirect ke beranda peran
├─ manifest.ts                PWA "Garda Lapangan"
├─ api/                       BFF: route handler auth, pengguna, profil, riwayat aktivitas/sesi, cold-storage
├─ offline/                   fallback service worker (public/sw.js)
├─ akses-ditolak/             403 kontekstual (peran + rute)
├─ (auth)/                    masuk (kata sandi, OTP, SSO Google), lupa/atur ulang sandi, aktivasi undangan, ganti-password dan mfa (alur wajib)
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
   ├─ profil/                 ▸ Akun sendiri — data diri, ganti kata sandi, MFA, sesi aktif, riwayat aktivitas (`?tab=`)
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

### Kontrak API (rute nyata)

Sumber kebenaran: [`packages/contracts/openapi.yaml`](../packages/contracts/openapi.yaml). Ringkasan per kelompok:

| Metode | Path | Dipakai oleh |
|---|---|---|
| POST | `/v1/auth/{login,refresh,logout,logout-all,forgot-password,reset-password}`, `/v1/auth/mfa/*` | masuk, sesi, MFA |
| GET/POST | `/v1/auth/sso/google/*` | SSO Google (OIDC + PKCE) |
| GET/PATCH | `/v1/auth/me` | profil sendiri (`PATCH`: nama dan telepon saja) |
| GET | `/v1/auth/me/aktivitas`, `/v1/auth/me/riwayat-sesi` | riwayat aktivitas dan sesi akun sendiri (tab "Aktivitas Akun") |
| GET/DELETE/POST | `/v1/auth/sessions`, `/v1/auth/sessions/{id}`, `/v1/auth/sessions/revoke-others` | perangkat dan sesi aktif |
| GET | `/v1/siklus/aktif` | warna ambient seluruh UI |
| GET | `/v1/dashboard/operasional/*`, `/v1/dashboard/eksekutif` | dashboard |
| GET | `/v1/spasial/*`, `/v1/harga/tren` | peta dan grafik tren |
| GET/POST | `/v1/prediksi`, `/v1/prediksi/{kode}`, `/v1/prediksi/registri/*` | prediksi, XAI, registri model, promosi |
| GET/POST | `/v1/harga`, `/v1/harga/bulk`, `/v1/lapangan/harga` (409 = konflik) | validasi dan sinkronisasi lapangan |
| GET | `/v1/ews`, SSE `/v1/ews/stream` | EWS |
| GET/POST/PATCH | `/v1/intervensi`, `/{id}/tahap`, `/{id}/persetujuan`, `/persetujuan/{pid}/keputusan`, `/bulk` | kanban dan persetujuan berjenjang |
| GET/POST | `/v1/cold-storage/*`, `/v1/cold-storage/telemetri[/batch]` | fasilitas, stok, telemetri suhu |
| GET | `/v1/audit`, `/v1/audit/integritas` | audit trail dan status rantai hash |
| POST | `/v1/policy-brief/generate` | generator brief |

Aksi massal memakai `POST .../bulk` dengan `{ action, ids }` dan melaporkan hasil per entri (`berhasil`, `gagal`, `rincian` beserta sebab); entri yang dilewati tidak menggagalkan yang lain.

## 3. Peran (tanpa TPID/Pemda)

TPID dan Pemda **bukan pengguna sistem**. Keluaran untuk mereka (policy brief, rekomendasi APBD) disalurkan oleh Unit Hubungan Pemerintah & Kebijakan. Tahap kanban "Usulan intervensi logistik kilat" tidak lagi bergantung pada akun TPID.

Prinsip yang ditegakkan:
- **Least privilege** — izin granular `domain.aksi` dari tabel `roles`, `permissions`, `role_permissions` (dapat diedit admin) ditambah `user_permissions` (izin langsung dengan masa berlaku dan alasan). API membaca izin efektif dari basis data per permintaan (cache ≤ 15 detik, gagal-tutup 503), bukan dari klaim token.
- **Pemisahan tugas** — ditegakkan di domain, repository (di bawah kunci baris), dan basis data (CHECK):
  - harga: pencatat ≠ validator ≠ penerbit. `domain.Evaluasi` menolak validasi oleh pencatat dan publikasi oleh pencatat maupun validator (`pemisahan_tugas`); CHECK `chk_entri_harga_pemisahan_tugas` dan `chk_entri_harga_pemisahan_penerbit` menjaga tulis langsung ke tabel; percobaan yang ditolak dicatat di audit dengan hasil `DITOLAK`.
  - intervensi: pengaju permintaan persetujuan dan pemutus langkah sebelumnya tidak boleh memutus langkah berikutnya (dicek di usecase dan diulang di transaksi repository).
  - master data: pengaju usulan tidak boleh memutusnya.
  - Admin Sistem tidak memegang izin `validasi`, `publikasi`, atau `setujui` apa pun (dikunci tes).
- **Cakupan baris** — dibaca segar dari `pengguna.peran` dan `pengguna.kabupaten_kode` per permintaan: Koordinator dan Agregator hanya kabupatennya (ditambah entri yang ia catat sendiri); Mahasiswa hanya entrinya sendiri; peran provinsi ke atas seluruh Aceh; peran tak dikenal hanya miliknya. Berlaku di `harga`, `lapangan`, `dashboard`, `ews`, dan `intervensi` (ajuan, pindah tahap, aksi massal, dan keputusan persetujuan atas kartu di luar kabupaten dijawab seperti tidak ada). Data di luar cakupan dijawab 404/tidak_ditemukan agar keberadaannya tidak bocor.
- **Persetujuan berjenjang** — rantai langkah didefinisikan sebagai data (`alur_persetujuan*`): "Petakan suplai" dan "Selesai" butuh `intervensi.setujui` (Dewan Pengarah); "Dukungan instrumen belanja" ditambah `intervensi.setujui_fiskal` (Kanwil DJPb). Pemindahan maju tidak boleh melompati tahap persetujuan yang izinnya tidak dimiliki; kartu bisa dibuat otomatis dari alert EWS dan dieskalasi bila melewati SLA.
- **Audit** — append-only, berantai hash SHA-256, berpartisi bulanan dengan arsip; peran aplikasi database tanpa UPDATE/DELETE. Pemilik akun dapat melihat jejaknya sendiri di `/profil` (aksi yang ia lakukan dan aksi orang lain terhadap akunnya) tanpa snapshot sebelum/sesudah dan tanpa identitas pelaku lain.
- **MFA** wajib untuk lapis strategis, manajerial, koordinator, dan admin (`MFA_ENFORCE`; di produksi selalu). Sesi yang masih wajib ganti sandi atau setup MFA hanya membuka rute pemulihannya.

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
| `/profil` | — (semua pengguna masuk) | ✓ | ✓ | ✓ | ✓ | ✓ | ✓ | ✓ | ✓ | ✓ | ✓ | ✓ | ✓ | ✓ | ✓ |
| `/alur-sistem` | — | ✓ | ✓ | ✓ | ✓ | ✓ | ✓ | ✓ | ✓ | ✓ | ✓ | ✓ | ✓ | ✓ | ✓ |

Keterangan singkatan: **FMIPA** = Pimpinan FMIPA USK; **BI** = Analis Bank Indonesia; **BPS** = Statistisi BPS; **DJPb** = Analis Kanwil DJPb; **Prog** = Manajer Program; **Data** = Unit Data & Statistik; **Kebj** = Unit Hubungan Pemerintah & Kebijakan; **Biaya** = Unit Pembiayaan & Investasi; **Risk** = Unit Monitoring & Risiko; **Mutu** = Unit Mutu & Pascapanen; **Koord** = Koordinator Lapangan; **Agreg** = Unit Agregator; **Mhs** = Agen Lapangan Mahasiswa; **Admin** = Administrator Sistem.

Aksi massal di tiap tabel juga difilter izin (mis. *Validasi* butuh `harga.validasi`, *Publikasikan* butuh `harga.publikasi`, *Promosikan model* butuh `model.kelola`). Matriks izin lengkap dapat dilihat di aplikasi: **Pengguna & peran → Matriks akses**.

## 4. Subsistem utama

### Aplikasi lapangan (PWA)
`/lapangan/*` memakai `FieldShell`, service worker (`public/sw.js`), antrean IndexedDB (`lib/offline/*`, Zustand `field-queue-store`), dan sync engine idempoten dengan `Idempotency-Key`; API menjawab 409 untuk konflik versi dan klien menyediakan penyelesaian konflik. Entri membawa geofence, foto (kode etik disetujui dulu), dan masuk ke state machine harga. Logout membersihkan data offline.

### Cold storage dan telemetri
Fasilitas (UUID), lot stok (shelf-life dan risiko food loss), dan transaksi stok. `telemetri_suhu` berpartisi bulanan (`PARTITION BY RANGE (waktu)`, fungsi `pastikan_partisi_telemetri_suhu`, partisi `default`); masukan lewat `POST /v1/cold-storage/telemetri` dan `/telemetri/batch` dari gateway IoT/MQTT. Pembacaan di luar batas alarm dilaporkan dan memicu evaluasi risiko food loss pada lot aktif.

### Prediksi, XAI, dan retraining
Worker `prediksi_retraining` memanggil layanan ML untuk melatih ulang model per komoditas dan kabupaten dari `entri_harga` tervalidasi plus fitur cuaca (`cuaca_harian`) dan hari libur. Hasil masuk `prediksi_harga` dan `model_registry` berstatus KANDIDAT; promosi dijaga gerbang MAPE terhadap baseline dan dicatat di `model_promosi`. Faktor XAI (tren, musiman, cuaca, hari libur) dihitung di layanan ML. Tanpa `ML_SERVICE_URL`, pelatihan menjawab 503 sedangkan data demo tetap tampil.

### Peta spasial
`/peta` menampilkan neraca surplus–defisit per kabupaten/kota (23 wilayah Aceh), rute distribusi, dan kandidat cold storage. Batas wilayah dari `public/geojson/aceh-kabupaten.geojson` (perkiraan, berlabel demo), dengan Mapbox opsional (`NEXT_PUBLIC_MAPBOX_TOKEN`).

### Autentikasi, sesi, dan profil
Refresh token berotasi atomik dengan deteksi pemakaian ulang; satu sesi = satu `family_id` (klaim `sid`). `GET /v1/auth/me/riwayat-sesi` menampilkan sesi aktif dan yang sudah berakhir (AKTIF/DICABUT/KEDALUWARSA) per halaman kursor, sedangkan `GET /v1/auth/me/aktivitas` mengambil jejak audit akun sendiri dengan filter aksi, hasil, pelaku/sasaran, dan tanggal. Keduanya terkunci ke `user_id` token dan tidak punya parameter pemilih akun.

### Basis data
Kunci primer: UUID untuk `pengguna`, `intervensi`, `policy_brief`, `sumber_data`, `cold_storage`; BIGINT identity untuk tabel transaksional dan log; kode unik untuk referensi. Detail dan invarian di [SKEMA-DATABASE.md](SKEMA-DATABASE.md) (`TestInvarianKunci`). Migrasi dijalankan nyata saat deploy dan gagal keras ([DEPLOYMENT.md](DEPLOYMENT.md)).
