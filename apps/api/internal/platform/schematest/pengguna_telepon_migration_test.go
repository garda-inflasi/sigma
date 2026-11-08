package schematest

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/garda-inflasi/api/internal/platform/migrate"
	"github.com/garda-inflasi/api/internal/platform/pgtest"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Migrasi 000123 (slice 20-A) menambah pengguna.telepon: varchar(20) NULL dengan CHECK format E.164 Indonesia.
// Tes dipatok ke versi 122 -> 123 supaya migrasi lain yang datang sesudahnya tidak mengubah hasilnya.
const (
	versiSebelumTelepon = 122
	versiTelepon        = 123
)

func TestMigrations_PenggunaTeleponKolomCheckHakDownUp(t *testing.T) {
	const namaDB = "migrate_pengguna_telepon"
	db := pg.NewDatabase(t, namaDB)
	migrator := pgtest.Migrator(t, db)
	require.NoError(t, migrator.UpTo(t.Context(), versiSebelumTelepon))
	exec(t, db, `INSERT INTO pengguna (id, nama, email, peran, lapis, lembaga) VALUES
		(garda_uuid_stabil('pengguna', 'lama'), 'Lama', 'lama@x.id', 'OPS_MAHASISWA', 'OPERASIONAL', 'FMIPA')`)
	assert.Zero(t, scanCount(t, db, `SELECT count(*) FROM information_schema.columns WHERE table_name = 'pengguna' AND column_name = 'telepon'`),
		"kontrol: sebelum 000123 kolom belum ada")

	t.Run("up: varchar(20) boleh NULL, baris lama NULL, ada komentar", func(t *testing.T) {
		require.NoError(t, migrator.Apply(t.Context(), versiTelepon, true))

		assert.Equal(t, "character varying", scanString(t, db, `SELECT data_type FROM information_schema.columns WHERE table_name = 'pengguna' AND column_name = 'telepon'`))
		assert.EqualValues(t, 20, scanCount(t, db, `SELECT character_maximum_length FROM information_schema.columns WHERE table_name = 'pengguna' AND column_name = 'telepon'`))
		assert.Equal(t, "YES", scanString(t, db, `SELECT is_nullable FROM information_schema.columns WHERE table_name = 'pengguna' AND column_name = 'telepon'`))
		assert.EqualValues(t, 1, scanCount(t, db, `SELECT count(*) FROM pengguna WHERE nama = 'Lama' AND telepon IS NULL`), "pengguna lama belum punya nomor")
		assert.NotEmpty(t, scanString(t, db, `SELECT coalesce(col_description('pengguna'::regclass, attnum), '') FROM pg_attribute WHERE attrelid = 'pengguna'::regclass AND attname = 'telepon'`))
	})

	t.Run("Up aman diulang (IF NOT EXISTS)", func(t *testing.T) {
		exec(t, db, `UPDATE pengguna SET telepon = '+6281234567890' WHERE nama = 'Lama'`)
		berkas, err := os.ReadFile(filepath.Join(pgtest.MigrationsDir(), "000123_20a_pengguna_telepon.sql"))
		require.NoError(t, err)
		up, _, _ := strings.Cut(string(berkas), "-- +goose Down")

		sqlDB, err := db.DB()
		require.NoError(t, err)
		// Seperti goose: tanpa argumen, jadi beberapa pernyataan boleh dalam satu Exec. Bagian Up dijalankan lagi
		// pada skema yang sudah punya kolom.
		_, err = sqlDB.ExecContext(t.Context(), up)
		require.NoError(t, err)

		assert.Equal(t, "+6281234567890", scanString(t, db, `SELECT telepon FROM pengguna WHERE nama = 'Lama'`), "nomor tidak terhapus")
		exec(t, db, `UPDATE pengguna SET telepon = NULL WHERE nama = 'Lama'`)
	})

	t.Run("CHECK menerima E.164 Indonesia dan NULL", func(t *testing.T) {
		for _, sah := range []string{"+6281234567890", "+6212345678", "+621234567890123", "+6281200000001"} {
			exec(t, db, `UPDATE pengguna SET telepon = ? WHERE nama = 'Lama'`, sah)
		}
		exec(t, db, `UPDATE pengguna SET telepon = NULL WHERE nama = 'Lama'`)
	})

	t.Run("CHECK menolak string kosong dan bentuk selain +62 E.164", func(t *testing.T) {
		for _, salah := range []string{
			"", " ", "081234567890", "6281234567890", "+620812345678", "+6212345", "+6212345678901234",
			"+1415555267", "+62abcdefghij", "+62 81234567890", "+6281234567890\n", "+62081234567890",
		} {
			err := db.Exec(`UPDATE pengguna SET telepon = ? WHERE nama = 'Lama'`, salah).Error
			require.Error(t, err, "%q harus ditolak", salah)
			assert.Contains(t, err.Error(), "chk_pengguna_telepon", "%q", salah)
		}
		assert.EqualValues(t, 1, scanCount(t, db, `SELECT count(*) FROM pengguna WHERE telepon IS NULL`), "tidak ada yang tertulis")
	})

	t.Run("hak garda_aplikasi: kolom baru terbaca dan terubah tanpa GRANT tambahan, DDL tetap ditolak", func(t *testing.T) {
		const nama, sandi = "garda_app_telepon", "sandi-uji-cukup-panjang-3"
		sqlDB, err := db.DB()
		require.NoError(t, err)
		require.NoError(t, migrate.SiapkanPenggunaAplikasi(t.Context(), sqlDB, nama, sandi))
		app := pg.BukaSebagai(t, namaDB, nama, sandi)

		require.NoError(t, app.Exec(`UPDATE pengguna SET telepon = '+6281200000001' WHERE nama = 'Lama'`).Error)
		assert.Equal(t, "+6281200000001", scanString(t, app, `SELECT telepon FROM pengguna WHERE nama = 'Lama'`))
		require.NoError(t, app.Exec(`UPDATE pengguna SET telepon = NULL WHERE nama = 'Lama'`).Error, "hapus nomor = NULL")
		err = app.Exec(`ALTER TABLE pengguna DROP COLUMN telepon`).Error
		require.Error(t, err)
		assert.Contains(t, err.Error(), "42501", "peran aplikasi tidak boleh DDL")
	})

	t.Run("down membuang kolom dan CHECK tanpa menyentuh baris; up memasang lagi", func(t *testing.T) {
		exec(t, db, `UPDATE pengguna SET telepon = '+6281234567890' WHERE nama = 'Lama'`)

		require.NoError(t, migrator.Apply(t.Context(), versiTelepon, false))

		assert.Zero(t, scanCount(t, db, `SELECT count(*) FROM information_schema.columns WHERE table_name = 'pengguna' AND column_name = 'telepon'`))
		assert.Zero(t, scanCount(t, db, `SELECT count(*) FROM pg_constraint WHERE conname = 'chk_pengguna_telepon'`), "CHECK ikut terbuang bersama kolomnya")
		assert.EqualValues(t, 1, scanCount(t, db, `SELECT count(*) FROM pengguna WHERE nama = 'Lama'`), "baris pengguna utuh")

		require.NoError(t, migrator.Apply(t.Context(), versiTelepon, true))
		assert.EqualValues(t, 1, scanCount(t, db, `SELECT count(*) FROM pengguna WHERE nama = 'Lama' AND telepon IS NULL`), "nomor yang terbuang tidak kembali")
		assert.EqualValues(t, 1, scanCount(t, db, `SELECT count(*) FROM pg_constraint WHERE conname = 'chk_pengguna_telepon'`))
	})
}
