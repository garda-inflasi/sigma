package domain

import (
	"context"
	"time"
)

// SesiRepository adalah operasi atas sesi: satu sesi (perangkat) adalah satu family_id pada refresh_tokens,
// yang juga menjadi klaim `sid` access token. Sesi hidup selama ada token dalam family-nya yang belum dicabut.
type SesiRepository interface {
	// RotateRefreshToken mengganti token oldID dengan next secara atomik (satu transaksi): token lama dicabut
	// dan ditandai hasil rotasi hanya bila masih hidup (revoked_at IS NULL) dan belum kedaluwarsa pada now,
	// lalu token pengganti disimpan. Mengembalikan false tanpa menulis apa pun bila token sudah dicabut oleh
	// permintaan lain, sehingga dua refresh serentak dengan token yang sama tidak mungkin sama-sama berhasil.
	RotateRefreshToken(ctx context.Context, oldID int64, next *RefreshToken, now time.Time) (bool, error)
	// IsSessionActive melaporkan apakah family masih punya token yang belum dicabut.
	IsSessionActive(ctx context.Context, familyID string) (bool, error)
	// ListActiveRefreshTokens mengembalikan token hidup (belum dicabut, belum kedaluwarsa) milik pengguna;
	// satu sesi biasanya satu baris.
	ListActiveRefreshTokens(ctx context.Context, userID string, now time.Time) ([]RefreshToken, error)
	// RevokeSessions mencabut token hidup milik pengguna dan mengembalikan id sesi (family) yang tercabut.
	// keepFamilyID tidak dicabut; kosong berarti cabut semuanya.
	RevokeSessions(ctx context.Context, userID, keepFamilyID string, now time.Time) ([]string, error)
	// RevokeSession mencabut satu sesi milik pengguna; false bila sesi itu tidak ada, bukan milik pengguna,
	// atau sudah tidak hidup.
	RevokeSession(ctx context.Context, userID, familyID string, now time.Time) (bool, error)
	// RetireFamilyTokensExcept menandai semua token hidup dalam family selain keepTokenHash sebagai hasil
	// rotasi (dipakai saat token pengganti diterbitkan tanpa menyajikan token lama, mis. ganti kata sandi).
	RetireFamilyTokensExcept(ctx context.Context, familyID, keepTokenHash string, now time.Time) error
}
