package pihps

import (
	"encoding/json"
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

var sekarang = time.Date(2025, 10, 15, 10, 0, 0, 0, time.UTC)

func fixture(t *testing.T, nama string) []byte {
	t.Helper()
	raw, err := os.ReadFile(filepath.Join("testdata", nama))
	require.NoError(t, err)
	return raw
}

// masukan memakai pemetaan referensi seeder ditambah satu wilayah uji (regency_id 101 = 1171).
func masukan(t *testing.T, konf string) domain.Masukan {
	t.Helper()
	rows := domain.SemuaPemetaan()
	rows = append(rows, domain.PemetaanSumber{SumberID: idSumber, Jenis: domain.JenisWilayah, Alias: "101", KodeMaster: new("1171")})
	satuan := map[string]string{
		"BRS-MED": "kg", "BRS-PRM": "kg", "CBM-KRT": "kg", "CBR-RWT": "kg", "BWM": "kg", "BWP": "kg",
		"TLR": "kg", "DAG-AYM": "kg", "DAG-SPI": "kg", "MYK": "liter", "GLA": "kg",
	}
	var sumberBI []domain.PemetaanSumber
	for _, r := range rows {
		if r.SumberID == idSumber {
			sumberBI = append(sumberBI, r)
		}
	}
	return domain.Masukan{
		SumberID: idSumber, Sekarang: sekarang, URLDasar: "https://www.bi.go.id/hargapangan/",
		Konfigurasi: json.RawMessage(konf), Pemetaan: domain.NewPemetaan(sumberBI, satuan),
	}
}

func TestPermintaan_MerancangSatuPermintaanPerWilayah(t *testing.T) {
	reqs, err := New().Permintaan(masukan(t, `{"province_id": 7, "hari": 3}`))
	require.NoError(t, err)
	require.Len(t, reqs, 1)
	assert.Equal(t, "wilayah=101", reqs[0].Kunci)

	u, err := url.Parse(reqs[0].URL)
	require.NoError(t, err)
	assert.Equal(t, "www.bi.go.id", u.Host)
	assert.Equal(t, "/hargapangan/WebSite/TabelHarga/GetGridDataDaerah", u.Path)
	q := u.Query()
	assert.Equal(t, "7", q.Get("province_id"))
	assert.Equal(t, "101", q.Get("regency_id"))
	assert.Equal(t, "1", q.Get("price_type_id"))
	assert.Equal(t, "2025-10-13", q.Get("start_date"))
	assert.Equal(t, "2025-10-15", q.Get("end_date"))
}

func TestPermintaan_MenolakKonfigurasiTakLengkap(t *testing.T) {
	cases := map[string]string{
		"province_id kosong": `{}`,
		"hari di luar batas": `{"province_id":"7","hari":99}`,
		"json rusak":         `{"province_id":`,
	}
	for nama, konf := range cases {
		_, err := New().Permintaan(masukan(t, konf))
		assert.Error(t, err, nama)
	}

	in := masukan(t, `{"province_id":"7"}`)
	in.URLDasar = ""
	_, err := New().Permintaan(in)
	assert.ErrorContains(t, err, "url_dasar")

	in = masukan(t, `{"province_id":"7"}`)
	in.Pemetaan = domain.NewPemetaan(nil, nil)
	_, err = New().Permintaan(in)
	assert.ErrorContains(t, err, "pemetaan_sumber WILAYAH")
}

func TestNormalisasi_FixtureGridKota(t *testing.T) {
	in := masukan(t, `{"province_id":"7"}`)
	hasil, err := New().Normalisasi(in, domain.Dokumen{Kunci: "wilayah=101", Konten: fixture(t, "grid_kota.json")})
	require.NoError(t, err)

	// Baris induk kategori dan varian yang diabaikan tidak dihitung sebagai ditolak.
	assert.Len(t, hasil.Harga, 14)
	assert.Equal(t, 3, hasil.TakTerpetakan["KOMODITAS:ikan segar baru"], "alias tak terpetakan dilaporkan, bukan dibuang diam-diam")
	assert.Equal(t, 3, hasil.JumlahTakTerpetakan())
	assert.Equal(t, map[string]int{
		"harga kosong atau bukan angka": 3, // "-", null, "abc"
		"harga di luar batas wajar":     1, // 999.999.999
	}, hasil.TidakValid)

	harga := map[string]float64{}
	for _, h := range hasil.Harga {
		assert.Equal(t, idSumber, h.SumberID)
		assert.Equal(t, "1171", h.KabupatenKode)
		assert.Equal(t, domain.TipeHargaKonsumen, h.Tipe)
		assert.NotEmpty(t, h.Satuan)
		harga[h.KomoditasKode+"@"+h.Tanggal.Format(time.DateOnly)] = h.Harga
	}
	assert.Equal(t, 13900.0, harga["BRS-MED@2025-10-13"])
	assert.NotContains(t, harga, "BRS-MED@2025-10-15", "tanda '-' berarti tidak ada harga")
	assert.Equal(t, 15850.0, harga["BRS-PRM@2025-10-15"], "alias dinormalisasi: spasi ganda dan huruf besar")
	assert.Equal(t, 54500.0, harga["CBM-KRT@2025-10-14"], "angka JSON")
	assert.Equal(t, 55000.0, harga["CBM-KRT@2025-10-15"], "awalan Rp")
	assert.Equal(t, "liter", hasilSatuan(hasil, "MYK"))
}

func TestNormalisasi_DeterministikDanBentukAlternatif(t *testing.T) {
	in := masukan(t, `{"province_id":"7"}`)
	a, err := New().Normalisasi(in, domain.Dokumen{Kunci: "wilayah=101", Konten: fixture(t, "grid_kota.json")})
	require.NoError(t, err)
	b, err := New().Normalisasi(in, domain.Dokumen{Kunci: "wilayah=101", Konten: fixture(t, "grid_kota.json")})
	require.NoError(t, err)
	assert.Equal(t, a.Harga, b.Harga, "urutan baris tidak bergantung pada iterasi peta")

	// Larik di puncak (tanpa amplop "data") juga diterima.
	larik := []byte(`[{"name":"Minyak Goreng Curah","2025-10-15":"17.600"}]`)
	hasil, err := New().Normalisasi(in, domain.Dokumen{Kunci: "wilayah=101", Konten: larik})
	require.NoError(t, err)
	require.Len(t, hasil.Harga, 1)
	assert.Equal(t, 17600.0, hasil.Harga[0].Harga)
}

func TestNormalisasi_MenolakResponsTakDikenal(t *testing.T) {
	in := masukan(t, `{"province_id":"7"}`)
	_, err := New().Normalisasi(in, domain.Dokumen{Kunci: "wilayah=101", Konten: []byte(`{"error":"x"}`)})
	assert.Error(t, err)
	_, err = New().Normalisasi(in, domain.Dokumen{Kunci: "wilayah=999", Konten: fixture(t, "grid_kota.json")})
	assert.ErrorContains(t, err, "tidak terpetakan")
	_, err = New().Normalisasi(in, domain.Dokumen{Kunci: "lain=1", Konten: fixture(t, "grid_kota.json")})
	assert.Error(t, err)
}

func hasilSatuan(h domain.Hasil, kode string) string {
	for _, x := range h.Harga {
		if x.KomoditasKode == kode {
			return x.Satuan
		}
	}
	return ""
}

func TestPeriksaKonfigurasi(t *testing.T) {
	k := New()
	assert.ElementsMatch(t, []string{"province_id", "hari", "price_type_id", "comcat_id"}, k.KunciKonfigurasi())

	assert.NoError(t, k.PeriksaKonfigurasi(json.RawMessage(`{"province_id":"7"}`)))
	assert.NoError(t, k.PeriksaKonfigurasi(json.RawMessage(`{"province_id":7,"hari":31}`)), "angka JSON diterima, seperti Permintaan")

	for nama, konf := range map[string]string{
		"belum diisi":       `{}`,
		"bukan angka":       `{"province_id":"Aceh"}`,
		"id berspasi":       `{"province_id":" 7"}`,
		"hari di luar":      `{"province_id":"7","hari":32}`,
		"hari negatif":      `{"province_id":"7","hari":-1}`,
		"json rusak":        `{"province_id":`,
		"tipe tak didukung": `{"province_id":[7]}`,
	} {
		assert.Error(t, k.PeriksaKonfigurasi(json.RawMessage(konf)), nama)
		// Permintaan menolak hal yang sama: pemeriksa tidak boleh lebih longgar daripada run.
		_, err := k.Permintaan(masukan(t, konf))
		assert.Error(t, err, "Permintaan "+nama)
	}
	assert.ErrorContains(t, k.PeriksaKonfigurasi(json.RawMessage(`{"province_id":"Aceh"}`)), "bukan id angka")
}
