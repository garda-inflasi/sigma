-- GARDA INFLASI - Tugas Pencacah Lapangan Demo
--
-- Lima tugas untuk tiap akun lapangan demo (koordinator, agregator, dan sepuluh mahasiswa) di pasar utama kabupatennya,
-- meniru contoh di mode mock: satu berjalan, dua belum mulai, satu terlambat, dan satu selesai. Tenggat dihitung dari
-- now() saat seed (negatif = sudah lewat), bukan tanggal tetap, jadi status tugas dan urutan tenggat selalu sama dengan
-- keadaan awal kapan pun seed dijalankan. Status yang ditampilkan API dihitung ulang terhadap waktu sekarang
-- (StatusEfektif), jadi tanggal tetap membuat tugas "BERJALAN" menjadi "TERLAMBAT" begitu tenggatnya lewat.
--
-- Prasyarat: reference/01_master.sql (pasar), demo/01_auth_users.sql (akun). Pasar utama kabupaten ditulis eksplisit
-- (pasar pertama menurut kode di kabupaten akun), bukan dicari dengan ORDER BY, supaya tidak bergantung pada collation.
-- Pembuat tugas adalah akun manajer_program.
--
-- Idempoten: kunci kode (TGS-<slug akun>-<urutan templat>). Dijalankan ulang, baris yang sudah ada dikembalikan ke
-- keadaan awal relatif terhadap now() (tenggat disegarkan, progres dan status direset). id bigint diberi basis data
-- menurut urutan UUID akun lalu urutan templat, sama dengan seeder Go yang digantikan.

INSERT INTO tugas_lapangan (kode, judul, pasar_id, petugas_id, komoditas, tipe, tenggat, status, target, dibuat_pada,
                            dibuat_oleh_id, jumlah_selesai, selesai_pada)
SELECT 'TGS-' || a.slug || '-' || t.urutan,
       t.judul,
       a.pasar_kode,
       garda_uuid_stabil('pengguna', a.slug),
       t.komoditas,
       t.tipe::tipe_harga,
       w.tenggat,
       t.status::status_tugas,
       t.target,
       w.tenggat - interval '8 hours',
       garda_uuid_stabil('pengguna', 'manajer_program'),
       t.jumlah_selesai,
       CASE WHEN t.status = 'SELESAI' THEN w.tenggat - interval '1 hour' END
  FROM (VALUES
         ('koordinator_lapangan', 'PSR-1171-01'),
         ('agregator_data',       'PSR-1171-01'),
         ('mahasiswa_1',          'PSR-1171-01'),
         ('mahasiswa_2',          'PSR-1108-01'),
         ('mahasiswa_3',          'PSR-1107-01'),
         ('mahasiswa_4',          'PSR-1109-01'),
         ('mahasiswa_5',          'PSR-1106-01'),
         ('mahasiswa_6',          'PSR-1105-01'),
         ('mahasiswa_7',          'PSR-1104-01'),
         ('mahasiswa_8',          'PSR-1103-01'),
         ('mahasiswa_9',          'PSR-1174-01'),
         ('mahasiswa_10',         'PSR-1173-01')
       ) AS a(slug, pasar_kode)
 CROSS JOIN (VALUES
         (1, 'Survei harga konsumen pagi',    ARRAY['CBM-KRT', 'CBR-RWT', 'BWM', 'BRS-MED', 'TLR', 'DAG-AYM']::text[], 'KONSUMEN',   2, 6, 4, 'BERJALAN'),
         (2, 'Verifikasi ulang harga cabai',  ARRAY['CBM-KRT', 'CBR-RWT']::text[],                                     'KONSUMEN',   5, 2, 0, 'BELUM'),
         (3, 'Harga produsen kelompok tani',  ARRAY['CBM-KRT', 'BWM', 'TMT', 'KTG']::text[],                           'PRODUSEN',  26, 4, 0, 'BELUM'),
         (4, 'Survei harga konsumen sore',    ARRAY['IKN-TGK', 'TLR', 'MYK', 'GLA']::text[],                           'KONSUMEN',  -3, 4, 1, 'TERLAMBAT'),
         (5, 'Survei harga konsumen pagi',    ARRAY['CBM-KRT', 'BWM', 'BRS-MED', 'TLR']::text[],                       'KONSUMEN', -22, 4, 4, 'SELESAI')
       ) AS t(urutan, judul, komoditas, tipe, tenggat_jam, target, jumlah_selesai, status)
 CROSS JOIN LATERAL (SELECT date_trunc('second', now()) + t.tenggat_jam * interval '1 hour' AS tenggat) AS w
 ORDER BY garda_uuid_stabil('pengguna', a.slug), t.urutan
ON CONFLICT (kode) DO UPDATE SET
    judul          = EXCLUDED.judul,
    pasar_id       = EXCLUDED.pasar_id,
    petugas_id     = EXCLUDED.petugas_id,
    komoditas      = EXCLUDED.komoditas,
    tipe           = EXCLUDED.tipe,
    tenggat        = EXCLUDED.tenggat,
    status         = EXCLUDED.status,
    target         = EXCLUDED.target,
    dibuat_pada    = EXCLUDED.dibuat_pada,
    dibuat_oleh_id = EXCLUDED.dibuat_oleh_id,
    jumlah_selesai = EXCLUDED.jumlah_selesai,
    selesai_pada   = EXCLUDED.selesai_pada;
