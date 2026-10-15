package totp

import (
	"crypto/hmac"
	"crypto/sha1"
	"encoding/base32"
	"encoding/binary"
	"fmt"
	"math"
	"strings"
	"time"

	"github.com/google/uuid"
)

const (
	// DefaultIssuer is the default issuer name for TOTP.
	DefaultIssuer = "Garda Inflasi"
	// DefaultDigits is the number of digits in TOTP code.
	DefaultDigits = 6
	// DefaultPeriod is the TOTP time step in seconds.
	DefaultPeriod = 30
	// DefaultSkew is the number of time steps to allow before/after.
	DefaultSkew = 1
)

// GenerateSecret creates a new random TOTP secret (base32 encoded).
func GenerateSecret() string {
	// Generate 20 random bytes (160 bits) for HMAC-SHA1
	key := uuid.New().String() + uuid.New().String()
	// Use first 20 bytes
	raw := []byte(key[:20])
	// Base32 encode (no padding for URL)
	secret := base32.StdEncoding.WithPadding(base32.NoPadding).EncodeToString(raw)
	return secret
}

// GenerateTOTP generates a TOTP code from a secret at a given time.
func GenerateTOTP(secret string, t time.Time) (string, error) {
	// Decode base32 secret
	secret = strings.ToUpper(secret)
	// Add padding if needed
	padding := 8 - (len(secret) % 8)
	if padding < 8 {
		secret += strings.Repeat("=", padding)
	}

	key, err := base32.StdEncoding.DecodeString(secret)
	if err != nil {
		return "", fmt.Errorf("decode secret: %w", err)
	}

	// Calculate counter
	counter := uint64(t.Unix()) / DefaultPeriod

	// Create HMAC-SHA1
	mac := hmac.New(sha1.New, key)
	counterBytes := make([]byte, 8)
	binary.BigEndian.PutUint64(counterBytes, counter)
	mac.Write(counterBytes)
	hash := mac.Sum(nil)

	// Dynamic truncation
	offset := hash[len(hash)-1] & 0x0F
	binaryCode := (uint32(hash[offset]&0x7F) << 24) |
		(uint32(hash[offset+1]) << 16) |
		(uint32(hash[offset+2]) << 8) |
		uint32(hash[offset+3])

	code := binaryCode % uint32(math.Pow10(DefaultDigits))

	// Format with leading zeros
	return fmt.Sprintf("%0*d", DefaultDigits, code), nil
}

// ValidateTOTP validates a TOTP code against a secret, with time skew allowance.
func ValidateTOTP(secret, code string, t time.Time) bool {
	for i := -DefaultSkew; i <= DefaultSkew; i++ {
		testTime := t.Add(time.Duration(i) * DefaultPeriod * time.Second)
		expected, err := GenerateTOTP(secret, testTime)
		if err != nil {
			return false
		}
		if hmac.Equal([]byte(expected), []byte(code)) {
			return true
		}
	}
	return false
}

// GenerateProvisioningURI creates a otpauth:// URI for QR code generation.
func GenerateProvisioningURI(secret, email string) string {
	return fmt.Sprintf(
		"otpauth://totp/%s:%s?secret=%s&issuer=%s&algorithm=SHA1&digits=%d&period=%d",
		DefaultIssuer, email, secret, DefaultIssuer, DefaultDigits, DefaultPeriod,
	)
}

// GenerateBackupCodes generates a set of backup codes.
func GenerateBackupCodes(count int) []string {
	codes := make([]string, count)
	for i := 0; i < count; i++ {
		code := uuid.New().String()[:8]
		codes[i] = strings.ToUpper(code)
	}
	return codes
}
