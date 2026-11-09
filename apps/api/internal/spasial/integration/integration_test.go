// Package integration menguji API spasial terhadap PostgreSQL sungguhan dengan
// seed lapis reference (master, geometri), akun demo, dan neraca, rute, serta kandidat spasial demo (db/seeds).
package integration

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"sync"
	"testing"

	authRoutes "github.com/garda-inflasi/api/internal/auth/routes"
	"github.com/garda-inflasi/api/internal/platform/pgtest"
	spasialRoutes "github.com/garda-inflasi/api/internal/spasial/routes"
	"github.com/garda-inflasi/api/pkg/logger"
	"github.com/garda-inflasi/api/pkg/seed"
	"github.com/garda-inflasi/api/pkg/token"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

const (
	jwtSecret = "spasial-integration-secret-key-32chars"

	// MANAJER_PROGRAM membawa spasial.lihat; mahasiswa tidak.
	emailLihat     = "manajer.program@gardainflasi.id"
	emailTanpaIzin = "mahasiswa.1@gardainflasi.id"
)

var pg *pgtest.Server

// berkasSeedDemo adalah berkas seed demo yang dibutuhkan tes spasial: akun demo dan neraca, rute, serta kandidat
// cold storage. Sengaja bukan seed.SeedDemo, yang menanam data demo semua modul (harga 30 hari, intervensi, dan
// seterusnya) yang tidak dipakai tes ini.
var berkasSeedDemo = []string{"demo/01_auth_users.sql", "demo/09_spasial_neraca.sql"}

// periodeSeed membaca periode neraca yang ditulis seed: awal bulan seed dijalankan (UTC), jadi nilainya berubah tiap
// bulan; tes menurunkan tanggal harapannya dari sini alih-alih menuliskan tanggal tetap.
func periodeSeed(t *testing.T, db *gorm.DB) string {
	t.Helper()
	var periode string
	require.NoError(t, db.Raw(`SELECT to_char(max(periode), 'YYYY-MM-DD') FROM neraca_wilayah`).Scan(&periode).Error)
	require.NotEmpty(t, periode, "neraca demo belum di-seed")
	return periode
}

func TestMain(m *testing.M) {
	gin.SetMode(gin.TestMode)
	os.Exit(pgtest.Main(m, &pg))
}

type suite struct {
	router *gin.Engine
	tokens map[string]string
	// periode adalah periode neraca demo (YYYY-MM-DD, awal bulan seed).
	periode string
}

var (
	once   sync.Once
	shared *suite
)

func sharedSuite(t *testing.T) *suite {
	t.Helper()
	once.Do(func() {
		db := pg.CreateDatabase(t, "spasial_shared")
		pgtest.MigrateUp(t, db)
		require.NoError(t, seed.SeedReference(t.Context(), db))
		require.NoError(t, seed.SeedFiles(t.Context(), db, berkasSeedDemo...))

		tokens, log := token.NewManager(jwtSecret), logger.NewDefault()
		router := gin.New()
		v1 := router.Group("/v1")
		authRoutes.Register(v1, &authRoutes.Dependencies{DB: db, Token: tokens, Logger: log})
		spasialRoutes.Register(v1, &spasialRoutes.Dependencies{DB: db, Token: tokens, Logger: log})
		shared = &suite{router: router, tokens: map[string]string{}, periode: periodeSeed(t, db)}
	})
	require.NotNil(t, shared, "suite bersama gagal dibuat")
	return shared
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

func data[T any](t *testing.T, w *httptest.ResponseRecorder) T {
	t.Helper()
	var res struct {
		Data T `json:"data"`
	}
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &res), w.Body.String())
	return res.Data
}

func TestSpasial_Authorization(t *testing.T) {
	s := sharedSuite(t)

	for _, path := range []string{"/v1/spasial/wilayah", "/v1/spasial/neraca", "/v1/spasial/rute", "/v1/spasial/kandidat-cold-storage"} {
		t.Run(path, func(t *testing.T) {
			assert.Equal(t, http.StatusUnauthorized, s.call(t, path, "").Code)
			assert.Equal(t, http.StatusForbidden, s.call(t, path, emailTanpaIzin).Code)
			assert.Equal(t, http.StatusOK, s.call(t, path, emailLihat).Code)
		})
	}
}

func TestSpasial_WilayahReturnsMarkersWithStatus(t *testing.T) {
	s := sharedSuite(t)

	type marker struct {
		Kode       string  `json:"kode"`
		Nama       string  `json:"nama"`
		Level      string  `json:"level"`
		Lat        float64 `json:"lat"`
		Lng        float64 `json:"lng"`
		Delta      float64 `json:"delta"`
		Komoditas  string  `json:"komoditas"`
		Status     string  `json:"status"`
		PunyaPasar bool    `json:"punya_pasar"`
	}
	got := data[[]marker](t, s.call(t, "/v1/spasial/wilayah", emailLihat))
	require.Len(t, got, 23)
	byKode := map[string]marker{}
	for _, m := range got {
		byKode[m.Kode] = m
		assert.True(t, m.PunyaPasar, "%s: seed master memberi tiap wilayah satu pasar", m.Kode)
		assert.NotZero(t, m.Lat)
	}

	assert.Equal(t, marker{Kode: "1171", Nama: "KOTA BANDA ACEH", Level: "kota", Lat: 5.55, Lng: 95.32, Delta: 21.4, Komoditas: "Cabai merah keriting", Status: "defisit", PunyaPasar: true}, byKode["1171"])
	assert.Equal(t, "waspada", byKode["1109"].Status, "bawang merah Pidie +9,4%")
	assert.Equal(t, "surplus", byKode["1106"].Status, "kentang Aceh Tengah -18%")
	assert.Equal(t, "Kentang", byKode["1106"].Komoditas, "komoditas dengan deviasi mutlak terbesar")
	assert.Equal(t, "surplus", byKode["1117"].Status)

	counts := map[string]int{}
	for _, m := range got {
		counts[m.Status]++
	}
	assert.NotZero(t, counts["defisit"])
	assert.NotZero(t, counts["waspada"])
	assert.NotZero(t, counts["surplus"])
	assert.NotZero(t, counts["normal"])
}

func TestSpasial_Neraca(t *testing.T) {
	s := sharedSuite(t)
	type row struct {
		KomoditasKode     string  `json:"komoditas_kode"`
		KomoditasNama     string  `json:"komoditas_nama"`
		KabupatenKode     string  `json:"kabupaten_kode"`
		Wilayah           string  `json:"wilayah"`
		Periode           string  `json:"periode"`
		SurplusDefisitTon float64 `json:"surplus_defisit_ton"`
		DeltaHargaPct     float64 `json:"delta_harga_pct"`
		Sumber            string  `json:"sumber"`
	}

	t.Run("periode terbaru, seluruh komoditas", func(t *testing.T) {
		got := data[[]row](t, s.call(t, "/v1/spasial/neraca", emailLihat))
		assert.Len(t, got, 8*23)
		for _, r := range got {
			assert.Equal(t, s.periode, r.Periode, "periode terbaru = periode neraca yang di-seed (awal bulan ini)")
			assert.Equal(t, "DEMO", r.Sumber)
		}
	})

	t.Run("filter komoditas", func(t *testing.T) {
		got := data[[]row](t, s.call(t, "/v1/spasial/neraca?komoditas=CBM-KRT", emailLihat))
		require.Len(t, got, 23)
		var banda row
		for _, r := range got {
			assert.Equal(t, "CBM-KRT", r.KomoditasKode)
			if r.KabupatenKode == "1171" {
				banda = r
			}
		}
		assert.Equal(t, 21.4, banda.DeltaHargaPct)
		assert.InDelta(t, -128.4, banda.SurplusDefisitTon, 0.01, "6 ton per poin x 21,4")
		assert.Equal(t, "KOTA BANDA ACEH", banda.Wilayah)
	})

	t.Run("periode tanpa data kosong, format salah ditolak", func(t *testing.T) {
		assert.Empty(t, data[[]row](t, s.call(t, "/v1/spasial/neraca?periode=2020-01-01", emailLihat)))
		assert.Equal(t, http.StatusBadRequest, s.call(t, "/v1/spasial/neraca?periode=kemarin", emailLihat).Code)
	})
}

func TestSpasial_RuteAndKandidat(t *testing.T) {
	s := sharedSuite(t)

	rute := data[[]struct {
		ID            string  `json:"id"`
		Kode          string  `json:"kode"`
		Asal          string  `json:"asal"`
		Tujuan        string  `json:"tujuan"`
		KomoditasNama string  `json:"komoditas_nama"`
		VolumeTon     float64 `json:"volume_ton"`
		JarakKm       int     `json:"jarak_km"`
		Biaya         int64   `json:"biaya"`
		Status        string  `json:"status"`
	}](t, s.call(t, "/v1/spasial/rute", emailLihat))
	require.Len(t, rute, 5)
	for i := 1; i < len(rute); i++ {
		assert.GreaterOrEqual(t, rute[i-1].Biaya, rute[i].Biaya, "terurut menurut biaya menurun")
	}
	kode := map[string]bool{}
	for _, r := range rute {
		kode[r.Kode] = true
		assert.Regexp(t, `^[1-9][0-9]*$`, r.ID, "ID BIGINT sebagai string desimal")
		assert.NotEmpty(t, r.Asal)
		assert.NotEmpty(t, r.KomoditasNama)
	}
	assert.True(t, kode["RT-1106-1171"], "kode = ID lama")

	kandidat := data[[]struct {
		ID           string             `json:"id"`
		Kode         string             `json:"kode"`
		Wilayah      string             `json:"wilayah"`
		Lokasi       string             `json:"lokasi"`
		Skor         int                `json:"skor"`
		KomponenSkor map[string]float64 `json:"komponen_skor"`
		Status       string             `json:"status"`
	}](t, s.call(t, "/v1/spasial/kandidat-cold-storage", emailLihat))
	require.Len(t, kandidat, 3)
	assert.Equal(t, "Kota Langsa", kandidat[0].Lokasi)
	assert.Equal(t, "KC-1173", kandidat[0].Kode)
	assert.Regexp(t, `^[1-9][0-9]*$`, kandidat[0].ID)
	assert.Equal(t, 82, kandidat[0].Skor)
	assert.Equal(t, "RENCANA", kandidat[0].Status)
	assert.Equal(t, "KOTA LANGSA", kandidat[0].Wilayah)
	assert.Len(t, kandidat[0].KomponenSkor, 3)
	assert.Greater(t, kandidat[0].Skor, kandidat[2].Skor, "terurut menurut skor")
}
