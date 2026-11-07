package bmkg

import (
	"net/url"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/garda-inflasi/api/internal/ingestion/domain"
	"github.com/garda-inflasi/api/pkg/idx"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// idSumber adalah UUID baris sumber_data konektor ini (garda_uuid_stabil dari kode), sama dengan seed.
var idSumber = idx.UUIDStabil("sumber_data", Kode)

func fixture(t *testing.T) []byte {
	t.Helper()
	raw, err := os.ReadFile(filepath.Join("testdata", "prakiraan_banda_aceh.json"))
	require.NoError(t, err)
	return raw
}

func masukan() domain.Masukan {
	return domain.Masukan{
		URLDasar: "https://api.bmkg.go.id/",
		Pemetaan: domain.NewPemetaan([]domain.PemetaanSumber{
			{SumberID: idSumber, Jenis: domain.JenisWilayah, Alias: "11.71.01.2001", KodeMaster: new("1171")},
			{SumberID: idSumber, Jenis: domain.JenisWilayah, Alias: "11.74.01.2001", KodeMaster: new("1174")},
		}, nil),
	}
}

func TestPermintaan_SatuPermintaanPerTitikPantau(t *testing.T) {
	reqs, err := New().Permintaan(masukan())
	require.NoError(t, err)
	require.Len(t, reqs, 2)
	assert.Equal(t, "wilayah=11.71.01.2001", reqs[0].Kunci)
	u, err := url.Parse(reqs[0].URL)
	require.NoError(t, err)
	assert.Equal(t, "api.bmkg.go.id", u.Host)
	assert.Equal(t, "/publik/prakiraan-cuaca", u.Path)
	assert.Equal(t, "11.71.01.2001", u.Query().Get("adm4"))
}

func TestPermintaan_MenolakBahanTakLengkap(t *testing.T) {
	in := masukan()
	in.URLDasar = ""
	_, err := New().Permintaan(in)
	assert.ErrorContains(t, err, "url_dasar")

	in = masukan()
	in.Pemetaan = domain.NewPemetaan(nil, nil)
	_, err = New().Permintaan(in)
	assert.ErrorContains(t, err, "pemetaan_sumber WILAYAH")
}

func TestNormalisasi_FixtureMeringkasPerHariDanMenolakHariParsial(t *testing.T) {
	hasil, err := New().Normalisasi(masukan(), domain.Dokumen{Kunci: "wilayah=11.71.01.2001", Konten: fixture(t)})
	require.NoError(t, err)

	require.Len(t, hasil.Cuaca, 2, "dua hari penuh; hari ketiga baru 5 dari 8 slot")
	assert.Equal(t, map[string]int{alasanHariTakLengkap: 1}, hasil.TidakValid, "hari parsial dilaporkan, bukan dibuang diam-diam")

	cerah := hasil.Cuaca[0]
	assert.Equal(t, "1171", cerah.KabupatenKode)
	assert.Equal(t, Sumber, cerah.Sumber)
	assert.True(t, time.Date(2025, 10, 15, 0, 0, 0, 0, time.UTC).Equal(cerah.Tanggal))
	assert.InDelta(t, 2.0, cerah.CurahHujanMm, 1e-9)
	assert.InDelta(t, 27.8, cerah.SuhuC, 1e-9)
	assert.Equal(t, 81, cerah.Kelembapan)
	assert.False(t, cerah.Anomali)

	lebat := hasil.Cuaca[1]
	assert.True(t, time.Date(2025, 10, 16, 0, 0, 0, 0, time.UTC).Equal(lebat.Tanggal))
	assert.InDelta(t, 60.0, lebat.CurahHujanMm, 1e-9)
	assert.InDelta(t, 25.8, lebat.SuhuC, 1e-9)
	assert.Equal(t, 92, lebat.Kelembapan)
	assert.True(t, lebat.Anomali, ">= 50 mm/hari ditandai anomali")
}

func TestNormalisasi_NilaiRusakMenolakHariItu(t *testing.T) {
	konten := []byte(`{"data":[{"cuaca":[[` + slotJSON(0, `"27"`, `"abc"`, `80`) + `,` + slotJSON(3, `27`, `0`, `120`) + `]]}]}`)

	hasil, err := New().Normalisasi(masukan(), domain.Dokumen{Kunci: "wilayah=11.71.01.2001", Konten: konten})

	require.NoError(t, err)
	assert.Empty(t, hasil.Cuaca)
	assert.Equal(t, 1, hasil.JumlahTidakValid())
}

func slotJSON(jam int, t, tp, hu string) string {
	return `{"local_datetime":"2025-10-15 0` + string(rune('0'+jam)) + `:00:00","t":` + t + `,"tp":` + tp + `,"hu":` + hu + `}`
}

func TestNormalisasi_TanggalDariDatetimeUTCDikonversiKeWIB(t *testing.T) {
	// 18:00 UTC tanggal 14 = 01:00 WIB tanggal 15.
	tgl, ok := tanggalLokal(slot{Datetime: "2025-10-14T18:00:00Z"})
	require.True(t, ok)
	assert.True(t, time.Date(2025, 10, 15, 0, 0, 0, 0, time.UTC).Equal(tgl))

	_, ok = tanggalLokal(slot{})
	assert.False(t, ok)
}

func TestNormalisasi_MenolakResponsTakDikenal(t *testing.T) {
	for nama, konten := range map[string][]byte{
		"bukan JSON objek": []byte(`[1]`),
		"tanpa data":       []byte(`{"data":[]}`),
	} {
		_, err := New().Normalisasi(masukan(), domain.Dokumen{Kunci: "wilayah=11.71.01.2001", Konten: konten})
		assert.Error(t, err, nama)
	}
	_, err := New().Normalisasi(masukan(), domain.Dokumen{Kunci: "wilayah=99.99", Konten: fixture(t)})
	assert.ErrorContains(t, err, "tidak terpetakan")
	_, err = New().Normalisasi(masukan(), domain.Dokumen{Kunci: "lain=1", Konten: fixture(t)})
	assert.Error(t, err)
}
