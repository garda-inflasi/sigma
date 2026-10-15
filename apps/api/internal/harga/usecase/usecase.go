package usecase

import (
	"context"
	"fmt"
	"time"

	"github.com/garda-inflasi/api/internal/harga/domain"
	"github.com/garda-inflasi/api/pkg/logger"
	"github.com/google/uuid"
)

// ── Errors ────────────────────────────────────────────────────────────

var (
	ErrNotFound      = fmt.Errorf("data tidak ditemukan")
	ErrInvalidAction = fmt.Errorf("aksi tidak valid")
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
func (uc *UseCase) ListHarga(ctx context.Context, limit int, status string) ([]domain.EntriHarga, error) {
	if limit <= 0 || limit > 1000 {
		limit = 500
	}

	entries, err := uc.repo.List(ctx, limit, status)
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

// CreateHarga creates a new harga entry.
func (uc *UseCase) CreateHarga(ctx context.Context, entri *domain.EntriHarga) error {
	if entri.ID == "" {
		entri.ID = uuid.New().String()
	}
	entri.Status = "DIAJUKAN"
	entri.DibuatPada = time.Now()
	entri.DiupdatePada = time.Now()

	if err := uc.repo.Create(ctx, entri); err != nil {
		uc.log.Error("create harga failed", map[string]interface{}{"error": err.Error()})
		return fmt.Errorf("create harga: %w", err)
	}

	uc.log.Info("harga created", map[string]interface{}{"id": entri.ID, "komoditas": entri.KomoditasKode})
	return nil
}

// GetHargaByID returns a harga entry by its ID.
func (uc *UseCase) GetHargaByID(ctx context.Context, id string) (*domain.EntriHarga, error) {
	entri, err := uc.repo.GetByID(ctx, id)
	if err != nil {
		uc.log.Warn("harga not found", map[string]interface{}{"id": id, "error": err.Error()})
		return nil, ErrNotFound
	}
	return entri, nil
}

// BulkAction processes a bulk action on harga entries.
func (uc *UseCase) BulkAction(ctx context.Context, ids []string, action string, userID string) (int64, error) {
	domainAction := domain.BulkAction(action)
	switch domainAction {
	case domain.BulkValidasi, domain.BulkPublikasi, domain.BulkTolak:
		// valid
	default:
		return 0, ErrInvalidAction
	}

	count, err := uc.repo.BulkAction(ctx, ids, domainAction, userID)
	if err != nil {
		uc.log.Error("bulk action failed", map[string]interface{}{
			"action": action,
			"count":  len(ids),
			"error":  err.Error(),
		})
		return 0, fmt.Errorf("bulk action: %w", err)
	}

	uc.log.Info("bulk action completed", map[string]interface{}{
		"action": action,
		"count":  count,
	})
	return count, nil
}
