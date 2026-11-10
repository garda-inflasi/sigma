package usecase

import (
	"context"
	"errors"
	"fmt"

	"github.com/garda-inflasi/api/internal/audit"
	"github.com/garda-inflasi/api/internal/siklus/domain"
	"github.com/garda-inflasi/api/pkg/logger"
)

var (
	ErrSiklusNotFound = errors.New("siklus aktif tidak ditemukan untuk wilayah ini")
	// ErrEvaluasiSibuk: evaluasi lain masih berjalan dan tidak selesai dalam batas waktu.
	ErrEvaluasiSibuk = domain.ErrEvaluasiSibuk
)

type UseCase struct {
	repo      domain.Repository
	evaluator domain.Evaluator
	audit     audit.Writer
	log       *logger.Logger
}

func New(repo domain.Repository, evaluator domain.Evaluator, auditWriter audit.Writer, log *logger.Logger) *UseCase {
	return &UseCase{
		repo:      repo,
		evaluator: evaluator,
		audit:     auditWriter,
		log:       log,
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

// ListAturan mengembalikan semua aturan penentuan siklus (aktif maupun tidak), terurut menurut prioritas.
func (uc *UseCase) ListAturan(ctx context.Context) ([]domain.AturanSiklus, error) {
	items, err := uc.repo.ListAturan(ctx)
	if err != nil {
		uc.log.Error("siklus: gagal membaca aturan", map[string]interface{}{"error": err.Error()})
		return nil, fmt.Errorf("list aturan siklus: %w", err)
	}
	return items, nil
}

// Evaluasi menjalankan mesin penentuan siklus sekarang juga untuk provinsi bawaan (di luar jadwal) dan mencatatnya
// di audit. Pergantian siklus yang terjadi dicatat mesin sendiri atas nama pemintanya (GANTI_SIKLUS); catatan
// EVALUASI di sini merekam bahwa pengguna meminta putaran itu, apa pun hasilnya.
func (uc *UseCase) Evaluasi(ctx context.Context, actorID string) (domain.HasilEvaluasi, error) {
	hasil, err := uc.evaluator.Evaluasi(ctx, domain.WilayahBawaan, domain.OpsiEvaluasi{AktorID: actorID, Pemicu: domain.PemicuManual})
	if err != nil {
		if errors.Is(err, domain.ErrEvaluasiSibuk) {
			return hasil, ErrEvaluasiSibuk
		}
		uc.log.Error("siklus: evaluasi manual gagal", map[string]interface{}{"error": err.Error()})
		return hasil, fmt.Errorf("evaluasi: %w", err)
	}
	metadata := map[string]any{
		"wilayah_kode": hasil.WilayahKode, "berubah": hasil.Berubah, "dilewati": hasil.Dilewati, "ringkasan": hasil.Ringkasan,
	}
	if hasil.Aktif != nil {
		metadata["siklus"] = hasil.Aktif.Siklus
	}
	if err := uc.audit.Write(ctx, audit.Entry{
		AktorID: actorID, Aksi: "EVALUASI", Entitas: "siklus_ekonomi", EntitasID: "mesin siklus", Hasil: audit.HasilBerhasil,
		Metadata: metadata,
	}); err != nil {
		uc.log.Warn("siklus: gagal mencatat audit", map[string]interface{}{"aksi": "EVALUASI", "error": err.Error()})
	}
	return hasil, nil
}
