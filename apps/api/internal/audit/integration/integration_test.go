// Package integration menguji jejak audit terhadap PostgreSQL sungguhan:
// penulisan lewat modul auth, trigger append-only, FK aktor, dan GET /v1/audit.
package integration

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
	"time"

	"github.com/garda-inflasi/api/internal/audit"
	auditRoutes "github.com/garda-inflasi/api/internal/audit/routes"
	authRoutes "github.com/garda-inflasi/api/internal/auth/routes"
	"github.com/garda-inflasi/api/internal/platform/pgtest"
	"github.com/garda-inflasi/api/pkg/crypto"
	"github.com/garda-inflasi/api/pkg/logger"
	"github.com/garda-inflasi/api/pkg/middleware"
	"github.com/garda-inflasi/api/pkg/token"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

const (
	jwtSecret = "audit-integration-secret-key-32chars!"
	sandi     = "Test1234!"

	adminEmail  = "admin@gardainflasi.id"
	viewerEmail = "viewer@gardainflasi.id"
	targetEmail = "target@gardainflasi.id"
)

var pg *pgtest.Server

func TestMain(m *testing.M) {
	gin.SetMode(gin.TestMode)
	os.Exit(pgtest.Main(m, &pg))
}

type suite struct {
	db     *gorm.DB
	router *gin.Engine
	writer audit.Writer
}

func newSuite(t *testing.T, name string) *suite {
	t.Helper()
	db := pg.NewDatabase(t, name)
	pgtest.MigrateUp(t, db)

	hash, err := crypto.HashPassword(sandi)
	require.NoError(t, err)
	for _, u := range []struct{ id, email, peran, lapis string }{
		{"admin-1", adminEmail, "ADMIN_SISTEM", "SISTEM"},
		{"viewer-1", viewerEmail, "OPS_MAHASISWA", "OPERASIONAL"},
		{"target-1", targetEmail, "OPS_MAHASISWA", "OPERASIONAL"},
	} {
		require.NoError(t, db.Exec(`
			INSERT INTO pengguna (id, nama, email, password_hash, peran, lapis, lembaga)
			VALUES (?, ?, ?, ?, ?, ?, 'Uji')`, u.id, "Nama "+u.id, u.email, hash, u.peran, u.lapis).Error)
	}

	tokens := token.NewManager(jwtSecret)
	log := logger.NewDefault()
	router := gin.New()
	router.Use(middleware.RequestID())
	v1 := router.Group("/v1")
	authRoutes.Register(v1, &authRoutes.Dependencies{DB: db, Token: tokens, Logger: log})
	auditRoutes.Register(v1, &auditRoutes.Dependencies{DB: db, Token: tokens, Logger: log})

	return &suite{db: db, router: router, writer: audit.NewWriter(db)}
}

func (s *suite) call(t *testing.T, method, path, bearer string, body any) *httptest.ResponseRecorder {
	t.Helper()
	var payload bytes.Buffer
	if body != nil {
		require.NoError(t, json.NewEncoder(&payload).Encode(body))
	}
	req := httptest.NewRequest(method, path, &payload)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("User-Agent", "uji-integrasi/1.0")
	if bearer != "" {
		req.Header.Set("Authorization", "Bearer "+bearer)
	}
	w := httptest.NewRecorder()
	s.router.ServeHTTP(w, req)
	return w
}

func (s *suite) login(t *testing.T, email string) string {
	t.Helper()
	w := s.call(t, http.MethodPost, "/v1/auth/login", "", map[string]string{"email": email, "password": sandi})
	require.Equal(t, http.StatusOK, w.Code, w.Body.String())
	return dataField(t, w, "access_token").(string)
}

func dataField(t *testing.T, w *httptest.ResponseRecorder, key string) any {
	t.Helper()
	var body struct {
		Data map[string]any `json:"data"`
	}
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &body))
	return body.Data[key]
}

type auditRow struct {
	AktorID       *string
	AktorNama     *string
	AktorEmail    *string
	AktorPeran    *string
	Aksi          string
	Entitas       string
	EntitasID     string
	Hasil         string
	IP            *string
	UserAgent     *string
	CorrelationID *string
	Sebelum       *string
	Sesudah       *string
}

func (s *suite) rows(t *testing.T, aksi string) []auditRow {
	t.Helper()
	var rows []auditRow
	require.NoError(t, s.db.Raw(`
		SELECT aktor_id, aktor_nama, aktor_email, aktor_peran, aksi, entitas, entitas_id, hasil, ip, user_agent,
		       correlation_id, sebelum::text AS sebelum, sesudah::text AS sesudah
		  FROM audit_log WHERE aksi = ? ORDER BY waktu`, aksi).Scan(&rows).Error)
	return rows
}

// ── Penulisan lewat modul auth ────────────────────────────────────────

func TestAuth_SensitiveActionsWriteExactlyOneRow(t *testing.T) {
	s := newSuite(t, "audit_auth_actions")
	adminToken := s.login(t, adminEmail)

	t.Run("login berhasil mencatat aktor, origin, dan correlation id", func(t *testing.T) {
		rows := s.rows(t, "LOGIN")
		require.Len(t, rows, 1)
		assert.Equal(t, "admin-1", *rows[0].AktorID)
		assert.Equal(t, "Nama admin-1", *rows[0].AktorNama)
		assert.Equal(t, adminEmail, *rows[0].AktorEmail)
		assert.Equal(t, "ADMIN_SISTEM", *rows[0].AktorPeran)
		assert.Equal(t, "BERHASIL", rows[0].Hasil)
		assert.Equal(t, "uji-integrasi/1.0", *rows[0].UserAgent)
		assert.NotEmpty(t, *rows[0].IP)
		assert.NotEmpty(t, *rows[0].CorrelationID)
	})

	t.Run("login gagal dicatat sebagai GAGAL", func(t *testing.T) {
		w := s.call(t, http.MethodPost, "/v1/auth/login", "", map[string]string{"email": targetEmail, "password": "Salah123!"})
		require.Equal(t, http.StatusUnauthorized, w.Code)

		var rows []auditRow
		require.NoError(t, s.db.Raw(`SELECT aktor_id, hasil, entitas_id FROM audit_log WHERE aksi = 'LOGIN' AND hasil = 'GAGAL'`).Scan(&rows).Error)
		require.Len(t, rows, 1)
		assert.Equal(t, "target-1", *rows[0].AktorID)
	})

	t.Run("login email tak dikenal dicatat tanpa aktor", func(t *testing.T) {
		s.call(t, http.MethodPost, "/v1/auth/login", "", map[string]string{"email": "hantu@gardainflasi.id", "password": sandi})

		var rows []auditRow
		require.NoError(t, s.db.Raw(`SELECT aktor_id, entitas_id FROM audit_log WHERE aksi = 'LOGIN' AND entitas_id = 'hantu@gardainflasi.id'`).Scan(&rows).Error)
		require.Len(t, rows, 1)
		assert.Nil(t, rows[0].AktorID)
	})

	t.Run("ganti peran mencatat sebelum dan sesudah", func(t *testing.T) {
		w := s.call(t, http.MethodPut, "/v1/users/target-1/roles", adminToken, map[string]string{"peran": "OPS_KOORDINATOR"})
		require.Equal(t, http.StatusOK, w.Code, w.Body.String())

		rows := s.rows(t, "ASSIGN_ROLE")
		require.Len(t, rows, 1)
		assert.Equal(t, "admin-1", *rows[0].AktorID)
		assert.Equal(t, "target-1", rows[0].EntitasID)
		assert.JSONEq(t, `"OPS_MAHASISWA"`, jsonField(t, *rows[0].Sebelum, "peran"))
		assert.JSONEq(t, `"OPS_KOORDINATOR"`, jsonField(t, *rows[0].Sesudah, "peran"))
	})

	t.Run("aksi massal pengguna mencatat pelaku", func(t *testing.T) {
		w := s.call(t, http.MethodPost, "/v1/users/bulk", adminToken, map[string]any{"action": "nonaktifkan", "ids": []string{"viewer-1"}})
		require.Equal(t, http.StatusOK, w.Code, w.Body.String())

		rows := s.rows(t, "BULK_nonaktifkan")
		require.Len(t, rows, 1)
		assert.Equal(t, "admin-1", *rows[0].AktorID)
	})

	t.Run("logout mencatat satu baris", func(t *testing.T) {
		w := s.call(t, http.MethodPost, "/v1/auth/logout", adminToken, nil)
		require.Equal(t, http.StatusOK, w.Code)

		assert.Len(t, s.rows(t, "LOGOUT"), 1)
	})
}

func jsonField(t *testing.T, raw, key string) string {
	t.Helper()
	var m map[string]json.RawMessage
	require.NoError(t, json.Unmarshal([]byte(raw), &m))
	return string(m[key])
}

// ── Append-only ───────────────────────────────────────────────────────

func TestAuditLog_IsAppendOnly(t *testing.T) {
	s := newSuite(t, "audit_append_only")
	require.NoError(t, s.writer.Write(t.Context(), audit.Entry{AktorID: "admin-1", Aksi: "UJI", Entitas: "uji", EntitasID: "1", Hasil: audit.HasilBerhasil}))

	cases := map[string]string{
		"UPDATE":   `UPDATE audit_log SET hasil = 'GAGAL'`,
		"DELETE":   `DELETE FROM audit_log`,
		"TRUNCATE": `TRUNCATE audit_log`,
	}
	for name, statement := range cases {
		t.Run(name, func(t *testing.T) {
			err := s.db.Exec(statement).Error

			require.Error(t, err)
			assert.Contains(t, err.Error(), "append-only")
		})
	}

	var n int64
	require.NoError(t, s.db.Table("audit_log").Count(&n).Error)
	assert.EqualValues(t, 1, n)
}

func TestAuditLog_DeletingActorKeepsRowWithSnapshot(t *testing.T) {
	s := newSuite(t, "audit_actor_deleted")
	require.NoError(t, s.writer.Write(t.Context(), audit.Entry{AktorID: "target-1", Aksi: "UJI", Entitas: "uji", EntitasID: "1", Hasil: audit.HasilBerhasil}))

	require.NoError(t, s.db.Exec(`DELETE FROM pengguna WHERE id = 'target-1'`).Error, "menghapus pengguna yang punya jejak harus berhasil (SET NULL)")

	rows := s.rows(t, "UJI")
	require.Len(t, rows, 1)
	assert.Nil(t, rows[0].AktorID)
	assert.Equal(t, targetEmail, *rows[0].AktorEmail, "snapshot aktor tetap menjelaskan pelaku")
	assert.Equal(t, "Nama target-1", *rows[0].AktorNama)
}

func TestAuditLog_DirectActorErasureIsRejected(t *testing.T) {
	s := newSuite(t, "audit_actor_erasure")
	require.NoError(t, s.writer.Write(t.Context(), audit.Entry{AktorID: "target-1", Aksi: "UJI", Entitas: "uji", EntitasID: "1", Hasil: audit.HasilBerhasil}))

	err := s.db.Exec(`UPDATE audit_log SET aktor_id = NULL`).Error

	require.Error(t, err, "menghapus jejak aktor dengan UPDATE langsung harus ditolak")
}

func TestWriter_UnknownActorIsRecordedWithoutActor(t *testing.T) {
	s := newSuite(t, "audit_unknown_actor")

	require.NoError(t, s.writer.Write(t.Context(), audit.Entry{AktorID: "tidak-ada", Aksi: "UJI", Entitas: "uji", EntitasID: "1", Hasil: audit.HasilBerhasil}))

	rows := s.rows(t, "UJI")
	require.Len(t, rows, 1)
	assert.Nil(t, rows[0].AktorID)
}

// ── GET /v1/audit ─────────────────────────────────────────────────────

// seedBase adalah waktu baris seed pertama; tetap agar uji rentang waktu deterministik.
var seedBase = time.Date(2026, 3, 1, 10, 0, 0, 0, time.UTC)

func (s *suite) seedLog(t *testing.T, n int) {
	t.Helper()
	base := seedBase
	for i := 0; i < n; i++ {
		aksi, hasil := "VALIDASI_HARGA", audit.HasilBerhasil
		if i%3 == 0 {
			aksi = "PUBLIKASI_HARGA"
		}
		if i%5 == 0 {
			hasil = audit.HasilDitolak
		}
		require.NoError(t, s.writer.Write(t.Context(), audit.Entry{
			AktorID: "admin-1", Aksi: aksi, Entitas: "entri_harga", EntitasID: fmt.Sprintf("ENT-%02d", i),
			Hasil: hasil, Waktu: base.Add(time.Duration(i) * time.Minute),
		}))
	}
}

type page struct {
	Items []struct {
		ID        string `json:"id"`
		Aksi      string `json:"aksi"`
		Hasil     string `json:"hasil"`
		EntitasID string `json:"entitas_id"`
	} `json:"items"`
	NextCursor string `json:"next_cursor"`
}

func (s *suite) listAudit(t *testing.T, token, query string) (int, page) {
	t.Helper()
	w := s.call(t, http.MethodGet, "/v1/audit"+query, token, nil)
	var body struct {
		Data page `json:"data"`
	}
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &body), w.Body.String())
	return w.Code, body.Data
}

func TestListAudit_RequiresAuditPermission(t *testing.T) {
	s := newSuite(t, "audit_list_rbac")

	code, _ := s.listAudit(t, s.login(t, viewerEmail), "")
	assert.Equal(t, http.StatusForbidden, code)

	code, _ = s.listAudit(t, s.login(t, adminEmail), "")
	assert.Equal(t, http.StatusOK, code)

	code, _ = s.listAudit(t, "", "")
	assert.Equal(t, http.StatusUnauthorized, code)
}

func TestListAudit_CursorPaginationWalksAllRowsOnce(t *testing.T) {
	s := newSuite(t, "audit_list_cursor")
	s.seedLog(t, 25)
	token := s.login(t, adminEmail)

	seen := map[string]bool{}
	var firstPageSize int
	cursor := ""
	for pages := 0; ; pages++ {
		require.Less(t, pages, 20, "paginasi tidak berhenti")
		query := "?limit=10"
		if cursor != "" {
			query += "&cursor=" + cursor
		}
		code, p := s.listAudit(t, token, query)
		require.Equal(t, http.StatusOK, code)
		if pages == 0 {
			firstPageSize = len(p.Items)
		}
		for _, item := range p.Items {
			assert.False(t, seen[item.ID], "baris dobel antar halaman: %s", item.ID)
			seen[item.ID] = true
		}
		if p.NextCursor == "" {
			break
		}
		cursor = p.NextCursor
	}

	assert.Equal(t, 10, firstPageSize, "default tidak lagi memuat semuanya sekaligus")
	var total int64
	require.NoError(t, s.db.Table("audit_log").Count(&total).Error)
	assert.EqualValues(t, total, len(seen), "seluruh baris (termasuk login) terbaca tepat sekali")
}

func TestListAudit_ServerSideFilters(t *testing.T) {
	s := newSuite(t, "audit_list_filters")
	s.seedLog(t, 20)
	token := s.login(t, adminEmail)

	t.Run("aksi", func(t *testing.T) {
		_, p := s.listAudit(t, token, "?aksi=PUBLIKASI_HARGA&limit=200")
		require.NotEmpty(t, p.Items)
		for _, item := range p.Items {
			assert.Equal(t, "PUBLIKASI_HARGA", item.Aksi)
		}
	})

	t.Run("hasil", func(t *testing.T) {
		_, p := s.listAudit(t, token, "?hasil=DITOLAK&limit=200")
		require.NotEmpty(t, p.Items)
		for _, item := range p.Items {
			assert.Equal(t, "DITOLAK", item.Hasil)
		}
	})

	t.Run("aktor menurut nama atau email", func(t *testing.T) {
		_, byEmail := s.listAudit(t, token, "?aktor=ADMIN@garda&limit=200")
		_, byID := s.listAudit(t, token, "?aktor=admin-1&limit=200")
		_, none := s.listAudit(t, token, "?aktor=tidak-ada&limit=200")
		assert.NotEmpty(t, byEmail.Items)
		assert.Equal(t, len(byID.Items), len(byEmail.Items))
		assert.Empty(t, none.Items)
	})

	t.Run("rentang waktu", func(t *testing.T) {
		_, p := s.listAudit(t, token, "?dari=2026-03-01T10:00:00Z&sampai=2026-03-01T10:05:00Z&limit=200")
		assert.Len(t, p.Items, 5, "menit 0 sampai 4; batas atas eksklusif")
	})

	t.Run("sampai bertanggal mencakup seluruh hari itu", func(t *testing.T) {
		_, p := s.listAudit(t, token, "?dari=2026-03-01&sampai=2026-03-01&limit=200")
		assert.Len(t, p.Items, 20)
	})

	t.Run("parameter tidak valid ditolak", func(t *testing.T) {
		for _, query := range []string{"?dari=kemarin", "?hasil=SUKSES", "?cursor=bukan-cursor"} {
			code, _ := s.listAudit(t, token, query)
			assert.Equal(t, http.StatusBadRequest, code, query)
		}
	})
}
