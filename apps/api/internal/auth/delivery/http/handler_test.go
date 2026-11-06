package http

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	nethttp "net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/garda-inflasi/api/internal/audit/audittest"
	"github.com/garda-inflasi/api/internal/auth/domain"
	"github.com/garda-inflasi/api/internal/auth/usecase"
	"github.com/garda-inflasi/api/pkg/email"
	"github.com/garda-inflasi/api/pkg/logger"
	"github.com/garda-inflasi/api/pkg/token"
)

// repoPalsu menyediakan hanya metode yang dipakai handler yang diuji; domain.Repository yang tertanam bernilai
// nil, jadi pemanggilan metode lain panik dan tes gagal (bukan diam-diam lolos).
type repoPalsu struct {
	domain.Repository
	bulk      func() (int64, error)
	byEmail   func() (*domain.Pengguna, error)
	createErr error
}

func (r *repoPalsu) BulkUpdateUsers(context.Context, []string, domain.BulkUserAction) (int64, error) {
	return r.bulk()
}

func (r *repoPalsu) GetUserByEmail(context.Context, string) (*domain.Pengguna, error) {
	return r.byEmail()
}

func (r *repoPalsu) CreateUser(context.Context, *domain.Pengguna) error { return r.createErr }

func router(t *testing.T, repo domain.Repository) *gin.Engine {
	t.Helper()
	gin.SetMode(gin.TestMode)
	uc := usecase.New(repo, token.NewManager("test-secret-key-32-chars!!"), usecase.DefaultConfig(),
		logger.NewDefault(), email.New(email.Config{}), &audittest.Recorder{})
	h := New(uc)
	r := gin.New()
	r.Use(func(c *gin.Context) { c.Set("user_id", "admin-1") })
	r.POST("/users", h.CreateUser)
	r.POST("/users/bulk", h.BulkUser)
	return r
}

func kirim(t *testing.T, r *gin.Engine, path string, body any) (*httptest.ResponseRecorder, map[string]any) {
	t.Helper()
	raw, err := json.Marshal(body)
	require.NoError(t, err)
	req := httptest.NewRequest(nethttp.MethodPost, path, bytes.NewReader(raw))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	var out map[string]any
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &out), w.Body.String())
	return w, out
}

func penggunaBaru(password string) map[string]any {
	return map[string]any{
		"id": "u-baru", "nama": "Baru", "email": "baru@garda.test", "password": password,
		"peran": "OPS_MAHASISWA", "lapis": "OPERASIONAL", "lembaga": "BPPSDMP",
	}
}

// Regresi: kata sandi lemah pada pembuatan pengguna dulu dijawab 500 "Gagal membuat pengguna" karena handler
// membandingkan galat dengan == dan galat usecase dibungkus alasannya.
func TestCreateUser_KataSandiLemahDijawab400BukanGalatServer(t *testing.T) {
	r := router(t, &repoPalsu{})

	for _, lemah := range []string{"semuahurufkecil", "TanpaAngka!!", "TanpaKhusus123", "password"} {
		w, body := kirim(t, r, "/users", penggunaBaru(lemah))

		assert.Equal(t, nethttp.StatusBadRequest, w.Code, lemah)
		assert.Equal(t, "400", body["code"])
		assert.Contains(t, body["message"], "kata sandi", "pesan memuat alasan kebijakan")
	}
}

func TestCreateUser_EmailSudahAdaDijawab409(t *testing.T) {
	r := router(t, &repoPalsu{byEmail: func() (*domain.Pengguna, error) { return &domain.Pengguna{ID: "lain"}, nil }})

	w, _ := kirim(t, r, "/users", penggunaBaru("Kuat#12345"))

	assert.Equal(t, nethttp.StatusConflict, w.Code)
}

func TestCreateUser_GalatPenyimpananDijawab500TanpaMembocorkanDetail(t *testing.T) {
	r := router(t, &repoPalsu{
		byEmail:   func() (*domain.Pengguna, error) { return nil, errors.New("tidak ada") },
		createErr: errors.New("pq: rahasia-skema-internal"),
	})

	w, body := kirim(t, r, "/users", penggunaBaru("Kuat#12345"))

	assert.Equal(t, nethttp.StatusInternalServerError, w.Code)
	assert.NotContains(t, w.Body.String(), "rahasia-skema-internal")
	assert.Equal(t, "500", body["code"])
}

func TestCreateUser_BerhasilDijawab201(t *testing.T) {
	r := router(t, &repoPalsu{byEmail: func() (*domain.Pengguna, error) { return nil, errors.New("tidak ada") }})

	w, _ := kirim(t, r, "/users", penggunaBaru("Kuat#12345"))

	assert.Equal(t, nethttp.StatusCreated, w.Code)
}

// Regresi: galat basis data pada aksi massal dulu dijawab 400 dengan teks galat mentah ("bulk user action: ...").
func TestBulkUser_GalatBasisDataDijawab500TanpaMembocorkanDetail(t *testing.T) {
	r := router(t, &repoPalsu{bulk: func() (int64, error) { return 0, errors.New("pq: relation pengguna rusak") }})

	w, body := kirim(t, r, "/users/bulk", map[string]any{"action": "nonaktifkan", "ids": []string{"a"}})

	assert.Equal(t, nethttp.StatusInternalServerError, w.Code)
	assert.Equal(t, "500", body["code"])
	assert.NotContains(t, w.Body.String(), "pengguna rusak")
}

func TestBulkUser_BerhasilMelaporkanJumlahYangBerubah(t *testing.T) {
	r := router(t, &repoPalsu{bulk: func() (int64, error) { return 2, nil }})

	w, body := kirim(t, r, "/users/bulk", map[string]any{"action": "aktifkan", "ids": []string{"a", "b", "c"}})

	require.Equal(t, nethttp.StatusOK, w.Code)
	assert.EqualValues(t, 2, body["data"].(map[string]any)["berhasil"])
}

func TestBulkUser_AksiTakDikenalDanIdKosongDitolak400(t *testing.T) {
	r := router(t, &repoPalsu{bulk: func() (int64, error) { panic("tidak boleh dipanggil") }})

	w, _ := kirim(t, r, "/users/bulk", map[string]any{"action": "hapus", "ids": []string{"a"}})
	assert.Equal(t, nethttp.StatusBadRequest, w.Code)

	w, _ = kirim(t, r, "/users/bulk", map[string]any{"action": "aktifkan", "ids": []string{}})
	assert.Equal(t, nethttp.StatusBadRequest, w.Code)
}

// Penulisan bersyarat yang kalah balapan dijawab 409 (bukan 500): setup MFA yang diganti permintaan lain dan
// aktivasi yang sudah dilakukan permintaan lain adalah konflik yang bisa diulang pengguna.
func TestMFAError_KonflikPenulisanSerentakDijawab409(t *testing.T) {
	gin.SetMode(gin.TestMode)
	for _, err := range []error{usecase.ErrMFASetupBerubah, usecase.ErrMFAAlreadyActive, errors.Join(usecase.ErrMFASetupBerubah, errors.New("lain"))} {
		w := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(w)

		jawab := mfaError(c, err)

		assert.True(t, jawab)
		assert.Equal(t, nethttp.StatusConflict, w.Code, err.Error())
	}
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	assert.True(t, mfaError(c, errors.New("db putus")))
	assert.Equal(t, nethttp.StatusInternalServerError, w.Code, "galat lain tetap 500")
}
