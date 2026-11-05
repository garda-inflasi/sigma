// Package integration menguji dashboard operasional dan siklus terhadap
// PostgreSQL sungguhan: agregat dihitung dari entri harga demo.
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
	dashboardRoutes "github.com/garda-inflasi/api/internal/dashboard/routes"
	dashboardseeder "github.com/garda-inflasi/api/internal/dashboard/seeder"
	hargaRoutes "github.com/garda-inflasi/api/internal/harga/routes"
	hargaseeder "github.com/garda-inflasi/api/internal/harga/seeder"
	lapanganseeder "github.com/garda-inflasi/api/internal/lapangan/seeder"
	masterseeder "github.com/garda-inflasi/api/internal/master/seeder"
	"github.com/garda-inflasi/api/internal/platform/pgtest"
	siklusRoutes "github.com/garda-inflasi/api/internal/siklus/routes"
	"github.com/garda-inflasi/api/pkg/logger"
	"github.com/garda-inflasi/api/pkg/token"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

const (
	jwtSecret = "dashboard-integration-secret-key-32chars!"

	// STRATEGIS_BI memegang dashboard.operasional; OPS_MAHASISWA tidak.
	emailBerhak      = "strategis.bi@gardainflasi.id"
	emailTakBerhak   = "mahasiswa.1@gardainflasi.id"
	emailKoordinator = "koordinator.lapangan@gardainflasi.id"
	emailManajerData = "manajer.data@gardainflasi.id"
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

// newSuite menyiapkan basis data berisi master dan akun demo; seed true menambah data harga, cold storage, tugas, dan dashboard.
func newSuite(t *testing.T, name string, seed bool) *suite {
	t.Helper()
	db := pg.CreateDatabase(t, name)
	pgtest.MigrateUp(t, db)
	_, err := masterseeder.SeedReference(t.Context(), db)
	require.NoError(t, err)
	_, err = authseeder.SeedDemo(t.Context(), db)
	require.NoError(t, err)
	if seed {
		now := time.Now()
		ctx := t.Context()
		_, err = hargaseeder.SeedDemo(ctx, db, now)
		require.NoError(t, err)
		_, err = coldseeder.SeedReference(ctx, db)
		require.NoError(t, err)
		_, err = coldseeder.SeedDemo(ctx, db, now)
		require.NoError(t, err)
		_, err = lapanganseeder.SeedDemo(ctx, db, now)
		require.NoError(t, err)
		_, err = dashboardseeder.SeedReference(ctx, db)
		require.NoError(t, err)
		_, err = dashboardseeder.SeedDemo(ctx, db, now)
		require.NoError(t, err)
	}

	tokens, log := token.NewManager(jwtSecret), logger.NewDefault()
	router := gin.New()
	v1 := router.Group("/v1")
	authRoutes.Register(v1, &authRoutes.Dependencies{DB: db, Token: tokens, Logger: log})
	dashboardRoutes.Register(v1, &dashboardRoutes.Dependencies{DB: db, Token: tokens, Logger: log})
	siklusRoutes.Register(v1, &siklusRoutes.Dependencies{DB: db, Token: tokens, Logger: log})
	hargaRoutes.Register(v1, &hargaRoutes.Dependencies{DB: db, Token: tokens, Logger: log})
	return &suite{db: db, router: router, tokens: map[string]string{}}
}

var (
	once   sync.Once
	shared *suite
)

func sharedSuite(t *testing.T) *suite {
	t.Helper()
	once.Do(func() { shared = newSuite(t, "dashboard_shared", true) })
	require.NotNil(t, shared, "suite bersama gagal dibuat")
	return shared
}

func (s *suite) call(t *testing.T, method, path, email string, body any) *httptest.ResponseRecorder {
	t.Helper()
	var payload bytes.Buffer
	if body != nil {
		require.NoError(t, json.NewEncoder(&payload).Encode(body))
	}
	req := httptest.NewRequest(method, path, &payload)
	req.Header.Set("Content-Type", "application/json")
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

func data[T any](t *testing.T, w *httptest.ResponseRecorder) T {
	t.Helper()
	var res struct {
		Data T `json:"data"`
	}
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &res), w.Body.String())
	return res.Data
}

type kpi struct {
	ID              string    `json:"id"`
	Label           string    `json:"label"`
	Nilai           float64   `json:"nilai"`
	Format          string    `json:"format"`
	Delta           float64   `json:"delta"`
	KeteranganDelta string    `json:"keterangan_delta"`
	Tren            []float64 `json:"tren"`
	Tone            string    `json:"tone"`
	Href            string    `json:"href"`
}

type heatmap struct {
	Kabupaten []struct {
		Kode string `json:"kode"`
		Nama string `json:"nama"`
	} `json:"kabupaten"`
	Komoditas []struct {
		Kode  string `json:"kode"`
		Pilot bool   `json:"pilot"`
	} `json:"komoditas"`
	Sel []struct {
		KomoditasKode string  `json:"komoditas_kode"`
		KabupatenKode string  `json:"kabupaten_kode"`
		Delta         float64 `json:"delta"`
		Harga         float64 `json:"harga"`
	} `json:"sel"`
}

func TestKpi_FourCardsFromData(t *testing.T) {
	s := sharedSuite(t)

	w := s.call(t, http.MethodGet, "/v1/dashboard/operasional/kpi", emailBerhak, nil)

	require.Equal(t, http.StatusOK, w.Code, w.Body.String())
	kpis := data[[]kpi](t, w)
	require.Len(t, kpis, 4)
	byID := map[string]kpi{}
	for _, k := range kpis {
		byID[k.ID] = k
		assert.Len(t, k.Tren, 14, k.ID)
		assert.NotEmpty(t, k.Label)
		assert.NotEmpty(t, k.KeteranganDelta)
		assert.Contains(t, []string{"data", "normal", "waspada", "defisit"}, k.Tone)
	}
	assert.Equal(t, []string{"entri", "validasi", "waspada", "cs"}, []string{kpis[0].ID, kpis[1].ID, kpis[2].ID, kpis[3].ID})

	assert.Positive(t, byID["entri"].Nilai, "ada entri hari ini")
	assert.Equal(t, "angka", byID["entri"].Format)
	assert.Equal(t, "data", byID["entri"].Tone, "entri tidak punya target")
	assert.Equal(t, "persen", byID["validasi"].Format)
	assert.InDelta(t, 80, byID["validasi"].Nilai, 20, "histori didominasi entri yang sudah terbit")
	assert.Positive(t, byID["waspada"].Nilai, "cabai sedang melonjak")
	assert.Positive(t, byID["cs"].Nilai)
	assert.LessOrEqual(t, byID["cs"].Nilai, 100.0)
	assert.Equal(t, "/data/harga?status=DIAJUKAN", byID["validasi"].Href)
}

func TestHeatmap_CabaiNaikTomatTurun(t *testing.T) {
	s := sharedSuite(t)

	w := s.call(t, http.MethodGet, "/v1/dashboard/operasional/heatmap", emailBerhak, nil)

	require.Equal(t, http.StatusOK, w.Code, w.Body.String())
	h := data[heatmap](t, w)
	assert.Len(t, h.Kabupaten, 10, "hanya kabupaten yang punya data harga")
	assert.Len(t, h.Komoditas, 14)
	assert.GreaterOrEqual(t, len(h.Sel), 100)

	kab, kom := map[string]bool{}, map[string]bool{}
	for _, k := range h.Kabupaten {
		kab[k.Kode] = true
	}
	for _, k := range h.Komoditas {
		kom[k.Kode] = true
	}
	var cabai, tomat []float64
	for _, c := range h.Sel {
		assert.True(t, kab[c.KabupatenKode], "sel merujuk kabupaten yang tidak tercantum: %s", c.KabupatenKode)
		assert.True(t, kom[c.KomoditasKode], "sel merujuk komoditas yang tidak tercantum: %s", c.KomoditasKode)
		assert.Positive(t, c.Harga)
		switch c.KomoditasKode {
		case "CBM-KRT":
			cabai = append(cabai, c.Delta)
		case "TMT":
			tomat = append(tomat, c.Delta)
		}
	}
	assert.Greater(t, mean(cabai), 3.0, "cabai merah dalam tekanan")
	assert.Less(t, mean(tomat), -3.0, "tomat surplus")
}

func mean(v []float64) float64 {
	if len(v) == 0 {
		return 0
	}
	var total float64
	for _, x := range v {
		total += x
	}
	return total / float64(len(v))
}

func TestAktivitasLapangan(t *testing.T) {
	s := sharedSuite(t)

	w := s.call(t, http.MethodGet, "/v1/dashboard/operasional/aktivitas-lapangan", emailBerhak, nil)

	require.Equal(t, http.StatusOK, w.Code, w.Body.String())
	items := data[[]struct {
		KabupatenKode string `json:"kabupaten_kode"`
		KabupatenNama string `json:"kabupaten_nama"`
		JumlahPetugas int    `json:"jumlah_petugas"`
		Entri         int    `json:"entri"`
		Target        int    `json:"target"`
	}](t, w)
	assert.Len(t, items, 10, "satu baris per kabupaten berpetugas")
	for _, a := range items {
		assert.Equal(t, 1, a.JumlahPetugas, a.KabupatenKode)
		assert.Equal(t, 14, a.Target, "satu petugas kali 14 komoditas aktif: %s", a.KabupatenKode)
		assert.NotEmpty(t, a.KabupatenNama)
		assert.LessOrEqual(t, a.Entri, a.Target+2, "entri tidak jauh melebihi target (duplikat saja): %s", a.KabupatenKode)
	}
}

func TestSiklusAktif(t *testing.T) {
	s := sharedSuite(t)

	w := s.call(t, http.MethodGet, "/v1/siklus/aktif", emailBerhak, nil)

	require.Equal(t, http.StatusOK, w.Code, w.Body.String())
	siklus := data[struct {
		Siklus string `json:"siklus"`
		Sejak  string `json:"sejak"`
	}](t, w)
	assert.Equal(t, "tekanan", siklus.Siklus)
	sejak, err := time.Parse("2006-01-02", siklus.Sejak)
	require.NoError(t, err)
	assert.WithinDuration(t, time.Now().AddDate(0, 0, -6), sejak, 48*time.Hour)

	assert.Equal(t, http.StatusOK, s.call(t, http.MethodGet, "/v1/siklus/aktif?wilayah_kode=11", emailBerhak, nil).Code)
	assert.Equal(t, http.StatusNotFound, s.call(t, http.MethodGet, "/v1/siklus/aktif?wilayah_kode=99", emailBerhak, nil).Code)
}

func TestAuthorization(t *testing.T) {
	s := sharedSuite(t)

	for _, path := range []string{
		"/v1/dashboard/operasional/kpi",
		"/v1/dashboard/operasional/heatmap",
		"/v1/dashboard/operasional/aktivitas-lapangan",
		"/v1/siklus/aktif",
	} {
		assert.Equal(t, http.StatusForbidden, s.call(t, http.MethodGet, path, emailTakBerhak, nil).Code, path)
		assert.Equal(t, http.StatusUnauthorized, s.call(t, http.MethodGet, path, "", nil).Code, path)
	}
}

func TestSiklus_OnlyOneActiveCyclePerRegion(t *testing.T) {
	s := sharedSuite(t)

	err := s.db.Exec(`INSERT INTO siklus_ekonomi (id, wilayah_kode, siklus, mulai_pada) VALUES ('SE-GANDA', '11', 'NORMAL', now())`).Error

	assert.Error(t, err, "indeks unik parsial menolak siklus aktif kedua")
}

// Memublikasikan harga menyegarkan agregat di latar belakang, sehingga heatmap ikut berubah.
func TestPublishingPricesRefreshesDashboardAggregates(t *testing.T) {
	s := newSuite(t, "dashboard_refresh", true)
	// Penyegaran latar belakang harus selesai sebelum basis data uji dibuang.
	t.Cleanup(func() {
		assert.Eventually(t, func() bool {
			var aktif int64
			_ = s.db.Raw("SELECT COUNT(*) FROM pg_stat_activity WHERE query ILIKE 'REFRESH MATERIALIZED VIEW%' AND state = 'active'").Scan(&aktif).Error
			return aktif == 0
		}, 20*time.Second, 100*time.Millisecond)
	})

	var sebelum int64
	require.NoError(t, s.db.Raw("SELECT COUNT(*) FROM agregat_harga_harian").Scan(&sebelum).Error)

	// Entri DIAJUKAN terbaru milik kabupaten yang belum tentu terwakili: validasi lalu terbitkan semuanya.
	var ids []string
	require.NoError(t, s.db.Raw("SELECT id FROM entri_harga WHERE status = 'DIAJUKAN' AND petugas_id <> 'koordinator_lapangan' LIMIT 400").Scan(&ids).Error)
	require.NotEmpty(t, ids)

	valid := s.call(t, http.MethodPost, "/v1/harga/bulk", emailKoordinator, map[string]any{"action": "validasi", "ids": ids})
	require.Equal(t, http.StatusOK, valid.Code, valid.Body.String())
	pub := s.call(t, http.MethodPost, "/v1/harga/bulk", emailManajerData, map[string]any{"action": "publikasi", "ids": ids})
	require.Equal(t, http.StatusOK, pub.Code, pub.Body.String())

	var sesudah int64
	require.Eventually(t, func() bool {
		require.NoError(t, s.db.Raw("SELECT COUNT(*) FROM agregat_harga_harian").Scan(&sesudah).Error)
		return sesudah > sebelum
	}, 20*time.Second, 200*time.Millisecond, "agregat harus bertambah setelah entri divalidasi dan diterbitkan")
}

// Tanpa data harga, dashboard tidak error: kartu bernilai nol, heatmap kosong, siklus belum ada.
func TestEmptyDatabaseDegradesGracefully(t *testing.T) {
	s := newSuite(t, "dashboard_kosong", false)

	kpis := data[[]kpi](t, s.call(t, http.MethodGet, "/v1/dashboard/operasional/kpi", emailBerhak, nil))
	require.Len(t, kpis, 4)
	for _, k := range kpis {
		assert.Len(t, k.Tren, 14, k.ID)
		assert.Zero(t, k.Nilai, k.ID)
	}

	w := s.call(t, http.MethodGet, "/v1/dashboard/operasional/heatmap", emailBerhak, nil)
	require.Equal(t, http.StatusOK, w.Code, w.Body.String())
	assert.JSONEq(t, `{"kabupaten":[],"komoditas":[],"sel":[]}`, string(data[json.RawMessage](t, w)))

	assert.Equal(t, http.StatusNotFound, s.call(t, http.MethodGet, "/v1/siklus/aktif", emailBerhak, nil).Code)
}
