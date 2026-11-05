package postgres

import (
	"context"
	"fmt"
	"time"

	"github.com/garda-inflasi/api/internal/auth/domain"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

var _ domain.SSORepository = (*Repository)(nil)

// SaveLoginState menyimpan langkah awal alur SSO. Baris yang sudah kedaluwarsa dihapus dulu sehingga
// tabel tidak membesar tanpa pekerjaan pembersih terpisah (alur baru hanya dimulai lewat rute berbatas laju).
func (r *Repository) SaveLoginState(ctx context.Context, state *domain.SSOLoginState) error {
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Where("expires_at < ?", time.Now()).Delete(&domain.SSOLoginState{}).Error; err != nil {
			return fmt.Errorf("bersihkan state SSO kedaluwarsa: %w", err)
		}
		if err := tx.Create(state).Error; err != nil {
			return fmt.Errorf("simpan state SSO: %w", err)
		}
		return nil
	})
}

// ConsumeLoginState menghapus dan mengembalikan baris state dalam satu pernyataan (DELETE ... RETURNING):
// dua permintaan serentak dengan state yang sama tidak mungkin sama-sama mendapatkannya.
func (r *Repository) ConsumeLoginState(ctx context.Context, stateHash string) (*domain.SSOLoginState, error) {
	var rows []domain.SSOLoginState
	res := r.db.WithContext(ctx).
		Clauses(clause.Returning{}).
		Where("state_hash = ?", stateHash).
		Delete(&rows)
	if res.Error != nil {
		return nil, fmt.Errorf("pakai state SSO: %w", res.Error)
	}
	if len(rows) == 0 {
		return nil, fmt.Errorf("pakai state SSO: %w", gorm.ErrRecordNotFound)
	}
	return &rows[0], nil
}

func (r *Repository) GetUserBySSO(ctx context.Context, provider, subject string) (*domain.Pengguna, error) {
	var user domain.Pengguna
	if err := r.db.WithContext(ctx).Where("sso_provider = ? AND sso_subject = ?", provider, subject).First(&user).Error; err != nil {
		return nil, fmt.Errorf("get user by sso: %w", err)
	}
	return &user, nil
}

func (r *Repository) LinkSSO(ctx context.Context, userID, provider, subject string, at time.Time) (bool, error) {
	res := r.db.WithContext(ctx).Model(&domain.Pengguna{}).
		Where("id = ? AND sso_subject IS NULL", userID).
		UpdateColumns(map[string]any{
			"sso_provider":       provider,
			"sso_subject":        subject,
			"sso_terhubung_pada": at,
		})
	if res.Error != nil {
		return false, fmt.Errorf("tautkan SSO: %w", res.Error)
	}
	return res.RowsAffected == 1, nil
}
