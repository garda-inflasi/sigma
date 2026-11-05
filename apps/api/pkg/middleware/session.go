package middleware

import (
	"context"
	"time"

	"github.com/gin-gonic/gin"
)

const (
	sessionIDKey   = "sid"
	tokenExpiryKey = "token_exp"
)

// SessionRevokedMessage adalah pesan 401 untuk access token yang sudah dicabut (logout, sesi dicabut,
// dipaksa keluar). Klien web memperlakukannya seperti token kedaluwarsa: mencoba refresh, dan bila
// refresh juga ditolak, mengarahkan pengguna ke halaman masuk.
const SessionRevokedMessage = "Sesi telah berakhir, silakan masuk kembali"

// SessionID mengembalikan id sesi (klaim sid) dari access token yang sedang dipakai; kosong untuk token
// yang terbit sebelum klaim itu ada. Hanya terisi setelah JWTAuth.
func SessionID(c *gin.Context) string {
	return c.GetString(sessionIDKey)
}

// TokenLifetimeContext mengembalikan context request yang berakhir saat access token kedaluwarsa. Untuk
// koneksi berumur panjang (SSE): JWTAuth hanya memeriksa token saat koneksi dibuka, sehingga tanpa batas ini
// aliran tetap hidup setelah sesinya dicabut atau tokennya habis. Dengan batas ini klien menyambung ulang
// paling lambat setiap masa berlaku access token, dan sambungan ulang melewati pemeriksaan pencabutan lagi.
func TokenLifetimeContext(c *gin.Context) (context.Context, context.CancelFunc) {
	if exp, ok := c.Get(tokenExpiryKey); ok {
		if t, ok := exp.(time.Time); ok {
			return context.WithDeadline(c.Request.Context(), t)
		}
	}
	return context.WithCancel(c.Request.Context())
}
