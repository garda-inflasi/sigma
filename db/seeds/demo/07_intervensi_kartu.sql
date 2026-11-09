-- GARDA INFLASI - Kartu Intervensi Demo (papan kanban TPID)
--
-- Setara dengan SeedDemo seeder Go lama (internal/intervensi/seeder): 19 kartu contoh yang meniru mockKanban
-- (apps/web/src/lib/api/mock.ts) = 7 kartu tekanan harga, 6 normal, 6 surplus, tersebar di semua tahap, beserta
-- penanggung jawab, riwayat pemindahan tahap, dan satu permintaan persetujuan terbuka (INT-304 ke t-fiskal).
-- TIDAK memuat kartu buatan workflow otomatis dari alert rule engine.
--
-- * Waktu relatif terhadap now() saat berkas dijalankan: kartu ke-n dibuat (240 - n) jam lalu, tenggat dalam hari dari
--   sekarang (negatif = terlambat), tahap_sejak = dibuat + (posisi tahap - 1) x 6 jam, kartu di tahap penutup
--   (*-selesai) selesai 24 jam lalu. Dengan begitu SLA tahap dan KPI bulanan demo selalu bermakna kapan pun dijalankan.
-- * Kode tampilan INT-301..INT-319 berada DI BAWAH rentang intervensi_kode_seq (mulai 1000): kartu baru tidak pernah
--   bentrok dengan seed. Jangan menaruh kode INT-1000 ke atas di sini.
-- * ID kartu = UUID stabil garda_uuid_stabil('intervensi', <kode>) (sama dengan idx.UUIDStabil dan migrasi 000112);
--   ews_alert_id dicari dari kode alert (demo/06_ews_alerts.sql). Pengguna dirujuk lewat slug akun demo.
-- * Idempoten: dijalankan ulang, kartu contoh, penanggung jawab, riwayat, dan permintaan persetujuannya dikembalikan
--   ke keadaan awal (waktu ikut disegarkan) tanpa menggandakan baris. Kartu lain tidak disentuh.
--
-- Prasyarat: reference/01_master.sql, reference/08_intervensi_tahap.sql, demo/01_auth_users.sql, demo/06_ews_alerts.sql.

-- Gagal dengan pesan jelas bila prasyarat belum di-seed: tanpa pemeriksaan ini ews_alert_id kartu diam-diam NULL.
DO $$
BEGIN
  IF NOT EXISTS (SELECT 1 FROM tahap_intervensi_def) THEN
    RAISE EXCEPTION 'seed kartu intervensi: definisi tahap belum ada (jalankan seed reference lebih dulu: reference/08_intervensi_tahap.sql)';
  END IF;
  IF (SELECT COUNT(*) FROM ews_alert WHERE kode IN ('EWS-1000', 'EWS-1002', 'EWS-1003', 'EWS-1012', 'EWS-1013')) < 5 THEN
    RAISE EXCEPTION 'seed kartu intervensi: alert ews demo belum ada (jalankan demo/06_ews_alerts.sql lebih dulu)';
  END IF;
END $$;

-- =====================================================================
-- Tabel: intervensi (19 baris)
-- =====================================================================
INSERT INTO intervensi (id, kode, siklus, tahap, urutan, judul, deskripsi, komoditas_kode, kabupaten_kode, prioritas, tenggat,
                        rekomendasi_ai, checklist_selesai, checklist_total, ews_alert_id, dibuat_oleh_id, dibuat_pada,
                        selesai_pada, versi, dieskalasi_pada, dieskalasi_oleh_id, tahap_sejak)
SELECT garda_uuid_stabil('intervensi', s.kode), s.kode, CAST(s.siklus AS siklus), s.tahap,
       -- urutan kartu dalam satu tahap mengikuti urutan kemunculan di daftar ini
       CAST(row_number() OVER (PARTITION BY s.tahap ORDER BY s.no) - 1 AS integer),
       s.judul, '', s.komoditas_kode, s.kabupaten_kode, CAST(s.prioritas AS prioritas),
       now() + s.tenggat_hari * interval '24 hours',
       s.rekomendasi_ai, s.checklist_selesai, s.checklist_total,
       (SELECT a.id FROM ews_alert a WHERE a.kode = s.alert_kode),
       garda_uuid_stabil('pengguna', 'manajer_program'),
       now() - interval '240 hours' + s.no * interval '1 hour',
       CASE WHEN s.tahap LIKE '%-selesai' THEN now() - interval '24 hours' END,
       1, NULL, NULL,
       now() - interval '240 hours' + s.no * interval '1 hour' + (d.urutan - 1) * interval '6 hours'
  FROM (VALUES
    (0, 'INT-301', 'TEKANAN', 't-ews', 'CBM-KRT', '1171', 'TINGGI', TRUE, 'EWS-1000', 1, 5, 1, 'Lonjakan cabai merah Banda Aceh'),
    (1, 'INT-302', 'TEKANAN', 't-ews', 'CBR-RWT', '1174', 'TINGGI', TRUE, 'EWS-1002', 2, 4, 0, 'Pasokan cabai rawit Lhokseumawe'),
    (2, 'INT-303', 'TEKANAN', 't-verifikasi', 'BWM', '1107', 'SEDANG', TRUE, 'EWS-1003', 3, 6, 2, 'Bawang merah Meulaboh naik 9%'),
    (3, 'INT-304', 'TEKANAN', 't-logistik', 'CBM-KRT', '1106', 'TINGGI', FALSE, NULL, 4, 7, 3, 'Kirim cabai Takengon → Banda Aceh'),
    (4, 'INT-305', 'TEKANAN', 't-fiskal', 'CBM-KRT', '1101', 'SEDANG', FALSE, NULL, 6, 5, 4, 'Subsidi ongkos angkut Simeulue'),
    (5, 'INT-306', 'TEKANAN', 't-verifikasi', 'IKN-TGK', '1172', 'RENDAH', FALSE, NULL, -1, 3, 1, 'Verifikasi harga ikan tongkol Sabang'),
    (6, 'INT-307', 'TEKANAN', 't-petakan', 'CBM-KRT', '1171', 'TINGGI', FALSE, NULL, 2, 6, 2, 'Operasi pasar cabai Peunayong'),
    (7, 'INT-308', 'NORMAL', 'n-pantau', 'BRS-MED', '1171', 'RENDAH', FALSE, NULL, 5, 4, 1, 'Rekap harga beras pekan 40'),
    (8, 'INT-309', 'NORMAL', 'n-verifikasi', 'BRS-MED', '1171', 'SEDANG', FALSE, NULL, 3, 5, 2, 'Cocokkan IHK BPS September'),
    (9, 'INT-310', 'NORMAL', 'n-jadwal', 'TLR', '1110', 'RENDAH', FALSE, NULL, 6, 4, 2, 'Jadwal armada Bireuen–Lhokseumawe'),
    (10, 'INT-311', 'NORMAL', 'n-agregasi', 'TLR', '1110', 'SEDANG', FALSE, NULL, 4, 6, 3, 'Agregasi telur peternak Bireuen'),
    (11, 'INT-312', 'NORMAL', 'n-pantau', 'DAG-AYM', '1173', 'RENDAH', FALSE, NULL, 1, 3, 0, 'Pantau daging ayam Langsa'),
    (12, 'INT-313', 'NORMAL', 'n-selesai', 'GLA', '1109', 'RENDAH', FALSE, NULL, -2, 3, 3, 'Verifikasi gula pasir Pidie'),
    (13, 'INT-314', 'SURPLUS', 's-deteksi', 'KTG', '1106', 'TINGGI', TRUE, 'EWS-1012', 2, 5, 1, 'Kentang Gayo: harga produsen −18%'),
    (14, 'INT-315', 'SURPLUS', 's-deteksi', 'TMT', '1117', 'TINGGI', TRUE, 'EWS-1013', 3, 4, 0, 'Tomat Bener Meriah surplus 1.200 ton'),
    (15, 'INT-316', 'SURPLUS', 's-coldstorage', 'KTG', '1106', 'SEDANG', FALSE, NULL, 4, 6, 3, 'Alokasi 80 ton ke CS Takengon'),
    (16, 'INT-317', 'SURPLUS', 's-olahan', 'TMT', '1117', 'SEDANG', FALSE, NULL, 7, 7, 2, 'Uji olahan saus tomat UMKM'),
    (17, 'INT-318', 'SURPLUS', 's-ekspansi', 'KTG', '1106', 'SEDANG', FALSE, NULL, 5, 5, 3, 'Pembeli kentang dari Medan'),
    (18, 'INT-319', 'SURPLUS', 's-selesai', 'KTG', '1106', 'RENDAH', FALSE, NULL, -1, 4, 4, 'Keripik kentang — mitra koperasi')
       ) AS s (no, kode, siklus, tahap, komoditas_kode, kabupaten_kode, prioritas, rekomendasi_ai, alert_kode, tenggat_hari,
               checklist_total, checklist_selesai, judul)
  LEFT JOIN tahap_intervensi_def d ON d.siklus = CAST(s.siklus AS siklus) AND d.kode_tahap = s.tahap
 ORDER BY s.no
ON CONFLICT (id) DO UPDATE SET
       kode = EXCLUDED.kode, siklus = EXCLUDED.siklus, tahap = EXCLUDED.tahap, urutan = EXCLUDED.urutan,
       judul = EXCLUDED.judul, deskripsi = EXCLUDED.deskripsi, komoditas_kode = EXCLUDED.komoditas_kode,
       kabupaten_kode = EXCLUDED.kabupaten_kode, prioritas = EXCLUDED.prioritas, tenggat = EXCLUDED.tenggat,
       rekomendasi_ai = EXCLUDED.rekomendasi_ai, checklist_selesai = EXCLUDED.checklist_selesai,
       checklist_total = EXCLUDED.checklist_total, ews_alert_id = EXCLUDED.ews_alert_id,
       dibuat_oleh_id = EXCLUDED.dibuat_oleh_id, dibuat_pada = EXCLUDED.dibuat_pada, selesai_pada = EXCLUDED.selesai_pada,
       versi = EXCLUDED.versi, dieskalasi_pada = EXCLUDED.dieskalasi_pada, dieskalasi_oleh_id = EXCLUDED.dieskalasi_oleh_id,
       tahap_sejak = EXCLUDED.tahap_sejak;

-- =====================================================================
-- Tabel: intervensi_pj (28 baris). Dikembalikan ke keadaan awal: baris lama kartu contoh dibuang lebih dulu.
-- =====================================================================
DELETE FROM intervensi_pj
 WHERE intervensi_id IN (SELECT garda_uuid_stabil('intervensi', 'INT-' || n) FROM generate_series(301, 319) AS n);

INSERT INTO intervensi_pj (intervensi_id, pengguna_id)
SELECT garda_uuid_stabil('intervensi', p.kode), garda_uuid_stabil('pengguna', p.pengguna)
  FROM (VALUES
    ('INT-301', 'manajer_program'),
    ('INT-302', 'manajer_mutu'),
    ('INT-302', 'manajer_data'),
    ('INT-303', 'manajer_kebijakan'),
    ('INT-304', 'manajer_risiko'),
    ('INT-304', 'agregator_data'),
    ('INT-305', 'manajer_mutu'),
    ('INT-306', 'koordinator_lapangan'),
    ('INT-306', 'manajer_data'),
    ('INT-307', 'agregator_data'),
    ('INT-308', 'manajer_risiko'),
    ('INT-308', 'manajer_program'),
    ('INT-309', 'manajer_data'),
    ('INT-310', 'manajer_kebijakan'),
    ('INT-310', 'koordinator_lapangan'),
    ('INT-311', 'manajer_risiko'),
    ('INT-312', 'manajer_mutu'),
    ('INT-312', 'manajer_program'),
    ('INT-313', 'koordinator_lapangan'),
    ('INT-314', 'manajer_kebijakan'),
    ('INT-314', 'agregator_data'),
    ('INT-315', 'manajer_program'),
    ('INT-316', 'manajer_mutu'),
    ('INT-316', 'manajer_data'),
    ('INT-317', 'manajer_kebijakan'),
    ('INT-318', 'manajer_risiko'),
    ('INT-318', 'agregator_data'),
    ('INT-319', 'manajer_mutu')
       ) AS p (kode, pengguna)
ON CONFLICT (intervensi_id, pengguna_id) DO NOTHING;

-- =====================================================================
-- Tabel: intervensi_riwayat (50 baris). Tiap kartu dibuat di tahap pertama siklusnya, lalu dipindah satu per satu
-- (selang 6 jam) sampai tahapnya sekarang oleh penanggung jawab pertamanya. jam = jam sesudah kartu dibuat.
-- Dikembalikan ke keadaan awal: riwayat lama kartu contoh dibuang lebih dulu (id baris dari sequence).
-- =====================================================================
DELETE FROM intervensi_riwayat
 WHERE intervensi_id IN (SELECT garda_uuid_stabil('intervensi', 'INT-' || n) FROM generate_series(301, 319) AS n);

INSERT INTO intervensi_riwayat (intervensi_id, aksi, dari, ke, oleh, waktu)
SELECT k.id, r.aksi, r.dari, r.ke, garda_uuid_stabil('pengguna', r.pelaku), k.dibuat_pada + r.jam * interval '1 hour'
  FROM (VALUES
    (1, 'INT-301', 'BUAT', NULL, 't-ews', 'manajer_program', 0),
    (2, 'INT-302', 'BUAT', NULL, 't-ews', 'manajer_program', 0),
    (3, 'INT-303', 'BUAT', NULL, 't-ews', 'manajer_program', 0),
    (4, 'INT-303', 'PINDAH_TAHAP', 't-ews', 't-verifikasi', 'manajer_kebijakan', 6),
    (5, 'INT-304', 'BUAT', NULL, 't-ews', 'manajer_program', 0),
    (6, 'INT-304', 'PINDAH_TAHAP', 't-ews', 't-verifikasi', 'manajer_risiko', 6),
    (7, 'INT-304', 'PINDAH_TAHAP', 't-verifikasi', 't-petakan', 'manajer_risiko', 12),
    (8, 'INT-304', 'PINDAH_TAHAP', 't-petakan', 't-logistik', 'manajer_risiko', 18),
    (9, 'INT-305', 'BUAT', NULL, 't-ews', 'manajer_program', 0),
    (10, 'INT-305', 'PINDAH_TAHAP', 't-ews', 't-verifikasi', 'manajer_mutu', 6),
    (11, 'INT-305', 'PINDAH_TAHAP', 't-verifikasi', 't-petakan', 'manajer_mutu', 12),
    (12, 'INT-305', 'PINDAH_TAHAP', 't-petakan', 't-logistik', 'manajer_mutu', 18),
    (13, 'INT-305', 'PINDAH_TAHAP', 't-logistik', 't-fiskal', 'manajer_mutu', 24),
    (14, 'INT-306', 'BUAT', NULL, 't-ews', 'manajer_program', 0),
    (15, 'INT-306', 'PINDAH_TAHAP', 't-ews', 't-verifikasi', 'koordinator_lapangan', 6),
    (16, 'INT-307', 'BUAT', NULL, 't-ews', 'manajer_program', 0),
    (17, 'INT-307', 'PINDAH_TAHAP', 't-ews', 't-verifikasi', 'agregator_data', 6),
    (18, 'INT-307', 'PINDAH_TAHAP', 't-verifikasi', 't-petakan', 'agregator_data', 12),
    (19, 'INT-308', 'BUAT', NULL, 'n-pantau', 'manajer_program', 0),
    (20, 'INT-309', 'BUAT', NULL, 'n-pantau', 'manajer_program', 0),
    (21, 'INT-309', 'PINDAH_TAHAP', 'n-pantau', 'n-verifikasi', 'manajer_data', 6),
    (22, 'INT-310', 'BUAT', NULL, 'n-pantau', 'manajer_program', 0),
    (23, 'INT-310', 'PINDAH_TAHAP', 'n-pantau', 'n-verifikasi', 'manajer_kebijakan', 6),
    (24, 'INT-310', 'PINDAH_TAHAP', 'n-verifikasi', 'n-jadwal', 'manajer_kebijakan', 12),
    (25, 'INT-311', 'BUAT', NULL, 'n-pantau', 'manajer_program', 0),
    (26, 'INT-311', 'PINDAH_TAHAP', 'n-pantau', 'n-verifikasi', 'manajer_risiko', 6),
    (27, 'INT-311', 'PINDAH_TAHAP', 'n-verifikasi', 'n-jadwal', 'manajer_risiko', 12),
    (28, 'INT-311', 'PINDAH_TAHAP', 'n-jadwal', 'n-agregasi', 'manajer_risiko', 18),
    (29, 'INT-312', 'BUAT', NULL, 'n-pantau', 'manajer_program', 0),
    (30, 'INT-313', 'BUAT', NULL, 'n-pantau', 'manajer_program', 0),
    (31, 'INT-313', 'PINDAH_TAHAP', 'n-pantau', 'n-verifikasi', 'koordinator_lapangan', 6),
    (32, 'INT-313', 'PINDAH_TAHAP', 'n-verifikasi', 'n-jadwal', 'koordinator_lapangan', 12),
    (33, 'INT-313', 'PINDAH_TAHAP', 'n-jadwal', 'n-agregasi', 'koordinator_lapangan', 18),
    (34, 'INT-313', 'PINDAH_TAHAP', 'n-agregasi', 'n-selesai', 'koordinator_lapangan', 24),
    (35, 'INT-314', 'BUAT', NULL, 's-deteksi', 'manajer_program', 0),
    (36, 'INT-315', 'BUAT', NULL, 's-deteksi', 'manajer_program', 0),
    (37, 'INT-316', 'BUAT', NULL, 's-deteksi', 'manajer_program', 0),
    (38, 'INT-316', 'PINDAH_TAHAP', 's-deteksi', 's-coldstorage', 'manajer_data', 6),
    (39, 'INT-317', 'BUAT', NULL, 's-deteksi', 'manajer_program', 0),
    (40, 'INT-317', 'PINDAH_TAHAP', 's-deteksi', 's-coldstorage', 'manajer_kebijakan', 6),
    (41, 'INT-317', 'PINDAH_TAHAP', 's-coldstorage', 's-olahan', 'manajer_kebijakan', 12),
    (42, 'INT-318', 'BUAT', NULL, 's-deteksi', 'manajer_program', 0),
    (43, 'INT-318', 'PINDAH_TAHAP', 's-deteksi', 's-coldstorage', 'manajer_risiko', 6),
    (44, 'INT-318', 'PINDAH_TAHAP', 's-coldstorage', 's-olahan', 'manajer_risiko', 12),
    (45, 'INT-318', 'PINDAH_TAHAP', 's-olahan', 's-ekspansi', 'manajer_risiko', 18),
    (46, 'INT-319', 'BUAT', NULL, 's-deteksi', 'manajer_program', 0),
    (47, 'INT-319', 'PINDAH_TAHAP', 's-deteksi', 's-coldstorage', 'manajer_mutu', 6),
    (48, 'INT-319', 'PINDAH_TAHAP', 's-coldstorage', 's-olahan', 'manajer_mutu', 12),
    (49, 'INT-319', 'PINDAH_TAHAP', 's-olahan', 's-ekspansi', 'manajer_mutu', 18),
    (50, 'INT-319', 'PINDAH_TAHAP', 's-ekspansi', 's-selesai', 'manajer_mutu', 24)
       ) AS r (urut, kode, aksi, dari, ke, pelaku, jam)
  JOIN intervensi k ON k.id = garda_uuid_stabil('intervensi', r.kode)
 ORDER BY r.urut;

-- =====================================================================
-- Tabel: intervensi_persetujuan (1 baris) dan intervensi_persetujuan_langkah (2 baris)
-- INT-304 (menunggu di Usulan intervensi logistik kilat) diajukan pindah ke Dukungan instrumen belanja; rantai
-- langkahnya disalin dari definisi alur seperti pada permintaan nyata. Dibangun ulang tiap dijalankan (id dari sequence).
-- =====================================================================
DELETE FROM intervensi_persetujuan
 WHERE intervensi_id IN (SELECT garda_uuid_stabil('intervensi', 'INT-' || n) FROM generate_series(301, 319) AS n);

INSERT INTO intervensi_persetujuan (intervensi_id, alur_kode, jenis, tahap_tujuan, diajukan_oleh, diajukan_pada, catatan)
VALUES (garda_uuid_stabil('intervensi', 'INT-304'), 'pindah-tekanan-fiskal', 'PINDAH_TAHAP', 't-fiskal',
        garda_uuid_stabil('pengguna', 'manajer_program'), now() - interval '3 hours',
        'Rute dan volume sudah siap; mohon dukungan ongkos angkut');

INSERT INTO intervensi_persetujuan_langkah (persetujuan_id, urutan, label, izin_dibutuhkan)
SELECT p.id, l.urutan, l.label, l.izin_dibutuhkan
  FROM intervensi_persetujuan p
  JOIN alur_persetujuan_langkah l ON l.alur_kode = p.alur_kode
 WHERE p.intervensi_id = garda_uuid_stabil('intervensi', 'INT-304') AND p.status = 'MENUNGGU'
ON CONFLICT (persetujuan_id, urutan) DO NOTHING;
