package schematest

import (
	"os"
	"strings"
	"testing"

	"github.com/garda-inflasi/api/internal/auth/usecase"
	"github.com/garda-inflasi/api/internal/platform/pgtest"
	"github.com/garda-inflasi/api/pkg/crypto"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

const (
	rahasiaLama = "JBSWY3DPEHPK3PXP"
	// Versi terakhir sebelum migrasi MFA 01-C (000063 kode cadangan, 64 enkripsi rahasia).
	versiSebelumMFA = 62
)

func kolomAda(t *testing.T, db *gorm.DB, kolom string) bool {
	t.Helper()
	var n int64
	require.NoError(t, db.Raw("SELECT count(*) FROM information_schema.columns WHERE table_name = 'pengguna' AND column_name = ?", kolom).Scan(&n).Error)
	return n == 1
}

// Migrasi 01-C mengenkripsi mfa_secret lama, memindahkan kode cadangan polos menjadi hash, dan dapat
// dibalik sejauh yang mungkin: rahasia kembali polos, kode cadangan (hash) tidak dapat dipulihkan.
func TestMigrations_MFADataIsEncryptedHashedAndReversible(t *testing.T) {
	db := pg.NewDatabase(t, "migrate_mfa_data")
	migrator := pgtest.Migrator(t, db)
	require.NoError(t, migrator.UpTo(t.Context(), versiSebelumMFA))
	require.NoError(t, db.Exec(`
		INSERT INTO pengguna (id, nama, email, peran, lapis, lembaga, mfa_aktif, mfa_secret, mfa_backup_codes)
		VALUES ('u-mfa', 'MFA', 'mfa@x.id', 'OPS_KOORDINATOR', 'OPERASIONAL', 'X', true, ?, '["BACKUP01", "abc-def"]'::jsonb),
		       ('u-biasa', 'Biasa', 'biasa@x.id', 'OPS_MAHASISWA', 'OPERASIONAL', 'X', false, NULL, '[]'::jsonb)`, rahasiaLama).Error)

	require.NoError(t, migrator.Up(t.Context()))

	t.Run("rahasia lama tersegel dan dapat dibuka dengan kunci", func(t *testing.T) {
		var tersimpan string
		require.NoError(t, db.Raw("SELECT mfa_secret FROM pengguna WHERE id = 'u-mfa'").Scan(&tersimpan).Error)
		assert.True(t, strings.HasPrefix(tersimpan, "v1:"), tersimpan)
		assert.NotContains(t, tersimpan, rahasiaLama)
		ring, err := crypto.KeyringFromEnv(os.Getenv)
		require.NoError(t, err)
		opened, err := ring.Open(tersimpan)
		require.NoError(t, err)
		assert.Equal(t, rahasiaLama, opened)
	})
	t.Run("akun tanpa rahasia tidak tersentuh", func(t *testing.T) {
		var n int64
		require.NoError(t, db.Raw("SELECT count(*) FROM pengguna WHERE id = 'u-biasa' AND mfa_secret IS NULL").Scan(&n).Error)
		assert.EqualValues(t, 1, n)
	})
	t.Run("kode cadangan polos menjadi hash dengan rumus yang sama dengan aplikasi", func(t *testing.T) {
		var hashes []string
		require.NoError(t, db.Raw("SELECT kode_hash FROM mfa_kode_cadangan WHERE user_id = 'u-mfa' ORDER BY kode_hash").Scan(&hashes).Error)
		assert.ElementsMatch(t, []string{
			usecase.HashKodeCadangan("u-mfa", "BACKUP01"),
			usecase.HashKodeCadangan("u-mfa", "ABCDEF"),
		}, hashes)
		assert.False(t, kolomAda(t, db, "mfa_backup_codes"), "kolom kode polos dibuang")
		assert.True(t, kolomAda(t, db, "mfa_langkah_terakhir"))
	})

	t.Run("down memulihkan rahasia polos dan kolom lama", func(t *testing.T) {
		require.NoError(t, migrator.Down(t.Context()), "down migrasi Go (dekripsi)")
		var polos string
		require.NoError(t, db.Raw("SELECT mfa_secret FROM pengguna WHERE id = 'u-mfa'").Scan(&polos).Error)
		assert.Equal(t, rahasiaLama, polos)

		require.NoError(t, migrator.Down(t.Context()), "down migrasi SQL (kode cadangan)")
		assert.True(t, kolomAda(t, db, "mfa_backup_codes"))
		assert.False(t, kolomAda(t, db, "mfa_langkah_terakhir"))
		var tabel int64
		require.NoError(t, db.Raw("SELECT count(*) FROM information_schema.tables WHERE table_name = 'mfa_kode_cadangan'").Scan(&tabel).Error)
		assert.Zero(t, tabel)
	})

	t.Run("up ulang menyegel kembali", func(t *testing.T) {
		require.NoError(t, migrator.Up(t.Context()))
		var tersimpan string
		require.NoError(t, db.Raw("SELECT mfa_secret FROM pengguna WHERE id = 'u-mfa'").Scan(&tersimpan).Error)
		assert.True(t, strings.HasPrefix(tersimpan, "v1:"))
	})
}

func TestMigrations_MFASecretMigrationNeedsNoKeyWhenNothingToEncrypt(t *testing.T) {
	t.Setenv("APP_ENV", "production")
	t.Setenv("AES_KEY", "")
	db := pg.NewDatabase(t, "migrate_mfa_tanpa_kunci")

	require.NoError(t, pgtest.Migrator(t, db).Up(t.Context()), "tanpa rahasia MFA, kunci tidak dibaca")
}

func TestMigrations_MFASecretMigrationFailsLoudlyWithoutKeyInProduction(t *testing.T) {
	db := pg.NewDatabase(t, "migrate_mfa_produksi_tanpa_kunci")
	migrator := pgtest.Migrator(t, db)
	require.NoError(t, migrator.UpTo(t.Context(), versiSebelumMFA))
	require.NoError(t, db.Exec(`
		INSERT INTO pengguna (id, nama, email, peran, lapis, lembaga, mfa_aktif, mfa_secret)
		VALUES ('u-mfa', 'MFA', 'mfa@x.id', 'OPS_KOORDINATOR', 'OPERASIONAL', 'X', true, ?)`, rahasiaLama).Error)
	t.Setenv("APP_ENV", "production")
	t.Setenv("AES_KEY", "")

	err := migrator.Up(t.Context())

	require.Error(t, err)
	assert.Contains(t, err.Error(), "AES_KEY")
	var polos string
	require.NoError(t, db.Raw("SELECT mfa_secret FROM pengguna WHERE id = 'u-mfa'").Scan(&polos).Error)
	assert.Equal(t, rahasiaLama, polos, "migrasi gagal utuh, rahasia tidak berubah")
}
