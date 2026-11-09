-- GARDA INFLASI - Batas Harga Acuan Awal Komoditas

-- =====================================================================
-- Tabel: batas_harga (32 baris)
-- =====================================================================
INSERT INTO batas_harga (kode, komoditas_kode, kabupaten_kode, batas_min, batas_maks, berlaku_dari) VALUES
  ('BH-BRS-MED', 'BRS-MED', NULL, 3500, 41700, '2024-01-01'),
  ('BH-BRS-PRM', 'BRS-PRM', NULL, 4000, 47400, '2024-01-01'),
  ('BH-BWM', 'BWM', NULL, 10300, 123000, '2024-01-01'),
  ('BH-BWP', 'BWP', NULL, 9500, 114000, '2024-01-01'),
  ('BH-CBM-KRT', 'CBM-KRT', NULL, 13000, 156000, '2024-01-01'),
  ('BH-CBR-RWT', 'CBR-RWT', NULL, 14500, 174000, '2024-01-01'),
  ('BH-DAG-AYM', 'DAG-AYM', NULL, 9100, 109500, '2024-01-01'),
  ('BH-DAG-SPI', 'DAG-SPI', NULL, 37000, 444000, '2024-01-01'),
  ('BH-GLA', 'GLA', NULL, 4500, 54000, '2024-01-01'),
  ('BH-IKN-TGK', 'IKN-TGK', NULL, 9500, 114000, '2024-01-01'),
  ('BH-KTG', 'KTG', NULL, 4000, 48000, '2024-01-01'),
  ('BH-MYK', 'MYK', NULL, 4400, 52500, '2024-01-01'),
  ('BH-TLR', 'TLR', NULL, 7400, 88500, '2024-01-01'),
  ('BH-TMT', 'TMT', NULL, 3000, 36000, '2024-01-01'),
  ('BH-bawang_merah', 'bawang_merah', NULL, 8800, 105000, '2024-01-01'),
  ('BH-bawang_putih', 'bawang_putih', NULL, 8000, 96000, '2024-01-01'),
  ('BH-beras_medium', 'beras_medium', NULL, 3500, 41700, '2024-01-01'),
  ('BH-beras_premium', 'beras_premium', NULL, 4000, 47700, '2024-01-01'),
  ('BH-cabai_merah_keriting', 'cabai_merah_keriting', NULL, 12000, 144000, '2024-01-01'),
  ('BH-cabai_rawit_merah', 'cabai_rawit_merah', NULL, 13800, 165000, '2024-01-01'),
  ('BH-daging_ayam', 'daging_ayam', NULL, 8800, 105000, '2024-01-01'),
  ('BH-daging_sapi', 'daging_sapi', NULL, 32500, 390000, '2024-01-01'),
  ('BH-gula_pasir', 'gula_pasir', NULL, 4000, 48000, '2024-01-01'),
  ('BH-ikan_kembung', 'ikan_kembung', NULL, 8800, 105000, '2024-01-01'),
  ('BH-ikan_tongkol', 'ikan_tongkol', NULL, 8000, 96000, '2024-01-01'),
  ('BH-jagung', 'jagung', NULL, 2000, 24000, '2024-01-01'),
  ('BH-kedelai', 'kedelai', NULL, 3500, 42000, '2024-01-01'),
  ('BH-kentang', 'kentang', NULL, 4000, 48000, '2024-01-01'),
  ('BH-minyak_goreng', 'minyak_goreng', NULL, 4500, 54000, '2024-01-01'),
  ('BH-telur_ayam', 'telur_ayam', NULL, 7000, 84000, '2024-01-01'),
  ('BH-tepung_terigu', 'tepung_terigu', NULL, 3000, 36000, '2024-01-01'),
  ('BH-tomat', 'tomat', NULL, 3000, 36000, '2024-01-01')
ON CONFLICT (kode) DO NOTHING;

