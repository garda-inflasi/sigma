-- Seed referensi kode etik pencacah lapangan (aman dijalankan di produksi & idempoten).
INSERT INTO kode_etik (versi, judul, isi, aktif)
VALUES (
    'v1.0',
    'Kode Etik Pencacah Harga Pangan Lapangan (TPID Aceh)',
    E'1. Kejujuran & Akurasi Data: Petugas wajib mencatat harga riil yang terjadi di pasar sesuai satuan, kualitas, dan waktu pencatatan sebenarnya tanpa manipulasi.\n'
    || E'2. Kehadiran Fisik (Geofence): Pencacahan dilakukan langsung di lokasi pasar yang ditugaskan serta melampirkan foto bukti kondisi komoditas atau papan harga bila diperlukan.\n'
    || E'3. Kerahasiaan Narasumber: Identitas pedagang atau responden pasar dijaga kerahasiaannya dan hanya digunakan untuk kepentingan verifikasi internal pengendalian inflasi.\n'
    || E'4. Bebas Konflik Kepentingan: Petugas dilarang menerima imbalan atau bersepakat dengan pihak mana pun untuk memengaruhi angka pencatatan harga pangan.\n'
    || E'5. Kepatuhan Jadwal: Tugas survei diselesaikan sesuai tenggat waktu yang ditetapkan oleh koordinator wilayah atau tim TPID.',
    true
)
ON CONFLICT (versi) DO NOTHING;
