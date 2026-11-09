-- GARDA INFLASI - Tahapan & Alur Persetujuan Intervensi

-- =====================================================================
-- Tabel: tahap_intervensi_def (16 baris)
-- =====================================================================
INSERT INTO tahap_intervensi_def (siklus, kode_tahap, urutan, label, petunjuk, izin_dibutuhkan, sla_hari) VALUES
  ('NORMAL', 'n-pantau', 1, 'Pantau rutin data produsen', 'Agregasi harian dari agen lapangan', 'intervensi.kelola', 1),
  ('NORMAL', 'n-verifikasi', 2, 'Verifikasi silang BPS', 'Cocokkan dengan rilis statistik resmi', 'intervensi.kelola', 3),
  ('NORMAL', 'n-jadwal', 3, 'Optimasi jadwal pengiriman', 'Susun rute & slot armada', 'intervensi.kelola', 3),
  ('NORMAL', 'n-agregasi', 4, 'Agregasi volume reguler', 'Konsolidasi pasokan produsen', 'intervensi.kelola', 5),
  ('NORMAL', 'n-selesai', 5, 'Selesai', 'Ditutup dan terdokumentasi', 'intervensi.kelola', NULL),
  ('TEKANAN', 't-ews', 1, 'EWS terpicu', 'Alarm otomatis dari model', 'intervensi.kelola', 1),
  ('TEKANAN', 't-verifikasi', 2, 'Verifikasi cepat BI/BPS', 'Konfirmasi sinyal dengan data resmi', 'intervensi.kelola', 2),
  ('TEKANAN', 't-petakan', 3, 'Petakan suplai sentra surplus', 'Butuh persetujuan Dewan Pengarah', 'intervensi.setujui', 3),
  ('TEKANAN', 't-logistik', 4, 'Usulan intervensi logistik kilat', 'Rute & volume pengiriman darurat', 'intervensi.kelola', 3),
  ('TEKANAN', 't-fiskal', 5, 'Dukungan instrumen belanja', 'Persetujuan Kanwil DJPb', 'intervensi.setujui_fiskal', 5),
  ('TEKANAN', 't-selesai', 6, 'Selesai', 'Harga kembali ke koridor', 'intervensi.setujui', NULL),
  ('SURPLUS', 's-deteksi', 1, 'Deteksi over-supply', 'Harga produsen jatuh di bawah koridor', 'intervensi.kelola', 2),
  ('SURPLUS', 's-coldstorage', 2, 'Arahkan ke cold storage', 'Alokasi kapasitas & shelf-life', 'intervensi.kelola', 3),
  ('SURPLUS', 's-olahan', 3, 'Rekomendasi pengolahan turunan', 'Uji mutu oleh Unit Mutu & Pascapanen', 'intervensi.kelola', 7),
  ('SURPLUS', 's-ekspansi', 4, 'Ekspansi ke pembeli antardaerah', 'Pencocokan pembeli & logistik', 'intervensi.kelola', 7),
  ('SURPLUS', 's-selesai', 5, 'Selesai', 'Surplus terserap', 'intervensi.setujui', NULL)
ON CONFLICT (siklus, kode_tahap) DO NOTHING;

-- =====================================================================
-- Tabel: alur_persetujuan (5 baris)
-- =====================================================================
INSERT INTO alur_persetujuan (kode, nama, jenis, siklus, tahap_tujuan, aktif) VALUES
  ('eskalasi', 'Telaah eskalasi kartu', 'ESKALASI', NULL, NULL, TRUE),
  ('pindah-surplus-selesai', 'Penutupan intervensi surplus', 'PINDAH_TAHAP', 'SURPLUS', 's-selesai', TRUE),
  ('pindah-tekanan-fiskal', 'Dukungan instrumen belanja', 'PINDAH_TAHAP', 'TEKANAN', 't-fiskal', TRUE),
  ('pindah-tekanan-petakan', 'Petakan suplai sentra surplus', 'PINDAH_TAHAP', 'TEKANAN', 't-petakan', TRUE),
  ('pindah-tekanan-selesai', 'Penutupan intervensi tekanan harga', 'PINDAH_TAHAP', 'TEKANAN', 't-selesai', TRUE)
ON CONFLICT (kode) DO NOTHING;

-- =====================================================================
-- Tabel: alur_persetujuan_langkah (6 baris)
-- =====================================================================
INSERT INTO alur_persetujuan_langkah (alur_kode, urutan, label, izin_dibutuhkan) VALUES
  ('eskalasi', 1, 'Telaah Dewan Pengarah', 'intervensi.setujui'),
  ('pindah-surplus-selesai', 1, 'Persetujuan Dewan Pengarah', 'intervensi.setujui'),
  ('pindah-tekanan-fiskal', 1, 'Persetujuan Dewan Pengarah', 'intervensi.setujui'),
  ('pindah-tekanan-fiskal', 2, 'Persetujuan Kanwil DJPb', 'intervensi.setujui_fiskal'),
  ('pindah-tekanan-petakan', 1, 'Persetujuan Dewan Pengarah', 'intervensi.setujui'),
  ('pindah-tekanan-selesai', 1, 'Persetujuan Dewan Pengarah', 'intervensi.setujui')
ON CONFLICT (alur_kode, urutan) DO NOTHING;

