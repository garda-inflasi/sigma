package usecase

import (
	"context"
	"errors"
	"fmt"

	"github.com/garda-inflasi/api/internal/intervensi/domain"
	"github.com/garda-inflasi/api/pkg/logger"
)

var (
	ErrNotFound      = errors.New("kartu intervensi tidak ditemukan")
	ErrInvalidTahap  = errors.New("tahap tidak valid")
	ErrInvalidAction = errors.New("aksi tidak valid")
)

type UseCase struct {
	repo domain.Repository
	log  *logger.Logger
}

func New(repo domain.Repository, log *logger.Logger) *UseCase {
	return &UseCase{repo: repo, log: log}
}

func (uc *UseCase) List(ctx context.Context) ([]domain.KartuIntervensi, error) {
	kartu, err := uc.repo.List(ctx)
	if err != nil {
		uc.log.Error("list kartu intervensi failed", map[string]interface{}{"error": err.Error()})
		return nil, fmt.Errorf("list kartu intervensi: %w", err)
	}
	return kartu, nil
}

func (uc *UseCase) Create(ctx context.Context, kartu *domain.KartuIntervensi) error {
	kartu.Tahap = domain.TahapUsulan
	if err := uc.repo.Create(ctx, kartu); err != nil {
		uc.log.Error("create kartu intervensi failed", map[string]interface{}{"error": err.Error()})
		return fmt.Errorf("create kartu intervensi: %w", err)
	}
	uc.log.Info("kartu intervensi created", map[string]interface{}{"id": kartu.ID})
	return nil
}

func (uc *UseCase) UpdateTahap(ctx context.Context, id int64, tahap string, urutan int) error {
	t := domain.TahapEnum(tahap)
	switch t {
	case domain.TahapUsulan, domain.TahapKaji, domain.TahapSetuju, domain.TahapLaksana, domain.TahapSelesai, domain.TahapBatal:
	default:
		return ErrInvalidTahap
	}

	kartu, err := uc.repo.GetByID(ctx, id)
	if err != nil {
		uc.log.Warn("kartu intervensi not found", map[string]interface{}{"id": id})
		return ErrNotFound
	}

	if err := uc.repo.UpdateTahap(ctx, kartu.ID, t, urutan); err != nil {
		uc.log.Error("update tahap failed", map[string]interface{}{"id": id, "error": err.Error()})
		return fmt.Errorf("update tahap: %w", err)
	}

	uc.log.Info("tahap updated", map[string]interface{}{"id": id, "tahap": tahap, "urutan": urutan})
	return nil
}

func (uc *UseCase) BulkAction(ctx context.Context, ids []int64, action string) (int64, error) {
	domainAction := domain.BulkAction(action)
	switch domainAction {
	case domain.BulkArsip, domain.BulkHapus:
		// valid
	default:
		return 0, ErrInvalidAction
	}

	count, err := uc.repo.BulkAction(ctx, ids, domainAction)
	if err != nil {
		uc.log.Error("intervensi bulk action failed", map[string]interface{}{
			"action": action,
			"count":  len(ids),
			"error":  err.Error(),
		})
		return 0, fmt.Errorf("bulk action: %w", err)
	}

	uc.log.Info("intervensi bulk action completed", map[string]interface{}{
		"action": action,
		"count":  count,
	})
	return count, nil
}
