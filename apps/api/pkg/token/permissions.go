package token

import (
	"context"
	"errors"
	"slices"
)

// ErrPrincipalNotFound dikembalikan PermissionResolver bila akun pemilik token sudah tidak ada atau tidak aktif.
// Token yang tanda tangannya sah tidak lagi berguna: JWTAuth menjawab 401.
var ErrPrincipalNotFound = errors.New("akun tidak ditemukan atau tidak aktif")

// PermissionResolver menjawab izin efektif TERKINI satu pengguna (izin peran digabung izin langsung yang
// masih berlaku, dari basis data). Klaim `permissions` pada access token hanyalah salinan saat terbit; yang
// ditegakkan API adalah jawaban resolver ini, sehingga pencabutan izin, izin langsung yang kedaluwarsa, dan
// perubahan matriks peran berlaku dalam batas waktu cache resolver, bukan menunggu token (15 menit) habis.
// Implementasi harus aman dipakai bersamaan dan murah (cache): dipanggil pada setiap request terotentikasi.
type PermissionResolver interface {
	// Permissions mengembalikan izin efektif userID. ErrPrincipalNotFound bila akunnya tidak ada atau tidak
	// aktif; galat lain berarti izin tidak dapat dipastikan (JWTAuth menolak 503, gagal-tutup).
	Permissions(ctx context.Context, userID string) ([]string, error)
}

// SetPermissionResolver memasang resolver izin yang dipakai middleware JWTAuth, dengan cara yang sama seperti
// SetRevocationChecker: dipasang sekali oleh internal/auth/routes.Register pada Manager yang dibagi seluruh
// modul, sehingga tanda tangan JWTAuth(tokenMgr) tidak berubah. Nil melepas resolver (izin diambil dari klaim).
func (m *Manager) SetPermissionResolver(r PermissionResolver) {
	if r == nil {
		m.permissions.Store(nil)
		return
	}
	m.permissions.Store(&r)
}

// ResolvePermissions mengembalikan izin efektif terkini pemilik klaim. resolved false (dan perms nil) bila
// tidak ada resolver terpasang: router tanpa modul auth (mis. tes modul lain) memakai klaim apa adanya.
// Galat dikembalikan apa adanya; kebijakan gagal-tutup ada di middleware.
func (m *Manager) ResolvePermissions(ctx context.Context, claims *Claims) (perms []string, resolved bool, err error) {
	r := m.permissions.Load()
	if r == nil || claims == nil {
		return nil, false, nil
	}
	perms, err = (*r).Permissions(ctx, claims.UserID)
	if err != nil {
		return nil, true, err
	}
	return perms, true, nil
}

// SamePermissions true bila kedua daftar berisi himpunan izin yang sama (urutan dan duplikat diabaikan).
func SamePermissions(a, b []string) bool {
	return slices.Equal(himpunan(a), himpunan(b))
}

func himpunan(izin []string) []string {
	out := slices.Clone(izin)
	slices.Sort(out)
	return slices.Compact(out)
}
