package usecase

import (
	"context"
	"errors"
	"fmt"

	"github.com/garda-inflasi/api/internal/audit"
	"gorm.io/gorm"
)

// ErrCannotResetOwnMFA: admin tidak boleh mereset MFA akunnya sendiri lewat endpoint admin. Pemilik akun punya
// jalurnya sendiri (POST /v1/auth/mfa/disable) yang meminta kata sandi dan menghormati MFA wajib. Tanpa larangan
// ini, sesi admin yang dibajak (access token curian) dapat melucuti MFA akunnya sendiri tanpa kata sandi dan
// melewati aturan MFA wajib.
var ErrCannotResetOwnMFA = errors.New("tidak dapat mereset MFA akun sendiri lewat halaman admin, gunakan pengaturan MFA di profil")

// ResetMFAResult adalah hasil reset MFA oleh admin.
type ResetMFAResult struct {
	// MFAAktifSebelumnya true bila MFA memang aktif ketika direset (false = tidak ada yang berubah).
	MFAAktifSebelumnya bool
	// SesiDicabut adalah jumlah sesi pengguna yang diakhiri.
	SesiDicabut int
}

// ResetMFAPengguna mereset MFA satu pengguna yang kehilangan perangkat autentikatornya (pengguna.kelola).
//
// Menonaktifkan MFA, mengosongkan rahasia TOTP terenkripsi, langkah TOTP anti-ulang, dan semua kode cadangan,
// lalu mengakhiri seluruh sesi pengguna (01-E): access token lama langsung ditolak, jadi sesi yang mungkin
// dipegang pihak lain bersama perangkat yang hilang tidak bertahan. Semuanya atomik (ResetMFARepository).
//
//   - Idempoten: pengguna tanpa MFA tidak berubah dan sesinya tidak disentuh; hasilnya tetap sukses dan
//     tetap tercatat di audit dengan mfa_aktif_sebelumnya false.
//   - Peran yang mewajibkan MFA (atau tanda mfa_wajib) tidak dilepas: login berikutnya pengguna itu masuk ke
//     setup MFA bila MFA diwajibkan lingkungan (klaim mfa_setup_diwajibkan yang sudah ada).
//   - Akun sendiri ditolak (ErrCannotResetOwnMFA).
func (uc *UseCase) ResetMFAPengguna(ctx context.Context, id, actorID string) (*ResetMFAResult, error) {
	if id == actorID {
		return nil, ErrCannotResetOwnMFA
	}

	hasil, err := uc.repo.ResetMFA(ctx, id, jam())
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrUserNotFound
		}
		return nil, fmt.Errorf("reset mfa: %w", err)
	}

	uc.writeAudit(ctx, audit.Entry{
		AktorID: actorID, Aksi: "RESET_MFA", Entitas: "pengguna", EntitasID: id,
		Hasil:   audit.HasilBerhasil,
		Sebelum: map[string]any{"mfa_aktif": hasil.MFAAktifSebelumnya},
		Sesudah: map[string]any{"mfa_aktif": false},
		Metadata: map[string]any{
			"mfa_aktif_sebelumnya":  hasil.MFAAktifSebelumnya,
			"rahasia_dihapus":       hasil.RahasiaDihapus,
			"kode_cadangan_dihapus": hasil.KodeCadanganDihapus,
			"sesi_dicabut":          len(hasil.SesiDicabut),
		},
	})
	uc.log.Info("reset mfa oleh admin", map[string]interface{}{
		"user_id": id, "aktor": actorID, "mfa_aktif_sebelumnya": hasil.MFAAktifSebelumnya, "sesi": len(hasil.SesiDicabut),
	})
	return &ResetMFAResult{MFAAktifSebelumnya: hasil.MFAAktifSebelumnya, SesiDicabut: len(hasil.SesiDicabut)}, nil
}
