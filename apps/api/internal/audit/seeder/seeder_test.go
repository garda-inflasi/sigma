package seeder

import (
	"os"
	"testing"
	"time"

	authseeder "github.com/garda-inflasi/api/internal/auth/seeder"
	masterseeder "github.com/garda-inflasi/api/internal/master/seeder"
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
	kode := map[string]bool{}
	for _, row := range first {
		require.NotNil(t, row.Kode, "kode adalah kunci idempotensi; ID dibuat basis data")
		assert.Zero(t, row.ID)
		assert.False(t, kode[*row.Kode], "kode ganda: %s", *row.Kode)
		kode[*row.Kode] = true
		assert.Contains(t, *row.Kode, kodePrefix)
		assert.JSONEq(t, `{"demo":true}`, *row.Metadata)
		assert.NotNil(t, row.AktorID)
		assert.True(t, row.Waktu.Before(seedNow))
	}
}

func TestSeedDemo_IsIdempotentAndUsesDemoAccounts(t *testing.T) {
	db := pg.NewDatabase(t, "audit_seed_demo")
	pgtest.MigrateUp(t, db)
	_, err := masterseeder.SeedReference(t.Context(), db)
	require.NoError(t, err)
	_, err = authseeder.SeedDemo(t.Context(), db)
	require.NoError(t, err)

	_, err = SeedDemo(t.Context(), db, seedNow)
	require.NoError(t, err)
	_, err = SeedDemo(t.Context(), db, seedNow.Add(time.Hour))
	require.NoError(t, err)

	var n int64
	require.NoError(t, db.Table("audit_log").Count(&n).Error)
	assert.EqualValues(t, jumlahEntri, n)
	require.NoError(t, db.Raw(`SELECT count(DISTINCT kode) FROM audit_log WHERE kode LIKE 'demo-audit-%'`).Scan(&n).Error)
	assert.EqualValues(t, jumlahEntri, n, "kode demo unik; menjalankan ulang tidak menambah baris")
}

func TestSeedDemo_BarisBerIDIdentityDanDataMigrasiTidakDigandakan(t *testing.T) {
	db := pg.NewDatabase(t, "audit_seed_demo_migrasi")
	pgtest.MigrateUp(t, db)
	_, err := masterseeder.SeedReference(t.Context(), db)
	require.NoError(t, err)
	_, err = authseeder.SeedDemo(t.Context(), db)
	require.NoError(t, err)

	// Baris hasil migrasi data lama: kode berisi ID lama yang sama dengan kunci seeder.
	require.NoError(t, db.Exec(`INSERT INTO audit_log (kode, aksi, entitas, entitas_id, hasil) VALUES ('demo-audit-0001', 'LOGIN', 'pengguna', 'x', 'BERHASIL')`).Error)

	_, err = SeedDemo(t.Context(), db, seedNow)
	require.NoError(t, err)

	var n int64
	require.NoError(t, db.Raw(`SELECT count(*) FROM audit_log WHERE kode = 'demo-audit-0001'`).Scan(&n).Error)
	assert.EqualValues(t, 1, n, "baris yang sudah ada tidak digandakan")
	require.NoError(t, db.Table("audit_log").Count(&n).Error)
	assert.EqualValues(t, jumlahEntri, n)
	require.NoError(t, db.Raw(`SELECT count(*) FROM audit_log WHERE id IS NOT NULL`).Scan(&n).Error)
	assert.EqualValues(t, jumlahEntri, n)
}
