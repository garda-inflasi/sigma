-- GARDA INFLASI - Master Reference Seed Data
-- Idempoten: aman dieksekusi berulang kali via psql maupun runner.

-- =====================================================================
-- Tabel: satuan (4 baris)
-- =====================================================================
INSERT INTO satuan (kode, nama) VALUES
  ('Kg', 'Kg'),
  ('Liter', 'Liter'),
  ('kg', 'Kilogram'),
  ('liter', 'Liter')
ON CONFLICT (kode) DO NOTHING;

-- =====================================================================
-- Tabel: provinsi (1 baris)
-- =====================================================================
INSERT INTO provinsi (kode, nama) VALUES
  ('11', 'ACEH')
ON CONFLICT (kode) DO NOTHING;

-- =====================================================================
-- Tabel: kabupaten (23 baris)
-- =====================================================================
INSERT INTO kabupaten (kode, nama, jenis, ibukota, lat, lng, provinsi_kode) VALUES
  ('1101', 'KABUPATEN SIMEULUE', 'Kabupaten', 'Sinabang', 2.48, 96.38, '11'),
  ('1102', 'KABUPATEN ACEH SINGKIL', 'Kabupaten', 'Singkil', 2.28, 97.79, '11'),
  ('1103', 'KABUPATEN ACEH SELATAN', 'Kabupaten', 'Tapaktuan', 3.26, 97.18, '11'),
  ('1104', 'KABUPATEN ACEH TENGGARA', 'Kabupaten', 'Kutacane', 3.49, 97.8, '11'),
  ('1105', 'KABUPATEN ACEH TIMUR', 'Kabupaten', 'Idi Rayeuk', 4.96, 97.77, '11'),
  ('1106', 'KABUPATEN ACEH TENGAH', 'Kabupaten', 'Takengon', 4.62, 96.85, '11'),
  ('1107', 'KABUPATEN ACEH BARAT', 'Kabupaten', 'Meulaboh', 4.14, 96.13, '11'),
  ('1108', 'KABUPATEN ACEH BESAR', 'Kabupaten', 'Kota Jantho', 5.3, 95.63, '11'),
  ('1109', 'KABUPATEN PIDIE', 'Kabupaten', 'Sigli', 5.38, 95.96, '11'),
  ('1110', 'KABUPATEN BIREUEN', 'Kabupaten', 'Bireuen', 5.2, 96.7, '11'),
  ('1111', 'KABUPATEN ACEH UTARA', 'Kabupaten', 'Lhoksukon', 5.05, 97.32, '11'),
  ('1112', 'KABUPATEN ACEH BARAT DAYA', 'Kabupaten', 'Blangpidie', 3.74, 96.84, '11'),
  ('1113', 'KABUPATEN GAYO LUES', 'Kabupaten', 'Blangkejeren', 3.98, 97.35, '11'),
  ('1114', 'KABUPATEN ACEH TAMIANG', 'Kabupaten', 'Karang Baru', 4.29, 98.05, '11'),
  ('1115', 'KABUPATEN NAGAN RAYA', 'Kabupaten', 'Suka Makmue', 4.13, 96.47, '11'),
  ('1116', 'KABUPATEN ACEH JAYA', 'Kabupaten', 'Calang', 4.63, 95.58, '11'),
  ('1117', 'KABUPATEN BENER MERIAH', 'Kabupaten', 'Simpang Tiga Redelong', 4.73, 96.87, '11'),
  ('1118', 'KABUPATEN PIDIE JAYA', 'Kabupaten', 'Meureudu', 5.25, 96.25, '11'),
  ('1171', 'KOTA BANDA ACEH', 'Kota', 'Banda Aceh', 5.55, 95.32, '11'),
  ('1172', 'KOTA SABANG', 'Kota', 'Sabang', 5.89, 95.32, '11'),
  ('1173', 'KOTA LANGSA', 'Kota', 'Langsa', 4.47, 97.97, '11'),
  ('1174', 'KOTA LHOKSEUMAWE', 'Kota', 'Lhokseumawe', 5.18, 97.15, '11'),
  ('1175', 'KOTA SUBULUSSALAM', 'Kota', 'Subulussalam', 2.64, 98, '11')
ON CONFLICT (kode) DO NOTHING;

-- =====================================================================
-- Tabel: komoditas (32 baris)
-- =====================================================================
INSERT INTO komoditas (kode, nama, satuan, kategori, harga_acuan, pilot, shelf_life_hari) VALUES
  ('BRS-MED', 'Beras medium', 'kg', 'Pangan pokok', 13900, TRUE, 180),
  ('BRS-PRM', 'Beras premium', 'kg', 'Pangan pokok', 15800, FALSE, 180),
  ('BWM', 'Bawang merah', 'kg', 'Hortikultura', 41000, TRUE, 60),
  ('BWP', 'Bawang putih', 'kg', 'Hortikultura', 38000, FALSE, 120),
  ('CBM-KRT', 'Cabai merah keriting', 'kg', 'Hortikultura', 52000, TRUE, 21),
  ('CBR-RWT', 'Cabai rawit merah', 'kg', 'Hortikultura', 58000, FALSE, 18),
  ('DAG-AYM', 'Daging ayam ras', 'kg', 'Peternakan', 36500, FALSE, 90),
  ('DAG-SPI', 'Daging sapi', 'kg', 'Peternakan', 148000, FALSE, 180),
  ('GLA', 'Gula pasir', 'kg', 'Pangan olahan', 18000, FALSE, 365),
  ('IKN-TGK', 'Ikan tongkol', 'kg', 'Perikanan', 38000, FALSE, 120),
  ('KTG', 'Kentang', 'kg', 'Hortikultura', 16000, FALSE, 75),
  ('MYK', 'Minyak goreng curah', 'liter', 'Pangan olahan', 17500, FALSE, 365),
  ('TLR', 'Telur ayam ras', 'kg', 'Peternakan', 29500, FALSE, 30),
  ('TMT', 'Tomat', 'kg', 'Hortikultura', 12000, FALSE, 14),
  ('bawang_merah', 'Bawang Merah', 'Kg', 'Hortikultura', 35000, TRUE, 14),
  ('bawang_putih', 'Bawang Putih', 'Kg', 'Hortikultura', 32000, TRUE, 30),
  ('beras_medium', 'Beras Medium', 'Kg', 'Pangan Pokok', 13900, TRUE, 180),
  ('beras_premium', 'Beras Premium', 'Kg', 'Pangan Pokok', 15900, TRUE, 180),
  ('cabai_merah_keriting', 'Cabai Merah Keriting', 'Kg', 'Hortikultura', 48000, TRUE, 7),
  ('cabai_rawit_merah', 'Cabai Rawit Merah', 'Kg', 'Hortikultura', 55000, TRUE, 7),
  ('daging_ayam', 'Daging Ayam', 'Kg', 'Protein', 35000, TRUE, 3),
  ('daging_sapi', 'Daging Sapi', 'Kg', 'Protein', 130000, TRUE, 3),
  ('gula_pasir', 'Gula Pasir', 'Kg', 'Pangan Pokok', 16000, TRUE, 365),
  ('ikan_kembung', 'Ikan Kembung', 'Kg', 'Protein', 35000, FALSE, 2),
  ('ikan_tongkol', 'Ikan Tongkol', 'Kg', 'Protein', 32000, FALSE, 2),
  ('jagung', 'Jagung', 'Kg', 'Pangan Pokok', 8000, FALSE, 30),
  ('kedelai', 'Kedelai', 'Kg', 'Pangan Pokok', 14000, FALSE, 180),
  ('kentang', 'Kentang', 'Kg', 'Hortikultura', 16000, FALSE, 75),
  ('minyak_goreng', 'Minyak Goreng', 'Liter', 'Pangan Pokok', 18000, TRUE, 365),
  ('telur_ayam', 'Telur Ayam', 'Kg', 'Protein', 28000, TRUE, 14),
  ('tepung_terigu', 'Tepung Terigu', 'Kg', 'Pangan Pokok', 12000, FALSE, 180),
  ('tomat', 'Tomat', 'Kg', 'Hortikultura', 12000, FALSE, 14)
ON CONFLICT (kode) DO NOTHING;

-- =====================================================================
-- Tabel: pasar (48 baris)
-- =====================================================================
INSERT INTO pasar (kode, nama, kabupaten_kode, lat, lng, radius_meter, aktif) VALUES
  ('PSR-1101-01', 'Pasar Sinabang', '1101', 2.48, 96.38, 400, TRUE),
  ('PSR-1102-01', 'Pasar Tradisional Singkil', '1102', 2.28, 97.79, 400, TRUE),
  ('PSR-1103-01', 'Pasar Tapaktuan', '1103', 3.26, 97.183, 400, TRUE),
  ('PSR-1104-01', 'Pasar Kutacane', '1104', 3.493, 97.8, 400, TRUE),
  ('PSR-1105-01', 'Pasar Tradisional Idi', '1105', 4.96, 97.77, 400, TRUE),
  ('PSR-1106-01', 'Pasar Inpres Takengon', '1106', 4.627, 96.846, 450, TRUE),
  ('PSR-1107-01', 'Pasar Bina Usaha Meulaboh', '1107', 4.145, 96.129, 450, TRUE),
  ('PSR-1108-01', 'Pasar Induk Lambaro', '1108', 5.5085, 95.364, 600, TRUE),
  ('PSR-1109-01', 'Pasar Kota Sigli', '1109', 5.386, 95.96, 450, TRUE),
  ('PSR-1110-01', 'Pasar Inpres Bireuen', '1110', 5.203, 96.703, 450, TRUE),
  ('PSR-1111-01', 'Pasar Tradisional Lhoksukon', '1111', 5.05, 97.32, 400, TRUE),
  ('PSR-1112-01', 'Pasar Tradisional Blangpidie', '1112', 3.74, 96.84, 400, TRUE),
  ('PSR-1113-01', 'Pasar Tradisional Blangkejeren', '1113', 3.98, 97.35, 400, TRUE),
  ('PSR-1114-01', 'Pasar Tradisional Karang Baru', '1114', 4.29, 98.05, 400, TRUE),
  ('PSR-1115-01', 'Pasar Tradisional Suka Makmue', '1115', 4.13, 96.47, 400, TRUE),
  ('PSR-1116-01', 'Pasar Tradisional Calang', '1116', 4.63, 95.58, 400, TRUE),
  ('PSR-1117-01', 'Pasar Tradisional Redelong', '1117', 4.73, 96.87, 400, TRUE),
  ('PSR-1118-01', 'Pasar Tradisional Meureudu', '1118', 5.25, 96.25, 400, TRUE),
  ('PSR-1171-01', 'Pasar Peunayong', '1171', 5.5603, 95.3237, 400, TRUE),
  ('PSR-1171-02', 'Pasar Al-Mahirah Lamdingin', '1171', 5.5765, 95.3352, 400, TRUE),
  ('PSR-1172-01', 'Pasar Tradisional Sabang', '1172', 5.89, 95.32, 400, TRUE),
  ('PSR-1173-01', 'Pasar Kota Langsa', '1173', 4.47, 97.968, 450, TRUE),
  ('PSR-1174-01', 'Pasar Inpres Lhokseumawe', '1174', 5.179, 97.146, 450, TRUE),
  ('PSR-1175-01', 'Pasar Tradisional Subulussalam', '1175', 2.64, 98, 400, TRUE),
  ('pasar_aceh', 'Pasar Aceh', '1171', 5.55, 95.32, 400, TRUE),
  ('pasar_bina_usaha_meulaboh', 'Pasar Bina Usaha Meulaboh', '1105', 4.14, 96.13, 400, TRUE),
  ('pasar_inpres_lhokseumawe', 'Pasar Inpres Lhokseumawe', '1173', 5.18, 97.14, 400, TRUE),
  ('pasar_lamnyong', 'Pasar Lamnyong', '1171', 5.54, 95.35, 400, TRUE),
  ('pasar_tradisional_bireuen', 'Pasar Tradisional Bireuen', '1111', 5.2, 96.7, 400, TRUE),
  ('pasar_tradisional_blangkejeren', 'Pasar Tradisional Blangkejeren', '1113', 3.97, 97.35, 400, TRUE),
  ('pasar_tradisional_blangpidie', 'Pasar Tradisional Blangpidie', '1112', 3.78, 96.8, 400, TRUE),
  ('pasar_tradisional_calang', 'Pasar Tradisional Calang', '1114', 4.82, 95.63, 400, TRUE),
  ('pasar_tradisional_idi', 'Pasar Tradisional Idi', '1103', 4.62, 97.95, 400, TRUE),
  ('pasar_tradisional_jantho', 'Pasar Tradisional Jantho', '1106', 5.37, 95.53, 400, TRUE),
  ('pasar_tradisional_karang_baru', 'Pasar Tradisional Karang Baru', '1116', 4.24, 97.96, 400, TRUE),
  ('pasar_tradisional_kutacane', 'Pasar Tradisional Kutacane', '1102', 3.5, 97.82, 400, TRUE),
  ('pasar_tradisional_langsa', 'Pasar Tradisional Langsa', '1174', 4.47, 97.97, 400, TRUE),
  ('pasar_tradisional_lhoksukon', 'Pasar Tradisional Lhoksukon', '1108', 5, 97.24, 400, TRUE),
  ('pasar_tradisional_meureudu', 'Pasar Tradisional Meureudu', '1118', 5.2, 96.25, 400, TRUE),
  ('pasar_tradisional_redelong', 'Pasar Tradisional Redelong', '1117', 4.73, 96.86, 400, TRUE),
  ('pasar_tradisional_sabang', 'Pasar Tradisional Sabang', '1172', 5.89, 95.32, 400, TRUE),
  ('pasar_tradisional_sigli', 'Pasar Tradisional Sigli', '1107', 5.24, 95.96, 400, TRUE),
  ('pasar_tradisional_sinabang', 'Pasar Tradisional Sinabang', '1109', 2.62, 96.09, 400, TRUE),
  ('pasar_tradisional_singkil', 'Pasar Tradisional Singkil', '1110', 2.28, 97.8, 400, TRUE),
  ('pasar_tradisional_subulussalam', 'Pasar Tradisional Subulussalam', '1175', 2.64, 98, 400, TRUE),
  ('pasar_tradisional_suka_makmue', 'Pasar Tradisional Suka Makmue', '1115', 4.17, 96.52, 400, TRUE),
  ('pasar_tradisional_takengon', 'Pasar Tradisional Takengon', '1104', 4.52, 96.85, 400, TRUE),
  ('pasar_tradisional_tapaktuan', 'Pasar Tradisional Tapaktuan', '1101', 3.26, 97.35, 400, TRUE)
ON CONFLICT (kode) DO NOTHING;

