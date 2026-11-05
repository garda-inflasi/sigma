// Package integration menguji GET /v1/dashboard/eksekutif terhadap PostgreSQL
// sungguhan dengan data seeder dan akun demo.
package integration

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"sync"
	"testing"
	"time"

	authRoutes "github.com/garda-inflasi/api/internal/auth/routes"
	authseeder "github.com/garda-inflasi/api/internal/auth/seeder"
	eksekutifRoutes "github.com/garda-inflasi/api/internal/eksekutif/routes"
	eksekutifseeder "github.com/garda-inflasi/api/internal/eksekutif/seeder"
	masterseeder "github.com/garda-inflasi/api/internal/master/seeder"
	"github.com/garda-inflasi/api/internal/platform/pgtest"
	"github.com/garda-inflasi/api/pkg/logger"
	"github.com/garda-inflasi/api/pkg/token"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

const (
	jwtSecret = "eksekutif-integration-secret-key-32chars!"

	// STRATEGIS_BI: dashboard.eksekutif + fiskal.lihat. MANAJER_DATA dan OPS_MAHASISWA tidak punya dashboard.eksekutif.
	emailEksekutif = "strategis.bi@gardainflasi.id"
	emailTanpaIzin = "manajer.data@gardainflasi.id"
)

var pg *pgtest.Server

func TestMain(m *testing.M) {
	gin.SetMode(gin.TestMode)
	os.Exit(pgtest.Main(m, &pg))
}

type suite struct {
	db     *gorm.DB
	router *gin.Engine
	tokens map[string]string
}

var (
	once   sync.Once
	shared *suite
)

func newRouter(t *testing.T, db *gorm.DB) *gin.Engine {
	t.Helper()
	tokens, log := token.NewManager(jwtSecret), logger.NewDefault()
	router := gin.New()
	v1 := router.Group("/v1")
	authRoutes.Register(v1, &authRoutes.Dependencies{DB: db, Token: tokens, Logger: log})
	eksekutifRoutes.Register(v1, &eksekutifRoutes.Dependencies{DB: db, Token: tokens, Logger: log})
	return router
}

func seeded(t *testing.T, name string, demo bool) *suite {
	t.Helper()
	db := pg.CreateDatabase(t, name)
	pgtest.MigrateUp(t, db)
	_, err := masterseeder.SeedReference(t.Context(), db)
	require.NoError(t, err)
	_, err = authseeder.SeedDemo(t.Context(), db)
	require.NoError(t, err)
	_, err = eksekutifseeder.SeedReference(t.Context(), db)
	require.NoError(t, err)
	if demo {
		_, err = eksekutifseeder.SeedDemo(t.Context(), db, time.Now())
		require.NoError(t, err)
	}
	return &suite{db: db, router: newRouter(t, db), tokens: map[string]string{}}
}

func sharedSuite(t *testing.T) *suite {
	t.Helper()
	once.Do(func() { shared = seeded(t, "eksekutif_shared", true) })
	require.NotNil(t, shared, "suite bersama gagal dibuat")
	return shared
}

func (s *suite) get(t *testing.T, path, email string) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequest(http.MethodGet, path, nil)
	if email != "" {
		req.Header.Set("Authorization", "Bearer "+s.login(t, email))
	}
	w := httptest.NewRecorder()
	s.router.ServeHTTP(w, req)
	return w
}

func (s *suite) login(t *testing.T, email string) string {
	t.Helper()
	if tok, ok := s.tokens[email]; ok {
		return tok
	}
	body, _ := json.Marshal(map[string]string{"email": email, "password": authseeder.DemoPassword})
	req := httptest.NewRequest(http.MethodPost, "/v1/auth/login", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	s.router.ServeHTTP(w, req)
	require.Equal(t, http.StatusOK, w.Code, w.Body.String())
	var res struct {
		Data struct {
			AccessToken string `json:"access_token"`
		} `json:"data"`
	}
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &res))
	s.tokens[email] = res.Data.AccessToken
	return res.Data.AccessToken
}

type ringkasan struct {
	Demo    bool `json:"demo"`
	Inflasi *struct {
		Periode string  `json:"periode"`
		Sumber  string  `json:"sumber"`
		Yoy     float64 `json:"yoy"`
		Sasaran struct {
			Bawah float64 `json:"bawah"`
			Atas  float64 `json:"atas"`
		} `json:"sasaran"`
		Seri       []struct{ Periode string } `json:"seri"`
		Penyumbang []struct {
			Kode  string  `json:"kode"`
			Andil float64 `json:"andil"`
		} `json:"penyumbang"`
	} `json:"inflasi"`
	Neraca *struct {
		Sumber string `json:"sumber"`
		Items  []struct {
			KomoditasKode string  `json:"komoditas_kode"`
			ProduksiTon   float64 `json:"produksi_ton"`
		} `json:"items"`
	} `json:"neraca"`
	Fiskal *struct {
		Sumber    string `json:"sumber"`
		Pagu      int64  `json:"pagu"`
		Realisasi int64  `json:"realisasi"`
		Status    string `json:"status"`
		Instrumen []struct{ Nama string }
	} `json:"fiskal"`
	EWS           map[string]int `json:"ews"`
	Intervensi    map[string]struct{ Aktif, Selesai int }
	SiklusDominan string `json:"siklus_dominan"`
}

func decode(t *testing.T, w *httptest.ResponseRecorder) ringkasan {
	t.Helper()
	var res struct {
		Data ringkasan `json:"data"`
	}
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &res), w.Body.String())
	return res.Data
}

func TestEksekutif_Authorization(t *testing.T) {
	s := sharedSuite(t)

	assert.Equal(t, http.StatusUnauthorized, s.get(t, "/v1/dashboard/eksekutif", "").Code)
	assert.Equal(t, http.StatusForbidden, s.get(t, "/v1/dashboard/eksekutif", emailTanpaIzin).Code)
	assert.Equal(t, http.StatusOK, s.get(t, "/v1/dashboard/eksekutif", emailEksekutif).Code)
}

func TestEksekutif_DemoDataIsLabelledAndComplete(t *testing.T) {
	s := sharedSuite(t)

	res := decode(t, s.get(t, "/v1/dashboard/eksekutif", emailEksekutif))

	assert.True(t, res.Demo, "data seed demo wajib berlabel demo")
	require.NotNil(t, res.Inflasi)
	assert.Equal(t, "DEMO", res.Inflasi.Sumber)
	assert.InDelta(t, 2.84, res.Inflasi.Yoy, 1e-9)
	assert.InDelta(t, 1.5, res.Inflasi.Sasaran.Bawah, 1e-9)
	assert.InDelta(t, 3.5, res.Inflasi.Sasaran.Atas, 1e-9)
	assert.Len(t, res.Inflasi.Seri, 12)
	require.Len(t, res.Inflasi.Penyumbang, 6)
	assert.Equal(t, "CBM-KRT", res.Inflasi.Penyumbang[0].Kode, "andil terbesar di atas")
	assert.Equal(t, "TMT", res.Inflasi.Penyumbang[5].Kode, "andil paling negatif di bawah")

	require.NotNil(t, res.Neraca)
	assert.Equal(t, "DEMO", res.Neraca.Sumber)
	assert.Len(t, res.Neraca.Items, 8)

	require.NotNil(t, res.Fiskal)
	assert.Equal(t, "DEMO", res.Fiskal.Sumber)
	assert.EqualValues(t, 186_000_000_000, res.Fiskal.Pagu)
	assert.EqualValues(t, 54_200_000_000, res.Fiskal.Realisasi)
	assert.Equal(t, "MEMADAI", res.Fiskal.Status)
	assert.Len(t, res.Fiskal.Instrumen, 4)

	assert.Equal(t, map[string]int{"INFO": 0, "WASPADA": 0, "SIAGA": 0, "KRITIS": 0}, res.EWS, "tanpa data EWS: nol, bukan galat")
	assert.Equal(t, "normal", res.SiklusDominan)
	assert.Len(t, res.Intervensi, 3)
}

func TestEksekutif_EmptyDatabaseReturnsEmptyDashboard(t *testing.T) {
	s := seeded(t, "eksekutif_kosong", false)

	w := s.get(t, "/v1/dashboard/eksekutif", emailEksekutif)

	require.Equal(t, http.StatusOK, w.Code, w.Body.String())
	res := decode(t, w)
	assert.False(t, res.Demo)
	assert.Nil(t, res.Inflasi)
	assert.Nil(t, res.Neraca)
	assert.Nil(t, res.Fiskal)
}

func TestEksekutif_OfficialDataIsNotLabelledDemo(t *testing.T) {
	s := seeded(t, "eksekutif_resmi", false)
	require.NoError(t, s.db.Exec(`
		INSERT INTO inflasi_bulanan (wilayah_kode, periode, ihk, inflasi_mtm, inflasi_yoy, inflasi_ytd, sumber)
		VALUES ('11', '2026-09-01', 110.2, 0.3, 2.9, 1.8, 'BPS')`).Error)

	res := decode(t, s.get(t, "/v1/dashboard/eksekutif", emailEksekutif))

	require.NotNil(t, res.Inflasi)
	assert.Equal(t, "BPS", res.Inflasi.Sumber)
	assert.False(t, res.Demo)
	assert.Equal(t, "2026-09-01", res.Inflasi.Periode)
}

func TestEksekutif_LatestPeriodWins(t *testing.T) {
	s := seeded(t, "eksekutif_periode", false)
	require.NoError(t, s.db.Exec(`
		INSERT INTO inflasi_bulanan (wilayah_kode, periode, ihk, inflasi_mtm, inflasi_yoy, inflasi_ytd, sumber) VALUES
		('11', '2026-07-01', 109, 0.1, 2.0, 1.0, 'BPS'),
		('11', '2026-08-01', 109.5, 0.2, 2.2, 1.2, 'BPS'),
		('1171', '2026-09-01', 120, 0.9, 4.0, 3.0, 'BPS')`).Error)

	res := decode(t, s.get(t, "/v1/dashboard/eksekutif", emailEksekutif))

	require.NotNil(t, res.Inflasi)
	assert.Equal(t, "2026-08-01", res.Inflasi.Periode, "baris kabupaten (1171) tidak boleh mengalahkan provinsi")
	assert.Len(t, res.Inflasi.Seri, 2)
}
