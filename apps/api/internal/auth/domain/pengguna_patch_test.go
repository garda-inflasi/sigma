package domain

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func str(s string) *string { return &s }

func TestPenggunaPatch_KolomHanyaYangDiubah(t *testing.T) {
	assert.True(t, PenggunaPatch{}.Kosong())
	assert.Empty(t, PenggunaPatch{}.Kolom())

	p := PenggunaPatch{Nama: str("N"), Peran: str("MANAJER_DATA"), Lapis: str("MANAJERIAL"), KabupatenKode: str("1171"), Status: str("NONAKTIF")}

	assert.False(t, p.Kosong())
	assert.Equal(t, map[string]any{
		"nama": "N", "peran": "MANAJER_DATA", "lapis": "MANAJERIAL", "kabupaten_kode": "1171", "status": "NONAKTIF",
	}, p.Kolom(), "kolom lain (email, lembaga, kata sandi, MFA) tidak ikut ditulis")
}

func TestPenggunaPatch_NilaiKosongTetapDitulisBilaDiminta(t *testing.T) {
	// Pointer ke string kosong berarti "kosongkan kolom ini"; hanya nil yang berarti "jangan sentuh".
	p := PenggunaPatch{Lembaga: str("")}

	assert.False(t, p.Kosong())
	assert.Equal(t, map[string]any{"lembaga": ""}, p.Kolom())
}

func TestPenggunaPatch_TerapkanHanyaMengubahKolomPatchDanMenyalinKodeKabupaten(t *testing.T) {
	kode := "1171"
	u := &Pengguna{Nama: "Lama", Email: "a@b.c", Peran: "OPS_MAHASISWA", Lapis: "OPS", Lembaga: "X", Status: "AKTIF"}

	PenggunaPatch{Nama: str("Baru"), KabupatenKode: &kode, Status: str("NONAKTIF")}.Terapkan(u)
	kode = "9999" // mengubah sumber tidak boleh mengubah hasil

	assert.Equal(t, "Baru", u.Nama)
	assert.Equal(t, "NONAKTIF", u.Status)
	assert.Equal(t, "a@b.c", u.Email)
	assert.Equal(t, "OPS_MAHASISWA", u.Peran)
	assert.Equal(t, "X", u.Lembaga)
	if assert.NotNil(t, u.KabupatenKode) {
		assert.Equal(t, "1171", *u.KabupatenKode)
	}
}
