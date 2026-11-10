package domain_test

import (
	"testing"

	"github.com/garda-inflasi/api/internal/dashboard/domain"
	harga "github.com/garda-inflasi/api/internal/harga/domain"
	"github.com/stretchr/testify/assert"
)

func TestCakupan_NilaiNolTertutup(t *testing.T) {
	var c domain.Cakupan
	assert.True(t, c.Tertutup(), "cakupan yang lupa diisi tidak membuka data")
	for _, filter := range []string{"", "1171"} {
		kab, ada := c.Irisi(filter)
		assert.False(t, ada, "filter %q", filter)
		assert.Empty(t, kab)
	}
	assert.True(t, domain.CakupanKabupaten("").Tertutup(), "kabupaten kosong bukan seluruh Aceh")
}

func TestCakupan_Irisi(t *testing.T) {
	tabel := []struct {
		nama    string
		cakupan domain.Cakupan
		filter  string
		wantKab string
		wantAda bool
	}{
		{"semua tanpa filter: seluruh Aceh", domain.CakupanSemua(), "", "", true},
		{"semua dengan filter: kabupaten itu", domain.CakupanSemua(), "1108", "1108", true},
		{"kabupaten tanpa filter: kabupatennya", domain.CakupanKabupaten("1171"), "", "1171", true},
		{"kabupaten dengan filter yang sama", domain.CakupanKabupaten("1171"), "1171", "1171", true},
		{"kabupaten dengan filter kabupaten lain: kosong", domain.CakupanKabupaten("1171"), "1108", "", false},
		{"tertutup", domain.Cakupan{}, "", "", false},
		// Semua menang atas KabupatenKode yang tersisa: cakupan Semua memang seluruh Aceh.
		{"semua dengan kabupaten tersisa tetap seluruh Aceh", domain.Cakupan{Semua: true, KabupatenKode: "1171"}, "", "", true},
	}
	for _, k := range tabel {
		kab, ada := k.cakupan.Irisi(k.filter)
		assert.Equal(t, k.wantAda, ada, k.nama)
		assert.Equal(t, k.wantKab, kab, k.nama)
	}
}

func TestCakupanPeran(t *testing.T) {
	const u = "uuid-pengguna"
	assert.Equal(t, domain.CakupanKabupaten("1171"), domain.CakupanPeran(harga.PeranKoordinator, "1171", u))
	assert.Equal(t, domain.CakupanKabupaten("1108"), domain.CakupanPeran(harga.PeranAgregator, "1108", u))

	for _, peran := range []string{"STRATEGIS_FMIPA", "STRATEGIS_BI", "STRATEGIS_BPS", "STRATEGIS_DJPB",
		"MANAJER_PROGRAM", "MANAJER_DATA", "MANAJER_KEBIJAKAN", "MANAJER_PEMBIAYAAN", "MANAJER_RISIKO", "MANAJER_MUTU", "ADMIN_SISTEM"} {
		assert.Equal(t, domain.CakupanSemua(), domain.CakupanPeran(peran, "", u), peran)
		// Kabupaten yang tersisa di akun tingkat provinsi tidak mempersempit cakupan.
		assert.Equal(t, domain.CakupanSemua(), domain.CakupanPeran(peran, "1171", u), peran)
	}

	t.Run("koordinator dan agregator tanpa kabupaten tugas tertutup, bukan seluruh Aceh", func(t *testing.T) {
		for _, peran := range []string{harga.PeranKoordinator, harga.PeranAgregator} {
			c := domain.CakupanPeran(peran, "", u)
			assert.True(t, c.Tertutup(), peran)
			assert.False(t, c.Semua, peran)
		}
	})

	t.Run("mahasiswa dan peran yang belum dikenal tertutup", func(t *testing.T) {
		for _, peran := range []string{harga.PeranMahasiswa, "PERAN_BARU", ""} {
			assert.True(t, domain.CakupanPeran(peran, "1171", u).Tertutup(), "peran %q walau punya kabupaten", peran)
		}
	})
}

// Aturan peran tidak boleh bercabang dari modul harga: untuk setiap peran yang dikenal harga, batas wilayah cakupan
// dashboard sama persis dengan cakupan harga (Semua, atau kabupaten yang sama).
func TestCakupanPeran_SelarasDenganModulHarga(t *testing.T) {
	const u = "uuid-pengguna"
	peran := []string{harga.PeranKoordinator, harga.PeranAgregator, harga.PeranMahasiswa, "PERAN_BARU", "",
		"STRATEGIS_BI", "MANAJER_DATA", "ADMIN_SISTEM"}
	for _, p := range peran {
		for _, kab := range []string{"", "1171"} {
			h := harga.CakupanPeran(p, kab, u)
			d := domain.CakupanPeran(p, kab, u)
			assert.Equal(t, h.Semua, d.Semua, "%q/%q", p, kab)
			assert.Equal(t, h.KabupatenKode, d.KabupatenKode, "%q/%q", p, kab)
		}
	}
}
