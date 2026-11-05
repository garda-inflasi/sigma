// Package usecase implements the business logic for the master module.
package usecase

import (
	"context"
	"errors"
	"fmt"

	"github.com/garda-inflasi/api/internal/audit"
	"github.com/garda-inflasi/api/internal/master/domain"
	"github.com/garda-inflasi/api/pkg/logger"
)

var ErrInvalidAction = errors.New("aksi tidak valid")

type UseCase struct {
	repo  domain.Repository
	audit audit.Writer
	log   *logger.Logger
}

func New(repo domain.Repository, auditWriter audit.Writer, log *logger.Logger) *UseCase {
	return &UseCase{repo: repo, audit: auditWriter, log: log}
}

func (uc *UseCase) ListKomoditas(ctx context.Context, q domain.ListQuery) ([]domain.Komoditas, int64, error) {
	return uc.repo.ListKomoditas(ctx, q)
}

func (uc *UseCase) ListWilayah(ctx context.Context, provinsiKode string, q domain.ListQuery) ([]domain.Kabupaten, int64, error) {
	return uc.repo.ListWilayah(ctx, provinsiKode, q)
}

func (uc *UseCase) ListPasar(ctx context.Context, kabupatenKode string, q domain.ListQuery) ([]domain.Pasar, int64, error) {
	return uc.repo.ListPasar(ctx, kabupatenKode, q)
}

// BulkKomoditas menjalankan aksi massal pada komoditas dan mencatatnya di audit trail.
func (uc *UseCase) BulkKomoditas(ctx context.Context, kode []string, action, actorID string) (int64, error) {
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

	err = uc.audit.Write(ctx, audit.Entry{
		AktorID: actorID, Aksi: "BULK_" + action, Entitas: "komoditas", EntitasID: fmt.Sprintf("%d komoditas", count),
		Hasil: audit.HasilBerhasil, Metadata: map[string]any{"kode": kode},
	})
	if err != nil {
		uc.log.Warn("master: gagal mencatat audit", map[string]interface{}{"aksi": action, "error": err.Error()})
	}
	return count, nil
}
