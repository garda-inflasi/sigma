package integration

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/garda-inflasi/api/internal/auth/routes"
	authseeder "github.com/garda-inflasi/api/internal/auth/seeder"
	"github.com/garda-inflasi/api/internal/auth/usecase"
	"github.com/garda-inflasi/api/internal/modules"
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

// ── Test Suite ────────────────────────────────────────────────────────

type Suite struct {
	db       *gorm.DB
	tokenMgr *token.Manager
	router   *gin.Engine
}

var jwtSecret = "integration-test-secret-key-32chars!!"

// Rahasia dan kode cadangan akun uji ber-MFA (uji.mfa@gardainflasi.id).
const mfaSecretUji = "JBSWY3DPEHPK3PXP"

var kodeCadanganUji = []string{"BACKUP01", "BACKUP02", "BACKUP03"}

var pg *pgtest.Server

func TestMain(m *testing.M) {
	gin.SetMode(gin.TestMode)
	os.Exit(pgtest.Main(m, &pg))
}

// setupSuite menyiapkan basis data khusus tes ini (migrasi + akun uji) dan router.
func setupSuite(t *testing.T) *Suite {
	t.Helper()
	return setupSuiteWith(t, seedData)
}

// setupSuiteWith seperti setupSuite dengan isi basis data yang ditentukan pemanggil.
func setupSuiteWith(t *testing.T, seed func(*testing.T, *gorm.DB)) *Suite {
	t.Helper()
	return newSuite(t, seed)
}

// newSuite seperti setupSuiteWith; opts (bila ada) menggantikan konfigurasi usecase dari lingkungan.
// Tanpa pembatas laju auth dan tanpa daftar proxy tepercaya khusus (seperti hampir semua tes).
func newSuite(t *testing.T, seed func(*testing.T, *gorm.DB), opts ...routes.Options) *Suite {
	t.Helper()
	return newSuiteBatas(t, seed, 0, nil, opts...)
}

// newSuiteBatas seperti newSuite dengan pembatas laju auth per IP (batasAuth permintaan per menit; 0 = mati) dan,
// bila proxies tidak nil, daftar proxy tepercaya untuk X-Forwarded-For (kosong tetapi tidak nil = tak seorang pun).
func newSuiteBatas(t *testing.T, seed func(*testing.T, *gorm.DB), batasAuth int, proxies []string, opts ...routes.Options) *Suite {
	t.Helper()

	db := pg.NewDatabase(t, strings.ToLower(t.Name()))
	pgtest.MigrateUp(t, db)
	// Izin dibaca dari basis data (02-C): tanpa referensi RBAC tidak ada peran yang punya izin apa pun.
	_, err := authseeder.SeedRBAC(t.Context(), db)
	require.NoError(t, err)
	seed(t, db)

	tokenMgr := token.NewManager(jwtSecret)

	router := gin.New()
	router.Use(gin.Recovery())
	if proxies != nil {
		require.NoError(t, router.SetTrustedProxies(proxies))
	}
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
		DB:                    db,
		Token:                 tokenMgr,
		Logger:                log,
		AuthRateLimitPerMenit: batasAuth,
	}, opts...)

	modules.RegisterAll(v1, db, log, tokenMgr)

	return &Suite{
		db:       db,
		tokenMgr: tokenMgr,
		router:   router,
	}
}

func seedData(t *testing.T, db *gorm.DB) {
	t.Helper()

	// Generate proper bcrypt hash for "Test1234!"
	passwordHash, err := crypto.HashPassword("Test1234!")
	require.NoError(t, err, "could not hash password")

	require.NoError(t, db.Exec(`INSERT INTO provinsi (kode, nama) VALUES ('11', 'ACEH') ON CONFLICT (kode) DO NOTHING`).Error)
	require.NoError(t, db.Exec(`INSERT INTO kabupaten (kode, nama, jenis, ibukota, lat, lng, provinsi_kode) VALUES ('1171', 'KOTA BANDA ACEH', 'Kota', 'Banda Aceh', 5.55, 95.32, '11') ON CONFLICT (kode) DO NOTHING`).Error)

	var count int64
	db.Table("pengguna").Where("email = ?", "test@gardainflasi.id").Count(&count)
	if count == 0 {
		require.NoError(t, db.Exec(`
			INSERT INTO pengguna (id, nama, email, password_hash, peran, lapis, lembaga, kabupaten_kode, status)
			VALUES (garda_uuid_stabil('pengguna', 'test-user-001'), 'Test User', 'test@gardainflasi.id', ?, 'ADMIN_SISTEM', 'SISTEM', 'BPPSDMP', '1171', 'AKTIF')
		`, passwordHash).Error)
	}

	db.Table("pengguna").Where("email = ?", "inactive@gardainflasi.id").Count(&count)
	if count == 0 {
		require.NoError(t, db.Exec(`
			INSERT INTO pengguna (id, nama, email, password_hash, peran, lapis, lembaga, status)
			VALUES (garda_uuid_stabil('pengguna', 'test-user-002'), 'Inactive User', 'inactive@gardainflasi.id', ?, 'OPS_MAHASISWA', 'OPERASIONAL', 'BPPSDMP', 'NONAKTIF')
		`, passwordHash).Error)
	}

	db.Table("pengguna").Where("email = ?", "uji.biasa@gardainflasi.id").Count(&count)
	if count == 0 {
		require.NoError(t, db.Exec(`
			INSERT INTO pengguna (id, nama, email, password_hash, peran, lapis, lembaga, status)
			VALUES (garda_uuid_stabil('pengguna', 'test-user-biasa'), 'Pengguna Biasa', 'uji.biasa@gardainflasi.id', ?, 'OPS_MAHASISWA', 'OPERASIONAL', 'BPPSDMP', 'AKTIF')
		`, passwordHash).Error)
	}

	db.Table("pengguna").Where("email = ?", "uji.mfa@gardainflasi.id").Count(&count)
	if count == 0 {
		ring, err := crypto.KeyringFromEnv(os.Getenv)
		require.NoError(t, err)
		sealed, err := ring.Seal(mfaSecretUji)
		require.NoError(t, err)
		require.NoError(t, db.Exec(`
			INSERT INTO pengguna (id, nama, email, password_hash, peran, lapis, lembaga, status, mfa_aktif, mfa_secret)
			VALUES (garda_uuid_stabil('pengguna', 'test-user-mfa'), 'MFA User', 'uji.mfa@gardainflasi.id', ?, 'OPS_KOORDINATOR', 'OPERASIONAL', 'BPPSDMP', 'AKTIF', true, ?)
		`, passwordHash, sealed).Error)
		for _, kode := range kodeCadanganUji {
			require.NoError(t, db.Exec("INSERT INTO mfa_kode_cadangan (user_id, kode_hash) VALUES (garda_uuid_stabil('pengguna', 'test-user-mfa'), ?)",
				usecase.HashKodeCadangan(uid("test-user-mfa"), kode)).Error)
		}
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
		accessToken, refreshToken := s.loginAs(t, "test@gardainflasi.id", "Test1234!")
		assert.NotEmpty(t, accessToken)
		assert.NotEmpty(t, refreshToken)
	})

	t.Run("Login Invalid Credentials", func(t *testing.T) {
		w := s.serveRequest(t, http.MethodPost, "/v1/auth/login", map[string]interface{}{
			"email":    "test@gardainflasi.id",
			"password": "wrongpassword",
		}, nil)

		assert.Equal(t, http.StatusUnauthorized, w.Code)
		result := s.parseResponse(t, w)
		assert.Contains(t, result["message"], "Email atau password salah")
		assert.Equal(t, "INVALID_CREDENTIALS", result["code"])
	})

	t.Run("Login Inactive Account", func(t *testing.T) {
		w := s.serveRequest(t, http.MethodPost, "/v1/auth/login", map[string]interface{}{
			"email":    "inactive@gardainflasi.id",
			"password": "Test1234!",
		}, nil)

		assert.Equal(t, http.StatusForbidden, w.Code)
		result := s.parseResponse(t, w)
		assert.Contains(t, result["message"], "Akun tidak aktif, hubungi administrator")
		assert.Equal(t, "ACCOUNT_INACTIVE", result["code"])
	})

	t.Run("Login Nonexistent Email", func(t *testing.T) {
		w := s.serveRequest(t, http.MethodPost, "/v1/auth/login", map[string]interface{}{
			"email":    "nonexistent@gardainflasi.id",
			"password": "Test1234!",
		}, nil)

		assert.Equal(t, http.StatusUnauthorized, w.Code)
	})

	t.Run("Get Me", func(t *testing.T) {
		accessToken, _ := s.loginAs(t, "test@gardainflasi.id", "Test1234!")

		w := s.serveRequest(t, http.MethodGet, "/v1/auth/me", nil, s.authHeader(accessToken))
		assert.Equal(t, http.StatusOK, w.Code)

		result := s.parseResponse(t, w)
		data, ok := result["data"].(map[string]interface{})
		require.True(t, ok)
		assert.Equal(t, uid("test-user-001"), data["id"])
		assert.Equal(t, "Test User", data["nama"])
		assert.Equal(t, "test@gardainflasi.id", data["email"])
	})

	t.Run("Get Me Unauthorized", func(t *testing.T) {
		w := s.serveRequest(t, http.MethodGet, "/v1/auth/me", nil, nil)
		assert.Equal(t, http.StatusUnauthorized, w.Code)
	})

	t.Run("Refresh Token", func(t *testing.T) {
		_, refreshToken := s.loginAs(t, "test@gardainflasi.id", "Test1234!")

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
		_, refreshToken := s.loginAs(t, "test@gardainflasi.id", "Test1234!")

		w1 := s.serveRequest(t, http.MethodPost, "/v1/auth/refresh", map[string]interface{}{
			"refresh_token": refreshToken,
		}, nil)
		assert.Equal(t, http.StatusOK, w1.Code)

		// Pemakaian ulang di luar jendela toleransi (rotasi dimundurkan) dianggap token dicuri.
		s.ageRotation(t, refreshToken)
		w2 := s.serveRequest(t, http.MethodPost, "/v1/auth/refresh", map[string]interface{}{
			"refresh_token": refreshToken,
		}, nil)
		assert.Equal(t, http.StatusUnauthorized, w2.Code)
	})

	t.Run("Logout", func(t *testing.T) {
		accessToken, _ := s.loginAs(t, "test@gardainflasi.id", "Test1234!")

		w := s.serveRequest(t, http.MethodPost, "/v1/auth/logout", nil, s.authHeader(accessToken))
		assert.Equal(t, http.StatusOK, w.Code)
	})

	t.Run("Change Password", func(t *testing.T) {
		accessToken, _ := s.loginAs(t, "test@gardainflasi.id", "Test1234!")

		w := s.serveRequest(t, http.MethodPost, "/v1/auth/change-password", map[string]interface{}{
			"old_password": "Test1234!",
			"new_password": "NewTest5678@",
		}, s.authHeader(accessToken))

		assert.Equal(t, http.StatusOK, w.Code)

		newAccessToken, _ := s.loginAs(t, "test@gardainflasi.id", "NewTest5678@")
		assert.NotEmpty(t, newAccessToken)

		w2 := s.serveRequest(t, http.MethodPost, "/v1/auth/change-password", map[string]interface{}{
			"old_password": "NewTest5678@",
			"new_password": "Test1234!",
		}, s.authHeader(newAccessToken))
		assert.Equal(t, http.StatusOK, w2.Code)
	})

	t.Run("Change Password Wrong Old Password", func(t *testing.T) {
		accessToken, _ := s.loginAs(t, "test@gardainflasi.id", "Test1234!")

		w := s.serveRequest(t, http.MethodPost, "/v1/auth/change-password", map[string]interface{}{
			"old_password": "WrongPassword1!",
			"new_password": "NewTest5678@",
		}, s.authHeader(accessToken))

		assert.Equal(t, http.StatusUnauthorized, w.Code)
	})

	t.Run("Forgot Password", func(t *testing.T) {
		w := s.serveRequest(t, http.MethodPost, "/v1/auth/forgot-password", map[string]interface{}{
			"email": "test@gardainflasi.id",
		}, nil)

		assert.Equal(t, http.StatusOK, w.Code)
	})

	t.Run("Forgot Password Nonexistent Email", func(t *testing.T) {
		w := s.serveRequest(t, http.MethodPost, "/v1/auth/forgot-password", map[string]interface{}{
			"email": "nonexistent@gardainflasi.id",
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

func TestIntegration_UserManagement(t *testing.T) {
	s := setupSuite(t)

	t.Run("List Users Requires Permission", func(t *testing.T) {
		accessToken, _ := s.loginAs(t, "uji.biasa@gardainflasi.id", "Test1234!")

		w := s.serveRequest(t, http.MethodGet, "/v1/users", nil, s.authHeader(accessToken))
		assert.Equal(t, http.StatusForbidden, w.Code)
	})

	t.Run("Create User Requires Permission", func(t *testing.T) {
		accessToken, _ := s.loginAs(t, "uji.biasa@gardainflasi.id", "Test1234!")

		w := s.serveRequest(t, http.MethodPost, "/v1/users", map[string]interface{}{
			"nama":    "New User",
			"email":   "newuser@gardainflasi.id",
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
				"email":    "test@gardainflasi.id",
				"password": fmt.Sprintf("WrongPass%d!", i),
			}, nil)
			assert.Equal(t, http.StatusUnauthorized, w.Code)
		}

		w := s.serveRequest(t, http.MethodPost, "/v1/auth/login", map[string]interface{}{
			"email":    "test@gardainflasi.id",
			"password": "Test1234!",
		}, nil)
		assert.Equal(t, http.StatusLocked, w.Code)
		result := s.parseResponse(t, w)
		assert.Equal(t, "ACCOUNT_LOCKED", result["code"])
		assert.Contains(t, result["message"], "Akun terkunci")
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
		accessToken, _ := s.loginAs(t, "test@gardainflasi.id", "Test1234!")

		w := s.serveRequest(t, http.MethodPost, "/v1/auth/change-password", map[string]interface{}{
			"old_password": "Test1234!",
			"new_password": "weak",
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
