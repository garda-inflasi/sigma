package openweather_test

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/garda-inflasi/api/internal/ingestion/connector/openweather"
	"github.com/garda-inflasi/api/internal/ingestion/domain"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func pemetaanUji() *domain.Pemetaan {
	return domain.NewPemetaan([]domain.PemetaanSumber{
		{Jenis: domain.JenisWilayah, Alias: "5.55,95.32", KodeMaster: new("1171")},
		{Jenis: domain.JenisWilayah, Alias: "lhokseumawe", KodeMaster: new("1174")},
		{Jenis: domain.JenisWilayah, Alias: "diabaikan", Diabaikan: true},
	}, nil)
}

func fixture(t *testing.T, nama string) []byte {
	t.Helper()
	b, err := os.ReadFile(filepath.Join("testdata", nama))
	require.NoError(t, err)
	return b
}

func TestPermintaan_ValidasiKunciDanURL(t *testing.T) {
	k := openweather.New("")
	// Tanpa URL
	_, err := k.Permintaan(domain.Masukan{})
	assert.ErrorContains(t, err, "url_dasar")

	// Tanpa Kunci
	_, err = k.Permintaan(domain.Masukan{URLDasar: "https://api.openweathermap.org/data/2.5"})
	assert.ErrorContains(t, err, "kunci API")

	// Tanpa Pemetaan Wilayah
	k = openweather.New("rahasia-123")
	_, err = k.Permintaan(domain.Masukan{
		URLDasar: "https://api.openweathermap.org/data/2.5",
		Pemetaan: domain.NewPemetaan(nil, nil),
	})
	assert.ErrorContains(t, err, "pemetaan_sumber WILAYAH")
}

func TestPermintaan_KoordinatDanNamaWilayah(t *testing.T) {
	k := openweather.New("rahasia-123")
	in := domain.Masukan{
		URLDasar: "https://api.openweathermap.org/data/2.5",
		Pemetaan: pemetaanUji(),
	}

	reqs, err := k.Permintaan(in)
	require.NoError(t, err)
	assert.Len(t, reqs, 2)

	assert.Equal(t, "wilayah=5.55,95.32", reqs[0].Kunci)
	assert.Contains(t, reqs[0].URL, "lat=5.55")
	assert.Contains(t, reqs[0].URL, "lon=95.32")
	assert.Contains(t, reqs[0].URL, "appid=rahasia-123")
	assert.Equal(t, []string{"rahasia-123"}, reqs[0].Samarkan)

	assert.Equal(t, "wilayah=lhokseumawe", reqs[1].Kunci)
	assert.Contains(t, reqs[1].URL, "q=lhokseumawe")
}

func TestNormalisasi_FixtureBandaAceh(t *testing.T) {
	k := openweather.New("test-key")
	konten := fixture(t, "cuaca_banda_aceh.json")

	in := domain.Masukan{
		SumberID: "src-uuid-owm",
		Sekarang: time.Date(2025, 10, 5, 12, 0, 0, 0, time.UTC),
		Pemetaan: pemetaanUji(),
	}

	dok := domain.Dokumen{
		Kunci:  "wilayah=5.55,95.32",
		Konten: konten,
	}

	hasil, err := k.Normalisasi(in, dok)
	require.NoError(t, err)
	assert.Len(t, hasil.Cuaca, 1)

	c := hasil.Cuaca[0]
	assert.Equal(t, "1171", c.KabupatenKode)
	assert.Equal(t, 28.5, c.SuhuC)
	assert.Equal(t, 82, c.Kelembapan)
	assert.Equal(t, 15.5, c.CurahHujanMm)
	assert.False(t, c.Anomali)
	assert.Equal(t, openweather.Sumber, c.Sumber)
}

func TestNormalisasi_TakTerpetakanDanDiabaikan(t *testing.T) {
	k := openweather.New("test-key")
	konten := fixture(t, "cuaca_banda_aceh.json")
	in := domain.Masukan{
		Pemetaan: pemetaanUji(),
	}

	// Diabaikan
	hasil, err := k.Normalisasi(in, domain.Dokumen{Kunci: "wilayah=diabaikan", Konten: konten})
	require.NoError(t, err)
	assert.Empty(t, hasil.Cuaca)
	assert.Equal(t, 0, hasil.JumlahTakTerpetakan())

	// Tak terpetakan
	hasil, err = k.Normalisasi(in, domain.Dokumen{Kunci: "wilayah=tak-dikenal", Konten: konten})
	require.NoError(t, err)
	assert.Empty(t, hasil.Cuaca)
	assert.Equal(t, 1, hasil.JumlahTakTerpetakan())
}
