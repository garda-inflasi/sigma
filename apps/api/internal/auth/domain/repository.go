package domain

import (
	"context"
	"time"
)

type Repository interface {
	// KunciRepository menyimpan lockout login: satu-satunya mekanisme penghitung percobaan gagal.
	KunciRepository
	SesiRepository
	GetUserByID(ctx context.Context, id string) (*Pengguna, error)
	GetUserByEmail(ctx context.Context, email string) (*Pengguna, error)
	CreateUser(ctx context.Context, user *Pengguna) error
	// Tidak ada metode yang menyimpan seluruh baris pengguna (GORM Save dari salinan yang dibaca lebih dulu):
	// salinan itu basi begitu ada perubahan serentak, dan penyimpanan penuh mengembalikan kolom yang bukan
	// miliknya (akun yang baru dinonaktifkan kembali AKTIF, hash hasil reset admin kembali ke yang lama).
	// Tiap jalur menulis hanya kolom yang menjadi miliknya, dengan syarat keadaan yang diharapkan bila perlu.

	// PatchUser menulis hanya kolom pada patch (profil, peran, status) dan mengembalikan baris persis sebelum dan
	// sesudah penulisan, dibaca dalam satu transaksi dengan kunci baris sehingga snapshot audit akurat walau ada
	// perubahan serentak pada kolom lain. Galat membungkus gorm.ErrRecordNotFound bila pengguna tidak ada.
	PatchUser(ctx context.Context, id string, patch PenggunaPatch) (sebelum, sesudah *Pengguna, err error)
	// SetPassword menulis hanya kolom kata sandi (hash, wajib_ganti_password, password_diubah_pada). false bila
	// tidak ada baris yang cocok: pengguna tidak ada, atau GantiPassword.HashSebelumnya sudah tidak sama.
	SetPassword(ctx context.Context, userID string, p GantiPassword) (bool, error)
	// SetMFASecret menyimpan rahasia setup MFA (bersegel) yang belum diverifikasi dan mengosongkan langkah TOTP,
	// hanya bila MFA belum aktif. false bila MFA sudah diaktifkan (permintaan lain lebih dulu) atau pengguna tidak ada.
	SetMFASecret(ctx context.Context, userID, secretTersegel string) (bool, error)
	// TouchLastLogin hanya menulis terakhir_masuk. Login memakainya agar tidak menimpa perubahan admin yang
	// terjadi selama pemeriksaan kata sandi.
	TouchLastLogin(ctx context.Context, userID string, at time.Time) error
	DeleteUser(ctx context.Context, id string) error
	ListUsers(ctx context.Context, page, limit int, search string) ([]Pengguna, int64, error)
	CreateRefreshToken(ctx context.Context, token *RefreshToken) error
	GetRefreshTokenByHash(ctx context.Context, hash string) (*RefreshToken, error)
	RevokeRefreshToken(ctx context.Context, id int64) error
	RevokeRefreshTokenFamily(ctx context.Context, familyID string) error
	RevokeAllUserRefreshTokens(ctx context.Context, userID string) error
	BlacklistJWT(ctx context.Context, jti string, ttl time.Duration) error
	IsJWTBlacklisted(ctx context.Context, jti string) (bool, error)
	CreateLoginAttempt(ctx context.Context, attempt *LoginAttempt) error
	SaveResetToken(ctx context.Context, userID, tokenHash string, expiresAt time.Time) error
	GetResetToken(ctx context.Context, tokenHash string) (*ResetToken, error)
	// ConsumeResetToken memakai token satu kali secara atomik: berhasil hanya bila token belum dipakai dan
	// belum kedaluwarsa pada now, lalu menghanguskan token lain milik pengguna yang sama. Mengembalikan
	// false bila token sudah dipakai (mis. oleh permintaan serentak) atau kedaluwarsa.
	ConsumeResetToken(ctx context.Context, id int64, now time.Time) (bool, error)
	BulkUpdateUsers(ctx context.Context, ids []string, action BulkUserAction) (int64, error)
	// ActivateMFA mengaktifkan MFA dan mengganti seluruh kode cadangan (hash) dalam satu transaksi, hanya bila MFA
	// belum aktif dan rahasia tersimpan masih secretTersegel (rahasia yang kodenya baru saja diverifikasi). false
	// bila syarat itu tidak terpenuhi: aktivasi serentak, atau setup diulang dengan rahasia baru di sela-selanya.
	ActivateMFA(ctx context.Context, userID string, kodeHash []string, langkah int64, secretTersegel string) (bool, error)
	// ReplaceBackupCodes mengganti seluruh kode cadangan (hash) milik pengguna.
	ReplaceBackupCodes(ctx context.Context, userID string, kodeHash []string) error
	// UseBackupCode menandai satu kode cadangan terpakai; true hanya bila kode ada dan belum terpakai.
	UseBackupCode(ctx context.Context, userID, kodeHash string, now time.Time) (bool, error)
	// CountBackupCodes menghitung kode cadangan yang belum terpakai.
	CountBackupCodes(ctx context.Context, userID string) (int64, error)
	// AdvanceMFAStep memajukan langkah TOTP terakhir; false bila langkah itu sudah pernah dipakai.
	AdvanceMFAStep(ctx context.Context, userID string, langkah int64) (bool, error)
	// ClearMFA menonaktifkan MFA: rahasia, langkah, dan semua kode cadangan dihapus.
	ClearMFA(ctx context.Context, userID string) error
}
