package postgres

import (
	"os"
	"testing"

	"github.com/garda-inflasi/api/internal/platform/pgtest"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

var pg *pgtest.Server

func TestMain(m *testing.M) {
	os.Exit(pgtest.Main(m, &pg))
}

// denganTabelSementara menjalankan fn pada satu koneksi yang tabel ews_alert dan
// intervensi-nya digantikan tabel sementara berkolom minimal, sehingga tes
// hanya bergantung pada kolom yang dibaca repository (severity, status, siklus, tahap).
func denganTabelSementara(t *testing.T, name string, fn func(repo *Repository, tx *gorm.DB)) {
	t.Helper()
	db := pg.NewDatabase(t, name)
	err := db.Connection(func(tx *gorm.DB) error {
		require.NoError(t, tx.Exec(`CREATE TEMP TABLE ews_alert (severity text, status text)`).Error)
		require.NoError(t, tx.Exec(`CREATE TEMP TABLE intervensi (siklus text, tahap text)`).Error)
		fn(New(tx), tx)
		return nil
	})
	require.NoError(t, err)
}

func TestHitungEWS_CountsOnlyUnfinishedPerSeverity(t *testing.T) {
	denganTabelSementara(t, "eksekutif_repo_ews", func(repo *Repository, tx *gorm.DB) {
		require.NoError(t, tx.Exec(`INSERT INTO ews_alert VALUES
			('KRITIS', 'BARU'), ('KRITIS', 'DIAKUI'), ('KRITIS', 'SELESAI'),
			('SIAGA', 'DIESKALASI'), ('INFO', 'SELESAI')`).Error)

		got, err := repo.HitungEWS(t.Context())

		require.NoError(t, err)
		assert.Equal(t, map[string]int{"KRITIS": 2, "SIAGA": 1}, got)
	})
}

func TestHitungIntervensi_SplitsActiveAndFinishedByStageSuffix(t *testing.T) {
	denganTabelSementara(t, "eksekutif_repo_intervensi", func(repo *Repository, tx *gorm.DB) {
		require.NoError(t, tx.Exec(`INSERT INTO intervensi VALUES
			('TEKANAN', 't-ews'), ('TEKANAN', 't-logistik'), ('TEKANAN', 't-selesai'),
			('SURPLUS', 's-selesai'), ('NORMAL', 'n-pantau')`).Error)

		got, err := repo.HitungIntervensi(t.Context())

		require.NoError(t, err)
		bySiklus := map[string][2]int{}
		for _, h := range got {
			bySiklus[h.Siklus] = [2]int{h.Aktif, h.Selesai}
		}
		assert.Equal(t, map[string][2]int{"tekanan": {2, 1}, "surplus": {0, 1}, "normal": {1, 0}}, bySiklus)
	})
}
