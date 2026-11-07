package bps

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/garda-inflasi/api/internal/ingestion/domain"
	"github.com/garda-inflasi/api/pkg/idx"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// idSumber adalah UUID baris sumber_data konektor ini (garda_uuid_stabil dari kode), sama dengan seed.
var idSumber = idx.UUIDStabil("sumber_data", Kode)

const kunciRahasia = "RAHASIA-API-BPS"

var sekarang = time.Date(2025, 10, 5, 8, 0, 0, 0, time.UTC)

const konfigLengkap = `{"domain":"1100","variabel":{"ihk":"10","inflasi_mtm":"11","inflasi_yoy":"12","inflasi_ytd":"13","andil":"20","andil_inflasi_pct":"21"}}`

func pemetaan() *domain.Pemetaan {
	wilayah := func(alias, kode string) domain.PemetaanSumber {
		return domain.PemetaanSumber{SumberID: idSumber, Jenis: domain.JenisWilayah, Alias: alias, KodeMaster: &kode}
	}
	komoditas := func(alias, kode string) domain.PemetaanSumber {
		return domain.PemetaanSumber{SumberID: idSumber, Jenis: domain.JenisKomoditas, Alias: alias, KodeMaster: &kode}
	}
	return domain.NewPemetaan([]domain.PemetaanSumber{
		wilayah("aceh", "11"), wilayah("banda aceh", "1171"), wilayah("lhokseumawe", "1174"),
		komoditas("cabai merah", "CBM-KRT"), komoditas("bawang merah", "BWM"),
	}, nil)
}

func masukan(konf string) domain.Masukan {
	return domain.Masukan{Sekarang: sekarang, URLDasar: "https://webapi.bps.go.id/v1/api/", Konfigurasi: json.RawMessage(konf), Pemetaan: pemetaan()}
}

func dokumen(t *testing.T) []domain.Dokumen {
	t.Helper()
	var dok []domain.Dokumen
	for _, nama := range []string{"ihk", "inflasi_mtm", "inflasi_yoy", "inflasi_ytd", "andil", "andil_inflasi_pct"} {
		berkas := nama + "_2025.json"
		raw, err := os.ReadFile(filepath.Join("testdata", berkas))
		require.NoError(t, err)
		dok = append(dok, domain.Dokumen{Kunci: "var=" + nama + "&th=2025", Konten: raw})
	}
	return dok
}

func TestPermintaan_SatuPermintaanPerMetrikDanTahun(t *testing.T) {
	reqs, err := New(kunciRahasia).Permintaan(masukan(konfigLengkap))
	require.NoError(t, err)

	require.Len(t, reqs, 12, "6 metrik x tahun berjalan dan tahun sebelumnya")
	assert.Equal(t, "var=ihk&th=2024", reqs[0].Kunci)
	assert.Equal(t, "https://webapi.bps.go.id/v1/api/list/model/data/lang/ind/domain/1100/var/10/th/124/key/"+kunciRahasia, reqs[0].URL)
	assert.Equal(t, []string{kunciRahasia}, reqs[0].Samarkan)
	for _, r := range reqs {
		assert.NotContains(t, r.Kunci, kunciRahasia, "kunci permintaan masuk bronze dan log")
	}
}

func TestPermintaan_TanpaAndilDanTahunSurutNol(t *testing.T) {
	reqs, err := New(kunciRahasia).Permintaan(masukan(`{"domain":"1100","tahun_surut":0,"variabel":{"ihk":"10","inflasi_mtm":"11","inflasi_yoy":"12","inflasi_ytd":"13"}}`))
	require.NoError(t, err)
	assert.Len(t, reqs, 4)
}

func TestPermintaan_MenolakKonfigurasiDanKredensialTakLengkap(t *testing.T) {
	kasus := map[string]struct {
		konektor *Konektor
		konf     string
		pesan    string
	}{
		"tanpa domain":        {New("k"), `{}`, "domain"},
		"variabel kosong":     {New("k"), `{"domain":"1100"}`, "ihk, inflasi_mtm, inflasi_yoy, inflasi_ytd"},
		"andil tak berpasang": {New("k"), `{"domain":"1100","variabel":{"ihk":"1","inflasi_mtm":"2","inflasi_yoy":"3","inflasi_ytd":"4","andil":"5"}}`, "berpasangan"},
		"tahun surut liar":    {New("k"), `{"domain":"1100","tahun_surut":99,"variabel":{"ihk":"1","inflasi_mtm":"2","inflasi_yoy":"3","inflasi_ytd":"4"}}`, "tahun_surut"},
		"json rusak":          {New("k"), `{"domain":`, "tidak valid"},
		"tanpa kunci API":     {New(""), konfigLengkap, EnvKunci},
	}
	for nama, c := range kasus {
		_, err := c.konektor.Permintaan(masukan(c.konf))
		if assert.Error(t, err, nama) {
			assert.Contains(t, err.Error(), c.pesan, nama)
		}
	}

	in := masukan(konfigLengkap)
	in.URLDasar = ""
	_, err := New("k").Permintaan(in)
	assert.ErrorContains(t, err, "url_dasar")
}

func TestGabungkan_FixtureMerakitInflasiDanAndil(t *testing.T) {
	hasil, err := New(kunciRahasia).Gabungkan(masukan(konfigLengkap), dokumen(t))
	require.NoError(t, err)

	require.Len(t, hasil.Inflasi, 3)
	agu, sep, banda := hasil.Inflasi[0], hasil.Inflasi[1], hasil.Inflasi[2]
	assert.Equal(t, "11", agu.WilayahKode)
	assert.True(t, time.Date(2025, 8, 1, 0, 0, 0, 0, time.UTC).Equal(agu.Periode), "diurutkan menurut bulan lalu wilayah")
	assert.Equal(t, 120.5, agu.IHK)
	assert.Equal(t, -0.1, agu.InflasiMtm)
	assert.Equal(t, "BPS", agu.Sumber)

	assert.Equal(t, "11", sep.WilayahKode)
	assert.True(t, time.Date(2025, 9, 1, 0, 0, 0, 0, time.UTC).Equal(sep.Periode))
	assert.Equal(t, 121.0, sep.IHK)
	assert.Equal(t, 2.84, sep.InflasiYoy)
	assert.Equal(t, 1.9, sep.InflasiYtd)
	assert.Equal(t, "1171", banda.WilayahKode)

	require.Len(t, hasil.Andil, 2)
	assert.Equal(t, "BWM", hasil.Andil[0].KomoditasKode)
	assert.Equal(t, "CBM-KRT", hasil.Andil[1].KomoditasKode)
	assert.Equal(t, 0.18, hasil.Andil[1].Andil)
	assert.Equal(t, 14.2, hasil.Andil[1].InflasiPct)
	assert.Equal(t, "11", hasil.Andil[1].WilayahKode)
	assert.Equal(t, "BPS", hasil.Andil[1].Sumber)

	// Yang tidak bisa dipakai dilaporkan: kota tak terpetakan (4 sel), komoditas tak terpetakan
	// (2 sel), dan Lhokseumawe yang tidak punya inflasi tahun kalender.
	assert.Equal(t, 4, hasil.TakTerpetakan["WILAYAH:kota asing"])
	assert.Equal(t, 2, hasil.TakTerpetakan["KOMODITAS:komoditas aneh"])
	assert.Equal(t, map[string]int{alasanTakLengkap: 1}, hasil.TidakValid)
}

func TestGabungkan_IdempotenDanDeterministik(t *testing.T) {
	a, err := New("k").Gabungkan(masukan(konfigLengkap), dokumen(t))
	require.NoError(t, err)
	b, err := New("k").Gabungkan(masukan(konfigLengkap), dokumen(t))
	require.NoError(t, err)
	assert.Equal(t, a, b)
}

func TestGabungkan_AndilOpsionalDanBulanTahunanDiabaikan(t *testing.T) {
	dok := dokumen(t)[:4]
	hasil, err := New("k").Gabungkan(masukan(`{"domain":"1100","variabel":{"ihk":"10","inflasi_mtm":"11","inflasi_yoy":"12","inflasi_ytd":"13"}}`), dok)
	require.NoError(t, err)
	assert.Len(t, hasil.Inflasi, 3)
	assert.Empty(t, hasil.Andil)
	for _, r := range hasil.Inflasi {
		assert.NotEqual(t, 999.0, r.IHK, "kolom Tahunan tidak dibaca sebagai bulan")
	}
}

func TestNormalisasi_MemeriksaDokumen(t *testing.T) {
	k := New("k")
	for _, d := range dokumen(t) {
		_, err := k.Normalisasi(masukan(konfigLengkap), d)
		assert.NoError(t, err, d.Kunci)
	}

	for nama, konten := range map[string]string{
		"bukan JSON":        `<html>`,
		"status gagal":      `{"status":"ERROR","var":[{"val":1}]}`,
		"data tak tersedia": `{"status":"OK","data-availability":"list-not-available","var":[{"val":1}]}`,
		"tanpa variabel":    `{"status":"OK","var":[]}`,
	} {
		_, err := k.Normalisasi(masukan(konfigLengkap), domain.Dokumen{Kunci: "var=ihk&th=2025", Konten: []byte(konten)})
		assert.Error(t, err, nama)
	}
}

func TestGabungkan_MenolakKunciAsingDanNilaiLiar(t *testing.T) {
	_, err := New("k").Gabungkan(masukan(konfigLengkap), []domain.Dokumen{{Kunci: "lain", Konten: []byte(`{}`)}})
	assert.Error(t, err)

	// Nilai di luar batas kolom (yoy 1000%) dan tak berangka dilewati, sehingga grup tak lengkap.
	liar := strings.ReplaceAll(string(dokumen(t)[2].Konten), "2.84", "1000")
	dok := dokumen(t)
	dok[2].Konten = []byte(liar)
	hasil, err := New("k").Gabungkan(masukan(konfigLengkap), dok)
	require.NoError(t, err)
	assert.Len(t, hasil.Inflasi, 2, "ACEH September gugur karena yoy liar")
	assert.Equal(t, 2, hasil.TidakValid[alasanTakLengkap])
}
