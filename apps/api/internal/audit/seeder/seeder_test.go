package seeder

import (
	"os"
	"testing"
	"time"

	authseeder "github.com/garda-inflasi/api/internal/auth/seeder"
	"github.com/garda-inflasi/api/internal/platform/pgtest"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

var (
	pg      *pgtest.Server
	seedNow = time.Date(2026, 10, 16, 9, 0, 0, 0, time.UTC)
)

func TestMain(m *testing.M) {
	os.Exit(pgtest.Main(m, &pg))
}

func TestBuildEntri_IsDeterministicAndMarkedDemo(t *testing.T) {
	first, second := BuildEntri(seedNow), BuildEntri(seedNow)

	assert.Equal(t, first, second)
	require.Len(t, first, jumlahEntri)
	ids := map[string]bool{}
	for _, row := range first {
		assert.False(t, ids[row.ID], "id ganda: %s", row.ID)
		ids[row.ID] = true
		assert.Contains(t, row.ID, idPrefix)
		assert.JSONEq(t, `{"demo":true}`, *row.Metadata)
		assert.NotNil(t, row.AktorID)
		assert.True(t, row.Waktu.Before(seedNow))
	}
}

func TestSeedDemo_IsIdempotentAndUsesDemoAccounts(t *testing.T) {
	db := pg.NewDatabase(t, "audit_seed_demo")
	pgtest.MigrateUp(t, db)
	require.NoError(t, db.Exec("DELETE FROM pengguna").Error) // buang akun migrasi lama (sampai F1)
	require.NoError(t, db.Exec(`INSERT INTO provinsi (kode, nama) VALUES ('11', 'ACEH') ON CONFLICT DO NOTHING`).Error)
	for _, akun := range authseeder.BuildDemoAkun() {
		if akun.KabupatenKode != nil {
			require.NoError(t, db.Exec(`INSERT INTO kabupaten (kode, nama, jenis, ibukota, lat, lng, provinsi_kode)
				VALUES (?, ?, 'Kabupaten', 'x', 1, 1, '11') ON CONFLICT DO NOTHING`, *akun.KabupatenKode, "K"+*akun.KabupatenKode).Error)
		}
	}
	_, err := authseeder.SeedDemo(t.Context(), db)
	require.NoError(t, err)

	_, err = SeedDemo(t.Context(), db, seedNow)
	require.NoError(t, err)
	_, err = SeedDemo(t.Context(), db, seedNow.Add(time.Hour))
	require.NoError(t, err)

	var n int64
	require.NoError(t, db.Table("audit_log").Count(&n).Error)
	assert.EqualValues(t, jumlahEntri, n)
}
