package usecase

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"

	"github.com/garda-inflasi/api/internal/audit"
	"github.com/garda-inflasi/api/internal/audit/audittest"
	auditdomain "github.com/garda-inflasi/api/internal/audit/domain"
	"github.com/garda-inflasi/api/internal/auth/domain"
)

const (
	sidA = "3f1c8a52-0a57-4b9a-8f0e-0d1f6f3c9a11"
	sidB = "8e2d9b63-1b68-4cab-9f1f-1e2a7a4dab22"
)

func liveToken(id int64, family string) *domain.RefreshToken {
	return &domain.RefreshToken{
		ID: id, UserID: "user-123", TokenHash: "h", FamilyID: family,
		ExpiresAt:        time.Now().Add(24 * time.Hour),
		SessionStartedAt: time.Now().Add(-48 * time.Hour),
	}
}

func rotatedToken(id int64, family string, sejak time.Duration) *domain.RefreshToken {
	t := liveToken(id, family)
	at := time.Now().Add(-sejak)
	t.RevokedAt, t.RotatedAt = &at, &at
	return t
}

func auditAksi(rec *audittest.Recorder) []string {
	var out []string
	for _, e := range rec.Entries() {
		out = append(out, e.Aksi)
	}
	return out
}

// ── Rotasi ────────────────────────────────────────────────────────────

func TestRefreshSession_RotatesWithinSameFamilyAndCarriesSession(t *testing.T) {
	repo := new(mockRepository)
	uc, _ := newTestUseCaseWithAudit(repo)
	stored := liveToken(7, sidA)
	repo.On("GetRefreshTokenByHash", mock.Anything, mock.Anything).Return(stored, nil)
	repo.On("GetUserByID", mock.Anything, "user-123").Return(activeUser(), nil)

	var next *domain.RefreshToken
	repo.On("RotateRefreshToken", mock.Anything, int64(7), mock.Anything, mock.Anything).
		Run(func(args mock.Arguments) { next = args.Get(2).(*domain.RefreshToken) }).
		Return(true, nil)

	ctx := auditdomain.WithOrigin(context.Background(), auditdomain.Origin{UserAgent: "Mozilla/5.0 Chrome/120"})
	res, err := uc.RefreshSession(ctx, "raw-token", "10.0.0.9")

	require.NoError(t, err)
	assert.False(t, res.Replayed)
	assert.NotEmpty(t, res.RefreshToken)
	claims, err := uc.token.VerifyToken(res.AccessToken)
	require.NoError(t, err)
	assert.Equal(t, sidA, claims.SessionID, "access token terikat ke sesi")

	require.NotNil(t, next)
	assert.Equal(t, sidA, next.FamilyID)
	assert.Equal(t, sha256Hex(res.RefreshToken), next.TokenHash)
	assert.Equal(t, stored.SessionStartedAt, next.SessionStartedAt, "awal sesi diwariskan")
	assert.WithinDuration(t, time.Now(), next.LastUsedAt, 5*time.Second)
	require.NotNil(t, next.UserAgent)
	assert.Equal(t, "Mozilla/5.0 Chrome/120", *next.UserAgent)
	require.NotNil(t, next.IP)
	assert.Equal(t, "10.0.0.9", *next.IP)
	repo.AssertExpectations(t)
}

func TestRefreshSession_LostRace_WithinGraceReturnsAccessTokenOnly(t *testing.T) {
	repo := new(mockRepository)
	uc, rec := newTestUseCaseWithAudit(repo)
	// Bacaan pertama: masih hidup. Setelah kalah berebut: sudah dirotasi 1 detik lalu.
	repo.On("GetRefreshTokenByHash", mock.Anything, mock.Anything).Return(liveToken(7, sidA), nil).Once()
	repo.On("GetRefreshTokenByHash", mock.Anything, mock.Anything).Return(rotatedToken(7, sidA, time.Second), nil).Once()
	repo.On("GetUserByID", mock.Anything, "user-123").Return(activeUser(), nil)
	repo.On("RotateRefreshToken", mock.Anything, int64(7), mock.Anything, mock.Anything).Return(false, nil)
	repo.On("IsSessionActive", mock.Anything, sidA).Return(true, nil)

	res, err := uc.RefreshSession(context.Background(), "raw-token", "10.0.0.9")

	require.NoError(t, err)
	assert.True(t, res.Replayed)
	assert.Empty(t, res.RefreshToken, "pemenang rotasi memegang satu-satunya refresh token pengganti")
	claims, err := uc.token.VerifyToken(res.AccessToken)
	require.NoError(t, err)
	assert.Equal(t, sidA, claims.SessionID)
	repo.AssertNotCalled(t, "RevokeRefreshTokenFamily", mock.Anything, mock.Anything)
	assert.Empty(t, auditAksi(rec), "tab serentak bukan insiden keamanan")
}

func TestRefreshSession_RotatedTokenWithinGraceButSessionRevoked(t *testing.T) {
	repo := new(mockRepository)
	uc, _ := newTestUseCaseWithAudit(repo)
	repo.On("GetRefreshTokenByHash", mock.Anything, mock.Anything).Return(rotatedToken(7, sidA, time.Second), nil)
	repo.On("IsSessionActive", mock.Anything, sidA).Return(false, nil)

	_, err := uc.RefreshSession(context.Background(), "raw-token", "10.0.0.9")

	assert.ErrorIs(t, err, ErrInvalidToken, "logout di dalam jendela toleransi tidak boleh dihidupkan lagi")
	repo.AssertNotCalled(t, "GetUserByID", mock.Anything, mock.Anything)
}

func TestRefreshSession_ReuseOutsideGraceRevokesFamilyAndAudits(t *testing.T) {
	repo := new(mockRepository)
	uc, rec := newTestUseCaseWithAudit(repo)
	repo.On("GetRefreshTokenByHash", mock.Anything, mock.Anything).Return(rotatedToken(7, sidA, time.Minute), nil)
	repo.On("RevokeRefreshTokenFamily", mock.Anything, sidA).Return(nil)

	_, err := uc.RefreshSession(context.Background(), "raw-token", "10.0.0.9")

	assert.ErrorIs(t, err, ErrInvalidToken)
	repo.AssertCalled(t, "RevokeRefreshTokenFamily", mock.Anything, sidA)
	entry, ok := rec.Last()
	require.True(t, ok)
	assert.Equal(t, "TOKEN_REUSE", entry.Aksi)
	assert.Equal(t, audit.HasilDitolak, entry.Hasil)
	assert.Equal(t, "user-123", entry.AktorID)
	assert.Equal(t, sidA, entry.Metadata["family_id"])
}

func TestRefreshSession_GraceWindowZeroTreatsReplayAsReuse(t *testing.T) {
	repo := new(mockRepository)
	uc, _ := newTestUseCaseWithAudit(repo)
	uc.cfg.RefreshGraceWindow = 0
	repo.On("GetRefreshTokenByHash", mock.Anything, mock.Anything).Return(rotatedToken(7, sidA, 100*time.Millisecond), nil)
	repo.On("RevokeRefreshTokenFamily", mock.Anything, sidA).Return(nil)

	_, err := uc.RefreshSession(context.Background(), "raw-token", "")

	assert.ErrorIs(t, err, ErrInvalidToken)
	repo.AssertCalled(t, "RevokeRefreshTokenFamily", mock.Anything, sidA)
}

func TestRefreshSession_ExplicitlyRevokedIsPlainInvalidNotReuse(t *testing.T) {
	repo := new(mockRepository)
	uc, rec := newTestUseCaseWithAudit(repo)
	stored := liveToken(7, sidA)
	at := time.Now().Add(-time.Minute)
	stored.RevokedAt = &at // dicabut (logout/"Cabut"), bukan dirotasi
	repo.On("GetRefreshTokenByHash", mock.Anything, mock.Anything).Return(stored, nil)

	_, err := uc.RefreshSession(context.Background(), "raw-token", "")

	assert.ErrorIs(t, err, ErrInvalidToken)
	repo.AssertNotCalled(t, "RevokeRefreshTokenFamily", mock.Anything, mock.Anything)
	assert.Empty(t, auditAksi(rec), "sesi yang sudah dicabut bukan pencurian token")
}

func TestRefreshSession_ExpiredTokenIsNotRotated(t *testing.T) {
	repo := new(mockRepository)
	uc, _ := newTestUseCaseWithAudit(repo)
	stored := liveToken(7, sidA)
	stored.ExpiresAt = time.Now().Add(-time.Minute)
	repo.On("GetRefreshTokenByHash", mock.Anything, mock.Anything).Return(stored, nil)

	_, err := uc.RefreshSession(context.Background(), "raw-token", "")

	assert.ErrorIs(t, err, ErrTokenExpired)
	repo.AssertNotCalled(t, "RotateRefreshToken", mock.Anything, mock.Anything, mock.Anything, mock.Anything)
}

func TestRefreshSession_InactiveUserRevokesFamilyAndIsRejected(t *testing.T) {
	repo := new(mockRepository)
	uc, _ := newTestUseCaseWithAudit(repo)
	user := activeUser()
	user.Status = "NONAKTIF"
	repo.On("GetRefreshTokenByHash", mock.Anything, mock.Anything).Return(liveToken(7, sidA), nil)
	repo.On("GetUserByID", mock.Anything, "user-123").Return(user, nil)
	repo.On("RevokeRefreshTokenFamily", mock.Anything, sidA).Return(nil)

	_, err := uc.RefreshSession(context.Background(), "raw-token", "")

	assert.ErrorIs(t, err, ErrInvalidToken)
	repo.AssertNotCalled(t, "RotateRefreshToken", mock.Anything, mock.Anything, mock.Anything, mock.Anything)
}

func TestRefreshSession_DatabaseErrorIsNotInvalidToken(t *testing.T) {
	repo := new(mockRepository)
	uc, _ := newTestUseCaseWithAudit(repo)
	repo.On("GetRefreshTokenByHash", mock.Anything, mock.Anything).Return(nil, errors.New("connection refused"))

	_, err := uc.RefreshSession(context.Background(), "raw-token", "")

	require.Error(t, err)
	assert.NotErrorIs(t, err, ErrInvalidToken, "galat basis data tidak boleh mengeluarkan pengguna")
	assert.NotErrorIs(t, err, ErrTokenExpired)
}

func TestRefreshSession_RotationErrorPropagates(t *testing.T) {
	repo := new(mockRepository)
	uc, _ := newTestUseCaseWithAudit(repo)
	repo.On("GetRefreshTokenByHash", mock.Anything, mock.Anything).Return(liveToken(7, sidA), nil)
	repo.On("GetUserByID", mock.Anything, "user-123").Return(activeUser(), nil)
	repo.On("RotateRefreshToken", mock.Anything, int64(7), mock.Anything, mock.Anything).Return(false, errors.New("deadlock"))

	_, err := uc.RefreshSession(context.Background(), "raw-token", "")

	require.Error(t, err)
	assert.NotErrorIs(t, err, ErrInvalidToken)
}

// ── Logout ────────────────────────────────────────────────────────────

func TestLogoutSession_RevokesOnlyCurrentSessionAndBlacklistsJTI(t *testing.T) {
	repo := new(mockRepository)
	uc, rec := newTestUseCaseWithAudit(repo)
	repo.On("RevokeSession", mock.Anything, "user-123", sidA, mock.Anything).Return(true, nil)
	repo.On("BlacklistJWT", mock.Anything, "jti-1", mock.Anything).Return(nil)

	err := uc.LogoutSession(WithSession(context.Background(), sidA), "user-123", "jti-1", "")

	require.NoError(t, err)
	repo.AssertNotCalled(t, "RevokeAllUserRefreshTokens", mock.Anything, mock.Anything)
	repo.AssertExpectations(t)
	entry, _ := rec.Last()
	assert.Equal(t, "LOGOUT", entry.Aksi)
	assert.Equal(t, sidA, entry.Metadata["sesi"])
}

func TestLogoutSession_FallsBackToRefreshTokenFamilyForTokenWithoutSid(t *testing.T) {
	repo := new(mockRepository)
	uc, _ := newTestUseCaseWithAudit(repo)
	repo.On("GetRefreshTokenByHash", mock.Anything, sha256Hex("cookie-token")).Return(liveToken(7, sidB), nil)
	repo.On("RevokeSession", mock.Anything, "user-123", sidB, mock.Anything).Return(true, nil)
	repo.On("BlacklistJWT", mock.Anything, "jti-1", mock.Anything).Return(nil)

	require.NoError(t, uc.LogoutSession(context.Background(), "user-123", "jti-1", "cookie-token"))
	repo.AssertNotCalled(t, "RevokeAllUserRefreshTokens", mock.Anything, mock.Anything)
}

func TestLogoutSession_RefreshTokenOfAnotherUserIsIgnored(t *testing.T) {
	repo := new(mockRepository)
	uc, _ := newTestUseCaseWithAudit(repo)
	foreign := liveToken(7, sidB)
	foreign.UserID = "user-lain"
	repo.On("GetRefreshTokenByHash", mock.Anything, mock.Anything).Return(foreign, nil)
	repo.On("RevokeAllUserRefreshTokens", mock.Anything, "user-123").Return(nil)

	require.NoError(t, uc.LogoutSession(context.Background(), "user-123", "", "foreign"))
	repo.AssertNotCalled(t, "RevokeSession", mock.Anything, mock.Anything, mock.Anything, mock.Anything)
}

func TestLogoutSession_RevokeErrorFailsAndDoesNotPretend(t *testing.T) {
	repo := new(mockRepository)
	uc, rec := newTestUseCaseWithAudit(repo)
	repo.On("RevokeSession", mock.Anything, "user-123", sidA, mock.Anything).Return(false, errors.New("db down"))

	err := uc.LogoutSession(WithSession(context.Background(), sidA), "user-123", "jti-1", "")

	require.Error(t, err)
	repo.AssertNotCalled(t, "BlacklistJWT", mock.Anything, mock.Anything, mock.Anything)
	assert.Empty(t, auditAksi(rec))
}

func TestLogoutAll_RevokesEverySessionAndBlacklists(t *testing.T) {
	repo := new(mockRepository)
	uc, rec := newTestUseCaseWithAudit(repo)
	repo.On("RevokeSessions", mock.Anything, "user-123", "", mock.Anything).Return([]string{sidA, sidB}, nil)
	repo.On("BlacklistJWT", mock.Anything, "jti-1", mock.Anything).Return(nil)

	n, err := uc.LogoutAll(WithSession(context.Background(), sidA), "user-123", "jti-1")

	require.NoError(t, err)
	assert.Equal(t, 2, n)
	entry, _ := rec.Last()
	assert.Equal(t, "LOGOUT_ALL", entry.Aksi)
	assert.Equal(t, 2, entry.Metadata["jumlah_sesi"])
}

// ── Daftar dan pencabutan ─────────────────────────────────────────────

func TestListSessions_OneEntryPerFamilyAndMarksCurrent(t *testing.T) {
	repo := new(mockRepository)
	uc, _ := newTestUseCaseWithAudit(repo)
	ua, ip := "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) Chrome/120.0", "10.0.0.5"
	terbaru := liveToken(9, sidA)
	terbaru.UserAgent, terbaru.IP = &ua, &ip
	terbaru.LastUsedAt = time.Now()
	lamaDiFamilySama := liveToken(8, sidA) // sibling hidup sesudah ganti kata sandi
	lain := liveToken(5, sidB)
	repo.On("ListActiveRefreshTokens", mock.Anything, "user-123", mock.Anything).
		Return([]domain.RefreshToken{*terbaru, *lamaDiFamilySama, *lain}, nil)

	items, err := uc.ListSessions(WithSession(context.Background(), sidA), "user-123")

	require.NoError(t, err)
	require.Len(t, items, 2)
	assert.Equal(t, sidA, items[0].ID)
	assert.True(t, items[0].Current)
	assert.Equal(t, ua, items[0].UserAgent)
	assert.Equal(t, ip, items[0].IP)
	assert.Equal(t, sidB, items[1].ID)
	assert.False(t, items[1].Current)
}

func TestListSessions_TokenWithoutSidMarksNothingCurrent(t *testing.T) {
	repo := new(mockRepository)
	uc, _ := newTestUseCaseWithAudit(repo)
	repo.On("ListActiveRefreshTokens", mock.Anything, "user-123", mock.Anything).Return([]domain.RefreshToken{*liveToken(5, sidB)}, nil)

	items, err := uc.ListSessions(context.Background(), "user-123")

	require.NoError(t, err)
	require.Len(t, items, 1)
	assert.False(t, items[0].Current)
}

func TestRevokeSession(t *testing.T) {
	ctx := WithSession(context.Background(), sidA)

	t.Run("sukses dan diaudit", func(t *testing.T) {
		repo := new(mockRepository)
		uc, rec := newTestUseCaseWithAudit(repo)
		repo.On("RevokeSession", mock.Anything, "user-123", sidB, mock.Anything).Return(true, nil)

		require.NoError(t, uc.RevokeSession(ctx, "user-123", sidB))

		entry, _ := rec.Last()
		assert.Equal(t, "REVOKE_SESSION", entry.Aksi)
		assert.Equal(t, sidB, entry.EntitasID)
		assert.Equal(t, "user-123", entry.AktorID)
	})
	t.Run("sesi yang sedang dipakai ditolak", func(t *testing.T) {
		repo := new(mockRepository)
		uc, _ := newTestUseCaseWithAudit(repo)
		assert.ErrorIs(t, uc.RevokeSession(ctx, "user-123", sidA), ErrCannotRevokeCurrent)
		repo.AssertNotCalled(t, "RevokeSession", mock.Anything, mock.Anything, mock.Anything, mock.Anything)
	})
	t.Run("sesi milik orang lain atau tidak ada", func(t *testing.T) {
		repo := new(mockRepository)
		uc, rec := newTestUseCaseWithAudit(repo)
		repo.On("RevokeSession", mock.Anything, "user-123", sidB, mock.Anything).Return(false, nil)
		assert.ErrorIs(t, uc.RevokeSession(ctx, "user-123", sidB), ErrSessionNotFound)
		assert.Empty(t, auditAksi(rec))
	})
	t.Run("id bukan UUID tidak menyentuh basis data", func(t *testing.T) {
		repo := new(mockRepository)
		uc, _ := newTestUseCaseWithAudit(repo)
		assert.ErrorIs(t, uc.RevokeSession(ctx, "user-123", "bukan-uuid"), ErrSessionNotFound)
		repo.AssertNotCalled(t, "RevokeSession", mock.Anything, mock.Anything, mock.Anything, mock.Anything)
	})
}

func TestRevokeOtherSessions(t *testing.T) {
	t.Run("mencabut semua selain sesi ini", func(t *testing.T) {
		repo := new(mockRepository)
		uc, rec := newTestUseCaseWithAudit(repo)
		repo.On("RevokeSessions", mock.Anything, "user-123", sidA, mock.Anything).Return([]string{sidB}, nil)

		n, err := uc.RevokeOtherSessions(WithSession(context.Background(), sidA), "user-123")

		require.NoError(t, err)
		assert.Equal(t, 1, n)
		entry, _ := rec.Last()
		assert.Equal(t, "REVOKE_OTHER_SESSIONS", entry.Aksi)
	})
	t.Run("tanpa sid tidak bisa menentukan sesi lain", func(t *testing.T) {
		repo := new(mockRepository)
		uc, _ := newTestUseCaseWithAudit(repo)
		_, err := uc.RevokeOtherSessions(context.Background(), "user-123")
		assert.ErrorIs(t, err, ErrCurrentSessionUnknown)
		repo.AssertNotCalled(t, "RevokeSessions", mock.Anything, mock.Anything, mock.Anything, mock.Anything)
	})
}

func TestForceLogoutUser(t *testing.T) {
	t.Run("mencabut semua sesi target dan mengaudit aktor admin", func(t *testing.T) {
		repo := new(mockRepository)
		uc, rec := newTestUseCaseWithAudit(repo)
		repo.On("GetUserByID", mock.Anything, "target-1").Return(activeUser(), nil)
		repo.On("RevokeSessions", mock.Anything, "target-1", "", mock.Anything).Return([]string{sidA, sidB}, nil)

		n, err := uc.ForceLogoutUser(context.Background(), "target-1", "admin-1")

		require.NoError(t, err)
		assert.Equal(t, 2, n)
		entry, _ := rec.Last()
		assert.Equal(t, "FORCE_LOGOUT", entry.Aksi)
		assert.Equal(t, "admin-1", entry.AktorID)
		assert.Equal(t, "target-1", entry.EntitasID)
		assert.Equal(t, 2, entry.Metadata["jumlah_sesi"])
	})
	t.Run("pengguna tidak ada", func(t *testing.T) {
		repo := new(mockRepository)
		uc, rec := newTestUseCaseWithAudit(repo)
		repo.On("GetUserByID", mock.Anything, "hilang").Return(nil, gorm.ErrRecordNotFound)
		_, err := uc.ForceLogoutUser(context.Background(), "hilang", "admin-1")
		assert.ErrorIs(t, err, ErrUserNotFound)
		assert.Empty(t, auditAksi(rec))
	})
}

// ── Ganti kata sandi / konsistensi pencabutan ─────────────────────────

func TestChangePassword_KeepsCurrentSessionAndRevokesOthers(t *testing.T) {
	repo := new(mockRepository)
	uc := newTestUseCase(repo)
	user := activeUser()
	started := time.Now().Add(-72 * time.Hour).Truncate(time.Second)
	cur := liveToken(5, sidA)
	cur.SessionStartedAt = started
	repo.On("GetUserByID", mock.Anything, "user-123").Return(user, nil)
	repo.On("SetPassword", mock.Anything, mock.Anything, mock.Anything).Return(nil)
	repo.On("ListActiveRefreshTokens", mock.Anything, "user-123", mock.Anything).Return([]domain.RefreshToken{*cur, *liveToken(6, sidB)}, nil)
	repo.On("RevokeSessions", mock.Anything, "user-123", sidA, mock.Anything).Return([]string{sidB}, nil)
	var row *domain.RefreshToken
	repo.On("CreateRefreshToken", mock.Anything, mock.Anything).
		Run(func(args mock.Arguments) { row = args.Get(1).(*domain.RefreshToken) }).Return(nil)
	repo.On("RetireFamilyTokensExcept", mock.Anything, sidA, mock.Anything, mock.Anything).Return(nil)

	access, refresh, err := uc.ChangePassword(WithSession(context.Background(), sidA), "user-123", "Test1234!", "NewTest1@", "10.0.0.1")

	require.NoError(t, err)
	repo.AssertNotCalled(t, "RevokeAllUserRefreshTokens", mock.Anything, mock.Anything)
	claims, err := uc.token.VerifyToken(access)
	require.NoError(t, err)
	assert.Equal(t, sidA, claims.SessionID, "sesi ini tetap sama")
	require.NotNil(t, row)
	assert.Equal(t, sidA, row.FamilyID)
	assert.Equal(t, started, row.SessionStartedAt, "awal sesi dipertahankan")
	assert.Equal(t, sha256Hex(refresh), row.TokenHash)
	repo.AssertCalled(t, "RetireFamilyTokensExcept", mock.Anything, sidA, row.TokenHash, mock.Anything)
}

func TestChangePassword_SessionNoLongerLiveStartsFreshSessionAfterRevokingAll(t *testing.T) {
	repo := new(mockRepository)
	uc := newTestUseCase(repo)
	repo.On("GetUserByID", mock.Anything, "user-123").Return(activeUser(), nil)
	repo.On("SetPassword", mock.Anything, mock.Anything, mock.Anything).Return(nil)
	repo.On("ListActiveRefreshTokens", mock.Anything, "user-123", mock.Anything).Return([]domain.RefreshToken{*liveToken(6, sidB)}, nil)
	repo.On("RevokeAllUserRefreshTokens", mock.Anything, "user-123").Return(nil)
	var row *domain.RefreshToken
	repo.On("CreateRefreshToken", mock.Anything, mock.Anything).
		Run(func(args mock.Arguments) { row = args.Get(1).(*domain.RefreshToken) }).Return(nil)

	_, _, err := uc.ChangePassword(WithSession(context.Background(), sidA), "user-123", "Test1234!", "NewTest1@", "10.0.0.1")

	require.NoError(t, err)
	require.NotNil(t, row)
	assert.NotEqual(t, sidA, row.FamilyID)
	repo.AssertNotCalled(t, "RevokeSessions", mock.Anything, mock.Anything, mock.Anything, mock.Anything)
}

func TestRevocationFailuresAreNotSwallowed(t *testing.T) {
	boom := errors.New("db down")

	t.Run("menonaktifkan akun", func(t *testing.T) {
		repo := new(mockRepository)
		uc := newTestUseCase(repo)
		repo.On("GetUserByID", mock.Anything, "user-123").Return(activeUser(), nil)
		repo.On("PatchUser", mock.Anything, mock.Anything, mock.Anything).Return(nil)
		repo.On("RevokeAllUserRefreshTokens", mock.Anything, "user-123").Return(boom)
		status := "NONAKTIF"

		_, err := uc.UpdateUser(context.Background(), "user-123", UserPatch{Status: &status}, "admin-1")

		assert.ErrorIs(t, err, boom)
	})
	t.Run("reset kata sandi oleh admin", func(t *testing.T) {
		repo := new(mockRepository)
		uc := newTestUseCase(repo)
		repo.On("GetUserByID", mock.Anything, "user-123").Return(activeUser(), nil)
		repo.On("SetPassword", mock.Anything, mock.Anything, mock.Anything).Return(nil)
		repo.On("RevokeAllUserRefreshTokens", mock.Anything, "user-123").Return(boom)

		err := uc.ResetPasswordAdmin(context.Background(), "user-123", "NewTest1@", "admin-1")

		assert.ErrorIs(t, err, boom)
	})
	t.Run("ganti peran", func(t *testing.T) {
		repo := new(mockRepository)
		uc := newTestUseCase(repo)
		repo.On("GetUserByID", mock.Anything, "user-123").Return(activeUser(), nil)
		repo.On("PatchUser", mock.Anything, mock.Anything, mock.Anything).Return(nil)
		repo.On("RevokeAllUserRefreshTokens", mock.Anything, "user-123").Return(boom)

		_, err := uc.AssignRole(context.Background(), "user-123", "ADMIN_SISTEM", "admin-1")

		assert.ErrorIs(t, err, boom)
	})
}

// ── Penerbitan ────────────────────────────────────────────────────────

func TestIssueSession_BindsAccessTokenToFamilyAndRecordsClient(t *testing.T) {
	repo := new(mockRepository)
	uc := newTestUseCase(repo)
	var row *domain.RefreshToken
	repo.On("CreateRefreshToken", mock.Anything, mock.Anything).
		Run(func(args mock.Arguments) { row = args.Get(1).(*domain.RefreshToken) }).Return(nil)

	ctx := auditdomain.WithOrigin(context.Background(), auditdomain.Origin{UserAgent: "UA-uji"})
	access, _, err := uc.issueSession(ctx, activeUser(), sidA, "203.0.113.7")

	require.NoError(t, err)
	claims, err := uc.token.VerifyToken(access)
	require.NoError(t, err)
	assert.Equal(t, sidA, claims.SessionID)
	require.NotNil(t, row)
	assert.Equal(t, sidA, row.FamilyID)
	assert.Equal(t, "UA-uji", *row.UserAgent)
	assert.Equal(t, "203.0.113.7", *row.IP)
	assert.WithinDuration(t, time.Now(), row.LastUsedAt, 5*time.Second)
	assert.Equal(t, row.LastUsedAt, row.SessionStartedAt)
}

func TestCleanUserAgent(t *testing.T) {
	assert.Nil(t, cleanUserAgent(""))
	assert.Nil(t, cleanUserAgent("   "))
	assert.Equal(t, "Mozilla/5.0", *cleanUserAgent(" Mozilla/5.0 "))
	assert.Equal(t, "ab", *cleanUserAgent("a\x00b"), "NUL ditolak PostgreSQL")

	panjang := strings.Repeat("é", maxUserAgentLen+50)
	got := cleanUserAgent(panjang)
	require.NotNil(t, got)
	assert.Equal(t, maxUserAgentLen, len([]rune(*got)), "dipotong per karakter, bukan per byte")
	assert.True(t, strings.HasSuffix(*got, "é"), "tidak memotong di tengah karakter UTF-8")
}
