// Package integration menguji API EWS terhadap PostgreSQL sungguhan dengan seed tier reference (master, RBAC, aturan
// EWS, ...) ditambah demo/01_auth_users.sql dan demo/06_ews_alerts.sql saja. Tier demo utuh sengaja tidak dipakai:
// harga, cuaca, dan cold storage demo akan terbaca rule engine sehingga tes mesin tidak lagi terisolasi, dan
// alert/kartu modul lain ikut terhitung.
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
	ewsRoutes "github.com/garda-inflasi/api/internal/ews/routes"
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
	jwtSecret = "ews-integration-secret-key-32chars!!"

	// MANAJER_RISIKO: ews.lihat + ews.akui + ews.eskalasi.
	emailPenuh = "manajer.risiko@gardainflasi.id"
	// OPS_KOORDINATOR: ews.lihat + ews.akui, tanpa ews.eskalasi.
	emailAkui = "koordinator.lapangan@gardainflasi.id"
	// STRATEGIS_BI: hanya ews.lihat.
	emailLihat = "strategis.bi@gardainflasi.id"
	// OPS_MAHASISWA: tanpa izin EWS.
	emailTanpaIzin = "mahasiswa.1@gardainflasi.id"
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

// newSuite membuat basis data baru lengkap dengan seed dan router. Hanya akun demo dan alert EWS demo yang di-seed
// dari tier demo (lihat dokumentasi paket).
func newSuite(t *testing.T, name string) *suite {
	t.Helper()
	db := pg.CreateDatabase(t, name)
	pgtest.MigrateUp(t, db)
	require.NoError(t, seed.SeedReference(t.Context(), db))
	require.NoError(t, seed.SeedFiles(t.Context(), db, "demo/01_auth_users.sql", "demo/06_ews_alerts.sql"))

	tokens, log := token.NewManager(jwtSecret), logger.NewDefault()
	router := gin.New()
	v1 := router.Group("/v1")
	authRoutes.Register(v1, &authRoutes.Dependencies{DB: db, Token: tokens, Logger: log})
	ewsRoutes.Register(v1, &ewsRoutes.Dependencies{DB: db, Token: tokens, Logger: log})
	return &suite{db: db, router: router, tokens: map[string]string{}}
}

// sharedSuite dipakai tes yang hanya membaca.
func sharedSuite(t *testing.T) *suite {
	t.Helper()
	once.Do(func() { shared = newSuite(t, "ews_shared") })
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

func data[T any](t *testing.T, w *httptest.ResponseRecorder) T {
	t.Helper()
	var res struct {
		Data T `json:"data"`
	}
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &res), w.Body.String())
	return res.Data
}

type alertRow struct {
	ID            string  `json:"id"`
	Kode          *string `json:"kode"`
	Severity      string  `json:"severity"`
	Jenis         string  `json:"jenis"`
	KomoditasKode *string `json:"komoditas_kode"`
	KomoditasNama *string `json:"komoditas_nama"`
	KabupatenKode string  `json:"kabupaten_kode"`
	Wilayah       string  `json:"wilayah"`
	SatuanNilai   string  `json:"satuan_nilai"`
	Status        string  `json:"status"`
	AturanID      *string `json:"aturan_id"`
	AturanKode    *string `json:"aturan_kode"`
	DiakuiOlehID  *string `json:"diakui_oleh_id"`
}

// alertID membaca ID BIGINT (desimal) alert menurut kode tampilan lamanya (EWS-...).
func (s *suite) alertID(t *testing.T, kode string) string {
	t.Helper()
	var id string
	require.NoError(t, s.db.Raw(`SELECT id::text FROM ews_alert WHERE kode = ?`, kode).Scan(&id).Error)
	require.NotEmpty(t, id, "alert %s tidak ada", kode)
	return id
}

// aturanID membaca ID BIGINT (desimal) aturan menurut kode tampilan lamanya (ATR-...).
func (s *suite) aturanID(t *testing.T, kode string) string {
	t.Helper()
	var id string
	require.NoError(t, s.db.Raw(`SELECT id::text FROM aturan_ews WHERE kode = ?`, kode).Scan(&id).Error)
	require.NotEmpty(t, id, "aturan %s tidak ada", kode)
	return id
}

func TestEWS_Authorization(t *testing.T) {
	s := sharedSuite(t)

	for _, path := range []string{"/v1/ews", "/v1/ews/aturan", "/v1/ews/EWS-1000", "/v1/ews/" + s.alertID(t, "EWS-1000")} {
		t.Run(path, func(t *testing.T) {
			assert.Equal(t, http.StatusUnauthorized, s.call(t, http.MethodGet, path, "", nil).Code)
			assert.Equal(t, http.StatusForbidden, s.call(t, http.MethodGet, path, emailTanpaIzin, nil).Code)
			assert.Equal(t, http.StatusOK, s.call(t, http.MethodGet, path, emailLihat, nil).Code)
		})
	}

	t.Run("bulk menuntut ews.akui atau ews.eskalasi", func(t *testing.T) {
		body := map[string]any{"action": "akui", "ids": []string{s.alertID(t, "EWS-1000")}}
		assert.Equal(t, http.StatusUnauthorized, s.call(t, http.MethodPost, "/v1/ews/bulk", "", body).Code)
		assert.Equal(t, http.StatusForbidden, s.call(t, http.MethodPost, "/v1/ews/bulk", emailLihat, body).Code)
	})
}

func TestEWS_ListReturnsTheSeedWithNamesAndRules(t *testing.T) {
	s := sharedSuite(t)

	w := s.call(t, http.MethodGet, "/v1/ews?limit=200", emailLihat, nil)
	require.Equal(t, http.StatusOK, w.Code, w.Body.String())
	assert.Equal(t, "17", w.Header().Get("X-Total-Count"))

	got := data[[]alertRow](t, w)
	require.Len(t, got, 17)
	byKode := map[string]alertRow{}
	for _, a := range got {
		require.NotNil(t, a.Kode, "alert seed membawa kode tampilan")
		assert.Regexp(t, `^[1-9][0-9]*$`, a.ID, "ID alert adalah string desimal (BIGINT), bukan kode")
		byKode[*a.Kode] = a
	}

	first := byKode["EWS-1000"]
	assert.Equal(t, "KRITIS", first.Severity)
	assert.Equal(t, "KOTA BANDA ACEH", first.Wilayah)
	require.NotNil(t, first.KomoditasNama)
	assert.Equal(t, "Cabai merah keriting", *first.KomoditasNama)
	require.NotNil(t, first.AturanID)
	assert.Equal(t, s.aturanID(t, "ATR-LH-CBM-KRT"), *first.AturanID)
	require.NotNil(t, first.AturanKode)
	assert.Equal(t, "ATR-LH-CBM-KRT", *first.AturanKode)

	assert.Nil(t, byKode["EWS-0903"].KomoditasKode, "alert tanpa komoditas dikembalikan null")
	assert.Nil(t, byKode["EWS-0903"].AturanID)
	assert.Nil(t, byKode["EWS-0903"].AturanKode)
	assert.Equal(t, "mm", byKode["EWS-0901"].SatuanNilai)
	require.NotNil(t, got[0].Kode)
	assert.Equal(t, "EWS-1000", *got[0].Kode, "terbaru dahulu")
}

func TestEWS_ListFiltersAndPaginates(t *testing.T) {
	s := sharedSuite(t)

	t.Run("filter status dan severity", func(t *testing.T) {
		w := s.call(t, http.MethodGet, "/v1/ews?status=DIESKALASI", emailLihat, nil)
		require.Equal(t, http.StatusOK, w.Code)
		got := data[[]alertRow](t, w)
		assert.Len(t, got, 3)
		for _, a := range got {
			assert.Equal(t, "DIESKALASI", a.Status)
		}

		kritis := data[[]alertRow](t, s.call(t, http.MethodGet, "/v1/ews?severity=KRITIS&jenis=LONJAKAN_HARGA", emailLihat, nil))
		require.NotEmpty(t, kritis)
		for _, a := range kritis {
			assert.Equal(t, "KRITIS", a.Severity)
		}
	})

	t.Run("filter kabupaten", func(t *testing.T) {
		for _, a := range data[[]alertRow](t, s.call(t, http.MethodGet, "/v1/ews?kabupaten_kode=1171", emailLihat, nil)) {
			assert.Equal(t, "1171", a.KabupatenKode)
		}
	})

	t.Run("paginasi tidak menumpuk dan total tetap", func(t *testing.T) {
		p1 := s.call(t, http.MethodGet, "/v1/ews?limit=10&page=1", emailLihat, nil)
		p2 := s.call(t, http.MethodGet, "/v1/ews?limit=10&page=2", emailLihat, nil)
		a, b := data[[]alertRow](t, p1), data[[]alertRow](t, p2)
		assert.Len(t, a, 10)
		assert.Len(t, b, 7)
		assert.Equal(t, "17", p2.Header().Get("X-Total-Count"))
		for _, x := range b {
			for _, y := range a {
				assert.NotEqual(t, x.ID, y.ID)
			}
		}
	})

	t.Run("nilai filter tidak valid ditolak", func(t *testing.T) {
		assert.Equal(t, http.StatusBadRequest, s.call(t, http.MethodGet, "/v1/ews?status=ACAK", emailLihat, nil).Code)
		assert.Equal(t, http.StatusBadRequest, s.call(t, http.MethodGet, "/v1/ews?limit=5000", emailLihat, nil).Code)
	})
}

func TestEWS_GetByID(t *testing.T) {
	s := sharedSuite(t)

	id := s.alertID(t, "EWS-0901")

	got := data[alertRow](t, s.call(t, http.MethodGet, "/v1/ews/"+id, emailLihat, nil))
	assert.Equal(t, "ANOMALI_CUACA", got.Jenis)
	assert.Equal(t, "SIAGA", got.Severity)
	assert.Equal(t, id, got.ID)
	require.NotNil(t, got.Kode)
	assert.Equal(t, "EWS-0901", *got.Kode)

	t.Run("kode lama (tautan notifikasi dan email lama) tetap berlaku sebagai alias", func(t *testing.T) {
		alias := data[alertRow](t, s.call(t, http.MethodGet, "/v1/ews/EWS-0901", emailLihat, nil))
		assert.Equal(t, id, alias.ID, "alias menunjuk baris yang sama dan menjawab dengan ID baru")
	})

	t.Run("ID salah bentuk atau tidak ada dijawab 404", func(t *testing.T) {
		for _, bukanID := range []string{"TIDAK-ADA", "0", "-1", "01", "1.5", "99999999", "9223372036854775808", "EWS-1", "%20", "EWS-" + strings.Repeat("9", 80)} {
			assert.Equal(t, http.StatusNotFound, s.call(t, http.MethodGet, "/v1/ews/"+bukanID, emailLihat, nil).Code, bukanID)
			assert.Equal(t, http.StatusNotFound, s.call(t, http.MethodGet, "/v1/ews/"+bukanID+"/riwayat", emailLihat, nil).Code, bukanID)
		}
	})
}

func TestEWS_RiwayatByIDAndAlias(t *testing.T) {
	s := sharedSuite(t)
	type riwayat struct {
		ID string `json:"id"`
		Ke string `json:"ke"`
	}

	byID := data[[]riwayat](t, s.call(t, http.MethodGet, "/v1/ews/"+s.alertID(t, "EWS-1009")+"/riwayat", emailLihat, nil))
	byKode := data[[]riwayat](t, s.call(t, http.MethodGet, "/v1/ews/EWS-1009/riwayat", emailLihat, nil))

	require.Len(t, byID, 3, "dieskalasi: BARU, DIAKUI, DIESKALASI")
	assert.Equal(t, byID, byKode)
	for _, r := range byID {
		assert.Regexp(t, `^[1-9][0-9]*$`, r.ID, "ID riwayat string desimal")
	}
}

func TestEWS_AturanListsSeededThresholds(t *testing.T) {
	s := sharedSuite(t)

	got := data[[]struct {
		ID            string   `json:"id"`
		Kode          *string  `json:"kode"`
		KomoditasKode *string  `json:"komoditas_kode"`
		KomoditasNama *string  `json:"komoditas_nama"`
		Jenis         string   `json:"jenis"`
		AmbangWaspada float64  `json:"ambang_waspada"`
		AmbangSiaga   float64  `json:"ambang_siaga"`
		AmbangKritis  float64  `json:"ambang_kritis"`
		Kanal         []string `json:"kanal"`
		Aktif         bool     `json:"aktif"`
		Keterangan    string   `json:"keterangan"`
	}](t, s.call(t, http.MethodGet, "/v1/ews/aturan", emailLihat, nil))

	require.Len(t, got, 9)
	for _, a := range got {
		require.NotNil(t, a.Kode)
		assert.Regexp(t, `^[1-9][0-9]*$`, a.ID, "ID aturan string desimal")
		assert.Regexp(t, `^ATR-`, *a.Kode)
		assert.Less(t, a.AmbangWaspada, a.AmbangSiaga, *a.Kode)
		assert.Less(t, a.AmbangSiaga, a.AmbangKritis, *a.Kode)
		assert.Equal(t, []string{"DASHBOARD", "PUSH"}, a.Kanal, *a.Kode)
		assert.True(t, a.Aktif)
		assert.NotEmpty(t, a.Keterangan)
		require.NotNil(t, a.KomoditasNama)
	}
}

func TestEWS_BulkTransitionsAndAudit(t *testing.T) {
	s := newSuite(t, "ews_bulk")

	type result struct {
		Berhasil int `json:"berhasil"`
		Gagal    int `json:"gagal"`
	}
	// Bulk menerima ID desimal; kode tampilan diterjemahkan ke ID dulu.
	bulk := func(email, action string, kodeAtauID ...string) (*httptest.ResponseRecorder, result) {
		ids := make([]string, len(kodeAtauID))
		for i, k := range kodeAtauID {
			ids[i] = k
			if strings.HasPrefix(k, "EWS-") {
				ids[i] = s.alertID(t, k)
			}
		}
		w := s.call(t, http.MethodPost, "/v1/ews/bulk", email, map[string]any{"action": action, "ids": ids})
		if w.Code != http.StatusOK {
			return w, result{}
		}
		return w, data[result](t, w)
	}
	status := func(kode string) alertRow {
		return data[alertRow](t, s.call(t, http.MethodGet, "/v1/ews/"+kode, emailLihat, nil))
	}

	t.Run("akui hanya alert BARU; yang lain dihitung gagal", func(t *testing.T) {
		// EWS-1000 BARU, EWS-1004 sudah DIAKUI, 999999 tidak ada.
		w, res := bulk(emailAkui, "akui", "EWS-1000", "EWS-1004", "999999", "EWS-1000")
		require.Equal(t, http.StatusOK, w.Code, w.Body.String())
		assert.Equal(t, result{Berhasil: 1, Gagal: 2}, res)

		a := status("EWS-1000")
		assert.Equal(t, "DIAKUI", a.Status)
		require.NotNil(t, a.DiakuiOlehID)
		assert.Equal(t, uid("koordinator_lapangan"), *a.DiakuiOlehID)
	})

	t.Run("eskalasi menuntut izin eskalasi", func(t *testing.T) {
		w, _ := bulk(emailAkui, "eskalasi", "EWS-1001")
		assert.Equal(t, http.StatusForbidden, w.Code)
		assert.Equal(t, "BARU", status("EWS-1001").Status, "tidak berubah bila ditolak")

		w, res := bulk(emailPenuh, "eskalasi", "EWS-1001")
		require.Equal(t, http.StatusOK, w.Code)
		assert.Equal(t, 1, res.Berhasil)
		assert.Equal(t, "DIESKALASI", status("EWS-1001").Status)
	})

	t.Run("selesai menutup alert dan tidak bisa diulang", func(t *testing.T) {
		_, res := bulk(emailPenuh, "selesai", "EWS-1001", "EWS-1002")
		assert.Equal(t, 2, res.Berhasil)
		assert.Equal(t, "SELESAI", status("EWS-1002").Status)

		_, res = bulk(emailPenuh, "selesai", "EWS-1001")
		assert.Equal(t, result{Gagal: 1}, res, "alert SELESAI tidak berubah lagi")
	})

	t.Run("masukan tidak valid ditolak", func(t *testing.T) {
		w, _ := bulk(emailPenuh, "hapus", "EWS-1003")
		assert.Equal(t, http.StatusBadRequest, w.Code)
		w, _ = bulk(emailPenuh, "akui")
		assert.Equal(t, http.StatusBadRequest, w.Code)
	})

	t.Run("ID di body harus desimal: kode lama dan bentuk salah ditolak 400, angka JSON diterima", func(t *testing.T) {
		for name, ids := range map[string]any{
			"kode lama": []string{"EWS-1003"}, "teks": []string{"abc"}, "nol": []string{"0"}, "negatif": []int{-1},
			"desimal": []string{"1.5"}, "campuran": []any{s.alertID(t, "EWS-1003"), "EWS-1003"}, "kosong": []string{""},
		} {
			w := s.call(t, http.MethodPost, "/v1/ews/bulk", emailPenuh, map[string]any{"action": "akui", "ids": ids})
			assert.Equal(t, http.StatusBadRequest, w.Code, name+": "+w.Body.String())
		}
		assert.Equal(t, "BARU", status("EWS-1003").Status, "permintaan yang ditolak tidak mengubah apa pun")

		angka, err := strconv.ParseInt(s.alertID(t, "EWS-1003"), 10, 64)
		require.NoError(t, err)
		w := s.call(t, http.MethodPost, "/v1/ews/bulk", emailPenuh, map[string]any{"action": "akui", "ids": []int64{angka}})
		require.Equal(t, http.StatusOK, w.Code, w.Body.String())
		assert.Equal(t, "DIAKUI", status("EWS-1003").Status)
	})

	t.Run("setiap aksi yang mengubah data tercatat di audit", func(t *testing.T) {
		var rows []struct {
			Aksi      string
			EntitasID string
			AktorID   string
		}
		require.NoError(t, s.db.Raw(`SELECT aksi, entitas_id, aktor_id FROM audit_log WHERE entitas = 'ews_alert' ORDER BY waktu`).Scan(&rows).Error)
		require.Len(t, rows, 4, "akui, eskalasi, dua selesai, dan akui lewat angka JSON; aksi tanpa perubahan tidak dicatat")
		assert.Equal(t, "BULK_AKUI", rows[0].Aksi)
		assert.Equal(t, "1 alert", rows[0].EntitasID)
		assert.Equal(t, "BULK_ESKALASI", rows[1].Aksi)
		assert.Equal(t, "BULK_SELESAI", rows[2].Aksi)
		assert.Equal(t, "2 alert", rows[2].EntitasID)
		assert.Equal(t, "BULK_AKUI", rows[3].Aksi)
	})
}
