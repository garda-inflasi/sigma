package usecase

import (
	"context"
	"errors"
	"fmt"

	"github.com/garda-inflasi/api/internal/cold-storage/domain"
	"github.com/garda-inflasi/api/pkg/logger"
)

var ErrInvalidAction = errors.New("aksi tidak valid")

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

// BulkAction processes a bulk action on cold storage items.
func (uc *UseCase) BulkAction(ctx context.Context, ids []string, action string) (int64, error) {
	domainAction := domain.BulkAction(action)
	switch domainAction {
	case domain.BulkRencanakanDistribusi, domain.BulkRekomendasiOlahan:
		// valid
	default:
		return 0, ErrInvalidAction
	}

	count, err := uc.repo.BulkAction(ctx, ids, domainAction)
	if err != nil {
		uc.log.Error("cold-storage: bulk action failed", map[string]interface{}{"action": action, "error": err.Error()})
		return 0, fmt.Errorf("bulk action: %w", err)
	}
	return count, nil
}
