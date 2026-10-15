package usecase

import (
	"context"
	"errors"

	"github.com/garda-inflasi/api/internal/prediksi/domain"
	"github.com/garda-inflasi/api/pkg/logger"
)

var (
	ErrNotFound = errors.New("proyeksi tidak ditemukan")
)

type UseCase struct {
	repo domain.Repository
	log  *logger.Logger
}

func New(repo domain.Repository, log *logger.Logger) *UseCase {
	return &UseCase{repo: repo, log: log}
}

func (uc *UseCase) GetByKode(ctx context.Context, kode string) (*domain.ProyeksiXai, error) {
	proyeksi, err := uc.repo.GetByKode(ctx, kode)
	if err != nil {
		uc.log.Warn("prediksi: get by kode not found", map[string]interface{}{"kode": kode})
		return nil, ErrNotFound
	}
	return proyeksi, nil
}

func (uc *UseCase) List(ctx context.Context, page, limit int) ([]domain.ProyeksiXai, int64, error) {
	return uc.repo.List(ctx, page, limit)
}
