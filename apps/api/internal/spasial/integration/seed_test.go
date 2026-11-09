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

// Tes seed SQL spasial (db/seeds/demo/09_spasial_neraca.sql). Pengganti tes seeder Go lama: lapis produksi tidak
// membawa neraca atau rute demo, seed demo idempoten dalam satu bulan dan menambah periode di bulan baru tanpa menimpa
// periode lama, ID BIGINT tidak berubah, dan cache jarak tidak diisi seed.

func hitungTabel(t *testing.T, db *gorm.DB, tabel string) int64 {
	t.Helper()
	var n int64
	require.NoError(t, db.Table(tabel).Count(&n).Error)
	return n
}

// idTeks membaca ID baris (urut menurut ID) sebagai satu teks, supaya perubahan ID atau jumlah baris terbaca.
func idTeks(t *testing.T, db *gorm.DB, tabel string) string {
	t.Helper()
	var out string
	require.NoError(t, db.Raw(`SELECT COALESCE(string_agg(id::text, ',' ORDER BY id), '') FROM `+tabel).Scan(&out).Error)
	return out
}

func awalBulan(t time.Time) string {
	t = t.UTC()
	return time.Date(t.Year(), t.Month(), 1, 0, 0, 0, 0, time.UTC).Format(time.DateOnly)
}

func TestSeed_LapisReferenceTidakMembawaDataDemoSpasial(t *testing.T) {
	db := pg.NewDatabase(t, "spasial_seed_reference")
	pgtest.MigrateUp(t, db)
	require.NoError(t, seed.SeedReference(t.Context(), db))

	// Angka yang tampak resmi (neraca pangan) dan usulan rute tidak boleh ikut ke produksi.
	for _, tabel := range []string{"neraca_wilayah", "rute_distribusi", "kandidat_cold_storage", "jarak_antar_wilayah"} {
		assert.Zero(t, hitungTabel(t, db, tabel), tabel)
	}
}

func TestSeed_DemoIdempotenDalamBulanDanMenambahPeriodeDiBulanBaru(t *testing.T) {
	db := pg.NewDatabase(t, "spasial_seed_demo")
	pgtest.MigrateUp(t, db)
	ctx := t.Context()
	require.NoError(t, seed.SeedReference(ctx, db))

	sebelum := jamDB(t, db)
	require.NoError(t, seed.SeedFiles(ctx, db, berkasSeedDemo...))
	sesudah := jamDB(t, db)

	assert.EqualValues(t, 8*23, hitungTabel(t, db, "neraca_wilayah"), "8 komoditas x 23 kabupaten/kota")
	assert.EqualValues(t, 5, hitungTabel(t, db, "rute_distribusi"))
	assert.EqualValues(t, 3, hitungTabel(t, db, "kandidat_cold_storage"))
	assert.EqualValues(t, 23, hitungTabel(t, db, "wilayah_geometri"))
	assert.Zero(t, hitungTabel(t, db, "jarak_antar_wilayah"), "cache jarak diisi optimasi rute, bukan seed")

	periode := periodeSeed(t, db)
	assert.Contains(t, []string{awalBulan(sebelum), awalBulan(sesudah)}, periode, "periode neraca = awal bulan ini UTC, bukan tanggal tertanam di SQL")

	neraca, rute, kandidat := idTeks(t, db, "neraca_wilayah"), idTeks(t, db, "rute_distribusi"), idTeks(t, db, "kandidat_cold_storage")

	t.Run("seed ulang di bulan yang sama tidak mengubah apa pun", func(t *testing.T) {
		require.NoError(t, seed.SeedFiles(ctx, db, "demo/09_spasial_neraca.sql"))
		assert.Equal(t, neraca, idTeks(t, db, "neraca_wilayah"))
		assert.Equal(t, rute, idTeks(t, db, "rute_distribusi"))
		assert.Equal(t, kandidat, idTeks(t, db, "kandidat_cold_storage"))
	})

	t.Run("bulan baru menambah periode dan tidak menimpa periode lama", func(t *testing.T) {
		// Seed lama dari bulan lalu: semua neraca berpindah ke periode sebelumnya, lalu seed dijalankan lagi.
		require.NoError(t, db.Exec(`UPDATE neraca_wilayah SET periode = (periode - interval '1 month')::date`).Error)
		require.NoError(t, db.Exec(`UPDATE rute_distribusi SET status = 'DISETUJUI' WHERE kode = 'RT-1106-1171'`).Error)

		require.NoError(t, seed.SeedFiles(ctx, db, "demo/09_spasial_neraca.sql"))

		assert.EqualValues(t, 2*8*23, hitungTabel(t, db, "neraca_wilayah"), "periode baru ditambahkan di samping periode lama")
		assert.Equal(t, periode, periodeSeed(t, db))
		assert.Equal(t, rute, idTeks(t, db, "rute_distribusi"), "rute tidak digandakan dan ID tidak berubah")
		assert.Equal(t, kandidat, idTeks(t, db, "kandidat_cold_storage"))
		var status string
		require.NoError(t, db.Raw(`SELECT status FROM rute_distribusi WHERE kode = 'RT-1106-1171'`).Scan(&status).Error)
		assert.Equal(t, "DISETUJUI", status, "perubahan status rute tidak ditimpa seed ulang")
	})
}

// jamDB membaca jam PostgreSQL. Waktu seed dihitung dari now() basis data, jadi pembandingnya harus jam yang sama:
// jam container Docker bisa tertinggal puluhan detik sampai menit dari host (mis. setelah Mac tidur).
func jamDB(t *testing.T, db *gorm.DB) time.Time {
	t.Helper()
	var jam time.Time
	require.NoError(t, db.Raw(`SELECT clock_timestamp()`).Scan(&jam).Error)
	return jam
}
