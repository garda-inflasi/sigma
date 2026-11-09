-- GARDA INFLASI - Aturan Ambang Batas Peringatan Dini (EWS)

-- =====================================================================
-- Tabel: aturan_ews (9 baris)
-- =====================================================================
INSERT INTO aturan_ews (kode, komoditas_kode, kabupaten_kode, jenis, ambang_waspada, ambang_siaga, ambang_kritis, jendela_hari, keterangan, aktif) VALUES
  ('ATR-CU-BRS-MED', 'BRS-MED', NULL, 'ANOMALI_CUACA', 150, 220, 300, 10, 'Ambang awal, harus dikalibrasi bersama BI/BPS.', TRUE),
  ('ATR-CU-BWM', 'BWM', NULL, 'ANOMALI_CUACA', 100, 140, 200, 10, 'Ambang awal, harus dikalibrasi bersama BI/BPS.', TRUE),
  ('ATR-CU-CBM-KRT', 'CBM-KRT', NULL, 'ANOMALI_CUACA', 100, 140, 200, 10, 'Ambang awal, harus dikalibrasi bersama BI/BPS.', TRUE),
  ('ATR-KS-BRS-MED', 'BRS-MED', NULL, 'KELANGKAAN_STOK', 20, 35, 50, 5, 'Ambang awal, harus dikalibrasi bersama BI/BPS.', TRUE),
  ('ATR-KS-BWM', 'BWM', NULL, 'KELANGKAAN_STOK', 25, 40, 55, 5, 'Ambang awal, harus dikalibrasi bersama BI/BPS.', TRUE),
  ('ATR-KS-CBM-KRT', 'CBM-KRT', NULL, 'KELANGKAAN_STOK', 25, 40, 55, 5, 'Ambang awal, harus dikalibrasi bersama BI/BPS.', TRUE),
  ('ATR-LH-BRS-MED', 'BRS-MED', NULL, 'LONJAKAN_HARGA', 3, 5, 8, 7, 'Ambang awal, harus dikalibrasi bersama BI/BPS.', TRUE),
  ('ATR-LH-BWM', 'BWM', NULL, 'LONJAKAN_HARGA', 8, 12, 18, 7, 'Ambang awal, harus dikalibrasi bersama BI/BPS.', TRUE),
  ('ATR-LH-CBM-KRT', 'CBM-KRT', NULL, 'LONJAKAN_HARGA', 8, 12, 18, 7, 'Ambang awal, harus dikalibrasi bersama BI/BPS.', TRUE)
ON CONFLICT (kode) DO NOTHING;

