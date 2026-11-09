-- GARDA INFLASI - Penyegaran Materialized View & Sinkronisasi Sequence Pasca-Seeding Demo

SELECT garda_segarkan_mv('agregat_harga_harian', false);
SELECT garda_segarkan_mv('mv_heatmap_harga', false);
SELECT garda_segarkan_mv('mv_kpi_operasional', false);
SELECT garda_segarkan_mv('gold_harga_harian', false);

-- Sinkronisasi sequence identity ke nilai ID tertinggi pasca-seeding
SELECT setval(pg_get_serial_sequence('entri_harga', 'id'), COALESCE((SELECT max(id) FROM entri_harga), 1));
SELECT setval(pg_get_serial_sequence('riwayat_status_entri', 'id'), COALESCE((SELECT max(id) FROM riwayat_status_entri), 1));
SELECT setval(pg_get_serial_sequence('entri_harga_versi', 'id'), COALESCE((SELECT max(id) FROM entri_harga_versi), 1));
SELECT setval(pg_get_serial_sequence('ews_alert', 'id'), COALESCE((SELECT max(id) FROM ews_alert), 1));
SELECT setval(pg_get_serial_sequence('ews_riwayat', 'id'), COALESCE((SELECT max(id) FROM ews_riwayat), 1));
SELECT setval(pg_get_serial_sequence('tugas_lapangan', 'id'), COALESCE((SELECT max(id) FROM tugas_lapangan), 1));
SELECT setval(pg_get_serial_sequence('prediksi_harga', 'id'), COALESCE((SELECT max(id) FROM prediksi_harga), 1));
SELECT setval(pg_get_serial_sequence('ruang_fiskal', 'id'), COALESCE((SELECT max(id) FROM ruang_fiskal), 1));
SELECT setval(pg_get_serial_sequence('siklus_ekonomi', 'id'), COALESCE((SELECT max(id) FROM siklus_ekonomi), 1));
SELECT setval(pg_get_serial_sequence('neraca_pangan', 'id'), COALESCE((SELECT max(id) FROM neraca_pangan), 1));
SELECT setval(pg_get_serial_sequence('kpi_nilai', 'id'), COALESCE((SELECT max(id) FROM kpi_nilai), 1));
SELECT setval(pg_get_serial_sequence('mfa_kode_cadangan', 'id'), COALESCE((SELECT max(id) FROM mfa_kode_cadangan), 1));
