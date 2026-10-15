// Package usecase implements the business logic for the master module.
package usecase

import (
	"context"
	"errors"
	"fmt"

	"github.com/garda-inflasi/api/internal/master/domain"
)

var ErrInvalidAction = errors.New("aksi tidak valid")

type UseCase struct {
	repo domain.Repository
}

func New(repo domain.Repository) *UseCase {
	return &UseCase{repo: repo}
}

func (uc *UseCase) ListKomoditas(ctx context.Context) ([]domain.Komoditas, error) {
	return uc.repo.ListKomoditas(ctx)
}

func (uc *UseCase) ListWilayah(ctx context.Context, provinsiKode string) ([]domain.Kabupaten, error) {
	return uc.repo.ListWilayah(ctx, provinsiKode)
}

func (uc *UseCase) ListPasar(ctx context.Context, kabupatenKode string) ([]domain.Pasar, error) {
	return uc.repo.ListPasar(ctx, kabupatenKode)
}

func (uc *UseCase) BulkKomoditas(ctx context.Context, kode []string, action string) (int64, error) {
	domainAction := domain.BulkAction(action)
	switch domainAction {
	case domain.BulkAjukanPerubahan, domain.BulkSetujui, domain.BulkArsipkan:
		// valid
	default:
		return 0, ErrInvalidAction
	}

	count, err := uc.repo.BulkKomoditas(ctx, kode, domainAction)
	if err != nil {
		return 0, fmt.Errorf("bulk komoditas: %w", err)
	}
	return count, nil
}
