-- GARDA INFLASI - Definisi & Target KPI

-- =====================================================================
-- Tabel: kpi_definisi (17 baris)
-- =====================================================================
INSERT INTO kpi_definisi (kode, kelompok, nama, deskripsi, rumus, satuan, arah_baik, sumber_query, urutan) VALUES
  ('DATA-PRODUSEN-TERVERIFIKASI', 'DATA_SISTEM', 'Kelengkapan produsen terverifikasi', 'Porsi produsen (kode pedagang) yang entri harganya lolos validasi.', '100 × produsen berentri DIVALIDASI/DIPUBLIKASI ÷ semua produsen berentri, pada bulan itu', '%', 'NAIK', 'produsen_terverifikasi', 10),
  ('DATA-KETEPATAN', 'DATA_SISTEM', 'Ketepatan data harga', 'Porsi entri harga yang sudah diputuskan dan lolos validasi. Target belum ditetapkan.', '100 × entri DIVALIDASI/DIPUBLIKASI ÷ entri DIVALIDASI/DIPUBLIKASI/DITOLAK, pada bulan itu', '%', 'NAIK', 'ketepatan_data', 20),
  ('DATA-KELENGKAPAN-HARIAN', 'DATA_SISTEM', 'Kelengkapan entri harian', 'Rata-rata harian pasangan pasar aktif × komoditas pilot yang punya entri tervalidasi.', 'rata-rata harian dari 100 × pasangan (pasar aktif, komoditas pilot) berentri tervalidasi ÷ seluruh pasangan', '%', 'NAIK', 'kelengkapan_harian', 30),
  ('DATA-AKURASI-PRODUKSI', 'DATA_SISTEM', 'Akurasi proyeksi produksi', 'Akurasi model proyeksi produksi. Menunggu model dan data evaluasi (16-B).', '100 − MAPE proyeksi produksi', '%', 'NAIK', NULL, 40),
  ('DATA-MAPE-HARGA', 'DATA_SISTEM', 'MAPE prediksi harga', 'Galat persentase absolut rata-rata proyeksi harga konsumen terhadap harga tervalidasi pada tanggal target. Muncul setelah proyeksi yang tanggal targetnya sudah lewat punya harga pembanding.', 'rata-rata |prediksi − aktual| ÷ aktual × 100, atas proyeksi bertanggal target pada bulan itu', '%', 'TURUN', 'mape_harga', 50),
  ('DATA-F1-EWS', 'DATA_SISTEM', 'F1-score EWS', 'Kinerja deteksi peringatan dini. Menunggu data evaluasi EWS (16-B).', '2 × presisi × recall ÷ (presisi + recall)', '', 'NAIK', NULL, 60),
  ('DATA-AUC-EWS', 'DATA_SISTEM', 'AUC-ROC EWS', 'Daya pembeda model EWS. Menunggu data evaluasi EWS (16-B).', 'luas di bawah kurva ROC', '', 'NAIK', NULL, 70),
  ('DATA-RMSE-SHELFLIFE', 'DATA_SISTEM', 'RMSE shelf-life', 'Galat model masa simpan. Menunggu data evaluasi model (16-B).', 'akar rata-rata (prediksi − aktual)² dalam hari', 'hari', 'TURUN', NULL, 80),
  ('LOG-UTILISASI-CS', 'LOGISTIK', 'Utilisasi cold storage', 'Volume stok tersimpan terhadap kapasitas fasilitas operasional. Pendekatan: lot dianggap tersimpan sampai batas masa simpannya karena stok keluar belum dicatat.', '100 × Σ volume lot aktif pada akhir periode ÷ Σ kapasitas fasilitas OPERASIONAL', '%', 'NAIK', 'utilisasi_cold_storage', 90),
  ('LOG-RISIKO-FOODLOSS', 'LOGISTIK', 'Risiko food loss stok', 'Estimasi risiko food loss stok cold storage (bukan kerugian terealisasi). Target belum ditetapkan.', '100 × Σ(volume × risiko) ÷ Σ volume lot aktif pada akhir periode', '%', 'TURUN', 'risiko_food_loss', 100),
  ('LOG-PENURUNAN-FOODLOSS', 'LOGISTIK', 'Penurunan food loss', 'Penurunan food loss terhadap baseline. Menunggu penetapan baseline dan data kerugian terealisasi (16-B).', '100 × (baseline − food loss periode) ÷ baseline', '%', 'NAIK', NULL, 110),
  ('LOG-VOLUME-AGREGASI', 'LOGISTIK', 'Volume teragregasi', 'Total volume yang masuk cold storage per bulan menurut buku besar transaksi stok (pendekatan untuk volume agregasi; pembelian di luar cold storage belum tercatat).', 'Σ volume transaksi MASUK dalam bulan', 'ton', 'NAIK', 'volume_agregasi', 120),
  ('EKO-VOLATILITAS', 'DAMPAK_EKONOMI', 'Volatilitas harga produsen (CV)', 'Koefisien variasi harga produsen tervalidasi, dirata-rata atas kelompok komoditas × kabupaten dengan minimal 5 entri. Baseline dan periode evaluasi menunggu penetapan pemangku kepentingan.', 'simpangan baku ÷ rata-rata harga × 100, dirata-rata atas komoditas × kabupaten', '%', 'TURUN', 'volatilitas_harga', 130),
  ('EKO-MARGIN', 'DAMPAK_EKONOMI', 'Margin produsen–konsumen', 'Selisih harga konsumen dan produsen tervalidasi, dirata-rata atas kelompok komoditas × kabupaten yang punya kedua harga. Baseline dan periode evaluasi menunggu penetapan pemangku kepentingan.', '(harga konsumen − harga produsen) ÷ harga konsumen × 100, dirata-rata atas komoditas × kabupaten', '%', 'TURUN', 'margin_produsen_konsumen', 140),
  ('EKO-EFISIENSI-BIAYA', 'DAMPAK_EKONOMI', 'Efisiensi biaya logistik', 'Penghematan biaya rencana distribusi optimal (12-C) terhadap rencana pembanding serakah (pasangan terdekat dulu) untuk volume yang sama. Hanya rencana yang disimpan; bukan penghematan terhadap biaya riil sebelum program.', '100 × (Σ biaya pembanding − Σ biaya optimal) ÷ Σ biaya pembanding, rencana terbaru per komoditas dan periode neraca', '%', 'NAIK', 'efisiensi_biaya_logistik', 150),
  ('TL-REKOMENDASI-AI', 'TINDAK_LANJUT', 'Rekomendasi AI ditindaklanjuti', 'Porsi intervensi berasal dari rekomendasi AI yang sudah selesai dieksekusi.', '100 × intervensi rekomendasi AI berstatus selesai ÷ semua intervensi rekomendasi AI yang dibuat pada bulan itu', '%', 'NAIK', 'rekomendasi_ai_dieksekusi', 160),
  ('TL-KOORDINASI', 'TINDAK_LANJUT', 'Indeks kelancaran koordinasi lintas lembaga', 'Skor survei koordinasi lintas lembaga. Menunggu data survei (16-B).', 'rata-rata skor survei (1–5)', '/5', 'NAIK', NULL, 170)
ON CONFLICT (kode) DO NOTHING;

-- =====================================================================
-- Tabel: kpi_target (15 baris)
-- =====================================================================
INSERT INTO kpi_target (kpi_kode, wilayah_kode, periode_mulai, target, ambang_waspada, baseline) VALUES
  ('DATA-AKURASI-PRODUKSI', NULL, '2025-01-01', 98.0, 88.2, NULL),
  ('DATA-AUC-EWS', NULL, '2025-01-01', 0.94, 0.846, NULL),
  ('DATA-F1-EWS', NULL, '2025-01-01', 0.88, 0.792, NULL),
  ('DATA-KELENGKAPAN-HARIAN', NULL, '2025-01-01', 95.0, 85.5, NULL),
  ('DATA-MAPE-HARGA', NULL, '2025-01-01', 5.4, 5.94, NULL),
  ('DATA-PRODUSEN-TERVERIFIKASI', NULL, '2025-01-01', 95.0, 85.5, NULL),
  ('DATA-RMSE-SHELFLIFE', NULL, '2025-01-01', 1.0, 1.1, NULL),
  ('EKO-EFISIENSI-BIAYA', NULL, '2025-01-01', 25.0, 22.5, NULL),
  ('EKO-MARGIN', NULL, '2025-01-01', 32.0, 35.2, NULL),
  ('EKO-VOLATILITAS', NULL, '2025-01-01', 7.8, 8.58, NULL),
  ('LOG-PENURUNAN-FOODLOSS', NULL, '2025-01-01', 60.0, 54.0, NULL),
  ('LOG-UTILISASI-CS', NULL, '2025-01-01', 75.0, 67.5, NULL),
  ('LOG-VOLUME-AGREGASI', NULL, '2025-01-01', 10000.0, 9000.0, NULL),
  ('TL-KOORDINASI', NULL, '2025-01-01', 4.2, 3.78, NULL),
  ('TL-REKOMENDASI-AI', NULL, '2025-01-01', 78.0, 70.2, NULL)
ON CONFLICT DO NOTHING;

-- =====================================================================
-- Tabel: target_kpi (3 baris)
-- =====================================================================
INSERT INTO target_kpi (indikator, periode_mulai, target, arah) VALUES
  ('cs', '2026-01-01', 75, 'NAIK'),
  ('validasi', '2026-01-01', 85, 'NAIK'),
  ('waspada', '2026-01-01', 3, 'TURUN')
ON CONFLICT (indikator, periode_mulai) DO NOTHING;

-- =====================================================================
-- Tabel: target_inflasi (3 baris)
-- =====================================================================
INSERT INTO target_inflasi (tahun, batas_bawah, batas_atas) VALUES
  (2025, 1.5, 3.5),
  (2026, 1.5, 3.5),
  (2027, 1.5, 3.5)
ON CONFLICT (tahun) DO NOTHING;

