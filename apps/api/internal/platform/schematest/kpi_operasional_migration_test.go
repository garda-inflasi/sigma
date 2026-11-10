package schematest

import (
	"fmt"
	"testing"

	"github.com/garda-inflasi/api/internal/dashboard/domain"
	"github.com/garda-inflasi/api/internal/platform/migrate"
	"github.com/garda-inflasi/api/internal/platform/pgtest"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

// Migrasi 000124 (slice 08-B) membangun ulang mv_kpi_operasional: dari nilai jadi tingkat provinsi untuk 14 hari
// (indikator, tanggal, nilai) menjadi fakta harian per kabupaten dan komoditas dengan pembilang dan penyebut, supaya KPI
// bisa difilter secara tepat. Tes dipatok ke versi 123 -> 124 supaya migrasi lain yang datang sesudahnya tidak mengubah
// hasilnya.
const (
	versiSebelumKpiHalus = 123
	versiKpiHalus        = 124
)

// hariIniSQL adalah tanggal Aceh hari ini menurut jam basis data (jam yang sama dengan yang dipakai view).
const hariIniSQL = `(now() AT TIME ZONE 'Asia/Jakarta')::date`

// isiDataKpi mengisi basis data versi 123 dengan data kecil yang angkanya mudah dihitung:
//   - Banda Aceh (1171): 8 hari terakhir tiap hari 1 entri CBM-KRT DIPUBLIKASI 10.000; hari ini CBM-KRT 12.000 DIPUBLIKASI
//     (naik 20%), BWM DIAJUKAN, dan satu entri DRAF.
//   - Aceh Besar (1108): hari ini CBM-KRT DIPUBLIKASI.
//   - Cold storage CS-X (1171, OPERASIONAL, 100 t) berisi lot BWM 25 t dari 5 hari lalu sampai 20 hari ke depan; CS-Y
//     (1108) PERAWATAN berisi lot 40 t yang tidak dihitung.
func isiDataKpi(t *testing.T, db *gorm.DB) {
	t.Helper()
	for _, sql := range []string{
		`INSERT INTO provinsi (kode, nama) VALUES ('11', 'Aceh')`,
		`INSERT INTO satuan (kode, nama) VALUES ('kg', 'Kilogram')`,
		`INSERT INTO kabupaten (kode, nama, jenis, ibukota, lat, lng, provinsi_kode) VALUES
			('1171', 'Banda Aceh', 'KOTA', 'Banda Aceh', 5.55, 95.32, '11'),
			('1108', 'Aceh Besar', 'KABUPATEN', 'Jantho', 5.3, 95.5, '11')`,
		`INSERT INTO komoditas (kode, nama, satuan, kategori, harga_acuan, shelf_life_hari) VALUES
			('CBM-KRT', 'Cabai Merah', 'kg', 'cabai', 50000, 7), ('BWM', 'Bawang Merah', 'kg', 'bawang', 40000, 30)`,
		`INSERT INTO pasar (kode, nama, kabupaten_kode, lat, lng) VALUES
			('PSR-A', 'Pasar A', '1171', 5.55, 95.32), ('PSR-B', 'Pasar B', '1108', 5.3, 95.5)`,

		`INSERT INTO entri_harga (komoditas_kode, pasar_id, tipe, harga, satuan, status, dicatat_pada)
		 SELECT 'CBM-KRT', 'PSR-A', 'KONSUMEN'::tipe_harga, 10000, 'kg', 'DIPUBLIKASI'::status_entri,
		        ((` + hariIniSQL + ` - g)::timestamp + time '09:00') AT TIME ZONE 'Asia/Jakarta'
		   FROM generate_series(1, 8) g`,
		`INSERT INTO entri_harga (komoditas_kode, pasar_id, tipe, harga, satuan, status, dicatat_pada) VALUES
			('CBM-KRT', 'PSR-A', 'KONSUMEN', 12000, 'kg', 'DIPUBLIKASI', (` + hariIniSQL + `::timestamp + time '09:00') AT TIME ZONE 'Asia/Jakarta'),
			('BWM', 'PSR-A', 'KONSUMEN', 30000, 'kg', 'DIAJUKAN', (` + hariIniSQL + `::timestamp + time '09:01') AT TIME ZONE 'Asia/Jakarta'),
			('BWM', 'PSR-A', 'KONSUMEN', 30000, 'kg', 'DRAF', (` + hariIniSQL + `::timestamp + time '09:02') AT TIME ZONE 'Asia/Jakarta'),
			('CBM-KRT', 'PSR-B', 'KONSUMEN', 11000, 'kg', 'DIPUBLIKASI', (` + hariIniSQL + `::timestamp + time '09:03') AT TIME ZONE 'Asia/Jakarta')`,

		`INSERT INTO cold_storage (nama, kabupaten_kode, lat, lng, kapasitas_ton, suhu_target_c, status, kode) VALUES
			('CS X', '1171', 5.55, 95.32, 100, 4, 'OPERASIONAL', 'CS-X'),
			('CS Y', '1108', 5.3, 95.5, 60, 4, 'PERAWATAN', 'CS-Y')`,
		`INSERT INTO stok_cold_storage (cold_storage_id, komoditas_kode, volume_ton, masuk_pada, pemilik, risiko_food_loss, kedaluwarsa_pada)
		 SELECT id, 'BWM', 25, ((` + hariIniSQL + ` - 5)::timestamp + time '08:00') AT TIME ZONE 'Asia/Jakarta', 'KUD', 0.1,
		        ((` + hariIniSQL + ` + 20)::timestamp + time '08:00') AT TIME ZONE 'Asia/Jakarta' FROM cold_storage WHERE kode = 'CS-X'`,
		`INSERT INTO stok_cold_storage (cold_storage_id, komoditas_kode, volume_ton, masuk_pada, pemilik, risiko_food_loss, kedaluwarsa_pada)
		 SELECT id, 'BWM', 40, ((` + hariIniSQL + ` - 5)::timestamp + time '08:00') AT TIME ZONE 'Asia/Jakarta', 'KUD', 0.1,
		        ((` + hariIniSQL + ` + 20)::timestamp + time '08:00') AT TIME ZONE 'Asia/Jakarta' FROM cold_storage WHERE kode = 'CS-Y'`,
		`REFRESH MATERIALIZED VIEW agregat_harga_harian`,
		`REFRESH MATERIALIZED VIEW mv_kpi_operasional`,
	} {
		exec(t, db, sql)
	}
}

// kolomMV mengembalikan nama kolom view berurutan.
func kolomMV(t *testing.T, db *gorm.DB, view string) string {
	t.Helper()
	return scanString(t, db, `SELECT string_agg(attname, ',' ORDER BY attnum) FROM pg_attribute
		WHERE attrelid = ?::regclass AND attnum > 0 AND NOT attisdropped`, view)
}

// nilaiJadiBaru menyatukan fakta view baru menjadi nilai kartu tanpa filter (empat indikator, 14 hari), dengan aturan
// yang dipakai usecase: jumlah, persentase dari jumlah pembilang/penyebut dalam jendela 7 hari, komoditas berbeda, dan
// pembilang volume terhadap penyebut kapasitas.
const nilaiJadiBaru = `
WITH hari AS (SELECT d::date AS tanggal FROM generate_series(` + hariIniSQL + ` - 13, ` + hariIniSQL + `, interval '1 day') d),
harian AS (
  SELECT indikator, tanggal,
         CASE indikator WHEN 'waspada' THEN COUNT(DISTINCT komoditas_kode) FILTER (WHERE pembilang > 0)
                        WHEN 'cs' THEN COALESCE(SUM(pembilang) FILTER (WHERE komoditas_kode <> ''), 0)
                        ELSE SUM(pembilang) END AS p,
         CASE indikator WHEN 'cs' THEN COALESCE(SUM(penyebut) FILTER (WHERE komoditas_kode = ''), 0)
                        ELSE SUM(penyebut) END AS q
    FROM mv_kpi_operasional GROUP BY 1, 2
)
SELECT 'entri'::text AS indikator, h.tanggal, COALESCE((SELECT p FROM harian x WHERE x.indikator = 'entri' AND x.tanggal = h.tanggal), 0)::float8 AS nilai FROM hari h
UNION ALL
SELECT 'validasi', h.tanggal, COALESCE((SELECT 100.0 * SUM(p) / NULLIF(SUM(q), 0) FROM harian x WHERE x.indikator = 'validasi' AND x.tanggal BETWEEN h.tanggal - 6 AND h.tanggal), 0)::float8 FROM hari h
UNION ALL
SELECT 'waspada', h.tanggal, COALESCE((SELECT p FROM harian x WHERE x.indikator = 'waspada' AND x.tanggal = h.tanggal), 0)::float8 FROM hari h
UNION ALL
SELECT 'cs', h.tanggal, COALESCE((SELECT 100.0 * p / NULLIF(q, 0) FROM harian x WHERE x.indikator = 'cs' AND x.tanggal = h.tanggal), 0)::float8 FROM hari h`

type nilaiKpi struct {
	Indikator string
	Tanggal   string
	Nilai     float64
}

func bacaNilaiLama(t *testing.T, db *gorm.DB) []nilaiKpi {
	t.Helper()
	var rows []nilaiKpi
	require.NoError(t, db.Raw(`SELECT indikator, to_char(tanggal, 'YYYY-MM-DD') AS tanggal, nilai FROM mv_kpi_operasional ORDER BY indikator, tanggal`).Scan(&rows).Error)
	return rows
}

func bacaNilaiBaru(t *testing.T, db *gorm.DB) []nilaiKpi {
	t.Helper()
	var rows []nilaiKpi
	require.NoError(t, db.Raw(`SELECT indikator, to_char(tanggal, 'YYYY-MM-DD') AS tanggal, nilai FROM (`+nilaiJadiBaru+`) x ORDER BY indikator, tanggal`).Scan(&rows).Error)
	return rows
}

func TestMigrations_KpiOperasionalFaktaHarianUpDownUp(t *testing.T) {
	const namaDB = "migrate_kpi_halus"
	db := pg.NewDatabase(t, namaDB)
	migrator := pgtest.Migrator(t, db)
	require.NoError(t, migrator.UpTo(t.Context(), versiSebelumKpiHalus))
	isiDataKpi(t, db)

	// Garis dasar: nilai jadi menurut definisi lama.
	assert.Equal(t, "indikator,tanggal,nilai", kolomMV(t, db, "mv_kpi_operasional"), "kontrol: definisi lama")
	lama := bacaNilaiLama(t, db)
	require.Len(t, lama, 4*14)
	byKey := func(rows []nilaiKpi) map[string]float64 {
		m := map[string]float64{}
		for _, r := range rows {
			m[r.Indikator+"/"+r.Tanggal] = r.Nilai
		}
		return m
	}
	hariIni := scanString(t, db, `SELECT to_char(`+hariIniSQL+`, 'YYYY-MM-DD')`)
	require.EqualValues(t, 3, byKey(lama)["entri/"+hariIni], "fixture: tiga entri non-DRAF hari ini")
	require.EqualValues(t, 1, byKey(lama)["waspada/"+hariIni], "fixture: cabai naik 20% di Banda Aceh")
	require.EqualValues(t, 25, byKey(lama)["cs/"+hariIni], "fixture: 25 t dari 100 t (lot di fasilitas perawatan tidak dihitung)")

	require.NoError(t, migrator.Apply(t.Context(), versiKpiHalus, true), "up 000124")

	t.Run("up: kolom, indeks unik tanpa NULL, terisi, berkomentar, bisa REFRESH CONCURRENTLY", func(t *testing.T) {
		assert.Equal(t, "indikator,tanggal,kabupaten_kode,komoditas_kode,pembilang,penyebut", kolomMV(t, db, "mv_kpi_operasional"))
		assert.Contains(t, definisiIndeks(t, db, "uq_mv_kpi_operasional"),
			"UNIQUE INDEX uq_mv_kpi_operasional ON public.mv_kpi_operasional USING btree (indikator, tanggal, kabupaten_kode, komoditas_kode)")
		assert.EqualValues(t, 1, scanCount(t, db, `SELECT count(*) FROM pg_matviews WHERE matviewname = 'mv_kpi_operasional' AND ispopulated`))
		assert.NotEmpty(t, scanString(t, db, `SELECT coalesce(obj_description('mv_kpi_operasional'::regclass, 'pg_class'), '')`))
		assert.Zero(t, scanCount(t, db, `SELECT count(*) FROM mv_kpi_operasional WHERE kabupaten_kode IS NULL OR komoditas_kode IS NULL OR pembilang IS NULL OR penyebut IS NULL`),
			"tanpa NULL: kunci unik harus menegakkan keunikan")

		jumlahSebelum := scanCount(t, db, `SELECT count(*) FROM mv_kpi_operasional`)
		require.NoError(t, db.Exec(`REFRESH MATERIALIZED VIEW CONCURRENTLY mv_kpi_operasional`).Error)
		assert.Equal(t, jumlahSebelum, scanCount(t, db, `SELECT count(*) FROM mv_kpi_operasional`))
		require.NoError(t, db.Exec(`SELECT garda_segarkan_mv('mv_kpi_operasional', true)`).Error, "lewat fungsi penyegar aplikasi")
	})

	t.Run("up: fakta harian per kabupaten dan komoditas", func(t *testing.T) {
		fakta := func(indikator, kab, kom string, hari string) (pembilang, penyebut float64) {
			var r struct{ Pembilang, Penyebut float64 }
			require.NoError(t, db.Raw(`SELECT pembilang, penyebut FROM mv_kpi_operasional
				WHERE indikator = ? AND kabupaten_kode = ? AND komoditas_kode = ? AND tanggal = ?::date`, indikator, kab, kom, hari).Scan(&r).Error)
			return r.Pembilang, r.Penyebut
		}
		p, q := fakta("entri", "1171", "CBM-KRT", hariIni)
		assert.EqualValues(t, 1, p)
		assert.Zero(t, q)
		p, _ = fakta("entri", "1171", "BWM", hariIni)
		assert.EqualValues(t, 1, p, "DIAJUKAN dihitung, DRAF tidak")
		p, _ = fakta("entri", "1108", "CBM-KRT", hariIni)
		assert.EqualValues(t, 1, p)

		p, q = fakta("validasi", "1171", "CBM-KRT", hariIni)
		assert.EqualValues(t, 1, p, "tervalidasi")
		assert.EqualValues(t, 1, q, "dari satu entri")
		p, q = fakta("validasi", "1171", "BWM", hariIni)
		assert.Zero(t, p, "DIAJUKAN belum tervalidasi")
		assert.EqualValues(t, 1, q)

		assert.EqualValues(t, 1, scanCount(t, db, `SELECT count(*) FROM mv_kpi_operasional WHERE indikator = 'waspada' AND tanggal = `+hariIniSQL),
			"hanya cabai Banda Aceh yang melampaui 7% (Aceh Besar belum punya pembanding)")
		p, _ = fakta("waspada", "1171", "CBM-KRT", hariIni)
		assert.EqualValues(t, 1, p)

		p, _ = fakta("cs", "1171", "BWM", hariIni)
		assert.EqualValues(t, 25, p, "volume lot aktif di fasilitas operasional")
		_, q = fakta("cs", "1171", "", hariIni)
		assert.EqualValues(t, 100, q, "baris kapasitas: komoditas kosong")
		assert.Zero(t, scanCount(t, db, `SELECT count(*) FROM mv_kpi_operasional WHERE indikator = 'cs' AND kabupaten_kode = '1108'`),
			"fasilitas perawatan tidak punya kapasitas operasional dan lotnya tidak dihitung")
		assert.Zero(t, scanCount(t, db, `SELECT count(*) FROM mv_kpi_operasional WHERE indikator = 'cs' AND komoditas_kode <> '' AND tanggal < `+hariIniSQL+` - 5`),
			"lot belum masuk sebelum 5 hari lalu")
	})

	t.Run("up: riwayat selebar domain.RiwayatMVHari ditambah satu hari ke depan untuk cs, entri apa adanya", func(t *testing.T) {
		// Konstanta Go dan SQL view harus sama: API menolak periode yang butuh fakta lebih lama dari ini.
		assert.EqualValues(t, domain.RiwayatMVHari+1, scanCount(t, db, `SELECT count(DISTINCT tanggal) FROM mv_kpi_operasional WHERE indikator = 'cs'`))
		assert.EqualValues(t, domain.RiwayatMVHari+1, scanCount(t, db, `SELECT count(*) FROM mv_kpi_operasional WHERE indikator = 'cs' AND komoditas_kode = ''`))
		assert.Zero(t, scanCount(t, db, fmt.Sprintf(`SELECT count(*) FROM mv_kpi_operasional WHERE tanggal < %s - %d OR tanggal > %s + 1`,
			hariIniSQL, domain.RiwayatMVHari-1, hariIniSQL)))
		assert.EqualValues(t, 1, scanCount(t, db, fmt.Sprintf(`SELECT count(*) FROM mv_kpi_operasional WHERE tanggal = %s - %d AND indikator = 'cs' AND komoditas_kode = ''`,
			hariIniSQL, domain.RiwayatMVHari-1)), "hari tertua riwayat ada")
		assert.EqualValues(t, 1, scanCount(t, db, `SELECT count(*) FROM mv_kpi_operasional WHERE indikator = 'cs' AND komoditas_kode <> '' AND tanggal = `+hariIniSQL+` + 1`),
			"lot yang berlaku besok sudah ada: pergantian hari tidak membuat cs 0 sebelum penyegaran berikutnya")
		assert.Zero(t, scanCount(t, db, `SELECT count(*) FROM mv_kpi_operasional WHERE indikator IN ('entri', 'validasi', 'waspada') AND tanggal > `+hariIniSQL))
	})

	t.Run("up: nilai kartu tanpa filter sama persis dengan definisi lama", func(t *testing.T) {
		baru := byKey(bacaNilaiBaru(t, db))
		require.Len(t, baru, 4*14)
		for k, v := range byKey(lama) {
			assert.InDelta(t, v, baru[k], 1e-9, k)
		}
	})

	t.Run("up: peran aplikasi membaca view dan menyegarkannya tanpa hak tambahan", func(t *testing.T) {
		const nama, sandi = "garda_app_kpi_halus", "sandi-uji-cukup-panjang-4"
		sqlDB, err := db.DB()
		require.NoError(t, err)
		require.NoError(t, migrate.SiapkanPenggunaAplikasi(t.Context(), sqlDB, nama, sandi))
		app := pg.BukaSebagai(t, namaDB, nama, sandi)

		assert.Positive(t, scanCount(t, app, `SELECT count(*) FROM mv_kpi_operasional`))
		require.NoError(t, app.Exec(`SELECT garda_segarkan_mv('mv_kpi_operasional', true)`).Error)
		err = app.Exec(`REFRESH MATERIALIZED VIEW mv_kpi_operasional`).Error
		require.Error(t, err, "REFRESH langsung butuh pemilik; hanya lewat fungsi SECURITY DEFINER")
		assert.Contains(t, err.Error(), "42501")
	})

	require.NoError(t, migrator.Apply(t.Context(), versiKpiHalus, false), "down 000124")

	t.Run("down: definisi lama pulih (kolom, indeks, isi sama dengan garis dasar)", func(t *testing.T) {
		assert.Equal(t, "indikator,tanggal,nilai", kolomMV(t, db, "mv_kpi_operasional"))
		assert.Contains(t, definisiIndeks(t, db, "uq_mv_kpi_operasional"), "UNIQUE INDEX uq_mv_kpi_operasional ON public.mv_kpi_operasional USING btree (indikator, tanggal)")
		assert.EqualValues(t, 4*14, scanCount(t, db, `SELECT count(*) FROM mv_kpi_operasional`))
		assert.Equal(t, lama, bacaNilaiLama(t, db), "nilai sama dengan sebelum 000124")
		assert.Empty(t, scanString(t, db, `SELECT coalesce(obj_description('mv_kpi_operasional'::regclass, 'pg_class'), '')`), "komentar baru ikut hilang")
		require.NoError(t, db.Exec(`REFRESH MATERIALIZED VIEW CONCURRENTLY mv_kpi_operasional`).Error)
		assert.True(t, adaIndeks(t, db, "uq_mv_kpi_operasional"))
	})

	t.Run("down: peran aplikasi tetap membaca view lama", func(t *testing.T) {
		app := pg.BukaSebagai(t, namaDB, "garda_app_kpi_halus", "sandi-uji-cukup-panjang-4")
		assert.EqualValues(t, 4*14, scanCount(t, app, `SELECT count(*) FROM mv_kpi_operasional`))
	})

	t.Run("up kedua menghasilkan fakta yang sama", func(t *testing.T) {
		require.NoError(t, migrator.Apply(t.Context(), versiKpiHalus, true), "up kedua")
		assert.Equal(t, "indikator,tanggal,kabupaten_kode,komoditas_kode,pembilang,penyebut", kolomMV(t, db, "mv_kpi_operasional"))
		baru := byKey(bacaNilaiBaru(t, db))
		for k, v := range byKey(lama) {
			assert.InDelta(t, v, baru[k], 1e-9, k)
		}
		app := pg.BukaSebagai(t, namaDB, "garda_app_kpi_halus", "sandi-uji-cukup-panjang-4")
		assert.Positive(t, scanCount(t, app, `SELECT count(*) FROM mv_kpi_operasional`), "hak peran aplikasi diterapkan ulang")
	})
}

// View baru harus bisa dibangun di basis data kosong (migrasi dari nol, seperti deploy pertama) dan disegarkan
// berkali-kali tanpa galat.
func TestMigrations_KpiOperasionalBasisDataKosong(t *testing.T) {
	db := pg.NewDatabase(t, "migrate_kpi_halus_kosong")
	pgtest.MigrateUp(t, db)

	assert.Zero(t, scanCount(t, db, `SELECT count(*) FROM mv_kpi_operasional WHERE indikator <> 'cs'`), "tanpa entri tak ada fakta")
	assert.Zero(t, scanCount(t, db, `SELECT count(*) FROM mv_kpi_operasional WHERE komoditas_kode <> ''`))
	for range 2 {
		require.NoError(t, db.Exec(`SELECT garda_segarkan_mv('mv_kpi_operasional', true)`).Error)
	}
}
