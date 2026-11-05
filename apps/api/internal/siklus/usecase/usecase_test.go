package usecase

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/garda-inflasi/api/internal/siklus/domain"
	"github.com/garda-inflasi/api/internal/siklus/dto"
	"github.com/garda-inflasi/api/pkg/logger"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type fakeRepository struct {
	wilayah string
	siklus  *domain.SiklusEkonomi
	err     error
}

func (f *fakeRepository) GetActiveByWilayah(_ context.Context, wilayahKode string) (*domain.SiklusEkonomi, error) {
	f.wilayah = wilayahKode
	return f.siklus, f.err
}

func TestGetAktif_DefaultsToAceh(t *testing.T) {
	repo := &fakeRepository{siklus: &domain.SiklusEkonomi{Siklus: domain.SiklusTekanan}}
	uc := New(repo, logger.NewDefault())

	got, err := uc.GetAktif(t.Context(), "")

	require.NoError(t, err)
	assert.Equal(t, domain.WilayahBawaan, repo.wilayah)
	assert.Equal(t, domain.SiklusTekanan, got.Siklus)
}

func TestGetAktif_NotFoundVersusFailure(t *testing.T) {
	uc := New(&fakeRepository{err: domain.ErrNotFound}, logger.NewDefault())
	_, err := uc.GetAktif(t.Context(), "99")
	assert.ErrorIs(t, err, ErrSiklusNotFound)

	uc = New(&fakeRepository{err: errors.New("db mati")}, logger.NewDefault())
	_, err = uc.GetAktif(t.Context(), "11")
	require.Error(t, err)
	assert.NotErrorIs(t, err, ErrSiklusNotFound, "galat basis data bukan 404")
}

func TestToSiklusAktifResponse_LowercaseAndAcehDate(t *testing.T) {
	// 26 September 20.00 UTC sudah 27 September 03.00 WIB.
	res := dto.ToSiklusAktifResponse("TEKANAN", time.Date(2026, 9, 26, 20, 0, 0, 0, time.UTC))

	assert.Equal(t, "tekanan", res.Siklus)
	assert.Equal(t, "2026-09-27", res.Sejak)
}
