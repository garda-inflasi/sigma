package usecase

import (
	"context"
	"errors"
	"fmt"
	"testing"
	"time"

	"github.com/garda-inflasi/api/internal/audit"
	"github.com/garda-inflasi/api/internal/audit/audittest"
	"github.com/garda-inflasi/api/internal/auth/domain"
	"github.com/garda-inflasi/api/pkg/crypto"
	"github.com/garda-inflasi/api/pkg/totp"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

func testKeyring() *crypto.Keyring {
	ring, err := crypto.NewKeyring("v1", map[string][]byte{"v1": []byte("abcdefghijklmnopqrstuvwxyz012345")})
	if err != nil {
		panic(err)
	}
	return ring
}

// userMFA adalah pengguna dengan MFA aktif dan rahasia bersegel; mengembalikan rahasia polos untuk membuat kode.
func userMFA(t *testing.T, uc *UseCase) (*domain.Pengguna, string) {
	t.Helper()
	user := activeUser()
	secret := totp.GenerateSecret()
	sealed, err := uc.cfg.MFAKey.Seal(secret)
	require.NoError(t, err)
	user.MfaAktif, user.MfaSecret = true, &sealed
	return user, secret
}

func kodeSekarang(t *testing.T, secret string) string {
	t.Helper()
	code, err := totp.GenerateTOTP(secret, time.Now())
	require.NoError(t, err)
	return code
}

// ── Login dengan MFA ──────────────────────────────────────────────────

func TestLogin_MFAActiveReturnsChallengeWithoutSession(t *testing.T) {
	repo := new(mockRepository)
	uc, recorder := newTestUseCaseWithAudit(repo)
	user, _ := userMFA(t, uc)
	repo.On("GetUserByEmail", mock.Anything, "test@example.com").Return(user, nil)

	result, err := uc.Login(context.Background(), "test@example.com", "Test1234!", "10.0.0.1")

	require.NoError(t, err)
	assert.True(t, result.MFARequired)
	assert.Empty(t, result.AccessToken)
	assert.Empty(t, result.RefreshToken)
	userID, err := uc.token.VerifyMFAChallengeToken(result.MFAToken)
	require.NoError(t, err)
	assert.Equal(t, uid("user-123"), userID)
	repo.AssertNotCalled(t, "CreateRefreshToken", mock.Anything, mock.Anything)
	assert.Empty(t, recorder.Entries(), "login baru tercatat setelah faktor kedua terbukti")
}

func TestLogin_MandatoryRoleWithoutMFAGetsSetupOnlySession(t *testing.T) {
	for _, tc := range []struct {
		name    string
		enforce bool
		peran   string
		wajib   bool
		want    bool
	}{
		{"admin dengan penegakan menyala", true, "ADMIN_SISTEM", false, true},
		{"admin tanpa penegakan (demo lokal)", false, "ADMIN_SISTEM", false, false},
		{"mahasiswa tidak wajib", true, "OPS_MAHASISWA", false, false},
		{"mahasiswa ditandai wajib", true, "OPS_MAHASISWA", true, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			repo := new(mockRepository)
			uc := newTestUseCase(repo)
			uc.cfg.MFAEnforce = tc.enforce
			user := activeUser()
			user.Peran, user.MfaWajib = tc.peran, tc.wajib
			repo.On("GetUserByEmail", mock.Anything, "test@example.com").Return(user, nil)
			repo.On("CreateRefreshToken", mock.Anything, mock.Anything).Return(nil)
			repo.On("CreateLoginAttempt", mock.Anything, mock.Anything).Return(nil)
			repo.On("TouchLastLogin", mock.Anything, uid("user-123"), mock.Anything).Return(nil)

			result, err := uc.Login(context.Background(), "test@example.com", "Test1234!", "10.0.0.1")

			require.NoError(t, err)
			assert.Equal(t, tc.want, result.MFASetupRequired)
			claims, err := uc.token.VerifyToken(result.AccessToken)
			require.NoError(t, err)
			assert.Equal(t, tc.want, claims.MFASetupRequired)
		})
	}
}

func TestMFAWajibPeran(t *testing.T) {
	wajib := map[string]bool{
		"STRATEGIS_FMIPA": true, "STRATEGIS_BI": true, "STRATEGIS_BPS": true, "STRATEGIS_DJPB": true,
		"MANAJER_PROGRAM": true, "MANAJER_DATA": true, "MANAJER_KEBIJAKAN": true, "MANAJER_PEMBIAYAAN": true,
		"MANAJER_RISIKO": true, "MANAJER_MUTU": true, "OPS_KOORDINATOR": true, "ADMIN_SISTEM": true,
		"OPS_AGREGATOR": false, "OPS_MAHASISWA": false,
	}
	assert.Len(t, wajib, len(roleOrder), "semua peran tercakup")
	for peran, want := range wajib {
		assert.Equal(t, want, mfaWajibPeran(peran), peran)
	}
}

// ── Setup dan aktivasi ────────────────────────────────────────────────

func TestSetupMFA_StoresSealedSecretAndReturnsProvisioningData(t *testing.T) {
	repo := new(mockRepository)
	uc, recorder := newTestUseCaseWithAudit(repo)
	user := activeUser()
	repo.On("GetUserByID", mock.Anything, uid("user-123")).Return(user, nil)
	var tersimpan string
	repo.On("SetMFASecret", mock.Anything, uid("user-123"), mock.Anything).
		Run(func(args mock.Arguments) { tersimpan = args.String(2) }).Return(nil)

	setup, err := uc.SetupMFA(context.Background(), uid("user-123"))

	require.NoError(t, err)
	assert.Contains(t, setup.ProvisioningURI, "otpauth://totp/")
	assert.Contains(t, setup.ProvisioningURI, setup.Secret)
	require.NotEmpty(t, tersimpan)
	assert.NotContains(t, tersimpan, setup.Secret, "rahasia tidak tersimpan polos")
	opened, err := uc.cfg.MFAKey.Open(tersimpan)
	require.NoError(t, err)
	assert.Equal(t, setup.Secret, opened)
	assert.False(t, user.MfaAktif, "belum aktif sampai verifikasi")
	entry, _ := recorder.Last()
	assert.Equal(t, "MFA_SETUP", entry.Aksi)
}

func TestSetupMFA_Rejections(t *testing.T) {
	t.Run("sudah aktif", func(t *testing.T) {
		repo := new(mockRepository)
		uc := newTestUseCase(repo)
		user, _ := userMFA(t, uc)
		repo.On("GetUserByID", mock.Anything, uid("user-123")).Return(user, nil)

		_, err := uc.SetupMFA(context.Background(), uid("user-123"))

		assert.ErrorIs(t, err, ErrMFAAlreadyActive)
	})
	t.Run("pengguna tidak ada", func(t *testing.T) {
		repo := new(mockRepository)
		uc := newTestUseCase(repo)
		repo.On("GetUserByID", mock.Anything, "x").Return(nil, errors.New("not found"))

		_, err := uc.SetupMFA(context.Background(), "x")

		assert.ErrorIs(t, err, ErrUserNotFound)
	})
	t.Run("kunci enkripsi tidak dikonfigurasi", func(t *testing.T) {
		repo := new(mockRepository)
		uc := newTestUseCase(repo)
		uc.cfg.MFAKey = nil

		_, err := uc.SetupMFA(context.Background(), uid("user-123"))

		assert.ErrorIs(t, err, ErrMFAUnavailable)
		repo.AssertNotCalled(t, "SetMFASecret", mock.Anything, mock.Anything, mock.Anything)
	})
}

func TestVerifyMFA_ActivatesIssuesBackupCodesAndReplacesSession(t *testing.T) {
	repo := new(mockRepository)
	uc, recorder := newTestUseCaseWithAudit(repo)
	uc.cfg.MFAEnforce = true
	user := activeUser()
	user.Peran = "ADMIN_SISTEM"
	secret := totp.GenerateSecret()
	sealed, err := uc.cfg.MFAKey.Seal(secret)
	require.NoError(t, err)
	user.MfaSecret = &sealed
	repo.On("GetUserByID", mock.Anything, uid("user-123")).Return(user, nil)
	var tersimpan []string
	repo.On("ActivateMFA", mock.Anything, uid("user-123"), mock.Anything, mock.Anything, mock.Anything).
		Run(func(args mock.Arguments) { tersimpan = args.Get(2).([]string) }).Return(nil)
	repo.On("RevokeAllUserRefreshTokens", mock.Anything, uid("user-123")).Return(nil)
	repo.On("CreateRefreshToken", mock.Anything, mock.Anything).Return(nil)

	hasil, err := uc.VerifyMFA(context.Background(), uid("user-123"), kodeSekarang(t, secret), "10.0.0.1")

	require.NoError(t, err)
	require.Len(t, hasil.KodeCadangan, JumlahKodeCadangan)
	require.Len(t, tersimpan, JumlahKodeCadangan)
	for i, kode := range hasil.KodeCadangan {
		assert.Equal(t, HashKodeCadangan(uid("user-123"), kode), tersimpan[i], "yang tersimpan hash, bukan kode")
		assert.NotContains(t, tersimpan[i], kode)
	}
	claims, err := uc.token.VerifyToken(hasil.AccessToken)
	require.NoError(t, err)
	assert.False(t, claims.MFASetupRequired, "sesi baru tidak lagi dibatasi ke setup MFA")
	repo.AssertCalled(t, "RevokeAllUserRefreshTokens", mock.Anything, uid("user-123"))
	entry, _ := recorder.Last()
	assert.Equal(t, "MFA_VERIFY", entry.Aksi)
}

func TestVerifyMFA_Rejections(t *testing.T) {
	t.Run("kode salah dicatat sebagai percobaan gagal", func(t *testing.T) {
		repo := new(mockRepository)
		uc := newTestUseCase(repo)
		user := activeUser()
		sealed, _ := uc.cfg.MFAKey.Seal(totp.GenerateSecret())
		user.MfaSecret = &sealed
		repo.On("GetUserByID", mock.Anything, uid("user-123")).Return(user, nil)
		repo.On("CreateLoginAttempt", mock.Anything, mock.MatchedBy(func(a *domain.LoginAttempt) bool {
			return !a.Success && a.FailureReason != nil && *a.FailureReason == "mfa_setup_invalid"
		})).Return(nil)

		_, err := uc.VerifyMFA(context.Background(), uid("user-123"), "000000", "10.0.0.1")

		assert.ErrorIs(t, err, ErrMFACodeInvalid)
		repo.AssertNotCalled(t, "ActivateMFA", mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything)
	})
	t.Run("setup belum dimulai", func(t *testing.T) {
		repo := new(mockRepository)
		uc := newTestUseCase(repo)
		repo.On("GetUserByID", mock.Anything, uid("user-123")).Return(activeUser(), nil)

		_, err := uc.VerifyMFA(context.Background(), uid("user-123"), "123456", "ip")

		assert.ErrorIs(t, err, ErrMFANotSetup)
	})
	t.Run("sudah aktif", func(t *testing.T) {
		repo := new(mockRepository)
		uc := newTestUseCase(repo)
		user, _ := userMFA(t, uc)
		repo.On("GetUserByID", mock.Anything, uid("user-123")).Return(user, nil)

		_, err := uc.VerifyMFA(context.Background(), uid("user-123"), "123456", "ip")

		assert.ErrorIs(t, err, ErrMFAAlreadyActive)
	})
	t.Run("terkunci karena terlalu banyak percobaan", func(t *testing.T) {
		repo := new(mockRepository)
		uc := newTestUseCase(repo)
		user := activeUser()
		sealed, _ := uc.cfg.MFAKey.Seal(totp.GenerateSecret())
		user.MfaSecret = &sealed
		repo.On("GetUserByID", mock.Anything, uid("user-123")).Return(user, nil)
		repo.kunciTerkunci("test@example.com")

		_, err := uc.VerifyMFA(context.Background(), uid("user-123"), "123456", "ip")

		assert.ErrorIs(t, err, ErrAccountLocked)
	})
}

// ── Validasi saat login ───────────────────────────────────────────────

func siapMFALogin(t *testing.T, repo *mockRepository) (*UseCase, *audittest.Recorder, *domain.Pengguna, string, string) {
	t.Helper()
	uc, recorder := newTestUseCaseWithAudit(repo)
	user, secret := userMFA(t, uc)
	challenge, err := uc.token.CreateMFAChallengeToken(user.ID)
	require.NoError(t, err)
	repo.On("GetUserByID", mock.Anything, uid("user-123")).Return(user, nil)
	repo.On("CreateLoginAttempt", mock.Anything, mock.Anything).Return(nil)
	repo.On("CreateRefreshToken", mock.Anything, mock.Anything).Return(nil)
	repo.On("TouchLastLogin", mock.Anything, uid("user-123"), mock.Anything).Return(nil)
	return uc, recorder, user, secret, challenge
}

func TestValidateMFALogin_TOTPIssuesSessionOnce(t *testing.T) {
	repo := new(mockRepository)
	uc, recorder, user, secret, challenge := siapMFALogin(t, repo)
	repo.On("AdvanceMFAStep", mock.Anything, uid("user-123"), mock.AnythingOfType("int64")).Return(true, nil).Once()

	result, err := uc.ValidateMFALogin(context.Background(), challenge, kodeSekarang(t, secret), "10.0.0.1")

	require.NoError(t, err)
	assert.NotEmpty(t, result.AccessToken)
	assert.NotEmpty(t, result.RefreshToken)
	assert.False(t, result.MFARequired)
	assert.NotZero(t, user.MfaLangkahTerakhir, "langkah terakhir ikut tersimpan agar kode yang sama tidak berlaku lagi")
	entries := recorder.Entries()
	require.Len(t, entries, 1)
	assert.Equal(t, "LOGIN", entries[0].Aksi)
	assert.Equal(t, map[string]any{"mfa": "totp"}, entries[0].Metadata)
}

func TestValidateMFALogin_ReplayedCodeIsRejected(t *testing.T) {
	repo := new(mockRepository)
	uc, _, _, secret, challenge := siapMFALogin(t, repo)
	repo.On("AdvanceMFAStep", mock.Anything, uid("user-123"), mock.Anything).Return(false, nil)

	_, err := uc.ValidateMFALogin(context.Background(), challenge, kodeSekarang(t, secret), "10.0.0.1")

	assert.ErrorIs(t, err, ErrMFACodeInvalid)
	repo.AssertNotCalled(t, "CreateRefreshToken", mock.Anything, mock.Anything)
}

func TestValidateMFALogin_WrongCodeCountsAsFailedAttempt(t *testing.T) {
	repo := new(mockRepository)
	uc, recorder, _, _, challenge := siapMFALogin(t, repo)

	_, err := uc.ValidateMFALogin(context.Background(), challenge, "000000", "10.0.0.1")

	assert.ErrorIs(t, err, ErrMFACodeInvalid)
	repo.AssertCalled(t, "CreateLoginAttempt", mock.Anything, mock.MatchedBy(func(a *domain.LoginAttempt) bool {
		return !a.Success && a.FailureReason != nil && *a.FailureReason == "mfa_invalid"
	}))
	entries := recorder.Entries()
	require.Len(t, entries, 1)
	assert.Equal(t, audit.HasilGagal, entries[0].Hasil)
	assert.Equal(t, "mfa_invalid", entries[0].Metadata["alasan"])
}

func TestValidateMFALogin_BackupCodeWorksOnce(t *testing.T) {
	repo := new(mockRepository)
	uc, recorder, _, _, challenge := siapMFALogin(t, repo)
	hash := HashKodeCadangan(uid("user-123"), "abcde-23456")
	repo.On("UseBackupCode", mock.Anything, uid("user-123"), hash, mock.Anything).Return(true, nil).Once()
	repo.On("UseBackupCode", mock.Anything, uid("user-123"), hash, mock.Anything).Return(false, nil).Once()

	result, err := uc.ValidateMFALogin(context.Background(), challenge, " abcde-23456 ", "10.0.0.1")
	require.NoError(t, err)
	assert.NotEmpty(t, result.AccessToken)
	assert.Equal(t, map[string]any{"mfa": "kode_cadangan"}, recorder.Entries()[0].Metadata)

	_, err = uc.ValidateMFALogin(context.Background(), challenge, "ABCDE23456", "10.0.0.1")
	assert.ErrorIs(t, err, ErrMFACodeInvalid, "kode cadangan yang sudah terpakai ditolak")
}

// Kode cadangan yang terbit sebelum pengguna.id menjadi UUID (000110) di-hash dari "<id lama>:<kode>". Akun hasil
// migrasi punya ID lama; kode lamanya tetap berlaku (sekali pakai), sedangkan akun baru tidak punya jalur kedua.
func TestValidateMFALogin_KodeCadanganSebelumMigrasiUUIDTetapBerlaku(t *testing.T) {
	repo := &mockRepository{idLama: map[string]string{uid("user-123"): "user-123"}}
	uc, recorder, _, _, challenge := siapMFALogin(t, repo)
	hashBaru := HashKodeCadangan(uid("user-123"), "ABCDE23456")
	hashLama := HashKodeCadangan("user-123", "ABCDE23456")
	require.NotEqual(t, hashBaru, hashLama)
	repo.On("UseBackupCode", mock.Anything, uid("user-123"), hashBaru, mock.Anything).Return(false, nil)
	repo.On("UseBackupCode", mock.Anything, uid("user-123"), hashLama, mock.Anything).Return(true, nil).Once()

	result, err := uc.ValidateMFALogin(context.Background(), challenge, "ABCDE23456", "10.0.0.1")

	require.NoError(t, err)
	assert.NotEmpty(t, result.AccessToken)
	assert.Equal(t, map[string]any{"mfa": "kode_cadangan"}, recorder.Entries()[0].Metadata)
}

func TestValidateMFALogin_AkunBaruTanpaIDLamaTidakMencobaJalurKedua(t *testing.T) {
	repo := new(mockRepository) // idLama kosong: akun dibuat sesudah migrasi
	uc, _, _, _, challenge := siapMFALogin(t, repo)
	repo.On("UseBackupCode", mock.Anything, uid("user-123"), HashKodeCadangan(uid("user-123"), "ABCDE23456"), mock.Anything).Return(false, nil).Once()

	_, err := uc.ValidateMFALogin(context.Background(), challenge, "ABCDE23456", "10.0.0.1")

	assert.ErrorIs(t, err, ErrMFACodeInvalid)
	repo.AssertNumberOfCalls(t, "UseBackupCode", 1)
}

func TestValidateMFALogin_TokenTantanganDenganSubSlugLamaDitolakBersih(t *testing.T) {
	repo := &mockRepository{}
	uc := newTestUseCase(repo)
	challenge, err := uc.token.CreateMFAChallengeToken("test-user-001") // token terbit sebelum migrasi 000110
	require.NoError(t, err)
	repo.On("GetUserByID", mock.Anything, "test-user-001").Return(nil, fmt.Errorf("get user by id: %w", gorm.ErrRecordNotFound))

	_, err = uc.ValidateMFALogin(context.Background(), challenge, "123456", "10.0.0.1")

	assert.ErrorIs(t, err, ErrMFAChallengeInvalid, "401 bersih, bukan galat basis data")
}

func TestValidateMFALogin_InvalidChallenge(t *testing.T) {
	t.Run("token sembarang", func(t *testing.T) {
		uc := newTestUseCase(new(mockRepository))

		_, err := uc.ValidateMFALogin(context.Background(), "bukan-token", "123456", "ip")

		assert.ErrorIs(t, err, ErrMFAChallengeInvalid)
	})
	t.Run("access token bukan token tantangan", func(t *testing.T) {
		uc := newTestUseCase(new(mockRepository))
		access, err := uc.token.CreateAccessToken(uid("user-123"), "ADMIN_SISTEM", nil)
		require.NoError(t, err)

		_, err = uc.ValidateMFALogin(context.Background(), access, "123456", "ip")

		assert.ErrorIs(t, err, ErrMFAChallengeInvalid)
	})
	t.Run("akun dinonaktifkan sesudah kata sandi benar", func(t *testing.T) {
		repo := new(mockRepository)
		uc, _, user, _, challenge := siapMFALogin(t, repo)
		user.Status = "NONAKTIF"

		_, err := uc.ValidateMFALogin(context.Background(), challenge, "123456", "ip")

		assert.ErrorIs(t, err, ErrMFAChallengeInvalid)
	})
	t.Run("MFA dimatikan sesudah kata sandi benar", func(t *testing.T) {
		repo := new(mockRepository)
		uc, _, user, _, challenge := siapMFALogin(t, repo)
		user.MfaAktif = false

		_, err := uc.ValidateMFALogin(context.Background(), challenge, "123456", "ip")

		assert.ErrorIs(t, err, ErrMFAChallengeInvalid)
	})
}

func TestValidateMFALogin_LockedAccount(t *testing.T) {
	repo := new(mockRepository)
	uc := newTestUseCase(repo)
	user, _ := userMFA(t, uc)
	challenge, _ := uc.token.CreateMFAChallengeToken(user.ID)
	repo.On("GetUserByID", mock.Anything, uid("user-123")).Return(user, nil)
	repo.kunciTerkunci("test@example.com")

	_, err := uc.ValidateMFALogin(context.Background(), challenge, "123456", "ip")

	assert.ErrorIs(t, err, ErrAccountLocked)
	repo.AssertNotCalled(t, "AdvanceMFAStep", mock.Anything, mock.Anything, mock.Anything)
}

// ── Kode cadangan, nonaktifkan, status ────────────────────────────────

func TestRegenerateBackupCodes(t *testing.T) {
	repo := new(mockRepository)
	uc, recorder := newTestUseCaseWithAudit(repo)
	user, _ := userMFA(t, uc)
	repo.On("GetUserByID", mock.Anything, uid("user-123")).Return(user, nil)
	repo.On("CreateLoginAttempt", mock.Anything, mock.Anything).Return(nil)
	var tersimpan []string
	repo.On("ReplaceBackupCodes", mock.Anything, uid("user-123"), mock.Anything).
		Run(func(args mock.Arguments) { tersimpan = args.Get(2).([]string) }).Return(nil)

	kode, err := uc.RegenerateBackupCodes(context.Background(), uid("user-123"), "Test1234!", "10.0.0.1")

	require.NoError(t, err)
	require.Len(t, kode, JumlahKodeCadangan)
	assert.Equal(t, hashKode(uid("user-123"), kode), tersimpan)
	entry, _ := recorder.Last()
	assert.Equal(t, "MFA_BACKUP_REGENERATE", entry.Aksi)

	_, err = uc.RegenerateBackupCodes(context.Background(), uid("user-123"), "SalahSandi1!", "10.0.0.1")
	assert.ErrorIs(t, err, ErrWrongPassword)
}

func TestRegenerateBackupCodes_RequiresActiveMFA(t *testing.T) {
	repo := new(mockRepository)
	uc := newTestUseCase(repo)
	repo.On("GetUserByID", mock.Anything, uid("user-123")).Return(activeUser(), nil)

	_, err := uc.RegenerateBackupCodes(context.Background(), uid("user-123"), "Test1234!", "10.0.0.1")

	assert.ErrorIs(t, err, ErrMFANotSetup)
}

func TestDisableMFA(t *testing.T) {
	t.Run("berhasil dan tercatat", func(t *testing.T) {
		repo := new(mockRepository)
		uc, recorder := newTestUseCaseWithAudit(repo)
		user, _ := userMFA(t, uc)
		repo.On("GetUserByID", mock.Anything, uid("user-123")).Return(user, nil)
		repo.On("ClearMFA", mock.Anything, uid("user-123")).Return(nil)

		require.NoError(t, uc.DisableMFA(context.Background(), uid("user-123"), "Test1234!", "10.0.0.1"))

		entry, _ := recorder.Last()
		assert.Equal(t, "MFA_DISABLE", entry.Aksi)
	})
	t.Run("kata sandi salah", func(t *testing.T) {
		repo := new(mockRepository)
		uc := newTestUseCase(repo)
		user, _ := userMFA(t, uc)
		repo.On("GetUserByID", mock.Anything, uid("user-123")).Return(user, nil)
		repo.On("CreateLoginAttempt", mock.Anything, mock.Anything).Return(nil)

		assert.ErrorIs(t, uc.DisableMFA(context.Background(), uid("user-123"), "SalahSandi1!", "10.0.0.1"), ErrWrongPassword)
		repo.AssertNotCalled(t, "ClearMFA", mock.Anything, mock.Anything)
	})
	t.Run("ditolak bila MFA diwajibkan", func(t *testing.T) {
		repo := new(mockRepository)
		uc := newTestUseCase(repo)
		uc.cfg.MFAEnforce = true
		user, _ := userMFA(t, uc)
		user.Peran = "ADMIN_SISTEM"
		repo.On("GetUserByID", mock.Anything, uid("user-123")).Return(user, nil)

		assert.ErrorIs(t, uc.DisableMFA(context.Background(), uid("user-123"), "Test1234!", "10.0.0.1"), ErrMFAMandatory)
		repo.AssertNotCalled(t, "ClearMFA", mock.Anything, mock.Anything)
	})
	t.Run("akun tanpa kata sandi tidak bisa dikonfirmasi", func(t *testing.T) {
		repo := new(mockRepository)
		uc := newTestUseCase(repo)
		user, _ := userMFA(t, uc)
		user.PasswordHash = nil
		repo.On("GetUserByID", mock.Anything, uid("user-123")).Return(user, nil)

		assert.ErrorIs(t, uc.DisableMFA(context.Background(), uid("user-123"), "apa saja", "10.0.0.1"), ErrWrongPassword)
	})
	t.Run("pengguna tidak ada", func(t *testing.T) {
		repo := new(mockRepository)
		uc := newTestUseCase(repo)
		repo.On("GetUserByID", mock.Anything, "x").Return(nil, errors.New("not found"))

		assert.ErrorIs(t, uc.DisableMFA(context.Background(), "x", "Test1234!", "10.0.0.1"), ErrUserNotFound)
	})
}

func TestMFAStatus(t *testing.T) {
	repo := new(mockRepository)
	uc := newTestUseCase(repo)
	uc.cfg.MFAEnforce = true

	belum := activeUser()
	belum.Peran = "ADMIN_SISTEM"
	status, err := uc.MFAStatus(context.Background(), belum)
	require.NoError(t, err)
	assert.Equal(t, MFAStatus{SetupDiwajibkan: true, Wajib: true}, status)

	aktif, _ := userMFA(t, uc)
	repo.On("CountBackupCodes", mock.Anything, uid("user-123")).Return(int64(7), nil)
	status, err = uc.MFAStatus(context.Background(), aktif)
	require.NoError(t, err)
	require.NotNil(t, status.KodeCadanganTersisa)
	assert.Equal(t, 7, *status.KodeCadanganTersisa)
	assert.False(t, status.SetupDiwajibkan)
}

func TestHashKodeCadangan_NormalizesAndIsPerUser(t *testing.T) {
	assert.Equal(t, HashKodeCadangan(uid("u1"), "ABCDE23456"), HashKodeCadangan(uid("u1"), " abcde-23456 "))
	assert.NotEqual(t, HashKodeCadangan(uid("u1"), "ABCDE23456"), HashKodeCadangan(uid("u2"), "ABCDE23456"), "garam per pengguna")
	assert.Len(t, HashKodeCadangan(uid("u1"), "x"), 64)
}
