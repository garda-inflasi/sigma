-- 12-B: geometri wilayah sebagai GeoJSON (jsonb) tanpa ekstensi PostGIS.
-- Keputusan: image Postgres bersama tetap postgres:16-alpine (demo Docker berjalan
-- dan tes memakai container yang sama); jarak dan titik-di-dalam-poligon dihitung di
-- aplikasi (internal/spasial/domain/geo.go). Kotak pembatas disimpan agar penyaringan
-- viewport tidak perlu mengurai GeoJSON. Tanpa data demo (seeder spasial.demo).

-- +goose Up
-- +goose StatementBegin
CREATE TABLE IF NOT EXISTS wilayah_geometri (
    kabupaten_kode VARCHAR(4) PRIMARY KEY REFERENCES kabupaten(kode),
    geojson JSONB NOT NULL,
    min_lat DOUBLE PRECISION NOT NULL,
    min_lng DOUBLE PRECISION NOT NULL,
    max_lat DOUBLE PRECISION NOT NULL,
    max_lng DOUBLE PRECISION NOT NULL,
    -- DEMO: poligon perkiraan, bukan batas administratif. RESMI: hasil impor batas resmi (BPS/Geoportal).
    sumber VARCHAR(10) NOT NULL DEFAULT 'DEMO',
    keterangan TEXT NOT NULL DEFAULT '',
    diperbarui_pada TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CONSTRAINT chk_wilayah_geometri_tipe CHECK (
        geojson->>'type' IN ('Polygon', 'MultiPolygon')
        AND jsonb_typeof(geojson->'coordinates') = 'array'
        AND jsonb_array_length(geojson->'coordinates') > 0
    ),
    CONSTRAINT chk_wilayah_geometri_bbox CHECK (
        min_lat < max_lat AND min_lng < max_lng
        AND min_lat >= -90 AND max_lat <= 90 AND min_lng >= -180 AND max_lng <= 180
    ),
    CONSTRAINT chk_wilayah_geometri_sumber CHECK (sumber IN ('DEMO', 'RESMI'))
);
CREATE INDEX IF NOT EXISTS idx_wilayah_geometri_bbox ON wilayah_geometri (min_lng, max_lng, min_lat, max_lat);
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DROP TABLE IF EXISTS wilayah_geometri;
-- +goose StatementEnd
