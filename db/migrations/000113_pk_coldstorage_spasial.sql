-- Standardisasi kunci primer (docs/SKEMA-DATABASE.md), langkah 4: cold storage dan spasial.
--
-- Bentuk akhir per tabel:
--   cold_storage           id VARCHAR     -> UUID  (garda_uuid_stabil('cold_storage', id_lama)); ID lama jadi kode
--   stok_cold_storage      id VARCHAR     -> BIGINT identity (urut masuk_pada, id lama); ID lama jadi kode
--   transaksi_stok         id VARCHAR(64) -> BIGINT identity (urut waktu, id lama); ID lama jadi kode
--   kandidat_cold_storage  id VARCHAR     -> BIGINT identity (urut dibuat_pada, id lama); ID lama jadi kode
--   optimasi_rute          id VARCHAR(64) -> BIGINT identity (urut dibuat_pada, id lama); ID lama jadi kode
--   rute_distribusi        id VARCHAR     -> BIGINT identity (urut dibuat_pada, id lama); ID lama jadi kode
--   neraca_wilayah         PK komposit (komoditas_kode, kabupaten_kode, periode) -> id BIGINT identity, kunci
--                          alami tetap UNIQUE (target ON CONFLICT upsert seeder)
--   telemetri_suhu         BIGSERIAL tetap; fasilitas_id -> uuid
--   jarak_antar_wilayah    KOMPOSIT (asal_kode, tujuan_kode) tetap
--   wilayah_geometri       KODE kabupaten_kode tetap (1:1 dengan kabupaten)
--
-- Kolom anak (semuanya FK nyata, jadi tidak ada baris yatim) dikonversi dengan ALTER COLUMN ... TYPE ... USING:
-- UUID lewat garda_uuid_stabil, BIGINT lewat fungsi sementara pg_temp.g3_* (USING tidak boleh berisi subkueri).
-- ALTER TYPE membangun ulang indeks dan constraint UNIQUE pada kolom itu, sehingga tidak ada yang hilang diam-diam.
--
-- ON DELETE (SKEMA §2):
--   CASCADE   milik agregat: stok dan telemetri milik fasilitas; rute milik rencana optimasi; matriks jarak dan
--             geometri milik kabupaten (data turunan wilayah)
--   RESTRICT  buku besar tidak boleh kehilangan fasilitas atau lot: transaksi_stok.{fasilitas_asal_id,
--             fasilitas_tujuan_id, lot_id, lot_baru_id}; seluruh rujukan ke kabupaten dan komoditas dari
--             cold_storage, stok_cold_storage, kandidat_cold_storage, optimasi_rute, rute_distribusi, neraca_wilayah
-- Setiap FK yang belum punya indeks berawalan kolomnya diberi indeks (nama idx_<tabel>_<kolom>).
--
-- Indeks lain: stok lot aktif per fasilitas (parsial, kueri FEFO/kedaluwarsa dan hitung isi), BRIN telemetri_suhu(waktu)
-- (tabel time-series append-only; komposit (fasilitas_id, waktu) untuk grafik per fasilitas sudah dilayani oleh
-- UNIQUE (fasilitas_id, waktu) lewat pemindaian mundur), rute usulan aktif, dan neraca per kabupaten.
--
-- mv_kpi_operasional membaca cold_storage.id dan stok_cold_storage.cold_storage_id, dan PostgreSQL menolak mengubah
-- tipe kolom yang dipakai view. View di-DROP lebih dulu lalu dibuat ulang (definisi 000022, tanpa perubahan logika,
-- beserta indeks uniknya uq_mv_kpi_operasional yang dibutuhkan REFRESH ... CONCURRENTLY), WITH DATA, di Up maupun Down.
-- Tidak ada view lain yang bergantung pada kolom yang diubah.
--
-- Rujukan lunak (tanpa FK): audit_log.entitas_id untuk entitas cold_storage, stok_cold_storage, dan optimasi_rute
-- dipetakan ke ID baru bila nilainya sama dengan kode (ID lama) baris yang ada. Nilai lain ("2 lot", "STO-2292" untuk
-- lot yang sudah tidak ada, daftar fasilitas dipisah koma pada telemetri lama) dibiarkan sebagai sejarah.
-- audit_log append-only: pemicunya dimatikan hanya selama UPDATE pemetaan, di transaksi yang sama.
--
-- ALTER COLUMN TYPE menulis ulang tabel dengan kunci ACCESS EXCLUSIVE; jalankan di jendela pemeliharaan untuk basis
-- data besar (telemetri_suhu, transaksi_stok).
--
-- Down mengembalikan tipe VARCHAR dan ID lama (dari kode; baris tanpa kode memakai id::text), PK komposit neraca,
-- ON DELETE semula (NO ACTION, kecuali CASCADE pada stok dan telemetri), lalu membuang kode, indeks baru, dan fungsi
-- sementara.

-- +goose Up
-- +goose StatementBegin
DO $mig$
DECLARE
    n      bigint;
    mulai  bigint;
BEGIN
    -- 0. Penjaga: ID lama harus muat di kode varchar(64).
    SELECT count(*) INTO n FROM (
        SELECT id FROM cold_storage WHERE length(id) > 64
        UNION ALL SELECT id FROM stok_cold_storage WHERE length(id) > 64
        UNION ALL SELECT id FROM transaksi_stok WHERE length(id) > 64
        UNION ALL SELECT id FROM kandidat_cold_storage WHERE length(id) > 64
        UNION ALL SELECT id FROM optimasi_rute WHERE length(id) > 64
        UNION ALL SELECT id FROM rute_distribusi WHERE length(id) > 64
    ) x;
    IF n > 0 THEN
        RAISE EXCEPTION 'migrasi 000113: % ID lama lebih panjang dari 64 karakter dan tidak muat di kolom kode', n;
    END IF;

    -- 1. View yang membaca kolom yang akan berubah tipe.
    DROP MATERIALIZED VIEW IF EXISTS mv_kpi_operasional;

    -- ── cold_storage: VARCHAR -> UUID ───────────────────────────────────────────────────────────────────────
    ALTER TABLE cold_storage ADD COLUMN kode varchar(64);
    UPDATE cold_storage SET kode = id;
    ALTER TABLE cold_storage ADD CONSTRAINT uq_cold_storage_kode UNIQUE (kode);

    -- CHECK yang membandingkan dua kolom yang berubah tipe satu per satu dilepas dulu (dipasang lagi di bawah).
    ALTER TABLE transaksi_stok DROP CONSTRAINT chk_transaksi_stok_lokasi;
    ALTER TABLE transaksi_stok DROP CONSTRAINT transaksi_stok_fasilitas_asal_id_fkey;
    ALTER TABLE transaksi_stok DROP CONSTRAINT transaksi_stok_fasilitas_tujuan_id_fkey;
    ALTER TABLE stok_cold_storage DROP CONSTRAINT stok_cold_storage_cold_storage_id_fkey;
    ALTER TABLE telemetri_suhu DROP CONSTRAINT telemetri_suhu_fasilitas_id_fkey;
    ALTER TABLE cold_storage DROP CONSTRAINT cold_storage_kabupaten_kode_fkey;

    ALTER TABLE cold_storage ALTER COLUMN id TYPE uuid USING garda_uuid_stabil('cold_storage', id);
    ALTER TABLE cold_storage ALTER COLUMN id SET DEFAULT gen_random_uuid();
    ALTER TABLE stok_cold_storage ALTER COLUMN cold_storage_id TYPE uuid USING garda_uuid_stabil('cold_storage', cold_storage_id);
    ALTER TABLE telemetri_suhu ALTER COLUMN fasilitas_id TYPE uuid USING garda_uuid_stabil('cold_storage', fasilitas_id);
    ALTER TABLE transaksi_stok ALTER COLUMN fasilitas_asal_id TYPE uuid USING garda_uuid_stabil('cold_storage', fasilitas_asal_id);
    ALTER TABLE transaksi_stok ALTER COLUMN fasilitas_tujuan_id TYPE uuid USING garda_uuid_stabil('cold_storage', fasilitas_tujuan_id);

    ALTER TABLE cold_storage
        ADD CONSTRAINT cold_storage_kabupaten_kode_fkey FOREIGN KEY (kabupaten_kode) REFERENCES kabupaten(kode) ON DELETE RESTRICT;
    CREATE INDEX idx_cold_storage_kabupaten_kode ON cold_storage (kabupaten_kode);
    ALTER TABLE stok_cold_storage
        ADD CONSTRAINT stok_cold_storage_cold_storage_id_fkey FOREIGN KEY (cold_storage_id) REFERENCES cold_storage(id) ON DELETE CASCADE;
    ALTER TABLE telemetri_suhu
        ADD CONSTRAINT telemetri_suhu_fasilitas_id_fkey FOREIGN KEY (fasilitas_id) REFERENCES cold_storage(id) ON DELETE CASCADE;
    ALTER TABLE transaksi_stok
        ADD CONSTRAINT transaksi_stok_fasilitas_asal_id_fkey FOREIGN KEY (fasilitas_asal_id) REFERENCES cold_storage(id) ON DELETE RESTRICT,
        ADD CONSTRAINT transaksi_stok_fasilitas_tujuan_id_fkey FOREIGN KEY (fasilitas_tujuan_id) REFERENCES cold_storage(id) ON DELETE RESTRICT,
        ADD CONSTRAINT chk_transaksi_stok_lokasi CHECK (
            (jenis = 'MASUK' AND fasilitas_tujuan_id IS NOT NULL AND fasilitas_asal_id IS NULL)
            OR (jenis = 'KELUAR' AND fasilitas_asal_id IS NOT NULL AND fasilitas_tujuan_id IS NULL)
            OR (jenis = 'PINDAH' AND fasilitas_asal_id IS NOT NULL AND fasilitas_tujuan_id IS NOT NULL
                AND fasilitas_asal_id <> fasilitas_tujuan_id)
        );

    -- Grafik per fasilitas: UNIQUE (fasilitas_id, waktu) sudah melayani WHERE fasilitas_id = ? ORDER BY waktu DESC
    -- (pemindaian mundur), jadi tidak ada indeks komposit kedua. BRIN untuk jendela waktu lintas fasilitas.
    CREATE INDEX idx_telemetri_suhu_waktu ON telemetri_suhu USING brin (waktu) WITH (pages_per_range = 32);

    -- ── stok_cold_storage: VARCHAR -> BIGINT ────────────────────────────────────────────────────────────────
    ALTER TABLE stok_cold_storage ADD COLUMN kode varchar(64);
    UPDATE stok_cold_storage SET kode = id;
    ALTER TABLE stok_cold_storage ADD COLUMN id_baru bigint;
    UPDATE stok_cold_storage s SET id_baru = u.urut
      FROM (SELECT id, row_number() OVER (ORDER BY masuk_pada NULLS LAST, id) AS urut FROM stok_cold_storage) u
     WHERE s.id = u.id;

    ALTER TABLE transaksi_stok DROP CONSTRAINT transaksi_stok_lot_id_fkey;
    ALTER TABLE transaksi_stok DROP CONSTRAINT transaksi_stok_lot_baru_id_fkey;
    CREATE FUNCTION pg_temp.g3_id_baru_stok(text) RETURNS bigint LANGUAGE sql STABLE
        AS $f$ SELECT id_baru FROM public.stok_cold_storage WHERE id = $1 $f$;
    SELECT count(*) INTO n FROM transaksi_stok t
     WHERE (t.lot_id IS NOT NULL AND pg_temp.g3_id_baru_stok(t.lot_id) IS NULL)
        OR (t.lot_baru_id IS NOT NULL AND pg_temp.g3_id_baru_stok(t.lot_baru_id) IS NULL);
    IF n > 0 THEN
        RAISE EXCEPTION 'migrasi 000113: % baris transaksi_stok merujuk lot yang tidak ada', n;
    END IF;
    ALTER TABLE transaksi_stok ALTER COLUMN lot_id TYPE bigint USING pg_temp.g3_id_baru_stok(lot_id);
    ALTER TABLE transaksi_stok ALTER COLUMN lot_baru_id TYPE bigint USING pg_temp.g3_id_baru_stok(lot_baru_id);
    DROP FUNCTION pg_temp.g3_id_baru_stok(text);

    ALTER TABLE stok_cold_storage DROP CONSTRAINT stok_cold_storage_pkey;
    ALTER TABLE stok_cold_storage DROP COLUMN id;
    ALTER TABLE stok_cold_storage RENAME COLUMN id_baru TO id;
    ALTER TABLE stok_cold_storage ALTER COLUMN id SET NOT NULL;
    ALTER TABLE stok_cold_storage ADD CONSTRAINT stok_cold_storage_pkey PRIMARY KEY (id);
    SELECT COALESCE(max(id), 0) + 1 INTO mulai FROM stok_cold_storage;
    EXECUTE format('ALTER TABLE stok_cold_storage ALTER COLUMN id ADD GENERATED ALWAYS AS IDENTITY (START WITH %s)', mulai);
    ALTER TABLE stok_cold_storage ADD CONSTRAINT uq_stok_cold_storage_kode UNIQUE (kode);

    ALTER TABLE stok_cold_storage DROP CONSTRAINT stok_cold_storage_komoditas_kode_fkey;
    ALTER TABLE stok_cold_storage
        ADD CONSTRAINT stok_cold_storage_komoditas_kode_fkey FOREIGN KEY (komoditas_kode) REFERENCES komoditas(kode) ON DELETE RESTRICT;
    CREATE INDEX idx_stok_cold_storage_komoditas_kode ON stok_cold_storage (komoditas_kode);
    -- Lot yang belum habis per fasilitas: StokFasilitas (ORDER BY kedaluwarsa_pada, id), isi terpakai (SUM volume), dan
    -- gabungan kartu fasilitas. Indeks FK penuh idx_stok_cold_storage_fasilitas tetap ada untuk lot HABIS dan CASCADE.
    CREATE INDEX idx_stok_cold_storage_aktif ON stok_cold_storage (cold_storage_id, kedaluwarsa_pada, id) WHERE status <> 'HABIS';

    -- ── transaksi_stok: VARCHAR(64) -> BIGINT ───────────────────────────────────────────────────────────────
    ALTER TABLE transaksi_stok ADD COLUMN kode varchar(64);
    UPDATE transaksi_stok SET kode = id;
    ALTER TABLE transaksi_stok ADD COLUMN id_baru bigint;
    UPDATE transaksi_stok t SET id_baru = u.urut
      FROM (SELECT id, row_number() OVER (ORDER BY waktu NULLS LAST, id) AS urut FROM transaksi_stok) u
     WHERE t.id = u.id;
    ALTER TABLE transaksi_stok DROP CONSTRAINT transaksi_stok_pkey;
    ALTER TABLE transaksi_stok DROP COLUMN id;
    ALTER TABLE transaksi_stok RENAME COLUMN id_baru TO id;
    ALTER TABLE transaksi_stok ALTER COLUMN id SET NOT NULL;
    ALTER TABLE transaksi_stok ADD CONSTRAINT transaksi_stok_pkey PRIMARY KEY (id);
    SELECT COALESCE(max(id), 0) + 1 INTO mulai FROM transaksi_stok;
    EXECUTE format('ALTER TABLE transaksi_stok ALTER COLUMN id ADD GENERATED ALWAYS AS IDENTITY (START WITH %s)', mulai);
    ALTER TABLE transaksi_stok ADD CONSTRAINT uq_transaksi_stok_kode UNIQUE (kode);

    ALTER TABLE transaksi_stok
        ADD CONSTRAINT transaksi_stok_lot_id_fkey FOREIGN KEY (lot_id) REFERENCES stok_cold_storage(id) ON DELETE RESTRICT,
        ADD CONSTRAINT transaksi_stok_lot_baru_id_fkey FOREIGN KEY (lot_baru_id) REFERENCES stok_cold_storage(id) ON DELETE RESTRICT;
    -- lot_baru_id terisi hanya pada pindah sebagian: indeks parsial cukup untuk pemeriksaan RESTRICT.
    CREATE INDEX idx_transaksi_stok_lot_baru ON transaksi_stok (lot_baru_id) WHERE lot_baru_id IS NOT NULL;

    -- ── kandidat_cold_storage: VARCHAR -> BIGINT ────────────────────────────────────────────────────────────
    ALTER TABLE kandidat_cold_storage ADD COLUMN kode varchar(64);
    UPDATE kandidat_cold_storage SET kode = id;
    ALTER TABLE kandidat_cold_storage ADD COLUMN id_baru bigint;
    UPDATE kandidat_cold_storage k SET id_baru = u.urut
      FROM (SELECT id, row_number() OVER (ORDER BY dibuat_pada NULLS LAST, id) AS urut FROM kandidat_cold_storage) u
     WHERE k.id = u.id;
    ALTER TABLE kandidat_cold_storage DROP CONSTRAINT kandidat_cold_storage_pkey;
    ALTER TABLE kandidat_cold_storage DROP COLUMN id;
    ALTER TABLE kandidat_cold_storage RENAME COLUMN id_baru TO id;
    ALTER TABLE kandidat_cold_storage ALTER COLUMN id SET NOT NULL;
    ALTER TABLE kandidat_cold_storage ADD CONSTRAINT kandidat_cold_storage_pkey PRIMARY KEY (id);
    SELECT COALESCE(max(id), 0) + 1 INTO mulai FROM kandidat_cold_storage;
    EXECUTE format('ALTER TABLE kandidat_cold_storage ALTER COLUMN id ADD GENERATED ALWAYS AS IDENTITY (START WITH %s)', mulai);
    ALTER TABLE kandidat_cold_storage ADD CONSTRAINT uq_kandidat_cold_storage_kode UNIQUE (kode);

    ALTER TABLE kandidat_cold_storage DROP CONSTRAINT kandidat_cold_storage_kabupaten_kode_fkey;
    ALTER TABLE kandidat_cold_storage
        ADD CONSTRAINT kandidat_cold_storage_kabupaten_kode_fkey FOREIGN KEY (kabupaten_kode) REFERENCES kabupaten(kode) ON DELETE RESTRICT;
    CREATE INDEX idx_kandidat_cold_storage_kabupaten_kode ON kandidat_cold_storage (kabupaten_kode);

    -- ── optimasi_rute dan rute_distribusi: VARCHAR -> BIGINT ────────────────────────────────────────────────
    ALTER TABLE optimasi_rute ADD COLUMN kode varchar(64);
    UPDATE optimasi_rute SET kode = id;
    ALTER TABLE optimasi_rute ADD COLUMN id_baru bigint;
    UPDATE optimasi_rute o SET id_baru = u.urut
      FROM (SELECT id, row_number() OVER (ORDER BY dibuat_pada NULLS LAST, id) AS urut FROM optimasi_rute) u
     WHERE o.id = u.id;

    ALTER TABLE rute_distribusi DROP CONSTRAINT rute_distribusi_optimasi_id_fkey;
    CREATE FUNCTION pg_temp.g3_id_baru_optimasi(text) RETURNS bigint LANGUAGE sql STABLE
        AS $f$ SELECT id_baru FROM public.optimasi_rute WHERE id = $1 $f$;
    SELECT count(*) INTO n FROM rute_distribusi r
     WHERE r.optimasi_id IS NOT NULL AND pg_temp.g3_id_baru_optimasi(r.optimasi_id) IS NULL;
    IF n > 0 THEN
        RAISE EXCEPTION 'migrasi 000113: % baris rute_distribusi merujuk rencana optimasi yang tidak ada', n;
    END IF;
    ALTER TABLE rute_distribusi ALTER COLUMN optimasi_id TYPE bigint USING pg_temp.g3_id_baru_optimasi(optimasi_id);
    DROP FUNCTION pg_temp.g3_id_baru_optimasi(text);

    ALTER TABLE optimasi_rute DROP CONSTRAINT optimasi_rute_pkey;
    ALTER TABLE optimasi_rute DROP COLUMN id;
    ALTER TABLE optimasi_rute RENAME COLUMN id_baru TO id;
    ALTER TABLE optimasi_rute ALTER COLUMN id SET NOT NULL;
    ALTER TABLE optimasi_rute ADD CONSTRAINT optimasi_rute_pkey PRIMARY KEY (id);
    SELECT COALESCE(max(id), 0) + 1 INTO mulai FROM optimasi_rute;
    EXECUTE format('ALTER TABLE optimasi_rute ALTER COLUMN id ADD GENERATED ALWAYS AS IDENTITY (START WITH %s)', mulai);
    ALTER TABLE optimasi_rute ADD CONSTRAINT uq_optimasi_rute_kode UNIQUE (kode);

    ALTER TABLE optimasi_rute DROP CONSTRAINT optimasi_rute_komoditas_kode_fkey;
    ALTER TABLE optimasi_rute
        ADD CONSTRAINT optimasi_rute_komoditas_kode_fkey FOREIGN KEY (komoditas_kode) REFERENCES komoditas(kode) ON DELETE RESTRICT;
    -- Melayani FK komoditas_kode dan penggantian rencana lama (komoditas_kode = ? AND periode = ?).
    CREATE INDEX idx_optimasi_rute_komoditas_periode ON optimasi_rute (komoditas_kode, periode);

    ALTER TABLE rute_distribusi ADD COLUMN kode varchar(64);
    UPDATE rute_distribusi SET kode = id;
    ALTER TABLE rute_distribusi ADD COLUMN id_baru bigint;
    UPDATE rute_distribusi r SET id_baru = u.urut
      FROM (SELECT id, row_number() OVER (ORDER BY dibuat_pada NULLS LAST, id) AS urut FROM rute_distribusi) u
     WHERE r.id = u.id;
    ALTER TABLE rute_distribusi DROP CONSTRAINT rute_distribusi_pkey;
    ALTER TABLE rute_distribusi DROP COLUMN id;
    ALTER TABLE rute_distribusi RENAME COLUMN id_baru TO id;
    ALTER TABLE rute_distribusi ALTER COLUMN id SET NOT NULL;
    ALTER TABLE rute_distribusi ADD CONSTRAINT rute_distribusi_pkey PRIMARY KEY (id);
    SELECT COALESCE(max(id), 0) + 1 INTO mulai FROM rute_distribusi;
    EXECUTE format('ALTER TABLE rute_distribusi ALTER COLUMN id ADD GENERATED ALWAYS AS IDENTITY (START WITH %s)', mulai);
    ALTER TABLE rute_distribusi ADD CONSTRAINT uq_rute_distribusi_kode UNIQUE (kode);

    ALTER TABLE rute_distribusi DROP CONSTRAINT rute_distribusi_asal_kode_fkey;
    ALTER TABLE rute_distribusi DROP CONSTRAINT rute_distribusi_tujuan_kode_fkey;
    ALTER TABLE rute_distribusi DROP CONSTRAINT rute_distribusi_komoditas_kode_fkey;
    ALTER TABLE rute_distribusi
        ADD CONSTRAINT rute_distribusi_asal_kode_fkey FOREIGN KEY (asal_kode) REFERENCES kabupaten(kode) ON DELETE RESTRICT,
        ADD CONSTRAINT rute_distribusi_tujuan_kode_fkey FOREIGN KEY (tujuan_kode) REFERENCES kabupaten(kode) ON DELETE RESTRICT,
        ADD CONSTRAINT rute_distribusi_komoditas_kode_fkey FOREIGN KEY (komoditas_kode) REFERENCES komoditas(kode) ON DELETE RESTRICT,
        ADD CONSTRAINT rute_distribusi_optimasi_id_fkey FOREIGN KEY (optimasi_id) REFERENCES optimasi_rute(id) ON DELETE CASCADE;
    CREATE INDEX idx_rute_distribusi_asal ON rute_distribusi (asal_kode);
    CREATE INDEX idx_rute_distribusi_komoditas ON rute_distribusi (komoditas_kode);
    -- ListRute: rute yang belum digantikan, terbesar biayanya lebih dulu; baris DIGANTIKAN menumpuk tiap rencana disimpan ulang.
    CREATE INDEX idx_rute_distribusi_aktif ON rute_distribusi (biaya DESC, id) WHERE status <> 'DIGANTIKAN';

    -- ── neraca_wilayah: PK komposit -> id BIGINT + UNIQUE kunci alami ───────────────────────────────────────
    ALTER TABLE neraca_wilayah DROP CONSTRAINT neraca_wilayah_pkey;
    ALTER TABLE neraca_wilayah ADD COLUMN id bigint GENERATED ALWAYS AS IDENTITY;
    ALTER TABLE neraca_wilayah ADD CONSTRAINT neraca_wilayah_pkey PRIMARY KEY (id);
    ALTER TABLE neraca_wilayah ADD CONSTRAINT uq_neraca_wilayah_alami UNIQUE (komoditas_kode, kabupaten_kode, periode);

    ALTER TABLE neraca_wilayah DROP CONSTRAINT neraca_wilayah_komoditas_kode_fkey;
    ALTER TABLE neraca_wilayah DROP CONSTRAINT neraca_wilayah_kabupaten_kode_fkey;
    ALTER TABLE neraca_wilayah
        ADD CONSTRAINT neraca_wilayah_komoditas_kode_fkey FOREIGN KEY (komoditas_kode) REFERENCES komoditas(kode) ON DELETE RESTRICT,
        ADD CONSTRAINT neraca_wilayah_kabupaten_kode_fkey FOREIGN KEY (kabupaten_kode) REFERENCES kabupaten(kode) ON DELETE RESTRICT;
    -- komoditas_kode dilayani awalan uq_neraca_wilayah_alami; kabupaten_kode + periode untuk marker per wilayah.
    CREATE INDEX idx_neraca_wilayah_kabupaten_periode ON neraca_wilayah (kabupaten_kode, periode DESC);

    -- ── jarak_antar_wilayah dan wilayah_geometri: tetap berkode, FK turunan wilayah CASCADE ──────────────────
    ALTER TABLE jarak_antar_wilayah DROP CONSTRAINT jarak_antar_wilayah_asal_kode_fkey;
    ALTER TABLE jarak_antar_wilayah DROP CONSTRAINT jarak_antar_wilayah_tujuan_kode_fkey;
    ALTER TABLE jarak_antar_wilayah
        ADD CONSTRAINT jarak_antar_wilayah_asal_kode_fkey FOREIGN KEY (asal_kode) REFERENCES kabupaten(kode) ON DELETE CASCADE,
        ADD CONSTRAINT jarak_antar_wilayah_tujuan_kode_fkey FOREIGN KEY (tujuan_kode) REFERENCES kabupaten(kode) ON DELETE CASCADE;
    CREATE INDEX idx_jarak_antar_wilayah_tujuan ON jarak_antar_wilayah (tujuan_kode);

    ALTER TABLE wilayah_geometri DROP CONSTRAINT wilayah_geometri_kabupaten_kode_fkey;
    ALTER TABLE wilayah_geometri
        ADD CONSTRAINT wilayah_geometri_kabupaten_kode_fkey FOREIGN KEY (kabupaten_kode) REFERENCES kabupaten(kode) ON DELETE CASCADE;

    -- ── Rujukan lunak: audit_log.entitas_id yang berisi ID lama baris yang ada ──────────────────────────────
    ALTER TABLE audit_log DISABLE TRIGGER trg_audit_log_append_only;
    UPDATE audit_log a SET entitas_id = c.id::text
      FROM cold_storage c WHERE a.entitas = 'cold_storage' AND a.entitas_id = c.kode;
    UPDATE audit_log a SET entitas_id = s.id::text
      FROM stok_cold_storage s WHERE a.entitas = 'stok_cold_storage' AND a.entitas_id = s.kode;
    UPDATE audit_log a SET entitas_id = o.id::text
      FROM optimasi_rute o WHERE a.entitas = 'optimasi_rute' AND a.entitas_id = o.kode;
    ALTER TABLE audit_log ENABLE TRIGGER trg_audit_log_append_only;

    -- ── View dibuat ulang (definisi 000022) ─────────────────────────────────────────────────────────────────
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
           COALESCE((SELECT 100.0 * COUNT(*) FILTER (WHERE e.status IN ('DIVALIDASI', 'DIPUBLIKASI')) / NULLIF(COUNT(*), 0)
                       FROM entri e WHERE e.tanggal BETWEEN h.tanggal - 6 AND h.tanggal), 0)::double precision
      FROM hari h
    UNION ALL
    SELECT 'waspada', h.tanggal,
           (SELECT COUNT(DISTINCT x.komoditas_kode) FROM harian x
             WHERE x.tanggal = h.tanggal AND x.basis IS NOT NULL AND x.rata_rata > x.basis * 1.07)::double precision
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
END
$mig$;
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DO $mig$
BEGIN
    DROP MATERIALIZED VIEW IF EXISTS mv_kpi_operasional;

    -- Rujukan lunak kembali ke ID lama (baris tanpa kode memakai id::text, yang sudah sama dengan nilai di audit).
    ALTER TABLE audit_log DISABLE TRIGGER trg_audit_log_append_only;
    UPDATE audit_log a SET entitas_id = c.kode
      FROM cold_storage c WHERE a.entitas = 'cold_storage' AND a.entitas_id = c.id::text AND c.kode IS NOT NULL;
    UPDATE audit_log a SET entitas_id = s.kode
      FROM stok_cold_storage s WHERE a.entitas = 'stok_cold_storage' AND a.entitas_id = s.id::text AND s.kode IS NOT NULL;
    UPDATE audit_log a SET entitas_id = o.kode
      FROM optimasi_rute o WHERE a.entitas = 'optimasi_rute' AND a.entitas_id = o.id::text AND o.kode IS NOT NULL;
    ALTER TABLE audit_log ENABLE TRIGGER trg_audit_log_append_only;

    -- ── wilayah_geometri dan jarak_antar_wilayah ────────────────────────────────────────────────────────────
    ALTER TABLE wilayah_geometri DROP CONSTRAINT wilayah_geometri_kabupaten_kode_fkey;
    ALTER TABLE wilayah_geometri
        ADD CONSTRAINT wilayah_geometri_kabupaten_kode_fkey FOREIGN KEY (kabupaten_kode) REFERENCES kabupaten(kode);

    DROP INDEX idx_jarak_antar_wilayah_tujuan;
    ALTER TABLE jarak_antar_wilayah DROP CONSTRAINT jarak_antar_wilayah_asal_kode_fkey;
    ALTER TABLE jarak_antar_wilayah DROP CONSTRAINT jarak_antar_wilayah_tujuan_kode_fkey;
    ALTER TABLE jarak_antar_wilayah
        ADD CONSTRAINT jarak_antar_wilayah_asal_kode_fkey FOREIGN KEY (asal_kode) REFERENCES kabupaten(kode),
        ADD CONSTRAINT jarak_antar_wilayah_tujuan_kode_fkey FOREIGN KEY (tujuan_kode) REFERENCES kabupaten(kode);

    -- ── neraca_wilayah: kembali ke PK komposit ──────────────────────────────────────────────────────────────
    DROP INDEX idx_neraca_wilayah_kabupaten_periode;
    ALTER TABLE neraca_wilayah DROP CONSTRAINT neraca_wilayah_komoditas_kode_fkey;
    ALTER TABLE neraca_wilayah DROP CONSTRAINT neraca_wilayah_kabupaten_kode_fkey;
    ALTER TABLE neraca_wilayah
        ADD CONSTRAINT neraca_wilayah_komoditas_kode_fkey FOREIGN KEY (komoditas_kode) REFERENCES komoditas(kode),
        ADD CONSTRAINT neraca_wilayah_kabupaten_kode_fkey FOREIGN KEY (kabupaten_kode) REFERENCES kabupaten(kode);
    ALTER TABLE neraca_wilayah DROP CONSTRAINT neraca_wilayah_pkey;
    ALTER TABLE neraca_wilayah DROP CONSTRAINT uq_neraca_wilayah_alami;
    ALTER TABLE neraca_wilayah DROP COLUMN id;
    ALTER TABLE neraca_wilayah ADD CONSTRAINT neraca_wilayah_pkey PRIMARY KEY (komoditas_kode, kabupaten_kode, periode);

    -- ── rute_distribusi dan optimasi_rute ───────────────────────────────────────────────────────────────────
    DROP INDEX idx_rute_distribusi_aktif;
    DROP INDEX idx_rute_distribusi_komoditas;
    DROP INDEX idx_rute_distribusi_asal;
    ALTER TABLE rute_distribusi DROP CONSTRAINT rute_distribusi_optimasi_id_fkey;
    ALTER TABLE rute_distribusi DROP CONSTRAINT rute_distribusi_asal_kode_fkey;
    ALTER TABLE rute_distribusi DROP CONSTRAINT rute_distribusi_tujuan_kode_fkey;
    ALTER TABLE rute_distribusi DROP CONSTRAINT rute_distribusi_komoditas_kode_fkey;

    CREATE FUNCTION pg_temp.g3_id_lama_optimasi(bigint) RETURNS varchar(64) LANGUAGE sql STABLE
        AS $f$ SELECT COALESCE(kode, id::text)::varchar(64) FROM public.optimasi_rute WHERE id = $1 $f$;
    ALTER TABLE rute_distribusi ALTER COLUMN optimasi_id TYPE varchar(64) USING pg_temp.g3_id_lama_optimasi(optimasi_id);
    DROP FUNCTION pg_temp.g3_id_lama_optimasi(bigint);

    ALTER TABLE rute_distribusi DROP CONSTRAINT uq_rute_distribusi_kode;
    ALTER TABLE rute_distribusi ALTER COLUMN id DROP IDENTITY;
    ALTER TABLE rute_distribusi ALTER COLUMN id TYPE varchar(255) USING COALESCE(kode, id::text);
    ALTER TABLE rute_distribusi DROP COLUMN kode;
    ALTER TABLE rute_distribusi
        ADD CONSTRAINT rute_distribusi_asal_kode_fkey FOREIGN KEY (asal_kode) REFERENCES kabupaten(kode),
        ADD CONSTRAINT rute_distribusi_tujuan_kode_fkey FOREIGN KEY (tujuan_kode) REFERENCES kabupaten(kode),
        ADD CONSTRAINT rute_distribusi_komoditas_kode_fkey FOREIGN KEY (komoditas_kode) REFERENCES komoditas(kode);

    DROP INDEX idx_optimasi_rute_komoditas_periode;
    ALTER TABLE optimasi_rute DROP CONSTRAINT optimasi_rute_komoditas_kode_fkey;
    ALTER TABLE optimasi_rute
        ADD CONSTRAINT optimasi_rute_komoditas_kode_fkey FOREIGN KEY (komoditas_kode) REFERENCES komoditas(kode);
    ALTER TABLE optimasi_rute DROP CONSTRAINT uq_optimasi_rute_kode;
    ALTER TABLE optimasi_rute ALTER COLUMN id DROP IDENTITY;
    ALTER TABLE optimasi_rute ALTER COLUMN id TYPE varchar(64) USING COALESCE(kode, id::text);
    ALTER TABLE optimasi_rute DROP COLUMN kode;
    ALTER TABLE rute_distribusi
        ADD CONSTRAINT rute_distribusi_optimasi_id_fkey FOREIGN KEY (optimasi_id) REFERENCES optimasi_rute(id);

    -- ── kandidat_cold_storage ───────────────────────────────────────────────────────────────────────────────
    DROP INDEX idx_kandidat_cold_storage_kabupaten_kode;
    ALTER TABLE kandidat_cold_storage DROP CONSTRAINT kandidat_cold_storage_kabupaten_kode_fkey;
    ALTER TABLE kandidat_cold_storage
        ADD CONSTRAINT kandidat_cold_storage_kabupaten_kode_fkey FOREIGN KEY (kabupaten_kode) REFERENCES kabupaten(kode);
    ALTER TABLE kandidat_cold_storage DROP CONSTRAINT uq_kandidat_cold_storage_kode;
    ALTER TABLE kandidat_cold_storage ALTER COLUMN id DROP IDENTITY;
    ALTER TABLE kandidat_cold_storage ALTER COLUMN id TYPE varchar(255) USING COALESCE(kode, id::text);
    ALTER TABLE kandidat_cold_storage DROP COLUMN kode;

    -- ── transaksi_stok dan stok_cold_storage ────────────────────────────────────────────────────────────────
    DROP INDEX idx_transaksi_stok_lot_baru;
    ALTER TABLE transaksi_stok DROP CONSTRAINT transaksi_stok_lot_id_fkey;
    ALTER TABLE transaksi_stok DROP CONSTRAINT transaksi_stok_lot_baru_id_fkey;
    ALTER TABLE transaksi_stok DROP CONSTRAINT chk_transaksi_stok_lokasi;
    ALTER TABLE transaksi_stok DROP CONSTRAINT transaksi_stok_fasilitas_asal_id_fkey;
    ALTER TABLE transaksi_stok DROP CONSTRAINT transaksi_stok_fasilitas_tujuan_id_fkey;
    ALTER TABLE stok_cold_storage DROP CONSTRAINT stok_cold_storage_cold_storage_id_fkey;
    ALTER TABLE telemetri_suhu DROP CONSTRAINT telemetri_suhu_fasilitas_id_fkey;
    ALTER TABLE stok_cold_storage DROP CONSTRAINT stok_cold_storage_komoditas_kode_fkey;
    DROP INDEX idx_stok_cold_storage_aktif;
    DROP INDEX idx_stok_cold_storage_komoditas_kode;
    DROP INDEX idx_telemetri_suhu_waktu;

    CREATE FUNCTION pg_temp.g3_id_lama_stok(bigint) RETURNS varchar(255) LANGUAGE sql STABLE
        AS $f$ SELECT COALESCE(kode, id::text)::varchar(255) FROM public.stok_cold_storage WHERE id = $1 $f$;
    ALTER TABLE transaksi_stok ALTER COLUMN lot_id TYPE varchar(255) USING pg_temp.g3_id_lama_stok(lot_id);
    ALTER TABLE transaksi_stok ALTER COLUMN lot_baru_id TYPE varchar(255) USING pg_temp.g3_id_lama_stok(lot_baru_id);
    DROP FUNCTION pg_temp.g3_id_lama_stok(bigint);

    ALTER TABLE transaksi_stok DROP CONSTRAINT uq_transaksi_stok_kode;
    ALTER TABLE transaksi_stok ALTER COLUMN id DROP IDENTITY;
    ALTER TABLE transaksi_stok ALTER COLUMN id TYPE varchar(64) USING COALESCE(kode, id::text);
    ALTER TABLE transaksi_stok DROP COLUMN kode;

    ALTER TABLE stok_cold_storage DROP CONSTRAINT uq_stok_cold_storage_kode;
    ALTER TABLE stok_cold_storage ALTER COLUMN id DROP IDENTITY;
    ALTER TABLE stok_cold_storage ALTER COLUMN id TYPE varchar(255) USING COALESCE(kode, id::text);
    ALTER TABLE stok_cold_storage DROP COLUMN kode;
    ALTER TABLE stok_cold_storage
        ADD CONSTRAINT stok_cold_storage_komoditas_kode_fkey FOREIGN KEY (komoditas_kode) REFERENCES komoditas(kode);

    -- ── cold_storage: UUID -> VARCHAR ───────────────────────────────────────────────────────────────────────
    CREATE FUNCTION pg_temp.g3_id_lama_fasilitas(uuid) RETURNS varchar(255) LANGUAGE sql STABLE
        AS $f$ SELECT COALESCE(kode, id::text)::varchar(255) FROM public.cold_storage WHERE id = $1 $f$;
    ALTER TABLE stok_cold_storage ALTER COLUMN cold_storage_id TYPE varchar(255) USING pg_temp.g3_id_lama_fasilitas(cold_storage_id);
    ALTER TABLE telemetri_suhu ALTER COLUMN fasilitas_id TYPE varchar(255) USING pg_temp.g3_id_lama_fasilitas(fasilitas_id);
    ALTER TABLE transaksi_stok ALTER COLUMN fasilitas_asal_id TYPE varchar(255) USING pg_temp.g3_id_lama_fasilitas(fasilitas_asal_id);
    ALTER TABLE transaksi_stok ALTER COLUMN fasilitas_tujuan_id TYPE varchar(255) USING pg_temp.g3_id_lama_fasilitas(fasilitas_tujuan_id);
    DROP FUNCTION pg_temp.g3_id_lama_fasilitas(uuid);

    ALTER TABLE cold_storage DROP CONSTRAINT uq_cold_storage_kode;
    ALTER TABLE cold_storage ALTER COLUMN id DROP DEFAULT;
    ALTER TABLE cold_storage ALTER COLUMN id TYPE varchar(255) USING COALESCE(kode, id::text);
    ALTER TABLE cold_storage DROP COLUMN kode;
    DROP INDEX idx_cold_storage_kabupaten_kode;
    ALTER TABLE cold_storage DROP CONSTRAINT cold_storage_kabupaten_kode_fkey;
    ALTER TABLE cold_storage
        ADD CONSTRAINT cold_storage_kabupaten_kode_fkey FOREIGN KEY (kabupaten_kode) REFERENCES kabupaten(kode);

    ALTER TABLE stok_cold_storage
        ADD CONSTRAINT stok_cold_storage_cold_storage_id_fkey FOREIGN KEY (cold_storage_id) REFERENCES cold_storage(id) ON DELETE CASCADE;
    ALTER TABLE telemetri_suhu
        ADD CONSTRAINT telemetri_suhu_fasilitas_id_fkey FOREIGN KEY (fasilitas_id) REFERENCES cold_storage(id) ON DELETE CASCADE;
    ALTER TABLE transaksi_stok
        ADD CONSTRAINT transaksi_stok_lot_id_fkey FOREIGN KEY (lot_id) REFERENCES stok_cold_storage(id),
        ADD CONSTRAINT transaksi_stok_lot_baru_id_fkey FOREIGN KEY (lot_baru_id) REFERENCES stok_cold_storage(id),
        ADD CONSTRAINT transaksi_stok_fasilitas_asal_id_fkey FOREIGN KEY (fasilitas_asal_id) REFERENCES cold_storage(id),
        ADD CONSTRAINT transaksi_stok_fasilitas_tujuan_id_fkey FOREIGN KEY (fasilitas_tujuan_id) REFERENCES cold_storage(id),
        ADD CONSTRAINT chk_transaksi_stok_lokasi CHECK (
            (jenis = 'MASUK' AND fasilitas_tujuan_id IS NOT NULL AND fasilitas_asal_id IS NULL)
            OR (jenis = 'KELUAR' AND fasilitas_asal_id IS NOT NULL AND fasilitas_tujuan_id IS NULL)
            OR (jenis = 'PINDAH' AND fasilitas_asal_id IS NOT NULL AND fasilitas_tujuan_id IS NOT NULL
                AND fasilitas_asal_id <> fasilitas_tujuan_id)
        );

    -- ── View dibuat ulang (definisi 000022) ─────────────────────────────────────────────────────────────────
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
           COALESCE((SELECT 100.0 * COUNT(*) FILTER (WHERE e.status IN ('DIVALIDASI', 'DIPUBLIKASI')) / NULLIF(COUNT(*), 0)
                       FROM entri e WHERE e.tanggal BETWEEN h.tanggal - 6 AND h.tanggal), 0)::double precision
      FROM hari h
    UNION ALL
    SELECT 'waspada', h.tanggal,
           (SELECT COUNT(DISTINCT x.komoditas_kode) FROM harian x
             WHERE x.tanggal = h.tanggal AND x.basis IS NOT NULL AND x.rata_rata > x.basis * 1.07)::double precision
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
END
$mig$;
-- +goose StatementEnd
