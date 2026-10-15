package usecase

import (
	"context"

	"github.com/garda-inflasi/api/internal/spasial/domain"
)

type UseCase struct {
	repo domain.Repository
}

func New(repo domain.Repository) *UseCase {
	return &UseCase{repo: repo}
}

func (uc *UseCase) ListWilayah(ctx context.Context) ([]domain.Wilayah, error) {
	return uc.repo.ListWilayah(ctx)
}
