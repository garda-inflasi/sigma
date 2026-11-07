package seeder

import (
	"os"
	"testing"
	"time"

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

func migrated(t *testing.T, name string) *gorm.DB {
	t.Helper()
	db := pg.NewDatabase(t, name)
	pgtest.MigrateUp(t, db)
	return db
}

func count(t *testing.T, db *gorm.DB, table string) int64 {
	t.Helper()
	var n int64
	require.NoError(t, db.Table(table).Count(&n).Error)
	return n
}

func TestSeedReference_WritesOnlyFacilitiesSoItIsSafeInProduction(t *testing.T) {
	db := migrated(t, "cs_seed_reference")
	_, err := masterseeder.SeedReference(t.Context(), db)
	require.NoError(t, err)

	sum, err := SeedReference(t.Context(), db)

	require.NoError(t, err)
	assert.Equal(t, ReferenceSummary{Fasilitas: 5}, sum)
	assert.EqualValues(t, 5, count(t, db, "cold_storage"))
	assert.Zero(t, count(t, db, "stok_cold_storage"), "lot adalah data demo, bukan referensi")
}

func TestSeedReference_WithoutMasterExplainsWhatToRunFirst(t *testing.T) {
	db := migrated(t, "cs_seed_without_master")

	_, err := SeedReference(t.Context(), db)

	require.Error(t, err)
	assert.Contains(t, err.Error(), "master.reference")
	assert.Zero(t, count(t, db, "cold_storage"), "tidak ada fasilitas boleh tertulis sebagian")
}

func TestSeedDemo_RerunMovesDatesWithoutDuplicatingLots(t *testing.T) {
	db := migrated(t, "cs_seed_demo_rerun")
	_, err := masterseeder.SeedReference(t.Context(), db)
	require.NoError(t, err)
	_, err = SeedReference(t.Context(), db)
	require.NoError(t, err)

	first, err := SeedDemo(t.Context(), db, seedNow)
	require.NoError(t, err)
	later := seedNow.Add(10 * 24 * time.Hour)
	second, err := SeedDemo(t.Context(), db, later)
	require.NoError(t, err)

	assert.Equal(t, DemoSummary{Stok: 23, Telemetri: 4 * titikTelemetri, Transaksi: 23}, first)
	assert.Equal(t, first, second)
	assert.EqualValues(t, 23, count(t, db, "stok_cold_storage"))

	var masuk, kedaluwarsa time.Time
	require.NoError(t, db.Raw(`SELECT masuk_pada, kedaluwarsa_pada FROM stok_cold_storage WHERE kode = 'STK-CS02-02'`).Row().Scan(&masuk, &kedaluwarsa))
	assert.True(t, masuk.Equal(later.Add(-106*24*time.Hour)), "masuk_pada mengikuti now terakhir, dapat %s", masuk)
	assert.True(t, kedaluwarsa.Equal(masuk.Add(120*24*time.Hour)), "masa simpan 120 hari dari masuk")
}

func TestSeedDemo_WithoutFacilitiesFailsOnForeignKey(t *testing.T) {
	db := migrated(t, "cs_seed_demo_no_reference")

	_, err := SeedDemo(t.Context(), db, seedNow)

	require.Error(t, err)
	assert.Zero(t, count(t, db, "stok_cold_storage"))
}

// Menjalankan seeder dua kali tidak menambah baris, dan ID BIGINT lot serta transaksi tidak berubah
// (kunci idempotensi: UUID stabil untuk fasilitas, kode untuk lot dan transaksi).
func TestSeed_RerunKeepsRowsAndIDs(t *testing.T) {
	db := migrated(t, "cs_seed_rerun_ids")
	_, err := masterseeder.SeedReference(t.Context(), db)
	require.NoError(t, err)

	jalankan := func() {
		_, err := SeedReference(t.Context(), db)
		require.NoError(t, err)
		_, err = SeedDemo(t.Context(), db, seedNow)
		require.NoError(t, err)
	}
	jalankan()
	var idsSebelum string
	require.NoError(t, db.Raw(`SELECT string_agg(id::text, ',' ORDER BY id) FROM (
		SELECT id FROM stok_cold_storage UNION ALL SELECT id FROM transaksi_stok) x`).Scan(&idsSebelum).Error)
	jalankan()
	var idsSesudah string
	require.NoError(t, db.Raw(`SELECT string_agg(id::text, ',' ORDER BY id) FROM (
		SELECT id FROM stok_cold_storage UNION ALL SELECT id FROM transaksi_stok) x`).Scan(&idsSesudah).Error)

	assert.Equal(t, idsSebelum, idsSesudah, "ID tidak berubah dan tidak ada baris baru")
	assert.EqualValues(t, 5, count(t, db, "cold_storage"))
	assert.EqualValues(t, 23, count(t, db, "stok_cold_storage"))
	assert.EqualValues(t, 23, count(t, db, "transaksi_stok"))
	assert.EqualValues(t, 4*titikTelemetri, count(t, db, "telemetri_suhu"))

	var id string
	require.NoError(t, db.Raw(`SELECT id::text FROM cold_storage WHERE kode = 'CS-03'`).Scan(&id).Error)
	assert.Equal(t, FasilitasID("CS-03"), id)
	var rujuk int64
	require.NoError(t, db.Raw(`SELECT COUNT(*) FROM transaksi_stok t JOIN stok_cold_storage s ON s.id = t.lot_id
		WHERE t.kode = 'TRX-DEMO-' || s.kode AND t.fasilitas_tujuan_id = s.cold_storage_id`).Scan(&rujuk).Error)
	assert.EqualValues(t, 23, rujuk, "setiap transaksi demo menunjuk lot dan fasilitas yang benar")
}
