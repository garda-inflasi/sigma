// Package integration menguji API prediksi terhadap PostgreSQL sungguhan hasil
// migrasi, dengan seed master.reference, akun demo, dan prediksi.demo.
package integration

import (
	"bytes"
	"encoding/csv"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"sync"
	"testing"
	"time"

	authRoutes "github.com/garda-inflasi/api/internal/auth/routes"
	"github.com/garda-inflasi/api/internal/master"
	"github.com/garda-inflasi/api/internal/platform/pgtest"
	prediksiRoutes "github.com/garda-inflasi/api/internal/prediksi/routes"
	"github.com/garda-inflasi/api/internal/seed"
	"github.com/garda-inflasi/api/pkg/logger"
	"github.com/garda-inflasi/api/pkg/token"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

const (
	jwtSecret = "prediksi-integration-secret-key-32chars!"

	// STRATEGIS_BI: prediksi.lihat; OPS_MAHASISWA: tanpa izin prediksi.
	emailBerhak    = "strategis.bi@gardainflasi.id"
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
		db := pg.CreateDatabase(t, "prediksi_shared")
		pgtest.MigrateUp(t, db)
		require.NoError(t, seed.SeedReference(t.Context(), db))
		require.NoError(t, seed.SeedDemo(t.Context(), db))
		seedRiwayat(t, db)

		tokens, log := token.NewManager(jwtSecret), logger.NewDefault()
		router := gin.New()
		v1 := router.Group("/v1")
		authRoutes.Register(v1, &authRoutes.Dependencies{DB: db, Token: tokens, Logger: log})
		prediksiRoutes.Register(v1, &prediksiRoutes.Dependencies{DB: db, Token: tokens, Logger: log})
		shared = &suite{db: db, router: router, tokens: map[string]string{}}
	})
	require.NotNil(t, shared, "suite bersama gagal dibuat")
	return shared
}

// seedRiwayat menanam harga konsumen tervalidasi cabai di Banda Aceh pada tiga hari
// sebelum hari dasar, ditambah entri yang harus diabaikan (belum divalidasi, harga produsen, kabupaten lain).
func seedRiwayat(t *testing.T, db *gorm.DB) {
	t.Helper()
	dasar := fixedNow.Truncate(24 * time.Hour)
	entri := []struct {
		id, pasar, tipe, status string
		harga, hariLalu         int
	}{
		{"E1", "PSR-1171-01", "KONSUMEN", "DIPUBLIKASI", 50000, 3},
		{"E2", "PSR-1171-02", "KONSUMEN", "DIVALIDASI", 52000, 3},
		{"E3", "PSR-1171-01", "KONSUMEN", "DIPUBLIKASI", 54000, 2},
		{"E4", "PSR-1171-01", "KONSUMEN", "DIPUBLIKASI", 56000, 1},
		{"E5", "PSR-1171-01", "KONSUMEN", "DIAJUKAN", 99000, 1},
		{"E6", "PSR-1171-01", "PRODUSEN", "DIPUBLIKASI", 20000, 1},
		{"E7", "PSR-1108-01", "KONSUMEN", "DIPUBLIKASI", 70000, 1},
	}
	for _, e := range entri {
		require.NoError(t, db.Exec(`
			INSERT INTO entri_harga (kode, komoditas_kode, pasar_id, tipe, harga, satuan, status, dicatat_pada)
			VALUES (?, 'CBM-KRT', ?, ?::tipe_harga, ?, 'kg', ?::status_entri, ?)`,
			e.id, e.pasar, e.tipe, e.harga, e.status, dasar.AddDate(0, 0, -e.hariLalu).Add(10*time.Hour)).Error)
	}
}

func (s *suite) call(t *testing.T, path, email string) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequest(http.MethodGet, path, nil)
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

func decode[T any](t *testing.T, w *httptest.ResponseRecorder) T {
	t.Helper()
	var res struct {
		Data T `json:"data"`
	}
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &res), w.Body.String())
	return res.Data
}

type ringkasanRow struct {
	KomoditasKode string  `json:"komoditas_kode"`
	KomoditasNama string  `json:"komoditas_nama"`
	HorizonHari   int     `json:"horizon_hari"`
	HargaKini     int     `json:"harga_kini"`
	Proyeksi      int     `json:"proyeksi"`
	PerubahanPct  float64 `json:"perubahan_pct"`
	BawahPct      float64 `json:"bawah_pct"`
	AtasPct       float64 `json:"atas_pct"`
	ModelVersi    string  `json:"model_versi"`
	Mape          float64 `json:"mape"`
	Sumber        string  `json:"sumber"`
}

type titik struct {
	Tanggal  string `json:"tanggal"`
	Aktual   *int   `json:"aktual"`
	Proyeksi *int   `json:"proyeksi"`
	Bawah    *int   `json:"bawah"`
	Atas     *int   `json:"atas"`
}

type detailResp struct {
	KomoditasKode string  `json:"komoditas_kode"`
	KabupatenKode string  `json:"kabupaten_kode"`
	KabupatenNama string  `json:"kabupaten_nama"`
	HorizonHari   int     `json:"horizon_hari"`
	HargaKini     int     `json:"harga_kini"`
	PerubahanPct  float64 `json:"perubahan_pct"`
	BawahPct      float64 `json:"bawah_pct"`
	AtasPct       float64 `json:"atas_pct"`
	Seri          []titik `json:"seri"`
	Faktor        []struct {
		Nama       string  `json:"nama"`
		Kontribusi float64 `json:"kontribusi"`
		Detail     string  `json:"detail"`
	} `json:"faktor"`
	Narasi string `json:"narasi"`
	Model  struct {
		Nama    string  `json:"nama"`
		Versi   string  `json:"versi"`
		Mape    float64 `json:"mape"`
		Dilatih string  `json:"dilatih"`
	} `json:"model"`
	Rekomendasi []string `json:"rekomendasi"`
	Sumber      string   `json:"sumber"`
}

func TestPrediksi_Authorization(t *testing.T) {
	s := sharedSuite(t)

	for _, path := range []string{"/v1/prediksi", "/v1/prediksi/CBM-KRT", "/v1/prediksi/CBM-KRT/export", "/v1/prediksi/registri"} {
		t.Run(path, func(t *testing.T) {
			assert.Equal(t, http.StatusUnauthorized, s.call(t, path, "").Code)
			assert.Equal(t, http.StatusForbidden, s.call(t, path, emailTanpaIzin).Code)
			assert.Equal(t, http.StatusOK, s.call(t, path, emailBerhak).Code)
		})
	}
}

func TestPrediksi_ListCoversEveryCommodityWithConsistentIntervals(t *testing.T) {
	s := sharedSuite(t)

	got := decode[[]ringkasanRow](t, s.call(t, "/v1/prediksi", emailBerhak))

	require.Len(t, got, len(master.Komoditas()))
	byKode := map[string]ringkasanRow{}
	for _, r := range got {
		byKode[r.KomoditasKode] = r
		assert.Equal(t, 14, r.HorizonHari, "bawaan horizon 14 hari")
		assert.LessOrEqual(t, r.BawahPct, r.PerubahanPct, r.KomoditasKode)
		assert.LessOrEqual(t, r.PerubahanPct, r.AtasPct, r.KomoditasKode)
		assert.Positive(t, r.HargaKini)
		assert.Equal(t, "v2.3.1", r.ModelVersi)
		assert.Positive(t, r.Mape)
		assert.Equal(t, "DEMO", r.Sumber)
	}
	assert.Equal(t, 14.2, byKode["CBM-KRT"].PerubahanPct)
	assert.Equal(t, -2.4, byKode["BRS-MED"].PerubahanPct)
}

func TestPrediksi_ListHonoursHorizonAndKabupaten(t *testing.T) {
	s := sharedSuite(t)

	t.Run("horizon 30 hari", func(t *testing.T) {
		got := decode[[]ringkasanRow](t, s.call(t, "/v1/prediksi?horizon=30", emailBerhak))
		require.NotEmpty(t, got)
		for _, r := range got {
			assert.Equal(t, 30, r.HorizonHari)
		}
	})

	t.Run("kabupaten lain hanya komoditas pilot", func(t *testing.T) {
		got := decode[[]ringkasanRow](t, s.call(t, "/v1/prediksi?kabupaten=1106", emailBerhak))
		var kode []string
		for _, r := range got {
			kode = append(kode, r.KomoditasKode)
		}
		assert.ElementsMatch(t, []string{"BRS-MED", "CBM-KRT", "BWM"}, kode)
	})

	t.Run("horizon tanpa proyeksi memberi daftar kosong", func(t *testing.T) {
		got := decode[[]ringkasanRow](t, s.call(t, "/v1/prediksi?horizon=3", emailBerhak))
		assert.Empty(t, got)
	})

	t.Run("parameter tidak valid ditolak", func(t *testing.T) {
		for _, q := range []string{"?horizon=0", "?horizon=abc", "?horizon=-1", "?horizon=366", "?kabupaten=11", "?kabupaten=abcd"} {
			assert.Equal(t, http.StatusBadRequest, s.call(t, "/v1/prediksi"+q, emailBerhak).Code, q)
		}
	})
}

func TestPrediksi_DetailJoinsHistoryProjectionFactorsAndModel(t *testing.T) {
	s := sharedSuite(t)

	w := s.call(t, "/v1/prediksi/CBM-KRT", emailBerhak)
	require.Equal(t, http.StatusOK, w.Code, w.Body.String())
	got := decode[detailResp](t, w)

	assert.Equal(t, "CBM-KRT", got.KomoditasKode)
	assert.Equal(t, "1171", got.KabupatenKode)
	assert.Equal(t, "KOTA BANDA ACEH", got.KabupatenNama)
	assert.Equal(t, 14, got.HorizonHari)
	assert.Equal(t, 14.2, got.PerubahanPct)
	assert.Equal(t, "DEMO", got.Sumber)
	assert.Contains(t, got.Narasi, "Cabai merah keriting di Kota Banda Aceh diproyeksi naik 14,2% dalam 14 hari")
	assert.Contains(t, got.Narasi, "curah hujan sentra gayo")
	assert.NotEmpty(t, got.Rekomendasi)
	assert.Equal(t, "Ensemble LightGBM + SARIMAX", got.Model.Nama)
	assert.Equal(t, "v2.3.1", got.Model.Versi)
	assert.Equal(t, "2026-10-29", got.Model.Dilatih)

	require.Len(t, got.Faktor, 5)
	assert.Equal(t, "Curah hujan sentra Gayo", got.Faktor[0].Nama)
	assert.Greater(t, got.Faktor[0].Kontribusi, 0.0)
	assert.Less(t, got.Faktor[4].Kontribusi, 0.0)

	// Seri: 3 hari riwayat (rata-rata harian entri valid saja), titik sambung di hari dasar, proyeksi 7 dan 14 hari.
	require.Len(t, got.Seri, 6, "%+v", got.Seri)
	assert.Equal(t, []string{"2026-11-02", "2026-11-03", "2026-11-04", "2026-11-05", "2026-11-12", "2026-11-19"}, tanggalSeri(got.Seri))
	assert.Equal(t, 51000, *got.Seri[0].Aktual, "rata-rata 50.000 dan 52.000")
	assert.Equal(t, 54000, *got.Seri[1].Aktual)
	assert.Equal(t, 56000, *got.Seri[2].Aktual)
	for _, p := range got.Seri[:3] {
		assert.Nil(t, p.Proyeksi)
	}
	sambung := got.Seri[3]
	assert.Equal(t, got.HargaKini, *sambung.Aktual)
	assert.Equal(t, got.HargaKini, *sambung.Proyeksi)
	for _, p := range got.Seri[4:] {
		assert.Nil(t, p.Aktual)
		assert.LessOrEqual(t, *p.Bawah, *p.Proyeksi)
		assert.LessOrEqual(t, *p.Proyeksi, *p.Atas)
	}
	assert.Less(t, *got.Seri[4].Proyeksi, *got.Seri[5].Proyeksi, "cabai diproyeksi terus naik")
}

func tanggalSeri(seri []titik) []string {
	out := make([]string, len(seri))
	for i, p := range seri {
		out[i] = p.Tanggal
	}
	return out
}

func TestPrediksi_DetailWithoutHistoryStillReturnsProjection(t *testing.T) {
	s := sharedSuite(t)

	got := decode[detailResp](t, s.call(t, "/v1/prediksi/BRS-MED?horizon=7", emailBerhak))

	require.Len(t, got.Seri, 2, "titik sambung dan proyeksi 7 hari")
	assert.Equal(t, 7, got.HorizonHari)
	assert.Equal(t, -1.2, got.PerubahanPct)
	assert.Contains(t, got.Narasi, "turun 1,2%")
}

func TestPrediksi_DetailOtherKabupaten(t *testing.T) {
	s := sharedSuite(t)

	got := decode[detailResp](t, s.call(t, "/v1/prediksi/BWM?kabupaten=1106&horizon=30", emailBerhak))

	assert.Equal(t, "1106", got.KabupatenKode)
	assert.Equal(t, "KABUPATEN ACEH TENGAH", got.KabupatenNama)
	assert.Equal(t, 30, got.HorizonHari)
	assert.Len(t, got.Seri, 4, "sambung + proyeksi 7, 14, 30 hari")
}

func TestPrediksi_DetailNotFound(t *testing.T) {
	s := sharedSuite(t)

	for _, path := range []string{
		"/v1/prediksi/XXX",                    // komoditas tak dikenal
		"/v1/prediksi/TLR?kabupaten=1106",     // bukan komoditas pilot: tak ada proyeksi di kabupaten itu
		"/v1/prediksi/CBM-KRT?kabupaten=9999", // kabupaten tak dikenal
		"/v1/prediksi/CBM-KRT?horizon=3",
	} {
		assert.Equal(t, http.StatusNotFound, s.call(t, path, emailBerhak).Code, path)
	}
}

func TestPrediksi_OnlyProductionModelIsServed(t *testing.T) {
	s := sharedSuite(t)
	require.NoError(t, s.db.Exec(`UPDATE model_registry SET status = 'ARSIP' WHERE kode = 'MDL-TMT-v2.3.1'`).Error)
	t.Cleanup(func() {
		require.NoError(t, s.db.Exec(`UPDATE model_registry SET status = 'PRODUKSI' WHERE kode = 'MDL-TMT-v2.3.1'`).Error)
	})

	assert.Equal(t, http.StatusNotFound, s.call(t, "/v1/prediksi/TMT", emailBerhak).Code, "tanpa model produksi tidak ada proyeksi")
	got := decode[[]ringkasanRow](t, s.call(t, "/v1/prediksi", emailBerhak))
	for _, r := range got {
		assert.NotEqual(t, "TMT", r.KomoditasKode)
	}
}

func TestPrediksi_RegistryListsModelsWithStatus(t *testing.T) {
	s := sharedSuite(t)

	got := decode[[]struct {
		ID            string  `json:"id"`
		Kode          *string `json:"kode"`
		KomoditasKode string  `json:"komoditas_kode"`
		KomoditasNama string  `json:"komoditas_nama"`
		Versi         string  `json:"versi"`
		Mape          float64 `json:"mape"`
		Drift         float64 `json:"drift"`
		Status        string  `json:"status"`
		DilatihPada   string  `json:"dilatih_pada"`
		Sumber        string  `json:"sumber"`
	}](t, s.call(t, "/v1/prediksi/registri", emailBerhak))

	require.Len(t, got, 2*len(master.Komoditas()))
	status := map[string]int{}
	for _, m := range got {
		status[m.Status]++
		assert.Equal(t, "DEMO", m.Sumber)
		assert.NotEmpty(t, m.KomoditasNama)
		assert.NotEmpty(t, m.DilatihPada)
	}
	assert.Equal(t, map[string]int{"PRODUKSI": 14, "KANDIDAT": 14}, status)
	require.NotNil(t, got[0].Kode)
	require.NotNil(t, got[1].Kode)
	assert.Equal(t, "MDL-BRS-MED-v2.4.0-rc1", *got[0].Kode, "berurut komoditas lalu pelatihan terbaru")
	assert.Equal(t, 2.0, got[0].Mape)
	assert.Equal(t, "MDL-BRS-MED-v2.3.1", *got[1].Kode)
	assert.Equal(t, 2.2, got[1].Mape)
	assert.Equal(t, "PRODUKSI", got[1].Status)
	for _, m := range got {
		assert.Regexp(t, `^[1-9][0-9]*$`, m.ID, "ID model adalah angka desimal (BIGINT), bukan lagi teks MDL-...")
		assert.NotEqual(t, *m.Kode, m.ID)
	}
}

func TestPrediksi_ExportIsDownloadableCSV(t *testing.T) {
	s := sharedSuite(t)
	const qEkspor = `SELECT COUNT(*) FROM audit_log WHERE aksi = 'EKSPOR_DATA' AND entitas = 'proyeksi_harga' AND entitas_id = 'CBM-KRT'`
	sebelum := s.hitung(t, qEkspor)

	w := s.call(t, "/v1/prediksi/CBM-KRT/export", emailBerhak)

	require.Equal(t, http.StatusOK, w.Code)
	assert.Equal(t, `attachment; filename="proyeksi_CBM-KRT.csv"`, w.Header().Get("Content-Disposition"))
	assert.Equal(t, "text/csv; charset=utf-8", w.Header().Get("Content-Type"))

	isi := strings.TrimPrefix(w.Body.String(), "\ufeff")
	r := csv.NewReader(strings.NewReader(isi))
	r.Comma = ';'
	baris, err := r.ReadAll()
	require.NoError(t, err)
	require.Greater(t, len(baris), 1, "header dan sedikitnya satu titik")
	assert.Equal(t, []string{
		"komoditas_kode", "komoditas_nama", "kabupaten_kode", "horizon_hari", "tanggal",
		"aktual", "proyeksi", "bawah", "atas", "model_versi", "sumber",
	}, baris[0])

	// Isi CSV harus sama dengan seri pada detail: satu baris per titik, urutan sama.
	detail := decode[detailResp](t, s.call(t, "/v1/prediksi/CBM-KRT", emailBerhak))
	require.Len(t, baris[1:], len(detail.Seri))
	var adaAktual, adaProyeksi bool
	for i, t0 := range detail.Seri {
		b := baris[i+1]
		assert.Equal(t, "CBM-KRT", b[0])
		assert.Equal(t, t0.Tanggal, b[4])
		assert.Equal(t, detail.Model.Versi, b[9])
		assert.Equal(t, detail.Sumber, b[10])
		assert.Equal(t, t0.Aktual != nil, b[5] != "", "sel aktual kosong hanya bila titik tanpa aktual")
		assert.Equal(t, t0.Proyeksi != nil, b[6] != "")
		adaAktual = adaAktual || b[5] != ""
		adaProyeksi = adaProyeksi || b[6] != ""
	}
	assert.True(t, adaAktual && adaProyeksi, "CSV memuat riwayat aktual dan jalur proyeksi")
	assert.Equal(t, sebelum+1, s.hitung(t, qEkspor), "ekspor tercatat di audit trail")

	assert.Equal(t, http.StatusNotFound, s.call(t, "/v1/prediksi/XXX/export", emailBerhak).Code)
	assert.Equal(t, http.StatusBadRequest, s.call(t, "/v1/prediksi/CBM-KRT/export?horizon=0", emailBerhak).Code)
}
