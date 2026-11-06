package dto

import (
	"strings"
	"testing"

	"github.com/garda-inflasi/api/internal/auth/domain"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestEksporPenggunaRequest_Filter(t *testing.T) {
	cari := func(s string) FilterPenggunaRequest { return FilterPenggunaRequest{Search: s} }
	kasus := map[string]struct {
		req    EksporPenggunaRequest
		search string
		ids    []string
	}{
		"kosong":                     {EksporPenggunaRequest{}, "", nil},
		"pencarian diteruskan":       {EksporPenggunaRequest{FilterPenggunaRequest: cari("budi")}, "budi", nil},
		"id dipisah koma":            {EksporPenggunaRequest{IDs: uid("a") + "," + uid("b") + "," + uid("c")}, "", []string{uid("a"), uid("b"), uid("c")}},
		"spasi, kosong, dan ganda":   {EksporPenggunaRequest{IDs: " " + uid("a") + " , ," + uid("b") + "," + uid("a") + ",," + uid("c") + ", "}, "", []string{uid("a"), uid("b"), uid("c")}},
		"hanya pemisah":              {EksporPenggunaRequest{IDs: ", ,,"}, "", nil},
		"huruf besar dinormalkan":    {EksporPenggunaRequest{IDs: strings.ToUpper(uid("a"))}, "", []string{uid("a")}},
		"yang bukan UUID dibuang":    {EksporPenggunaRequest{IDs: "superadmin," + uid("a") + ",u-1,123"}, "", []string{uid("a")}},
		"semuanya bukan UUID":        {EksporPenggunaRequest{IDs: "superadmin,u-1"}, "", nil},
		"pencarian dan id bersamaan": {EksporPenggunaRequest{FilterPenggunaRequest: cari("x"), IDs: uid("u1")}, "x", []string{uid("u1")}},
	}
	for nama, k := range kasus {
		t.Run(nama, func(t *testing.T) {
			f, err := k.req.Filter()
			require.NoError(t, err)
			assert.Equal(t, k.search, f.Cari)
			assert.Equal(t, k.ids, f.IDs)
		})
	}
}

// Ekspor menerima filter daftar yang sama (peran, status, lapis, mfa, terkunci) dan galat yang sama.
func TestEksporPenggunaRequest_FilterDaftarIkutDiteruskan(t *testing.T) {
	req := EksporPenggunaRequest{
		FilterPenggunaRequest: FilterPenggunaRequest{Peran: "ADMIN_SISTEM", Status: "AKTIF", Lapis: "SISTEM", MFA: "true", Terkunci: "false"},
		IDs:                   uid("u1"),
	}
	f, err := req.Filter()
	require.NoError(t, err)
	assert.Equal(t, "ADMIN_SISTEM", f.Peran)
	assert.Equal(t, "AKTIF", f.Status)
	assert.Equal(t, "SISTEM", f.Lapis)
	require.NotNil(t, f.MFA)
	assert.True(t, *f.MFA)
	require.NotNil(t, f.Terkunci)
	assert.False(t, *f.Terkunci)
	assert.Equal(t, []string{uid("u1")}, f.IDs)

	_, err = EksporPenggunaRequest{FilterPenggunaRequest: FilterPenggunaRequest{MFA: "1"}}.Filter()
	var salah *domain.FilterPenggunaError
	require.ErrorAs(t, err, &salah)
	assert.Equal(t, "mfa", salah.Param)
}
