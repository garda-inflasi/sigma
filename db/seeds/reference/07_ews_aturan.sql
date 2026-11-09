-- GARDA INFLASI - Aturan Ambang Batas Peringatan Dini (EWS)
--
-- Setara dengan SeedReference seeder Go lama (internal/ews/seeder): tiga komoditas pilot fase 1 x tiga jenis alert
-- terukur. Ambang awal, bukan hasil kalibrasi: harus dikalibrasi bersama BI/BPS.
-- Satuan: lonjakan harga = % di atas rata-rata jendela; kelangkaan stok = % penurunan stok jendela; anomali cuaca =
-- curah hujan kumulatif jendela (mm).
--
-- Idempoten. ID baris (BIGINT identity) dibuat basis data; kunci idempotensi adalah kode (ATR-<LH|KS|CU>-<komoditas>).
-- Aturan yang sudah diubah lewat UI (versi > 1) tidak disentuh sama sekali; yang belum diubah mengikuti nilai berkas
-- ini, kecuali status aktif dan kanal yang dibiarkan.

-- =====================================================================
-- Tabel: aturan_ews (9 baris)
-- =====================================================================
INSERT INTO aturan_ews (kode, komoditas_kode, kabupaten_kode, jenis, ambang_waspada, ambang_siaga, ambang_kritis, jendela_hari, cooldown_jam, keterangan, aktif) VALUES
  ('ATR-LH-BRS-MED', 'BRS-MED', NULL, 'LONJAKAN_HARGA', 3, 5, 8, 7, 24, 'Ambang awal, harus dikalibrasi bersama BI/BPS.', TRUE),
  ('ATR-KS-BRS-MED', 'BRS-MED', NULL, 'KELANGKAAN_STOK', 20, 35, 50, 5, 24, 'Ambang awal, harus dikalibrasi bersama BI/BPS.', TRUE),
  ('ATR-CU-BRS-MED', 'BRS-MED', NULL, 'ANOMALI_CUACA', 150, 220, 300, 10, 24, 'Ambang awal, harus dikalibrasi bersama BI/BPS.', TRUE),
  ('ATR-LH-CBM-KRT', 'CBM-KRT', NULL, 'LONJAKAN_HARGA', 8, 12, 18, 7, 24, 'Ambang awal, harus dikalibrasi bersama BI/BPS.', TRUE),
  ('ATR-KS-CBM-KRT', 'CBM-KRT', NULL, 'KELANGKAAN_STOK', 25, 40, 55, 5, 24, 'Ambang awal, harus dikalibrasi bersama BI/BPS.', TRUE),
  ('ATR-CU-CBM-KRT', 'CBM-KRT', NULL, 'ANOMALI_CUACA', 100, 140, 200, 10, 24, 'Ambang awal, harus dikalibrasi bersama BI/BPS.', TRUE),
  ('ATR-LH-BWM', 'BWM', NULL, 'LONJAKAN_HARGA', 8, 12, 18, 7, 24, 'Ambang awal, harus dikalibrasi bersama BI/BPS.', TRUE),
  ('ATR-KS-BWM', 'BWM', NULL, 'KELANGKAAN_STOK', 25, 40, 55, 5, 24, 'Ambang awal, harus dikalibrasi bersama BI/BPS.', TRUE),
  ('ATR-CU-BWM', 'BWM', NULL, 'ANOMALI_CUACA', 100, 140, 200, 10, 24, 'Ambang awal, harus dikalibrasi bersama BI/BPS.', TRUE)
ON CONFLICT (kode) DO UPDATE SET
  komoditas_kode = EXCLUDED.komoditas_kode,
  jenis = EXCLUDED.jenis,
  ambang_waspada = EXCLUDED.ambang_waspada,
  ambang_siaga = EXCLUDED.ambang_siaga,
  ambang_kritis = EXCLUDED.ambang_kritis,
  jendela_hari = EXCLUDED.jendela_hari,
  cooldown_jam = EXCLUDED.cooldown_jam,
  keterangan = EXCLUDED.keterangan
WHERE aturan_ews.versi = 1;
