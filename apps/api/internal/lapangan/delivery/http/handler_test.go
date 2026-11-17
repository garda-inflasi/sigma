package http

import (
	"bytes"
	"context"
	"encoding/json"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/garda-inflasi/api/internal/audit/audittest"
	hargadomain "github.com/garda-inflasi/api/internal/harga/domain"
	"github.com/garda-inflasi/api/internal/lapangan/domain"
	"github.com/garda-inflasi/api/internal/lapangan/usecase"
	"github.com/garda-inflasi/api/pkg/idx"
	"github.com/garda-inflasi/api/pkg/logger"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

type stubRepo struct {
	mock.Mock
}

func (s *stubRepo) PenggunaCakupan(ctx context.Context, userID string) (string, string, bool, error) {
	args := s.Called(ctx, userID)
	return args.String(0), args.String(1), args.Bool(2), args.Error(3)
}
func (s *stubRepo) Acuan(ctx context.Context, sejak time.Time, minSampel int) (domain.DataAcuan, error) {
	return domain.DataAcuan{}, nil
}
func (s *stubRepo) ListSaya(ctx context.Context, cakupan hargadomain.Cakupan) ([]domain.TugasView, error) {
	return nil, nil
}
func (s *stubRepo) GetByID(ctx context.Context, ref idx.RujukanID, cakupan hargadomain.Cakupan) (*domain.TugasView, error) {
	return nil, domain.ErrNotFound
}
func (s *stubRepo) Rincian(ctx context.Context, tugasID idx.ID, kode []string) ([]domain.RincianKomoditas, error) {
	return nil, nil
}
func (s *stubRepo) Aksesibel(ctx context.Context, refs []idx.RujukanID, cakupan hargadomain.Cakupan) ([]domain.TugasRef, error) {
	return nil, nil
}
func (s *stubRepo) TandaiSelesai(ctx context.Context, refs []idx.RujukanID, cakupan hargadomain.Cakupan, waktu time.Time) ([]domain.TugasRef, error) {
	return nil, nil
}
func (s *stubRepo) EntriMilikPetugas(ctx context.Context, entriID idx.ID) (string, string, bool, error) {
	args := s.Called(ctx, entriID)
	return args.String(0), args.String(1), args.Bool(2), args.Error(3)
}
func (s *stubRepo) SimpanLampiranEntri(ctx context.Context, lampiran *domain.LampiranEntri, fotoURL string) error {
	args := s.Called(ctx, lampiran, fotoURL)
	return args.Error(0)
}
func (s *stubRepo) LampiranTerbaru(ctx context.Context, entriID idx.ID) (*domain.LampiranEntri, error) {
	args := s.Called(ctx, entriID)
	item, _ := args.Get(0).(*domain.LampiranEntri)
	return item, args.Error(1)
}
func (s *stubRepo) KodeEtikAktif(ctx context.Context, userID string) (*domain.KodeEtikStatus, error) {
	args := s.Called(ctx, userID)
	item, _ := args.Get(0).(*domain.KodeEtikStatus)
	return item, args.Error(1)
}
func (s *stubRepo) SetujuiKodeEtik(ctx context.Context, userID, versi string) (*domain.PersetujuanKodeEtik, error) {
	args := s.Called(ctx, userID, versi)
	item, _ := args.Get(0).(*domain.PersetujuanKodeEtik)
	return item, args.Error(1)
}
func (s *stubRepo) ListKinerja(ctx context.Context, cakupan hargadomain.Cakupan) ([]domain.KinerjaPetugas, error) {
	args := s.Called(ctx, cakupan)
	items, _ := args.Get(0).([]domain.KinerjaPetugas)
	return items, args.Error(1)
}

func setupRouter(h *Handler, userID string, perms ...string) *gin.Engine {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.Use(func(c *gin.Context) {
		c.Set("user_id", userID)
		c.Set("permissions", perms)
		c.Next()
	})
	r.POST("/v1/lapangan/harga/:id/foto", h.UploadFoto)
	r.GET("/v1/lapangan/kode-etik", h.GetKodeEtik)
	r.POST("/v1/lapangan/kode-etik/setuju", h.SetujuiKodeEtik)
	r.GET("/v1/lapangan/kinerja", h.ListKinerja)
	return r
}

func multipartRequest(t *testing.T, url, fieldName, fileName string, content []byte) *http.Request {
	t.Helper()
	var buf bytes.Buffer
	mw := multipart.NewWriter(&buf)
	fw, err := mw.CreateFormFile(fieldName, fileName)
	require.NoError(t, err)
	_, err = fw.Write(content)
	require.NoError(t, err)
	require.NoError(t, mw.Close())

	req := httptest.NewRequest(http.MethodPost, url, &buf)
	req.Header.Set("Content-Type", mw.FormDataContentType())
	return req
}

func TestHandlerUploadFoto(t *testing.T) {
	ownerID := "11111111-1111-1111-1111-111111111111"
	otherID := "22222222-2222-2222-2222-222222222222"
	jpegBytes := []byte{0xFF, 0xD8, 0xFF, 0xE0, 0x00, 0x10, 'J', 'F', 'I', 'F', 0x00, 0x01, 0x01, 0x01, 0x00, 0x48, 0x00, 0x48, 0x00, 0x00, 0xFF, 0xD9}

	t.Run("ID bukan BIGINT dijawab 404 tanpa menyentuh repo", func(t *testing.T) {
		repo := &stubRepo{}
		uc := usecase.New(repo, &audittest.Recorder{}, logger.NewDefault()).WithStorageDir(t.TempDir())
		r := setupRouter(New(uc), ownerID, "harga.input")

		for _, badID := range []string{"abc", "0", "-5", "HRG-1"} {
			req := multipartRequest(t, "/v1/lapangan/harga/"+badID+"/foto", "foto", "test.jpg", jpegBytes)
			w := httptest.NewRecorder()
			r.ServeHTTP(w, req)
			assert.Equal(t, http.StatusNotFound, w.Code, "id=%s harus 404", badID)
		}
		repo.AssertNotCalled(t, "EntriMilikPetugas")
	})

	t.Run("MIME bukan gambar dijawab 400", func(t *testing.T) {
		repo := &stubRepo{}
		uc := usecase.New(repo, &audittest.Recorder{}, logger.NewDefault()).WithStorageDir(t.TempDir())
		r := setupRouter(New(uc), ownerID, "harga.input")

		req := multipartRequest(t, "/v1/lapangan/harga/12/foto", "foto", "evil.txt", []byte("plain text payload"))
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)
		assert.Equal(t, http.StatusBadRequest, w.Code)
	})

	t.Run("bukan pemilik entri dijawab 404", func(t *testing.T) {
		repo := &stubRepo{}
		repo.On("EntriMilikPetugas", mock.Anything, idx.ID(12)).Return(ownerID, "1171", true, nil)
		uc := usecase.New(repo, &audittest.Recorder{}, logger.NewDefault()).WithStorageDir(t.TempDir())
		r := setupRouter(New(uc), otherID, "harga.input")

		req := multipartRequest(t, "/v1/lapangan/harga/12/foto", "foto", "test.jpg", jpegBytes)
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)
		assert.Equal(t, http.StatusNotFound, w.Code)
	})

	t.Run("sukses unggah foto mengembalikan 201", func(t *testing.T) {
		repo := &stubRepo{}
		repo.On("EntriMilikPetugas", mock.Anything, idx.ID(12)).Return(ownerID, "1171", true, nil)
		repo.On("SimpanLampiranEntri", mock.Anything, mock.AnythingOfType("*domain.LampiranEntri"), "/v1/lapangan/harga/12/foto").
			Run(func(args mock.Arguments) {
				args.Get(1).(*domain.LampiranEntri).ID = 99
			}).Return(nil)
		uc := usecase.New(repo, &audittest.Recorder{}, logger.NewDefault()).WithStorageDir(t.TempDir())
		r := setupRouter(New(uc), ownerID, "harga.input")

		req := multipartRequest(t, "/v1/lapangan/harga/12/foto", "foto", "test.jpg", jpegBytes)
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)
		assert.Equal(t, http.StatusCreated, w.Code)

		var body map[string]any
		require.NoError(t, json.Unmarshal(w.Body.Bytes(), &body))
		data := body["data"].(map[string]any)
		assert.Equal(t, "/v1/lapangan/harga/12/foto", data["foto_url"])
		assert.Equal(t, "image/jpeg", data["mime"])
	})
}

func TestHandlerKodeEtikDanKinerja(t *testing.T) {
	userID := "11111111-1111-1111-1111-111111111111"

	t.Run("GET dan POST kode etik", func(t *testing.T) {
		repo := &stubRepo{}
		repo.On("KodeEtikAktif", mock.Anything, userID).Return(&domain.KodeEtikStatus{
			Versi: "v1.0", Judul: "Kode Etik", Isi: "Isi", SudahSetuju: false,
		}, nil)
		repo.On("SetujuiKodeEtik", mock.Anything, userID, "v1.0").Return(&domain.PersetujuanKodeEtik{
			ID: 1, UserID: userID, Versi: "v1.0", DisetujuiPada: time.Now(),
		}, nil)

		uc := usecase.New(repo, &audittest.Recorder{}, logger.NewDefault())
		r := setupRouter(New(uc), userID, "harga.input")

		w := httptest.NewRecorder()
		r.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/v1/lapangan/kode-etik", nil))
		assert.Equal(t, http.StatusOK, w.Code)

		badReq := httptest.NewRequest(http.MethodPost, "/v1/lapangan/kode-etik/setuju", bytes.NewBufferString(`{}`))
		badReq.Header.Set("Content-Type", "application/json")
		wBad := httptest.NewRecorder()
		r.ServeHTTP(wBad, badReq)
		assert.Equal(t, http.StatusBadRequest, wBad.Code)

		okReq := httptest.NewRequest(http.MethodPost, "/v1/lapangan/kode-etik/setuju", bytes.NewBufferString(`{"versi":"v1.0"}`))
		okReq.Header.Set("Content-Type", "application/json")
		wOK := httptest.NewRecorder()
		r.ServeHTTP(wOK, okReq)
		assert.Equal(t, http.StatusOK, wOK.Code)
	})

	t.Run("GET kinerja", func(t *testing.T) {
		repo := &stubRepo{}
		repo.On("ListKinerja", mock.Anything, hargadomain.CakupanMilik(userID)).Return([]domain.KinerjaPetugas{
			{PetugasID: userID, NamaPetugas: "Petugas Satu", TotalEntri: 5, EntriValid: 4},
		}, nil)

		uc := usecase.New(repo, &audittest.Recorder{}, logger.NewDefault())
		r := setupRouter(New(uc), userID, "tugas.lihat_sendiri")

		w := httptest.NewRecorder()
		r.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/v1/lapangan/kinerja", nil))
		assert.Equal(t, http.StatusOK, w.Code)
	})
}
