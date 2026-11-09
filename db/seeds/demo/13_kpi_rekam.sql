-- GARDA INFLASI - Rekaman Realisasi KPI Bulanan (Demo)
--
-- Setara dengan seeder Go lama kpi.SeedDemo(now): TIDAK ADA angka tulis tangan. Berkas ini menghitung nilai tiap KPI
-- yang punya penghitung otomatis (kpi_definisi.sumber_query) dari data yang SUDAH ADA di basis data (entri harga,
-- cold storage dan buku besar stok, intervensi, prediksi) untuk 12 bulan yang berakhir di bulan berjalan, memakai
-- rumus yang sama dengan penghitung Go (kpi/repository/postgres/repo.go, map `penghitung`):
--   * bulan berjalan menurut UTC, (now() AT TIME ZONE 'UTC'), bukan CURRENT_DATE/zona sesi: sesi aplikasi/tes
--     (TimeZone=Asia/Jakarta) dan sesi cmd/seed (zona bawaan server) berbeda; UTC sama dengan time.Now() pada container;
--   * batas bulan menurut WIB; @mulai/@akhir = j.mulai/j.akhir (akhir eksklusif); @asof = j.asof = min(akhir, sekarang);
--   * hasil NULL = tanpa dasar data: tidak ada baris (menunggu data), bukan nol;
--   * nilai dibulatkan 4 desimal; nilai sama (selisih < 5e-5) tidak ditulis ulang, nilai berbeda menjadi versi
--     baru (revisi, versi lama tetap ada) seperti SimpanNilai. Berkas ini idempoten.
-- Karena itu hasilnya mengikuti data yang di-seed: hanya cold storage -> hanya KPI logistik; tanpa data sumber tidak
-- ada baris sama sekali. Jalankan setelah berkas demo sumber (harga, intervensi, prediksi, cold storage) dan
-- reference/13_kpi_definisi.sql. Paritas dengan penghitung Go dijaga tes integrasi KPI (job Go tidak boleh
-- menemukan selisih pada data hasil seed).

-- =====================================================================
-- Tabel: kpi_nilai (dihitung; wilayah_kode NULL = seluruh provinsi)
-- =====================================================================
WITH bulan AS (
    SELECT m::date AS periode,
           m AT TIME ZONE 'Asia/Jakarta' AS mulai,
           (m + interval '1 month') AT TIME ZONE 'Asia/Jakarta' AS akhir
      FROM generate_series(
             date_trunc('month', now() AT TIME ZONE 'UTC') - interval '11 months',
             date_trunc('month', now() AT TIME ZONE 'UTC'),
             interval '1 month') AS m
), jendela AS (
    -- Bulan di masa depan dilewati; asof = min(akhir bulan, sekarang) untuk ukuran berbasis keadaan seperti stok.
    SELECT periode, mulai, akhir, LEAST(akhir, now()) AS asof FROM bulan WHERE mulai <= now()
), hitung (sumber_query, periode, nilai) AS (

    SELECT 'produsen_terverifikasi', j.periode, q.nilai
      FROM jendela j CROSS JOIN LATERAL (
        SELECT 100.0 * COUNT(DISTINCT kode_pedagang) FILTER (WHERE status IN ('DIVALIDASI', 'DIPUBLIKASI'))
               / NULLIF(COUNT(DISTINCT kode_pedagang), 0) AS nilai
          FROM entri_harga
         WHERE tipe = 'PRODUSEN' AND kode_pedagang IS NOT NULL
           AND dicatat_pada >= j.mulai AND dicatat_pada < j.akhir) q

    UNION ALL
    SELECT 'ketepatan_data', j.periode, q.nilai
      FROM jendela j CROSS JOIN LATERAL (
        SELECT 100.0 * COUNT(*) FILTER (WHERE status IN ('DIVALIDASI', 'DIPUBLIKASI'))
               / NULLIF(COUNT(*) FILTER (WHERE status IN ('DIVALIDASI', 'DIPUBLIKASI', 'DITOLAK')), 0) AS nilai
          FROM entri_harga
         WHERE dicatat_pada >= j.mulai AND dicatat_pada < j.akhir) q

    -- Rata-rata harian: pasangan (pasar aktif x komoditas pilot) yang punya entri tervalidasi pada hari itu, dibagi
    -- seluruh pasangan. Hari yang dihitung hanya yang sudah berlalu pada asof.
    UNION ALL
    SELECT 'kelengkapan_harian', j.periode, q.nilai
      FROM jendela j CROSS JOIN LATERAL (
        WITH hari AS (
            SELECT d::date AS tgl
              FROM generate_series(
                   (j.mulai AT TIME ZONE 'Asia/Jakarta')::date,
                   LEAST((j.akhir AT TIME ZONE 'Asia/Jakarta')::date - 1, (j.asof AT TIME ZONE 'Asia/Jakarta')::date),
                   interval '1 day') d
        ), pasangan AS (
            SELECT COUNT(*) AS n FROM pasar p CROSS JOIN komoditas k WHERE p.aktif AND k.pilot
        ), terisi AS (
            SELECT (e.dicatat_pada AT TIME ZONE 'Asia/Jakarta')::date AS tgl,
                   COUNT(DISTINCT (e.pasar_id, e.komoditas_kode)) AS n
              FROM entri_harga e
              JOIN pasar p ON p.kode = e.pasar_id AND p.aktif
              JOIN komoditas k ON k.kode = e.komoditas_kode AND k.pilot
             WHERE e.status IN ('DIVALIDASI', 'DIPUBLIKASI')
               AND e.dicatat_pada >= j.mulai AND e.dicatat_pada < j.akhir
             GROUP BY 1
        )
        SELECT CASE WHEN EXISTS (SELECT 1 FROM terisi) AND (SELECT n FROM pasangan) > 0 AND EXISTS (SELECT 1 FROM hari)
                    THEN 100.0 * (SELECT COALESCE(SUM(n), 0) FROM terisi WHERE tgl IN (SELECT tgl FROM hari))
                         / ((SELECT COUNT(*) FROM hari) * (SELECT n FROM pasangan))
               END AS nilai) q

    -- Stok per fasilitas pada asof dari buku besar transaksi_stok (masuk menambah tujuan, keluar mengurangi asal,
    -- pindah memindahkan keduanya). Sebelum ada satu pun transaksi dipakai pendekatan lama: lot dianggap tersimpan dari
    -- masuk_pada sampai kedaluwarsa_pada. Kapasitas memakai fasilitas operasional saat ini.
    UNION ALL
    SELECT 'utilisasi_cold_storage', j.periode, q.nilai
      FROM jendela j CROSS JOIN LATERAL (
        WITH kapasitas AS (
            SELECT COALESCE(SUM(kapasitas_ton), 0) AS total FROM cold_storage WHERE status = 'OPERASIONAL'
        ), gerak AS (
            SELECT fasilitas_tujuan_id AS fasilitas, volume_ton AS delta FROM transaksi_stok
             WHERE waktu <= j.asof AND fasilitas_tujuan_id IS NOT NULL
            UNION ALL
            SELECT fasilitas_asal_id, -volume_ton FROM transaksi_stok
             WHERE waktu <= j.asof AND fasilitas_asal_id IS NOT NULL
        ), buku AS (
            SELECT SUM(g.delta) AS volume
              FROM gerak g JOIN cold_storage c ON c.id = g.fasilitas AND c.status = 'OPERASIONAL'
        ), lama AS (
            SELECT SUM(s.volume_ton) AS volume
              FROM stok_cold_storage s JOIN cold_storage c ON c.id = s.cold_storage_id
             WHERE c.status = 'OPERASIONAL' AND s.masuk_pada <= j.asof AND s.kedaluwarsa_pada > j.asof
        )
        SELECT CASE
                 WHEN (SELECT total FROM kapasitas) = 0 THEN NULL
                 WHEN EXISTS (SELECT 1 FROM transaksi_stok) THEN
                      CASE WHEN EXISTS (SELECT 1 FROM transaksi_stok WHERE waktu <= j.asof)
                           THEN 100.0 * (SELECT volume FROM buku) / (SELECT total FROM kapasitas) END
                 ELSE 100.0 * (SELECT volume FROM lama) / (SELECT total FROM kapasitas)
               END AS nilai) q

    -- Volume tiap lot pada asof dari buku besar: masuk + terima pindah sebagian - keluar - kirim pindah sebagian.
    -- Risiko memakai nilai risiko lot saat ini.
    UNION ALL
    SELECT 'risiko_food_loss', j.periode, q.nilai
      FROM jendela j CROSS JOIN LATERAL (
        WITH lot AS (
            SELECT s.risiko_food_loss AS risiko,
                   COALESCE((SELECT SUM(t.volume_ton) FROM transaksi_stok t WHERE t.lot_id = s.id AND t.jenis = 'MASUK' AND t.waktu <= j.asof), 0)
                 + COALESCE((SELECT SUM(t.volume_ton) FROM transaksi_stok t WHERE t.lot_baru_id = s.id AND t.waktu <= j.asof), 0)
                 - COALESCE((SELECT SUM(t.volume_ton) FROM transaksi_stok t WHERE t.lot_id = s.id AND t.jenis = 'KELUAR' AND t.waktu <= j.asof), 0)
                 - COALESCE((SELECT SUM(t.volume_ton) FROM transaksi_stok t WHERE t.lot_id = s.id AND t.jenis = 'PINDAH' AND t.lot_baru_id IS NOT NULL AND t.waktu <= j.asof), 0)
                   AS volume
              FROM stok_cold_storage s
        ), buku AS (
            SELECT volume, risiko FROM lot WHERE volume > 0.000001
        ), lama AS (
            SELECT volume_ton AS volume, risiko_food_loss AS risiko
              FROM stok_cold_storage WHERE masuk_pada <= j.asof AND kedaluwarsa_pada > j.asof
        ), dasar AS (
            SELECT volume, risiko FROM buku WHERE EXISTS (SELECT 1 FROM transaksi_stok)
            UNION ALL
            SELECT volume, risiko FROM lama WHERE NOT EXISTS (SELECT 1 FROM transaksi_stok)
        )
        SELECT 100.0 * SUM(volume * risiko) / NULLIF(SUM(volume), 0) AS nilai FROM dasar) q

    UNION ALL
    SELECT 'rekomendasi_ai_dieksekusi', j.periode, q.nilai
      FROM jendela j CROSS JOIN LATERAL (
        SELECT 100.0 * COUNT(*) FILTER (WHERE tahap LIKE '%-selesai') / NULLIF(COUNT(*), 0) AS nilai
          FROM intervensi
         WHERE rekomendasi_ai AND dibuat_pada >= j.mulai AND dibuat_pada < j.akhir) q

    -- 16-B: model dan dampak. Hanya entri harga yang lolos validasi dianggap kenyataan.
    -- MAPE prediksi harga: proyeksi yang tanggal targetnya jatuh pada bulan itu dibandingkan dengan rata-rata harga
    -- konsumen tervalidasi pada hari dan kabupaten yang sama. Proyeksi tanpa kenyataan pembanding diabaikan.
    UNION ALL
    SELECT 'mape_harga', j.periode, q.nilai
      FROM jendela j CROSS JOIN LATERAL (
        WITH aktual AS (
            SELECT (e.dicatat_pada AT TIME ZONE 'Asia/Jakarta')::date AS tgl, p.kabupaten_kode, e.komoditas_kode, AVG(e.harga) AS harga
              FROM entri_harga e JOIN pasar p ON p.kode = e.pasar_id
             WHERE e.tipe = 'KONSUMEN' AND e.status IN ('DIVALIDASI', 'DIPUBLIKASI')
               AND e.dicatat_pada >= j.mulai AND e.dicatat_pada < j.akhir
             GROUP BY 1, 2, 3
        )
        SELECT 100.0 * AVG(ABS(pr.nilai - a.harga) / a.harga) AS nilai
          FROM prediksi_harga pr
          JOIN aktual a ON a.tgl = pr.tanggal_target AND a.kabupaten_kode = pr.kabupaten_kode AND a.komoditas_kode = pr.komoditas_kode
         WHERE a.harga > 0
           AND pr.tanggal_target >= (j.mulai AT TIME ZONE 'Asia/Jakarta')::date
           AND pr.tanggal_target < (j.akhir AT TIME ZONE 'Asia/Jakarta')::date) q

    -- Koefisien variasi harga produsen, dirata-rata atas kelompok (komoditas, kabupaten) dengan minimal 5 entri
    -- tervalidasi pada bulan itu.
    UNION ALL
    SELECT 'volatilitas_harga', j.periode, q.nilai
      FROM jendela j CROSS JOIN LATERAL (
        SELECT AVG(100.0 * sd / NULLIF(rata, 0)) AS nilai FROM (
            SELECT STDDEV_SAMP(e.harga) AS sd, AVG(e.harga) AS rata
              FROM entri_harga e JOIN pasar p ON p.kode = e.pasar_id
             WHERE e.tipe = 'PRODUSEN' AND e.status IN ('DIVALIDASI', 'DIPUBLIKASI')
               AND e.dicatat_pada >= j.mulai AND e.dicatat_pada < j.akhir
             GROUP BY e.komoditas_kode, p.kabupaten_kode
            HAVING COUNT(*) >= 5) g) q

    -- Margin = (konsumen - produsen) / konsumen, dirata-rata atas kelompok (komoditas, kabupaten) yang punya kedua
    -- jenis harga tervalidasi pada bulan itu.
    UNION ALL
    SELECT 'margin_produsen_konsumen', j.periode, q.nilai
      FROM jendela j CROSS JOIN LATERAL (
        WITH rata AS (
            SELECT e.komoditas_kode, p.kabupaten_kode, e.tipe, AVG(e.harga) AS harga
              FROM entri_harga e JOIN pasar p ON p.kode = e.pasar_id
             WHERE e.tipe IN ('PRODUSEN', 'KONSUMEN') AND e.status IN ('DIVALIDASI', 'DIPUBLIKASI')
               AND e.dicatat_pada >= j.mulai AND e.dicatat_pada < j.akhir
             GROUP BY 1, 2, 3
        )
        SELECT AVG(100.0 * (k.harga - pr.harga) / NULLIF(k.harga, 0)) AS nilai
          FROM rata k JOIN rata pr ON pr.komoditas_kode = k.komoditas_kode AND pr.kabupaten_kode = k.kabupaten_kode
         WHERE k.tipe = 'KONSUMEN' AND pr.tipe = 'PRODUSEN') q

    -- Volume yang masuk cold storage pada bulan itu (buku besar transaksi_stok).
    UNION ALL
    SELECT 'volume_agregasi', j.periode, q.nilai
      FROM jendela j CROSS JOIN LATERAL (
        SELECT SUM(volume_ton) AS nilai FROM transaksi_stok
         WHERE jenis = 'MASUK' AND waktu >= j.mulai AND waktu < j.akhir) q

    -- Penghematan rencana optimal terhadap rencana pembanding serakah pada rencana yang disimpan di bulan itu
    -- (hanya rencana terbaru per komoditas dan periode neraca).
    UNION ALL
    SELECT 'efisiensi_biaya_logistik', j.periode, q.nilai
      FROM jendela j CROSS JOIN LATERAL (
        SELECT 100.0 * (SUM(biaya_pembanding) - SUM(total_biaya)) / NULLIF(SUM(biaya_pembanding), 0) AS nilai
          FROM optimasi_rute o
         WHERE o.biaya_pembanding IS NOT NULL AND o.dibuat_pada >= j.mulai AND o.dibuat_pada < j.akhir
           AND NOT EXISTS (SELECT 1 FROM optimasi_rute baru
                            WHERE baru.komoditas_kode = o.komoditas_kode AND baru.periode = o.periode
                              AND baru.dibuat_pada < j.akhir
                              AND (baru.dibuat_pada, baru.id) > (o.dibuat_pada, o.id))) q

), terhitung AS (
    SELECT d.kode AS kpi_kode, d.urutan, h.periode, round(h.nilai::numeric, 4) AS nilai
      FROM hitung h JOIN kpi_definisi d ON d.sumber_query = h.sumber_query
     WHERE h.nilai IS NOT NULL
)
INSERT INTO kpi_nilai (kpi_kode, wilayah_kode, periode, versi, nilai, dihitung_pada)
SELECT t.kpi_kode, NULL, t.periode, COALESCE(lama.versi, 0) + 1, t.nilai, now()
  FROM terhitung t
  LEFT JOIN LATERAL (
        SELECT n.versi, n.nilai FROM kpi_nilai n
         WHERE n.kpi_kode = t.kpi_kode AND n.wilayah_kode IS NULL AND n.periode = t.periode
         ORDER BY n.versi DESC LIMIT 1) lama ON true
 WHERE lama.versi IS NULL OR abs(lama.nilai - t.nilai) >= 0.00005
 ORDER BY t.periode, t.urutan;
