package usecase

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/garda-inflasi/api/internal/dashboard/domain"
	"github.com/garda-inflasi/api/pkg/logger"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

type mockRepository struct {
	mock.Mock
}

func (m *mockRepository) DeretKpi(ctx context.Context) ([]domain.TitikKpi, error) {
	args := m.Called(ctx)
	deret, _ := args.Get(0).([]domain.TitikKpi)
	return deret, args.Error(1)
}

func (m *mockRepository) TargetKpi(ctx context.Context, tanggal time.Time) ([]domain.TargetKpi, error) {
	args := m.Called(ctx, tanggal)
	targets, _ := args.Get(0).([]domain.TargetKpi)
	return targets, args.Error(1)
}

func (m *mockRepository) Heatmap(ctx context.Context) (*domain.HeatmapData, error) {
	args := m.Called(ctx)
	data, _ := args.Get(0).(*domain.HeatmapData)
	return data, args.Error(1)
}

func (m *mockRepository) AktivitasLapangan(ctx context.Context, awal, akhir time.Time) ([]domain.AktivitasKabupaten, error) {
	args := m.Called(ctx, awal, akhir)
	items, _ := args.Get(0).([]domain.AktivitasKabupaten)
	return items, args.Error(1)
}

// deret membuat 14 titik harian berakhir 16 Oktober 2026 dari nilai terlama ke terbaru.
func deret(indikator string, nilai ...float64) []domain.TitikKpi {
	akhir := time.Date(2026, 10, 16, 0, 0, 0, 0, time.UTC)
	out := make([]domain.TitikKpi, len(nilai))
	for i, v := range nilai {
		out[i] = domain.TitikKpi{Indikator: indikator, Tanggal: akhir.AddDate(0, 0, i-len(nilai)+1), Nilai: v}
	}
	return out
}

func naik(dari, sampai float64, n int) []float64 {
	out := make([]float64, n)
	for i := range out {
		out[i] = dari + (sampai-dari)*float64(i)/float64(n-1)
	}
	return out
}

func TestRakitKpi_OrderLabelsAndSparkline(t *testing.T) {
	var titik []domain.TitikKpi
	for _, indikator := range []string{domain.IndikatorCS, domain.IndikatorEntri, domain.IndikatorWaspada, domain.IndikatorValidasi} {
		titik = append(titik, deret(indikator, naik(1, 14, 14)...)...)
	}

	kpis := RakitKpi(titik, nil)

	require.Len(t, kpis, 4)
	assert.Equal(t, []string{"entri", "validasi", "waspada", "cs"}, []string{kpis[0].ID, kpis[1].ID, kpis[2].ID, kpis[3].ID})
	for _, k := range kpis {
		assert.Len(t, k.Tren, 14, k.ID)
		assert.Equal(t, 14.0, k.Nilai, "nilai adalah titik terbaru: %s", k.ID)
		assert.Equal(t, "data", k.Tone, "tanpa target warna netral: %s", k.ID)
		assert.NotEmpty(t, k.Label)
		assert.NotEmpty(t, k.Href)
	}
}

func TestRakitKpi_SortsUnorderedSeries(t *testing.T) {
	titik := deret(domain.IndikatorEntri, naik(1, 14, 14)...)
	titik[0], titik[13] = titik[13], titik[0]

	kpis := RakitKpi(titik, nil)

	require.Len(t, kpis, 1)
	assert.Equal(t, 14.0, kpis[0].Nilai)
	assert.Equal(t, 1.0, kpis[0].Tren[0])
}

func TestRakitKpi_SkipsIndicatorsWithoutSeries(t *testing.T) {
	assert.Empty(t, RakitKpi(nil, nil))
	assert.Len(t, RakitKpi(deret(domain.IndikatorCS, 50), nil), 1)
}

func TestRakitKpi_DeltaPerIndicator(t *testing.T) {
	t.Run("entri: persen 7 hari terakhir terhadap 7 hari sebelumnya", func(t *testing.T) {
		nilai := append(make([]float64, 0, 14), 100, 100, 100, 100, 100, 100, 100, 110, 110, 110, 110, 110, 110, 110)
		k := RakitKpi(deret(domain.IndikatorEntri, nilai...), nil)[0]
		assert.Equal(t, 10.0, k.Delta)
	})
	t.Run("entri: pembanding kosong memberi nol", func(t *testing.T) {
		nilai := append(make([]float64, 7), 5, 5, 5, 5, 5, 5, 5)
		assert.Zero(t, RakitKpi(deret(domain.IndikatorEntri, nilai...), nil)[0].Delta)
	})
	t.Run("validasi: poin terhadap sepekan lalu", func(t *testing.T) {
		nilai := naik(80, 80, 14)
		nilai[6], nilai[13] = 82.5, 87.5 // 7 hari sebelum titik terakhir adalah indeks 6
		assert.Equal(t, 5.0, RakitKpi(deret(domain.IndikatorValidasi, nilai...), nil)[0].Delta)
	})
	t.Run("waspada: selisih dengan kemarin", func(t *testing.T) {
		nilai := make([]float64, 14)
		nilai[12], nilai[13] = 2, 5
		assert.Equal(t, 3.0, RakitKpi(deret(domain.IndikatorWaspada, nilai...), nil)[0].Delta)
	})
	t.Run("deret pendek memberi nol", func(t *testing.T) {
		assert.Zero(t, RakitKpi(deret(domain.IndikatorCS, 10, 20, 30), nil)[0].Delta)
		assert.Zero(t, RakitKpi(deret(domain.IndikatorWaspada, 4), nil)[0].Delta)
		assert.Zero(t, RakitKpi(deret(domain.IndikatorEntri, 1, 2, 3), nil)[0].Delta)
	})
}

func TestRakitKpi_ToneFromTarget(t *testing.T) {
	cases := []struct {
		nama      string
		indikator string
		nilai     float64
		target    domain.TargetKpi
		tone      string
	}{
		{"naik tercapai", domain.IndikatorValidasi, 90, domain.TargetKpi{Indikator: "validasi", Target: 85, Arah: domain.ArahNaik}, "normal"},
		{"naik hampir", domain.IndikatorValidasi, 70, domain.TargetKpi{Indikator: "validasi", Target: 85, Arah: domain.ArahNaik}, "waspada"},
		{"naik jauh", domain.IndikatorValidasi, 50, domain.TargetKpi{Indikator: "validasi", Target: 85, Arah: domain.ArahNaik}, "defisit"},
		{"turun tercapai", domain.IndikatorWaspada, 2, domain.TargetKpi{Indikator: "waspada", Target: 3, Arah: domain.ArahTurun}, "normal"},
		{"turun nol selalu baik", domain.IndikatorWaspada, 0, domain.TargetKpi{Indikator: "waspada", Target: 3, Arah: domain.ArahTurun}, "normal"},
		{"turun melewati batas", domain.IndikatorWaspada, 6, domain.TargetKpi{Indikator: "waspada", Target: 3, Arah: domain.ArahTurun}, "defisit"},
	}
	for _, c := range cases {
		t.Run(c.nama, func(t *testing.T) {
			kpis := RakitKpi(deret(c.indikator, c.nilai), []domain.TargetKpi{c.target})
			require.Len(t, kpis, 1)
			assert.Equal(t, c.tone, kpis[0].Tone)
		})
	}
}

func TestRakitKpi_RoundsToOneDecimal(t *testing.T) {
	k := RakitKpi(deret(domain.IndikatorValidasi, 87.4567), nil)[0]
	assert.Equal(t, 87.5, k.Nilai)
	assert.Equal(t, []float64{87.5}, k.Tren)
}

func TestGetKpiOperasional(t *testing.T) {
	repo := new(mockRepository)
	uc := New(repo, logger.NewDefault())
	sekarang := time.Date(2026, 10, 16, 20, 0, 0, 0, time.UTC) // sudah 17 Oktober 03.00 WIB
	uc.now = func() time.Time { return sekarang }
	repo.On("DeretKpi", mock.Anything).Return(deret(domain.IndikatorEntri, 10), nil)
	repo.On("TargetKpi", mock.Anything, sekarang.In(wib)).Return([]domain.TargetKpi{}, nil)

	kpis, err := uc.GetKpiOperasional(t.Context())

	require.NoError(t, err)
	assert.Len(t, kpis, 1)
	repo.AssertExpectations(t)
}

func TestGetKpiOperasional_Errors(t *testing.T) {
	repo := new(mockRepository)
	uc := New(repo, logger.NewDefault())
	repo.On("DeretKpi", mock.Anything).Return(nil, errors.New("db mati")).Once()

	_, err := uc.GetKpiOperasional(t.Context())
	require.Error(t, err)

	repo.On("DeretKpi", mock.Anything).Return(deret(domain.IndikatorEntri, 1), nil)
	repo.On("TargetKpi", mock.Anything, mock.Anything).Return(nil, errors.New("db mati"))
	_, err = uc.GetKpiOperasional(t.Context())
	require.Error(t, err)
}

func TestGetAktivitasLapangan_UsesTodayInAcehTime(t *testing.T) {
	repo := new(mockRepository)
	uc := New(repo, logger.NewDefault())
	// 16 Oktober 19.00 UTC = 17 Oktober 02.00 WIB: "hari ini" adalah 17 Oktober WIB.
	uc.now = func() time.Time { return time.Date(2026, 10, 16, 19, 0, 0, 0, time.UTC) }
	awal := time.Date(2026, 10, 17, 0, 0, 0, 0, wib)
	repo.On("AktivitasLapangan", mock.Anything, awal, awal.AddDate(0, 0, 1)).
		Return([]domain.AktivitasKabupaten{{KabupatenKode: "1171", Entri: 10, Target: 14}}, nil)

	items, err := uc.GetAktivitasLapangan(t.Context())

	require.NoError(t, err)
	require.Len(t, items, 1)
	repo.AssertExpectations(t)
}

func TestGetHeatmap(t *testing.T) {
	repo := new(mockRepository)
	uc := New(repo, logger.NewDefault())
	repo.On("Heatmap", mock.Anything).Return(&domain.HeatmapData{Sel: []domain.SelHeatmap{{KomoditasKode: "BWM"}}}, nil).Once()

	data, err := uc.GetHeatmap(t.Context())
	require.NoError(t, err)
	assert.Len(t, data.Sel, 1)

	repo.On("Heatmap", mock.Anything).Return(nil, errors.New("db mati"))
	_, err = uc.GetHeatmap(t.Context())
	assert.Error(t, err)
}
