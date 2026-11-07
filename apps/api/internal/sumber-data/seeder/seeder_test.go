package seeder

import (
	"os"
	"testing"
	"time"

	"github.com/garda-inflasi/api/internal/platform/pgtest"
	"github.com/garda-inflasi/api/internal/sumber-data/domain"
	"github.com/garda-inflasi/api/pkg/idx"
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

	ids, kode := map[string]bool{}, map[string]bool{}
	for _, r := range rows {
		assert.False(t, ids[r.ID], "id ganda %s", r.ID)
		assert.False(t, kode[r.Kode], "kode ganda %s", r.Kode)
		ids[r.ID], kode[r.Kode] = true, true
		assert.Equal(t, IDSumber(r.Kode), r.ID, "id = UUID stabil dari kode (sama dengan hasil migrasi data lama)")
		assert.True(t, idx.ValidUUID(r.ID), r.Kode)
		assert.Contains(t, []string{domain.JenisAPI, domain.JenisScraper}, r.Jenis, r.ID)
		assert.NotEmpty(t, r.JadwalCron, r.ID)
		assert.NotEmpty(t, r.Atribusi, r.ID)
		assert.True(t, r.RobotsTxtPatuh, r.ID)
		assert.Contains(t, demoKeadaan, r.Kode, "tiap sumber katalog punya keadaan demo")
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
		assert.Equal(t, domain.StatusBelumAktif, r.Status, r.Kode)
		assert.Nil(t, r.TerakhirJalan, r.Kode)
		assert.Nil(t, r.Keberhasilan7h, r.Kode)
		assert.Zero(t, r.LatensiMs, r.Kode)
	}
}

func TestSeedReference_IsIdempotentAndKeepsOperationalState(t *testing.T) {
	db := newDB(t, "sumber_ref_idem")
	_, err := SeedReference(t.Context(), db)
	require.NoError(t, err)
	require.NoError(t, db.Exec(`UPDATE sumber_data SET nama = 'diubah', status = 'DIJEDA' WHERE kode = 'SRC-BPS'`).Error)

	_, err = SeedReference(t.Context(), db)
	require.NoError(t, err)

	assert.EqualValues(t, 8, count(t, db))
	var bps domain.SumberData
	require.NoError(t, db.First(&bps, "kode = ?", "SRC-BPS").Error)
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
	require.NoError(t, db.Order("kode").Find(&first).Error)
	_, err = SeedDemo(t.Context(), db, waktuTetap)
	require.NoError(t, err)
	var second []domain.SumberData
	require.NoError(t, db.Order("kode").Find(&second).Error)

	assert.EqualValues(t, 8, count(t, db))
	assert.Equal(t, first, second)

	by := map[string]domain.SumberData{}
	for _, r := range second {
		by[r.Kode] = r
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

// Basis data hasil migrasi 000115 sudah berisi baris sumber dengan kode = ID lama dan UUID stabil: seeder
// mengenalinya lewat kode dan tidak menggandakannya.
func TestSeedReference_TidakMenggandakanBarisHasilMigrasi(t *testing.T) {
	db := newDB(t, "sumber_ref_migrasi")
	require.NoError(t, db.Exec(`
		INSERT INTO sumber_data (id, kode, nama, penyedia, jenis, cakupan, jadwal_cron, status, atribusi)
		VALUES (?, 'SRC-BPS', 'Nama lama', 'BPS', 'API', 'Aceh', '0 6 * * *', 'SEHAT', 'atribusi lama')`, IDSumber("SRC-BPS")).Error)

	_, err := SeedReference(t.Context(), db)
	require.NoError(t, err)
	_, err = SeedReference(t.Context(), db)
	require.NoError(t, err)

	assert.EqualValues(t, 8, count(t, db))
	var bps domain.SumberData
	require.NoError(t, db.First(&bps, "kode = ?", "SRC-BPS").Error)
	assert.Equal(t, IDSumber("SRC-BPS"), bps.ID)
	assert.Equal(t, "BPS WebAPI — IHK & inflasi", bps.Nama, "identitas katalog diperbarui")
	assert.Equal(t, domain.StatusSehat, bps.Status, "status hasil operasi tidak ditimpa")
}
