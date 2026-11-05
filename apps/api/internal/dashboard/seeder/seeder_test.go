package seeder

import (
	"os"
	"testing"
	"time"

	authseeder "github.com/garda-inflasi/api/internal/auth/seeder"
	hargaseeder "github.com/garda-inflasi/api/internal/harga/seeder"
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

var seedNow = time.Now()

func count(t *testing.T, db *gorm.DB, table string) (n int64) {
	t.Helper()
	require.NoError(t, db.Raw("SELECT COUNT(*) FROM "+table).Scan(&n).Error)
	return n
}

func TestSeedReference_IsIdempotent(t *testing.T) {
	db := pg.NewDatabase(t, "dashboard_seed_ref")
	pgtest.MigrateUp(t, db)

	first, err := SeedReference(t.Context(), db)
	require.NoError(t, err)
	second, err := SeedReference(t.Context(), db)
	require.NoError(t, err)

	assert.Equal(t, len(targetAwal), first.Target)
	assert.Equal(t, first, second)
	assert.EqualValues(t, len(targetAwal), count(t, db, "target_kpi"))
}

func TestSeedDemo_CreatesActiveTekananCycleAndFillsAggregates(t *testing.T) {
	db := pg.NewDatabase(t, "dashboard_seed_demo")
	pgtest.MigrateUp(t, db)
	ctx := t.Context()
	_, err := masterseeder.SeedReference(ctx, db)
	require.NoError(t, err)
	_, err = authseeder.SeedDemo(ctx, db)
	require.NoError(t, err)
	_, err = hargaseeder.SeedDemo(ctx, db, seedNow)
	require.NoError(t, err)

	sum, err := SeedDemo(ctx, db, seedNow)
	require.NoError(t, err)
	assert.Equal(t, 1, sum.Siklus)

	var siklus []struct {
		Siklus    string
		MulaiPada time.Time
	}
	require.NoError(t, db.Raw("SELECT siklus, mulai_pada FROM siklus_ekonomi WHERE wilayah_kode = '11' AND akhir_pada IS NULL").Scan(&siklus).Error)
	require.Len(t, siklus, 1)
	assert.Equal(t, "TEKANAN", siklus[0].Siklus)
	assert.WithinDuration(t, seedNow.AddDate(0, 0, -hariTekanan), siklus[0].MulaiPada, time.Second)

	assert.Positive(t, count(t, db, "agregat_harga_harian"), "agregat dihitung dari entri harga")
	assert.Positive(t, count(t, db, "mv_heatmap_harga"))
	assert.EqualValues(t, 4*14, count(t, db, "mv_kpi_operasional"), "empat indikator kali empat belas hari")

	// Dijalankan ulang di hari berikutnya: tetap satu siklus, tanggal mulai bergeser.
	besok := seedNow.Add(24 * time.Hour)
	_, err = SeedDemo(ctx, db, besok)
	require.NoError(t, err)
	assert.EqualValues(t, 1, count(t, db, "siklus_ekonomi"))
	var mulai time.Time
	require.NoError(t, db.Raw("SELECT mulai_pada FROM siklus_ekonomi WHERE id = ?", idSiklusDemo).Scan(&mulai).Error)
	assert.WithinDuration(t, besok.AddDate(0, 0, -hariTekanan), mulai, time.Second)
}
