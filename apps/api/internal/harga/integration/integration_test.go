// Package integration menguji API harga terhadap PostgreSQL sungguhan dengan
// seed master.reference, akun demo, dan histori harga demo.
package integration

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"strconv"
	"sync"
	"testing"
	"time"

	authRoutes "github.com/garda-inflasi/api/internal/auth/routes"
	hargaRoutes "github.com/garda-inflasi/api/internal/harga/routes"
	"github.com/garda-inflasi/api/internal/platform/pgtest"
	"github.com/garda-inflasi/api/internal/seed"
	"github.com/garda-inflasi/api/pkg/logger"
	"github.com/garda-inflasi/api/pkg/token"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

const (
	jwtSecret = "harga-integration-secret-key-32chars!"

	// Izin: OPS_KOORDINATOR = lihat + input + validasi; MANAJER_DATA = lihat + publikasi;
	// OPS_MAHASISWA = input saja; STRATEGIS_BI = lihat saja.
	emailKoordinator = "koordinator.lapangan@gardainflasi.id"
	emailManajerData = "manajer.data@gardainflasi.id"
	emailMahasiswa   = "mahasiswa.1@gardainflasi.id"
	emailLihatSaja   = "strategis.bi@gardainflasi.id"
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
		db := pg.CreateDatabase(t, "harga_shared")
		pgtest.MigrateUp(t, db)
		require.NoError(t, seed.SeedReference(t.Context(), db))
		require.NoError(t, seed.SeedDemo(t.Context(), db))

		tokens, log := token.NewManager(jwtSecret), logger.NewDefault()
		router := gin.New()
		v1 := router.Group("/v1")
		authRoutes.Register(v1, &authRoutes.Dependencies{DB: db, Token: tokens, Logger: log})
		hargaRoutes.Register(v1, &hargaRoutes.Dependencies{DB: db, Token: tokens, Logger: log})
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

type entri struct {
	ID            string  `json:"id"`
	KomoditasKode string  `json:"komoditas_kode"`
	KomoditasNama string  `json:"komoditas_nama"`
	PasarID       string  `json:"pasar_id"`
	PasarNama     string  `json:"pasar_nama"`
	KabupatenKode string  `json:"kabupaten_kode"`
	Wilayah       string  `json:"wilayah"`
	Tipe          string  `json:"tipe"`
	Harga         int     `json:"harga"`
	Satuan        string  `json:"satuan"`
	PetugasID     *string `json:"petugas_id"`
	PetugasNama   *string `json:"petugas_nama"`
	Status        string  `json:"status"`
	DicatatPada   string  `json:"dicatat_pada"`
	Sumber        string  `json:"sumber"`
}

func data[T any](t *testing.T, w *httptest.ResponseRecorder) T {
	t.Helper()
	var res struct {
		Data T `json:"data"`
	}
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &res), w.Body.String())
	return res.Data
}

func total(t *testing.T, w *httptest.ResponseRecorder) int {
	t.Helper()
	n, err := strconv.Atoi(w.Header().Get("X-Total-Count"))
	require.NoError(t, err, "X-Total-Count")
	return n
}

func TestList_ReturnsJoinedRowsWithTotal(t *testing.T) {
	s := sharedSuite(t)

	w := s.call(t, http.MethodGet, "/v1/harga?limit=50", emailKoordinator, nil)

	require.Equal(t, http.StatusOK, w.Code, w.Body.String())
	rows := data[[]entri](t, w)
	assert.Len(t, rows, 50)
	assert.Greater(t, total(t, w), 10_000)
	for _, e := range rows {
		assert.NotEmpty(t, e.KomoditasNama)
		assert.NotEmpty(t, e.PasarNama)
		assert.NotEmpty(t, e.Wilayah)
		assert.NotEmpty(t, e.Satuan)
		assert.Contains(t, []string{"PRODUSEN", "KONSUMEN"}, e.Tipe)
		require.NotNil(t, e.PetugasNama)
	}
	for i := 1; i < len(rows); i++ {
		assert.GreaterOrEqual(t, rows[i-1].DicatatPada, rows[i].DicatatPada, "terbaru lebih dulu")
	}
}

func TestList_PaginationDoesNotOverlap(t *testing.T) {
	s := sharedSuite(t)

	first := data[[]entri](t, s.call(t, http.MethodGet, "/v1/harga?limit=25&page=1", emailKoordinator, nil))
	second := data[[]entri](t, s.call(t, http.MethodGet, "/v1/harga?limit=25&page=2", emailKoordinator, nil))

	require.Len(t, first, 25)
	require.Len(t, second, 25)
	seen := map[string]bool{}
	for _, e := range first {
		seen[e.ID] = true
	}
	for _, e := range second {
		assert.False(t, seen[e.ID], "halaman 2 mengulang %s", e.ID)
	}
}

func TestList_Filters(t *testing.T) {
	s := sharedSuite(t)

	t.Run("status", func(t *testing.T) {
		w := s.call(t, http.MethodGet, "/v1/harga?status=DIAJUKAN&limit=500", emailKoordinator, nil)
		require.Equal(t, http.StatusOK, w.Code)
		rows := data[[]entri](t, w)
		require.NotEmpty(t, rows)
		for _, e := range rows {
			assert.Equal(t, "DIAJUKAN", e.Status)
		}
	})
	t.Run("wilayah dan komoditas", func(t *testing.T) {
		w := s.call(t, http.MethodGet, "/v1/harga?wilayah=1171&komoditas=CBM-KRT&limit=500", emailKoordinator, nil)
		require.Equal(t, http.StatusOK, w.Code)
		rows := data[[]entri](t, w)
		require.NotEmpty(t, rows)
		for _, e := range rows {
			assert.Equal(t, "1171", e.KabupatenKode)
			assert.Equal(t, "CBM-KRT", e.KomoditasKode)
		}
		assert.Equal(t, len(rows), total(t, w))
	})
	t.Run("rentang tanggal", func(t *testing.T) {
		hariIni := time.Now().In(time.FixedZone("WIB", 7*3600)).Format("2006-01-02")
		w := s.call(t, http.MethodGet, "/v1/harga?dari="+hariIni+"&sampai="+hariIni+"&limit=1", emailKoordinator, nil)
		require.Equal(t, http.StatusOK, w.Code)
		hanyaHariIni := total(t, w)

		semua := total(t, s.call(t, http.MethodGet, "/v1/harga?limit=1", emailKoordinator, nil))
		assert.Positive(t, hanyaHariIni)
		assert.Less(t, hanyaHariIni, semua)
	})
	t.Run("status tidak dikenal", func(t *testing.T) {
		assert.Equal(t, http.StatusBadRequest, s.call(t, http.MethodGet, "/v1/harga?status=BASI", emailKoordinator, nil).Code)
	})
	t.Run("limit di luar batas", func(t *testing.T) {
		assert.Equal(t, http.StatusBadRequest, s.call(t, http.MethodGet, "/v1/harga?limit=5000", emailKoordinator, nil).Code)
	})
	t.Run("tanggal salah format", func(t *testing.T) {
		assert.Equal(t, http.StatusBadRequest, s.call(t, http.MethodGet, "/v1/harga?dari=kemarin", emailKoordinator, nil).Code)
	})
}

func TestList_Authorization(t *testing.T) {
	s := sharedSuite(t)

	assert.Equal(t, http.StatusOK, s.call(t, http.MethodGet, "/v1/harga?limit=1", emailLihatSaja, nil).Code, "harga.lihat cukup")
	assert.Equal(t, http.StatusForbidden, s.call(t, http.MethodGet, "/v1/harga?limit=1", emailMahasiswa, nil).Code, "tanpa harga.lihat")
	assert.Equal(t, http.StatusUnauthorized, s.call(t, http.MethodGet, "/v1/harga?limit=1", "", nil).Code)
}

func TestDetail(t *testing.T) {
	s := sharedSuite(t)
	rows := data[[]entri](t, s.call(t, http.MethodGet, "/v1/harga?limit=1", emailKoordinator, nil))
	require.Len(t, rows, 1)

	w := s.call(t, http.MethodGet, "/v1/harga/"+url.PathEscape(rows[0].ID), emailKoordinator, nil)

	require.Equal(t, http.StatusOK, w.Code, w.Body.String())
	detail := data[entri](t, w)
	assert.Equal(t, rows[0].ID, detail.ID)
	assert.Equal(t, "LAPANGAN", detail.Sumber)
	assert.Equal(t, http.StatusNotFound, s.call(t, http.MethodGet, "/v1/harga/tidak-ada", emailKoordinator, nil).Code)
	assert.Equal(t, http.StatusNotFound, s.call(t, http.MethodGet, "/v1/harga/999999999", emailKoordinator, nil).Code, "ID bentuk sah tetapi tidak ada")
	assert.Equal(t, http.StatusForbidden, s.call(t, http.MethodGet, "/v1/harga/"+url.PathEscape(rows[0].ID), emailMahasiswa, nil).Code)
}

func TestTren(t *testing.T) {
	s := sharedSuite(t)

	w := s.call(t, http.MethodGet, "/v1/harga/tren?komoditas=CBM-KRT&wilayah=1171&hari=45", emailLihatSaja, nil)

	require.Equal(t, http.StatusOK, w.Code, w.Body.String())
	titik := data[[]struct {
		Tanggal  string   `json:"tanggal"`
		Produsen *float64 `json:"produsen"`
		Konsumen *float64 `json:"konsumen"`
	}](t, w)
	assert.GreaterOrEqual(t, len(titik), 30, "tren 45 hari memuat hampir tiap hari")
	assert.LessOrEqual(t, len(titik), 46)
	var denganProdusen, denganKonsumen int
	for i, p := range titik {
		if p.Konsumen != nil {
			assert.Positive(t, *p.Konsumen)
			denganKonsumen++
		}
		if p.Produsen != nil {
			assert.Positive(t, *p.Produsen)
			denganProdusen++
		}
		if i > 0 {
			assert.Greater(t, p.Tanggal, titik[i-1].Tanggal, "urut menaik")
		}
	}
	assert.GreaterOrEqual(t, denganKonsumen, 30)
	assert.Positive(t, denganProdusen, "cabai juga dicatat harga produsennya di Banda Aceh")

	mingguan := data[[]json.RawMessage](t, s.call(t, http.MethodGet, "/v1/harga/tren?komoditas=CBM-KRT&wilayah=1171&hari=90&interval=mingguan", emailLihatSaja, nil))
	assert.Less(t, len(mingguan), 20)
	assert.Greater(t, len(mingguan), 8)

	assert.Equal(t, http.StatusBadRequest, s.call(t, http.MethodGet, "/v1/harga/tren?komoditas=CBM-KRT", emailLihatSaja, nil).Code)
	assert.Equal(t, http.StatusBadRequest, s.call(t, http.MethodGet, "/v1/harga/tren?komoditas=CBM-KRT&wilayah=1171&interval=tahunan", emailLihatSaja, nil).Code)
	assert.Equal(t, http.StatusForbidden, s.call(t, http.MethodGet, "/v1/harga/tren?komoditas=CBM-KRT&wilayah=1171", emailMahasiswa, nil).Code)
}

func TestRingkasan(t *testing.T) {
	s := sharedSuite(t)

	w := s.call(t, http.MethodGet, "/v1/harga/ringkasan?hari=7", emailLihatSaja, nil)

	require.Equal(t, http.StatusOK, w.Code, w.Body.String())
	r := data[map[string]int](t, w)
	for _, kunci := range []string{"draf", "diajukan", "divalidasi", "dipublikasi", "ditolak", "anomali"} {
		assert.Contains(t, r, kunci)
	}
	assert.Positive(t, r["diajukan"])
	assert.Positive(t, r["dipublikasi"])
	assert.Positive(t, r["anomali"])

	lebihLuas := data[map[string]int](t, s.call(t, http.MethodGet, "/v1/harga/ringkasan?hari=90", emailLihatSaja, nil))
	assert.Greater(t, lebihLuas["dipublikasi"], r["dipublikasi"])
	assert.Equal(t, http.StatusBadRequest, s.call(t, http.MethodGet, "/v1/harga/ringkasan?hari=400", emailLihatSaja, nil).Code)
}

func TestSaya_OnlyOwnEntries(t *testing.T) {
	s := sharedSuite(t)

	w := s.call(t, http.MethodGet, "/v1/harga/saya?limit=100", emailMahasiswa, nil)

	require.Equal(t, http.StatusOK, w.Code, w.Body.String())
	rows := data[[]entri](t, w)
	require.NotEmpty(t, rows)
	for _, e := range rows {
		require.NotNil(t, e.PetugasID)
		assert.Equal(t, uid("mahasiswa_1"), *e.PetugasID)
	}
	assert.Equal(t, http.StatusForbidden, s.call(t, http.MethodGet, "/v1/harga/saya", emailLihatSaja, nil).Code, "tanpa harga.input")
}

func TestCreate(t *testing.T) {
	s := sharedSuite(t)
	body := map[string]any{"komoditas_kode": "BWM", "pasar_id": "PSR-1171-01", "tipe": "KONSUMEN", "harga": 43500, "kualitas": "PREMIUM", "catatan": "stok melimpah"}

	w := s.call(t, http.MethodPost, "/v1/harga", emailMahasiswa, body)

	require.Equal(t, http.StatusCreated, w.Code, w.Body.String())
	id := data[struct {
		ID string `json:"id"`
	}](t, w).ID
	require.NotEmpty(t, id)

	detail := data[entri](t, s.call(t, http.MethodGet, "/v1/harga/"+id, emailKoordinator, nil))
	assert.Equal(t, "DIAJUKAN", detail.Status)
	assert.Equal(t, "kg", detail.Satuan, "satuan diambil dari master komoditas")
	assert.Equal(t, 43500, detail.Harga)
	require.NotNil(t, detail.PetugasID)
	assert.Equal(t, uid("mahasiswa_1"), *detail.PetugasID)

	var audit int64
	require.NoError(t, s.db.Raw("SELECT count(*) FROM audit_log WHERE aksi = 'HARGA_INPUT' AND entitas_id = ?", id).Scan(&audit).Error)
	assert.EqualValues(t, 1, audit, "input harga tercatat di audit trail")

	t.Run("validasi", func(t *testing.T) {
		kasus := map[string]map[string]any{
			"komoditas tidak ada": {"komoditas_kode": "XXX", "pasar_id": "PSR-1171-01", "tipe": "KONSUMEN", "harga": 1000},
			"pasar tidak ada":     {"komoditas_kode": "BWM", "pasar_id": "PSR-XXX", "tipe": "KONSUMEN", "harga": 1000},
			"harga nol":           {"komoditas_kode": "BWM", "pasar_id": "PSR-1171-01", "tipe": "KONSUMEN", "harga": 0},
			"tipe salah":          {"komoditas_kode": "BWM", "pasar_id": "PSR-1171-01", "tipe": "ECERAN", "harga": 1000},
			"kualitas salah":      {"komoditas_kode": "BWM", "pasar_id": "PSR-1171-01", "tipe": "KONSUMEN", "harga": 1000, "kualitas": "SUPER"},
			"waktu masa depan":    {"komoditas_kode": "BWM", "pasar_id": "PSR-1171-01", "tipe": "KONSUMEN", "harga": 1000, "dicatat_pada": time.Now().Add(2 * time.Hour).Format(time.RFC3339)},
		}
		for nama, payload := range kasus {
			assert.Equal(t, http.StatusBadRequest, s.call(t, http.MethodPost, "/v1/harga", emailMahasiswa, payload).Code, nama)
		}
	})
	t.Run("izin", func(t *testing.T) {
		assert.Equal(t, http.StatusForbidden, s.call(t, http.MethodPost, "/v1/harga", emailLihatSaja, body).Code)
		assert.Equal(t, http.StatusUnauthorized, s.call(t, http.MethodPost, "/v1/harga", "", body).Code)
	})
}

// pasar_id adalah kode pasar (FK ke pasar.kode, 000122). ID surrogate numerik bukan rujukan sah: ditolak 400 pada
// validasi, bukan lolos lalu gagal di FK saat INSERT (500).
func TestCreate_PasarIDSurrogateDitolak(t *testing.T) {
	s := sharedSuite(t)
	var id string
	require.NoError(t, s.db.Raw("SELECT id::text FROM pasar WHERE kode = 'PSR-1171-01'").Scan(&id).Error)

	w := s.call(t, http.MethodPost, "/v1/harga", emailMahasiswa, map[string]any{"komoditas_kode": "BWM", "pasar_id": id, "tipe": "KONSUMEN", "harga": 43500})

	assert.Equal(t, http.StatusBadRequest, w.Code, w.Body.String())
}

// buat mencatat entri baru oleh email dan mengembalikan id-nya.
func (s *suite) buat(t *testing.T, email string) string {
	t.Helper()
	w := s.call(t, http.MethodPost, "/v1/harga", email, map[string]any{"komoditas_kode": "TLR", "pasar_id": "PSR-1171-01", "tipe": "KONSUMEN", "harga": 31000})
	require.Equal(t, http.StatusCreated, w.Code, w.Body.String())
	return data[struct {
		ID string `json:"id"`
	}](t, w).ID
}

func (s *suite) status(t *testing.T, id string) string {
	t.Helper()
	return data[entri](t, s.call(t, http.MethodGet, "/v1/harga/"+id, emailKoordinator, nil)).Status
}

type hasilBulk struct {
	Berhasil int `json:"berhasil"`
	Gagal    int `json:"gagal"`
}

func TestBulk_ValidasiThenPublikasi(t *testing.T) {
	s := sharedSuite(t)
	a, b := s.buat(t, emailMahasiswa), s.buat(t, emailMahasiswa)

	w := s.call(t, http.MethodPost, "/v1/harga/bulk", emailKoordinator, map[string]any{"action": "validasi", "ids": []string{a, b, "999999999"}})
	require.Equal(t, http.StatusOK, w.Code, w.Body.String())
	assert.Equal(t, hasilBulk{Berhasil: 2, Gagal: 1}, data[hasilBulk](t, w))
	assert.Equal(t, "DIVALIDASI", s.status(t, a))

	var audit int64
	require.NoError(t, s.db.Raw("SELECT count(*) FROM audit_log WHERE aksi = 'HARGA_VALIDASI' AND aktor_id = garda_uuid_stabil('pengguna', 'koordinator_lapangan')").Scan(&audit).Error)
	assert.Positive(t, audit, "aksi massal tercatat di audit trail")

	// Validasi ulang: sudah DIVALIDASI, tidak ada yang berubah.
	again := data[hasilBulk](t, s.call(t, http.MethodPost, "/v1/harga/bulk", emailKoordinator, map[string]any{"action": "validasi", "ids": []string{a}}))
	assert.Equal(t, hasilBulk{Berhasil: 0, Gagal: 1}, again)

	// Koordinator tidak memegang harga.publikasi.
	assert.Equal(t, http.StatusForbidden, s.call(t, http.MethodPost, "/v1/harga/bulk", emailKoordinator, map[string]any{"action": "publikasi", "ids": []string{a}}).Code)

	pub := s.call(t, http.MethodPost, "/v1/harga/bulk", emailManajerData, map[string]any{"action": "publikasi", "ids": []string{a, b}})
	require.Equal(t, http.StatusOK, pub.Code, pub.Body.String())
	assert.Equal(t, hasilBulk{Berhasil: 2}, data[hasilBulk](t, pub))
	assert.Equal(t, "DIPUBLIKASI", s.status(t, a))

	// Manajer data hanya berhak publikasi, bukan validasi.
	assert.Equal(t, http.StatusForbidden, s.call(t, http.MethodPost, "/v1/harga/bulk", emailManajerData, map[string]any{"action": "validasi", "ids": []string{a}}).Code)
}

func TestBulk_PublikasiRequiresValidatedEntry(t *testing.T) {
	s := sharedSuite(t)
	id := s.buat(t, emailMahasiswa)

	got := data[hasilBulk](t, s.call(t, http.MethodPost, "/v1/harga/bulk", emailManajerData, map[string]any{"action": "publikasi", "ids": []string{id}}))

	assert.Equal(t, hasilBulk{Gagal: 1}, got)
	assert.Equal(t, "DIAJUKAN", s.status(t, id))
}

func TestBulk_SeparationOfDuties(t *testing.T) {
	s := sharedSuite(t)
	own := s.buat(t, emailKoordinator)

	got := data[hasilBulk](t, s.call(t, http.MethodPost, "/v1/harga/bulk", emailKoordinator, map[string]any{"action": "validasi", "ids": []string{own}}))

	assert.Equal(t, hasilBulk{Gagal: 1}, got, "pencatat tidak boleh memvalidasi entrinya sendiri")
	assert.Equal(t, "DIAJUKAN", s.status(t, own))
}

func TestBulk_TolakDanKembalikan(t *testing.T) {
	s := sharedSuite(t)
	ditolak, dikembalikan := s.buat(t, emailMahasiswa), s.buat(t, emailMahasiswa)

	assert.Equal(t, hasilBulk{Berhasil: 1}, data[hasilBulk](t, s.call(t, http.MethodPost, "/v1/harga/bulk", emailKoordinator, map[string]any{"action": "tolak", "ids": []string{ditolak}, "alasan": "harga tidak wajar"})))
	assert.Equal(t, "DITOLAK", s.status(t, ditolak))

	assert.Equal(t, hasilBulk{Berhasil: 1}, data[hasilBulk](t, s.call(t, http.MethodPost, "/v1/harga/bulk", emailKoordinator, map[string]any{"action": "kembalikan", "ids": []string{dikembalikan}})))
	assert.Equal(t, "DRAF", s.status(t, dikembalikan))

	// Entri yang sudah ditolak tidak bisa ditolak lagi atau dikembalikan.
	again := data[hasilBulk](t, s.call(t, http.MethodPost, "/v1/harga/bulk", emailKoordinator, map[string]any{"action": "tolak", "ids": []string{ditolak}, "alasan": "ulang"}))
	assert.Equal(t, hasilBulk{Gagal: 1}, again)
}

func TestBulk_BadRequests(t *testing.T) {
	s := sharedSuite(t)

	cases := map[string]map[string]any{
		"aksi tidak dikenal":  {"action": "hapus", "ids": []string{"1"}},
		"tanpa aksi":          {"ids": []string{"1"}},
		"ids kosong":          {"action": "validasi", "ids": []string{}},
		"id bukan angka":      {"action": "validasi", "ids": []string{"a"}},
		"kode lama":           {"action": "validasi", "ids": []string{"HRG-D00-00-01-K"}},
		"id nol":              {"action": "validasi", "ids": []string{"0"}},
		"id negatif":          {"action": "validasi", "ids": []string{"-4"}},
		"id berawalan nol":    {"action": "validasi", "ids": []string{"007"}},
		"satu salah dari dua": {"action": "validasi", "ids": []string{"12", "dua"}},
		"id pecahan":          {"action": "validasi", "ids": []any{1.5}},
	}
	for name, body := range cases {
		assert.Equal(t, http.StatusBadRequest, s.call(t, http.MethodPost, "/v1/harga/bulk", emailKoordinator, body).Code, name)
	}
	assert.Equal(t, http.StatusForbidden, s.call(t, http.MethodPost, "/v1/harga/bulk", emailMahasiswa, map[string]any{"action": "validasi", "ids": []string{"a"}}).Code)
	assert.Equal(t, http.StatusForbidden, s.call(t, http.MethodPost, "/v1/harga/bulk", emailLihatSaja, map[string]any{"action": "validasi", "ids": []string{"a"}}).Code)
	assert.Equal(t, http.StatusUnauthorized, s.call(t, http.MethodPost, "/v1/harga/bulk", "", map[string]any{"action": "validasi", "ids": []string{"a"}}).Code)
}
