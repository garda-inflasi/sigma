package usecase

import (
	"context"
	"errors"
	"fmt"

	"github.com/garda-inflasi/api/internal/sumber-data/domain"
)

var (
	ErrNotFound      = errors.New("sumber data tidak ditemukan")
	ErrInvalidAction = errors.New("aksi tidak valid")
)

type UseCase struct {
	repo domain.Repository
}

func New(repo domain.Repository) *UseCase {
	return &UseCase{repo: repo}
}

func (uc *UseCase) List(ctx context.Context) ([]domain.SumberData, error) {
	return uc.repo.List(ctx)
}

func (uc *UseCase) GetByID(ctx context.Context, id int64) (*domain.SumberData, error) {
	item, err := uc.repo.GetByID(ctx, id)
	if err != nil {
		return nil, ErrNotFound
	}
	return item, nil
}

func (uc *UseCase) BulkAction(ctx context.Context, ids []int64, action string) (int64, error) {
	domainAction := domain.BulkAction(action)
	switch domainAction {
	case domain.BulkJalankan, domain.BulkJeda, domain.BulkLanjutkan:
		// valid
	default:
		return 0, ErrInvalidAction
	}

	count, err := uc.repo.BulkAction(ctx, ids, domainAction)
	if err != nil {
		return 0, fmt.Errorf("bulk action: %w", err)
	}
	return count, nil
}
