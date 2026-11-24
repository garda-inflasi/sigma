-- Medallion Architecture operational tables for ingestion idempotency (Bronze),
-- canonical cross-source region dimension (Silver), and model serving outputs (Gold/Serving).

-- +goose Up
-- +goose StatementBegin
CREATE TABLE IF NOT EXISTS crawl_logs (
    id              BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    source_code     VARCHAR(32) NOT NULL,
    dataset_code    VARCHAR(80) NOT NULL,
    endpoint        TEXT NOT NULL,
    params_hash     CHAR(64) NOT NULL,
    params_json     JSONB NOT NULL DEFAULT '{}'::jsonb,
    payload_sha256  CHAR(64) NOT NULL,
    bronze_uri      TEXT NOT NULL DEFAULT '',
    status          VARCHAR(24) NOT NULL CHECK (status IN ('WRITTEN', 'SKIPPED_DUPLICATE', 'ERROR')),
    byte_size       INTEGER NOT NULL DEFAULT 0 CHECK (byte_size >= 0),
    error_message   TEXT NOT NULL DEFAULT '',
    crawled_at      TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX IF NOT EXISTS idx_crawl_logs_lookup
    ON crawl_logs (source_code, dataset_code, params_hash, crawled_at DESC);
CREATE INDEX IF NOT EXISTS idx_crawl_logs_sha256
    ON crawl_logs (payload_sha256);

CREATE TABLE IF NOT EXISTS dim_wilayah (
    kode_bps        VARCHAR(10) PRIMARY KEY,
    nama_resmi      VARCHAR(160) NOT NULL,
    tingkat         VARCHAR(20) NOT NULL CHECK (tingkat IN ('NASIONAL', 'PROVINSI', 'KABUPATEN', 'KOTA')),
    kode_djpk_prov  VARCHAR(8) NULL,
    kode_djpk_pemda VARCHAR(8) NULL,
    kode_pihps      INTEGER NULL,
    kode_kemendag   VARCHAR(32) NULL,
    kode_bmkg_adm4  VARCHAR(24) NULL,
    lat             DOUBLE PRECISION NULL,
    lng             DOUBLE PRECISION NULL,
    diperbarui_pada TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE UNIQUE INDEX IF NOT EXISTS idx_dim_wilayah_djpk
    ON dim_wilayah (kode_djpk_prov, kode_djpk_pemda)
    WHERE kode_djpk_prov IS NOT NULL AND kode_djpk_pemda IS NOT NULL;
CREATE INDEX IF NOT EXISTS idx_dim_wilayah_pihps ON dim_wilayah (kode_pihps) WHERE kode_pihps IS NOT NULL;
CREATE INDEX IF NOT EXISTS idx_dim_wilayah_bmkg ON dim_wilayah (kode_bmkg_adm4) WHERE kode_bmkg_adm4 IS NOT NULL;

INSERT INTO dim_wilayah (
    kode_bps, nama_resmi, tingkat, kode_djpk_prov, kode_djpk_pemda,
    kode_pihps, kode_kemendag, kode_bmkg_adm4, lat, lng
) VALUES
    ('0000', 'Indonesia', 'NASIONAL', '00', '00', NULL, '0000', NULL, -2.50, 118.00),
    ('11',   'Provinsi Aceh', 'PROVINSI', '01', '00', NULL, '11', NULL, 4.6951, 96.7494),
    ('1101', 'Kab. Simeulue', 'KABUPATEN', '01', '12', 491, '1101', '11.09.02.2001', 2.48, 96.36),
    ('1102', 'Kab. Aceh Singkil', 'KABUPATEN', '01', '10', 493, '1102', '11.10.04.2001', 2.28, 97.79),
    ('1103', 'Kab. Aceh Selatan', 'KABUPATEN', '01', '02', 494, '1103', '11.01.08.2001', 3.25, 97.18),
    ('1104', 'Kab. Aceh Tenggara', 'KABUPATEN', '01', '03', 478, '1104', '11.02.04.2001', 3.49, 97.81),
    ('1105', 'Kab. Aceh Timur', 'KABUPATEN', '01', '04', 479, '1105', '11.03.03.2001', 4.96, 97.76),
    ('1106', 'Kab. Aceh Tengah', 'KABUPATEN', '01', '05', 484, '1106', '11.04.17.2001', 4.63, 96.84),
    ('1107', 'Kab. Aceh Barat', 'KABUPATEN', '01', '01', 3,   '1107', '11.05.01.2001', 4.14, 96.13),
    ('1108', 'Kab. Aceh Besar', 'KABUPATEN', '01', '06', 495, '1108', '11.06.03.2001', 5.38, 95.52),
    ('1109', 'Kab. Pidie', 'KABUPATEN', '01', '07', 489, '1109', '11.07.16.2001', 5.38, 95.96),
    ('1110', 'Kab. Bireuen', 'KABUPATEN', '01', '11', 483, '1110', '11.11.13.2001', 5.20, 96.70),
    ('1111', 'Kab. Aceh Utara', 'KABUPATEN', '01', '08', 485, '1111', '11.08.02.2001', 5.14, 97.27),
    ('1112', 'Kab. Aceh Barat Daya', 'KABUPATEN', '01', '13', 477, '1112', '11.12.01.2001', 3.74, 96.84),
    ('1113', 'Kab. Gayo Lues', 'KABUPATEN', '01', '14', 492, '1113', '11.13.01.2001', 3.98, 97.35),
    ('1114', 'Kab. Aceh Tamiang', 'KABUPATEN', '01', '17', 488, '1114', '11.16.05.2001', 4.29, 98.06),
    ('1115', 'Kab. Nagan Raya', 'KABUPATEN', '01', '16', 487, '1115', '11.15.01.2001', 4.18, 96.38),
    ('1116', 'Kab. Aceh Jaya', 'KABUPATEN', '01', '15', 482, '1116', '11.14.01.2001', 4.65, 95.59),
    ('1117', 'Kab. Bener Meriah', 'KABUPATEN', '01', '18', 476, '1117', '11.17.05.2001', 4.73, 96.87),
    ('1118', 'Kab. Pidie Jaya', 'KABUPATEN', '01', '19', 481, '1118', '11.18.01.2001', 5.22, 96.25),
    ('1171', 'Kota Banda Aceh', 'KOTA', '01', '09', 1,   '1171', '11.71.01.2001', 5.55, 95.32),
    ('1172', 'Kota Sabang', 'KOTA', '01', '51', 490, '1172', '11.72.01.2001', 5.89, 95.32),
    ('1173', 'Kota Langsa', 'KOTA', '01', '53', 480, '1173', '11.74.03.2001', 4.47, 97.96),
    ('1174', 'Kota Lhokseumawe', 'KOTA', '01', '52', 2,   '1174', '11.73.01.2002', 5.18, 97.15),
    ('1175', 'Kota Subulussalam', 'KOTA', '01', '54', 486, '1175', '11.75.01.2001', 2.64, 98.00)
ON CONFLICT (kode_bps) DO NOTHING;

CREATE TABLE IF NOT EXISTS ml_predictions (
    id              BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    cycle_id        VARCHAR(40) NOT NULL,
    kode_bps        VARCHAR(10) NOT NULL REFERENCES dim_wilayah(kode_bps) ON DELETE RESTRICT,
    komoditas_kode  VARCHAR(32) NOT NULL,
    periode         DATE NOT NULL,
    horizon_hari    INTEGER NOT NULL DEFAULT 7 CHECK (horizon_hari >= 0),
    task_type       VARCHAR(32) NOT NULL CHECK (task_type IN ('FORECAST', 'RISK_CLASSIFICATION')),
    yhat            DOUBLE PRECISION NULL,
    yhat_lower      DOUBLE PRECISION NULL,
    yhat_upper      DOUBLE PRECISION NULL,
    risk_prob       DOUBLE PRECISION NULL,
    risk_level      VARCHAR(24) NULL,
    model_family    VARCHAR(48) NOT NULL,
    model_version   VARCHAR(80) NOT NULL,
    parquet_version VARCHAR(64) NOT NULL DEFAULT '',
    created_at      TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX IF NOT EXISTS idx_ml_predictions_serving
    ON ml_predictions (kode_bps, komoditas_kode, periode DESC, horizon_hari);
CREATE INDEX IF NOT EXISTS idx_ml_predictions_cycle
    ON ml_predictions (cycle_id, created_at DESC);
CREATE INDEX IF NOT EXISTS idx_ml_predictions_version
    ON ml_predictions (model_version, created_at DESC);

CREATE TABLE IF NOT EXISTS region_clusters (
    id                BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    cycle_id          VARCHAR(40) NOT NULL,
    kode_bps          VARCHAR(10) NOT NULL REFERENCES dim_wilayah(kode_bps) ON DELETE RESTRICT,
    periode           DATE NOT NULL,
    cluster_id        INTEGER NOT NULL,
    cluster_label     VARCHAR(64) NOT NULL,
    silhouette_score  DOUBLE PRECISION NULL,
    centroid_distance DOUBLE PRECISION NOT NULL,
    feature_summary   JSONB NOT NULL DEFAULT '{}'::jsonb,
    model_version     VARCHAR(80) NOT NULL,
    created_at        TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX IF NOT EXISTS idx_region_clusters_serving
    ON region_clusters (kode_bps, periode DESC, model_version);
CREATE INDEX IF NOT EXISTS idx_region_clusters_cycle
    ON region_clusters (cycle_id, cluster_id);

DO $$
BEGIN
    IF EXISTS (SELECT 1 FROM pg_roles WHERE rolname = 'garda_aplikasi') THEN
        GRANT SELECT, INSERT, UPDATE, DELETE ON
            public.crawl_logs,
            public.dim_wilayah,
            public.ml_predictions,
            public.region_clusters
        TO garda_aplikasi;
        GRANT USAGE, SELECT ON ALL SEQUENCES IN SCHEMA public TO garda_aplikasi;
    END IF;
END $$;
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DROP TABLE IF EXISTS region_clusters;
DROP TABLE IF EXISTS ml_predictions;
DROP TABLE IF EXISTS dim_wilayah;
DROP TABLE IF EXISTS crawl_logs;
-- +goose StatementEnd
