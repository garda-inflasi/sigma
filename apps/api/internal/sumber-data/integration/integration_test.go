// Package integration menguji API sumber-data terhadap PostgreSQL sungguhan
// dengan katalog hasil seeder dan akun demo.
package integration

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"strings"
	"sync"
	"testing"

	authRoutes "github.com/garda-inflasi/api/internal/auth/routes"
	"github.com/garda-inflasi/api/internal/ingestion/connector/pihps"
	"github.com/garda-inflasi/api/internal/ingestion/engine"
	"github.com/garda-inflasi/api/internal/ingestion/ingestiontest"
	ingestrepo "github.com/garda-inflasi/api/internal/ingestion/repository/postgres"
	"github.com/garda-inflasi/api/internal/platform/pgtest"
	"github.com/garda-inflasi/api/internal/sumber-data/domain"
	sumberRoutes "github.com/garda-inflasi/api/internal/sumber-data/routes"
	"github.com/garda-inflasi/api/pkg/idx"
	"github.com/garda-inflasi/api/pkg/logger"
	"github.com/garda-inflasi/api/pkg/seed"
	"github.com/garda-inflasi/api/pkg/token"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

const (
	jwtSecret = "sumber-integration-secret-key-32chars!"

	// MANAJER_DATA: sumber.lihat + sumber.kelola; STRATEGIS_BI: hanya sumber.lihat; OPS_MAHASISWA: tidak ada.
	emailKelola   = "manajer.data@gardainflasi.id"
	emailLihat    = "strategis.bi@gardainflasi.id"
	emailTanpaIzn = "mahasiswa.1@gardainflasi.id"
)

var pg *pgtest.Server

// sid memberi UUID sumber seed dari kodenya (sama dengan hasil migrasi data lama).
func sid(kode string) string { return domain.IDSumber(kode) }

func TestMain(m *testing.M) {
	gin.SetMode(gin.TestMode)
	os.Exit(pgtest.Main(m, &pg))
}

type suite struct {
	db      *gorm.DB
	router  *gin.Engine
	tokens  map[string]string
	fetcher *ingestiontest.Fetcher
}

var (
	once   sync.Once
	shared *suite
)

func sharedSuite(t *testing.T) *suite {
	t.Helper()
	once.Do(func() {
		db := pg.CreateDatabase(t, "sumber_shared")
		pgtest.MigrateUp(t, db)
		require.NoError(t, seed.SeedReference(t.Context(), db))
		require.NoError(t, seed.SeedDemo(t.Context(), db))
		require.NoError(t, db.Exec(`UPDATE sumber_data SET konfigurasi = '{"province_id":"7"}' WHERE kode = 'SRC-BI'`).Error)
		require.NoError(t, db.Exec(`INSERT INTO pemetaan_sumber (sumber_id, jenis, alias, kode_master) VALUES (?, 'WILAYAH', '101', '1171')`, sid("SRC-BI")).Error)
		grid, err := os.ReadFile("../../ingestion/connector/pihps/testdata/grid_kota.json")
		require.NoError(t, err)
		fetcher := &ingestiontest.Fetcher{Respons: map[string][]byte{"wilayah=101": grid}}

		tokens, log := token.NewManager(jwtSecret), logger.NewDefault()
		runner := engine.New(ingestrepo.New(db), fetcher, log, pihps.New())
		router := gin.New()
		v1 := router.Group("/v1")
		authRoutes.Register(v1, &authRoutes.Dependencies{DB: db, Token: tokens, Logger: log})
		sumberRoutes.Register(v1, &sumberRoutes.Dependencies{DB: db, Token: tokens, Logger: log, Runner: runner})
		shared = &suite{db: db, router: router, tokens: map[string]string{}, fetcher: fetcher}
	})
	require.NotNil(t, shared, "suite bersama gagal dibuat")
	return shared
}

// reset mengembalikan katalog ke keadaan demo agar tiap tes mulai dari data yang sama.
func (s *suite) reset(t *testing.T) {
	t.Helper()
	require.NoError(t, seed.SeedReference(t.Context(), s.db))
	require.NoError(t, seed.SeedDemo(t.Context(), s.db))
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
	body, _ := json.Marshal(map[string]string{"email": email, "password": seed.DemoPassword})
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

type sumberRow struct {
	ID             string   `json:"id"`
	Kode           string   `json:"kode"`
	Jenis          string   `json:"jenis"`
	JadwalCron     string   `json:"jadwal_cron"`
	Status         string   `json:"status"`
	TerakhirJalan  *string  `json:"terakhir_jalan"`
	Keberhasilan7h *float64 `json:"keberhasilan_7h"`
}

func decode[T any](t *testing.T, w *httptest.ResponseRecorder) T {
	t.Helper()
	var res struct {
		Data T `json:"data"`
	}
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &res), w.Body.String())
	return res.Data
}

// statusOf membaca status sumber ber-kode itu.
func (s *suite) statusOf(t *testing.T, kode string) string {
	t.Helper()
	var status string
	require.NoError(t, s.db.Raw(`SELECT status::text FROM sumber_data WHERE kode = ?`, kode).Scan(&status).Error)
	return status
}

func TestSumberData_Authorization(t *testing.T) {
	s := sharedSuite(t)
	s.reset(t)

	for _, path := range []string{"/v1/sumber-data", "/v1/sumber-data/SRC-BPS"} {
		assert.Equal(t, http.StatusUnauthorized, s.call(t, http.MethodGet, path, "", nil).Code, path)
		assert.Equal(t, http.StatusForbidden, s.call(t, http.MethodGet, path, emailTanpaIzn, nil).Code, path)
		assert.Equal(t, http.StatusOK, s.call(t, http.MethodGet, path, emailLihat, nil).Code, path)
	}

	body := map[string]any{"action": "jeda", "ids": []string{"SRC-BPS"}}
	assert.Equal(t, http.StatusUnauthorized, s.call(t, http.MethodPost, "/v1/sumber-data/bulk", "", body).Code)
	assert.Equal(t, http.StatusForbidden, s.call(t, http.MethodPost, "/v1/sumber-data/bulk", emailLihat, body).Code, "sumber.lihat saja tidak boleh mengubah")
	assert.Equal(t, "SEHAT", s.statusOf(t, "SRC-BPS"), "penolakan tidak boleh mengubah data")
}

func TestSumberData_ListMatchesSeededCatalog(t *testing.T) {
	s := sharedSuite(t)
	s.reset(t)

	rows := decode[[]sumberRow](t, s.call(t, http.MethodGet, "/v1/sumber-data", emailLihat, nil))

	require.Len(t, rows, 8)
	by := map[string]sumberRow{}
	for _, r := range rows {
		by[r.Kode] = r
		assert.Equal(t, sid(r.Kode), r.ID, "id = UUID stabil dari kode, sama dengan hasil migrasi data lama")
	}
	assert.Equal(t, "SCRAPER", by["SRC-BAPANAS"].Jenis)
	assert.Equal(t, "TERTUNDA", by["SRC-BAPANAS"].Status)
	assert.Equal(t, "GAGAL", by["SRC-PORTAL"].Status)
	assert.Equal(t, "0 6 * * *", by["SRC-BPS"].JadwalCron)
	assert.NotNil(t, by["SRC-BPS"].TerakhirJalan)
	require.NotNil(t, by["SRC-BPS"].Keberhasilan7h)
	assert.InDelta(t, 100, *by["SRC-BPS"].Keberhasilan7h, 0.001)
}

func TestSumberData_NeverRunSourceSerializesNulls(t *testing.T) {
	s := sharedSuite(t)
	require.NoError(t, s.db.Exec(`UPDATE sumber_data SET status = 'BELUM_AKTIF', terakhir_jalan = NULL, keberhasilan_7h = NULL, latensi_ms = 0 WHERE kode = 'SRC-OWM'`).Error)

	row := decode[sumberRow](t, s.call(t, http.MethodGet, "/v1/sumber-data/SRC-OWM", emailLihat, nil))

	assert.Equal(t, "BELUM_AKTIF", row.Status)
	assert.Nil(t, row.TerakhirJalan)
	assert.Nil(t, row.Keberhasilan7h, "belum ada eksekusi: tidak ada angka keberhasilan")
}

func TestSumberData_Get(t *testing.T) {
	s := sharedSuite(t)
	s.reset(t)

	t.Run("lewat UUID", func(t *testing.T) {
		row := decode[sumberRow](t, s.call(t, http.MethodGet, "/v1/sumber-data/"+sid("SRC-BI"), emailLihat, nil))
		assert.Equal(t, sid("SRC-BI"), row.ID)
		assert.Equal(t, "SRC-BI", row.Kode)
	})

	t.Run("lewat kode sebagai alias (tautan lama tetap berlaku)", func(t *testing.T) {
		row := decode[sumberRow](t, s.call(t, http.MethodGet, "/v1/sumber-data/SRC-BI", emailLihat, nil))
		assert.Equal(t, sid("SRC-BI"), row.ID)
		assert.Equal(t, "SRC-BI", row.Kode)
	})

	t.Run("UUID huruf besar dikanonkan", func(t *testing.T) {
		row := decode[sumberRow](t, s.call(t, http.MethodGet, "/v1/sumber-data/"+strings.ToUpper(sid("SRC-BI")), emailLihat, nil))
		assert.Equal(t, sid("SRC-BI"), row.ID)
	})

	t.Run("tidak ada atau salah bentuk dijawab 404", func(t *testing.T) {
		for _, ref := range []string{
			"TIDAK-ADA",                  // kode yang tidak ada
			idx.UUIDStabil("x", "y"),     // UUID yang sah tetapi tidak ada
			strings.Repeat("a", 65),      // lebih panjang dari kolom kode
			"12345",                      // angka murni bukan UUID dan tidak ada sebagai kode
			"1;DROP TABLE sumber_data--", // teks bebas tidak pernah sampai ke kolom uuid
		} {
			assert.Equal(t, http.StatusNotFound, s.call(t, http.MethodGet, "/v1/sumber-data/"+url.PathEscape(ref), emailLihat, nil).Code, ref)
		}
	})
}

func TestSumberData_BulkJedaAndLanjutkan(t *testing.T) {
	s := sharedSuite(t)
	s.reset(t)
	type hasil struct{ Berhasil, Gagal int }

	t.Run("jeda mengubah status dan tercatat di audit", func(t *testing.T) {
		w := s.call(t, http.MethodPost, "/v1/sumber-data/bulk", emailKelola, map[string]any{"action": "jeda", "ids": []string{"SRC-BPS", "SRC-OWM", "TIDAK-ADA"}})
		require.Equal(t, http.StatusOK, w.Code, w.Body.String())

		assert.Equal(t, hasil{Berhasil: 2, Gagal: 1}, decode[hasil](t, w))
		assert.Equal(t, "DIJEDA", s.statusOf(t, "SRC-BPS"))
		assert.Equal(t, "DIJEDA", s.statusOf(t, "SRC-OWM"))
		assert.Equal(t, "SEHAT", s.statusOf(t, "SRC-BI"), "sumber lain tidak tersentuh")

		var n int64
		require.NoError(t, s.db.Raw(`SELECT COUNT(*) FROM audit_log WHERE aksi = 'BULK_jeda' AND entitas = 'sumber_data' AND aktor_email = ?`, emailKelola).Scan(&n).Error)
		assert.EqualValues(t, 1, n)
	})

	t.Run("jeda ulang pada yang sudah dijeda dihitung gagal", func(t *testing.T) {
		w := s.call(t, http.MethodPost, "/v1/sumber-data/bulk", emailKelola, map[string]any{"action": "jeda", "ids": []string{"SRC-BPS"}})
		assert.Equal(t, hasil{Berhasil: 0, Gagal: 1}, decode[hasil](t, w))
	})

	t.Run("lanjutkan memulihkan ke TERTUNDA, bukan SEHAT", func(t *testing.T) {
		w := s.call(t, http.MethodPost, "/v1/sumber-data/bulk", emailKelola, map[string]any{"action": "lanjutkan", "ids": []string{"SRC-BPS", "SRC-BI"}})

		assert.Equal(t, hasil{Berhasil: 1, Gagal: 1}, decode[hasil](t, w), "SRC-BI tidak sedang dijeda")
		assert.Equal(t, "TERTUNDA", s.statusOf(t, "SRC-BPS"))
	})

	t.Run("jeda dan lanjutkan menerima UUID maupun kode, ejaan ganda dihitung satu", func(t *testing.T) {
		s.reset(t)
		w := s.call(t, http.MethodPost, "/v1/sumber-data/bulk", emailKelola, map[string]any{
			"action": "jeda", "ids": []string{sid("SRC-DJPB"), "SRC-DJPB", strings.ToUpper(sid("SRC-OWM"))},
		})
		require.Equal(t, http.StatusOK, w.Code, w.Body.String())
		assert.Equal(t, hasil{Berhasil: 2, Gagal: 0}, decode[hasil](t, w))
		assert.Equal(t, "DIJEDA", s.statusOf(t, "SRC-DJPB"))
		assert.Equal(t, "DIJEDA", s.statusOf(t, "SRC-OWM"))
		s.reset(t)
	})

	t.Run("lanjutkan sumber yang belum pernah berjalan kembali BELUM_AKTIF", func(t *testing.T) {
		require.NoError(t, s.db.Exec(`UPDATE sumber_data SET status = 'DIJEDA', terakhir_jalan = NULL WHERE kode = 'SRC-CS'`).Error)

		s.call(t, http.MethodPost, "/v1/sumber-data/bulk", emailKelola, map[string]any{"action": "lanjutkan", "ids": []string{"SRC-CS"}})

		assert.Equal(t, "BELUM_AKTIF", s.statusOf(t, "SRC-CS"))
	})
}

func TestSumberData_BulkValidation(t *testing.T) {
	s := sharedSuite(t)
	s.reset(t)

	for name, body := range map[string]map[string]any{
		"aksi tidak dikenal":       {"action": "hapus", "ids": []string{"SRC-BPS"}},
		"jalankan belum didukung":  {"action": "jalankan", "ids": []string{"SRC-BPS"}},
		"tanpa id":                 {"action": "jeda", "ids": []string{}},
		"id kosong":                {"action": "jeda", "ids": []string{""}},
		"id lebih panjang dari 64": {"action": "jeda", "ids": []string{strings.Repeat("a", 65)}},
		"tanpa aksi":               {"ids": []string{"SRC-BPS"}},
	} {
		assert.Equal(t, http.StatusBadRequest, s.call(t, http.MethodPost, "/v1/sumber-data/bulk", emailKelola, body).Code, name)
	}
	assert.Equal(t, "SEHAT", s.statusOf(t, "SRC-BPS"))
}
