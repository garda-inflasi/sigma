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

// Tes kontrak seed lapangan: demo/08_lapangan_tugas.sql. Seeder Go lama (internal/lapangan/seeder) punya tes bentuk
// data (tiap petugas punya tugas aktif, campuran status dengan tenggat relatif) dan idempotensi; tes itu ikut terhapus
// bersama paketnya, jadi kontraknya dijaga di sini terhadap PostgreSQL sungguhan. Perilaku API ada di berkas tes lain.

var berkasSeedTugas = []string{
	"reference/01_master.sql", "reference/03_auth_rbac.sql", "demo/01_auth_users.sql", "demo/08_lapangan_tugas.sql",
}

func seedTugas(t *testing.T, nama string) *gorm.DB {
	t.Helper()
	db := pg.NewDatabase(t, nama)
	pgtest.MigrateUp(t, db)
	require.NoError(t, seed.SeedFiles(t.Context(), db, berkasSeedTugas...))
	return db
}

func hitungSeed(t *testing.T, db *gorm.DB, sql string, args ...any) int64 {
	t.Helper()
	var n int64
	require.NoError(t, db.Raw(sql, args...).Scan(&n).Error)
	return n
}

func TestSeed_TugasDemoLimaPerAkunLapanganDenganTenggatRelatif(t *testing.T) {
	db := seedTugas(t, "lapangan_seed_bentuk")

	assert.EqualValues(t, 60, hitungSeed(t, db, `SELECT count(*) FROM tugas_lapangan`), "12 akun lapangan x 5 tugas")
	assert.EqualValues(t, 12, hitungSeed(t, db, `SELECT count(DISTINCT petugas_id) FROM tugas_lapangan`))
	assert.EqualValues(t, 60, hitungSeed(t, db, `SELECT count(DISTINCT kode) FROM tugas_lapangan WHERE kode ~ '^TGS-[a-z_0-9]+-[1-5]$'`))
	assert.Zero(t, hitungSeed(t, db, `SELECT count(*) FROM tugas_lapangan t JOIN pengguna u ON u.id = t.petugas_id
		WHERE u.peran NOT IN ('OPS_KOORDINATOR', 'OPS_AGREGATOR', 'OPS_MAHASISWA') OR u.status <> 'AKTIF'`), "hanya akun lapangan aktif")
	assert.Zero(t, hitungSeed(t, db, `SELECT count(*) FROM tugas_lapangan t JOIN pasar p ON p.kode = t.pasar_id JOIN pengguna u ON u.id = t.petugas_id
		WHERE p.kabupaten_kode IS DISTINCT FROM u.kabupaten_kode`), "pasar tugas ada di kabupaten petugas")
	assert.EqualValues(t, 60, hitungSeed(t, db, `SELECT count(*) FROM tugas_lapangan WHERE dibuat_oleh_id = garda_uuid_stabil('pengguna', 'manajer_program')`))

	// Tiap petugas: satu berjalan, dua belum mulai, satu terlambat, satu selesai; tenggat relatif terhadap now().
	type templat struct {
		Urutan                  int
		Status                  string
		JamDariSekarang         float64
		Target, JumlahSelesai   int
		SelesaiPada             bool
		DibuatSebelumTenggatJam float64
		JumlahPetugas           int
	}
	var got []templat
	require.NoError(t, db.Raw(`
		SELECT right(kode, 1)::int AS urutan, status::text AS status,
		       round((extract(epoch FROM tenggat - now()) / 3600)::numeric, 1)::float8 AS jam_dari_sekarang,
		       target, jumlah_selesai, selesai_pada IS NOT NULL AS selesai_pada,
		       round((extract(epoch FROM tenggat - dibuat_pada) / 3600)::numeric, 1)::float8 AS dibuat_sebelum_tenggat_jam,
		       count(*) AS jumlah_petugas
		  FROM tugas_lapangan GROUP BY 1, 2, 3, 4, 5, 6, 7 ORDER BY 1`).Scan(&got).Error)
	require.Len(t, got, 5, "lima templat tugas")
	want := []struct {
		status                string
		jam                   float64
		target, jumlahSelesai int
		selesaiPada           bool
	}{
		{"BERJALAN", 2, 6, 4, false}, {"BELUM", 5, 2, 0, false}, {"BELUM", 26, 4, 0, false},
		{"TERLAMBAT", -3, 4, 1, false}, {"SELESAI", -22, 4, 4, true},
	}
	for i, w := range want {
		assert.Equal(t, i+1, got[i].Urutan)
		assert.Equal(t, w.status, got[i].Status, "templat %d", i+1)
		assert.InDelta(t, w.jam, got[i].JamDariSekarang, 0.1, "templat %d: tenggat relatif terhadap now()", i+1)
		assert.Equal(t, w.target, got[i].Target, "templat %d", i+1)
		assert.Equal(t, w.jumlahSelesai, got[i].JumlahSelesai, "templat %d", i+1)
		assert.Equal(t, w.selesaiPada, got[i].SelesaiPada, "templat %d: selesai_pada hanya untuk tugas selesai", i+1)
		assert.InDelta(t, 8, got[i].DibuatSebelumTenggatJam, 0.01, "dibuat delapan jam sebelum tenggat")
		assert.EqualValues(t, 12, got[i].JumlahPetugas, "tiap templat dimiliki 12 petugas")
	}
}

func TestSeed_TugasDemoIdempotenDanMenyegarkanTenggat(t *testing.T) {
	db := seedTugas(t, "lapangan_seed_ulang")
	var idAwal []int64
	require.NoError(t, db.Raw(`SELECT id FROM tugas_lapangan ORDER BY kode`).Scan(&idAwal).Error)

	// Keadaan basi: tenggat tertinggal tiga hari dan progres sudah digeser operator; seed ulang mengembalikan keadaan
	// awal relatif terhadap now() tanpa menggandakan baris maupun mengubah ID.
	require.NoError(t, db.Exec(`UPDATE tugas_lapangan SET tenggat = tenggat - interval '3 days', jumlah_selesai = 0, status = 'BELUM'`).Error)
	require.NoError(t, seed.SeedFiles(t.Context(), db, "demo/08_lapangan_tugas.sql"))

	var idAkhir []int64
	require.NoError(t, db.Raw(`SELECT id FROM tugas_lapangan ORDER BY kode`).Scan(&idAkhir).Error)
	assert.Equal(t, idAwal, idAkhir, "ID bigint stabil")
	assert.EqualValues(t, 60, hitungSeed(t, db, `SELECT count(*) FROM tugas_lapangan`))
	assert.EqualValues(t, 12, hitungSeed(t, db, `SELECT count(*) FROM tugas_lapangan WHERE status = 'SELESAI' AND jumlah_selesai = target`))
	assert.EqualValues(t, 12, hitungSeed(t, db, `SELECT count(*) FROM tugas_lapangan WHERE status = 'TERLAMBAT' AND tenggat < now()`))
	var terdekat time.Time
	require.NoError(t, db.Raw(`SELECT min(tenggat) FILTER (WHERE status <> 'SELESAI' AND status <> 'TERLAMBAT') FROM tugas_lapangan`).Scan(&terdekat).Error)
	assert.True(t, terdekat.After(jamDB(t, db)), "tugas aktif bertenggat di masa depan lagi: %s", terdekat)
}

// jamDB membaca jam PostgreSQL. Waktu seed dihitung dari now() basis data, jadi pembandingnya harus jam yang sama:
// jam container Docker bisa tertinggal puluhan detik sampai menit dari host (mis. setelah Mac tidur).
func jamDB(t *testing.T, db *gorm.DB) time.Time {
	t.Helper()
	var jam time.Time
	require.NoError(t, db.Raw(`SELECT clock_timestamp()`).Scan(&jam).Error)
	return jam
}
