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

func TestPenggunaPatch_TeleponDiisiLaluDihapusSebagaiNULL(t *testing.T) {
	isi := PenggunaPatch{Telepon: str("+6281234567890")}
	assert.False(t, isi.Kosong())
	assert.Equal(t, map[string]any{"telepon": "+6281234567890"}, isi.Kolom())

	// "" berarti hapus nomor: kolom ditulis NULL (nil), bukan string kosong, dan patch tidak dianggap kosong.
	hapus := PenggunaPatch{Telepon: str("")}
	assert.False(t, hapus.Kosong())
	kolom := hapus.Kolom()
	nilai, ada := kolom["telepon"]
	assert.True(t, ada, "kolom telepon harus ikut ditulis")
	assert.Nil(t, nilai, "hapus nomor menulis NULL")
	assert.Len(t, kolom, 1)
}

func TestPenggunaPatch_TerapkanTeleponMenyalinNilaiDanMenghapusBilaKosong(t *testing.T) {
	lama := "+6281200000001"
	u := &Pengguna{Nama: "N", Telepon: &lama}

	nomor := "+6285712345678"
	PenggunaPatch{Telepon: &nomor}.Terapkan(u)
	nomor = "+6280000000000" // mengubah sumber tidak boleh mengubah hasil
	if assert.NotNil(t, u.Telepon) {
		assert.Equal(t, "+6285712345678", *u.Telepon)
	}
	assert.Equal(t, "+6281200000001", lama, "nilai lama pemanggil tidak ikut berubah")

	PenggunaPatch{Telepon: str("")}.Terapkan(u)
	assert.Nil(t, u.Telepon, "hapus nomor mengembalikan nil, bukan pointer ke string kosong")

	PenggunaPatch{Nama: str("Lain")}.Terapkan(u)
	assert.Nil(t, u.Telepon, "patch tanpa telepon tidak menyentuhnya")
}
