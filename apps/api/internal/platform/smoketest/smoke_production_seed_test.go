package smoketest

import (
	"os"
	"os/exec"
	"testing"

	authseeder "github.com/garda-inflasi/api/internal/auth/seeder"
	"github.com/garda-inflasi/api/internal/platform/pgtest"
	"github.com/garda-inflasi/api/pkg/crypto"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// runSeedWithEnv menjalankan cmd/seed dengan tambahan variabel lingkungan.
func runSeedWithEnv(t *testing.T, dbName string, env []string, tiers ...string) (string, error) {
	t.Helper()
	cmd := exec.Command(seedBinary(t), tiers...)
	cmd.Env = append(os.Environ(), append([]string{"DATABASE_URL=" + pg.DSN(dbName)}, env...)...)
	out, err := cmd.CombinedOutput()
	return string(out), err
}

// Deploy produksi menjalankan `seed reference admin`. Hasilnya: data referensi dan
// satu admin awal wajib ganti kata sandi, tanpa akun ber-password default.
func TestSeed_ProductionReferenceAndAdminLeavesNoDefaultPasswordAccount(t *testing.T) {
	const dbName = "smoke_production_admin"
	db := pg.NewDatabase(t, dbName)
	pgtest.MigrateUp(t, db)
	env := []string{"APP_ENV=production", "SEED_ADMIN_EMAIL=admin@gardainflasi.id", "SEED_ADMIN_PASSWORD="}

	out, err := runSeedWithEnv(t, dbName, env, "reference", "admin")
	require.NoError(t, err, out)
	assert.Contains(t, out, "kata sandi sementara", "kata sandi acak dicetak sekali")

	var accounts []struct {
		Email              string
		PasswordHash       string
		WajibGantiPassword bool
	}
	require.NoError(t, db.Raw(`SELECT email, password_hash, wajib_ganti_password FROM pengguna`).Scan(&accounts).Error)
	require.Len(t, accounts, 1)
	assert.Equal(t, "admin@gardainflasi.id", accounts[0].Email)
	assert.True(t, accounts[0].WajibGantiPassword)
	assert.False(t, crypto.VerifyPassword(authseeder.DemoPassword, accounts[0].PasswordHash))

	counts := rowCounts(t, db)
	assert.NotZero(t, counts["kabupaten"], "data referensi terpasang")

	out, err = runSeedWithEnv(t, dbName, env, "reference", "admin")
	require.NoError(t, err, out)
	assert.NotContains(t, out, "kata sandi sementara", "dijalankan ulang: tidak ada admin baru")
	assert.Equal(t, counts, rowCounts(t, db), "dijalankan ulang tidak menambah baris")
}

func TestSeed_ProductionRefusesDemoAccounts(t *testing.T) {
	const dbName = "smoke_production_demo"
	db := pg.NewDatabase(t, dbName)
	pgtest.MigrateUp(t, db)

	out, err := runSeedWithEnv(t, dbName, []string{"APP_ENV=production"}, "all")

	require.Error(t, err)
	assert.Contains(t, out, "APP_ENV=production")
	var n int64
	require.NoError(t, db.Table("pengguna").Count(&n).Error)
	assert.Zero(t, n, "tidak ada akun demo boleh masuk")
}
