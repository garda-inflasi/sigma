package postgres

import (
	"context"
	"fmt"
	"strings"
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

// ListSessionHistory mengelompokkan refresh_tokens milik pengguna per sesi (family_id). Sesi aktif bila ada token
// yang belum dicabut dan belum kedaluwarsa. Selain itu DICABUT bila ada token yang dicabut tanpa rotasi
// (rotated_at kosong: keluar, pencabutan, atau pemakaian ulang token), dan KEDALUWARSA bila hanya masa berlakunya
// yang habis. Pencarian memakai idx_rt_user; status dan cursor disaring di HAVING setelah pengelompokan, dan
// semua nilai terparameter (fragmen SQL yang disambung berasal dari konstanta, bukan masukan).
func (r *Repository) ListSessionHistory(ctx context.Context, userID string, f domain.RiwayatFilter, now time.Time) ([]domain.RiwayatSesi, error) {
	query := `SELECT family_id::text AS id,
		MIN(session_started_at) AS mulai_pada,
		MAX(last_used_at) AS terakhir_dipakai_pada,
		(ARRAY_AGG(ip ORDER BY created_at, id))[1] AS ip,
		(ARRAY_AGG(user_agent ORDER BY created_at, id))[1] AS user_agent,
		BOOL_OR(revoked_at IS NULL AND expires_at > @now) AS aktif,
		MAX(revoked_at) FILTER (WHERE rotated_at IS NULL) AS dicabut_pada,
		MAX(expires_at) AS kedaluwarsa_pada
	FROM refresh_tokens WHERE user_id = @user GROUP BY family_id`
	args := map[string]any{"now": now, "user": userID, "limit": f.Limit}
	var having []string
	switch f.Status {
	case domain.RiwayatAktif:
		having = append(having, "BOOL_OR(revoked_at IS NULL AND expires_at > @now)")
	case domain.RiwayatBerakhir:
		having = append(having, "NOT BOOL_OR(revoked_at IS NULL AND expires_at > @now)")
	}
	if f.After != nil {
		having = append(having, "(MIN(session_started_at), family_id) < (@after_mulai, CAST(@after_id AS uuid))")
		args["after_mulai"], args["after_id"] = f.After.MulaiPada, f.After.ID
	}
	if len(having) > 0 {
		query += " HAVING " + strings.Join(having, " AND ")
	}
	query += " ORDER BY MIN(session_started_at) DESC, family_id DESC LIMIT @limit"

	var rows []struct {
		ID                  string     `gorm:"column:id"`
		MulaiPada           time.Time  `gorm:"column:mulai_pada"`
		TerakhirDipakaiPada time.Time  `gorm:"column:terakhir_dipakai_pada"`
		IP                  *string    `gorm:"column:ip"`
		UserAgent           *string    `gorm:"column:user_agent"`
		Aktif               bool       `gorm:"column:aktif"`
		DicabutPada         *time.Time `gorm:"column:dicabut_pada"`
		KedaluwarsaPada     time.Time  `gorm:"column:kedaluwarsa_pada"`
	}
	if err := r.db.WithContext(ctx).Raw(query, args).Scan(&rows).Error; err != nil {
		return nil, fmt.Errorf("riwayat sesi: %w", err)
	}
	out := make([]domain.RiwayatSesi, len(rows))
	for i, row := range rows {
		item := domain.RiwayatSesi{
			ID: row.ID, MulaiPada: row.MulaiPada, TerakhirDipakaiPada: row.TerakhirDipakaiPada,
			IP: row.IP, UserAgent: row.UserAgent, Status: domain.SesiAktif,
		}
		switch {
		case row.Aktif:
		case row.DicabutPada != nil:
			item.Status, item.BerakhirPada = domain.SesiDicabut, row.DicabutPada
		default:
			item.Status = domain.SesiKedaluwarsa
			kedaluwarsa := row.KedaluwarsaPada
			item.BerakhirPada = &kedaluwarsa
		}
		out[i] = item
	}
	return out, nil
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
