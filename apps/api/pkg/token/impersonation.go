package token

import (
	"context"
	"errors"
	"strings"
)

// Impersonasi (docs/impersonasi.md): selama impersonasi, access token milik akun SASARAN (sub, role, permissions,
// profil = sasaran) dan membawa klaim `imp` yang menunjuk pelaku asli. Klaim ini hanya penunjuk: JWTAuth selalu
// memeriksanya terhadap catatan impersonasi di basis data (ImpersonationGuard), jadi token yang catatannya sudah
// berakhir, kebijakannya dinonaktifkan, atau pelakunya sudah tidak berhak langsung ditolak walau tanda tangannya sah.

// ErrImpersonationEnded dikembalikan ImpersonationGuard bila sesi impersonasi pada token sudah tidak sah (berakhir,
// kedaluwarsa, kebijakan nonaktif, pelaku atau sasaran tidak lagi berhak). JWTAuth menjawabnya 401.
var ErrImpersonationEnded = errors.New("sesi impersonasi telah berakhir")

// ImpersonatedBy adalah pelaku asli (akun yang sesungguhnya login) pada klaim `imp`.
type ImpersonatedBy struct {
	UserID string `json:"user_id"`
	Name   string `json:"name"`
	Email  string `json:"email"`
	Role   string `json:"role"`
}

// Impersonation adalah klaim `imp` access token: id catatan impersonasi (impersonasi_sesi.id), pelaku asli, dan
// waktu mulai (detik unix). Web membacanya hanya untuk tampilan; otorisasi tidak bergantung padanya.
type Impersonation struct {
	ID        string         `json:"id"`
	By        ImpersonatedBy `json:"by"`
	StartedAt int64          `json:"started_at"`
}

// WithImpersonation menandai access token sebagai token impersonasi (sub/role/permissions sudah milik sasaran).
func WithImpersonation(imp Impersonation) ClaimOption {
	return func(c *Claims) {
		c.Imp = &imp
	}
}

// ── Konteks ───────────────────────────────────────────────────────────

// ImpersonationContext dibawa JWTAuth di context request selama impersonasi supaya penulis audit dan usecase
// dapat menautkan setiap aksi ke pelaku asli tanpa mengubah tanda tangan fungsi mana pun.
type ImpersonationContext struct {
	Impersonation
	// EffectiveUserID dan EffectiveRole adalah akun dan peran sasaran yang sedang dipakai.
	EffectiveUserID string
	EffectiveRole   string
}

type impersonationKey struct{}

// ContextWithImpersonation menyimpan keadaan impersonasi pada context.
func ContextWithImpersonation(ctx context.Context, ic ImpersonationContext) context.Context {
	return context.WithValue(ctx, impersonationKey{}, ic)
}

// ImpersonationFrom membaca keadaan impersonasi dari context; ok false bila request bukan impersonasi.
func ImpersonationFrom(ctx context.Context) (ImpersonationContext, bool) {
	ic, ok := ctx.Value(impersonationKey{}).(ImpersonationContext)
	return ic, ok
}

// ── Penjaga ───────────────────────────────────────────────────────────

// MutationInfo menjelaskan satu request mutasi yang selesai selama impersonasi.
type MutationInfo struct {
	Method    string
	Route     string
	Status    int
	IP        string
	UserAgent string
	RequestID string
}

// ImpersonationGuard memvalidasi token impersonasi pada setiap request dan mencatat mutasi yang dilakukan lewatnya.
// Implementasi harus aman dipakai bersamaan dan murah (cache pendek): dipanggil pada setiap request ber-`imp`.
type ImpersonationGuard interface {
	// ValidateImpersonation memeriksa catatan impersonasi pada klaim dan mengembalikan izin efektif TERKINI akun
	// sasaran. ErrImpersonationEnded bila sesinya tidak lagi sah; ErrPrincipalNotFound bila akun sasaran sudah tidak
	// ada atau tidak aktif; galat lain berarti keadaan tidak dapat dipastikan (JWTAuth menolak 503, gagal-tutup).
	ValidateImpersonation(ctx context.Context, claims *Claims) ([]string, error)
	// RecordImpersonationMutation mencatat satu request mutasi (POST/PUT/PATCH/DELETE) selama impersonasi.
	RecordImpersonationMutation(ctx context.Context, ic ImpersonationContext, info MutationInfo)
}

// SetImpersonationGuard memasang penjaga impersonasi yang dipakai JWTAuth, dengan cara yang sama seperti
// SetPermissionResolver: dipasang sekali oleh internal/auth/routes.Register pada Manager yang dibagi seluruh modul.
// Nil melepas penjaga; token ber-`imp` lalu selalu ditolak (gagal-tutup).
func (m *Manager) SetImpersonationGuard(g ImpersonationGuard) {
	if g == nil {
		m.impersonation.Store(nil)
		return
	}
	m.impersonation.Store(&g)
}

// ImpersonationGuard mengembalikan penjaga terpasang (nil bila tidak ada).
func (m *Manager) ImpersonationGuard() ImpersonationGuard {
	g := m.impersonation.Load()
	if g == nil {
		return nil
	}
	return *g
}

// ValidImpersonationClaim true bila klaim `imp` lengkap (id dan pelaku asli terisi). Klaim cacat tidak pernah
// dianggap "bukan impersonasi": token seperti itu ditolak, bukan diperlakukan sebagai sesi biasa.
func ValidImpersonationClaim(imp *Impersonation) bool {
	return imp != nil && strings.TrimSpace(imp.ID) != "" && strings.TrimSpace(imp.By.UserID) != ""
}
