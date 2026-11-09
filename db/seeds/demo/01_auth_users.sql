-- GARDA INFLASI - 25 Akun Pengguna Demo (Password: Garda2024!)

-- =====================================================================
-- Tabel: pengguna (25 baris)
-- =====================================================================
INSERT INTO pengguna (id, nama, email, password_hash, peran, lapis, lembaga, kabupaten_kode, mfa_aktif, status, telepon, wajib_ganti_password, mfa_secret) OVERRIDING SYSTEM VALUE VALUES
  ('b8ab1221-4c1c-322b-ba15-55e2e8f17ec1', 'Maya Sari, S.Stat.', 'agregator.data@gardainflasi.id', '$2b$12$oMNxZPfOYtSKzOgpaw8cIOXK.XC3zprOgW7uDEpHNzPtSFbB9I.I2', 'OPS_AGREGATOR', 'OPERASIONAL', 'BPPSDMP', '1171', FALSE, 'AKTIF', '+6281200000013', FALSE, NULL),
  ('51dc3298-0976-3724-b5cc-45c2c6c20714', 'Zulkifli, S.P.', 'koordinator.lapangan@gardainflasi.id', '$2b$12$oMNxZPfOYtSKzOgpaw8cIOXK.XC3zprOgW7uDEpHNzPtSFbB9I.I2', 'OPS_KOORDINATOR', 'OPERASIONAL', 'BPPSDMP', '1171', FALSE, 'AKTIF', '+6281200000012', FALSE, NULL),
  ('84e84ff6-57a7-3f4f-af48-f4e2f5f31fd2', 'Fitriani', 'mahasiswa.10@gardainflasi.id', '$2b$12$oMNxZPfOYtSKzOgpaw8cIOXK.XC3zprOgW7uDEpHNzPtSFbB9I.I2', 'OPS_MAHASISWA', 'OPERASIONAL', 'FMIPA USK', '1173', FALSE, 'AKTIF', '+6281200000023', FALSE, NULL),
  ('03dc79aa-484b-322e-a0ae-3937fb4f3a23', 'Aulia Rahman', 'mahasiswa.1@gardainflasi.id', '$2b$12$oMNxZPfOYtSKzOgpaw8cIOXK.XC3zprOgW7uDEpHNzPtSFbB9I.I2', 'OPS_MAHASISWA', 'OPERASIONAL', 'FMIPA USK', '1171', FALSE, 'AKTIF', '+6281200000014', FALSE, NULL),
  ('e3461775-ad20-374a-931e-cf99dd9b8424', 'Cut Nanda', 'mahasiswa.2@gardainflasi.id', '$2b$12$oMNxZPfOYtSKzOgpaw8cIOXK.XC3zprOgW7uDEpHNzPtSFbB9I.I2', 'OPS_MAHASISWA', 'OPERASIONAL', 'FMIPA USK', '1108', FALSE, 'AKTIF', '+6281200000015', FALSE, NULL),
  ('cc4334e9-8d14-3d07-a045-8799202e0a28', 'Teuku Irfan', 'mahasiswa.3@gardainflasi.id', '$2b$12$oMNxZPfOYtSKzOgpaw8cIOXK.XC3zprOgW7uDEpHNzPtSFbB9I.I2', 'OPS_MAHASISWA', 'OPERASIONAL', 'FMIPA USK', '1107', FALSE, 'AKTIF', '+6281200000016', FALSE, NULL),
  ('5ef43834-8a06-3b81-a981-64addf62a5d1', 'Siti Rahmah', 'mahasiswa.4@gardainflasi.id', '$2b$12$oMNxZPfOYtSKzOgpaw8cIOXK.XC3zprOgW7uDEpHNzPtSFbB9I.I2', 'OPS_MAHASISWA', 'OPERASIONAL', 'FMIPA USK', '1109', FALSE, 'AKTIF', '+6281200000017', FALSE, NULL),
  ('5008d2a8-68fd-3c21-81bf-430a2b59c9be', 'Muhammad Rizki', 'mahasiswa.5@gardainflasi.id', '$2b$12$oMNxZPfOYtSKzOgpaw8cIOXK.XC3zprOgW7uDEpHNzPtSFbB9I.I2', 'OPS_MAHASISWA', 'OPERASIONAL', 'FMIPA USK', '1106', FALSE, 'AKTIF', '+6281200000018', FALSE, NULL),
  ('c754ff9b-1b8f-3955-b93e-231d5cd836ab', 'Nurul Fajri', 'mahasiswa.6@gardainflasi.id', '$2b$12$oMNxZPfOYtSKzOgpaw8cIOXK.XC3zprOgW7uDEpHNzPtSFbB9I.I2', 'OPS_MAHASISWA', 'OPERASIONAL', 'FMIPA USK', '1105', FALSE, 'AKTIF', '+6281200000019', FALSE, NULL),
  ('b12895e7-dfe7-3f10-8991-c8684703ae40', 'Rahmat Hidayat', 'mahasiswa.7@gardainflasi.id', '$2b$12$oMNxZPfOYtSKzOgpaw8cIOXK.XC3zprOgW7uDEpHNzPtSFbB9I.I2', 'OPS_MAHASISWA', 'OPERASIONAL', 'FMIPA USK', '1104', FALSE, 'AKTIF', '+6281200000020', FALSE, NULL),
  ('e9fe2b0c-4426-3eec-92fd-2e3b85842140', 'Dewi Sartika', 'mahasiswa.8@gardainflasi.id', '$2b$12$oMNxZPfOYtSKzOgpaw8cIOXK.XC3zprOgW7uDEpHNzPtSFbB9I.I2', 'OPS_MAHASISWA', 'OPERASIONAL', 'FMIPA USK', '1103', FALSE, 'AKTIF', '+6281200000021', FALSE, NULL),
  ('29a832f8-99f5-38a2-a350-5536c6be397d', 'Rizky Pratama', 'mahasiswa.9@gardainflasi.id', '$2b$12$oMNxZPfOYtSKzOgpaw8cIOXK.XC3zprOgW7uDEpHNzPtSFbB9I.I2', 'OPS_MAHASISWA', 'OPERASIONAL', 'FMIPA USK', '1174', FALSE, 'AKTIF', '+6281200000022', FALSE, NULL),
  ('e41b54c1-8d8f-39df-8efe-c661d1ec7d15', 'Bambang Wijaya, S.Kom., M.Cs.', 'manajer.data@gardainflasi.id', '$2b$12$oMNxZPfOYtSKzOgpaw8cIOXK.XC3zprOgW7uDEpHNzPtSFbB9I.I2', 'MANAJER_DATA', 'MANAJERIAL', 'BPPSDMP', NULL, FALSE, 'AKTIF', '+6281200000007', FALSE, NULL),
  ('4a5595b3-31f9-3231-97cc-91bb5aa27394', 'Dr. Nurul Aini, S.E., M.E.', 'manajer.kebijakan@gardainflasi.id', '$2b$12$oMNxZPfOYtSKzOgpaw8cIOXK.XC3zprOgW7uDEpHNzPtSFbB9I.I2', 'MANAJER_KEBIJAKAN', 'MANAJERIAL', 'BPPSDMP', NULL, FALSE, 'AKTIF', '+6281200000008', FALSE, NULL),
  ('8618501a-4585-3e4a-b466-31fcdd4abb83', 'Hendra Gunawan, S.T., M.T.', 'manajer.mutu@gardainflasi.id', '$2b$12$oMNxZPfOYtSKzOgpaw8cIOXK.XC3zprOgW7uDEpHNzPtSFbB9I.I2', 'MANAJER_MUTU', 'MANAJERIAL', 'BPPSDMP', NULL, FALSE, 'AKTIF', '+6281200000011', FALSE, NULL),
  ('6ed084bd-4b50-35a2-89bd-951a79b0fe15', 'Fauzan, S.E., M.Si.', 'manajer.pembiayaan@gardainflasi.id', '$2b$12$oMNxZPfOYtSKzOgpaw8cIOXK.XC3zprOgW7uDEpHNzPtSFbB9I.I2', 'MANAJER_PEMBIAYAAN', 'MANAJERIAL', 'BPPSDMP', NULL, FALSE, 'AKTIF', '+6281200000009', FALSE, NULL),
  ('e810ffa6-9117-330a-aee6-f281fa031dfe', 'Rina Safitri, S.T., M.T.', 'manajer.program@gardainflasi.id', '$2b$12$oMNxZPfOYtSKzOgpaw8cIOXK.XC3zprOgW7uDEpHNzPtSFbB9I.I2', 'MANAJER_PROGRAM', 'MANAJERIAL', 'BPPSDMP', NULL, FALSE, 'AKTIF', '+6281200000006', FALSE, NULL),
  ('32bfeac0-23e5-3218-b0ba-26b03d9675f0', 'Dian Permata, S.Si., M.M.', 'manajer.risiko@gardainflasi.id', '$2b$12$oMNxZPfOYtSKzOgpaw8cIOXK.XC3zprOgW7uDEpHNzPtSFbB9I.I2', 'MANAJER_RISIKO', 'MANAJERIAL', 'BPPSDMP', NULL, FALSE, 'AKTIF', '+6281200000010', FALSE, NULL),
  ('852d0433-a80e-3d20-85d7-53c3bdaeaeb7', 'User MFA', 'mfa@gardainflasi.id', '$2b$12$oMNxZPfOYtSKzOgpaw8cIOXK.XC3zprOgW7uDEpHNzPtSFbB9I.I2', 'OPS_KOORDINATOR', 'OPERASIONAL', 'BPPSDMP', NULL, TRUE, 'AKTIF', '+6281200000025', FALSE, 'v1:mZDYbRSeZTSQEdL3kFc2NHHcZFurC2JsTWtOATwqi2L87b1rutVlfs7vK/U=' ),
  ('8d61b017-6856-3774-9df2-02d104a5a771', 'User Nonaktif', 'nonaktif@gardainflasi.id', '$2b$12$oMNxZPfOYtSKzOgpaw8cIOXK.XC3zprOgW7uDEpHNzPtSFbB9I.I2', 'OPS_MAHASISWA', 'OPERASIONAL', 'FMIPA USK', NULL, FALSE, 'NONAKTIF', '+6281200000024', FALSE, NULL),
  ('96815d37-8076-3986-aae4-c350a1bb14cd', 'Dr. Muhammad Rizal', 'strategis.bi@gardainflasi.id', '$2b$12$oMNxZPfOYtSKzOgpaw8cIOXK.XC3zprOgW7uDEpHNzPtSFbB9I.I2', 'STRATEGIS_BI', 'STRATEGIS', 'Bank Indonesia', NULL, FALSE, 'AKTIF', '+6281200000003', FALSE, NULL),
  ('59faa110-1226-3a31-a737-10346bb0024e', 'Ir. Cut Putri, M.Ec.Dev', 'strategis.bps@gardainflasi.id', '$2b$12$oMNxZPfOYtSKzOgpaw8cIOXK.XC3zprOgW7uDEpHNzPtSFbB9I.I2', 'STRATEGIS_BPS', 'STRATEGIS', 'BPS Aceh', NULL, FALSE, 'AKTIF', '+6281200000004', FALSE, NULL),
  ('9e93356e-ae55-3f97-8aab-8dd459c17f0d', 'Drs. Syahrul, M.M.', 'strategis.djpb@gardainflasi.id', '$2b$12$oMNxZPfOYtSKzOgpaw8cIOXK.XC3zprOgW7uDEpHNzPtSFbB9I.I2', 'STRATEGIS_DJPB', 'STRATEGIS', 'DJPb Aceh', NULL, FALSE, 'AKTIF', '+6281200000005', FALSE, NULL),
  ('71603c01-d81a-3c0d-951b-0c9f9fd8e499', 'Prof. Dr.rer.nat. Ilham Maulana, S.Si.', 'strategis.fmipa@gardainflasi.id', '$2b$12$oMNxZPfOYtSKzOgpaw8cIOXK.XC3zprOgW7uDEpHNzPtSFbB9I.I2', 'STRATEGIS_FMIPA', 'STRATEGIS', 'FMIPA USK', NULL, FALSE, 'AKTIF', '+6281200000002', FALSE, NULL),
  ('f2356c7b-bb80-3d01-84c3-ba32f27a2910', 'Super Admin', 'superadmin@gardainflasi.id', '$2b$12$oMNxZPfOYtSKzOgpaw8cIOXK.XC3zprOgW7uDEpHNzPtSFbB9I.I2', 'ADMIN_SISTEM', 'SISTEM', 'BPPSDMP', NULL, FALSE, 'AKTIF', '+6281200000001', FALSE, NULL)
ON CONFLICT (email) DO UPDATE SET mfa_aktif = EXCLUDED.mfa_aktif, mfa_secret = EXCLUDED.mfa_secret;

-- =====================================================================
-- Tabel: mfa_kode_cadangan (3 baris)
-- =====================================================================
INSERT INTO mfa_kode_cadangan (user_id, kode_hash) VALUES
  ('852d0433-a80e-3d20-85d7-53c3bdaeaeb7', '48b9b65769307390ab276f0dc8d6836a201f2d160c50f08744778406a9d3172b'),
  ('852d0433-a80e-3d20-85d7-53c3bdaeaeb7', '5486a700ee2266f1ded18ef69faea125b8d71109b2875cdc555658ae4448b104'),
  ('852d0433-a80e-3d20-85d7-53c3bdaeaeb7', '0efd399f3e510a3563f2ed501c2798052968523cca89690abe707395d3be302e')
ON CONFLICT (user_id, kode_hash) DO NOTHING;

