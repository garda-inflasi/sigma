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

// Tes kontrak seed harga: reference/06_harga_pasar.sql, demo/02_master_pedagang.sql, dan demo/03_harga_harian.sql.
// Seeder Go lama (internal/harga/seeder) punya tes bentuk data, idempotensi, dan versi entri; tes itu ikut terhapus
// bersama paketnya, jadi kontraknya dijaga di sini terhadap PostgreSQL sungguhan. Yang diperiksa hanya isi seed;
// perilaku API ada di berkas tes lain.

// berkasSeedHarga adalah prasyarat dan isi seed harga, berurutan.
var berkasSeedHarga = []string{
	"reference/01_master.sql", "reference/03_auth_rbac.sql", "demo/01_auth_users.sql",
	"reference/06_harga_pasar.sql", "demo/02_master_pedagang.sql", "demo/03_harga_harian.sql",
}

func seedHarga(t *testing.T, nama string) *gorm.DB {
	t.Helper()
	db := pg.NewDatabase(t, nama)
	pgtest.MigrateUp(t, db)
	require.NoError(t, seed.SeedFiles(t.Context(), db, berkasSeedHarga...))
	return db
}

func hitungSeed(t *testing.T, db *gorm.DB, sql string, args ...any) int64 {
	t.Helper()
	var n int64
	require.NoError(t, db.Raw(sql, args...).Scan(&n).Error)
	return n
}

func TestSeed_HistoriDemoBerbentukSepertiSeederLama(t *testing.T) {
	db := seedHarga(t, "harga_seed_bentuk")

	t.Run("hanya komoditas master, semua status terwakili", func(t *testing.T) {
		assert.EqualValues(t, 14, hitungSeed(t, db, `SELECT count(DISTINCT komoditas_kode) FROM entri_harga`), "14 komoditas master, bukan kode lama")
		assert.Zero(t, hitungSeed(t, db, `SELECT count(*) FROM entri_harga WHERE komoditas_kode !~ '^[A-Z]+(-[A-Z]+)?$'`))
		var status []string
		require.NoError(t, db.Raw(`SELECT DISTINCT status::text FROM entri_harga ORDER BY 1`).Scan(&status).Error)
		assert.Equal(t, []string{"DIAJUKAN", "DIPUBLIKASI", "DITOLAK", "DIVALIDASI", "DRAF"}, status)
		assert.Equal(t, hitungSeed(t, db, `SELECT count(*) FROM entri_harga`), hitungSeed(t, db, `SELECT count(DISTINCT kode) FROM entri_harga WHERE kode LIKE 'HRG-D%'`), "kode HRG-D... unik")
	})

	t.Run("hasil validasi otomatis terisi dan konsisten", func(t *testing.T) {
		assert.Positive(t, hitungSeed(t, db, `SELECT count(*) FROM entri_harga WHERE (flag_validasi->>'outlier')::boolean`), "histori memuat outlier bertanda")
		assert.Positive(t, hitungSeed(t, db, `SELECT count(*) FROM entri_harga WHERE abs(deviasi_pct) > 0`))
		assert.Positive(t, hitungSeed(t, db, `SELECT count(*) FROM entri_harga WHERE skor_anomali > 0`))
		assert.Positive(t, hitungSeed(t, db, `SELECT count(*) FROM entri_harga WHERE foto_url IS NOT NULL`))
		assert.Zero(t, hitungSeed(t, db, `SELECT count(*) FROM entri_harga
			WHERE NOT (flag_validasi ?& ARRAY['median', 'outlier', 'duplikat', 'luar_geofence', 'bentrok', 'tanpa_lokasi', 'di_luar_rentang'])`), "semua kunci flag ada")
		assert.Zero(t, hitungSeed(t, db, `SELECT count(*) FROM entri_harga WHERE (flag_validasi->>'outlier')::boolean <> (abs(deviasi_pct) >= 20)`), "outlier = deviasi besar")
		assert.Zero(t, hitungSeed(t, db, `SELECT count(*) FROM entri_harga WHERE (flag_validasi->>'luar_geofence')::boolean = dalam_geofence`), "luar geofence = tidak dalam geofence")
		assert.Zero(t, hitungSeed(t, db, `SELECT count(*) FROM entri_harga WHERE (flag_validasi->>'duplikat')::boolean <> (kode LIKE '%-dup')`), "duplikat = kode -dup")
		assert.Positive(t, hitungSeed(t, db, `SELECT count(*) FROM entri_harga WHERE kode LIKE '%-dup'`))
	})

	t.Run("jejak transisi lengkap", func(t *testing.T) {
		assert.Zero(t, hitungSeed(t, db, `SELECT count(*) FROM entri_harga WHERE status = 'DITOLAK' AND (ditolak_oleh IS NULL OR ditolak_pada IS NULL OR alasan_penolakan IS NULL)`))
		assert.Zero(t, hitungSeed(t, db, `SELECT count(*) FROM entri_harga WHERE status IN ('DIVALIDASI', 'DIPUBLIKASI') AND (divalidasi_oleh IS NULL OR divalidasi_pada IS NULL)`))
		assert.Zero(t, hitungSeed(t, db, `SELECT count(*) FROM entri_harga WHERE status = 'DIPUBLIKASI' AND (dipublikasi_oleh IS NULL OR dipublikasi_pada IS NULL)`))
		assert.Zero(t, hitungSeed(t, db, `SELECT count(*) FROM entri_harga WHERE dibuat_pada <> dicatat_pada`), "dibuat = dicatat, seperti seeder lama")
		assert.Zero(t, hitungSeed(t, db, `SELECT count(*) FROM entri_harga
			WHERE diupdate_pada <> coalesce(dipublikasi_pada, ditolak_pada, divalidasi_pada, dicatat_pada)`), "diupdate = waktu transisi terakhir")
		// Riwayat: transisi awal bertanda waktu catat entri, transisi akhir sama dengan status entri.
		assert.Zero(t, hitungSeed(t, db, `SELECT count(*) FROM riwayat_status_entri r JOIN entri_harga e ON e.id = r.entri_id
			WHERE r.dari IS NULL AND r.waktu <> e.dicatat_pada`))
		assert.Zero(t, hitungSeed(t, db, `SELECT count(*) FROM entri_harga e
			WHERE (SELECT r.ke FROM riwayat_status_entri r WHERE r.entri_id = e.id ORDER BY r.waktu DESC, r.id DESC LIMIT 1) <> e.status`))
		// Pemicu versi menulis satu versi INPUT per entri.
		assert.Equal(t, hitungSeed(t, db, `SELECT count(*) FROM entri_harga`), hitungSeed(t, db, `SELECT count(*) FROM entri_harga_versi WHERE aksi = 'INPUT' AND versi = 1`))
	})

	t.Run("waktu relatif terhadap sekarang", func(t *testing.T) {
		var r struct{ Terbaru, Tertua time.Time }
		require.NoError(t, db.Raw(`SELECT max(dicatat_pada) AS terbaru, min(dicatat_pada) AS tertua FROM entri_harga`).Scan(&r).Error)
		sekarang := jamDB(t, db)
		assert.False(t, r.Terbaru.After(sekarang), "tidak ada entri di masa depan: %s", r.Terbaru)
		assert.WithinDuration(t, sekarang, r.Terbaru, 3*time.Minute, "entri terbaru berumur hanya beberapa menit")
		assert.Positive(t, hitungSeed(t, db, `SELECT count(*) FROM entri_harga WHERE dicatat_pada >= date_trunc('day', now() AT TIME ZONE 'Asia/Jakarta') AT TIME ZONE 'Asia/Jakarta'`),
			"hari ini (WIB) sudah ada entri, seperti seeder lama")
		assert.InDelta(t, 89*24, r.Terbaru.Sub(r.Tertua).Hours(), 24, "sembilan puluh hari histori (hari ini sampai 89 hari lalu)")
		// Hari histori sejajar kalender WIB seperti seeder lama: entri HRG-D<n> dicatat n hari sebelum hari ini (WIB),
		// berapa pun jam saat seed dijalankan. Deret harian dashboard dan lonjakan cabai bergantung pada ini.
		assert.Zero(t, hitungSeed(t, db, `SELECT count(*) FROM entri_harga
			WHERE (now() AT TIME ZONE 'Asia/Jakarta')::date - (dicatat_pada AT TIME ZONE 'Asia/Jakarta')::date
			      <> substring(kode FROM '^HRG-D(\d+)-')::int`), "hari histori = hari kalender WIB")
	})

	t.Run("batas harga satu per komoditas aktif", func(t *testing.T) {
		assert.EqualValues(t, 14, hitungSeed(t, db, `SELECT count(*) FROM batas_harga WHERE kabupaten_kode IS NULL`))
	})
}

func TestSeed_PedagangDemoSamaDenganPasanganPadaEntri(t *testing.T) {
	db := seedHarga(t, "harga_seed_pedagang")

	assert.Zero(t, hitungSeed(t, db, `SELECT count(*) FROM (
		SELECT DISTINCT pasar_id, kode_pedagang FROM entri_harga WHERE kode_pedagang IS NOT NULL
		EXCEPT SELECT pasar_id, kode FROM pedagang) x`), "setiap pedagang pada entri terdaftar")
	assert.Zero(t, hitungSeed(t, db, `SELECT count(*) FROM (
		SELECT pasar_id, kode FROM pedagang
		EXCEPT SELECT DISTINCT pasar_id, kode_pedagang FROM entri_harga) x`), "tidak ada pedagang tanpa entri")
}

func TestSeed_DijalankanUlangTidakMenggandakanDanWaktuBerikutnyaHanyaMenggeserWaktu(t *testing.T) {
	db := seedHarga(t, "harga_seed_ulang")
	hitung := func() [4]int64 {
		return [4]int64{
			hitungSeed(t, db, `SELECT count(*) FROM entri_harga`),
			hitungSeed(t, db, `SELECT count(*) FROM riwayat_status_entri`),
			hitungSeed(t, db, `SELECT count(*) FROM entri_harga_versi`),
			hitungSeed(t, db, `SELECT count(*) FROM batas_harga`),
		}
	}
	awal := hitung()
	var tertinggi, terbaru, lamaAwal, lamaAkhir time.Time
	require.NoError(t, db.Raw(`SELECT max(dicatat_pada) FROM entri_harga`).Scan(&terbaru).Error)
	require.NoError(t, db.Raw(`SELECT dicatat_pada FROM entri_harga WHERE kode = 'HRG-D05-00-02-K'`).Scan(&lamaAwal).Error)

	require.NoError(t, seed.SeedFiles(t.Context(), db, berkasSeedHarga[3:]...))
	assert.Equal(t, awal, hitung(), "dijalankan ulang segera: tidak menambah baris maupun versi")

	// Seed di waktu yang lebih akhir: stempel waktu baris yang ada tertinggal tiga hari. Perubahan operator pada kolom isi
	// (catatan) bertahan, dan menggeser waktu tidak membuat versi entri baru (kolom waktu diabaikan pemicu versi).
	require.NoError(t, db.Exec(`UPDATE entri_harga SET catatan = 'diubah operator' WHERE kode = 'HRG-D05-00-02-K'`).Error)
	require.NoError(t, db.Exec(`UPDATE entri_harga SET
		dicatat_pada = dicatat_pada - interval '3 days', dibuat_pada = dibuat_pada - interval '3 days',
		diupdate_pada = diupdate_pada - interval '3 days', divalidasi_pada = divalidasi_pada - interval '3 days',
		dipublikasi_pada = dipublikasi_pada - interval '3 days', ditolak_pada = ditolak_pada - interval '3 days'`).Error)
	require.NoError(t, db.Exec(`UPDATE riwayat_status_entri SET waktu = waktu - interval '3 days'`).Error)
	sesudahOperator := hitung()
	assert.Equal(t, awal[2]+1, sesudahOperator[2], "hanya perubahan catatan yang membuat versi")

	require.NoError(t, seed.SeedFiles(t.Context(), db, "demo/03_harga_harian.sql"))

	assert.Equal(t, sesudahOperator, hitung(), "menggeser waktu tidak menambah baris maupun versi")
	require.NoError(t, db.Raw(`SELECT max(dicatat_pada) FROM entri_harga`).Scan(&tertinggi).Error)
	// Entri hari ini dipetakan ke [00.00 WIB, now()], jadi yang terbaru tetap mengikuti now(); entri hari lampau
	// digeser per hari penuh sehingga kembali tepat ke stempel seed pertama (kelipatan 24 jam bila hari berganti).
	assert.WithinDuration(t, terbaru, tertinggi, 5*time.Minute, "waktu kembali ke keadaan seed")
	require.NoError(t, db.Raw(`SELECT dicatat_pada FROM entri_harga WHERE kode = 'HRG-D05-00-02-K'`).Scan(&lamaAkhir).Error)
	assert.Zero(t, lamaAkhir.Sub(lamaAwal)%(24*time.Hour), "hari lampau bergeser per hari penuh: %s -> %s", lamaAwal, lamaAkhir)
	assert.EqualValues(t, 1, hitungSeed(t, db, `SELECT count(*) FROM entri_harga WHERE kode = 'HRG-D05-00-02-K' AND catatan = 'diubah operator'`), "kolom isi tidak ditimpa")
	assert.Zero(t, hitungSeed(t, db, `SELECT count(*) FROM riwayat_status_entri r JOIN entri_harga e ON e.id = r.entri_id WHERE r.dari IS NULL AND r.waktu <> e.dicatat_pada`))
}

// jamDB membaca jam PostgreSQL. Waktu seed dihitung dari now() basis data, jadi pembandingnya harus jam yang sama:
// jam container Docker bisa tertinggal puluhan detik sampai menit dari host (mis. setelah Mac tidur).
func jamDB(t *testing.T, db *gorm.DB) time.Time {
	t.Helper()
	var jam time.Time
	require.NoError(t, db.Raw(`SELECT clock_timestamp()`).Scan(&jam).Error)
	return jam
}
