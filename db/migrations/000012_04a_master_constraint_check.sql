-- Slice 04-A (docs/todos/04-master-data.md): invarian murah pada master data
-- dijaga di basis data, bukan hanya di aplikasi.

-- +goose Up
-- +goose StatementBegin
DO $$
DECLARE
    aturan CONSTANT text[][] := ARRAY[
        ['kabupaten', 'chk_kabupaten_lat',         'lat BETWEEN -90 AND 90'],
        ['kabupaten', 'chk_kabupaten_lng',         'lng BETWEEN -180 AND 180'],
        ['pasar',     'chk_pasar_lat',             'lat BETWEEN -90 AND 90'],
        ['pasar',     'chk_pasar_lng',             'lng BETWEEN -180 AND 180'],
        ['pasar',     'chk_pasar_radius_meter',    'radius_meter > 0'],
        ['komoditas', 'chk_komoditas_harga_acuan', 'harga_acuan > 0'],
        ['komoditas', 'chk_komoditas_shelf_life',  'shelf_life_hari > 0']
    ];
    i int;
BEGIN
    FOR i IN 1 .. array_length(aturan, 1) LOOP
        IF NOT EXISTS (SELECT 1 FROM pg_constraint WHERE conname = aturan[i][2]) THEN
            EXECUTE format('ALTER TABLE %I ADD CONSTRAINT %I CHECK (%s)', aturan[i][1], aturan[i][2], aturan[i][3]);
        END IF;
    END LOOP;
END $$;
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
ALTER TABLE komoditas DROP CONSTRAINT IF EXISTS chk_komoditas_shelf_life;
ALTER TABLE komoditas DROP CONSTRAINT IF EXISTS chk_komoditas_harga_acuan;
ALTER TABLE pasar DROP CONSTRAINT IF EXISTS chk_pasar_radius_meter;
ALTER TABLE pasar DROP CONSTRAINT IF EXISTS chk_pasar_lng;
ALTER TABLE pasar DROP CONSTRAINT IF EXISTS chk_pasar_lat;
ALTER TABLE kabupaten DROP CONSTRAINT IF EXISTS chk_kabupaten_lng;
ALTER TABLE kabupaten DROP CONSTRAINT IF EXISTS chk_kabupaten_lat;
-- +goose StatementEnd
