package usecase

import (
	"context"

	"github.com/garda-inflasi/api/internal/audit/domain"
)

type UseCase struct {
	repo domain.Repository
}

func New(repo domain.Repository) *UseCase {
	return &UseCase{repo: repo}
}

func (uc *UseCase) List(ctx context.Context, limit int) ([]domain.AuditLog, error) {
	if limit < 1 || limit > 500 {
		limit = 500
	}
	return uc.repo.List(ctx, limit)
}
