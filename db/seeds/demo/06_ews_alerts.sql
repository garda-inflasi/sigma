-- GARDA INFLASI - Alert & Riwayat Peringatan Dini EWS Demo
--
-- Setara dengan SeedDemo seeder Go lama (internal/ews/seeder): 17 alert contoh yang meniru mockEws
-- (apps/web/src/lib/api/mock.ts) = 12 lonjakan harga (4 baru, 5 diakui, 3 dieskalasi), 2 indikasi surplus, dan tiga
-- alert tetap (cuaca, stok, data), beserta 31 baris riwayat status. TIDAK memuat alert buatan rule engine.
--
-- * Waktu relatif terhadap now() saat berkas dijalankan (EWS-1000 dibuat 4 menit lalu; alert yang sudah diakui
--   diakui 12 menit sesudah dibuat, dieskalasi 30 menit sesudah diakui, selesai 60 menit sesudah diakui), jadi data
--   demo selalu segar dan alert BARU yang lebih tua dari ambang eskalasi otomatis ikut terdeteksi.
-- * Kode tampilan EWS-0901..0903 dan EWS-1000..1013 berada DI BAWAH rentang ews_alert_kode_seq (mulai 2000): alert
--   buatan rule engine tidak pernah bentrok dengan seed. Jangan menaruh kode EWS-2000 ke atas di sini.
-- * ID baris (BIGINT identity) dibuat basis data menurut urutan baris di bawah; kunci idempotensi adalah kode
--   (EWS-n untuk alert, RW-D-<kode alert>-<n> untuk riwayat). aturan_id dicari dari kode aturan, pengakui dari akun demo.
-- * Idempoten: dijalankan ulang, alert contoh dikembalikan ke keadaan awal (waktu ikut disegarkan) tanpa menggandakan baris.
--
-- Prasyarat: reference/01_master.sql (komoditas, kabupaten), reference/07_ews_aturan.sql, demo/01_auth_users.sql.

-- Gagal dengan pesan jelas bila prasyarat belum di-seed: tanpa pemeriksaan ini aturan_id alert diam-diam NULL.
DO $$
BEGIN
  IF (SELECT COUNT(*) FROM aturan_ews
       WHERE kode IN ('ATR-LH-CBM-KRT', 'ATR-LH-BWM', 'ATR-LH-BRS-MED', 'ATR-CU-CBM-KRT', 'ATR-KS-CBM-KRT')) < 5 THEN
    RAISE EXCEPTION 'seed alert ews: aturan ews belum ada (jalankan seed reference lebih dulu: reference/07_ews_aturan.sql)';
  END IF;
END $$;

-- =====================================================================
-- Tabel: ews_alert (17 baris) dan ews_riwayat (31 baris)
-- =====================================================================
WITH seed (urut, kode, severity, jenis, komoditas_kode, kabupaten_kode, nilai, satuan_nilai, skor_dampak, status, menit_lalu, aturan_kode, pengakui, sumber, judul, pesan) AS (
  VALUES
    (1, 'EWS-1000', 'KRITIS', 'LONJAKAN_HARGA', 'CBM-KRT', '1171', 21.4, '%', 91, 'BARU', 4, 'ATR-LH-CBM-KRT', NULL, 'Model EWS v2.3 + entri lapangan', 'Lonjakan harga cabai merah keriting', 'Harga konsumen naik di atas rata-rata 7 hari. Harga terakhir Rp66.500/kg.'),
    (2, 'EWS-1001', 'KRITIS', 'LONJAKAN_HARGA', 'CBM-KRT', '1108', 19.2, '%', 86, 'BARU', 27, 'ATR-LH-CBM-KRT', NULL, 'Model EWS v2.3 + entri lapangan', 'Lonjakan harga cabai merah keriting', 'Harga konsumen naik di atas rata-rata 7 hari. Harga terakhir Rp64.800/kg.'),
    (3, 'EWS-1002', 'SIAGA', 'LONJAKAN_HARGA', 'CBR-RWT', '1174', 15.6, '%', 77, 'BARU', 50, NULL, NULL, 'Model EWS v2.3 + entri lapangan', 'Lonjakan harga cabai rawit merah', 'Harga konsumen naik di atas rata-rata 7 hari. Harga terakhir Rp71.200/kg.'),
    (4, 'EWS-1003', 'SIAGA', 'LONJAKAN_HARGA', 'BWM', '1107', 12.8, '%', 71, 'BARU', 73, 'ATR-LH-BWM', NULL, 'Model EWS v2.3 + entri lapangan', 'Lonjakan harga bawang merah', 'Harga konsumen naik di atas rata-rata 7 hari. Harga terakhir Rp49.700/kg.'),
    (5, 'EWS-1004', 'WASPADA', 'LONJAKAN_HARGA', 'CBM-KRT', '1106', 11.3, '%', 67, 'DIAKUI', 96, 'ATR-LH-CBM-KRT', 'manajer_data', 'Model EWS v2.3 + entri lapangan', 'Lonjakan harga cabai merah keriting', 'Harga konsumen naik di atas rata-rata 7 hari. Harga terakhir Rp58.400/kg.'),
    (6, 'EWS-1005', 'WASPADA', 'LONJAKAN_HARGA', 'BWM', '1109', 9.4, '%', 63, 'DIAKUI', 119, 'ATR-LH-BWM', 'koordinator_lapangan', 'Model EWS v2.3 + entri lapangan', 'Lonjakan harga bawang merah', 'Harga konsumen naik di atas rata-rata 7 hari. Harga terakhir Rp47.300/kg.'),
    (7, 'EWS-1006', 'WASPADA', 'LONJAKAN_HARGA', 'TLR', '1110', 8.7, '%', 61, 'DIAKUI', 142, NULL, 'manajer_risiko', 'Model EWS v2.3 + entri lapangan', 'Lonjakan harga telur ayam ras', 'Harga konsumen naik di atas rata-rata 7 hari. Harga terakhir Rp32.100/kg.'),
    (8, 'EWS-1007', 'WASPADA', 'LONJAKAN_HARGA', 'DAG-AYM', '1173', 8.2, '%', 60, 'DIAKUI', 165, NULL, 'manajer_data', 'Model EWS v2.3 + entri lapangan', 'Lonjakan harga daging ayam ras', 'Harga konsumen naik di atas rata-rata 7 hari. Harga terakhir Rp39.600/kg.'),
    (9, 'EWS-1008', 'WASPADA', 'LONJAKAN_HARGA', 'BRS-MED', '1174', 4.2, '%', 50, 'DIAKUI', 188, 'ATR-LH-BRS-MED', 'koordinator_lapangan', 'Model EWS v2.3 + entri lapangan', 'Lonjakan harga beras medium', 'Harga konsumen naik di atas rata-rata 7 hari. Harga terakhir Rp14.500/kg.'),
    (10, 'EWS-1009', 'SIAGA', 'LONJAKAN_HARGA', 'CBR-RWT', '1171', 13.9, '%', 73, 'DIESKALASI', 211, NULL, 'manajer_risiko', 'Model EWS v2.3 + entri lapangan', 'Lonjakan harga cabai rawit merah', 'Harga konsumen naik di atas rata-rata 7 hari. Harga terakhir Rp68.900/kg.'),
    (11, 'EWS-1010', 'SIAGA', 'LONJAKAN_HARGA', 'CBM-KRT', '1174', 16.4, '%', 79, 'DIESKALASI', 234, 'ATR-LH-CBM-KRT', 'manajer_data', 'Model EWS v2.3 + entri lapangan', 'Lonjakan harga cabai merah keriting', 'Harga konsumen naik di atas rata-rata 7 hari. Harga terakhir Rp62.700/kg.'),
    (12, 'EWS-1011', 'WASPADA', 'LONJAKAN_HARGA', 'BWM', '1171', 10.2, '%', 64, 'DIESKALASI', 257, 'ATR-LH-BWM', 'koordinator_lapangan', 'Model EWS v2.3 + entri lapangan', 'Lonjakan harga bawang merah', 'Harga konsumen naik di atas rata-rata 7 hari. Harga terakhir Rp46.800/kg.'),
    (13, 'EWS-1012', 'INFO', 'OVER_SUPPLY', 'KTG', '1106', -18, '%', 66, 'BARU', 280, NULL, NULL, 'Model EWS v2.3', 'Indikasi surplus kentang', 'Harga turun di bawah koridor. Pertimbangkan penyerapan ke cold storage.'),
    (14, 'EWS-1013', 'INFO', 'OVER_SUPPLY', 'TMT', '1117', -14.6, '%', 59, 'BARU', 311, NULL, NULL, 'Model EWS v2.3', 'Indikasi surplus tomat', 'Harga turun di bawah koridor. Pertimbangkan penyerapan ke cold storage.'),
    (15, 'EWS-0901', 'SIAGA', 'ANOMALI_CUACA', 'CBM-KRT', '1106', 148, 'mm', 81, 'DIAKUI', 95, 'ATR-CU-CBM-KRT', 'koordinator_lapangan', 'BMKG + OpenWeather', 'Curah hujan ekstrem di sentra Gayo', 'Curah hujan 2,1× normal selama 10 hari. Risiko gagal panen cabai & tomat meningkat.'),
    (16, 'EWS-0902', 'WASPADA', 'KELANGKAAN_STOK', 'CBM-KRT', '1108', -32, '%', 74, 'BARU', 140, 'ATR-KS-CBM-KRT', NULL, 'Telemetri cold storage', 'Stok cabai di Cold Storage Lambaro menipis', 'Stok turun 32% dalam 5 hari; cukup untuk ±4 hari permintaan Banda Aceh.'),
    (17, 'EWS-0903', 'INFO', 'ANOMALI_DATA', NULL, '1104', 3, '%', 22, 'SELESAI', 210, NULL, 'manajer_data', 'Validasi otomatis', 'Entri harga di luar geofence', '3 entri tercatat lebih dari 1 km dari Pasar Kutacane. Perlu verifikasi koordinator.')
), dasar AS (
  SELECT s.*,
         now() - s.menit_lalu * interval '1 minute' AS dibuat,
         now() - (s.menit_lalu - 12) * interval '1 minute' AS diakui
    FROM seed s
), alert AS (
  INSERT INTO ews_alert (kode, severity, jenis, judul, pesan, komoditas_kode, kabupaten_kode, nilai, satuan_nilai, skor_dampak,
                         status, sumber, dibuat_pada, diakui_oleh_id, diakui_pada, aturan_id, dieskalasi_pada,
                         dieskalasi_oleh_id, selesai_pada)
  SELECT d.kode, CAST(d.severity AS severity), CAST(d.jenis AS jenis_ews), d.judul, d.pesan, d.komoditas_kode, d.kabupaten_kode,
         d.nilai, d.satuan_nilai, d.skor_dampak, CAST(d.status AS status_ews), d.sumber, d.dibuat,
         CASE WHEN d.status <> 'BARU' THEN garda_uuid_stabil('pengguna', d.pengakui) END,
         CASE WHEN d.status <> 'BARU' THEN d.diakui END,
         (SELECT t.id FROM aturan_ews t WHERE t.kode = d.aturan_kode),
         CASE WHEN d.status = 'DIESKALASI' THEN d.diakui + interval '30 minutes' END,
         CASE WHEN d.status = 'DIESKALASI' THEN garda_uuid_stabil('pengguna', d.pengakui) END,
         CASE WHEN d.status = 'SELESAI' THEN d.diakui + interval '60 minutes' END
    FROM dasar d
   ORDER BY d.urut
  ON CONFLICT (kode) DO UPDATE SET
         severity = EXCLUDED.severity, jenis = EXCLUDED.jenis, judul = EXCLUDED.judul, pesan = EXCLUDED.pesan,
         komoditas_kode = EXCLUDED.komoditas_kode, kabupaten_kode = EXCLUDED.kabupaten_kode, nilai = EXCLUDED.nilai,
         satuan_nilai = EXCLUDED.satuan_nilai, skor_dampak = EXCLUDED.skor_dampak, status = EXCLUDED.status,
         sumber = EXCLUDED.sumber, dibuat_pada = EXCLUDED.dibuat_pada, diakui_oleh_id = EXCLUDED.diakui_oleh_id,
         diakui_pada = EXCLUDED.diakui_pada, aturan_id = EXCLUDED.aturan_id, dieskalasi_pada = EXCLUDED.dieskalasi_pada,
         dieskalasi_oleh_id = EXCLUDED.dieskalasi_oleh_id, selesai_pada = EXCLUDED.selesai_pada
  RETURNING id, kode, dibuat_pada, diakui_oleh_id, diakui_pada, dieskalasi_oleh_id, dieskalasi_pada, selesai_pada
)
-- Riwayat mengikuti daur hidup tiap alert: dibuat model, lalu diakui, lalu dieskalasi atau diselesaikan.
INSERT INTO ews_riwayat (kode, alert_id, dari, ke, oleh, waktu, catatan)
SELECT 'RW-D-' || r.alert_kode || '-' || r.no, r.alert_id, CAST(r.dari AS status_ews), CAST(r.ke AS status_ews), r.oleh, r.waktu, r.catatan
  FROM (
        SELECT s.urut, 0 AS no, a.kode AS alert_kode, a.id AS alert_id, NULL AS dari, 'BARU' AS ke,
               CAST(NULL AS uuid) AS oleh, a.dibuat_pada AS waktu, 'Dibuat oleh model EWS' AS catatan
          FROM alert a JOIN seed s ON s.kode = a.kode
        UNION ALL
        SELECT s.urut, 1, a.kode, a.id, 'BARU', 'DIAKUI', a.diakui_oleh_id, a.diakui_pada, ''
          FROM alert a JOIN seed s ON s.kode = a.kode WHERE a.diakui_pada IS NOT NULL
        UNION ALL
        SELECT s.urut, 2, a.kode, a.id, 'DIAKUI', 'DIESKALASI', a.dieskalasi_oleh_id, a.dieskalasi_pada, ''
          FROM alert a JOIN seed s ON s.kode = a.kode WHERE a.dieskalasi_pada IS NOT NULL
        UNION ALL
        SELECT s.urut, 2, a.kode, a.id, 'DIAKUI', 'SELESAI', a.diakui_oleh_id, a.selesai_pada, ''
          FROM alert a JOIN seed s ON s.kode = a.kode WHERE a.selesai_pada IS NOT NULL
       ) r
 ORDER BY r.urut, r.no
ON CONFLICT (kode) DO UPDATE SET
       alert_id = EXCLUDED.alert_id, dari = EXCLUDED.dari, ke = EXCLUDED.ke, oleh = EXCLUDED.oleh,
       waktu = EXCLUDED.waktu, catatan = EXCLUDED.catatan;
