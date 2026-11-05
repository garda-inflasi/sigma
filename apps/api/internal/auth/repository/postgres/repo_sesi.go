package postgres

import (
	"context"
	"fmt"
	"time"

	"github.com/garda-inflasi/api/internal/auth/domain"
	"github.com/google/uuid"
	"gorm.io/gorm"
)

var _ domain.SesiRepository = (*Repository)(nil)

// ── Rotasi ────────────────────────────────────────────────────────────

// RotateRefreshToken mencabut token lama dan menyimpan penggantinya dalam satu transaksi. Pencabutan
// bersyarat (revoked_at IS NULL) lalu memeriksa RowsAffected: bila dua transaksi berebut token yang sama,
// yang kedua menunggu kunci baris milik yang pertama, melihat revoked_at sudah terisi setelah commit, dan
// mendapat 0 baris, jadi tidak pernah menyimpan token pengganti kedua.
func (r *Repository) RotateRefreshToken(ctx context.Context, oldID int64, next *domain.RefreshToken, now time.Time) (bool, error) {
	var rotated bool
	err := r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		res := tx.Exec(
			"UPDATE refresh_tokens SET revoked_at = ?, rotated_at = ? WHERE id = ? AND revoked_at IS NULL AND expires_at > ?",
			now, now, oldID, now,
		)
		if res.Error != nil {
			return fmt.Errorf("cabut token lama: %w", res.Error)
		}
		if res.RowsAffected == 0 {
			return nil
		}
		if err := tx.Create(next).Error; err != nil {
			return fmt.Errorf("simpan token pengganti: %w", err)
		}
		rotated = true
		return nil
	})
	if err != nil {
		return false, err
	}
	return rotated, nil
}

// RetireFamilyTokensExcept menandai token hidup dalam family selain keepTokenHash sebagai hasil rotasi.
func (r *Repository) RetireFamilyTokensExcept(ctx context.Context, familyID, keepTokenHash string, now time.Time) error {
	if err := r.db.WithContext(ctx).Exec(
		"UPDATE refresh_tokens SET revoked_at = ?, rotated_at = ? WHERE family_id = ? AND revoked_at IS NULL AND token_hash <> ?",
		now, now, familyID, keepTokenHash,
	).Error; err != nil {
		return fmt.Errorf("pensiunkan token lama: %w", err)
	}
	return nil
}

// ── Sesi aktif ────────────────────────────────────────────────────────

func (r *Repository) IsSessionActive(ctx context.Context, familyID string) (bool, error) {
	var hasil struct{ Aktif bool }
	err := r.db.WithContext(ctx).
		Raw("SELECT EXISTS (SELECT 1 FROM refresh_tokens WHERE family_id = ? AND revoked_at IS NULL) AS aktif", familyID).
		Scan(&hasil).Error
	if err != nil {
		return false, fmt.Errorf("periksa sesi: %w", err)
	}
	return hasil.Aktif, nil
}

func (r *Repository) ListActiveRefreshTokens(ctx context.Context, userID string, now time.Time) ([]domain.RefreshToken, error) {
	var rows []domain.RefreshToken
	if err := r.db.WithContext(ctx).
		Where("user_id = ? AND revoked_at IS NULL AND expires_at > ?", userID, now).
		Order("last_used_at DESC, id DESC").
		Find(&rows).Error; err != nil {
		return nil, fmt.Errorf("daftar sesi aktif: %w", err)
	}
	return rows, nil
}

// ── Pencabutan ────────────────────────────────────────────────────────

func (r *Repository) RevokeSessions(ctx context.Context, userID, keepFamilyID string, now time.Time) ([]string, error) {
	query := "UPDATE refresh_tokens SET revoked_at = ? WHERE user_id = ? AND revoked_at IS NULL"
	args := []any{now, userID}
	if keepFamilyID != "" {
		query += " AND family_id <> ?"
		args = append(args, keepFamilyID)
	}
	query += " RETURNING family_id::text"

	var families []string
	if err := r.db.WithContext(ctx).Raw(query, args...).Scan(&families).Error; err != nil {
		return nil, fmt.Errorf("cabut sesi: %w", err)
	}
	return distinct(families), nil
}

func (r *Repository) RevokeSession(ctx context.Context, userID, familyID string, now time.Time) (bool, error) {
	res := r.db.WithContext(ctx).Exec(
		"UPDATE refresh_tokens SET revoked_at = ? WHERE user_id = ? AND family_id = ? AND revoked_at IS NULL",
		now, userID, familyID,
	)
	if res.Error != nil {
		return false, fmt.Errorf("cabut sesi: %w", res.Error)
	}
	return res.RowsAffected > 0, nil
}

func distinct(in []string) []string {
	seen := make(map[string]struct{}, len(in))
	out := make([]string, 0, len(in))
	for _, v := range in {
		if _, ok := seen[v]; ok {
			continue
		}
		seen[v] = struct{}{}
		out = append(out, v)
	}
	return out
}

// ── Pemeriksa pencabutan access token ─────────────────────────────────

// RevocationChecker memeriksa access token yang tanda tangannya sah terhadap basis data. Satu pernyataan SQL
// per request: lookup kunci utama jwt_blacklist dan, bila token membawa sid, satu entri indeks parsial
// idx_rt_family_aktif (family yang masih punya token hidup). Dipasang pada token.Manager oleh rute auth.
type RevocationChecker struct {
	db *gorm.DB
}

func NewRevocationChecker(db *gorm.DB) *RevocationChecker {
	return &RevocationChecker{db: db}
}

// Revoked benar bila jti ada di daftar cabut yang belum kedaluwarsa, atau sesi (sid) tidak lagi punya token
// hidup. sid kosong (token dari sebelum klaim sid ada) hanya diperiksa lewat jti.
func (c *RevocationChecker) Revoked(ctx context.Context, jti, sessionID string) (bool, error) {
	db := c.db.WithContext(ctx)

	// Scan lewat GORM (bukan Row().Scan): saat query gagal Row() mengembalikan nil dan Scan-nya panik.
	if sessionID == "" {
		var hasil struct{ Dicabut bool }
		if err := db.Raw("SELECT EXISTS (SELECT 1 FROM jwt_blacklist WHERE jti = ? AND expires_at > now()) AS dicabut", jti).
			Scan(&hasil).Error; err != nil {
			return false, fmt.Errorf("periksa daftar cabut: %w", err)
		}
		return hasil.Dicabut, nil
	}

	// Token yang kita terbitkan selalu membawa sid UUID; yang lain tidak mungkin lolos tanda tangan, tetapi
	// jangan sampai cast ke uuid menjadi 500.
	if _, err := uuid.Parse(sessionID); err != nil {
		return true, nil
	}

	var hasil struct {
		Diblacklist bool
		Hidup       bool
	}
	err := db.Raw(`SELECT
		EXISTS (SELECT 1 FROM jwt_blacklist WHERE jti = ? AND expires_at > now()) AS diblacklist,
		EXISTS (SELECT 1 FROM refresh_tokens WHERE family_id = ? AND revoked_at IS NULL) AS hidup`,
		jti, sessionID).Scan(&hasil).Error
	if err != nil {
		return false, fmt.Errorf("periksa pencabutan sesi: %w", err)
	}
	return hasil.Diblacklist || !hasil.Hidup, nil
}
