package seeder

import (
	"os"
	"testing"
	"time"

	"github.com/garda-inflasi/api/internal/eksekutif/domain"
	masterseeder "github.com/garda-inflasi/api/internal/master/seeder"
	"github.com/garda-inflasi/api/internal/platform/pgtest"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

var pg *pgtest.Server

func TestMain(m *testing.M) {
	os.Exit(pgtest.Main(m, &pg))
}

var waktuTetap = time.Date(2026, 11, 5, 8, 0, 0, 0, time.UTC)

func newDB(t *testing.T, name string) *gorm.DB {
	t.Helper()
	db := pg.NewDatabase(t, name)
	pgtest.MigrateUp(t, db)
	_, err := masterseeder.SeedReference(t.Context(), db)
	require.NoError(t, err)
	return db
}

func counts(t *testing.T, db *gorm.DB) map[string]int64 {
	t.Helper()
	out := map[string]int64{}
	for _, table := range []string{"inflasi_bulanan", "andil_inflasi", "neraca_pangan", "ruang_fiskal", "target_inflasi"} {
		var n int64
		require.NoError(t, db.Table(table).Count(&n).Error)
		out[table] = n
	}
	return out
}

func TestBuildInflasi_TwelveMonthsEndingLastMonth(t *testing.T) {
	rows := BuildInflasi(waktuTetap)

	require.Len(t, rows, 12)
	assert.Equal(t, time.Date(2025, 11, 1, 0, 0, 0, 0, time.UTC), rows[0].Periode)
	assert.Equal(t, time.Date(2026, 10, 1, 0, 0, 0, 0, time.UTC), rows[11].Periode)
	assert.InDelta(t, 2.84, rows[11].InflasiYoy, 1e-9, "bulan terakhir setara mockEksekutif")
	assert.InDelta(t, 0.41, rows[11].InflasiMtm, 1e-9)
	for i, r := range rows {
		assert.Equal(t, 1, r.Periode.Day(), "periode selalu tanggal 1")
		assert.Equal(t, domain.SumberDemo, r.Sumber)
		assert.Equal(t, domain.WilayahProvinsi, r.WilayahKode)
		if i > 0 {
			assert.True(t, r.Periode.After(rows[i-1].Periode))
		}
	}
}

func TestBuildFiskal_LastDayOfLastMonth(t *testing.T) {
	rows := BuildFiskal(waktuTetap)

	require.Len(t, rows, 4)
	assert.Equal(t, time.Date(2026, 10, 31, 0, 0, 0, 0, time.UTC), rows[0].PerTanggal)
	var pagu int64
	kode := map[string]bool{}
	for _, r := range rows {
		pagu += r.Pagu
		require.NotNil(t, r.Kode, "kode adalah ID lama dan kunci baris seed; ID dibuat basis data")
		assert.Zero(t, r.ID)
		assert.False(t, kode[*r.Kode], "kode ganda %s", *r.Kode)
		kode[*r.Kode] = true
		assert.LessOrEqual(t, r.Realisasi, r.Pagu)
		assert.Equal(t, domain.SumberDemo, r.Sumber)
	}
	assert.EqualValues(t, 186_000_000_000, pagu, "pagu setara mockEksekutif")
}

func TestBuildFiskal_JanuaryReportsDecemberOfPreviousYear(t *testing.T) {
	rows := BuildFiskal(time.Date(2027, 1, 10, 0, 0, 0, 0, time.UTC))

	assert.Equal(t, 2026, rows[0].Tahun)
	assert.Equal(t, time.Date(2026, 12, 31, 0, 0, 0, 0, time.UTC), rows[0].PerTanggal)
}

func TestSeedReference_IsIdempotent(t *testing.T) {
	db := newDB(t, "eksekutif_ref_idem")

	for range 2 {
		sum, err := SeedReference(t.Context(), db)
		require.NoError(t, err)
		assert.Equal(t, 3, sum.Target)
	}

	assert.EqualValues(t, 3, counts(t, db)["target_inflasi"])
	assert.Zero(t, counts(t, db)["inflasi_bulanan"], "referensi tidak boleh membawa angka inflasi")
}

func TestSeedDemo_IsIdempotentAndAllRowsAreDemo(t *testing.T) {
	db := newDB(t, "eksekutif_demo_idem")

	for range 2 {
		sum, err := SeedDemo(t.Context(), db, waktuTetap)
		require.NoError(t, err)
		assert.Equal(t, DemoSummary{Inflasi: 12, Andil: 6, Neraca: 8, Fiskal: 4}, sum)
	}

	got := counts(t, db)
	assert.EqualValues(t, 12, got["inflasi_bulanan"])
	assert.EqualValues(t, 6, got["andil_inflasi"])
	assert.EqualValues(t, 8, got["neraca_pangan"])
	assert.EqualValues(t, 4, got["ruang_fiskal"])
	for _, table := range []string{"inflasi_bulanan", "andil_inflasi", "neraca_pangan", "ruang_fiskal"} {
		var bukanDemo int64
		require.NoError(t, db.Table(table).Where("sumber <> ?", domain.SumberDemo).Count(&bukanDemo).Error)
		assert.Zero(t, bukanDemo, "%s: semua baris seed harus ber-sumber DEMO", table)
	}
}

func TestSeedDemo_NeverOverwritesOfficialRows(t *testing.T) {
	db := newDB(t, "eksekutif_demo_official")
	periode := time.Date(2026, 10, 1, 0, 0, 0, 0, time.UTC)
	require.NoError(t, db.Create(&domain.InflasiBulanan{
		WilayahKode: domain.WilayahProvinsi, Periode: periode, IHK: 111, InflasiMtm: 0.5, InflasiYoy: 3.1, InflasiYtd: 2.5, Sumber: domain.SumberBPS,
	}).Error)
	require.NoError(t, db.Create(&domain.RuangFiskal{
		Kode: new("DJPB-2026-10"), Tahun: 2026, Instrumen: "Belanja tidak terduga (BTT)", Pagu: 1, Realisasi: 1,
		PerTanggal: time.Date(2026, 10, 31, 0, 0, 0, 0, time.UTC), Sumber: domain.SumberDJPB,
	}).Error)

	_, err := SeedDemo(t.Context(), db, waktuTetap)

	require.NoError(t, err)
	var bps domain.InflasiBulanan
	require.NoError(t, db.First(&bps, "wilayah_kode = ? AND periode = ?", domain.WilayahProvinsi, periode).Error)
	assert.Equal(t, domain.SumberBPS, bps.Sumber)
	assert.InDelta(t, 3.1, bps.InflasiYoy, 1e-9, "angka BPS tidak boleh tertimpa data demo")
	var resmi domain.RuangFiskal
	require.NoError(t, db.First(&resmi, "kode = ?", "DJPB-2026-10").Error)
	assert.EqualValues(t, 1, resmi.Pagu)
}

func TestSeedDemo_RefreshesItsOwnRows(t *testing.T) {
	db := newDB(t, "eksekutif_demo_refresh")
	_, err := SeedDemo(t.Context(), db, waktuTetap)
	require.NoError(t, err)
	require.NoError(t, db.Exec(`UPDATE inflasi_bulanan SET inflasi_yoy = 9 WHERE sumber = 'DEMO'`).Error)

	_, err = SeedDemo(t.Context(), db, waktuTetap)

	require.NoError(t, err)
	var yoy float64
	require.NoError(t, db.Raw(`SELECT inflasi_yoy FROM inflasi_bulanan WHERE periode = '2026-10-01'`).Scan(&yoy).Error)
	assert.InDelta(t, 2.84, yoy, 1e-9)
}

func TestNeracaSurplusIsDerivedByTheDatabase(t *testing.T) {
	db := newDB(t, "eksekutif_neraca_derived")
	_, err := SeedDemo(t.Context(), db, waktuTetap)
	require.NoError(t, err)

	var beras domain.NeracaPangan
	require.NoError(t, db.First(&beras, "komoditas_kode = ?", "BRS-MED").Error)

	require.NotNil(t, beras.SurplusTon)
	assert.InDelta(t, 146200-52400, *beras.SurplusTon, 0.05)
	assert.Nil(t, beras.StokTon, "stok tidak diketahui, bukan 0")
}

// Seeder mengunci baris agregat dengan kunci alami (UNIQUE), bukan ID identity: jalan ulang memperbarui baris yang
// ada tanpa mengganti ID-nya dan tanpa menambah baris, termasuk atas baris hasil migrasi 000115.
func TestSeedDemo_IDIdentityTetapDanKunciAlamiMenjagaKeunikan(t *testing.T) {
	db := newDB(t, "eksekutif_demo_id_tetap")
	_, err := SeedDemo(t.Context(), db, waktuTetap)
	require.NoError(t, err)
	var sebelum []int64
	require.NoError(t, db.Raw(`SELECT id FROM inflasi_bulanan ORDER BY periode`).Scan(&sebelum).Error)
	var fiskalSebelum int64
	require.NoError(t, db.Raw(`SELECT min(id) FROM ruang_fiskal`).Scan(&fiskalSebelum).Error)
	require.Len(t, sebelum, 12)
	assert.Positive(t, sebelum[0])

	_, err = SeedDemo(t.Context(), db, waktuTetap)
	require.NoError(t, err)

	var sesudah []int64
	require.NoError(t, db.Raw(`SELECT id FROM inflasi_bulanan ORDER BY periode`).Scan(&sesudah).Error)
	assert.Equal(t, sebelum, sesudah, "ID tidak berganti oleh upsert")
	var fiskalSesudah int64
	require.NoError(t, db.Raw(`SELECT min(id) FROM ruang_fiskal`).Scan(&fiskalSesudah).Error)
	assert.Equal(t, fiskalSebelum, fiskalSesudah)

	// Kunci alami ganda ditolak basis data.
	err = db.Exec(`INSERT INTO inflasi_bulanan (wilayah_kode, periode, ihk, inflasi_mtm, inflasi_yoy, inflasi_ytd, sumber)
		SELECT wilayah_kode, periode, ihk, inflasi_mtm, inflasi_yoy, inflasi_ytd, sumber FROM inflasi_bulanan LIMIT 1`).Error
	require.Error(t, err)
	assert.Contains(t, err.Error(), "uq_inflasi_bulanan_alami")
}

func TestSeedDemo_BarisFiskalHasilMigrasiTidakDigandakan(t *testing.T) {
	db := newDB(t, "eksekutif_demo_fiskal_migrasi")
	_, err := SeedDemo(t.Context(), db, waktuTetap)
	require.NoError(t, err)
	// Meniru hasil migrasi: baris yang ada memegang kode ID lama, dan seed berikutnya memakai kode yang sama.
	var jumlah int64
	require.NoError(t, db.Raw(`SELECT count(*) FROM ruang_fiskal WHERE kode LIKE 'DEMO-RF-%'`).Scan(&jumlah).Error)
	assert.EqualValues(t, 4, jumlah)

	_, err = SeedDemo(t.Context(), db, waktuTetap)
	require.NoError(t, err)

	require.NoError(t, db.Raw(`SELECT count(*) FROM ruang_fiskal`).Scan(&jumlah).Error)
	assert.EqualValues(t, 4, jumlah)
}
