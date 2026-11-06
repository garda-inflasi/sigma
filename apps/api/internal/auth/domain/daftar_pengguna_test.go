package domain

import (
	"errors"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestDaftarPenggunaQuery_Normal_NilaiBawaan(t *testing.T) {
	q, err := DaftarPenggunaQuery{}.Normal()

	require.NoError(t, err)
	assert.Equal(t, UrutDibuatPada, q.Urut)
	assert.Equal(t, ArahTurun, q.Arah, "tanpa sort: terbaru dulu, seperti sebelum ada urutan")
	assert.Equal(t, 1, q.Page)
	assert.Equal(t, LimitBawaanPengguna, q.Limit)
	assert.Zero(t, q.Offset())
}

func TestDaftarPenggunaQuery_Normal_ArahBawaanMengikutiAdanyaSort(t *testing.T) {
	cases := []struct {
		nama, urut, arah, wantArah string
	}{
		{"sort tanpa arah naik", UrutNama, "", ArahNaik},
		{"arah eksplisit menang", UrutNama, ArahTurun, ArahTurun},
		{"arah tanpa sort memakai kolom bawaan", "", ArahNaik, ArahNaik},
		{"tanpa keduanya turun", "", "", ArahTurun},
	}
	for _, tc := range cases {
		t.Run(tc.nama, func(t *testing.T) {
			q, err := DaftarPenggunaQuery{Urut: tc.urut, Arah: tc.arah}.Normal()
			require.NoError(t, err)
			assert.Equal(t, tc.wantArah, q.Arah)
		})
	}
}

func TestDaftarPenggunaQuery_Normal_SemuaKolomUrutDiterima(t *testing.T) {
	for _, kolom := range KolomUrutPengguna {
		for _, arah := range []string{ArahNaik, ArahTurun} {
			q, err := DaftarPenggunaQuery{Urut: kolom, Arah: arah}.Normal()
			require.NoError(t, err, kolom+" "+arah)
			assert.Equal(t, kolom, q.Urut)
			assert.Equal(t, arah, q.Arah)
		}
	}
}

func TestDaftarPenggunaQuery_Normal_MasukanSalahDitolakDenganNamaParameter(t *testing.T) {
	cases := []struct {
		nama  string
		q     DaftarPenggunaQuery
		param string
	}{
		{"sort bukan daftar putih", DaftarPenggunaQuery{Urut: "password_hash"}, "sort"},
		{"sort tanda kutip SQL", DaftarPenggunaQuery{Urut: "nama; DROP TABLE pengguna"}, "sort"},
		{"sort huruf besar", DaftarPenggunaQuery{Urut: "NAMA"}, "sort"},
		{"arah salah", DaftarPenggunaQuery{Urut: UrutNama, Arah: "ASC"}, "arah"},
		{"arah naik-turun", DaftarPenggunaQuery{Arah: "naik"}, "arah"},
		{"status tak dikenal", DaftarPenggunaQuery{Filter: FilterPengguna{Status: "aktif"}}, "status"},
		{"lapis tak dikenal", DaftarPenggunaQuery{Filter: FilterPengguna{Lapis: "OPS"}}, "lapis"},
		{"search terlalu panjang", DaftarPenggunaQuery{Filter: FilterPengguna{Cari: strings.Repeat("a", MaksPanjangCari+1)}}, "search"},
	}
	for _, tc := range cases {
		t.Run(tc.nama, func(t *testing.T) {
			_, err := tc.q.Normal()

			var galat *FilterPenggunaError
			require.True(t, errors.As(err, &galat), "harus *FilterPenggunaError, dapat %v", err)
			assert.Equal(t, tc.param, galat.Param)
			assert.NotEmpty(t, galat.Pesan)
			assert.Equal(t, galat.Pesan, galat.Error())
		})
	}
}

func TestDaftarPenggunaQuery_Normal_FilterDiterima(t *testing.T) {
	ya := true
	q, err := DaftarPenggunaQuery{Filter: FilterPengguna{
		Cari: "  Siti Rahma \t", Status: "NONAKTIF", Lapis: "SISTEM", Peran: "ADMIN_SISTEM", MFA: &ya, Terkunci: &ya,
	}}.Normal()

	require.NoError(t, err)
	assert.Equal(t, "Siti Rahma", q.Filter.Cari, "spasi tepi dibuang")
	assert.Equal(t, "ADMIN_SISTEM", q.Filter.Peran, "peran divalidasi usecase, bukan di sini")
}

func TestDaftarPenggunaQuery_Normal_PanjangCariDihitungDalamKarakter(t *testing.T) {
	// 100 karakter multibita tepat di batas: bukan 400 hanya karena byte-nya lebih dari 100.
	_, err := DaftarPenggunaQuery{Filter: FilterPengguna{Cari: strings.Repeat("é", MaksPanjangCari)}}.Normal()
	assert.NoError(t, err)
	_, err = DaftarPenggunaQuery{Filter: FilterPengguna{Cari: strings.Repeat("é", MaksPanjangCari+1)}}.Normal()
	assert.Error(t, err)
}

func TestDaftarPenggunaQuery_Normal_PaginasiDanOffset(t *testing.T) {
	cases := []struct {
		page, limit             int
		wantPage, wantLim, wOff int
	}{
		{0, 0, 1, LimitBawaanPengguna, 0},
		{-3, -1, 1, LimitBawaanPengguna, 0},
		{1, 10, 1, 10, 0},
		{3, 25, 3, 25, 50},
		{7, 100, 7, 100, 600},
	}
	for _, tc := range cases {
		q, err := DaftarPenggunaQuery{Page: tc.page, Limit: tc.limit}.Normal()
		require.NoError(t, err)
		assert.Equal(t, tc.wantPage, q.Page)
		assert.Equal(t, tc.wantLim, q.Limit)
		assert.Equal(t, tc.wOff, q.Offset())
	}
}

func TestDaftarPenggunaQuery_Normal_TidakMengubahPemanggil(t *testing.T) {
	asli := DaftarPenggunaQuery{Filter: FilterPengguna{Cari: " x "}}
	_, err := asli.Normal()
	require.NoError(t, err)
	assert.Equal(t, " x ", asli.Filter.Cari, "Normal bekerja pada salinan")
	assert.Zero(t, asli.Page)
}
