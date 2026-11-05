package seeder

import (
	"context"
	"errors"
	"fmt"
	"io"
	"strings"

	"github.com/garda-inflasi/api/internal/auth/domain"
	"github.com/garda-inflasi/api/pkg/crypto"
	"gorm.io/gorm"
)

const (
	adminID      = "admin-awal"
	adminPeran   = "ADMIN_SISTEM"
	adminLembaga = "Garda Inflasi"
	// generatedPasswordBytes adalah panjang acak (byte) kata sandi yang dibuatkan.
	generatedPasswordBytes = 12
)

// ErrAdminEmailRequired dikembalikan bila belum ada admin dan SEED_ADMIN_EMAIL kosong.
var ErrAdminEmailRequired = errors.New("SEED_ADMIN_EMAIL wajib diisi karena belum ada akun admin")

// AdminConfig adalah masukan admin awal dari lingkungan.
type AdminConfig struct {
	Email string
	// Password kosong berarti dibuatkan acak dan dicetak sekali ke Output.
	Password string
	// Output menerima kata sandi acak; nil membuangnya.
	Output io.Writer
}

// AdminSummary melaporkan hasil SeedAdmin.
type AdminSummary struct {
	Dibuat int
}

// SeedAdmin membuat satu akun admin awal bila belum ada akun ADMIN_SISTEM. Akun
// itu wajib mengganti kata sandi saat masuk pertama dan wajib MFA. Bila sudah ada
// admin, tidak ada yang berubah (aman dijalankan pada setiap deploy).
func SeedAdmin(ctx context.Context, db *gorm.DB, cfg AdminConfig) (AdminSummary, error) {
	var existing int64
	if err := db.WithContext(ctx).Model(&domain.Pengguna{}).Where("peran = ?", adminPeran).Count(&existing).Error; err != nil {
		return AdminSummary{}, fmt.Errorf("periksa admin: %w", err)
	}
	if existing > 0 {
		return AdminSummary{}, nil
	}

	email := strings.ToLower(strings.TrimSpace(cfg.Email))
	if email == "" {
		return AdminSummary{}, ErrAdminEmailRequired
	}

	password, generated := cfg.Password, false
	if password == "" {
		var err error
		if password, err = randomPassword(); err != nil {
			return AdminSummary{}, err
		}
		generated = true
	}
	hash, err := crypto.HashPassword(password)
	if err != nil {
		return AdminSummary{}, fmt.Errorf("hash kata sandi admin: %w", err)
	}

	admin := domain.Pengguna{
		ID: adminID, Nama: "Administrator Sistem", Email: email, PasswordHash: &hash,
		Peran: adminPeran, Lapis: "SISTEM", Lembaga: adminLembaga, Status: "AKTIF",
		WajibGantiPassword: true, MfaWajib: true,
	}
	if err := db.WithContext(ctx).Create(&admin).Error; err != nil {
		return AdminSummary{}, fmt.Errorf("simpan admin awal: %w", err)
	}

	if generated && cfg.Output != nil {
		fmt.Fprintf(cfg.Output, "Admin awal %s dibuat dengan kata sandi sementara (tampil sekali, wajib diganti saat masuk): %s\n", email, password)
	}
	return AdminSummary{Dibuat: 1}, nil
}

// randomPassword membuat kata sandi acak yang memenuhi kebijakan (huruf besar,
// kecil, angka, simbol).
func randomPassword() (string, error) {
	raw, err := crypto.RandomBytes(generatedPasswordBytes)
	if err != nil {
		return "", fmt.Errorf("buat kata sandi acak: %w", err)
	}
	return fmt.Sprintf("Ga-%x-Ra9", raw), nil
}
