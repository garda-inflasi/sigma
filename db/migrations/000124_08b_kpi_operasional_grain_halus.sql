-- Slice 08-B (docs/todos/08-dashboard-operasional.md): filter wilayah, komoditas, dan periode pada KPI dashboard
-- operasional, serta row scope Koordinator dan Agregator.
--
-- mv_kpi_operasional sebelumnya hanya berisi nilai jadi tingkat provinsi untuk 14 hari terakhir (indikator, tanggal,
-- nilai). Nilai jadi tidak bisa difilter: rata-rata persentase antarkabupaten bukan persentase gabungan, dan jumlah
-- komoditas yang berbeda tidak bisa dijumlahkan antarkabupaten. Migrasi ini membangunnya ulang sebagai FAKTA HARIAN
-- per kabupaten dan komoditas, dengan pembilang dan penyebut; nilai kartu dihitung dari fakta itu sesuai filter pada
-- saat dibaca (internal/dashboard/usecase), sehingga hasilnya tepat untuk kombinasi filter apa pun:
--
--   indikator  baris (satu per tanggal WIB, kabupaten, komoditas)     pembilang            penyebut
--   ---------  -----------------------------------------------------  -------------------  ---------------------
--   entri      entri harga selain DRAF yang dicatat pada hari itu       jumlah entri         0 (tidak dipakai)
--   validasi   entri yang sama; DIVALIDASI/DIPUBLIKASI dari seluruhnya  entri tervalidasi    jumlah entri
--   waspada    sel yang harga konsumennya > 7% di atas rata-rata        1                    0 (tidak dipakai)
--              tertimbang 7 hari sebelumnya (hanya sel yang melampaui)
--   cs         volume lot aktif pada hari itu di fasilitas OPERASIONAL  volume (ton)         0 (tidak dipakai)
--   cs         kapasitas fasilitas OPERASIONAL di kabupaten itu         0 (tidak dipakai)   kapasitas (ton)
--              (komoditas_kode = '': baris tingkat kabupaten tanpa dimensi komoditas)
--
-- Cara menyatukan baris menjadi nilai kartu (dilakukan pembaca, bukan view):
--   entri     jumlah pembilang pada hari itu.
--   validasi  100 x jumlah pembilang / jumlah penyebut dalam jendela 7 hari yang berakhir pada hari itu (0 bila tak ada
--             entri). Pembilang dan penyebut dijumlahkan dulu, baru dibagi: bukan rata-rata persentase.
--   waspada   jumlah komoditas BERBEDA yang punya baris pada hari itu (komoditas yang melampaui di dua kabupaten tetap
--             dihitung satu kali).
--   cs        100 x jumlah volume / jumlah kapasitas pada hari itu (0 bila tak ada kapasitas). Dengan filter komoditas,
--             pembilangnya hanya volume komoditas itu sedangkan penyebutnya tetap kapasitas seluruh fasilitas.
--
-- Rumus tiap indikator sama dengan definisi lama (000122), sehingga tampilan tanpa filter tidak berubah.
--
-- Riwayat: 120 hari (hari ini dan 119 hari sebelumnya) supaya periode yang diminta (paling lama 90 hari ke belakang,
-- domain.RiwayatHari) masih punya 13 hari pembanding delta dan 6 hari jendela validasi di belakangnya. Baris cs
-- dihitung sampai besok: lot yang berlaku diketahui di muka, sehingga pergantian hari sebelum penyegaran berikutnya
-- tidak membuat utilisasi hari baru tampil 0. Ukuran: sekitar 90 ribu baris (11 MB) pada 2,4 juta entri harga
-- (2.352 entri per hari); REFRESH CONCURRENTLY ~1,2 detik (view lama: ~0,7 detik).
--
-- Kunci unik (indikator, tanggal, kabupaten_kode, komoditas_kode) wajib ada untuk REFRESH ... CONCURRENTLY dan tidak
-- boleh memuat NULL, itu sebabnya baris kapasitas memakai komoditas '' (bukan NULL).

-- +goose Up
-- +goose StatementBegin
DROP MATERIALIZED VIEW IF EXISTS mv_kpi_operasional;

CREATE MATERIALIZED VIEW mv_kpi_operasional AS
WITH p AS (
    SELECT (now() AT TIME ZONE 'Asia/Jakarta')::date AS hari_ini
),
hari AS (
    SELECT p.hari_ini + g.n AS tanggal FROM p, generate_series(-119, 1) AS g(n)
),
entri AS (
    SELECT (e.dicatat_pada AT TIME ZONE 'Asia/Jakarta')::date AS tanggal, ps.kabupaten_kode, e.komoditas_kode,
           COUNT(*) AS total,
           COUNT(*) FILTER (WHERE e.status IN ('DIVALIDASI', 'DIPUBLIKASI')) AS valid
      FROM entri_harga e
      JOIN pasar ps ON ps.kode = e.pasar_id
     WHERE e.status <> 'DRAF'
       AND e.dicatat_pada >= (SELECT (p.hari_ini - 119)::timestamp AT TIME ZONE 'Asia/Jakarta' FROM p)
       AND e.dicatat_pada <  (SELECT (p.hari_ini + 1)::timestamp AT TIME ZONE 'Asia/Jakarta' FROM p)
     GROUP BY 1, 2, 3
),
harga AS (
    SELECT g.tanggal, g.kabupaten_kode, g.komoditas_kode, g.rata_rata,
           (SELECT SUM(b.rata_rata * b.jumlah) / SUM(b.jumlah)
              FROM agregat_harga_harian b
             WHERE b.tipe = 'KONSUMEN' AND b.komoditas_kode = g.komoditas_kode AND b.kabupaten_kode = g.kabupaten_kode
               AND b.tanggal BETWEEN g.tanggal - 7 AND g.tanggal - 1) AS basis
      FROM agregat_harga_harian g
     WHERE g.tipe = 'KONSUMEN'
       AND g.tanggal BETWEEN (SELECT p.hari_ini - 119 FROM p) AND (SELECT p.hari_ini FROM p)
),
lot AS (
    SELECT c.kabupaten_kode, s.komoditas_kode, s.volume_ton,
           (s.masuk_pada AT TIME ZONE 'Asia/Jakarta')::date AS masuk,
           (s.kedaluwarsa_pada AT TIME ZONE 'Asia/Jakarta')::date AS kedaluwarsa
      FROM stok_cold_storage s
      JOIN cold_storage c ON c.id = s.cold_storage_id AND c.status = 'OPERASIONAL'
     WHERE s.kedaluwarsa_pada >= (SELECT (p.hari_ini - 119)::timestamp AT TIME ZONE 'Asia/Jakarta' FROM p)
),
kapasitas AS (
    SELECT kabupaten_kode, SUM(kapasitas_ton)::double precision AS ton
      FROM cold_storage
     WHERE status = 'OPERASIONAL'
     GROUP BY kabupaten_kode
)
SELECT 'entri'::text AS indikator, tanggal, kabupaten_kode, komoditas_kode,
       total::double precision AS pembilang, 0::double precision AS penyebut
  FROM entri
UNION ALL
SELECT 'validasi', tanggal, kabupaten_kode, komoditas_kode, valid::double precision, total::double precision
  FROM entri
UNION ALL
SELECT 'waspada', tanggal, kabupaten_kode, komoditas_kode, 1::double precision, 0::double precision
  FROM harga
 WHERE basis > 0 AND (rata_rata - basis) / basis > 0.07
UNION ALL
SELECT 'cs', h.tanggal, l.kabupaten_kode, l.komoditas_kode, SUM(l.volume_ton)::double precision, 0::double precision
  FROM hari h
  JOIN lot l ON l.masuk <= h.tanggal AND l.kedaluwarsa > h.tanggal
 GROUP BY h.tanggal, l.kabupaten_kode, l.komoditas_kode
UNION ALL
SELECT 'cs', h.tanggal, k.kabupaten_kode, ''::varchar(255), 0::double precision, k.ton
  FROM hari h CROSS JOIN kapasitas k
WITH DATA;

CREATE UNIQUE INDEX uq_mv_kpi_operasional
    ON mv_kpi_operasional (indikator, tanggal, kabupaten_kode, komoditas_kode);

COMMENT ON MATERIALIZED VIEW mv_kpi_operasional IS
    'Fakta harian KPI operasional per kabupaten dan komoditas (pembilang/penyebut); lihat 000124. Nilai kartu dihitung pembaca.';

DO $hak$
BEGIN
    IF EXISTS (SELECT 1 FROM pg_roles WHERE rolname = 'garda_aplikasi') THEN
        PERFORM garda_terapkan_hak_aplikasi();
    END IF;
END
$hak$;
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DROP MATERIALIZED VIEW IF EXISTS mv_kpi_operasional;

-- Definisi 000122 (nilai jadi tingkat provinsi, 14 hari).
CREATE MATERIALIZED VIEW mv_kpi_operasional AS
WITH hari AS (
    SELECT d::date AS tanggal
      FROM generate_series((now() AT TIME ZONE 'Asia/Jakarta')::date - 13, (now() AT TIME ZONE 'Asia/Jakarta')::date, interval '1 day') AS d
),
entri AS (
    SELECT (dicatat_pada AT TIME ZONE 'Asia/Jakarta')::date AS tanggal, status
      FROM entri_harga
     WHERE status <> 'DRAF' AND dicatat_pada >= now() - interval '25 days'
),
harian AS (
    SELECT komoditas_kode, kabupaten_kode, tanggal, rata_rata,
           (SELECT SUM(b.rata_rata * b.jumlah) / SUM(b.jumlah)
              FROM agregat_harga_harian b
             WHERE b.tipe = 'KONSUMEN' AND b.komoditas_kode = a.komoditas_kode AND b.kabupaten_kode = a.kabupaten_kode
               AND b.tanggal BETWEEN a.tanggal - 7 AND a.tanggal - 1) AS basis
      FROM agregat_harga_harian a
     WHERE a.tipe = 'KONSUMEN' AND a.tanggal >= (now() AT TIME ZONE 'Asia/Jakarta')::date - 13
),
kapasitas AS (
    SELECT COALESCE(SUM(kapasitas_ton), 0) AS ton FROM cold_storage WHERE status = 'OPERASIONAL'
)
SELECT 'entri'::text AS indikator, h.tanggal,
       (SELECT COUNT(*) FROM entri e WHERE e.tanggal = h.tanggal)::double precision AS nilai
  FROM hari h
UNION ALL
SELECT 'validasi', h.tanggal,
       COALESCE((SELECT 100.0 * COUNT(*) FILTER (WHERE status IN ('DIVALIDASI', 'DIPUBLIKASI')) / NULLIF(COUNT(*), 0)
                   FROM entri e
                  WHERE e.tanggal BETWEEN h.tanggal - 6 AND h.tanggal), 0)::double precision
  FROM hari h
UNION ALL
SELECT 'waspada', h.tanggal,
       (SELECT COUNT(DISTINCT komoditas_kode)
          FROM harian a
         WHERE a.tanggal = h.tanggal AND a.basis > 0 AND (a.rata_rata - a.basis) / a.basis > 0.07)::double precision
  FROM hari h
UNION ALL
SELECT 'cs', h.tanggal,
       CASE WHEN k.ton > 0 THEN
           100.0 * COALESCE((SELECT SUM(s.volume_ton)
                               FROM stok_cold_storage s
                               JOIN cold_storage c ON c.id = s.cold_storage_id AND c.status = 'OPERASIONAL'
                              WHERE (s.masuk_pada AT TIME ZONE 'Asia/Jakarta')::date <= h.tanggal
                                AND (s.kedaluwarsa_pada AT TIME ZONE 'Asia/Jakarta')::date > h.tanggal), 0) / k.ton
       ELSE 0 END::double precision
  FROM hari h CROSS JOIN kapasitas k
WITH DATA;

CREATE UNIQUE INDEX uq_mv_kpi_operasional ON mv_kpi_operasional(indikator, tanggal);

DO $hak$
BEGIN
    IF EXISTS (SELECT 1 FROM pg_roles WHERE rolname = 'garda_aplikasi') THEN
        PERFORM garda_terapkan_hak_aplikasi();
    END IF;
END
$hak$;
-- +goose StatementEnd
