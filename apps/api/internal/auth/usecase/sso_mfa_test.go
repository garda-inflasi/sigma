package usecase

import (
	"context"
	"testing"

	"github.com/garda-inflasi/api/internal/audit"
	"github.com/garda-inflasi/api/pkg/totp"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

// Regresi: audit LOGIN akhir untuk SSO + OTP hanya memuat {"mfa": "totp"}; "metode: sso_google" hanya ada pada
// SSO_LOGIN sebelumnya sehingga jejak masuk lewat Google hilang dari entri LOGIN yang menandai sesi terbit.
// Kini metode login ikut dalam token tantangan MFA dan ditulis ulang saat OTP lolos.

// fixtureSSOMFA adalah akun ber-MFA yang sudah tertaut, siap melewati Callback lalu ValidateMFALogin.
func fixtureSSOMFA(t *testing.T) (*ssoFixture, string) {
	t.Helper()
	u := ssoUser("u1", "ani@kemenkeu.go.id")
	f := newSSOFixture(t, u)
	secret := totp.GenerateSecret()
	sealed, err := f.uc.cfg.MFAKey.Seal(secret)
	require.NoError(t, err)
	f.repo.users["u1"].MfaAktif, f.repo.users["u1"].MfaSecret = true, &sealed
	return f, secret
}

func entriLoginBerhasil(t *testing.T, f *ssoFixture) audit.Entry {
	t.Helper()
	var hasil []audit.Entry
	for _, e := range f.auditAksi("LOGIN") {
		if e.Hasil == audit.HasilBerhasil {
			hasil = append(hasil, e)
		}
	}
	require.Len(t, hasil, 1, "tepat satu LOGIN berhasil setelah OTP")
	return hasil[0]
}

func TestSSOMFA_TokenTantanganMembawaMetodeLogin(t *testing.T) {
	f, _ := fixtureSSOMFA(t)

	res, err := f.masuk()

	require.NoError(t, err)
	require.True(t, res.MFARequired)
	challenge, err := f.tokens.VerifyMFAChallenge(res.MFAToken)
	require.NoError(t, err)
	assert.Equal(t, "u1", challenge.UserID)
	assert.Equal(t, MetodeSSOGoogle, challenge.LoginMethod)
}

func TestSSOMFA_AuditLoginAkhirMemuatMetodeSSODanFaktorKedua(t *testing.T) {
	f, secret := fixtureSSOMFA(t)
	f.repo.On("AdvanceMFAStep", mock.Anything, "u1", mock.AnythingOfType("int64")).Return(true, nil)
	res, err := f.masuk()
	require.NoError(t, err)

	sesi, err := f.uc.ValidateMFALogin(context.Background(), res.MFAToken, kodeSekarang(t, secret), "10.0.0.1")

	require.NoError(t, err)
	assert.NotEmpty(t, sesi.AccessToken)
	entri := entriLoginBerhasil(t, f)
	assert.Equal(t, map[string]any{"mfa": metodeTOTP, "metode": MetodeSSOGoogle}, entri.Metadata)
	assert.Equal(t, "u1", entri.AktorID)
}

func TestSSOMFA_KodeCadanganJugaMencatatMetodeSSO(t *testing.T) {
	f, _ := fixtureSSOMFA(t)
	f.repo.On("UseBackupCode", mock.Anything, "u1", mock.Anything, mock.Anything).Return(true, nil)
	res, err := f.masuk()
	require.NoError(t, err)

	_, err = f.uc.ValidateMFALogin(context.Background(), res.MFAToken, "ABCD-EFGH-IJKL", "10.0.0.1")

	require.NoError(t, err)
	entri := entriLoginBerhasil(t, f)
	assert.Equal(t, map[string]any{"mfa": metodeKodeCadangan, "metode": MetodeSSOGoogle}, entri.Metadata)
}

func TestSSOMFA_LoginKataSandiTidakMemilikiMetode(t *testing.T) {
	f, secret := fixtureSSOMFA(t)
	f.repo.On("AdvanceMFAStep", mock.Anything, "u1", mock.AnythingOfType("int64")).Return(true, nil)
	// Tantangan dari alur kata sandi: tidak membawa metode.
	challenge, err := f.tokens.CreateMFAChallengeToken("u1")
	require.NoError(t, err)

	_, err = f.uc.ValidateMFALogin(context.Background(), challenge, kodeSekarang(t, secret), "10.0.0.1")

	require.NoError(t, err)
	entri := entriLoginBerhasil(t, f)
	assert.Equal(t, map[string]any{"mfa": metodeTOTP}, entri.Metadata, "kata sandi + OTP tidak diberi metode sso_google")
}

func TestSSOMFA_KodeSalahTidakMenulisLoginBerhasil(t *testing.T) {
	f, _ := fixtureSSOMFA(t)
	res, err := f.masuk()
	require.NoError(t, err)

	_, err = f.uc.ValidateMFALogin(context.Background(), res.MFAToken, "000000", "10.0.0.1")

	require.ErrorIs(t, err, ErrMFACodeInvalid)
	for _, e := range f.auditAksi("LOGIN") {
		assert.NotEqual(t, audit.HasilBerhasil, e.Hasil)
	}
}
