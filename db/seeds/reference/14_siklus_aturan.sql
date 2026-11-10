-- GARDA INFLASI - Aturan Penentuan Siklus Ekonomi Otomatis (08-B)
--
-- Mesin penentuan siklus (internal/siklus/engine) membaca tabel ini setiap evaluasi dan menetapkan siklus aktif
-- provinsi (NORMAL, TEKANAN, SURPLUS). Ambang awal, bukan hasil kalibrasi: harus dikalibrasi bersama BI/BPS, sama
-- seperti aturan_ews (reference/07_ews_aturan.sql). Aman dijalankan di produksi.
--
-- * TEKANAN (prioritas 10): alert EWS belum SELESAI bertingkat SIAGA ke atas untuk lonjakan harga, kelangkaan stok,
--   atau anomali cuaca. Pemetaan jenis -> siklus sama dengan kartu intervensi otomatis (14-C,
--   intervensi/workflow.SiklusUntuk); tes integrasi siklus menjaga keduanya tetap sama.
-- * SURPLUS (prioritas 20): alert OVER_SUPPLY bertingkat SIAGA ke atas, atau utilisasi cold storage OPERASIONAL
--   >= 90% (fasilitas nyaris penuh = pasokan menumpuk). Rule engine EWS belum membuat alert OVER_SUPPLY, jadi
--   aturan utilisasi adalah satu-satunya jalur surplus dari data nyata saat ini.
-- * Prioritas: bila pemicu TEKANAN dan SURPLUS muncul bersamaan, TEKANAN yang menjadi siklus aktif. Alasannya:
--   (1) tekanan harga/pasokan langsung menggerus daya beli dan menuntut respons paling cepat (SLA tahap tekanan 1-5
--   hari), sedangkan surplus bersifat melindungi pendapatan petani dan dapat ditangani lewat penyerapan ke cold
--   storage; (2) salah mengabaikan tekanan lebih mahal daripada salah mengabaikan surplus; (3) sinyal campuran lazim
--   (cabai melonjak sementara kentang melimpah), dan satu warna ambient harus menandai kondisi paling mendesak
--   sementara pemicu surplus tetap tercatat di alasan siklus (alasan.ditekan).
-- * Masa pulih (histeresis): tekanan baru dilepas setelah 48 jam tanpa pemicu, lebih panjang dari peredaman alert
--   (aturan_ews.cooldown_jam = 24 jam) supaya kondisi yang masih berlangsung dan memicu alert baru sesudah peredaman
--   tidak menyebabkan siklus berkedip NORMAL <-> TEKANAN; surplus 72 jam karena tahap penanganannya lebih lambat
--   (SLA tahap surplus sampai 7 hari).
--
-- Idempoten menurut kode (ON CONFLICT DO NOTHING): aturan yang sudah ada TIDAK PERNAH ditimpa, jadi ambang, prioritas,
-- masa pulih, dan status aktif yang diubah operator bertahan di antara deploy. Untuk mematikan aturan, ubah aktif
-- menjadi false (jangan DELETE: aturan yang dihapus akan ditanam kembali oleh seed ini).

-- =====================================================================
-- Tabel: aturan_siklus (5 baris)
-- =====================================================================
INSERT INTO aturan_siklus (kode, siklus, indikator, jenis_ews, severity_min, jumlah_min, ambang, prioritas, pulih_jam, aktif, keterangan) VALUES
  ('SKL-TKN-LONJAKAN-HARGA', 'TEKANAN', 'ALERT_EWS', 'LONJAKAN_HARGA', 'SIAGA', 1, NULL, 10, 48, TRUE,
   'Satu alert lonjakan harga SIAGA ke atas yang belum selesai memicu siklus Tekanan. Ambang awal, harus dikalibrasi bersama BI/BPS.'),
  ('SKL-TKN-KELANGKAAN-STOK', 'TEKANAN', 'ALERT_EWS', 'KELANGKAAN_STOK', 'SIAGA', 1, NULL, 10, 48, TRUE,
   'Satu alert kelangkaan stok SIAGA ke atas yang belum selesai memicu siklus Tekanan. Ambang awal, harus dikalibrasi bersama BI/BPS.'),
  ('SKL-TKN-ANOMALI-CUACA', 'TEKANAN', 'ALERT_EWS', 'ANOMALI_CUACA', 'SIAGA', 1, NULL, 10, 48, TRUE,
   'Satu alert anomali cuaca SIAGA ke atas yang belum selesai memicu siklus Tekanan. Ambang awal, harus dikalibrasi bersama BI/BPS.'),
  ('SKL-SRP-OVER-SUPPLY', 'SURPLUS', 'ALERT_EWS', 'OVER_SUPPLY', 'SIAGA', 1, NULL, 20, 72, TRUE,
   'Satu alert over-supply SIAGA ke atas yang belum selesai memicu siklus Surplus. Ambang awal, harus dikalibrasi bersama BI/BPS.'),
  ('SKL-SRP-UTILISASI-STOK', 'SURPLUS', 'UTILISASI_STOK', NULL, NULL, NULL, 90, 20, 72, TRUE,
   'Utilisasi cold storage OPERASIONAL >= 90% (persen terisi dari kapasitas) memicu siklus Surplus. Ambang awal, harus dikalibrasi bersama BI/BPS.')
ON CONFLICT (kode) DO NOTHING;
