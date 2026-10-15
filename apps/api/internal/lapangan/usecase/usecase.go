package usecase

import (
	"context"

	"github.com/garda-inflasi/api/internal/lapangan/domain"
)

type UseCase struct {
	repo domain.Repository
}

func New(repo domain.Repository) *UseCase {
	return &UseCase{repo: repo}
}

func (uc *UseCase) ListSaya(ctx context.Context, pic string) ([]domain.TugasLapangan, error) {
	return uc.repo.ListSaya(ctx, pic)
}
