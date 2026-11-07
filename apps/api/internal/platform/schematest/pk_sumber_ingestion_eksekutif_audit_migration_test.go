package schematest

import (
	"fmt"
	"testing"

	"github.com/garda-inflasi/api/internal/platform/pgtest"
	"github.com/garda-inflasi/api/pkg/idx"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

// Migrasi 000115 mengubah PK sumber data, ingestion, eksekutif, siklus, KPI, dashboard, dan audit. Tes dipatok
// ke versi 114 dan 115 (bukan "terbaru") supaya migrasi konversi berikutnya tidak mengubah bentuk fixture lama.
const (
	versiSebelumPKSumberAudit = 114
	versiPKSumberAudit        = 115
)

func g5UUIDSumber(kode string) string { return idx.UUIDStabil("sumber_data", kode) }

func g5ID(t *testing.T, db *gorm.DB, sql string, args ...any) int64 {
	t.Helper()
	return scanCount(t, db, sql, args...)
}

// g5IsiDataLama mengisi basis data versi 114 dengan baris berbentuk data lama pada tabel milik 000115, ditambah
// tabel referensi dan pengguna. ID pengguna sudah UUID sejak 000110.
func g5IsiDataLama(t *testing.T, db *gorm.DB) {
	t.Helper()
	sql := []string{
		`INSERT INTO provinsi (kode, nama) VALUES ('11', 'Aceh')`,
		`INSERT INTO kabupaten (kode, nama, jenis, ibukota, lat, lng, provinsi_kode) VALUES
			('1171', 'Banda Aceh', 'KOTA', 'Banda Aceh', 5.55, 95.32, '11'),
			('1174', 'Lhokseumawe', 'KOTA', 'Lhokseumawe', 5.18, 97.15, '11')`,
		`INSERT INTO komoditas (kode, nama, satuan, kategori, harga_acuan, shelf_life_hari) VALUES
			('CBM-KRT', 'Cabai Merah', 'kg', 'cabai', 50000, 7), ('BRS-MED', 'Beras Medium', 'kg', 'beras', 13000, 90)`,
		fmt.Sprintf(`INSERT INTO pengguna (id, nama, email, peran, lapis, lembaga) VALUES
			('%s', 'Super Admin', 'superadmin@x.id', 'ADMIN_SISTEM', 'SISTEM', 'BPPSDMP'),
			('%s', 'Manajer Data', 'manajer@x.id', 'OPS_MAHASISWA', 'OPERASIONAL', 'FMIPA')`, uuidPengguna("superadmin"), uuidPengguna("manajer_data")),
		// Pemetaan ID lama pengguna (dibuat 000110 untuk akun yang ada saat itu): dipakai memetakan rujukan lunak audit.
		fmt.Sprintf(`INSERT INTO pengguna_id_lama (id, id_lama) VALUES ('%s', 'superadmin'), ('%s', 'manajer_data')`, uuidPengguna("superadmin"), uuidPengguna("manajer_data")),

		// Sumber data (varchar) dan turunannya.
		`INSERT INTO sumber_data (id, nama, penyedia, jenis, cakupan, jadwal_cron, atribusi, konfigurasi) VALUES
			('SRC-BI', 'PIHPS', 'BI', 'API', 'Aceh', '0 10 * * *', 'atribusi BI', '{"province_id":"7"}'),
			('SRC-BMKG', 'BMKG', 'BMKG', 'API', 'Aceh', '0 */3 * * *', 'atribusi BMKG', '{}'),
			('SRC-LAMA', 'Tanpa anak', 'X', 'SCRAPER', 'Aceh', '0 12 * * *', 'atribusi', '{}')`,
		fmt.Sprintf(`INSERT INTO eksekusi_sumber (sumber_id, pemicu, aktor_id, status, mulai, selesai) VALUES
			('SRC-BI', 'JADWAL', NULL, 'SUKSES', '2026-03-01 03:00+00', '2026-03-01 03:01+00'),
			('SRC-BI', 'MANUAL', '%s', 'SUKSES', '2026-03-02 03:00+00', '2026-03-02 03:01+00'),
			('SRC-BMKG', 'CLI', NULL, 'SUKSES', '2026-03-03 03:00+00', '2026-03-03 03:01+00')`, uuidPengguna("superadmin")),
		`INSERT INTO eksekusi_sumber (sumber_id, pemicu, status, mulai) VALUES ('SRC-BI', 'JADWAL', 'BERJALAN', '2026-03-04 03:00+00')`,
		`INSERT INTO data_mentah (sumber_id, eksekusi_id, kunci_permintaan, diambil_pada, payload, checksum)
			SELECT 'SRC-BI', min(id), 'wilayah=101', '2026-03-01 03:00:30+00', '{"a":1}'::jsonb, repeat('a', 64) FROM eksekusi_sumber WHERE sumber_id = 'SRC-BI'`,
		`INSERT INTO data_mentah (sumber_id, eksekusi_id, kunci_permintaan, diambil_pada, payload, checksum)
			SELECT 'SRC-BMKG', id, 'wilayah=x', '2026-03-03 03:00:30+00', '{"b":2}'::jsonb, repeat('b', 64) FROM eksekusi_sumber WHERE sumber_id = 'SRC-BMKG'`,
		`INSERT INTO pemetaan_sumber (sumber_id, jenis, alias, kode_master) VALUES
			('SRC-BI', 'WILAYAH', '101', '1171'), ('SRC-BI', 'KOMODITAS', 'beras', 'BRS-MED'), ('SRC-BMKG', 'WILAYAH', '11.71.01.2001', '1171')`,
		`INSERT INTO harga_eksternal (sumber_id, komoditas_kode, kabupaten_kode, tanggal, tipe, harga, satuan, data_mentah_id, eksekusi_id)
			SELECT 'SRC-BI', 'CBM-KRT', '1171', d, 'KONSUMEN', 50000, 'kg', m.id, m.eksekusi_id
			  FROM data_mentah m, (VALUES ('2026-01-01'::date), ('2026-01-02')) t(d) WHERE m.sumber_id = 'SRC-BI'`,
		`INSERT INTO harga_eksternal (sumber_id, komoditas_kode, kabupaten_kode, tanggal, tipe, harga, satuan)
			VALUES ('SRC-BI', 'BRS-MED', '1174', '2026-01-01', 'KONSUMEN', 13000, 'kg')`,
		`INSERT INTO cuaca_harian (id, kabupaten_kode, tanggal, curah_hujan_mm, suhu_c, kelembapan, anomali, sumber, eksekusi_id, data_mentah_id)
			SELECT x.id, x.kab, x.tgl::date, 5, 27, 80, false, 'BMKG', m.eksekusi_id, m.id
			  FROM data_mentah m, (VALUES ('CUA-1171-20260102-BMKG', '1171', '2026-01-02'), ('CUA-1174-20260101-BMKG', '1174', '2026-01-01')) x(id, kab, tgl)
			 WHERE m.sumber_id = 'SRC-BMKG'`,
		`INSERT INTO cuaca_harian (id, kabupaten_kode, tanggal, curah_hujan_mm, suhu_c, kelembapan, sumber)
			VALUES ('CUA-1171-20260101-BMKG', '1171', '2026-01-01', 0, 26, 70, 'BMKG')`,

		// Agregat eksekutif: PK komposit alami.
		`INSERT INTO inflasi_bulanan (wilayah_kode, periode, ihk, inflasi_mtm, inflasi_yoy, inflasi_ytd, sumber) VALUES
			('11', '2026-01-01', 110, 0.1, 2.5, 0.1, 'DEMO'), ('11', '2026-02-01', 110.5, 0.4, 2.6, 0.5, 'BPS'), ('1171', '2026-01-01', 109, 0.2, 2.4, 0.2, 'BPS')`,
		`INSERT INTO andil_inflasi (periode, wilayah_kode, komoditas_kode, andil, inflasi_pct, sumber) VALUES
			('2026-01-01', '11', 'CBM-KRT', 0.18, 14.2, 'DEMO'), ('2026-01-01', '11', 'BRS-MED', 0.05, 2, 'DEMO'), ('2026-02-01', '11', 'CBM-KRT', 0.1, 5, 'BPS')`,
		`INSERT INTO neraca_pangan (komoditas_kode, wilayah_kode, periode, produksi_ton, konsumsi_ton, stok_ton, sumber) VALUES
			('BRS-MED', '11', '2026-01-01', 100, 50, NULL, 'DEMO'), ('CBM-KRT', '11', '2026-01-01', 10, 20, 5, 'BPS')`,
		`INSERT INTO ruang_fiskal (id, tahun, instrumen, pagu, realisasi, per_tanggal, sumber, diimpor_pada) VALUES
			('RF-B', 2026, 'BTT', 100, 10, '2026-01-31', 'DJPB', '2026-02-01 00:00+00'),
			('RF-A', 2026, 'OPS', 50, 5, '2026-01-31', 'DJPB', '2026-02-01 00:00+00'),
			('RF-C', 2026, 'TANAM', 20, 2, '2026-01-31', 'DEMO', '2026-03-01 00:00+00')`,
		`INSERT INTO siklus_ekonomi (id, wilayah_kode, siklus, mulai_pada, akhir_pada, dibuat_pada) VALUES
			('SE-11-LAMA', '11', 'NORMAL', '2026-01-01 00:00+00', '2026-02-01 00:00+00', '2026-01-01 00:00+00'),
			('SE-11-DEMO', '11', 'TEKANAN', '2026-02-01 00:00+00', NULL, '2026-02-01 00:00+00')`,
		`INSERT INTO target_kpi (indikator, periode_mulai, target, arah) VALUES
			('validasi', '2026-01-01', 85, 'NAIK'), ('waspada', '2026-01-01', 3, 'TURUN'), ('validasi', '2026-07-01', 90, 'NAIK')`,

		// KPI (BIGSERIAL tetap).
		`INSERT INTO kpi_definisi (kode, kelompok, nama, deskripsi, rumus, arah_baik) VALUES ('LOG-UTILISASI-CS', 'LOGISTIK', 'Utilisasi', 'd', 'r', 'NAIK')`,
		`INSERT INTO kpi_nilai (kpi_kode, wilayah_kode, periode, versi, nilai) VALUES
			('LOG-UTILISASI-CS', '1171', '2026-01-01', 1, 80), ('LOG-UTILISASI-CS', NULL, '2026-01-01', 1, 70)`,
		`INSERT INTO kpi_target (kpi_kode, wilayah_kode, periode_mulai, target) VALUES
			('LOG-UTILISASI-CS', '1171', '2026-01-01', 75), ('LOG-UTILISASI-CS', NULL, '2026-01-01', 70)`,

		// Audit: ID lama berupa teks. Waktu aud-2 dan aud-2b sama (pemutus seri: ID lama). Rujukan lunak:
		// sumber_data (kode lama), langganan_push/preferensi_notifikasi (slug pengguna lama), dan baris yang tidak
		// boleh disentuh (bukan ID, entitas milik grup lain, kpi_nilai berisi rentang periode).
		fmt.Sprintf(`INSERT INTO audit_log (id, waktu, aktor_id, aksi, entitas, entitas_id, hasil) VALUES
			('aud-3',  '2026-03-03 00:00+00', '%s', 'JALANKAN_SUMBER', 'sumber_data', 'SRC-BI', 'BERHASIL'),
			('aud-1',  '2026-03-01 00:00+00', NULL, 'LOGIN', 'pengguna', 'x', 'GAGAL'),
			('aud-2',  '2026-03-02 00:00+00', '%s', 'BULK_jeda', 'sumber_data', '2 sumber', 'BERHASIL'),
			('aud-2b', '2026-03-02 00:00+00', NULL, 'UBAH', 'langganan_push', 'manajer_data', 'BERHASIL'),
			('aud-4',  '2026-03-04 00:00+00', NULL, 'UBAH', 'preferensi_notifikasi', 'manajer_data', 'BERHASIL'),
			('aud-5',  '2026-03-05 00:00+00', NULL, 'UBAH', 'langganan_push', 'tidak-ada', 'BERHASIL'),
			('aud-6',  '2026-03-06 00:00+00', NULL, 'HITUNG_KPI', 'kpi_nilai', '2026-01..2026-03', 'BERHASIL'),
			('aud-7',  '2026-03-07 00:00+00', NULL, 'JALANKAN_SUMBER', 'sumber_data', 'SRC-HILANG', 'DITOLAK'),
			('aud-8',  '2026-03-08 00:00+00', NULL, 'HARGA_INPUT', 'entri_harga', 'SRC-BI', 'BERHASIL'),
			('aud-9',  '2026-03-09 00:00+00', NULL, 'JALANKAN_SUMBER', 'sumber_data', 'SRC-BMKG', 'BERHASIL'),
			('0a1b2c3d-1111-4222-8333-444455556666', '2026-03-10 00:00+00', NULL, 'EKSPOR_DATA', 'audit_log', 'export', 'BERHASIL')`,
			uuidPengguna("superadmin"), uuidPengguna("superadmin")),
	}
	for _, s := range sql {
		exec(t, db, s)
	}
}

type g5Kolom struct {
	Tipe, Identitas string
}

func g5BacaKolom(t *testing.T, db *gorm.DB, tabel, kolom string) g5Kolom {
	t.Helper()
	var k g5Kolom
	require.NoError(t, db.Raw(`
		SELECT format_type(atttypid, atttypmod) AS tipe, attidentity::text AS identitas
		  FROM pg_attribute WHERE attrelid = ?::regclass AND attname = ? AND NOT attisdropped`, tabel, kolom).Scan(&k).Error, "%s.%s", tabel, kolom)
	return k
}

func g5KolomAda(t *testing.T, db *gorm.DB, tabel, kolom string) bool {
	t.Helper()
	return scanCount(t, db, `SELECT count(*) FROM pg_attribute WHERE attrelid = ?::regclass AND attname = ? AND NOT attisdropped`, tabel, kolom) > 0
}

func g5PK(t *testing.T, db *gorm.DB, tabel string) string {
	t.Helper()
	return scanString(t, db, `
		SELECT string_agg(a.attname, ',' ORDER BY k.ord)
		  FROM pg_index i
		  CROSS JOIN LATERAL unnest(i.indkey) WITH ORDINALITY k(attnum, ord)
		  JOIN pg_attribute a ON a.attrelid = i.indrelid AND a.attnum = k.attnum
		 WHERE i.indrelid = ?::regclass AND i.indisprimary`, tabel)
}

// g5AturanHapus membaca ON DELETE tiap FK pada tabel milik 000115: kunci "tabel.kolom".
func g5AturanHapus(t *testing.T, db *gorm.DB) map[string]string {
	t.Helper()
	var rows []struct{ Kunci, Aturan string }
	require.NoError(t, db.Raw(`
		SELECT con.conrelid::regclass::text || '.' || a.attname AS kunci,
		       CASE con.confdeltype WHEN 'a' THEN 'NO ACTION' WHEN 'r' THEN 'RESTRICT' WHEN 'c' THEN 'CASCADE'
		                            WHEN 'n' THEN 'SET NULL' ELSE 'SET DEFAULT' END AS aturan
		  FROM pg_constraint con
		  JOIN pg_attribute a ON a.attrelid = con.conrelid AND a.attnum = con.conkey[1]
		 WHERE con.contype = 'f' AND con.conrelid::regclass::text = ANY (ARRAY['eksekusi_sumber', 'data_mentah', 'pemetaan_sumber',
		       'harga_eksternal', 'cuaca_harian', 'andil_inflasi', 'neraca_pangan', 'siklus_ekonomi', 'kpi_nilai', 'kpi_target'])`).Scan(&rows).Error)
	out := map[string]string{}
	for _, r := range rows {
		out[r.Kunci] = r.Aturan
	}
	return out
}

var g5AturanBaru = map[string]string{
	"eksekusi_sumber.sumber_id":      "CASCADE",
	"eksekusi_sumber.aktor_id":       "SET NULL", // dari 000110, tidak diubah
	"data_mentah.sumber_id":          "CASCADE",
	"data_mentah.eksekusi_id":        "CASCADE",
	"pemetaan_sumber.sumber_id":      "CASCADE",
	"harga_eksternal.sumber_id":      "RESTRICT",
	"harga_eksternal.komoditas_kode": "RESTRICT",
	"harga_eksternal.kabupaten_kode": "RESTRICT",
	"harga_eksternal.data_mentah_id": "SET NULL",
	"harga_eksternal.eksekusi_id":    "SET NULL",
	"cuaca_harian.kabupaten_kode":    "RESTRICT",
	"cuaca_harian.eksekusi_id":       "SET NULL",
	"cuaca_harian.data_mentah_id":    "SET NULL",
	"andil_inflasi.komoditas_kode":   "RESTRICT",
	"neraca_pangan.komoditas_kode":   "RESTRICT",
	"siklus_ekonomi.wilayah_kode":    "RESTRICT",
	"kpi_nilai.kpi_kode":             "CASCADE", // tidak diubah
	"kpi_nilai.wilayah_kode":         "RESTRICT",
	"kpi_target.kpi_kode":            "CASCADE", // tidak diubah
	"kpi_target.wilayah_kode":        "RESTRICT",
}

func g5IndeksAda(t *testing.T, db *gorm.DB, nama string) bool {
	t.Helper()
	return scanCount(t, db, `SELECT count(*) FROM pg_class WHERE relname = ? AND relkind = 'i'`, nama) > 0
}

// TestMigrasiPKSumberAudit_DataLamaIkutDanBisaDibalik: 000115 mengubah ID di sepuluh tabel, memetakan FK anak dan
// rujukan lunak audit_log, mengurutkan ID BIGINT menurut kolom waktu, dan Down memulihkan semuanya.
func TestMigrasiPKSumberAudit_DataLamaIkutDanBisaDibalik(t *testing.T) {
	db := pg.NewDatabase(t, "migrate_pk_sumber_audit")
	migrator := pgtest.Migrator(t, db)
	require.NoError(t, migrator.UpTo(t.Context(), versiSebelumPKSumberAudit))
	g5IsiDataLama(t, db)
	require.Equal(t, "character varying(255)", g5BacaKolom(t, db, "sumber_data", "id").Tipe)
	require.Equal(t, "character varying(255)", g5BacaKolom(t, db, "audit_log", "id").Tipe)

	require.NoError(t, migrator.UpTo(t.Context(), versiPKSumberAudit), "up 000115")

	t.Run("sumber_data: id uuid stabil, kode = ID lama, bawaan acak", func(t *testing.T) {
		assert.Equal(t, "uuid", g5BacaKolom(t, db, "sumber_data", "id").Tipe)
		assert.Equal(t, "gen_random_uuid()", scanString(t, db, `
			SELECT pg_get_expr(adbin, adrelid) FROM pg_attrdef WHERE adrelid = 'sumber_data'::regclass AND adnum =
			   (SELECT attnum FROM pg_attribute WHERE attrelid = 'sumber_data'::regclass AND attname = 'id')`))
		for _, kode := range []string{"SRC-BI", "SRC-BMKG", "SRC-LAMA"} {
			assert.Equal(t, g5UUIDSumber(kode), scanString(t, db, `SELECT id::text FROM sumber_data WHERE kode = ?`, kode), kode)
		}
		assert.EqualValues(t, 3, scanCount(t, db, `SELECT count(*) FROM sumber_data`))
		assert.Equal(t, `{"province_id": "7"}`, scanString(t, db, `SELECT konfigurasi::text FROM sumber_data WHERE kode = 'SRC-BI'`), "kolom lain utuh")
		// kode NOT NULL dan UNIQUE.
		err := db.Exec(`INSERT INTO sumber_data (nama, penyedia, jenis, cakupan, jadwal_cron, atribusi) VALUES ('x', 'x', 'API', 'x', '* * * * *', 'x')`).Error
		require.Error(t, err, "kode wajib")
		err = db.Exec(`INSERT INTO sumber_data (kode, nama, penyedia, jenis, cakupan, jadwal_cron, atribusi) VALUES ('SRC-BI', 'x', 'x', 'API', 'x', '* * * * *', 'x')`).Error
		require.Error(t, err)
		assert.Contains(t, err.Error(), "uq_sumber_data_kode")
	})

	t.Run("FK sumber_id anak ikut ke UUID yang sama", func(t *testing.T) {
		for _, tabel := range []string{"eksekusi_sumber", "data_mentah", "pemetaan_sumber", "harga_eksternal"} {
			assert.Equal(t, "uuid", g5BacaKolom(t, db, tabel, "sumber_id").Tipe, tabel)
		}
		assert.EqualValues(t, 3, scanCount(t, db, `SELECT count(*) FROM eksekusi_sumber WHERE sumber_id = ?`, g5UUIDSumber("SRC-BI")))
		assert.EqualValues(t, 1, scanCount(t, db, `SELECT count(*) FROM eksekusi_sumber WHERE sumber_id = ?`, g5UUIDSumber("SRC-BMKG")))
		assert.EqualValues(t, 1, scanCount(t, db, `SELECT count(*) FROM data_mentah WHERE sumber_id = ?`, g5UUIDSumber("SRC-BI")))
		assert.EqualValues(t, 2, scanCount(t, db, `SELECT count(*) FROM pemetaan_sumber WHERE sumber_id = ?`, g5UUIDSumber("SRC-BI")))
		assert.EqualValues(t, 3, scanCount(t, db, `SELECT count(*) FROM harga_eksternal WHERE sumber_id = ?`, g5UUIDSumber("SRC-BI")))
		assert.EqualValues(t, 1, scanCount(t, db, `SELECT count(*) FROM eksekusi_sumber WHERE aktor_id = ?`, uuidPengguna("superadmin")), "aktor (uuid dari 000110) tetap")
		assert.Equal(t, "SRC-BI", scanString(t, db, `SELECT s.kode FROM harga_eksternal h JOIN sumber_data s ON s.id = h.sumber_id LIMIT 1`))
	})

	t.Run("eksekusi_sumber dan data_mentah tetap BIGSERIAL; lineage utuh", func(t *testing.T) {
		assert.Equal(t, "bigint", g5BacaKolom(t, db, "eksekusi_sumber", "id").Tipe)
		assert.Empty(t, g5BacaKolom(t, db, "eksekusi_sumber", "id").Identitas)
		assert.EqualValues(t, 4, scanCount(t, db, `SELECT count(*) FROM eksekusi_sumber`))
		assert.EqualValues(t, 2, scanCount(t, db, `SELECT count(*) FROM harga_eksternal WHERE data_mentah_id IS NOT NULL AND eksekusi_id IS NOT NULL`))
		assert.EqualValues(t, 2, scanCount(t, db, `SELECT count(*) FROM cuaca_harian WHERE data_mentah_id IS NOT NULL AND eksekusi_id IS NOT NULL`))
		// Indeks parsial eksekusi berjalan tetap menolak run kedua dan kini bertipe uuid.
		err := db.Exec(`INSERT INTO eksekusi_sumber (sumber_id, pemicu, status) VALUES (?, 'MANUAL', 'BERJALAN')`, g5UUIDSumber("SRC-BI")).Error
		require.Error(t, err)
		assert.Contains(t, err.Error(), "uq_eksekusi_sumber_berjalan")
	})

	t.Run("cuaca_harian: ID BIGINT menurut tanggal lalu ID lama, kode = ID lama", func(t *testing.T) {
		assert.Equal(t, "bigint", g5BacaKolom(t, db, "cuaca_harian", "id").Tipe)
		assert.Equal(t, "a", g5BacaKolom(t, db, "cuaca_harian", "id").Identitas)
		urut := scanString(t, db, `SELECT string_agg(kode, ',' ORDER BY id) FROM cuaca_harian`)
		assert.Equal(t, "CUA-1171-20260101-BMKG,CUA-1174-20260101-BMKG,CUA-1171-20260102-BMKG", urut)
		assert.Equal(t, "1,2,3", scanString(t, db, `SELECT string_agg(id::text, ',' ORDER BY id) FROM cuaca_harian`))
		assert.Equal(t, "kabupaten_kode,tanggal,sumber", scanString(t, db, `
			SELECT string_agg(a.attname, ',' ORDER BY k.ord) FROM pg_constraint c
			  CROSS JOIN LATERAL unnest(c.conkey) WITH ORDINALITY k(n, ord) JOIN pg_attribute a ON a.attrelid = c.conrelid AND a.attnum = k.n
			 WHERE c.conrelid = 'cuaca_harian'::regclass AND c.conname = 'cuaca_harian_kabupaten_kode_tanggal_sumber_key'`), "kunci alami (target upsert) tetap UNIQUE")
	})

	t.Run("ruang_fiskal dan siklus_ekonomi: ID BIGINT menurut waktu impor/dibuat lalu ID lama", func(t *testing.T) {
		assert.Equal(t, "RF-A,RF-B,RF-C", scanString(t, db, `SELECT string_agg(kode, ',' ORDER BY id) FROM ruang_fiskal`),
			"diimpor_pada sama untuk RF-A dan RF-B: ID lama memutus seri")
		assert.Equal(t, "SE-11-LAMA,SE-11-DEMO", scanString(t, db, `SELECT string_agg(kode, ',' ORDER BY id) FROM siklus_ekonomi`))
		assert.Equal(t, "1,2,3", scanString(t, db, `SELECT string_agg(id::text, ',' ORDER BY id) FROM ruang_fiskal`))
		assert.Equal(t, "RF-C", scanString(t, db, `SELECT kode FROM ruang_fiskal WHERE sumber = 'DEMO'`))
		assert.EqualValues(t, 1, scanCount(t, db, `SELECT count(*) FROM siklus_ekonomi WHERE akhir_pada IS NULL`), "siklus aktif tetap satu")
	})

	t.Run("PK komposit alami menjadi BIGINT id + UNIQUE alami", func(t *testing.T) {
		kasus := []struct{ tabel, alami string }{
			{"harga_eksternal", "sumber_id,komoditas_kode,kabupaten_kode,tanggal,tipe"},
			{"andil_inflasi", "periode,wilayah_kode,komoditas_kode"},
			{"inflasi_bulanan", "wilayah_kode,periode"},
			{"neraca_pangan", "komoditas_kode,wilayah_kode,periode"},
			{"target_kpi", "indikator,periode_mulai"},
		}
		jumlah := map[string]int64{"harga_eksternal": 3, "andil_inflasi": 3, "inflasi_bulanan": 3, "neraca_pangan": 2, "target_kpi": 3}
		for _, k := range kasus {
			assert.Equal(t, "id", g5PK(t, db, k.tabel), k.tabel)
			assert.Equal(t, "bigint", g5BacaKolom(t, db, k.tabel, "id").Tipe, k.tabel)
			assert.Equal(t, "a", g5BacaKolom(t, db, k.tabel, "id").Identitas, k.tabel)
			assert.Equal(t, k.alami, scanString(t, db, `
				SELECT string_agg(a.attname, ',' ORDER BY k.ord) FROM pg_constraint c
				  CROSS JOIN LATERAL unnest(c.conkey) WITH ORDINALITY k(n, ord) JOIN pg_attribute a ON a.attrelid = c.conrelid AND a.attnum = k.n
				 WHERE c.conrelid = ?::regclass AND c.conname = ?`, k.tabel, "uq_"+k.tabel+"_alami"), k.tabel)
			assert.Equal(t, jumlah[k.tabel], scanCount(t, db, `SELECT count(*) FROM `+k.tabel), k.tabel)
			assert.Equal(t, jumlah[k.tabel], scanCount(t, db, `SELECT count(DISTINCT id) FROM `+k.tabel), "ID unik: "+k.tabel)
		}
		// Kolom turunan (stored generated) ikut terhitung.
		assert.EqualValues(t, 50, scanCount(t, db, `SELECT surplus_ton::int FROM neraca_pangan WHERE komoditas_kode = 'BRS-MED'`))
	})

	t.Run("audit_log: ID BIGINT menurut waktu lalu ID lama, kode = ID lama", func(t *testing.T) {
		assert.Equal(t, "bigint", g5BacaKolom(t, db, "audit_log", "id").Tipe)
		assert.Equal(t, "a", g5BacaKolom(t, db, "audit_log", "id").Identitas)
		assert.Equal(t, "aud-1,aud-2,aud-2b,aud-3,aud-4,aud-5,aud-6,aud-7,aud-8,aud-9,0a1b2c3d-1111-4222-8333-444455556666",
			scanString(t, db, `SELECT string_agg(kode, ',' ORDER BY id) FROM audit_log`))
		assert.Equal(t, "1,2,3,4,5,6,7,8,9,10,11", scanString(t, db, `SELECT string_agg(id::text, ',' ORDER BY id) FROM audit_log`))
		assert.Equal(t, uuidPengguna("superadmin"), scanString(t, db, `SELECT aktor_id::text FROM audit_log WHERE kode = 'aud-3'`), "FK aktor (000110) utuh")
		assert.EqualValues(t, 1, scanCount(t, db, `SELECT count(*) FROM pg_constraint WHERE conname = 'audit_log_aktor_id_fkey' AND confdeltype = 'n'`))
	})

	t.Run("rujukan lunak audit_log.entitas_id", func(t *testing.T) {
		soft := func(kode string) string {
			return scanString(t, db, `SELECT entitas_id FROM audit_log WHERE kode = ?`, kode)
		}
		assert.Equal(t, g5UUIDSumber("SRC-BI"), soft("aud-3"), "sumber_data: kode lama -> UUID")
		assert.Equal(t, g5UUIDSumber("SRC-BMKG"), soft("aud-9"))
		assert.Equal(t, uuidPengguna("manajer_data"), soft("aud-2b"), "langganan_push: slug pengguna lama -> UUID")
		assert.Equal(t, uuidPengguna("manajer_data"), soft("aud-4"), "preferensi_notifikasi: slug pengguna lama -> UUID")
		assert.Equal(t, "2 sumber", soft("aud-2"), "bukan ID: tidak disentuh")
		assert.Equal(t, "SRC-HILANG", soft("aud-7"), "sumber yang tidak ada: tidak dikarang")
		assert.Equal(t, "tidak-ada", soft("aud-5"), "pengguna yang tidak ada: tidak dikarang")
		assert.Equal(t, "2026-01..2026-03", soft("aud-6"), "kpi_nilai berisi rentang periode, bukan ID")
		assert.Equal(t, "SRC-BI", soft("aud-8"), "entitas milik grup lain tidak disentuh")
		assert.Equal(t, "export", soft("0a1b2c3d-1111-4222-8333-444455556666"))
	})

	t.Run("audit_log tetap append-only dan pemicunya menyala", func(t *testing.T) {
		for _, trg := range []string{"trg_audit_log_append_only", "trg_audit_log_no_truncate"} {
			assert.Equal(t, "O", scanString(t, db, `SELECT tgenabled::text FROM pg_trigger WHERE tgname = ?`, trg), trg)
		}
		err := db.Exec(`UPDATE audit_log SET aksi = 'DIUBAH' WHERE kode = 'aud-1'`).Error
		require.Error(t, err)
		assert.Contains(t, err.Error(), "append-only")
		require.Error(t, db.Exec(`DELETE FROM audit_log WHERE kode = 'aud-1'`).Error)
		require.Error(t, db.Exec(`TRUNCATE audit_log`).Error)
	})

	t.Run("identity berlanjut dari max+1 dan ID eksplisit ditolak", func(t *testing.T) {
		// audit_log (11 baris), cuaca_harian (3), ruang_fiskal (3), siklus_ekonomi (2).
		assert.EqualValues(t, 12, g5ID(t, db, `INSERT INTO audit_log (aksi, entitas, entitas_id, hasil) VALUES ('X', 'x', '1', 'BERHASIL') RETURNING id`))
		assert.EqualValues(t, 4, g5ID(t, db, `INSERT INTO cuaca_harian (kabupaten_kode, tanggal, curah_hujan_mm, suhu_c, kelembapan) VALUES ('1171', '2026-02-01', 0, 25, 70) RETURNING id`))
		assert.EqualValues(t, 4, g5ID(t, db, `INSERT INTO ruang_fiskal (tahun, instrumen, pagu, realisasi, per_tanggal, sumber) VALUES (2026, 'BARU', 10, 1, '2026-02-28', 'DJPB') RETURNING id`))
		assert.EqualValues(t, 3, g5ID(t, db, `INSERT INTO siklus_ekonomi (wilayah_kode, siklus, mulai_pada, akhir_pada) VALUES ('11', 'SURPLUS', '2026-03-01 00:00+00', '2026-03-02 00:00+00') RETURNING id`))
		for _, tabel := range []string{"harga_eksternal", "andil_inflasi", "inflasi_bulanan", "neraca_pangan", "target_kpi"} {
			assert.Equal(t, int64(1), scanCount(t, db, `SELECT count(*) FROM pg_attribute WHERE attrelid = ?::regclass AND attname = 'id' AND attidentity = 'a'`, tabel), tabel)
		}
		for _, q := range []string{
			`INSERT INTO audit_log (id, aksi, entitas, entitas_id, hasil) VALUES (999, 'X', 'x', '1', 'BERHASIL')`,
			`INSERT INTO cuaca_harian (id, kabupaten_kode, tanggal, curah_hujan_mm, suhu_c, kelembapan) VALUES (999, '1171', '2026-02-02', 0, 25, 70)`,
			`INSERT INTO ruang_fiskal (id, tahun, instrumen, pagu, realisasi, per_tanggal, sumber) VALUES (999, 2026, 'X', 10, 1, '2026-02-28', 'DJPB')`,
			`INSERT INTO inflasi_bulanan (id, wilayah_kode, periode, ihk, inflasi_mtm, inflasi_yoy, inflasi_ytd, sumber) VALUES (999, '11', '2026-05-01', 1, 1, 1, 1, 'BPS')`,
		} {
			err := db.Exec(q).Error
			require.Error(t, err, q)
			assert.Contains(t, err.Error(), "428C9", q)
		}
	})

	t.Run("semua FK bertipe sama, ON DELETE eksplisit sesuai rencana", func(t *testing.T) {
		assert.Equal(t, g5AturanBaru, g5AturanHapus(t, db))
	})

	t.Run("indeks: FK baru, BRIN, dan yang redundan dibuang", func(t *testing.T) {
		for _, idxName := range []string{
			"idx_harga_eksternal_kabupaten_kode", "idx_harga_eksternal_data_mentah_id", "idx_harga_eksternal_eksekusi_id",
			"idx_cuaca_harian_eksekusi_id", "idx_cuaca_harian_data_mentah_id", "idx_cuaca_harian_tanggal",
			"idx_andil_inflasi_komoditas_kode", "idx_kpi_nilai_wilayah_kode", "idx_kpi_target_wilayah_kode",
			"idx_audit_log_waktu_brin", "idx_data_mentah_diambil_brin", "idx_harga_eksternal_tanggal_brin",
			"idx_audit_log_waktu_id", "idx_audit_log_entitas_waktu", "idx_audit_log_aktor_waktu", "idx_audit_log_aksi_waktu",
			"uq_sumber_data_kode", "uq_audit_log_kode", "uq_cuaca_harian_kode", "uq_ruang_fiskal_kode", "uq_siklus_ekonomi_kode",
		} {
			assert.True(t, g5IndeksAda(t, db, idxName), "indeks %s harus ada", idxName)
		}
		for _, idxName := range []string{"idx_audit_log_entitas", "idx_audit_log_waktu"} {
			assert.False(t, g5IndeksAda(t, db, idxName), "indeks %s redundan dan dibuang", idxName)
		}
		for _, brin := range []string{"idx_audit_log_waktu_brin", "idx_data_mentah_diambil_brin", "idx_harga_eksternal_tanggal_brin"} {
			assert.Equal(t, "brin", scanString(t, db, `SELECT am.amname FROM pg_class c JOIN pg_am am ON am.oid = c.relam WHERE c.relname = ?`, brin), brin)
		}
		assert.Contains(t, scanString(t, db, `SELECT pg_get_indexdef('idx_audit_log_waktu_id'::regclass)`), "(waktu DESC, id DESC)")
	})

	t.Run("view gold_harga_harian tetap ada dan bisa disegarkan", func(t *testing.T) {
		exec(t, db, `REFRESH MATERIALIZED VIEW gold_harga_harian`)
		// Tiga kombinasi (komoditas, kabupaten, tanggal) harga eksternal KONSUMEN.
		assert.EqualValues(t, 3, scanCount(t, db, `SELECT count(*) FROM gold_harga_harian`))
	})

	t.Run("down memulihkan ID lama, kolom, PK, dan aturan semula", func(t *testing.T) {
		// Baris yang dibuat sesudah Up tidak punya kode: kembali sebagai id::text.
		require.NoError(t, migrator.DownTo(t.Context(), versiSebelumPKSumberAudit), "down 000115")

		for _, tabel := range []string{"sumber_data", "audit_log", "cuaca_harian", "ruang_fiskal", "siklus_ekonomi"} {
			assert.Equal(t, "character varying(255)", g5BacaKolom(t, db, tabel, "id").Tipe, tabel)
			assert.Empty(t, g5BacaKolom(t, db, tabel, "id").Identitas, tabel)
			assert.False(t, g5KolomAda(t, db, tabel, "kode"), "kolom kode dibuang: "+tabel)
			assert.Equal(t, "id", g5PK(t, db, tabel), tabel)
		}
		for _, tabel := range []string{"eksekusi_sumber", "data_mentah", "pemetaan_sumber", "harga_eksternal"} {
			assert.Equal(t, "character varying(255)", g5BacaKolom(t, db, tabel, "sumber_id").Tipe, tabel)
		}

		// ID lama kembali; baris baru memakai id::text.
		assert.EqualValues(t, 3, scanCount(t, db, `SELECT count(*) FROM sumber_data WHERE id IN ('SRC-BI', 'SRC-BMKG', 'SRC-LAMA')`))
		assert.Equal(t, "4,CUA-1171-20260101-BMKG,CUA-1171-20260102-BMKG,CUA-1174-20260101-BMKG", scanString(t, db, `SELECT string_agg(id, ',' ORDER BY id COLLATE "C") FROM cuaca_harian`))
		assert.Equal(t, "4,RF-A,RF-B,RF-C", scanString(t, db, `SELECT string_agg(id, ',' ORDER BY id COLLATE "C") FROM ruang_fiskal`))
		assert.Equal(t, "3,SE-11-DEMO,SE-11-LAMA", scanString(t, db, `SELECT string_agg(id, ',' ORDER BY id COLLATE "C") FROM siklus_ekonomi`))
		assert.EqualValues(t, 1, scanCount(t, db, `SELECT count(*) FROM audit_log WHERE id = '12'`), "baris audit baru: id::text")
		assert.EqualValues(t, 1, scanCount(t, db, `SELECT count(*) FROM audit_log WHERE id = '0a1b2c3d-1111-4222-8333-444455556666'`))
		assert.EqualValues(t, 3, scanCount(t, db, `SELECT count(*) FROM harga_eksternal WHERE sumber_id = 'SRC-BI'`))
		assert.EqualValues(t, 2, scanCount(t, db, `SELECT count(*) FROM pemetaan_sumber WHERE sumber_id = 'SRC-BI'`))
		assert.EqualValues(t, 1, scanCount(t, db, `SELECT count(*) FROM data_mentah WHERE sumber_id = 'SRC-BMKG'`))
		assert.EqualValues(t, 4, scanCount(t, db, `SELECT count(*) FROM eksekusi_sumber`))

		// Rujukan lunak kembali ke ID lama.
		soft := func(id string) string { return scanString(t, db, `SELECT entitas_id FROM audit_log WHERE id = ?`, id) }
		assert.Equal(t, "SRC-BI", soft("aud-3"))
		assert.Equal(t, "SRC-BMKG", soft("aud-9"))
		assert.Equal(t, "manajer_data", soft("aud-2b"))
		assert.Equal(t, "manajer_data", soft("aud-4"))
		assert.Equal(t, "2 sumber", soft("aud-2"))
		assert.Equal(t, "SRC-BI", soft("aud-8"))

		// PK komposit alami kembali; kolom id dan UNIQUE tambahan hilang.
		for tabel, pk := range map[string]string{
			"harga_eksternal": "sumber_id,komoditas_kode,kabupaten_kode,tanggal,tipe", "andil_inflasi": "periode,wilayah_kode,komoditas_kode",
			"inflasi_bulanan": "wilayah_kode,periode", "neraca_pangan": "komoditas_kode,wilayah_kode,periode", "target_kpi": "indikator,periode_mulai",
		} {
			assert.Equal(t, pk, g5PK(t, db, tabel), tabel)
			assert.False(t, g5KolomAda(t, db, tabel, "id"), "kolom id dibuang: "+tabel)
		}
		assert.EqualValues(t, 3, scanCount(t, db, `SELECT count(*) FROM andil_inflasi`))

		// Aturan ON DELETE semula (NO ACTION), kecuali yang memang tidak diubah.
		semula := map[string]string{}
		for k, v := range g5AturanBaru {
			semula[k] = "NO ACTION"
			if v == "SET NULL" && (k == "eksekusi_sumber.aktor_id") || k == "kpi_nilai.kpi_kode" || k == "kpi_target.kpi_kode" {
				semula[k] = v
			}
		}
		assert.Equal(t, semula, g5AturanHapus(t, db))

		// Indeks dan pemicu.
		for _, idxName := range []string{"idx_audit_log_entitas", "idx_audit_log_waktu", "idx_audit_log_waktu_id", "idx_audit_log_entitas_waktu"} {
			assert.True(t, g5IndeksAda(t, db, idxName), "indeks %s dipulihkan", idxName)
		}
		for _, idxName := range []string{"idx_audit_log_waktu_brin", "idx_data_mentah_diambil_brin", "idx_harga_eksternal_tanggal_brin",
			"idx_cuaca_harian_tanggal", "idx_harga_eksternal_kabupaten_kode", "idx_kpi_nilai_wilayah_kode", "uq_sumber_data_kode"} {
			assert.False(t, g5IndeksAda(t, db, idxName), "indeks %s dibuang", idxName)
		}
		for _, trg := range []string{"trg_audit_log_append_only", "trg_audit_log_no_truncate"} {
			assert.Equal(t, "O", scanString(t, db, `SELECT tgenabled::text FROM pg_trigger WHERE tgname = ?`, trg), trg)
		}
		require.Error(t, db.Exec(`UPDATE audit_log SET aksi = 'DIUBAH' WHERE id = 'aud-1'`).Error)
	})

	t.Run("up ulang atas data yang sudah dibalik menghasilkan UUID dan urutan yang sama", func(t *testing.T) {
		require.NoError(t, migrator.UpTo(t.Context(), versiPKSumberAudit), "up kedua")
		assert.Equal(t, g5UUIDSumber("SRC-BI"), scanString(t, db, `SELECT id::text FROM sumber_data WHERE kode = 'SRC-BI'`))
		assert.Equal(t, g5UUIDSumber("SRC-BI"), scanString(t, db, `SELECT entitas_id FROM audit_log WHERE kode = 'aud-3'`))
		assert.Equal(t, uuidPengguna("manajer_data"), scanString(t, db, `SELECT entitas_id FROM audit_log WHERE kode = 'aud-2b'`))
		assert.EqualValues(t, 3, scanCount(t, db, `SELECT count(*) FROM harga_eksternal WHERE sumber_id = ?`, g5UUIDSumber("SRC-BI")))
		assert.EqualValues(t, 12, scanCount(t, db, `SELECT count(*) FROM audit_log`))
		assert.Equal(t, "aud-1", scanString(t, db, `SELECT kode FROM audit_log ORDER BY id LIMIT 1`))
		assert.Equal(t, g5AturanBaru, g5AturanHapus(t, db))
	})
}

// TestMigrasiPKSumberAudit_AturanHapusBekerjaPadaDataHasilMigrasi: setelah konversi, ON DELETE benar-benar bekerja
// pada baris hasil migrasi (CASCADE, RESTRICT, SET NULL).
func TestMigrasiPKSumberAudit_AturanHapusBekerjaPadaDataHasilMigrasi(t *testing.T) {
	db := pg.NewDatabase(t, "migrate_pk_sumber_audit_hapus")
	migrator := pgtest.Migrator(t, db)
	require.NoError(t, migrator.UpTo(t.Context(), versiSebelumPKSumberAudit))
	g5IsiDataLama(t, db)
	require.NoError(t, migrator.UpTo(t.Context(), versiPKSumberAudit))

	t.Run("RESTRICT: sumber yang punya harga eksternal tidak bisa dihapus", func(t *testing.T) {
		err := db.Exec(`DELETE FROM sumber_data WHERE kode = 'SRC-BI'`).Error
		require.Error(t, err)
		assert.Contains(t, err.Error(), "23503")
		assert.EqualValues(t, 3, scanCount(t, db, `SELECT count(*) FROM harga_eksternal`))
	})

	t.Run("RESTRICT: komoditas dan kabupaten yang dirujuk silver agregat ditahan", func(t *testing.T) {
		for _, q := range []string{
			`DELETE FROM komoditas WHERE kode = 'CBM-KRT'`, // andil_inflasi, neraca_pangan, harga_eksternal
			`DELETE FROM provinsi WHERE kode = '11'`,       // siklus_ekonomi (dan kabupaten)
		} {
			err := db.Exec(q).Error
			require.Error(t, err, q)
			assert.Contains(t, err.Error(), "23503", q)
		}
		for _, q := range []string{
			`DELETE FROM kabupaten WHERE kode = '1171'`, // kpi_nilai, kpi_target, cuaca_harian, harga_eksternal
		} {
			require.Error(t, db.Exec(q).Error, q)
		}
	})

	t.Run("SET NULL: menghapus eksekusi melepas harga dan cuaca, bronze ikut terhapus (CASCADE)", func(t *testing.T) {
		eksekusi := scanCount(t, db, `SELECT id FROM eksekusi_sumber WHERE sumber_id = ? AND pemicu = 'JADWAL' AND status = 'SUKSES'`, g5UUIDSumber("SRC-BI"))
		require.NoError(t, db.Exec(`DELETE FROM eksekusi_sumber WHERE id = ?`, eksekusi).Error)
		assert.Zero(t, scanCount(t, db, `SELECT count(*) FROM data_mentah WHERE sumber_id = ?`, g5UUIDSumber("SRC-BI")))
		assert.EqualValues(t, 3, scanCount(t, db, `SELECT count(*) FROM harga_eksternal`))
		assert.Zero(t, scanCount(t, db, `SELECT count(*) FROM harga_eksternal WHERE eksekusi_id IS NOT NULL OR data_mentah_id IS NOT NULL`))
	})

	t.Run("SET NULL: menghapus data_mentah melepas cuaca", func(t *testing.T) {
		require.EqualValues(t, 2, scanCount(t, db, `SELECT count(*) FROM cuaca_harian WHERE data_mentah_id IS NOT NULL`))
		require.NoError(t, db.Exec(`DELETE FROM data_mentah WHERE sumber_id = ?`, g5UUIDSumber("SRC-BMKG")).Error)
		assert.Zero(t, scanCount(t, db, `SELECT count(*) FROM cuaca_harian WHERE data_mentah_id IS NOT NULL`))
		assert.EqualValues(t, 3, scanCount(t, db, `SELECT count(*) FROM cuaca_harian`))
	})

	t.Run("CASCADE: sumber tanpa harga membawa eksekusi dan pemetaannya", func(t *testing.T) {
		exec(t, db, `INSERT INTO pemetaan_sumber (sumber_id, jenis, alias, kode_master)
			SELECT id, 'WILAYAH', 'x', '1171' FROM sumber_data WHERE kode = 'SRC-LAMA'`)
		exec(t, db, `INSERT INTO eksekusi_sumber (sumber_id, pemicu, status, selesai) SELECT id, 'CLI', 'SUKSES', now() FROM sumber_data WHERE kode = 'SRC-LAMA'`)
		require.NoError(t, db.Exec(`DELETE FROM sumber_data WHERE kode = 'SRC-LAMA'`).Error)
		assert.Zero(t, scanCount(t, db, `SELECT count(*) FROM pemetaan_sumber WHERE alias = 'x'`))
		assert.Zero(t, scanCount(t, db, `SELECT count(*) FROM eksekusi_sumber WHERE pemicu = 'CLI' AND sumber_id NOT IN (SELECT id FROM sumber_data)`))
	})

	t.Run("CASCADE: menghapus definisi KPI membawa nilai dan targetnya (aturan lama dipertahankan)", func(t *testing.T) {
		require.NoError(t, db.Exec(`DELETE FROM kpi_definisi WHERE kode = 'LOG-UTILISASI-CS'`).Error)
		assert.Zero(t, scanCount(t, db, `SELECT count(*) FROM kpi_nilai`))
		assert.Zero(t, scanCount(t, db, `SELECT count(*) FROM kpi_target`))
	})
}

// TestMigrasiPKSumberAudit_IDLamaTerlaluPanjangDitolakDenganPesanJelas: penjaga kode varchar(64).
func TestMigrasiPKSumberAudit_IDLamaTerlaluPanjangDitolakDenganPesanJelas(t *testing.T) {
	db := pg.NewDatabase(t, "migrate_pk_sumber_audit_panjang")
	migrator := pgtest.Migrator(t, db)
	require.NoError(t, migrator.UpTo(t.Context(), versiSebelumPKSumberAudit))
	exec(t, db, `INSERT INTO audit_log (id, aksi, entitas, entitas_id, hasil) VALUES (repeat('x', 65), 'X', 'x', '1', 'BERHASIL')`)

	err := migrator.UpTo(t.Context(), versiPKSumberAudit)

	require.Error(t, err)
	assert.Contains(t, err.Error(), "lebih dari 64 karakter")
	// Transaksi digulung: tidak ada yang berubah.
	assert.Equal(t, "character varying(255)", g5BacaKolom(t, db, "audit_log", "id").Tipe)
}
