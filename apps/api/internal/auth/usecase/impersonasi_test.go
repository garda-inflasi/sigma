package usecase

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/garda-inflasi/api/internal/auth/domain"
)

func TestTerapkanIzinEksklusif(t *testing.T) {
	// ADMIN_SISTEM selalu memilikinya, terurut dan tanpa mengubah masukan.
	masuk := []string{"pengguna.kelola", "audit.lihat"}
	assert.Equal(t, []string{"audit.lihat", "impersonasi.kelola", "pengguna.kelola"}, terapkanIzinEksklusif(domain.PeranSuperadmin, masuk))
	assert.Equal(t, []string{"pengguna.kelola", "audit.lihat"}, masuk)

	// Sudah ada: tidak digandakan.
	assert.Equal(t, []string{"impersonasi.kelola"}, terapkanIzinEksklusif(domain.PeranSuperadmin, []string{"impersonasi.kelola"}))

	// Peran lain tidak pernah memilikinya, dari mana pun asalnya (matriks, izin langsung, SQL).
	assert.Equal(t, []string{"kpi.lihat"}, terapkanIzinEksklusif("MANAJER_MUTU", []string{"impersonasi.kelola", "kpi.lihat"}))
	assert.Empty(t, terapkanIzinEksklusif("OPS_MAHASISWA", []string{"impersonasi.kelola"}))
	assert.Equal(t, []string{"kpi.lihat"}, terapkanIzinEksklusif("MANAJER_MUTU", []string{"kpi.lihat"}))
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
