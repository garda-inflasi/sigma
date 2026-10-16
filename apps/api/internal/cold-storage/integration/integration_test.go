// Package integration menguji modul cold storage terhadap PostgreSQL sungguhan
// (container dockertest): migrasi, seeder, query SQL, RBAC, dan kontrak OpenAPI.
//
// Memerlukan Docker. Lewati dengan `go test -short`.
package integration

import (
	"bytes"
	"encoding/json"
	"flag"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"testing"
	"time"

	authRoutes "github.com/garda-inflasi/api/internal/auth/routes"
	coldRoutes "github.com/garda-inflasi/api/internal/cold-storage/routes"
	"github.com/garda-inflasi/api/internal/cold-storage/seeder"
	"github.com/garda-inflasi/api/pkg/database"
	"github.com/garda-inflasi/api/pkg/logger"
	"github.com/garda-inflasi/api/pkg/token"
	"github.com/gin-gonic/gin"
	"github.com/ory/dockertest/v3"
	"github.com/ory/dockertest/v3/docker"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gopkg.in/yaml.v3"
	"gorm.io/gorm"
)

const (
	jwtSecret = "integration-test-secret-key-32chars!!"
	// Kata sandi akun contoh dari migrasi 000006.
	seedPassword = "Garda2024!"

	migrationsDir = "../../../../../db/migrations"
	contractPath  = "../../../../../packages/contracts/openapi.yaml"
)

// ── Container PostgreSQL bersama ──────────────────────────────────────

type pgServer struct {
	pool       *dockertest.Pool
	resource   *dockertest.Resource
	host, port string
}

var pg *pgServer

func TestMain(m *testing.M) {
	flag.Parse()
	gin.SetMode(gin.TestMode)

	if !testing.Short() {
		srv, err := startPostgres()
		if err != nil {
			fmt.Fprintf(os.Stderr, "integration: %v\n", err)
			os.Exit(1)
		}
		pg = srv
	}

	code := m.Run()

	if pg != nil {
		if err := pg.pool.Purge(pg.resource); err != nil {
			fmt.Fprintf(os.Stderr, "integration: could not purge container: %v\n", err)
		}
	}
	os.Exit(code)
}

func startPostgres() (*pgServer, error) {
	pool, err := dockertest.NewPool("")
	if err != nil {
		return nil, fmt.Errorf("could not connect to Docker (gunakan -short untuk melewati): %w", err)
	}

	resource, err := pool.RunWithOptions(&dockertest.RunOptions{
		Repository: "postgres",
		Tag:        "16-alpine",
		Env: []string{
			"POSTGRES_USER=garda",
			"POSTGRES_PASSWORD=garda_secret",
			"POSTGRES_DB=postgres",
		},
	}, func(hc *docker.HostConfig) {
		hc.AutoRemove = true
		hc.RestartPolicy = docker.RestartPolicy{Name: "no"}
	})
	if err != nil {
		return nil, fmt.Errorf("could not start PostgreSQL container: %w", err)
	}
	// Jaring pengaman bila proses uji mati sebelum Purge.
	_ = resource.Expire(600)

	parts := strings.Split(resource.GetHostPort("5432/tcp"), ":")
	srv := &pgServer{pool: pool, resource: resource, host: parts[0], port: parts[1]}

	pool.MaxWait = 90 * time.Second
	if err := pool.Retry(func() error {
		db, err := srv.open("postgres")
		if err != nil {
			return err
		}
		defer closeDB(db)
		sqlDB, err := db.DB()
		if err != nil {
			return err
		}
		return sqlDB.Ping()
	}); err != nil {
		_ = pool.Purge(resource)
		return nil, fmt.Errorf("could not connect to database: %w", err)
	}
	return srv, nil
}

func (s *pgServer) open(dbName string) (*gorm.DB, error) {
	return database.New(database.Config{
		Host: s.host, Port: s.port, User: "garda", Password: "garda_secret", DBName: dbName, SSLMode: "disable",
	})
}

func closeDB(db *gorm.DB) {
	if sqlDB, err := db.DB(); err == nil {
		_ = sqlDB.Close()
	}
}

// newDatabase membuat basis data kosong di container bersama; koneksinya
// ditutup saat uji yang membuatnya selesai.
func newDatabase(t *testing.T, name string) *gorm.DB {
	t.Helper()
	db := createDatabase(t, name)
	t.Cleanup(func() { closeDB(db) })
	return db
}

// createDatabase membuat basis data tanpa mengikat masa hidup koneksinya pada uji.
func createDatabase(t *testing.T, name string) *gorm.DB {
	t.Helper()
	if pg == nil {
		t.Skip("integration test dilewati (-short)")
	}

	admin, err := pg.open("postgres")
	require.NoError(t, err)
	require.NoError(t, admin.Exec(`CREATE DATABASE "`+name+`"`).Error)
	closeDB(admin)

	db, err := pg.open(name)
	require.NoError(t, err)
	return db
}

// ── Migrasi ───────────────────────────────────────────────────────────

type migration struct {
	version  int
	name     string
	up, down string
}

// loadMigrations membaca db/migrations/*.sql terurut menurut nama, sehingga
// migrasi baru otomatis ikut diuji.
func loadMigrations(t *testing.T) []migration {
	t.Helper()
	files, err := filepath.Glob(filepath.Join(migrationsDir, "*.sql"))
	require.NoError(t, err)
	require.NotEmpty(t, files, "tidak ada migrasi di %s", migrationsDir)
	sort.Strings(files)

	out := make([]migration, 0, len(files))
	for _, f := range files {
		raw, err := os.ReadFile(f)
		require.NoError(t, err)

		var version int
		_, err = fmt.Sscanf(filepath.Base(f), "%d_", &version)
		require.NoError(t, err, "nama migrasi harus diawali nomor: %s", f)

		up, down := splitGoose(string(raw))
		out = append(out, migration{version: version, name: filepath.Base(f), up: up, down: down})
	}
	return out
}

// splitGoose memisahkan bagian Up/Down dan membuang penanda goose.
func splitGoose(content string) (up, down string) {
	var upLines, downLines []string
	section := ""
	for _, line := range strings.Split(content, "\n") {
		trimmed := strings.TrimSpace(line)
		switch {
		case strings.Contains(trimmed, "+goose Up"):
			section = "up"
		case strings.Contains(trimmed, "+goose Down"):
			section = "down"
		case strings.Contains(trimmed, "+goose StatementBegin"), strings.Contains(trimmed, "+goose StatementEnd"):
		case section == "up":
			upLines = append(upLines, line)
		case section == "down":
			downLines = append(downLines, line)
		}
	}
	return strings.Join(upLines, "\n"), strings.Join(downLines, "\n")
}

func execSQL(t *testing.T, db *gorm.DB, sqlText, what string) {
	t.Helper()
	sqlDB, err := db.DB()
	require.NoError(t, err)
	// database/sql tanpa argumen memakai simple protocol sehingga skrip multi-statement dapat dijalankan.
	_, err = sqlDB.Exec(sqlText)
	require.NoError(t, err, what)
}

// migrateUp menjalankan migrasi Up yang lolos filter, berurutan.
func migrateUp(t *testing.T, db *gorm.DB, include func(version int) bool) {
	t.Helper()
	for _, m := range loadMigrations(t) {
		if include == nil || include(m.version) {
			execSQL(t, db, m.up, "migrate up "+m.name)
		}
	}
}

func findMigration(t *testing.T, version int) migration {
	t.Helper()
	for _, m := range loadMigrations(t) {
		if m.version == version {
			return m
		}
	}
	require.FailNow(t, "migrasi tidak ditemukan", "versi %d", version)
	return migration{}
}

// ── Suite HTTP ────────────────────────────────────────────────────────

type suite struct {
	db     *gorm.DB
	router *gin.Engine
	tokens map[string]string // email → access token
}

// Akun contoh dari migrasi 000006 dan izin cold storage yang dibawa perannya
// (rolePermissions di modul auth).
const (
	userLihatSaja   = "manajer.risiko@garda-inflasi.id"       // MANAJER_RISIKO: coldstorage.lihat
	userKelola      = "manajer.mutu@garda-inflasi.id"         // MANAJER_MUTU: lihat + coldstorage.kelola
	userLogistik    = "manajer.program@garda-inflasi.id"      // MANAJER_PROGRAM: lihat + logistik.kelola
	userAgregator   = "agregator.data@garda-inflasi.id"       // OPS_AGREGATOR: lihat + kelola + logistik
	userTanpaIzin   = "mahasiswa.1@garda-inflasi.id"          // OPS_MAHASISWA: tidak ada izin cold storage
	userKoordinator = "koordinator.lapangan@garda-inflasi.id" // OPS_KOORDINATOR: tidak ada izin cold storage
)

// newSuite menyiapkan basis data (migrasi + seed), router, dan token login.
func newSuite(t *testing.T, db *gorm.DB) *suite {
	t.Helper()
	migrateUp(t, db, nil)
	_, err := seeder.Seed(t.Context(), db, time.Now())
	require.NoError(t, err, "seed cold storage")

	tokenMgr := token.NewManager(jwtSecret)
	log := logger.NewDefault()

	router := gin.New()
	router.Use(gin.Recovery())
	v1 := router.Group("/v1")
	authRoutes.Register(v1, &authRoutes.Dependencies{DB: db, Token: tokenMgr, Logger: log})
	coldRoutes.Register(v1, &coldRoutes.Dependencies{DB: db, Logger: log, Token: tokenMgr})

	s := &suite{db: db, router: router, tokens: map[string]string{}}
	for _, email := range []string{userLihatSaja, userKelola, userLogistik, userAgregator, userTanpaIzin, userKoordinator} {
		s.tokens[email] = s.login(t, email)
	}
	return s
}

var (
	sharedOnce sync.Once
	shared     *suite
)

// sharedSuite membuat satu basis data (migrasi + seed + login) untuk seluruh
// uji yang hanya membaca atau yang efek sampingnya tidak mengubah stok.
// Koneksinya hidup sampai container dibuang di akhir TestMain.
func sharedSuite(t *testing.T) *suite {
	t.Helper()
	sharedOnce.Do(func() {
		shared = newSuite(t, createDatabase(t, "garda_shared"))
	})
	require.NotNil(t, shared, "suite bersama gagal dibuat pada uji pertama yang memakainya")
	return shared
}

func (s *suite) call(t *testing.T, method, path, email string, body any) *httptest.ResponseRecorder {
	t.Helper()

	var rd *bytes.Reader
	switch b := body.(type) {
	case nil:
		rd = bytes.NewReader(nil)
	case string:
		rd = bytes.NewReader([]byte(b))
	default:
		raw, err := json.Marshal(b)
		require.NoError(t, err)
		rd = bytes.NewReader(raw)
	}

	req := httptest.NewRequest(method, path, rd)
	req.Header.Set("Content-Type", "application/json")
	if email != "" {
		req.Header.Set("Authorization", "Bearer "+s.tokens[email])
	}
	w := httptest.NewRecorder()
	s.router.ServeHTTP(w, req)
	return w
}

func (s *suite) login(t *testing.T, email string) string {
	t.Helper()
	w := s.call(t, http.MethodPost, "/v1/auth/login", "", map[string]any{"email": email, "password": seedPassword})
	require.Equal(t, http.StatusOK, w.Code, "login %s: %s", email, w.Body.String())
	data := dataOf(t, w)
	tok, _ := data["access_token"].(string)
	require.NotEmpty(t, tok)
	return tok
}

// ── Pembaca respons ───────────────────────────────────────────────────

func bodyOf(t *testing.T, w *httptest.ResponseRecorder) map[string]any {
	t.Helper()
	var m map[string]any
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &m), w.Body.String())
	return m
}

func dataOf(t *testing.T, w *httptest.ResponseRecorder) map[string]any {
	t.Helper()
	data, ok := bodyOf(t, w)["data"].(map[string]any)
	require.True(t, ok, "data bukan objek: %s", w.Body.String())
	return data
}

func list(t *testing.T, v any) []map[string]any {
	t.Helper()
	arr, ok := v.([]any)
	require.True(t, ok, "bukan array: %T", v)
	out := make([]map[string]any, len(arr))
	for i, e := range arr {
		out[i], ok = e.(map[string]any)
		require.True(t, ok, "elemen %d bukan objek", i)
	}
	return out
}

func ids(rows []map[string]any) []string {
	out := make([]string, len(rows))
	for i, r := range rows {
		out[i], _ = r["id"].(string)
	}
	return out
}

func getCold(t *testing.T, s *suite, email string, params url.Values) (map[string]any, []map[string]any) {
	t.Helper()
	path := "/v1/cold-storage"
	if len(params) > 0 {
		path += "?" + params.Encode()
	}
	w := s.call(t, http.MethodGet, path, email, nil)
	require.Equal(t, http.StatusOK, w.Code, w.Body.String())
	data := dataOf(t, w)
	return data, list(t, data["stok"])
}

// ══════════════════════════════════════════════════════════════════════
// Migrasi 000007
// ══════════════════════════════════════════════════════════════════════

func TestMigration000007_BackfillsConstraintsAndReverses(t *testing.T) {
	db := newDatabase(t, "mig_000007")
	migrateUp(t, db, func(v int) bool { return v <= 6 })

	// Data yang sudah ada sebelum 000007 (tanpa kolom kedaluwarsa_pada).
	execSQL(t, db, `
		INSERT INTO cold_storage (id, nama, kabupaten_kode, lat, lng, kapasitas_ton, suhu_target_c, status)
		VALUES ('CS-LAMA', 'Fasilitas Lama', '1171', 5.5, 95.3, 100, 4, 'OPERASIONAL');
		INSERT INTO stok_cold_storage (id, cold_storage_id, komoditas_kode, volume_ton, masuk_pada, pemilik, risiko_food_loss)
		VALUES ('STK-LAMA', 'CS-LAMA', 'cabai_merah_keriting', 10, '2026-10-01T00:00:00Z', 'Pemilik Lama', 0.5);
	`, "data pra-000007")

	m := findMigration(t, 7)
	execSQL(t, db, m.up, "000007 up")

	t.Run("lot lama diisi dari masa simpan komoditas", func(t *testing.T) {
		var days float64
		require.NoError(t, db.Raw(`
			SELECT EXTRACT(EPOCH FROM (kedaluwarsa_pada - masuk_pada)) / 86400
			  FROM stok_cold_storage WHERE id = 'STK-LAMA'`).Scan(&days).Error)
		assert.Equal(t, 7.0, days, "cabai_merah_keriting memiliki shelf_life_hari = 7 pada master seed")
	})

	t.Run("bentuk kolom", func(t *testing.T) {
		nullable := func(table, column string) string {
			var v string
			require.NoError(t, db.Raw(`SELECT is_nullable FROM information_schema.columns WHERE table_name = ? AND column_name = ?`, table, column).Scan(&v).Error)
			return v
		}
		assert.Equal(t, "NO", nullable("stok_cold_storage", "kedaluwarsa_pada"))
		assert.Equal(t, "YES", nullable("cold_storage", "suhu_aktual_c"), "suhu aktual boleh kosong (fasilitas rencana)")
	})

	t.Run("indeks terbentuk", func(t *testing.T) {
		var names []string
		require.NoError(t, db.Raw(`SELECT indexname FROM pg_indexes WHERE tablename = 'stok_cold_storage'`).Scan(&names).Error)
		assert.Contains(t, names, "idx_stok_cold_storage_fasilitas")
		assert.Contains(t, names, "idx_stok_cold_storage_kedaluwarsa")
	})

	t.Run("CHECK menolak risiko dan volume di luar rentang", func(t *testing.T) {
		insert := func(id string, volume, risiko float64) error {
			return db.Exec(`
				INSERT INTO stok_cold_storage (id, cold_storage_id, komoditas_kode, volume_ton, masuk_pada, kedaluwarsa_pada, pemilik, risiko_food_loss)
				VALUES (?, 'CS-LAMA', 'cabai_merah_keriting', ?, now(), now() + interval '3 day', 'x', ?)`, id, volume, risiko).Error
		}
		assert.Error(t, insert("S-RISIKO-1", 5, 1.5), "risiko > 1")
		assert.Error(t, insert("S-RISIKO-2", 5, -0.1), "risiko < 0")
		assert.Error(t, insert("S-VOL", -1, 0.5), "volume negatif")
		assert.NoError(t, insert("S-OK-0", 0, 0), "batas bawah sah")
		assert.NoError(t, insert("S-OK-1", 5, 1), "batas atas sah")
	})

	t.Run("Up dapat diulang tanpa galat", func(t *testing.T) {
		execSQL(t, db, m.up, "000007 up (ulang)")
	})

	t.Run("Down mengembalikan skema dan menyisakan data", func(t *testing.T) {
		execSQL(t, db, m.down, "000007 down")

		var cols []string
		require.NoError(t, db.Raw(`SELECT column_name FROM information_schema.columns
			WHERE (table_name = 'stok_cold_storage' AND column_name = 'kedaluwarsa_pada')
			   OR (table_name = 'cold_storage' AND column_name = 'suhu_aktual_c')`).Scan(&cols).Error)
		assert.Empty(t, cols)

		var n int64
		require.NoError(t, db.Raw(`SELECT COUNT(*) FROM stok_cold_storage WHERE id = 'STK-LAMA'`).Scan(&n).Error)
		assert.Equal(t, int64(1), n, "data lot tidak boleh hilang saat Down")

		// Up setelah Down harus kembali berhasil dan mengisi ulang kedaluwarsa_pada.
		execSQL(t, db, m.up, "000007 up (setelah down)")
		var days float64
		require.NoError(t, db.Raw(`SELECT EXTRACT(EPOCH FROM (kedaluwarsa_pada - masuk_pada)) / 86400 FROM stok_cold_storage WHERE id = 'STK-LAMA'`).Scan(&days).Error)
		assert.Equal(t, 7.0, days)
	})
}

// ══════════════════════════════════════════════════════════════════════
// Seeder
// ══════════════════════════════════════════════════════════════════════

func TestSeeder_IsIdempotentAndRefreshesDates(t *testing.T) {
	db := newDatabase(t, "seed_idem")
	migrateUp(t, db, nil)

	now := time.Date(2026, 10, 16, 9, 0, 0, 0, time.UTC)
	first, err := seeder.Seed(t.Context(), db, now)
	require.NoError(t, err)
	assert.Equal(t, seeder.Summary{Komoditas: 2, Fasilitas: 5, Stok: 23}, first)

	count := func(table string) int64 {
		var n int64
		require.NoError(t, db.Table(table).Count(&n).Error)
		return n
	}
	assert.Equal(t, int64(5), count("cold_storage"))
	assert.Equal(t, int64(23), count("stok_cold_storage"))

	// Menjalankan ulang pada waktu lain: tidak menggandakan baris, komoditas
	// sudah ada, tanggal bergeser mengikuti now.
	later := now.Add(10 * 24 * time.Hour)
	second, err := seeder.Seed(t.Context(), db, later)
	require.NoError(t, err)
	assert.Equal(t, seeder.Summary{Komoditas: 0, Fasilitas: 5, Stok: 23}, second)
	assert.Equal(t, int64(5), count("cold_storage"))
	assert.Equal(t, int64(23), count("stok_cold_storage"))

	var masuk time.Time
	require.NoError(t, db.Raw(`SELECT masuk_pada FROM stok_cold_storage WHERE id = 'STK-CS01-01'`).Scan(&masuk).Error)
	assert.True(t, masuk.Equal(later.Add(-16*24*time.Hour)), "lot STK-CS01-01 berumur 16 hari pada waktu seed terakhir, dapat %s", masuk)

	t.Run("komoditas pelengkap tersedia", func(t *testing.T) {
		var kode []string
		require.NoError(t, db.Raw(`SELECT kode FROM komoditas WHERE kode IN ('tomat', 'kentang') ORDER BY kode`).Scan(&kode).Error)
		assert.Equal(t, []string{"kentang", "tomat"}, kode)
	})

	t.Run("wilayah dirujuk menurut nama master", func(t *testing.T) {
		var rows []struct{ ID, Nama string }
		require.NoError(t, db.Raw(`SELECT c.id, kb.nama FROM cold_storage c JOIN kabupaten kb ON kb.kode = c.kabupaten_kode ORDER BY c.id`).Scan(&rows).Error)
		got := map[string]string{}
		for _, r := range rows {
			got[r.ID] = r.Nama
		}
		assert.Equal(t, map[string]string{
			"CS-01": "KABUPATEN ACEH BESAR",
			"CS-02": "KOTA LHOKSEUMAWE",
			"CS-03": "KABUPATEN ACEH TENGAH",
			"CS-04": "KABUPATEN ACEH BARAT",
			"CS-05": "KOTA LANGSA",
		}, got)
	})

	t.Run("fasilitas rencana tanpa telemetri", func(t *testing.T) {
		var n int64
		require.NoError(t, db.Raw(`SELECT COUNT(*) FROM cold_storage WHERE status = 'RENCANA' AND suhu_aktual_c IS NULL`).Scan(&n).Error)
		assert.Equal(t, int64(1), n)
	})
}

func TestSeeder_FailsAtomicallyWhenMasterDataMissing(t *testing.T) {
	db := newDatabase(t, "seed_missing_master")
	// Skema tanpa 000006: master kabupaten dan komoditas kosong.
	migrateUp(t, db, func(v int) bool { return v <= 5 || v == 7 })

	_, err := seeder.Seed(t.Context(), db, time.Now())

	require.Error(t, err)
	assert.Contains(t, err.Error(), "tidak ditemukan")
	assert.Contains(t, err.Error(), "migrasi")

	var n int64
	for _, table := range []string{"komoditas", "cold_storage", "stok_cold_storage"} {
		require.NoError(t, db.Table(table).Count(&n).Error)
		assert.Zero(t, n, "%s harus kosong: transaksi seed di-rollback", table)
	}
}

// ══════════════════════════════════════════════════════════════════════
// GET /v1/cold-storage
// ══════════════════════════════════════════════════════════════════════

func TestGetColdStorage_Authorization(t *testing.T) {
	s := sharedSuite(t)

	t.Run("tanpa token", func(t *testing.T) {
		w := s.call(t, http.MethodGet, "/v1/cold-storage", "", nil)
		assert.Equal(t, http.StatusUnauthorized, w.Code)
	})

	t.Run("token tidak valid", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/v1/cold-storage", nil)
		req.Header.Set("Authorization", "Bearer bukan-token")
		w := httptest.NewRecorder()
		s.router.ServeHTTP(w, req)
		assert.Equal(t, http.StatusUnauthorized, w.Code)
	})

	for name, email := range map[string]string{"mahasiswa": userTanpaIzin, "koordinator lapangan": userKoordinator} {
		t.Run("peran tanpa izin coldstorage.lihat: "+name, func(t *testing.T) {
			w := s.call(t, http.MethodGet, "/v1/cold-storage", email, nil)
			assert.Equal(t, http.StatusForbidden, w.Code)
		})
	}

	for name, email := range map[string]string{"lihat saja": userLihatSaja, "kelola": userKelola, "logistik": userLogistik, "agregator": userAgregator} {
		t.Run("peran dengan izin: "+name, func(t *testing.T) {
			w := s.call(t, http.MethodGet, "/v1/cold-storage", email, nil)
			assert.Equal(t, http.StatusOK, w.Code, w.Body.String())
		})
	}
}

func TestGetColdStorage_FasilitasCards(t *testing.T) {
	s := sharedSuite(t)

	data, _ := getCold(t, s, userLihatSaja, nil)
	fasilitas := list(t, data["fasilitas"])

	require.Equal(t, []string{"CS-01", "CS-02", "CS-03", "CS-04", "CS-05"}, ids(fasilitas),
		"operasional dulu, lalu perawatan, lalu rencana")

	type card struct {
		nama, wilayah, status string
		kapasitas, terisi     float64
		utilisasi             float64
		target                float64
		aktual                any
	}
	want := map[string]card{
		"CS-01": {"Cold Storage Lambaro", "KABUPATEN ACEH BESAR", "OPERASIONAL", 400, 347, 86.8, 4, 3.5},
		"CS-02": {"Cold Storage Perikanan Lhokseumawe", "KOTA LHOKSEUMAWE", "OPERASIONAL", 300, 240, 80, -20, -20.1},
		"CS-03": {"Sentra Hortikultura Takengon", "KABUPATEN ACEH TENGAH", "OPERASIONAL", 250, 193, 77.2, 6, 6.0},
		"CS-04": {"Gudang Dingin Meulaboh", "KABUPATEN ACEH BARAT", "PERAWATAN", 150, 33, 22, 4, 4.8},
		"CS-05": {"Gudang Dingin Langsa", "KOTA LANGSA", "RENCANA", 120, 0, 0, 4, nil},
	}
	for _, f := range fasilitas {
		id := f["id"].(string)
		w := want[id]
		assert.Equal(t, w.nama, f["nama"], id)
		assert.Equal(t, w.wilayah, f["kabupaten_nama"], id)
		assert.Equal(t, w.status, f["status"], id)
		assert.Equal(t, w.kapasitas, f["kapasitas_ton"], id)
		assert.Equal(t, w.terisi, f["terisi_ton"], id)
		assert.Equal(t, w.utilisasi, f["utilisasi_persen"], id)
		assert.Equal(t, w.target, f["suhu_target_c"], id)
		assert.Equal(t, w.aktual, f["suhu_aktual_c"], id)
	}
}

func TestGetColdStorage_DefaultListIsShortestShelfLifeFirst(t *testing.T) {
	s := sharedSuite(t)

	data, stok := getCold(t, s, userLihatSaja, nil)

	assert.Len(t, stok, 10, "halaman pertama berisi 10 lot")
	assert.Equal(t, []string{
		"STK-CS01-01", "STK-CS03-01", "STK-CS03-02", "STK-CS03-03", "STK-CS03-04",
	}, ids(stok)[:5], "lima baris pertama sama dengan desain halaman")

	wantSisa := []float64{5, 6, 6, 7, 10}
	for i, w := range wantSisa {
		assert.Equal(t, w, stok[i]["sisa_masa_simpan_hari"], "baris %d", i)
	}

	first := stok[0]
	assert.Equal(t, "Cabai Merah Keriting", first["komoditas_nama"])
	assert.Equal(t, "cabai_merah_keriting", first["komoditas_kode"])
	assert.Equal(t, "Cold Storage Lambaro", first["cold_storage_nama"])
	assert.Equal(t, 34.9, first["volume_ton"])
	assert.Equal(t, 0.68, first["risiko_food_loss"])
	assert.Equal(t, "KUD Pidie Jaya", first["pemilik"])

	meta := data["meta"].(map[string]any)
	assert.Equal(t, 1.0, meta["page"])
	assert.Equal(t, 10.0, meta["limit"])
	assert.Equal(t, 23.0, meta["total"])
	assert.Equal(t, 3.0, meta["total_pages"])
	assert.Equal(t, 813.0, meta["volume_hasil_ton"])

	ringkasan := data["ringkasan"].(map[string]any)
	assert.Equal(t, 23.0, ringkasan["total_lot"])
	assert.Equal(t, 813.0, ringkasan["total_volume_ton"])
	assert.Equal(t, 172.7, ringkasan["risiko_tinggi_ton"], "subjudul tabel: 172,7 ton berisiko tinggi")
	assert.Equal(t, 4.0, ringkasan["risiko_tinggi_lot"])
	assert.Equal(t, 0.6, ringkasan["ambang_risiko_tinggi"])
}

func TestGetColdStorage_PaginationCoversEveryLotExactlyOnce(t *testing.T) {
	s := sharedSuite(t)

	seen := map[string]int{}
	sizes := []int{}
	for page := 1; page <= 3; page++ {
		_, stok := getCold(t, s, userLihatSaja, url.Values{"page": {fmt.Sprint(page)}})
		sizes = append(sizes, len(stok))
		for _, id := range ids(stok) {
			seen[id]++
		}
	}

	assert.Equal(t, []int{10, 10, 3}, sizes)
	assert.Len(t, seen, 23)
	for id, n := range seen {
		assert.Equal(t, 1, n, "lot %s muncul %d kali lintas halaman", id, n)
	}

	t.Run("halaman di luar jangkauan kosong, bukan galat", func(t *testing.T) {
		data, stok := getCold(t, s, userLihatSaja, url.Values{"page": {"99"}})
		assert.Empty(t, stok)
		assert.Equal(t, 23.0, data["meta"].(map[string]any)["total"])
		assert.JSONEq(t, `[]`, mustJSON(t, data["stok"]), "array kosong, bukan null")
	})

	t.Run("limit kustom", func(t *testing.T) {
		data, stok := getCold(t, s, userLihatSaja, url.Values{"limit": {"25"}})
		assert.Len(t, stok, 23)
		assert.Equal(t, 1.0, data["meta"].(map[string]any)["total_pages"])
	})
}

func TestGetColdStorage_SortOrdersEveryColumn(t *testing.T) {
	s := sharedSuite(t)

	parseTime := func(v any) time.Time {
		ts, err := time.Parse(time.RFC3339Nano, v.(string))
		require.NoError(t, err)
		return ts
	}
	lower := func(field string) func(map[string]any) any {
		return func(r map[string]any) any { return strings.ToLower(r[field].(string)) }
	}
	num := func(field string) func(map[string]any) any {
		return func(r map[string]any) any { return r[field].(float64) }
	}
	byTime := func(field string) func(map[string]any) any {
		return func(r map[string]any) any { return parseTime(r[field]).UnixNano() }
	}

	// less membandingkan dua kunci bertipe sama.
	less := func(a, b any) bool {
		switch x := a.(type) {
		case float64:
			return x < b.(float64)
		case int64:
			return x < b.(int64)
		case string:
			return x < b.(string)
		}
		panic("tipe kunci tak didukung")
	}

	cases := []struct {
		sort string
		key  func(map[string]any) any
	}{
		{"sisa_masa_simpan", byTime("kedaluwarsa_pada")},
		{"komoditas", lower("komoditas_nama")},
		{"fasilitas", lower("cold_storage_nama")},
		{"volume", num("volume_ton")},
		{"masuk", byTime("masuk_pada")},
		{"risiko", num("risiko_food_loss")},
		{"pemilik", lower("pemilik")},
	}

	for _, tc := range cases {
		for _, order := range []string{"asc", "desc"} {
			t.Run(tc.sort+"/"+order, func(t *testing.T) {
				_, stok := getCold(t, s, userLihatSaja, url.Values{"sort": {tc.sort}, "order": {order}, "limit": {"100"}})
				require.Len(t, stok, 23)

				for i := 1; i < len(stok); i++ {
					prev, cur := tc.key(stok[i-1]), tc.key(stok[i])
					switch {
					case less(prev, cur):
						assert.Equal(t, "asc", order, "urutan naik tetapi diminta %s pada baris %d", order, i)
					case less(cur, prev):
						assert.Equal(t, "desc", order, "urutan turun tetapi diminta %s pada baris %d", order, i)
					default:
						assert.Less(t, stok[i-1]["id"].(string), stok[i]["id"].(string), "kunci sama diputus id naik (baris %d)", i)
					}
				}
			})
		}
	}

	t.Run("volume terbesar dan risiko tertinggi", func(t *testing.T) {
		_, byVolume := getCold(t, s, userLihatSaja, url.Values{"sort": {"volume"}, "order": {"desc"}, "limit": {"1"}})
		assert.Equal(t, "STK-CS02-01", ids(byVolume)[0])

		_, byRisk := getCold(t, s, userLihatSaja, url.Values{"sort": {"risiko"}, "order": {"desc"}, "limit": {"1"}})
		assert.Equal(t, "STK-CS02-02", ids(byRisk)[0])
	})
}

func TestGetColdStorage_FilterAndSearch(t *testing.T) {
	s := sharedSuite(t)

	t.Run("filter fasilitas", func(t *testing.T) {
		data, stok := getCold(t, s, userLihatSaja, url.Values{"cold_storage_id": {"CS-03"}, "limit": {"100"}})

		assert.Len(t, stok, 7)
		for _, row := range stok {
			assert.Equal(t, "CS-03", row["cold_storage_id"])
		}
		meta := data["meta"].(map[string]any)
		assert.Equal(t, 7.0, meta["total"])
		assert.Equal(t, 193.0, meta["volume_hasil_ton"], "total volume mengikuti filter")

		ringkasan := data["ringkasan"].(map[string]any)
		assert.Equal(t, 23.0, ringkasan["total_lot"], "ringkasan tidak terpengaruh filter")
		assert.Equal(t, 172.7, ringkasan["risiko_tinggi_ton"])
		assert.Len(t, list(t, data["fasilitas"]), 5, "kartu fasilitas selalu lengkap")
	})

	t.Run("fasilitas tanpa stok", func(t *testing.T) {
		data, stok := getCold(t, s, userLihatSaja, url.Values{"cold_storage_id": {"CS-05"}})
		assert.Empty(t, stok)
		assert.Equal(t, 0.0, data["meta"].(map[string]any)["total"])
		assert.Equal(t, 0.0, data["meta"].(map[string]any)["total_pages"])
	})

	searches := []struct {
		name, q string
		want    int
	}{
		{"nama komoditas", "tomat", 3},
		{"tanpa membedakan huruf besar/kecil", "TOMAT", 3},
		{"sebagian nama komoditas", "cabai", 7},
		{"frasa lebih dari satu kata", "cabai merah", 5},
		{"pemilik", "pidie", 6},
		{"komoditas perikanan", "ikan", 5},
		{"tidak ada yang cocok", "durian", 0},
		{"persen dibaca literal, bukan wildcard", "%", 0},
		{"garis bawah dibaca literal, bukan wildcard", "_", 0},
		{"garis miring terbalik dibaca literal", `\`, 0},
	}
	for _, tc := range searches {
		t.Run("cari "+tc.name, func(t *testing.T) {
			data, stok := getCold(t, s, userLihatSaja, url.Values{"q": {tc.q}, "limit": {"100"}})
			assert.Len(t, stok, tc.want)
			assert.Equal(t, float64(tc.want), data["meta"].(map[string]any)["total"])
		})
	}

	t.Run("cari dan filter fasilitas digabung", func(t *testing.T) {
		_, stok := getCold(t, s, userLihatSaja, url.Values{"q": {"cabai"}, "cold_storage_id": {"CS-03"}})
		assert.Equal(t, []string{"STK-CS03-04"}, ids(stok))
	})

	t.Run("kata kunci dengan tanda kutip tidak merusak query", func(t *testing.T) {
		_, stok := getCold(t, s, userLihatSaja, url.Values{"q": {`'; DROP TABLE stok_cold_storage; --`}})
		assert.Empty(t, stok)

		data, _ := getCold(t, s, userLihatSaja, nil)
		assert.Equal(t, 23.0, data["meta"].(map[string]any)["total"], "tabel stok utuh")
	})
}

func TestGetColdStorage_InvalidParams(t *testing.T) {
	s := sharedSuite(t)

	cases := map[string]url.Values{
		"sort tidak dikenal":   {"sort": {"password; DROP TABLE x"}},
		"order tidak dikenal":  {"order": {"random"}},
		"halaman negatif":      {"page": {"-1"}},
		"limit melebihi batas": {"limit": {"101"}},
		"limit bukan angka":    {"limit": {"banyak"}},
	}
	for name, params := range cases {
		t.Run(name, func(t *testing.T) {
			w := s.call(t, http.MethodGet, "/v1/cold-storage?"+params.Encode(), userLihatSaja, nil)
			assert.Equal(t, http.StatusBadRequest, w.Code, w.Body.String())
		})
	}
}

// ══════════════════════════════════════════════════════════════════════
// POST /v1/cold-storage/bulk
// ══════════════════════════════════════════════════════════════════════

func bulk(t *testing.T, s *suite, email, action string, ids ...string) *httptest.ResponseRecorder {
	t.Helper()
	return s.call(t, http.MethodPost, "/v1/cold-storage/bulk", email, map[string]any{"action": action, "ids": ids})
}

func TestBulk_AuthorizationMatrix(t *testing.T) {
	s := sharedSuite(t)

	cases := []struct {
		name   string
		email  string
		action string
		want   int
	}{
		{"tanpa token", "", "rekomendasi_olahan", http.StatusUnauthorized},
		{"hanya lihat: olahan", userLihatSaja, "rekomendasi_olahan", http.StatusForbidden},
		{"hanya lihat: distribusi", userLihatSaja, "rencanakan_distribusi", http.StatusForbidden},
		{"tanpa izin cold storage", userTanpaIzin, "rekomendasi_olahan", http.StatusForbidden},
		{"coldstorage.kelola: olahan", userKelola, "rekomendasi_olahan", http.StatusOK},
		{"coldstorage.kelola: distribusi ditolak", userKelola, "rencanakan_distribusi", http.StatusForbidden},
		{"logistik.kelola: distribusi", userLogistik, "rencanakan_distribusi", http.StatusOK},
		{"logistik.kelola: olahan ditolak", userLogistik, "rekomendasi_olahan", http.StatusForbidden},
		{"agregator: olahan", userAgregator, "rekomendasi_olahan", http.StatusOK},
		{"agregator: distribusi", userAgregator, "rencanakan_distribusi", http.StatusOK},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			w := bulk(t, s, tc.email, tc.action, "STK-CS01-01")
			assert.Equal(t, tc.want, w.Code, w.Body.String())
		})
	}
}

func TestBulk_ResultAuditAndNoSideEffectsOnStock(t *testing.T) {
	s := sharedSuite(t)

	var before struct {
		N   int64
		Sum float64
	}
	require.NoError(t, s.db.Raw(`SELECT COUNT(*) AS n, SUM(volume_ton) AS sum FROM stok_cold_storage`).Scan(&before).Error)

	w := bulk(t, s, userKelola, "rekomendasi_olahan", "STK-CS01-01", "STK-CS03-01", "STK-TIDAK-ADA", "STK-CS01-01")
	require.Equal(t, http.StatusOK, w.Code, w.Body.String())

	data := dataOf(t, w)
	assert.Equal(t, 2.0, data["berhasil"], "id ganda dihitung sekali")
	assert.Equal(t, 1.0, data["gagal"], "id yang tidak ada dilaporkan gagal")
	assert.Equal(t, 65.9, data["volume_ton"], "34,9 + 31,0 ton")

	t.Run("aksi advisori: stok tidak berubah", func(t *testing.T) {
		var after struct {
			N   int64
			Sum float64
		}
		require.NoError(t, s.db.Raw(`SELECT COUNT(*) AS n, SUM(volume_ton) AS sum FROM stok_cold_storage`).Scan(&after).Error)
		assert.Equal(t, before, after)
	})

	t.Run("tercatat di audit trail", func(t *testing.T) {
		var row struct {
			AktorID   string
			Aksi      string
			Entitas   string
			EntitasID string
			Hasil     string
			Metadata  string
		}
		require.NoError(t, s.db.Raw(`
			SELECT aktor_id, aksi, entitas, entitas_id, hasil, metadata::text AS metadata
			  FROM audit_log
			 WHERE aksi = 'BULK_rekomendasi_olahan' AND aktor_id = 'manajer_mutu'
			 ORDER BY waktu DESC LIMIT 1`).Scan(&row).Error)

		assert.Equal(t, "manajer_mutu", row.AktorID)
		assert.Equal(t, "stok_cold_storage", row.Entitas)
		assert.Equal(t, "2 lot", row.EntitasID)
		assert.Equal(t, "BERHASIL", row.Hasil)

		var meta struct {
			IDs       []string `json:"ids"`
			Gagal     int      `json:"gagal"`
			VolumeTon float64  `json:"volume_ton"`
		}
		require.NoError(t, json.Unmarshal([]byte(row.Metadata), &meta))
		assert.Equal(t, []string{"STK-CS01-01", "STK-CS03-01", "STK-TIDAK-ADA"}, meta.IDs)
		assert.Equal(t, 1, meta.Gagal)
		assert.Equal(t, 65.9, meta.VolumeTon)
	})
}

func TestBulk_AllMissingReportsFailureAndAudit(t *testing.T) {
	s := sharedSuite(t)

	w := bulk(t, s, userLogistik, "rencanakan_distribusi", "STK-X", "STK-Y")
	require.Equal(t, http.StatusOK, w.Code)

	data := dataOf(t, w)
	assert.Equal(t, 0.0, data["berhasil"])
	assert.Equal(t, 2.0, data["gagal"])

	var hasil string
	require.NoError(t, s.db.Raw(`SELECT hasil FROM audit_log WHERE aksi = 'BULK_rencanakan_distribusi' AND entitas_id = '0 lot' ORDER BY waktu DESC LIMIT 1`).Scan(&hasil).Error)
	assert.Equal(t, "GAGAL", hasil)
}

func TestBulk_Validation(t *testing.T) {
	s := sharedSuite(t)

	cases := map[string]any{
		"JSON rusak":         "{bukan json",
		"aksi tidak dikenal": map[string]any{"action": "hapus", "ids": []string{"STK-CS01-01"}},
		"tanpa ids":          map[string]any{"action": "rekomendasi_olahan"},
		"ids kosong":         map[string]any{"action": "rekomendasi_olahan", "ids": []string{}},
		"id hanya spasi":     map[string]any{"action": "rekomendasi_olahan", "ids": []string{"  "}},
	}
	for name, body := range cases {
		t.Run(name, func(t *testing.T) {
			w := s.call(t, http.MethodPost, "/v1/cold-storage/bulk", userKelola, body)
			assert.Equal(t, http.StatusBadRequest, w.Code, w.Body.String())
		})
	}
}

// ══════════════════════════════════════════════════════════════════════
// Kontrak OpenAPI (packages/contracts/openapi.yaml)
// ══════════════════════════════════════════════════════════════════════

func loadSpec(t *testing.T) map[string]any {
	t.Helper()
	raw, err := os.ReadFile(contractPath)
	require.NoError(t, err)
	var spec map[string]any
	require.NoError(t, yaml.Unmarshal(raw, &spec), "openapi.yaml harus YAML yang valid")
	return spec
}

// dig menelusuri peta bersarang menurut kunci.
func dig(t *testing.T, node any, path ...string) any {
	t.Helper()
	for _, key := range path {
		m, ok := node.(map[string]any)
		require.True(t, ok, "tidak dapat menelusuri %q: bukan objek", key)
		node, ok = m[key]
		require.True(t, ok, "kunci %q tidak ada pada spesifikasi", key)
	}
	return node
}

func resolve(t *testing.T, spec map[string]any, schema any) map[string]any {
	t.Helper()
	m, ok := schema.(map[string]any)
	require.True(t, ok, "skema bukan objek: %v", schema)
	if ref, ok := m["$ref"].(string); ok {
		require.True(t, strings.HasPrefix(ref, "#/"), "hanya $ref lokal yang didukung: %s", ref)
		return resolve(t, spec, dig(t, spec, strings.Split(strings.TrimPrefix(ref, "#/"), "/")...))
	}
	return m
}

func responseSchema(t *testing.T, spec map[string]any, path, method, status string) map[string]any {
	t.Helper()
	return resolve(t, spec, dig(t, spec, "paths", path, method, "responses", status, "content", "application/json", "schema"))
}

// conforms memeriksa nilai JSON terhadap skema OpenAPI: setiap properti
// terdokumentasi harus ada, dan tidak boleh ada properti yang tidak terdokumentasi.
func conforms(t *testing.T, spec map[string]any, schema map[string]any, value any, path string) {
	t.Helper()
	schema = resolve(t, spec, schema)

	if value == nil {
		assert.Equal(t, true, schema["nullable"], "%s: null padahal skema tidak nullable", path)
		return
	}

	if enum, ok := schema["enum"].([]any); ok {
		assert.Contains(t, enum, value, "%s: di luar enum", path)
	}

	switch schema["type"] {
	case "object":
		obj, ok := value.(map[string]any)
		require.True(t, ok, "%s: harus objek, dapat %T", path, value)
		props, _ := schema["properties"].(map[string]any)
		for name := range props {
			_, present := obj[name]
			assert.True(t, present, "%s.%s terdokumentasi tetapi tidak ada pada respons", path, name)
		}
		for name, v := range obj {
			prop, documented := props[name]
			if !assert.True(t, documented, "%s.%s ada pada respons tetapi tidak terdokumentasi", path, name) {
				continue
			}
			conforms(t, spec, prop.(map[string]any), v, path+"."+name)
		}
	case "array":
		arr, ok := value.([]any)
		require.True(t, ok, "%s: harus array, dapat %T", path, value)
		items := schema["items"].(map[string]any)
		for i, e := range arr {
			conforms(t, spec, items, e, fmt.Sprintf("%s[%d]", path, i))
		}
	case "string":
		str, ok := value.(string)
		require.True(t, ok, "%s: harus string, dapat %T", path, value)
		if schema["format"] == "date-time" {
			_, err := time.Parse(time.RFC3339Nano, str)
			assert.NoError(t, err, "%s: bukan date-time", path)
		}
	case "integer":
		n, ok := value.(float64)
		require.True(t, ok, "%s: harus integer, dapat %T", path, value)
		assert.Equal(t, float64(int64(n)), n, "%s: harus bilangan bulat", path)
	case "number":
		_, ok := value.(float64)
		assert.True(t, ok, "%s: harus number, dapat %T", path, value)
	case "boolean":
		_, ok := value.(bool)
		assert.True(t, ok, "%s: harus boolean, dapat %T", path, value)
	}
}

func TestContract_GetColdStorageResponseMatchesOpenAPI(t *testing.T) {
	s := sharedSuite(t)
	spec := loadSpec(t)
	schema := responseSchema(t, spec, "/v1/cold-storage", "get", "200")

	cases := map[string]url.Values{
		"bawaan":           nil,
		"terfilter":        {"cold_storage_id": {"CS-03"}, "q": {"tomat"}, "sort": {"volume"}, "order": {"desc"}},
		"hasil kosong":     {"q": {"durian"}},
		"halaman terakhir": {"page": {"3"}},
	}
	for name, params := range cases {
		t.Run(name, func(t *testing.T) {
			path := "/v1/cold-storage"
			if params != nil {
				path += "?" + params.Encode()
			}
			w := s.call(t, http.MethodGet, path, userLihatSaja, nil)
			require.Equal(t, http.StatusOK, w.Code)
			conforms(t, spec, schema, bodyOf(t, w), "$")
		})
	}
}

func TestContract_BulkResponseMatchesOpenAPI(t *testing.T) {
	s := sharedSuite(t)
	spec := loadSpec(t)
	schema := responseSchema(t, spec, "/v1/cold-storage/bulk", "post", "200")

	w := bulk(t, s, userAgregator, "rekomendasi_olahan", "STK-CS01-01")
	require.Equal(t, http.StatusOK, w.Code)
	conforms(t, spec, schema, bodyOf(t, w), "$")
}

func TestContract_DocumentedQueryParamsMatchBehaviour(t *testing.T) {
	s := sharedSuite(t)
	spec := loadSpec(t)

	params := dig(t, spec, "paths", "/v1/cold-storage", "get", "parameters").([]any)
	byName := map[string]map[string]any{}
	for _, p := range params {
		pm := p.(map[string]any)
		byName[pm["name"].(string)] = pm["schema"].(map[string]any)
	}
	require.ElementsMatch(t, []string{"cold_storage_id", "q", "sort", "order", "page", "limit"}, mapKeys(byName),
		"parameter terdokumentasi harus sama dengan yang diterima API")

	t.Run("setiap nilai sort terdokumentasi diterima", func(t *testing.T) {
		for _, v := range byName["sort"]["enum"].([]any) {
			w := s.call(t, http.MethodGet, "/v1/cold-storage?sort="+v.(string), userLihatSaja, nil)
			assert.Equal(t, http.StatusOK, w.Code, "sort=%v", v)
		}
	})

	t.Run("setiap nilai order terdokumentasi diterima", func(t *testing.T) {
		for _, v := range byName["order"]["enum"].([]any) {
			w := s.call(t, http.MethodGet, "/v1/cold-storage?order="+v.(string), userLihatSaja, nil)
			assert.Equal(t, http.StatusOK, w.Code, "order=%v", v)
		}
	})

	t.Run("batas limit terdokumentasi ditegakkan", func(t *testing.T) {
		maxLimit := byName["limit"]["maximum"].(int)
		assert.Equal(t, http.StatusOK, s.call(t, http.MethodGet, fmt.Sprintf("/v1/cold-storage?limit=%d", maxLimit), userLihatSaja, nil).Code)
		assert.Equal(t, http.StatusBadRequest, s.call(t, http.MethodGet, fmt.Sprintf("/v1/cold-storage?limit=%d", maxLimit+1), userLihatSaja, nil).Code)
	})

	t.Run("batas panjang q ditegakkan", func(t *testing.T) {
		maxLen := byName["q"]["maxLength"].(int)
		assert.Equal(t, http.StatusOK, s.call(t, http.MethodGet, "/v1/cold-storage?q="+strings.Repeat("a", maxLen), userLihatSaja, nil).Code)
		assert.Equal(t, http.StatusBadRequest, s.call(t, http.MethodGet, "/v1/cold-storage?q="+strings.Repeat("a", maxLen+1), userLihatSaja, nil).Code)
	})
}

func TestContract_BulkDocumentedActionsAreAccepted(t *testing.T) {
	s := sharedSuite(t)
	spec := loadSpec(t)

	reqSchema := resolve(t, spec, dig(t, spec, "paths", "/v1/cold-storage/bulk", "post", "requestBody", "content", "application/json", "schema"))
	enum := dig(t, reqSchema, "properties", "action", "enum").([]any)
	require.NotEmpty(t, enum)

	for _, a := range enum {
		w := bulk(t, s, userAgregator, a.(string), "STK-CS01-01")
		assert.Equal(t, http.StatusOK, w.Code, "aksi terdokumentasi %v harus diterima", a)
	}
}

func mapKeys[V any](m map[string]V) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}

func mustJSON(t *testing.T, v any) string {
	t.Helper()
	raw, err := json.Marshal(v)
	require.NoError(t, err)
	return string(raw)
}
