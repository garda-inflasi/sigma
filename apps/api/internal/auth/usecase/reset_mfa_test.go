package usecase

import (
	"context"
	"errors"
	"fmt"
	"testing"
	"time"

	"github.com/garda-inflasi/api/internal/audit"
	"github.com/garda-inflasi/api/internal/auth/domain"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

func TestResetMFAPengguna_MenolakAkunSendiriTanpaMenyentuhApaPun(t *testing.T) {
	repo := new(mockRepository)
	uc, recorder := newTestUseCaseWithAudit(repo)

	_, err := uc.ResetMFAPengguna(context.Background(), uid("admin-1"), uid("admin-1"))

	assert.ErrorIs(t, err, ErrCannotResetOwnMFA)
	repo.AssertNotCalled(t, "ResetMFA", mock.Anything, mock.Anything, mock.Anything)
	assert.Empty(t, recorder.Entries(), "penolakan bukan perubahan: tidak ada audit")
}

func TestResetMFAPengguna_PenggunaTidakAdaDijawabTidakDitemukan(t *testing.T) {
	repo := new(mockRepository)
	uc, recorder := newTestUseCaseWithAudit(repo)
	repo.On("ResetMFA", mock.Anything, "hilang", mock.Anything).Return(nil, fmt.Errorf("reset mfa: %w", gorm.ErrRecordNotFound))

	_, err := uc.ResetMFAPengguna(context.Background(), "hilang", uid("admin-1"))

	assert.ErrorIs(t, err, ErrUserNotFound)
	assert.Empty(t, recorder.Entries())
}

func TestResetMFAPengguna_GalatBasisDataBukanTidakDitemukanDanTidakDiaudit(t *testing.T) {
	repo := new(mockRepository)
	uc, recorder := newTestUseCaseWithAudit(repo)
	repo.On("ResetMFA", mock.Anything, uid("u-1"), mock.Anything).Return(nil, errors.New("koneksi putus"))

	_, err := uc.ResetMFAPengguna(context.Background(), uid("u-1"), uid("admin-1"))

	require.Error(t, err)
	assert.NotErrorIs(t, err, ErrUserNotFound)
	assert.NotErrorIs(t, err, ErrCannotResetOwnMFA)
	assert.Empty(t, recorder.Entries(), "reset yang gagal tidak dicatat sebagai berhasil")
}

func TestResetMFAPengguna_MencatatAuditSebelumSesudahDanMengembalikanHasil(t *testing.T) {
	repo := new(mockRepository)
	uc, recorder := newTestUseCaseWithAudit(repo)
	repo.On("ResetMFA", mock.Anything, uid("u-1"), mock.Anything).Return(&domain.HasilResetMFA{
		MFAAktifSebelumnya: true, RahasiaDihapus: true, KodeCadanganDihapus: 10, SesiDicabut: []string{"s-1", "s-2"},
	}, nil)

	hasil, err := uc.ResetMFAPengguna(context.Background(), uid("u-1"), uid("admin-1"))

	require.NoError(t, err)
	assert.Equal(t, &ResetMFAResult{MFAAktifSebelumnya: true, SesiDicabut: 2}, hasil)

	entries := recorder.Entries()
	require.Len(t, entries, 1)
	e := entries[0]
	assert.Equal(t, "RESET_MFA", e.Aksi)
	assert.Equal(t, uid("admin-1"), e.AktorID)
	assert.Equal(t, "pengguna", e.Entitas)
	assert.Equal(t, uid("u-1"), e.EntitasID)
	assert.Equal(t, audit.HasilBerhasil, e.Hasil)
	assert.Equal(t, map[string]any{"mfa_aktif": true}, e.Sebelum)
	assert.Equal(t, map[string]any{"mfa_aktif": false}, e.Sesudah)
	assert.Equal(t, map[string]any{
		"mfa_aktif_sebelumnya": true, "rahasia_dihapus": true, "kode_cadangan_dihapus": int64(10), "sesi_dicabut": 2,
	}, e.Metadata)
}

func TestResetMFAPengguna_TanpaMFAIdempotenTetapDicatat(t *testing.T) {
	repo := new(mockRepository)
	uc, recorder := newTestUseCaseWithAudit(repo)
	repo.On("ResetMFA", mock.Anything, uid("u-1"), mock.Anything).Return(&domain.HasilResetMFA{}, nil)

	for i := 0; i < 2; i++ {
		hasil, err := uc.ResetMFAPengguna(context.Background(), uid("u-1"), uid("admin-1"))
		require.NoError(t, err)
		assert.Equal(t, &ResetMFAResult{}, hasil)
	}

	entries := recorder.Entries()
	require.Len(t, entries, 2)
	assert.Equal(t, false, entries[0].Metadata["mfa_aktif_sebelumnya"])
	assert.Equal(t, 0, entries[0].Metadata["sesi_dicabut"])
}

func TestResetMFAPengguna_KegagalanAuditTidakMenggagalkanReset(t *testing.T) {
	repo := new(mockRepository)
	uc, recorder := newTestUseCaseWithAudit(repo)
	recorder.Err = errors.New("audit mati")
	repo.On("ResetMFA", mock.Anything, uid("u-1"), mock.Anything).Return(&domain.HasilResetMFA{MFAAktifSebelumnya: true}, nil)

	hasil, err := uc.ResetMFAPengguna(context.Background(), uid("u-1"), uid("admin-1"))

	require.NoError(t, err)
	assert.True(t, hasil.MFAAktifSebelumnya)
}

func TestResetMFAPengguna_MemakaiJamUsecaseUntukPencabutanSesi(t *testing.T) {
	repo := new(mockRepository)
	uc := newTestUseCase(repo)
	sekarang := jam()
	asli := jam
	jam = func() time.Time { return sekarang }
	t.Cleanup(func() { jam = asli })
	repo.On("ResetMFA", mock.Anything, uid("u-1"), sekarang).Return(&domain.HasilResetMFA{}, nil)

	_, err := uc.ResetMFAPengguna(context.Background(), uid("u-1"), uid("admin-1"))

	require.NoError(t, err)
	repo.AssertExpectations(t)
}

// Peran yang mewajibkan MFA tidak dilepas oleh reset: sesudahnya login pertama masuk ke setup MFA.
func TestResetMFAPengguna_PeranWajibMFAMasukSetupSaatLoginBerikutnya(t *testing.T) {
	repo := new(mockRepository)
	uc := newTestUseCase(repo)
	uc.cfg.MFAEnforce = true

	user := activeUser()
	user.Peran, user.Lapis = "ADMIN_SISTEM", "SISTEM"
	user.MfaAktif = false // keadaan sesudah reset
	repo.On("GetUserByEmail", mock.Anything, "test@example.com").Return(user, nil)
	repo.On("CreateRefreshToken", mock.Anything, mock.Anything).Return(nil)
	repo.On("CreateLoginAttempt", mock.Anything, mock.Anything).Return(nil)
	repo.On("TouchLastLogin", mock.Anything, uid("user-123"), mock.Anything).Return(nil)

	hasil, err := uc.Login(context.Background(), "test@example.com", "Test1234!", "10.0.0.1")

	require.NoError(t, err)
	assert.False(t, hasil.MFARequired, "tidak ada faktor kedua lagi untuk ditantang")
	assert.True(t, hasil.MFASetupRequired, "peran wajib MFA diarahkan ke setup")
}
