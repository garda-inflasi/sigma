// Package integration menguji API intervensi terhadap PostgreSQL sungguhan
// dengan seed master.reference, auth.demo, ews.*, dan intervensi.*.
package integration

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"sync"
	"testing"
	"time"

	authRoutes "github.com/garda-inflasi/api/internal/auth/routes"
	authseeder "github.com/garda-inflasi/api/internal/auth/seeder"
	ewsseeder "github.com/garda-inflasi/api/internal/ews/seeder"
	intervensiRoutes "github.com/garda-inflasi/api/internal/intervensi/routes"
	intervensiseeder "github.com/garda-inflasi/api/internal/intervensi/seeder"
	masterseeder "github.com/garda-inflasi/api/internal/master/seeder"
	"github.com/garda-inflasi/api/internal/platform/pgtest"
	"github.com/garda-inflasi/api/pkg/logger"
	"github.com/garda-inflasi/api/pkg/token"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

const (
	jwtSecret = "intervensi-integration-secret-32chars!"

	// MANAJER_PROGRAM: intervensi.lihat + intervensi.kelola.
	emailKelola = "manajer.program@gardainflasi.id"
	// STRATEGIS_BI: intervensi.lihat + intervensi.setujui (tanpa kelola).
	emailSetuju = "strategis.bi@gardainflasi.id"
	// STRATEGIS_DJPB: setujui + setujui_fiskal (tanpa kelola).
	emailFiskal = "strategis.djpb@gardainflasi.id"
	// MANAJER_RISIKO: hanya intervensi.lihat.
	emailLihat = "manajer.risiko@gardainflasi.id"
	// OPS_MAHASISWA: tanpa izin intervensi.
	emailTanpaIzin = "mahasiswa.1@gardainflasi.id"
)

var (
	pg      *pgtest.Server
	seedNow = time.Now().UTC()
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

func newSuite(t *testing.T, name string) *suite {
	t.Helper()
	db := pg.CreateDatabase(t, name)
	pgtest.MigrateUp(t, db)
	ctx := t.Context()
	_, err := masterseeder.SeedReference(ctx, db)
	require.NoError(t, err)
	_, err = authseeder.SeedDemo(ctx, db)
	require.NoError(t, err)
	_, err = ewsseeder.SeedReference(ctx, db)
	require.NoError(t, err)
	_, err = ewsseeder.SeedDemo(ctx, db, seedNow)
	require.NoError(t, err)
	_, err = intervensiseeder.SeedReference(ctx, db)
	require.NoError(t, err)
	_, err = intervensiseeder.SeedDemo(ctx, db, seedNow)
	require.NoError(t, err)

	tokens, log := token.NewManager(jwtSecret), logger.NewDefault()
	router := gin.New()
	v1 := router.Group("/v1")
	authRoutes.Register(v1, &authRoutes.Dependencies{DB: db, Token: tokens, Logger: log})
	intervensiRoutes.Register(v1, &intervensiRoutes.Dependencies{DB: db, Token: tokens, Logger: log})
	return &suite{db: db, router: router, tokens: map[string]string{}}
}

func sharedSuite(t *testing.T) *suite {
	t.Helper()
	once.Do(func() { shared = newSuite(t, "intervensi_shared") })
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
	body, _ := json.Marshal(map[string]string{"email": email, "password": authseeder.DemoPassword})
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

type kartuRow struct {
	ID            string `json:"id"`
	Siklus        string `json:"siklus"`
	Tahap         string `json:"tahap"`
	Urutan        int    `json:"urutan"`
	Judul         string `json:"judul"`
	KomoditasKode string `json:"komoditas_kode"`
	KabupatenKode string `json:"kabupaten_kode"`
	Prioritas     string `json:"prioritas"`
	RekomendasiAI bool   `json:"rekomendasi_ai"`
	Progres       struct {
		Selesai int `json:"selesai"`
		Total   int `json:"total"`
	} `json:"progres"`
	EwsAlertID      *string `json:"ews_alert_id"`
	SelesaiPada     *string `json:"selesai_pada"`
	Versi           int     `json:"versi"`
	DieskalasiPada  *string `json:"dieskalasi_pada"`
	PenanggungJawab []struct {
		ID   string `json:"id"`
		Nama string `json:"nama"`
	} `json:"penanggung_jawab"`
}

func listCards(t *testing.T, s *suite, query string) []kartuRow {
	t.Helper()
	w := s.call(t, http.MethodGet, "/v1/intervensi"+query, emailLihat, nil)
	require.Equal(t, http.StatusOK, w.Code, w.Body.String())
	return data[[]kartuRow](t, w)
}

func card(t *testing.T, s *suite, id string) kartuRow {
	t.Helper()
	for _, k := range listCards(t, s, "") {
		if k.ID == id {
			return k
		}
	}
	require.FailNow(t, "kartu tidak ada", id)
	return kartuRow{}
}

// column mengembalikan id kartu satu tahap menurut urutan API.
func column(t *testing.T, s *suite, tahap string) []string {
	t.Helper()
	var ids []string
	for _, k := range listCards(t, s, "?tahap="+tahap) {
		ids = append(ids, k.ID)
	}
	return ids
}

func TestIntervensi_Authorization(t *testing.T) {
	s := sharedSuite(t)

	for _, path := range []string{"/v1/intervensi", "/v1/intervensi/tahap"} {
		t.Run(path, func(t *testing.T) {
			assert.Equal(t, http.StatusUnauthorized, s.call(t, http.MethodGet, path, "", nil).Code)
			assert.Equal(t, http.StatusForbidden, s.call(t, http.MethodGet, path, emailTanpaIzin, nil).Code)
			assert.Equal(t, http.StatusOK, s.call(t, http.MethodGet, path, emailLihat, nil).Code)
		})
	}

	t.Run("membuat kartu menuntut intervensi.kelola", func(t *testing.T) {
		body := map[string]any{}
		assert.Equal(t, http.StatusUnauthorized, s.call(t, http.MethodPost, "/v1/intervensi", "", body).Code)
		assert.Equal(t, http.StatusForbidden, s.call(t, http.MethodPost, "/v1/intervensi", emailLihat, body).Code)
		assert.Equal(t, http.StatusForbidden, s.call(t, http.MethodPost, "/v1/intervensi", emailSetuju, body).Code)
	})

	t.Run("memindahkan kartu menuntut salah satu izin kelola atau setujui", func(t *testing.T) {
		body := map[string]any{"tahap": "t-ews", "urutan": 0}
		assert.Equal(t, http.StatusUnauthorized, s.call(t, http.MethodPatch, "/v1/intervensi/INT-301/tahap", "", body).Code)
		assert.Equal(t, http.StatusForbidden, s.call(t, http.MethodPatch, "/v1/intervensi/INT-301/tahap", emailLihat, body).Code)
	})
}

func TestIntervensi_ListReturnsTheSeedShapedForTheBoard(t *testing.T) {
	s := sharedSuite(t)

	cards := listCards(t, s, "")
	require.Len(t, cards, 19)

	first := card(t, s, "INT-301")
	assert.Equal(t, "TEKANAN", first.Siklus)
	assert.Equal(t, "t-ews", first.Tahap)
	assert.Equal(t, "Lonjakan cabai merah Banda Aceh", first.Judul)
	assert.Equal(t, "CBM-KRT", first.KomoditasKode)
	assert.Equal(t, "1171", first.KabupatenKode)
	assert.Equal(t, "TINGGI", first.Prioritas)
	assert.True(t, first.RekomendasiAI)
	assert.Equal(t, 5, first.Progres.Total)
	require.NotNil(t, first.EwsAlertID)
	assert.Equal(t, "EWS-1000", *first.EwsAlertID)
	require.Len(t, first.PenanggungJawab, 1)
	assert.Equal(t, "manajer_program", first.PenanggungJawab[0].ID)
	assert.Equal(t, "Rina Safitri, S.T., M.T.", first.PenanggungJawab[0].Nama)

	assert.Len(t, card(t, s, "INT-302").PenanggungJawab, 2, "kartu genap-ganjil dengan dua penanggung jawab")

	// Kartu dikembalikan terurut per tahap menurut urutan.
	last := map[string]int{}
	for _, k := range cards {
		if prev, ok := last[k.Tahap]; ok {
			assert.Greater(t, k.Urutan, prev, "urutan kartu di %s harus menaik", k.Tahap)
		}
		last[k.Tahap] = k.Urutan
	}

	done := card(t, s, "INT-319")
	assert.NotNil(t, done.SelesaiPada)
}

func TestIntervensi_ListFilters(t *testing.T) {
	s := sharedSuite(t)

	assert.Len(t, listCards(t, s, "?siklus=SURPLUS"), 6)
	assert.Len(t, listCards(t, s, "?siklus=TEKANAN&prioritas=TINGGI"), 4)
	assert.Equal(t, []string{"INT-301", "INT-302"}, column(t, s, "t-ews"))
	assert.Equal(t, http.StatusBadRequest, s.call(t, http.MethodGet, "/v1/intervensi?siklus=ACAK", emailLihat, nil).Code)
}

func TestIntervensi_TahapDefinitions(t *testing.T) {
	s := sharedSuite(t)

	defs := data[[]struct {
		Siklus         string `json:"siklus"`
		KodeTahap      string `json:"kode_tahap"`
		Urutan         int    `json:"urutan"`
		Label          string `json:"label"`
		IzinDibutuhkan string `json:"izin_dibutuhkan"`
		SLAHari        *int   `json:"sla_hari"`
	}](t, s.call(t, http.MethodGet, "/v1/intervensi/tahap", emailLihat, nil))

	require.Len(t, defs, 16)
	byKode := map[string]int{}
	for i, d := range defs {
		byKode[d.KodeTahap] = i
	}
	petakan := defs[byKode["t-petakan"]]
	assert.Equal(t, "TEKANAN", petakan.Siklus)
	assert.Equal(t, "intervensi.setujui", petakan.IzinDibutuhkan)
	assert.Equal(t, "intervensi.setujui_fiskal", defs[byKode["t-fiskal"]].IzinDibutuhkan)
	require.NotNil(t, defs[byKode["t-ews"]].SLAHari)
	assert.Nil(t, defs[byKode["t-selesai"]].SLAHari)
}

func TestIntervensi_CreateAndAudit(t *testing.T) {
	s := newSuite(t, "intervensi_create")
	valid := func() map[string]any {
		return map[string]any{
			"judul": "Operasi pasar beras Banda Aceh", "deskripsi": "Gelontorkan 20 ton", "siklus": "TEKANAN", "prioritas": "TINGGI",
			"komoditas_kode": "BRS-MED", "kabupaten_kode": "1171", "tenggat": time.Now().Add(72 * time.Hour).UTC().Format(time.RFC3339),
			"ews_alert_id": "EWS-1008", "penanggung_jawab_ids": []string{"manajer_data", "manajer_data", "agregator_data"},
		}
	}

	t.Run("kartu baru masuk tahap pertama siklusnya di urutan paling bawah", func(t *testing.T) {
		w := s.call(t, http.MethodPost, "/v1/intervensi", emailKelola, valid())
		require.Equal(t, http.StatusCreated, w.Code, w.Body.String())
		created := data[struct {
			ID    string `json:"id"`
			Tahap string `json:"tahap"`
		}](t, w)
		assert.Equal(t, "INT-1000", created.ID)
		assert.Equal(t, "t-ews", created.Tahap)

		k := card(t, s, created.ID)
		assert.Equal(t, 2, k.Urutan, "setelah INT-301 dan INT-302")
		assert.Len(t, k.PenanggungJawab, 2, "penanggung jawab ganda dibuang")
		require.NotNil(t, k.EwsAlertID)
	})

	t.Run("nomor kartu berikutnya menaik", func(t *testing.T) {
		w := s.call(t, http.MethodPost, "/v1/intervensi", emailKelola, valid())
		require.Equal(t, http.StatusCreated, w.Code)
		assert.Equal(t, "INT-1001", data[struct {
			ID string `json:"id"`
		}](t, w).ID)
	})

	t.Run("rujukan yang tidak ada ditolak", func(t *testing.T) {
		for field, value := range map[string]any{
			"komoditas_kode": "TIDAK-ADA", "kabupaten_kode": "9999", "ews_alert_id": "EWS-TIDAK-ADA",
			"penanggung_jawab_ids": []string{"bukan_pengguna"},
		} {
			body := valid()
			body[field] = value
			assert.Equal(t, http.StatusBadRequest, s.call(t, http.MethodPost, "/v1/intervensi", emailKelola, body).Code, field)
		}
	})

	t.Run("masukan tidak valid ditolak", func(t *testing.T) {
		for field, value := range map[string]any{"siklus": "ACAK", "prioritas": "KRITIS", "judul": "", "tenggat": nil} {
			body := valid()
			body[field] = value
			assert.Equal(t, http.StatusBadRequest, s.call(t, http.MethodPost, "/v1/intervensi", emailKelola, body).Code, field)
		}
	})

	t.Run("pembuatan tercatat di audit", func(t *testing.T) {
		var n int64
		require.NoError(t, s.db.Table("audit_log").Where("entitas = 'intervensi' AND aksi = 'BUAT' AND hasil = 'BERHASIL'").Count(&n).Error)
		assert.EqualValues(t, 2, n, "hanya dua pembuatan yang berhasil")
	})
}

func TestIntervensi_MoveEnforcesTahapPermissionsAndKeepsOrder(t *testing.T) {
	s := newSuite(t, "intervensi_move")
	// versiTerkini memakai versi kartu saat ini (1 bila kartu tidak ada) seperti klien yang baru memuat papan.
	versiTerkini := func(id string) int {
		for _, k := range listCards(t, s, "") {
			if k.ID == id {
				return k.Versi
			}
		}
		return 1
	}
	move := func(email, id, tahap string, urutan int) *httptest.ResponseRecorder {
		return s.call(t, http.MethodPatch, "/v1/intervensi/"+id+"/tahap", email, map[string]any{"tahap": tahap, "urutan": urutan, "versi": versiTerkini(id)})
	}

	t.Run("pindah antartahap menyisipkan di posisi dan menomori ulang kolom", func(t *testing.T) {
		require.Equal(t, []string{"INT-303", "INT-306"}, column(t, s, "t-verifikasi"))

		require.Equal(t, http.StatusOK, move(emailKelola, "INT-301", "t-verifikasi", 1).Code)

		assert.Equal(t, []string{"INT-303", "INT-301", "INT-306"}, column(t, s, "t-verifikasi"))
		assert.Equal(t, []string{"INT-302"}, column(t, s, "t-ews"))
		for i, id := range column(t, s, "t-verifikasi") {
			assert.Equal(t, i, card(t, s, id).Urutan, "urutan harus berurutan dari 0")
		}
	})

	t.Run("urutan di luar rentang dijepit ke ujung kolom", func(t *testing.T) {
		require.Equal(t, http.StatusOK, move(emailKelola, "INT-302", "t-verifikasi", 99).Code)
		assert.Equal(t, []string{"INT-303", "INT-301", "INT-306", "INT-302"}, column(t, s, "t-verifikasi"))
	})

	t.Run("mengurutkan ulang dalam kolom yang sama", func(t *testing.T) {
		require.Equal(t, http.StatusOK, move(emailKelola, "INT-302", "t-verifikasi", 0).Code)
		assert.Equal(t, []string{"INT-302", "INT-303", "INT-301", "INT-306"}, column(t, s, "t-verifikasi"))
	})

	t.Run("tahap persetujuan ditolak bagi pemegang kelola saja", func(t *testing.T) {
		w := move(emailKelola, "INT-301", "t-petakan", 0)
		assert.Equal(t, http.StatusForbidden, w.Code, w.Body.String())
		assert.Contains(t, w.Body.String(), "intervensi.setujui")
		assert.Equal(t, "t-verifikasi", card(t, s, "INT-301").Tahap, "kartu tidak berpindah")
	})

	t.Run("penyetuju boleh ke tahap persetujuan tetapi tidak ke tahap biasa", func(t *testing.T) {
		assert.Equal(t, http.StatusOK, move(emailSetuju, "INT-301", "t-petakan", 0).Code)
		assert.Equal(t, http.StatusForbidden, move(emailSetuju, "INT-301", "t-logistik", 0).Code)
	})

	t.Run("dukungan fiskal hanya untuk penyetuju fiskal", func(t *testing.T) {
		assert.Equal(t, http.StatusForbidden, move(emailSetuju, "INT-301", "t-fiskal", 0).Code)
		assert.Equal(t, http.StatusOK, move(emailFiskal, "INT-301", "t-fiskal", 0).Code)
	})

	t.Run("tahap penutup mengisi selesai_pada dan meninggalkannya mengosongkan", func(t *testing.T) {
		assert.Nil(t, card(t, s, "INT-301").SelesaiPada)
		require.Equal(t, http.StatusOK, move(emailSetuju, "INT-301", "t-selesai", 0).Code)
		assert.NotNil(t, card(t, s, "INT-301").SelesaiPada)

		require.Equal(t, http.StatusOK, move(emailKelola, "INT-301", "t-ews", 0).Code)
		assert.Nil(t, card(t, s, "INT-301").SelesaiPada)
	})

	t.Run("tahap milik siklus lain, kartu tak ada, dan masukan salah", func(t *testing.T) {
		assert.Equal(t, http.StatusBadRequest, move(emailKelola, "INT-301", "n-pantau", 0).Code)
		assert.Equal(t, http.StatusBadRequest, move(emailKelola, "INT-301", "tidak-ada", 0).Code)
		assert.Equal(t, http.StatusNotFound, move(emailKelola, "INT-9999", "t-ews", 0).Code)
		assert.Equal(t, http.StatusBadRequest, s.call(t, http.MethodPatch, "/v1/intervensi/INT-301/tahap", emailKelola, map[string]any{"urutan": 0}).Code)
		assert.Equal(t, http.StatusBadRequest, move(emailKelola, "INT-301", "t-ews", -1).Code)
	})

	t.Run("pemindahan dan penolakan tercatat di audit", func(t *testing.T) {
		var rows []struct {
			Hasil string
			Aksi  string
		}
		require.NoError(t, s.db.Raw(`SELECT hasil, aksi FROM audit_log WHERE entitas = 'intervensi' ORDER BY waktu`).Scan(&rows).Error)
		require.NotEmpty(t, rows)
		var berhasil, ditolak int
		for _, r := range rows {
			assert.Equal(t, "PINDAH_TAHAP", r.Aksi)
			switch r.Hasil {
			case "BERHASIL":
				berhasil++
			case "DITOLAK":
				ditolak++
			}
		}
		assert.Equal(t, 7, berhasil)
		assert.Equal(t, 3, ditolak)
	})
}
