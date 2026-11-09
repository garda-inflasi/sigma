package integration

// Tes seed SQL dashboard: target indikator (db/seeds/reference/13_kpi_definisi.sql) serta siklus TEKANAN contoh dan
// penyegaran agregat (db/seeds/demo/16_refresh_views.sql). Menggantikan tes seeder Go lama (dashboard/seeder): target
// idempoten menurut kunci alami, siklus contoh terkunci pada kode sehingga tanggal mulai bergeser tanpa menggandakan
// baris atau mengganti ID, dan agregat dihitung dari data harga yang sudah di-seed.

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

const berkasSegarkan = "demo/16_refresh_views.sql"

// seedDB membuat basis data baru bermigrasi lalu menjalankan berkas seed (relatif terhadap db/seeds) berurutan.
func seedDB(t *testing.T, name string, berkas ...string) *gorm.DB {
	t.Helper()
	db := pg.NewDatabase(t, name)
	pgtest.MigrateUp(t, db)
	require.NoError(t, seed.SeedFiles(t.Context(), db, berkas...))
	return db
}

func jumlah(t *testing.T, db *gorm.DB, tabel string) int64 {
	t.Helper()
	var n int64
	require.NoError(t, db.Table(tabel).Count(&n).Error)
	return n
}

func TestSeedDashboard_TargetKPIIsIdempotentByNaturalKey(t *testing.T) {
	db := seedDB(t, "dashboard_seed_target", "reference/13_kpi_definisi.sql")

	require.NoError(t, seed.SeedFiles(t.Context(), db, "reference/13_kpi_definisi.sql"))

	assert.EqualValues(t, 3, jumlah(t, db, "target_kpi"))
	var target []struct {
		Indikator string
		Target    float64
		Arah      string
	}
	require.NoError(t, db.Raw(`SELECT indikator, target, arah FROM target_kpi WHERE periode_mulai = '2026-01-01' ORDER BY indikator`).Scan(&target).Error)
	require.Len(t, target, 3)
	assert.Equal(t, "cs", target[0].Indikator)
	assert.InDelta(t, 75, target[0].Target, 1e-9)
	assert.Equal(t, "NAIK", target[0].Arah)
	assert.Equal(t, "validasi", target[1].Indikator)
	assert.InDelta(t, 85, target[1].Target, 1e-9)
	assert.Equal(t, "waspada", target[2].Indikator)
	assert.InDelta(t, 3, target[2].Target, 1e-9)
	assert.Equal(t, "TURUN", target[2].Arah)

	err := db.Exec(`INSERT INTO target_kpi (indikator, periode_mulai, target, arah) VALUES ('validasi', '2026-01-01', 1, 'NAIK')`).Error
	require.Error(t, err)
	assert.Contains(t, err.Error(), "uq_target_kpi_alami")
}

// Siklus contoh dikunci dengan kode (UNIQUE), bukan ID identity: dijalankan di hari berikutnya tetap satu siklus, tanggal mulai
// bergeser, dan ID-nya tidak berganti.
func TestSeedDashboard_SiklusDemoKeepsItsCodeAndShiftsItsStart(t *testing.T) {
	db := seedDB(t, "dashboard_seed_siklus", "reference/01_master.sql", berkasSegarkan)
	type siklus struct {
		ID        int64
		Kode      string
		Siklus    string
		MulaiPada time.Time
		AkhirPada *time.Time
	}
	baca := func() []siklus {
		var rows []siklus
		require.NoError(t, db.Raw(`SELECT id, kode, siklus::text AS siklus, mulai_pada, akhir_pada FROM siklus_ekonomi WHERE wilayah_kode = '11'`).Scan(&rows).Error)
		return rows
	}
	var sekarang time.Time
	require.NoError(t, db.Raw(`SELECT now()`).Scan(&sekarang).Error)

	pertama := baca()

	require.Len(t, pertama, 1)
	assert.Equal(t, "SE-11-DEMO", pertama[0].Kode)
	assert.Equal(t, "TEKANAN", pertama[0].Siklus)
	assert.Nil(t, pertama[0].AkhirPada, "siklus aktif")
	assert.WithinDuration(t, sekarang.AddDate(0, 0, -6), pertama[0].MulaiPada, time.Minute, "dimulai enam hari sebelum eksekusi")

	// Dijalankan lagi dua hari kemudian (now() digeser): satu siklus, ID tetap, mulai ikut bergeser.
	dir, err := seed.LocateSeedsDir()
	require.NoError(t, err)
	raw, err := os.ReadFile(filepath.Join(dir, berkasSegarkan))
	require.NoError(t, err)
	nanti := strings.ReplaceAll(string(raw), "now()", "(now() + interval '2 days')")
	require.NotEqual(t, string(raw), nanti, "berkas tidak memakai now()")
	sqlDB, err := db.DB()
	require.NoError(t, err)
	_, err = sqlDB.ExecContext(t.Context(), nanti)
	require.NoError(t, err)

	kedua := baca()

	require.Len(t, kedua, 1)
	assert.Equal(t, pertama[0].ID, kedua[0].ID, "upsert menurut kode tidak mengganti ID")
	assert.WithinDuration(t, pertama[0].MulaiPada.Add(48*time.Hour), kedua[0].MulaiPada, time.Minute)
	assert.EqualValues(t, 1, jumlah(t, db, "siklus_ekonomi"))

	// Berkas yang sama dijalankan ulang tanpa pergeseran waktu: tetap satu baris.
	require.NoError(t, seed.SeedFiles(t.Context(), db, berkasSegarkan))
	assert.EqualValues(t, 1, jumlah(t, db, "siklus_ekonomi"))
}

// Tabel yang tidak terisi tidak menggeser sequence-nya: ID pertama tetap 1 walau hanya sebagian berkas seed yang dijalankan.
func TestSeedDashboard_EmptyTablesDoNotShiftSequences(t *testing.T) {
	db := seedDB(t, "dashboard_seed_sequence", "reference/01_master.sql", berkasSegarkan)

	var berikut int64
	require.NoError(t, db.Raw(`SELECT nextval(pg_get_serial_sequence('entri_harga', 'id'))`).Scan(&berikut).Error)

	assert.EqualValues(t, 1, berikut)
}

// Agregat dashboard tidak di-seed langsung; ia dihitung dari entri harga demo oleh berkas penyegaran.
func TestSeedDashboard_RefreshFillsAggregatesFromPriceData(t *testing.T) {
	s := sharedSuite(t)

	assert.Positive(t, jumlah(t, s.db, "agregat_harga_harian"), "agregat dihitung dari entri harga")
	assert.Positive(t, jumlah(t, s.db, "mv_heatmap_harga"))
	assert.Positive(t, jumlah(t, s.db, "gold_harga_harian"))
	assert.EqualValues(t, 4*14, jumlah(t, s.db, "mv_kpi_operasional"), "empat indikator kali empat belas hari")
}
