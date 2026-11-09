-- GARDA INFLASI - Data Inflasi & Fiskal Eksekutif Demo

-- =====================================================================
-- Tabel: inflasi_bulanan (12 baris)
-- =====================================================================
INSERT INTO inflasi_bulanan (wilayah_kode, periode, ihk, inflasi_mtm, inflasi_yoy, inflasi_ytd, sumber) VALUES
  ('11', '2025-11-01', 108.4, 0.12, 2.12, 2.34, 'DEMO'),
  ('11', '2025-12-01', 108.313, -0.08, 1.94, 2.56, 'DEMO'),
  ('11', '2026-01-01', 108.692, 0.35, 2.31, 0.21, 'DEMO'),
  ('11', '2026-02-01', 108.997, 0.28, 2.58, 0.43, 'DEMO'),
  ('11', '2026-03-01', 109.149, 0.14, 2.41, 0.64, 'DEMO'),
  ('11', '2026-04-01', 109.095, -0.05, 2.2, 0.85, 'DEMO'),
  ('11', '2026-05-01', 109.433, 0.31, 2.47, 1.07, 'DEMO'),
  ('11', '2026-06-01', 109.717, 0.26, 2.69, 1.28, 'DEMO'),
  ('11', '2026-07-01', 109.597, -0.11, 2.33, 1.49, 'DEMO'),
  ('11', '2026-08-01', 109.805, 0.19, 2.51, 1.7, 'DEMO'),
  ('11', '2026-09-01', 110.047, 0.22, 2.63, 1.92, 'DEMO'),
  ('11', '2026-10-01', 110.498, 0.41, 2.84, 2.13, 'DEMO')
ON CONFLICT (wilayah_kode, periode) DO NOTHING;

-- =====================================================================
-- Tabel: andil_inflasi (6 baris)
-- =====================================================================
INSERT INTO andil_inflasi (periode, wilayah_kode, komoditas_kode, andil, inflasi_pct, sumber) VALUES
  ('2026-10-01', '11', 'CBM-KRT', 0.18, 14.2, 'DEMO'),
  ('2026-10-01', '11', 'BWM', 0.06, 6.8, 'DEMO'),
  ('2026-10-01', '11', 'IKN-TGK', 0.04, 3.9, 'DEMO'),
  ('2026-10-01', '11', 'BRS-MED', 0.03, 1.6, 'DEMO'),
  ('2026-10-01', '11', 'TMT', -0.02, -5.4, 'DEMO'),
  ('2026-10-01', '11', 'KTG', -0.01, -3.1, 'DEMO')
ON CONFLICT (periode, wilayah_kode, komoditas_kode) DO NOTHING;

-- =====================================================================
-- Tabel: neraca_pangan (8 baris)
-- =====================================================================
INSERT INTO neraca_pangan (komoditas_kode, wilayah_kode, periode, produksi_ton, konsumsi_ton, sumber) VALUES
  ('BRS-MED', '11', '2026-10-01', 146200, 52400, 'DEMO'),
  ('KTG', '11', '2026-10-01', 9800, 3200, 'DEMO'),
  ('TMT', '11', '2026-10-01', 4100, 2900, 'DEMO'),
  ('IKN-TGK', '11', '2026-10-01', 7400, 6100, 'DEMO'),
  ('DAG-AYM', '11', '2026-10-01', 6200, 6000, 'DEMO'),
  ('TLR', '11', '2026-10-01', 4800, 5600, 'DEMO'),
  ('CBM-KRT', '11', '2026-10-01', 3100, 3900, 'DEMO'),
  ('BWM', '11', '2026-10-01', 900, 2600, 'DEMO')
ON CONFLICT (komoditas_kode, wilayah_kode, periode) DO NOTHING;

-- =====================================================================
-- Tabel: ruang_fiskal (4 baris)
-- =====================================================================
INSERT INTO ruang_fiskal (id, kode, tahun, instrumen, pagu, realisasi, per_tanggal, sumber) OVERRIDING SYSTEM VALUE VALUES
  (1, 'DEMO-RF-202610-1', 2026, 'Belanja tidak terduga (BTT)', 120000000000, 31200000000, '2026-10-31', 'DEMO'),
  (2, 'DEMO-RF-202610-2', 2026, 'Operasi pasar murah', 24000000000, 11200000000, '2026-10-31', 'DEMO'),
  (3, 'DEMO-RF-202610-3', 2026, 'Subsidi ongkos angkut', 18000000000, 9400000000, '2026-10-31', 'DEMO'),
  (4, 'DEMO-RF-202610-4', 2026, 'Gerakan tanam pangan', 24000000000, 2400000000, '2026-10-31', 'DEMO')
ON CONFLICT (kode) DO NOTHING;

-- =====================================================================
-- Tabel: siklus_ekonomi (1 baris)
-- =====================================================================
INSERT INTO siklus_ekonomi (kode, wilayah_kode, siklus, mulai_pada, akhir_pada) VALUES
  ('SE-11-DEMO', '11', 'TEKANAN', CURRENT_DATE - INTERVAL '6 days', NULL)
ON CONFLICT (kode) DO UPDATE SET wilayah_kode = EXCLUDED.wilayah_kode, siklus = EXCLUDED.siklus, mulai_pada = EXCLUDED.mulai_pada, akhir_pada = EXCLUDED.akhir_pada;
