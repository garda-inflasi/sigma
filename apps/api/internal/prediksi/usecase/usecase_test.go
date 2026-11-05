package usecase

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/garda-inflasi/api/internal/prediksi/domain"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

type fakeRepo struct {
	view    *domain.ProyeksiView
	jalur   []domain.TitikProyeksi
	riwayat []domain.TitikHarga
	err     error

	riwayatDari, riwayatSampai time.Time
}

func (r *fakeRepo) ListTerbaru(context.Context, string, int) ([]domain.ProyeksiView, error) {
	if r.view == nil {
		return nil, r.err
	}
	return []domain.ProyeksiView{*r.view}, r.err
}

func (r *fakeRepo) GetTerbaru(context.Context, string, string, int) (*domain.ProyeksiView, error) {
	if r.view == nil {
		return nil, gorm.ErrRecordNotFound
	}
	return r.view, r.err
}

func (r *fakeRepo) Jalur(context.Context, domain.ProyeksiView, int) ([]domain.TitikProyeksi, error) {
	return r.jalur, nil
}

func (r *fakeRepo) Riwayat(_ context.Context, _, _ string, dari, sampai time.Time) ([]domain.TitikHarga, error) {
	r.riwayatDari, r.riwayatSampai = dari, sampai
	return r.riwayat, nil
}

func (r *fakeRepo) ListModel(context.Context) ([]domain.ModelView, error) { return nil, r.err }

func tanggal(m time.Month, d int) time.Time { return time.Date(2026, m, d, 0, 0, 0, 0, time.UTC) }

func contohView(nilai int) *domain.ProyeksiView {
	return &domain.ProyeksiView{
		ID: "PRD-X", KomoditasKode: "CBM-KRT", KomoditasNama: "Cabai merah keriting", KabupatenKode: "1171", KabupatenNama: "KOTA BANDA ACEH",
		TanggalTarget: tanggal(time.November, 19), HorizonHari: 14,
		Nilai: nilai, Bawah: nilai - 5000, Atas: nilai + 5000, HargaDasar: 50000,
		Faktor:     `[{"nama":"Stok cold storage","kontribusi":2.1,"detail":"Stok turun"},{"nama":"Curah hujan sentra","kontribusi":5.4,"detail":"2,1× normal"}]`,
		ModelVersi: "v2.3.1", ModelNama: "Ensemble", ModelMape: 6.8, Sumber: domain.SumberDemo,
	}
}

func TestDetail_ComputesPercentagesAndNarrative(t *testing.T) {
	repo := &fakeRepo{view: contohView(57000), jalur: []domain.TitikProyeksi{{HorizonHari: 7, Nilai: 53500}}}

	got, err := New(repo).Detail(t.Context(), "CBM-KRT", "1171", 14)

	require.NoError(t, err)
	assert.Equal(t, 14.0, got.PerubahanPct)
	assert.Equal(t, 4.0, got.BawahPct)
	assert.Equal(t, 24.0, got.AtasPct)
	assert.Equal(t, "Cabai merah keriting di Kota Banda Aceh diproyeksi naik 14,0% dalam 14 hari. Faktor paling berpengaruh: curah hujan sentra (2,1× normal).", got.Narasi)
	assert.Len(t, got.Faktor, 2)
	assert.Len(t, got.Jalur, 1)
	assert.Contains(t, got.Rekomendasi[0], "operasi pasar")
}

func TestDetail_HistoryWindowEndsAtBaseDay(t *testing.T) {
	repo := &fakeRepo{view: contohView(52000)}

	_, err := New(repo).Detail(t.Context(), "CBM-KRT", "1171", 14)

	require.NoError(t, err)
	assert.Equal(t, tanggal(time.November, 5), repo.riwayatSampai, "hari dasar = tanggal target - horizon")
	assert.Equal(t, tanggal(time.October, 6), repo.riwayatDari)
}

func TestDetail_FallingPriceNarrativeAndRecommendation(t *testing.T) {
	repo := &fakeRepo{view: contohView(47500)}

	got, err := New(repo).Detail(t.Context(), "CBM-KRT", "1171", 14)

	require.NoError(t, err)
	assert.Equal(t, -5.0, got.PerubahanPct)
	assert.Contains(t, got.Narasi, "turun 5,0%")
	assert.Contains(t, got.Rekomendasi[0], "cold storage")
}

func TestRekomendasi_ThresholdsAreExclusive(t *testing.T) {
	p := *contohView(0)
	assert.Contains(t, rekomendasi(p, 5.1)[0], "operasi pasar")
	assert.Equal(t, "Lanjutkan pemantauan rutin", rekomendasi(p, 5.0)[0])
	assert.Equal(t, "Lanjutkan pemantauan rutin", rekomendasi(p, -3.0)[0])
	assert.Contains(t, rekomendasi(p, -3.1)[0], "cold storage")
}

func TestDetail_NotFound(t *testing.T) {
	_, err := New(&fakeRepo{}).Detail(t.Context(), "XXX", "1171", 14)
	assert.ErrorIs(t, err, ErrNotFound)
}

func TestDetail_InvalidFactorDocumentIsAnError(t *testing.T) {
	view := contohView(52000)
	view.Faktor = `not json`

	_, err := New(&fakeRepo{view: view}).Detail(t.Context(), "CBM-KRT", "1171", 14)

	require.Error(t, err)
	assert.NotErrorIs(t, err, ErrNotFound)
}

func TestDetail_PropagatesRepositoryError(t *testing.T) {
	boom := errors.New("db down")
	_, err := New(&fakeRepo{view: contohView(52000), err: boom}).Detail(t.Context(), "CBM-KRT", "1171", 14)
	assert.ErrorIs(t, err, boom)
}

func TestRingkasan_AddsPercentages(t *testing.T) {
	got, err := New(&fakeRepo{view: contohView(52500)}).Ringkasan(t.Context(), "1171", 14)

	require.NoError(t, err)
	require.Len(t, got, 1)
	assert.Equal(t, 5.0, got[0].PerubahanPct)
	assert.Equal(t, -5.0, got[0].BawahPct)
	assert.Equal(t, 15.0, got[0].AtasPct)
}
