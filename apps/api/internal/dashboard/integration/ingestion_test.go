package integration

import (
	"math"
	"net/http"
	"sync"
	"testing"
	"time"

	"github.com/garda-inflasi/api/internal/dashboard/refresher"
	ingestionseeder "github.com/garda-inflasi/api/internal/ingestion/seeder"
	sumberseeder "github.com/garda-inflasi/api/internal/sumber-data/seeder"
	"github.com/garda-inflasi/api/pkg/logger"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

var (
	onceIngestion   sync.Once
	sharedIngestion *suite
)

// ingestionSuite menambah data hasil ingestion contoh (harga eksternal, cuaca) di atas suite bersama.
func ingestionSuite(t *testing.T) *suite {
	t.Helper()
	onceIngestion.Do(func() {
		s := newSuite(t, "dashboard_ingestion", true)
		_, err := sumberseeder.SeedReference(t.Context(), s.db)
		require.NoError(t, err)
		_, err = ingestionseeder.SeedDemo(t.Context(), s.db, time.Now())
		require.NoError(t, err)
		sharedIngestion = s
	})
	require.NotNil(t, sharedIngestion, "suite ingestion gagal dibuat")
	return sharedIngestion
}

type pembanding struct {
	Pasangan int     `json:"pasangan"`
	Sepakat  int     `json:"sepakat"`
	BatasPct float64 `json:"batas_pct"`
	Items    []struct {
		KomoditasKode  string  `json:"komoditas_kode"`
		KabupatenKode  string  `json:"kabupaten_kode"`
		Tanggal        string  `json:"tanggal"`
		HargaLapangan  float64 `json:"harga_lapangan"`
		HargaEksternal float64 `json:"harga_eksternal"`
		SelisihPct     float64 `json:"selisih_pct"`
		Sepakat        bool    `json:"sepakat"`
		Demo           bool    `json:"demo"`
	} `json:"items"`
}

func TestPembandingHarga_LapanganVsEksternal(t *testing.T) {
	s := ingestionSuite(t)

	w := s.call(t, http.MethodGet, "/v1/dashboard/operasional/pembanding-harga", emailBerhak, nil)

	require.Equal(t, http.StatusOK, w.Code, w.Body.String())
	p := data[pembanding](t, w)
	assert.Equal(t, 10.0, p.BatasPct)
	require.Positive(t, p.Pasangan, "harga lapangan dan eksternal contoh bertemu di kota yang sama")
	assert.LessOrEqual(t, p.Sepakat, p.Pasangan)
	require.NotEmpty(t, p.Items)
	assert.LessOrEqual(t, len(p.Items), 20)

	for i, it := range p.Items {
		assert.Positive(t, it.HargaLapangan)
		assert.Positive(t, it.HargaEksternal)
		assert.InDelta(t, (it.HargaLapangan-it.HargaEksternal)/it.HargaEksternal*100, it.SelisihPct, 0.06, "selisih dihitung terhadap harga eksternal")
		assert.Equal(t, math.Abs(it.SelisihPct) <= 10, it.Sepakat)
		assert.True(t, it.Demo, "sisi eksternal berasal dari eksekusi contoh")
		if i > 0 {
			assert.GreaterOrEqual(t, math.Abs(p.Items[i-1].SelisihPct), math.Abs(it.SelisihPct), "selisih terbesar dulu")
		}
	}
}

type cuacaRow struct {
	KabupatenKode string  `json:"kabupaten_kode"`
	Tanggal       string  `json:"tanggal"`
	CurahHujanMm  float64 `json:"curah_hujan_mm"`
	Anomali       bool    `json:"anomali"`
	Sumber        string  `json:"sumber"`
	Demo          bool    `json:"demo"`
}

func TestCuaca_TerbaruPerKabupatenAnomaliDulu(t *testing.T) {
	s := ingestionSuite(t)

	w := s.call(t, http.MethodGet, "/v1/dashboard/operasional/cuaca", emailBerhak, nil)

	require.Equal(t, http.StatusOK, w.Code, w.Body.String())
	items := data[[]cuacaRow](t, w)
	require.Len(t, items, 23, "satu baris per kabupaten/kota")
	seen := map[string]bool{}
	hariIni := time.Now().In(time.FixedZone("WIB", 7*3600)).Format("2006-01-02")
	sudahBukanAnomali := false
	for _, c := range items {
		assert.False(t, seen[c.KabupatenKode], "kabupaten ganda %s", c.KabupatenKode)
		seen[c.KabupatenKode] = true
		assert.Equal(t, hariIni, c.Tanggal, "cuaca hari ini")
		assert.Equal(t, "BMKG", c.Sumber)
		assert.True(t, c.Demo)
		assert.Equal(t, c.CurahHujanMm >= 50, c.Anomali)
		if !c.Anomali {
			sudahBukanAnomali = true
		}
		assert.False(t, c.Anomali && sudahBukanAnomali, "baris anomali berada di depan")
	}
}

func TestOperasionalHasilIngestion_Otorisasi(t *testing.T) {
	s := ingestionSuite(t)

	for _, path := range []string{"/v1/dashboard/operasional/pembanding-harga", "/v1/dashboard/operasional/cuaca"} {
		assert.Equal(t, http.StatusUnauthorized, s.call(t, http.MethodGet, path, "", nil).Code, path)
		assert.Equal(t, http.StatusForbidden, s.call(t, http.MethodGet, path, emailTakBerhak, nil).Code, path)
		assert.Equal(t, http.StatusOK, s.call(t, http.MethodGet, path, emailBerhak, nil).Code, path)
	}
}

func TestOperasionalHasilIngestion_KosongBukanGalat(t *testing.T) {
	s := newSuite(t, "dashboard_ingestion_kosong", false)

	pb := data[pembanding](t, s.call(t, http.MethodGet, "/v1/dashboard/operasional/pembanding-harga", emailBerhak, nil))
	assert.Zero(t, pb.Pasangan)
	assert.NotNil(t, pb.Items)
	assert.Empty(t, pb.Items)

	cuaca := data[[]cuacaRow](t, s.call(t, http.MethodGet, "/v1/dashboard/operasional/cuaca", emailBerhak, nil))
	assert.NotNil(t, cuaca)
	assert.Empty(t, cuaca)
}

func TestHeatmap_MelengkapiSelTanpaDataLapanganDariHargaEksternal(t *testing.T) {
	s := newSuite(t, "dashboard_heatmap_eksternal", true)
	_, err := sumberseeder.SeedReference(t.Context(), s.db)
	require.NoError(t, err)

	type sel struct {
		KomoditasKode string  `json:"komoditas_kode"`
		KabupatenKode string  `json:"kabupaten_kode"`
		Delta         float64 `json:"delta"`
		Harga         float64 `json:"harga"`
		Sumber        string  `json:"sumber"`
	}
	ambil := func() ([]sel, []string) {
		h := data[struct {
			Kabupaten []struct {
				Kode string `json:"kode"`
			} `json:"kabupaten"`
			Sel []sel `json:"sel"`
		}](t, s.call(t, http.MethodGet, "/v1/dashboard/operasional/heatmap", emailBerhak, nil))
		kab := make([]string, len(h.Kabupaten))
		for i, k := range h.Kabupaten {
			kab[i] = k.Kode
		}
		return h.Sel, kab
	}

	sebelum, kabSebelum := ambil()
	require.NotEmpty(t, sebelum)
	for _, c := range sebelum {
		assert.Equal(t, "LAPANGAN", c.Sumber)
	}
	require.NotContains(t, kabSebelum, "1101", "Simeulue belum punya data lapangan")

	// Harga eksternal Simeulue: sepekan di 150.000, lalu naik ke 160.000 kemarin (+6,7%).
	require.NoError(t, s.db.Exec(`
		INSERT INTO harga_eksternal (sumber_id, komoditas_kode, kabupaten_kode, tanggal, tipe, harga, satuan)
		SELECT 'SRC-BI', 'DAG-SPI', '1101', d::date, 'KONSUMEN',
		       CASE WHEN d::date = (now() AT TIME ZONE 'Asia/Jakarta')::date - 1 THEN 160000 ELSE 150000 END, 'kg'
		  FROM generate_series((now() AT TIME ZONE 'Asia/Jakarta')::date - 8, (now() AT TIME ZONE 'Asia/Jakarta')::date - 1, interval '1 day') d`).Error)
	// Sel yang sudah punya data lapangan tidak boleh digantikan oleh harga eksternal.
	require.NoError(t, s.db.Exec(`
		INSERT INTO harga_eksternal (sumber_id, komoditas_kode, kabupaten_kode, tanggal, tipe, harga, satuan)
		SELECT 'SRC-BI', komoditas_kode, kabupaten_kode, (now() AT TIME ZONE 'Asia/Jakarta')::date - 1, 'KONSUMEN', 1000, 'kg'
		  FROM mv_heatmap_harga LIMIT 1`).Error)
	require.NoError(t, refresher.Shared(s.db, logger.NewDefault()).Refresh(t.Context()))

	sesudah, kabSesudah := ambil()
	assert.Contains(t, kabSesudah, "1101")
	var eksternal []sel
	for _, c := range sesudah {
		if c.Sumber == "EKSTERNAL" {
			eksternal = append(eksternal, c)
		}
	}
	require.Len(t, eksternal, 1, "hanya sel tanpa data lapangan")
	assert.Equal(t, "1101", eksternal[0].KabupatenKode)
	assert.Equal(t, "DAG-SPI", eksternal[0].KomoditasKode)
	assert.Equal(t, 160000.0, eksternal[0].Harga)
	assert.InDelta(t, 6.7, eksternal[0].Delta, 0.001)
	assert.Len(t, sesudah, len(sebelum)+1)
}
