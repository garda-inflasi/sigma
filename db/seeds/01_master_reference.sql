-- =====================================================================
-- GARDA INFLASI - Master Reference Seed Data
-- Berkas: db/seeds/01_master_reference.sql
-- Keterangan: Data referensi master statis (satuan, provinsi, kabupaten/kota,
--             komoditas pangan, dan pasar percontohan).
-- Idempoten: aman dieksekusi berulang kali via psql.
-- =====================================================================

-- 1. Satuan Komoditas Baku
INSERT INTO satuan (kode, nama) VALUES
    ('kg', 'Kilogram'),
    ('liter', 'Liter')
ON CONFLICT (kode) DO NOTHING;

-- 2. Provinsi Cakupan
INSERT INTO provinsi (kode, nama) VALUES
    ('11', 'ACEH')
ON CONFLICT (kode) DO NOTHING;

-- 3. 23 Kabupaten / Kota di Provinsi Aceh
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
    ('1175', 'KOTA SUBULUSSALAM', 'Kota', 'Subulussalam', 2.64, 98.0, '11')
ON CONFLICT (kode) DO NOTHING;

-- 4. Komoditas Pangan Utama
INSERT INTO komoditas (kode, nama, satuan, kategori, harga_acuan, pilot, shelf_life_hari) VALUES
    ('BRS-MED', 'Beras medium', 'kg', 'Pangan pokok', 13900, true, 180),
    ('BRS-PRM', 'Beras premium', 'kg', 'Pangan pokok', 15800, false, 180),
    ('CBM-KRT', 'Cabai merah keriting', 'kg', 'Hortikultura', 52000, true, 21),
    ('CBR-RWT', 'Cabai rawit merah', 'kg', 'Hortikultura', 58000, false, 18),
    ('BWM', 'Bawang merah', 'kg', 'Hortikultura', 41000, true, 60),
    ('BWP', 'Bawang putih', 'kg', 'Hortikultura', 38000, false, 120),
    ('TLR', 'Telur ayam ras', 'kg', 'Peternakan', 29500, false, 30),
    ('DAG-AYM', 'Daging ayam ras', 'kg', 'Peternakan', 36500, false, 90),
    ('DAG-SPI', 'Daging sapi', 'kg', 'Peternakan', 148000, false, 180),
    ('IKN-TGK', 'Ikan tongkol', 'kg', 'Perikanan', 38000, false, 120),
    ('MYK', 'Minyak goreng curah', 'liter', 'Pangan olahan', 17500, false, 365),
    ('GLA', 'Gula pasir', 'kg', 'Pangan olahan', 18000, false, 365),
    ('KTG', 'Kentang', 'kg', 'Hortikultura', 16000, false, 75),
    ('TMT', 'Tomat', 'kg', 'Hortikultura', 12000, false, 14)
ON CONFLICT (kode) DO NOTHING;

-- 5. Pasar Percontohan di Seluruh Kabupaten/Kota Aceh
INSERT INTO pasar (kode, nama, kabupaten_kode, lat, lng, radius_meter, aktif) VALUES
    ('PSR-1171-01', 'Pasar Peunayong', '1171', 5.5603, 95.3237, 400, true),
    ('PSR-1171-02', 'Pasar Al-Mahirah Lamdingin', '1171', 5.5765, 95.3352, 400, true),
    ('PSR-1108-01', 'Pasar Induk Lambaro', '1108', 5.5085, 95.364, 600, true),
    ('PSR-1174-01', 'Pasar Inpres Lhokseumawe', '1174', 5.179, 97.146, 450, true),
    ('PSR-1110-01', 'Pasar Inpres Bireuen', '1110', 5.203, 96.703, 450, true),
    ('PSR-1109-01', 'Pasar Kota Sigli', '1109', 5.386, 95.96, 450, true),
    ('PSR-1107-01', 'Pasar Bina Usaha Meulaboh', '1107', 4.145, 96.129, 450, true),
    ('PSR-1106-01', 'Pasar Inpres Takengon', '1106', 4.627, 96.846, 450, true),
    ('PSR-1173-01', 'Pasar Kota Langsa', '1173', 4.47, 97.968, 450, true),
    ('PSR-1103-01', 'Pasar Tapaktuan', '1103', 3.26, 97.183, 400, true),
    ('PSR-1104-01', 'Pasar Kutacane', '1104', 3.493, 97.8, 400, true),
    ('PSR-1101-01', 'Pasar Sinabang', '1101', 2.48, 96.38, 400, true),
    ('PSR-1102-01', 'Pasar Tradisional Singkil', '1102', 2.28, 97.79, 400, true),
    ('PSR-1105-01', 'Pasar Tradisional Idi', '1105', 4.96, 97.77, 400, true),
    ('PSR-1111-01', 'Pasar Tradisional Lhoksukon', '1111', 5.05, 97.32, 400, true),
    ('PSR-1112-01', 'Pasar Tradisional Blangpidie', '1112', 3.74, 96.84, 400, true),
    ('PSR-1113-01', 'Pasar Tradisional Blangkejeren', '1113', 3.98, 97.35, 400, true),
    ('PSR-1114-01', 'Pasar Tradisional Karang Baru', '1114', 4.29, 98.05, 400, true),
    ('PSR-1115-01', 'Pasar Tradisional Suka Makmue', '1115', 4.13, 96.47, 400, true),
    ('PSR-1116-01', 'Pasar Tradisional Calang', '1116', 4.63, 95.58, 400, true),
    ('PSR-1117-01', 'Pasar Tradisional Redelong', '1117', 4.73, 96.87, 400, true),
    ('PSR-1118-01', 'Pasar Tradisional Meureudu', '1118', 5.25, 96.25, 400, true),
    ('PSR-1172-01', 'Pasar Tradisional Sabang', '1172', 5.89, 95.32, 400, true),
    ('PSR-1175-01', 'Pasar Tradisional Subulussalam', '1175', 2.64, 98.0, 400, true)
ON CONFLICT (kode) DO NOTHING;
