package usecase

import (
	"context"

	"github.com/garda-inflasi/api/internal/cold-storage/domain"
	"github.com/garda-inflasi/api/pkg/logger"
)

type UseCase struct {
	repo domain.Repository
	log  *logger.Logger
}

func New(repo domain.Repository, log *logger.Logger) *UseCase {
	return &UseCase{repo: repo, log: log}
}

// GetAll returns all fasilitas and stok data.
func (uc *UseCase) GetAll(ctx context.Context) ([]domain.FasilitasColdStorage, []domain.StokColdStorage, error) {
	fasilitas, err := uc.repo.GetAllFasilitas(ctx)
	if err != nil {
		uc.log.Error("cold-storage: failed to get fasilitas", map[string]interface{}{"error": err.Error()})
		return nil, nil, err
	}

	stok, err := uc.repo.GetAllStok(ctx)
	if err != nil {
		uc.log.Error("cold-storage: failed to get stok", map[string]interface{}{"error": err.Error()})
		return nil, nil, err
	}

	return fasilitas, stok, nil
}
