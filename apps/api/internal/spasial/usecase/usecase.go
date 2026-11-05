// Package usecase berisi logika bisnis (baca-saja) modul spasial.
package usecase

import (
	"context"

	"github.com/garda-inflasi/api/internal/spasial/domain"
	"github.com/garda-inflasi/api/pkg/logger"
)

type UseCase struct {
	repo domain.Repository
	log  *logger.Logger
}

func New(repo domain.Repository, log *logger.Logger) *UseCase {
	return &UseCase{repo: repo, log: log}
}

func (uc *UseCase) ListMarker(ctx context.Context) ([]domain.Marker, error) {
	items, err := uc.repo.ListMarker(ctx)
	if err != nil {
		uc.log.Error("spasial: marker gagal", map[string]interface{}{"error": err.Error()})
	}
	return items, err
}

func (uc *UseCase) ListNeraca(ctx context.Context, q domain.NeracaQuery) ([]domain.Neraca, error) {
	items, err := uc.repo.ListNeraca(ctx, q)
	if err != nil {
		uc.log.Error("spasial: neraca gagal", map[string]interface{}{"error": err.Error()})
	}
	return items, err
}

func (uc *UseCase) ListRute(ctx context.Context) ([]domain.Rute, error) {
	items, err := uc.repo.ListRute(ctx)
	if err != nil {
		uc.log.Error("spasial: rute gagal", map[string]interface{}{"error": err.Error()})
	}
	return items, err
}

func (uc *UseCase) ListKandidat(ctx context.Context) ([]domain.Kandidat, error) {
	items, err := uc.repo.ListKandidat(ctx)
	if err != nil {
		uc.log.Error("spasial: kandidat gagal", map[string]interface{}{"error": err.Error()})
	}
	return items, err
}
