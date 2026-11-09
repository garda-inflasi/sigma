package integration

import (
	"hash/fnv"
	"slices"
	"strings"
	"testing"

	"github.com/garda-inflasi/api/internal/master"
	"github.com/garda-inflasi/api/internal/platform/pgtest"
	"github.com/garda-inflasi/api/pkg/seed"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

// Tes kontrak seed master: reference/01_master.sql, reference/02_kecamatan.sql, dan demo/02_master_pedagang.sql.
// Seeder Go lama (internal/master/seeder) punya tes bentuk data, idempotensi, dan kolom operasional; tes itu ikut
// terhapus bersama paketnya, jadi kontraknya dijaga di sini terhadap PostgreSQL sungguhan dan katalog Go
// (internal/master/data.go), yang tetap menjadi rujukan isi seed.

// seedDB membuat basis data bermigrasi khusus tes ini lalu menyemai berkas yang diminta.
func seedDB(t *testing.T, nama string, berkas ...string) *gorm.DB {
	t.Helper()
	db := pg.NewDatabase(t, nama)
	pgtest.MigrateUp(t, db)
	require.NoError(t, seed.SeedFiles(t.Context(), db, berkas...))
	return db
}

func hitungSeed(t *testing.T, db *gorm.DB, sql string, args ...any) int64 {
	t.Helper()
	var n int64
	require.NoError(t, db.Raw(sql, args...).Scan(&n).Error)
	return n
}

func TestSeed_ReferensiSamaDenganKatalogGo(t *testing.T) {
	db := seedDB(t, "master_seed_katalog", "reference/01_master.sql", "reference/02_kecamatan.sql")

	t.Run("satuan hanya kg dan liter", func(t *testing.T) {
		var got []struct{ Kode, Nama string }
		require.NoError(t, db.Raw(`SELECT kode, nama FROM satuan ORDER BY kode`).Scan(&got).Error)
		var want []struct{ Kode, Nama string }
		for _, s := range master.Satuan() {
			want = append(want, struct{ Kode, Nama string }{s.Kode, s.Nama})
		}
		assert.Equal(t, want, got, "katalog satuan tepat dua baris; satuan ganda seperti 'Kg' tidak boleh ada")
	})

	t.Run("komoditas", func(t *testing.T) {
		type baris struct {
			Kode, Nama, Satuan, Kategori string
			HargaAcuan                   int
			Pilot                        bool
			ShelfLifeHari                int
		}
		var got []baris
		require.NoError(t, db.Raw(`SELECT kode, nama, satuan, kategori, harga_acuan, pilot, shelf_life_hari FROM komoditas ORDER BY kode`).Scan(&got).Error)
		var want []baris
		for _, k := range master.Komoditas() {
			want = append(want, baris{k.Kode, k.Nama, k.Satuan, k.Kategori, k.HargaAcuan, k.Pilot, k.ShelfLifeHari})
		}
		slices.SortFunc(want, func(a, b baris) int { return strings.Compare(a.Kode, b.Kode) })
		assert.Equal(t, want, got)
		assert.EqualValues(t, 3, hitungSeed(t, db, `SELECT count(*) FROM komoditas WHERE pilot`), "tiga komoditas pilot fase 1")
	})

	t.Run("wilayah dan pasar", func(t *testing.T) {
		assert.EqualValues(t, 1, hitungSeed(t, db, `SELECT count(*) FROM provinsi`))
		assert.EqualValues(t, len(master.Kabupaten()), hitungSeed(t, db, `SELECT count(*) FROM kabupaten`))
		assert.EqualValues(t, len(master.Pasar()), hitungSeed(t, db, `SELECT count(*) FROM pasar`))

		type pasar struct {
			Kode, Nama, KabupatenKode string
			Lat, Lng                  float64
			RadiusMeter               int
			Aktif                     bool
		}
		var got []pasar
		require.NoError(t, db.Raw(`SELECT kode, nama, kabupaten_kode, lat, lng, radius_meter, aktif FROM pasar ORDER BY kode`).Scan(&got).Error)
		var want []pasar
		for _, p := range master.Pasar() {
			want = append(want, pasar{p.Kode, p.Nama, p.KabupatenKode, p.Lat, p.Lng, p.RadiusMeter, p.Aktif})
		}
		slices.SortFunc(want, func(a, b pasar) int { return strings.Compare(a.Kode, b.Kode) })
		assert.Equal(t, want, got)
	})

	t.Run("kecamatan", func(t *testing.T) {
		kecamatan, err := master.Kecamatan()
		require.NoError(t, err)
		assert.EqualValues(t, len(kecamatan), hitungSeed(t, db, `SELECT count(*) FROM kecamatan`))
		tanpaDagri := hitungSeed(t, db, `SELECT count(*) FROM kecamatan WHERE kode_dagri IS NULL`)
		var seedTanpaDagri int64
		for _, k := range kecamatan {
			if k.KodeDagri == nil {
				seedTanpaDagri++
			}
		}
		assert.Equal(t, seedTanpaDagri, tanpaDagri)
	})
}

func TestSeed_ReferensiIdempotenDanTidakMenimpaPerubahanOperasional(t *testing.T) {
	berkas := []string{"reference/01_master.sql", "reference/02_kecamatan.sql"}
	db := seedDB(t, "master_seed_idempoten", berkas...)
	tabel := []string{"satuan", "provinsi", "kabupaten", "kecamatan", "pasar", "komoditas"}
	sebelum := map[string]int64{}
	for _, tb := range tabel {
		sebelum[tb] = hitungSeed(t, db, `SELECT count(*) FROM `+tb)
	}
	// Perubahan operasional (arsip, harga acuan baru lewat persetujuan) bertahan sesudah seed dijalankan ulang.
	require.NoError(t, db.Exec(`UPDATE komoditas SET status = 'DIARSIPKAN', harga_acuan = 99999, versi = 2 WHERE kode = 'TMT'`).Error)
	require.NoError(t, db.Exec(`UPDATE pasar SET aktif = false, versi = 2 WHERE kode = 'PSR-1171-02'`).Error)
	var idKomoditas int64
	require.NoError(t, db.Raw(`SELECT id FROM komoditas WHERE kode = 'BWM'`).Scan(&idKomoditas).Error)

	require.NoError(t, seed.SeedFiles(t.Context(), db, berkas...))

	for _, tb := range tabel {
		assert.Equal(t, sebelum[tb], hitungSeed(t, db, `SELECT count(*) FROM `+tb), tb+": seed ulang tidak menambah baris")
	}
	assert.EqualValues(t, 1, hitungSeed(t, db, `SELECT count(*) FROM komoditas WHERE kode = 'TMT' AND status = 'DIARSIPKAN' AND harga_acuan = 99999 AND versi = 2`))
	assert.EqualValues(t, 1, hitungSeed(t, db, `SELECT count(*) FROM pasar WHERE kode = 'PSR-1171-02' AND NOT aktif AND versi = 2`))
	assert.EqualValues(t, idKomoditas, hitungSeed(t, db, `SELECT id FROM komoditas WHERE kode = 'BWM'`), "id surrogate baris yang sudah ada tidak berubah")
}

// namaPedagangDemo adalah NamaPedagangDemo seeder master Go yang digantikan: FNV-1a 64 bit atas "<pasar>|<kode>".
func namaPedagangDemo(pasarID, kode string) string {
	depan := []string{"Cut", "Teuku", "Tgk", "Nyak", "Ibu", "Bapak", "Pak", "Bu", "Abang", "Kak", "Dek", "Haji", "Hajjah", "Said", "Sofyan", "Zainal"}
	belakang := []string{"Aminah", "Bakri", "Dian", "Daud", "Erna", "Fauzi", "Gade", "Hasan", "Ismail", "Jamilah", "Kamal", "Latifah",
		"Mahdi", "Nurhayati", "Pocut", "Putra", "Qadri", "Rahmad", "Safwan", "Taufik", "Umar", "Yusnidar", "Zakiah", "Azhari"}
	h := fnv.New64a()
	_, _ = h.Write([]byte(pasarID + "|" + kode))
	v := h.Sum64()
	return depan[v%uint64(len(depan))] + " " + belakang[(v/uint64(len(depan)))%uint64(len(belakang))]
}

func TestSeed_PedagangDemoBernamaDeterministikDanIdempoten(t *testing.T) {
	berkas := []string{"reference/01_master.sql", "demo/02_master_pedagang.sql"}
	db := seedDB(t, "master_seed_pedagang", berkas...)
	jumlah := hitungSeed(t, db, `SELECT count(*) FROM pedagang`)
	require.Positive(t, jumlah)

	var rows []struct{ PasarID, Kode, Nama string }
	require.NoError(t, db.Raw(`SELECT pasar_id, kode, coalesce(nama, '') AS nama FROM pedagang`).Scan(&rows).Error)
	for _, r := range rows {
		assert.Equal(t, namaPedagangDemo(r.PasarID, r.Kode), r.Nama, "%s/%s", r.PasarID, r.Kode)
	}
	assert.Zero(t, hitungSeed(t, db, `SELECT count(*) FROM pedagang WHERE NOT aktif`), "semua pedagang demo aktif")

	// Pedagang yang namanya diubah atau dinonaktifkan tidak disentuh seed.
	require.NoError(t, db.Exec(`UPDATE pedagang SET nama = 'Nama Diubah', aktif = false WHERE kode = (SELECT min(kode) FROM pedagang)`).Error)
	require.NoError(t, seed.SeedFiles(t.Context(), db, "demo/02_master_pedagang.sql"))
	assert.Equal(t, jumlah, hitungSeed(t, db, `SELECT count(*) FROM pedagang`))
	assert.Positive(t, hitungSeed(t, db, `SELECT count(*) FROM pedagang WHERE nama = 'Nama Diubah' AND NOT aktif`))
}
