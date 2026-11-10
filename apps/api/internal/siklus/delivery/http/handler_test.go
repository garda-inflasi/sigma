package http

import (
	"context"
	"encoding/json"
	"errors"
	nethttp "net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/garda-inflasi/api/internal/audit/audittest"
	"github.com/garda-inflasi/api/internal/siklus/domain"
	"github.com/garda-inflasi/api/internal/siklus/usecase"
	"github.com/garda-inflasi/api/pkg/idx"
	"github.com/garda-inflasi/api/pkg/logger"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type fakeRepo struct {
	siklus *domain.SiklusEkonomi
	aturan []domain.AturanSiklus
	err    error
}

func (f *fakeRepo) GetActiveByWilayah(context.Context, string) (*domain.SiklusEkonomi, error) {
	return f.siklus, f.err
}
func (f *fakeRepo) ListAturan(context.Context) ([]domain.AturanSiklus, error) { return f.aturan, f.err }

type fakeEvaluator struct {
	hasil domain.HasilEvaluasi
	err   error
}

func (f *fakeEvaluator) Evaluasi(context.Context, string, domain.OpsiEvaluasi) (domain.HasilEvaluasi, error) {
	return f.hasil, f.err
}

func router(repo *fakeRepo, ev *fakeEvaluator) *gin.Engine {
	gin.SetMode(gin.TestMode)
	h := New(usecase.New(repo, ev, &audittest.Recorder{}, logger.New(logger.ERROR, nil)))
	r := gin.New()
	r.GET("/aktif", h.GetAktif)
	r.GET("/aturan", h.ListAturan)
	r.POST("/evaluasi", h.Evaluasi)
	return r
}

func do(r *gin.Engine, method, path string) *httptest.ResponseRecorder {
	w := httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequest(method, path, nil))
	return w
}

func TestGetAktif_BentukLamaDitambahOtomatisDanAlasan(t *testing.T) {
	alasan := `{"ringkasan":"Siklus awal TEKANAN: ...","pemicu":[{"aturan":"SKL-X","siklus":"TEKANAN","indikator":"ALERT_EWS","nilai":2,"ambang":1,"satuan":"alert","prioritas":10}],"ditekan":[]}`
	repo := &fakeRepo{siklus: &domain.SiklusEkonomi{
		Siklus: domain.SiklusTekanan, MulaiPada: time.Date(2026, 9, 26, 20, 0, 0, 0, time.UTC), Otomatis: true, Alasan: &alasan,
	}}

	w := do(router(repo, &fakeEvaluator{}), nethttp.MethodGet, "/aktif")

	require.Equal(t, nethttp.StatusOK, w.Code, w.Body.String())
	var res struct {
		Data struct {
			Siklus   string `json:"siklus"`
			Sejak    string `json:"sejak"`
			Otomatis bool   `json:"otomatis"`
			Alasan   struct {
				Ringkasan string `json:"ringkasan"`
				Pemicu    []struct {
					Aturan  string   `json:"aturan"`
					Siklus  string   `json:"siklus"`
					Rujukan []string `json:"rujukan"`
				} `json:"pemicu"`
				Ditekan []any `json:"ditekan"`
			} `json:"alasan"`
		} `json:"data"`
	}
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &res))
	assert.Equal(t, "tekanan", res.Data.Siklus)
	assert.Equal(t, "2026-09-27", res.Data.Sejak)
	assert.True(t, res.Data.Otomatis)
	assert.Equal(t, "Siklus awal TEKANAN: ...", res.Data.Alasan.Ringkasan)
	require.Len(t, res.Data.Alasan.Pemicu, 1)
	assert.Equal(t, "SKL-X", res.Data.Alasan.Pemicu[0].Aturan)
	assert.Equal(t, "tekanan", res.Data.Alasan.Pemicu[0].Siklus, "siklus huruf kecil seperti di seluruh modul ini")
	assert.NotNil(t, res.Data.Alasan.Pemicu[0].Rujukan, "rujukan kosong menjadi [] (bukan null)")
	assert.Empty(t, res.Data.Alasan.Ditekan)
}

func TestGetAktif_AlasanRusakDijawabNullBukanGalat(t *testing.T) {
	rusak := `bukan json`
	repo := &fakeRepo{siklus: &domain.SiklusEkonomi{Siklus: domain.SiklusNormal, MulaiPada: time.Now(), Otomatis: true, Alasan: &rusak}}

	w := do(router(repo, &fakeEvaluator{}), nethttp.MethodGet, "/aktif")

	require.Equal(t, nethttp.StatusOK, w.Code)
	assert.Contains(t, w.Body.String(), `"alasan":null`)
	assert.Contains(t, w.Body.String(), `"siklus":"normal"`)
}

func TestGetAktif_TidakAdaSiklusDan404(t *testing.T) {
	w := do(router(&fakeRepo{err: domain.ErrNotFound}, &fakeEvaluator{}), nethttp.MethodGet, "/aktif")
	assert.Equal(t, nethttp.StatusNotFound, w.Code)

	w = do(router(&fakeRepo{err: errors.New("db mati")}, &fakeEvaluator{}), nethttp.MethodGet, "/aktif")
	assert.Equal(t, nethttp.StatusInternalServerError, w.Code)
}

func TestListAturan_DaftarKosongAdalahArrayBukanNull(t *testing.T) {
	w := do(router(&fakeRepo{}, &fakeEvaluator{}), nethttp.MethodGet, "/aturan")
	require.Equal(t, nethttp.StatusOK, w.Code)
	assert.Contains(t, w.Body.String(), `"data":[]`)

	jenis, minimal, jumlah := "LONJAKAN_HARGA", "SIAGA", 1
	repo := &fakeRepo{aturan: []domain.AturanSiklus{{
		ID: 3, Kode: "SKL-A", Siklus: domain.SiklusTekanan, Indikator: domain.IndikatorAlertEws, JenisEws: &jenis, SeverityMin: &minimal,
		JumlahMin: &jumlah, Prioritas: 10, PulihJam: 48, Aktif: true,
	}}}
	w = do(router(repo, &fakeEvaluator{}), nethttp.MethodGet, "/aturan")
	require.Equal(t, nethttp.StatusOK, w.Code)
	assert.Contains(t, w.Body.String(), `"id":"3"`, "ID BIGINT sebagai string")
	assert.Contains(t, w.Body.String(), `"siklus":"tekanan"`)
	assert.Contains(t, w.Body.String(), `"ambang":null`)
}

func TestEvaluasi_StatusDanBentukRespons(t *testing.T) {
	aktif := &domain.SiklusEkonomi{ID: idx.ID(9), Siklus: domain.SiklusTekanan, MulaiPada: time.Date(2026, 11, 9, 20, 0, 0, 0, time.UTC), Otomatis: true}
	ev := &fakeEvaluator{hasil: domain.HasilEvaluasi{
		WilayahKode: "11", Berubah: true, Sebelumnya: domain.SiklusNormal, Aktif: aktif, Ringkasan: "Berpindah ...",
		Aturan: []domain.HasilAturan{{
			Aturan:     domain.AturanSiklus{Kode: "SKL-A", Siklus: domain.SiklusTekanan, Indikator: domain.IndikatorAlertEws, JumlahMin: new(int), Prioritas: 10},
			Pengukuran: domain.Pengukuran{Terukur: true, Nilai: 2, Rujukan: nil}, Memicu: true,
		}},
	}}

	w := do(router(&fakeRepo{}, ev), nethttp.MethodPost, "/evaluasi")

	require.Equal(t, nethttp.StatusOK, w.Code, w.Body.String())
	var res struct {
		Data struct {
			Berubah    bool    `json:"berubah"`
			Sebelumnya *string `json:"sebelumnya"`
			Siklus     *string `json:"siklus"`
			Sejak      *string `json:"sejak"`
			Dilewati   bool    `json:"dilewati"`
			Aturan     []struct {
				Kode    string   `json:"kode"`
				Memicu  bool     `json:"memicu"`
				Rujukan []string `json:"rujukan"`
			} `json:"aturan"`
		} `json:"data"`
	}
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &res))
	assert.True(t, res.Data.Berubah)
	require.NotNil(t, res.Data.Sebelumnya)
	assert.Equal(t, "normal", *res.Data.Sebelumnya)
	require.NotNil(t, res.Data.Siklus)
	assert.Equal(t, "tekanan", *res.Data.Siklus)
	require.NotNil(t, res.Data.Sejak)
	assert.Equal(t, "2026-11-10", *res.Data.Sejak, "tanggal waktu Aceh (WIB)")
	assert.False(t, res.Data.Dilewati)
	require.Len(t, res.Data.Aturan, 1)
	assert.True(t, res.Data.Aturan[0].Memicu)
	assert.NotNil(t, res.Data.Aturan[0].Rujukan, "[] bukan null")
}

func TestEvaluasi_DilewatiTanpaSiklusMengisiNull(t *testing.T) {
	ev := &fakeEvaluator{hasil: domain.HasilEvaluasi{WilayahKode: "11", Dilewati: true, SebabDilewati: domain.SebabWilayahTakDikenal, Ringkasan: "dilewati"}}

	w := do(router(&fakeRepo{}, ev), nethttp.MethodPost, "/evaluasi")

	require.Equal(t, nethttp.StatusOK, w.Code)
	assert.Contains(t, w.Body.String(), `"dilewati":true`)
	assert.Contains(t, w.Body.String(), `"sebab_dilewati":"wilayah_tak_dikenal"`)
	assert.Contains(t, w.Body.String(), `"siklus":null`)
	assert.Contains(t, w.Body.String(), `"sebelumnya":null`)
	assert.Contains(t, w.Body.String(), `"aturan":[]`)
}

func TestEvaluasi_SibukAdalah409DanGalatLainAdalah500(t *testing.T) {
	w := do(router(&fakeRepo{}, &fakeEvaluator{err: domain.ErrEvaluasiSibuk}), nethttp.MethodPost, "/evaluasi")
	assert.Equal(t, nethttp.StatusConflict, w.Code)

	w = do(router(&fakeRepo{}, &fakeEvaluator{err: errors.New("db mati")}), nethttp.MethodPost, "/evaluasi")
	assert.Equal(t, nethttp.StatusInternalServerError, w.Code)
	assert.NotContains(t, w.Body.String(), "db mati", "detail galat internal tidak bocor ke klien")
}
