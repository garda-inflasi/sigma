package domain

import (
	"context"
	"time"
)

// PembersihanRepository menghapus baris autentikasi yang sudah mati. Dipisah dari Repository karena
// hanya dipakai job pembersihan di cmd/worker, bukan jalur request.
//
// Setiap metode menghapus paling banyak batas baris (bukan seluruhnya) dalam satu pernyataan singkat
// dan mengembalikan jumlah baris yang terhapus; pemanggil mengulang sampai hasilnya kurang dari batas.
// Baris yang sedang dikunci transaksi lain dilewati (dihapus pada putaran berikutnya), tidak ditunggu.
// Batas waktu (sebelum) selalu perbandingan KETAT: baris yang waktunya tepat sama dengan sebelum dipertahankan.
type PembersihanRepository interface {
	// HapusBlacklistKedaluwarsa menghapus jwt_blacklist dengan expires_at < sebelum.
	HapusBlacklistKedaluwarsa(ctx context.Context, sebelum time.Time, batas int) (int64, error)
	// HapusRefreshTokenKedaluwarsa menghapus refresh_tokens dengan expires_at < sebelum. Baris yang sudah
	// dicabut tetapi belum kedaluwarsa TIDAK disentuh: deteksi pemakaian ulang pada Refresh membutuhkannya.
	HapusRefreshTokenKedaluwarsa(ctx context.Context, sebelum time.Time, batas int) (int64, error)
	// HapusResetTokenMati menghapus password_reset_tokens yang kedaluwarsa (expires_at < sebelum) atau
	// sudah dipakai (used_at < sebelum). Token yang belum dipakai dan belum kedaluwarsa tidak disentuh.
	HapusResetTokenMati(ctx context.Context, sebelum time.Time, batas int) (int64, error)
	// HapusUndanganMati menghapus undangan_pengguna yang kedaluwarsa (expires_at < sebelum), sudah dipakai
	// (used_at < sebelum), atau dicabut (dicabut_pada < sebelum). Undangan hidup tidak disentuh.
	HapusUndanganMati(ctx context.Context, sebelum time.Time, batas int) (int64, error)
	// HapusLoginAttemptLama menghapus login_attempts dengan attempted_at < sebelum.
	HapusLoginAttemptLama(ctx context.Context, sebelum time.Time, batas int) (int64, error)
	// HapusKunciLoginUsang menghapus kunci_login yang terakhir diperbarui sebelum `sebelum` dan tidak sedang
	// terkunci pada `sekarang`.
	HapusKunciLoginUsang(ctx context.Context, sebelum, sekarang time.Time, batas int) (int64, error)
	// HapusSSOStateKedaluwarsa menghapus sso_login_state dengan expires_at < sebelum (alur SSO yang ditinggalkan).
	HapusSSOStateKedaluwarsa(ctx context.Context, sebelum time.Time, batas int) (int64, error)
}
