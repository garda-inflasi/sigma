package domain

import (
	"regexp"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Titik pantau BMKG memakai kode adm4 Kemendagri, bukan kode BPS master. Urutan kabupaten keduanya berbeda, jadi
// awalan adm4 tidak boleh diturunkan dari kode master (itu tebakan lama yang memetakan cuaca ke kabupaten lain).
func TestWilayahBMKG_KodeKemendagriBukanTurunanKodeBPS(t *testing.T) {
	adm4 := regexp.MustCompile(`^11\.\d{2}\.\d{2}\.\d{4}$`)
	kodeMaster := map[string]string{}
	awalan := map[string]string{}
	for _, a := range wilayahBMKG {
		require.Regexp(t, adm4, a.Alias)
		kodeMaster[a.Kode] = a.Alias
		awalan[a.Alias[:5]] = a.Kode
	}
	assert.Len(t, wilayahBMKG, 23, "satu titik per kabupaten/kota Aceh")
	assert.Len(t, kodeMaster, 23, "tiap kode master tepat satu titik")
	assert.Len(t, awalan, 23, "tiap kabupaten/kota BMKG tepat satu titik")

	for kode, awal := range map[string]string{
		"1101": "11.09", // Simeulue
		"1102": "11.10", // Aceh Singkil
		"1103": "11.01", // Aceh Selatan
		"1110": "11.11", // Bireuen
		"1111": "11.08", // Aceh Utara
		"1114": "11.16", // Aceh Tamiang
		"1116": "11.14", // Aceh Jaya
		"1173": "11.74", // Kota Langsa
		"1174": "11.73", // Kota Lhokseumawe
	} {
		assert.Equal(t, awal, kodeMaster[kode][:5], "kode master %s", kode)
	}
}

func TestSemuaPemetaan_TitikBMKGBerlabelTerverifikasi(t *testing.T) {
	var n int
	for _, r := range SemuaPemetaan() {
		if r.SumberID == SumberBMKG && r.Jenis == JenisWilayah {
			n++
			require.NotNil(t, r.Catatan)
			assert.Equal(t, catatanTitikBMKG, *r.Catatan)
			assert.False(t, r.Diabaikan)
			require.NotNil(t, r.KodeMaster)
		}
	}
	assert.Equal(t, 23, n)
}
