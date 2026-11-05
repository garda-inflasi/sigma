// Package integration menguji API KPI terhadap PostgreSQL sungguhan: definisi
// dan target dari seeder referensi, nilai dari job yang menghitung data cold storage demo.
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
	coldseeder "github.com/garda-inflasi/api/internal/cold-storage/seeder"
	kpiRoutes "github.com/garda-inflasi/api/internal/kpi/routes"
	kpiseeder "github.com/garda-inflasi/api/internal/kpi/seeder"
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
	jwtSecret = "kpi-integration-secret-key-32chars!!!"

	// STRATEGIS_BI punya kpi.lihat; OPS_MAHASISWA tidak.
	emailLihat    = "strategis.bi@gardainflasi.id"
	emailTanpaIzn = "mahasiswa.1@gardainflasi.id"
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

func sharedSuite(t *testing.T) *suite {
	t.Helper()
	once.Do(func() {
		db := pg.CreateDatabase(t, "kpi_shared")
		pgtest.MigrateUp(t, db)
		now := time.Now()
		for _, seed := range []func() error{
			func() error { _, err := masterseeder.SeedReference(t.Context(), db); return err },
			func() error { _, err := authseeder.SeedDemo(t.Context(), db); return err },
			func() error { _, err := kpiseeder.SeedReference(t.Context(), db); return err },
			func() error { _, err := coldseeder.SeedReference(t.Context(), db); return err },
			func() error { _, err := coldseeder.SeedDemo(t.Context(), db, now); return err },
			func() error { _, err := kpiseeder.SeedDemo(t.Context(), db, now); return err },
		} {
			require.NoError(t, seed())
		}

		tokens, log := token.NewManager(jwtSecret), logger.NewDefault()
		router := gin.New()
		v1 := router.Group("/v1")
		authRoutes.Register(v1, &authRoutes.Dependencies{DB: db, Token: tokens, Logger: log})
		kpiRoutes.Register(v1, &kpiRoutes.Dependencies{DB: db, Token: tokens, Logger: log})
		shared = &suite{db: db, router: router, tokens: map[string]string{}}
	})
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

type nilaiRow struct {
	Periode string  `json:"periode"`
	Nilai   float64 `json:"nilai"`
	Versi   int     `json:"versi"`
}

type indikator struct {
	Kode     string `json:"kode"`
	Kelompok string `json:"kelompok"`
	Otomatis bool   `json:"otomatis"`
	Target   *struct {
		Nilai    float64  `json:"nilai"`
		Baseline *float64 `json:"baseline"`
	} `json:"target"`
	Nilai  *nilaiRow  `json:"nilai"`
	Tren   []nilaiRow `json:"tren"`
	Status string     `json:"status"`
}

func daftar(t *testing.T, w *httptest.ResponseRecorder) map[string]indikator {
	t.Helper()
	require.Equal(t, http.StatusOK, w.Code, w.Body.String())
	var res struct {
		Data []indikator `json:"data"`
	}
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &res), w.Body.String())
	by := map[string]indikator{}
	for _, i := range res.Data {
		by[i.Kode] = i
	}
	require.Len(t, by, len(res.Data), "kode ganda")
	return by
}

func TestKPI_Authorization(t *testing.T) {
	s := sharedSuite(t)

	for _, path := range []string{"/v1/kpi", "/v1/kpi/LOG-UTILISASI-CS/riwayat"} {
		assert.Equal(t, http.StatusUnauthorized, s.get(t, path, "").Code, path)
		assert.Equal(t, http.StatusForbidden, s.get(t, path, emailTanpaIzn).Code, path)
		assert.Equal(t, http.StatusOK, s.get(t, path, emailLihat).Code, path)
	}
}

func TestKPI_ListShowsComputedAndWaitingIndicators(t *testing.T) {
	s := sharedSuite(t)

	kpi := daftar(t, s.get(t, "/v1/kpi", emailLihat))

	require.Len(t, kpi, 17)
	util := kpi["LOG-UTILISASI-CS"]
	assert.Equal(t, "LOGISTIK", util.Kelompok)
	assert.True(t, util.Otomatis)
	require.NotNil(t, util.Nilai)
	assert.InDelta(t, 100.0*780/950, util.Nilai.Nilai, 1e-3, "dihitung job dari lot cold storage demo, bukan angka tulis tangan")
	assert.Equal(t, "TERCAPAI", util.Status, "82% melewati target PRD 75%")
	require.NotNil(t, util.Target)
	assert.InDelta(t, 75, util.Target.Nilai, 1e-9)
	assert.Nil(t, util.Target.Baseline, "baseline menunggu keputusan bisnis")
	assert.NotEmpty(t, util.Tren)

	// KPI berbasis model: ada definisi dan target, belum ada nilai.
	for _, kode := range []string{"DATA-MAPE-HARGA", "DATA-F1-EWS", "DATA-AUC-EWS", "DATA-RMSE-SHELFLIFE"} {
		model := kpi[kode]
		assert.False(t, model.Otomatis, kode)
		assert.Nil(t, model.Nilai, kode)
		assert.Equal(t, "MENUNGGU_DATA", model.Status, kode)
		assert.NotNil(t, model.Target, kode)
	}

	// Punya penghitung tetapi sumbernya (harga, intervensi) kosong: menunggu, bukan 0.
	assert.Equal(t, "MENUNGGU_DATA", kpi["DATA-KELENGKAPAN-HARIAN"].Status)
	assert.True(t, kpi["DATA-KELENGKAPAN-HARIAN"].Otomatis)
	assert.Equal(t, "MENUNGGU_DATA", kpi["TL-REKOMENDASI-AI"].Status)

	// Bernilai tetapi targetnya belum ditetapkan.
	assert.Equal(t, "TANPA_TARGET", kpi["LOG-RISIKO-FOODLOSS"].Status)
	assert.Nil(t, kpi["LOG-RISIKO-FOODLOSS"].Target)
}

func TestKPI_PeriodeFilter(t *testing.T) {
	s := sharedSuite(t)

	tahun := time.Now().Year() - 2
	lampau := daftar(t, s.get(t, "/v1/kpi?periode="+time.Date(tahun, 1, 1, 0, 0, 0, 0, time.UTC).Format("2006-01"), emailLihat))

	assert.Nil(t, lampau["LOG-UTILISASI-CS"].Nilai, "sebelum ada data tidak ada nilai")
	assert.Equal(t, "MENUNGGU_DATA", lampau["LOG-UTILISASI-CS"].Status)

	for _, query := range []string{"?periode=2026-13", "?periode=oktober", "?wilayah=abc", "?wilayah=11"} {
		assert.Equal(t, http.StatusBadRequest, s.get(t, "/v1/kpi"+query, emailLihat).Code, query)
	}
}

func TestKPI_RegionalViewHasNoProvinceValues(t *testing.T) {
	s := sharedSuite(t)

	kpi := daftar(t, s.get(t, "/v1/kpi?wilayah=1171", emailLihat))

	assert.Nil(t, kpi["LOG-UTILISASI-CS"].Nilai, "nilai provinsi tidak boleh tampil sebagai nilai kota")
	assert.NotNil(t, kpi["LOG-UTILISASI-CS"].Target, "target umum tetap berlaku untuk wilayah")
}

func TestKPI_Riwayat(t *testing.T) {
	s := sharedSuite(t)

	w := s.get(t, "/v1/kpi/LOG-UTILISASI-CS/riwayat", emailLihat)

	require.Equal(t, http.StatusOK, w.Code, w.Body.String())
	var res struct {
		Data struct {
			Kpi     indikator  `json:"kpi"`
			Riwayat []nilaiRow `json:"riwayat"`
		} `json:"data"`
	}
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &res))
	assert.Equal(t, "LOG-UTILISASI-CS", res.Data.Kpi.Kode)
	require.NotEmpty(t, res.Data.Riwayat)
	for i := 1; i < len(res.Data.Riwayat); i++ {
		assert.Less(t, res.Data.Riwayat[i-1].Periode, res.Data.Riwayat[i].Periode, "urut naik menurut periode")
	}

	assert.Equal(t, http.StatusNotFound, s.get(t, "/v1/kpi/TAK-ADA/riwayat", emailLihat).Code)

	menunggu := s.get(t, "/v1/kpi/DATA-MAPE-HARGA/riwayat", emailLihat)
	require.Equal(t, http.StatusOK, menunggu.Code)
	assert.Contains(t, menunggu.Body.String(), `"riwayat":[]`, "KPI menunggu data: riwayat kosong, bukan null")
}
