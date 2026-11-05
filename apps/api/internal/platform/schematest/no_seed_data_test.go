package schematest

import (
	"testing"

	"github.com/garda-inflasi/api/internal/platform/pgtest"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Gerbang produksi (F1): migrasi hanya membuat skema. Tidak boleh ada akun demo,
// data referensi, atau penugasan izin yang ikut terpasang di basis data baru.
func TestMigrationsLeaveEveryTableEmpty(t *testing.T) {
	db := pg.NewDatabase(t, "migrate_no_seed_data")
	pgtest.MigrateUp(t, db)

	var tables []string
	require.NoError(t, db.Raw(`
		SELECT tablename FROM pg_tables
		 WHERE schemaname = 'public' AND tablename <> 'goose_db_version'
		 ORDER BY tablename`).Scan(&tables).Error)
	require.NotEmpty(t, tables)

	for _, table := range tables {
		var n int64
		require.NoError(t, db.Table(table).Count(&n).Error)
		assert.Zero(t, n, "tabel %s harus kosong setelah migrasi; data awal ada di seeder (cmd/seed)", table)
	}
}
