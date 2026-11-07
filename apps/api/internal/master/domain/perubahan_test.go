package domain

import (
	"encoding/json"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func raw(m map[string]string) map[string]json.RawMessage {
	out := map[string]json.RawMessage{}
	for k, v := range m {
		out[k] = json.RawMessage(v)
	}
	return out
}

func TestValidasiUsulan_NormalisasiNilai(t *testing.T) {
	isi, err := ValidasiUsulan(EntitasKomoditas, JenisUbah, "BWM", raw(map[string]string{
		"nama": `"  Bawang merah  "`, "harga_acuan": `42000`, "pilot": `true`,
	}))
	require.NoError(t, err)
	assert.Equal(t, map[string]any{"nama": "Bawang merah", "harga_acuan": int64(42000), "pilot": true}, isi)

	isi, err = ValidasiUsulan(EntitasPasar, JenisUbah, "PSR-1171-01", raw(map[string]string{
		"lat": `5.5`, "kecamatan_kode": `null`, "radius_meter": `400`,
	}))
	require.NoError(t, err)
	assert.Equal(t, map[string]any{"lat": 5.5, "kecamatan_kode": nil, "radius_meter": int64(400)}, isi)
}

func TestValidasiUsulan_Menolak(t *testing.T) {
	kasus := map[string]struct {
		entitas, jenis, kode string
		isi                  map[string]string
		galat                error
	}{
		"entitas lain":          {"kabupaten", JenisUbah, "1171", map[string]string{"nama": `"x"`}, ErrValidasi},
		"jenis lain":            {EntitasKomoditas, JenisTinjau, "BWM", map[string]string{"nama": `"x"`}, ErrValidasi},
		"bidang terlarang":      {EntitasKomoditas, JenisUbah, "BWM", map[string]string{"versi": `2`}, ErrValidasi},
		"pecahan":               {EntitasKomoditas, JenisUbah, "BWM", map[string]string{"harga_acuan": `1.5`}, ErrValidasi},
		"nol":                   {EntitasKomoditas, JenisUbah, "BWM", map[string]string{"harga_acuan": `0`}, ErrValidasi},
		"teks kosong":           {EntitasKomoditas, JenisUbah, "BWM", map[string]string{"nama": `"   "`}, ErrValidasi},
		"null bidang wajib":     {EntitasKomoditas, JenisUbah, "BWM", map[string]string{"nama": `null`}, ErrValidasi},
		"lintang di luar":       {EntitasPasar, JenisUbah, "P", map[string]string{"lat": `91`}, ErrValidasi},
		"kecamatan salah":       {EntitasPasar, JenisUbah, "P", map[string]string{"kecamatan_kode": `"117101"`}, ErrValidasi},
		"kabupaten saat ubah":   {EntitasPasar, JenisUbah, "P", map[string]string{"kabupaten_kode": `"1171"`}, ErrValidasi},
		"tambah tanpa wajib":    {EntitasKomoditas, JenisTambah, "BRU", map[string]string{"nama": `"x"`}, ErrValidasi},
		"kode baru huruf kecil": {EntitasKomoditas, JenisTambah, "bru", map[string]string{"nama": `"x"`}, ErrValidasi},
		"ubah kosong":           {EntitasKomoditas, JenisUbah, "BWM", map[string]string{}, ErrTanpaPerubahan},
	}
	for nama, k := range kasus {
		t.Run(nama, func(t *testing.T) {
			_, err := ValidasiUsulan(k.entitas, k.jenis, k.kode, raw(k.isi))
			assert.True(t, errors.Is(err, k.galat), "%v", err)
		})
	}
}

func TestValidasiUsulan_TambahPasarMengisiBawaan(t *testing.T) {
	isi, err := ValidasiUsulan(EntitasPasar, JenisTambah, "PSR-1171-09", raw(map[string]string{
		"nama": `"Pasar"`, "kabupaten_kode": `"1171"`, "lat": `5`, "lng": `95`,
	}))
	require.NoError(t, err)
	assert.Equal(t, int64(400), isi["radius_meter"])
	assert.Equal(t, true, isi["aktif"])
	assert.Equal(t, 5.0, isi["lat"], "lintang dibaca sebagai desimal walau ditulis bulat")
}

func TestDiffUsulan_UrutBidangDanNilaiSebelum(t *testing.T) {
	diff, err := DiffUsulan(PerubahanMaster{
		Entitas: EntitasKomoditas, Perubahan: `{"pilot": true, "nama": "B", "harga_acuan": 2}`, Sebelum: `{"nama": "A", "harga_acuan": 1}`,
	})
	require.NoError(t, err)
	require.Len(t, diff, 3)
	assert.Equal(t, []string{"nama", "harga_acuan", "pilot"}, []string{diff[0].Bidang, diff[1].Bidang, diff[2].Bidang})
	assert.JSONEq(t, `"A"`, string(diff[0].Sebelum))
	assert.Nil(t, diff[2].Sebelum, "bidang tanpa nilai sebelum")

	diff, err = DiffUsulan(PerubahanMaster{Entitas: EntitasKomoditas, Perubahan: "", Sebelum: ""})
	require.NoError(t, err)
	assert.Empty(t, diff)
}

func TestKonversiTersimpanDanSamaNilai(t *testing.T) {
	isi := KonversiTersimpan(EntitasPasar, map[string]any{
		"lat": json.Number("5"), "radius_meter": json.Number("400"), "nama": "P",
	})
	assert.Equal(t, 5.0, isi["lat"])
	assert.Equal(t, int64(400), isi["radius_meter"])
	assert.True(t, SamaNilai(json.Number("42000"), int64(42000)))
	assert.True(t, SamaNilai(json.Number("5.55"), 5.55))
	assert.True(t, SamaNilai(nil, nil))
	assert.False(t, SamaNilai("a", "b"))
}

func TestValidasiAlasan(t *testing.T) {
	_, err := ValidasiAlasan("  ", true)
	assert.ErrorIs(t, err, ErrValidasi)
	alasan, err := ValidasiAlasan("", false)
	require.NoError(t, err)
	assert.Empty(t, alasan)
	alasan, err = ValidasiAlasan("  cukup  ", true)
	require.NoError(t, err)
	assert.Equal(t, "cukup", alasan)
}
