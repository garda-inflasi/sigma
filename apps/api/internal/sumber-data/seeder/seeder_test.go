package seeder

import (
	"os"
	"testing"
	"time"

	"github.com/garda-inflasi/api/internal/platform/pgtest"
	"github.com/garda-inflasi/api/internal/sumber-data/domain"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

var pg *pgtest.Server

func TestMain(m *testing.M) {
	os.Exit(pgtest.Main(m, &pg))
}

var waktuTetap = time.Date(2026, 9, 30, 8, 0, 0, 0, time.UTC)

func newDB(t *testing.T, name string) *gorm.DB {
	t.Helper()
	db := pg.NewDatabase(t, name)
	pgtest.MigrateUp(t, db)
	return db
}

func count(t *testing.T, db *gorm.DB) int64 {
	t.Helper()
	var n int64
	require.NoError(t, db.Model(&domain.SumberData{}).Count(&n).Error)
	return n
}

func TestKatalog_Shape(t *testing.T) {
	rows := Katalog()
	require.Len(t, rows, 8, "setara dengan mockSumber")

	ids := map[string]bool{}
	for _, r := range rows {
		assert.False(t, ids[r.ID], "id ganda %s", r.ID)
		ids[r.ID] = true
		assert.Contains(t, []string{domain.JenisAPI, domain.JenisScraper}, r.Jenis, r.ID)
		assert.NotEmpty(t, r.JadwalCron, r.ID)
		assert.NotEmpty(t, r.Atribusi, r.ID)
		assert.True(t, r.RobotsTxtPatuh, r.ID)
		assert.Contains(t, demoKeadaan, r.ID, "tiap sumber katalog punya keadaan demo")
	}
}

func TestSeedReference_StatusIsHonest(t *testing.T) {
	db := newDB(t, "sumber_ref_honest")

	sum, err := SeedReference(t.Context(), db)

	require.NoError(t, err)
	assert.Equal(t, 8, sum.Sumber)
	var rows []domain.SumberData
	require.NoError(t, db.Find(&rows).Error)
	require.Len(t, rows, 8)
	for _, r := range rows {
		assert.Equal(t, domain.StatusBelumAktif, r.Status, r.ID)
		assert.Nil(t, r.TerakhirJalan, r.ID)
		assert.Nil(t, r.Keberhasilan7h, r.ID)
		assert.Zero(t, r.LatensiMs, r.ID)
	}
}

func TestSeedReference_IsIdempotentAndKeepsOperationalState(t *testing.T) {
	db := newDB(t, "sumber_ref_idem")
	_, err := SeedReference(t.Context(), db)
	require.NoError(t, err)
	require.NoError(t, db.Exec(`UPDATE sumber_data SET nama = 'diubah', status = 'DIJEDA' WHERE id = 'SRC-BPS'`).Error)

	_, err = SeedReference(t.Context(), db)
	require.NoError(t, err)

	assert.EqualValues(t, 8, count(t, db))
	var bps domain.SumberData
	require.NoError(t, db.First(&bps, "id = ?", "SRC-BPS").Error)
	assert.Equal(t, "BPS WebAPI — IHK & inflasi", bps.Nama, "identitas katalog dipulihkan")
	assert.Equal(t, domain.StatusDijeda, bps.Status, "status operasional tidak boleh ditimpa")
}

func TestSeedDemo_IsIdempotentAndDeterministic(t *testing.T) {
	db := newDB(t, "sumber_demo_idem")
	_, err := SeedReference(t.Context(), db)
	require.NoError(t, err)

	_, err = SeedDemo(t.Context(), db, waktuTetap)
	require.NoError(t, err)
	var first []domain.SumberData
	require.NoError(t, db.Order("id").Find(&first).Error)
	_, err = SeedDemo(t.Context(), db, waktuTetap)
	require.NoError(t, err)
	var second []domain.SumberData
	require.NoError(t, db.Order("id").Find(&second).Error)

	assert.EqualValues(t, 8, count(t, db))
	assert.Equal(t, first, second)

	by := map[string]domain.SumberData{}
	for _, r := range second {
		by[r.ID] = r
	}
	assert.Equal(t, domain.StatusGagal, by["SRC-PORTAL"].Status)
	assert.Equal(t, domain.StatusTertunda, by["SRC-BAPANAS"].Status)
	require.NotNil(t, by["SRC-BI"].TerakhirJalan)
	assert.True(t, by["SRC-BI"].TerakhirJalan.Equal(waktuTetap.Add(-38*time.Minute)), "waktu jalan relatif terhadap now")
	require.NotNil(t, by["SRC-BI"].Keberhasilan7h)
	assert.InDelta(t, 98.6, *by["SRC-BI"].Keberhasilan7h, 0.001)
}

func TestSeedDemo_StandsAloneWithoutReference(t *testing.T) {
	db := newDB(t, "sumber_demo_alone")

	_, err := SeedDemo(t.Context(), db, waktuTetap)

	require.NoError(t, err)
	assert.EqualValues(t, 8, count(t, db))
}
