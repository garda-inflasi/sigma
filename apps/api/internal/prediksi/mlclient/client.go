// Package mlclient adalah klien HTTP ke layanan ML (apps/ml): pelatihan dan
// peramalan harga. Layanan hanya terjangkau di jaringan internal; bila mati atau
// lambat, pemanggil menerima ErrTidakTersedia dan API tetap berjalan.
package mlclient

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"time"
)

var (
	// ErrTidakTersedia berarti layanan tidak terjangkau, kehabisan waktu, atau
	// menjawab galat server.
	ErrTidakTersedia = errors.New("layanan ML tidak tersedia")
	// ErrDitolak berarti layanan menolak permintaan (4xx), mis. muatan tidak valid.
	ErrDitolak = errors.New("layanan ML menolak permintaan")
)

const (
	// timeoutBawaan lebih pendek dari WriteTimeout server API (15 detik).
	timeoutBawaan = 10 * time.Second
	// maksRespons membatasi badan jawaban yang dibaca.
	maksRespons = 32 << 20
	// EnvURL adalah nama variabel lingkungan alamat layanan.
	EnvURL = "ML_SERVICE_URL"
)

// Client adalah antarmuka yang dipakai usecase; tes memakai fake atau httptest.
type Client interface {
	Train(ctx context.Context, req TrainRequest) (*TrainResponse, error)
	Predict(ctx context.Context, req PredictRequest) (*PredictResponse, error)
	Health(ctx context.Context) (*Health, error)
}

// Titik adalah harga rata-rata satu hari.
type Titik struct {
	Tanggal string `json:"tanggal"`
	Nilai   int    `json:"nilai"`
}

// Seri adalah deret harga satu wilayah.
type Seri struct {
	ID    string  `json:"id"`
	Titik []Titik `json:"titik"`
}

type TrainRequest struct {
	KomoditasKode string `json:"komoditas_kode"`
	Horizon       []int  `json:"horizon"`
	Seri          []Seri `json:"seri"`
}

// SeriPrediksi adalah deret beserta model terlatihnya.
type SeriPrediksi struct {
	Seri
	Algoritma string             `json:"algoritma"`
	Params    map[string]float64 `json:"params"`
	GalatRMSE map[string]float64 `json:"galat_rmse"`
}

type PredictRequest struct {
	KomoditasKode string         `json:"komoditas_kode"`
	Horizon       []int          `json:"horizon"`
	Seri          []SeriPrediksi `json:"seri"`
}

type Faktor struct {
	Nama       string  `json:"nama"`
	Kontribusi float64 `json:"kontribusi"`
	Detail     string  `json:"detail"`
}

// Ramalan adalah satu proyeksi dengan rentang keyakinan 80% dan penjelasannya.
type Ramalan struct {
	HorizonHari   int      `json:"horizon_hari"`
	TanggalTarget string   `json:"tanggal_target"`
	Nilai         int      `json:"nilai"`
	Bawah         int      `json:"bawah"`
	Atas          int      `json:"atas"`
	Faktor        []Faktor `json:"faktor"`
}

// Metrik adalah hasil backtest walk-forward satu seri; persen untuk MAPE.
type Metrik struct {
	Mape         float64            `json:"mape"`
	Rmse         float64            `json:"rmse"`
	BaselineMape float64            `json:"baseline_mape"`
	BaselineRmse float64            `json:"baseline_rmse"`
	MapePerHari  map[string]float64 `json:"mape_per_horizon"`
	NLipatanUji  int                `json:"n_lipatan_uji"`
	Optimistis   bool               `json:"optimistis"`
}

// HasilSeri adalah jawaban untuk satu seri; Status "OK" atau "DILEWATI" (dengan Alasan).
type HasilSeri struct {
	ID           string             `json:"id"`
	Status       string             `json:"status"`
	Alasan       string             `json:"alasan"`
	Algoritma    string             `json:"algoritma"`
	Params       map[string]float64 `json:"params"`
	GalatRMSE    map[string]float64 `json:"galat_rmse"`
	Metrik       Metrik             `json:"metrik"`
	HargaDasar   int                `json:"harga_dasar"`
	TanggalDasar string             `json:"tanggal_dasar"`
	NHari        int                `json:"n_hari"`
	Ramalan      []Ramalan          `json:"ramalan"`
}

// Ringkasan adalah rata-rata metrik seri yang berhasil dilatih.
type Ringkasan struct {
	NSeriOK       int            `json:"n_seri_ok"`
	NSeriDilewati int            `json:"n_seri_dilewati"`
	Mape          float64        `json:"mape"`
	Rmse          float64        `json:"rmse"`
	BaselineMape  float64        `json:"baseline_mape"`
	BaselineRmse  float64        `json:"baseline_rmse"`
	Optimistis    bool           `json:"optimistis"`
	Algoritma     map[string]int `json:"algoritma"`
}

type TrainResponse struct {
	VersiLayanan  string      `json:"versi_layanan"`
	KomoditasKode string      `json:"komoditas_kode"`
	Seri          []HasilSeri `json:"seri"`
	Ringkasan     Ringkasan   `json:"ringkasan"`
}

type PredictResponse struct {
	VersiLayanan  string      `json:"versi_layanan"`
	KomoditasKode string      `json:"komoditas_kode"`
	Seri          []HasilSeri `json:"seri"`
}

type Health struct {
	Status string `json:"status"`
	Versi  string `json:"versi"`
}

// HTTPClient memanggil layanan ML lewat HTTP.
type HTTPClient struct {
	baseURL string
	http    *http.Client
}

// New membuat klien; httpClient nil memakai klien dengan timeout bawaan.
func New(baseURL string, httpClient *http.Client) *HTTPClient {
	if httpClient == nil {
		httpClient = &http.Client{Timeout: timeoutBawaan}
	}
	return &HTTPClient{baseURL: strings.TrimRight(baseURL, "/"), http: httpClient}
}

// FromEnv membuat klien dari ML_SERVICE_URL; nil bila variabel kosong, yaitu
// layanan ML tidak dikonfigurasi.
func FromEnv() Client {
	url := strings.TrimSpace(os.Getenv(EnvURL))
	if url == "" {
		return nil
	}
	return New(url, nil)
}

func (c *HTTPClient) Train(ctx context.Context, req TrainRequest) (*TrainResponse, error) {
	var res TrainResponse
	if err := c.do(ctx, http.MethodPost, "/train", req, &res); err != nil {
		return nil, err
	}
	return &res, nil
}

func (c *HTTPClient) Predict(ctx context.Context, req PredictRequest) (*PredictResponse, error) {
	var res PredictResponse
	if err := c.do(ctx, http.MethodPost, "/predict", req, &res); err != nil {
		return nil, err
	}
	return &res, nil
}

func (c *HTTPClient) Health(ctx context.Context) (*Health, error) {
	var res Health
	if err := c.do(ctx, http.MethodGet, "/health", nil, &res); err != nil {
		return nil, err
	}
	return &res, nil
}

func (c *HTTPClient) do(ctx context.Context, method, path string, body, out any) error {
	var reader io.Reader
	if body != nil {
		raw, err := json.Marshal(body)
		if err != nil {
			return fmt.Errorf("mlclient: encode permintaan: %w", err)
		}
		reader = bytes.NewReader(raw)
	}
	req, err := http.NewRequestWithContext(ctx, method, c.baseURL+path, reader)
	if err != nil {
		return fmt.Errorf("mlclient: buat permintaan: %w", err)
	}
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}

	res, err := c.http.Do(req)
	if err != nil {
		return fmt.Errorf("%w: %v", ErrTidakTersedia, err)
	}
	defer res.Body.Close()
	raw, err := io.ReadAll(io.LimitReader(res.Body, maksRespons))
	if err != nil {
		return fmt.Errorf("%w: baca jawaban: %v", ErrTidakTersedia, err)
	}

	switch {
	case res.StatusCode >= 500:
		return fmt.Errorf("%w: status %d", ErrTidakTersedia, res.StatusCode)
	case res.StatusCode >= 400:
		return fmt.Errorf("%w: %s", ErrDitolak, galatLayanan(raw, res.StatusCode))
	}
	if err := json.Unmarshal(raw, out); err != nil {
		return fmt.Errorf("%w: jawaban bukan JSON yang valid: %v", ErrTidakTersedia, err)
	}
	return nil
}

// galatLayanan mengambil pesan "error" dari badan jawaban galat layanan.
func galatLayanan(raw []byte, status int) string {
	var e struct {
		Error string `json:"error"`
	}
	if json.Unmarshal(raw, &e) == nil && e.Error != "" {
		return e.Error
	}
	return fmt.Sprintf("status %d", status)
}
