package usecase

import (
	"context"
	"fmt"

	"github.com/garda-inflasi/api/internal/harga/domain"
	"github.com/garda-inflasi/api/pkg/logger"
)

// ── Errors ────────────────────────────────────────────────────────────

var (
	ErrNotFound = fmt.Errorf("data tidak ditemukan")
)

// ── UseCase ───────────────────────────────────────────────────────────

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

// ListHarga returns a list of harga entries.
func (uc *UseCase) ListHarga(ctx context.Context, limit int) ([]domain.EntriHarga, error) {
	if limit <= 0 || limit > 1000 {
		limit = 500
	}

	entries, err := uc.repo.List(ctx, limit)
	if err != nil {
		uc.log.Error("list harga failed", map[string]interface{}{"error": err.Error()})
		return nil, fmt.Errorf("list harga: %w", err)
	}

	return entries, nil
}

// GetTren returns trend data for a commodity in a region.
func (uc *UseCase) GetTren(ctx context.Context, komoditas, wilayah string) ([]domain.TrenHarga, error) {
	if komoditas == "" || wilayah == "" {
		return nil, fmt.Errorf("komoditas dan wilayah wajib diisi")
	}

	tren, err := uc.repo.GetTren(ctx, komoditas, wilayah)
	if err != nil {
		uc.log.Error("get tren failed", map[string]interface{}{
			"komoditas": komoditas,
			"wilayah":   wilayah,
			"error":     err.Error(),
		})
		return nil, fmt.Errorf("get tren: %w", err)
	}

	return tren, nil
}
