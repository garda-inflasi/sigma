package usecase

import (
	"context"
	"errors"
	"fmt"

	"github.com/garda-inflasi/api/internal/ews/domain"
	"github.com/garda-inflasi/api/pkg/logger"
)

var (
	ErrNotFound      = errors.New("alert tidak ditemukan")
	ErrInvalidAction = errors.New("aksi tidak valid")
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

func (uc *UseCase) GetByID(ctx context.Context, id int64) (*domain.EwsAlert, error) {
	alert, err := uc.repo.GetByID(ctx, id)
	if err != nil {
		uc.log.Warn("ews alert not found", map[string]interface{}{"id": id, "error": err.Error()})
		return nil, ErrNotFound
	}

	return alert, nil
}

func (uc *UseCase) BulkAction(ctx context.Context, ids []int64, action string, userID string) (int64, error) {
	domainAction := domain.BulkAction(action)
	switch domainAction {
	case domain.BulkAkui, domain.BulkEskalasi, domain.BulkSelesai:
		// valid
	default:
		return 0, ErrInvalidAction
	}

	count, err := uc.repo.BulkAction(ctx, ids, domainAction, userID)
	if err != nil {
		uc.log.Error("ews bulk action failed", map[string]interface{}{
			"action": action,
			"count":  len(ids),
			"error":  err.Error(),
		})
		return 0, fmt.Errorf("bulk action: %w", err)
	}

	uc.log.Info("ews bulk action completed", map[string]interface{}{
		"action": action,
		"count":  count,
	})
	return count, nil
}
