-- GARDA INFLASI - Batas Harga Acuan Awal Komoditas
--
-- Satu batas harga awal (berlaku untuk semua kabupaten) per komoditas aktif, diturunkan dari harga acuan master dan
-- dibulatkan ke ratusan: batas_min = 25% harga acuan (paling sedikit 100) dan batas_maks = 300% harga acuan. Harga di
-- bawah seperempat atau di atas tiga kali harga acuan hampir pasti salah satuan atau salah ketik. Sama dengan
-- SeedReference seeder harga Go yang digantikan: dihitung dari tabel komoditas, bukan daftar tetap, sehingga komoditas
-- yang ditambahkan seed lain ikut tercakup. berlaku_dari tetap (2024-01-01) supaya seed deterministik.
--
-- Prasyarat: reference/01_master.sql (komoditas).
--
-- Idempoten: kunci kode ('BH-' + kode komoditas) dan kunci alami (komoditas, kabupaten, berlaku dari). ON CONFLICT DO
-- NOTHING tanpa target menutup keduanya, sehingga batas yang sudah ada tidak ditimpa (perubahan admin bertahan) dan
-- batas yang dibuat admin untuk komoditas yang sama dengan kode lain tidak ditabrak.

INSERT INTO batas_harga (kode, komoditas_kode, kabupaten_kode, batas_min, batas_maks, berlaku_dari)
SELECT 'BH-' || k.kode, k.kode, NULL,
       GREATEST(100, round(k.harga_acuan * CAST(0.25 AS numeric) / 100) * 100)::int,
       (round(k.harga_acuan * CAST(3.0 AS numeric) / 100) * 100)::int,
       DATE '2024-01-01'
  FROM komoditas k
 WHERE k.status = 'AKTIF'
 ORDER BY k.kode
ON CONFLICT DO NOTHING;
