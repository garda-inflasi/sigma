package smoketest

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"testing"

	authRoutes "github.com/garda-inflasi/api/internal/auth/routes"
	"github.com/garda-inflasi/api/internal/modules"
	"github.com/garda-inflasi/api/internal/platform/pgtest"
	"github.com/garda-inflasi/api/pkg/logger"
	"github.com/garda-inflasi/api/pkg/token"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

const (
	jwtSecret = "smoke-test-secret-key-at-least-32-chars"
	// Kata sandi seluruh akun demo.
	demoPassword = "Garda2024!"
)

var (
	pg *pgtest.Server

	seedBinaryOnce sync.Once
	seedBinaryPath string
	seedBinaryErr  error
)

// apiRoot adalah direktori apps/api (modul Go).
func apiRoot() string {
	_, file, _, _ := runtime.Caller(0)
	return filepath.Join(filepath.Dir(file), "..", "..", "..")
}

// seedBinary membangun cmd/seed sekali per proses uji, sehingga tes juga
// menguji pendaftaran seeder dan CLI yang dipakai di deploy.
func seedBinary(t *testing.T) string {
	t.Helper()
	seedBinaryOnce.Do(func() {
		dir, err := os.MkdirTemp("", "garda-seed-*")
		if err != nil {
			seedBinaryErr = err
			return
		}
		seedBinaryPath = filepath.Join(dir, "seed")
		build := exec.Command("go", "build", "-o", seedBinaryPath, "./cmd/seed")
		build.Dir = apiRoot()
		if out, err := build.CombinedOutput(); err != nil {
			seedBinaryErr = fmt.Errorf("go build ./cmd/seed: %w\n%s", err, out)
		}
	})
	require.NoError(t, seedBinaryErr)
	return seedBinaryPath
}

func removeSeedBinary() {
	if seedBinaryPath != "" {
		_ = os.RemoveAll(filepath.Dir(seedBinaryPath))
	}
}

// maksBarisKeluaran membatasi panjang satu baris keluaran seed yang ditampilkan di pesan tes.
const maksBarisKeluaran = 2000

// runSeed menjalankan cmd/seed terhadap basis data dbName dan mengembalikan
// keluaran gabungannya.
func runSeed(t *testing.T, dbName, appEnv, tier string) (string, error) {
	t.Helper()
	cmd := exec.Command(seedBinary(t), tier)
	cmd.Env = append(os.Environ(), "DATABASE_URL="+pg.DSN(dbName), "APP_ENV="+appEnv)
	out, err := cmd.CombinedOutput()
	return ringkasKeluaran(string(out)), err
}

// ringkasKeluaran memotong baris yang sangat panjang. Saat INSERT massal gagal, GORM mencetak SQL beserta seluruh
// nilainya dalam satu baris ratusan KB; testify tidak sanggup menampilkan pesan dengan baris di atas 64 KB ("token
// too long"), sehingga penyebab galat seed hilang dari laporan tes.
func ringkasKeluaran(out string) string {
	baris := strings.Split(out, "\n")
	for i, b := range baris {
		if len(b) > maksBarisKeluaran {
			baris[i] = b[:maksBarisKeluaran] + "... [dipotong]"
		}
	}
	return strings.Join(baris, "\n")
}

// seedAll menjalankan lapis reference lalu demo sebagai pengembang lokal.
func seedAll(t *testing.T, dbName string) {
	t.Helper()
	for _, tier := range []string{"reference", "demo"} {
		out, err := runSeed(t, dbName, "development", tier)
		require.NoError(t, err, "seed %s:\n%s", tier, out)
	}
}

// stack adalah router penuh di atas basis data yang sudah dimigrasi dan di-seed.
type stack struct {
	db     *gorm.DB
	router *gin.Engine
	tokens map[string]string // email -> access token
}

var (
	sharedOnce  sync.Once
	sharedStack *stack
)

// shared membuat satu stack untuk semua tes yang hanya membaca.
func shared(t *testing.T) *stack {
	t.Helper()
	sharedOnce.Do(func() {
		sharedStack = newStack(t, pg.CreateDatabase(t, "smoke_shared"), "smoke_shared")
	})
	require.NotNil(t, sharedStack, "stack bersama gagal dibuat pada tes pertama yang memakainya")
	return sharedStack
}

func newStack(t *testing.T, db *gorm.DB, dbName string) *stack {
	t.Helper()
	pgtest.MigrateUp(t, db)
	seedAll(t, dbName)
	return newRouterStack(db)
}

// newRouterStack merakit router penuh di atas basis data yang sudah dimigrasi dan di-seed. opts (bila ada)
// menggantikan konfigurasi autentikasi dari lingkungan, mis. untuk menyalakan penegakan MFA.
func newRouterStack(db *gorm.DB, opts ...authRoutes.Options) *stack {
	tokenMgr := token.NewManager(jwtSecret)
	log := logger.NewDefault()

	router := gin.New()
	router.Use(gin.Recovery())
	v1 := router.Group("/v1")
	authRoutes.Register(v1, &authRoutes.Dependencies{DB: db, Token: tokenMgr, Logger: log}, opts...)
	modules.RegisterAll(v1, db, log, tokenMgr)

	return &stack{db: db, router: router, tokens: map[string]string{}}
}

// envelope adalah bentuk respons standar {code, message, data}.
type envelope struct {
	Code    string          `json:"code"`
	Message string          `json:"message"`
	Data    json.RawMessage `json:"data"`
}

func readEnvelope(t *testing.T, w *httptest.ResponseRecorder) envelope {
	t.Helper()
	var env envelope
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &env), "respons bukan envelope: %s", w.Body.String())
	return env
}

func (s *stack) do(t *testing.T, method, path, bearer string, body any) *httptest.ResponseRecorder {
	t.Helper()
	return s.doWith(t, method, path, bearer, nil, body)
}

func (s *stack) doWith(t *testing.T, method, path, bearer string, headers map[string]string, body any) *httptest.ResponseRecorder {
	t.Helper()
	var payload io.Reader = http.NoBody
	if body != nil {
		raw, err := json.Marshal(body)
		require.NoError(t, err)
		payload = bytes.NewReader(raw)
	}
	req := httptest.NewRequest(method, path, payload)
	req.Header.Set("Content-Type", "application/json")
	for k, v := range headers {
		req.Header.Set(k, v)
	}
	if bearer != "" {
		req.Header.Set("Authorization", "Bearer "+bearer)
	}
	w := httptest.NewRecorder()
	s.router.ServeHTTP(w, req)
	return w
}

// login mengembalikan access token akun demo; hasilnya di-cache per stack.
func (s *stack) login(t *testing.T, email string) string {
	t.Helper()
	if tok, ok := s.tokens[email]; ok {
		return tok
	}
	w := s.do(t, http.MethodPost, "/v1/auth/login", "", map[string]string{"email": email, "password": demoPassword})
	require.Equal(t, http.StatusOK, w.Code, "login %s: %s", email, w.Body.String())

	var data struct {
		AccessToken string `json:"access_token"`
	}
	require.NoError(t, json.Unmarshal(readEnvelope(t, w).Data, &data))
	require.NotEmpty(t, data.AccessToken, "login %s tidak mengembalikan access_token", email)
	s.tokens[email] = data.AccessToken
	return data.AccessToken
}

// countItems menghitung elemen larik di data pada path bertitik ("stok" atau
// "a.b"); path kosong berarti data itu sendiri larik.
func countItems(t *testing.T, data json.RawMessage, path string) int {
	t.Helper()
	current := data
	if path != "" {
		for _, key := range strings.Split(path, ".") {
			var object map[string]json.RawMessage
			require.NoError(t, json.Unmarshal(current, &object), "data bukan objek saat membuka %q", key)
			next, ok := object[key]
			require.True(t, ok, "kunci %q tidak ada di data", key)
			current = next
		}
	}
	var items []json.RawMessage
	require.NoError(t, json.Unmarshal(current, &items), "%q bukan larik", path)
	return len(items)
}
