package postgres

import (
	"context"
	"fmt"
	"time"

	"github.com/garda-inflasi/api/internal/siklus/domain"
	"gorm.io/gorm"
)

type Repository struct {
	db *gorm.DB
	// batasKunci adalah lama maksimum menunggu kunci evaluasi (lihat BatasKunciBawaan).
	batasKunci time.Duration
}

func New(db *gorm.DB) *Repository {
	return &Repository{db: db, batasKunci: BatasKunciBawaan}
}

// DenganBatasKunci mengembalikan salinan repositori dengan batas tunggu kunci evaluasi lain (pembulatan ke milidetik;
// minimal 1 ms). Dipakai tes untuk menguji ErrEvaluasiSibuk tanpa menunggu batas bawaan.
func (r *Repository) DenganBatasKunci(d time.Duration) *Repository {
	salinan := *r
	salinan.batasKunci = max(d.Round(time.Millisecond), time.Millisecond)
	return &salinan
}

var _ domain.Repository = (*Repository)(nil)

// GetActiveByWilayah returns the currently active cycle for a region.
// Active means AkhirPada IS NULL (the cycle has not ended yet).
func (r *Repository) GetActiveByWilayah(ctx context.Context, wilayahKode string) (*domain.SiklusEkonomi, error) {
	var siklus []domain.SiklusEkonomi
	if err := r.db.WithContext(ctx).
		Where("wilayah_kode = ? AND akhir_pada IS NULL", wilayahKode).
		Order("mulai_pada DESC").
		Limit(1).
		Find(&siklus).Error; err != nil {
		return nil, fmt.Errorf("get active siklus by wilayah: %w", err)
	}
	if len(siklus) == 0 {
		return nil, domain.ErrNotFound
	}
	return &siklus[0], nil
}

// urutanAturan mengurutkan aturan seperti mesin: prioritas, TEKANAN sebelum SURPLUS, lalu ID.
const urutanAturan = "prioritas, CASE siklus WHEN 'TEKANAN' THEN 1 WHEN 'SURPLUS' THEN 2 ELSE 3 END, id"

// ListAturan mengembalikan semua aturan penentuan siklus, aktif maupun tidak.
func (r *Repository) ListAturan(ctx context.Context) ([]domain.AturanSiklus, error) {
	var items []domain.AturanSiklus
	if err := r.db.WithContext(ctx).Order(urutanAturan).Find(&items).Error; err != nil {
		return nil, fmt.Errorf("list aturan siklus: %w", err)
	}
	return items, nil
}
