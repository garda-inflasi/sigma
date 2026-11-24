-- GARDA INFLASI - Titik Sentra Pertanian & Perkampungan Aceh (Referensi Spasial)
-- Memakai kode kabupaten BPS Aceh (prasyarat: reference/01_master.sql).

INSERT INTO lokasi_spasial (
  kode, jenis, nama, kabupaten_kode, komoditas_utama, luas_ha, estimasi_panen_ton, populasi, lat, lng, aktif
)
SELECT v.kode, v.jenis, v.nama, v.kabupaten_kode, v.komoditas_utama, v.luas_ha, v.estimasi_panen_ton, v.populasi, v.lat, v.lng, TRUE
FROM (
  VALUES
    ('STN-1106-01', 'PERTANIAN', 'Sentra Hortikultura Pegasing', '1106', 'Cabai merah keriting & Tomat', 420.5, 680.0, NULL::integer, 4.5975, 96.8142),
    ('STN-1117-01', 'PERTANIAN', 'Sentra Sayuran Bukit Redelong', '1117', 'Cabai merah & Bawang merah', 365.0, 540.0, NULL::integer, 4.7495, 96.8850),
    ('STN-1109-01', 'PERTANIAN', 'Sentra Cabai & Padi Indrajaya', '1109', 'Beras medium & Cabai rawit', 890.0, 1450.0, NULL::integer, 5.3312, 95.9218),
    ('STN-1108-01', 'PERTANIAN', 'Sentra Padi Seulimeum', '1108', 'Beras premium & Bawang merah', 760.0, 1180.0, NULL::integer, 5.4428, 95.5914),
    ('STN-1110-01', 'PERTANIAN', 'Sentra Pangan Peusangan', '1110', 'Beras medium & Tomat', 640.0, 920.0, NULL::integer, 5.1884, 96.7745),
    ('STN-1111-01', 'PERTANIAN', 'Sentra Padi Lhoksukon', '1111', 'Beras medium', 980.0, 1620.0, NULL::integer, 5.0412, 97.3108),
    ('STN-1113-01', 'PERTANIAN', 'Sentra Cabai Dataran Blangkejeren', '1113', 'Cabai merah keriting', 310.0, 410.0, NULL::integer, 3.9912, 97.3448),
    ('STN-1107-01', 'PERTANIAN', 'Sentra Pangan Johan Pahlawan', '1107', 'Beras & Cabai rawit', 480.0, 690.0, NULL::integer, 4.1620, 96.1425),
    ('KMP-1171-01', 'PERKAMPUNGAN', 'Gampong Lambaro Skep', '1171', 'Konsumsi Rumah Tangga', NULL::numeric, NULL::numeric, 6420, 5.5681, 95.3372),
    ('KMP-1108-01', 'PERKAMPUNGAN', 'Gampong Cot Irie Darussalam', '1108', 'Distribusi Pangan Peri-Urban', NULL::numeric, NULL::numeric, 4850, 5.5348, 95.3821),
    ('KMP-1106-01', 'PERKAMPUNGAN', 'Kampung Asir-Asir Lut Tawar', '1106', 'Petani Hortikultura', NULL::numeric, NULL::numeric, 3920, 4.6215, 96.8540),
    ('KMP-1174-01', 'PERKAMPUNGAN', 'Gampong Pusong Baru', '1174', 'Permukiman Pesisir & Pasar', NULL::numeric, NULL::numeric, 8150, 5.1745, 97.1492)
) AS v(kode, jenis, nama, kabupaten_kode, komoditas_utama, luas_ha, estimasi_panen_ton, populasi, lat, lng)
WHERE EXISTS (SELECT 1 FROM kabupaten k WHERE k.kode = v.kabupaten_kode)
ON CONFLICT (kode) DO NOTHING;
