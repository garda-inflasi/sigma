-- Slice 10-D (docs/todos/10-ews.md): notifikasi multi-kanal.
-- - ews_notifikasi: outbox pengiriman (satu baris per alert x pengguna x kanal; UNIQUE membuat
--   antrean idempoten). Status MENUNGGU -> TERKIRIM | GAGAL (habis percobaan) | DICATAT
--   (kanal tanpa adaptor terkonfigurasi atau tanpa tujuan: hanya tercatat, tidak dikirim);
-- - preferensi_notifikasi: kanal dan tingkat minimum per pengguna;
-- - langganan_push: langganan Web Push (PWA) per perangkat.

-- +goose Up
-- +goose StatementBegin
CREATE TABLE IF NOT EXISTS ews_notifikasi (
    id BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    alert_id VARCHAR(255) NOT NULL REFERENCES ews_alert(id) ON DELETE CASCADE,
    pengguna_id VARCHAR(255) NOT NULL REFERENCES pengguna(id) ON DELETE CASCADE,
    kanal VARCHAR(20) NOT NULL,
    tujuan VARCHAR(255) NOT NULL,
    status VARCHAR(20) NOT NULL DEFAULT 'MENUNGGU',
    percobaan INT NOT NULL DEFAULT 0,
    jadwal_berikut TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    dikirim_pada TIMESTAMPTZ,
    galat TEXT,
    dibuat_pada TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CONSTRAINT uq_ews_notifikasi UNIQUE (alert_id, pengguna_id, kanal),
    CONSTRAINT chk_ews_notifikasi_kanal CHECK (kanal IN ('EMAIL', 'PUSH', 'WHATSAPP')),
    CONSTRAINT chk_ews_notifikasi_status CHECK (status IN ('MENUNGGU', 'TERKIRIM', 'GAGAL', 'DICATAT')),
    CONSTRAINT chk_ews_notifikasi_percobaan CHECK (percobaan >= 0)
);
CREATE INDEX IF NOT EXISTS idx_ews_notifikasi_jatuh_tempo ON ews_notifikasi (jadwal_berikut) WHERE status = 'MENUNGGU';
CREATE INDEX IF NOT EXISTS idx_ews_notifikasi_alert ON ews_notifikasi (alert_id);

CREATE TABLE IF NOT EXISTS preferensi_notifikasi (
    pengguna_id VARCHAR(255) NOT NULL REFERENCES pengguna(id) ON DELETE CASCADE,
    kanal VARCHAR(20) NOT NULL,
    severity_min severity NOT NULL DEFAULT 'SIAGA',
    aktif BOOLEAN NOT NULL DEFAULT true,
    diubah_pada TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    PRIMARY KEY (pengguna_id, kanal),
    CONSTRAINT chk_preferensi_notifikasi_kanal CHECK (kanal IN ('EMAIL', 'PUSH', 'WHATSAPP'))
);

CREATE TABLE IF NOT EXISTS langganan_push (
    id BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    pengguna_id VARCHAR(255) NOT NULL REFERENCES pengguna(id) ON DELETE CASCADE,
    endpoint TEXT NOT NULL UNIQUE,
    p256dh TEXT NOT NULL,
    auth TEXT NOT NULL,
    dibuat_pada TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
CREATE INDEX IF NOT EXISTS idx_langganan_push_pengguna ON langganan_push (pengguna_id);
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DROP TABLE IF EXISTS langganan_push;
DROP TABLE IF EXISTS preferensi_notifikasi;
DROP TABLE IF EXISTS ews_notifikasi;
-- +goose StatementEnd
