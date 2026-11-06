package usecase

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"

	"github.com/garda-inflasi/api/internal/audit"
	"github.com/garda-inflasi/api/internal/auth/domain"
	"github.com/garda-inflasi/api/pkg/totp"
)

// Kegagalan infrastruktur (basis data putus, kunci enkripsi hilang) tidak boleh berakibat salah arah:
//   - dihitung sebagai tebakan salah (mengunci pemilik sah),
//   - dijawab "token tidak sah" (klien mengeluarkan pengguna), atau
//   - meloloskan sesuatu (gagal-buka).
// Tes di sini memeriksa akibat itu, bukan sekadar bahwa galatnya diteruskan.

var errDB = errors.New("db putus")

// ── MFA: bukan tebakan salah ──────────────────────────────────────────

func TestValidateMFALogin_KegagalanSelainKodeSalahTidakMenghitungSebagaiTebakan(t *testing.T) {
	kasus := map[string]struct {
		siapkan func(uc *UseCase, repo *mockRepository)
		kode    func(secret string) string
	}{
		"kunci enkripsi MFA tidak terkonfigurasi": {
			siapkan: func(uc *UseCase, _ *mockRepository) { uc.cfg.MFAKey = nil },
			kode:    func(string) string { return "123456" },
		},
		"gagal mencatat langkah TOTP": {
			siapkan: func(_ *UseCase, repo *mockRepository) {
				repo.On("AdvanceMFAStep", mock.Anything, "user-123", mock.Anything).Return(false, errDB)
			},
			kode: nil, // kode TOTP sah dihitung dari rahasia pengguna
		},
		"gagal memakai kode cadangan": {
			siapkan: func(_ *UseCase, repo *mockRepository) {
				repo.On("UseBackupCode", mock.Anything, "user-123", mock.Anything, mock.Anything).Return(false, errDB)
			},
			kode: func(string) string { return "ABCD-EFGH-1234" },
		},
	}
	for nama, tc := range kasus {
		t.Run(nama, func(t *testing.T) {
			jamBeku(t, awalUji)
			repo := new(mockRepository)
			uc, recorder, _, secret, challenge := siapMFALogin(t, repo)
			tc.siapkan(uc, repo)
			kode := kodeSekarang(t, secret)
			if tc.kode != nil {
				kode = tc.kode(secret)
			}

			_, err := uc.ValidateMFALogin(context.Background(), challenge, kode, "10.0.0.1")

			require.Error(t, err)
			assert.NotErrorIs(t, err, ErrMFACodeInvalid, "bukan kode salah")
			assert.NotErrorIs(t, err, ErrAccountLocked)
			state := repo.keadaan(emailUji)
			require.NotNil(t, state)
			assert.Zero(t, state.Gagal, "percobaan yang dipesan dikembalikan: pemilik sah tidak terkunci oleh galat server")
			repo.AssertNotCalled(t, "CreateRefreshToken", mock.Anything, mock.Anything)
			for _, e := range recorder.Entries() {
				assert.NotEqual(t, audit.HasilBerhasil, e.Hasil, "tidak ada audit berhasil tanpa sesi")
			}
		})
	}
}

func TestVerifyMFA_GagalMengaktifkanTidakMenghitungDanTidakMenerbitkanSesi(t *testing.T) {
	jamBeku(t, awalUji)
	repo := new(mockRepository)
	uc := newTestUseCase(repo)
	user := activeUser()
	secret := totp.GenerateSecret()
	sealed, err := uc.cfg.MFAKey.Seal(secret)
	require.NoError(t, err)
	user.MfaSecret = &sealed
	repo.On("GetUserByID", mock.Anything, "user-123").Return(user, nil)
	repo.On("ActivateMFA", mock.Anything, "user-123", mock.Anything, mock.Anything).Return(errDB)

	_, err = uc.VerifyMFA(context.Background(), "user-123", kodeSekarang(t, secret), "10.0.0.1")

	require.Error(t, err)
	assert.False(t, user.MfaAktif, "MFA tidak dianggap aktif di salinan pengguna")
	state := repo.keadaan(emailUji)
	require.NotNil(t, state)
	assert.Zero(t, state.Gagal)
	repo.AssertNotCalled(t, "CreateRefreshToken", mock.Anything, mock.Anything)
}

func TestVerifyMFA_RahasiaRusakDilaporkanBukanSebagaiBelumDisiapkan(t *testing.T) {
	repo := new(mockRepository)
	uc := newTestUseCase(repo)
	user := activeUser()
	rusak := "v1:bukan-ciphertext-sah"
	user.MfaSecret = &rusak
	repo.On("GetUserByID", mock.Anything, "user-123").Return(user, nil)

	_, err := uc.VerifyMFA(context.Background(), "user-123", "123456", "10.0.0.1")

	require.Error(t, err)
	assert.NotErrorIs(t, err, ErrMFANotSetup, "menyuruh pengguna mengulang setup akan menimpa rahasia yang masih bisa dipulihkan")
	assert.Contains(t, err.Error(), "rahasia MFA")
}

func TestRegenerateBackupCodes_KataSandiSalahTidakMenyentuhKodeLama(t *testing.T) {
	repo := new(mockRepository)
	uc, recorder := newTestUseCaseWithAudit(repo)
	user, _ := userMFA(t, uc)
	repo.On("GetUserByID", mock.Anything, "user-123").Return(user, nil)
	repo.On("CreateLoginAttempt", mock.Anything, mock.Anything).Return(nil)

	_, err := uc.RegenerateBackupCodes(context.Background(), "user-123", "SalahSandi1!", "10.0.0.1")

	require.ErrorIs(t, err, ErrWrongPassword)
	repo.AssertNotCalled(t, "ReplaceBackupCodes", mock.Anything, mock.Anything, mock.Anything)
	assert.Empty(t, recorder.Entries())
}

func TestRegenerateBackupCodes_GagalMenyimpanTidakMengembalikanKodeYangTidakTersimpan(t *testing.T) {
	repo := new(mockRepository)
	uc, recorder := newTestUseCaseWithAudit(repo)
	user, _ := userMFA(t, uc)
	repo.On("GetUserByID", mock.Anything, "user-123").Return(user, nil)
	repo.On("ReplaceBackupCodes", mock.Anything, "user-123", mock.Anything).Return(errDB)

	kode, err := uc.RegenerateBackupCodes(context.Background(), "user-123", "Test1234!", "10.0.0.1")

	require.Error(t, err)
	assert.Empty(t, kode, "kode polos yang tidak pernah tersimpan tidak boleh sampai ke pengguna")
	assert.Empty(t, recorder.Entries())
}

func TestDisableMFA_GagalMenghapusTidakDiauditBerhasil(t *testing.T) {
	repo := new(mockRepository)
	uc, recorder := newTestUseCaseWithAudit(repo)
	user, _ := userMFA(t, uc)
	repo.On("GetUserByID", mock.Anything, "user-123").Return(user, nil)
	repo.On("ClearMFA", mock.Anything, "user-123").Return(errDB)

	require.Error(t, uc.DisableMFA(context.Background(), "user-123", "Test1234!", "10.0.0.1"))
	assert.Empty(t, recorder.Entries())
}

func TestMFAStatus_GalatHitungKodeCadanganDiteruskan(t *testing.T) {
	repo := new(mockRepository)
	uc := newTestUseCase(repo)
	user, _ := userMFA(t, uc)
	repo.On("CountBackupCodes", mock.Anything, "user-123").Return(int64(0), errDB)

	status, err := uc.MFAStatus(context.Background(), user)

	require.Error(t, err)
	assert.Nil(t, status.KodeCadanganTersisa, "tidak melaporkan angka yang tidak diketahui sebagai nol")
}

// ── Sesi: pergantian token saat kredensial berubah ───────────────────

func siapGantiSandi(repo *mockRepository) {
	repo.On("GetUserByID", mock.Anything, "user-123").Return(activeUser(), nil)
	repo.On("UpdateUser", mock.Anything, mock.Anything).Return(nil)
}

func TestReplaceCurrentSession_KegagalanBerhentiSebelumMerusakSesiYangBerjalan(t *testing.T) {
	ctx := WithSession(context.Background(), sidA)
	ganti := func(uc *UseCase) error {
		_, _, err := uc.ChangePassword(ctx, "user-123", "Test1234!", "NewTest1@", "10.0.0.1")
		return err
	}

	t.Run("daftar sesi gagal dibaca: tidak ada yang dicabut atau diterbitkan", func(t *testing.T) {
		repo := new(mockRepository)
		uc := newTestUseCase(repo)
		siapGantiSandi(repo)
		repo.On("ListActiveRefreshTokens", mock.Anything, "user-123", mock.Anything).Return(nil, errDB)

		require.ErrorIs(t, ganti(uc), errDB)
		repo.AssertNotCalled(t, "RevokeSessions", mock.Anything, mock.Anything, mock.Anything, mock.Anything)
		repo.AssertNotCalled(t, "RevokeAllUserRefreshTokens", mock.Anything, mock.Anything)
		repo.AssertNotCalled(t, "CreateRefreshToken", mock.Anything, mock.Anything)
	})
	t.Run("sesi lain gagal dicabut: tidak menerbitkan token pengganti", func(t *testing.T) {
		repo := new(mockRepository)
		uc := newTestUseCase(repo)
		siapGantiSandi(repo)
		repo.On("ListActiveRefreshTokens", mock.Anything, "user-123", mock.Anything).Return([]domain.RefreshToken{*liveToken(5, sidA)}, nil)
		repo.On("RevokeSessions", mock.Anything, "user-123", sidA, mock.Anything).Return(nil, errDB)

		require.ErrorIs(t, ganti(uc), errDB)
		repo.AssertNotCalled(t, "CreateRefreshToken", mock.Anything, mock.Anything)
	})
	t.Run("token pengganti gagal disimpan: token lama sesi ini tidak dipensiunkan", func(t *testing.T) {
		repo := new(mockRepository)
		uc := newTestUseCase(repo)
		siapGantiSandi(repo)
		repo.On("ListActiveRefreshTokens", mock.Anything, "user-123", mock.Anything).Return([]domain.RefreshToken{*liveToken(5, sidA)}, nil)
		repo.On("RevokeSessions", mock.Anything, "user-123", sidA, mock.Anything).Return([]string{}, nil)
		repo.On("CreateRefreshToken", mock.Anything, mock.Anything).Return(errDB)

		require.ErrorIs(t, ganti(uc), errDB)
		repo.AssertNotCalled(t, "RetireFamilyTokensExcept", mock.Anything, mock.Anything, mock.Anything, mock.Anything)
	})
	t.Run("pensiun token lama gagal: dilaporkan", func(t *testing.T) {
		repo := new(mockRepository)
		uc := newTestUseCase(repo)
		siapGantiSandi(repo)
		repo.On("ListActiveRefreshTokens", mock.Anything, "user-123", mock.Anything).Return([]domain.RefreshToken{*liveToken(5, sidA)}, nil)
		repo.On("RevokeSessions", mock.Anything, "user-123", sidA, mock.Anything).Return([]string{}, nil)
		repo.On("CreateRefreshToken", mock.Anything, mock.Anything).Return(nil)
		repo.On("RetireFamilyTokensExcept", mock.Anything, sidA, mock.Anything, mock.Anything).Return(errDB)

		require.ErrorIs(t, ganti(uc), errDB)
	})
	t.Run("tanpa sesi dikenal: pencabutan semua gagal, sesi baru tidak terbit", func(t *testing.T) {
		repo := new(mockRepository)
		uc := newTestUseCase(repo)
		siapGantiSandi(repo)
		repo.On("RevokeAllUserRefreshTokens", mock.Anything, "user-123").Return(errDB)

		_, _, err := uc.ChangePassword(context.Background(), "user-123", "Test1234!", "NewTest1@", "10.0.0.1")

		require.Error(t, err)
		repo.AssertNotCalled(t, "CreateRefreshToken", mock.Anything, mock.Anything)
	})
}

func TestMintAccess_MembawaProfilTampilanDanWilayahTugas(t *testing.T) {
	repo := new(mockRepository)
	uc := newTestUseCase(repo)
	user := activeUser()
	kab := "1171"
	user.KabupatenKode, user.Lembaga = &kab, "BPS Aceh"

	tk, err := uc.mintAccess(user, sidA)

	require.NoError(t, err)
	claims, err := uc.token.VerifyToken(tk)
	require.NoError(t, err)
	assert.Equal(t, "Test User", claims.Name)
	assert.Equal(t, "test@example.com", claims.Email)
	assert.Equal(t, "BPS Aceh", claims.Org)
	assert.Equal(t, "1171", claims.Kab)
	assert.Equal(t, sidA, claims.SessionID)
	assert.Equal(t, uc.getPermissionsForRole("OPS_MAHASISWA"), claims.Permissions)

	user.KabupatenKode = nil
	tk, err = uc.mintAccess(user, sidA)
	require.NoError(t, err)
	claims, err = uc.token.VerifyToken(tk)
	require.NoError(t, err)
	assert.Empty(t, claims.Kab, "pengguna tingkat provinsi tidak punya kabupaten tugas")
}

// ── Refresh: balapan dan pemakaian ulang ─────────────────────────────

func TestRefreshSession_KalahBerebutLaluTokenAnehTidakMengeluarkanSesi(t *testing.T) {
	t.Run("pembacaan ulang menemukan token masih hidup: kedaluwarsa, bukan sesi baru", func(t *testing.T) {
		repo := new(mockRepository)
		uc := newTestUseCase(repo)
		stored := liveToken(7, sidA)
		repo.On("GetRefreshTokenByHash", mock.Anything, mock.Anything).Return(stored, nil).Twice()
		repo.On("GetUserByID", mock.Anything, "user-123").Return(activeUser(), nil)
		repo.On("RotateRefreshToken", mock.Anything, int64(7), mock.Anything, mock.Anything).Return(false, nil)

		_, err := uc.RefreshSession(context.Background(), "rt", "10.0.0.1")

		assert.ErrorIs(t, err, ErrTokenExpired)
		repo.AssertNotCalled(t, "RevokeRefreshTokenFamily", mock.Anything, mock.Anything)
	})
	t.Run("pembacaan ulang gagal: galat server, bukan token tidak sah", func(t *testing.T) {
		repo := new(mockRepository)
		uc := newTestUseCase(repo)
		repo.On("GetRefreshTokenByHash", mock.Anything, mock.Anything).Return(liveToken(7, sidA), nil).Once()
		repo.On("GetRefreshTokenByHash", mock.Anything, mock.Anything).Return(nil, errDB).Once()
		repo.On("GetUserByID", mock.Anything, "user-123").Return(activeUser(), nil)
		repo.On("RotateRefreshToken", mock.Anything, int64(7), mock.Anything, mock.Anything).Return(false, nil)

		_, err := uc.RefreshSession(context.Background(), "rt", "10.0.0.1")

		require.Error(t, err)
		assert.NotErrorIs(t, err, ErrInvalidToken)
	})
	t.Run("token tidak dikenal", func(t *testing.T) {
		repo := new(mockRepository)
		uc := newTestUseCase(repo)
		repo.On("GetRefreshTokenByHash", mock.Anything, mock.Anything).Return(nil, gorm.ErrRecordNotFound)

		_, err := uc.RefreshSession(context.Background(), "rt", "10.0.0.1")

		assert.ErrorIs(t, err, ErrInvalidToken)
	})
}

func TestReplayOrReuse_JalurTepi(t *testing.T) {
	baruDirotasi := func() *domain.RefreshToken { return rotatedToken(7, sidA, 2*time.Second) }

	t.Run("pemeriksaan sesi gagal: galat server, bukan pencabutan", func(t *testing.T) {
		repo := new(mockRepository)
		uc := newTestUseCase(repo)
		repo.On("GetRefreshTokenByHash", mock.Anything, mock.Anything).Return(baruDirotasi(), nil)
		repo.On("IsSessionActive", mock.Anything, sidA).Return(false, errDB)

		_, err := uc.RefreshSession(context.Background(), "rt", "10.0.0.1")

		require.Error(t, err)
		assert.NotErrorIs(t, err, ErrInvalidToken)
		repo.AssertNotCalled(t, "RevokeRefreshTokenFamily", mock.Anything, mock.Anything)
	})
	t.Run("sesi sudah dicabut: tidak menghidupkan lagi dan tidak membuat access token", func(t *testing.T) {
		repo := new(mockRepository)
		uc := newTestUseCase(repo)
		repo.On("GetRefreshTokenByHash", mock.Anything, mock.Anything).Return(baruDirotasi(), nil)
		repo.On("IsSessionActive", mock.Anything, sidA).Return(false, nil)

		res, err := uc.RefreshSession(context.Background(), "rt", "10.0.0.1")

		assert.ErrorIs(t, err, ErrInvalidToken)
		assert.Nil(t, res)
		repo.AssertNotCalled(t, "GetUserByID", mock.Anything, mock.Anything)
	})
	t.Run("akun dinonaktifkan di sela jendela toleransi: sesi dimatikan, tidak ada access token", func(t *testing.T) {
		repo := new(mockRepository)
		uc := newTestUseCase(repo)
		nonaktif := activeUser()
		nonaktif.Status = "NONAKTIF"
		repo.On("GetRefreshTokenByHash", mock.Anything, mock.Anything).Return(baruDirotasi(), nil)
		repo.On("IsSessionActive", mock.Anything, sidA).Return(true, nil)
		repo.On("GetUserByID", mock.Anything, "user-123").Return(nonaktif, nil)
		repo.On("RevokeRefreshTokenFamily", mock.Anything, sidA).Return(nil)

		res, err := uc.RefreshSession(context.Background(), "rt", "10.0.0.1")

		assert.ErrorIs(t, err, ErrInvalidToken)
		assert.Nil(t, res)
		repo.AssertCalled(t, "RevokeRefreshTokenFamily", mock.Anything, sidA)
	})
	t.Run("pemakaian ulang: family tetap dianggap dicabut dan diaudit walau pencabutan gagal", func(t *testing.T) {
		repo := new(mockRepository)
		uc, recorder := newTestUseCaseWithAudit(repo)
		repo.On("GetRefreshTokenByHash", mock.Anything, mock.Anything).Return(rotatedToken(7, sidA, time.Hour), nil)
		repo.On("RevokeRefreshTokenFamily", mock.Anything, sidA).Return(errDB)

		_, err := uc.RefreshSession(context.Background(), "rt", "10.0.0.1")

		assert.ErrorIs(t, err, ErrInvalidToken, "pemegang token curian tidak mendapat apa pun")
		assert.Equal(t, []string{"TOKEN_REUSE"}, auditAksi(recorder), "kejadian pencurian tercatat walau pencabutan gagal")
	})
	t.Run("toleransi dimatikan (0): putar ulang langsung dihitung pemakaian ulang", func(t *testing.T) {
		repo := new(mockRepository)
		uc, recorder := newTestUseCaseWithAudit(repo)
		uc.cfg.RefreshGraceWindow = 0
		repo.On("GetRefreshTokenByHash", mock.Anything, mock.Anything).Return(baruDirotasi(), nil)
		repo.On("RevokeRefreshTokenFamily", mock.Anything, sidA).Return(nil)

		_, err := uc.RefreshSession(context.Background(), "rt", "10.0.0.1")

		assert.ErrorIs(t, err, ErrInvalidToken)
		assert.Equal(t, []string{"TOKEN_REUSE"}, auditAksi(recorder))
		repo.AssertNotCalled(t, "IsSessionActive", mock.Anything, mock.Anything)
	})
}

// ── Keluar, daftar, dan pencabutan sesi ───────────────────────────────

func TestLogoutSession_KegagalanPencabutanTidakMemasukkanTokenKeDaftarCabut(t *testing.T) {
	repo := new(mockRepository)
	uc, recorder := newTestUseCaseWithAudit(repo)
	repo.On("RevokeSession", mock.Anything, "user-123", sidA, mock.Anything).Return(false, errDB)

	err := uc.LogoutSession(WithSession(context.Background(), sidA), "user-123", "jti-1", "")

	require.Error(t, err)
	repo.AssertNotCalled(t, "BlacklistJWT", mock.Anything, mock.Anything, mock.Anything)
	assert.Empty(t, recorder.Entries(), "keluar yang gagal tidak diaudit sebagai berhasil")
}

func TestLogoutSession_GagalMengisiDaftarCabutTidakMenggagalkanKeluar(t *testing.T) {
	repo := new(mockRepository)
	uc, recorder := newTestUseCaseWithAudit(repo)
	repo.On("RevokeSession", mock.Anything, "user-123", sidA, mock.Anything).Return(true, nil)
	repo.On("BlacklistJWT", mock.Anything, "jti-1", mock.Anything).Return(errDB)

	require.NoError(t, uc.LogoutSession(WithSession(context.Background(), sidA), "user-123", "jti-1", ""),
		"sesinya sudah dicabut, jadi access token mati lewat sid walau daftar cabut gagal ditulis")
	assert.Equal(t, []string{"LOGOUT"}, auditAksi(recorder))
}

func TestLogoutSession_TanpaSidMencabutFamilyDariRefreshTokenHanyaBilaMilikPenggunaItu(t *testing.T) {
	t.Run("milik pengguna yang sama: hanya family itu", func(t *testing.T) {
		repo := new(mockRepository)
		uc := newTestUseCase(repo)
		repo.On("GetRefreshTokenByHash", mock.Anything, mock.Anything).Return(liveToken(7, sidB), nil)
		repo.On("RevokeSession", mock.Anything, "user-123", sidB, mock.Anything).Return(true, nil)
		repo.On("BlacklistJWT", mock.Anything, "jti-1", mock.Anything).Return(nil)

		require.NoError(t, uc.LogoutSession(context.Background(), "user-123", "jti-1", "rt-cookie"))
		repo.AssertNotCalled(t, "RevokeAllUserRefreshTokens", mock.Anything, mock.Anything)
	})
	t.Run("milik pengguna lain: diabaikan, semua sesi pemanggil dicabut (bukan sesi orang lain)", func(t *testing.T) {
		repo := new(mockRepository)
		uc := newTestUseCase(repo)
		milikLain := liveToken(7, sidB)
		milikLain.UserID = "user-lain"
		repo.On("GetRefreshTokenByHash", mock.Anything, mock.Anything).Return(milikLain, nil)
		repo.On("RevokeAllUserRefreshTokens", mock.Anything, "user-123").Return(nil)
		repo.On("BlacklistJWT", mock.Anything, "jti-1", mock.Anything).Return(nil)

		require.NoError(t, uc.LogoutSession(context.Background(), "user-123", "jti-1", "rt-orang-lain"))
		repo.AssertNotCalled(t, "RevokeSession", mock.Anything, mock.Anything, mock.Anything, mock.Anything)
	})
}

func TestLogoutSession_TanpaJtiTidakMenulisDaftarCabut(t *testing.T) {
	repo := new(mockRepository)
	uc := newTestUseCase(repo)
	repo.On("RevokeSession", mock.Anything, "user-123", sidA, mock.Anything).Return(true, nil)

	require.NoError(t, uc.LogoutSession(WithSession(context.Background(), sidA), "user-123", "", ""))
	repo.AssertNotCalled(t, "BlacklistJWT", mock.Anything, mock.Anything, mock.Anything)
}

func TestLogoutAll_Kegagalan(t *testing.T) {
	t.Run("pencabutan gagal: tidak ada daftar cabut atau audit", func(t *testing.T) {
		repo := new(mockRepository)
		uc, recorder := newTestUseCaseWithAudit(repo)
		repo.On("RevokeSessions", mock.Anything, "user-123", "", mock.Anything).Return(nil, errDB)

		_, err := uc.LogoutAll(context.Background(), "user-123", "jti-1")

		require.Error(t, err)
		repo.AssertNotCalled(t, "BlacklistJWT", mock.Anything, mock.Anything, mock.Anything)
		assert.Empty(t, recorder.Entries())
	})
	t.Run("daftar cabut gagal: tetap berhasil dan jumlah sesi benar", func(t *testing.T) {
		repo := new(mockRepository)
		uc, recorder := newTestUseCaseWithAudit(repo)
		repo.On("RevokeSessions", mock.Anything, "user-123", "", mock.Anything).Return([]string{sidA, sidB}, nil)
		repo.On("BlacklistJWT", mock.Anything, "jti-1", mock.Anything).Return(errDB)

		n, err := uc.LogoutAll(context.Background(), "user-123", "jti-1")

		require.NoError(t, err)
		assert.Equal(t, 2, n)
		entry, _ := recorder.Last()
		assert.Equal(t, "LOGOUT_ALL", entry.Aksi)
		assert.Equal(t, 2, entry.Metadata["jumlah_sesi"])
	})
}

func TestDaftarDanPencabutanSesi_GalatRepositoriDiteruskanTanpaAudit(t *testing.T) {
	t.Run("ListSessions", func(t *testing.T) {
		repo := new(mockRepository)
		uc := newTestUseCase(repo)
		repo.On("ListActiveRefreshTokens", mock.Anything, "user-123", mock.Anything).Return(nil, errDB)
		_, err := uc.ListSessions(context.Background(), "user-123")
		require.ErrorIs(t, err, errDB)
	})
	t.Run("RevokeSession: galat basis data bukan 'sesi tidak ditemukan'", func(t *testing.T) {
		repo := new(mockRepository)
		uc, recorder := newTestUseCaseWithAudit(repo)
		repo.On("RevokeSession", mock.Anything, "user-123", sidB, mock.Anything).Return(false, errDB)
		err := uc.RevokeSession(WithSession(context.Background(), sidA), "user-123", sidB)
		require.ErrorIs(t, err, errDB)
		assert.NotErrorIs(t, err, ErrSessionNotFound)
		assert.Empty(t, recorder.Entries())
	})
	t.Run("RevokeOtherSessions", func(t *testing.T) {
		repo := new(mockRepository)
		uc, recorder := newTestUseCaseWithAudit(repo)
		repo.On("RevokeSessions", mock.Anything, "user-123", sidA, mock.Anything).Return(nil, errDB)
		_, err := uc.RevokeOtherSessions(WithSession(context.Background(), sidA), "user-123")
		require.ErrorIs(t, err, errDB)
		assert.Empty(t, recorder.Entries())
	})
	t.Run("ForceLogoutUser: galat baca pengguna bukan 'pengguna tidak ada'", func(t *testing.T) {
		repo := new(mockRepository)
		uc := newTestUseCase(repo)
		repo.On("GetUserByID", mock.Anything, "user-123").Return(nil, errDB)
		_, err := uc.ForceLogoutUser(context.Background(), "user-123", "admin-1")
		require.ErrorIs(t, err, errDB)
		assert.NotErrorIs(t, err, ErrUserNotFound)
		repo.AssertNotCalled(t, "RevokeSessions", mock.Anything, mock.Anything, mock.Anything, mock.Anything)
	})
	t.Run("ForceLogoutUser: pencabutan gagal tidak diaudit", func(t *testing.T) {
		repo := new(mockRepository)
		uc, recorder := newTestUseCaseWithAudit(repo)
		repo.On("GetUserByID", mock.Anything, "user-123").Return(activeUser(), nil)
		repo.On("RevokeSessions", mock.Anything, "user-123", "", mock.Anything).Return(nil, errDB)
		_, err := uc.ForceLogoutUser(context.Background(), "user-123", "admin-1")
		require.ErrorIs(t, err, errDB)
		assert.Empty(t, recorder.Entries())
	})
}

// ── Lockout: pembacaan kunci SSO gagal-tutup ─────────────────────────

func TestPeriksaKunci_GalatBacaDitolakBukanDilewati(t *testing.T) {
	repo := new(mockRepository)
	uc := newTestUseCase(repo)
	repo.errAktif = errDB

	err := uc.periksaKunci(context.Background(), "test@example.com")

	require.Error(t, err)
	assert.NotErrorIs(t, err, ErrAccountLocked, "bukan terkunci, tetapi juga tidak boleh lolos")
}

func TestKunciPengguna_DaftarKosongTanpaKuerydanGalatDiteruskan(t *testing.T) {
	repo := new(mockRepository)
	uc := newTestUseCase(repo)
	hasil, err := uc.KunciPengguna(context.Background(), nil)
	require.NoError(t, err)
	assert.Empty(t, hasil)
	assert.Zero(t, repo.nMulai+repo.nKembali+repo.nHapus)

	repo.errAktif = errDB
	_, err = uc.KunciPengguna(context.Background(), []domain.Pengguna{*activeUser()})
	require.ErrorIs(t, err, errDB)
}

// ── SSO: kegagalan infrastruktur tidak membuka jalur pintas ──────────

func TestSSOCallback_PembacaanKunciGagalDitolakDanTidakMenautkanAkun(t *testing.T) {
	f := newSSOFixture(t, ssoUser("u1", "ani@kemenkeu.go.id"))
	f.repo.errAktif = errDB

	_, err := f.masuk()

	require.Error(t, err)
	assert.Nil(t, f.repo.stored("u1").SSOSubject, "akun tidak ditautkan bila status kunci tidak dapat dipastikan")
	assert.Empty(t, f.repo.refresh, "tidak ada sesi")
}

func TestSSOCallback_AkunHilangSaatMenautMenolakDanTidakMenerbitkanSesi(t *testing.T) {
	f := newSSOFixture(t, ssoUser("u1", "ani@kemenkeu.go.id"))
	f.repo.linkOverride = func(r *memRepo, userID string) (bool, error) {
		r.mu.Lock()
		delete(r.users, userID) // dihapus admin di sela pemeriksaan dan penautan
		r.mu.Unlock()
		return false, nil
	}

	_, err := f.masuk()

	require.Error(t, err)
	assert.NotErrorIs(t, err, ErrSSOAccountConflict)
	assert.Empty(t, f.repo.refresh)
}

type ssoRepoGalat struct {
	*memRepo
	errSubjek, errState error
}

func (r *ssoRepoGalat) GetUserBySSO(ctx context.Context, provider, subject string) (*domain.Pengguna, error) {
	if r.errSubjek != nil {
		return nil, r.errSubjek
	}
	return r.memRepo.GetUserBySSO(ctx, provider, subject)
}

func (r *ssoRepoGalat) ConsumeLoginState(ctx context.Context, hash string) (*domain.SSOLoginState, error) {
	if r.errState != nil {
		return nil, r.errState
	}
	return r.memRepo.ConsumeLoginState(ctx, hash)
}

func TestSSOCallback_GalatBasisDataBukanStateTidakValid(t *testing.T) {
	f := newSSOFixture(t, ssoUser("u1", "ani@kemenkeu.go.id"))
	repo := &ssoRepoGalat{memRepo: f.repo, errState: errDB}
	f.sso = NewSSO(f.uc, repo, SSOConfig{Provider: f.provider, Now: func() time.Time { return f.now }})
	state, binding := f.mulai()

	_, err := f.callback(state, binding)

	require.Error(t, err)
	assert.NotErrorIs(t, err, ErrSSOStateInvalid, "pengguna tidak disuruh mengulang bila yang rusak adalah server")
	assert.Zero(t, f.provider.exchangeCalls, "kode otorisasi tidak ditukar")
}

// Regresi desain: bila pencarian menurut subjek gagal karena basis data, TIDAK boleh jatuh ke pencocokan email
// (itu akan menautkan atau memasukkan akun lewat jalur yang lebih lemah).
func TestSSOCallback_PencarianSubjekGagalTidakJatuhKePencocokanEmail(t *testing.T) {
	f := newSSOFixture(t, ssoUser("u1", "ani@kemenkeu.go.id"))
	repo := &ssoRepoGalat{memRepo: f.repo, errSubjek: errDB}
	f.sso = NewSSO(f.uc, repo, SSOConfig{Provider: f.provider, Now: func() time.Time { return f.now }})

	_, err := f.masuk()

	require.ErrorIs(t, err, errDB)
	assert.Nil(t, f.repo.stored("u1").SSOSubject)
	assert.Empty(t, f.repo.refresh)
}
