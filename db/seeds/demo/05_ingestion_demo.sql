-- GARDA INFLASI - Data Eksekusi Ingestion, Data Mentah Bronze, dan Harga Eksternal Demo
--
-- Setara dengan SeedDemo seeder Go ingestion yang digantikan (internal/ingestion/seeder/demo.go) plus keadaan eksekusi
-- sumber dari seeder sumber-data:
--   * keadaan eksekusi tiap sumber di katalog (status, terakhir_jalan, keberhasilan 7 hari, latensi);
--   * sembilan eksekusi contoh (pemicu DEMO): PIHPS harian 10.00 WIB, BMKG tiap 3 jam, BPS harian 06.00 WIB;
--   * satu payload bronze contoh (data_mentah) untuk tiap eksekusi yang punya dokumen (tujuh);
--   * harga eksternal PIHPS (SRC-BI) 14 hari untuk empat komoditas pilot di lima kota (280 baris), dengan lineage ke
--     eksekusi dan payload PIHPS terakhir;
--   * lineage eksekusi/payload BMKG pada cuaca harian demo 7 hari (demo/04_cuaca_harian.sql).
--
-- Waktu relatif terhadap now() saat berkas dijalankan, bukan tanggal tetap: eksekusi jatuh pada jam tetap WIB di hari-hari
-- sebelum hari ini, harga eksternal berakhir kemarin, dan keadaan sumber dihitung dari menit lalu. Nilai harga memakai
-- PRNG tetap seeder lama, jadi sama setiap kali dijalankan (harga bergantung pada jarak hari, bukan tanggal).
--
-- Prasyarat: reference/01_master.sql (komoditas, kabupaten), reference/04_sumber_data.sql (katalog sumber), dan
-- demo/04_cuaca_harian.sql (untuk lineage cuaca; tanpa itu langkah terakhir tidak mengubah apa pun).
--
-- Idempoten: eksekusi dikunci menurut (sumber, pemicu DEMO, mulai) sehingga dijalankan ulang pada hari yang sama tidak
-- menambah baris; hari berikutnya menambah satu set eksekusi contoh baru (riwayat bertambah, seperti seeder lama).
-- Payload bronze dikunci menurut (sumber, checksum). Harga eksternal diperbarui hanya bila barisnya berasal dari
-- eksekusi contoh; baris hasil ingestion nyata tidak pernah ditimpa. id bigint diberi basis data.

-- =====================================================================
-- Keadaan eksekusi sumber di katalog (relatif terhadap now())
-- =====================================================================
UPDATE sumber_data SET
  status = v.status::status_sumber,
  terakhir_jalan = now() - (v.menit_lalu || ' minutes')::interval,
  keberhasilan_7h = v.keberhasilan_7h,
  latensi_ms = v.latensi_ms
FROM (VALUES
  ('SRC-BPS', 'SEHAT', 142, 100.0, 640),
  ('SRC-BI', 'SEHAT', 38, 98.6, 910),
  ('SRC-DJPB', 'SEHAT', 2880, 100.0, 1220),
  ('SRC-BMKG', 'SEHAT', 51, 99.1, 480),
  ('SRC-OWM', 'SEHAT', 12, 99.8, 210),
  ('SRC-BAPANAS', 'TERTUNDA', 325, 86.4, 3400),
  ('SRC-PORTAL', 'GAGAL', 1460, 57.1, 0),
  ('SRC-CS', 'SEHAT', 3, 99.97, 95)
) AS v(kode, status, menit_lalu, keberhasilan_7h, latensi_ms)
WHERE sumber_data.kode = v.kode;

-- =====================================================================
-- Eksekusi contoh (pemicu DEMO): 9 baris
-- =====================================================================
INSERT INTO eksekusi_sumber (sumber_id, pemicu, status, mulai, selesai, durasi_ms, latensi_ms, dokumen, baris_dibaca,
                             baris_diambil, baris_tak_terpetakan, baris_tidak_valid, galat, ringkasan)
SELECT s.id, 'DEMO', j.status, w.mulai, w.mulai + j.durasi_ms * interval '1 millisecond', j.durasi_ms,
       NULLIF(j.latensi_ms, 0), j.dokumen, j.diambil + j.tak_terpetakan + j.tidak_valid, j.diambil, j.tak_terpetakan,
       j.tidak_valid, j.galat,
       jsonb_build_object('permintaan', jsonb_build_object('berhasil', j.dokumen, 'gagal', CASE WHEN j.galat IS NULL THEN 0 ELSE 1 END))
  FROM (VALUES
         (1, 'SRC-BI',   3, 10, 'SUKSES',   5, 640, 15, 30,  910,  9800, NULL::text),
         (2, 'SRC-BI',   2, 10, 'SUKSES',   5, 655, 15, 20,  880,  9300, NULL),
         (3, 'SRC-BI',   1, 10, 'SEBAGIAN', 4, 520, 12, 45, 1450, 21000, 'wilayah=104: GET https://www.bi.go.id/hargapangan/WebSite/TabelHarga/GetGridDataDaerah: status 503'),
         (4, 'SRC-BMKG', 1,  6, 'SUKSES',  23,  46,  0, 23,  480, 30500, NULL),
         (5, 'SRC-BMKG', 1,  9, 'SUKSES',  23,  46,  0, 23,  495, 30900, NULL),
         (6, 'SRC-BMKG', 1, 12, 'SEBAGIAN',22,  44,  0, 22,  520, 31200, 'wilayah=11.05.01.2001: GET https://api.bmkg.go.id/publik/prakiraan-cuaca: status 404'),
         (7, 'SRC-BMKG', 1, 15, 'SUKSES',  23,  46,  0, 23,  470, 30100, NULL),
         (8, 'SRC-BPS',  2,  6, 'GAGAL',    0,   0,  0,  0,    0,    40, 'konfigurasi.variabel SRC-BPS belum lengkap, isi id variabel BPS untuk: ihk, inflasi_mtm, inflasi_yoy, inflasi_ytd'),
         (9, 'SRC-BPS',  1,  6, 'GAGAL',    0,   0,  0,  0,    0,    38, 'konfigurasi.variabel SRC-BPS belum lengkap, isi id variabel BPS untuk: ihk, inflasi_mtm, inflasi_yoy, inflasi_ytd')
       ) AS j(urutan, kode, hari_lalu, jam, status, dokumen, diambil, tak_terpetakan, tidak_valid, latensi_ms, durasi_ms, galat)
  JOIN sumber_data s ON s.kode = j.kode
 CROSS JOIN LATERAL (SELECT (((now() AT TIME ZONE 'Asia/Jakarta')::date - j.hari_lalu) + make_time(j.jam, 0, 0)) AT TIME ZONE 'Asia/Jakarta' AS mulai) AS w
 WHERE NOT EXISTS (SELECT 1 FROM eksekusi_sumber e WHERE e.sumber_id = s.id AND e.pemicu = 'DEMO' AND e.mulai = w.mulai)
 ORDER BY j.urutan;

-- =====================================================================
-- Payload bronze contoh: satu per eksekusi contoh yang punya dokumen (7 baris)
-- Checksum adalah SHA-256 teks payload, sama dengan seeder lama.
-- =====================================================================
INSERT INTO data_mentah (sumber_id, eksekusi_id, kunci_permintaan, diambil_pada, payload, checksum)
SELECT p.sumber_id, p.eksekusi_id, 'demo', p.mulai, p.teks::jsonb, encode(sha256(convert_to(p.teks, 'UTF8')), 'hex')
  FROM (SELECT e.sumber_id, e.id AS eksekusi_id, e.mulai,
               '{"demo":true,"sumber":"' || s.kode || '","catatan":"payload contoh, bukan respons layanan nyata","diambil":"'
                 || to_char(e.mulai AT TIME ZONE 'UTC', 'YYYY-MM-DD"T"HH24:MI:SS"Z"') || '"}' AS teks
          FROM eksekusi_sumber e
          JOIN sumber_data s ON s.id = e.sumber_id
         WHERE e.pemicu = 'DEMO' AND e.dokumen > 0) AS p
 ORDER BY p.eksekusi_id
ON CONFLICT (sumber_id, checksum) DO NOTHING;

-- =====================================================================
-- Harga eksternal PIHPS contoh: 5 kota x 4 komoditas pilot x 14 hari yang berakhir kemarin (280 baris)
-- Harga = harga acuan x tren pelan x noise tetap (PRNG seeder lama), dibulatkan ke ratusan.
-- Kolom h = berapa hari sebelum hari ini (WIB); lineage ke eksekusi dan payload PIHPS contoh terakhir.
-- =====================================================================
INSERT INTO harga_eksternal (sumber_id, komoditas_kode, kabupaten_kode, tanggal, tipe, harga, satuan, data_mentah_id, eksekusi_id)
SELECT s.id, v.komoditas_kode, v.kabupaten_kode, (now() AT TIME ZONE 'Asia/Jakarta')::date - v.h, 'KONSUMEN', v.harga, k.satuan,
       lin.data_mentah_id, lin.eksekusi_id
  FROM (VALUES
  ('1171', 'BRS-MED', 14, 13900),
  ('1171', 'BRS-MED', 13, 14100),
  ('1171', 'BRS-MED', 12, 13900),
  ('1171', 'BRS-MED', 11, 13600),
  ('1171', 'BRS-MED', 10, 13700),
  ('1171', 'BRS-MED', 9, 13900),
  ('1171', 'BRS-MED', 8, 13800),
  ('1171', 'BRS-MED', 7, 13900),
  ('1171', 'BRS-MED', 6, 13800),
  ('1171', 'BRS-MED', 5, 14000),
  ('1171', 'BRS-MED', 4, 13800),
  ('1171', 'BRS-MED', 3, 14500),
  ('1171', 'BRS-MED', 2, 14500),
  ('1171', 'BRS-MED', 1, 14100),
  ('1171', 'CBM-KRT', 14, 51600),
  ('1171', 'CBM-KRT', 13, 52200),
  ('1171', 'CBM-KRT', 12, 51600),
  ('1171', 'CBM-KRT', 11, 52400),
  ('1171', 'CBM-KRT', 10, 51700),
  ('1171', 'CBM-KRT', 9, 51300),
  ('1171', 'CBM-KRT', 8, 53500),
  ('1171', 'CBM-KRT', 7, 54200),
  ('1171', 'CBM-KRT', 6, 53800),
  ('1171', 'CBM-KRT', 5, 54300),
  ('1171', 'CBM-KRT', 4, 54500),
  ('1171', 'CBM-KRT', 3, 53400),
  ('1171', 'CBM-KRT', 2, 52400),
  ('1171', 'CBM-KRT', 1, 52500),
  ('1171', 'BWM', 14, 40400),
  ('1171', 'BWM', 13, 40200),
  ('1171', 'BWM', 12, 40800),
  ('1171', 'BWM', 11, 40000),
  ('1171', 'BWM', 10, 42100),
  ('1171', 'BWM', 9, 41000),
  ('1171', 'BWM', 8, 42600),
  ('1171', 'BWM', 7, 42700),
  ('1171', 'BWM', 6, 40900),
  ('1171', 'BWM', 5, 41600),
  ('1171', 'BWM', 4, 43100),
  ('1171', 'BWM', 3, 40800),
  ('1171', 'BWM', 2, 41500),
  ('1171', 'BWM', 1, 41200),
  ('1171', 'TLR', 14, 30100),
  ('1171', 'TLR', 13, 29800),
  ('1171', 'TLR', 12, 30400),
  ('1171', 'TLR', 11, 30000),
  ('1171', 'TLR', 10, 30400),
  ('1171', 'TLR', 9, 30400),
  ('1171', 'TLR', 8, 29100),
  ('1171', 'TLR', 7, 30200),
  ('1171', 'TLR', 6, 30300),
  ('1171', 'TLR', 5, 30000),
  ('1171', 'TLR', 4, 30800),
  ('1171', 'TLR', 3, 29900),
  ('1171', 'TLR', 2, 30400),
  ('1171', 'TLR', 1, 29600),
  ('1108', 'BRS-MED', 14, 13800),
  ('1108', 'BRS-MED', 13, 14100),
  ('1108', 'BRS-MED', 12, 13600),
  ('1108', 'BRS-MED', 11, 14300),
  ('1108', 'BRS-MED', 10, 13700),
  ('1108', 'BRS-MED', 9, 14200),
  ('1108', 'BRS-MED', 8, 14200),
  ('1108', 'BRS-MED', 7, 14500),
  ('1108', 'BRS-MED', 6, 13800),
  ('1108', 'BRS-MED', 5, 14000),
  ('1108', 'BRS-MED', 4, 14500),
  ('1108', 'BRS-MED', 3, 14500),
  ('1108', 'BRS-MED', 2, 14500),
  ('1108', 'BRS-MED', 1, 14200),
  ('1108', 'CBM-KRT', 14, 53200),
  ('1108', 'CBM-KRT', 13, 53100),
  ('1108', 'CBM-KRT', 12, 51500),
  ('1108', 'CBM-KRT', 11, 51900),
  ('1108', 'CBM-KRT', 10, 53000),
  ('1108', 'CBM-KRT', 9, 53600),
  ('1108', 'CBM-KRT', 8, 52200),
  ('1108', 'CBM-KRT', 7, 53000),
  ('1108', 'CBM-KRT', 6, 53500),
  ('1108', 'CBM-KRT', 5, 52600),
  ('1108', 'CBM-KRT', 4, 52400),
  ('1108', 'CBM-KRT', 3, 53300),
  ('1108', 'CBM-KRT', 2, 52300),
  ('1108', 'CBM-KRT', 1, 51900),
  ('1108', 'BWM', 14, 40100),
  ('1108', 'BWM', 13, 40000),
  ('1108', 'BWM', 12, 40200),
  ('1108', 'BWM', 11, 42200),
  ('1108', 'BWM', 10, 40700),
  ('1108', 'BWM', 9, 40500),
  ('1108', 'BWM', 8, 40700),
  ('1108', 'BWM', 7, 42600),
  ('1108', 'BWM', 6, 42300),
  ('1108', 'BWM', 5, 40800),
  ('1108', 'BWM', 4, 41500),
  ('1108', 'BWM', 3, 41200),
  ('1108', 'BWM', 2, 41100),
  ('1108', 'BWM', 1, 41200),
  ('1108', 'TLR', 14, 29700),
  ('1108', 'TLR', 13, 29500),
  ('1108', 'TLR', 12, 30200),
  ('1108', 'TLR', 11, 30500),
  ('1108', 'TLR', 10, 29700),
  ('1108', 'TLR', 9, 29700),
  ('1108', 'TLR', 8, 30500),
  ('1108', 'TLR', 7, 29200),
  ('1108', 'TLR', 6, 29100),
  ('1108', 'TLR', 5, 30100),
  ('1108', 'TLR', 4, 30200),
  ('1108', 'TLR', 3, 29300),
  ('1108', 'TLR', 2, 31000),
  ('1108', 'TLR', 1, 29800),
  ('1174', 'BRS-MED', 14, 14000),
  ('1174', 'BRS-MED', 13, 14000),
  ('1174', 'BRS-MED', 12, 13800),
  ('1174', 'BRS-MED', 11, 14200),
  ('1174', 'BRS-MED', 10, 14100),
  ('1174', 'BRS-MED', 9, 13800),
  ('1174', 'BRS-MED', 8, 14300),
  ('1174', 'BRS-MED', 7, 14100),
  ('1174', 'BRS-MED', 6, 14500),
  ('1174', 'BRS-MED', 5, 14000),
  ('1174', 'BRS-MED', 4, 13900),
  ('1174', 'BRS-MED', 3, 14200),
  ('1174', 'BRS-MED', 2, 13800),
  ('1174', 'BRS-MED', 1, 14400),
  ('1174', 'CBM-KRT', 14, 53500),
  ('1174', 'CBM-KRT', 13, 52900),
  ('1174', 'CBM-KRT', 12, 52300),
  ('1174', 'CBM-KRT', 11, 51300),
  ('1174', 'CBM-KRT', 10, 53200),
  ('1174', 'CBM-KRT', 9, 51500),
  ('1174', 'CBM-KRT', 8, 53700),
  ('1174', 'CBM-KRT', 7, 51200),
  ('1174', 'CBM-KRT', 6, 52500),
  ('1174', 'CBM-KRT', 5, 53300),
  ('1174', 'CBM-KRT', 4, 54100),
  ('1174', 'CBM-KRT', 3, 52100),
  ('1174', 'CBM-KRT', 2, 53000),
  ('1174', 'CBM-KRT', 1, 53300),
  ('1174', 'BWM', 14, 40500),
  ('1174', 'BWM', 13, 41300),
  ('1174', 'BWM', 12, 40900),
  ('1174', 'BWM', 11, 41400),
  ('1174', 'BWM', 10, 41300),
  ('1174', 'BWM', 9, 40300),
  ('1174', 'BWM', 8, 42700),
  ('1174', 'BWM', 7, 42800),
  ('1174', 'BWM', 6, 41200),
  ('1174', 'BWM', 5, 42600),
  ('1174', 'BWM', 4, 42500),
  ('1174', 'BWM', 3, 40900),
  ('1174', 'BWM', 2, 42500),
  ('1174', 'BWM', 1, 42500),
  ('1174', 'TLR', 14, 29500),
  ('1174', 'TLR', 13, 29700),
  ('1174', 'TLR', 12, 30400),
  ('1174', 'TLR', 11, 28900),
  ('1174', 'TLR', 10, 30300),
  ('1174', 'TLR', 9, 29600),
  ('1174', 'TLR', 8, 30700),
  ('1174', 'TLR', 7, 30200),
  ('1174', 'TLR', 6, 29800),
  ('1174', 'TLR', 5, 29500),
  ('1174', 'TLR', 4, 29200),
  ('1174', 'TLR', 3, 30000),
  ('1174', 'TLR', 2, 30300),
  ('1174', 'TLR', 1, 29500),
  ('1110', 'BRS-MED', 14, 14100),
  ('1110', 'BRS-MED', 13, 14300),
  ('1110', 'BRS-MED', 12, 13900),
  ('1110', 'BRS-MED', 11, 13700),
  ('1110', 'BRS-MED', 10, 14000),
  ('1110', 'BRS-MED', 9, 13900),
  ('1110', 'BRS-MED', 8, 14000),
  ('1110', 'BRS-MED', 7, 14300),
  ('1110', 'BRS-MED', 6, 13700),
  ('1110', 'BRS-MED', 5, 14100),
  ('1110', 'BRS-MED', 4, 14300),
  ('1110', 'BRS-MED', 3, 14400),
  ('1110', 'BRS-MED', 2, 14400),
  ('1110', 'BRS-MED', 1, 14300),
  ('1110', 'CBM-KRT', 14, 51100),
  ('1110', 'CBM-KRT', 13, 51400),
  ('1110', 'CBM-KRT', 12, 52100),
  ('1110', 'CBM-KRT', 11, 53100),
  ('1110', 'CBM-KRT', 10, 52400),
  ('1110', 'CBM-KRT', 9, 52300),
  ('1110', 'CBM-KRT', 8, 51100),
  ('1110', 'CBM-KRT', 7, 53000),
  ('1110', 'CBM-KRT', 6, 51600),
  ('1110', 'CBM-KRT', 5, 53900),
  ('1110', 'CBM-KRT', 4, 53600),
  ('1110', 'CBM-KRT', 3, 52300),
  ('1110', 'CBM-KRT', 2, 51700),
  ('1110', 'CBM-KRT', 1, 53400),
  ('1110', 'BWM', 14, 40000),
  ('1110', 'BWM', 13, 39900),
  ('1110', 'BWM', 12, 40000),
  ('1110', 'BWM', 11, 40600),
  ('1110', 'BWM', 10, 40600),
  ('1110', 'BWM', 9, 42400),
  ('1110', 'BWM', 8, 40300),
  ('1110', 'BWM', 7, 41500),
  ('1110', 'BWM', 6, 40700),
  ('1110', 'BWM', 5, 41400),
  ('1110', 'BWM', 4, 42400),
  ('1110', 'BWM', 3, 42800),
  ('1110', 'BWM', 2, 42000),
  ('1110', 'BWM', 1, 40900),
  ('1110', 'TLR', 14, 29800),
  ('1110', 'TLR', 13, 29900),
  ('1110', 'TLR', 12, 28800),
  ('1110', 'TLR', 11, 30100),
  ('1110', 'TLR', 10, 29100),
  ('1110', 'TLR', 9, 29900),
  ('1110', 'TLR', 8, 30300),
  ('1110', 'TLR', 7, 29100),
  ('1110', 'TLR', 6, 29800),
  ('1110', 'TLR', 5, 29500),
  ('1110', 'TLR', 4, 30300),
  ('1110', 'TLR', 3, 30900),
  ('1110', 'TLR', 2, 31100),
  ('1110', 'TLR', 1, 29900),
  ('1109', 'BRS-MED', 14, 13900),
  ('1109', 'BRS-MED', 13, 13800),
  ('1109', 'BRS-MED', 12, 13700),
  ('1109', 'BRS-MED', 11, 14100),
  ('1109', 'BRS-MED', 10, 13600),
  ('1109', 'BRS-MED', 9, 14400),
  ('1109', 'BRS-MED', 8, 14400),
  ('1109', 'BRS-MED', 7, 13700),
  ('1109', 'BRS-MED', 6, 14500),
  ('1109', 'BRS-MED', 5, 14500),
  ('1109', 'BRS-MED', 4, 14400),
  ('1109', 'BRS-MED', 3, 13900),
  ('1109', 'BRS-MED', 2, 14200),
  ('1109', 'BRS-MED', 1, 14200),
  ('1109', 'CBM-KRT', 14, 53200),
  ('1109', 'CBM-KRT', 13, 50600),
  ('1109', 'CBM-KRT', 12, 53100),
  ('1109', 'CBM-KRT', 11, 53700),
  ('1109', 'CBM-KRT', 10, 52900),
  ('1109', 'CBM-KRT', 9, 51600),
  ('1109', 'CBM-KRT', 8, 54000),
  ('1109', 'CBM-KRT', 7, 51700),
  ('1109', 'CBM-KRT', 6, 52200),
  ('1109', 'CBM-KRT', 5, 53700),
  ('1109', 'CBM-KRT', 4, 53100),
  ('1109', 'CBM-KRT', 3, 54000),
  ('1109', 'CBM-KRT', 2, 54500),
  ('1109', 'CBM-KRT', 1, 53500),
  ('1109', 'BWM', 14, 40400),
  ('1109', 'BWM', 13, 39900),
  ('1109', 'BWM', 12, 40600),
  ('1109', 'BWM', 11, 41300),
  ('1109', 'BWM', 10, 40700),
  ('1109', 'BWM', 9, 40700),
  ('1109', 'BWM', 8, 42200),
  ('1109', 'BWM', 7, 41700),
  ('1109', 'BWM', 6, 42700),
  ('1109', 'BWM', 5, 41900),
  ('1109', 'BWM', 4, 43100),
  ('1109', 'BWM', 3, 40900),
  ('1109', 'BWM', 2, 42000),
  ('1109', 'BWM', 1, 43000),
  ('1109', 'TLR', 14, 30000),
  ('1109', 'TLR', 13, 30300),
  ('1109', 'TLR', 12, 30200),
  ('1109', 'TLR', 11, 29800),
  ('1109', 'TLR', 10, 30500),
  ('1109', 'TLR', 9, 30600),
  ('1109', 'TLR', 8, 30400),
  ('1109', 'TLR', 7, 29000),
  ('1109', 'TLR', 6, 30400),
  ('1109', 'TLR', 5, 29900),
  ('1109', 'TLR', 4, 30400),
  ('1109', 'TLR', 3, 29300),
  ('1109', 'TLR', 2, 31100),
  ('1109', 'TLR', 1, 30300)
       ) AS v(kabupaten_kode, komoditas_kode, h, harga)
  JOIN komoditas k ON k.kode = v.komoditas_kode
  JOIN sumber_data s ON s.kode = 'SRC-BI'
  LEFT JOIN LATERAL (
        SELECT e.id AS eksekusi_id, m.id AS data_mentah_id
          FROM eksekusi_sumber e JOIN data_mentah m ON m.eksekusi_id = e.id
         WHERE e.sumber_id = s.id AND e.pemicu = 'DEMO'
         ORDER BY e.mulai DESC, e.id DESC LIMIT 1) AS lin ON true
ON CONFLICT (sumber_id, komoditas_kode, kabupaten_kode, tanggal, tipe) DO UPDATE SET
    harga = EXCLUDED.harga, satuan = EXCLUDED.satuan,
    data_mentah_id = EXCLUDED.data_mentah_id, eksekusi_id = EXCLUDED.eksekusi_id
 WHERE harga_eksternal.eksekusi_id IN (SELECT id FROM eksekusi_sumber WHERE pemicu = 'DEMO');

-- =====================================================================
-- Lineage cuaca harian demo: cuaca BMKG 7 hari terakhir (demo/04_cuaca_harian.sql) menunjuk eksekusi dan payload BMKG
-- contoh terakhir. Hanya baris tanpa lineage atau yang sudah berasal dari eksekusi contoh yang disentuh; cuaca hasil
-- ingestion nyata tidak pernah ditimpa.
-- =====================================================================
UPDATE cuaca_harian c SET eksekusi_id = lin.eksekusi_id, data_mentah_id = lin.data_mentah_id
  FROM (SELECT e.id AS eksekusi_id, m.id AS data_mentah_id
          FROM eksekusi_sumber e
          JOIN data_mentah m ON m.eksekusi_id = e.id
          JOIN sumber_data s ON s.id = e.sumber_id AND s.kode = 'SRC-BMKG'
         WHERE e.pemicu = 'DEMO'
         ORDER BY e.mulai DESC, e.id DESC LIMIT 1) AS lin
 WHERE c.sumber = 'BMKG'
   AND c.tanggal BETWEEN (now() AT TIME ZONE 'Asia/Jakarta')::date - 6 AND (now() AT TIME ZONE 'Asia/Jakarta')::date
   AND (c.eksekusi_id IS NULL OR c.eksekusi_id IN (SELECT id FROM eksekusi_sumber WHERE pemicu = 'DEMO'))
   AND (c.eksekusi_id IS DISTINCT FROM lin.eksekusi_id OR c.data_mentah_id IS DISTINCT FROM lin.data_mentah_id);
