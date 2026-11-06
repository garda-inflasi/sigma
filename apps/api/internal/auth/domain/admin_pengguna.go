package domain

import (
	"context"
	"time"
)

// HasilResetMFA adalah keadaan MFA sebelum reset oleh admin dan apa yang dihapusnya.
type HasilResetMFA struct {
	// MFAAktifSebelumnya true bila MFA memang aktif ketika direset.
	MFAAktifSebelumnya bool
	// RahasiaDihapus true bila ada rahasia TOTP tersimpan (aktif, atau setup yang belum diverifikasi).
	RahasiaDihapus bool
	// KodeCadanganDihapus adalah jumlah baris kode cadangan (terpakai maupun belum) yang dihapus.
	KodeCadanganDihapus int64
	// SesiDicabut adalah id sesi (family) yang dicabut; hanya terisi bila MFA memang aktif.
	SesiDicabut []string
}

// ResetMFARepository menghapus faktor kedua satu pengguna atas perintah admin.
type ResetMFARepository interface {
	// ResetMFA menonaktifkan MFA pengguna dalam SATU transaksi: baris pengguna dikunci, mfa_aktif dimatikan,
	// rahasia dan langkah TOTP (anti-ulang) dikosongkan, semua kode cadangan dihapus, dan, hanya bila MFA
	// memang aktif, semua sesi pengguna dicabut pada waktu now. Atomik: tidak ada keadaan setengah jadi,
	// misalnya MFA hilang tetapi sesi lama tetap hidup. Idempoten: pengguna tanpa MFA tidak ditulis ulang
	// (diupdate_pada tidak berubah) dan sesinya tidak disentuh. Galat membungkus gorm.ErrRecordNotFound
	// bila pengguna tidak ada.
	ResetMFA(ctx context.Context, userID string, now time.Time) (*HasilResetMFA, error)
}

// PerubahanPeran adalah satu entri riwayat peran, dibaca dari audit_log.
type PerubahanPeran struct {
	// AuditID adalah id baris audit sumbernya (untuk tautan ke /audit).
	AuditID string
	Waktu   time.Time
	// Aksi adalah aksi audit sumber: CREATE_USER (peran awal), ASSIGN_ROLE, UPDATE_USER, dan aksi lain yang
	// mencatat peran sebelum/sesudah pada entitas pengguna.
	Aksi string
	// Dari kosong untuk peran awal (akun dibuat).
	Dari *string
	Ke   string
	// AktorID kosong bila akun aktor sudah dihapus atau aksinya tanpa aktor; AktorNama tetap terbaca dari snapshot.
	AktorID   *string
	AktorNama *string
}

// RiwayatPeranRepository membaca riwayat peran satu pengguna.
type RiwayatPeranRepository interface {
	// RiwayatPeran mengembalikan perubahan peran pengguna (baris audit BERHASIL pada entitas pengguna yang
	// mencatat peran sesudah dan berbeda dari peran sebelum, atau peran awal), terbaru lebih dulu, sampai
	// limit baris.
	RiwayatPeran(ctx context.Context, userID string, limit int) ([]PerubahanPeran, error)
}
