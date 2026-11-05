// Package integration menguji API policy brief terhadap PostgreSQL sungguhan
// hasil migrasi, dengan seed master.reference, akun demo, dan brief demo.
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

	auditdomain "github.com/garda-inflasi/api/internal/audit/domain"
	authRoutes "github.com/garda-inflasi/api/internal/auth/routes"
	authseeder "github.com/garda-inflasi/api/internal/auth/seeder"
	masterseeder "github.com/garda-inflasi/api/internal/master/seeder"
	"github.com/garda-inflasi/api/internal/platform/pgtest"
	briefRoutes "github.com/garda-inflasi/api/internal/policy-brief/routes"
	briefseeder "github.com/garda-inflasi/api/internal/policy-brief/seeder"
	"github.com/garda-inflasi/api/pkg/logger"
	"github.com/garda-inflasi/api/pkg/token"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

const (
	jwtSecret = "brief-integration-secret-key-32chars!"

	// MANAJER_KEBIJAKAN: brief.lihat + brief.susun; STRATEGIS_BI: brief.lihat saja; OPS_MAHASISWA: tanpa izin brief.
	emailPenyusun  = "manajer.kebijakan@gardainflasi.id"
	emailPembaca   = "strategis.bi@gardainflasi.id"
	emailTanpaIzin = "mahasiswa.1@gardainflasi.id"
)

var (
	pg       *pgtest.Server
	fixedNow = time.Date(2026, 11, 5, 9, 0, 0, 0, time.UTC)
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

func sharedSuite(t *testing.T) *suite {
	t.Helper()
	once.Do(func() {
		db := pg.CreateDatabase(t, "brief_shared")
		pgtest.MigrateUp(t, db)
		_, err := masterseeder.SeedReference(t.Context(), db)
		require.NoError(t, err)
		_, err = authseeder.SeedDemo(t.Context(), db)
		require.NoError(t, err)
		_, err = briefseeder.SeedDemo(t.Context(), db, fixedNow)
		require.NoError(t, err)

		tokens, log := token.NewManager(jwtSecret), logger.NewDefault()
		router := gin.New()
		v1 := router.Group("/v1")
		authRoutes.Register(v1, &authRoutes.Dependencies{DB: db, Token: tokens, Logger: log})
		briefRoutes.Register(v1, &briefRoutes.Dependencies{DB: db, Token: tokens, Logger: log})
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

type briefRow struct {
	ID                string `json:"id"`
	Periode           string `json:"periode"`
	Judul             string `json:"judul"`
	Status            string `json:"status"`
	Ringkasan         string `json:"ringkasan"`
	DisusunOleh       string `json:"disusun_oleh"`
	JumlahRekomendasi int    `json:"jumlah_rekomendasi"`
	Halaman           int    `json:"halaman"`
}

func decode[T any](t *testing.T, w *httptest.ResponseRecorder) T {
	t.Helper()
	var res struct {
		Data T `json:"data"`
	}
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &res), w.Body.String())
	return res.Data
}

func TestPolicyBrief_Authorization(t *testing.T) {
	s := sharedSuite(t)
	generate := map[string]string{"periode": "2027-01"}

	assert.Equal(t, http.StatusUnauthorized, s.call(t, http.MethodGet, "/v1/policy-brief", "", nil).Code)
	assert.Equal(t, http.StatusForbidden, s.call(t, http.MethodGet, "/v1/policy-brief", emailTanpaIzin, nil).Code)
	assert.Equal(t, http.StatusOK, s.call(t, http.MethodGet, "/v1/policy-brief", emailPembaca, nil).Code)

	assert.Equal(t, http.StatusUnauthorized, s.call(t, http.MethodPost, "/v1/policy-brief/generate", "", generate).Code)
	assert.Equal(t, http.StatusForbidden, s.call(t, http.MethodPost, "/v1/policy-brief/generate", emailTanpaIzin, generate).Code)
	assert.Equal(t, http.StatusForbidden, s.call(t, http.MethodPost, "/v1/policy-brief/generate", emailPembaca, generate).Code,
		"pembaca tanpa brief.susun tidak boleh menyusun")

	var n int64
	require.NoError(t, s.db.Table("policy_brief").Where("id = ?", "PB-2027-01").Count(&n).Error)
	assert.Zero(t, n, "permintaan yang ditolak tidak boleh menyimpan brief")
}

func TestPolicyBrief_ListReturnsSeededBriefsNewestFirst(t *testing.T) {
	s := sharedSuite(t)

	got := decode[[]briefRow](t, s.call(t, http.MethodGet, "/v1/policy-brief", emailPembaca, nil))

	byID := map[string]briefRow{}
	for _, b := range got {
		byID[b.ID] = b
	}
	for _, id := range []string{"PB-2026-10", "PB-2026-09", "PB-2026-08", "PB-2026-07", "PB-2026-06"} {
		require.Contains(t, byID, id)
	}
	oct := byID["PB-2026-10"]
	assert.Equal(t, "Oktober 2026", oct.Periode)
	assert.Equal(t, "DRAF", oct.Status)
	assert.Equal(t, 5, oct.JumlahRekomendasi)
	assert.Equal(t, 8, oct.Halaman)
	assert.Equal(t, "Dr. Nurul Aini, S.E., M.E.", oct.DisusunOleh)
	assert.Equal(t, "DIDISTRIBUSIKAN", byID["PB-2026-08"].Status)

	for i := 1; i < len(got); i++ {
		assert.Greater(t, got[i-1].ID, got[i].ID, "daftar harus berurut periode terbaru lebih dulu")
	}
}

func TestPolicyBrief_GenerateStoresValidDraftAndAudits(t *testing.T) {
	s := sharedSuite(t)

	w := s.call(t, http.MethodPost, "/v1/policy-brief/generate", emailPenyusun, map[string]string{"periode": "2026-11"})
	require.Equal(t, http.StatusCreated, w.Code, w.Body.String())

	got := decode[briefRow](t, w)
	assert.Equal(t, "PB-2026-11", got.ID)
	assert.Equal(t, "November 2026", got.Periode)
	assert.Equal(t, "DRAF", got.Status)
	assert.Equal(t, "Dr. Nurul Aini, S.E., M.E.", got.DisusunOleh, "penyusun diambil dari token")
	assert.Contains(t, got.Ringkasan, "non-LLM")
	assert.Zero(t, got.JumlahRekomendasi)

	var stored struct {
		Status, DisusunOlehID, Rekomendasi string
		Halaman                            int
	}
	require.NoError(t, s.db.Raw(`SELECT status::text AS status, disusun_oleh_id, rekomendasi::text AS rekomendasi, halaman
		FROM policy_brief WHERE id = ?`, "PB-2026-11").Scan(&stored).Error)
	assert.Equal(t, "DRAF", stored.Status)
	assert.Equal(t, "manajer_kebijakan", stored.DisusunOlehID)
	assert.JSONEq(t, "[]", stored.Rekomendasi)
	assert.Positive(t, stored.Halaman)

	var trail auditdomain.AuditLog
	require.NoError(t, s.db.Where("aksi = ? AND entitas_id = ?", "GENERATE_BRIEF", "PB-2026-11").First(&trail).Error)
	assert.Equal(t, "policy_brief", trail.Entitas)
	assert.Equal(t, auditdomain.HasilBerhasil, trail.Hasil)
	require.NotNil(t, trail.AktorID)
	assert.Equal(t, "manajer_kebijakan", *trail.AktorID)

	listed := decode[[]briefRow](t, s.call(t, http.MethodGet, "/v1/policy-brief", emailPembaca, nil))
	assert.Equal(t, "PB-2026-11", listed[0].ID, "brief baru tampil di daftar")
}

func TestPolicyBrief_GenerateRejectsDuplicateAndInvalidPeriod(t *testing.T) {
	s := sharedSuite(t)

	t.Run("periode yang sudah punya brief ditolak dan brief lama utuh", func(t *testing.T) {
		w := s.call(t, http.MethodPost, "/v1/policy-brief/generate", emailPenyusun, map[string]string{"periode": "2026-09"})
		assert.Equal(t, http.StatusConflict, w.Code, w.Body.String())

		var status string
		require.NoError(t, s.db.Raw("SELECT status::text FROM policy_brief WHERE id = ?", "PB-2026-09").Scan(&status).Error)
		assert.Equal(t, "DISETUJUI", status)
	})

	t.Run("periode tidak valid", func(t *testing.T) {
		for _, periode := range []string{"Oktober 2026", "2026-13", "2026-1", "26-10", "2026-10-01"} {
			w := s.call(t, http.MethodPost, "/v1/policy-brief/generate", emailPenyusun, map[string]string{"periode": periode})
			assert.Equal(t, http.StatusBadRequest, w.Code, periode)
		}
	})

	t.Run("periode kosong", func(t *testing.T) {
		w := s.call(t, http.MethodPost, "/v1/policy-brief/generate", emailPenyusun, map[string]string{})
		assert.Equal(t, http.StatusBadRequest, w.Code)
	})
}

func TestPolicyBrief_BulkMovesStatusWithPermissionsAndAudit(t *testing.T) {
	s := sharedSuite(t)
	const emailPenyetuju = "strategis.djpb@gardainflasi.id" // brief.setujui, tanpa brief.susun

	created := s.call(t, http.MethodPost, "/v1/policy-brief/generate", emailPenyusun, map[string]string{"periode": "2027-02"})
	require.Equal(t, http.StatusCreated, created.Code, created.Body.String())
	ids := []string{"PB-2027-02", "tidak-ada"}

	statusBrief := func() string {
		var status string
		require.NoError(t, s.db.Raw("SELECT status::text FROM policy_brief WHERE id = ?", "PB-2027-02").Scan(&status).Error)
		return status
	}

	t.Run("izin per aksi ditegakkan", func(t *testing.T) {
		telaah := map[string]any{"action": "telaah", "ids": ids}
		setujui := map[string]any{"action": "setujui", "ids": ids}
		assert.Equal(t, http.StatusUnauthorized, s.call(t, http.MethodPost, "/v1/policy-brief/bulk", "", telaah).Code)
		assert.Equal(t, http.StatusForbidden, s.call(t, http.MethodPost, "/v1/policy-brief/bulk", emailPembaca, telaah).Code)
		assert.Equal(t, http.StatusForbidden, s.call(t, http.MethodPost, "/v1/policy-brief/bulk", emailPenyetuju, telaah).Code)
		assert.Equal(t, http.StatusForbidden, s.call(t, http.MethodPost, "/v1/policy-brief/bulk", emailPenyusun, setujui).Code)
		assert.Equal(t, "DRAF", statusBrief())
	})

	t.Run("aksi dan id tidak valid", func(t *testing.T) {
		assert.Equal(t, http.StatusBadRequest, s.call(t, http.MethodPost, "/v1/policy-brief/bulk", emailPenyusun, map[string]any{"action": "hapus", "ids": ids}).Code)
		assert.Equal(t, http.StatusBadRequest, s.call(t, http.MethodPost, "/v1/policy-brief/bulk", emailPenyusun, map[string]any{"action": "telaah", "ids": []string{}}).Code)
	})

	t.Run("setujui dari DRAF dilewati", func(t *testing.T) {
		w := s.call(t, http.MethodPost, "/v1/policy-brief/bulk", emailPenyetuju, map[string]any{"action": "setujui", "ids": ids})
		require.Equal(t, http.StatusOK, w.Code, w.Body.String())
		assert.Equal(t, bulkRes{Gagal: 2}, decode[bulkRes](t, w))
		assert.Equal(t, "DRAF", statusBrief())
	})

	t.Run("telaah lalu setujui, sekali jalan per tahap", func(t *testing.T) {
		w := s.call(t, http.MethodPost, "/v1/policy-brief/bulk", emailPenyusun, map[string]any{"action": "telaah", "ids": ids})
		require.Equal(t, http.StatusOK, w.Code, w.Body.String())
		assert.Equal(t, bulkRes{Berhasil: 1, Gagal: 1}, decode[bulkRes](t, w))
		assert.Equal(t, "TELAAH", statusBrief())

		again := s.call(t, http.MethodPost, "/v1/policy-brief/bulk", emailPenyusun, map[string]any{"action": "telaah", "ids": ids})
		assert.Equal(t, bulkRes{Gagal: 2}, decode[bulkRes](t, again), "mengulang tidak mengubah apa pun")

		w = s.call(t, http.MethodPost, "/v1/policy-brief/bulk", emailPenyetuju, map[string]any{"action": "setujui", "ids": ids})
		require.Equal(t, http.StatusOK, w.Code, w.Body.String())
		assert.Equal(t, bulkRes{Berhasil: 1, Gagal: 1}, decode[bulkRes](t, w))
		assert.Equal(t, "DISETUJUI", statusBrief())
	})

	t.Run("setiap perubahan tercatat tepat sekali", func(t *testing.T) {
		for aksi, aktor := range map[string]string{"BRIEF_TELAAH": "manajer_kebijakan", "BRIEF_SETUJUI": "strategis_djpb"} {
			var rows []auditdomain.AuditLog
			require.NoError(t, s.db.Where("aksi = ? AND entitas_id = ?", aksi, "PB-2027-02").Find(&rows).Error)
			require.Len(t, rows, 1, aksi)
			require.NotNil(t, rows[0].AktorID)
			assert.Equal(t, aktor, *rows[0].AktorID)
		}
	})
}

type bulkRes struct {
	Berhasil int `json:"berhasil"`
	Gagal    int `json:"gagal"`
}
