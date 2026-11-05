package http

import (
	"context"
	"encoding/json"
	"errors"
	nethttp "net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/garda-inflasi/api/internal/spasial/domain"
	"github.com/garda-inflasi/api/internal/spasial/usecase"
	"github.com/garda-inflasi/api/pkg/logger"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// fakeRepo memenuhi domain.Repository dan domain.OptimasiRepository tanpa basis data.
type fakeRepo struct {
	markers  []domain.Marker
	neraca   []domain.Neraca
	riwayat  []domain.RencanaOptimasi
	geometri []domain.GeometriWilayah
	err      error
}

func (f *fakeRepo) ListMarker(context.Context) ([]domain.Marker, error) { return f.markers, f.err }
func (f *fakeRepo) ListNeraca(context.Context, domain.NeracaQuery) ([]domain.Neraca, error) {
	return f.neraca, f.err
}
func (f *fakeRepo) ListRute(context.Context) ([]domain.Rute, error)         { return nil, f.err }
func (f *fakeRepo) ListKandidat(context.Context) ([]domain.Kandidat, error) { return nil, f.err }
func (f *fakeRepo) ListGeometri(context.Context, *domain.BBox) ([]domain.GeometriWilayah, error) {
	return f.geometri, f.err
}
func (f *fakeRepo) ListJarak(context.Context) ([]domain.JarakAntarWilayah, error) { return nil, f.err }
func (f *fakeRepo) SimpanOptimasi(context.Context, domain.OptimasiRute, []domain.RuteDistribusi, []domain.JarakAntarWilayah) error {
	return f.err
}
func (f *fakeRepo) ListOptimasi(context.Context, int) ([]domain.RencanaOptimasi, error) {
	return f.riwayat, f.err
}

func router(repo *fakeRepo) *gin.Engine {
	gin.SetMode(gin.TestMode)
	log := logger.NewDefault()
	h := New(usecase.New(repo, log), usecase.NewOptimasi(repo, log, nil))
	r := gin.New()
	r.GET("/spasial/neraca", h.ListNeraca)
	r.GET("/spasial/lokasi", h.Lokasi)
	r.GET("/spasial/optimasi-rute", h.RiwayatOptimasi)
	return r
}

func get(r *gin.Engine, path string) *httptest.ResponseRecorder {
	w := httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequest(nethttp.MethodGet, path, nil))
	return w
}

func body(t *testing.T, w *httptest.ResponseRecorder) map[string]any {
	t.Helper()
	var out map[string]any
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &out), w.Body.String())
	return out
}

func pusat(kode, nama string, lat, lng float64) domain.Marker {
	return domain.Marker{Wilayah: domain.Wilayah{Kode: kode, Nama: nama, Lat: lat, Lng: lng, Level: "kabupaten"}}
}

func TestLokasi_Handler(t *testing.T) {
	t.Run("tanpa data wilayah 404", func(t *testing.T) {
		w := get(router(&fakeRepo{}), "/spasial/lokasi?lat=5.5&lng=95.3")
		assert.Equal(t, nethttp.StatusNotFound, w.Code)
	})

	t.Run("parameter hilang, bukan angka, atau di luar rentang 400", func(t *testing.T) {
		r := router(&fakeRepo{markers: []domain.Marker{pusat("1171", "KOTA BANDA ACEH", 5.55, 95.32)}})
		for _, path := range []string{
			"/spasial/lokasi",
			"/spasial/lokasi?lat=5.5",
			"/spasial/lokasi?lng=95.3",
			"/spasial/lokasi?lat=abc&lng=95.3",
			"/spasial/lokasi?lat=91&lng=95.3",
			"/spasial/lokasi?lat=5.5&lng=-181",
		} {
			assert.Equal(t, nethttp.StatusBadRequest, get(r, path).Code, path)
		}
	})

	t.Run("titik 0,0 sah dan jatuh ke wilayah terdekat", func(t *testing.T) {
		r := router(&fakeRepo{markers: []domain.Marker{pusat("1171", "KOTA BANDA ACEH", 5.55, 95.32)}})
		w := get(r, "/spasial/lokasi?lat=0&lng=0")
		require.Equal(t, nethttp.StatusOK, w.Code, w.Body.String())
		d := body(t, w)["data"].(map[string]any)
		assert.Equal(t, "1171", d["kode"])
		assert.Equal(t, false, d["di_dalam"])
		assert.Greater(t, d["jarak_pusat_km"].(float64), 1000.0)
	})

	t.Run("galat repository 500", func(t *testing.T) {
		w := get(router(&fakeRepo{err: errors.New("db mati")}), "/spasial/lokasi?lat=5.5&lng=95.3")
		assert.Equal(t, nethttp.StatusInternalServerError, w.Code)
	})
}

func TestNeraca_Handler(t *testing.T) {
	t.Run("kosong menghasilkan daftar kosong, bukan null", func(t *testing.T) {
		w := get(router(&fakeRepo{}), "/spasial/neraca")
		require.Equal(t, nethttp.StatusOK, w.Code)
		assert.Equal(t, []any{}, body(t, w)["data"])
	})

	t.Run("memetakan baris termasuk label sumber", func(t *testing.T) {
		n := domain.Neraca{KomoditasNama: "Cabai merah keriting", KabupatenNama: "KOTA BANDA ACEH"}
		n.KomoditasKode, n.KabupatenKode = "CBM-KRT", "1171"
		n.Periode = time.Date(2026, 11, 1, 0, 0, 0, 0, time.UTC)
		n.SurplusDefisitTon, n.DeltaHargaPct, n.Sumber = -31.2, 5.2, "DEMO"

		w := get(router(&fakeRepo{neraca: []domain.Neraca{n}}), "/spasial/neraca?komoditas=CBM-KRT&periode=2026-11-01")
		require.Equal(t, nethttp.StatusOK, w.Code)
		rows := body(t, w)["data"].([]any)
		require.Len(t, rows, 1)
		row := rows[0].(map[string]any)
		assert.Equal(t, "CBM-KRT", row["komoditas_kode"])
		assert.Equal(t, "KOTA BANDA ACEH", row["wilayah"])
		assert.Equal(t, "2026-11-01", row["periode"])
		assert.Equal(t, -31.2, row["surplus_defisit_ton"])
		assert.Equal(t, "DEMO", row["sumber"])
	})

	t.Run("periode salah format 400, galat repository 500", func(t *testing.T) {
		assert.Equal(t, nethttp.StatusBadRequest, get(router(&fakeRepo{}), "/spasial/neraca?periode=kemarin").Code)
		assert.Equal(t, nethttp.StatusInternalServerError, get(router(&fakeRepo{err: errors.New("db mati")}), "/spasial/neraca").Code)
	})
}

func TestRiwayatOptimasi_Handler(t *testing.T) {
	t.Run("kosong menghasilkan daftar kosong, bukan null", func(t *testing.T) {
		w := get(router(&fakeRepo{}), "/spasial/optimasi-rute")
		require.Equal(t, nethttp.StatusOK, w.Code)
		assert.Equal(t, []any{}, body(t, w)["data"])
	})

	t.Run("memetakan ringkasan, pembuat terhapus menjadi null", func(t *testing.T) {
		nama := "Manajer Data"
		a := domain.RencanaOptimasi{KomoditasNama: "Bawang merah", DibuatOlehNama: &nama}
		a.ID, a.KomoditasKode = "OPT-1", "BWM"
		a.Periode = time.Date(2026, 11, 1, 0, 0, 0, 0, time.UTC)
		a.TotalVolumeTon, a.TotalBiaya, a.BelumTerpenuhiTon = 54, 61_200_000, 3.5
		a.DibuatPada = time.Date(2026, 11, 2, 14, 40, 0, 0, time.UTC)
		b := a
		b.ID, b.DibuatOlehNama = "OPT-2", nil

		w := get(router(&fakeRepo{riwayat: []domain.RencanaOptimasi{a, b}}), "/spasial/optimasi-rute")
		require.Equal(t, nethttp.StatusOK, w.Code)
		rows := body(t, w)["data"].([]any)
		require.Len(t, rows, 2)
		first := rows[0].(map[string]any)
		assert.Equal(t, "OPT-1", first["id"])
		assert.Equal(t, "Bawang merah", first["komoditas_nama"])
		assert.Equal(t, "2026-11-01", first["periode"])
		assert.Equal(t, 54.0, first["total_volume_ton"])
		assert.Equal(t, 3.5, first["belum_terpenuhi_ton"])
		assert.Equal(t, "Manajer Data", first["dibuat_oleh"])
		assert.Nil(t, rows[1].(map[string]any)["dibuat_oleh"])
	})

	t.Run("galat repository 500", func(t *testing.T) {
		w := get(router(&fakeRepo{err: errors.New("db mati")}), "/spasial/optimasi-rute")
		assert.Equal(t, nethttp.StatusInternalServerError, w.Code)
	})
}
