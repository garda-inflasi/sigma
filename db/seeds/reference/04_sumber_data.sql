-- GARDA INFLASI - Katalog Sumber Data Eksternal (API & Scraper)

-- =====================================================================
-- Tabel: sumber_data (8 baris)
-- =====================================================================
INSERT INTO sumber_data (id, kode, nama, penyedia, jenis, cakupan, jadwal_cron, robots_txt_patuh, atribusi, url_dasar, konfigurasi) OVERRIDING SYSTEM VALUE VALUES
  ('b15a05c6-4945-3be3-a1c9-1f10e1225e45', 'SRC-BAPANAS', 'Panel Harga Bapanas', 'Badan Pangan Nasional', 'SCRAPER', 'Harga produsen & konsumen', '0 11 * * *', TRUE, 'Sumber: Panel Harga Pangan, Badan Pangan Nasional', 'https://panelharga.badanpangan.go.id', '{}'::jsonb),
  ('40745615-f152-32aa-b1e3-f54add179514', 'SRC-BI', 'PIHPS Nasional — harga pangan', 'Bank Indonesia', 'API', 'Pasar tradisional & modern', '0 10 * * *', TRUE, 'Sumber: Pusat Informasi Harga Pangan Strategis Nasional (PIHPS), Bank Indonesia', 'https://www.bi.go.id/hargapangan', '{}'::jsonb),
  ('45586311-7d7f-3312-ac86-f004956548be', 'SRC-BMKG', 'BMKG — prakiraan & curah hujan', 'BMKG', 'API', '23 kab/kota', '0 */3 * * *', TRUE, 'Sumber: Badan Meteorologi, Klimatologi, dan Geofisika (BMKG)', 'https://api.bmkg.go.id', '{}'::jsonb),
  ('e1f49004-97cf-3b83-9c13-15c6eb3112d6', 'SRC-BPS', 'BPS WebAPI — IHK & inflasi', 'BPS', 'API', 'Provinsi & 3 kota IHK', '0 6 * * *', TRUE, 'Sumber: Badan Pusat Statistik (BPS), webapi.bps.go.id', 'https://webapi.bps.go.id/v1/api', '{"domain": "1100"}'::jsonb),
  ('aa32af81-bc1d-30e3-82af-e242d5ec8a2a', 'SRC-CS', 'Telemetri cold storage', 'Sensor IoT fasilitas', 'API', '3 fasilitas operasional', '*/5 * * * *', TRUE, 'Sumber: sensor suhu fasilitas cold storage mitra', NULL, '{}'::jsonb),
  ('28a1cdfe-2973-34f2-a274-8de8ccf4735b', 'SRC-DJPB', 'Kanwil DJPb — realisasi belanja', 'Kanwil DJPb Aceh', 'API', 'APBD & BTT 23 kab/kota', '0 7 * * 1', TRUE, 'Sumber: Kanwil Direktorat Jenderal Perbendaharaan Provinsi Aceh', NULL, '{}'::jsonb),
  ('5a2dfe7a-a025-389a-81f9-04db72abc477', 'SRC-OWM', 'OpenWeather — cuaca aktual', 'OpenWeather', 'API', 'Titik sentra produksi', '0 * * * *', TRUE, 'Weather data provided by OpenWeather', 'https://api.openweathermap.org/data/2.5', '{}'::jsonb),
  ('371f5560-5b0f-3e09-9937-ba0d9af88fb2', 'SRC-PORTAL', 'Portal harga Disperindag Aceh', 'Pemerintah Aceh', 'SCRAPER', 'Pasar induk provinsi', '0 12 * * *', TRUE, 'Sumber: Dinas Perindustrian dan Perdagangan Aceh', NULL, '{}'::jsonb)
ON CONFLICT (kode) DO NOTHING;

