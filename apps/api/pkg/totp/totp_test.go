package totp

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestGenerateSecret(t *testing.T) {
	secret := GenerateSecret()
	require.NotEmpty(t, secret)
	// Base32 encoded, should be alphanumeric uppercase
	assert.Greater(t, len(secret), 20)
}

func TestGenerateTOTP_Valid(t *testing.T) {
	secret := GenerateSecret()
	code, err := GenerateTOTP(secret, time.Now())
	require.NoError(t, err)
	assert.Len(t, code, 6)
}

func TestValidateTOTP_Valid(t *testing.T) {
	secret := GenerateSecret()
	now := time.Now()
	code, err := GenerateTOTP(secret, now)
	require.NoError(t, err)

	assert.True(t, ValidateTOTP(secret, code, now))
}

func TestValidateTOTP_Invalid(t *testing.T) {
	secret := GenerateSecret()
	assert.False(t, ValidateTOTP(secret, "000000", time.Now()))
}

func TestValidateTOTP_TimeSkew(t *testing.T) {
	secret := GenerateSecret()
	now := time.Now()
	code, err := GenerateTOTP(secret, now.Add(-30*time.Second)) // 1 step ago
	require.NoError(t, err)

	// Should still be valid within skew window
	assert.True(t, ValidateTOTP(secret, code, now))
}

func TestGenerateProvisioningURI(t *testing.T) {
	secret := GenerateSecret()
	uri := GenerateProvisioningURI(secret, "test@example.com")
	assert.Contains(t, uri, "otpauth://totp/")
	assert.Contains(t, uri, "test@example.com")
	assert.Contains(t, uri, secret)
}

func TestGenerateBackupCodes(t *testing.T) {
	codes := GenerateBackupCodes(8)
	require.Len(t, codes, 8)
	for _, code := range codes {
		assert.Regexp(t, `^[A-HJKMNP-Z2-9]{5}-[A-HJKMNP-Z2-9]{5}$`, code)
	}
}

func TestGenerateBackupCodes_Uniqueness(t *testing.T) {
	codes := GenerateBackupCodes(10)
	seen := make(map[string]bool)
	for _, c := range codes {
		assert.False(t, seen[c], "duplicate backup code: %s", c)
		seen[c] = true
	}
}

func TestGenerateSecret_IsRandomBase32OfTwentyBytes(t *testing.T) {
	a, b := GenerateSecret(), GenerateSecret()

	assert.NotEqual(t, a, b)
	assert.Len(t, a, 32, "20 byte menjadi 32 karakter base32 tanpa padding")
	assert.Regexp(t, `^[A-Z2-7]+$`, a)
}

func TestValidateTOTPStep_ReturnsMatchingStep(t *testing.T) {
	secret := GenerateSecret()
	now := time.Unix(1_700_000_010, 0)
	code, err := GenerateTOTP(secret, now)
	require.NoError(t, err)

	step, ok := ValidateTOTPStep(secret, code, now)
	assert.True(t, ok)
	assert.Equal(t, now.Unix()/DefaultPeriod, step)

	// Kode langkah sebelumnya masih diterima (toleransi jam) tetapi bernomor langkah lebih kecil.
	lama, err := GenerateTOTP(secret, now.Add(-DefaultPeriod*time.Second))
	require.NoError(t, err)
	stepLama, ok := ValidateTOTPStep(secret, lama, now)
	assert.True(t, ok)
	assert.Equal(t, step-1, stepLama)

	_, ok = ValidateTOTPStep(secret, "abcdef", now)
	assert.False(t, ok)
}

func TestNormalizeBackupCode(t *testing.T) {
	assert.Equal(t, "ABCDE23456", NormalizeBackupCode(" abcde-23456 "))
	assert.Equal(t, "BACKUP01", NormalizeBackupCode("backup01"))
}

func TestProvisioningURI_EscapesLabel(t *testing.T) {
	uri := GenerateProvisioningURI("JBSWY3DPEHPK3PXP", "a b+c@gardainflasi.id")

	assert.Contains(t, uri, "otpauth://totp/Garda%20Inflasi:a%20b+c@gardainflasi.id?")
	assert.Contains(t, uri, "issuer=Garda+Inflasi")
}
