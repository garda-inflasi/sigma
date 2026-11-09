-- GARDA INFLASI - Data Inflasi, Neraca Pangan, dan Ruang Fiskal Eksekutif (Demo)
--
-- Setara dengan seeder Go lama eksekutif.SeedDemo(now): angka TIRUAN ber-sumber 'DEMO' yang relatif terhadap
-- tanggal eksekusi, bukan tanggal tetap. "Hari ini" dan "bulan lalu" dihitung dari (now() AT TIME ZONE 'UTC'), bukan
-- CURRENT_DATE: CURRENT_DATE mengikuti zona waktu sesi, dan sesi aplikasi/tes (pkg/database, TimeZone=Asia/Jakarta)
-- berbeda dari sesi cmd/seed (zona bawaan server, UTC), sehingga bulan bisa berbeda di sekitar pergantian bulan.
--   * inflasi_bulanan : 12 bulan berurutan yang berakhir di BULAN LALU (bulan terakhir = 2,84% yoy, 0,41% mtm);
--   * andil_inflasi   : andil enam komoditas pada bulan lalu;
--   * neraca_pangan   : neraca delapan komoditas pada bulan lalu (stok tidak diketahui, NULL);
--   * ruang_fiskal    : empat instrumen per akhir bulan lalu.
-- Baris yang sudah berasal dari sumber resmi (BPS, BI, DJPB, BAPANAS) tidak pernah ditimpa: pembaruan hanya
-- berlaku untuk baris ber-sumber 'DEMO'. Idempoten; ID identity dibuat basis data (kunci upsert = kunci alami).
-- Prasyarat: komoditas (reference/01_master.sql) dan target inflasi (reference/13_kpi_definisi.sql).

-- =====================================================================
-- Tabel: inflasi_bulanan (12 baris, provinsi Aceh)
-- IHK mengikuti kompaun mtm dari 108,4; ytd = nomor bulan x 0,213 (pendekatan linear, dibulatkan 2 desimal).
-- =====================================================================
INSERT INTO inflasi_bulanan (wilayah_kode, periode, ihk, inflasi_mtm, inflasi_yoy, inflasi_ytd, sumber)
SELECT '11', p.periode, s.ihk, s.mtm, s.yoy, round(EXTRACT(MONTH FROM p.periode) * 0.213, 2), 'DEMO'
  FROM (VALUES
         (0, 108.4, 0.12, 2.12), (1, 108.313, -0.08, 1.94), (2, 108.692, 0.35, 2.31), (3, 108.997, 0.28, 2.58),
         (4, 109.149, 0.14, 2.41), (5, 109.095, -0.05, 2.2), (6, 109.433, 0.31, 2.47), (7, 109.717, 0.26, 2.69),
         (8, 109.597, -0.11, 2.33), (9, 109.805, 0.19, 2.51), (10, 110.047, 0.22, 2.63), (11, 110.498, 0.41, 2.84)
       ) AS s (urut, ihk, mtm, yoy)
 CROSS JOIN LATERAL (
         SELECT (date_trunc('month', (now() AT TIME ZONE 'UTC')) - make_interval(months => 12 - s.urut))::date AS periode
       ) AS p
ON CONFLICT (wilayah_kode, periode) DO UPDATE
   SET ihk = EXCLUDED.ihk, inflasi_mtm = EXCLUDED.inflasi_mtm, inflasi_yoy = EXCLUDED.inflasi_yoy, inflasi_ytd = EXCLUDED.inflasi_ytd
 WHERE inflasi_bulanan.sumber = 'DEMO';

-- =====================================================================
-- Tabel: andil_inflasi (6 baris, bulan lalu)
-- =====================================================================
INSERT INTO andil_inflasi (periode, wilayah_kode, komoditas_kode, andil, inflasi_pct, sumber)
SELECT (date_trunc('month', (now() AT TIME ZONE 'UTC')) - interval '1 month')::date, '11', v.komoditas_kode, v.andil, v.inflasi_pct, 'DEMO'
  FROM (VALUES
         ('CBM-KRT', 0.18, 14.2), ('BWM', 0.06, 6.8), ('IKN-TGK', 0.04, 3.9),
         ('BRS-MED', 0.03, 1.6), ('TMT', -0.02, -5.4), ('KTG', -0.01, -3.1)
       ) AS v (komoditas_kode, andil, inflasi_pct)
ON CONFLICT (periode, wilayah_kode, komoditas_kode) DO UPDATE
   SET andil = EXCLUDED.andil, inflasi_pct = EXCLUDED.inflasi_pct
 WHERE andil_inflasi.sumber = 'DEMO';

-- =====================================================================
-- Tabel: neraca_pangan (8 baris, bulan lalu; stok_ton NULL = tidak diketahui, bukan 0)
-- =====================================================================
INSERT INTO neraca_pangan (komoditas_kode, wilayah_kode, periode, produksi_ton, konsumsi_ton, stok_ton, sumber)
SELECT v.komoditas_kode, '11', (date_trunc('month', (now() AT TIME ZONE 'UTC')) - interval '1 month')::date, v.produksi_ton, v.konsumsi_ton, NULL::numeric, 'DEMO'
  FROM (VALUES
         ('BRS-MED', 146200, 52400), ('KTG', 9800, 3200), ('TMT', 4100, 2900), ('IKN-TGK', 7400, 6100),
         ('DAG-AYM', 6200, 6000), ('TLR', 4800, 5600), ('CBM-KRT', 3100, 3900), ('BWM', 900, 2600)
       ) AS v (komoditas_kode, produksi_ton, konsumsi_ton)
ON CONFLICT (komoditas_kode, wilayah_kode, periode) DO UPDATE
   SET produksi_ton = EXCLUDED.produksi_ton, konsumsi_ton = EXCLUDED.konsumsi_ton, stok_ton = EXCLUDED.stok_ton
 WHERE neraca_pangan.sumber = 'DEMO';

-- =====================================================================
-- Tabel: ruang_fiskal (4 baris, per akhir bulan lalu)
-- kode = 'DEMO-RF-<YYYYMM>-<urut>'. Kunci upsert adalah kunci alami (tahun, instrumen, per_tanggal): baris resmi untuk
-- instrumen dan tanggal yang sama dilewati, bukan menabrak UNIQUE.
-- =====================================================================
INSERT INTO ruang_fiskal (kode, tahun, instrumen, pagu, realisasi, per_tanggal, sumber)
SELECT 'DEMO-RF-' || to_char(t.per_tanggal, 'YYYYMM') || '-' || v.urut, EXTRACT(YEAR FROM t.per_tanggal)::int,
       v.instrumen, v.pagu, v.realisasi, t.per_tanggal, 'DEMO'
  FROM (SELECT (date_trunc('month', (now() AT TIME ZONE 'UTC')) - interval '1 day')::date AS per_tanggal) AS t
 CROSS JOIN (VALUES
         (1, 'Belanja tidak terduga (BTT)', 120000000000::bigint, 31200000000::bigint),
         (2, 'Operasi pasar murah', 24000000000::bigint, 11200000000::bigint),
         (3, 'Subsidi ongkos angkut', 18000000000::bigint, 9400000000::bigint),
         (4, 'Gerakan tanam pangan', 24000000000::bigint, 2400000000::bigint)
       ) AS v (urut, instrumen, pagu, realisasi)
ON CONFLICT (tahun, instrumen, per_tanggal) DO UPDATE
   SET pagu = EXCLUDED.pagu, realisasi = EXCLUDED.realisasi
 WHERE ruang_fiskal.sumber = 'DEMO';
