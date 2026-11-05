package usecase

import (
	"context"
	"errors"
	"fmt"
	"slices"

	"github.com/garda-inflasi/api/internal/audit"
	"github.com/garda-inflasi/api/internal/sumber-data/domain"
	"github.com/garda-inflasi/api/pkg/logger"
)

var (
	ErrNotFound      = domain.ErrNotFound
	ErrInvalidAction = errors.New("aksi tidak valid")
)

// BulkResult: Berhasil adalah jumlah sumber yang berubah; Gagal sisanya dari id yang diminta.
type BulkResult struct {
	Berhasil, Gagal int
}

type UseCase struct {
	repo  domain.Repository
	audit audit.Writer
	log   *logger.Logger
}

func New(repo domain.Repository, auditWriter audit.Writer, log *logger.Logger) *UseCase {
	return &UseCase{repo: repo, audit: auditWriter, log: log}
}

func (uc *UseCase) List(ctx context.Context) ([]domain.SumberData, error) {
	return uc.repo.List(ctx)
}

func (uc *UseCase) GetByID(ctx context.Context, id string) (*domain.SumberData, error) {
	return uc.repo.GetByID(ctx, id)
}

// BulkAction menjeda atau melanjutkan sumber dan mencatatnya di audit trail.
// Id ganda dihitung sekali.
func (uc *UseCase) BulkAction(ctx context.Context, ids []string, action, actorID string) (BulkResult, error) {
	domainAction := domain.BulkAction(action)
	if domainAction != domain.BulkJeda && domainAction != domain.BulkLanjutkan {
		return BulkResult{}, ErrInvalidAction
	}

	ids = slices.Compact(slices.Sorted(slices.Values(ids)))
	changed, err := uc.repo.BulkAction(ctx, ids, domainAction)
	if err != nil {
		return BulkResult{}, fmt.Errorf("bulk sumber data: %w", err)
	}

	err = uc.audit.Write(ctx, audit.Entry{
		AktorID: actorID, Aksi: "BULK_" + action, Entitas: "sumber_data", EntitasID: fmt.Sprintf("%d sumber", len(changed)),
		Hasil: audit.HasilBerhasil, Metadata: map[string]any{"diminta": ids, "berubah": changed},
	})
	if err != nil {
		uc.log.Warn("sumber-data: gagal mencatat audit", map[string]interface{}{"aksi": action, "error": err.Error()})
	}
	return BulkResult{Berhasil: len(changed), Gagal: len(ids) - len(changed)}, nil
}
