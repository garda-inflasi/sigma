// Package integration menguji API tugas lapangan terhadap PostgreSQL sungguhan
// dengan seed master.reference, akun demo, dan tugas demo.
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
	"time"

	authRoutes "github.com/garda-inflasi/api/internal/auth/routes"
	lapanganRoutes "github.com/garda-inflasi/api/internal/lapangan/routes"
	"github.com/garda-inflasi/api/internal/platform/pgtest"
	"github.com/garda-inflasi/api/pkg/logger"
	"github.com/garda-inflasi/api/pkg/seed"
	"github.com/garda-inflasi/api/pkg/token"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

const (
	jwtSecret = "lapangan-integration-secret-key-32chars!"

	// OPS_MAHASISWA: tugas.lihat_sendiri; OPS_KOORDINATOR: juga tugas.kelola; STRATEGIS_BI: tanpa keduanya.
	emailMahasiswa1  = "mahasiswa.1@gardainflasi.id"
	emailMahasiswa2  = "mahasiswa.2@gardainflasi.id"
	emailKoordinator = "koordinator.lapangan@gardainflasi.id"
	emailTanpaIzin   = "strategis.bi@gardainflasi.id"
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
		db := pg.CreateDatabase(t, "lapangan_shared")
		pgtest.MigrateUp(t, db)
		// Master dan akun (beserta matriks izin), tugas lapangan demo, dan histori entri harga demo; modul lain tidak
		// ikut disemai (SeedDemo penuh juga mengisi pedagang, EWS, prediksi, dan seterusnya yang tidak dipakai tes ini).
		require.NoError(t, seed.SeedFiles(t.Context(), db,
			"reference/01_master.sql", "reference/02_kecamatan.sql", "reference/03_auth_rbac.sql", "demo/01_auth_users.sql",
			"demo/08_lapangan_tugas.sql", "demo/03_harga_harian.sql"))

		tokens, log := token.NewManager(jwtSecret), logger.NewDefault()
		router := gin.New()
		v1 := router.Group("/v1")
		authRoutes.Register(v1, &authRoutes.Dependencies{DB: db, Token: tokens, Logger: log})
		lapanganRoutes.Register(v1, &lapanganRoutes.Dependencies{DB: db, Token: tokens, Logger: log})
		shared = &suite{db: db, router: router, tokens: map[string]string{}}
	})
	require.NotNil(t, shared, "suite bersama gagal dibuat")
	return shared
}

func (s *suite) call(t *testing.T, method, path, email string, body any) *httptest.ResponseRecorder {
	t.Helper()
	return s.callWith(t, method, path, email, nil, body)
}

func (s *suite) callWith(t *testing.T, method, path, email string, headers map[string]string, body any) *httptest.ResponseRecorder {
	t.Helper()
	var payload bytes.Buffer
	if body != nil {
		require.NoError(t, json.NewEncoder(&payload).Encode(body))
	}
	req := httptest.NewRequest(method, path, &payload)
	req.Header.Set("Content-Type", "application/json")
	for k, v := range headers {
		req.Header.Set(k, v)
	}
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

type tugas struct {
	ID        string   `json:"id"`
	Kode      *string  `json:"kode"`
	Judul     string   `json:"judul"`
	PasarID   string   `json:"pasar_id"`
	PasarNama string   `json:"pasar_nama"`
	Komoditas []string `json:"komoditas"`
	Tenggat   string   `json:"tenggat"`
	Status    string   `json:"status"`
	Target    int      `json:"target"`
	Terisi    int      `json:"terisi"`
	Tipe      string   `json:"tipe"`
}

// tugasDetail adalah GET /v1/lapangan/tugas/{id}: tugas ditambah rincian per komoditas.
type tugasDetail struct {
	tugas
	Rincian []struct {
		Kode   string `json:"kode"`
		Nama   string `json:"nama"`
		Terisi bool   `json:"terisi"`
	} `json:"rincian_komoditas"`
}

func data[T any](t *testing.T, w *httptest.ResponseRecorder) T {
	t.Helper()
	var res struct {
		Data T `json:"data"`
	}
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &res), w.Body.String())
	return res.Data
}

type hasilBulk struct {
	Berhasil int `json:"berhasil"`
	Gagal    int `json:"gagal"`
}

func TestListSaya_ReturnsOwnTasksWithComputedProgress(t *testing.T) {
	s := sharedSuite(t)

	w := s.call(t, http.MethodGet, "/v1/lapangan/tugas/saya", emailMahasiswa1, nil)

	require.Equal(t, http.StatusOK, w.Code, w.Body.String())
	items := data[[]tugas](t, w)
	require.Len(t, items, 5)

	perStatus := map[string]int{}
	for i, item := range items {
		perStatus[item.Status]++
		assert.Equal(t, "PSR-1171-01", item.PasarID)
		assert.Equal(t, "Pasar Peunayong", item.PasarNama)
		assert.NotEmpty(t, item.Komoditas)
		assert.Positive(t, item.Target)
		assert.LessOrEqual(t, item.Terisi, item.Target)
		if i > 0 && items[i-1].Status != "SELESAI" && item.Status != "SELESAI" {
			assert.LessOrEqual(t, items[i-1].Tenggat, item.Tenggat, "tugas aktif terurut menurut tenggat")
		}
	}
	assert.Equal(t, map[string]int{"BERJALAN": 1, "BELUM": 2, "TERLAMBAT": 1, "SELESAI": 1}, perStatus)
	assert.Equal(t, "SELESAI", items[len(items)-1].Status, "tugas selesai di urutan akhir")
}

func TestListSaya_EveryFieldAccountHasAnActiveTask(t *testing.T) {
	s := sharedSuite(t)

	for _, email := range []string{emailKoordinator, "agregator.data@gardainflasi.id", emailMahasiswa1, emailMahasiswa2, "mahasiswa.10@gardainflasi.id"} {
		items := data[[]tugas](t, s.call(t, http.MethodGet, "/v1/lapangan/tugas/saya", email, nil))
		var aktif int
		for _, item := range items {
			if item.Status != "SELESAI" {
				aktif++
			}
		}
		assert.Positive(t, aktif, email)
	}
}

func TestListSaya_Authorization(t *testing.T) {
	s := sharedSuite(t)

	assert.Equal(t, http.StatusForbidden, s.call(t, http.MethodGet, "/v1/lapangan/tugas/saya", emailTanpaIzin, nil).Code)
	assert.Equal(t, http.StatusUnauthorized, s.call(t, http.MethodGet, "/v1/lapangan/tugas/saya", "", nil).Code)
}

func TestGet_OwnerAndManagerOnly(t *testing.T) {
	s := sharedSuite(t)
	path := "/v1/lapangan/tugas/TGS-mahasiswa_1-1"

	w := s.call(t, http.MethodGet, path, emailMahasiswa1, nil)
	require.Equal(t, http.StatusOK, w.Code, w.Body.String())
	detail := data[tugasDetail](t, w)
	// Rujukan lewat kode lama (alias) menjawab tugas yang sama dengan ID BIGINT-nya.
	require.NotNil(t, detail.Kode)
	assert.Equal(t, "TGS-mahasiswa_1-1", *detail.Kode)
	_, err := strconv.ParseInt(detail.ID, 10, 64)
	require.NoError(t, err, "id string angka: %q", detail.ID)
	lewatID := data[tugasDetail](t, s.call(t, http.MethodGet, "/v1/lapangan/tugas/"+detail.ID, emailMahasiswa1, nil))
	assert.Equal(t, detail, lewatID)
	require.Len(t, detail.Rincian, len(detail.Komoditas), "rincian mencakup semua komoditas tugas")
	for i, kode := range detail.Komoditas {
		assert.Equal(t, kode, detail.Rincian[i].Kode, "urutan rincian sama dengan tugas")
		assert.NotEmpty(t, detail.Rincian[i].Nama)
	}

	assert.Equal(t, http.StatusNotFound, s.call(t, http.MethodGet, path, emailMahasiswa2, nil).Code, "tugas petugas lain tidak terlihat")
	assert.Equal(t, http.StatusOK, s.call(t, http.MethodGet, path, emailKoordinator, nil).Code, "tugas.kelola melihat tugas petugas lain")
	assert.Equal(t, http.StatusNotFound, s.call(t, http.MethodGet, "/v1/lapangan/tugas/tidak-ada", emailMahasiswa1, nil).Code)
	assert.Equal(t, http.StatusOK, s.call(t, http.MethodGet, "/v1/lapangan/tugas/"+detail.ID, emailKoordinator, nil).Code)
	assert.Equal(t, http.StatusNotFound, s.call(t, http.MethodGet, "/v1/lapangan/tugas/"+detail.ID, emailMahasiswa2, nil).Code, "ID BIGINT tidak membuka tugas petugas lain")
}

func TestBulk_TandaiSelesai(t *testing.T) {
	s := sharedSuite(t)
	bulk := func(email, aksi string, ids ...string) *httptest.ResponseRecorder {
		return s.call(t, http.MethodPost, "/v1/lapangan/tugas/bulk", email, map[string]any{"action": aksi, "ids": ids})
	}

	// Target belum terpenuhi (0 dari 2): tidak ditutup.
	assert.Equal(t, hasilBulk{Gagal: 1}, data[hasilBulk](t, bulk(emailMahasiswa1, "tandai_selesai", "TGS-mahasiswa_1-2")))

	require.NoError(t, s.db.Exec("UPDATE tugas_lapangan SET jumlah_selesai = target WHERE kode = 'TGS-mahasiswa_1-2'").Error)

	// Petugas lain tidak boleh menutup tugas ini.
	assert.Equal(t, hasilBulk{Gagal: 1}, data[hasilBulk](t, bulk(emailMahasiswa2, "tandai_selesai", "TGS-mahasiswa_1-2")))

	w := bulk(emailMahasiswa1, "tandai_selesai", "TGS-mahasiswa_1-2")
	require.Equal(t, http.StatusOK, w.Code, w.Body.String())
	assert.Equal(t, hasilBulk{Berhasil: 1}, data[hasilBulk](t, w))

	got := data[tugas](t, s.call(t, http.MethodGet, "/v1/lapangan/tugas/TGS-mahasiswa_1-2", emailMahasiswa1, nil))
	assert.Equal(t, "SELESAI", got.Status)

	var selesaiPada *time.Time
	require.NoError(t, s.db.Raw("SELECT selesai_pada FROM tugas_lapangan WHERE kode = 'TGS-mahasiswa_1-2'").Scan(&selesaiPada).Error)
	assert.NotNil(t, selesaiPada)

	var audit int64
	require.NoError(t, s.db.Raw("SELECT count(*) FROM audit_log WHERE aksi = 'TUGAS_TANDAI_SELESAI' AND aktor_id = garda_uuid_stabil('pengguna', 'mahasiswa_1')").Scan(&audit).Error)
	assert.EqualValues(t, 1, audit, "aksi tercatat di audit trail")

	// Sudah selesai: menutup lagi tidak mengubah apa pun.
	assert.Equal(t, hasilBulk{Gagal: 1}, data[hasilBulk](t, bulk(emailMahasiswa1, "tandai_selesai", "TGS-mahasiswa_1-2")))
}

func TestBulk_ManagerClosesOthersTask(t *testing.T) {
	s := sharedSuite(t)
	require.NoError(t, s.db.Exec("UPDATE tugas_lapangan SET jumlah_selesai = target WHERE kode = 'TGS-mahasiswa_3-2'").Error)

	w := s.call(t, http.MethodPost, "/v1/lapangan/tugas/bulk", emailKoordinator, map[string]any{"action": "tandai_selesai", "ids": []string{"TGS-mahasiswa_3-2"}})

	require.Equal(t, http.StatusOK, w.Code, w.Body.String())
	assert.Equal(t, hasilBulk{Berhasil: 1}, data[hasilBulk](t, w))
}

func TestBulk_UnduhDataAcuanCountsAccessibleTasks(t *testing.T) {
	s := sharedSuite(t)

	w := s.call(t, http.MethodPost, "/v1/lapangan/tugas/bulk", emailMahasiswa1,
		map[string]any{"action": "unduh_data_acuan", "ids": []string{"TGS-mahasiswa_1-1", "TGS-mahasiswa_2-1"}})

	require.Equal(t, http.StatusOK, w.Code, w.Body.String())
	assert.Equal(t, hasilBulk{Berhasil: 1, Gagal: 1}, data[hasilBulk](t, w))
}

func TestBulk_BadRequestsAndAuthorization(t *testing.T) {
	s := sharedSuite(t)
	post := func(email string, body map[string]any) int {
		return s.call(t, http.MethodPost, "/v1/lapangan/tugas/bulk", email, body).Code
	}

	assert.Equal(t, http.StatusBadRequest, post(emailMahasiswa1, map[string]any{"action": "hapus", "ids": []string{"a"}}))
	assert.Equal(t, http.StatusBadRequest, post(emailMahasiswa1, map[string]any{"action": "tandai_selesai", "ids": []string{}}))
	assert.Equal(t, http.StatusBadRequest, post(emailMahasiswa1, map[string]any{"action": "tandai_selesai", "ids": []string{strings.Repeat("T", 65)}}), "rujukan lebih dari 64 karakter")
	assert.Equal(t, http.StatusBadRequest, post(emailMahasiswa1, map[string]any{"ids": []string{"a"}}))
	assert.Equal(t, http.StatusForbidden, post(emailTanpaIzin, map[string]any{"action": "tandai_selesai", "ids": []string{"a"}}))
	assert.Equal(t, http.StatusUnauthorized, post("", map[string]any{"action": "tandai_selesai", "ids": []string{"a"}}))
}
