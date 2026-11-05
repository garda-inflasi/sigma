// Package usecase berisi logika bisnis modul EWS.
package usecase

import (
	"context"
	"errors"
	"fmt"
	"slices"
	"strings"

	"github.com/garda-inflasi/api/internal/audit"
	"github.com/garda-inflasi/api/internal/ews/domain"
	"github.com/garda-inflasi/api/pkg/logger"
	"gorm.io/gorm"
)

var (
	ErrNotFound      = errors.New("alert tidak ditemukan")
	ErrInvalidAction = errors.New("aksi tidak valid")
)

// Batas ukuran halaman daftar alert.
const (
	DefaultLimit = 200
	MaxLimit     = 1000
)

// BulkResult melaporkan hasil aksi massal. Gagal mencakup id yang tidak ada dan
// alert yang statusnya tidak boleh bertransisi.
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

// List mengembalikan satu halaman alert (terbaru dahulu) dan jumlah seluruh hasil.
func (uc *UseCase) List(ctx context.Context, q domain.ListQuery) ([]domain.Alert, int64, error) {
	if q.Page < 1 {
		q.Page = 1
	}
	if q.Limit < 1 || q.Limit > MaxLimit {
		q.Limit = DefaultLimit
	}
	items, total, err := uc.repo.List(ctx, q)
	if err != nil {
		uc.log.Error("ews: daftar alert gagal", map[string]interface{}{"error": err.Error()})
		return nil, 0, err
	}
	return items, total, nil
}

func (uc *UseCase) GetByID(ctx context.Context, id string) (*domain.Alert, error) {
	alert, err := uc.repo.GetByID(ctx, id)
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, ErrNotFound
	}
	if err != nil {
		uc.log.Error("ews: ambil alert gagal", map[string]interface{}{"id": id, "error": err.Error()})
		return nil, err
	}
	return alert, nil
}

// ListAturan mengembalikan aturan ambang beserta kanal notifikasinya.
func (uc *UseCase) ListAturan(ctx context.Context) ([]domain.Aturan, error) {
	items, err := uc.repo.ListAturan(ctx)
	if err != nil {
		uc.log.Error("ews: daftar aturan gagal", map[string]interface{}{"error": err.Error()})
		return nil, err
	}
	return items, nil
}

// BulkAction menerapkan aksi massal dan mencatatnya di audit trail.
func (uc *UseCase) BulkAction(ctx context.Context, ids []string, action, actorID string) (BulkResult, error) {
	domainAction := domain.BulkAction(action)
	switch domainAction {
	case domain.BulkAkui, domain.BulkEskalasi, domain.BulkSelesai:
	default:
		return BulkResult{}, ErrInvalidAction
	}

	ids = unique(ids)
	changed, err := uc.repo.BulkAction(ctx, ids, domainAction, actorID)
	if err != nil {
		uc.log.Error("ews: aksi massal gagal", map[string]interface{}{"aksi": action, "jumlah": len(ids), "error": err.Error()})
		return BulkResult{}, fmt.Errorf("aksi massal: %w", err)
	}
	result := BulkResult{Berhasil: len(changed), Gagal: len(ids) - len(changed)}
	if len(changed) == 0 {
		return result, nil
	}

	err = uc.audit.Write(ctx, audit.Entry{
		AktorID: actorID, Aksi: "BULK_" + strings.ToUpper(action), Entitas: "ews_alert",
		EntitasID: fmt.Sprintf("%d alert", len(changed)), Hasil: audit.HasilBerhasil,
		Metadata: map[string]any{"ids": changed, "diminta": len(ids)},
	})
	if err != nil {
		uc.log.Warn("ews: gagal mencatat audit", map[string]interface{}{"aksi": action, "error": err.Error()})
	}
	return result, nil
}

// unique membuang id ganda dengan tetap menjaga urutan.
func unique(ids []string) []string {
	out := make([]string, 0, len(ids))
	for _, id := range ids {
		if !slices.Contains(out, id) {
			out = append(out, id)
		}
	}
	return out
}
