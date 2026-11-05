// Package integration menguji API EWS terhadap PostgreSQL sungguhan dengan seed
// master.reference, auth.demo, ews.reference, dan ews.demo.
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
	ewsRoutes "github.com/garda-inflasi/api/internal/ews/routes"
	ewsseeder "github.com/garda-inflasi/api/internal/ews/seeder"
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
	jwtSecret = "ews-integration-secret-key-32chars!!"

	// MANAJER_RISIKO: ews.lihat + ews.akui + ews.eskalasi.
	emailPenuh = "manajer.risiko@gardainflasi.id"
	// OPS_KOORDINATOR: ews.lihat + ews.akui, tanpa ews.eskalasi.
	emailAkui = "koordinator.lapangan@gardainflasi.id"
	// STRATEGIS_BI: hanya ews.lihat.
	emailLihat = "strategis.bi@gardainflasi.id"
	// OPS_MAHASISWA: tanpa izin EWS.
	emailTanpaIzin = "mahasiswa.1@gardainflasi.id"
)

var (
	pg      *pgtest.Server
	seedNow = time.Now().UTC()
)

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

// newSuite membuat basis data baru lengkap dengan seed dan router.
func newSuite(t *testing.T, name string) *suite {
	t.Helper()
	db := pg.CreateDatabase(t, name)
	pgtest.MigrateUp(t, db)
	_, err := masterseeder.SeedReference(t.Context(), db)
	require.NoError(t, err)
	_, err = authseeder.SeedDemo(t.Context(), db)
	require.NoError(t, err)
	_, err = ewsseeder.SeedReference(t.Context(), db)
	require.NoError(t, err)
	_, err = ewsseeder.SeedDemo(t.Context(), db, seedNow)
	require.NoError(t, err)

	tokens, log := token.NewManager(jwtSecret), logger.NewDefault()
	router := gin.New()
	v1 := router.Group("/v1")
	authRoutes.Register(v1, &authRoutes.Dependencies{DB: db, Token: tokens, Logger: log})
	ewsRoutes.Register(v1, &ewsRoutes.Dependencies{DB: db, Token: tokens, Logger: log})
	return &suite{db: db, router: router, tokens: map[string]string{}}
}

// sharedSuite dipakai tes yang hanya membaca.
func sharedSuite(t *testing.T) *suite {
	t.Helper()
	once.Do(func() { shared = newSuite(t, "ews_shared") })
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

type alertRow struct {
	ID            string  `json:"id"`
	Severity      string  `json:"severity"`
	Jenis         string  `json:"jenis"`
	KomoditasKode *string `json:"komoditas_kode"`
	KomoditasNama *string `json:"komoditas_nama"`
	KabupatenKode string  `json:"kabupaten_kode"`
	Wilayah       string  `json:"wilayah"`
	SatuanNilai   string  `json:"satuan_nilai"`
	Status        string  `json:"status"`
	AturanID      *string `json:"aturan_id"`
	DiakuiOlehID  *string `json:"diakui_oleh_id"`
}

func TestEWS_Authorization(t *testing.T) {
	s := sharedSuite(t)

	for _, path := range []string{"/v1/ews", "/v1/ews/aturan", "/v1/ews/EWS-1000"} {
		t.Run(path, func(t *testing.T) {
			assert.Equal(t, http.StatusUnauthorized, s.call(t, http.MethodGet, path, "", nil).Code)
			assert.Equal(t, http.StatusForbidden, s.call(t, http.MethodGet, path, emailTanpaIzin, nil).Code)
			assert.Equal(t, http.StatusOK, s.call(t, http.MethodGet, path, emailLihat, nil).Code)
		})
	}

	t.Run("bulk menuntut ews.akui atau ews.eskalasi", func(t *testing.T) {
		body := map[string]any{"action": "akui", "ids": []string{"EWS-1000"}}
		assert.Equal(t, http.StatusUnauthorized, s.call(t, http.MethodPost, "/v1/ews/bulk", "", body).Code)
		assert.Equal(t, http.StatusForbidden, s.call(t, http.MethodPost, "/v1/ews/bulk", emailLihat, body).Code)
	})
}

func TestEWS_ListReturnsTheSeedWithNamesAndRules(t *testing.T) {
	s := sharedSuite(t)

	w := s.call(t, http.MethodGet, "/v1/ews?limit=200", emailLihat, nil)
	require.Equal(t, http.StatusOK, w.Code, w.Body.String())
	assert.Equal(t, "17", w.Header().Get("X-Total-Count"))

	got := data[[]alertRow](t, w)
	require.Len(t, got, 17)
	byID := map[string]alertRow{}
	for _, a := range got {
		byID[a.ID] = a
	}

	first := byID["EWS-1000"]
	assert.Equal(t, "KRITIS", first.Severity)
	assert.Equal(t, "KOTA BANDA ACEH", first.Wilayah)
	require.NotNil(t, first.KomoditasNama)
	assert.Equal(t, "Cabai merah keriting", *first.KomoditasNama)
	require.NotNil(t, first.AturanID)
	assert.Equal(t, "ATR-LH-CBM-KRT", *first.AturanID)

	assert.Nil(t, byID["EWS-0903"].KomoditasKode, "alert tanpa komoditas dikembalikan null")
	assert.Equal(t, "mm", byID["EWS-0901"].SatuanNilai)
	assert.Equal(t, got[0].ID, "EWS-1000", "terbaru dahulu")
}

func TestEWS_ListFiltersAndPaginates(t *testing.T) {
	s := sharedSuite(t)

	t.Run("filter status dan severity", func(t *testing.T) {
		w := s.call(t, http.MethodGet, "/v1/ews?status=DIESKALASI", emailLihat, nil)
		require.Equal(t, http.StatusOK, w.Code)
		got := data[[]alertRow](t, w)
		assert.Len(t, got, 3)
		for _, a := range got {
			assert.Equal(t, "DIESKALASI", a.Status)
		}

		kritis := data[[]alertRow](t, s.call(t, http.MethodGet, "/v1/ews?severity=KRITIS&jenis=LONJAKAN_HARGA", emailLihat, nil))
		require.NotEmpty(t, kritis)
		for _, a := range kritis {
			assert.Equal(t, "KRITIS", a.Severity)
		}
	})

	t.Run("filter kabupaten", func(t *testing.T) {
		for _, a := range data[[]alertRow](t, s.call(t, http.MethodGet, "/v1/ews?kabupaten_kode=1171", emailLihat, nil)) {
			assert.Equal(t, "1171", a.KabupatenKode)
		}
	})

	t.Run("paginasi tidak menumpuk dan total tetap", func(t *testing.T) {
		p1 := s.call(t, http.MethodGet, "/v1/ews?limit=10&page=1", emailLihat, nil)
		p2 := s.call(t, http.MethodGet, "/v1/ews?limit=10&page=2", emailLihat, nil)
		a, b := data[[]alertRow](t, p1), data[[]alertRow](t, p2)
		assert.Len(t, a, 10)
		assert.Len(t, b, 7)
		assert.Equal(t, "17", p2.Header().Get("X-Total-Count"))
		for _, x := range b {
			for _, y := range a {
				assert.NotEqual(t, x.ID, y.ID)
			}
		}
	})

	t.Run("nilai filter tidak valid ditolak", func(t *testing.T) {
		assert.Equal(t, http.StatusBadRequest, s.call(t, http.MethodGet, "/v1/ews?status=ACAK", emailLihat, nil).Code)
		assert.Equal(t, http.StatusBadRequest, s.call(t, http.MethodGet, "/v1/ews?limit=5000", emailLihat, nil).Code)
	})
}

func TestEWS_GetByID(t *testing.T) {
	s := sharedSuite(t)

	got := data[alertRow](t, s.call(t, http.MethodGet, "/v1/ews/EWS-0901", emailLihat, nil))
	assert.Equal(t, "ANOMALI_CUACA", got.Jenis)
	assert.Equal(t, "SIAGA", got.Severity)

	assert.Equal(t, http.StatusNotFound, s.call(t, http.MethodGet, "/v1/ews/TIDAK-ADA", emailLihat, nil).Code)
}

func TestEWS_AturanListsSeededThresholds(t *testing.T) {
	s := sharedSuite(t)

	got := data[[]struct {
		ID            string   `json:"id"`
		KomoditasKode *string  `json:"komoditas_kode"`
		KomoditasNama *string  `json:"komoditas_nama"`
		Jenis         string   `json:"jenis"`
		AmbangWaspada float64  `json:"ambang_waspada"`
		AmbangSiaga   float64  `json:"ambang_siaga"`
		AmbangKritis  float64  `json:"ambang_kritis"`
		Kanal         []string `json:"kanal"`
		Aktif         bool     `json:"aktif"`
		Keterangan    string   `json:"keterangan"`
	}](t, s.call(t, http.MethodGet, "/v1/ews/aturan", emailLihat, nil))

	require.Len(t, got, 9)
	for _, a := range got {
		assert.Less(t, a.AmbangWaspada, a.AmbangSiaga, a.ID)
		assert.Less(t, a.AmbangSiaga, a.AmbangKritis, a.ID)
		assert.Equal(t, []string{"DASHBOARD", "PUSH"}, a.Kanal, a.ID)
		assert.True(t, a.Aktif)
		assert.NotEmpty(t, a.Keterangan)
		require.NotNil(t, a.KomoditasNama)
	}
}

func TestEWS_BulkTransitionsAndAudit(t *testing.T) {
	s := newSuite(t, "ews_bulk")

	type result struct {
		Berhasil int `json:"berhasil"`
		Gagal    int `json:"gagal"`
	}
	bulk := func(email, action string, ids ...string) (*httptest.ResponseRecorder, result) {
		w := s.call(t, http.MethodPost, "/v1/ews/bulk", email, map[string]any{"action": action, "ids": ids})
		if w.Code != http.StatusOK {
			return w, result{}
		}
		return w, data[result](t, w)
	}
	status := func(id string) alertRow {
		return data[alertRow](t, s.call(t, http.MethodGet, "/v1/ews/"+id, emailLihat, nil))
	}

	t.Run("akui hanya alert BARU; yang lain dihitung gagal", func(t *testing.T) {
		// EWS-1000 BARU, EWS-1004 sudah DIAKUI, TIDAK-ADA tidak ada.
		w, res := bulk(emailAkui, "akui", "EWS-1000", "EWS-1004", "TIDAK-ADA", "EWS-1000")
		require.Equal(t, http.StatusOK, w.Code, w.Body.String())
		assert.Equal(t, result{Berhasil: 1, Gagal: 2}, res)

		a := status("EWS-1000")
		assert.Equal(t, "DIAKUI", a.Status)
		require.NotNil(t, a.DiakuiOlehID)
		assert.Equal(t, "koordinator_lapangan", *a.DiakuiOlehID)
	})

	t.Run("eskalasi menuntut izin eskalasi", func(t *testing.T) {
		w, _ := bulk(emailAkui, "eskalasi", "EWS-1001")
		assert.Equal(t, http.StatusForbidden, w.Code)
		assert.Equal(t, "BARU", status("EWS-1001").Status, "tidak berubah bila ditolak")

		w, res := bulk(emailPenuh, "eskalasi", "EWS-1001")
		require.Equal(t, http.StatusOK, w.Code)
		assert.Equal(t, 1, res.Berhasil)
		assert.Equal(t, "DIESKALASI", status("EWS-1001").Status)
	})

	t.Run("selesai menutup alert dan tidak bisa diulang", func(t *testing.T) {
		_, res := bulk(emailPenuh, "selesai", "EWS-1001", "EWS-1002")
		assert.Equal(t, 2, res.Berhasil)
		assert.Equal(t, "SELESAI", status("EWS-1002").Status)

		_, res = bulk(emailPenuh, "selesai", "EWS-1001")
		assert.Equal(t, result{Gagal: 1}, res, "alert SELESAI tidak berubah lagi")
	})

	t.Run("masukan tidak valid ditolak", func(t *testing.T) {
		w, _ := bulk(emailPenuh, "hapus", "EWS-1003")
		assert.Equal(t, http.StatusBadRequest, w.Code)
		w, _ = bulk(emailPenuh, "akui")
		assert.Equal(t, http.StatusBadRequest, w.Code)
	})

	t.Run("setiap aksi yang mengubah data tercatat di audit", func(t *testing.T) {
		var rows []struct {
			Aksi      string
			EntitasID string
			AktorID   string
		}
		require.NoError(t, s.db.Raw(`SELECT aksi, entitas_id, aktor_id FROM audit_log WHERE entitas = 'ews_alert' ORDER BY waktu`).Scan(&rows).Error)
		require.Len(t, rows, 3, "akui, eskalasi, dan dua selesai; aksi tanpa perubahan tidak dicatat")
		assert.Equal(t, "BULK_AKUI", rows[0].Aksi)
		assert.Equal(t, "1 alert", rows[0].EntitasID)
		assert.Equal(t, "BULK_ESKALASI", rows[1].Aksi)
		assert.Equal(t, "BULK_SELESAI", rows[2].Aksi)
		assert.Equal(t, "2 alert", rows[2].EntitasID)
	})
}
