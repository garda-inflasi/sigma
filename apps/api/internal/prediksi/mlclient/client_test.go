package mlclient

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestFromEnv_KosongBerartiTidakDikonfigurasi(t *testing.T) {
	t.Setenv(EnvURL, "  ")
	assert.Nil(t, FromEnv())

	t.Setenv(EnvURL, "http://ml:8000/")
	c, ok := FromEnv().(*HTTPClient)
	require.True(t, ok)
	assert.Equal(t, "http://ml:8000", c.baseURL)
}

func TestTrain_MengirimKontrakDanMembacaJawaban(t *testing.T) {
	var diterima TrainRequest
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/train", r.URL.Path)
		assert.Equal(t, http.MethodPost, r.Method)
		assert.Equal(t, "application/json", r.Header.Get("Content-Type"))
		require.NoError(t, json.NewDecoder(r.Body).Decode(&diterima))
		_, _ = w.Write([]byte(`{"versi_layanan":"0.1.0","komoditas_kode":"CBM-KRT","seri":[
			{"id":"1171","status":"OK","algoritma":"holt","params":{"alpha":0.5},"galat_rmse":{"7":120.5},
			 "metrik":{"mape":2.5,"rmse":900,"baseline_mape":3.1,"baseline_rmse":1000,"optimistis":false},
			 "harga_dasar":50000,"tanggal_dasar":"2026-11-04","n_hari":90,
			 "ramalan":[{"horizon_hari":7,"tanggal_target":"2026-11-11","nilai":51000,"bawah":50000,"atas":52000,
			   "faktor":[{"nama":"Tren harga","kontribusi":2.0,"detail":"d"}]}]},
			{"id":"1101","status":"DILEWATI","alasan":"riwayat kurang"}],
			"ringkasan":{"n_seri_ok":1,"n_seri_dilewati":1,"mape":2.5,"rmse":900,"baseline_mape":3.1,"baseline_rmse":1000,"algoritma":{"holt":1}}}`))
	}))
	defer srv.Close()

	res, err := New(srv.URL, nil).Train(t.Context(), TrainRequest{
		KomoditasKode: "CBM-KRT", Horizon: []int{7},
		Seri: []Seri{{ID: "1171", Titik: []Titik{{Tanggal: "2026-11-04", Nilai: 50000}}}},
	})

	require.NoError(t, err)
	assert.Equal(t, "CBM-KRT", diterima.KomoditasKode)
	assert.Equal(t, []int{7}, diterima.Horizon)
	assert.Equal(t, 50000, diterima.Seri[0].Titik[0].Nilai)
	assert.Equal(t, 1, res.Ringkasan.NSeriOK)
	assert.Equal(t, "riwayat kurang", res.Seri[1].Alasan)
	assert.Equal(t, 51000, res.Seri[0].Ramalan[0].Nilai)
	assert.Equal(t, 2.0, res.Seri[0].Ramalan[0].Faktor[0].Kontribusi)
}

func TestPredict_MengirimParameterTerlatih(t *testing.T) {
	var diterima map[string]any
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/predict", r.URL.Path)
		require.NoError(t, json.NewDecoder(r.Body).Decode(&diterima))
		_, _ = w.Write([]byte(`{"versi_layanan":"0.1.0","komoditas_kode":"CBM-KRT","seri":[]}`))
	}))
	defer srv.Close()

	_, err := New(srv.URL, nil).Predict(t.Context(), PredictRequest{
		KomoditasKode: "CBM-KRT", Horizon: []int{7},
		Seri: []SeriPrediksi{{Seri: Seri{ID: "1171"}, Algoritma: "holt", Params: map[string]float64{"alpha": 0.5}, GalatRMSE: map[string]float64{"7": 10}}},
	})

	require.NoError(t, err)
	seri := diterima["seri"].([]any)[0].(map[string]any)
	assert.Equal(t, "1171", seri["id"])
	assert.Equal(t, "holt", seri["algoritma"])
	assert.Contains(t, seri, "galat_rmse")
}

func TestHealth(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/health", r.URL.Path)
		_, _ = w.Write([]byte(`{"status":"ok","versi":"0.1.0"}`))
	}))
	defer srv.Close()

	h, err := New(srv.URL, nil).Health(t.Context())

	require.NoError(t, err)
	assert.Equal(t, Health{Status: "ok", Versi: "0.1.0"}, *h)
}

func TestGalat_DipetakanKeSentinel(t *testing.T) {
	kasus := []struct {
		nama    string
		handler http.HandlerFunc
		want    error
	}{
		{"galat server", func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(http.StatusInternalServerError) }, ErrTidakTersedia},
		{"muatan ditolak", func(w http.ResponseWriter, _ *http.Request) {
			w.WriteHeader(http.StatusBadRequest)
			_, _ = w.Write([]byte(`{"error":"horizon tidak valid"}`))
		}, ErrDitolak},
		{"bukan JSON", func(w http.ResponseWriter, _ *http.Request) { _, _ = w.Write([]byte("<html>")) }, ErrTidakTersedia},
	}
	for _, k := range kasus {
		t.Run(k.nama, func(t *testing.T) {
			srv := httptest.NewServer(k.handler)
			defer srv.Close()

			_, err := New(srv.URL, nil).Train(t.Context(), TrainRequest{})

			assert.ErrorIs(t, err, k.want)
		})
	}
}

func TestGalat_PesanLayananDisertakan(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusBadRequest)
		_, _ = w.Write([]byte(`{"error":"horizon tidak valid"}`))
	}))
	defer srv.Close()

	_, err := New(srv.URL, nil).Train(t.Context(), TrainRequest{})

	assert.ErrorContains(t, err, "horizon tidak valid")
}

func TestLayananMati_ErrTidakTersedia(t *testing.T) {
	srv := httptest.NewServer(http.NotFoundHandler())
	url := srv.URL
	srv.Close() // port sudah tidak mendengarkan

	_, err := New(url, nil).Health(t.Context())

	assert.ErrorIs(t, err, ErrTidakTersedia)
}

func TestLayananLambat_TimeoutMenjadiTidakTersedia(t *testing.T) {
	selesai := make(chan struct{})
	srv := httptest.NewServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) { <-selesai }))
	defer srv.Close()
	defer close(selesai)

	ctx, batal := context.WithTimeout(t.Context(), 50*time.Millisecond)
	defer batal()
	_, err := New(srv.URL, nil).Health(ctx)

	assert.ErrorIs(t, err, ErrTidakTersedia)
}
