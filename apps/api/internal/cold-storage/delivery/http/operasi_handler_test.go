package http

import (
	"context"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/garda-inflasi/api/internal/audit/audittest"
	"github.com/garda-inflasi/api/internal/cold-storage/domain"
	"github.com/garda-inflasi/api/internal/cold-storage/usecase"
	"github.com/garda-inflasi/api/pkg/idx"
	"github.com/garda-inflasi/api/pkg/logger"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// fakeOps adalah OperasiRepository untuk menguji lapisan HTTP rute fasilitas dan transaksi: yang diperiksa adalah
// bagaimana :id dan isian ID dibaca (UUID, kode, atau salah bentuk), bukan logika penyimpanan.
type fakeOps struct {
	domain.OperasiRepository
	kode map[string]string // kode fasilitas -> UUID

	dicari    []string // kode yang dicari
	dibaca    []string // UUID yang dibaca lewat GetFasilitas
	diubah    []string // UUID yang diubah
	transaksi []domain.TransaksiInput
	disimpan  []domain.TelemetriSuhu
}

func newFakeOps() *fakeOps {
	return &fakeOps{kode: map[string]string{"CS-01": fas("CS-01"), "CS-02": fas("CS-02")}}
}

func (f *fakeOps) IDFasilitasDariKode(_ context.Context, kode string) (string, error) {
	f.dicari = append(f.dicari, kode)
	id, ok := f.kode[kode]
	if !ok {
		return "", domain.ErrFasilitasTidakAda
	}
	return id, nil
}

func (f *fakeOps) GetFasilitas(_ context.Context, id string) (*domain.FasilitasView, error) {
	f.dibaca = append(f.dibaca, id)
	for kode, v := range f.kode {
		if v == id {
			k := kode
			return &domain.FasilitasView{ID: id, Kode: &k, Nama: "Fasilitas " + kode, SuhuTargetC: 4, KapasitasTon: 100, Status: domain.StatusFasilitasOperasional}, nil
		}
	}
	return nil, domain.ErrFasilitasTidakAda
}

func (f *fakeOps) UpdateFasilitas(ctx context.Context, id string, _ domain.FasilitasPatch) (before, after domain.FasilitasView, err error) {
	v, err := f.GetFasilitas(ctx, id)
	if err != nil {
		return before, after, err
	}
	f.diubah = append(f.diubah, id) // hanya fasilitas yang ada yang benar-benar diubah
	return *v, *v, nil
}

func (f *fakeOps) ListTelemetri(context.Context, string, time.Time, int) ([]domain.TelemetriSuhu, error) {
	return nil, nil
}
func (f *fakeOps) StokFasilitas(context.Context, string) ([]domain.StokView, error) { return nil, nil }
func (f *fakeOps) ListTransaksi(context.Context, string, int) ([]domain.TransaksiView, error) {
	return nil, nil
}

func (f *fakeOps) SimpanTelemetri(_ context.Context, rows []domain.TelemetriSuhu) (int64, error) {
	f.disimpan = append(f.disimpan, rows...)
	return int64(len(rows)), nil
}

func (f *fakeOps) CatatTransaksi(_ context.Context, in domain.TransaksiInput) (domain.TransaksiHasil, error) {
	f.transaksi = append(f.transaksi, in)
	hasil := domain.TransaksiHasil{TransaksiID: 42, Jenis: in.Jenis, LotID: in.LotID, VolumeTon: in.VolumeTon}
	switch in.Jenis {
	case domain.TransaksiMasuk:
		hasil.LotID = 77
	case domain.TransaksiPindah:
		baru := idx.ID(78)
		hasil.LotBaruID = &baru
	}
	return hasil, nil
}

func newOpsRouter(repo *fakeOps, perms ...string) *gin.Engine {
	gin.SetMode(gin.TestMode)
	rec := &audittest.Recorder{}
	ops := usecase.NewOperasi(repo, logger.NewDefault(), rec).WithClock(func() time.Time { return now })
	h := New(nil, ops)

	r := gin.New()
	r.Use(func(c *gin.Context) {
		c.Set("user_id", uid("u1"))
		c.Set("permissions", perms)
		c.Next()
	})
	r.GET("/cold-storage/fasilitas/:id", h.Detail)
	r.PATCH("/cold-storage/fasilitas/:id", h.UbahFasilitas)
	r.POST("/cold-storage/telemetri", h.IngestTelemetri)
	r.POST("/cold-storage/transaksi", h.Transaksi)
	return r
}

func TestFasilitasRoute_AcceptsUUIDOrKodeAndReturnsBothInTheBody(t *testing.T) {
	for name, ref := range map[string]string{
		"kode":             "CS-01",
		"uuid":             fas("CS-01"),
		"uuid huruf besar": strings.ToUpper(fas("CS-01")),
	} {
		t.Run(name, func(t *testing.T) {
			repo := newFakeOps()
			w := do(newOpsRouter(repo, "coldstorage.lihat"), http.MethodGet, "/cold-storage/fasilitas/"+ref, nil)

			require.Equal(t, http.StatusOK, w.Code, w.Body.String())
			f := decode[map[string]any](t, w)["fasilitas"].(map[string]any)
			assert.Equal(t, fas("CS-01"), f["id"])
			assert.Equal(t, "CS-01", f["kode"])
			assert.Equal(t, []string{fas("CS-01")}, repo.dibaca, "repositori selalu menerima UUID kanonik")
		})
	}
}

func TestFasilitasRoute_MalformedOrUnknownIDIsNotFoundWithoutTouchingTheUUIDColumn(t *testing.T) {
	cases := map[string]string{
		"kode tak ada":      "CS-99",
		"uuid tak ada":      fas("CS-99"),
		"lebih dari 64":     strings.Repeat("a", 65),
		"angka":             "12345",
		"uuid tanpa strip":  strings.ReplaceAll(fas("CS-01"), "-", ""),
		"uuid berisi huruf": "zzzzzzzz-zzzz-zzzz-zzzz-zzzzzzzzzzzz",
	}
	for name, ref := range cases {
		t.Run(name, func(t *testing.T) {
			repo := newFakeOps()
			r := newOpsRouter(repo, "coldstorage.lihat", "coldstorage.kelola")

			assert.Equal(t, http.StatusNotFound, do(r, http.MethodGet, "/cold-storage/fasilitas/"+ref, nil).Code)
			assert.Equal(t, http.StatusNotFound, do(r, http.MethodPatch, "/cold-storage/fasilitas/"+ref, map[string]any{"suhu_min_c": 1}).Code)
			assert.Empty(t, repo.diubah, "tidak ada UPDATE untuk fasilitas yang tidak ada")
			for _, id := range repo.dibaca {
				assert.True(t, idx.ValidUUID(id), "hanya UUID sah yang sampai ke repositori: %q", id)
			}
		})
	}
}

func TestFasilitasRoute_PatchByKodeUpdatesTheUUID(t *testing.T) {
	repo := newFakeOps()

	w := do(newOpsRouter(repo, "coldstorage.kelola"), http.MethodPatch, "/cold-storage/fasilitas/CS-02", map[string]any{"suhu_min_c": 1.0})

	require.Equal(t, http.StatusOK, w.Code, w.Body.String())
	assert.Equal(t, []string{fas("CS-02")}, repo.diubah)
}

func TestTelemetriRoute_FasilitasIDAcceptsKodeAndUUID(t *testing.T) {
	repo := newFakeOps()
	waktu := now.Add(-time.Minute).Format(time.RFC3339)

	w := do(newOpsRouter(repo, "coldstorage.kelola"), http.MethodPost, "/cold-storage/telemetri", map[string]any{"pembacaan": []map[string]any{
		{"fasilitas_id": "CS-01", "waktu": waktu, "suhu_c": 4.0},
		{"fasilitas_id": fas("CS-02"), "waktu": waktu, "suhu_c": 4.0},
	}})

	require.Equal(t, http.StatusOK, w.Code, w.Body.String())
	require.Len(t, repo.disimpan, 2)
	assert.Equal(t, fas("CS-01"), repo.disimpan[0].FasilitasID)
	assert.Equal(t, fas("CS-02"), repo.disimpan[1].FasilitasID)

	w = do(newOpsRouter(repo, "coldstorage.kelola"), http.MethodPost, "/cold-storage/telemetri", map[string]any{"pembacaan": []map[string]any{
		{"fasilitas_id": strings.Repeat("a", 65), "waktu": waktu, "suhu_c": 4.0},
	}})
	assert.Equal(t, http.StatusBadRequest, w.Code, "fasilitas_id lebih dari 64 karakter")
	w = do(newOpsRouter(repo, "coldstorage.kelola"), http.MethodPost, "/cold-storage/telemetri", map[string]any{"pembacaan": []map[string]any{
		{"fasilitas_id": "CS-99", "waktu": waktu, "suhu_c": 4.0},
	}})
	assert.Equal(t, http.StatusNotFound, w.Code)
}

func TestTransaksiRoute_LotIDMustBeAnIntegerID(t *testing.T) {
	cases := map[string]any{
		"kode lama":      "STK-CS01-05",
		"uuid":           fas("CS-01"),
		"nol":            "0",
		"negatif":        "-4",
		"pecahan":        1.5,
		"awalan nol":     "007",
		"kosong":         "",
		"melebihi int64": "9223372036854775808",
	}
	for name, lot := range cases {
		t.Run(name, func(t *testing.T) {
			repo := newFakeOps()
			w := do(newOpsRouter(repo, "coldstorage.kelola"), http.MethodPost, "/cold-storage/transaksi",
				map[string]any{"jenis": "KELUAR", "lot_id": lot, "volume_ton": 1, "alasan": "x"})

			assert.Equal(t, http.StatusBadRequest, w.Code, w.Body.String())
			assert.Empty(t, repo.transaksi, "buku besar tidak disentuh")
		})
	}
}

func TestTransaksiRoute_ResponseUsesStringIDsFromRepository(t *testing.T) {
	repo := newFakeOps()
	r := newOpsRouter(repo, "coldstorage.kelola", "logistik.kelola")

	// lot_id sebagai angka JSON maupun string desimal.
	for _, lot := range []any{7, "7"} {
		w := do(r, http.MethodPost, "/cold-storage/transaksi", map[string]any{"jenis": "PINDAH", "lot_id": lot, "fasilitas_tujuan_id": "CS-02", "volume_ton": 3})

		require.Equal(t, http.StatusCreated, w.Code, w.Body.String())
		out := decode[map[string]any](t, w)
		assert.Equal(t, "42", out["transaksi_id"], "ID BIGINT sebagai string desimal")
		assert.Equal(t, "7", out["lot_id"])
		assert.Equal(t, "78", out["lot_baru_id"])
	}
	require.Len(t, repo.transaksi, 2)
	assert.Equal(t, idx.ID(7), repo.transaksi[0].LotID)
	assert.Equal(t, fas("CS-02"), repo.transaksi[0].FasilitasTujuanID, "alias kode dipetakan ke UUID sebelum ke repositori")

	w := do(r, http.MethodPost, "/cold-storage/transaksi", map[string]any{"jenis": "MASUK", "fasilitas_id": fas("CS-01"), "komoditas_kode": "TLR", "pemilik": "KUD", "volume_ton": 2})
	require.Equal(t, http.StatusCreated, w.Code, w.Body.String())
	out := decode[map[string]any](t, w)
	assert.Equal(t, "77", out["lot_id"], "lot MASUK diberi ID oleh basis data")
	assert.Nil(t, out["lot_baru_id"])
}
