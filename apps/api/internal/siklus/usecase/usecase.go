package usecase

import (
	"context"
	"errors"
	"fmt"

	"github.com/garda-inflasi/api/internal/siklus/domain"
	"github.com/garda-inflasi/api/pkg/logger"
)

var (
	ErrSiklusNotFound = errors.New("siklus aktif tidak ditemukan untuk wilayah ini")
)

type UseCase struct {
	repo domain.Repository
	log  *logger.Logger
}

func New(repo domain.Repository, log *logger.Logger) *UseCase {
	return &UseCase{
		repo: repo,
		log:  log,
	}
}

// GetAktif returns the currently active economic cycle for a region.
// Wilayah kosong berarti provinsi Aceh.
func (uc *UseCase) GetAktif(ctx context.Context, wilayahKode string) (*domain.SiklusEkonomi, error) {
	if wilayahKode == "" {
		wilayahKode = domain.WilayahBawaan
	}
	siklus, err := uc.repo.GetActiveByWilayah(ctx, wilayahKode)
	if err != nil {
		if errors.Is(err, domain.ErrNotFound) {
			return nil, ErrSiklusNotFound
		}
		uc.log.Error("siklus: gagal membaca siklus aktif", map[string]interface{}{"wilayah_kode": wilayahKode, "error": err.Error()})
		return nil, fmt.Errorf("get siklus aktif: %w", err)
	}
	return siklus, nil
}
