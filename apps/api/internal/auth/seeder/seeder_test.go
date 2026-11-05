package seeder

import (
	"bytes"
	"os"
	"strings"
	"testing"

	masterseeder "github.com/garda-inflasi/api/internal/master/seeder"
	"github.com/garda-inflasi/api/internal/platform/pgtest"
	"github.com/garda-inflasi/api/pkg/crypto"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

var pg *pgtest.Server

func TestMain(m *testing.M) {
	os.Exit(pgtest.Main(m, &pg))
}

func TestDemoPasswordHashMatchesDemoPassword(t *testing.T) {
	assert.True(t, crypto.VerifyPassword(DemoPassword, demoPasswordHash))
}

func TestAkunDemo_Shape(t *testing.T) {
	akun := BuildDemoAkun()

	require.Len(t, akun, 25)
	roles, emails, ids := map[string]int{}, map[string]bool{}, map[string]bool{}
	var nonaktif, mfa int
	for _, a := range akun {
		roles[a.Peran]++
		assert.True(t, strings.HasSuffix(a.Email, "@gardainflasi.id"), "domain demo: %s", a.Email)
		assert.False(t, emails[strings.ToLower(a.Email)], "email ganda: %s", a.Email)
		assert.False(t, ids[a.ID], "id ganda: %s", a.ID)
		emails[strings.ToLower(a.Email)], ids[a.ID] = true, true
		assert.False(t, a.WajibGantiPassword, "akun demo tidak dipaksa ganti password: %s", a.ID)
		if a.Status == "NONAKTIF" {
			nonaktif++
		}
		if a.MfaAktif {
			mfa++
		}
	}
	assert.Len(t, roles, 14, "semua 14 peran terwakili")
	assert.Equal(t, 10, roles["OPS_MAHASISWA"]-nonaktif, "10 mahasiswa aktif")
	assert.Equal(t, 1, nonaktif)
	assert.Equal(t, 1, mfa)
}

// seedWilayahDemo mengisi master kabupaten yang dirujuk akun demo.
func seedWilayahDemo(t *testing.T, db *gorm.DB) {
	t.Helper()
	_, err := masterseeder.SeedReference(t.Context(), db)
	require.NoError(t, err)
}

func rowCount(t *testing.T, db *gorm.DB, table string) int64 {
	t.Helper()
	var n int64
	require.NoError(t, db.Table(table).Count(&n).Error)
	return n
}

func TestSeedDemo_IsIdempotentAndLoginable(t *testing.T) {
	db := pg.NewDatabase(t, "auth_seed_demo")
	pgtest.MigrateUp(t, db)
	seedWilayahDemo(t, db)

	first, err := SeedDemo(t.Context(), db)
	require.NoError(t, err)
	require.NoError(t, db.Exec("UPDATE pengguna SET nama = 'diubah' WHERE id = 'superadmin'").Error)
	_, err = SeedDemo(t.Context(), db)
	require.NoError(t, err)

	assert.Equal(t, 25, first.Akun)
	assert.EqualValues(t, 25, rowCount(t, db, "pengguna"))

	var nama, hash string
	require.NoError(t, db.Raw("SELECT nama, password_hash FROM pengguna WHERE id = 'superadmin'").Row().Scan(&nama, &hash))
	assert.Equal(t, "Super Admin", nama, "seed demo mengembalikan akun ke keadaan awal")
	assert.True(t, crypto.VerifyPassword(DemoPassword, hash))
}

func TestSeedAdmin(t *testing.T) {
	newDB := func(t *testing.T, name string) *gorm.DB {
		db := pg.NewDatabase(t, name)
		pgtest.MigrateUp(t, db)
		return db
	}

	t.Run("membuat admin dengan password dari env, wajib ganti dan wajib MFA", func(t *testing.T) {
		db := newDB(t, "auth_seed_admin_env")

		sum, err := SeedAdmin(t.Context(), db, AdminConfig{Email: " Admin@GardaInflasi.id ", Password: "Sementara1!"})

		require.NoError(t, err)
		assert.Equal(t, 1, sum.Dibuat)
		var row struct {
			Email              string
			Peran              string
			WajibGantiPassword bool
			MfaWajib           bool
			PasswordHash       string
		}
		require.NoError(t, db.Raw("SELECT email, peran, wajib_ganti_password, mfa_wajib, password_hash FROM pengguna").Scan(&row).Error)
		assert.Equal(t, "admin@gardainflasi.id", row.Email)
		assert.Equal(t, "ADMIN_SISTEM", row.Peran)
		assert.True(t, row.WajibGantiPassword)
		assert.True(t, row.MfaWajib)
		assert.True(t, crypto.VerifyPassword("Sementara1!", row.PasswordHash))
	})

	t.Run("password acak dicetak sekali dan memenuhi kebijakan", func(t *testing.T) {
		db := newDB(t, "auth_seed_admin_random")
		var out bytes.Buffer

		_, err := SeedAdmin(t.Context(), db, AdminConfig{Email: "admin@gardainflasi.id", Output: &out})
		require.NoError(t, err)

		printed := out.String()
		require.Contains(t, printed, "kata sandi sementara")
		password := strings.TrimSpace(printed[strings.LastIndex(printed, ": ")+2:])
		var hash string
		require.NoError(t, db.Raw("SELECT password_hash FROM pengguna").Row().Scan(&hash))
		assert.True(t, crypto.VerifyPassword(password, hash))

		out.Reset()
		sum, err := SeedAdmin(t.Context(), db, AdminConfig{Email: "admin@gardainflasi.id", Output: &out})
		require.NoError(t, err)
		assert.Zero(t, sum.Dibuat)
		assert.Empty(t, out.String(), "kata sandi tidak dicetak lagi")
	})

	t.Run("tidak membuat admin bila sudah ada", func(t *testing.T) {
		db := newDB(t, "auth_seed_admin_exists")
		require.NoError(t, db.Exec(`
			INSERT INTO pengguna (id, nama, email, peran, lapis, lembaga)
			VALUES ('ada', 'Ada', 'ada@gardainflasi.id', 'ADMIN_SISTEM', 'SISTEM', 'X')`).Error)

		sum, err := SeedAdmin(t.Context(), db, AdminConfig{})

		require.NoError(t, err, "env kosong tidak masalah bila admin sudah ada")
		assert.Zero(t, sum.Dibuat)
		assert.EqualValues(t, 1, rowCount(t, db, "pengguna"))
	})

	t.Run("email wajib bila belum ada admin", func(t *testing.T) {
		db := newDB(t, "auth_seed_admin_noemail")

		_, err := SeedAdmin(t.Context(), db, AdminConfig{})

		assert.ErrorIs(t, err, ErrAdminEmailRequired)
	})
}

// Setelah reference + admin tidak boleh ada akun ber-password default (19 §6).
func TestAdminSeed_LeavesNoAccountWithDefaultPassword(t *testing.T) {
	db := pg.NewDatabase(t, "auth_seed_no_default")
	pgtest.MigrateUp(t, db)

	_, err := SeedAdmin(t.Context(), db, AdminConfig{Email: "admin@gardainflasi.id"})
	require.NoError(t, err)

	var hashes []string
	require.NoError(t, db.Raw("SELECT password_hash FROM pengguna").Scan(&hashes).Error)
	require.NotEmpty(t, hashes)
	for _, h := range hashes {
		assert.False(t, crypto.VerifyPassword(DemoPassword, h))
	}
}

func TestSeedDemo_MFAAccountHasSealedSecretAndHashedBackupCodes(t *testing.T) {
	db := pg.NewDatabase(t, "auth_seed_demo_mfa")
	pgtest.MigrateUp(t, db)
	seedWilayahDemo(t, db)

	_, err := SeedDemo(t.Context(), db)
	require.NoError(t, err)
	_, err = SeedDemo(t.Context(), db)
	require.NoError(t, err)

	var secret string
	require.NoError(t, db.Raw("SELECT mfa_secret FROM pengguna WHERE id = 'user_mfa'").Scan(&secret).Error)
	assert.True(t, strings.HasPrefix(secret, "v1:"), secret)
	assert.NotContains(t, secret, DemoMFASecret)
	ring, err := crypto.KeyringFromEnv(os.Getenv)
	require.NoError(t, err)
	opened, err := ring.Open(secret)
	require.NoError(t, err)
	assert.Equal(t, DemoMFASecret, opened)

	var hashes []string
	require.NoError(t, db.Raw("SELECT kode_hash FROM mfa_kode_cadangan WHERE user_id = 'user_mfa' ORDER BY kode_hash").Scan(&hashes).Error)
	assert.Len(t, hashes, len(DemoKodeCadangan), "seed ulang tidak menggandakan kode")
	for _, h := range hashes {
		assert.Len(t, h, 64)
	}

	var akunLain int64
	require.NoError(t, db.Raw("SELECT count(*) FROM pengguna WHERE id <> 'user_mfa' AND (mfa_secret IS NOT NULL OR mfa_aktif)").Scan(&akunLain).Error)
	assert.Zero(t, akunLain, "hanya satu akun demo ber-MFA")
}
