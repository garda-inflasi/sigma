package usecase

import (
	"context"

	"github.com/garda-inflasi/api/internal/sumber-data/domain"
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
