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
		assert.Len(t, code, 8)
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
