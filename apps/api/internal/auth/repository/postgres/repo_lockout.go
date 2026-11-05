package postgres

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/garda-inflasi/api/internal/auth/domain"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

// ── Lockout (kunci_login) ─────────────────────────────────────────────

var _ domain.KunciRepository = (*Repository)(nil)

// percobaanMaksBaris membatasi ulangan bila baris hilang di antara penyisipan dan penguncian
// (dihapus oleh login berhasil atau buka-kunci serentak); dalam praktik satu ulangan sudah cukup.
const percobaanMaksBaris = 5

// MulaiPercobaan memesan satu percobaan secara atomik: baris email dikunci (SELECT ... FOR UPDATE) selama
// keputusan dan pembaruan, sehingga permintaan serentak diperiksa satu per satu dan tidak ada yang lolos
// melewati batas.
func (r *Repository) MulaiPercobaan(ctx context.Context, emailHash string, now time.Time, maks int, durasi time.Duration) (domain.KunciLogin, bool, error) {
	var (
		hasil     domain.KunciLogin
		diizinkan bool
	)
	for range percobaanMaksBaris {
		err := r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
			if err := tx.Exec(
				"INSERT INTO kunci_login (email_hash, gagal, jendela_mulai, diperbarui_pada) VALUES (?, 0, ?, ?) ON CONFLICT (email_hash) DO NOTHING",
				emailHash, now, now,
			).Error; err != nil {
				return fmt.Errorf("siapkan kunci login: %w", err)
			}
			var saat domain.KunciLogin
			if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Where("email_hash = ?", emailHash).First(&saat).Error; err != nil {
				return err
			}
			baru, boleh := saat.CatatPercobaan(now, maks, durasi)
			if boleh {
				if err := tx.Model(&domain.KunciLogin{}).Where("email_hash = ?", emailHash).Updates(map[string]any{
					"gagal":           baru.Gagal,
					"jendela_mulai":   baru.JendelaMulai,
					"terkunci_sampai": baru.TerkunciSampai,
					"diperbarui_pada": baru.DiperbaruiPada,
				}).Error; err != nil {
					return fmt.Errorf("simpan kunci login: %w", err)
				}
			}
			hasil, diizinkan = baru, boleh
			return nil
		})
		if errors.Is(err, gorm.ErrRecordNotFound) {
			continue
		}
		if err != nil {
			return domain.KunciLogin{}, false, err
		}
		return hasil, diizinkan, nil
	}
	return domain.KunciLogin{}, false, fmt.Errorf("pesan percobaan login: baris kunci terus hilang")
}

// KembalikanPercobaan membatalkan satu percobaan yang dipesan; tanpa baris, tidak ada yang dibatalkan.
func (r *Repository) KembalikanPercobaan(ctx context.Context, emailHash string, maks int, now time.Time) error {
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var saat domain.KunciLogin
		err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Where("email_hash = ?", emailHash).First(&saat).Error
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil
		}
		if err != nil {
			return fmt.Errorf("baca kunci login: %w", err)
		}
		baru := saat.Kembalikan(maks, now)
		if err := tx.Model(&domain.KunciLogin{}).Where("email_hash = ?", emailHash).Updates(map[string]any{
			"gagal":           baru.Gagal,
			"terkunci_sampai": baru.TerkunciSampai,
			"diperbarui_pada": baru.DiperbaruiPada,
		}).Error; err != nil {
			return fmt.Errorf("kembalikan percobaan: %w", err)
		}
		return nil
	})
}

// HapusKunci menghapus keadaan satu email dan mengembalikan baris yang dihapus (nil bila tidak ada).
func (r *Repository) HapusKunci(ctx context.Context, emailHash string) (*domain.KunciLogin, error) {
	var dihapus []domain.KunciLogin
	if err := r.db.WithContext(ctx).Clauses(clause.Returning{}).
		Where("email_hash = ?", emailHash).Delete(&dihapus).Error; err != nil {
		return nil, fmt.Errorf("hapus kunci login: %w", err)
	}
	if len(dihapus) == 0 {
		return nil, nil
	}
	return &dihapus[0], nil
}

// KunciAktif mengembalikan akhir kunci untuk email yang sedang terkunci pada waktu now.
func (r *Repository) KunciAktif(ctx context.Context, emailHash []string, now time.Time) (map[string]time.Time, error) {
	aktif := map[string]time.Time{}
	if len(emailHash) == 0 {
		return aktif, nil
	}
	var baris []domain.KunciLogin
	if err := r.db.WithContext(ctx).
		Where("email_hash IN ? AND terkunci_sampai > ?", emailHash, now).
		Find(&baris).Error; err != nil {
		return nil, fmt.Errorf("baca kunci login: %w", err)
	}
	for _, b := range baris {
		aktif[b.EmailHash] = *b.TerkunciSampai
	}
	return aktif, nil
}
