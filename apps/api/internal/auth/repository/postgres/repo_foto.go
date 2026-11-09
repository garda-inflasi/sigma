package postgres

import (
	"context"
	"fmt"

	"github.com/garda-inflasi/api/internal/auth/domain"
	"github.com/garda-inflasi/api/pkg/idx"
	"gorm.io/gorm"
)

// ── Foto profil (tabel pengguna_foto, migrasi 000124) ─────────────────

var _ domain.FotoRepository = (*Repository)(nil)

// GetFoto membaca foto pemilik beserta byte-nya. ID yang bukan UUID tidak mungkin punya foto; jangan kirim ke kolom
// uuid (22P02).
func (r *Repository) GetFoto(ctx context.Context, userID string) (*domain.PenggunaFoto, error) {
	if !idx.ValidUUID(userID) {
		return nil, fmt.Errorf("get foto: %w", gorm.ErrRecordNotFound)
	}
	var foto domain.PenggunaFoto
	if err := r.db.WithContext(ctx).Where("pengguna_id = ?", userID).First(&foto).Error; err != nil {
		return nil, fmt.Errorf("get foto: %w", err)
	}
	return &foto, nil
}

// GetFotoSHA256 hanya membaca sidik jari foto: kolom bytea tidak ikut dimuat.
func (r *Repository) GetFotoSHA256(ctx context.Context, userID string) (string, error) {
	if !idx.ValidUUID(userID) {
		return "", nil
	}
	var sha string
	if err := r.db.WithContext(ctx).Raw("SELECT sha256 FROM pengguna_foto WHERE pengguna_id = ?", userID).Scan(&sha).Error; err != nil {
		return "", fmt.Errorf("get sha256 foto: %w", err)
	}
	return sha, nil
}

// SimpanFoto menulis foto pemilik dalam satu pernyataan (upsert atas pengguna_id), jadi dua penggantian serentak
// tidak pernah meninggalkan baris setengah jadi: yang terakhir menang.
func (r *Repository) SimpanFoto(ctx context.Context, foto *domain.PenggunaFoto) error {
	if !idx.ValidUUID(foto.PenggunaID) {
		return fmt.Errorf("simpan foto: %w", gorm.ErrForeignKeyViolated)
	}
	err := r.db.WithContext(ctx).Exec(`
		INSERT INTO pengguna_foto (pengguna_id, tipe, lebar, tinggi, ukuran, sha256, data, diubah_pada)
		VALUES (?, ?, ?, ?, ?, ?, ?, now())
		ON CONFLICT (pengguna_id) DO UPDATE
		   SET tipe = EXCLUDED.tipe, lebar = EXCLUDED.lebar, tinggi = EXCLUDED.tinggi, ukuran = EXCLUDED.ukuran,
		       sha256 = EXCLUDED.sha256, data = EXCLUDED.data, diubah_pada = now()`,
		foto.PenggunaID, foto.Tipe, foto.Lebar, foto.Tinggi, foto.Ukuran, foto.SHA256, foto.Data).Error
	if err != nil {
		return fmt.Errorf("simpan foto: %w", err)
	}
	return nil
}

// HapusFoto menghapus foto pemilik; false bila memang belum ada.
func (r *Repository) HapusFoto(ctx context.Context, userID string) (bool, error) {
	if !idx.ValidUUID(userID) {
		return false, nil
	}
	res := r.db.WithContext(ctx).Where("pengguna_id = ?", userID).Delete(&domain.PenggunaFoto{})
	if res.Error != nil {
		return false, fmt.Errorf("hapus foto: %w", res.Error)
	}
	return res.RowsAffected > 0, nil
}
