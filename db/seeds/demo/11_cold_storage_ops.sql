-- GARDA INFLASI - Operasional Cold Storage (Lot, Telemetri, Transaksi) Demo
--
-- Semua tanggal dihitung dari now() saat berkas dijalankan (lot berumur N hari sejak masuk, telemetri 48 jam
-- terakhir), jadi data selalu segar kapan pun dieksekusi. Menjalankan ulang menyegarkan tanggal tanpa
-- menggandakan baris:
--   * lot dan transaksi di-upsert menurut kode (STK-CS01-01, TRX-DEMO-STK-CS01-01). ID BIGINT diberi identity
--     basis data menurut urutan baris di bawah (STK-CS01-01 = lot pertama), tidak pernah ditulis di sini, sehingga
--     sekuens ikut maju dan lot baru dari aplikasi tidak bentrok; lot hasil migrasi 000113 mempertahankan ID-nya;
--   * telemetri di-upsert menurut (fasilitas_id, waktu) pada grid 30 menit, dan telemetri DEMO yang jatuh di luar
--     jendela baru dibuang sehingga jumlah barisnya tetap.
-- Fasilitas dirujuk menurut kode (CS-01); fasilitas yang tidak ada menggagalkan berkas (NOT NULL). Runner seed
-- mengirim satu berkas sebagai satu pesan multi-statement (satu transaksi implisit), jadi kegagalan di tengah
-- tidak meninggalkan data sebagian, dan now() sama di seluruh pernyataan.
--
-- Prasyarat: reference/01_master.sql (komoditas) dan reference/09_cold_storage.sql (fasilitas).

-- =====================================================================
-- Tabel: stok_cold_storage (23 baris) dan transaksi_stok (23 baris, satu MASUK per lot)
-- =====================================================================
-- Rancangan data (diuji tes integrasi cold-storage):
--   * volume per fasilitas = utilisasi di kartu (347/400, 240/300, 193/250, 33/150 ton), CS-05 tanpa stok;
--   * lot berisiko tinggi (risiko > 0,6) berjumlah 4 lot, 172,7 ton;
--   * 23 lot, cukup untuk tiga halaman dengan 10 baris per halaman;
--   * sisa masa simpan (masa_simpan_hari - umur_hari) paling sedikit 5 hari.
WITH seed (urut, kode, fasilitas, komoditas_kode, volume_ton, umur_hari, masa_simpan_hari, risiko_food_loss, pemilik) AS (VALUES
  -- CS-01 Lambaro: 347,0 ton
  ( 1, 'STK-CS01-01', 'CS-01', 'CBM-KRT', 34.9,  16,  21, 0.68, 'KUD Pidie Jaya'),
  ( 2, 'STK-CS01-02', 'CS-01', 'CBR-RWT', 29.4,   6,  18, 0.38, 'CV Lambaro Segar'),
  ( 3, 'STK-CS01-03', 'CS-01', 'CBM-KRT', 27.8,   9,  21, 0.47, 'Koperasi Tani Gayo Makmur'),
  ( 4, 'STK-CS01-04', 'CS-01', 'CBM-KRT', 38.2,   5,  21, 0.26, 'Kelompok Tani Bener Meriah'),
  ( 5, 'STK-CS01-05', 'CS-01', 'CBR-RWT', 18.6,   3,  18, 0.21, 'KUD Pidie Jaya'),
  ( 6, 'STK-CS01-06', 'CS-01', 'CBM-KRT', 36.0,   3,  21, 0.22, 'Kelompok Tani Bener Meriah'),
  ( 7, 'STK-CS01-07', 'CS-01', 'BWM',     52.7,  38,  60, 0.55, 'Koperasi Tani Gayo Makmur'),
  ( 8, 'STK-CS01-08', 'CS-01', 'BWM',     61.4,  21,  60, 0.31, 'CV Lambaro Segar'),
  ( 9, 'STK-CS01-09', 'CS-01', 'BWM',     48.0,  10,  60, 0.17, 'KUD Pidie Jaya'),
  -- CS-02 Lhokseumawe: 240,0 ton (beku, -20 C)
  (10, 'STK-CS02-01', 'CS-02', 'IKN-TGK', 64.5, 101, 120, 0.72, 'Koperasi Nelayan Samudera Pasai'),
  (11, 'STK-CS02-02', 'CS-02', 'IKN-TGK', 42.3, 106, 120, 0.81, 'CV Mina Lhokseumawe'),
  (12, 'STK-CS02-03', 'CS-02', 'IKN-TGK', 52.8,  70, 120, 0.38, 'Koperasi Nelayan Samudera Pasai'),
  (13, 'STK-CS02-04', 'CS-02', 'IKN-TGK', 47.3,  44, 120, 0.24, 'CV Mina Lhokseumawe'),
  (14, 'STK-CS02-05', 'CS-02', 'IKN-TGK', 33.1,  12, 120, 0.09, 'Koperasi Nelayan Samudera Pasai'),
  -- CS-03 Takengon: 193,0 ton
  (15, 'STK-CS03-01', 'CS-03', 'TMT',     31.0,   8,  14, 0.63, 'Kelompok Tani Bener Meriah'),
  (16, 'STK-CS03-02', 'CS-03', 'TMT',     15.4,   8,  14, 0.57, 'CV Lambaro Segar'),
  (17, 'STK-CS03-03', 'CS-03', 'TMT',     33.1,   7,  14, 0.44, 'KUD Pidie Jaya'),
  (18, 'STK-CS03-04', 'CS-03', 'CBM-KRT', 14.4,  11,  21, 0.54, 'KUD Pidie Jaya'),
  (19, 'STK-CS03-05', 'CS-03', 'KTG',     41.3,  20,  75, 0.18, 'Koperasi Tani Gayo Makmur'),
  (20, 'STK-CS03-06', 'CS-03', 'KTG',     36.5,  31,  75, 0.33, 'Kelompok Tani Bener Meriah'),
  (21, 'STK-CS03-07', 'CS-03', 'KTG',     21.3,  12,  75, 0.15, 'KUD Pidie Jaya'),
  -- CS-04 Meulaboh: 33,0 ton (dalam perawatan)
  (22, 'STK-CS04-01', 'CS-04', 'TLR',     12.9,  12,  30, 0.41, 'KUD Aceh Barat Sejahtera'),
  (23, 'STK-CS04-02', 'CS-04', 'BWM',     20.1,  27,  60, 0.41, 'KUD Aceh Barat Sejahtera')
),
lot AS (
  INSERT INTO stok_cold_storage (kode, cold_storage_id, komoditas_kode, volume_ton, masuk_pada, kedaluwarsa_pada, pemilik, risiko_food_loss, status)
  SELECT s.kode,
         (SELECT c.id FROM cold_storage c WHERE c.kode = s.fasilitas),
         s.komoditas_kode,
         s.volume_ton,
         now() - s.umur_hari * INTERVAL '1 day',
         now() - s.umur_hari * INTERVAL '1 day' + s.masa_simpan_hari * INTERVAL '1 day',
         s.pemilik,
         s.risiko_food_loss,
         'AKTIF'
    FROM seed s
   ORDER BY s.urut
  ON CONFLICT (kode) DO UPDATE SET
    cold_storage_id = EXCLUDED.cold_storage_id,
    komoditas_kode = EXCLUDED.komoditas_kode,
    volume_ton = EXCLUDED.volume_ton,
    masuk_pada = EXCLUDED.masuk_pada,
    kedaluwarsa_pada = EXCLUDED.kedaluwarsa_pada,
    pemilik = EXCLUDED.pemilik,
    risiko_food_loss = EXCLUDED.risiko_food_loss,
    status = EXCLUDED.status
  RETURNING id, kode, cold_storage_id, volume_ton, masuk_pada
)
-- Buku besar konsisten dengan volume awal lot: satu transaksi MASUK pada saat lot masuk.
INSERT INTO transaksi_stok (kode, lot_id, jenis, volume_ton, fasilitas_tujuan_id, waktu, alasan, sumber)
SELECT 'TRX-DEMO-' || l.kode, l.id, 'MASUK', l.volume_ton, l.cold_storage_id, l.masuk_pada, 'Penerimaan awal (data demo)', 'DEMO'
  FROM lot l
 ORDER BY l.id
ON CONFLICT (kode) DO UPDATE SET
  lot_id = EXCLUDED.lot_id,
  jenis = EXCLUDED.jenis,
  volume_ton = EXCLUDED.volume_ton,
  fasilitas_tujuan_id = EXCLUDED.fasilitas_tujuan_id,
  waktu = EXCLUDED.waktu,
  alasan = EXCLUDED.alasan,
  sumber = EXCLUDED.sumber;

-- =====================================================================
-- Tabel: telemetri_suhu (384 baris = 96 titik x 4 fasilitas beroperasi)
-- =====================================================================
-- 96 pembacaan per 30 menit menutup 48 jam terakhir; grid menempel pada kelipatan 30 menit epoch (:00 dan :30 UTC)
-- sehingga menjalankan ulang tidak menggandakan baris. CS-05 masih rencana: tanpa telemetri.
-- Suhu = target + 0,35 sin(jam dalam sehari) + derau +-0,25, dibulatkan 2 desimal; kelembapan = 82 + 4 sin(jam + 1)
-- + derau +-2 (%). Derau deterministik (md5 dari kode fasilitas dan nomor titik), jadi sama di setiap jalan.
-- Pembacaan terbaru (k = 0) sama dengan suhu aktual fasilitas supaya kartu fasilitas konsisten dengan riwayatnya.
-- CS-04 punya lonjakan suhu 10-12 jam lalu (k = 20..25) yang kini sudah pulih.

-- Jendela bergeser mengikuti waktu eksekusi: baris DEMO yang tertinggal dibuang agar jumlahnya tetap.
DELETE FROM telemetri_suhu
 WHERE sumber = 'DEMO'
   AND waktu < to_timestamp(floor(extract(epoch FROM now()) / 1800) * 1800) - 95 * INTERVAL '30 minutes';

WITH fasilitas (kode, suhu_target_c, suhu_aktual_c) AS (VALUES
  ('CS-01',   4.0,   3.5),
  ('CS-02', -20.0, -20.1),
  ('CS-03',   6.0,   6.0),
  ('CS-04',   4.0,   4.8)
),
lonjakan (kode, k, tambah_c) AS (VALUES
  ('CS-04', 20, 1.6), ('CS-04', 21, 2.6), ('CS-04', 22, 3.1),
  ('CS-04', 23, 2.8), ('CS-04', 24, 1.9), ('CS-04', 25, 1.0)
),
titik AS (
  SELECT f.kode, f.suhu_target_c, f.suhu_aktual_c, g.k,
         to_timestamp(floor(extract(epoch FROM now()) / 1800) * 1800) - g.k * INTERVAL '30 minutes' AS waktu,
         COALESCE(l.tambah_c, 0) AS tambah_c,
         (('x' || left(md5(f.kode || ':' || g.k || ':suhu'), 7))::bit(28)::int)::float8 / 268435456 AS derau_suhu,
         (('x' || left(md5(f.kode || ':' || g.k || ':lembap'), 7))::bit(28)::int)::float8 / 268435456 AS derau_lembap
    FROM fasilitas f
   CROSS JOIN generate_series(0, 95) AS g(k)
    LEFT JOIN lonjakan l ON l.kode = f.kode AND l.k = g.k
),
fase AS (
  SELECT t.*,
         2 * pi() * (extract(hour FROM t.waktu AT TIME ZONE 'UTC') * 60 + extract(minute FROM t.waktu AT TIME ZONE 'UTC')) / 1440 AS sudut
    FROM titik t
)
INSERT INTO telemetri_suhu (fasilitas_id, waktu, suhu_c, kelembapan_pct, sumber)
SELECT (SELECT c.id FROM cold_storage c WHERE c.kode = p.kode),
       p.waktu,
       CASE WHEN p.k = 0
            THEN p.suhu_aktual_c
            ELSE round((p.suhu_target_c + 0.35 * sin(p.sudut) + (p.derau_suhu - 0.5) * 0.5 + p.tambah_c)::numeric, 2)::float8
       END,
       round(least(100, greatest(0, 82 + 4 * sin(p.sudut + 1) + (p.derau_lembap - 0.5) * 4))::numeric, 1)::float8,
       'DEMO'
  FROM fase p
ON CONFLICT (fasilitas_id, waktu) DO UPDATE SET
  suhu_c = EXCLUDED.suhu_c,
  kelembapan_pct = EXCLUDED.kelembapan_pct,
  sumber = EXCLUDED.sumber;

-- Suhu, kelembapan, dan waktu aktual fasilitas diturunkan dari pembacaan demo terbarunya (seperti ingest telemetri).
UPDATE cold_storage c
   SET suhu_aktual_c = t.suhu_c,
       kelembapan_aktual_pct = t.kelembapan_pct,
       telemetri_pada = t.waktu
  FROM (SELECT DISTINCT ON (fasilitas_id) fasilitas_id, waktu, suhu_c, kelembapan_pct
          FROM telemetri_suhu
         WHERE sumber = 'DEMO'
         ORDER BY fasilitas_id, waktu DESC) t
 WHERE c.id = t.fasilitas_id;
