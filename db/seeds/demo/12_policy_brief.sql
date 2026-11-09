-- GARDA INFLASI - Policy Brief TPID Demo (5 brief contoh)
--
-- Padanan seeder Go lama internal/policy-brief/seeder. Bentuk dan isinya meniru mockBriefs di web supaya halaman
-- tampil sama dengan mode mock. Semua brief berlabel DEMO (konten.label = "Data demo — bukan brief resmi"); produksi
-- tidak punya brief contoh, jadi berkas ini hanya ada di lapis demo.
--
-- Prasyarat: reference/01_master.sql dan demo/01_auth_users.sql (penyusun = akun manajer_kebijakan).
--
-- Waktu relatif terhadap saat berkas dijalankan: diperbarui = now() - menit_lalu, dibuat_pada = diperbarui - 72 jam.
-- Idempoten menurut kode (PB-YYYY-MM, kunci unik): brief yang sudah ada dibiarkan apa adanya (status dan isi yang sudah
-- berubah tidak ditimpa); satu-satunya pengecualian, brief contoh yang kontennya masih kosong ('{}', dari sebelum kolom
-- konten ada) diberi konten dan penanda DEMO. ID brief = garda_uuid_stabil('policy_brief', kode), sama dengan hasil
-- migrasi 000114 dan idx.UUIDStabil di Go.

-- =====================================================================
-- Tabel: policy_brief (5 baris)
-- =====================================================================
INSERT INTO policy_brief (id, kode, periode, judul, status, ringkasan, rekomendasi, halaman, disusun_oleh_id, konten, snapshot_data, sumber_narasi, dibuat_pada, diperbarui)
SELECT
  garda_uuid_stabil('policy_brief', v.kode),
  v.kode,
  v.periode,
  v.judul,
  v.status::status_brief,
  v.ringkasan,
  v.rekomendasi::jsonb,
  v.halaman,
  garda_uuid_stabil('pengguna', 'manajer_kebijakan'),
  jsonb_build_object(
    'label', 'Data demo — bukan brief resmi',
    'penyusun', 'DEMO',
    'bagian', jsonb_build_array(
      jsonb_build_object('kode', 'ringkasan', 'judul', 'Ringkasan', 'butir', jsonb_build_array(v.ringkasan)),
      jsonb_build_object('kode', 'rekomendasi', 'judul', 'Rekomendasi', 'butir', v.rekomendasi::jsonb)
    ),
    'lampiran', '[]'::jsonb
  ),
  '{}'::jsonb,
  'DEMO',
  now() - make_interval(mins => v.menit_lalu) - interval '72 hours',
  now() - make_interval(mins => v.menit_lalu)
FROM (VALUES
  ('PB-2026-10', 'Oktober 2026', 'Meredam tekanan harga cabai pascacurah hujan ekstrem di dataran tinggi Gayo', 'DRAF', 8, 45,
   'Inflasi tahunan Aceh masih dalam koridor sasaran, namun curah hujan ekstrem di sentra Gayo menekan pasokan cabai merah dan mendorong harga naik.',
   '["Siapkan operasi pasar cabai di Pasar Peunayong dan Lamdingin", "Petakan pasokan alternatif dari Bireuen dan Sumatera Utara", "Ajukan dukungan subsidi ongkos angkut ke Kanwil DJPb", "Perkuat pemantauan harga harian di 11 pasar sentinel", "Koordinasikan jadwal tanam ulang dengan dinas pertanian dataran tinggi"]'),
  ('PB-2026-09', 'September 2026', 'Optimalisasi cold storage untuk serapan surplus kentang dan tomat', 'DISETUJUI', 7, 9800,
   'Surplus panen kentang dan tomat menekan harga di tingkat petani; kapasitas cold storage perlu diarahkan untuk menyerap kelebihan pasokan.',
   '["Alihkan surplus ke cold storage dengan kapasitas tersisa terbesar", "Hubungkan agregator dengan pembeli antardaerah", "Tetapkan harga serap minimum sementara untuk kentang", "Pantau risiko food loss pada lot berumur simpan pendek"]'),
  ('PB-2026-08', 'Agustus 2026', 'Stabilisasi harga beras menjelang panen gadu', 'DIDISTRIBUSIKAN', 6, 52000,
   'Stok beras memadai untuk beberapa bulan konsumsi; harga diperkirakan stabil hingga panen gadu mulai masuk pasar.',
   '["Jaga ketersediaan stok Bulog di pasar rawan gejolak", "Percepat penyerapan gabah petani saat panen raya", "Lanjutkan pemantauan harga mingguan"]'),
  ('PB-2026-07', 'Juli 2026', 'Pola distribusi ikan tongkol di wilayah pesisir timur', 'DIDISTRIBUSIKAN', 9, 95000,
   'Harga ikan tongkol di pesisir timur dipengaruhi cuaca laut dan rantai distribusi yang panjang menuju wilayah tengah.',
   '["Perpendek rantai distribusi dari pelabuhan ke pasar tengah", "Sediakan rantai dingin di titik pendaratan utama", "Sinkronkan jadwal pasokan dengan prakiraan cuaca laut", "Libatkan koperasi nelayan dalam pelaporan harga"]'),
  ('PB-2026-06', 'Juni 2026', 'Kesiapan pasokan pangan Iduladha di 23 kabupaten/kota', 'DIDISTRIBUSIKAN', 10, 138000,
   'Permintaan daging, cabai, dan bawang meningkat menjelang Iduladha; kesiapan pasokan tiap kabupaten/kota perlu dipantau.',
   '["Petakan kabupaten/kota dengan neraca pangan defisit", "Jadwalkan operasi pasar sebelum puncak permintaan", "Amankan pasokan daging sapi dan ayam antardaerah", "Aktifkan peringatan dini untuk komoditas pilot", "Siapkan kanal pengaduan harga bagi masyarakat", "Evaluasi dampak kebijakan segera setelah hari raya"]')
) AS v(kode, periode, judul, status, halaman, menit_lalu, ringkasan, rekomendasi)
ON CONFLICT (kode) DO UPDATE
  SET konten = EXCLUDED.konten, sumber_narasi = EXCLUDED.sumber_narasi
  WHERE policy_brief.konten = '{}'::jsonb;
