package integration

import (
	"testing"
	"time"

	"github.com/garda-inflasi/api/internal/platform/pgtest"
	"github.com/garda-inflasi/api/pkg/seed"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

// Tes seed SQL prediksi (db/seeds/demo/10_prediksi_harga.sql). Pengganti tes seeder Go lama: lapis produksi tidak
// membawa model atau proyeksi, seed demo idempoten lintas hari, ID BIGINT diberikan basis data, dan proyeksi tertaut
// ke model produksi demo komoditasnya.

// idMenurutKode membaca ID BIGINT baris menurut kodenya.
func idMenurutKode(t *testing.T, db *gorm.DB, tabel string) map[string]int64 {
	t.Helper()
	var rows []struct {
		Kode string
		ID   int64
	}
	require.NoError(t, db.Raw("SELECT kode, id FROM "+tabel).Scan(&rows).Error)
	out := make(map[string]int64, len(rows))
	for _, r := range rows {
		out[r.Kode] = r.ID
	}
	return out
}

func hitungTabel(t *testing.T, db *gorm.DB, tabel string) int64 {
	t.Helper()
	var n int64
	require.NoError(t, db.Table(tabel).Count(&n).Error)
	return n
}

func TestSeed_LapisReferenceTidakMembawaModelDanProyeksiDemo(t *testing.T) {
	db := pg.NewDatabase(t, "prediksi_seed_reference")
	pgtest.MigrateUp(t, db)
	require.NoError(t, seed.SeedReference(t.Context(), db))

	// Lapis reference ikut dijalankan di produksi: model berstatus PRODUKSI bersumber DEMO tidak boleh ada di sana.
	assert.Zero(t, hitungTabel(t, db, "model_registry"), "registri diisi pipeline ML, bukan seed produksi")
	assert.Zero(t, hitungTabel(t, db, "prediksi_harga"))
}

func TestSeed_DemoIdempotenLintasHariDanProyeksiTertautKeModel(t *testing.T) {
	db := pg.NewDatabase(t, "prediksi_seed_demo")
	pgtest.MigrateUp(t, db)
	ctx := t.Context()
	require.NoError(t, seed.SeedReference(ctx, db))

	sebelum := jamDB(t, db).UTC()
	require.NoError(t, seed.SeedFiles(ctx, db, berkasSeedDemo...))
	sesudah := jamDB(t, db).UTC()

	modelAwal, proyeksiAwal := idMenurutKode(t, db, "model_registry"), idMenurutKode(t, db, "prediksi_harga")
	assert.Len(t, modelAwal, 28, "satu model produksi dan satu kandidat untuk 14 komoditas")
	assert.Len(t, proyeksiAwal, 240, "14 komoditas di Banda Aceh + 3 komoditas pilot di 22 kabupaten/kota lain, 3 horizon")
	assert.Contains(t, modelAwal, "MDL-CBM-KRT-v2.3.1")
	assert.Contains(t, proyeksiAwal, "PRD-CBM-KRT-1171-14")

	t.Run("tanggal mengikuti hari ini UTC", func(t *testing.T) {
		dasar := hariDasarSeed(t, db)
		assert.Contains(t, []string{sebelum.Format(time.DateOnly), sesudah.Format(time.DateOnly)}, dasar.Format(time.DateOnly),
			"hari dasar = hari ini UTC, bukan tanggal tertanam di SQL")
		assert.Zero(t, hitungSQL(t, db, `SELECT COUNT(*) FROM prediksi_harga WHERE tanggal_target <> ?::date + horizon_hari OR dibuat_pada <> ?::date::timestamp AT TIME ZONE 'UTC'`,
			dasar.Format(time.DateOnly), dasar.Format(time.DateOnly)), "tanggal target = hari dasar + horizon")
		assert.Equal(t, 28, hitungSQL(t, db, `SELECT COUNT(*) FROM model_registry WHERE dilatih_pada = (?::date - CASE status WHEN 'PRODUKSI' THEN 7 ELSE 1 END)::timestamp AT TIME ZONE 'UTC'`,
			dasar.Format(time.DateOnly)), "model produksi dilatih 7 hari, kandidat 1 hari sebelum hari dasar")
	})

	t.Run("proyeksi tertaut ke model produksi demo komoditasnya", func(t *testing.T) {
		assert.Equal(t, 240, hitungSQL(t, db, `SELECT COUNT(*) FROM prediksi_harga p JOIN model_registry m ON m.id = p.model_id
			WHERE m.komoditas_kode = p.komoditas_kode AND m.versi = p.model_versi AND m.status = 'PRODUKSI' AND m.sumber = 'DEMO' AND p.sumber = 'DEMO'`),
			"model_id dan model_versi konsisten")
		assert.Zero(t, hitungSQL(t, db, `SELECT COUNT(*) FROM prediksi_harga WHERE NOT (bawah <= nilai AND nilai <= atas) OR harga_dasar <= 0 OR jsonb_array_length(faktor) = 0`))
	})

	// Menjalankan ulang di hari yang sama tidak mengubah apa pun.
	require.NoError(t, seed.SeedFiles(ctx, db, "demo/10_prediksi_harga.sql"))
	assert.Equal(t, modelAwal, idMenurutKode(t, db, "model_registry"))
	assert.Equal(t, proyeksiAwal, idMenurutKode(t, db, "prediksi_harga"))

	// Seed lama dari 4 hari lalu, dan kandidat CBM-KRT sudah dipromosikan lewat registri: seed ulang menggeser tanggal
	// ke hari ini tanpa menambah baris, mengganti ID, atau membatalkan promosi.
	require.NoError(t, db.Exec(`UPDATE prediksi_harga SET tanggal_target = tanggal_target - 4, dibuat_pada = dibuat_pada - interval '4 days'`).Error)
	require.NoError(t, db.Exec(`UPDATE model_registry SET dilatih_pada = dilatih_pada - interval '4 days'`).Error)
	require.NoError(t, db.Exec(`UPDATE model_registry SET status = 'ARSIP' WHERE kode = 'MDL-CBM-KRT-v2.3.1'`).Error)
	require.NoError(t, db.Exec(`UPDATE model_registry SET status = 'PRODUKSI' WHERE kode = 'MDL-CBM-KRT-v2.4.0-rc1'`).Error)

	require.NoError(t, seed.SeedFiles(ctx, db, "demo/10_prediksi_harga.sql"), "seed ulang tidak boleh gagal setelah promosi")

	assert.Equal(t, modelAwal, idMenurutKode(t, db, "model_registry"), "tidak ada baris baru dan ID tidak berubah")
	assert.Equal(t, proyeksiAwal, idMenurutKode(t, db, "prediksi_harga"), "seed ulang menggeser tanggal, bukan menambah baris")
	dasar := hariDasarSeed(t, db)
	var tanggal time.Time
	require.NoError(t, db.Raw("SELECT tanggal_target FROM prediksi_harga WHERE kode = 'PRD-CBM-KRT-1171-14'").Scan(&tanggal).Error)
	assert.Equal(t, dasar.AddDate(0, 0, 14).Format(time.DateOnly), tanggal.Format(time.DateOnly), "tanggal target bergeser mengikuti hari seed")
	assert.Zero(t, hitungSQL(t, db, `SELECT COUNT(*) FROM prediksi_harga WHERE tanggal_target <> ?::date + horizon_hari`, dasar.Format(time.DateOnly)))
	assert.Equal(t, 28, hitungSQL(t, db, `SELECT COUNT(*) FROM model_registry WHERE dilatih_pada = (?::date - CASE versi WHEN 'v2.3.1' THEN 7 ELSE 1 END)::timestamp AT TIME ZONE 'UTC'`,
		dasar.Format(time.DateOnly)))
	assert.Equal(t, "ARSIP", statusMenurutKode(t, db, "MDL-CBM-KRT-v2.3.1"), "promosi lewat registri tidak dibatalkan seed")
	assert.Equal(t, "PRODUKSI", statusMenurutKode(t, db, "MDL-CBM-KRT-v2.4.0-rc1"))
}

func hitungSQL(t *testing.T, db *gorm.DB, query string, args ...any) int {
	t.Helper()
	var n int
	require.NoError(t, db.Raw(query, args...).Scan(&n).Error)
	return n
}

func statusMenurutKode(t *testing.T, db *gorm.DB, kode string) string {
	t.Helper()
	var status string
	require.NoError(t, db.Raw(`SELECT status::text FROM model_registry WHERE kode = ?`, kode).Scan(&status).Error)
	return status
}

// jamDB membaca jam PostgreSQL. Waktu seed dihitung dari now() basis data, jadi pembandingnya harus jam yang sama:
// jam container Docker bisa tertinggal puluhan detik sampai menit dari host (mis. setelah Mac tidur).
func jamDB(t *testing.T, db *gorm.DB) time.Time {
	t.Helper()
	var jam time.Time
	require.NoError(t, db.Raw(`SELECT clock_timestamp()`).Scan(&jam).Error)
	return jam
}
