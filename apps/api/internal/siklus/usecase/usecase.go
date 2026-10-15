package usecase

import (
	"context"
	"errors"

	"github.com/garda-inflasi/api/internal/siklus/domain"
	"github.com/garda-inflasi/api/pkg/logger"
)

var (
	ErrSiklusNotFound = errors.New("siklus aktif tidak ditemukan untuk wilayah ini")
)

type UseCase struct {
	repo domain.Repository
	log  *logger.Logger
}

func New(repo domain.Repository, log *logger.Logger) *UseCase {
	return &UseCase{
		repo: repo,
		log:  log,
	}
}

// GetAktif returns the currently active economic cycle for a region.
func (uc *UseCase) GetAktif(ctx context.Context, wilayahKode string) (*domain.SiklusEkonomi, error) {
	siklus, err := uc.repo.GetActiveByWilayah(ctx, wilayahKode)
	if err != nil {
		uc.log.Warn("siklus: active cycle not found", map[string]interface{}{"wilayah_kode": wilayahKode})
		return nil, ErrSiklusNotFound
	}
	return siklus, nil
}
