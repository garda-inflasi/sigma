-- GARDA INFLASI - Master Fasilitas Cold Storage

-- =====================================================================
-- Tabel: cold_storage (5 baris)
-- =====================================================================
INSERT INTO cold_storage (id, kode, nama, kabupaten_kode, lat, lng, kapasitas_ton, suhu_target_c, suhu_aktual_c, status) OVERRIDING SYSTEM VALUE VALUES
  ('aef12e36-fce3-3252-8c8a-ccdf9bd9dfcd', 'CS-01', 'Cold Storage Lambaro', '1108', 5.505, 95.37, 400, 4, 3.5, 'OPERASIONAL'),
  ('63235a58-7c88-32c5-8232-f56715b7c4ca', 'CS-02', 'Cold Storage Perikanan Lhokseumawe', '1174', 5.17, 97.13, 300, -20, -20.1, 'OPERASIONAL'),
  ('a67c5b46-1b27-3586-b058-5695a4f66709', 'CS-03', 'Sentra Hortikultura Takengon', '1106', 4.64, 96.86, 250, 6, 6, 'OPERASIONAL'),
  ('311f0ea8-91fc-39e0-a295-2776a2a8965b', 'CS-04', 'Gudang Dingin Meulaboh', '1107', 4.15, 96.14, 150, 4, 4.8, 'PERAWATAN'),
  ('5d9529d7-a236-3790-9cee-e1ede85f8833', 'CS-05', 'Gudang Dingin Langsa', '1173', 4.48, 97.96, 120, 4, NULL, 'RENCANA')
ON CONFLICT (kode) DO NOTHING;

