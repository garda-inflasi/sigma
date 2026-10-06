# PRODUCT REQUIREMENTS DOCUMENT (PRD)
## Platform Aplikasi "AI Garda Inflasi" — Provinsi Aceh
### Platform Terintegrasi Pengendalian Inflasi Daerah Berbasis Data & Rantai Pasok

**Versi:** 1.0
**Tanggal:** 2025
**Disusun untuk:** Penelitian Magister Kecerdasan Buatan FMIPA USK
**Status:** Draft untuk Pengembangan
**Klasifikasi:** Akademik — Riset Terapan Kebijakan Publik

## DAFTAR ISI

1. Ringkasan Eksekutif
2. Latar Belakang & Problem Statement
3. Tujuan Produk & Sasaran
4. Definisi Peran (Roles) & Persona
5. Arsitektur Informasi & Struktur Menu per Role
6. Kebutuhan Fungsional per Role
7. Kebutuhan Non-Fungsional
8. Arsitektur Teknis & AI Stack
9. Model Data & Integrasi
10. Design System & UX Guidelines
11. Keamanan, Privasi & Tata Kelola
12. Metrik Keberhasilan (KPI)
13. Roadmap & Fase Rilis
14. Risiko & Mitigasi
15. Lampiran

## 1. RINGKASAN EKSEKUTIF

**AI Garda Inflasi** adalah platform terintegrasi pengendalian inflasi daerah yang mentransformasi paradigma tata kelola dari **reaktif-manual** menjadi **prediktif-presisi**. Platform ini mengorkestrasi 4 lapis kecerdasan buatan (AI Stack), mengintegrasikan data multi-sumber (telemetri produsen, BPS/BI, logistik, fiskal DJPb), dan menyediakan ruang kendali keputusan berbasis peran bagi ekosistem TPID, Pemda, BI, BPS, Kanwil DJPb, dan FMIPA USK.

Sistem menyediakan **dua kanal utama** (Web Desktop untuk eksekutif/manajerial, Mobile Android untuk operasional lapangan) dengan **kontrol akses berbasis peran (RBAC)** yang ketat, serta **workflow engine** yang mengorkestrasi respons taktis pada 3 siklus ekonomi: Normal, Tekanan Harga/Defisit, dan Surplus Pangan.

## 2. LATAR BELAKANG & PROBLEM STATEMENT

### 2.1 Konteks
Inflasi daerah di Aceh dipicu fluktuasi harga komoditas pangan strategis (cabai, bawang merah, beras) akibat ketidakseimbangan pasokan-permintaan, disparitas spasial, dan lemahnya visibilitas rantai pasok. Satgas Pangan & TPID saat ini bekerja **reaktif** — operasi pasar setelah harga melonjak.

### 2.2 Masalah Utama
| No | Masalah | Dampak |
|----|---------|--------|
| M1 | Data lapangan tidak terstruktur & tersilo | Prediksi tidak akurat, keputusan lambat |
| M2 | Tidak ada sistem peringatan dini | Lonjakan harga tidak terantisipasi |
| M3 | Rantai pasok tidak transparan | Food loss tinggi, margin tidak efisien |
| M4 | Koordinasi lintas lembaga lemah | Duplikasi program, respons tidak sinkron |
| M5 | Kebijakan berbasis anekdot, bukan data | Intervensi tidak tepat sasaran |

### 2.3 Peluang
Digitalisasi tata kelola publik + ketersediaan AI/ML untuk time-series forecasting, optimasi spasial, dan workflow automation memungkinkan pembangunan sistem prediktif-presisi dengan fondasi akademik (FMIPA USK).

## 3. TUJUAN PRODUK & SASARAN

### 3.1 Tujuan Utama (Product Goals)
1. **G1** — Menyediakan satu platform terintegrasi multi-peran untuk pengendalian inflasi daerah.
2. **G2** — Mengubah data mentah multi-sumber menjadi rekomendasi kebijakan presisi & tepat waktu.
3. **G3** — Mengorkestrasi respons taktis otomatis pada 3 siklus ekonomi.
4. **G4** — Menjadi cetak biru nasional (Aceh sebagai pilot, replikasi plug-and-play).

### 3.2 Sasaran Terukur (OKR)
- **O1:** Akurasi proyeksi produksi ≥ 98% (MAPE ≤ 5.4% untuk hybrid ARIMA+LSTM).
- **O2:** F1-Score EWS ≥ 0.88, AUC-ROC ≥ 0.94.
- **O3:** Penurunan volatilitas harga ≥ 30%, food loss ≥ 60%.
- **O4:** Rasio tindak lanjut rekomendasi AI oleh Pemda ≥ 78%.
- **O5:** SUS Score ≥ 82/100, NPS ≥ +45.

## 4. DEFINISI PERAN (ROLES) & PERSONA

Platform menggunakan **RBAC 3-Lapis Koordinasi** + unit fungsional di dalamnya.

### 4.1 Klasifikasi Peran

```
┌─────────────────────────────────────────────────────────────┐
│  LAPIS 1: STRATEGIS (Dewan Pengarah)                        │
│  → TPID, Pemda, BI, BPS, Kanwil DJPb, FMIPA USK             │
├─────────────────────────────────────────────────────────────┤
│  LAPIS 2: MANAJERIAL (Tim Pelaksana Inti)                   │
│  → USK Manager, Manajer Unit (Data, Kebijakan,              │
│    Pembiayaan, Monitoring, Operasional)                     │
├─────────────────────────────────────────────────────────────┤
│  LAPIS 3: OPERASIONAL WILAYAH                               │
│  → Unit Agregator, Koordinator Lapangan, Mahasiswa          │
│    Agen Lapangan, Operator Cold Storage                     │
└─────────────────────────────────────────────────────────────┘
```

### 4.2 Detail Persona per Role

#### **ROLE 01 — DEWAN PENGARAH (Eksekutif Strategis)**
- **Siapa:** Pimpinan TPID, Kepala Pemda, Kepala Perwakilan BI, Kepala BPS, Kepala Kanwil DJPb, Dekan/Pimpinan FMIPA USK.
- **Kebutuhan:** Ringkasan eksekutif, peta neraca pangan, ruang fiskal, policy brief bulanan, persetujuan intervensi strategis.
- **Frekuensi Akses:** Bulanan / triwulanan (rapat koordinasi).
- **Device Utama:** Web Desktop (1440px+), sesekali tablet.

#### **ROLE 02 — MANAJER DATA & STATISTIK**
- **Siapa:** Peneliti/analis USK pengelola API BPS/BI, EWS, dashboard pasokan.
- **Kebutuhan:** Monitoring pipeline data, anomali ingestion, kontrol kualitas, retraining model.
- **Device:** Web Desktop.

#### **ROLE 03 — MANAJER HUBUNGAN PEMERINTAH & KEBIJAKAN**
- **Siapa:** Penghubung TPID & DJPb, konversi temuan AI → rekomendasi APBD.
- **Kebutuhan:** Policy brief generator, tracking tindak lanjut, kalender kebijakan.
- **Device:** Web Desktop.

#### **ROLE 04 — MANAJER PEMBIAYAAN & INVESTASI**
- **Siapa:** Analis studi kelayakan infrastruktur logistik, desain tarif, identifikasi dana CSR/perbankan.
- **Kebutuhan:** Kalkulator kelayakan, pipeline pembiayaan, monitoring anggaran.
- **Device:** Web Desktop.

#### **ROLE 05 — MANAJER MONITORING & RISIKO**
- **Siapa:** Auditor silang data lapangan, pemantau utilisasi gudang, mitigasi risiko.
- **Kebutuhan:** Audit trail, alert anomali, risk register, verifikasi silang BPS/BI.
- **Device:** Web Desktop + Mobile.

#### **ROLE 06 — MANAJER OPERASIONAL & LOGISTIK**
- **Siapa:** Pengelola agregasi produsen, cold storage, mutu pascapanen, agen lapangan.
- **Kebutuhan:** Peta sentra, stok real-time, rute distribusi, kualitas & shelf-life.
- **Device:** Web Desktop + Mobile.

#### **ROLE 07 — KOORDINATOR LAPANGAN**
- **Siapa:** Supervisor mahasiswa agen lapangan per kabupaten/kota.
- **Kebutuhan:** Manajemen tim, verifikasi input, jadwal kunjungan, KPI tim.
- **Device:** Mobile Android (utama) + Web.

#### **ROLE 08 — MAHASISWA AGEN LAPANGAN**
- **Siapa:** Mahasiswa FMIPA USK di lapangan (kode etik ketat: dilarang menentukan harga / menjadi perantara).
- **Kebutuhan:** Input telemetri produsen (volume/kalender panen, harga produsen, geospasial), unggah foto, sinkronisasi offline.
- **Device:** Mobile Android (offline-first).

#### **ROLE 09 — OPERATOR COLD STORAGE**
- **Siapa:** Pengelola gudang fisik.
- **Kebutuhan:** Tracking stok in/out, monitoring suhu-kelembapan (IoT), shelf-life alert.
- **Device:** Mobile Android + Tablet/Web.

#### **ROLE 10 — UNIT AGREGATOR PRODUSEN**
- **Siapa:** Konsolidator pasokan tingkat kecamatan.
- **Kebutuhan:** Verifikasi volume, transparansi harga dasar, agregasi order.
- **Device:** Mobile Android.

#### **ROLE 11 — UNIT MUTU & PASCAPANEN (FMIPA)**
- **Siapa:** Analis laboratorium kualitas, standar kemasan, reduksi food loss.
- **Kebutuhan:** Computer vision QC, input hasil uji, standar mutu.
- **Device:** Web + Mobile.

## 5. ARSITEKTUR INFORMASI & STRUKTUR MENU PER ROLE

### 5.1 Prinsip Desain Navigasi
- **Web Desktop:** Sidebar kiri dengan grouping modul; top-bar untuk role switcher, notifikasi, profil.
- **Mobile Android:** Drawer navigasi dengan grouping; bottom-nav 4 item utama + FAB untuk aksi cepat.
- **Kontrol Akses:** Setiap role hanya melihat menu yang relevan (RBAC + menu filtering).

### 5.2 Struktur Menu Web per Role (Sidebar)

#### **A. DEWAN PENGARAH**
```
📊 RINGKASAN EKSEKUTIF
   ├── Executive Dashboard
   ├── Peta Neraca Pangan
   └── Ruang Fiskal & Inflasi

📄 KEBIJAKAN
   ├── Policy Brief Generator
   ├── Persetujuan Intervensi
   └── Kalender Kebijakan

📈 ANALITIK STRATEGIS
   ├── Proyeksi Inflasi Regional
   ├── Peta Surplus-Defisit
   └── Evaluasi Triwulanan

⚙️ TATA KELOLA
   ├── Risk Register
   ├── Audit Trail
   └── Pengaturan Kelembagaan
```

#### **B. MANAJER DATA & STATISTIK**
```
🗄️ DATA PIPELINE
   ├── Monitoring Ingesti
   ├── Kualitas Data
   ├── Anomali & Pembersihan
   └── Sumber Data (BPS/BI/Logistik/Fiskal)

🧠 AI MODEL MANAGEMENT
   ├── Model Registry
   ├── Training Pipeline
   ├── Evaluasi Model (RMSE/MAE/MAPE)
   └── Explainability (SHAP/LIME)

⚠️ EARLY WARNING SYSTEM
   ├── Konfigurasi Threshold
   ├── Sinyal Aktif
   └── Log Alarm

📊 DASHBOARD POKOK
   └── Ringkasan Statistik
```

#### **C. MANAJER HUBUNGAN PEMERINTAH & KEBIJAKAN**
```
📝 POLICY BRIEF
   ├── Generator Otomatis (LLM)
   ├── Draft & Reviu
   ├── Arsip Brief
   └── Distribusi ke TPID/Pemda

🔗 KONVERSI KEBIJAKAN
   ├── Temuan AI → Rekomendasi APBD
   ├── Tracking Tindak Lanjut
   └── Matriks Sinergi Lembaga

📅 KOORDINASI
   ├── Kalender Rapat
   ├── Notulen & Tindak Lanjut
   └── Korespondensi Resmi
```

#### **D. MANAJER PEMBIAYAAN & INVESTASI**
```
💰 STUDI KELAYAKAN
   ├── Kalkulator Kelayakan
   ├── Proyeksi ROI Infrastruktur
   └── Simulasi Tarif

🏦 PIPELINE PEMBIAYAAN
   ├── Identifikasi Sumber Dana (CSR/Perbankan)
   ├── Status Pengajuan
   └── Monitoring Anggaran

📊 ANALITIK FISKAL
   ├── Dampak Ekonomi Daerah
   └── Integrasi DJPb
```

#### **E. MANAJER MONITORING & RISIKO**
```
🔍 AUDIT & VERIFIKASI
   ├── Audit Silang Data Lapangan
   ├── Verifikasi BPS/BI
   └── Log Audit

⚠️ MANAJEMEN RISIKO
   ├── Risk Register
   ├── Mitigasi Operasional
   └── Eskalasi Risiko

🏭 MONITORING GUDANG
   ├── Utilisasi Cold Storage
   ├── Food Loss Tracking
   └── Stok Real-Time
```

#### **F. MANAJER OPERASIONAL & LOGISTIK**
```
🌾 AGREGASI PRODUSEN
   ├── Konsolidasi Pasokan
   ├── Transparansi Harga Dasar
   └── Peta Sentra Produksi

❄️ COLD STORAGE
   ├── Peta Lokasi Sentra
   ├── Stok Real-Time
   ├── Manajemen Distribusi
   └── Shelf-Life Monitoring

🚚 LOGISTIK & DISTRIBUSI
   ├── Optimasi Rute (VRP)
   ├── Network Flow
   └── Estimasi Biaya

🔬 MUTU & PASCAPANEN
   ├── QC Computer Vision
   ├── Standar Kemasan
   └── Reduksi Food Loss
```

#### **G. KOORDINATOR LAPANGAN**
```
👥 MANAJEMEN TIM
   ├── Daftar Mahasiswa Agen
   ├── Jadwal Kunjungan
   └── KPI Tim

✅ VERIFIKASI INPUT
   ├── Antrian Verifikasi
   ├── Validasi Telemetri
   └── Cross-check BPS

📍 PETA WILAYAH
   ├── Sebaran Kelompok Tani
   └── Heatmap Aktivitas
```

#### **H. OPERATOR COLD STORAGE**
```
📦 MANAJEMEN STOK
   ├── Stok In/Out
   ├── Kapasitas Real-Time
   └── Batch Tracking

🌡️ IOT MONITORING
   ├── Suhu & Kelembapan
   ├── Gas Sensor
   └── Alert Anomali

⏳ SHELF-LIFE
   ├── Prediksi Masa Simpan
   ├── Prioritas Distribusi
   └── Rekomendasi Intervensi
```

#### **I. UNIT MUTU & PASCAPANEN**
```
🔬 LAB ANALITIK
   ├── Input Hasil Uji
   ├── Standar Mutu
   └── Sertifikasi

📷 COMPUTER VISION QC
   ├── Deteksi Cacat
   ├── Klasifikasi Mutu
   └── Riwayat QC

📉 REDUKSI FOOD LOSS
   ├── Analitik Kehilangan
   └── Rekomendasi Perbaikan
```

#### **J. UNIT AGREGATOR PRODUSEN**
```
📥 AGREGASI
   ├── Input Volume
   ├── Verifikasi Produsen
   └── Daftar Order

💵 TRANSPARANSI HARGA
   ├── Harga Dasar Produsen
   ├── Riwayat Harga
   └── Perbandingan Pasar
```

### 5.3 Struktur Menu Mobile Android per Role (Drawer)

#### **A. MAHASISWA AGEN LAPANGAN (Offline-First)**
```
🏠 Beranda
   ├── Tugas Hari Ini
   ├── Ringkasan Input
   └── Notifikasi

📝 INPUT TELEMETRI
   ├── Data Panen (Volume/Kalender)
   ├── Harga Produsen
   ├── Geospasial Kelompok Tani
   └── Unggah Foto

📚 EDUKASI
   ├── Modul Literasi Digital
   ├── SOP Kode Etik
   └── FAQ Produsen

👤 Profil
   ├── Data Diri
   ├── Riwayat Input
   └── Sinkronisasi Offline
```

#### **B. KOORDINATOR LAPANGAN**
```
🏠 Beranda
   ├── KPI Tim
   ├── Antrian Verifikasi
   └── Alert

👥 Tim Saya
   ├── Daftar Agen
   ├── Performa
   └── Jadwal

✅ VERIFIKASI
   ├── Antrian
   ├── Cross-check
   └── Persetujuan

📍 PETA
   └── Sebaran Aktivitas
```

#### **C. OPERATOR COLD STORAGE**
```
🏠 Beranda
   ├── Kapasitas
   ├── Alert Suhu
   └── Stok Kritis

📦 STOK
   ├── In/Out
   ├── Batch
   └── Pindai QR

🌡️ IOT
   ├── Sensor Real-Time
   └── Riwayat

⏳ SHELF-LIFE
   └── Prioritas Distribusi
```

#### **D. UNIT AGREGATOR**
```
🏠 Beranda
📥 AGREGASI
💵 HARGA
👤 PROFIL
```

#### **E. DEWAN PENGARAH (Mobile Ringkas)**
```
🏠 Ringkasan Eksekutif
📊 KPI Utama
📄 Policy Brief
🔔 Persetujuan Intervensi
```

### 5.4 Peta Menu Lengkap (Matrix Role × Modul)

| Modul | Dewan Pengarah | Mgr Data | Mgr Kebijakan | Mgr Pembiayaan | Mgr Monitoring | Mgr Operasional | Koord Lapangan | Agen Lapangan | Cold Storage | Mutu | Agregator |
|-------|:---:|:---:|:---:|:---:|:---:|:---:|:---:|:---:|:---:|:---:|:---:|
| Executive Dashboard | ✅ | – | – | – | – | – | – | – | – | – | – |
| Peta Neraca Pangan | ✅ | ✅ | ✅ | – | ✅ | ✅ | – | – | – | – | – |
| Policy Brief Generator | ✅ | – | ✅ | – | – | – | – | – | – | – | – |
| Persetujuan Intervensi | ✅ | – | – | – | – | – | – | – | – | – | – |
| Data Pipeline | – | ✅ | – | – | – | – | – | – | – | – | – |
| AI Model Mgmt | – | ✅ | – | – | – | – | – | – | – | – | – |
| Early Warning System | ✅ | ✅ | ✅ | – | ✅ | ✅ | ✅ | – | – | – | – |
| Predictive Price | ✅ | ✅ | ✅ | – | ✅ | ✅ | – | – | – | – | – |
| Spatial Optimization | – | ✅ | – | – | – | ✅ | – | – | – | – | – |
| Logistik & Rute | – | – | – | – | – | ✅ | – | – | ✅ | – | – |
| Cold Storage Mgmt | – | – | – | – | ✅ | ✅ | – | – | ✅ | – | – |
| Mutu & Food Loss | – | – | – | – | – | ✅ | – | – | – | ✅ | – |
| Telemetri Produsen | – | – | – | – | – | – | ✅ | ✅ | – | – | ✅ |
| Agregasi Pasokan | – | – | – | – | – | ✅ | – | – | – | – | ✅ |
| Audit & Risiko | ✅ | – | – | – | ✅ | – | – | – | – | – | – |
| Studi Kelayakan | – | – | – | ✅ | – | – | – | – | – | – | – |
| Pembiayaan | – | – | – | ✅ | – | – | – | – | – | – | – |
| User Management | – | ✅ | – | – | ✅ | – | – | – | – | – | – |

## 6. KEBUTUHAN FUNGSIONAL PER ROLE

### 6.1 DEWAN PENGARAH

| ID | Fitur | Deskripsi | Prioritas |
|----|-------|-----------|-----------|
| F-EP-01 | Executive Dashboard | Ringkasan KPI (akurasi proyeksi, volatilitas, food loss, tindak lanjut) dalam satu tampilan | P0 |
| F-EP-02 | Peta Neraca Pangan | Visualisasi surplus-defisit spasial per kabupaten/kota | P0 |
| F-EP-03 | Ruang Fiskal | Indikator fiskal regional + dampak inflasi | P1 |
| F-EP-04 | Policy Brief Viewer | Baca & setujui draft policy brief generatif | P0 |
| F-EP-05 | Persetujuan Intervensi | Approval workflow untuk intervensi strategis | P0 |
| F-EP-06 | Evaluasi Triwulanan | Laporan periodik dengan ekspor PDF/Excel | P1 |
| F-EP-07 | Notifikasi EWS Strategis | Alert level merah untuk keputusan cepat | P0 |

**Acceptance Criteria:**
- Dashboard load < 3 detik.
- Data refresh setiap 1 menit untuk indikator real-time.
- Ekspor laporan tersedia dalam PDF, Excel, PPT.

### 6.2 MANAJER DATA & STATISTIK

| ID | Fitur | Deskripsi | Prioritas |
|----|-------|-----------|-----------|
| F-DS-01 | Data Ingestion Monitor | Pantau status pipeline (BPS, BI, telemetri, logistik, fiskal) | P0 |
| F-DS-02 | Data Quality Dashboard | Skor kualitas, missing value, duplikasi, outlier | P0 |
| F-DS-03 | Anomaly Detection Log | Daftar anomali terdeteksi (Isolation Forest, Autoencoder) | P0 |
| F-DS-04 | Cross-Validation Tool | Bandingkan telemetri vs BPS/BI | P0 |
| F-DS-05 | Model Registry | Daftar model, versi, metrik, status deploy | P0 |
| F-DS-06 | Training Pipeline Trigger | Jalankan ulang training manual/scheduled | P1 |
| F-DS-07 | Model Evaluation | RMSE, MAE, MAPE, R², F1, AUC-ROC | P0 |
| F-DS-08 | SHAP/LIME Explorer | Visualisasi explainability model | P1 |
| F-DS-09 | EWS Configuration | Atur threshold alarm (hijau/kuning/merah) | P0 |

**Acceptance Criteria:**
- Pipeline failure auto-retry 3x + alert.
- Model retraining dapat dijadwalkan (cron).
- SHAP plot interaktif per prediksi.

### 6.3 MANAJER HUBUNGAN PEMERINTAH & KEBIJAKAN

| ID | Fitur | Deskripsi | Prioritas |
|----|-------|-----------|-----------|
| F-PK-01 | Policy Brief Generator | LLM + RAG untuk draft otomatis bulanan | P0 |
| F-PK-02 | Draft Editor | Editor kolaboratif dengan komentar | P0 |
| F-PK-03 | Brief Archive | Pencarian & filter arsip | P1 |
| F-PK-04 | Distribution Tracker | Status kirim ke TPID/Pemda | P0 |
| F-PK-05 | Recommendation Converter | Ubah temuan AI → rekomendasi APBD | P0 |
| F-PK-06 | Follow-up Tracker | Rasio tindak lanjut rekomendasi | P0 |
| F-PK-07 | Institutional Matrix | Matriks sinergi TPID/BI/BPS/DJPb | P1 |

**Acceptance Criteria:**
- Policy brief dihasilkan < 30 detik.
- Sitasi otomatis ke sumber data.
- Tracking tindak lanjut dengan deadline & reminder.

### 6.4 MANAJER PEMBIAYAAN & INVESTASI

| ID | Fitur | Deskripsi | Prioritas |
|----|-------|-----------|-----------|
| F-PB-01 | Feasibility Calculator | NPV, IRR, BCR, Payback Period | P0 |
| F-PB-02 | Infrastructure Projection | Proyeksi ROI cold storage & logistik | P0 |
| F-PB-03 | Tariff Simulator | Simulasi tarif penyimpanan/distribusi | P1 |
| F-PB-04 | Funding Pipeline | Identifikasi CSR, perbankan, APBD | P0 |
| F-PB-05 | Budget Monitoring | Monitoring realisasi anggaran | P0 |
| F-PB-06 | Economic Impact Analysis | Analisis dampak ekonomi daerah | P1 |

**Acceptance Criteria:**
- Kalkulator dengan asumsi dapat dikonfigurasi.
- Integrasi data DJPb (API/manual).
- Ekspor laporan studi kelayakan.

### 6.5 MANAJER MONITORING & RISIKO

| ID | Fitur | Deskripsi | Prioritas |
|----|-------|-----------|-----------|
| F-MR-01 | Audit Trail | Log semua aktivitas sistem | P0 |
| F-MR-02 | Cross-Audit Tool | Verifikasi silang data lapangan vs BPS/BI | P0 |
| F-MR-03 | Risk Register | Daftar risiko + mitigasi + owner | P0 |
| F-MR-04 | Warehouse Utilization | Monitoring utilisasi cold storage | P0 |
| F-MR-05 | Food Loss Tracking | Tracking kehilangan pascapanen | P0 |
| F-MR-06 | Escalation Workflow | Eskalasi risiko ke level atas | P1 |
| F-MR-07 | Compliance Checker | Kepatuhan SOP & kode etik | P1 |

**Acceptance Criteria:**
- Audit trail immutable (append-only).
- Alert risiko otomatis berdasarkan threshold.
- Laporan audit bulanan otomatis.

### 6.6 MANAJER OPERASIONAL & LOGISTIK

| ID | Fitur | Deskripsi | Prioritas |
|----|-------|-----------|-----------|
| F-OP-01 | Producer Aggregation | Konsolidasi pasokan per wilayah | P0 |
| F-OP-02 | Price Transparency | Harga dasar produsen real-time | P0 |
| F-OP-03 | Production Center Map | Peta sentra produksi | P0 |
| F-OP-04 | Route Optimization | VRP (OR-Tools/GA/SA) | P0 |
| F-OP-05 | Network Flow Optimization | Min-cost flow antar-daerah | P1 |
| F-OP-06 | Cold Storage Locator | Penentuan lokasi sentra | P0 |
| F-OP-07 | Stock Real-Time | Stok gudang real-time | P0 |
| F-OP-08 | Shelf-Life Predictor | Prediksi masa simpan | P0 |
| F-OP-09 | Distribution Scheduler | Jadwal distribusi optimal | P0 |
| F-OP-10 | QC Computer Vision | Deteksi cacat & klasifikasi mutu | P1 |

**Acceptance Criteria:**
- Optimasi rute < 10 detik untuk 50 titik.
- Peta interaktif dengan layer surplus-defisit.
- Sinkronisasi stok real-time < 5 detik.

### 6.7 KOORDINATOR LAPANGAN

| ID | Fitur | Deskripsi | Prioritas |
|----|-------|-----------|-----------|
| F-KL-01 | Team Management | Daftar & profil mahasiswa agen | P0 |
| F-KL-02 | Visit Scheduling | Jadwal kunjungan lapangan | P0 |
| F-KL-03 | KPI Dashboard Tim | Performa per agen | P0 |
| F-KL-04 | Verification Queue | Antrian verifikasi input | P0 |
| F-KL-05 | Cross-check Tool | Validasi dengan BPS/BI | P0 |
| F-KL-06 | Activity Heatmap | Peta sebaran aktivitas | P1 |
| F-KL-07 | Field Reporting | Laporan kondisi lapangan | P1 |

**Acceptance Criteria:**
- Verifikasi < 2 menit per entri.
- Notifikasi real-time untuk input baru.
- Offline mode dengan sync otomatis.

### 6.8 MAHASISWA AGEN LAPANGAN

| ID | Fitur | Deskripsi | Prioritas |
|----|-------|-----------|-----------|
| F-AL-01 | Telemetry Input | Volume, kalender panen, harga produsen | P0 |
| F-AL-02 | Geolocation Capture | Geospasial kelompok tani | P0 |
| F-AL-03 | Photo Upload | Foto kondisi panen/lahan | P0 |
| F-AL-04 | Offline Mode | Input tanpa internet, sync otomatis | P0 |
| F-AL-05 | Digital Literacy Module | Modul edukasi produsen | P1 |
| F-AL-06 | Code of Ethics SOP | Kode etik (larangan tentukan harga) | P0 |
| F-AL-07 | Task List | Tugas harian dari koordinator | P0 |
| F-AL-08 | Input History | Riwayat input & status verifikasi | P1 |

**Acceptance Criteria:**
- Input offline tersimpan lokal (SQLite/Room).
- Sync otomatis saat online.
- Validasi input (range harga, koordinat).
- UI ramah pengguna non-teknis.

### 6.9 OPERATOR COLD STORAGE

| ID | Fitur | Deskripsi | Prioritas |
|----|-------|-----------|-----------|
| F-CS-01 | Stock In/Out | Pencatatan masuk/keluar | P0 |
| F-CS-02 | Capacity Monitor | Kapasitas real-time | P0 |
| F-CS-03 | Batch Tracking | Pelacakan batch komoditas | P0 |
| F-CS-04 | IoT Sensor Dashboard | Suhu, kelembapan, gas | P0 |
| F-CS-05 | Anomaly Alert | Alert jika di luar ambang | P0 |
| F-CS-06 | Shelf-Life Prediction | Prediksi masa simpan per batch | P0 |
| F-CS-07 | Distribution Priority | Prioritas berdasarkan umur simpan | P0 |
| F-CS-08 | QR/Barcode Scan | Pindai cepat | P1 |

**Acceptance Criteria:**
- Update sensor setiap 1 menit.
- Alert < 30 detik setelah anomali.
- Prediksi shelf-life akurat (RMSE < 1 hari).

### 6.10 UNIT MUTU & PASCAPANEN

| ID | Fitur | Deskripsi | Prioritas |
|----|-------|-----------|-----------|
| F-MP-01 | Lab Result Input | Input hasil uji kimia/kualitas | P0 |
| F-MP-02 | Quality Standards | Standar mutu per komoditas | P0 |
| F-MP-03 | Computer Vision QC | Deteksi cacat & klasifikasi | P1 |
| F-MP-04 | Packaging Standard | Rekomendasi kemasan | P1 |
| F-MP-05 | Food Loss Analytics | Analitik kehilangan | P0 |
| F-MP-06 | Certification | Sertifikasi mutu | P2 |

**Acceptance Criteria:**
- CV QC akurasi > 90%.
- Integrasi hasil uji ke batch tracking.
- Rekomendasi perbaikan berbasis data.

### 6.11 UNIT AGREGATOR PRODUSEN

| ID | Fitur | Deskripsi | Prioritas |
|----|-------|-----------|-----------|
| F-AP-01 | Volume Input | Input volume dari produsen | P0 |
| F-AP-02 | Producer Verification | Verifikasi identitas produsen | P0 |
| F-AP-03 | Order Management | Daftar order & status | P0 |
| F-AP-04 | Price Transparency | Harga dasar real-time | P0 |
| F-AP-05 | Price History | Riwayat harga | P1 |
| F-AP-06 | Market Comparison | Perbandingan harga pasar | P1 |

**Acceptance Criteria:**
- Input volume < 1 menit.
- Harga ter-update real-time.
- Tidak ada fitur penentuan harga (kode etik).

## 7. KEBUTUHAN NON-FUNGSIONAL

### 7.1 Performa
| Metrik | Target |
|--------|--------|
| Waktu muat dashboard | < 3 detik (P95) |
| Waktu respons API | < 500 ms (P95) |
| Throughput ingestion | ≥ 1000 event/detik |
| Latensi prediksi model | < 2 detik |
| Uptime | ≥ 99.5% |

### 7.2 Skalabilitas
- Horizontal scaling microservices (Kubernetes).
- Mendukung 10.000+ produsen terverifikasi.
- Mendukung 500+ concurrent users.

### 7.3 Keandalan
- Failover otomatis.
- Backup harian (retensi 30 hari).
- Disaster recovery RTO < 4 jam, RPO < 1 jam.

### 7.4 Usability
- SUS Score ≥ 82.
- NPS ≥ +45.
- UAT pass rate ≥ 85%.
- Mobile-first untuk agen lapangan.

### 7.5 Kompatibilitas
- Web: Chrome, Firefox, Safari, Edge (2 versi terakhir).
- Mobile: Android 8.0+ (utama), iOS 14+ (opsional).
- Responsif: 360px – 1920px.

### 7.6 Lokalisasi
- Bahasa Indonesia (utama).
- Bahasa Aceh (opsional untuk agen lapangan).
- Format tanggal/waktu/angka Indonesia.
- Zona waktu WIB.

### 7.7 Aksesibilitas
- WCAG 2.1 Level AA.
- Kontras warna memadai.
- Dukungan screen reader.

## 8. ARSITEKTUR TEKNIS & AI STACK

### 8.1 Arsitektur Umum: Microservices + Polyglot Persistence

```
┌──────────────────────────────────────────────────────────────┐
│                    CLIENT LAYER                              │
│  ┌────────────────┐  ┌────────────────┐  ┌────────────────┐  │
│  │  Web (Next.js) │  │ Mobile  Android│  │  Admin Panel   │  │
│  └────────────────┘  └────────────────┘  └────────────────┘  │
└──────────────────────────────────────────────────────────────┘
                            │
┌──────────────────────────────────────────────────────────────┐
│              API GATEWAY (Golang)                            │
│  REST API · WebSocket · Auth Service                         │
└──────────────────────────────────────────────────────────────┘
                            │
┌──────────────────────────────────────────────────────────────┐
│                  MICROSERVICES LAYER                         │
│  ┌──────────────┐  ┌──────────────┐  ┌──────────────┐        │
│  │ Python AI    │  │ Golang Data  │  │ Golang       │        │
│  │ Engine       │  │ Service      │  │ Workflow     │        │
│  │ (FastAPI)    │  │              │  │ (Temporal)   │        │
│  └──────────────┘  └──────────────┘  └──────────────┘        │
└──────────────────────────────────────────────────────────────┘
                            │
┌──────────────────────────────────────────────────────────────┐
│                    DATA LAYER                                │
│  ┌──────────────┐  ┌──────────────┐  ┌──────────────┐        │
│  │ TimescaleDB  │  │  MongoDB     │  │  Redis       │        │
│  │ (time-series)│  │  (documents) │  │  (cache)     │        │
│  └──────────────┘  └──────────────┘  └──────────────┘        │
└──────────────────────────────────────────────────────────────┘
```

### 8.2 Tech Stack

| Layer | Teknologi | Alasan |
|-------|-----------|--------|
| Frontend Web | Next.js 14, TypeScript, Tailwind CSS | SSR, performa, DX |
| Mobile | Flutter / React Native | Cross-platform, offline-first |
| API Gateway | Golang | Performa tinggi, konkurensi |
| AI Engine | Python, FastAPI, PyTorch, TensorFlow | Ekosistem ML |
| ML Libraries | Scikit-learn, XGBoost, LightGBM, SHAP | Model klasik & explainability |
| Time-Series DB | PostgreSQL + TimescaleDB | Hypertable untuk time-series |
| Document DB | MongoDB | Data semi-terstruktur |
| Cache/Queue | Redis | Cache + message broker |
| Workflow | Temporal / Cadence | Orkestrasi Layer 4 |
| Container | Docker + Kubernetes | Manajemen microservices |
| Monitoring | Prometheus + Grafana | Observabilitas |
| CI/CD | GitHub Actions / GitLab CI | Otomasi deployment |
| LLM | GPT-4 / Llama + RAG | Policy brief generator |
| Map | react-map-gl / Leaflet | Visualisasi geospasial |

### 8.3 AI Stack (4 Layer)

#### **Layer 1: Multi-Source Data Ingestion & Sensing**
- **Sumber:** Telemetri produsen, BPS, BI, logistik, fiskal DJPb.
- **Teknik:** Stream processing (Kafka/Redis Streams), batch ETL.
- **Metode AI:** Data cleaning (KNN-imputation, MICE), anomaly detection (Isolation Forest, Autoencoder, One-Class SVM), entity resolution.

#### **Layer 2: AI Analytics Engine & Predictive Core**
- **Predictive Supply & Price Modeling:** LSTM, GRU, Transformer (Informer, Autoformer, TFT), Hybrid ARIMA+LSTM.
- **AI Early Warning System:** XGBoost, LightGBM, Random Forest, CUSUM, Bayesian Online Change Point Detection, Survival Analysis.
- **Spatial & Supply Chain Optimization:** DBSCAN, K-Means, VRP (GA/SA/OR-Tools), Min-cost Flow.
- **Quality & Food Loss Analytics:** Random Forest, Gradient Boosting, EOQ, Predictive Maintenance.

#### **Layer 3: Dashboard & Decision Support System**
- Dashboard Operasional & Eksekutif.
- Automated Policy Brief Generator (LLM + RAG + RLHF).
- Explainable AI (SHAP, LIME, Attention Visualization).

#### **Layer 4: Smart Intervention & Workflow Engine**
- 3 Siklus: Normal, Tekanan Harga/Defisit, Surplus Pangan.
- Workflow engine (Temporal/Cadence).
- Decision making under uncertainty (MDP/POMDP).
- Multi-agent coordination.
- Reinforcement Learning untuk kebijakan intervensi optimal.

### 8.4 Model AI & Performa Target

| Model | Metrik | Target |
|-------|--------|--------|
| ARIMA | RMSE | 1250.5 |
| SVR | RMSE | 1120.3 |
| Random Forest | RMSE | 980.7 |
| XGBoost | RMSE | 920.4 |
| LSTM | RMSE | 850.2 |
| TFT | RMSE | 780.5 |
| **Hybrid ARIMA+LSTM** | **RMSE** | **800.3** |
| **Hybrid ARIMA+LSTM** | **MAPE** | **5.4%** |
| **Hybrid ARIMA+LSTM** | **R²** | **0.90** |
| XGBoost (EWS) | F1 | 0.86 |
| LSTM Autoencoder (EWS) | F1 | 0.88 |
| LSTM Autoencoder (EWS) | AUC-ROC | 0.94 |

## 9. MODEL DATA & INTEGRASI

### 9.1 Skema Database (Ringkasan)

#### **TimescaleDB (Time-Series)**
```sql
-- Hypertable: harga
CREATE TABLE prices (
  time TIMESTAMPTZ NOT NULL,
  commodity_id UUID,
  region_id UUID,
  price DECIMAL,
  source VARCHAR(50),
  confidence DECIMAL
);

-- Hypertable: volume panen
CREATE TABLE harvest_volumes (
  time TIMESTAMPTZ NOT NULL,
  commodity_id UUID,
  farmer_group_id UUID,
  volume DECIMAL,
  unit VARCHAR(20)
);

-- Hypertable: logistik
CREATE TABLE logistics (
  time TIMESTAMPTZ NOT NULL,
  warehouse_id UUID,
  stock DECIMAL,
  food_loss DECIMAL,
  temperature DECIMAL,
  humidity DECIMAL
);

-- Hypertable: fiskal
CREATE TABLE fiscal_indicators (
  time TIMESTAMPTZ NOT NULL,
  region_id UUID,
  indicator VARCHAR(50),
  value DECIMAL
);
```

#### **MongoDB (Documents)**
```javascript
// Collection: produsen
{
  _id: ObjectId,
  name: String,
  nik: String,
  location: { type: "Point", coordinates: [lon, lat] },
  commodity: [String],
  verified: Boolean,
  created_at: Date
}

// Collection: logistik
{
  _id: ObjectId,
  batch_id: String,
  commodity: String,
  origin: ObjectId,
  destination: ObjectId,
  status: String,
  shelf_life_days: Number
}

// Collection: log_aktivitas
{
  _id: ObjectId,
  user_id: ObjectId,
  role: String,
  action: String,
  entity: String,
  timestamp: Date,
  metadata: Object
}
```

#### **Redis (Cache & Queue)**
- Session management
- Cache data sering diakses (harga terbaru, KPI)
- Message broker (telemetry:new)
- Pub/Sub untuk real-time notification

### 9.2 Integrasi Eksternal

| Sumber | Metode | Frekuensi |
|--------|--------|-----------|
| BPS API | REST API | Harian |
| BI API | REST API | Harian |
| DJPb | REST API / Manual | Mingguan |
| IoT Sensor | MQTT | Real-time (1 menit) |
| Mobile App | REST + WebSocket | Real-time |

### 9.3 API Specification (Ringkasan)

#### **Endpoint Utama**
```
POST   /api/v1/auth/login
GET    /api/v1/users/me
GET    /api/v1/dashboard/executive
GET    /api/v1/dashboard/operational
POST   /api/v1/telemetry
GET    /api/v1/predict/price
POST   /api/v1/predict/ews
POST   /api/v1/predict/shelf-life
GET    /api/v1/optimize/routes
GET    /api/v1/policy-brief/generate
POST   /api/v1/workflow/trigger
GET    /api/v1/audit/logs
```

#### **Contoh Request: Prediksi Harga**
```json
POST /api/v1/predict/price
{
  "commodity": "cabai_merah",
  "region": "aceh_besar",
  "historical_prices": [45000, 47000, 52000, ...],
  "historical_volumes": [120, 150, 100, ...],
  "exogenous_features": {
    "weather_index": 0.8,
    "holiday_proximity": 0.2
  }
}
```

#### **Contoh Response**
```json
{
  "predicted_price": 58000,
  "confidence_interval": [54000, 62000],
  "feature_importance": {
    "lag_1": 0.35,
    "ma_7": 0.25,
    "supply_demand_ratio": 0.20
  },
  "explanation": "Prediksi harga: Rp58.000. Faktor utama: lag_1 (+0.35), ma_7 (+0.25), supply_demand_ratio (+0.20)"
}
```

## 10. DESIGN SYSTEM & UX GUIDELINES

### 10.1 Prinsip Desain
1. **Data-first:** Visualisasi jelas, hierarki informasi kuat.
2. **Role-aware:** Tampilan menyesuaikan peran (bukan hanya konten).
3. **Actionable:** Setiap insight memiliki CTA (call-to-action).
4. **Trustworthy:** XAI (explainability) tertanam di UI.
5. **Offline-resilient:** Mobile-first untuk lapangan.

### 10.2 Design Tokens

#### **Warna**
```
Primary:    #1E3A5F (Deep Navy)   — Header, CTA utama
Secondary:  #2E86AB (Ocean Blue)  — Aksen, link
Success:    #27AE60 (Green)       — Status normal
Warning:    #F39C12 (Amber)       — Status kuning
Danger:     #E74C3C (Red)         — Status merah
Neutral:    #F5F4EF (Cream)       — Background
Text:       #1F1E1D (Charcoal)    — Teks utama
Muted:      #6B6A68 (Gray)        — Teks sekunder
```

#### **Tipografi**
```
Font Family: Inter, system-ui, sans-serif
H1: 32px / 700
H2: 24px / 600
H3: 18px / 600
Body: 14px / 400
Caption: 12px / 400
```

#### **Spacing**
```
xs: 4px  sm: 8px  md: 16px  lg: 24px  xl: 32px  2xl: 48px
```

#### **Radius**
```
sm: 2px  md: 3px  lg: 6px  full: 9999px
```

#### **Shadow**
```
sm: 0 1px 3px rgba(251, 250, 250, 0.12)
md: 0 4px 6px rgba(251, 250, 250, 0.12)
lg: 0 10px 15px rgba(251, 250, 250, 0.12)
```

### 10.3 Komponen Utama

| Komponen | Deskripsi |
|----------|-----------|
| Sidebar (Web) | Navigasi kiri, collapsible, grouping modul |
| Drawer (Mobile) | Navigasi geser, grouping per role |
| KPI Card | Kartu metrik dengan trend indicator |
| Map View | Peta interaktif (react-map-gl/Leaflet) |
| Chart | Line, bar, gauge (Recharts) |
| Data Table | Sortable, filterable, pagination |
| Alert Banner | Notifikasi EWS (hijau/kuning/merah) |
| Workflow Stepper | Visualisasi siklus intervensi |
| Policy Brief Editor | Rich text dengan komentar |
| Offline Indicator | Status koneksi & sync |

### 10.4 Pola UX Kunci

#### **Untuk Eksekutif (Dewan Pengarah)**
- **Progressive disclosure:** Ringkasan → detail → raw data.
- **One-glance dashboard:** KPI utama dalam 1 layar.
- **Approval cepat:** Tombol setujui/tolak dengan konteks.

#### **Untuk Operasional (Agen Lapangan)**
- **Offline-first:** Input tanpa internet.
- **Large tap targets:** Minimal 48px.
- **Camera integration:** Foto langsung.
- **GPS auto-capture:** Koordinat otomatis.
- **Simple forms:** Minimal field, validasi inline.

#### **Untuk Analis (Manajer Data)**
- **Data density tinggi:** Tabel & chart detail.
- **Keyboard shortcuts:** Untuk power user.
- **Export options:** CSV, Excel, PDF.
- **Version history:** Untuk model & data.

### 10.5 Aksesibilitas
- Kontras minimal 4.5:1.
- Fokus state jelas.
- Alt text untuk gambar.
- ARIA labels untuk screen reader.
- Keyboard navigation penuh.

## 11. KEAMANAN, PRIVASI & TATA KELOLA

### 11.1 Kontrol Akses (RBAC)

```
Role → Permission → Resource → Action
```

**Matriks Permission (Ringkasan):**
| Resource | Dewan | Mgr Data | Mgr Kebijakan | Mgr Pembiayaan | Mgr Monitoring | Mgr Operasional | Koord | Agen | Cold Storage | Mutu | Agregator |
|----------|:---:|:---:|:---:|:---:|:---:|:---:|:---:|:---:|:---:|:---:|:---:|
| Data Telemetri | R | RW | R | – | R | R | RW | RW | R | R | RW |
| Data BPS/BI | R | RW | R | R | R | R | R | – | – | – | – |
| Data Fiskal | R | RW | R | RW | R | – | – | – | – | – | – |
| Model AI | – | RW | – | – | – | – | – | – | – | – | – |
| Policy Brief | RW | – | RW | – | – | – | – | – | – | – | – |
| Intervensi | A | – | – | – | – | RW | – | – | – | – | – |
| Audit Log | R | R | R | R | RW | R | – | – | – | – | – |

*Keterangan: R=Read, W=Write, A=Approve, RW=Read+Write*

### 11.2 Keamanan Teknis
- **Autentikasi:** JWT + Refresh Token, OAuth 2.0.
- **Otorisasi:** RBAC + ABAC (attribute-based).
- **Enkripsi:** TLS 1.3, AES-256 at rest.
- **API Security:** Rate limiting, API key, CORS.
- **Secrets Management:** Vault / K8s Secrets.
- **Audit Log:** Immutable, append-only.

### 11.3 Privasi Data
- **Data pribadi produsen:** Enkripsi, akses terbatas.
- **Federated Learning:** Latih model tanpa pindah data sensitif.
- **Differential Privacy:** Untuk data fiskal.
- **Homomorphic Encryption:** Untuk data sensitif.
- **Consent Management:** Persetujuan produsen.

### 11.4 Tata Kelola AI
- **Transparansi:** Model dapat dijelaskan (SHAP/LIME).
- **Akuntabilitas:** Human-in-the-loop untuk keputusan kritis.
- **Keadilan:** Tidak diskriminatif.
- **Audit:** Review berkala model & data.
- **Etika:** Kode etik agen lapangan (dilarang tentukan harga).

## 12. METRIK KEBERHASILAN (KPI)

### 12.1 KPI Teknis
| Kategori | Indikator | Target |
|----------|-----------|--------|
| Data & Sistem | Akurasi proyeksi produksi | ≥ 98% |
| Data & Sistem | Tingkat kelengkapan produsen terverifikasi | ≥ 95% |
| Data & Sistem | Uptime sistem | ≥ 99.5% |
| Data & Sistem | Latensi prediksi | < 2 detik |

### 12.2 KPI Logistik & Infrastruktur
| Indikator | Target |
|-----------|--------|
| Utilisasi cold storage | ≥ 75% |
| Penurunan food loss | ≥ 60% |
| Total volume diagregasi | ≥ 10.000 ton/bulan |
| Efisiensi biaya logistik | ≥ 25% |

### 12.3 KPI Dampak Ekonomi
| Indikator | Sebelum | Target |
|-----------|---------|--------|
| Volatilitas harga (CV) | 12.5% | ≤ 7.8% |
| Margin harga produsen-konsumen | 45% | ≤ 32% |
| Inflasi pangan regional | 6.8% | ≤ 4.2% |

### 12.4 KPI Kelembagaan
| Indikator | Target |
|-----------|--------|
| Rasio tindak lanjut rekomendasi AI | ≥ 78% |
| Indeks kelancaran koordinasi silang | ≥ 4.2/5.0 |
| UAT pass rate | ≥ 85% |
| SUS Score | ≥ 82/100 |
| NPS | ≥ +45 |

### 12.5 KPI Model AI
| Model | Metrik | Target |
|-------|--------|--------|
| Prediksi Harga | MAPE | ≤ 5.4% |
| Prediksi Harga | R² | ≥ 0.90 |
| EWS | F1-Score | ≥ 0.88 |
| EWS | AUC-ROC | ≥ 0.94 |
| Shelf-Life | RMSE | ≤ 1 hari |

## 13. ROADMAP & FASE RILIS

### 13.1 Fase Pengembangan

#### **TAHAP 1: Fondasi & Pilot (Bulan 0-3)**
- Penetapan SK Mandat.
- Penguncian 2-3 komoditas prioritas (cabai, bawang merah, beras).
- Penyusunan database pilot.
- Validasi SOP mahasiswa.
- Pengembangan MVP: Auth, RBAC, Dashboard dasar, Input telemetri mobile.

**Deliverable:**
- Prototipe platform (web + mobile).
- Database pilot.
- SOP & kode etik.

#### **TAHAP 2: Agregasi & Distribusi (Bulan 4-9)**
- Operasionalisasi dashboard fase awal.
- Uji coba distribusi antardaerah.
- Desain studi kelayakan cold storage.
- Pengembangan model AI (prediksi harga, EWS).

**Deliverable:**
- Dashboard operasional & eksekutif.
- Model prediksi harga (MAPE ≤ 5.4%).
- Model EWS (F1 ≥ 0.88).
- Studi kelayakan cold storage.

#### **TAHAP 3: Eskalasi & Fisik (Bulan 10-24)**
- Operasional fasilitas percontohan.
- Eksekusi kontrak pasokan jangka panjang.
- Integrasi rekomendasi AI ke APBD/perencanaan daerah.
- Pengembangan penuh Layer 4 (Workflow Engine).

**Deliverable:**
- Cold storage percontohan.
- Workflow engine aktif.
- Integrasi APBD.
- Policy brief generator (LLM).

### 13.2 Prioritas Rilis Fitur

| Prioritas | Fitur | Fase |
|-----------|-------|------|
| P0 | Auth & RBAC | Fase 1 |
| P0 | Input Telemetri Mobile | Fase 1 |
| P0 | Dashboard Dasar | Fase 1 |
| P0 | Prediksi Harga | Fase 2 |
| P0 | EWS | Fase 2 |
| P0 | Dashboard Eksekutif | Fase 2 |
| P0 | Policy Brief Generator | Fase 2 |
| P0 | Workflow Engine | Fase 3 |
| P1 | Optimasi Rute | Fase 2 |
| P1 | Cold Storage Mgmt | Fase 3 |
| P1 | Computer Vision QC | Fase 3 |
| P2 | Federated Learning | Fase 3+ |
| P2 | RL untuk Intervensi | Fase 3+ |

## 14. RISIKO & MITIGASI

| No | Risiko | Dampak | Probabilitas | Mitigasi |
|----|--------|--------|--------------|----------|
| R1 | Kualitas data lapangan rendah | Tinggi | Tinggi | Validasi berlapis, anomaly detection, cross-check BPS/BI |
| R2 | Adopsi pengguna rendah | Tinggi | Sedang | UAT, pelatihan, insentif, UI ramah |
| R3 | Model AI tidak akurat | Tinggi | Sedang | Hybrid model, ensemble, continuous retraining |
| R4 | Kebocoran data sensitif | Tinggi | Rendah | Enkripsi, RBAC, federated learning |
| R5 | Ketergantungan API eksternal | Sedang | Sedang | Fallback manual, caching, multiple sources |
| R6 | Resistensi kelembagaan | Sedang | Sedang | Sosialisasi, quick wins, policy brief |
| R7 | Skalabilitas infrastruktur | Sedang | Rendah | Kubernetes, cloud-native, load testing |
| R8 | Etika AI dilanggar | Tinggi | Rendah | AI governance, audit, human-in-the-loop |

## 15. LAMPIRAN

### Lampiran A: Daftar Istilah
| Istilah | Definisi |
|---------|----------|
| TPID | Tim Pengendalian Inflasi Daerah |
| EWS | Early Warning System |
| DSS | Decision Support System |
| RBAC | Role-Based Access Control |
| VRP | Vehicle Routing Problem |
| SHAP | SHapley Additive exPlanations |
| LIME | Local Interpretable Model-agnostic Explanations |
| TFT | Temporal Fusion Transformer |
| MAPE | Mean Absolute Percentage Error |
| AUC-ROC | Area Under the Receiver Operating Characteristic Curve |
| SUS | System Usability Scale |
| NPS | Net Promoter Score |
| RAG | Retrieval-Augmented Generation |
| RLHF | Reinforcement Learning from Human Feedback |
| MDP | Markov Decision Process |
| POMDP | Partially Observable Markov Decision Process |

### Lampiran B: Referensi
1. Hevner, A. R., et al. (2004). Design Science in Information Systems Research. MIS Quarterly.
2. Lim, B., et al. (2021). Temporal Fusion Transformers. International Journal of Forecasting.
3. Lundberg, S. M., & Lee, S. I. (2017). A Unified Approach to Interpreting Model Predictions. NeurIPS.
4. Ribeiro, M. T., et al. (2016). "Why Should I Trust You?" KDD.
5. Bank Indonesia. (2023). Laporan Perekonomian Indonesia.
6. Badan Pusat Statistik. (2023). Statistik Indonesia.

### Lampiran C: Wireframe Referensi
- Web: Sidebar per role (1440×900)
- Android: Drawer per role (390×844)
- Peta menu semua role (1760×2080)

### Lampiran D: Kontak & Stakeholder
| Peran | Institusi | Tanggung Jawab |
|-------|-----------|----------------|
| Product Owner | FMIPA USK | Visi produk, prioritas |
| Tech Lead | FMIPA USK | Arsitektur, teknis |
| AI Lead | FMIPA USK | Model, evaluasi |
| UX Lead | FMIPA USK | Desain, riset pengguna |
| Stakeholder | TPID, Pemda, BI, BPS, DJPb | Kebutuhan, validasi |

**Dokumen ini merupakan bagian dari penelitian magister Kecerdasan Buatan FMIPA USK dan menjadi acuan pengembangan Platform AI Garda Inflasi.**

**Disusun oleh:** Tim Riset AI Garda Inflasi
**Institusi:** FMIPA Universitas Syiah Kuala
**Tahun:** 2025
