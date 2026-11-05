// Package integration menguji API master terhadap PostgreSQL sungguhan dengan
// seed master.reference dan akun demo.
package integration

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"strconv"
	"strings"
	"sync"
	"testing"

	authRoutes "github.com/garda-inflasi/api/internal/auth/routes"
	authseeder "github.com/garda-inflasi/api/internal/auth/seeder"
	masterRoutes "github.com/garda-inflasi/api/internal/master/routes"
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
	jwtSecret = "master-integration-secret-key-32chars!"

	// ADMIN_SISTEM: master.lihat + master.ajukan; OPS_KOORDINATOR: hanya master.lihat; OPS_MAHASISWA: tidak ada.
	emailAjukan   = "superadmin@gardainflasi.id"
	emailLihat    = "koordinator.lapangan@gardainflasi.id"
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
		db := pg.CreateDatabase(t, "master_shared")
		pgtest.MigrateUp(t, db)
		_, err := masterseeder.SeedReference(t.Context(), db)
		require.NoError(t, err)
		_, err = authseeder.SeedDemo(t.Context(), db)
		require.NoError(t, err)

		tokens, log := token.NewManager(jwtSecret), logger.NewDefault()
		router := gin.New()
		v1 := router.Group("/v1")
		authRoutes.Register(v1, &authRoutes.Dependencies{DB: db, Token: tokens, Logger: log})
		masterRoutes.Register(v1, &masterRoutes.Dependencies{DB: db, Token: tokens, Logger: log})
		shared = &suite{db: db, router: router, tokens: map[string]string{}}
	})
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

func rows[T any](t *testing.T, w *httptest.ResponseRecorder) []T {
	t.Helper()
	var res struct {
		Data []T `json:"data"`
	}
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &res), w.Body.String())
	return res.Data
}

type komoditasRow struct {
	Kode       string `json:"kode"`
	Nama       string `json:"nama"`
	HargaAcuan int    `json:"harga_acuan"`
	Pilot      bool   `json:"pilot"`
	Versi      int    `json:"versi"`
	Status     string `json:"status"`
}

func TestMaster_Authorization(t *testing.T) {
	s := sharedSuite(t)

	for _, path := range []string{"/v1/master/komoditas", "/v1/master/wilayah", "/v1/master/pasar"} {
		t.Run(path, func(t *testing.T) {
			assert.Equal(t, http.StatusUnauthorized, s.call(t, http.MethodGet, path, "", nil).Code)
			assert.Equal(t, http.StatusForbidden, s.call(t, http.MethodGet, path, emailTanpaIzn, nil).Code)
			assert.Equal(t, http.StatusOK, s.call(t, http.MethodGet, path, emailLihat, nil).Code)
		})
	}
}

func TestMaster_ListsContainTheWholeSeed(t *testing.T) {
	s := sharedSuite(t)

	t.Run("komoditas", func(t *testing.T) {
		got := rows[komoditasRow](t, s.call(t, http.MethodGet, "/v1/master/komoditas", emailLihat, nil))
		byKode := map[string]komoditasRow{}
		for _, k := range got {
			byKode[k.Kode] = k
		}
		for _, want := range masterseeder.Komoditas() {
			row, ok := byKode[want.Kode]
			require.True(t, ok, "komoditas %s tidak dikembalikan", want.Kode)
			assert.Equal(t, want.Nama, row.Nama)
			assert.Equal(t, want.HargaAcuan, row.HargaAcuan)
			assert.Equal(t, want.Pilot, row.Pilot)
			assert.Equal(t, 1, row.Versi)
			assert.Equal(t, "AKTIF", row.Status)
		}
	})

	t.Run("wilayah", func(t *testing.T) {
		w := s.call(t, http.MethodGet, "/v1/master/wilayah?provinsi_kode=11", emailLihat, nil)
		got := rows[struct {
			Kode  string `json:"kode"`
			Jenis string `json:"jenis"`
		}](t, w)
		kode := map[string]bool{}
		for _, k := range got {
			kode[k.Kode] = true
		}
		for _, want := range masterseeder.Kabupaten() {
			assert.True(t, kode[want.Kode], "wilayah %s tidak dikembalikan", want.Kode)
		}
	})

	t.Run("pasar menurut kabupaten", func(t *testing.T) {
		got := rows[struct {
			ID            string `json:"id"`
			KabupatenKode string `json:"kabupaten_kode"`
			RadiusMeter   int    `json:"radius_meter"`
		}](t, s.call(t, http.MethodGet, "/v1/master/pasar?kabupaten_kode=1171", emailLihat, nil))
		var ids []string
		for _, p := range got {
			assert.Equal(t, "1171", p.KabupatenKode)
			assert.Positive(t, p.RadiusMeter)
			ids = append(ids, p.ID)
		}
		assert.Contains(t, ids, "PSR-1171-01")
		assert.Contains(t, ids, "PSR-1171-02")
	})
}

func TestMaster_SearchAndPagination(t *testing.T) {
	s := sharedSuite(t)

	t.Run("pencarian menurut nama tanpa peka huruf besar-kecil", func(t *testing.T) {
		got := rows[komoditasRow](t, s.call(t, http.MethodGet, "/v1/master/komoditas?q=CABAI", emailLihat, nil))

		assert.Subset(t, kodes(got), []string{"CBM-KRT", "CBR-RWT"})
		for _, k := range got {
			assert.Contains(t, strings.ToLower(k.Nama), "cabai")
		}
	})

	t.Run("pencarian menurut kode memberi total di header", func(t *testing.T) {
		w := s.call(t, http.MethodGet, "/v1/master/komoditas?q=cbm-krt", emailLihat, nil)

		assert.Equal(t, []string{"CBM-KRT"}, kodes(rows[komoditasRow](t, w)))
		assert.Equal(t, "1", w.Header().Get("X-Total-Count"))
	})

	t.Run("wildcard dianggap literal", func(t *testing.T) {
		got := rows[komoditasRow](t, s.call(t, http.MethodGet, "/v1/master/komoditas?q=%25", emailLihat, nil))
		assert.Empty(t, got)
	})

	t.Run("paginasi menelusuri semua baris tanpa dobel", func(t *testing.T) {
		all := rows[komoditasRow](t, s.call(t, http.MethodGet, "/v1/master/komoditas", emailLihat, nil))
		seen := map[string]bool{}
		for page := 1; page <= len(all); page++ {
			w := s.call(t, http.MethodGet, "/v1/master/komoditas?limit=4&page="+strconv.Itoa(page), emailLihat, nil)
			require.Equal(t, strconv.Itoa(len(all)), w.Header().Get("X-Total-Count"))
			chunk := rows[komoditasRow](t, w)
			if len(chunk) == 0 {
				break
			}
			assert.LessOrEqual(t, len(chunk), 4)
			for _, k := range chunk {
				assert.False(t, seen[k.Kode], "dobel antar halaman: %s", k.Kode)
				seen[k.Kode] = true
			}
		}
		assert.Len(t, seen, len(all))
	})

	t.Run("parameter tidak valid ditolak", func(t *testing.T) {
		for _, query := range []string{"?page=-1&limit=5", "?limit=-1", "?limit=9999", "?page=abc"} {
			assert.Equal(t, http.StatusBadRequest, s.call(t, http.MethodGet, "/v1/master/komoditas"+query, emailLihat, nil).Code, query)
		}
	})
}

func kodes(rows []komoditasRow) []string {
	out := make([]string, len(rows))
	for i, r := range rows {
		out[i] = r.Kode
	}
	return out
}

func TestMaster_BulkKomoditas(t *testing.T) {
	s := sharedSuite(t)
	body := func(action string, kode ...string) map[string]any {
		return map[string]any{"action": action, "kode": kode}
	}
	statusOf := func(kode string) string {
		var status string
		require.NoError(t, s.db.Raw(`SELECT status::text FROM komoditas WHERE kode = ?`, kode).Scan(&status).Error)
		return status
	}

	t.Run("hanya pemegang master.ajukan", func(t *testing.T) {
		assert.Equal(t, http.StatusForbidden, s.call(t, http.MethodPost, "/v1/master/komoditas/bulk", emailLihat, body("arsipkan", "GLA")).Code)
		assert.Equal(t, "AKTIF", statusOf("GLA"))
	})

	t.Run("ajukan, setujui, dan arsipkan mengubah status serta tercatat di audit", func(t *testing.T) {
		for action, want := range map[string]string{
			"ajukan_perubahan": "MENUNGGU_PERSETUJUAN",
			"setujui":          "AKTIF",
			"arsipkan":         "DIARSIPKAN",
		} {
			w := s.call(t, http.MethodPost, "/v1/master/komoditas/bulk", emailAjukan, body(action, "GLA", "MYK"))
			require.Equal(t, http.StatusOK, w.Code, w.Body.String())
			assert.Equal(t, want, statusOf("GLA"), action)
			assert.Equal(t, want, statusOf("MYK"), action)

			var n int64
			require.NoError(t, s.db.Raw(`SELECT COUNT(*) FROM audit_log WHERE aksi = ? AND aktor_id = 'superadmin' AND entitas = 'komoditas'`, "BULK_"+action).Scan(&n).Error)
			assert.EqualValues(t, 1, n, "tepat satu baris audit untuk %s", action)
		}
	})

	t.Run("aksi tidak dikenal", func(t *testing.T) {
		assert.Equal(t, http.StatusBadRequest, s.call(t, http.MethodPost, "/v1/master/komoditas/bulk", emailAjukan, body("hapus", "GLA")).Code)
	})
}
