package bapanas_test

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/garda-inflasi/api/internal/ingestion/connector/bapanas"
	"github.com/garda-inflasi/api/internal/ingestion/domain"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func pemetaanUji() *domain.Pemetaan {
	satuan := map[string]string{
		"BRS-MED": "kg",
		"CBM-KRT": "kg",
		"BWM":     "kg",
	}
	return domain.NewPemetaan([]domain.PemetaanSumber{
		{Jenis: domain.JenisKomoditas, Alias: "beras medium", KodeMaster: new("BRS-MED")},
		{Jenis: domain.JenisKomoditas, Alias: "cabai merah keriting", KodeMaster: new("CBM-KRT")},
		{Jenis: domain.JenisKomoditas, Alias: "bawang merah", KodeMaster: new("BWM")},
		{Jenis: domain.JenisKomoditas, Alias: "minyak goreng curah", KodeMaster: new("MYK")},
		{Jenis: domain.JenisKomoditas, Alias: "beras premium", KodeMaster: new("BRS-PRM")},
		{Jenis: domain.JenisWilayah, Alias: "1171", KodeMaster: new("1171")},
		{Jenis: domain.JenisWilayah, Alias: "1174", KodeMaster: new("1174")},
		{Jenis: domain.JenisWilayah, Alias: "9999", Diabaikan: true},
	}, satuan)
}

func fixture(t *testing.T, nama string) []byte {
	t.Helper()
	b, err := os.ReadFile(filepath.Join("testdata", nama))
	require.NoError(t, err)
	return b
}

func TestPermintaan_ValidasiURL(t *testing.T) {
	k := bapanas.New()
	_, err := k.Permintaan(domain.Masukan{})
	assert.ErrorContains(t, err, "url_dasar")
}

func TestPermintaan_ProvinsiCustom(t *testing.T) {
	k := bapanas.New()
	in := domain.Masukan{
		URLDasar:    "https://panelharga.badanpangan.go.id",
		Konfigurasi: json.RawMessage(`{"provinsi_id":"12"}`),
	}

	reqs, err := k.Permintaan(in)
	require.NoError(t, err)
	require.Len(t, reqs, 1)
	assert.Contains(t, reqs[0].URL, "provinsi=12")
}

func TestNormalisasi_FixtureAceh(t *testing.T) {
	k := bapanas.New()
	konten := fixture(t, "harga_bapanas_aceh.json")

	in := domain.Masukan{
		SumberID: "src-uuid-bapanas",
		Sekarang: time.Date(2025, 10, 5, 12, 0, 0, 0, time.UTC),
		Pemetaan: pemetaanUji(),
	}

	dok := domain.Dokumen{
		Kunci:  "provinsi=11",
		Konten: konten,
	}

	hasil, err := k.Normalisasi(in, dok)
	require.NoError(t, err)

	// 3 baris sukses: Beras Medium, Cabai Merah Keriting, Bawang Merah
	assert.Len(t, hasil.Harga, 3)

	h0 := hasil.Harga[0]
	assert.Equal(t, "BRS-MED", h0.KomoditasKode)
	assert.Equal(t, "1171", h0.KabupatenKode)
	assert.Equal(t, 14500.0, h0.Harga)
	assert.Equal(t, "kg", h0.Satuan)
	assert.Equal(t, "KONSUMEN", h0.Tipe)

	h1 := hasil.Harga[1]
	assert.Equal(t, "CBM-KRT", h1.KomoditasKode)
	assert.Equal(t, 42000.0, h1.Harga)

	h2 := hasil.Harga[2]
	assert.Equal(t, "BWM", h2.KomoditasKode)
	assert.Equal(t, "1174", h2.KabupatenKode)
	assert.Equal(t, 35000.0, h2.Harga)

	// Komoditas Tidak Dikenal -> Tak Terpetakan
	assert.Equal(t, 1, hasil.JumlahTakTerpetakan())

	// Minyak Goreng Curah harga "-" -> Tidak Valid
	assert.Equal(t, 1, hasil.JumlahTidakValid())
}
