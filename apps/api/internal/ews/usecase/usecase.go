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
	// ErrAmbangTidakValid: ambang harus positif dan menaik (waspada < siaga < kritis).
	ErrAmbangTidakValid = errors.New("ambang harus positif dan menaik: waspada < siaga < kritis")

	ErrAturanNotFound = domain.ErrAturanNotFound
	ErrVersiUsang     = domain.ErrVersiUsang
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
	repo      domain.Repository
	evaluator domain.Evaluator
	audit     audit.Writer
	log       *logger.Logger
}

func New(repo domain.Repository, evaluator domain.Evaluator, auditWriter audit.Writer, log *logger.Logger) *UseCase {
	return &UseCase{repo: repo, evaluator: evaluator, audit: auditWriter, log: log}
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

// ListRiwayat mengembalikan perubahan status satu alert, terlama dahulu.
func (uc *UseCase) ListRiwayat(ctx context.Context, alertID string) ([]domain.Riwayat, error) {
	if _, err := uc.GetByID(ctx, alertID); err != nil {
		return nil, err
	}
	items, err := uc.repo.ListRiwayat(ctx, alertID)
	if err != nil {
		uc.log.Error("ews: riwayat alert gagal", map[string]interface{}{"id": alertID, "error": err.Error()})
		return nil, err
	}
	return items, nil
}

// UpdateAturan mengubah ambang (dan kanal, jendela, peredaman) suatu aturan dengan
// optimistic locking menurut versi, lalu mencatatnya di audit.
func (uc *UseCase) UpdateAturan(ctx context.Context, id string, upd domain.AturanUpdate, actorID string) (*domain.Aturan, error) {
	if upd.AmbangWaspada <= 0 || upd.AmbangWaspada >= upd.AmbangSiaga || upd.AmbangSiaga >= upd.AmbangKritis {
		return nil, ErrAmbangTidakValid
	}
	before, err := uc.repo.GetAturan(ctx, id)
	if err != nil {
		return nil, uc.aturanError(id, err)
	}
	after, err := uc.repo.UpdateAturan(ctx, id, upd, actorID)
	if err != nil {
		return nil, uc.aturanError(id, err)
	}
	uc.writeAudit(ctx, audit.Entry{
		AktorID: actorID, Aksi: "UBAH_AMBANG", Entitas: "aturan_ews", EntitasID: id, Hasil: audit.HasilBerhasil,
		Sebelum: ringkasAturan(before), Sesudah: ringkasAturan(after),
	})
	return after, nil
}

func (uc *UseCase) aturanError(id string, err error) error {
	if errors.Is(err, ErrAturanNotFound) || errors.Is(err, ErrVersiUsang) {
		return err
	}
	uc.log.Error("ews: aturan gagal diproses", map[string]interface{}{"id": id, "error": err.Error()})
	return err
}

// Evaluasi menjalankan rule engine sekarang juga (di luar jadwal) dan mencatatnya di audit.
func (uc *UseCase) Evaluasi(ctx context.Context, actorID string) (domain.RingkasanEvaluasi, error) {
	ringkasan, err := uc.evaluator.Evaluasi(ctx)
	if err != nil {
		uc.log.Error("ews: evaluasi manual gagal", map[string]interface{}{"error": err.Error()})
		return ringkasan, fmt.Errorf("evaluasi: %w", err)
	}
	uc.writeAudit(ctx, audit.Entry{
		AktorID: actorID, Aksi: "EVALUASI", Entitas: "ews_alert", EntitasID: "rule engine", Hasil: audit.HasilBerhasil,
		Metadata: map[string]any{"ringkasan": ringkasan},
	})
	return ringkasan, nil
}

// ringkasAturan memilih kolom aturan yang bermakna untuk jejak audit.
func ringkasAturan(a *domain.Aturan) map[string]any {
	return map[string]any{
		"versi": a.Versi, "ambang_waspada": a.AmbangWaspada, "ambang_siaga": a.AmbangSiaga, "ambang_kritis": a.AmbangKritis,
		"jendela_hari": a.JendelaHari, "cooldown_jam": a.CooldownJam, "kanal": []string(a.Kanal), "aktif": a.Aktif,
	}
}

func (uc *UseCase) writeAudit(ctx context.Context, entry audit.Entry) {
	if err := uc.audit.Write(ctx, entry); err != nil {
		uc.log.Warn("ews: gagal mencatat audit", map[string]interface{}{"aksi": entry.Aksi, "error": err.Error()})
	}
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

	uc.writeAudit(ctx, audit.Entry{
		AktorID: actorID, Aksi: "BULK_" + strings.ToUpper(action), Entitas: "ews_alert",
		EntitasID: fmt.Sprintf("%d alert", len(changed)), Hasil: audit.HasilBerhasil,
		Metadata: map[string]any{"ids": changed, "diminta": len(ids)},
	})
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
