package usecase

import (
	"context"
	"errors"
	"fmt"

	"github.com/garda-inflasi/api/internal/intervensi/domain"
	"github.com/garda-inflasi/api/pkg/logger"
)

var (
	ErrNotFound = errors.New("kartu intervensi tidak ditemukan")
	ErrInvalidTahap = errors.New("tahap tidak valid")
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
