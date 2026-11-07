package schematest

import (
	"testing"

	"github.com/garda-inflasi/api/internal/platform/pgtest"
	"github.com/garda-inflasi/api/pkg/idx"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

// Migrasi 000113 mengubah kunci primer cold storage dan spasial. Tes dipatok ke versi 112 dan 113 (bukan
// "terbaru") supaya migrasi konversi berikutnya tidak mengubah bentuk fixture ID lama di bawah. Hanya tabel
// milik 000113 dan tabel referensi (provinsi, kabupaten, komoditas) serta pengguna yang diisi.
const (
	versiSebelumColdSpasial = 112
	versiColdSpasial        = 113
)

func uuidFasilitas(kode string) string { return idx.UUIDStabil("cold_storage", kode) }

// isiDataColdSpasialLama mengisi basis data versi 112 dengan baris berbentuk data lama. Urutan sisip sengaja
// tidak sama dengan urutan waktu, supaya pengurutan ID BIGINT menurut kolom waktu terbukti.
func isiDataColdSpasialLama(t *testing.T, db *gorm.DB) {
	t.Helper()
	sql := []string{
		`INSERT INTO provinsi (kode, nama) VALUES ('11', 'Aceh')`,
		`INSERT INTO kabupaten (kode, nama, jenis, ibukota, lat, lng, provinsi_kode) VALUES
			('1101', 'Kab Satu', 'KABUPATEN', 'Satu', 5.5, 95.3, '11'),
			('1102', 'Kab Dua', 'KABUPATEN', 'Dua', 5.1, 97.1, '11'),
			('1103', 'Kab Tiga', 'KABUPATEN', 'Tiga', 4.6, 96.8, '11'),
			('1104', 'Kab Empat', 'KABUPATEN', 'Empat', 4.1, 96.1, '11')`,
		`INSERT INTO komoditas (kode, nama, satuan, kategori, harga_acuan, shelf_life_hari) VALUES
			('CBM-KRT', 'Cabai Merah', 'kg', 'cabai', 50000, 7), ('BWM', 'Bawang Merah', 'kg', 'bawang', 40000, 30)`,
		`INSERT INTO pengguna (id, nama, email, peran, lapis, lembaga) VALUES
			('` + uuidPengguna("operator") + `', 'Operator', 'operator@x.id', 'OPS_MAHASISWA', 'OPERASIONAL', 'FMIPA')`,

		// Fasilitas: CS-01 dan CS-02 punya buku besar (RESTRICT), CS-03 tanpa anak.
		`INSERT INTO cold_storage (id, nama, kabupaten_kode, lat, lng, kapasitas_ton, suhu_target_c, status) VALUES
			('CS-01', 'Lambaro', '1101', 5.5, 95.3, 400, 4, 'OPERASIONAL'),
			('CS-02', 'Lhokseumawe', '1102', 5.1, 97.1, 300, -20, 'OPERASIONAL'),
			('CS-03', 'Takengon', '1103', 4.6, 96.8, 250, 6, 'OPERASIONAL')`,

		// Lot: urutan menurut masuk_pada, lalu ID lama (STK-A dan STK-M kembar waktu).
		`INSERT INTO stok_cold_storage (id, cold_storage_id, komoditas_kode, volume_ton, masuk_pada, pemilik, risiko_food_loss, kedaluwarsa_pada, status) VALUES
			('STK-Z', 'CS-01', 'CBM-KRT', 30, '2026-01-01 00:00+00', 'KUD', 0.1, '2026-02-01 00:00+00', 'AKTIF'),
			('STK-M', 'CS-02', 'CBM-KRT', 10, '2026-01-02 00:00+00', 'KUD', 0.1, '2026-02-02 00:00+00', 'AKTIF'),
			('STK-A', 'CS-01', 'BWM', 40, '2026-01-02 00:00+00', 'KUD', 0.2, '2026-03-02 00:00+00', 'AKTIF'),
			('STK-H', 'CS-01', 'BWM', 0, '2025-12-01 00:00+00', 'KUD', 0.2, '2026-01-01 00:00+00', 'HABIS')`,
		// Lot yang masih berlaku hari ini, supaya mv_kpi_operasional punya nilai 'cs' bukan nol.
		`INSERT INTO stok_cold_storage (id, cold_storage_id, komoditas_kode, volume_ton, masuk_pada, pemilik, risiko_food_loss, kedaluwarsa_pada, status)
			VALUES ('STK-LIVE', 'CS-02', 'BWM', 19, now() - interval '5 days', 'KUD', 0.1, now() + interval '20 days', 'AKTIF')`,

		// Buku besar: TRX-2 (pindah sebagian, punya lot baru) disisip lebih dulu dari TRX-1 yang lebih tua.
		`INSERT INTO transaksi_stok (id, lot_id, lot_baru_id, jenis, volume_ton, fasilitas_asal_id, fasilitas_tujuan_id, oleh_id, waktu, alasan, sumber) VALUES
			('TRX-3', 'STK-H', NULL, 'KELUAR', 5, 'CS-01', NULL, NULL, '2026-01-05 00:00+00', 'dibuang', 'APLIKASI'),
			('TRX-2', 'STK-A', 'STK-M', 'PINDAH', 10, 'CS-01', 'CS-02', '` + uuidPengguna("operator") + `', '2026-01-03 00:00+00', 'pindah', 'APLIKASI'),
			('TRX-1', 'STK-Z', NULL, 'MASUK', 30, NULL, 'CS-01', NULL, '2026-01-01 10:00+00', 'awal', 'DEMO')`,

		`INSERT INTO telemetri_suhu (fasilitas_id, waktu, suhu_c, kelembapan_pct, sumber) VALUES
			('CS-01', '2026-01-01 00:00+00', 4.1, 80, 'SENSOR'), ('CS-01', '2026-01-01 00:30+00', 4.3, 81, 'SENSOR'),
			('CS-02', '2026-01-01 00:00+00', -19.8, 70, 'MANUAL')`,

		`INSERT INTO kandidat_cold_storage (id, kabupaten_kode, lokasi, skor, komponen_skor, dibuat_pada) VALUES
			('KC-B', '1101', 'Lokasi B', 70, '{"akses_jalan": 70}', '2026-02-02 00:00+00'),
			('KC-A', '1102', 'Lokasi A', 80, '{"akses_jalan": 80}', '2026-02-01 00:00+00')`,

		`INSERT INTO optimasi_rute (id, komoditas_kode, periode, parameter, total_volume_ton, total_biaya, belum_terpenuhi_ton, dibuat_oleh, dibuat_pada) VALUES
			('OPT-2', 'CBM-KRT', '2026-01-01', '{}', 10, 1000, 0, '` + uuidPengguna("operator") + `', '2026-03-02 00:00+00'),
			('OPT-1', 'BWM', '2026-01-01', '{}', 20, 2000, 0, NULL, '2026-03-01 00:00+00')`,
		`INSERT INTO rute_distribusi (id, asal_kode, tujuan_kode, komoditas_kode, volume_ton, jarak_km, jam_tempuh, biaya, status, sumber, dibuat_pada, optimasi_id) VALUES
			('RT-OPT-2-01', '1101', '1102', 'CBM-KRT', 5, 10, 1.0, 100, 'USULAN', 'OPTIMASI', '2026-03-02 00:00:02+00', 'OPT-2'),
			('RT-OPT-1-01', '1101', '1102', 'BWM', 6, 11, 1.1, 110, 'USULAN', 'OPTIMASI', '2026-03-01 00:00:02+00', 'OPT-1'),
			('RT-1101-1103', '1101', '1103', 'BWM', 8, 20, 2.0, 200, 'USULAN', 'DEMO', '2026-03-01 00:00:00+00', NULL)`,

		`INSERT INTO neraca_wilayah (komoditas_kode, kabupaten_kode, periode, surplus_defisit_ton, delta_harga_pct) VALUES
			('CBM-KRT', '1101', '2026-01-01', 10.5, -3.2), ('CBM-KRT', '1102', '2026-01-01', -4.0, 6.0), ('BWM', '1101', '2026-01-01', 1.0, 0.5)`,
		`INSERT INTO jarak_antar_wilayah (asal_kode, tujuan_kode, jarak_km, jam_tempuh) VALUES
			('1101', '1102', 10.0, 1.0), ('1102', '1101', 10.0, 1.0), ('1104', '1101', 30.0, 2.0), ('1101', '1104', 30.0, 2.0)`,
		`INSERT INTO wilayah_geometri (kabupaten_kode, geojson, min_lat, min_lng, max_lat, max_lng) VALUES
			('1101', '{"type":"Polygon","coordinates":[[[95,5],[96,5],[96,6],[95,5]]]}', 5, 95, 6, 96),
			('1104', '{"type":"Polygon","coordinates":[[[96,4],[97,4],[97,5],[96,4]]]}', 4, 96, 5, 97)`,

		// Rujukan lunak: dipetakan hanya bila sama dengan ID lama baris yang ada.
		`INSERT INTO audit_log (id, aksi, entitas, entitas_id, hasil) VALUES
			('G1', 'UBAH_FASILITAS_COLD_STORAGE', 'cold_storage', 'CS-01', 'BERHASIL'),
			('G2', 'INGEST_TELEMETRI', 'cold_storage', 'CS-01,CS-02', 'BERHASIL'),
			('G3', 'TRANSAKSI_STOK_PINDAH', 'stok_cold_storage', 'STK-A', 'BERHASIL'),
			('G4', 'BULK_rekomendasi_olahan', 'stok_cold_storage', 'STO-2292', 'BERHASIL'),
			('G5', 'BULK_rekomendasi_olahan', 'stok_cold_storage', '2 lot', 'BERHASIL'),
			('G6', 'OPTIMASI_RUTE', 'optimasi_rute', 'OPT-1', 'BERHASIL'),
			('G7', 'LOGIN', 'pengguna', 'CS-01', 'BERHASIL'),
			('G8', 'UBAH_FASILITAS_COLD_STORAGE', 'cold_storage', 'CS-99', 'BERHASIL')`,
	}
	for _, s := range sql {
		exec(t, db, s)
	}
}

func tipeKolom(t *testing.T, db *gorm.DB, tabel, kolom string) string {
	t.Helper()
	return scanString(t, db, `SELECT format_type(atttypid, atttypmod) FROM pg_attribute
		WHERE attrelid = ?::text::regclass AND attname = ? AND NOT attisdropped`, tabel, kolom)
}

func adaKolom(t *testing.T, db *gorm.DB, tabel, kolom string) bool {
	t.Helper()
	return scanCount(t, db, `SELECT count(*) FROM pg_attribute WHERE attrelid = ?::text::regclass AND attname = ? AND NOT attisdropped`, tabel, kolom) > 0
}

func aturanHapusFK(t *testing.T, db *gorm.DB, nama string) string {
	t.Helper()
	return scanString(t, db, `SELECT CASE confdeltype WHEN 'a' THEN 'NO ACTION' WHEN 'r' THEN 'RESTRICT' WHEN 'c' THEN 'CASCADE'
		WHEN 'n' THEN 'SET NULL' ELSE 'SET DEFAULT' END FROM pg_constraint WHERE conname = ? AND contype = 'f'`, nama)
}

func adaIndeks(t *testing.T, db *gorm.DB, nama string) bool {
	t.Helper()
	return scanCount(t, db, `SELECT count(*) FROM pg_class WHERE relname = ? AND relkind = 'i'`, nama) > 0
}

func definisiIndeks(t *testing.T, db *gorm.DB, nama string) string {
	t.Helper()
	return scanString(t, db, `SELECT indexdef FROM pg_indexes WHERE indexname = ?`, nama)
}

func identitasKolom(t *testing.T, db *gorm.DB, tabel, kolom string) string {
	t.Helper()
	return scanString(t, db, `SELECT attidentity::text FROM pg_attribute WHERE attrelid = ?::text::regclass AND attname = ?`, tabel, kolom)
}

// TestMigrasiColdSpasial_DataLamaIkutDanBisaDibalik: 000113 memetakan ID lama ke UUID stabil (fasilitas) atau ke
// BIGINT berurutan (lot, transaksi, kandidat, optimasi, rute), menjaga FK anak dan rujukan lunak, memasang ON DELETE
// eksplisit, dan Down memulihkan ID lama persis.
func TestMigrasiColdSpasial_DataLamaIkutDanBisaDibalik(t *testing.T) {
	db := pg.NewDatabase(t, "migrate_cold_spasial")
	migrator := pgtest.Migrator(t, db)
	require.NoError(t, migrator.UpTo(t.Context(), versiSebelumColdSpasial))
	isiDataColdSpasialLama(t, db)
	require.Equal(t, "character varying(255)", tipeKolom(t, db, "cold_storage", "id"))
	// Nilai 'cs' tanpa 000113 adalah baseline; setelah Up nilainya harus sama (definisi view tidak berubah).
	require.NoError(t, db.Exec(`REFRESH MATERIALIZED VIEW mv_kpi_operasional`).Error)
	csSebelum := scanString(t, db, `SELECT nilai::text FROM mv_kpi_operasional WHERE indikator = 'cs' ORDER BY tanggal DESC LIMIT 1`)
	require.NotEqual(t, "0", csSebelum, "fixture harus menghasilkan utilisasi bukan nol")

	require.NoError(t, migrator.UpTo(t.Context(), versiColdSpasial), "up 000113")

	t.Run("cold_storage.id menjadi uuid stabil, ID lama menjadi kode", func(t *testing.T) {
		assert.Equal(t, "uuid", tipeKolom(t, db, "cold_storage", "id"))
		assert.Equal(t, "gen_random_uuid()", scanString(t, db, `SELECT pg_get_expr(adbin, adrelid) FROM pg_attrdef
			WHERE adrelid = 'cold_storage'::regclass AND adnum = (SELECT attnum FROM pg_attribute WHERE attrelid = 'cold_storage'::regclass AND attname = 'id')`))
		for _, kode := range []string{"CS-01", "CS-02", "CS-03"} {
			assert.Equal(t, kode, scanString(t, db, `SELECT kode FROM cold_storage WHERE id = ?::uuid`, uuidFasilitas(kode)), kode)
		}
		assert.EqualValues(t, 3, scanCount(t, db, `SELECT count(*) FROM cold_storage`))
	})

	t.Run("kolom anak fasilitas ikut ke UUID yang sama", func(t *testing.T) {
		kasus := []struct{ nama, sql, kode string }{
			{"stok.cold_storage_id", `SELECT cold_storage_id::text FROM stok_cold_storage WHERE kode = 'STK-A'`, "CS-01"},
			{"stok.cold_storage_id CS-02", `SELECT cold_storage_id::text FROM stok_cold_storage WHERE kode = 'STK-M'`, "CS-02"},
			{"telemetri.fasilitas_id", `SELECT fasilitas_id::text FROM telemetri_suhu WHERE suhu_c = -19.8`, "CS-02"},
			{"transaksi.fasilitas_asal_id", `SELECT fasilitas_asal_id::text FROM transaksi_stok WHERE kode = 'TRX-2'`, "CS-01"},
			{"transaksi.fasilitas_tujuan_id", `SELECT fasilitas_tujuan_id::text FROM transaksi_stok WHERE kode = 'TRX-2'`, "CS-02"},
			{"transaksi masuk tujuan", `SELECT fasilitas_tujuan_id::text FROM transaksi_stok WHERE kode = 'TRX-1'`, "CS-01"},
		}
		for _, k := range kasus {
			assert.Equal(t, uuidFasilitas(k.kode), scanString(t, db, k.sql), k.nama)
		}
		assert.EqualValues(t, 1, scanCount(t, db, `SELECT count(*) FROM transaksi_stok WHERE kode = 'TRX-3' AND fasilitas_tujuan_id IS NULL`), "NULL tetap NULL")
	})

	t.Run("lot: BIGINT berurutan menurut masuk_pada lalu ID lama, kode = ID lama", func(t *testing.T) {
		assert.Equal(t, "bigint", tipeKolom(t, db, "stok_cold_storage", "id"))
		urutan := map[string]int64{"STK-H": 1, "STK-Z": 2, "STK-A": 3, "STK-M": 4, "STK-LIVE": 5}
		for kode, id := range urutan {
			assert.Equal(t, id, scanCount(t, db, `SELECT id FROM stok_cold_storage WHERE kode = ?`, kode), kode)
		}
		assert.Equal(t, "a", identitasKolom(t, db, "stok_cold_storage", "id"), "GENERATED ALWAYS")
	})

	t.Run("transaksi: BIGINT berurutan menurut waktu, lot_id dan lot_baru_id menunjuk ID baru", func(t *testing.T) {
		assert.Equal(t, "bigint", tipeKolom(t, db, "transaksi_stok", "id"))
		for kode, id := range map[string]int64{"TRX-1": 1, "TRX-2": 2, "TRX-3": 3} {
			assert.Equal(t, id, scanCount(t, db, `SELECT id FROM transaksi_stok WHERE kode = ?`, kode), kode)
		}
		assert.Equal(t, "bigint", tipeKolom(t, db, "transaksi_stok", "lot_id"))
		assert.Equal(t, "bigint", tipeKolom(t, db, "transaksi_stok", "lot_baru_id"))
		assert.Equal(t, "STK-A", scanString(t, db, `SELECT s.kode FROM transaksi_stok t JOIN stok_cold_storage s ON s.id = t.lot_id WHERE t.kode = 'TRX-2'`))
		assert.Equal(t, "STK-M", scanString(t, db, `SELECT s.kode FROM transaksi_stok t JOIN stok_cold_storage s ON s.id = t.lot_baru_id WHERE t.kode = 'TRX-2'`))
		assert.Equal(t, "STK-H", scanString(t, db, `SELECT s.kode FROM transaksi_stok t JOIN stok_cold_storage s ON s.id = t.lot_id WHERE t.kode = 'TRX-3'`))
		assert.EqualValues(t, 1, scanCount(t, db, `SELECT count(*) FROM transaksi_stok WHERE kode = 'TRX-1' AND lot_baru_id IS NULL`))
		assert.Equal(t, uuidPengguna("operator"), scanString(t, db, `SELECT oleh_id::text FROM transaksi_stok WHERE kode = 'TRX-2'`), "FK pengguna tidak berubah")
	})

	t.Run("kandidat, optimasi, dan rute: BIGINT menurut dibuat_pada, rute menunjuk rencana barunya", func(t *testing.T) {
		for kode, id := range map[string]int64{"KC-A": 1, "KC-B": 2} {
			assert.Equal(t, id, scanCount(t, db, `SELECT id FROM kandidat_cold_storage WHERE kode = ?`, kode), kode)
		}
		for kode, id := range map[string]int64{"OPT-1": 1, "OPT-2": 2} {
			assert.Equal(t, id, scanCount(t, db, `SELECT id FROM optimasi_rute WHERE kode = ?`, kode), kode)
		}
		for kode, id := range map[string]int64{"RT-1101-1103": 1, "RT-OPT-1-01": 2, "RT-OPT-2-01": 3} {
			assert.Equal(t, id, scanCount(t, db, `SELECT id FROM rute_distribusi WHERE kode = ?`, kode), kode)
		}
		assert.Equal(t, "bigint", tipeKolom(t, db, "rute_distribusi", "optimasi_id"))
		assert.Equal(t, "OPT-1", scanString(t, db, `SELECT o.kode FROM rute_distribusi r JOIN optimasi_rute o ON o.id = r.optimasi_id WHERE r.kode = 'RT-OPT-1-01'`))
		assert.Equal(t, "OPT-2", scanString(t, db, `SELECT o.kode FROM rute_distribusi r JOIN optimasi_rute o ON o.id = r.optimasi_id WHERE r.kode = 'RT-OPT-2-01'`))
		assert.EqualValues(t, 1, scanCount(t, db, `SELECT count(*) FROM rute_distribusi WHERE kode = 'RT-1101-1103' AND optimasi_id IS NULL`))
		assert.Equal(t, uuidPengguna("operator"), scanString(t, db, `SELECT dibuat_oleh::text FROM optimasi_rute WHERE kode = 'OPT-2'`))
	})

	t.Run("neraca_wilayah: id BIGINT identity, kunci alami tetap unik dan jadi target upsert", func(t *testing.T) {
		assert.Equal(t, "bigint", tipeKolom(t, db, "neraca_wilayah", "id"))
		assert.Equal(t, "a", identitasKolom(t, db, "neraca_wilayah", "id"))
		assert.EqualValues(t, 3, scanCount(t, db, `SELECT count(DISTINCT id) FROM neraca_wilayah`))
		idAwal := scanCount(t, db, `SELECT id FROM neraca_wilayah WHERE komoditas_kode = 'CBM-KRT' AND kabupaten_kode = '1101'`)

		exec(t, db, `INSERT INTO neraca_wilayah (komoditas_kode, kabupaten_kode, periode, surplus_defisit_ton, delta_harga_pct)
			VALUES ('CBM-KRT', '1101', '2026-01-01', 99.9, 1.1)
			ON CONFLICT (komoditas_kode, kabupaten_kode, periode) DO UPDATE SET surplus_defisit_ton = EXCLUDED.surplus_defisit_ton`)
		assert.EqualValues(t, 3, scanCount(t, db, `SELECT count(*) FROM neraca_wilayah`), "upsert tidak menambah baris")
		assert.Equal(t, idAwal, scanCount(t, db, `SELECT id FROM neraca_wilayah WHERE komoditas_kode = 'CBM-KRT' AND kabupaten_kode = '1101'`), "id tetap")
		assert.Equal(t, "99.9", scanString(t, db, `SELECT surplus_defisit_ton::text FROM neraca_wilayah WHERE komoditas_kode = 'CBM-KRT' AND kabupaten_kode = '1101'`))

		err := db.Exec(`INSERT INTO neraca_wilayah (komoditas_kode, kabupaten_kode, periode, surplus_defisit_ton, delta_harga_pct)
			VALUES ('BWM', '1101', '2026-01-01', 1, 1)`).Error
		require.Error(t, err)
		assert.Contains(t, err.Error(), "23505")
	})

	t.Run("identity berlanjut dari max+1 dan menolak id eksplisit", func(t *testing.T) {
		// Lot uji di CS-02 dengan masa simpan yang sudah lewat (tidak memengaruhi mv_kpi_operasional).
		stok := scanCount(t, db, `INSERT INTO stok_cold_storage (cold_storage_id, komoditas_kode, volume_ton, masuk_pada, pemilik, risiko_food_loss, kedaluwarsa_pada)
			VALUES (?::uuid, 'BWM', 1, '2026-01-01 00:00+00', 'X', 0.1, '2026-01-10 00:00+00') RETURNING id`, uuidFasilitas("CS-02"))
		assert.EqualValues(t, 6, stok)
		trx := scanCount(t, db, `INSERT INTO transaksi_stok (lot_id, jenis, volume_ton, fasilitas_tujuan_id) VALUES (?, 'MASUK', 1, ?::uuid) RETURNING id`, stok, uuidFasilitas("CS-02"))
		assert.EqualValues(t, 4, trx)
		kandidat := scanCount(t, db, `INSERT INTO kandidat_cold_storage (kabupaten_kode, lokasi, skor, komponen_skor) VALUES ('1101', 'Baru', 1, '{}') RETURNING id`)
		assert.EqualValues(t, 3, kandidat)
		optimasi := scanCount(t, db, `INSERT INTO optimasi_rute (komoditas_kode, periode, parameter, total_volume_ton, total_biaya, belum_terpenuhi_ton)
			VALUES ('BWM', '2026-02-01', '{}', 1, 1, 0) RETURNING id`)
		assert.EqualValues(t, 3, optimasi)
		rute := scanCount(t, db, `INSERT INTO rute_distribusi (asal_kode, tujuan_kode, komoditas_kode, volume_ton, jarak_km, jam_tempuh, biaya)
			VALUES ('1102', '1103', 'BWM', 1, 1, 1, 1) RETURNING id`)
		assert.EqualValues(t, 4, rute)
		maksNeraca := scanCount(t, db, `SELECT max(id) FROM neraca_wilayah`)
		neraca := scanCount(t, db, `INSERT INTO neraca_wilayah (komoditas_kode, kabupaten_kode, periode, surplus_defisit_ton, delta_harga_pct)
			VALUES ('BWM', '1102', '2026-01-01', 1, 1) RETURNING id`)
		assert.Greater(t, neraca, maksNeraca, "identity neraca berlanjut, tidak bentrok dengan id yang sudah ada")

		// Kode baris baru boleh NULL (banyak NULL diizinkan oleh UNIQUE).
		assert.EqualValues(t, 1, scanCount(t, db, `SELECT count(*) FROM stok_cold_storage WHERE id = ? AND kode IS NULL`, stok))
		assert.EqualValues(t, 1, scanCount(t, db, `SELECT count(*) FROM optimasi_rute WHERE id = ? AND kode IS NULL`, optimasi))

		for _, tabel := range []string{"stok_cold_storage", "transaksi_stok", "kandidat_cold_storage", "optimasi_rute", "rute_distribusi", "neraca_wilayah"} {
			err := db.Exec(`INSERT INTO ` + tabel + ` (id) VALUES (999)`).Error
			require.Error(t, err, tabel)
			assert.Contains(t, err.Error(), "428C9", tabel+": GENERATED ALWAYS menolak id eksplisit")
		}
		// Hanya lot uji yang dipertahankan (untuk pemeriksaan Down); baris uji lain dibuang.
		exec(t, db, `DELETE FROM transaksi_stok WHERE id = ?`, trx)
		exec(t, db, `DELETE FROM kandidat_cold_storage WHERE id = ?`, kandidat)
		exec(t, db, `DELETE FROM optimasi_rute WHERE id = ?`, optimasi)
		exec(t, db, `DELETE FROM rute_distribusi WHERE id = ?`, rute)
		exec(t, db, `DELETE FROM neraca_wilayah WHERE id = ?`, neraca)
	})

	t.Run("mv_kpi_operasional dibuat ulang: nama, kolom, indeks unik, nilai sama, dan REFRESH CONCURRENTLY", func(t *testing.T) {
		assert.EqualValues(t, 1, scanCount(t, db, `SELECT count(*) FROM pg_matviews WHERE matviewname = 'mv_kpi_operasional' AND ispopulated`))
		assert.Equal(t, "indikator,tanggal,nilai", scanString(t, db, `SELECT string_agg(attname, ',' ORDER BY attnum) FROM pg_attribute
			WHERE attrelid = 'mv_kpi_operasional'::regclass AND attnum > 0 AND NOT attisdropped`))
		assert.Contains(t, definisiIndeks(t, db, "uq_mv_kpi_operasional"), "UNIQUE INDEX uq_mv_kpi_operasional ON public.mv_kpi_operasional USING btree (indikator, tanggal)")
		assert.EqualValues(t, 14*4, scanCount(t, db, `SELECT count(*) FROM mv_kpi_operasional`))
		require.NoError(t, db.Exec(`REFRESH MATERIALIZED VIEW CONCURRENTLY mv_kpi_operasional`).Error)
		// Lot STK-LIVE (19 ton) dari kapasitas 950 ton fasilitas operasional: sama dengan sebelum migrasi.
		assert.Equal(t, csSebelum, scanString(t, db, `SELECT nilai::text FROM mv_kpi_operasional WHERE indikator = 'cs' ORDER BY tanggal DESC LIMIT 1`))
		assert.Equal(t, "2", csSebelum)
	})

	t.Run("ON DELETE: buku besar menahan, stok dan telemetri ikut, rute ikut rencananya", func(t *testing.T) {
		// Fasilitas dengan buku besar tidak bisa dihapus (RESTRICT), dan lot yang dirujuk transaksi juga.
		for _, sql := range []string{
			`DELETE FROM cold_storage WHERE kode = 'CS-01'`,
			`DELETE FROM cold_storage WHERE kode = 'CS-02'`,
			`DELETE FROM stok_cold_storage WHERE kode = 'STK-A'`,
			`DELETE FROM stok_cold_storage WHERE kode = 'STK-M'`,
			`DELETE FROM stok_cold_storage WHERE kode = 'STK-Z'`,
		} {
			err := db.Exec(sql).Error
			require.Error(t, err, sql)
			assert.Contains(t, err.Error(), "23503", sql)
		}

		// CS-03 hanya punya satu lot (tanpa transaksi) dan telemetri: menghapusnya menghapus keduanya.
		exec(t, db, `INSERT INTO stok_cold_storage (cold_storage_id, komoditas_kode, volume_ton, masuk_pada, pemilik, risiko_food_loss, kedaluwarsa_pada)
			VALUES (?::uuid, 'BWM', 1, '2026-01-01 00:00+00', 'X', 0.1, '2026-01-10 00:00+00')`, uuidFasilitas("CS-03"))
		exec(t, db, `INSERT INTO telemetri_suhu (fasilitas_id, waktu, suhu_c) VALUES (?::uuid, now(), 5)`, uuidFasilitas("CS-03"))
		require.EqualValues(t, 1, scanCount(t, db, `SELECT count(*) FROM stok_cold_storage WHERE cold_storage_id = ?::uuid`, uuidFasilitas("CS-03")))
		exec(t, db, `DELETE FROM cold_storage WHERE kode = 'CS-03'`)
		assert.Zero(t, scanCount(t, db, `SELECT count(*) FROM stok_cold_storage WHERE cold_storage_id = ?::uuid`, uuidFasilitas("CS-03")), "stok ikut terhapus")
		assert.Zero(t, scanCount(t, db, `SELECT count(*) FROM telemetri_suhu WHERE fasilitas_id = ?::uuid`, uuidFasilitas("CS-03")), "telemetri ikut terhapus")

		// Menghapus rencana optimasi menghapus rutenya (CASCADE); rute lain tidak tersentuh.
		exec(t, db, `DELETE FROM optimasi_rute WHERE kode = 'OPT-1'`)
		assert.Zero(t, scanCount(t, db, `SELECT count(*) FROM rute_distribusi WHERE kode = 'RT-OPT-1-01'`))
		assert.EqualValues(t, 1, scanCount(t, db, `SELECT count(*) FROM rute_distribusi WHERE kode = 'RT-OPT-2-01'`))
		assert.EqualValues(t, 1, scanCount(t, db, `SELECT count(*) FROM rute_distribusi WHERE kode = 'RT-1101-1103'`))

		// Kabupaten/komoditas yang dirujuk tabel transaksional ditahan (RESTRICT).
		for _, sql := range []string{
			`DELETE FROM kabupaten WHERE kode = '1101'`,    // cold_storage, kandidat, rute, neraca
			`DELETE FROM kabupaten WHERE kode = '1102'`,    // cold_storage, kandidat, rute, neraca
			`DELETE FROM komoditas WHERE kode = 'CBM-KRT'`, // stok, optimasi, rute, neraca
		} {
			err := db.Exec(sql).Error
			require.Error(t, err, sql)
			assert.Contains(t, err.Error(), "23503", sql)
		}

		// Matriks jarak dan geometri adalah turunan wilayah: ikut terhapus bersama kabupatennya (CASCADE).
		require.EqualValues(t, 2, scanCount(t, db, `SELECT count(*) FROM jarak_antar_wilayah WHERE asal_kode = '1104' OR tujuan_kode = '1104'`))
		exec(t, db, `DELETE FROM kabupaten WHERE kode = '1104'`)
		assert.Zero(t, scanCount(t, db, `SELECT count(*) FROM jarak_antar_wilayah WHERE asal_kode = '1104' OR tujuan_kode = '1104'`))
		assert.Zero(t, scanCount(t, db, `SELECT count(*) FROM wilayah_geometri WHERE kabupaten_kode = '1104'`))
		assert.EqualValues(t, 2, scanCount(t, db, `SELECT count(*) FROM jarak_antar_wilayah`), "pasangan 1101 dan 1102 tidak tersentuh")
		assert.EqualValues(t, 1, scanCount(t, db, `SELECT count(*) FROM wilayah_geometri WHERE kabupaten_kode = '1101'`))
	})

	t.Run("aturan ON DELETE dan indeks baru terpasang", func(t *testing.T) {
		hapus := map[string]string{
			"stok_cold_storage_cold_storage_id_fkey":    "CASCADE",
			"telemetri_suhu_fasilitas_id_fkey":          "CASCADE",
			"transaksi_stok_fasilitas_asal_id_fkey":     "RESTRICT",
			"transaksi_stok_fasilitas_tujuan_id_fkey":   "RESTRICT",
			"transaksi_stok_lot_id_fkey":                "RESTRICT",
			"transaksi_stok_lot_baru_id_fkey":           "RESTRICT",
			"transaksi_stok_oleh_id_fkey":               "SET NULL",
			"cold_storage_kabupaten_kode_fkey":          "RESTRICT",
			"stok_cold_storage_komoditas_kode_fkey":     "RESTRICT",
			"kandidat_cold_storage_kabupaten_kode_fkey": "RESTRICT",
			"optimasi_rute_komoditas_kode_fkey":         "RESTRICT",
			"optimasi_rute_dibuat_oleh_fkey":            "SET NULL",
			"rute_distribusi_optimasi_id_fkey":          "CASCADE",
			"rute_distribusi_asal_kode_fkey":            "RESTRICT",
			"rute_distribusi_tujuan_kode_fkey":          "RESTRICT",
			"rute_distribusi_komoditas_kode_fkey":       "RESTRICT",
			"neraca_wilayah_komoditas_kode_fkey":        "RESTRICT",
			"neraca_wilayah_kabupaten_kode_fkey":        "RESTRICT",
			"jarak_antar_wilayah_asal_kode_fkey":        "CASCADE",
			"jarak_antar_wilayah_tujuan_kode_fkey":      "CASCADE",
			"wilayah_geometri_kabupaten_kode_fkey":      "CASCADE",
		}
		for nama, aturan := range hapus {
			assert.Equal(t, aturan, aturanHapusFK(t, db, nama), nama)
		}
		for _, nama := range []string{
			"idx_cold_storage_kabupaten_kode", "idx_stok_cold_storage_komoditas_kode", "idx_stok_cold_storage_aktif",
			"idx_transaksi_stok_lot_baru", "idx_telemetri_suhu_waktu", "idx_kandidat_cold_storage_kabupaten_kode",
			"idx_optimasi_rute_komoditas_periode", "idx_rute_distribusi_asal", "idx_rute_distribusi_komoditas",
			"idx_rute_distribusi_aktif", "idx_neraca_wilayah_kabupaten_periode", "idx_jarak_antar_wilayah_tujuan",
		} {
			assert.True(t, adaIndeks(t, db, nama), nama)
		}
		assert.Contains(t, definisiIndeks(t, db, "idx_telemetri_suhu_waktu"), "USING brin (waktu)")
		assert.Contains(t, definisiIndeks(t, db, "idx_stok_cold_storage_aktif"), "WHERE ((status)::text <> 'HABIS'::text)")
		assert.Contains(t, definisiIndeks(t, db, "idx_rute_distribusi_aktif"), "WHERE ((status)::text <> 'DIGANTIKAN'::text)")
		// Grafik per fasilitas dilayani UNIQUE (fasilitas_id, waktu).
		assert.Contains(t, definisiIndeks(t, db, "telemetri_suhu_fasilitas_id_waktu_key"), "(fasilitas_id, waktu)")
		// Indeks FK lama yang ikut dikonversi tetap ada.
		for _, nama := range []string{"idx_stok_cold_storage_fasilitas", "idx_stok_cold_storage_kedaluwarsa", "idx_transaksi_stok_lot",
			"idx_transaksi_stok_asal", "idx_transaksi_stok_tujuan", "idx_rute_distribusi_optimasi", "idx_rute_distribusi_tujuan", "idx_optimasi_rute_dibuat"} {
			assert.True(t, adaIndeks(t, db, nama), nama)
		}
		assert.Equal(t, "bigint", tipeKolom(t, db, "telemetri_suhu", "id"), "telemetri tetap BIGSERIAL")
		assert.Contains(t, scanString(t, db, `SELECT pg_get_expr(adbin, adrelid) FROM pg_attrdef
			WHERE adrelid = 'telemetri_suhu'::regclass AND adnum = 1`), "nextval(")
	})

	t.Run("CHECK buku besar dipasang ulang dan tetap menolak lokasi yang salah", func(t *testing.T) {
		assert.EqualValues(t, 1, scanCount(t, db, `SELECT count(*) FROM pg_constraint WHERE conname = 'chk_transaksi_stok_lokasi' AND convalidated`))
		err := db.Exec(`INSERT INTO transaksi_stok (lot_id, jenis, volume_ton, fasilitas_asal_id, fasilitas_tujuan_id)
			SELECT s.id, 'PINDAH', 1, c.id, c.id FROM stok_cold_storage s, cold_storage c WHERE s.kode = 'STK-A' AND c.kode = 'CS-01'`).Error
		require.Error(t, err)
		assert.Contains(t, err.Error(), "chk_transaksi_stok_lokasi")
	})

	t.Run("rujukan lunak audit_log dipetakan, sisanya tidak, dan audit tetap append-only", func(t *testing.T) {
		assert.Equal(t, uuidFasilitas("CS-01"), scanString(t, db, `SELECT entitas_id FROM audit_log WHERE id = 'G1'`))
		assert.Equal(t, "CS-01,CS-02", scanString(t, db, `SELECT entitas_id FROM audit_log WHERE id = 'G2'`), "daftar bukan satu ID")
		assert.Equal(t, "3", scanString(t, db, `SELECT entitas_id FROM audit_log WHERE id = 'G3'`), "STK-A menjadi id lot barunya")
		assert.Equal(t, "STO-2292", scanString(t, db, `SELECT entitas_id FROM audit_log WHERE id = 'G4'`), "lot yang tidak ada tidak dikarang")
		assert.Equal(t, "2 lot", scanString(t, db, `SELECT entitas_id FROM audit_log WHERE id = 'G5'`))
		assert.Equal(t, "1", scanString(t, db, `SELECT entitas_id FROM audit_log WHERE id = 'G6'`), "OPT-1 menjadi id rencananya")
		assert.Equal(t, "CS-01", scanString(t, db, `SELECT entitas_id FROM audit_log WHERE id = 'G7'`), "entitas lain tidak disentuh")
		assert.Equal(t, "CS-99", scanString(t, db, `SELECT entitas_id FROM audit_log WHERE id = 'G8'`))
		assert.Equal(t, "O", scanString(t, db, `SELECT tgenabled::text FROM pg_trigger WHERE tgname = 'trg_audit_log_append_only'`))
		err := db.Exec(`UPDATE audit_log SET aksi = 'DIUBAH' WHERE id = 'G1'`).Error
		require.Error(t, err)
		assert.Contains(t, err.Error(), "append-only")
	})

	t.Run("down memulihkan tipe, ID lama, aturan semula, dan membuang kode, indeks baru, dan view tetap ada", func(t *testing.T) {
		// Baris yang dibuat sesudah Up: tanpa kode memakai id::text.
		exec(t, db, `INSERT INTO optimasi_rute (komoditas_kode, periode, parameter, total_volume_ton, total_biaya, belum_terpenuhi_ton, kode)
			VALUES ('BWM', '2026-03-01', '{}', 1, 1, 0, 'OPT-000009')`)
		exec(t, db, `INSERT INTO cold_storage (nama, kabupaten_kode, lat, lng, kapasitas_ton, suhu_target_c, status)
			VALUES ('Baru', '1102', 1, 1, 10, 4, 'RENCANA')`)
		baru := scanString(t, db, `SELECT id::text FROM cold_storage WHERE nama = 'Baru'`)
		require.True(t, idx.ValidUUID(baru))

		require.NoError(t, migrator.DownTo(t.Context(), versiSebelumColdSpasial), "down 000113")

		for _, k := range [][2]string{
			{"cold_storage", "id"}, {"stok_cold_storage", "cold_storage_id"}, {"telemetri_suhu", "fasilitas_id"},
			{"transaksi_stok", "fasilitas_asal_id"}, {"transaksi_stok", "fasilitas_tujuan_id"}, {"stok_cold_storage", "id"},
			{"transaksi_stok", "lot_id"}, {"transaksi_stok", "lot_baru_id"}, {"kandidat_cold_storage", "id"},
			{"rute_distribusi", "id"},
		} {
			assert.Equal(t, "character varying(255)", tipeKolom(t, db, k[0], k[1]), k[0]+"."+k[1])
		}
		assert.Equal(t, "character varying(64)", tipeKolom(t, db, "transaksi_stok", "id"))
		assert.Equal(t, "character varying(64)", tipeKolom(t, db, "optimasi_rute", "id"))
		assert.Equal(t, "character varying(64)", tipeKolom(t, db, "rute_distribusi", "optimasi_id"))

		// ID lama kembali persis (dari kode); baris sesudah Up memakai id::text.
		assert.EqualValues(t, 3, scanCount(t, db, `SELECT count(*) FROM cold_storage`))
		assert.EqualValues(t, 1, scanCount(t, db, `SELECT count(*) FROM cold_storage WHERE id = 'CS-01'`))
		assert.EqualValues(t, 1, scanCount(t, db, `SELECT count(*) FROM cold_storage WHERE id = 'CS-02'`))
		assert.EqualValues(t, 1, scanCount(t, db, `SELECT count(*) FROM cold_storage WHERE id = ?`, baru))
		assert.Zero(t, scanCount(t, db, `SELECT count(*) FROM cold_storage WHERE id = 'CS-03'`), "dihapus sesudah Up: tidak kembali")
		assert.Equal(t, "STK-A,STK-H,STK-LIVE,STK-M,STK-Z", scanString(t, db, `SELECT string_agg(id, ',' ORDER BY id COLLATE "C") FROM stok_cold_storage WHERE id LIKE 'STK-%'`))
		assert.EqualValues(t, 1, scanCount(t, db, `SELECT count(*) FROM stok_cold_storage WHERE id = '6'`), "lot baru tanpa kode memakai id::text")
		assert.Equal(t, "TRX-1,TRX-2,TRX-3", scanString(t, db, `SELECT string_agg(id, ',' ORDER BY id COLLATE "C") FROM transaksi_stok`))
		assert.Equal(t, "STK-A", scanString(t, db, `SELECT lot_id FROM transaksi_stok WHERE id = 'TRX-2'`))
		assert.Equal(t, "STK-M", scanString(t, db, `SELECT lot_baru_id FROM transaksi_stok WHERE id = 'TRX-2'`))
		assert.Equal(t, "CS-01", scanString(t, db, `SELECT fasilitas_asal_id FROM transaksi_stok WHERE id = 'TRX-2'`))
		assert.Equal(t, "CS-02", scanString(t, db, `SELECT fasilitas_tujuan_id FROM transaksi_stok WHERE id = 'TRX-2'`))
		assert.Equal(t, "CS-02", scanString(t, db, `SELECT cold_storage_id FROM stok_cold_storage WHERE id = 'STK-M'`))
		assert.Equal(t, "CS-02", scanString(t, db, `SELECT fasilitas_id FROM telemetri_suhu WHERE suhu_c = -19.8`))
		assert.Equal(t, "KC-A,KC-B", scanString(t, db, `SELECT string_agg(id, ',' ORDER BY id COLLATE "C") FROM kandidat_cold_storage`))
		assert.Equal(t, "OPT-000009,OPT-2", scanString(t, db, `SELECT string_agg(id, ',' ORDER BY id COLLATE "C") FROM optimasi_rute`))
		assert.Equal(t, "RT-1101-1103,RT-OPT-2-01", scanString(t, db, `SELECT string_agg(id, ',' ORDER BY id COLLATE "C") FROM rute_distribusi`))
		assert.Equal(t, "OPT-2", scanString(t, db, `SELECT optimasi_id FROM rute_distribusi WHERE id = 'RT-OPT-2-01'`))

		// neraca_wilayah kembali ke PK komposit tanpa id.
		assert.False(t, adaKolom(t, db, "neraca_wilayah", "id"))
		assert.Equal(t, "komoditas_kode,kabupaten_kode,periode", scanString(t, db, `SELECT string_agg(a.attname, ',' ORDER BY k.ord) FROM pg_index i
			CROSS JOIN LATERAL unnest(i.indkey) WITH ORDINALITY k(attnum, ord)
			JOIN pg_attribute a ON a.attrelid = i.indrelid AND a.attnum = k.attnum
			WHERE i.indrelid = 'neraca_wilayah'::regclass AND i.indisprimary`))
		assert.Zero(t, scanCount(t, db, `SELECT count(*) FROM pg_constraint WHERE conname = 'uq_neraca_wilayah_alami'`))

		// Kolom kode, constraint, dan indeks baru dibuang.
		for _, tabel := range []string{"cold_storage", "stok_cold_storage", "transaksi_stok", "kandidat_cold_storage", "optimasi_rute", "rute_distribusi"} {
			assert.False(t, adaKolom(t, db, tabel, "kode"), tabel)
		}
		for _, nama := range []string{
			"idx_cold_storage_kabupaten_kode", "idx_stok_cold_storage_komoditas_kode", "idx_stok_cold_storage_aktif",
			"idx_transaksi_stok_lot_baru", "idx_telemetri_suhu_waktu", "idx_kandidat_cold_storage_kabupaten_kode",
			"idx_optimasi_rute_komoditas_periode", "idx_rute_distribusi_asal", "idx_rute_distribusi_komoditas",
			"idx_rute_distribusi_aktif", "idx_neraca_wilayah_kabupaten_periode", "idx_jarak_antar_wilayah_tujuan",
		} {
			assert.False(t, adaIndeks(t, db, nama), nama+" harus dibuang")
		}
		for _, nama := range []string{"idx_stok_cold_storage_fasilitas", "idx_transaksi_stok_lot", "idx_transaksi_stok_tujuan", "idx_rute_distribusi_optimasi"} {
			assert.True(t, adaIndeks(t, db, nama), nama+" tetap ada")
		}

		// ON DELETE semula.
		lama := map[string]string{
			"stok_cold_storage_cold_storage_id_fkey":    "CASCADE",
			"telemetri_suhu_fasilitas_id_fkey":          "CASCADE",
			"transaksi_stok_fasilitas_asal_id_fkey":     "NO ACTION",
			"transaksi_stok_lot_id_fkey":                "NO ACTION",
			"transaksi_stok_lot_baru_id_fkey":           "NO ACTION",
			"transaksi_stok_oleh_id_fkey":               "SET NULL",
			"cold_storage_kabupaten_kode_fkey":          "NO ACTION",
			"stok_cold_storage_komoditas_kode_fkey":     "NO ACTION",
			"rute_distribusi_optimasi_id_fkey":          "NO ACTION",
			"rute_distribusi_asal_kode_fkey":            "NO ACTION",
			"neraca_wilayah_kabupaten_kode_fkey":        "NO ACTION",
			"jarak_antar_wilayah_asal_kode_fkey":        "NO ACTION",
			"wilayah_geometri_kabupaten_kode_fkey":      "NO ACTION",
			"optimasi_rute_komoditas_kode_fkey":         "NO ACTION",
			"kandidat_cold_storage_kabupaten_kode_fkey": "NO ACTION",
		}
		for nama, aturan := range lama {
			assert.Equal(t, aturan, aturanHapusFK(t, db, nama), nama)
		}
		assert.EqualValues(t, 1, scanCount(t, db, `SELECT count(*) FROM pg_constraint WHERE conname = 'chk_transaksi_stok_lokasi'`))

		// Rujukan lunak kembali ke ID lama.
		assert.Equal(t, "CS-01", scanString(t, db, `SELECT entitas_id FROM audit_log WHERE id = 'G1'`))
		assert.Equal(t, "STK-A", scanString(t, db, `SELECT entitas_id FROM audit_log WHERE id = 'G3'`))
		assert.Equal(t, "1", scanString(t, db, `SELECT entitas_id FROM audit_log WHERE id = 'G6'`), "OPT-1 dihapus sesudah Up: tidak ada baris untuk dipetakan balik, nilai tetap")
		assert.Equal(t, "STO-2292", scanString(t, db, `SELECT entitas_id FROM audit_log WHERE id = 'G4'`))
		assert.Equal(t, "O", scanString(t, db, `SELECT tgenabled::text FROM pg_trigger WHERE tgname = 'trg_audit_log_append_only'`))

		// View dan indeks uniknya ada lagi.
		assert.EqualValues(t, 1, scanCount(t, db, `SELECT count(*) FROM pg_matviews WHERE matviewname = 'mv_kpi_operasional' AND ispopulated`))
		assert.True(t, adaIndeks(t, db, "uq_mv_kpi_operasional"))
		require.NoError(t, db.Exec(`REFRESH MATERIALIZED VIEW CONCURRENTLY mv_kpi_operasional`).Error)
	})

	t.Run("up ulang atas data yang sudah dibalik menghasilkan ID yang sama untuk fasilitas", func(t *testing.T) {
		require.NoError(t, migrator.UpTo(t.Context(), versiColdSpasial), "up kedua")
		assert.Equal(t, "CS-01", scanString(t, db, `SELECT kode FROM cold_storage WHERE id = ?::uuid`, uuidFasilitas("CS-01")))
		assert.Equal(t, uuidFasilitas("CS-02"), scanString(t, db, `SELECT fasilitas_tujuan_id::text FROM transaksi_stok WHERE kode = 'TRX-2'`))
		assert.EqualValues(t, 5+1, scanCount(t, db, `SELECT count(*) FROM stok_cold_storage`), "lima lot fixture ditambah lot uji yang ikut kembali")
		assert.Equal(t, "bigint", tipeKolom(t, db, "transaksi_stok", "lot_id"))
	})
}

// TestMigrasiColdSpasial_IDTerlaluPanjangDitolakJelas: ID lama lebih panjang dari 64 karakter tidak muat di
// kolom kode, jadi migrasi berhenti dengan pesan yang jelas alih-alih memotong diam-diam.
func TestMigrasiColdSpasial_IDTerlaluPanjangDitolakJelas(t *testing.T) {
	db := pg.NewDatabase(t, "migrate_cold_spasial_panjang")
	migrator := pgtest.Migrator(t, db)
	require.NoError(t, migrator.UpTo(t.Context(), versiSebelumColdSpasial))
	exec(t, db, `INSERT INTO provinsi (kode, nama) VALUES ('11', 'Aceh')`)
	exec(t, db, `INSERT INTO kabupaten (kode, nama, jenis, ibukota, lat, lng, provinsi_kode) VALUES ('1101', 'K', 'KABUPATEN', 'K', 5, 95, '11')`)
	exec(t, db, `INSERT INTO cold_storage (id, nama, kabupaten_kode, lat, lng, kapasitas_ton, suhu_target_c, status)
		VALUES (repeat('X', 65), 'Panjang', '1101', 5, 95, 10, 4, 'RENCANA')`)

	err := migrator.UpTo(t.Context(), versiColdSpasial)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "lebih panjang dari 64 karakter")
}
