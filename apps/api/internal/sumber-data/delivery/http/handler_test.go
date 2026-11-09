package http_test

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/garda-inflasi/api/internal/audit/audittest"
	ingest "github.com/garda-inflasi/api/internal/ingestion/domain"
	delivery "github.com/garda-inflasi/api/internal/sumber-data/delivery/http"
	"github.com/garda-inflasi/api/internal/sumber-data/domain"
	"github.com/garda-inflasi/api/internal/sumber-data/dto"
	"github.com/garda-inflasi/api/internal/sumber-data/usecase"
	"github.com/garda-inflasi/api/pkg/idx"
	"github.com/garda-inflasi/api/pkg/logger"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type fakeRepo struct {
	items []domain.SumberData
	err   error
}

func (f *fakeRepo) List(context.Context) ([]domain.SumberData, error) {
	return f.items, f.err
}

func (f *fakeRepo) Get(_ context.Context, ref idx.RujukanUUID) (*domain.SumberData, error) {
	for _, it := range f.items {
		if (ref.ID != "" && ref.ID == it.ID) || (ref.Kode != "" && ref.Kode == it.Kode) {
			return &it, nil
		}
	}
	return nil, domain.ErrNotFound
}

func (f *fakeRepo) Cari(_ context.Context, uuids, kodes []string) ([]domain.Ringkas, error) {
	var res []domain.Ringkas
	for _, it := range f.items {
		for _, u := range uuids {
			if it.ID == u {
				res = append(res, domain.Ringkas{ID: it.ID, Kode: it.Kode})
			}
		}
		for _, k := range kodes {
			if it.Kode == k {
				res = append(res, domain.Ringkas{ID: it.ID, Kode: it.Kode})
			}
		}
	}
	return res, f.err
}

func (f *fakeRepo) BulkAction(_ context.Context, ids []string, _ domain.BulkAction) ([]string, error) {
	return ids, f.err
}

type fakeIngestion struct {
	konektor map[string]bool
	eksekusi []ingest.EksekusiSumber
	sampel   []ingest.SampelMentah
	pipeline ingest.Pipeline
	sukses   map[string]time.Time
}

func (f *fakeIngestion) Punya(kode string) bool {
	return f.konektor[kode]
}

func (f *fakeIngestion) Mulai(_ context.Context, sumberID, pemicu string, _ *string) (usecase.Berjalan, error) {
	if sumberID == "b0000000-0000-3000-8000-000000000002" || sumberID == "SRC-NO-CONN" {
		return nil, ingest.ErrTanpaKonektor
	}
	if sumberID == "b0000000-0000-3000-8000-000000000001" || sumberID == "SRC-PAUSED" {
		return nil, ingest.ErrDijeda
	}
	if sumberID == "SRC-RUNNING" {
		return nil, ingest.ErrSudahBerjalan
	}
	return &fakeBerjalan{
		eksekusi: ingest.EksekusiSumber{
			ID:       1,
			SumberID: sumberID,
			Pemicu:   pemicu,
			Status:   ingest.StatusBerjalan,
		},
	}, nil
}

func (f *fakeIngestion) Eksekusi(_ context.Context, _ string, _ int) ([]ingest.EksekusiSumber, error) {
	return f.eksekusi, nil
}

func (f *fakeIngestion) SampelMentah(_ context.Context, _ string, _ int) ([]ingest.SampelMentah, error) {
	return f.sampel, nil
}

func (f *fakeIngestion) Pipeline(_ context.Context) (ingest.Pipeline, error) {
	return f.pipeline, nil
}

func (f *fakeIngestion) TerakhirSukses(_ context.Context) (map[string]time.Time, error) {
	return f.sukses, nil
}

type fakeBerjalan struct {
	eksekusi ingest.EksekusiSumber
}

func (b *fakeBerjalan) Eksekusi() ingest.EksekusiSumber { return b.eksekusi }
func (b *fakeBerjalan) Jalankan(context.Context) (*ingest.EksekusiSumber, error) {
	e := b.eksekusi
	e.Status = ingest.StatusSukses
	return &e, nil
}

func setupRouter() (*gin.Engine, *fakeRepo, *fakeIngestion) {
	gin.SetMode(gin.TestMode)
	repo := &fakeRepo{
		items: []domain.SumberData{
			{
				ID:             "40745615-f152-32aa-b1e3-f54add179514",
				Kode:           "SRC-BI",
				Nama:           "PIHPS Nasional",
				Penyedia:       "Bank Indonesia",
				Jenis:          domain.JenisAPI,
				Cakupan:        "Pasar tradisional",
				JadwalCron:     "0 10 * * *",
				Status:         domain.StatusSehat,
				RobotsTxtPatuh: true,
			},
			{
				ID:             "b0000000-0000-3000-8000-000000000001",
				Kode:           "SRC-PAUSED",
				Nama:           "Sumber Dijeda",
				Penyedia:       "Instansi X",
				Jenis:          domain.JenisScraper,
				JadwalCron:     "0 12 * * *",
				Status:         domain.StatusDijeda,
				RobotsTxtPatuh: true,
			},
			{
				ID:             "b0000000-0000-3000-8000-000000000002",
				Kode:           "SRC-NO-CONN",
				Nama:           "Tanpa Konektor",
				Penyedia:       "Instansi Y",
				Jenis:          domain.JenisAPI,
				JadwalCron:     "0 1 * * *",
				Status:         domain.StatusTertunda,
				RobotsTxtPatuh: false,
			},
		},
	}
	fakeIng := &fakeIngestion{
		konektor: map[string]bool{
			"SRC-BI":                               true,
			"40745615-f152-32aa-b1e3-f54add179514": true,
		},
		eksekusi: []ingest.EksekusiSumber{
			{ID: 10, SumberID: "40745615-f152-32aa-b1e3-f54add179514", Status: ingest.StatusSukses},
		},
		sampel: []ingest.SampelMentah{
			{ID: 100, EksekusiID: 10, KunciPermintaan: "wilayah=1171", Cuplikan: `{"data":[]}`},
		},
		pipeline: ingest.Pipeline{
			Bronze: 100, Silver: 50, Gold: 20,
		},
		sukses: map[string]time.Time{
			"40745615-f152-32aa-b1e3-f54add179514": time.Now(),
		},
	}
	uc := usecase.New(repo, fakeIng, &audittest.Recorder{}, logger.NewDefault())
	h := delivery.New(uc)

	r := gin.New()
	v1 := r.Group("/v1/sumber-data")
	{
		v1.GET("", h.List)
		v1.GET("/pipeline", h.Pipeline)
		v1.POST("/bulk", h.Bulk)
		v1.GET("/:id", h.Get)
		v1.GET("/:id/eksekusi", h.Eksekusi)
		v1.GET("/:id/sampel", h.Sampel)
		v1.POST("/:id/jalankan", h.Jalankan)
	}
	return r, repo, fakeIng
}

func TestList(t *testing.T) {
	r, _, _ := setupRouter()

	req := httptest.NewRequest(http.MethodGet, "/v1/sumber-data", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	var resp struct {
		Data []dto.SumberDataResponse `json:"data"`
	}
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
	assert.Len(t, resp.Data, 3)
	assert.Equal(t, "SRC-BI", resp.Data[0].Kode)
	assert.True(t, resp.Data[0].BisaDijalankan)
}

func TestGet(t *testing.T) {
	r, _, _ := setupRouter()

	// Sukses lewat kode
	req := httptest.NewRequest(http.MethodGet, "/v1/sumber-data/SRC-BI", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	assert.Equal(t, http.StatusOK, w.Code)

	// Sukses lewat UUID
	req = httptest.NewRequest(http.MethodGet, "/v1/sumber-data/40745615-f152-32aa-b1e3-f54add179514", nil)
	w = httptest.NewRecorder()
	r.ServeHTTP(w, req)
	assert.Equal(t, http.StatusOK, w.Code)

	// Tidak ada
	req = httptest.NewRequest(http.MethodGet, "/v1/sumber-data/SRC-TIDAK-ADA", nil)
	w = httptest.NewRecorder()
	r.ServeHTTP(w, req)
	assert.Equal(t, http.StatusNotFound, w.Code)
}

func TestBulk(t *testing.T) {
	r, _, _ := setupRouter()

	body, _ := json.Marshal(dto.BulkSumberDataRequest{
		IDs:    []string{"SRC-BI"},
		Action: "jeda",
	})
	req := httptest.NewRequest(http.MethodPost, "/v1/sumber-data/bulk", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	// Aksi salah
	bodyWrong, _ := json.Marshal(dto.BulkSumberDataRequest{
		IDs:    []string{"SRC-BI"},
		Action: "aksi_salah",
	})
	req = httptest.NewRequest(http.MethodPost, "/v1/sumber-data/bulk", bytes.NewReader(bodyWrong))
	req.Header.Set("Content-Type", "application/json")
	w = httptest.NewRecorder()
	r.ServeHTTP(w, req)
	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestPipeline(t *testing.T) {
	r, _, _ := setupRouter()

	req := httptest.NewRequest(http.MethodGet, "/v1/sumber-data/pipeline", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	var resp struct {
		Data dto.PipelineResponse `json:"data"`
	}
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
	assert.EqualValues(t, 100, resp.Data.Bronze)
	assert.EqualValues(t, 50, resp.Data.Silver)
	assert.EqualValues(t, 20, resp.Data.Gold)
}

func TestEksekusiDanSampel(t *testing.T) {
	r, _, _ := setupRouter()

	// Eksekusi
	req := httptest.NewRequest(http.MethodGet, "/v1/sumber-data/SRC-BI/eksekusi?limit=5", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	assert.Equal(t, http.StatusOK, w.Code)

	// Sampel
	req = httptest.NewRequest(http.MethodGet, "/v1/sumber-data/SRC-BI/sampel?limit=5", nil)
	w = httptest.NewRecorder()
	r.ServeHTTP(w, req)
	assert.Equal(t, http.StatusOK, w.Code)

	// Limit tidak valid
	req = httptest.NewRequest(http.MethodGet, "/v1/sumber-data/SRC-BI/eksekusi?limit=999", nil)
	w = httptest.NewRecorder()
	r.ServeHTTP(w, req)
	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestJalankan(t *testing.T) {
	r, _, _ := setupRouter()

	// Sukses dijalankan (202 Accepted)
	req := httptest.NewRequest(http.MethodPost, "/v1/sumber-data/SRC-BI/jalankan", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	assert.Equal(t, http.StatusAccepted, w.Code)

	// Sumber tidak ada (404)
	req = httptest.NewRequest(http.MethodPost, "/v1/sumber-data/SRC-HILANG/jalankan", nil)
	w = httptest.NewRecorder()
	r.ServeHTTP(w, req)
	assert.Equal(t, http.StatusNotFound, w.Code)

	// Tanpa konektor (422)
	req = httptest.NewRequest(http.MethodPost, "/v1/sumber-data/SRC-NO-CONN/jalankan", nil)
	w = httptest.NewRecorder()
	r.ServeHTTP(w, req)
	assert.Equal(t, http.StatusUnprocessableEntity, w.Code)

	// Sedang dijeda (409)
	req = httptest.NewRequest(http.MethodPost, "/v1/sumber-data/SRC-PAUSED/jalankan", nil)
	w = httptest.NewRecorder()
	r.ServeHTTP(w, req)
	assert.Equal(t, http.StatusConflict, w.Code)
}
