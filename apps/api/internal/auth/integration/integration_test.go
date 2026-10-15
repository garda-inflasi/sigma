package integration

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/garda-inflasi/api/internal/auth/routes"
	"github.com/garda-inflasi/api/internal/modules"
	"github.com/garda-inflasi/api/pkg/crypto"
	"github.com/garda-inflasi/api/pkg/database"
	"github.com/garda-inflasi/api/pkg/logger"
	"github.com/garda-inflasi/api/pkg/middleware"
	"github.com/garda-inflasi/api/pkg/token"
	"github.com/ory/dockertest/v3"
	"github.com/ory/dockertest/v3/docker"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

// ── Test Suite ────────────────────────────────────────────────────────

type Suite struct {
	db       *gorm.DB
	tokenMgr *token.Manager
	router   *gin.Engine
	pool     *dockertest.Pool
	resource *dockertest.Resource
}

var jwtSecret = "integration-test-secret-key-32chars!!"

func TestMain(m *testing.M) {
	gin.SetMode(gin.TestMode)
	os.Exit(m.Run())
}

func setupSuite(t *testing.T) *Suite {
	t.Helper()

	pool, err := dockertest.NewPool("")
	require.NoError(t, err, "could not connect to Docker")

	resource, err := pool.RunWithOptions(&dockertest.RunOptions{
		Repository: "postgres",
		Tag:        "16-alpine",
		Env: []string{
			"POSTGRES_USER=garda",
			"POSTGRES_PASSWORD=garda_secret",
			"POSTGRES_DB=garda_inflasi_test",
			"listen_addresses = '*'",
		},
	}, func(config *docker.HostConfig) {
		config.AutoRemove = true
		config.RestartPolicy = docker.RestartPolicy{Name: "no"}
	})
	require.NoError(t, err, "could not start PostgreSQL container")

	t.Cleanup(func() {
		if err := pool.Purge(resource); err != nil {
			t.Logf("could not purge resource: %v", err)
		}
	})

	hostPort := resource.GetHostPort("5432/tcp")
	parts := strings.Split(hostPort, ":")
	host := parts[0]
	port := parts[1]

	os.Setenv("DB_HOST", host)
	os.Setenv("DB_PORT", port)
	os.Setenv("DB_USER", "garda")
	os.Setenv("DB_PASSWORD", "garda_secret")
	os.Setenv("DB_NAME", "garda_inflasi_test")
	os.Setenv("DB_SSLMODE", "disable")

	var db *gorm.DB
	if err := pool.Retry(func() error {
		cfg := database.DefaultConfig()
		var connErr error
		db, connErr = database.New(cfg)
		return connErr
	}); err != nil {
		t.Fatalf("could not connect to database: %v", err)
	}

	runMigrations(t, db)
	seedData(t, db)

	tokenMgr := token.NewManager(jwtSecret)

	router := gin.New()
	router.Use(gin.Recovery())
	router.Use(
		middleware.RequestID(),
		middleware.SecurityHeaders(false),
		middleware.CSP(),
		middleware.CORSMiddleware(),
		middleware.AccessLog(logger.NewDefault()),
		middleware.RateLimit(1000, time.Minute),
	)

	router.GET("/healthz", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"status": "ok"})
	})

	v1 := router.Group("/v1")
	log := logger.NewDefault()

	routes.Register(v1, &routes.Dependencies{
		DB:     db,
		Token:  tokenMgr,
		Logger: log,
	})

	modules.RegisterAll(v1, db, log, tokenMgr)

	return &Suite{
		db:       db,
		tokenMgr: tokenMgr,
		router:   router,
		pool:     pool,
		resource: resource,
	}
}

func runMigrations(t *testing.T, db *gorm.DB) {
	t.Helper()

	migrationFiles := []string{
		"../../../../../db/migrations/000001_init.sql",
		"../../../../../db/migrations/000002_tables.sql",
		"../../../../../db/migrations/000003_auth_tables.sql",
		"../../../../../db/migrations/000004_mfa_tables.sql",
	}

	sqlDB, err := db.DB()
	require.NoError(t, err, "could not get underlying sql.DB")

	for _, f := range migrationFiles {
		absPath, err := filepath.Abs(f)
		require.NoError(t, err, "could not resolve migration path: %s", f)

		content, err := os.ReadFile(absPath)
		require.NoError(t, err, "could not read migration file: %s", f)

		sql := extractUpSQL(string(content))
		if sql == "" {
			sql = string(content)
		}

		_, err = sqlDB.Exec(sql)
		require.NoError(t, err, "could not execute migration: %s", f)
		t.Logf("executed migration: %s", f)
	}
}

func extractUpSQL(content string) string {
	lines := strings.Split(content, "\n")
	var inUp bool
	var sql []string

	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if strings.Contains(trimmed, "+goose Up") {
			inUp = true
			continue
		}
		if strings.Contains(trimmed, "+goose Down") {
			break
		}
		if strings.Contains(trimmed, "+goose StatementBegin") || strings.Contains(trimmed, "+goose StatementEnd") {
			continue
		}
		if inUp {
			sql = append(sql, line)
		}
	}

	return strings.Join(sql, "\n")
}

func seedData(t *testing.T, db *gorm.DB) {
	t.Helper()

	// Generate proper bcrypt hash for "Test1234!"
	passwordHash, err := crypto.HashPassword("Test1234!")
	require.NoError(t, err, "could not hash password")

	require.NoError(t, db.Exec(`INSERT INTO provinsi (kode, nama) VALUES ('11', 'ACEH') ON CONFLICT (kode) DO NOTHING`).Error)
	require.NoError(t, db.Exec(`INSERT INTO kabupaten (kode, nama, jenis, ibukota, lat, lng, provinsi_kode) VALUES ('1171', 'KOTA BANDA ACEH', 'Kota', 'Banda Aceh', 5.55, 95.32, '11') ON CONFLICT (kode) DO NOTHING`).Error)

	var count int64
	db.Table("pengguna").Where("email = ?", "test@garda-inflasi.id").Count(&count)
	if count == 0 {
		require.NoError(t, db.Exec(`
			INSERT INTO pengguna (id, nama, email, password_hash, peran, lapis, lembaga, kabupaten_kode, status)
			VALUES ('test-user-001', 'Test User', 'test@garda-inflasi.id', ?, 'ADMIN_SISTEM', 'SISTEM', 'BPPSDMP', '1171', 'AKTIF')
		`, passwordHash).Error)
	}

	db.Table("pengguna").Where("email = ?", "inactive@garda-inflasi.id").Count(&count)
	if count == 0 {
		require.NoError(t, db.Exec(`
			INSERT INTO pengguna (id, nama, email, password_hash, peran, lapis, lembaga, status)
			VALUES ('test-user-002', 'Inactive User', 'inactive@garda-inflasi.id', ?, 'OPS_MAHASISWA', 'OPERASIONAL', 'BPPSDMP', 'NONAKTIF')
		`, passwordHash).Error)
	}

	db.Table("pengguna").Where("email = ?", "mfa@garda-inflasi.id").Count(&count)
	if count == 0 {
		mfaSecret := "JBSWY3DPEHPK3PXP"
		backupCodes := `["BACKUP01", "BACKUP02", "BACKUP03"]`
		require.NoError(t, db.Exec(`
			INSERT INTO pengguna (id, nama, email, password_hash, peran, lapis, lembaga, status, mfa_aktif, mfa_secret, mfa_backup_codes)
			VALUES ('test-user-mfa', 'MFA User', 'mfa@garda-inflasi.id', ?, 'OPS_KOORDINATOR', 'OPERASIONAL', 'BPPSDMP', 'AKTIF', true, ?, ?::jsonb)
		`, passwordHash, mfaSecret, backupCodes).Error)
	}

	t.Log("seed data inserted")
}

// ── Helper Functions ──────────────────────────────────────────────────

func (s *Suite) serveRequest(t *testing.T, method, path string, body interface{}, headers map[string]string) *httptest.ResponseRecorder {
	t.Helper()

	var reqBody io.Reader
	if body != nil {
		b, err := json.Marshal(body)
		require.NoError(t, err)
		reqBody = bytes.NewReader(b)
	}

	req := httptest.NewRequest(method, path, reqBody)
	req.Header.Set("Content-Type", "application/json")
	for k, v := range headers {
		req.Header.Set(k, v)
	}

	w := httptest.NewRecorder()
	s.router.ServeHTTP(w, req)
	return w
}

func (s *Suite) parseResponse(t *testing.T, w *httptest.ResponseRecorder) map[string]interface{} {
	t.Helper()

	var result map[string]interface{}
	err := json.Unmarshal(w.Body.Bytes(), &result)
	require.NoError(t, err, "could not parse response body: %s", w.Body.String())
	return result
}

func (s *Suite) loginAs(t *testing.T, email, password string) (string, string) {
	t.Helper()

	w := s.serveRequest(t, http.MethodPost, "/v1/auth/login", map[string]interface{}{
		"email":    email,
		"password": password,
	}, nil)

	require.Equal(t, http.StatusOK, w.Code, "login failed: %s", w.Body.String())

	result := s.parseResponse(t, w)
	data, ok := result["data"].(map[string]interface{})
	require.True(t, ok, "no data in response")

	accessToken, _ := data["access_token"].(string)
	refreshToken, _ := data["refresh_token"].(string)
	require.NotEmpty(t, accessToken, "no access token")
	require.NotEmpty(t, refreshToken, "no refresh token")

	return accessToken, refreshToken
}

func (s *Suite) authHeader(token string) map[string]string {
	return map[string]string{
		"Authorization": "Bearer " + token,
	}
}

// ── Tests ─────────────────────────────────────────────────────────────

func TestIntegration_AuthFlow(t *testing.T) {
	s := setupSuite(t)

	t.Run("Health Check", func(t *testing.T) {
		w := s.serveRequest(t, http.MethodGet, "/healthz", nil, nil)
		assert.Equal(t, http.StatusOK, w.Code)

		result := s.parseResponse(t, w)
		assert.Equal(t, "ok", result["status"])
	})

	t.Run("Login Success", func(t *testing.T) {
		accessToken, refreshToken := s.loginAs(t, "test@garda-inflasi.id", "Test1234!")
		assert.NotEmpty(t, accessToken)
		assert.NotEmpty(t, refreshToken)
	})

	t.Run("Login Invalid Credentials", func(t *testing.T) {
		w := s.serveRequest(t, http.MethodPost, "/v1/auth/login", map[string]interface{}{
			"email":    "test@garda-inflasi.id",
			"password": "wrongpassword",
		}, nil)

		assert.Equal(t, http.StatusUnauthorized, w.Code)
		result := s.parseResponse(t, w)
		assert.Contains(t, result["message"], "Email atau password salah")
	})

	t.Run("Login Inactive Account", func(t *testing.T) {
		w := s.serveRequest(t, http.MethodPost, "/v1/auth/login", map[string]interface{}{
			"email":    "inactive@garda-inflasi.id",
			"password": "Test1234!",
		}, nil)

		assert.Equal(t, http.StatusForbidden, w.Code)
		result := s.parseResponse(t, w)
		assert.Contains(t, result["message"], "Akun tidak aktif, hubungi administrator")
	})

	t.Run("Login Nonexistent Email", func(t *testing.T) {
		w := s.serveRequest(t, http.MethodPost, "/v1/auth/login", map[string]interface{}{
			"email":    "nonexistent@garda-inflasi.id",
			"password": "Test1234!",
		}, nil)

		assert.Equal(t, http.StatusUnauthorized, w.Code)
	})

	t.Run("Get Me", func(t *testing.T) {
		accessToken, _ := s.loginAs(t, "test@garda-inflasi.id", "Test1234!")

		w := s.serveRequest(t, http.MethodGet, "/v1/auth/me", nil, s.authHeader(accessToken))
		assert.Equal(t, http.StatusOK, w.Code)

		result := s.parseResponse(t, w)
		data, ok := result["data"].(map[string]interface{})
		require.True(t, ok)
		assert.Equal(t, "test-user-001", data["id"])
		assert.Equal(t, "Test User", data["nama"])
		assert.Equal(t, "test@garda-inflasi.id", data["email"])
	})

	t.Run("Get Me Unauthorized", func(t *testing.T) {
		w := s.serveRequest(t, http.MethodGet, "/v1/auth/me", nil, nil)
		assert.Equal(t, http.StatusUnauthorized, w.Code)
	})

	t.Run("Refresh Token", func(t *testing.T) {
		_, refreshToken := s.loginAs(t, "test@garda-inflasi.id", "Test1234!")

		w := s.serveRequest(t, http.MethodPost, "/v1/auth/refresh", map[string]interface{}{
			"refresh_token": refreshToken,
		}, nil)

		assert.Equal(t, http.StatusOK, w.Code)
		result := s.parseResponse(t, w)
		data, ok := result["data"].(map[string]interface{})
		require.True(t, ok)
		assert.NotEmpty(t, data["access_token"])
		assert.NotEmpty(t, data["refresh_token"])
	})

	t.Run("Refresh Token Reuse Detection", func(t *testing.T) {
		_, refreshToken := s.loginAs(t, "test@garda-inflasi.id", "Test1234!")

		w1 := s.serveRequest(t, http.MethodPost, "/v1/auth/refresh", map[string]interface{}{
			"refresh_token": refreshToken,
		}, nil)
		assert.Equal(t, http.StatusOK, w1.Code)

		w2 := s.serveRequest(t, http.MethodPost, "/v1/auth/refresh", map[string]interface{}{
			"refresh_token": refreshToken,
		}, nil)
		assert.Equal(t, http.StatusUnauthorized, w2.Code)
	})

	t.Run("Logout", func(t *testing.T) {
		accessToken, _ := s.loginAs(t, "test@garda-inflasi.id", "Test1234!")

		w := s.serveRequest(t, http.MethodPost, "/v1/auth/logout", nil, s.authHeader(accessToken))
		assert.Equal(t, http.StatusOK, w.Code)
	})

	t.Run("Change Password", func(t *testing.T) {
		accessToken, _ := s.loginAs(t, "test@garda-inflasi.id", "Test1234!")

		w := s.serveRequest(t, http.MethodPost, "/v1/auth/change-password", map[string]interface{}{
			"old_password": "Test1234!",
			"new_password": "NewTest5678@",
		}, s.authHeader(accessToken))

		assert.Equal(t, http.StatusOK, w.Code)

		newAccessToken, _ := s.loginAs(t, "test@garda-inflasi.id", "NewTest5678@")
		assert.NotEmpty(t, newAccessToken)

		w2 := s.serveRequest(t, http.MethodPost, "/v1/auth/change-password", map[string]interface{}{
			"old_password": "NewTest5678@",
			"new_password": "Test1234!",
		}, s.authHeader(newAccessToken))
		assert.Equal(t, http.StatusOK, w2.Code)
	})

	t.Run("Change Password Wrong Old Password", func(t *testing.T) {
		accessToken, _ := s.loginAs(t, "test@garda-inflasi.id", "Test1234!")

		w := s.serveRequest(t, http.MethodPost, "/v1/auth/change-password", map[string]interface{}{
			"old_password": "WrongPassword1!",
			"new_password": "NewTest5678@",
		}, s.authHeader(accessToken))

		assert.Equal(t, http.StatusUnauthorized, w.Code)
	})

	t.Run("Forgot Password", func(t *testing.T) {
		w := s.serveRequest(t, http.MethodPost, "/v1/auth/forgot-password", map[string]interface{}{
			"email": "test@garda-inflasi.id",
		}, nil)

		assert.Equal(t, http.StatusOK, w.Code)
	})

	t.Run("Forgot Password Nonexistent Email", func(t *testing.T) {
		w := s.serveRequest(t, http.MethodPost, "/v1/auth/forgot-password", map[string]interface{}{
			"email": "nonexistent@garda-inflasi.id",
		}, nil)

		assert.Equal(t, http.StatusOK, w.Code)
	})

	t.Run("Security Headers", func(t *testing.T) {
		w := s.serveRequest(t, http.MethodGet, "/healthz", nil, nil)

		assert.Equal(t, "nosniff", w.Header().Get("X-Content-Type-Options"))
		assert.Equal(t, "DENY", w.Header().Get("X-Frame-Options"))
		assert.NotEmpty(t, w.Header().Get("Content-Security-Policy"))
		assert.NotEmpty(t, w.Header().Get("X-Request-ID"))
	})

	t.Run("CORS Headers", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/healthz", nil)
		req.Header.Set("Origin", "http://localhost:3000")
		w := httptest.NewRecorder()
		s.router.ServeHTTP(w, req)

		assert.Equal(t, "http://localhost:3000", w.Header().Get("Access-Control-Allow-Origin"))
	})
}

func TestIntegration_MFAFlow(t *testing.T) {
	s := setupSuite(t)

	t.Run("MFA Setup", func(t *testing.T) {
		accessToken, _ := s.loginAs(t, "test@garda-inflasi.id", "Test1234!")

		w := s.serveRequest(t, http.MethodPost, "/v1/auth/mfa/setup", nil, s.authHeader(accessToken))
		t.Logf("MFA Setup status: %d, body: %s", w.Code, w.Body.String())
		assert.Equal(t, http.StatusOK, w.Code)

		result := s.parseResponse(t, w)
		t.Logf("result: %+v", result)
		data, ok := result["data"].(map[string]interface{})
		require.True(t, ok, "data should be a map, got type: %T, value: %+v", result["data"], result["data"])
		assert.NotEmpty(t, data["secret"])
		assert.NotEmpty(t, data["provisioning_uri"])
		assert.NotEmpty(t, data["backup_codes"])
	})

	t.Run("MFA Setup Already Active", func(t *testing.T) {
		accessToken, _ := s.loginAs(t, "mfa@garda-inflasi.id", "Test1234!")

		w := s.serveRequest(t, http.MethodPost, "/v1/auth/mfa/setup", nil, s.authHeader(accessToken))
		assert.Equal(t, http.StatusBadRequest, w.Code)
	})

	t.Run("MFA Verify Invalid Code", func(t *testing.T) {
		accessToken, _ := s.loginAs(t, "test@garda-inflasi.id", "Test1234!")

		s.serveRequest(t, http.MethodPost, "/v1/auth/mfa/setup", nil, s.authHeader(accessToken))

		w := s.serveRequest(t, http.MethodPost, "/v1/auth/mfa/verify", map[string]interface{}{
			"code": "000000",
		}, s.authHeader(accessToken))

		assert.Equal(t, http.StatusBadRequest, w.Code)
	})

	t.Run("MFA Disable", func(t *testing.T) {
		accessToken, _ := s.loginAs(t, "mfa@garda-inflasi.id", "Test1234!")

		w := s.serveRequest(t, http.MethodPost, "/v1/auth/mfa/disable", map[string]interface{}{
			"password": "Test1234!",
		}, s.authHeader(accessToken))

		assert.Equal(t, http.StatusOK, w.Code)
	})

	t.Run("MFA Disable Wrong Password", func(t *testing.T) {
		accessToken, _ := s.loginAs(t, "mfa@garda-inflasi.id", "Test1234!")

		w := s.serveRequest(t, http.MethodPost, "/v1/auth/mfa/disable", map[string]interface{}{
			"password": "WrongPassword1!",
		}, s.authHeader(accessToken))

		assert.Equal(t, http.StatusUnauthorized, w.Code)
	})
}

func TestIntegration_UserManagement(t *testing.T) {
	s := setupSuite(t)

	t.Run("List Users Requires Permission", func(t *testing.T) {
		accessToken, _ := s.loginAs(t, "mfa@garda-inflasi.id", "Test1234!")

		w := s.serveRequest(t, http.MethodGet, "/v1/users", nil, s.authHeader(accessToken))
		assert.Equal(t, http.StatusForbidden, w.Code)
	})

	t.Run("Create User Requires Permission", func(t *testing.T) {
		accessToken, _ := s.loginAs(t, "mfa@garda-inflasi.id", "Test1234!")

		w := s.serveRequest(t, http.MethodPost, "/v1/users", map[string]interface{}{
			"nama":    "New User",
			"email":   "newuser@garda-inflasi.id",
			"peran":   "OPS_MAHASISWA",
			"lapis":   "OPERASIONAL",
			"lembaga": "BPPSDMP",
		}, s.authHeader(accessToken))
		assert.Equal(t, http.StatusForbidden, w.Code)
	})
}

func TestIntegration_InvalidInputs(t *testing.T) {
	s := setupSuite(t)

	t.Run("Login Empty Body", func(t *testing.T) {
		w := s.serveRequest(t, http.MethodPost, "/v1/auth/login", map[string]interface{}{}, nil)
		assert.Equal(t, http.StatusBadRequest, w.Code)
	})

	t.Run("Login Invalid JSON", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodPost, "/v1/auth/login", bytes.NewReader([]byte("invalid json")))
		req.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()
		s.router.ServeHTTP(w, req)
		assert.Equal(t, http.StatusBadRequest, w.Code)
	})

	t.Run("Refresh Empty Token", func(t *testing.T) {
		w := s.serveRequest(t, http.MethodPost, "/v1/auth/refresh", map[string]interface{}{
			"refresh_token": "",
		}, nil)
		assert.Equal(t, http.StatusBadRequest, w.Code)
	})

	t.Run("Access Protected Route Without Token", func(t *testing.T) {
		w := s.serveRequest(t, http.MethodGet, "/v1/auth/me", nil, nil)
		assert.Equal(t, http.StatusUnauthorized, w.Code)
	})

	t.Run("Access Protected Route With Invalid Token", func(t *testing.T) {
		w := s.serveRequest(t, http.MethodGet, "/v1/auth/me", nil, map[string]string{
			"Authorization": "Bearer invalidtoken123",
		})
		assert.Equal(t, http.StatusUnauthorized, w.Code)
	})
}

func TestIntegration_AccountLockout(t *testing.T) {
	s := setupSuite(t)

	t.Run("Account Locked After Multiple Failed Attempts", func(t *testing.T) {
		for i := 0; i < 5; i++ {
			w := s.serveRequest(t, http.MethodPost, "/v1/auth/login", map[string]interface{}{
				"email":    "test@garda-inflasi.id",
				"password": fmt.Sprintf("WrongPass%d!", i),
			}, nil)
			assert.Equal(t, http.StatusUnauthorized, w.Code)
		}

		w := s.serveRequest(t, http.MethodPost, "/v1/auth/login", map[string]interface{}{
			"email":    "test@garda-inflasi.id",
			"password": "Test1234!",
		}, nil)
		assert.Equal(t, http.StatusTooManyRequests, w.Code)
		result := s.parseResponse(t, w)
		assert.Contains(t, result["message"], "Akun terkunci, coba lagi nanti")
	})
}

func TestIntegration_ResetPassword(t *testing.T) {
	s := setupSuite(t)

	t.Run("Reset Password Invalid Token", func(t *testing.T) {
		w := s.serveRequest(t, http.MethodPost, "/v1/auth/reset-password", map[string]interface{}{
			"token":    "invalid-token",
			"password": "NewTest5678@",
		}, nil)

		assert.Equal(t, http.StatusUnauthorized, w.Code)
	})

	t.Run("Reset Password Weak Password", func(t *testing.T) {
		w := s.serveRequest(t, http.MethodPost, "/v1/auth/reset-password", map[string]interface{}{
			"token":    "some-token",
			"password": "weak",
		}, nil)

		assert.Equal(t, http.StatusBadRequest, w.Code)
	})
}

func TestIntegration_WeakPassword(t *testing.T) {
	s := setupSuite(t)

	t.Run("Change Password Weak Password", func(t *testing.T) {
		accessToken, _ := s.loginAs(t, "test@garda-inflasi.id", "Test1234!")

		w := s.serveRequest(t, http.MethodPost, "/v1/auth/change-password", map[string]interface{}{
			"old_password": "Test1234!",
			"new_password": "weak",
		}, s.authHeader(accessToken))

		assert.Equal(t, http.StatusBadRequest, w.Code)
	})
}

func TestIntegration_BackupCode(t *testing.T) {
	s := setupSuite(t)

	t.Run("Validate Backup Code", func(t *testing.T) {
		accessToken, _ := s.loginAs(t, "mfa@garda-inflasi.id", "Test1234!")

		w := s.serveRequest(t, http.MethodPost, "/v1/auth/mfa/validate", map[string]interface{}{
			"code": "BACKUP01",
		}, s.authHeader(accessToken))

		assert.Equal(t, http.StatusOK, w.Code)
	})

	t.Run("Validate Invalid Backup Code", func(t *testing.T) {
		accessToken, _ := s.loginAs(t, "mfa@garda-inflasi.id", "Test1234!")

		w := s.serveRequest(t, http.MethodPost, "/v1/auth/mfa/validate", map[string]interface{}{
			"code": "INVALID99",
		}, s.authHeader(accessToken))

		assert.Equal(t, http.StatusBadRequest, w.Code)
	})
}

func TestIntegration_AllRoutesRegistered(t *testing.T) {
	s := setupSuite(t)

	t.Run("API v1 Routes Exist", func(t *testing.T) {
		routes := []string{
			"/v1/auth/login",
			"/v1/auth/refresh",
			"/v1/auth/forgot-password",
			"/v1/auth/reset-password",
		}

		for _, route := range routes {
			w := s.serveRequest(t, http.MethodPost, route, map[string]interface{}{}, nil)
			assert.NotEqual(t, http.StatusNotFound, w.Code, "route %s should exist", route)
		}
	})
}
