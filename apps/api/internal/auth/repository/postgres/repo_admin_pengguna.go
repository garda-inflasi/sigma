package postgres

import (
	"context"
	"fmt"
	"time"

	"github.com/garda-inflasi/api/internal/auth/domain"
	"gorm.io/gorm"
)

// ── Reset MFA oleh admin ──────────────────────────────────────────────

var _ domain.ResetMFARepository = (*Repository)(nil)

func (r *Repository) ResetMFA(ctx context.Context, userID string, now time.Time) (*domain.HasilResetMFA, error) {
	hasil := &domain.HasilResetMFA{}
	err := r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		// Baris dikunci: reset serentak dan aktivasi/penonaktifan MFA oleh pemilik diperiksa satu per satu,
		// jadi keadaan "sebelum" yang dilaporkan benar-benar yang dihapus.
		var saat struct {
			MfaAktif   bool
			AdaRahasia bool
		}
		baca := tx.Raw("SELECT mfa_aktif, (mfa_secret IS NOT NULL) AS ada_rahasia FROM pengguna WHERE id = ? FOR UPDATE", userID).Scan(&saat)
		if baca.Error != nil {
			return fmt.Errorf("baca mfa pengguna: %w", baca.Error)
		}
		if baca.RowsAffected == 0 {
			return fmt.Errorf("reset mfa: %w", gorm.ErrRecordNotFound)
		}

		res := tx.Exec("DELETE FROM mfa_kode_cadangan WHERE user_id = ?", userID)
		if res.Error != nil {
			return fmt.Errorf("hapus kode cadangan: %w", res.Error)
		}
		hasil.KodeCadanganDihapus = res.RowsAffected
		hasil.MFAAktifSebelumnya, hasil.RahasiaDihapus = saat.MfaAktif, saat.AdaRahasia

		// Pengguna tanpa MFA tidak ditulis ulang: diupdate_pada tetap, dan kosongnya rahasia/langkah sudah benar.
		if saat.MfaAktif || saat.AdaRahasia {
			if err := tx.Exec(
				"UPDATE pengguna SET mfa_aktif = false, mfa_secret = NULL, mfa_langkah_terakhir = 0, diupdate_pada = now() WHERE id = ?",
				userID,
			).Error; err != nil {
				return fmt.Errorf("nonaktifkan mfa: %w", err)
			}
		}

		// Sesi hanya dicabut bila faktor kedua yang aktif memang dilepas: pengguna tanpa MFA tidak diganggu.
		if saat.MfaAktif {
			var families []string
			if err := tx.Raw(
				"UPDATE refresh_tokens SET revoked_at = ? WHERE user_id = ? AND revoked_at IS NULL RETURNING family_id::text",
				now, userID,
			).Scan(&families).Error; err != nil {
				return fmt.Errorf("cabut sesi: %w", err)
			}
			hasil.SesiDicabut = distinct(families)
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	return hasil, nil
}

// ── Riwayat peran (dari audit_log) ────────────────────────────────────

var _ domain.RiwayatPeranRepository = (*Repository)(nil)

// RiwayatPeran membaca perubahan peran dari audit_log, yang append-only dan menyimpan keadaan sebelum/sesudah
// tiap aksi (snapshot userSnapshot: nama, email, peran, lapis, ...). Satu entri = satu baris audit BERHASIL
// pada entitas "pengguna" dengan id ini yang mencatat peran sesudah dan berbeda dari peran sebelum; peran
// sebelum kosong berarti akun dibuat (peran awal). Aksi tidak dibatasi pada daftar tertentu: jalur apa pun yang
// mengubah peran dan mencatat snapshot (pembuatan, PUT /roles, PATCH, undangan, impor) ikut tanpa perubahan di
// sini. Memakai indeks idx_audit_log_entitas_waktu (entitas, entitas_id, waktu DESC) dari migrasi 000011.
func (r *Repository) RiwayatPeran(ctx context.Context, userID string, limit int) ([]domain.PerubahanPeran, error) {
	var baris []struct {
		ID        string
		Waktu     time.Time
		Aksi      string
		Dari      *string
		Ke        string
		AktorID   *string
		AktorNama *string
	}
	err := r.db.WithContext(ctx).Raw(`
		SELECT id, waktu, aksi, sebelum->>'peran' AS dari, sesudah->>'peran' AS ke, aktor_id, aktor_nama
		FROM audit_log
		WHERE entitas = 'pengguna' AND entitas_id = ? AND hasil = 'BERHASIL'
		  AND sesudah->>'peran' IS NOT NULL
		  AND sebelum->>'peran' IS DISTINCT FROM sesudah->>'peran'
		ORDER BY waktu DESC, id DESC
		LIMIT ?`, userID, limit).Scan(&baris).Error
	if err != nil {
		return nil, fmt.Errorf("baca riwayat peran: %w", err)
	}

	out := make([]domain.PerubahanPeran, len(baris))
	for i, b := range baris {
		out[i] = domain.PerubahanPeran{
			AuditID: b.ID, Waktu: b.Waktu, Aksi: b.Aksi, Dari: b.Dari, Ke: b.Ke, AktorID: b.AktorID, AktorNama: b.AktorNama,
		}
	}
	return out, nil
}
