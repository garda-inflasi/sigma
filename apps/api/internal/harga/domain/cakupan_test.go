package domain_test

import (
	"testing"

	"github.com/garda-inflasi/api/internal/harga/domain"
	"github.com/stretchr/testify/assert"
)

func ptr(s string) *string { return &s }

func TestCakupan_NilaiNolTertutup(t *testing.T) {
	var c domain.Cakupan
	assert.False(t, c.Mengizinkan("1171", ptr("u1")), "cakupan yang lupa diisi tidak membuka data")
	assert.False(t, c.Mengizinkan("", nil))
}

func TestCakupan_Mengizinkan(t *testing.T) {
	tabel := []struct {
		nama      string
		cakupan   domain.Cakupan
		kabupaten string
		petugas   *string
		want      bool
	}{
		{"semua", domain.CakupanSemua(), "1108", ptr("lain"), true},
		{"semua, tanpa petugas", domain.CakupanSemua(), "1108", nil, true},
		{"kabupaten cocok", domain.Cakupan{KabupatenKode: "1171"}, "1171", ptr("lain"), true},
		{"kabupaten beda", domain.Cakupan{KabupatenKode: "1171"}, "1108", ptr("lain"), false},
		{"milik sendiri di kabupaten lain", domain.Cakupan{KabupatenKode: "1171", PemilikID: "u1"}, "1108", ptr("u1"), true},
		{"milik orang lain di kabupaten lain", domain.Cakupan{KabupatenKode: "1171", PemilikID: "u1"}, "1108", ptr("u2"), false},
		{"milik orang lain di kabupaten sendiri", domain.Cakupan{KabupatenKode: "1171", PemilikID: "u1"}, "1171", ptr("u2"), true},
		{"hanya milik: entri sendiri", domain.CakupanMilik("u1"), "1108", ptr("u1"), true},
		{"hanya milik: entri orang lain", domain.CakupanMilik("u1"), "1171", ptr("u2"), false},
		{"hanya milik: entri tanpa petugas", domain.CakupanMilik("u1"), "1171", nil, false},
		{"kabupaten kosong tidak cocok dengan pasar kosong", domain.Cakupan{PemilikID: "u1"}, "", ptr("u2"), false},
	}
	for _, k := range tabel {
		assert.Equal(t, k.want, k.cakupan.Mengizinkan(k.kabupaten, k.petugas), k.nama)
	}
}

func TestCakupanPeran(t *testing.T) {
	const u = "uuid-pengguna"
	assert.Equal(t, domain.Cakupan{KabupatenKode: "1171", PemilikID: u}, domain.CakupanPeran(domain.PeranKoordinator, "1171", u))
	assert.Equal(t, domain.Cakupan{KabupatenKode: "1108", PemilikID: u}, domain.CakupanPeran(domain.PeranAgregator, "1108", u))
	assert.Equal(t, domain.CakupanMilik(u), domain.CakupanPeran(domain.PeranMahasiswa, "1171", u), "mahasiswa tidak dibatasi kabupaten, hanya entrinya")

	for _, peran := range []string{"STRATEGIS_FMIPA", "STRATEGIS_BI", "STRATEGIS_BPS", "STRATEGIS_DJPB",
		"MANAJER_PROGRAM", "MANAJER_DATA", "MANAJER_KEBIJAKAN", "MANAJER_PEMBIAYAAN", "MANAJER_RISIKO", "MANAJER_MUTU", "ADMIN_SISTEM"} {
		assert.Equal(t, domain.CakupanSemua(), domain.CakupanPeran(peran, "", u), peran)
		assert.True(t, domain.PeranDikenal(peran), peran)
	}

	t.Run("koordinator tanpa kabupaten tugas: hanya entri miliknya", func(t *testing.T) {
		c := domain.CakupanPeran(domain.PeranKoordinator, "", u)
		assert.Equal(t, domain.CakupanMilik(u), c)
		assert.False(t, c.Mengizinkan("1171", ptr("lain")), "bukan seluruh Aceh dan bukan 'kabupaten kosong'")
	})

	t.Run("peran yang belum dikenal tidak melihat seluruh Aceh", func(t *testing.T) {
		assert.Equal(t, domain.CakupanMilik(u), domain.CakupanPeran("PERAN_BARU", "", u))
		assert.Equal(t, domain.CakupanMilik(u), domain.CakupanPeran("", "", u))
		assert.False(t, domain.PeranDikenal("PERAN_BARU"))
		assert.False(t, domain.PeranDikenal(""))
	})
}
