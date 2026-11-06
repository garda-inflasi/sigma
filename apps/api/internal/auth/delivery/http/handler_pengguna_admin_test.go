package http

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	nethttp "net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"

	"github.com/garda-inflasi/api/internal/audit/audittest"
	"github.com/garda-inflasi/api/internal/auth/domain"
	"github.com/garda-inflasi/api/internal/auth/usecase"
	"github.com/garda-inflasi/api/pkg/email"
	"github.com/garda-inflasi/api/pkg/logger"
	"github.com/garda-inflasi/api/pkg/token"
)

// repoAdmin menyediakan metode yang dipakai handler daftar, reset MFA, dan riwayat peran; metode lain panik.
type repoAdmin struct {
	domain.Repository
	daftar   func(domain.DaftarPenggunaQuery) ([]domain.Pengguna, int64, error)
	reset    func(string) (*domain.HasilResetMFA, error)
	riwayat  func(string, int) ([]domain.PerubahanPeran, error)
	pengguna func(string) (*domain.Pengguna, error)
	diterima domain.DaftarPenggunaQuery
}

func (r *repoAdmin) ListUsers(_ context.Context, q domain.DaftarPenggunaQuery) ([]domain.Pengguna, int64, error) {
	r.diterima = q
	return r.daftar(q)
}

func (r *repoAdmin) ResetMFA(_ context.Context, id string, _ time.Time) (*domain.HasilResetMFA, error) {
	return r.reset(id)
}

func (r *repoAdmin) RiwayatPeran(_ context.Context, id string, limit int) ([]domain.PerubahanPeran, error) {
	return r.riwayat(id, limit)
}

func (r *repoAdmin) GetUserByID(_ context.Context, id string) (*domain.Pengguna, error) {
	return r.pengguna(id)
}

func (r *repoAdmin) KunciAktif(context.Context, []string, time.Time) (map[string]time.Time, error) {
	return map[string]time.Time{}, nil
}

func routerAdmin(t *testing.T, repo domain.Repository) *gin.Engine {
	t.Helper()
	gin.SetMode(gin.TestMode)
	uc := usecase.New(repo, token.NewManager("test-secret-key-32-chars!!"), usecase.DefaultConfig(),
		logger.NewDefault(), email.New(email.Config{}), &audittest.Recorder{})
	h := New(uc)
	r := gin.New()
	r.Use(func(c *gin.Context) { c.Set("user_id", uid("admin-1")) })
	r.GET("/users", h.ListUsers)
	r.POST("/users/:id/reset-mfa", h.ResetMFAPengguna)
	r.GET("/users/:id/riwayat-peran", h.RiwayatPeranPengguna)
	return r
}

func panggil(t *testing.T, r *gin.Engine, method, path string) (*httptest.ResponseRecorder, map[string]any) {
	t.Helper()
	w := httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequest(method, path, nil))
	var out map[string]any
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &out), w.Body.String())
	return w, out
}

func TestListUsers_ParameterSalahDijawab400TanpaKeBasisData(t *testing.T) {
	repo := &repoAdmin{daftar: func(domain.DaftarPenggunaQuery) ([]domain.Pengguna, int64, error) {
		panic("tidak boleh sampai ke basis data")
	}}
	r := routerAdmin(t, repo)

	for nama, query := range map[string]string{
		"sort tak dikenal":   "sort=password_hash",
		"sort injeksi":       "sort=nama%3BDROP+TABLE+pengguna",
		"arah tak dikenal":   "sort=nama&arah=ASC",
		"peran tak dikenal":  "peran=SUPERADMIN",
		"status tak dikenal": "status=aktif",
		"lapis tak dikenal":  "lapis=OPS",
		"mfa bukan boolean":  "mfa=1",
		"terkunci kosong-ya": "terkunci=ya",
		"limit melebihi 100": "limit=101",
		"page negatif":       "page=-1",
		"search kepanjangan": "search=" + strings.Repeat("a", 101),
	} {
		w, body := panggil(t, r, nethttp.MethodGet, "/users?"+query)
		assert.Equal(t, nethttp.StatusBadRequest, w.Code, nama)
		assert.Equal(t, "400", body["code"], nama)
	}
}

func TestListUsers_PesanGalatMenyebutParameterYangSalah(t *testing.T) {
	r := routerAdmin(t, &repoAdmin{})

	w, body := panggil(t, r, nethttp.MethodGet, "/users?sort=rahasia")

	require.Equal(t, nethttp.StatusBadRequest, w.Code)
	assert.Contains(t, body["message"], "sort")
	assert.Contains(t, body["message"], "dibuat_pada", "pesan mencantumkan pilihan yang sah")
}

func TestListUsers_MeneruskanQueryTernormalDanMenjawabHalamanYangDipakai(t *testing.T) {
	repo := &repoAdmin{daftar: func(domain.DaftarPenggunaQuery) ([]domain.Pengguna, int64, error) {
		return []domain.Pengguna{{ID: uid("u-1"), Nama: "Ani", Email: "ani@garda.test", Peran: "OPS_MAHASISWA", Status: "AKTIF"}}, 41, nil
	}}
	r := routerAdmin(t, repo)

	w, body := panggil(t, r, nethttp.MethodGet, "/users?search=+ani+&peran=ADMIN_SISTEM&status=AKTIF&lapis=SISTEM&mfa=false&terkunci=true&sort=email&arah=desc&page=2&limit=10")

	require.Equal(t, nethttp.StatusOK, w.Code, w.Body.String())
	q := repo.diterima
	assert.Equal(t, "ani", q.Filter.Cari)
	assert.Equal(t, "ADMIN_SISTEM", q.Filter.Peran)
	assert.Equal(t, "AKTIF", q.Filter.Status)
	assert.Equal(t, "SISTEM", q.Filter.Lapis)
	require.NotNil(t, q.Filter.MFA)
	assert.False(t, *q.Filter.MFA)
	require.NotNil(t, q.Filter.Terkunci)
	assert.True(t, *q.Filter.Terkunci)
	assert.Equal(t, "email", q.Urut)
	assert.Equal(t, "desc", q.Arah)
	assert.Equal(t, 2, q.Page)
	assert.Equal(t, 10, q.Limit)
	assert.False(t, q.Sekarang.IsZero(), "jam acuan filter terkunci terisi")

	data := body["data"].(map[string]any)
	assert.EqualValues(t, 41, data["total"])
	assert.EqualValues(t, 2, data["page"])
	assert.EqualValues(t, 10, data["limit"])
	assert.Len(t, data["items"], 1)
}

func TestListUsers_TanpaParameterMemakaiBawaanLamaTerbaruDuluan(t *testing.T) {
	repo := &repoAdmin{daftar: func(domain.DaftarPenggunaQuery) ([]domain.Pengguna, int64, error) {
		return []domain.Pengguna{}, 0, nil
	}}
	r := routerAdmin(t, repo)

	w, body := panggil(t, r, nethttp.MethodGet, "/users")

	require.Equal(t, nethttp.StatusOK, w.Code)
	assert.Equal(t, "dibuat_pada", repo.diterima.Urut)
	assert.Equal(t, "desc", repo.diterima.Arah)
	data := body["data"].(map[string]any)
	assert.EqualValues(t, 1, data["page"])
	assert.EqualValues(t, 20, data["limit"])
	assert.Empty(t, data["items"])
}

func TestListUsers_GalatBasisDataDijawab500TanpaMembocorkanDetail(t *testing.T) {
	r := routerAdmin(t, &repoAdmin{daftar: func(domain.DaftarPenggunaQuery) ([]domain.Pengguna, int64, error) {
		return nil, 0, errors.New("pq: relation pengguna rusak")
	}})

	w, body := panggil(t, r, nethttp.MethodGet, "/users")

	assert.Equal(t, nethttp.StatusInternalServerError, w.Code)
	assert.Equal(t, "500", body["code"])
	assert.NotContains(t, w.Body.String(), "pengguna rusak")
}

func TestResetMFA_Handler(t *testing.T) {
	cases := []struct {
		nama       string
		id         string
		reset      func(string) (*domain.HasilResetMFA, error)
		wantStatus int
	}{
		{"berhasil", uid("u-1"), func(string) (*domain.HasilResetMFA, error) {
			return &domain.HasilResetMFA{MFAAktifSebelumnya: true, SesiDicabut: []string{"a", "b"}}, nil
		}, nethttp.StatusOK},
		{"tanpa MFA tetap 200", uid("u-1"), func(string) (*domain.HasilResetMFA, error) { return &domain.HasilResetMFA{}, nil }, nethttp.StatusOK},
		{"tidak ada", uid("hilang"), func(string) (*domain.HasilResetMFA, error) {
			return nil, fmt.Errorf("reset mfa: %w", gorm.ErrRecordNotFound)
		}, nethttp.StatusNotFound},
		{"akun sendiri", uid("admin-1"), func(string) (*domain.HasilResetMFA, error) { panic("tidak boleh dipanggil") }, nethttp.StatusBadRequest},
		{"galat basis data", uid("u-1"), func(string) (*domain.HasilResetMFA, error) { return nil, errors.New("pq: rahasia-internal") }, nethttp.StatusInternalServerError},
	}
	for _, tc := range cases {
		t.Run(tc.nama, func(t *testing.T) {
			r := routerAdmin(t, &repoAdmin{reset: tc.reset})

			w, body := panggil(t, r, nethttp.MethodPost, "/users/"+tc.id+"/reset-mfa")

			assert.Equal(t, tc.wantStatus, w.Code, w.Body.String())
			assert.NotContains(t, w.Body.String(), "rahasia-internal")
			if tc.wantStatus == nethttp.StatusOK {
				data := body["data"].(map[string]any)
				assert.Equal(t, tc.id, data["id"])
				assert.Contains(t, data, "mfa_aktif_sebelumnya")
				assert.Contains(t, data, "sesi_dicabut")
			}
		})
	}
}

func TestResetMFA_PesanAkunSendiriMenunjukkanJalurPemilik(t *testing.T) {
	r := routerAdmin(t, &repoAdmin{})

	w, body := panggil(t, r, nethttp.MethodPost, "/users/"+uid("admin-1")+"/reset-mfa")

	require.Equal(t, nethttp.StatusBadRequest, w.Code)
	assert.Contains(t, body["message"], "akun sendiri")
}

func TestRiwayatPeran_Handler(t *testing.T) {
	dari, id, nama := "OPS_MAHASISWA", uid("admin-1"), "Admin Satu"
	repo := &repoAdmin{
		pengguna: func(id string) (*domain.Pengguna, error) {
			if id == uid("hilang") {
				return nil, gorm.ErrRecordNotFound
			}
			return &domain.Pengguna{ID: id}, nil
		},
		riwayat: func(penggunaID string, limit int) ([]domain.PerubahanPeran, error) {
			if penggunaID == uid("rusak") {
				return nil, errors.New("pq: rahasia-internal")
			}
			return []domain.PerubahanPeran{
				{AuditID: "a-2", Waktu: time.Date(2026, 11, 6, 9, 0, 0, 0, time.UTC), Aksi: "ASSIGN_ROLE", Dari: &dari, Ke: "MANAJER_DATA", AktorID: &id, AktorNama: &nama},
				{AuditID: "a-1", Waktu: time.Date(2026, 11, 5, 9, 0, 0, 0, time.UTC), Aksi: "CREATE_USER", Ke: "OPS_MAHASISWA"},
			}, nil
		},
	}
	r := routerAdmin(t, repo)

	t.Run("berhasil", func(t *testing.T) {
		w, body := panggil(t, r, nethttp.MethodGet, "/users/"+uid("u-1")+"/riwayat-peran")

		require.Equal(t, nethttp.StatusOK, w.Code, w.Body.String())
		data := body["data"].(map[string]any)
		assert.Equal(t, false, data["terpotong"])
		items := data["items"].([]any)
		require.Len(t, items, 2)
		pertama := items[0].(map[string]any)
		assert.Equal(t, "a-2", pertama["id"])
		assert.Equal(t, "ASSIGN_ROLE", pertama["aksi"])
		assert.Equal(t, "OPS_MAHASISWA", pertama["dari"])
		assert.Equal(t, "MANAJER_DATA", pertama["ke"])
		assert.Equal(t, map[string]any{"id": uid("admin-1"), "nama": "Admin Satu"}, pertama["aktor"])
		kedua := items[1].(map[string]any)
		assert.Nil(t, kedua["dari"])
		assert.Nil(t, kedua["aktor"])
	})
	t.Run("pengguna tidak ada", func(t *testing.T) {
		w, _ := panggil(t, r, nethttp.MethodGet, "/users/"+uid("hilang")+"/riwayat-peran")
		assert.Equal(t, nethttp.StatusNotFound, w.Code)
	})
	t.Run("galat basis data", func(t *testing.T) {
		w, _ := panggil(t, r, nethttp.MethodGet, "/users/"+uid("rusak")+"/riwayat-peran")
		assert.Equal(t, nethttp.StatusInternalServerError, w.Code)
		assert.NotContains(t, w.Body.String(), "rahasia-internal")
	})
	t.Run("limit di luar rentang", func(t *testing.T) {
		for _, q := range []string{"limit=1.5", "limit=201", "limit=-1", "limit=abc"} {
			w, _ := panggil(t, r, nethttp.MethodGet, "/users/"+uid("u-1")+"/riwayat-peran?"+q)
			assert.Equal(t, nethttp.StatusBadRequest, w.Code, q)
		}
	})
}
