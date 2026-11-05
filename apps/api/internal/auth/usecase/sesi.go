package usecase

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/garda-inflasi/api/internal/audit"
	auditdomain "github.com/garda-inflasi/api/internal/audit/domain"
	"github.com/garda-inflasi/api/internal/auth/domain"
	"github.com/garda-inflasi/api/pkg/token"
	"github.com/google/uuid"
	"gorm.io/gorm"
)

// Sesi: satu sesi (perangkat) = satu family refresh token = klaim `sid` pada access token. Sesi hidup selama
// ada token dalam family-nya yang belum dicabut; mencabut sesi seketika mematikan semua access token-nya
// (pkg/middleware.JWTAuth memeriksa sid pada setiap request).
//
// Rotasi. Setiap refresh mencabut token yang disajikan dan menerbitkan penggantinya dalam satu transaksi
// bersyarat (RotateRefreshToken), sehingga dua refresh serentak dengan token yang sama tidak mungkin keduanya
// menang. Token yang sudah dirotasi lalu disajikan lagi:
//   - dalam jendela toleransi (Config.RefreshGraceWindow) dan sesinya masih hidup: dianggap tab/permintaan
//     serentak yang sah. Hanya access token baru yang diberikan (RefreshToken kosong); pemenang rotasi sudah
//     menerbitkan satu-satunya token pengganti dan klien yang menang memasangnya di cookie bersama.
//   - lewat jendela itu: pemakaian ulang token lama (kemungkinan dicuri). Seluruh family dicabut, termasuk
//     access token-nya, dan TOKEN_REUSE dicatat di audit.
//
// Trade-off jendela toleransi: pemegang token lama yang mengulang dalam beberapa detik setelah rotasi
// memperoleh access token (maks. 15 menit) tanpa dapat memutarnya lagi, sedangkan penolakan keras akan
// mengeluarkan pengguna dari semua tab setiap kali dua tab menyegarkan sesi bersamaan. Jendela pendek
// (bawaan 10 detik) dan hanya untuk sesi yang masih hidup menekan risikonya; Config.RefreshGraceWindow 0
// mematikannya.
var (
	ErrSessionNotFound       = errors.New("sesi tidak ditemukan")
	ErrCannotRevokeCurrent   = errors.New("sesi yang sedang dipakai tidak dapat dicabut dari daftar, gunakan keluar")
	ErrCurrentSessionUnknown = errors.New("sesi ini dibuat sebelum pembaruan sistem, masuk ulang lalu coba lagi")
)

// DefaultRefreshGraceWindow adalah jendela toleransi bawaan untuk refresh serentak dengan token yang sama.
const DefaultRefreshGraceWindow = 10 * time.Second

// maxUserAgentLen membatasi user agent yang disimpan (kolom VARCHAR(512)).
const maxUserAgentLen = 512

// ── Konteks sesi ──────────────────────────────────────────────────────

type sesiCtxKey struct{}

// WithSession membawa id sesi (klaim sid) access token yang sedang dipakai ke usecase, seperti
// audit.Origin membawa IP: tanda tangan usecase tidak perlu berubah. Kosong untuk token tanpa sid.
func WithSession(ctx context.Context, sessionID string) context.Context {
	return context.WithValue(ctx, sesiCtxKey{}, sessionID)
}

// currentSession adalah id sesi yang dibawa WithSession; kosong bila tidak ada.
func currentSession(ctx context.Context) string {
	sid, _ := ctx.Value(sesiCtxKey{}).(string)
	return sid
}

// ── Penerbitan ────────────────────────────────────────────────────────

type mintedSession struct {
	accessToken  string
	refreshToken string
	row          *domain.RefreshToken
}

// mintAccess membuat access token yang terikat ke sesi familyID (klaim sid).
func (uc *UseCase) mintAccess(user *domain.Pengguna, familyID string) (string, error) {
	kabupaten := ""
	if user.KabupatenKode != nil {
		kabupaten = *user.KabupatenKode
	}
	accessToken, err := uc.token.CreateAccessToken(user.ID, user.Peran, uc.getPermissionsForRole(user.Peran),
		token.WithPasswordChangeRequired(user.WajibGantiPassword),
		token.WithMFASetupRequired(uc.mfaSetupRequired(user)),
		token.WithProfile(user.Nama, user.Email, user.Lembaga, kabupaten),
		token.WithSession(familyID))
	if err != nil {
		return "", fmt.Errorf("create access token: %w", err)
	}
	return accessToken, nil
}

// mintSession membuat access token (terikat sid = familyID) dan baris refresh token baru tanpa menyimpannya.
// startedAt nol berarti sesi baru dimulai now. IP dan user agent mengikuti permintaan yang sedang berjalan.
func (uc *UseCase) mintSession(ctx context.Context, user *domain.Pengguna, familyID, ip string, startedAt, now time.Time) (*mintedSession, error) {
	accessToken, err := uc.mintAccess(user, familyID)
	if err != nil {
		return nil, err
	}

	refreshToken, refreshHash, err := uc.token.CreateRefreshToken()
	if err != nil {
		return nil, fmt.Errorf("create refresh token: %w", err)
	}
	if startedAt.IsZero() {
		startedAt = now
	}
	row := &domain.RefreshToken{
		UserID:           user.ID,
		TokenHash:        refreshHash,
		FamilyID:         familyID,
		UserAgent:        cleanUserAgent(auditdomain.OriginFrom(ctx).UserAgent),
		ExpiresAt:        now.Add(uc.cfg.RefreshTokenTTL),
		LastUsedAt:       now,
		SessionStartedAt: startedAt,
	}
	if ip != "" {
		row.IP = &ip
	}
	return &mintedSession{accessToken: accessToken, refreshToken: refreshToken, row: row}, nil
}

// cleanUserAgent memangkas user agent ke batas kolom tanpa memotong karakter UTF-8 dan membuang byte yang
// ditolak PostgreSQL; nil bila kosong.
func cleanUserAgent(ua string) *string {
	ua = strings.TrimSpace(strings.ToValidUTF8(ua, ""))
	ua = strings.ReplaceAll(ua, "\x00", "")
	if ua == "" {
		return nil
	}
	if utf8.RuneCountInString(ua) > maxUserAgentLen {
		ua = string([]rune(ua)[:maxUserAgentLen])
	}
	return &ua
}

// replaceCurrentSession menerbitkan token baru untuk sesi yang sedang dipakai setelah perubahan kredensial
// (ganti kata sandi, aktivasi MFA): sesi lain dicabut, sesi ini tetap hidup dalam family yang sama dengan
// refresh token baru, dan token lamanya dipensiunkan. Tanpa sesi yang dikenal (token lama tanpa sid, atau sesi
// sudah dicabut di sela-sela) semua sesi dicabut dan sesi baru dimulai, seperti perilaku sebelumnya.
func (uc *UseCase) replaceCurrentSession(ctx context.Context, user *domain.Pengguna, ip string) (accessToken, refreshToken string, err error) {
	now := time.Now()
	sid := currentSession(ctx)

	var current *domain.RefreshToken
	if sid != "" {
		rows, err := uc.repo.ListActiveRefreshTokens(ctx, user.ID, now)
		if err != nil {
			return "", "", fmt.Errorf("list sessions: %w", err)
		}
		for i := range rows {
			if rows[i].FamilyID == sid {
				current = &rows[i]
				break
			}
		}
	}

	if current == nil {
		if err := uc.repo.RevokeAllUserRefreshTokens(ctx, user.ID); err != nil {
			return "", "", fmt.Errorf("revoke sessions: %w", err)
		}
		return uc.issueSession(ctx, user, uuid.New().String(), ip)
	}

	if _, err := uc.repo.RevokeSessions(ctx, user.ID, sid, now); err != nil {
		return "", "", fmt.Errorf("revoke other sessions: %w", err)
	}
	minted, err := uc.mintSession(ctx, user, sid, ip, current.SessionStartedAt, now)
	if err != nil {
		return "", "", err
	}
	if err := uc.repo.CreateRefreshToken(ctx, minted.row); err != nil {
		return "", "", fmt.Errorf("save refresh token: %w", err)
	}
	if err := uc.repo.RetireFamilyTokensExcept(ctx, sid, minted.row.TokenHash, now); err != nil {
		return "", "", err
	}
	return minted.accessToken, minted.refreshToken, nil
}

// revokeAllSessions mencabut semua sesi pengguna dan melaporkan kegagalannya: akun yang dinonaktifkan atau
// kata sandinya direset tidak boleh tetap punya sesi hidup hanya karena pencabutan gagal tanpa suara.
func (uc *UseCase) revokeAllSessions(ctx context.Context, userID string) error {
	if err := uc.repo.RevokeAllUserRefreshTokens(ctx, userID); err != nil {
		return fmt.Errorf("revoke sessions: %w", err)
	}
	return nil
}

// ── Refresh ───────────────────────────────────────────────────────────

// RefreshResult adalah hasil refresh. RefreshToken kosong bila permintaan ini adalah pemutaran ulang dalam
// jendela toleransi (Replayed): klien harus mempertahankan refresh token yang sudah diterimanya dari
// permintaan pemenang.
type RefreshResult struct {
	AccessToken  string
	RefreshToken string
	Replayed     bool
}

// RefreshSession menukar refresh token dengan access token baru dan, kecuali pada pemutaran ulang yang
// ditoleransi, refresh token pengganti. Lihat komentar paket sesi untuk aturan rotasi dan pemakaian ulang.
func (uc *UseCase) RefreshSession(ctx context.Context, refreshTokenStr, ip string) (*RefreshResult, error) {
	stored, err := uc.findRefreshToken(ctx, refreshTokenStr)
	if err != nil {
		return nil, err
	}

	if stored.RevokedAt == nil {
		result, rotated, err := uc.rotate(ctx, stored, ip)
		if err != nil || rotated {
			return result, err
		}
		// Kalah berebut: permintaan lain memutar token yang sama lebih dulu. Baca ulang untuk melihat
		// apakah itu rotasi (tab serentak) atau pencabutan sesi.
		stored, err = uc.findRefreshToken(ctx, refreshTokenStr)
		if err != nil {
			return nil, err
		}
		if stored.RevokedAt == nil {
			// Tidak mungkin terjadi kecuali token kedaluwarsa di sela-sela pembacaan dan pembaruan.
			return nil, ErrTokenExpired
		}
	}
	return uc.replayOrReuse(ctx, stored)
}

func (uc *UseCase) findRefreshToken(ctx context.Context, refreshTokenStr string) (*domain.RefreshToken, error) {
	stored, err := uc.repo.GetRefreshTokenByHash(ctx, sha256Hex(refreshTokenStr))
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrInvalidToken
		}
		// Galat basis data tidak boleh menyamar sebagai token tidak sah: klien akan mengeluarkan pengguna.
		return nil, fmt.Errorf("get refresh token: %w", err)
	}
	return stored, nil
}

// rotate memutar token yang masih hidup. rotated false bila token keburu dicabut permintaan lain.
func (uc *UseCase) rotate(ctx context.Context, stored *domain.RefreshToken, ip string) (*RefreshResult, bool, error) {
	now := time.Now()
	if !now.Before(stored.ExpiresAt) {
		return nil, false, ErrTokenExpired
	}

	user, err := uc.activeUser(ctx, stored)
	if err != nil {
		return nil, false, err
	}
	minted, err := uc.mintSession(ctx, user, stored.FamilyID, ip, stored.SessionStartedAt, now)
	if err != nil {
		return nil, false, err
	}
	rotated, err := uc.repo.RotateRefreshToken(ctx, stored.ID, minted.row, now)
	if err != nil {
		return nil, false, fmt.Errorf("rotate refresh token: %w", err)
	}
	if !rotated {
		return nil, false, nil
	}
	uc.log.Info("refresh success", map[string]interface{}{"user_id": stored.UserID, "ip": ip})
	return &RefreshResult{AccessToken: minted.accessToken, RefreshToken: minted.refreshToken}, true, nil
}

// activeUser memuat pemilik token dan menolak akun yang sudah tidak aktif; sesinya dicabut sekalian.
func (uc *UseCase) activeUser(ctx context.Context, stored *domain.RefreshToken) (*domain.Pengguna, error) {
	user, err := uc.repo.GetUserByID(ctx, stored.UserID)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrInvalidToken
		}
		return nil, fmt.Errorf("get user: %w", err)
	}
	if user.Status != "AKTIF" {
		_ = uc.repo.RevokeRefreshTokenFamily(ctx, stored.FamilyID)
		return nil, ErrInvalidToken
	}
	return user, nil
}

// replayOrReuse memutuskan nasib token yang sudah dicabut: pemutaran ulang yang ditoleransi, pencabutan
// biasa, atau pemakaian ulang token hasil rotasi.
func (uc *UseCase) replayOrReuse(ctx context.Context, stored *domain.RefreshToken) (*RefreshResult, error) {
	// Dicabut tanpa digantikan (logout, "Cabut", paksa keluar, penonaktifan): sesi memang sudah berakhir;
	// ini kejadian biasa, bukan tanda pencurian.
	if stored.RotatedAt == nil {
		return nil, ErrInvalidToken
	}

	now := time.Now()
	if uc.cfg.RefreshGraceWindow > 0 && now.Sub(*stored.RotatedAt) <= uc.cfg.RefreshGraceWindow {
		active, err := uc.repo.IsSessionActive(ctx, stored.FamilyID)
		if err != nil {
			return nil, fmt.Errorf("check session: %w", err)
		}
		if active {
			user, err := uc.activeUser(ctx, stored)
			if err != nil {
				return nil, err
			}
			accessToken, err := uc.mintAccess(user, stored.FamilyID)
			if err != nil {
				return nil, err
			}
			uc.log.Info("refresh replay dalam jendela toleransi", map[string]interface{}{"user_id": stored.UserID, "family_id": stored.FamilyID})
			return &RefreshResult{AccessToken: accessToken, Replayed: true}, nil
		}
		return nil, ErrInvalidToken
	}

	// Pemakaian ulang token yang sudah diganti: cabut seluruh family (juga access token-nya lewat sid).
	uc.log.Warn("refresh: reuse detected, revoking family", map[string]interface{}{
		"family_id": stored.FamilyID,
		"user_id":   stored.UserID,
	})
	if err := uc.repo.RevokeRefreshTokenFamily(ctx, stored.FamilyID); err != nil {
		uc.log.Error("refresh: gagal mencabut family setelah reuse", map[string]interface{}{"family_id": stored.FamilyID, "error": err.Error()})
	}
	uc.writeAudit(ctx, audit.Entry{
		AktorID: stored.UserID, Aksi: "TOKEN_REUSE", Entitas: "refresh_token", EntitasID: fmt.Sprintf("%d", stored.ID),
		Hasil: audit.HasilDitolak, Metadata: map[string]any{"family_id": stored.FamilyID},
	})
	return nil, ErrInvalidToken
}

// ── Logout ────────────────────────────────────────────────────────────

// LogoutSession mengakhiri hanya sesi yang sedang dipakai (sid dari konteks; bila token tidak membawa sid,
// family dari refreshToken milik pengguna yang sama) dan memasukkan jti ke daftar cabut. Tanpa keduanya sesi
// tidak dapat ditentukan, sehingga semua sesi dicabut seperti perilaku sebelum ada klaim sid.
func (uc *UseCase) LogoutSession(ctx context.Context, userID, jti, refreshToken string) error {
	family := currentSession(ctx)
	if family == "" && refreshToken != "" {
		if stored, err := uc.repo.GetRefreshTokenByHash(ctx, sha256Hex(refreshToken)); err == nil && stored.UserID == userID {
			family = stored.FamilyID
		}
	}

	if family == "" {
		if err := uc.repo.RevokeAllUserRefreshTokens(ctx, userID); err != nil {
			return fmt.Errorf("revoke all tokens: %w", err)
		}
	} else if _, err := uc.repo.RevokeSession(ctx, userID, family, time.Now()); err != nil {
		return fmt.Errorf("revoke session: %w", err)
	}

	uc.blacklist(ctx, userID, jti)

	entry := audit.Entry{AktorID: userID, Aksi: "LOGOUT", Entitas: "pengguna", EntitasID: userID, Hasil: audit.HasilBerhasil}
	if family != "" {
		entry.Metadata = map[string]any{"sesi": family}
	}
	uc.writeAudit(ctx, entry)
	uc.log.Info("logout success", map[string]interface{}{"user_id": userID})
	return nil
}

// LogoutAll mengakhiri semua sesi pengguna, termasuk yang sedang dipakai ("keluar dari semua perangkat").
func (uc *UseCase) LogoutAll(ctx context.Context, userID, jti string) (int, error) {
	families, err := uc.repo.RevokeSessions(ctx, userID, "", time.Now())
	if err != nil {
		return 0, fmt.Errorf("revoke all sessions: %w", err)
	}
	uc.blacklist(ctx, userID, jti)
	uc.writeAudit(ctx, audit.Entry{
		AktorID: userID, Aksi: "LOGOUT_ALL", Entitas: "pengguna", EntitasID: userID,
		Hasil: audit.HasilBerhasil, Metadata: map[string]any{"jumlah_sesi": len(families)},
	})
	uc.log.Info("logout all success", map[string]interface{}{"user_id": userID, "sessions": len(families)})
	return len(families), nil
}

// blacklist memasukkan jti access token yang sedang dipakai ke daftar cabut (token tanpa sid hanya mati
// lewat jti). Gagal tidak menggagalkan logout karena sesinya sudah dicabut.
func (uc *UseCase) blacklist(ctx context.Context, userID, jti string) {
	if jti == "" {
		return
	}
	if err := uc.repo.BlacklistJWT(ctx, jti, uc.cfg.AccessTokenTTL); err != nil {
		uc.log.Warn("logout: failed to blacklist JWT", map[string]interface{}{"jti": jti, "user_id": userID})
	}
}

// ── Daftar dan pencabutan sesi ────────────────────────────────────────

// SessionInfo adalah satu sesi aktif milik pengguna.
type SessionInfo struct {
	ID         string
	UserAgent  string
	IP         string
	StartedAt  time.Time
	LastUsedAt time.Time
	ExpiresAt  time.Time
	// Current true untuk sesi tempat permintaan ini berasal.
	Current bool
}

// ListSessions mengembalikan sesi aktif pengguna, yang terakhir dipakai paling awal.
func (uc *UseCase) ListSessions(ctx context.Context, userID string) ([]SessionInfo, error) {
	rows, err := uc.repo.ListActiveRefreshTokens(ctx, userID, time.Now())
	if err != nil {
		return nil, fmt.Errorf("list sessions: %w", err)
	}
	sid := currentSession(ctx)
	seen := make(map[string]bool, len(rows))
	out := make([]SessionInfo, 0, len(rows))
	// Baris sudah terurut dari yang terbaru; satu sesi diwakili token terbarunya.
	for _, r := range rows {
		if seen[r.FamilyID] {
			continue
		}
		seen[r.FamilyID] = true
		info := SessionInfo{
			ID:         r.FamilyID,
			StartedAt:  r.SessionStartedAt,
			LastUsedAt: r.LastUsedAt,
			ExpiresAt:  r.ExpiresAt,
			Current:    sid != "" && r.FamilyID == sid,
		}
		if r.UserAgent != nil {
			info.UserAgent = *r.UserAgent
		}
		if r.IP != nil {
			info.IP = *r.IP
		}
		out = append(out, info)
	}
	return out, nil
}

// RevokeSession mencabut satu sesi milik pengguna selain yang sedang dipakai. Sesi milik pengguna lain,
// tidak ada, atau sudah berakhir dijawab sama (ErrSessionNotFound) agar id sesi orang lain tidak bisa diselidiki.
func (uc *UseCase) RevokeSession(ctx context.Context, userID, sessionID string) error {
	if _, err := uuid.Parse(sessionID); err != nil {
		return ErrSessionNotFound
	}
	if sid := currentSession(ctx); sid != "" && sid == sessionID {
		return ErrCannotRevokeCurrent
	}
	revoked, err := uc.repo.RevokeSession(ctx, userID, sessionID, time.Now())
	if err != nil {
		return fmt.Errorf("revoke session: %w", err)
	}
	if !revoked {
		return ErrSessionNotFound
	}
	uc.writeAudit(ctx, audit.Entry{
		AktorID: userID, Aksi: "REVOKE_SESSION", Entitas: "sesi", EntitasID: sessionID,
		Hasil: audit.HasilBerhasil,
	})
	return nil
}

// RevokeOtherSessions mencabut semua sesi pengguna selain yang sedang dipakai dan mengembalikan jumlahnya.
func (uc *UseCase) RevokeOtherSessions(ctx context.Context, userID string) (int, error) {
	sid := currentSession(ctx)
	if sid == "" {
		return 0, ErrCurrentSessionUnknown
	}
	families, err := uc.repo.RevokeSessions(ctx, userID, sid, time.Now())
	if err != nil {
		return 0, fmt.Errorf("revoke other sessions: %w", err)
	}
	uc.writeAudit(ctx, audit.Entry{
		AktorID: userID, Aksi: "REVOKE_OTHER_SESSIONS", Entitas: "pengguna", EntitasID: userID,
		Hasil: audit.HasilBerhasil, Metadata: map[string]any{"jumlah_sesi": len(families)},
	})
	return len(families), nil
}

// ForceLogoutUser mengakhiri semua sesi pengguna lain (aksi admin, izin pengguna.kelola) dan mengembalikan
// jumlah sesi yang dicabut. Access token mereka mati seketika lewat sid.
func (uc *UseCase) ForceLogoutUser(ctx context.Context, targetID, actorID string) (int, error) {
	if _, err := uc.repo.GetUserByID(ctx, targetID); err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return 0, ErrUserNotFound
		}
		return 0, fmt.Errorf("get user: %w", err)
	}
	families, err := uc.repo.RevokeSessions(ctx, targetID, "", time.Now())
	if err != nil {
		return 0, fmt.Errorf("force logout: %w", err)
	}
	uc.writeAudit(ctx, audit.Entry{
		AktorID: actorID, Aksi: "FORCE_LOGOUT", Entitas: "pengguna", EntitasID: targetID,
		Hasil: audit.HasilBerhasil, Metadata: map[string]any{"jumlah_sesi": len(families)},
	})
	uc.log.Info("force logout", map[string]interface{}{"user_id": targetID, "actor_id": actorID, "sessions": len(families)})
	return len(families), nil
}
