package seeder

import (
	"bytes"
	"fmt"
	"os"
	"strings"
	"testing"

	"github.com/garda-inflasi/api/internal/auth/usecase"
	masterseeder "github.com/garda-inflasi/api/internal/master/seeder"
	"github.com/garda-inflasi/api/internal/platform/pgtest"
	"github.com/garda-inflasi/api/internal/seedkit"
	"github.com/garda-inflasi/api/pkg/crypto"
	"github.com/garda-inflasi/api/pkg/idx"
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

// Telepon akun demo: fiktif, deterministik (+62 812-0000-xxxx, xxxx = urutan di daftar), unik, dan sudah dalam
// bentuk normal E.164 sehingga lolos aturan yang sama dengan PATCH /v1/auth/me.
func TestAkunDemo_TeleponFiktifDeterministikDanValid(t *testing.T) {
	akun := BuildDemoAkun()

	telepon := map[string]bool{}
	for i, a := range akun {
		require.NotNil(t, a.Telepon, "akun demo %s punya telepon", a.Email)
		assert.Equal(t, fmt.Sprintf("+628120000%04d", i+1), *a.Telepon, a.Email)
		normal, err := usecase.NormalisasiTelepon(*a.Telepon)
		require.NoError(t, err, *a.Telepon)
		assert.Equal(t, *a.Telepon, normal, "telepon seed sudah dalam bentuk normal")
		assert.False(t, telepon[*a.Telepon], "telepon ganda: %s", *a.Telepon)
		telepon[*a.Telepon] = true
	}
	assert.Len(t, telepon, 25)
	assert.Equal(t, "+6281200000001", *akun[0].Telepon, "superadmin")
	assert.Equal(t, "+6281200000025", *akun[24].Telepon, "user_mfa")
}

// ID akun demo adalah UUID stabil dari slug lamanya (seedkit.IDPengguna): sama dengan hasil migrasi 000110 atas
// basis data lama, dan dengan yang dirujuk seeder modul lain. SlugDemo (dasar seedkit.SlugPengguna) harus memuat
// persis slug akun demo ditambah admin awal.
func TestAkunDemo_IDAdalahUUIDStabilDariSlug(t *testing.T) {
	slug := []string{adminSlug}
	for _, a := range akunDemoSeeds {
		slug = append(slug, a.ID)
	}
	assert.ElementsMatch(t, slug, seedkit.SlugDemo, "seedkit.SlugDemo harus sama dengan akun yang dibuat seeder")

	for i, a := range BuildDemoAkun() {
		assert.True(t, idx.ValidUUID(a.ID), a.Email)
		assert.Equal(t, seedkit.IDPengguna(akunDemoSeeds[i].ID), a.ID, akunDemoSeeds[i].ID)
	}
	assert.Equal(t, seedkit.IDPengguna("user_mfa"), DemoMFAAkunID)
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

func rowCountWhere(t *testing.T, db *gorm.DB, table, where string) int64 {
	t.Helper()
	var n int64
	require.NoError(t, db.Table(table).Where(where).Count(&n).Error)
	return n
}

func TestSeedDemo_IsIdempotentAndLoginable(t *testing.T) {
	db := pg.NewDatabase(t, "auth_seed_demo")
	pgtest.MigrateUp(t, db)
	seedWilayahDemo(t, db)

	first, err := SeedDemo(t.Context(), db)
	require.NoError(t, err)
	require.NoError(t, db.Exec("UPDATE pengguna SET nama = 'diubah', telepon = '+6285712345678' WHERE id = garda_uuid_stabil('pengguna', 'superadmin')").Error)
	require.NoError(t, db.Exec("UPDATE pengguna SET telepon = NULL WHERE id = garda_uuid_stabil('pengguna', 'user_mfa')").Error)
	_, err = SeedDemo(t.Context(), db)
	require.NoError(t, err)

	assert.Equal(t, 25, first.Akun)
	assert.EqualValues(t, 25, rowCount(t, db, "pengguna"))

	var telepon []string
	require.NoError(t, db.Raw("SELECT telepon FROM pengguna WHERE telepon IS NOT NULL ORDER BY telepon").Scan(&telepon).Error)
	assert.Len(t, telepon, 25, "seed ulang tidak menambah baris dan mengembalikan telepon yang diubah atau dihapus")
	assert.Equal(t, "+6281200000001", telepon[0])
	assert.Equal(t, "+6281200000025", telepon[24])
	var superadmin string
	require.NoError(t, db.Raw("SELECT telepon FROM pengguna WHERE id = garda_uuid_stabil('pengguna', 'superadmin')").Scan(&superadmin).Error)
	assert.Equal(t, "+6281200000001", superadmin)

	var nama, hash string
	require.NoError(t, db.Raw("SELECT nama, password_hash FROM pengguna WHERE id = garda_uuid_stabil('pengguna', 'superadmin')").Row().Scan(&nama, &hash))
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
		assert.EqualValues(t, 0, rowCountWhere(t, db, "pengguna", "telepon IS NOT NULL"), "lapis admin tidak mengisi telepon")
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
			VALUES (garda_uuid_stabil('pengguna', 'ada'), 'Ada', 'ada@gardainflasi.id', 'ADMIN_SISTEM', 'SISTEM', 'X')`).Error)

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
	require.NoError(t, db.Raw("SELECT mfa_secret FROM pengguna WHERE id = garda_uuid_stabil('pengguna', 'user_mfa')").Scan(&secret).Error)
	assert.True(t, strings.HasPrefix(secret, "v1:"), secret)
	assert.NotContains(t, secret, DemoMFASecret)
	ring, err := crypto.KeyringFromEnv(os.Getenv)
	require.NoError(t, err)
	opened, err := ring.Open(secret)
	require.NoError(t, err)
	assert.Equal(t, DemoMFASecret, opened)

	var hashes []string
	require.NoError(t, db.Raw("SELECT kode_hash FROM mfa_kode_cadangan WHERE user_id = garda_uuid_stabil('pengguna', 'user_mfa') ORDER BY kode_hash").Scan(&hashes).Error)
	assert.Len(t, hashes, len(DemoKodeCadangan), "seed ulang tidak menggandakan kode")
	for _, h := range hashes {
		assert.Len(t, h, 64)
	}

	var akunLain int64
	require.NoError(t, db.Raw("SELECT count(*) FROM pengguna WHERE id <> garda_uuid_stabil('pengguna', 'user_mfa') AND (mfa_secret IS NOT NULL OR mfa_aktif)").Scan(&akunLain).Error)
	assert.Zero(t, akunLain, "hanya satu akun demo ber-MFA")
}
