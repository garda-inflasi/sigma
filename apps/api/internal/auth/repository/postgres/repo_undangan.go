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

var _ domain.UndanganRepository = (*Repository)(nil)

func (r *Repository) BuatUndangan(ctx context.Context, baru []domain.UndanganBaru) error {
	if len(baru) == 0 {
		return nil
	}
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		for _, b := range baru {
			if err := tx.Create(b.User).Error; err != nil {
				return fmt.Errorf("buat akun undangan: %w", err)
			}
			b.Undangan.UserID = b.User.ID
			if err := tx.Create(b.Undangan).Error; err != nil {
				return fmt.Errorf("simpan undangan: %w", err)
			}
		}
		return nil
	})
}

func (r *Repository) GantiUndangan(ctx context.Context, userID string, u *domain.Undangan, now time.Time) (*domain.Pengguna, error) {
	var user domain.Pengguna
	err := r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		// Kunci baris akun: kirim ulang serentak berurutan, dan perubahan email/status yang sedang berjalan
		// selesai lebih dulu, sehingga undangan baru selalu ditujukan ke email akun saat ini.
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Where("id = ?", userID).First(&user).Error; err != nil {
			return fmt.Errorf("kunci akun undangan: %w", err)
		}
		if user.Status != domain.StatusMenunggu {
			return domain.ErrBukanMenunggu
		}
		if err := tx.Exec(
			"UPDATE undangan_pengguna SET dicabut_pada = ? WHERE user_id = ? AND used_at IS NULL AND dicabut_pada IS NULL",
			now, userID,
		).Error; err != nil {
			return fmt.Errorf("cabut undangan lama: %w", err)
		}
		u.UserID = userID
		u.Email = user.Email
		if err := tx.Create(u).Error; err != nil {
			return fmt.Errorf("simpan undangan: %w", err)
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	return &user, nil
}

func (r *Repository) GetUndangan(ctx context.Context, tokenHash string) (*domain.Undangan, error) {
	var u domain.Undangan
	if err := r.db.WithContext(ctx).Where("token_hash = ?", tokenHash).First(&u).Error; err != nil {
		return nil, fmt.Errorf("get undangan: %w", err)
	}
	return &u, nil
}

// errAktivasiGagal membatalkan transaksi Aktifkan tanpa dianggap galat basis data.
var errAktivasiGagal = errors.New("syarat aktivasi tidak terpenuhi")

func (r *Repository) Aktifkan(ctx context.Context, tokenHash, passwordHash string, now time.Time) (*domain.Pengguna, error) {
	var user domain.Pengguna
	err := r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		// Token dipakai lebih dulu dan atomik: dua permintaan serentak dengan token yang sama tidak bisa
		// sama-sama menetapkan kata sandi. Syarat akun (MENUNGGU, email masih sama) ada di pernyataan yang
		// sama, jadi undangan untuk akun yang sudah dinonaktifkan atau berganti email tidak pernah terpakai.
		var userIDs []string
		if err := tx.Raw(
			`UPDATE undangan_pengguna u SET used_at = ?
			 FROM pengguna p
			 WHERE u.token_hash = ? AND p.id = u.user_id
			   AND u.used_at IS NULL AND u.dicabut_pada IS NULL AND u.expires_at > ?
			   AND p.status = 'MENUNGGU' AND lower(p.email) = lower(u.email)
			 RETURNING u.user_id`,
			now, tokenHash, now,
		).Scan(&userIDs).Error; err != nil {
			return fmt.Errorf("pakai undangan: %w", err)
		}
		if len(userIDs) != 1 {
			return errAktivasiGagal
		}
		res := tx.Exec(
			`UPDATE pengguna
			 SET password_hash = ?, status = 'AKTIF', wajib_ganti_password = false,
			     password_diubah_pada = ?, diupdate_pada = now()
			 WHERE id = ? AND status = 'MENUNGGU'`,
			passwordHash, now, userIDs[0],
		)
		if res.Error != nil {
			return fmt.Errorf("aktifkan akun: %w", res.Error)
		}
		if res.RowsAffected != 1 {
			return errAktivasiGagal
		}
		if err := tx.Where("id = ?", userIDs[0]).First(&user).Error; err != nil {
			return fmt.Errorf("muat akun teraktivasi: %w", err)
		}
		return nil
	})
	if errors.Is(err, errAktivasiGagal) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &user, nil
}

func (r *Repository) EmailTerdaftar(ctx context.Context, emailKecil []string) (map[string]bool, error) {
	ada := map[string]bool{}
	if len(emailKecil) == 0 {
		return ada, nil
	}
	var kecil []string
	if err := r.db.WithContext(ctx).Model(&domain.Pengguna{}).
		Where("lower(email) IN ?", emailKecil).Pluck("lower(email)", &kecil).Error; err != nil {
		return nil, fmt.Errorf("cek email terdaftar: %w", err)
	}
	for _, e := range kecil {
		ada[e] = true
	}
	return ada, nil
}

func (r *Repository) KabupatenAda(ctx context.Context, kode []string) (map[string]bool, error) {
	ada := map[string]bool{}
	if len(kode) == 0 {
		return ada, nil
	}
	var ditemukan []string
	if err := r.db.WithContext(ctx).Table("kabupaten").Where("kode IN ?", kode).Pluck("kode", &ditemukan).Error; err != nil {
		return nil, fmt.Errorf("cek kabupaten: %w", err)
	}
	for _, k := range ditemukan {
		ada[k] = true
	}
	return ada, nil
}
