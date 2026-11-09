-- GARDA INFLASI - Akun Superadmin Awal (Bootstrap Produksi)
-- Idempoten: hanya dibuat bila belum ada akun dengan email terkait.

INSERT INTO pengguna (id, nama, email, password_hash, peran, lapis, lembaga, status, mfa_aktif, mfa_wajib, wajib_ganti_password, telepon)
VALUES (
  garda_uuid_stabil('pengguna', 'admin-awal'),
  'Administrator Sistem',
  'superadmin@gardainflasi.id',
  '$2b$12$oMNxZPfOYtSKzOgpaw8cIOXK.XC3zprOgW7uDEpHNzPtSFbB9I.I2',
  'ADMIN_SISTEM',
  'SISTEM',
  'Garda Inflasi',
  'AKTIF',
  false,
  true,
  true,
  '+6281200000001'
)
ON CONFLICT (email) DO NOTHING;
