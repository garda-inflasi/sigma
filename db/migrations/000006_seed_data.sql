-- +goose Up
-- +goose StatementBegin

-- ── Seed: Provinsi Aceh ────────────────────────────────────────────────
INSERT INTO provinsi (kode, nama) VALUES ('11', 'ACEH')
ON CONFLICT (kode) DO NOTHING;

-- ── Seed: Kabupaten/Kota se-Aceh (23 Kab/Kota) ────────────────────────
INSERT INTO kabupaten (kode, nama, jenis, ibukota, lat, lng, provinsi_kode) VALUES
    ('1101', 'KABUPATEN ACEH SELATAN', 'Kabupaten', 'Tapaktuan', 3.26, 97.35, '11'),
    ('1102', 'KABUPATEN ACEH TENGGARA', 'Kabupaten', 'Kutacane', 3.50, 97.82, '11'),
    ('1103', 'KABUPATEN ACEH TIMUR', 'Kabupaten', 'Idi Rayeuk', 4.62, 97.95, '11'),
    ('1104', 'KABUPATEN ACEH TENGAH', 'Kabupaten', 'Takengon', 4.52, 96.85, '11'),
    ('1105', 'KABUPATEN ACEH BARAT', 'Kabupaten', 'Meulaboh', 4.14, 96.13, '11'),
    ('1106', 'KABUPATEN ACEH BESAR', 'Kabupaten', 'Jantho', 5.37, 95.53, '11'),
    ('1107', 'KABUPATEN PIDIE', 'Kabupaten', 'Sigli', 5.24, 95.96, '11'),
    ('1108', 'KABUPATEN ACEH UTARA', 'Kabupaten', 'Lhoksukon', 5.00, 97.24, '11'),
    ('1109', 'KABUPATEN SIMEULUE', 'Kabupaten', 'Sinabang', 2.62, 96.09, '11'),
    ('1110', 'KABUPATEN ACEH SINGKIL', 'Kabupaten', 'Singkil', 2.28, 97.80, '11'),
    ('1111', 'KABUPATEN BIREUEN', 'Kabupaten', 'Bireuen', 5.20, 96.70, '11'),
    ('1112', 'KABUPATEN ACEH BARAT DAYA', 'Kabupaten', 'Blangpidie', 3.78, 96.80, '11'),
    ('1113', 'KABUPATEN GAYO LUES', 'Kabupaten', 'Blangkejeren', 3.97, 97.35, '11'),
    ('1114', 'KABUPATEN ACEH JAYA', 'Kabupaten', 'Calang', 4.82, 95.63, '11'),
    ('1115', 'KABUPATEN NAGAN RAYA', 'Kabupaten', 'Suka Makmue', 4.17, 96.52, '11'),
    ('1116', 'KABUPATEN ACEH TAMIANG', 'Kabupaten', 'Karang Baru', 4.24, 97.96, '11'),
    ('1117', 'KABUPATEN BENER MERIAH', 'Kabupaten', 'Simpang Tiga Redelong', 4.73, 96.86, '11'),
    ('1118', 'KABUPATEN PIDIE JAYA', 'Kabupaten', 'Meureudu', 5.20, 96.25, '11'),
    ('1171', 'KOTA BANDA ACEH', 'Kota', 'Banda Aceh', 5.55, 95.32, '11'),
    ('1172', 'KOTA SABANG', 'Kota', 'Sabang', 5.89, 95.32, '11'),
    ('1173', 'KOTA LHOKSEUMAWE', 'Kota', 'Lhokseumawe', 5.18, 97.14, '11'),
    ('1174', 'KOTA LANGSA', 'Kota', 'Langsa', 4.47, 97.97, '11'),
    ('1175', 'KOTA SUBULUSSALAM', 'Kota', 'Subulussalam', 2.64, 98.00, '11')
ON CONFLICT (kode) DO NOTHING;

-- ── Seed: Pasar-pasar utama di Aceh ───────────────────────────────────
INSERT INTO pasar (id, nama, kabupaten_kode, lat, lng) VALUES
    ('pasar_aceh', 'Pasar Aceh', '1171', 5.55, 95.32),
    ('pasar_lamnyong', 'Pasar Lamnyong', '1171', 5.54, 95.35),
    ('pasar_bina_usaha_meulaboh', 'Pasar Bina Usaha Meulaboh', '1105', 4.14, 96.13),
    ('pasar_tradisional_sigli', 'Pasar Tradisional Sigli', '1107', 5.24, 95.96),
    ('pasar_inpres_lhokseumawe', 'Pasar Inpres Lhokseumawe', '1173', 5.18, 97.14),
    ('pasar_tradisional_takengon', 'Pasar Tradisional Takengon', '1104', 4.52, 96.85),
    ('pasar_tradisional_kutacane', 'Pasar Tradisional Kutacane', '1102', 3.50, 97.82),
    ('pasar_tradisional_idi', 'Pasar Tradisional Idi', '1103', 4.62, 97.95),
    ('pasar_tradisional_tapaktuan', 'Pasar Tradisional Tapaktuan', '1101', 3.26, 97.35),
    ('pasar_tradisional_bireuen', 'Pasar Tradisional Bireuen', '1111', 5.20, 96.70),
    ('pasar_tradisional_langsa', 'Pasar Tradisional Langsa', '1174', 4.47, 97.97),
    ('pasar_tradisional_subulussalam', 'Pasar Tradisional Subulussalam', '1175', 2.64, 98.00),
    ('pasar_tradisional_sabang', 'Pasar Tradisional Sabang', '1172', 5.89, 95.32),
    ('pasar_tradisional_meureudu', 'Pasar Tradisional Meureudu', '1118', 5.20, 96.25),
    ('pasar_tradisional_calang', 'Pasar Tradisional Calang', '1114', 4.82, 95.63),
    ('pasar_tradisional_blangpidie', 'Pasar Tradisional Blangpidie', '1112', 3.78, 96.80),
    ('pasar_tradisional_suka_makmue', 'Pasar Tradisional Suka Makmue', '1115', 4.17, 96.52),
    ('pasar_tradisional_karang_baru', 'Pasar Tradisional Karang Baru', '1116', 4.24, 97.96),
    ('pasar_tradisional_singkil', 'Pasar Tradisional Singkil', '1110', 2.28, 97.80),
    ('pasar_tradisional_sinabang', 'Pasar Tradisional Sinabang', '1109', 2.62, 96.09),
    ('pasar_tradisional_blangkejeren', 'Pasar Tradisional Blangkejeren', '1113', 3.97, 97.35),
    ('pasar_tradisional_redelong', 'Pasar Tradisional Redelong', '1117', 4.73, 96.86),
    ('pasar_tradisional_jantho', 'Pasar Tradisional Jantho', '1106', 5.37, 95.53),
    ('pasar_tradisional_lhoksukon', 'Pasar Tradisional Lhoksukon', '1108', 5.00, 97.24)
ON CONFLICT (id) DO NOTHING;

-- ── Seed: Komoditas pangan utama ──────────────────────────────────────
INSERT INTO komoditas (kode, nama, satuan, kategori, harga_acuan, pilot, shelf_life_hari) VALUES
    ('beras_premium', 'Beras Premium', 'Kg', 'Pangan Pokok', 15900, true, 180),
    ('beras_medium', 'Beras Medium', 'Kg', 'Pangan Pokok', 13900, true, 180),
    ('cabai_merah_keriting', 'Cabai Merah Keriting', 'Kg', 'Hortikultura', 48000, true, 7),
    ('cabai_rawit_merah', 'Cabai Rawit Merah', 'Kg', 'Hortikultura', 55000, true, 7),
    ('bawang_merah', 'Bawang Merah', 'Kg', 'Hortikultura', 35000, true, 14),
    ('bawang_putih', 'Bawang Putih', 'Kg', 'Hortikultura', 32000, true, 30),
    ('minyak_goreng', 'Minyak Goreng', 'Liter', 'Pangan Pokok', 18000, true, 365),
    ('gula_pasir', 'Gula Pasir', 'Kg', 'Pangan Pokok', 16000, true, 365),
    ('tepung_terigu', 'Tepung Terigu', 'Kg', 'Pangan Pokok', 12000, false, 180),
    ('telur_ayam', 'Telur Ayam', 'Kg', 'Protein', 28000, true, 14),
    ('daging_ayam', 'Daging Ayam', 'Kg', 'Protein', 35000, true, 3),
    ('daging_sapi', 'Daging Sapi', 'Kg', 'Protein', 130000, true, 3),
    ('ikan_kembung', 'Ikan Kembung', 'Kg', 'Protein', 35000, false, 2),
    ('ikan_tongkol', 'Ikan Tongkol', 'Kg', 'Protein', 32000, false, 2),
    ('jagung', 'Jagung', 'Kg', 'Pangan Pokok', 8000, false, 30),
    ('kedelai', 'Kedelai', 'Kg', 'Pangan Pokok', 14000, false, 180)
ON CONFLICT (kode) DO NOTHING;

-- ── Seed: Pengguna (Users) dengan berbagai peran ──────────────────────
-- Password untuk semua user: Garda2024!
-- Hash bcrypt dari 'Garda2024!'
DO $$
DECLARE
    pwd_hash VARCHAR(255) := '$2b$12$oMNxZPfOYtSKzOgpaw8cIOXK.XC3zprOgW7uDEpHNzPtSFbB9I.I2';
BEGIN

-- 1. Superadmin
INSERT INTO pengguna (id, nama, email, password_hash, peran, lapis, lembaga, status) VALUES
    ('superadmin', 'Super Admin', 'superadmin@garda-inflasi.id', pwd_hash, 'ADMIN_SISTEM', 'SISTEM', 'BPPSDMP', 'AKTIF')
ON CONFLICT (id) DO NOTHING;

-- 2. Strategis (Eksekutif)
INSERT INTO pengguna (id, nama, email, password_hash, peran, lapis, lembaga, status) VALUES
    ('strategis_fmipa', 'Dr. Teuku Ahmad, M.Si', 'strategis.fmipa@garda-inflasi.id', pwd_hash, 'STRATEGIS_FMIPA', 'STRATEGIS', 'FMIPA USK', 'AKTIF'),
    ('strategis_bi', 'Dr. Muhammad Rizal', 'strategis.bi@garda-inflasi.id', pwd_hash, 'STRATEGIS_BI', 'STRATEGIS', 'Bank Indonesia', 'AKTIF'),
    ('strategis_bps', 'Ir. Cut Putri, M.Ec.Dev', 'strategis.bps@garda-inflasi.id', pwd_hash, 'STRATEGIS_BPS', 'STRATEGIS', 'BPS Aceh', 'AKTIF'),
    ('strategis_djpb', 'Drs. Syahrul, M.M.', 'strategis.djpb@garda-inflasi.id', pwd_hash, 'STRATEGIS_DJPB', 'STRATEGIS', 'DJPb Aceh', 'AKTIF')
ON CONFLICT (id) DO NOTHING;

-- 3. Manajer
INSERT INTO pengguna (id, nama, email, password_hash, peran, lapis, lembaga, status) VALUES
    ('manajer_program', 'Rina Safitri, S.T., M.T.', 'manajer.program@garda-inflasi.id', pwd_hash, 'MANAJER_PROGRAM', 'MANAJERIAL', 'BPPSDMP', 'AKTIF'),
    ('manajer_data', 'Bambang Wijaya, S.Kom., M.Cs.', 'manajer.data@garda-inflasi.id', pwd_hash, 'MANAJER_DATA', 'MANAJERIAL', 'BPPSDMP', 'AKTIF'),
    ('manajer_kebijakan', 'Dr. Nurul Aini, S.E., M.E.', 'manajer.kebijakan@garda-inflasi.id', pwd_hash, 'MANAJER_KEBIJAKAN', 'MANAJERIAL', 'BPPSDMP', 'AKTIF'),
    ('manajer_pembiayaan', 'Fauzan, S.E., M.Si.', 'manajer.pembiayaan@garda-inflasi.id', pwd_hash, 'MANAJER_PEMBIAYAAN', 'MANAJERIAL', 'BPPSDMP', 'AKTIF'),
    ('manajer_risiko', 'Dian Permata, S.Si., M.M.', 'manajer.risiko@garda-inflasi.id', pwd_hash, 'MANAJER_RISIKO', 'MANAJERIAL', 'BPPSDMP', 'AKTIF'),
    ('manajer_mutu', 'Hendra Gunawan, S.T., M.T.', 'manajer.mutu@garda-inflasi.id', pwd_hash, 'MANAJER_MUTU', 'MANAJERIAL', 'BPPSDMP', 'AKTIF')
ON CONFLICT (id) DO NOTHING;

-- 4. Operasional (Lapangan)
INSERT INTO pengguna (id, nama, email, password_hash, peran, lapis, lembaga, kabupaten_kode, status) VALUES
    ('koordinator_lapangan', 'Zulkifli, S.P.', 'koordinator.lapangan@garda-inflasi.id', pwd_hash, 'OPS_KOORDINATOR', 'OPERASIONAL', 'BPPSDMP', '1171', 'AKTIF'),
    ('agregator_data', 'Maya Sari, S.Stat.', 'agregator.data@garda-inflasi.id', pwd_hash, 'OPS_AGREGATOR', 'OPERASIONAL', 'BPPSDMP', '1171', 'AKTIF'),
    ('mahasiswa_1', 'Aulia Rahman', 'mahasiswa.1@garda-inflasi.id', pwd_hash, 'OPS_MAHASISWA', 'OPERASIONAL', 'FMIPA USK', '1171', 'AKTIF'),
    ('mahasiswa_2', 'Cut Nanda', 'mahasiswa.2@garda-inflasi.id', pwd_hash, 'OPS_MAHASISWA', 'OPERASIONAL', 'FMIPA USK', '1106', 'AKTIF'),
    ('mahasiswa_3', 'Teuku Irfan', 'mahasiswa.3@garda-inflasi.id', pwd_hash, 'OPS_MAHASISWA', 'OPERASIONAL', 'FMIPA USK', '1105', 'AKTIF'),
    ('mahasiswa_4', 'Siti Rahmah', 'mahasiswa.4@garda-inflasi.id', pwd_hash, 'OPS_MAHASISWA', 'OPERASIONAL', 'FMIPA USK', '1107', 'AKTIF'),
    ('mahasiswa_5', 'Muhammad Rizki', 'mahasiswa.5@garda-inflasi.id', pwd_hash, 'OPS_MAHASISWA', 'OPERASIONAL', 'FMIPA USK', '1104', 'AKTIF'),
    ('mahasiswa_6', 'Nurul Fajri', 'mahasiswa.6@garda-inflasi.id', pwd_hash, 'OPS_MAHASISWA', 'OPERASIONAL', 'FMIPA USK', '1103', 'AKTIF'),
    ('mahasiswa_7', 'Rahmat Hidayat', 'mahasiswa.7@garda-inflasi.id', pwd_hash, 'OPS_MAHASISWA', 'OPERASIONAL', 'FMIPA USK', '1102', 'AKTIF'),
    ('mahasiswa_8', 'Dewi Sartika', 'mahasiswa.8@garda-inflasi.id', pwd_hash, 'OPS_MAHASISWA', 'OPERASIONAL', 'FMIPA USK', '1101', 'AKTIF'),
    ('mahasiswa_9', 'Rizky Pratama', 'mahasiswa.9@garda-inflasi.id', pwd_hash, 'OPS_MAHASISWA', 'OPERASIONAL', 'FMIPA USK', '1173', 'AKTIF'),
    ('mahasiswa_10', 'Fitriani', 'mahasiswa.10@garda-inflasi.id', pwd_hash, 'OPS_MAHASISWA', 'OPERASIONAL', 'FMIPA USK', '1174', 'AKTIF')
ON CONFLICT (id) DO NOTHING;

-- 5. User nonaktif untuk testing
INSERT INTO pengguna (id, nama, email, password_hash, peran, lapis, lembaga, status) VALUES
    ('user_nonaktif', 'User Nonaktif', 'nonaktif@garda-inflasi.id', pwd_hash, 'OPS_MAHASISWA', 'OPERASIONAL', 'FMIPA USK', 'NONAKTIF')
ON CONFLICT (id) DO NOTHING;

-- 6. User dengan MFA aktif untuk testing
INSERT INTO pengguna (id, nama, email, password_hash, peran, lapis, lembaga, status, mfa_aktif, mfa_secret, mfa_backup_codes) VALUES
    ('user_mfa', 'User MFA', 'mfa@garda-inflasi.id', pwd_hash, 'OPS_KOORDINATOR', 'OPERASIONAL', 'BPPSDMP', 'AKTIF', true, 'JBSWY3DPEHPK3PXP', '["BACKUP01","BACKUP02","BACKUP03"]'::jsonb)
ON CONFLICT (id) DO NOTHING;

END $$;

-- ── Seed: Role-Permission assignments untuk setiap peran ───────────────
-- Hapus role_permissions yang sudah ada untuk menghindari duplikasi
DELETE FROM role_permissions;

-- Superadmin: semua permissions
INSERT INTO role_permissions (role_id, permission_id)
SELECT r.id, p.id
FROM roles r, permissions p
WHERE r.slug = 'superadmin'
ON CONFLICT DO NOTHING;

-- Strategis FMIPA: dashboard eksekutif, prediksi, laporan
INSERT INTO role_permissions (role_id, permission_id)
SELECT r.id, p.id
FROM roles r, permissions p
WHERE r.slug = 'strategis_fmipa'
  AND p.name IN (
    'auth:logout', 'auth:self',
    'dashboard:read',
    'prediksi:read',
    'laporan:read', 'laporan:generate',
    'harga:read',
    'ews:read',
    'intervensi:read',
    'coldstorage:read'
  )
ON CONFLICT DO NOTHING;

-- Strategis BI: dashboard eksekutif, prediksi, intervensi
INSERT INTO role_permissions (role_id, permission_id)
SELECT r.id, p.id
FROM roles r, permissions p
WHERE r.slug = 'strategis_bi'
  AND p.name IN (
    'auth:logout', 'auth:self',
    'dashboard:read',
    'prediksi:read',
    'intervensi:read',
    'harga:read',
    'ews:read',
    'laporan:read'
  )
ON CONFLICT DO NOTHING;

-- Strategis BPS: dashboard eksekutif, validasi data
INSERT INTO role_permissions (role_id, permission_id)
SELECT r.id, p.id
FROM roles r, permissions p
WHERE r.slug = 'strategis_bps'
  AND p.name IN (
    'auth:logout', 'auth:self',
    'dashboard:read',
    'harga:read', 'harga:validate',
    'prediksi:read',
    'laporan:read'
  )
ON CONFLICT DO NOTHING;

-- Strategis DJPb: dashboard eksekutif, intervensi, laporan keuangan
INSERT INTO role_permissions (role_id, permission_id)
SELECT r.id, p.id
FROM roles r, permissions p
WHERE r.slug = 'strategis_djpb'
  AND p.name IN (
    'auth:logout', 'auth:self',
    'dashboard:read',
    'intervensi:read', 'intervensi:write',
    'laporan:read', 'laporan:generate'
  )
ON CONFLICT DO NOTHING;

-- Manajer Program: dashboard eksekutif + operasional, intervensi
INSERT INTO role_permissions (role_id, permission_id)
SELECT r.id, p.id
FROM roles r, permissions p
WHERE r.slug = 'manajer_program'
  AND p.name IN (
    'auth:logout', 'auth:self',
    'dashboard:read',
    'prediksi:read',
    'intervensi:read', 'intervensi:write',
    'harga:read',
    'ews:read', 'ews:acknowledge',
    'coldstorage:read',
    'laporan:read', 'laporan:generate'
  )
ON CONFLICT DO NOTHING;

-- Manajer Data: akses data & validasi
INSERT INTO role_permissions (role_id, permission_id)
SELECT r.id, p.id
FROM roles r, permissions p
WHERE r.slug = 'manajer_data'
  AND p.name IN (
    'auth:logout', 'auth:self',
    'dashboard:read',
    'harga:read', 'harga:write', 'harga:validate', 'harga:publish',
    'prediksi:read',
    'ews:read', 'ews:acknowledge',
    'sumber:read', 'sumber:write',
    'audit:read'
  )
ON CONFLICT DO NOTHING;

-- Manajer Kebijakan: dashboard, prediksi, laporan
INSERT INTO role_permissions (role_id, permission_id)
SELECT r.id, p.id
FROM roles r, permissions p
WHERE r.slug = 'manajer_kebijakan'
  AND p.name IN (
    'auth:logout', 'auth:self',
    'dashboard:read',
    'prediksi:read',
    'harga:read',
    'ews:read',
    'intervensi:read',
    'laporan:read', 'laporan:generate'
  )
ON CONFLICT DO NOTHING;

-- Manajer Pembiayaan: intervensi, laporan keuangan
INSERT INTO role_permissions (role_id, permission_id)
SELECT r.id, p.id
FROM roles r, permissions p
WHERE r.slug = 'manajer_pembiayaan'
  AND p.name IN (
    'auth:logout', 'auth:self',
    'intervensi:read', 'intervensi:write',
    'dashboard:read',
    'laporan:read', 'laporan:generate'
  )
ON CONFLICT DO NOTHING;

-- Manajer Risiko: EWS, prediksi, dashboard
INSERT INTO role_permissions (role_id, permission_id)
SELECT r.id, p.id
FROM roles r, permissions p
WHERE r.slug = 'manajer_risiko'
  AND p.name IN (
    'auth:logout', 'auth:self',
    'ews:read', 'ews:acknowledge',
    'prediksi:read',
    'dashboard:read',
    'harga:read',
    'coldstorage:read'
  )
ON CONFLICT DO NOTHING;

-- Manajer Mutu: validasi data, audit
INSERT INTO role_permissions (role_id, permission_id)
SELECT r.id, p.id
FROM roles r, permissions p
WHERE r.slug = 'manajer_mutu'
  AND p.name IN (
    'auth:logout', 'auth:self',
    'harga:read', 'harga:validate',
    'audit:read',
    'dashboard:read',
    'sumber:read'
  )
ON CONFLICT DO NOTHING;

-- Ops Koordinator: operasional, validasi lapangan
INSERT INTO role_permissions (role_id, permission_id)
SELECT r.id, p.id
FROM roles r, permissions p
WHERE r.slug = 'ops_koordinator'
  AND p.name IN (
    'auth:logout', 'auth:self',
    'harga:read', 'harga:write', 'harga:validate',
    'ews:read', 'ews:acknowledge',
    'dashboard:read',
    'coldstorage:read', 'coldstorage:write',
    'sumber:read'
  )
ON CONFLICT DO NOTHING;

-- Ops Agregator: input & lihat data
INSERT INTO role_permissions (role_id, permission_id)
SELECT r.id, p.id
FROM roles r, permissions p
WHERE r.slug = 'ops_agregator'
  AND p.name IN (
    'auth:logout', 'auth:self',
    'harga:read', 'harga:write',
    'ews:read',
    'dashboard:read',
    'coldstorage:read'
  )
ON CONFLICT DO NOTHING;

-- Ops Mahasiswa: input data lapangan
INSERT INTO role_permissions (role_id, permission_id)
SELECT r.id, p.id
FROM roles r, permissions p
WHERE r.slug = 'ops_mahasiswa'
  AND p.name IN (
    'auth:logout', 'auth:self',
    'harga:read', 'harga:write',
    'ews:read'
  )
ON CONFLICT DO NOTHING;

-- Admin Sistem: manajemen pengguna, role, audit
INSERT INTO role_permissions (role_id, permission_id)
SELECT r.id, p.id
FROM roles r, permissions p
WHERE r.slug = 'admin_sistem'
  AND p.name IN (
    'auth:logout', 'auth:self',
    'user:read', 'user:write', 'user:delete',
    'role:read', 'role:write', 'role:delete',
    'permission:read', 'permission:write', 'permission:delete',
    'audit:read'
  )
ON CONFLICT DO NOTHING;

-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
-- Hapus data seed (kebalikan dari Up)
DELETE FROM pengguna WHERE id IN (
    'superadmin',
    'strategis_fmipa', 'strategis_bi', 'strategis_bps', 'strategis_djpb',
    'manajer_program', 'manajer_data', 'manajer_kebijakan', 'manajer_pembiayaan', 'manajer_risiko', 'manajer_mutu',
    'koordinator_lapangan', 'agregator_data',
    'mahasiswa_1', 'mahasiswa_2', 'mahasiswa_3', 'mahasiswa_4', 'mahasiswa_5',
    'mahasiswa_6', 'mahasiswa_7', 'mahasiswa_8', 'mahasiswa_9', 'mahasiswa_10',
    'user_nonaktif', 'user_mfa'
);
DELETE FROM pasar;
DELETE FROM komoditas;
DELETE FROM kabupaten;
DELETE FROM provinsi;
-- +goose StatementEnd
