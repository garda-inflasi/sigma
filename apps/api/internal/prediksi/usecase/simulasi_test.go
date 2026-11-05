package usecase

import (
	"context"
	"encoding/json"
	"errors"
	"testing"
	"time"

	"github.com/garda-inflasi/api/internal/audit/audittest"
	"github.com/garda-inflasi/api/internal/prediksi/domain"
	"github.com/garda-inflasi/api/pkg/logger"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

type fakeSimulasiRepo struct {
	view     *domain.ProyeksiView
	err      error
	disimpan *domain.SkenarioSimulasi
}

func (r *fakeSimulasiRepo) GetTerbaru(context.Context, string, string, int) (*domain.ProyeksiView, error) {
	if r.view == nil {
		return nil, gorm.ErrRecordNotFound
	}
	return r.view, r.err
}

func (r *fakeSimulasiRepo) SimpanSkenario(_ context.Context, s *domain.SkenarioSimulasi) error {
	r.disimpan = s
	return r.err
}

func netral() ParameterSimulasi {
	return ParameterSimulasi{KomoditasKode: "CBM-KRT", KabupatenKode: "1171", HorizonHari: 14, HujanPct: CurahHujanNormal}
}

func TestHitungSkenario_NetralSamaDenganProyeksiModel(t *testing.T) {
	hasil := hitungSkenario(*contohView(57000), netral())

	assert.Equal(t, 14.0, hasil.DasarPct)
	assert.Equal(t, 14.0, hasil.HasilPct)
	assert.Equal(t, 4.0, hasil.BawahPct)
	assert.Equal(t, 24.0, hasil.AtasPct)
	assert.Equal(t, 57000, hasil.Proyeksi)
	for _, e := range hasil.Efek {
		assert.Zero(t, e.EfekPct, e.Faktor)
	}
}

func TestHitungSkenario_EfekKoefisienKaliParameter(t *testing.T) {
	p := netral()
	p.HujanPct, p.PasokanPct, p.SubsidiPct = 200, 20, 10 // hujan +100 pp, pasokan +20%, subsidi 10%

	hasil := hitungSkenario(*contohView(57000), p)

	require.Len(t, hasil.Efek, 3)
	assert.Equal(t, 6.0, hasil.Efek[0].EfekPct, "+100 pp × 0,06")
	assert.Equal(t, -7.0, hasil.Efek[1].EfekPct, "20 × -0,35")
	assert.Equal(t, -1.8, hasil.Efek[2].EfekPct, "10 × -0,18")
	assert.Equal(t, 200.0, hasil.Efek[0].Parameter, "parameter hujan tampil sebagai nilai yang dimasukkan")
	assert.Equal(t, 11.2, hasil.HasilPct, "14,0 + 6,0 - 7,0 - 1,8")
	assert.Equal(t, 1.2, hasil.BawahPct, "rentang bergeser sebesar total efek")
	assert.Equal(t, 21.2, hasil.AtasPct)
	assert.Equal(t, 55600, hasil.Proyeksi, "50.000 × 1,112")
}

func TestHitungSkenario_JumlahEfekKonsistenDenganHasil(t *testing.T) {
	p := ParameterSimulasi{HujanPct: 137, PasokanPct: -13.5, SubsidiPct: 7}
	hasil := hitungSkenario(*contohView(52000), p)

	var total float64
	for _, e := range hasil.Efek {
		total += e.EfekPct
	}
	assert.InDelta(t, hasil.DasarPct+total, hasil.HasilPct, 0.051)
}

func TestHitungSkenario_KoefisienAsumsiDilabeliTerbatas(t *testing.T) {
	hasil := hitungSkenario(*contohView(52000), netral())

	assert.True(t, hasil.Terbatas)
	assert.Contains(t, hasil.Catatan, "belum diestimasi dari data")
	for _, e := range hasil.Efek {
		assert.Equal(t, SumberKoefisienAsumsi, e.Sumber)
	}
	assert.Equal(t, domain.SumberDemo, hasil.SumberModel, "label sumber proyeksi dasar ikut dibawa")
}

func TestSimulasiHitung_MenyimpanSkenarioDanMencatatAudit(t *testing.T) {
	repo := &fakeSimulasiRepo{view: contohView(57000)}
	rec := &audittest.Recorder{}
	u := NewSimulasi(repo, rec, logger.NewDefault())
	u.now = func() time.Time { return sekarang }
	p := netral()
	p.PasokanPct = 10

	hasil, err := u.Hitung(t.Context(), "u-1", p)

	require.NoError(t, err)
	assert.Equal(t, 10.5, hasil.HasilPct, "14,0 - 3,5")
	assert.NotEmpty(t, hasil.ID)
	assert.Equal(t, sekarang, hasil.DibuatPada)

	s := repo.disimpan
	require.NotNil(t, s)
	assert.Equal(t, hasil.ID, s.ID)
	assert.Equal(t, "u-1", s.DibuatOleh)
	assert.Equal(t, "CBM-KRT", s.KomoditasKode)
	assert.Equal(t, 14, s.HorizonHari)
	var parameter ParameterSimulasi
	require.NoError(t, json.Unmarshal([]byte(s.Parameter), &parameter))
	assert.Equal(t, p, parameter)
	var tersimpan HasilSimulasi
	require.NoError(t, json.Unmarshal([]byte(s.Hasil), &tersimpan))
	assert.Equal(t, 10.5, tersimpan.HasilPct)
	assert.Len(t, tersimpan.Efek, 3)

	e, ok := rec.Last()
	require.True(t, ok)
	assert.Equal(t, "SIMULASI_SKENARIO", e.Aksi)
	assert.Equal(t, hasil.ID, e.EntitasID)
}

func TestSimulasiHitung_TanpaProyeksiModelProduksi(t *testing.T) {
	repo := &fakeSimulasiRepo{}
	u := NewSimulasi(repo, &audittest.Recorder{}, logger.NewDefault())

	_, err := u.Hitung(t.Context(), "u-1", netral())

	assert.ErrorIs(t, err, ErrProyeksiTidakAda)
	assert.Nil(t, repo.disimpan)
}

func TestSimulasiHitung_GalatRepositori(t *testing.T) {
	boom := errors.New("db down")
	u := NewSimulasi(&fakeSimulasiRepo{view: contohView(57000), err: boom}, &audittest.Recorder{}, logger.NewDefault())

	_, err := u.Hitung(t.Context(), "u-1", netral())

	assert.ErrorIs(t, err, boom)
}
