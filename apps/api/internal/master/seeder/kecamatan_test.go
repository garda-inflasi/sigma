package seeder

import (
	"bytes"
	"strings"
	"testing"

	"github.com/garda-inflasi/api/internal/master/domain"
	"github.com/garda-inflasi/api/internal/platform/pgtest"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func ptr(s string) *string { return &s }

func TestKecamatanCSV_TertanamValid(t *testing.T) {
	rows, err := Kecamatan()
	require.NoError(t, err)
	meta, err := MetaKecamatanTertanam()
	require.NoError(t, err)

	t.Run("jumlah baris sama dengan kepala berkas", func(t *testing.T) {
		assert.Equal(t, meta.Jumlah, len(rows))
		assert.NotEmpty(t, rows)
	})

	t.Run("catatan sumber lengkap", func(t *testing.T) {
		assert.Contains(t, meta.URL, "sig.bps.go.id/rest-bridging/getwilayah")
		assert.NotEmpty(t, meta.PeriodeMerge)
		assert.Regexp(t, `^\d{4}-\d{2}-\d{2}$`, meta.Diambil)
		assert.Contains(t, string(kecamatanCSV), "Sumber: Badan Pusat Statistik (BPS)")
	})

	t.Run("kode unik, tujuh digit, dan kabupatennya salah satu dari 23 seed", func(t *testing.T) {
		kab := KodeKabupaten()
		require.Len(t, kab, 23)
		seen := map[string]bool{}
		perKab := map[string]int{}
		for _, r := range rows {
			assert.False(t, seen[r.Kode], "kode ganda %s", r.Kode)
			seen[r.Kode] = true
			assert.True(t, kab[r.KabupatenKode], "%s: kabupaten %s bukan seed", r.Kode, r.KabupatenKode)
			assert.True(t, strings.HasPrefix(r.Kode, r.KabupatenKode), r.Kode)
			assert.NotEmpty(t, strings.TrimSpace(r.Nama), r.Kode)
			perKab[r.KabupatenKode]++
		}
		assert.Len(t, perKab, 23, "setiap kabupaten/kota punya kecamatan")
	})

	t.Run("terurut menurut kode", func(t *testing.T) {
		for i := 1; i < len(rows); i++ {
			assert.Less(t, rows[i-1].Kode, rows[i].Kode)
		}
	})
}

func TestValidasiKecamatan(t *testing.T) {
	kab := map[string]bool{"1171": true, "1101": true}
	baik := domain.Kecamatan{Kode: "1171010", Nama: "Meuraxa", KabupatenKode: "1171", KodeDagri: ptr("117103")}

	assert.NoError(t, ValidasiKecamatan([]domain.Kecamatan{baik}, kab))

	cases := map[string]struct {
		rows []domain.Kecamatan
		want string
	}{
		"kode bukan tujuh digit":     {[]domain.Kecamatan{{Kode: "117101", Nama: "X", KabupatenKode: "1171"}}, "tujuh digit"},
		"kode bukan angka":           {[]domain.Kecamatan{{Kode: "11710AB", Nama: "X", KabupatenKode: "1171"}}, "tujuh digit"},
		"awalan beda dengan induk":   {[]domain.Kecamatan{{Kode: "1101010", Nama: "X", KabupatenKode: "1171"}}, "empat digit awal"},
		"kabupaten bukan seed":       {[]domain.Kecamatan{{Kode: "1199010", Nama: "X", KabupatenKode: "1199"}}, "bukan kabupaten/kota seed"},
		"kode ganda":                 {[]domain.Kecamatan{baik, baik}, "kode ganda"},
		"nama kosong":                {[]domain.Kecamatan{{Kode: "1171010", Nama: "  ", KabupatenKode: "1171"}}, "nama kosong"},
		"kode dagri kepanjangan":     {[]domain.Kecamatan{{Kode: "1171010", Nama: "X", KabupatenKode: "1171", KodeDagri: ptr("12345678901")}}, "kode_dagri"},
		"kode dagri kosong tersurat": {[]domain.Kecamatan{{Kode: "1171010", Nama: "X", KabupatenKode: "1171", KodeDagri: ptr(" ")}}, "kode_dagri"},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			err := ValidasiKecamatan(tc.rows, kab)
			require.Error(t, err)
			assert.Contains(t, err.Error(), tc.want)
		})
	}
}

func TestKecamatanCSV_TulisLaluBacaDeterministik(t *testing.T) {
	kab := map[string]bool{"1171": true, "1101": true}
	rows := []domain.Kecamatan{
		{Kode: "1171020", Nama: "Baiturrahman, Pusat", KabupatenKode: "1171", KodeDagri: ptr("117101")},
		{Kode: "1101010", Nama: "Teupah Selatan", KabupatenKode: "1101"},
		{Kode: "1171010", Nama: "Meuraxa", KabupatenKode: "1171", KodeDagri: ptr("117103")},
	}
	meta := MetaKecamatan{URL: "https://contoh.test/getwilayah?parent=<kode kabupaten>", PeriodeMerge: "2024_1.2022", Diambil: "2026-11-07"}

	var a, b bytes.Buffer
	require.NoError(t, TulisKecamatanCSV(&a, meta, rows))
	require.NoError(t, TulisKecamatanCSV(&b, meta, []domain.Kecamatan{rows[2], rows[0], rows[1]}))
	assert.Equal(t, a.String(), b.String(), "urutan masukan tidak memengaruhi keluaran")
	assert.Contains(t, a.String(), "# Jumlah baris: 3\n")

	gotMeta, got, err := BacaKecamatanCSV(bytes.NewReader(a.Bytes()), kab)
	require.NoError(t, err)
	meta.Jumlah = 3
	assert.Equal(t, meta, gotMeta)
	require.Len(t, got, 3)
	assert.Equal(t, "1101010", got[0].Kode)
	assert.Nil(t, got[0].KodeDagri)
	assert.Equal(t, "Baiturrahman, Pusat", got[2].Nama, "koma dalam nama terlindungi kutip")
}

func TestBacaKecamatanCSV_MenolakBerkasRusak(t *testing.T) {
	kab := map[string]bool{"1171": true}
	kepala := "# Sumber: BPS\n# URL: u\n# periode_merge: p\n# Diambil: 2026-11-07\n"
	cases := map[string]string{
		"tanpa catatan sumber":     "# URL: u\n# periode_merge: p\n# Diambil: d\n# Jumlah baris: 0\nkode,nama,kabupaten_kode,kode_dagri\n",
		"tanpa jumlah baris":       kepala + "kode,nama,kabupaten_kode,kode_dagri\n",
		"jumlah tidak cocok":       kepala + "# Jumlah baris: 2\nkode,nama,kabupaten_kode,kode_dagri\n1171010,Meuraxa,1171,117103\n",
		"kepala kolom salah":       kepala + "# Jumlah baris: 0\nkode,nama\n",
		"jumlah kolom salah":       kepala + "# Jumlah baris: 1\nkode,nama,kabupaten_kode,kode_dagri\n1171010,Meuraxa\n",
		"tidak terurut":            kepala + "# Jumlah baris: 2\nkode,nama,kabupaten_kode,kode_dagri\n1171020,B,1171,\n1171010,A,1171,\n",
		"kode ganda":               kepala + "# Jumlah baris: 2\nkode,nama,kabupaten_kode,kode_dagri\n1171010,A,1171,\n1171010,B,1171,\n",
		"kabupaten tak dikenal":    kepala + "# Jumlah baris: 1\nkode,nama,kabupaten_kode,kode_dagri\n1199010,A,1199,\n",
		"jumlah baris bukan angka": kepala + "# Jumlah baris: banyak\nkode,nama,kabupaten_kode,kode_dagri\n",
	}
	for name, isi := range cases {
		t.Run(name, func(t *testing.T) {
			_, _, err := BacaKecamatanCSV(strings.NewReader(isi), kab)
			assert.Error(t, err)
		})
	}
}

func TestNamaPedagangDemo_Deterministik(t *testing.T) {
	assert.Equal(t, NamaPedagangDemo("PSR-1171-01", "PDG-001"), NamaPedagangDemo("PSR-1171-01", "PDG-001"))
	assert.NotEmpty(t, NamaPedagangDemo("PSR-1171-01", "PDG-001"))
	variasi := map[string]bool{}
	for i := 0; i < 50; i++ {
		variasi[NamaPedagangDemo("PSR-1171-01", "PDG-"+strings.Repeat("x", i))] = true
	}
	assert.Greater(t, len(variasi), 10, "nama beragam, bukan satu nama untuk semua")
}

func TestSeedReference_KecamatanIdempotenDanTerisiLengkap(t *testing.T) {
	db := pg.NewDatabase(t, "master_seed_kecamatan")
	pgtest.MigrateUp(t, db)

	_, err := SeedReference(t.Context(), db)
	require.NoError(t, err)
	want, err := Kecamatan()
	require.NoError(t, err)
	assert.EqualValues(t, len(want), rowCount(t, db, "kecamatan"))

	// Nama yang menyimpang dikembalikan ke CSV; baris tak berubah tidak bertambah.
	require.NoError(t, db.Exec(`UPDATE kecamatan SET nama = 'diubah', kode_dagri = NULL WHERE kode = '1171010'`).Error)
	before := rowCounts(t, db)
	_, err = SeedReference(t.Context(), db)
	require.NoError(t, err)
	assert.Equal(t, before, rowCounts(t, db))

	var row struct {
		Nama      string
		KodeDagri *string
	}
	require.NoError(t, db.Raw(`SELECT nama, kode_dagri FROM kecamatan WHERE kode = '1171010'`).Scan(&row).Error)
	assert.Equal(t, "Meuraxa", row.Nama)
	require.NotNil(t, row.KodeDagri)
	assert.Equal(t, "117103", *row.KodeDagri)

	// Kecamatan yang dirujuk pasar tidak bisa dihapus (FK RESTRICT) dan seed tetap berjalan di atasnya.
	require.NoError(t, db.Exec(`UPDATE pasar SET kecamatan_kode = '1171030' WHERE kode = 'PSR-1171-01'`).Error)
	err = db.Exec(`DELETE FROM kecamatan WHERE kode = '1171030'`).Error
	require.Error(t, err)
	assert.Contains(t, err.Error(), "23503")
	_, err = SeedReference(t.Context(), db)
	require.NoError(t, err)
	var kec *string
	require.NoError(t, db.Raw(`SELECT kecamatan_kode FROM pasar WHERE kode = 'PSR-1171-01'`).Scan(&kec).Error)
	require.NotNil(t, kec, "seed tidak menimpa kecamatan_kode pasar")
	assert.Equal(t, "1171030", *kec)
}

func TestSeedDemo_PedagangDariEntriHargaIdempoten(t *testing.T) {
	db := pg.NewDatabase(t, "master_seed_pedagang")
	pgtest.MigrateUp(t, db)
	_, err := SeedReference(t.Context(), db)
	require.NoError(t, err)

	// Tanpa entri harga: tidak ada yang dibuat.
	sum, err := SeedDemo(t.Context(), db)
	require.NoError(t, err)
	assert.Zero(t, sum.Pedagang)
	assert.Zero(t, rowCount(t, db, "pedagang"))

	require.NoError(t, db.Exec(`
		INSERT INTO entri_harga (komoditas_kode, pasar_id, tipe, harga, satuan, dicatat_pada, kode_pedagang) VALUES
		('CBM-KRT', 'PSR-1171-01', 'KONSUMEN', 52000, 'kg', now(), 'PDG-001'),
		('BWM',     'PSR-1171-01', 'KONSUMEN', 41000, 'kg', now(), 'PDG-001'),
		('BWM',     'PSR-1171-01', 'KONSUMEN', 41500, 'kg', now(), 'PDG-002'),
		('BWM',     'PSR-1171-02', 'KONSUMEN', 41500, 'kg', now(), 'PDG-001'),
		('BWM',     'PSR-1171-02', 'KONSUMEN', 41500, 'kg', now(), NULL),
		('BWM',     'PSR-1171-02', 'KONSUMEN', 41500, 'kg', now(), '  ')`).Error)

	first, err := SeedDemo(t.Context(), db)
	require.NoError(t, err)
	assert.Equal(t, 3, first.Pedagang, "pasangan (pasar, kode) berbeda; kode kosong dilewati")
	assert.EqualValues(t, 3, rowCount(t, db, "pedagang"))

	var nama string
	require.NoError(t, db.Raw(`SELECT nama FROM pedagang WHERE pasar_id = 'PSR-1171-01' AND kode = 'PDG-001'`).Scan(&nama).Error)
	assert.Equal(t, NamaPedagangDemo("PSR-1171-01", "PDG-001"), nama)

	// Penyuntingan manual dan penonaktifan tidak ditimpa; jalan ulang tidak menambah baris.
	require.NoError(t, db.Exec(`UPDATE pedagang SET nama = 'Nama sunting', aktif = false WHERE pasar_id = 'PSR-1171-01' AND kode = 'PDG-001'`).Error)
	second, err := SeedDemo(t.Context(), db)
	require.NoError(t, err)
	assert.Equal(t, first, second)
	assert.EqualValues(t, 3, rowCount(t, db, "pedagang"))
	var row struct {
		Nama  string
		Aktif bool
	}
	require.NoError(t, db.Raw(`SELECT nama, aktif FROM pedagang WHERE pasar_id = 'PSR-1171-01' AND kode = 'PDG-001'`).Scan(&row).Error)
	assert.Equal(t, "Nama sunting", row.Nama)
	assert.False(t, row.Aktif)
}
