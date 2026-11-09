-- GARDA INFLASI - Siklus Ekonomi Demo, Penyegaran Materialized View, dan Sinkronisasi Sequence Pasca-Seeding Demo
--
-- Setara dengan seeder Go lama dashboard.SeedDemo(now): satu siklus TEKANAN aktif untuk Aceh yang dimulai enam hari
-- sebelum waktu eksekusi (skenario demo: cabai melonjak), lalu agregat dashboard disegarkan dari data yang sudah
-- di-seed. Agregat harga dan KPI operasional tidak di-seed langsung; ia dihitung dari entri harga, stok cold storage,
-- dan tugas yang ada, sehingga berkas ini dijalankan paling akhir.

-- =====================================================================
-- Tabel: siklus_ekonomi (1 baris)
-- Kunci idempotensi = kode (UNIQUE): menjalankan ulang menggeser mulai_pada mengikuti waktu eksekusi, tanpa
-- menggandakan baris maupun mengganti ID identity-nya.
-- =====================================================================
INSERT INTO siklus_ekonomi (kode, wilayah_kode, siklus, mulai_pada, akhir_pada) VALUES
  ('SE-11-DEMO', '11', 'TEKANAN', date_trunc('second', now()) - interval '6 days', NULL)
ON CONFLICT (kode) DO UPDATE
   SET wilayah_kode = EXCLUDED.wilayah_kode, siklus = EXCLUDED.siklus, mulai_pada = EXCLUDED.mulai_pada, akhir_pada = EXCLUDED.akhir_pada;

-- =====================================================================
-- Penyegaran materialized view (urutan menurut ketergantungan: agregat lebih dulu, lalu turunannya)
-- =====================================================================
SELECT garda_segarkan_mv('agregat_harga_harian', false);
SELECT garda_segarkan_mv('mv_heatmap_harga', false);
SELECT garda_segarkan_mv('mv_kpi_operasional', false);
SELECT garda_segarkan_mv('gold_harga_harian', false);

-- =====================================================================
-- Sinkronisasi sequence identity ke nilai ID tertinggi pasca-seeding. Tabel kosong tidak menggeser sequence
-- (is_called = false), sehingga ID pertama tetap 1 walau hanya sebagian berkas seed yang dijalankan.
-- =====================================================================
SELECT setval(pg_get_serial_sequence('entri_harga', 'id'), COALESCE(max(id), 1), max(id) IS NOT NULL) FROM entri_harga;
SELECT setval(pg_get_serial_sequence('riwayat_status_entri', 'id'), COALESCE(max(id), 1), max(id) IS NOT NULL) FROM riwayat_status_entri;
SELECT setval(pg_get_serial_sequence('entri_harga_versi', 'id'), COALESCE(max(id), 1), max(id) IS NOT NULL) FROM entri_harga_versi;
SELECT setval(pg_get_serial_sequence('ews_alert', 'id'), COALESCE(max(id), 1), max(id) IS NOT NULL) FROM ews_alert;
SELECT setval(pg_get_serial_sequence('ews_riwayat', 'id'), COALESCE(max(id), 1), max(id) IS NOT NULL) FROM ews_riwayat;
SELECT setval(pg_get_serial_sequence('tugas_lapangan', 'id'), COALESCE(max(id), 1), max(id) IS NOT NULL) FROM tugas_lapangan;
SELECT setval(pg_get_serial_sequence('prediksi_harga', 'id'), COALESCE(max(id), 1), max(id) IS NOT NULL) FROM prediksi_harga;
SELECT setval(pg_get_serial_sequence('ruang_fiskal', 'id'), COALESCE(max(id), 1), max(id) IS NOT NULL) FROM ruang_fiskal;
SELECT setval(pg_get_serial_sequence('siklus_ekonomi', 'id'), COALESCE(max(id), 1), max(id) IS NOT NULL) FROM siklus_ekonomi;
SELECT setval(pg_get_serial_sequence('neraca_pangan', 'id'), COALESCE(max(id), 1), max(id) IS NOT NULL) FROM neraca_pangan;
SELECT setval(pg_get_serial_sequence('kpi_nilai', 'id'), COALESCE(max(id), 1), max(id) IS NOT NULL) FROM kpi_nilai;
SELECT setval(pg_get_serial_sequence('mfa_kode_cadangan', 'id'), COALESCE(max(id), 1), max(id) IS NOT NULL) FROM mfa_kode_cadangan;
