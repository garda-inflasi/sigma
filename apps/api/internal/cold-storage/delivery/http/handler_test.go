package http

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"
	"time"

	"github.com/garda-inflasi/api/internal/cold-storage/domain"
	"github.com/garda-inflasi/api/internal/cold-storage/usecase"
	"github.com/garda-inflasi/api/pkg/logger"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// ── Fake repository ───────────────────────────────────────────────────

type fakeRepo struct {
	fasilitas []domain.FasilitasView
	stok      domain.StokPage
	ringkasan domain.RingkasanStok
	bulk      domain.BulkResult
	err       error

	gotQuery  domain.StokQuery
	gotIDs    []string
	gotAction domain.BulkAction
	gotAudit  *domain.AuditLog
}

func (f *fakeRepo) ListFasilitas(context.Context) ([]domain.FasilitasView, error) {
	return f.fasilitas, f.err
}

func (f *fakeRepo) ListStok(_ context.Context, q domain.StokQuery) (domain.StokPage, error) {
	f.gotQuery = q
	return f.stok, f.err
}

func (f *fakeRepo) RingkasanStok(context.Context, float64) (domain.RingkasanStok, error) {
	return f.ringkasan, f.err
}

func (f *fakeRepo) BulkAction(_ context.Context, ids []string, a domain.BulkAction) (domain.BulkResult, error) {
	f.gotIDs, f.gotAction = ids, a
	return f.bulk, f.err
}

func (f *fakeRepo) CreateAuditLog(_ context.Context, l *domain.AuditLog) error {
	f.gotAudit = l
	return nil
}

// ── Helpers ───────────────────────────────────────────────────────────

var now = time.Date(2026, 10, 16, 9, 0, 0, 0, time.UTC)

// newRouter memasang handler dengan identitas dan izin seperti yang diisi JWTAuth.
func newRouter(repo *fakeRepo, userID string, perms ...string) *gin.Engine {
	gin.SetMode(gin.TestMode)
	uc := usecase.New(repo, logger.NewDefault()).WithClock(func() time.Time { return now })
	h := New(uc)

	r := gin.New()
	r.Use(func(c *gin.Context) {
		c.Set("user_id", userID)
		c.Set("permissions", perms)
		c.Next()
	})
	r.GET("/cold-storage", h.GetAll)
	r.POST("/cold-storage/bulk", h.Bulk)
	r.GET("/cold-storage/health", h.Health)
	return r
}

func do(r *gin.Engine, method, target string, body any) *httptest.ResponseRecorder {
	var rd *bytes.Reader
	switch b := body.(type) {
	case nil:
		rd = bytes.NewReader(nil)
	case string:
		rd = bytes.NewReader([]byte(b))
	default:
		raw, _ := json.Marshal(b)
		rd = bytes.NewReader(raw)
	}
	req := httptest.NewRequest(method, target, rd)
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	return w
}

type envelope struct {
	Code    string          `json:"code"`
	Message string          `json:"message"`
	Data    json.RawMessage `json:"data"`
}

func decode[T any](t *testing.T, w *httptest.ResponseRecorder) T {
	t.Helper()
	var env envelope
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &env), w.Body.String())
	var out T
	require.NoError(t, json.Unmarshal(env.Data, &out), string(env.Data))
	return out
}

// ── GET /cold-storage ─────────────────────────────────────────────────

func TestGetAll_ResponseShape(t *testing.T) {
	aktual := -20.1
	repo := &fakeRepo{
		fasilitas: []domain.FasilitasView{
			{ID: "CS-02", Nama: "Cold Storage Perikanan Lhokseumawe", KabupatenKode: "1173", KabupatenNama: "KOTA LHOKSEUMAWE",
				Lat: 5.17, Lng: 97.13, KapasitasTon: 300, TerisiTon: 240, SuhuTargetC: -20, SuhuAktualC: &aktual, Status: "OPERASIONAL"},
			{ID: "CS-05", Nama: "Gudang Dingin Langsa", KabupatenKode: "1174", KabupatenNama: "KOTA LANGSA", KapasitasTon: 120, SuhuTargetC: 4, Status: "RENCANA"},
		},
		stok: domain.StokPage{
			Items: []domain.StokView{{
				ID: "STK-1", ColdStorageID: "CS-02", ColdStorageNama: "Cold Storage Perikanan Lhokseumawe",
				KomoditasKode: "ikan_tongkol", KomoditasNama: "Ikan Tongkol", VolumeTon: 64.5,
				MasukPada: now.Add(-101 * 24 * time.Hour), KedaluwarsaPada: now.Add(19 * 24 * time.Hour),
				RisikoFoodLoss: 0.72, Pemilik: "CV Mina Lhokseumawe",
			}},
			Total: 23, TotalVolumeTon: 812,
		},
		ringkasan: domain.RingkasanStok{TotalLot: 23, TotalVolumeTon: 812, RisikoTinggiLot: 4, RisikoTinggiTon: 172.7},
	}

	w := do(newRouter(repo, "u1", "coldstorage.lihat"), http.MethodGet, "/cold-storage", nil)

	require.Equal(t, http.StatusOK, w.Code, w.Body.String())

	var raw map[string]any
	var env envelope
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &env))
	assert.Equal(t, "200", env.Code)
	require.NoError(t, json.Unmarshal(env.Data, &raw))
	assert.ElementsMatch(t, []string{"fasilitas", "stok", "meta", "ringkasan"}, keys(raw))

	data := decode[struct {
		Fasilitas []map[string]any `json:"fasilitas"`
		Stok      []map[string]any `json:"stok"`
		Meta      map[string]any   `json:"meta"`
		Ringkasan map[string]any   `json:"ringkasan"`
	}](t, w)

	require.Len(t, data.Fasilitas, 2)
	f0 := data.Fasilitas[0]
	assert.Equal(t, "CS-02", f0["id"])
	assert.Equal(t, "KOTA LHOKSEUMAWE", f0["kabupaten_nama"])
	assert.Equal(t, 240.0, f0["terisi_ton"])
	assert.Equal(t, 80.0, f0["utilisasi_persen"])
	assert.Equal(t, -20.1, f0["suhu_aktual_c"])
	assert.Nil(t, data.Fasilitas[1]["suhu_aktual_c"], "fasilitas rencana tanpa telemetri bernilai null")
	assert.Contains(t, data.Fasilitas[1], "suhu_aktual_c", "kunci tetap ada agar kontrak stabil")

	require.Len(t, data.Stok, 1)
	s0 := data.Stok[0]
	assert.Equal(t, "ikan_tongkol", s0["komoditas_kode"])
	assert.Equal(t, "Ikan Tongkol", s0["komoditas_nama"])
	assert.Equal(t, "Cold Storage Perikanan Lhokseumawe", s0["cold_storage_nama"])
	assert.Equal(t, 19.0, s0["sisa_masa_simpan_hari"])
	assert.Equal(t, 0.72, s0["risiko_food_loss"])
	assert.NotEmpty(t, s0["masuk_pada"])
	assert.NotEmpty(t, s0["kedaluwarsa_pada"])

	assert.Equal(t, 1.0, data.Meta["page"])
	assert.Equal(t, 10.0, data.Meta["limit"])
	assert.Equal(t, 23.0, data.Meta["total"])
	assert.Equal(t, 3.0, data.Meta["total_pages"])
	assert.Equal(t, 812.0, data.Meta["volume_hasil_ton"])

	assert.Equal(t, 172.7, data.Ringkasan["risiko_tinggi_ton"])
	assert.Equal(t, 4.0, data.Ringkasan["risiko_tinggi_lot"])
	assert.Equal(t, domain.RisikoTinggiAmbang, data.Ringkasan["ambang_risiko_tinggi"])
}

func TestGetAll_EmptyListsAreArraysNotNull(t *testing.T) {
	w := do(newRouter(&fakeRepo{}, "u1", "coldstorage.lihat"), http.MethodGet, "/cold-storage", nil)

	require.Equal(t, http.StatusOK, w.Code)
	assert.Contains(t, w.Body.String(), `"fasilitas":[]`)
	assert.Contains(t, w.Body.String(), `"stok":[]`)
}

func TestGetAll_PassesQueryToUseCase(t *testing.T) {
	repo := &fakeRepo{}
	q := url.Values{
		"cold_storage_id": {"CS-03"},
		"q":               {"tomat"},
		"sort":            {"volume"},
		"order":           {"desc"},
		"page":            {"2"},
		"limit":           {"25"},
	}

	w := do(newRouter(repo, "u1"), http.MethodGet, "/cold-storage?"+q.Encode(), nil)

	require.Equal(t, http.StatusOK, w.Code, w.Body.String())
	assert.Equal(t, domain.StokQuery{
		ColdStorageID: "CS-03", Search: "tomat", Sort: domain.SortVolume, Desc: true, Page: 2, Limit: 25,
	}, repo.gotQuery)
}

func TestGetAll_DefaultsWhenNoQuery(t *testing.T) {
	repo := &fakeRepo{}

	w := do(newRouter(repo, "u1"), http.MethodGet, "/cold-storage", nil)

	require.Equal(t, http.StatusOK, w.Code)
	assert.Equal(t, domain.StokQuery{Sort: domain.SortSisaMasaSimpan, Page: 1, Limit: usecase.DefaultLimit}, repo.gotQuery)
}

func TestGetAll_InvalidQuery(t *testing.T) {
	longQ := make([]byte, 101)
	for i := range longQ {
		longQ[i] = 'a'
	}

	cases := map[string]string{
		"sort tidak dikenal":         "sort=password",
		"order tidak dikenal":        "order=sideways",
		"halaman negatif":            "page=-1",
		"halaman bukan angka":        "page=abc",
		"limit negatif":              "limit=-5",
		"limit melebihi batas":       "limit=101",
		"kata kunci terlalu panjang": "q=" + string(longQ),
	}
	for name, qs := range cases {
		t.Run(name, func(t *testing.T) {
			repo := &fakeRepo{}
			w := do(newRouter(repo, "u1"), http.MethodGet, "/cold-storage?"+qs, nil)

			assert.Equal(t, http.StatusBadRequest, w.Code, w.Body.String())
			assert.Contains(t, w.Body.String(), "Validasi gagal")
		})
	}
}

// Nol diperlakukan sebagai "tidak diisi" (omitempty), sama seperti endpoint daftar lain.
func TestGetAll_ZeroPageAndLimitFallBackToDefaults(t *testing.T) {
	repo := &fakeRepo{}

	w := do(newRouter(repo, "u1"), http.MethodGet, "/cold-storage?page=0&limit=0", nil)

	require.Equal(t, http.StatusOK, w.Code)
	assert.Equal(t, 1, repo.gotQuery.Page)
	assert.Equal(t, usecase.DefaultLimit, repo.gotQuery.Limit)
}

func TestGetAll_InternalError(t *testing.T) {
	w := do(newRouter(&fakeRepo{err: errors.New("db down")}, "u1"), http.MethodGet, "/cold-storage", nil)

	assert.Equal(t, http.StatusInternalServerError, w.Code)
	assert.NotContains(t, w.Body.String(), "db down", "detail galat internal tidak boleh bocor")
}

// ── POST /cold-storage/bulk ───────────────────────────────────────────

func TestBulk_Success(t *testing.T) {
	repo := &fakeRepo{bulk: domain.BulkResult{Berhasil: 2, VolumeTon: 65.9}}
	r := newRouter(repo, "manajer_program", "logistik.kelola")

	w := do(r, http.MethodPost, "/cold-storage/bulk", map[string]any{"action": "rencanakan_distribusi", "ids": []string{"S1", "S2", "S3"}})

	require.Equal(t, http.StatusOK, w.Code, w.Body.String())
	out := decode[map[string]any](t, w)
	assert.Equal(t, 2.0, out["berhasil"])
	assert.Equal(t, 1.0, out["gagal"])
	assert.Equal(t, 65.9, out["volume_ton"])

	assert.Equal(t, []string{"S1", "S2", "S3"}, repo.gotIDs)
	assert.Equal(t, domain.BulkRencanakanDistribusi, repo.gotAction)
	require.NotNil(t, repo.gotAudit, "aksi massal tercatat di audit")
	require.NotNil(t, repo.gotAudit.AktorID)
	assert.Equal(t, "manajer_program", *repo.gotAudit.AktorID)
}

func TestBulk_PermissionPerAction(t *testing.T) {
	cases := []struct {
		name   string
		action string
		perms  []string
		want   int
	}{
		{"distribusi dengan logistik.kelola", "rencanakan_distribusi", []string{"logistik.kelola"}, http.StatusOK},
		{"distribusi hanya dengan coldstorage.kelola ditolak", "rencanakan_distribusi", []string{"coldstorage.kelola"}, http.StatusForbidden},
		{"olahan dengan coldstorage.kelola", "rekomendasi_olahan", []string{"coldstorage.kelola"}, http.StatusOK},
		{"olahan hanya dengan logistik.kelola ditolak", "rekomendasi_olahan", []string{"logistik.kelola"}, http.StatusForbidden},
		{"pemegang kedua izin dapat keduanya", "rekomendasi_olahan", []string{"coldstorage.kelola", "logistik.kelola"}, http.StatusOK},
		{"tanpa izin ditolak", "rekomendasi_olahan", nil, http.StatusForbidden},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			repo := &fakeRepo{bulk: domain.BulkResult{Berhasil: 1}}
			w := do(newRouter(repo, "u1", tc.perms...), http.MethodPost, "/cold-storage/bulk", map[string]any{"action": tc.action, "ids": []string{"S1"}})

			assert.Equal(t, tc.want, w.Code, w.Body.String())
			if tc.want == http.StatusForbidden {
				assert.Nil(t, repo.gotIDs, "aksi yang ditolak tidak boleh diproses")
				assert.Nil(t, repo.gotAudit)
			}
		})
	}
}

func TestBulk_Validation(t *testing.T) {
	tooMany := make([]string, 501)
	for i := range tooMany {
		tooMany[i] = "S"
	}

	cases := map[string]any{
		"JSON rusak":         "{not json",
		"tanpa aksi":         map[string]any{"ids": []string{"S1"}},
		"aksi tidak dikenal": map[string]any{"action": "hapus", "ids": []string{"S1"}},
		"tanpa ids":          map[string]any{"action": "rekomendasi_olahan"},
		"ids kosong":         map[string]any{"action": "rekomendasi_olahan", "ids": []string{}},
		"id kosong":          map[string]any{"action": "rekomendasi_olahan", "ids": []string{""}},
		"ids melebihi batas": map[string]any{"action": "rekomendasi_olahan", "ids": tooMany},
		"id hanya spasi":     map[string]any{"action": "rekomendasi_olahan", "ids": []string{"   "}},
	}
	for name, body := range cases {
		t.Run(name, func(t *testing.T) {
			repo := &fakeRepo{}
			w := do(newRouter(repo, "u1", "coldstorage.kelola"), http.MethodPost, "/cold-storage/bulk", body)

			assert.Equal(t, http.StatusBadRequest, w.Code, w.Body.String())
			assert.Nil(t, repo.gotIDs)
		})
	}
}

func TestBulk_InternalError(t *testing.T) {
	repo := &fakeRepo{err: errors.New("db down")}

	w := do(newRouter(repo, "u1", "coldstorage.kelola"), http.MethodPost, "/cold-storage/bulk", map[string]any{"action": "rekomendasi_olahan", "ids": []string{"S1"}})

	assert.Equal(t, http.StatusInternalServerError, w.Code)
	assert.NotContains(t, w.Body.String(), "db down")
}

func TestHealth(t *testing.T) {
	w := do(newRouter(&fakeRepo{}, "u1"), http.MethodGet, "/cold-storage/health", nil)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.JSONEq(t, `{"status":"ok"}`, w.Body.String())
}

func keys(m map[string]any) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	return out
}
