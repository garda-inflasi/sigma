-- +goose Up
-- +goose StatementBegin
CREATE TABLE IF NOT EXISTS perubahan_master (
    id VARCHAR(255) PRIMARY KEY,
    entitas VARCHAR(100) NOT NULL,
    entitas_id VARCHAR(255) NOT NULL,
    versi INT NOT NULL,
    perubahan JSONB NOT NULL,
    status status_persetujuan NOT NULL DEFAULT 'MENUNGGU',
    diajukan_oleh_id VARCHAR(255) NOT NULL REFERENCES pengguna(id),
    disetujui_oleh_id VARCHAR(255) REFERENCES pengguna(id),
    dibuat_pada TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
CREATE INDEX IF NOT EXISTS idx_perubahan_master_entitas ON perubahan_master(entitas, entitas_id);

CREATE TABLE IF NOT EXISTS entri_harga (
    id VARCHAR(255) PRIMARY KEY,
    komoditas_kode VARCHAR(255) NOT NULL REFERENCES komoditas(kode),
    pasar_id VARCHAR(255) NOT NULL REFERENCES pasar(id),
    tipe tipe_harga NOT NULL,
    harga INT NOT NULL,
    satuan VARCHAR(50) NOT NULL,
    kualitas kualitas NOT NULL DEFAULT 'MEDIUM',
    sumber sumber_harga NOT NULL DEFAULT 'LAPANGAN',
    status status_entri NOT NULL DEFAULT 'DIAJUKAN',
    deviasi_pct FLOAT NOT NULL DEFAULT 0,
    skor_anomali FLOAT NOT NULL DEFAULT 0,
    lat FLOAT,
    lng FLOAT,
    dalam_geofence BOOLEAN NOT NULL DEFAULT true,
    foto_url VARCHAR(500),
    kode_pedagang VARCHAR(100),
    catatan TEXT,
    idempotency_key VARCHAR(255) UNIQUE,
    dicatat_pada TIMESTAMPTZ NOT NULL,
    petugas_id VARCHAR(255) REFERENCES pengguna(id),
    divalidasi_oleh VARCHAR(255) REFERENCES pengguna(id),
    divalidasi_pada TIMESTAMPTZ,
    dipublikasi_oleh VARCHAR(255) REFERENCES pengguna(id),
    dipublikasi_pada TIMESTAMPTZ
);
CREATE INDEX IF NOT EXISTS idx_entri_harga_komoditas_pasar ON entri_harga(komoditas_kode, pasar_id, dicatat_pada);
CREATE INDEX IF NOT EXISTS idx_entri_harga_status ON entri_harga(status, dicatat_pada);

CREATE TABLE IF NOT EXISTS cuaca_harian (
    id VARCHAR(255) PRIMARY KEY,
    kabupaten_kode VARCHAR(4) NOT NULL REFERENCES kabupaten(kode),
    tanggal DATE NOT NULL,
    curah_hujan_mm FLOAT NOT NULL,
    suhu_c FLOAT NOT NULL,
    kelembapan INT NOT NULL,
    anomali BOOLEAN NOT NULL DEFAULT false,
    sumber VARCHAR(50) NOT NULL DEFAULT 'BMKG',
    UNIQUE(kabupaten_kode, tanggal, sumber)
);

CREATE TABLE IF NOT EXISTS sumber_data (
    id VARCHAR(255) PRIMARY KEY,
    nama VARCHAR(255) NOT NULL,
    penyedia VARCHAR(255) NOT NULL,
    jenis jenis_sumber NOT NULL,
    cakupan VARCHAR(255) NOT NULL,
    jadwal_cron VARCHAR(100) NOT NULL,
    status status_sumber NOT NULL DEFAULT 'SEHAT',
    terakhir_jalan TIMESTAMPTZ,
    keberhasilan_7h FLOAT NOT NULL DEFAULT 100,
    latensi_ms INT NOT NULL DEFAULT 0,
    robots_txt_patuh BOOLEAN NOT NULL DEFAULT true,
    atribusi TEXT NOT NULL
);

CREATE TABLE IF NOT EXISTS aturan_ews (
    id VARCHAR(255) PRIMARY KEY,
    komoditas_kode VARCHAR(255) REFERENCES komoditas(kode),
    jenis jenis_ews NOT NULL,
    ambang_waspada FLOAT NOT NULL,
    ambang_siaga FLOAT NOT NULL,
    ambang_kritis FLOAT NOT NULL,
    jendela_hari INT NOT NULL DEFAULT 7,
    kanal TEXT[] NOT NULL DEFAULT ARRAY['DASHBOARD', 'PUSH'],
    aktif BOOLEAN NOT NULL DEFAULT true
);

CREATE TABLE IF NOT EXISTS ews_alert (
    id VARCHAR(255) PRIMARY KEY,
    severity severity NOT NULL,
    jenis jenis_ews NOT NULL,
    judul VARCHAR(255) NOT NULL,
    pesan TEXT NOT NULL,
    komoditas_kode VARCHAR(255) REFERENCES komoditas(kode),
    kabupaten_kode VARCHAR(4) NOT NULL REFERENCES kabupaten(kode),
    nilai FLOAT NOT NULL,
    satuan_nilai VARCHAR(50) NOT NULL,
    skor_dampak INT NOT NULL,
    status status_ews NOT NULL DEFAULT 'BARU',
    sumber VARCHAR(255) NOT NULL,
    dibuat_pada TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    diakui_oleh_id VARCHAR(255) REFERENCES pengguna(id),
    diakui_pada TIMESTAMPTZ
);
CREATE INDEX IF NOT EXISTS idx_ews_status_severity ON ews_alert(status, severity);

CREATE TABLE IF NOT EXISTS model_registry (
    id VARCHAR(255) PRIMARY KEY,
    komoditas_kode VARCHAR(255) NOT NULL REFERENCES komoditas(kode),
    nama VARCHAR(255) NOT NULL,
    versi VARCHAR(50) NOT NULL,
    mape FLOAT NOT NULL,
    drift FLOAT NOT NULL DEFAULT 0,
    status status_model NOT NULL DEFAULT 'KANDIDAT',
    dilatih_pada TIMESTAMPTZ NOT NULL,
    UNIQUE(komoditas_kode, versi)
);

CREATE TABLE IF NOT EXISTS prediksi_harga (
    id VARCHAR(255) PRIMARY KEY,
    komoditas_kode VARCHAR(255) NOT NULL REFERENCES komoditas(kode),
    kabupaten_kode VARCHAR(4) NOT NULL REFERENCES kabupaten(kode),
    tanggal_target DATE NOT NULL,
    horizon_hari INT NOT NULL,
    nilai INT NOT NULL,
    bawah INT NOT NULL,
    atas INT NOT NULL,
    model_versi VARCHAR(50) NOT NULL,
    faktor JSONB NOT NULL,
    dibuat_pada TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
CREATE INDEX IF NOT EXISTS idx_prediksi_harga_lookup ON prediksi_harga(komoditas_kode, kabupaten_kode, tanggal_target);

CREATE TABLE IF NOT EXISTS cold_storage (
    id VARCHAR(255) PRIMARY KEY,
    nama VARCHAR(255) NOT NULL,
    kabupaten_kode VARCHAR(4) NOT NULL REFERENCES kabupaten(kode),
    lat FLOAT NOT NULL,
    lng FLOAT NOT NULL,
    kapasitas_ton INT NOT NULL,
    suhu_target_c FLOAT NOT NULL,
    status status_fasilitas NOT NULL
);

CREATE TABLE IF NOT EXISTS stok_cold_storage (
    id VARCHAR(255) PRIMARY KEY,
    cold_storage_id VARCHAR(255) NOT NULL REFERENCES cold_storage(id) ON DELETE CASCADE,
    komoditas_kode VARCHAR(255) NOT NULL REFERENCES komoditas(kode),
    volume_ton FLOAT NOT NULL,
    masuk_pada TIMESTAMPTZ NOT NULL,
    pemilik VARCHAR(255) NOT NULL,
    risiko_food_loss FLOAT NOT NULL
);

CREATE TABLE IF NOT EXISTS ruang_fiskal (
    id VARCHAR(255) PRIMARY KEY,
    tahun INT NOT NULL,
    instrumen VARCHAR(255) NOT NULL,
    pagu BIGINT NOT NULL,
    realisasi BIGINT NOT NULL,
    per_tanggal DATE NOT NULL,
    UNIQUE(tahun, instrumen, per_tanggal)
);

CREATE TABLE IF NOT EXISTS policy_brief (
    id VARCHAR(255) PRIMARY KEY,
    periode VARCHAR(50) NOT NULL,
    judul VARCHAR(255) NOT NULL,
    status status_brief NOT NULL DEFAULT 'DRAF',
    ringkasan TEXT NOT NULL,
    rekomendasi JSONB NOT NULL,
    halaman INT NOT NULL,
    disusun_oleh_id VARCHAR(255) NOT NULL REFERENCES pengguna(id),
    diperbarui TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE TABLE IF NOT EXISTS intervensi (
    id VARCHAR(255) PRIMARY KEY,
    siklus siklus NOT NULL,
    tahap VARCHAR(100) NOT NULL,
    urutan INT NOT NULL DEFAULT 0,
    judul VARCHAR(255) NOT NULL,
    komoditas_kode VARCHAR(255) NOT NULL REFERENCES komoditas(kode),
    kabupaten_kode VARCHAR(4) NOT NULL REFERENCES kabupaten(kode),
    prioritas prioritas NOT NULL,
    tenggat TIMESTAMPTZ NOT NULL,
    rekomendasi_ai BOOLEAN NOT NULL DEFAULT false,
    checklist_selesai INT NOT NULL DEFAULT 0,
    checklist_total INT NOT NULL DEFAULT 0,
    ews_alert_id VARCHAR(255) REFERENCES ews_alert(id),
    dibuat_oleh_id VARCHAR(255) NOT NULL REFERENCES pengguna(id),
    dibuat_pada TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
CREATE INDEX IF NOT EXISTS idx_intervensi_siklus ON intervensi(siklus, tahap, urutan);

CREATE TABLE IF NOT EXISTS intervensi_pj (
    intervensi_id VARCHAR(255) NOT NULL REFERENCES intervensi(id) ON DELETE CASCADE,
    pengguna_id VARCHAR(255) NOT NULL REFERENCES pengguna(id) ON DELETE CASCADE,
    PRIMARY KEY (intervensi_id, pengguna_id)
);

CREATE TABLE IF NOT EXISTS tugas_lapangan (
    id VARCHAR(255) PRIMARY KEY,
    judul VARCHAR(255) NOT NULL,
    pasar_id VARCHAR(255) NOT NULL REFERENCES pasar(id),
    petugas_id VARCHAR(255) NOT NULL REFERENCES pengguna(id),
    komoditas TEXT[] NOT NULL,
    tipe tipe_harga NOT NULL,
    tenggat TIMESTAMPTZ NOT NULL,
    status status_tugas NOT NULL DEFAULT 'BELUM',
    target INT NOT NULL
);
CREATE INDEX IF NOT EXISTS idx_tugas_lapangan_petugas ON tugas_lapangan(petugas_id, status);

CREATE TABLE IF NOT EXISTS audit_log (
    id VARCHAR(255) PRIMARY KEY,
    waktu TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    aktor_id VARCHAR(255) REFERENCES pengguna(id),
    aksi VARCHAR(100) NOT NULL,
    entitas VARCHAR(100) NOT NULL,
    entitas_id VARCHAR(255) NOT NULL,
    hasil VARCHAR(20) NOT NULL,
    ip VARCHAR(45),
    metadata JSONB
);
CREATE INDEX IF NOT EXISTS idx_audit_log_entitas ON audit_log(entitas, entitas_id);
CREATE INDEX IF NOT EXISTS idx_audit_log_waktu ON audit_log(waktu);
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DROP TABLE IF EXISTS audit_log;
DROP TABLE IF EXISTS tugas_lapangan;
DROP TABLE IF EXISTS intervensi_pj;
DROP TABLE IF EXISTS intervensi;
DROP TABLE IF EXISTS policy_brief;
DROP TABLE IF EXISTS ruang_fiskal;
DROP TABLE IF EXISTS stok_cold_storage;
DROP TABLE IF EXISTS cold_storage;
DROP TABLE IF EXISTS prediksi_harga;
DROP TABLE IF EXISTS model_registry;
DROP TABLE IF EXISTS ews_alert;
DROP TABLE IF EXISTS aturan_ews;
DROP TABLE IF EXISTS sumber_data;
DROP TABLE IF EXISTS cuaca_harian;
DROP TABLE IF EXISTS entri_harga;
DROP TABLE IF EXISTS perubahan_master;
-- +goose StatementEnd