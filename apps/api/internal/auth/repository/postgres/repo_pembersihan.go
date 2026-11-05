package postgres

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/garda-inflasi/api/internal/auth/domain"
	"gorm.io/gorm"
)

// batasWaktuBatch membatasi lama satu pernyataan DELETE (statement_timeout lokal transaksi), sehingga
// satu batch yang macet tidak menahan koneksi tanpa batas; job juga dibatasi Timeout-nya sendiri.
const batasWaktuBatch = 30 * time.Second

var _ domain.PembersihanRepository = (*Repository)(nil)

// Pola semua kueri: pilih kunci primer baris mati dalam urutan id (id berkorelasi dengan waktu sehingga
// baris tertua ditemukan lebih dulu lewat indeks primer, tanpa memindai seluruh tabel tiap batch), batasi
// LIMIT, kunci dengan FOR UPDATE SKIP LOCKED (baris yang sedang diubah transaksi lain dilewati, bukan ditunggu),
// lalu hapus.

func (r *Repository) HapusBlacklistKedaluwarsa(ctx context.Context, sebelum time.Time, batas int) (int64, error) {
	return r.hapusBatch(ctx, batas,
		`DELETE FROM jwt_blacklist WHERE jti IN (
			SELECT jti FROM jwt_blacklist WHERE expires_at < ? LIMIT ? FOR UPDATE SKIP LOCKED)`,
		sebelum, batas)
}

func (r *Repository) HapusRefreshTokenKedaluwarsa(ctx context.Context, sebelum time.Time, batas int) (int64, error) {
	return r.hapusBatch(ctx, batas,
		`DELETE FROM refresh_tokens WHERE id IN (
			SELECT id FROM refresh_tokens WHERE expires_at < ? ORDER BY id LIMIT ? FOR UPDATE SKIP LOCKED)`,
		sebelum, batas)
}

func (r *Repository) HapusResetTokenMati(ctx context.Context, sebelum time.Time, batas int) (int64, error) {
	return r.hapusBatch(ctx, batas,
		`DELETE FROM password_reset_tokens WHERE id IN (
			SELECT id FROM password_reset_tokens
			WHERE expires_at < ? OR (used_at IS NOT NULL AND used_at < ?)
			ORDER BY id LIMIT ? FOR UPDATE SKIP LOCKED)`,
		sebelum, sebelum, batas)
}

func (r *Repository) HapusLoginAttemptLama(ctx context.Context, sebelum time.Time, batas int) (int64, error) {
	return r.hapusBatch(ctx, batas,
		`DELETE FROM login_attempts WHERE id IN (
			SELECT id FROM login_attempts WHERE attempted_at < ? ORDER BY id LIMIT ? FOR UPDATE SKIP LOCKED)`,
		sebelum, batas)
}

func (r *Repository) HapusKunciLoginUsang(ctx context.Context, sebelum, sekarang time.Time, batas int) (int64, error) {
	return r.hapusBatch(ctx, batas,
		`DELETE FROM kunci_login WHERE email_hash IN (
			SELECT email_hash FROM kunci_login
			WHERE diperbarui_pada < ? AND (terkunci_sampai IS NULL OR terkunci_sampai < ?)
			ORDER BY diperbarui_pada LIMIT ? FOR UPDATE SKIP LOCKED)`,
		sebelum, sekarang, batas)
}

func (r *Repository) HapusSSOStateKedaluwarsa(ctx context.Context, sebelum time.Time, batas int) (int64, error) {
	return r.hapusBatch(ctx, batas,
		`DELETE FROM sso_login_state WHERE state_hash IN (
			SELECT state_hash FROM sso_login_state WHERE expires_at < ? ORDER BY expires_at LIMIT ? FOR UPDATE SKIP LOCKED)`,
		sebelum, batas)
}

// hapusBatch menjalankan satu DELETE berbatas waktu dalam transaksi pendek dan mengembalikan jumlah baris terhapus.
func (r *Repository) hapusBatch(ctx context.Context, batas int, query string, args ...any) (int64, error) {
	if batas < 1 {
		return 0, errors.New("pembersihan: ukuran batch harus minimal 1")
	}
	var dihapus int64
	err := r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		// SET tidak menerima parameter bind; nilainya konstanta bilangan bulat, bukan masukan pengguna.
		if err := tx.Exec(fmt.Sprintf("SET LOCAL statement_timeout = %d", batasWaktuBatch.Milliseconds())).Error; err != nil {
			return fmt.Errorf("atur statement_timeout: %w", err)
		}
		res := tx.Exec(query, args...)
		if res.Error != nil {
			return res.Error
		}
		dihapus = res.RowsAffected
		return nil
	})
	if err != nil {
		return 0, fmt.Errorf("hapus batch: %w", err)
	}
	return dihapus, nil
}
