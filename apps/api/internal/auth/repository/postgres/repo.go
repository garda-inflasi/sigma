package postgres

import (
	"context"
	"fmt"
	"time"

	"github.com/garda-inflasi/api/internal/auth/domain"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

type Repository struct {
	db *gorm.DB
}

func New(db *gorm.DB) *Repository {
	return &Repository{db: db}
}

// ── Pengguna ──────────────────────────────────────────────────────────

func (r *Repository) GetUserByID(ctx context.Context, id string) (*domain.Pengguna, error) {
	var user domain.Pengguna
	if err := r.db.WithContext(ctx).Where("id = ?", id).First(&user).Error; err != nil {
		return nil, fmt.Errorf("get user by id: %w", err)
	}
	return &user, nil
}

func (r *Repository) GetUserByEmail(ctx context.Context, email string) (*domain.Pengguna, error) {
	var user domain.Pengguna
	if err := r.db.WithContext(ctx).Where("lower(email) = lower(?)", email).First(&user).Error; err != nil {
		return nil, fmt.Errorf("get user by email: %w", err)
	}
	return &user, nil
}

func (r *Repository) CreateUser(ctx context.Context, user *domain.Pengguna) error {
	return r.db.WithContext(ctx).Create(user).Error
}

func (r *Repository) PatchUser(ctx context.Context, id string, patch domain.PenggunaPatch) (*domain.Pengguna, *domain.Pengguna, error) {
	var sebelum, sesudah *domain.Pengguna
	err := r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		// Ganti peran atau status dapat menghilangkan pemegang terakhir izin pengelolaan: serialkan dan jaga
		// (kunci penasihat diambil sebelum kunci baris).
		jaga := patch.Peran != nil || patch.Status != nil
		if jaga {
			return jagaPemegang(tx, func() error { return r.patchUserDalam(tx, id, patch, &sebelum, &sesudah) })
		}
		return r.patchUserDalam(tx, id, patch, &sebelum, &sesudah)
	})
	if err != nil {
		return nil, nil, err
	}
	return sebelum, sesudah, nil
}

// patchUserDalam menulis patch di dalam transaksi tx: baris dikunci, dibaca (sebelum), ditulis, lalu dibaca ulang (sesudah).
func (r *Repository) patchUserDalam(tx *gorm.DB, id string, patch domain.PenggunaPatch, sebelum, sesudah **domain.Pengguna) error {
	var saat domain.Pengguna
	if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Where("id = ?", id).First(&saat).Error; err != nil {
		return fmt.Errorf("get user for update: %w", err)
	}
	lama := saat
	kolom := patch.Kolom()
	if len(kolom) == 0 {
		*sebelum, *sesudah = &lama, &saat
		return nil
	}
	kolom["diupdate_pada"] = gorm.Expr("now()")
	if err := tx.Model(&domain.Pengguna{}).Where("id = ?", id).Updates(kolom).Error; err != nil {
		return fmt.Errorf("patch user: %w", err)
	}
	var baru domain.Pengguna
	if err := tx.Where("id = ?", id).First(&baru).Error; err != nil {
		return fmt.Errorf("reload user: %w", err)
	}
	*sebelum, *sesudah = &lama, &baru
	return nil
}

func (r *Repository) SetPassword(ctx context.Context, userID string, p domain.GantiPassword) (bool, error) {
	q := r.db.WithContext(ctx).Model(&domain.Pengguna{}).Where("id = ?", userID)
	if p.HashSebelumnya != nil {
		q = q.Where("password_hash = ?", *p.HashSebelumnya)
	}
	res := q.Updates(map[string]any{
		"password_hash":        p.Hash,
		"wajib_ganti_password": p.WajibGanti,
		"password_diubah_pada": p.At,
		"diupdate_pada":        gorm.Expr("now()"),
	})
	if res.Error != nil {
		return false, fmt.Errorf("set password: %w", res.Error)
	}
	return res.RowsAffected == 1, nil
}

func (r *Repository) SetMFASecret(ctx context.Context, userID, secretTersegel string) (bool, error) {
	res := r.db.WithContext(ctx).Exec(
		"UPDATE pengguna SET mfa_secret = ?, mfa_langkah_terakhir = 0, diupdate_pada = now() WHERE id = ? AND mfa_aktif = false",
		secretTersegel, userID,
	)
	if res.Error != nil {
		return false, fmt.Errorf("simpan rahasia mfa: %w", res.Error)
	}
	return res.RowsAffected == 1, nil
}

func (r *Repository) TouchLastLogin(ctx context.Context, userID string, at time.Time) error {
	return r.db.WithContext(ctx).Model(&domain.Pengguna{}).Where("id = ?", userID).UpdateColumn("terakhir_masuk", at).Error
}

func (r *Repository) DeleteUser(ctx context.Context, id string) error {
	// Menghapus pemegang terakhir izin pengelolaan ditolak (domain.ErrPemegangTerakhir).
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		return jagaPemegang(tx, func() error {
			return tx.Where("id = ?", id).Delete(&domain.Pengguna{}).Error
		})
	})
}

// ── Refresh Token ─────────────────────────────────────────────────────

func (r *Repository) CreateRefreshToken(ctx context.Context, token *domain.RefreshToken) error {
	return r.db.WithContext(ctx).Create(token).Error
}

func (r *Repository) GetRefreshTokenByHash(ctx context.Context, hash string) (*domain.RefreshToken, error) {
	var token domain.RefreshToken
	if err := r.db.WithContext(ctx).Where("token_hash = ?", hash).First(&token).Error; err != nil {
		return nil, fmt.Errorf("get refresh token: %w", err)
	}
	return &token, nil
}

func (r *Repository) RevokeRefreshToken(ctx context.Context, id int64) error {
	now := time.Now()
	return r.db.WithContext(ctx).Model(&domain.RefreshToken{}).Where("id = ?", id).Update("revoked_at", now).Error
}

func (r *Repository) RevokeRefreshTokenFamily(ctx context.Context, familyID string) error {
	now := time.Now()
	// Hanya token hidup: token hasil rotasi mempertahankan revoked_at/rotated_at aslinya (jendela toleransi dan
	// deteksi pemakaian ulang bergantung padanya).
	return r.db.WithContext(ctx).Model(&domain.RefreshToken{}).Where("family_id = ? AND revoked_at IS NULL", familyID).Update("revoked_at", now).Error
}

// ── Login Attempt ─────────────────────────────────────────────────────

func (r *Repository) CreateLoginAttempt(ctx context.Context, attempt *domain.LoginAttempt) error {
	return r.db.WithContext(ctx).Create(attempt).Error
}

// ── JWT Blacklist ──────────────────────────────────────────────────────

func (r *Repository) BlacklistJWT(ctx context.Context, jti string, ttl time.Duration) error {
	expiresAt := time.Now().Add(ttl)
	return r.db.WithContext(ctx).Exec(
		"INSERT INTO jwt_blacklist (jti, expires_at) VALUES (?, ?) ON CONFLICT (jti) DO NOTHING",
		jti, expiresAt,
	).Error
}

func (r *Repository) IsJWTBlacklisted(ctx context.Context, jti string) (bool, error) {
	var count int64
	err := r.db.WithContext(ctx).Model(&struct{}{}).
		Table("jwt_blacklist").
		Where("jti = ? AND expires_at > now()", jti).
		Count(&count).Error
	return count > 0, err
}

// ── Password Reset Token ──────────────────────────────────────────────

func (r *Repository) SaveResetToken(ctx context.Context, userID, tokenHash string, expiresAt time.Time) error {
	return r.db.WithContext(ctx).Exec(
		"INSERT INTO password_reset_tokens (user_id, token_hash, expires_at) VALUES (?, ?, ?)",
		userID, tokenHash, expiresAt,
	).Error
}

func (r *Repository) GetResetToken(ctx context.Context, tokenHash string) (*domain.ResetToken, error) {
	var rt domain.ResetToken
	if err := r.db.WithContext(ctx).Where("token_hash = ?", tokenHash).First(&rt).Error; err != nil {
		return nil, fmt.Errorf("get reset token: %w", err)
	}
	return &rt, nil
}

func (r *Repository) ConsumeResetToken(ctx context.Context, id int64, now time.Time) (bool, error) {
	var claimed bool
	err := r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var userIDs []string
		if err := tx.Raw(
			"UPDATE password_reset_tokens SET used_at = ? WHERE id = ? AND used_at IS NULL AND expires_at > ? RETURNING user_id",
			now, id, now,
		).Scan(&userIDs).Error; err != nil {
			return fmt.Errorf("pakai token reset: %w", err)
		}
		if len(userIDs) == 0 {
			return nil
		}
		claimed = true
		if err := tx.Exec(
			"UPDATE password_reset_tokens SET used_at = ? WHERE user_id = ? AND used_at IS NULL",
			now, userIDs[0],
		).Error; err != nil {
			return fmt.Errorf("hanguskan token reset lain: %w", err)
		}
		return nil
	})
	if err != nil {
		return false, err
	}
	return claimed, nil
}

// ── Revoke All User Refresh Tokens ────────────────────────────────────

func (r *Repository) RevokeAllUserRefreshTokens(ctx context.Context, userID string) error {
	now := time.Now()
	return r.db.WithContext(ctx).Model(&domain.RefreshToken{}).
		Where("user_id = ? AND revoked_at IS NULL", userID).
		Update("revoked_at", now).Error
}

// ── MFA ───────────────────────────────────────────────────────────────

func (r *Repository) ActivateMFA(ctx context.Context, userID string, kodeHash []string, langkah int64, secretTersegel string) (bool, error) {
	var aktif bool
	err := r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		res := tx.Exec(
			"UPDATE pengguna SET mfa_aktif = true, mfa_langkah_terakhir = ?, diupdate_pada = now() WHERE id = ? AND mfa_aktif = false AND mfa_secret = ?",
			langkah, userID, secretTersegel,
		)
		if res.Error != nil {
			return fmt.Errorf("aktifkan mfa: %w", res.Error)
		}
		if res.RowsAffected != 1 {
			return nil
		}
		aktif = true
		return replaceBackupCodes(tx, userID, kodeHash)
	})
	if err != nil {
		return false, err
	}
	return aktif, nil
}

func (r *Repository) ReplaceBackupCodes(ctx context.Context, userID string, kodeHash []string) error {
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		return replaceBackupCodes(tx, userID, kodeHash)
	})
}

func replaceBackupCodes(tx *gorm.DB, userID string, kodeHash []string) error {
	if err := tx.Exec("DELETE FROM mfa_kode_cadangan WHERE user_id = ?", userID).Error; err != nil {
		return fmt.Errorf("hapus kode cadangan lama: %w", err)
	}
	if len(kodeHash) == 0 {
		return nil
	}
	rows := make([]domain.KodeCadanganMFA, len(kodeHash))
	for i, h := range kodeHash {
		rows[i] = domain.KodeCadanganMFA{UserID: userID, KodeHash: h}
	}
	if err := tx.Create(&rows).Error; err != nil {
		return fmt.Errorf("simpan kode cadangan: %w", err)
	}
	return nil
}

func (r *Repository) UseBackupCode(ctx context.Context, userID, kodeHash string, now time.Time) (bool, error) {
	res := r.db.WithContext(ctx).Exec(
		"UPDATE mfa_kode_cadangan SET dipakai_pada = ? WHERE user_id = ? AND kode_hash = ? AND dipakai_pada IS NULL",
		now, userID, kodeHash,
	)
	if res.Error != nil {
		return false, fmt.Errorf("pakai kode cadangan: %w", res.Error)
	}
	return res.RowsAffected == 1, nil
}

func (r *Repository) CountBackupCodes(ctx context.Context, userID string) (int64, error) {
	var n int64
	err := r.db.WithContext(ctx).Model(&domain.KodeCadanganMFA{}).Where("user_id = ? AND dipakai_pada IS NULL", userID).Count(&n).Error
	return n, err
}

func (r *Repository) AdvanceMFAStep(ctx context.Context, userID string, langkah int64) (bool, error) {
	res := r.db.WithContext(ctx).Exec(
		"UPDATE pengguna SET mfa_langkah_terakhir = ? WHERE id = ? AND mfa_langkah_terakhir < ?",
		langkah, userID, langkah,
	)
	if res.Error != nil {
		return false, fmt.Errorf("majukan langkah mfa: %w", res.Error)
	}
	return res.RowsAffected == 1, nil
}

func (r *Repository) ClearMFA(ctx context.Context, userID string) error {
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Exec(
			"UPDATE pengguna SET mfa_aktif = false, mfa_secret = NULL, mfa_langkah_terakhir = 0, diupdate_pada = now() WHERE id = ?",
			userID,
		).Error; err != nil {
			return fmt.Errorf("nonaktifkan mfa: %w", err)
		}
		if err := tx.Exec("DELETE FROM mfa_kode_cadangan WHERE user_id = ?", userID).Error; err != nil {
			return fmt.Errorf("hapus kode cadangan: %w", err)
		}
		return nil
	})
}

// ── Bulk User Actions ─────────────────────────────────────────────────

func (r *Repository) BulkUpdateUsers(ctx context.Context, ids []string, action domain.BulkUserAction) (int64, error) {
	tx := r.db.WithContext(ctx).Model(&domain.Pengguna{}).Where("id IN ?", ids)

	switch action {
	case domain.BulkAktifkan:
		// Akun MENUNGGU (undangan) tidak boleh diaktifkan tanpa kata sandi: hanya aktivasi oleh pemiliknya.
		tx = tx.Where("status <> ?", domain.StatusMenunggu).Update("status", "AKTIF")
	case domain.BulkNonaktifkan:
		return r.nonaktifkanPengguna(ctx, ids)
	case domain.BulkWajibkanMFA:
		tx = tx.Update("mfa_wajib", true)
	case domain.BulkAkhiriSesi:
		// Revoke all refresh tokens for these users
		now := time.Now()
		if err := r.db.WithContext(ctx).Model(&domain.RefreshToken{}).
			Where("user_id IN ? AND revoked_at IS NULL", ids).
			Update("revoked_at", now).Error; err != nil {
			return 0, fmt.Errorf("revoke sessions: %w", err)
		}
		return int64(len(ids)), nil
	default:
		return 0, fmt.Errorf("unknown bulk action: %s", action)
	}

	if tx.Error != nil {
		return 0, fmt.Errorf("bulk action %s: %w", action, tx.Error)
	}

	return tx.RowsAffected, nil
}

// nonaktifkanPengguna menonaktifkan akun dan mencabut sesinya dalam satu transaksi, ditolak
// (domain.ErrPemegangTerakhir) bila membuat sistem kehilangan pemegang terakhir izin pengelolaan.
// Akun nonaktif tidak boleh meneruskan sesinya: access token mati seketika lewat sid.
func (r *Repository) nonaktifkanPengguna(ctx context.Context, ids []string) (int64, error) {
	var diubah int64
	err := r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		return jagaPemegang(tx, func() error {
			// Akun MENUNGGU dibatalkan dengan menghapusnya, bukan dinonaktifkan: akun tanpa kata sandi tidak punya
			// jalan kembali ke AKTIF selain undangan.
			res := tx.Model(&domain.Pengguna{}).Where("id IN ? AND status <> ?", ids, domain.StatusMenunggu).Update("status", "NONAKTIF")
			if res.Error != nil {
				return fmt.Errorf("bulk action %s: %w", domain.BulkNonaktifkan, res.Error)
			}
			diubah = res.RowsAffected
			if err := tx.Model(&domain.RefreshToken{}).
				Where("user_id IN ? AND revoked_at IS NULL", ids).
				Update("revoked_at", time.Now()).Error; err != nil {
				return fmt.Errorf("revoke sessions: %w", err)
			}
			return nil
		})
	})
	if err != nil {
		return 0, err
	}
	return diubah, nil
}
