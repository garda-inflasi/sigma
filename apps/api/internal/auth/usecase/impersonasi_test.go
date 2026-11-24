package usecase

import (
	"slices"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/garda-inflasi/api/internal/auth/domain"
)

func TestTerapkanIzinEksklusif(t *testing.T) {
	katalog := semuaIzinKatalog()

	// ADMIN_SISTEM selalu memegang seluruh katalog, terurut, tanpa duplikat, dan tanpa mengubah masukan.
	masuk := []string{"pengguna.kelola", "audit.lihat"}
	hasil := terapkanIzinEksklusif(domain.PeranSuperadmin, masuk)
	assert.ElementsMatch(t, katalog, hasil)
	assert.True(t, slices.IsSorted(hasil))
	assert.Equal(t, []string{"pengguna.kelola", "audit.lihat"}, masuk)
	assert.ElementsMatch(t, katalog, terapkanIzinEksklusif(domain.PeranSuperadmin, nil), "tabel kosong sekalipun")
	assert.ElementsMatch(t, katalog, terapkanIzinEksklusif(domain.PeranSuperadmin, katalog), "tidak digandakan")

	// Izin yang kelak ditambahkan ke katalog ikut dimiliki tanpa migrasi.
	asli := katalogIzin
	t.Cleanup(func() { katalogIzin = asli })
	katalogIzin = append(slices.Clone(asli), DefinisiIzin{Nama: "fitur.baru", Deskripsi: "uji", Kelompok: "Uji"})
	assert.Contains(t, terapkanIzinEksklusif(domain.PeranSuperadmin, masuk), "fitur.baru")
	assert.NotContains(t, terapkanIzinEksklusif("MANAJER_MUTU", []string{"kpi.lihat"}), "fitur.baru")
	katalogIzin = asli

	// Peran lain tidak pernah memiliki impersonasi.kelola, dari mana pun asalnya (matriks, izin langsung, SQL).
	assert.Equal(t, []string{"kpi.lihat"}, terapkanIzinEksklusif("MANAJER_MUTU", []string{"impersonasi.kelola", "kpi.lihat"}))
	assert.Empty(t, terapkanIzinEksklusif("OPS_MAHASISWA", []string{"impersonasi.kelola"}))
	assert.Equal(t, []string{"kpi.lihat"}, terapkanIzinEksklusif("MANAJER_MUTU", []string{"kpi.lihat"}))
}

// Sesi impersonasi memakai izin akun sasaran: pelaku ADMIN_SISTEM tidak membawa katalognya ke sesi itu.
func TestIzinImpersonasi_TidakMenaikkanHakSasaran(t *testing.T) {
	sasaran := []string{"harga.input", "tugas.lihat_sendiri"}
	assert.Equal(t, sasaran, terapkanIzinEksklusif("OPS_MAHASISWA", sasaran))
	for _, peran := range roleOrder {
		if peran == domain.PeranSuperadmin {
			continue
		}
		assert.NotContains(t, terapkanIzinEksklusif(peran, IzinBawaan(peran)), domain.IzinImpersonasi, peran)
		assert.ElementsMatch(t, IzinBawaan(peran), terapkanIzinEksklusif(peran, IzinBawaan(peran)), peran)
	}
}

func TestAturanSasaran_Mengizinkan(t *testing.T) {
	a := aturanSasaran{peran: []string{"OPS_MAHASISWA"}, penggunaID: []string{uid("akun-khusus")}}

	assert.True(t, a.mengizinkan(&domain.Pengguna{ID: uid("x"), Peran: "OPS_MAHASISWA"}), "menurut peran")
	assert.True(t, a.mengizinkan(&domain.Pengguna{ID: uid("akun-khusus"), Peran: "STRATEGIS_BI"}), "menurut akun")
	assert.False(t, a.mengizinkan(&domain.Pengguna{ID: uid("y"), Peran: "OPS_KOORDINATOR"}))
	assert.False(t, aturanSasaran{}.mengizinkan(&domain.Pengguna{ID: uid("x"), Peran: "OPS_MAHASISWA"}), "tanpa aturan tidak ada yang diizinkan")
}

func TestIzinImpersonasiAdaDiKatalogDanHanyaUntukAdminSistemDiMatriksBawaan(t *testing.T) {
	assert.True(t, IzinDikenal(domain.IzinImpersonasi))
	for _, peran := range roleOrder {
		assert.Equal(t, peran == domain.PeranSuperadmin, contains(IzinBawaan(peran), domain.IzinImpersonasi), peran)
	}
}

func contains(s []string, v string) bool {
	for _, x := range s {
		if x == v {
			return true
		}
	}
	return false
}
