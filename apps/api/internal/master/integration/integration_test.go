// Package integration menguji API master terhadap PostgreSQL sungguhan dengan
// seed master.reference dan akun demo.
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

	authRoutes "github.com/garda-inflasi/api/internal/auth/routes"
	"github.com/garda-inflasi/api/internal/master"
	masterRoutes "github.com/garda-inflasi/api/internal/master/routes"
	"github.com/garda-inflasi/api/internal/platform/pgtest"
	"github.com/garda-inflasi/api/internal/seed"
	"github.com/garda-inflasi/api/pkg/idx"
	"github.com/garda-inflasi/api/pkg/logger"
	"github.com/garda-inflasi/api/pkg/token"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

const (
	jwtSecret = "master-integration-secret-key-32chars!"

	// ADMIN_SISTEM dan MANAJER_DATA: master.lihat + master.ajukan; STRATEGIS_BPS dan STRATEGIS_FMIPA:
	// master.lihat + master.setujui; OPS_KOORDINATOR: hanya master.lihat; OPS_MAHASISWA: tidak ada.
	emailAjukan   = "superadmin@gardainflasi.id"
	emailManajer  = "manajer.data@gardainflasi.id"
	emailSetujui  = "strategis.bps@gardainflasi.id"
	emailFMIPA    = "strategis.fmipa@gardainflasi.id"
	emailLihat    = "koordinator.lapangan@gardainflasi.id"
	emailTanpaIzn = "mahasiswa.1@gardainflasi.id"
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
		db := pg.CreateDatabase(t, "master_shared")
		pgtest.MigrateUp(t, db)
		require.NoError(t, seed.SeedReference(t.Context(), db))
		require.NoError(t, seed.SeedDemo(t.Context(), db))

		tokens, log := token.NewManager(jwtSecret), logger.NewDefault()
		router := gin.New()
		v1 := router.Group("/v1")
		authRoutes.Register(v1, &authRoutes.Dependencies{DB: db, Token: tokens, Logger: log})
		masterRoutes.Register(v1, &masterRoutes.Dependencies{DB: db, Token: tokens, Logger: log})
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

func rows[T any](t *testing.T, w *httptest.ResponseRecorder) []T {
	t.Helper()
	var res struct {
		Data []T `json:"data"`
	}
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &res), w.Body.String())
	return res.Data
}

type komoditasRow struct {
	Kode       string `json:"kode"`
	Nama       string `json:"nama"`
	HargaAcuan int    `json:"harga_acuan"`
	Pilot      bool   `json:"pilot"`
	Versi      int    `json:"versi"`
	Status     string `json:"status"`
}

func TestMaster_Authorization(t *testing.T) {
	s := sharedSuite(t)

	for _, path := range []string{"/v1/master/komoditas", "/v1/master/wilayah", "/v1/master/pasar"} {
		t.Run(path, func(t *testing.T) {
			assert.Equal(t, http.StatusUnauthorized, s.call(t, http.MethodGet, path, "", nil).Code)
			assert.Equal(t, http.StatusForbidden, s.call(t, http.MethodGet, path, emailTanpaIzn, nil).Code)
			assert.Equal(t, http.StatusOK, s.call(t, http.MethodGet, path, emailLihat, nil).Code)
		})
	}
}

func TestMaster_ListsContainTheWholeSeed(t *testing.T) {
	s := sharedSuite(t)

	t.Run("komoditas", func(t *testing.T) {
		got := rows[komoditasRow](t, s.call(t, http.MethodGet, "/v1/master/komoditas", emailLihat, nil))
		byKode := map[string]komoditasRow{}
		for _, k := range got {
			byKode[k.Kode] = k
		}
		for _, want := range master.Komoditas() {
			row, ok := byKode[want.Kode]
			require.True(t, ok, "komoditas %s tidak dikembalikan", want.Kode)
			assert.Equal(t, want.Nama, row.Nama)
			assert.Equal(t, want.HargaAcuan, row.HargaAcuan)
			assert.Equal(t, want.Pilot, row.Pilot)
			assert.Equal(t, 1, row.Versi)
			assert.Equal(t, "AKTIF", row.Status)
		}
	})

	t.Run("wilayah", func(t *testing.T) {
		w := s.call(t, http.MethodGet, "/v1/master/wilayah?provinsi_kode=11", emailLihat, nil)
		got := rows[struct {
			Kode  string `json:"kode"`
			Jenis string `json:"jenis"`
		}](t, w)
		kode := map[string]bool{}
		for _, k := range got {
			kode[k.Kode] = true
		}
		for _, want := range master.Kabupaten() {
			assert.True(t, kode[want.Kode], "wilayah %s tidak dikembalikan", want.Kode)
		}
	})

	t.Run("pasar menurut kabupaten", func(t *testing.T) {
		got := rows[struct {
			ID            idx.ID `json:"id"`
			Kode          string `json:"kode"`
			KabupatenKode string `json:"kabupaten_kode"`
			RadiusMeter   int    `json:"radius_meter"`
		}](t, s.call(t, http.MethodGet, "/v1/master/pasar?kabupaten_kode=1171", emailLihat, nil))
		var kodes []string
		for _, p := range got {
			assert.Equal(t, "1171", p.KabupatenKode)
			assert.Positive(t, p.RadiusMeter)
			assert.Positive(t, p.ID.Int64())
			kodes = append(kodes, p.Kode)
		}
		assert.Contains(t, kodes, "PSR-1171-01")
		assert.Contains(t, kodes, "PSR-1171-02")
	})
}

func TestMaster_SearchAndPagination(t *testing.T) {
	s := sharedSuite(t)

	t.Run("pencarian menurut nama tanpa peka huruf besar-kecil", func(t *testing.T) {
		got := rows[komoditasRow](t, s.call(t, http.MethodGet, "/v1/master/komoditas?q=CABAI", emailLihat, nil))

		assert.Subset(t, kodes(got), []string{"CBM-KRT", "CBR-RWT"})
		for _, k := range got {
			assert.Contains(t, strings.ToLower(k.Nama), "cabai")
		}
	})

	t.Run("pencarian menurut kode memberi total di header", func(t *testing.T) {
		w := s.call(t, http.MethodGet, "/v1/master/komoditas?q=cbm-krt", emailLihat, nil)

		assert.Equal(t, []string{"CBM-KRT"}, kodes(rows[komoditasRow](t, w)))
		assert.Equal(t, "1", w.Header().Get("X-Total-Count"))
	})

	t.Run("wildcard dianggap literal", func(t *testing.T) {
		got := rows[komoditasRow](t, s.call(t, http.MethodGet, "/v1/master/komoditas?q=%25", emailLihat, nil))
		assert.Empty(t, got)
	})

	t.Run("paginasi menelusuri semua baris tanpa dobel", func(t *testing.T) {
		all := rows[komoditasRow](t, s.call(t, http.MethodGet, "/v1/master/komoditas", emailLihat, nil))
		seen := map[string]bool{}
		for page := 1; page <= len(all); page++ {
			w := s.call(t, http.MethodGet, "/v1/master/komoditas?limit=4&page="+strconv.Itoa(page), emailLihat, nil)
			require.Equal(t, strconv.Itoa(len(all)), w.Header().Get("X-Total-Count"))
			chunk := rows[komoditasRow](t, w)
			if len(chunk) == 0 {
				break
			}
			assert.LessOrEqual(t, len(chunk), 4)
			for _, k := range chunk {
				assert.False(t, seen[k.Kode], "dobel antar halaman: %s", k.Kode)
				seen[k.Kode] = true
			}
		}
		assert.Len(t, seen, len(all))
	})

	t.Run("parameter tidak valid ditolak", func(t *testing.T) {
		for _, query := range []string{"?page=-1&limit=5", "?limit=-1", "?limit=9999", "?page=abc"} {
			assert.Equal(t, http.StatusBadRequest, s.call(t, http.MethodGet, "/v1/master/komoditas"+query, emailLihat, nil).Code, query)
		}
	})
}

func kodes(rows []komoditasRow) []string {
	out := make([]string, len(rows))
	for i, r := range rows {
		out[i] = r.Kode
	}
	return out
}

type rincianBulk struct {
	Kode     string `json:"kode"`
	Berhasil bool   `json:"berhasil"`
	Pesan    string `json:"pesan"`
}

type hasilBulk struct {
	Berhasil int           `json:"berhasil"`
	Gagal    int           `json:"gagal"`
	Rincian  []rincianBulk `json:"rincian"`
}

func decodeBulk(t *testing.T, w *httptest.ResponseRecorder) hasilBulk {
	t.Helper()
	var res struct {
		Data hasilBulk `json:"data"`
	}
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &res), w.Body.String())
	return res.Data
}

func TestMaster_BulkKomoditas(t *testing.T) {
	s := sharedSuite(t)
	const path = "/v1/master/komoditas/bulk"
	// GLA dan MYK hanya disentuh tes ini; keduanya dikembalikan ke AKTIF v1 di akhir.
	t.Cleanup(func() {
		require.NoError(t, s.db.Exec(`UPDATE komoditas SET status = 'AKTIF', versi = 1 WHERE kode IN ('GLA', 'MYK')`).Error)
	})
	body := func(action string, kode ...string) map[string]any {
		return map[string]any{"action": action, "kode": kode}
	}
	statusOf := func(kode string) string {
		var status string
		require.NoError(t, s.db.Raw(`SELECT status::text FROM komoditas WHERE kode = ?`, kode).Scan(&status).Error)
		return status
	}
	versiOf := func(kode string) int {
		var versi int
		require.NoError(t, s.db.Raw(`SELECT versi FROM komoditas WHERE kode = ?`, kode).Scan(&versi).Error)
		return versi
	}
	auditCount := func(aksi, aktor string) int64 {
		var n int64
		require.NoError(t, s.db.Raw(`SELECT COUNT(*) FROM audit_log WHERE aksi = ? AND aktor_id = ? AND entitas = 'komoditas'`, aksi, aktor).Scan(&n).Error)
		return n
	}

	t.Run("izin per aksi", func(t *testing.T) {
		// Hanya melihat: tidak boleh apa pun (ditolak di rute).
		for _, action := range []string{"ajukan_perubahan", "setujui", "arsipkan"} {
			assert.Equal(t, http.StatusForbidden, s.call(t, http.MethodPost, path, emailLihat, body(action, "GLA")).Code, action)
		}
		// Pengaju (master.ajukan) tidak boleh menyetujui.
		assert.Equal(t, http.StatusForbidden, s.call(t, http.MethodPost, path, emailManajer, body("setujui", "GLA")).Code)
		assert.Equal(t, http.StatusForbidden, s.call(t, http.MethodPost, path, emailAjukan, body("setujui", "GLA")).Code)
		// Penyetuju (master.setujui) tidak boleh mengajukan atau mengarsipkan.
		for _, email := range []string{emailSetujui, emailFMIPA} {
			assert.Equal(t, http.StatusForbidden, s.call(t, http.MethodPost, path, email, body("ajukan_perubahan", "GLA")).Code)
			assert.Equal(t, http.StatusForbidden, s.call(t, http.MethodPost, path, email, body("arsipkan", "GLA")).Code)
		}
		assert.Equal(t, "AKTIF", statusOf("GLA"), "penolakan izin tidak mengubah data")
		assert.Equal(t, http.StatusUnauthorized, s.call(t, http.MethodPost, path, "", body("arsipkan", "GLA")).Code)
	})

	t.Run("ajukan lalu setujui menaikkan versi dan tercatat di audit", func(t *testing.T) {
		before := auditCount("BULK_ajukan_perubahan", uid("manajer_data"))
		w := s.call(t, http.MethodPost, path, emailManajer, body("ajukan_perubahan", "GLA", "MYK"))
		require.Equal(t, http.StatusOK, w.Code, w.Body.String())
		hasil := decodeBulk(t, w)
		assert.Equal(t, 2, hasil.Berhasil)
		assert.Zero(t, hasil.Gagal)
		assert.Equal(t, "MENUNGGU_PERSETUJUAN", statusOf("GLA"))
		assert.Equal(t, 1, versiOf("GLA"), "versi naik saat disetujui, bukan saat diajukan")
		assert.Equal(t, before+1, auditCount("BULK_ajukan_perubahan", uid("manajer_data")))

		beforeSetuju := auditCount("BULK_setujui", uid("strategis_bps"))
		w = s.call(t, http.MethodPost, path, emailSetujui, body("setujui", "GLA", "MYK"))
		require.Equal(t, http.StatusOK, w.Code, w.Body.String())
		assert.Equal(t, 2, decodeBulk(t, w).Berhasil)
		assert.Equal(t, "AKTIF", statusOf("GLA"))
		assert.Equal(t, 2, versiOf("GLA"))
		assert.Equal(t, 2, versiOf("MYK"))
		assert.Equal(t, beforeSetuju+1, auditCount("BULK_setujui", uid("strategis_bps")))
	})

	t.Run("penyetuju kedua juga boleh (FMIPA)", func(t *testing.T) {
		require.Equal(t, http.StatusOK, s.call(t, http.MethodPost, path, emailManajer, body("ajukan_perubahan", "GLA")).Code)
		w := s.call(t, http.MethodPost, path, emailFMIPA, body("setujui", "GLA"))
		require.Equal(t, http.StatusOK, w.Code, w.Body.String())
		assert.Equal(t, 1, decodeBulk(t, w).Berhasil)
		assert.Equal(t, 3, versiOf("GLA"))
	})

	t.Run("arsipkan dari AKTIF dan dari MENUNGGU_PERSETUJUAN", func(t *testing.T) {
		require.Equal(t, http.StatusOK, s.call(t, http.MethodPost, path, emailAjukan, body("ajukan_perubahan", "MYK")).Code)
		w := s.call(t, http.MethodPost, path, emailAjukan, body("arsipkan", "GLA", "MYK"))
		require.Equal(t, http.StatusOK, w.Code, w.Body.String())
		assert.Equal(t, 2, decodeBulk(t, w).Berhasil)
		assert.Equal(t, "DIARSIPKAN", statusOf("GLA"))
		assert.Equal(t, "DIARSIPKAN", statusOf("MYK"))
		assert.EqualValues(t, 1, auditCount("BULK_arsipkan", uid("superadmin")))
	})

	t.Run("transisi tidak valid dan kode tak dikenal menjadi item gagal", func(t *testing.T) {
		// GLA dan MYK kini DIARSIPKAN; BRS-MED AKTIF.
		w := s.call(t, http.MethodPost, path, emailManajer, body("ajukan_perubahan", "BRS-MED", "GLA", "NGAWUR"))
		require.Equal(t, http.StatusOK, w.Code, w.Body.String())
		hasil := decodeBulk(t, w)
		assert.Equal(t, 1, hasil.Berhasil)
		assert.Equal(t, 2, hasil.Gagal)
		byKode := map[string]rincianBulk{}
		for _, r := range hasil.Rincian {
			byKode[r.Kode] = r
		}
		assert.True(t, byKode["BRS-MED"].Berhasil)
		assert.False(t, byKode["GLA"].Berhasil)
		assert.Contains(t, byKode["GLA"].Pesan, "DIARSIPKAN")
		assert.False(t, byKode["NGAWUR"].Berhasil)
		assert.Contains(t, byKode["NGAWUR"].Pesan, "tidak ditemukan")
		assert.Equal(t, "DIARSIPKAN", statusOf("GLA"), "item gagal tidak berubah")

		// Setujui hanya berlaku untuk yang menunggu: BRS-MED menunggu, GLA arsip.
		hasil = decodeBulk(t, s.call(t, http.MethodPost, path, emailSetujui, body("setujui", "GLA", "BRS-MED")))
		assert.Equal(t, 1, hasil.Berhasil)
		assert.Equal(t, 1, hasil.Gagal)
		assert.Equal(t, "AKTIF", statusOf("BRS-MED"))
		assert.Equal(t, 2, versiOf("BRS-MED"))
		t.Cleanup(func() {
			require.NoError(t, s.db.Exec(`UPDATE komoditas SET status = 'AKTIF', versi = 1 WHERE kode = 'BRS-MED'`).Error)
		})

		// Mengajukan ulang item yang sudah menunggu ditolak, bukan diam-diam dianggap berhasil.
		require.Equal(t, http.StatusOK, s.call(t, http.MethodPost, path, emailManajer, body("ajukan_perubahan", "MYK")).Code)
		hasil = decodeBulk(t, s.call(t, http.MethodPost, path, emailManajer, body("ajukan_perubahan", "MYK")))
		assert.Equal(t, 0, hasil.Berhasil)
		assert.Equal(t, 1, hasil.Gagal)
	})

	t.Run("kode ganda diproses sekali", func(t *testing.T) {
		require.NoError(t, s.db.Exec(`UPDATE komoditas SET status = 'AKTIF', versi = 1 WHERE kode = 'GLA'`).Error)
		hasil := decodeBulk(t, s.call(t, http.MethodPost, path, emailManajer, body("ajukan_perubahan", "GLA", "GLA")))
		assert.Equal(t, 1, hasil.Berhasil)
		assert.Zero(t, hasil.Gagal)
		assert.Len(t, hasil.Rincian, 1)
	})

	t.Run("aksi tidak dikenal dan badan tidak valid", func(t *testing.T) {
		assert.Equal(t, http.StatusBadRequest, s.call(t, http.MethodPost, path, emailAjukan, body("hapus", "GLA")).Code)
		assert.Equal(t, http.StatusBadRequest, s.call(t, http.MethodPost, path, emailSetujui, body("hapus", "GLA")).Code)
		assert.Equal(t, http.StatusBadRequest, s.call(t, http.MethodPost, path, emailAjukan, map[string]any{"action": "arsipkan", "kode": []string{}}).Code)
	})
}
