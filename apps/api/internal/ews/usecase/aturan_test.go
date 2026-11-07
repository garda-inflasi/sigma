package usecase

import (
	"context"
	"testing"

	"github.com/garda-inflasi/api/internal/ews/domain"
	"github.com/garda-inflasi/api/pkg/idx"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

func aturanContoh(versi int, kritis float64) *domain.Aturan {
	return &domain.Aturan{AturanEws: domain.AturanEws{
		ID: 2, AmbangWaspada: 8, AmbangSiaga: 12, AmbangKritis: kritis, JendelaHari: 7, CooldownJam: 24,
		Kanal: domain.Kanal{"DASHBOARD"}, Aktif: true, Versi: versi,
	}}
}

func TestUpdateAturan_RejectsInvalidThresholdsWithoutTouchingRepository(t *testing.T) {
	for name, upd := range map[string]domain.AturanUpdate{
		"sama":    {AmbangWaspada: 10, AmbangSiaga: 10, AmbangKritis: 20},
		"menurun": {AmbangWaspada: 20, AmbangSiaga: 10, AmbangKritis: 5},
		"nol":     {AmbangWaspada: 0, AmbangSiaga: 10, AmbangKritis: 20},
		"negatif": {AmbangWaspada: -1, AmbangSiaga: 10, AmbangKritis: 20},
	} {
		t.Run(name, func(t *testing.T) {
			uc, recorder := newTestUseCase(new(mockRepository))

			_, err := uc.UpdateAturan(context.Background(), idx.RujukanID{ID: 2}, upd, "u")

			assert.ErrorIs(t, err, ErrAmbangTidakValid)
			assert.Empty(t, recorder.Entries())
		})
	}
}

func TestUpdateAturan_SavesAndAuditsBeforeAndAfter(t *testing.T) {
	repo := new(mockRepository)
	uc, recorder := newTestUseCase(repo)
	upd := domain.AturanUpdate{Versi: 1, AmbangWaspada: 8, AmbangSiaga: 12, AmbangKritis: 25, JendelaHari: 7, CooldownJam: 24, Kanal: domain.Kanal{"DASHBOARD"}, Aktif: true}
	repo.On("GetAturan", mock.Anything, idx.RujukanID{ID: 2}).Return(aturanContoh(1, 18), nil)
	repo.On("UpdateAturan", mock.Anything, idx.ID(2), upd, uid("user-001")).Return(aturanContoh(2, 25), nil)

	got, err := uc.UpdateAturan(context.Background(), idx.RujukanID{ID: 2}, upd, uid("user-001"))

	require.NoError(t, err)
	assert.Equal(t, 2, got.Versi)
	entry, ok := recorder.Last()
	require.True(t, ok)
	assert.Equal(t, "UBAH_AMBANG", entry.Aksi)
	assert.Equal(t, "2", entry.EntitasID, "audit mencatat ID aturan (desimal)")
	assert.Equal(t, uid("user-001"), entry.AktorID)
	assert.EqualValues(t, 18, entry.Sebelum.(map[string]any)["ambang_kritis"])
	assert.EqualValues(t, 25, entry.Sesudah.(map[string]any)["ambang_kritis"])
}

func TestUpdateAturan_KodeLamaDiselesaikanKeIDDanAuditMemakaiID(t *testing.T) {
	repo := new(mockRepository)
	uc, recorder := newTestUseCase(repo)
	upd := domain.AturanUpdate{Versi: 1, AmbangWaspada: 8, AmbangSiaga: 12, AmbangKritis: 25}
	repo.On("GetAturan", mock.Anything, idx.RujukanID{Kode: "ATR-LH-BWM"}).Return(aturanContoh(1, 18), nil)
	repo.On("UpdateAturan", mock.Anything, idx.ID(2), upd, "u").Return(aturanContoh(2, 25), nil)

	_, err := uc.UpdateAturan(context.Background(), idx.RujukanID{Kode: "ATR-LH-BWM"}, upd, "u")

	require.NoError(t, err)
	repo.AssertExpectations(t)
	entry, _ := recorder.Last()
	assert.Equal(t, "aturan_ews", entry.Entitas)
	assert.Equal(t, "2", entry.EntitasID, "kode lama tidak ditulis ke audit; ID aturan yang ditulis")
}

func TestUpdateAturan_StaleVersionAndMissingRule(t *testing.T) {
	upd := domain.AturanUpdate{Versi: 1, AmbangWaspada: 8, AmbangSiaga: 12, AmbangKritis: 25}

	t.Run("versi usang", func(t *testing.T) {
		repo := new(mockRepository)
		uc, recorder := newTestUseCase(repo)
		repo.On("GetAturan", mock.Anything, idx.RujukanID{ID: 2}).Return(aturanContoh(2, 18), nil)
		repo.On("UpdateAturan", mock.Anything, idx.ID(2), upd, "u").Return(nil, domain.ErrVersiUsang)

		_, err := uc.UpdateAturan(context.Background(), idx.RujukanID{ID: 2}, upd, "u")

		assert.ErrorIs(t, err, ErrVersiUsang)
		assert.Empty(t, recorder.Entries(), "perubahan yang ditolak tidak dicatat sebagai perubahan")
	})
	t.Run("aturan tidak ada", func(t *testing.T) {
		repo := new(mockRepository)
		uc, _ := newTestUseCase(repo)
		repo.On("GetAturan", mock.Anything, idx.RujukanID{ID: 99}).Return(nil, domain.ErrAturanNotFound)

		_, err := uc.UpdateAturan(context.Background(), idx.RujukanID{ID: 99}, upd, "u")

		assert.ErrorIs(t, err, ErrAturanNotFound)
	})
}

type fakeEvaluator struct {
	ringkasan domain.RingkasanEvaluasi
	err       error
}

func (f fakeEvaluator) Evaluasi(context.Context) (domain.RingkasanEvaluasi, error) {
	return f.ringkasan, f.err
}

func TestEvaluasi_AuditsSummaryAndPropagatesErrors(t *testing.T) {
	uc, recorder := newTestUseCase(new(mockRepository))
	uc.evaluator = fakeEvaluator{ringkasan: domain.RingkasanEvaluasi{Aturan: 9, Dibuat: 2}}

	got, err := uc.Evaluasi(context.Background(), uid("user-001"))

	require.NoError(t, err)
	assert.Equal(t, 2, got.Dibuat)
	entry, _ := recorder.Last()
	assert.Equal(t, "EVALUASI", entry.Aksi)

	uc, recorder = newTestUseCase(new(mockRepository))
	uc.evaluator = fakeEvaluator{err: assert.AnError}
	_, err = uc.Evaluasi(context.Background(), uid("user-001"))
	assert.ErrorIs(t, err, assert.AnError)
	assert.Empty(t, recorder.Entries())
}

func TestListRiwayat(t *testing.T) {
	t.Run("alert tidak ada", func(t *testing.T) {
		repo := new(mockRepository)
		uc, _ := newTestUseCase(repo)
		repo.On("GetByID", mock.Anything, idx.RujukanID{ID: 99}).Return(nil, gorm.ErrRecordNotFound)

		_, err := uc.ListRiwayat(context.Background(), idx.RujukanID{ID: 99})

		assert.ErrorIs(t, err, ErrNotFound)
	})
	t.Run("mengembalikan riwayat", func(t *testing.T) {
		repo := new(mockRepository)
		uc, _ := newTestUseCase(repo)
		alert := sampleAlert()
		repo.On("GetByID", mock.Anything, idx.RujukanID{Kode: "EWS-1000"}).Return(&alert, nil)
		repo.On("ListRiwayat", mock.Anything, idx.ID(1000)).Return([]domain.Riwayat{{EwsRiwayat: domain.EwsRiwayat{Ke: "BARU"}}}, nil)

		items, err := uc.ListRiwayat(context.Background(), idx.RujukanID{Kode: "EWS-1000"})

		require.NoError(t, err)
		assert.Len(t, items, 1)
	})
}
