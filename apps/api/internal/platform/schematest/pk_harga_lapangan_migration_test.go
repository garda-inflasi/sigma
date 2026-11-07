package schematest

import (
	"testing"

	"github.com/garda-inflasi/api/internal/platform/pgtest"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

// Migrasi 000111 mengubah PK VARCHAR entri_harga, tugas_lapangan, batas_harga, dan perubahan_master menjadi BIGINT
// identity (ID lama pindah ke kode), mengonversi anak (riwayat_status_entri, tugas_entri), memberi FK ke tabel
// kode ON DELETE RESTRICT, dan menambah indeks. Tes dipatok ke versi 110 dan 111 (bukan "terbaru") supaya migrasi
// konversi berikutnya tidak mengubah bentuk fixture ID lama di bawah.
const (
	versiSebelumHargaLapangan = 110
	versiHargaLapangan        = 111
)

// uuidEntriAplikasi adalah ID entri yang dibuat aplikasi sebelum 000111 (uuid.NewString()).
const uuidEntriAplikasi = "3f2b8c1e-9d4a-4e6f-8a21-5c7d9e0b1a34"

// isiDataHargaLapanganLama mengisi basis data versi 110 dengan baris berbentuk data lama, hanya pada tabel milik
// grup 000111, referensi (provinsi, kabupaten, komoditas, pasar), dan pengguna. Urutan dicatat_pada dan dibuat_pada
// sengaja diacak terhadap urutan ID lama supaya urutan ID baru terbukti mengikuti kolom waktu.
func isiDataHargaLapanganLama(t *testing.T, db *gorm.DB) {
	t.Helper()
	sql := []string{
		`INSERT INTO provinsi (kode, nama) VALUES ('11', 'Aceh')`,
		`INSERT INTO kabupaten (kode, nama, jenis, ibukota, lat, lng, provinsi_kode) VALUES
			('1174', 'Lhokseumawe', 'KOTA', 'Lhokseumawe', 5.18, 97.15, '11'),
			('1171', 'Banda Aceh', 'KOTA', 'Banda Aceh', 5.55, 95.32, '11')`,
		`INSERT INTO komoditas (kode, nama, satuan, kategori, harga_acuan, shelf_life_hari) VALUES
			('CBM-KRT', 'Cabai Merah', 'kg', 'cabai', 50000, 7), ('BWM', 'Bawang Merah', 'kg', 'bawang', 40000, 14)`,
		`INSERT INTO pasar (id, nama, kabupaten_kode, lat, lng) VALUES
			('PSR-1', 'Pasar Inti', '1174', 5.18, 97.15), ('PSR-2', 'Pasar Dua', '1171', 5.55, 95.32)`,
		`INSERT INTO pengguna (id, nama, email, peran, lapis, lembaga) VALUES
			(garda_uuid_stabil('pengguna', 'petugas'), 'Petugas', 'petugas@x.id', 'OPS_MAHASISWA', 'OPERASIONAL', 'FMIPA'),
			(garda_uuid_stabil('pengguna', 'validator'), 'Validator', 'validator@x.id', 'OPS_KOORDINATOR', 'OPERASIONAL', 'BPPSDMP')`,

		// Entri: waktu pencatatan acak terhadap ID lama; dua pasang kembar untuk pemutus seri.
		`INSERT INTO entri_harga (id, komoditas_kode, pasar_id, tipe, harga, satuan, status, dicatat_pada, dibuat_pada, petugas_id, idempotency_key) VALUES
			('HRG-C', 'CBM-KRT', 'PSR-1', 'KONSUMEN', 52000, 'kg', 'DIAJUKAN', '2026-10-04 01:00+00', '2026-10-04 01:00+00', garda_uuid_stabil('pengguna', 'petugas'), 'kunci-c'),
			('HRG-B', 'CBM-KRT', 'PSR-1', 'KONSUMEN', 51000, 'kg', 'DIAJUKAN', '2026-10-02 01:00+00', '2026-10-02 02:00+00', garda_uuid_stabil('pengguna', 'petugas'), 'kunci-b'),
			('` + uuidEntriAplikasi + `', 'BWM', 'PSR-2', 'PRODUSEN', 30000, 'kg', 'DIAJUKAN', '2026-10-02 01:00+00', '2026-10-02 01:30+00', garda_uuid_stabil('pengguna', 'petugas'), 'kunci-uuid'),
			('HRG-A', 'CBM-KRT', 'PSR-2', 'KONSUMEN', 50000, 'kg', 'DIVALIDASI', '2026-10-01 01:00+00', '2026-10-01 01:00+00', garda_uuid_stabil('pengguna', 'petugas'), NULL),
			('HRG-T2', 'BWM', 'PSR-1', 'KONSUMEN', 41000, 'kg', 'DIAJUKAN', '2026-10-03 01:00+00', '2026-10-03 01:00+00', garda_uuid_stabil('pengguna', 'petugas'), NULL),
			('HRG-T1', 'BWM', 'PSR-1', 'KONSUMEN', 40500, 'kg', 'DIAJUKAN', '2026-10-03 01:00+00', '2026-10-03 01:00+00', garda_uuid_stabil('pengguna', 'petugas'), NULL)`,
		`UPDATE entri_harga SET divalidasi_oleh = garda_uuid_stabil('pengguna', 'validator'), divalidasi_pada = now() WHERE id = 'HRG-A'`,
		`INSERT INTO riwayat_status_entri (entri_id, dari, ke, oleh) VALUES
			('HRG-A', NULL, 'DIAJUKAN', garda_uuid_stabil('pengguna', 'petugas')),
			('HRG-A', 'DIAJUKAN', 'DIVALIDASI', garda_uuid_stabil('pengguna', 'validator')),
			('HRG-C', NULL, 'DIAJUKAN', garda_uuid_stabil('pengguna', 'petugas'))`,

		`INSERT INTO tugas_lapangan (id, judul, pasar_id, petugas_id, komoditas, tipe, tenggat, status, target, dibuat_pada, dibuat_oleh_id) VALUES
			('TGS-petugas-1', 'Survei', 'PSR-1', garda_uuid_stabil('pengguna', 'petugas'), ARRAY['CBM-KRT'], 'KONSUMEN', now() + interval '1 day', 'BERJALAN', 3, '2026-10-05 08:00+00', garda_uuid_stabil('pengguna', 'validator')),
			('TGS-petugas-2', 'Survei dua', 'PSR-2', garda_uuid_stabil('pengguna', 'petugas'), ARRAY['BWM'], 'PRODUSEN', now() - interval '1 day', 'SELESAI', 1, '2026-10-05 07:00+00', NULL)`,
		`INSERT INTO tugas_entri (tugas_id, entri_id) VALUES
			('TGS-petugas-1', 'HRG-A'), ('TGS-petugas-1', 'HRG-C'), ('TGS-petugas-2', 'HRG-C')`,

		// batas_harga: tidak punya kolom waktu pembuatan, urutan: berlaku_dari, komoditas, kabupaten (NULL dulu).
		`INSERT INTO batas_harga (id, komoditas_kode, kabupaten_kode, batas_min, batas_maks, berlaku_dari) VALUES
			('BH-CBM-1174', 'CBM-KRT', '1174', 20000, 150000, '2025-01-01'),
			('BH-CBM-KRT', 'CBM-KRT', NULL, 12500, 150000, '2024-01-01'),
			('BH-BWM', 'BWM', NULL, 10000, 120000, '2024-01-01')`,

		`INSERT INTO perubahan_master (id, entitas, entitas_id, versi, perubahan, status, diajukan_oleh_id, disetujui_oleh_id, dibuat_pada) VALUES
			('PM-2', 'komoditas', 'BWM', 2, '{"harga_acuan": 41000}'::jsonb, 'MENUNGGU', garda_uuid_stabil('pengguna', 'petugas'), NULL, '2026-10-02 00:00+00'),
			('PM-1', 'komoditas', 'CBM-KRT', 1, '{"harga_acuan": 51000}'::jsonb, 'DISETUJUI', garda_uuid_stabil('pengguna', 'petugas'), garda_uuid_stabil('pengguna', 'validator'), '2026-10-01 00:00+00')`,

		// Rujukan lunak: yang cocok dengan baris ikut dipetakan, sisanya tidak.
		`INSERT INTO audit_log (id, aktor_id, aksi, entitas, entitas_id, hasil) VALUES
			('G1A1', garda_uuid_stabil('pengguna', 'petugas'), 'HARGA_INPUT', 'entri_harga', 'HRG-C', 'BERHASIL'),
			('G1A2', garda_uuid_stabil('pengguna', 'petugas'), 'HARGA_INPUT', 'entri_harga', 'ENT-9999', 'BERHASIL'),
			('G1A3', garda_uuid_stabil('pengguna', 'validator'), 'HARGA_VALIDASI', 'entri_harga', '2 entri', 'BERHASIL'),
			('G1A4', garda_uuid_stabil('pengguna', 'petugas'), 'TUGAS_SELESAI', 'tugas_lapangan', 'TGS-petugas-1', 'BERHASIL'),
			('G1A5', garda_uuid_stabil('pengguna', 'petugas'), 'UBAH', 'komoditas', 'HRG-A', 'BERHASIL'),
			('G1A6', garda_uuid_stabil('pengguna', 'petugas'), 'UBAH', 'batas_harga', 'BH-BWM', 'BERHASIL'),
			('G1A7', garda_uuid_stabil('pengguna', 'petugas'), 'AJUKAN', 'perubahan_master', 'PM-1', 'BERHASIL'),
			('G1A8', garda_uuid_stabil('pengguna', 'petugas'), 'HARGA_INPUT', 'entri_harga', '` + uuidEntriAplikasi + `', 'BERHASIL')`,
	}
	for _, s := range sql {
		exec(t, db, s)
	}
}

type g1Kolom struct{ Tipe, Identitas, Bawaan string }

// g1BacaKolom membaca tipe, jenis identity, dan bawaan satu kolom.
func g1BacaKolom(t *testing.T, db *gorm.DB, tabel, kolom string) g1Kolom {
	t.Helper()
	var k g1Kolom
	require.NoError(t, db.Raw(`
		SELECT format_type(a.atttypid, a.atttypmod) AS tipe, a.attidentity::text AS identitas,
		       COALESCE(pg_get_expr(d.adbin, d.adrelid), '') AS bawaan
		  FROM pg_attribute a LEFT JOIN pg_attrdef d ON d.adrelid = a.attrelid AND d.adnum = a.attnum
		 WHERE a.attrelid = ?::regclass AND a.attname = ? AND NOT a.attisdropped`, tabel, kolom).Scan(&k).Error, "%s.%s", tabel, kolom)
	return k
}

func g1AdaKolom(t *testing.T, db *gorm.DB, tabel, kolom string) bool {
	t.Helper()
	return scanCount(t, db, `SELECT count(*) FROM pg_attribute WHERE attrelid = ?::regclass AND attname = ? AND NOT attisdropped`, tabel, kolom) > 0
}

// g1SaatHapus membaca aturan ON DELETE sebuah FK menurut namanya: CASCADE, RESTRICT, NO ACTION, SET NULL.
func g1SaatHapus(t *testing.T, db *gorm.DB, nama string) string {
	t.Helper()
	return scanString(t, db, `
		SELECT CASE confdeltype WHEN 'a' THEN 'NO ACTION' WHEN 'r' THEN 'RESTRICT' WHEN 'c' THEN 'CASCADE'
		                        WHEN 'n' THEN 'SET NULL' ELSE 'SET DEFAULT' END
		  FROM pg_constraint WHERE conname = ? AND contype = 'f'`, nama)
}

func g1AdaIndeks(t *testing.T, db *gorm.DB, nama string) bool {
	t.Helper()
	return scanCount(t, db, `SELECT count(*) FROM pg_indexes WHERE schemaname = 'public' AND indexname = ?`, nama) > 0
}

func g1DefIndeks(t *testing.T, db *gorm.DB, nama string) string {
	t.Helper()
	return scanString(t, db, `SELECT indexdef FROM pg_indexes WHERE schemaname = 'public' AND indexname = ?`, nama)
}

func g1KodeUrut(t *testing.T, db *gorm.DB, tabel string) []string {
	t.Helper()
	var kode []string
	require.NoError(t, db.Raw(`SELECT kode FROM `+tabel+` ORDER BY id`).Scan(&kode).Error)
	return kode
}

// g1IDDariKode membaca id baris menurut kode.
func g1IDDariKode(t *testing.T, db *gorm.DB, tabel, kode string) string {
	t.Helper()
	return scanString(t, db, `SELECT id::text FROM `+tabel+` WHERE kode = ?`, kode)
}

// g1HarusDitolak menjalankan SQL dan memastikan PostgreSQL menolaknya dengan kode SQLSTATE tertentu.
func g1HarusDitolak(t *testing.T, db *gorm.DB, sqlstate string, sql string, args ...any) {
	t.Helper()
	err := db.Exec(sql, args...).Error
	require.Error(t, err, sql)
	assert.Contains(t, err.Error(), sqlstate, sql)
}

var (
	g1TabelBigint = []string{"entri_harga", "tugas_lapangan", "batas_harga", "perubahan_master"}
	// g1FKRestrict adalah FK tabel grup 000111 ke tabel kode: aturan ON DELETE sebelum dan sesudah 000111.
	g1FKRestrict = []string{
		"entri_harga_pasar_id_fkey", "entri_harga_komoditas_kode_fkey", "tugas_lapangan_pasar_id_fkey",
		"batas_harga_komoditas_kode_fkey", "batas_harga_kabupaten_kode_fkey", "pasar_kabupaten_kode_fkey",
		"kabupaten_provinsi_kode_fkey",
	}
	g1IndeksBaru = []string{
		"idx_entri_harga_dicatat_brin", "idx_entri_harga_petugas_dicatat", "idx_entri_harga_antrean_skor",
		"idx_tugas_lapangan_aktif", "idx_batas_harga_kabupaten_kode", "idx_perubahan_master_menunggu",
	}
)

// TestMigrasiHargaLapanganMaster_DataLamaIkutDanBisaDibalik: 000111 memberi ID BIGINT menurut kolom waktu, memindah
// ID lama ke kode, mengonversi anak, memetakan rujukan lunak, mengganti ON DELETE, dan Down memulihkan semuanya.
func TestMigrasiHargaLapanganMaster_DataLamaIkutDanBisaDibalik(t *testing.T) {
	db := pg.NewDatabase(t, "migrate_harga_lapangan")
	migrator := pgtest.Migrator(t, db)
	require.NoError(t, migrator.UpTo(t.Context(), versiSebelumHargaLapangan))
	isiDataHargaLapanganLama(t, db)
	for _, tabel := range g1TabelBigint {
		require.Equal(t, "character varying(255)", g1BacaKolom(t, db, tabel, "id").Tipe, tabel)
	}
	require.EqualValues(t, 6, scanCount(t, db, `SELECT count(*) FROM entri_harga`))

	require.NoError(t, migrator.UpTo(t.Context(), versiHargaLapangan), "up 000111")

	t.Run("id menjadi bigint GENERATED ALWAYS dengan kode unik", func(t *testing.T) {
		for _, tabel := range g1TabelBigint {
			k := g1BacaKolom(t, db, tabel, "id")
			assert.Equal(t, "bigint", k.Tipe, tabel)
			assert.Equal(t, "a", k.Identitas, "%s: GENERATED ALWAYS AS IDENTITY", tabel)
			assert.Empty(t, k.Bawaan, "%s: tanpa default nextval lama", tabel)
			assert.Equal(t, "character varying(64)", g1BacaKolom(t, db, tabel, "kode").Tipe, tabel)
			assert.EqualValues(t, 1, scanCount(t, db, `
				SELECT count(*) FROM pg_constraint WHERE conrelid = ?::regclass AND contype = 'u' AND conname = ?`, tabel, "uq_"+tabel+"_kode"), tabel)
			assert.Equal(t, "PRIMARY KEY (id)", scanString(t, db, `SELECT pg_get_constraintdef(oid) FROM pg_constraint WHERE conrelid = ?::regclass AND contype = 'p'`, tabel), tabel)
		}
	})

	t.Run("urutan ID mengikuti kolom waktu, pemutus seri dibuat_pada lalu ID lama", func(t *testing.T) {
		// dicatat_pada: A(1/10) < uuid dan B (2/10, dibuat_pada uuid lebih awal) < T1, T2 (3/10, seri: ID lama) < C (4/10).
		assert.Equal(t, []string{"HRG-A", uuidEntriAplikasi, "HRG-B", "HRG-T1", "HRG-T2", "HRG-C"}, g1KodeUrut(t, db, "entri_harga"))
		assert.EqualValues(t, 6, scanCount(t, db, `SELECT max(id) FROM entri_harga`))
		assert.EqualValues(t, 1, scanCount(t, db, `SELECT min(id) FROM entri_harga`))
		// dibuat_pada: TGS-petugas-2 (07.00) sebelum TGS-petugas-1 (08.00).
		assert.Equal(t, []string{"TGS-petugas-2", "TGS-petugas-1"}, g1KodeUrut(t, db, "tugas_lapangan"))
		// berlaku_dari, komoditas, kabupaten NULL lebih dulu.
		assert.Equal(t, []string{"BH-BWM", "BH-CBM-KRT", "BH-CBM-1174"}, g1KodeUrut(t, db, "batas_harga"))
		assert.Equal(t, []string{"PM-1", "PM-2"}, g1KodeUrut(t, db, "perubahan_master"))
	})

	t.Run("data baris tidak berubah selain id dan kode", func(t *testing.T) {
		assert.EqualValues(t, 52000, scanCount(t, db, `SELECT harga FROM entri_harga WHERE kode = 'HRG-C'`))
		assert.Equal(t, "kunci-uuid", scanString(t, db, `SELECT idempotency_key FROM entri_harga WHERE kode = ?`, uuidEntriAplikasi))
		assert.Equal(t, uuidPengguna("petugas"), scanString(t, db, `SELECT petugas_id::text FROM entri_harga WHERE kode = 'HRG-B'`))
		assert.Equal(t, uuidPengguna("validator"), scanString(t, db, `SELECT divalidasi_oleh::text FROM entri_harga WHERE kode = 'HRG-A'`))
		assert.EqualValues(t, 3, scanCount(t, db, `SELECT target FROM tugas_lapangan WHERE kode = 'TGS-petugas-1'`))
		assert.EqualValues(t, 41000, scanCount(t, db, `SELECT (perubahan->>'harga_acuan')::int FROM perubahan_master WHERE kode = 'PM-2'`))
		assert.EqualValues(t, 150000, scanCount(t, db, `SELECT batas_maks FROM batas_harga WHERE kode = 'BH-CBM-1174'`))
	})

	t.Run("anak menunjuk ID baru yang benar", func(t *testing.T) {
		assert.Equal(t, "bigint", g1BacaKolom(t, db, "riwayat_status_entri", "entri_id").Tipe)
		assert.Equal(t, "bigint", g1BacaKolom(t, db, "tugas_entri", "entri_id").Tipe)
		assert.Equal(t, "bigint", g1BacaKolom(t, db, "tugas_entri", "tugas_id").Tipe)
		assert.Equal(t, "bigint", g1BacaKolom(t, db, "riwayat_status_entri", "id").Tipe, "PK BIGSERIAL riwayat tetap")

		assert.EqualValues(t, 2, scanCount(t, db, `
			SELECT count(*) FROM riwayat_status_entri h JOIN entri_harga e ON e.id = h.entri_id WHERE e.kode = 'HRG-A'`))
		assert.EqualValues(t, 1, scanCount(t, db, `
			SELECT count(*) FROM riwayat_status_entri h JOIN entri_harga e ON e.id = h.entri_id WHERE e.kode = 'HRG-C'`))
		assert.EqualValues(t, 3, scanCount(t, db, `SELECT count(*) FROM riwayat_status_entri`))

		pasangan := scanString(t, db, `
			SELECT string_agg(t.kode || '>' || e.kode, ',' ORDER BY t.kode, e.kode)
			  FROM tugas_entri te JOIN tugas_lapangan t ON t.id = te.tugas_id JOIN entri_harga e ON e.id = te.entri_id`)
		assert.Equal(t, "TGS-petugas-1>HRG-A,TGS-petugas-1>HRG-C,TGS-petugas-2>HRG-C", pasangan)
		assert.Equal(t, "PRIMARY KEY (tugas_id, entri_id)", scanString(t, db, `SELECT pg_get_constraintdef(oid) FROM pg_constraint WHERE conrelid = 'tugas_entri'::regclass AND contype = 'p'`))
	})

	t.Run("rujukan lunak audit_log dipetakan bila cocok, sisanya tidak", func(t *testing.T) {
		assert.Equal(t, g1IDDariKode(t, db, "entri_harga", "HRG-C"), scanString(t, db, `SELECT entitas_id FROM audit_log WHERE id = 'G1A1'`))
		assert.Equal(t, "ENT-9999", scanString(t, db, `SELECT entitas_id FROM audit_log WHERE id = 'G1A2'`), "tidak cocok dengan baris mana pun")
		assert.Equal(t, "2 entri", scanString(t, db, `SELECT entitas_id FROM audit_log WHERE id = 'G1A3'`), "bukan ID")
		assert.Equal(t, g1IDDariKode(t, db, "tugas_lapangan", "TGS-petugas-1"), scanString(t, db, `SELECT entitas_id FROM audit_log WHERE id = 'G1A4'`))
		assert.Equal(t, "HRG-A", scanString(t, db, `SELECT entitas_id FROM audit_log WHERE id = 'G1A5'`), "entitas lain tidak disentuh")
		assert.Equal(t, g1IDDariKode(t, db, "batas_harga", "BH-BWM"), scanString(t, db, `SELECT entitas_id FROM audit_log WHERE id = 'G1A6'`))
		assert.Equal(t, g1IDDariKode(t, db, "perubahan_master", "PM-1"), scanString(t, db, `SELECT entitas_id FROM audit_log WHERE id = 'G1A7'`))
		assert.Equal(t, g1IDDariKode(t, db, "entri_harga", uuidEntriAplikasi), scanString(t, db, `SELECT entitas_id FROM audit_log WHERE id = 'G1A8'`), "ID UUID buatan aplikasi")
		assert.Equal(t, "O", scanString(t, db, `SELECT tgenabled::text FROM pg_trigger WHERE tgname = 'trg_audit_log_append_only'`), "audit_log tetap append-only")
		err := db.Exec(`UPDATE audit_log SET aksi = 'DIUBAH' WHERE id = 'G1A1'`).Error
		require.Error(t, err)
		assert.Contains(t, err.Error(), "append-only")
	})

	t.Run("FK ke tabel kode RESTRICT, anak CASCADE, semuanya berindeks", func(t *testing.T) {
		for _, fk := range g1FKRestrict {
			assert.Equal(t, "RESTRICT", g1SaatHapus(t, db, fk), fk)
		}
		for _, fk := range []string{"riwayat_status_entri_entri_id_fkey", "tugas_entri_entri_id_fkey", "tugas_entri_tugas_id_fkey"} {
			assert.Equal(t, "CASCADE", g1SaatHapus(t, db, fk), fk)
		}
		// FK pengguna dari 000110 tidak disentuh.
		assert.Equal(t, "RESTRICT", g1SaatHapus(t, db, "entri_harga_petugas_id_fkey"))
		assert.Equal(t, "RESTRICT", g1SaatHapus(t, db, "perubahan_master_diajukan_oleh_id_fkey"))
		assert.EqualValues(t, 1, scanCount(t, db, `SELECT count(*) FROM pg_constraint WHERE conname = 'chk_entri_harga_pemisahan_tugas'`))
		assert.EqualValues(t, 1, scanCount(t, db, `SELECT count(*) FROM pg_constraint WHERE conname = 'uq_entri_harga_petugas_kunci'`))
	})

	t.Run("indeks baru, indeks yang memuat id dibuat ulang, indeks lama utuh", func(t *testing.T) {
		for _, nama := range g1IndeksBaru {
			assert.True(t, g1AdaIndeks(t, db, nama), nama)
		}
		assert.Contains(t, g1DefIndeks(t, db, "idx_entri_harga_dicatat_brin"), "USING brin (dicatat_pada)")
		assert.Contains(t, g1DefIndeks(t, db, "idx_entri_harga_dicatat_brin"), "pages_per_range='32'")
		assert.Contains(t, g1DefIndeks(t, db, "idx_entri_harga_dicatat"), "(dicatat_pada DESC, id)")
		assert.Contains(t, g1DefIndeks(t, db, "idx_entri_harga_antrean_skor"), "WHERE (status = 'DIAJUKAN'::status_entri)")
		assert.Contains(t, g1DefIndeks(t, db, "idx_tugas_lapangan_aktif"), "(petugas_id, tenggat)")
		assert.Contains(t, g1DefIndeks(t, db, "idx_tugas_lapangan_aktif"), "WHERE (status <> 'SELESAI'::status_tugas)")
		assert.Contains(t, g1DefIndeks(t, db, "idx_perubahan_master_menunggu"), "WHERE (status = 'MENUNGGU'::status_persetujuan)")
		for _, nama := range []string{
			"idx_entri_harga_komoditas_pasar", "idx_entri_harga_status", "idx_entri_harga_bentrok", "idx_entri_harga_antrean",
			"uq_batas_harga_berlaku", "idx_riwayat_status_entri", "idx_tugas_entri_entri", "idx_tugas_lapangan_petugas",
			"idx_tugas_lapangan_pasar_tenggat", "idx_perubahan_master_entitas",
		} {
			assert.True(t, g1AdaIndeks(t, db, nama), nama+" tetap ada")
		}
	})

	t.Run("materialized view agregat tetap hidup dan bisa disegarkan", func(t *testing.T) {
		assert.EqualValues(t, 2, scanCount(t, db, `
			SELECT count(*) FROM pg_matviews WHERE matviewname IN ('agregat_harga_harian', 'mv_kpi_operasional')`))
		exec(t, db, `REFRESH MATERIALIZED VIEW agregat_harga_harian`)
		exec(t, db, `REFRESH MATERIALIZED VIEW mv_kpi_operasional`)
		// Hanya HRG-A berstatus DIVALIDASI.
		assert.EqualValues(t, 1, scanCount(t, db, `SELECT count(*) FROM agregat_harga_harian`))
	})

	t.Run("identity berlanjut dari max+1 dan menolak id eksplisit", func(t *testing.T) {
		baru := scanString(t, db, `
			INSERT INTO entri_harga (komoditas_kode, pasar_id, tipe, harga, satuan, dicatat_pada)
			VALUES ('CBM-KRT', 'PSR-1', 'KONSUMEN', 53000, 'kg', '2026-10-06 01:00+00') RETURNING id::text`)
		assert.Equal(t, "7", baru)
		assert.EqualValues(t, 0, scanCount(t, db, `SELECT count(*) FROM entri_harga WHERE id = 7 AND kode IS NOT NULL`), "baris baru: kode NULL")
		assert.Equal(t, "3", scanString(t, db, `
			INSERT INTO tugas_lapangan (judul, pasar_id, petugas_id, komoditas, tipe, tenggat, target)
			VALUES ('Baru', 'PSR-1', garda_uuid_stabil('pengguna', 'petugas'), ARRAY['BWM'], 'KONSUMEN', now(), 1) RETURNING id::text`))
		assert.Equal(t, "4", scanString(t, db, `
			INSERT INTO batas_harga (komoditas_kode, kabupaten_kode, batas_min, batas_maks, berlaku_dari)
			VALUES ('BWM', '1174', 1000, 2000, '2026-01-01') RETURNING id::text`))
		assert.Equal(t, "3", scanString(t, db, `
			INSERT INTO perubahan_master (entitas, entitas_id, versi, perubahan, diajukan_oleh_id)
			VALUES ('komoditas', 'BWM', 3, '{}'::jsonb, garda_uuid_stabil('pengguna', 'petugas')) RETURNING id::text`))

		g1HarusDitolak(t, db, "428C9", `INSERT INTO entri_harga (id, komoditas_kode, pasar_id, tipe, harga, satuan, dicatat_pada)
			VALUES (999, 'CBM-KRT', 'PSR-1', 'KONSUMEN', 1000, 'kg', now())`)
		g1HarusDitolak(t, db, "428C9", `INSERT INTO tugas_lapangan (id, judul, pasar_id, petugas_id, komoditas, tipe, tenggat, target)
			VALUES (999, 'X', 'PSR-1', garda_uuid_stabil('pengguna', 'petugas'), ARRAY['BWM'], 'KONSUMEN', now(), 1)`)
	})

	t.Run("kode unik, banyak kode NULL diperbolehkan", func(t *testing.T) {
		g1HarusDitolak(t, db, "23505", `INSERT INTO entri_harga (kode, komoditas_kode, pasar_id, tipe, harga, satuan, dicatat_pada)
			VALUES ('HRG-A', 'CBM-KRT', 'PSR-1', 'KONSUMEN', 1000, 'kg', now())`)
		g1HarusDitolak(t, db, "23505", `INSERT INTO batas_harga (kode, komoditas_kode, kabupaten_kode, batas_min, batas_maks, berlaku_dari)
			VALUES ('BH-BWM', 'CBM-KRT', '1171', 1000, 2000, '2026-02-01')`)
		exec(t, db, `INSERT INTO entri_harga (komoditas_kode, pasar_id, tipe, harga, satuan, dicatat_pada) VALUES
			('CBM-KRT', 'PSR-1', 'KONSUMEN', 54000, 'kg', '2026-10-07 01:00+00'), ('CBM-KRT', 'PSR-1', 'KONSUMEN', 55000, 'kg', '2026-10-07 02:00+00')`)
		assert.EqualValues(t, 3, scanCount(t, db, `SELECT count(*) FROM entri_harga WHERE kode IS NULL`))
		// Kunci alami batas_harga tetap dijaga walau kode berbeda, dan bisa menjadi target ON CONFLICT untuk upsert.
		g1HarusDitolak(t, db, "23505", `INSERT INTO batas_harga (kode, komoditas_kode, kabupaten_kode, batas_min, batas_maks, berlaku_dari)
			VALUES ('BH-LAIN', 'BWM', NULL, 1000, 2000, '2024-01-01')`)
		exec(t, db, `INSERT INTO batas_harga (komoditas_kode, kabupaten_kode, batas_min, batas_maks, berlaku_dari)
			VALUES ('BWM', NULL, 10000, 130000, '2024-01-01')
			ON CONFLICT (komoditas_kode, COALESCE(kabupaten_kode, ''), berlaku_dari) DO UPDATE SET batas_maks = EXCLUDED.batas_maks`)
		assert.EqualValues(t, 130000, scanCount(t, db, `SELECT batas_maks FROM batas_harga WHERE kode = 'BH-BWM'`), "upsert menurut kunci alami memperbarui baris lama")
		assert.EqualValues(t, 1, scanCount(t, db, `SELECT count(*) FROM batas_harga WHERE komoditas_kode = 'BWM' AND kabupaten_kode IS NULL`))
	})

	t.Run("aturan hapus: RESTRICT menolak dengan 23503, CASCADE menghapus anak", func(t *testing.T) {
		g1HarusDitolak(t, db, "23503", `DELETE FROM pasar WHERE id = 'PSR-1'`)              // entri_harga, tugas_lapangan
		g1HarusDitolak(t, db, "23503", `DELETE FROM komoditas WHERE kode = 'CBM-KRT'`)      // entri_harga, batas_harga
		g1HarusDitolak(t, db, "23503", `DELETE FROM kabupaten WHERE kode = '1174'`)         // pasar, batas_harga
		g1HarusDitolak(t, db, "23503", `DELETE FROM provinsi WHERE kode = '11'`)            // kabupaten
		g1HarusDitolak(t, db, "23503", `DELETE FROM pengguna WHERE email = 'petugas@x.id'`) // 000110: bukti asal data

		// Tugas dihapus: tautan ke entri ikut hilang, entrinya tidak.
		exec(t, db, `DELETE FROM tugas_lapangan WHERE kode = 'TGS-petugas-2'`)
		assert.EqualValues(t, 2, scanCount(t, db, `SELECT count(*) FROM tugas_entri`))
		assert.EqualValues(t, 1, scanCount(t, db, `SELECT count(*) FROM entri_harga WHERE kode = 'HRG-C'`))

		// Entri dihapus: riwayat dan tautan tugas ikut hilang.
		exec(t, db, `DELETE FROM entri_harga WHERE kode = 'HRG-A'`)
		assert.Zero(t, scanCount(t, db, `SELECT count(*) FROM riwayat_status_entri h WHERE h.entri_id NOT IN (SELECT id FROM entri_harga)`))
		assert.EqualValues(t, 1, scanCount(t, db, `SELECT count(*) FROM riwayat_status_entri`), "hanya riwayat HRG-C yang tersisa")
		assert.EqualValues(t, 1, scanCount(t, db, `SELECT count(*) FROM tugas_entri`))

		// Tanpa rujukan, master bisa dihapus (RESTRICT hanya menahan yang masih dirujuk).
		exec(t, db, `INSERT INTO kabupaten (kode, nama, jenis, ibukota, lat, lng, provinsi_kode) VALUES ('1199', 'Kosong', 'KOTA', 'K', 5, 97, '11')`)
		exec(t, db, `INSERT INTO pasar (id, nama, kabupaten_kode, lat, lng) VALUES ('PSR-9', 'Kosong', '1199', 5, 97)`)
		g1HarusDitolak(t, db, "23503", `DELETE FROM kabupaten WHERE kode = '1199'`)
		exec(t, db, `DELETE FROM pasar WHERE id = 'PSR-9'`)
		exec(t, db, `DELETE FROM kabupaten WHERE kode = '1199'`)
	})

	t.Run("down memulihkan ID lama, aturan semula, dan membuang kode serta indeks baru", func(t *testing.T) {
		// Baris yang dibuat sesudah Up tidak punya kode: Down memakai id::text.
		baruID := scanString(t, db, `SELECT id::text FROM entri_harga WHERE kode IS NULL ORDER BY id LIMIT 1`)
		require.NoError(t, migrator.DownTo(t.Context(), versiSebelumHargaLapangan), "down 000111")

		for _, tabel := range g1TabelBigint {
			assert.Equal(t, "character varying(255)", g1BacaKolom(t, db, tabel, "id").Tipe, tabel)
			assert.Empty(t, g1BacaKolom(t, db, tabel, "id").Identitas, tabel)
			assert.False(t, g1AdaKolom(t, db, tabel, "kode"), "%s: kolom kode dibuang", tabel)
			assert.EqualValues(t, 1, scanCount(t, db, `SELECT count(*) FROM pg_constraint WHERE conrelid = ?::regclass AND contype = 'p' AND conname = ?`, tabel, tabel+"_pkey"), tabel)
			assert.Zero(t, scanCount(t, db, `SELECT count(*) FROM pg_class WHERE relname = ?`, "uq_"+tabel+"_kode"), "%s: unik kode dibuang", tabel)
		}
		for _, k := range [][2]string{{"riwayat_status_entri", "entri_id"}, {"tugas_entri", "entri_id"}, {"tugas_entri", "tugas_id"}} {
			assert.Equal(t, "character varying(255)", g1BacaKolom(t, db, k[0], k[1]).Tipe, k[0]+"."+k[1])
		}

		// ID lama kembali; HRG-A dan TGS-petugas-2 dihapus sesudah Up dan memang tidak kembali.
		assert.Equal(t, "HRG-C", scanString(t, db, `SELECT id FROM entri_harga WHERE id = 'HRG-C'`))
		assert.EqualValues(t, 4, scanCount(t, db, `SELECT count(*) FROM entri_harga WHERE id IN ('HRG-B', 'HRG-T1', 'HRG-T2', ?)`, uuidEntriAplikasi))
		assert.Zero(t, scanCount(t, db, `SELECT count(*) FROM entri_harga WHERE id = 'HRG-A'`))
		assert.EqualValues(t, 1, scanCount(t, db, `SELECT count(*) FROM entri_harga WHERE id = ?`, baruID), "baris tanpa kode: id::text")
		assert.Equal(t, "HRG-C", scanString(t, db, `SELECT entri_id FROM riwayat_status_entri`), "riwayat HRG-C")
		assert.Equal(t, "TGS-petugas-1>HRG-C", scanString(t, db, `SELECT tugas_id || '>' || entri_id FROM tugas_entri`))
		assert.Equal(t, "TGS-petugas-1", scanString(t, db, `SELECT id FROM tugas_lapangan WHERE id = 'TGS-petugas-1'`))
		assert.Equal(t, "BH-BWM", scanString(t, db, `SELECT id FROM batas_harga WHERE id = 'BH-BWM'`))
		assert.Equal(t, "PM-1", scanString(t, db, `SELECT id FROM perubahan_master WHERE id = 'PM-1'`))

		// Rujukan lunak kembali ke ID lama.
		assert.Equal(t, "HRG-C", scanString(t, db, `SELECT entitas_id FROM audit_log WHERE id = 'G1A1'`))
		assert.Equal(t, "ENT-9999", scanString(t, db, `SELECT entitas_id FROM audit_log WHERE id = 'G1A2'`))
		assert.Equal(t, "TGS-petugas-1", scanString(t, db, `SELECT entitas_id FROM audit_log WHERE id = 'G1A4'`))
		assert.Equal(t, "BH-BWM", scanString(t, db, `SELECT entitas_id FROM audit_log WHERE id = 'G1A6'`))
		assert.Equal(t, "PM-1", scanString(t, db, `SELECT entitas_id FROM audit_log WHERE id = 'G1A7'`))
		assert.Equal(t, uuidEntriAplikasi, scanString(t, db, `SELECT entitas_id FROM audit_log WHERE id = 'G1A8'`))
		assert.Equal(t, "O", scanString(t, db, `SELECT tgenabled::text FROM pg_trigger WHERE tgname = 'trg_audit_log_append_only'`))

		// Aturan ON DELETE semula: NO ACTION untuk FK ke tabel kode, CASCADE untuk anak.
		for _, fk := range g1FKRestrict {
			assert.Equal(t, "NO ACTION", g1SaatHapus(t, db, fk), fk)
		}
		for _, fk := range []string{"riwayat_status_entri_entri_id_fkey", "tugas_entri_entri_id_fkey", "tugas_entri_tugas_id_fkey"} {
			assert.Equal(t, "CASCADE", g1SaatHapus(t, db, fk), fk)
		}
		for _, nama := range g1IndeksBaru {
			assert.False(t, g1AdaIndeks(t, db, nama), nama+" dibuang")
		}
		assert.Contains(t, g1DefIndeks(t, db, "idx_entri_harga_dicatat"), "(dicatat_pada DESC, id)")
		assert.EqualValues(t, 0, scanCount(t, db, `SELECT count(*) FROM pg_proc WHERE proname LIKE 'g1_id_%'`), "fungsi bantu dibuang")
		assert.Equal(t, "RESTRICT", g1SaatHapus(t, db, "entri_harga_petugas_id_fkey"), "FK pengguna 000110 utuh")
	})

	t.Run("up ulang atas data yang sudah dibalik berhasil dan ID tetap berurutan menurut waktu", func(t *testing.T) {
		require.NoError(t, migrator.UpTo(t.Context(), versiHargaLapangan), "up kedua")
		assert.Equal(t, "bigint", g1BacaKolom(t, db, "entri_harga", "id").Tipe)
		// HRG-A sudah dihapus: urutan waktu sisa B (uuid lebih dulu), T1, T2, C, lalu baris-baris sesudah Up.
		kode := g1KodeUrut(t, db, "entri_harga")
		assert.Equal(t, []string{uuidEntriAplikasi, "HRG-B", "HRG-T1", "HRG-T2", "HRG-C"}, kode[:5])
		assert.EqualValues(t, len(kode), scanCount(t, db, `SELECT max(id) FROM entri_harga`), "ID rapat dari 1")
		assert.EqualValues(t, 1, scanCount(t, db, `SELECT count(*) FROM riwayat_status_entri h JOIN entri_harga e ON e.id = h.entri_id WHERE e.kode = 'HRG-C'`))
		assert.Equal(t, g1IDDariKode(t, db, "entri_harga", "HRG-C"), scanString(t, db, `SELECT entitas_id FROM audit_log WHERE id = 'G1A1'`))
		for _, nama := range g1IndeksBaru {
			assert.True(t, g1AdaIndeks(t, db, nama), nama)
		}
	})
}

// TestMigrasiHargaLapangan_TabelKosong: basis data baru (tanpa baris) menerapkan Up dan Down, identity
// mulai dari 1, dan insert pertama tidak bentrok.
func TestMigrasiHargaLapangan_TabelKosong(t *testing.T) {
	db := pg.NewDatabase(t, "migrate_harga_lapangan_kosong")
	migrator := pgtest.Migrator(t, db)
	require.NoError(t, migrator.UpTo(t.Context(), versiHargaLapangan))

	exec(t, db, `INSERT INTO provinsi (kode, nama) VALUES ('11', 'Aceh')`)
	exec(t, db, `INSERT INTO kabupaten (kode, nama, jenis, ibukota, lat, lng, provinsi_kode) VALUES ('1174', 'L', 'KOTA', 'L', 5, 97, '11')`)
	exec(t, db, `INSERT INTO pasar (id, nama, kabupaten_kode, lat, lng) VALUES ('PSR-1', 'P', '1174', 5, 97)`)
	exec(t, db, `INSERT INTO komoditas (kode, nama, satuan, kategori, harga_acuan, shelf_life_hari) VALUES ('K', 'K', 'kg', 'x', 1, 1)`)
	assert.Equal(t, "1", scanString(t, db, `
		INSERT INTO entri_harga (komoditas_kode, pasar_id, tipe, harga, satuan, dicatat_pada)
		VALUES ('K', 'PSR-1', 'KONSUMEN', 1, 'kg', now()) RETURNING id::text`))
	assert.Equal(t, "1", scanString(t, db, `
		INSERT INTO batas_harga (komoditas_kode, batas_min, batas_maks, berlaku_dari) VALUES ('K', 1, 2, '2026-01-01') RETURNING id::text`))

	require.NoError(t, migrator.DownTo(t.Context(), versiSebelumHargaLapangan))
	assert.Equal(t, "character varying(255)", g1BacaKolom(t, db, "entri_harga", "id").Tipe)
	assert.EqualValues(t, 1, scanCount(t, db, `SELECT count(*) FROM entri_harga WHERE id = '1'`), "baris tanpa kode memakai id::text")
	require.NoError(t, migrator.UpTo(t.Context(), versiHargaLapangan))
	assert.Equal(t, "1", scanString(t, db, `SELECT kode FROM entri_harga`), "kode = ID lama hasil pemulihan")
}
