package integration

// Tes seed SQL eksekutif: db/seeds/demo/14_eksekutif_data.sql (data demo) dan target inflasi di
// db/seeds/reference/13_kpi_definisi.sql. Menggantikan tes seeder Go lama (eksekutif/seeder): deret 12 bulan sampai bulan
// lalu, semua baris ber-sumber DEMO, idempoten, tidak menimpa angka resmi, memperbarui baris demo sendiri, kolom turunan
// dijaga basis data, dan ID identity tidak berganti.

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/garda-inflasi/api/internal/platform/pgtest"
	"github.com/garda-inflasi/api/pkg/seed"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

// seedDB membuat basis data bermigrasi berisi master (komoditas) dan target inflasi, tanpa data demo.
func seedDB(t *testing.T, name string) *gorm.DB {
	t.Helper()
	db := pg.NewDatabase(t, name)
	pgtest.MigrateUp(t, db)
	require.NoError(t, seed.SeedFiles(t.Context(), db, "reference/01_master.sql", "reference/13_kpi_definisi.sql"))
	return db
}

func jumlah(t *testing.T, db *gorm.DB, tabel string, kondisi ...string) int64 {
	t.Helper()
	q := db.Table(tabel)
	if len(kondisi) > 0 {
		q = q.Where(kondisi[0])
	}
	var n int64
	require.NoError(t, q.Count(&n).Error)
	return n
}

// jalankanSebagai menjalankan berkas seed seolah-olah sekarang adalah instan itu (mengganti now()), pada sesi dengan zona
// waktu sesi tertentu (kosong = zona sesi koneksi). Dipakai untuk batas bulan dan tahun yang tidak bisa dicapai dengan jam
// sungguhan.
func jalankanSebagai(t *testing.T, db *gorm.DB, berkas, instan, zonaSesi string) {
	t.Helper()
	dir, err := seed.LocateSeedsDir()
	require.NoError(t, err)
	raw, err := os.ReadFile(filepath.Join(dir, berkas))
	require.NoError(t, err)
	sql := strings.ReplaceAll(string(raw), "now()", "TIMESTAMPTZ '"+instan+"'")
	require.NotEqual(t, string(raw), sql, "%s tidak memakai now()", berkas)
	if zonaSesi != "" {
		// Satu pesan multi-pernyataan berjalan dalam satu transaksi implisit, jadi SET LOCAL berlaku untuk berkas ini saja.
		sql = "SET LOCAL TIME ZONE '" + zonaSesi + "';\n" + sql
	}
	sqlDB, err := db.DB()
	require.NoError(t, err)
	_, err = sqlDB.ExecContext(t.Context(), sql)
	require.NoError(t, err)
}

func tanggalSQL(t time.Time) string { return t.Format("2006-01-02") }

func TestSeedEksekutif_ReferenceCarriesTheInflationTargetButNoFigures(t *testing.T) {
	db := seedDB(t, "eksekutif_seed_ref")

	require.NoError(t, seed.SeedFiles(t.Context(), db, "reference/13_kpi_definisi.sql")) // putaran kedua

	assert.EqualValues(t, 3, jumlah(t, db, "target_inflasi"))
	var sasaran []struct {
		Tahun      int
		BatasBawah float64
		BatasAtas  float64
	}
	require.NoError(t, db.Raw(`SELECT tahun, batas_bawah, batas_atas FROM target_inflasi ORDER BY tahun`).Scan(&sasaran).Error)
	require.Len(t, sasaran, 3)
	for i, tahun := range []int{2025, 2026, 2027} {
		assert.Equal(t, tahun, sasaran[i].Tahun)
		assert.InDelta(t, 1.5, sasaran[i].BatasBawah, 1e-9, "sasaran 2,5% ± 1")
		assert.InDelta(t, 3.5, sasaran[i].BatasAtas, 1e-9)
	}
	for _, tabel := range []string{"inflasi_bulanan", "andil_inflasi", "neraca_pangan", "ruang_fiskal"} {
		assert.Zero(t, jumlah(t, db, tabel), "referensi tidak boleh membawa angka: %s", tabel)
	}
}

func TestSeedEksekutif_TwelveMonthsEndingLastMonth(t *testing.T) {
	db := seedDB(t, "eksekutif_seed_12bulan")

	require.NoError(t, seed.SeedFiles(t.Context(), db, berkasDemoEksekutif))

	lalu := bulanLalu(t, db)
	var rows []struct {
		Periode     time.Time
		WilayahKode string
		Ihk         float64
		InflasiMtm  float64
		InflasiYoy  float64
		InflasiYtd  float64
		Sumber      string
	}
	require.NoError(t, db.Raw(`SELECT periode, wilayah_kode, ihk, inflasi_mtm, inflasi_yoy, inflasi_ytd, sumber
		FROM inflasi_bulanan ORDER BY periode`).Scan(&rows).Error)
	require.Len(t, rows, 12)
	assert.Equal(t, tanggalSQL(lalu.AddDate(0, -11, 0)), tanggalSQL(rows[0].Periode))
	assert.Equal(t, tanggalSQL(lalu), tanggalSQL(rows[11].Periode))
	assert.InDelta(t, 2.84, rows[11].InflasiYoy, 1e-9, "bulan terakhir setara mockEksekutif")
	assert.InDelta(t, 0.41, rows[11].InflasiMtm, 1e-9)
	assert.InDelta(t, 110.498, rows[11].Ihk, 1e-9)
	for i, r := range rows {
		assert.Equal(t, 1, r.Periode.Day(), "periode selalu tanggal 1")
		assert.Equal(t, "DEMO", r.Sumber)
		assert.Equal(t, "11", r.WilayahKode)
		assert.InDelta(t, float64(r.Periode.Month())*0.213, r.InflasiYtd, 0.0051, "ytd = nomor bulan x 0,213, dibulatkan 2 desimal")
		if i > 0 {
			assert.True(t, r.Periode.After(rows[i-1].Periode))
		}
	}
}

func TestSeedEksekutif_IsIdempotentAndAllRowsAreDemo(t *testing.T) {
	db := seedDB(t, "eksekutif_seed_idem")

	for range 2 {
		require.NoError(t, seed.SeedFiles(t.Context(), db, berkasDemoEksekutif))
	}

	assert.EqualValues(t, 12, jumlah(t, db, "inflasi_bulanan"))
	assert.EqualValues(t, 6, jumlah(t, db, "andil_inflasi"))
	assert.EqualValues(t, 8, jumlah(t, db, "neraca_pangan"))
	assert.EqualValues(t, 4, jumlah(t, db, "ruang_fiskal"))
	for _, tabel := range []string{"inflasi_bulanan", "andil_inflasi", "neraca_pangan", "ruang_fiskal"} {
		assert.Zero(t, jumlah(t, db, tabel, "sumber <> 'DEMO'"), "%s: semua baris seed harus ber-sumber DEMO", tabel)
	}
	// Andil, neraca, dan fiskal jatuh pada bulan lalu / akhir bulan lalu.
	lalu := bulanLalu(t, db)
	assert.EqualValues(t, 6, jumlah(t, db, "andil_inflasi", "periode = '"+tanggalSQL(lalu)+"'"))
	assert.EqualValues(t, 8, jumlah(t, db, "neraca_pangan", "periode = '"+tanggalSQL(lalu)+"'"))
	akhirBulan := lalu.AddDate(0, 1, -1)
	assert.EqualValues(t, 4, jumlah(t, db, "ruang_fiskal", "per_tanggal = '"+tanggalSQL(akhirBulan)+"'"))
	var pagu int64
	require.NoError(t, db.Raw(`SELECT SUM(pagu) FROM ruang_fiskal`).Scan(&pagu).Error)
	assert.EqualValues(t, 186_000_000_000, pagu, "pagu setara mockEksekutif")
}

func TestSeedEksekutif_NeverOverwritesOfficialRows(t *testing.T) {
	db := seedDB(t, "eksekutif_seed_resmi")
	lalu := bulanLalu(t, db)
	akhirBulan := lalu.AddDate(0, 1, -1)
	require.NoError(t, db.Exec(`INSERT INTO inflasi_bulanan (wilayah_kode, periode, ihk, inflasi_mtm, inflasi_yoy, inflasi_ytd, sumber)
		VALUES ('11', ?, 111, 0.5, 3.1, 2.5, 'BPS')`, lalu).Error)
	require.NoError(t, db.Exec(`INSERT INTO andil_inflasi (periode, wilayah_kode, komoditas_kode, andil, inflasi_pct, sumber)
		VALUES (?, '11', 'CBM-KRT', 0.5, 20, 'BPS')`, lalu).Error)
	require.NoError(t, db.Exec(`INSERT INTO neraca_pangan (komoditas_kode, wilayah_kode, periode, produksi_ton, konsumsi_ton, sumber)
		VALUES ('BRS-MED', '11', ?, 10, 5, 'BAPANAS')`, lalu).Error)
	require.NoError(t, db.Exec(`INSERT INTO ruang_fiskal (kode, tahun, instrumen, pagu, realisasi, per_tanggal, sumber)
		VALUES ('DJPB-RESMI', ?, 'Belanja tidak terduga (BTT)', 1, 1, ?, 'DJPB')`, akhirBulan.Year(), akhirBulan).Error)

	for range 2 {
		require.NoError(t, seed.SeedFiles(t.Context(), db, berkasDemoEksekutif))
	}

	var bps struct {
		Sumber     string
		InflasiYoy float64
	}
	require.NoError(t, db.Raw(`SELECT sumber, inflasi_yoy FROM inflasi_bulanan WHERE wilayah_kode = '11' AND periode = ?`, lalu).Scan(&bps).Error)
	assert.Equal(t, "BPS", bps.Sumber)
	assert.InDelta(t, 3.1, bps.InflasiYoy, 1e-9, "angka BPS tidak boleh tertimpa data demo")
	var andil float64
	require.NoError(t, db.Raw(`SELECT andil FROM andil_inflasi WHERE periode = ? AND komoditas_kode = 'CBM-KRT'`, lalu).Scan(&andil).Error)
	assert.InDelta(t, 0.5, andil, 1e-9)
	var produksi float64
	require.NoError(t, db.Raw(`SELECT produksi_ton FROM neraca_pangan WHERE periode = ? AND komoditas_kode = 'BRS-MED'`, lalu).Scan(&produksi).Error)
	assert.InDelta(t, 10, produksi, 1e-9)
	var resmi struct {
		Sumber string
		Pagu   int64
	}
	require.NoError(t, db.Raw(`SELECT sumber, pagu FROM ruang_fiskal WHERE kode = 'DJPB-RESMI'`).Scan(&resmi).Error)
	assert.Equal(t, "DJPB", resmi.Sumber)
	assert.EqualValues(t, 1, resmi.Pagu)
	assert.EqualValues(t, 3, jumlah(t, db, "ruang_fiskal", "sumber = 'DEMO'"), "instrumen yang sudah resmi dilewati, bukan menabrak UNIQUE")
	assert.EqualValues(t, 11, jumlah(t, db, "inflasi_bulanan", "sumber = 'DEMO'"))
}

func TestSeedEksekutif_RefreshesItsOwnRows(t *testing.T) {
	db := seedDB(t, "eksekutif_seed_segar")
	require.NoError(t, seed.SeedFiles(t.Context(), db, berkasDemoEksekutif))
	require.NoError(t, db.Exec(`UPDATE inflasi_bulanan SET inflasi_yoy = 9 WHERE sumber = 'DEMO'`).Error)
	require.NoError(t, db.Exec(`UPDATE andil_inflasi SET andil = 0.9 WHERE sumber = 'DEMO'`).Error)
	require.NoError(t, db.Exec(`UPDATE neraca_pangan SET produksi_ton = 1 WHERE sumber = 'DEMO'`).Error)
	require.NoError(t, db.Exec(`UPDATE ruang_fiskal SET realisasi = 0 WHERE sumber = 'DEMO'`).Error)

	require.NoError(t, seed.SeedFiles(t.Context(), db, berkasDemoEksekutif))

	lalu := bulanLalu(t, db)
	var yoy, andil, produksi float64
	var realisasi int64
	require.NoError(t, db.Raw(`SELECT inflasi_yoy FROM inflasi_bulanan WHERE periode = ?`, lalu).Scan(&yoy).Error)
	require.NoError(t, db.Raw(`SELECT andil FROM andil_inflasi WHERE periode = ? AND komoditas_kode = 'CBM-KRT'`, lalu).Scan(&andil).Error)
	require.NoError(t, db.Raw(`SELECT produksi_ton FROM neraca_pangan WHERE periode = ? AND komoditas_kode = 'BRS-MED'`, lalu).Scan(&produksi).Error)
	require.NoError(t, db.Raw(`SELECT realisasi FROM ruang_fiskal WHERE instrumen = 'Operasi pasar murah'`).Scan(&realisasi).Error)
	assert.InDelta(t, 2.84, yoy, 1e-9)
	assert.InDelta(t, 0.18, andil, 1e-9)
	assert.InDelta(t, 146200, produksi, 1e-9)
	assert.EqualValues(t, 11_200_000_000, realisasi)
}

func TestSeedEksekutif_NeracaSurplusIsDerivedByTheDatabase(t *testing.T) {
	db := seedDB(t, "eksekutif_seed_neraca")
	require.NoError(t, seed.SeedFiles(t.Context(), db, berkasDemoEksekutif))

	var beras struct {
		SurplusTon float64
		StokTon    *float64
	}
	require.NoError(t, db.Raw(`SELECT surplus_ton, stok_ton FROM neraca_pangan WHERE komoditas_kode = 'BRS-MED'`).Scan(&beras).Error)

	assert.InDelta(t, 146200-52400, beras.SurplusTon, 0.05)
	assert.Nil(t, beras.StokTon, "stok tidak diketahui, bukan 0")
}

// Seed mengunci baris dengan kunci alami (UNIQUE), bukan ID identity: jalan ulang memperbarui baris yang ada tanpa mengganti
// ID-nya dan tanpa menambah baris; kode fiskal ('DEMO-RF-<YYYYMM>-<urut>') ikut kunci itu.
func TestSeedEksekutif_IdentityIDsStayAndNaturalKeysGuardUniqueness(t *testing.T) {
	db := seedDB(t, "eksekutif_seed_id")
	require.NoError(t, seed.SeedFiles(t.Context(), db, berkasDemoEksekutif))
	ids := func() (inflasi, fiskal []int64) {
		require.NoError(t, db.Raw(`SELECT id FROM inflasi_bulanan ORDER BY periode`).Scan(&inflasi).Error)
		require.NoError(t, db.Raw(`SELECT id FROM ruang_fiskal ORDER BY kode`).Scan(&fiskal).Error)
		return inflasi, fiskal
	}
	inflasiSebelum, fiskalSebelum := ids()
	require.Len(t, inflasiSebelum, 12)
	require.Len(t, fiskalSebelum, 4)
	assert.Positive(t, inflasiSebelum[0])

	require.NoError(t, seed.SeedFiles(t.Context(), db, berkasDemoEksekutif))

	inflasiSesudah, fiskalSesudah := ids()
	assert.Equal(t, inflasiSebelum, inflasiSesudah, "ID tidak berganti oleh upsert")
	assert.Equal(t, fiskalSebelum, fiskalSesudah)
	assert.EqualValues(t, 4, jumlah(t, db, "ruang_fiskal", "kode LIKE 'DEMO-RF-%'"))

	// Kunci alami ganda ditolak basis data.
	err := db.Exec(`INSERT INTO inflasi_bulanan (wilayah_kode, periode, ihk, inflasi_mtm, inflasi_yoy, inflasi_ytd, sumber)
		SELECT wilayah_kode, periode, ihk, inflasi_mtm, inflasi_yoy, inflasi_ytd, sumber FROM inflasi_bulanan LIMIT 1`).Error
	require.Error(t, err)
	assert.Contains(t, err.Error(), "uq_inflasi_bulanan_alami")
}

// Berkas seed relatif terhadap tanggal eksekusi: batas bulan, awal tahun, dan tahun kabisat tetap menghasilkan 12 bulan
// berurutan yang berakhir di bulan lalu dan fiskal per akhir bulan lalu (tahun fiskal mengikuti tanggal itu).
func TestSeedEksekutif_FollowsTheExecutionDateAcrossMonthAndYearBoundaries(t *testing.T) {
	for _, hari := range []string{"2027-01-10", "2028-03-05", "2026-12-31", "2026-03-01"} {
		t.Run(hari, func(t *testing.T) {
			db := seedDB(t, "eksekutif_seed_"+strings.ReplaceAll(hari, "-", ""))
			tgl, err := time.Parse("2006-01-02", hari)
			require.NoError(t, err)
			akhir := time.Date(tgl.Year(), tgl.Month()-1, 1, 0, 0, 0, 0, time.UTC) // bulan lalu
			perTanggal := akhir.AddDate(0, 1, -1)

			jalankanSebagai(t, db, berkasDemoEksekutif, hari+" 12:00:00+00", "")

			var periode []time.Time
			require.NoError(t, db.Raw(`SELECT periode FROM inflasi_bulanan ORDER BY periode`).Scan(&periode).Error)
			require.Len(t, periode, 12)
			assert.Equal(t, tanggalSQL(akhir.AddDate(0, -11, 0)), tanggalSQL(periode[0]))
			assert.Equal(t, tanggalSQL(akhir), tanggalSQL(periode[11]))
			var fiskal struct {
				Tahun      int
				PerTanggal time.Time
				Kode       string
			}
			require.NoError(t, db.Raw(`SELECT tahun, per_tanggal, kode FROM ruang_fiskal ORDER BY kode LIMIT 1`).Scan(&fiskal).Error)
			assert.Equal(t, perTanggal.Year(), fiskal.Tahun)
			assert.Equal(t, tanggalSQL(perTanggal), tanggalSQL(fiskal.PerTanggal))
			assert.Equal(t, "DEMO-RF-"+perTanggal.Format("200601")+"-1", fiskal.Kode)
			assert.EqualValues(t, 6, jumlah(t, db, "andil_inflasi", "periode = '"+tanggalSQL(akhir)+"'"))
		})
	}
}

// Dijalankan pada bulan berikutnya, seed menambah deret bulan baru tanpa menyentuh baris demo bulan lama.
func TestSeedEksekutif_RollsForwardWhenRunInALaterMonth(t *testing.T) {
	db := seedDB(t, "eksekutif_seed_geser")
	jalankanSebagai(t, db, berkasDemoEksekutif, "2026-11-09 12:00:00+00", "")
	jalankanSebagai(t, db, berkasDemoEksekutif, "2026-12-09 12:00:00+00", "")

	assert.EqualValues(t, 13, jumlah(t, db, "inflasi_bulanan"), "deret Nov 2025 - Okt 2026 ditambah Nov 2026")
	assert.EqualValues(t, 8, jumlah(t, db, "ruang_fiskal"), "satu set fiskal per akhir bulan")
	var terbaru time.Time
	require.NoError(t, db.Raw(`SELECT max(periode) FROM inflasi_bulanan`).Scan(&terbaru).Error)
	assert.Equal(t, "2026-11-01", tanggalSQL(terbaru))
}

// "Bulan lalu" mengikuti UTC, bukan zona sesi: sesi aplikasi/tes (TimeZone=Asia/Jakarta) dan sesi cmd/seed (zona bawaan
// server) menghasilkan data yang sama. Pada 31 Desember 2026 pukul 20.00 UTC tanggal lokal sudah 1 Januari 2027 di WIB dan
// zona yang lebih timur, tetapi bulan UTC masih Desember: bulan lalu adalah November, bukan Desember.
func TestSeedEksekutif_IsIndependentOfTheSessionTimeZone(t *testing.T) {
	for _, zona := range []string{"UTC", "Asia/Jakarta", "Pacific/Kiritimati", "Pacific/Pago_Pago"} {
		t.Run(zona, func(t *testing.T) {
			db := seedDB(t, "eksekutif_seed_zona_"+strings.NewReplacer("/", "_", "-", "_").Replace(strings.ToLower(zona)))

			jalankanSebagai(t, db, berkasDemoEksekutif, "2026-12-31 20:00:00+00", zona)

			var periode []time.Time
			require.NoError(t, db.Raw(`SELECT periode FROM inflasi_bulanan ORDER BY periode`).Scan(&periode).Error)
			require.Len(t, periode, 12)
			assert.Equal(t, "2025-12-01", tanggalSQL(periode[0]))
			assert.Equal(t, "2026-11-01", tanggalSQL(periode[11]), "bulan lalu menurut UTC")
			var perTanggal time.Time
			require.NoError(t, db.Raw(`SELECT max(per_tanggal) FROM ruang_fiskal`).Scan(&perTanggal).Error)
			assert.Equal(t, "2026-11-30", tanggalSQL(perTanggal))
		})
	}
}
