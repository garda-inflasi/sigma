package usecase

import (
	"context"

	"github.com/garda-inflasi/api/internal/master/domain"
)

type UseCase struct {
	repo domain.Repository
}

func New(repo domain.Repository) *UseCase {
	return &UseCase{repo: repo}
}

func (uc *UseCase) ListKomoditas(ctx context.Context) ([]domain.Komoditas, error) {
	return uc.repo.ListKomoditas(ctx)
}
