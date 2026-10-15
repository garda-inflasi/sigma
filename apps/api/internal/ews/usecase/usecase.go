package usecase

import (
	"context"

	"github.com/garda-inflasi/api/internal/ews/domain"
	"github.com/garda-inflasi/api/pkg/logger"
)

type UseCase struct {
	repo domain.Repository
	log  *logger.Logger
}

func New(repo domain.Repository, log *logger.Logger) *UseCase {
	return &UseCase{repo: repo, log: log}
}

func (uc *UseCase) List(ctx context.Context, limit int) ([]domain.EwsAlert, error) {
	if limit < 1 || limit > 1000 {
		limit = 200
	}

	alerts, err := uc.repo.List(ctx, limit)
	if err != nil {
		uc.log.Error("ews list failed", map[string]interface{}{"error": err.Error()})
		return nil, err
	}

	return alerts, nil
}
