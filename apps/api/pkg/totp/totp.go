package totp

import (
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha1"
	"encoding/base32"
	"encoding/binary"
	"fmt"
	"math"
	"net/url"
	"strings"
	"time"
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

// secretBytes adalah panjang rahasia TOTP (160 bit, sesuai RFC 4226 untuk HMAC-SHA1).
const secretBytes = 20

// GenerateSecret creates a new random TOTP secret (base32 encoded) from the system CSPRNG.
func GenerateSecret() string {
	raw := make([]byte, secretBytes)
	if _, err := rand.Read(raw); err != nil {
		// crypto/rand tidak pernah gagal pada platform yang didukung; gagal berarti lingkungan rusak.
		panic("totp: sumber acak sistem tidak tersedia: " + err.Error())
	}
	return base32.StdEncoding.WithPadding(base32.NoPadding).EncodeToString(raw)
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
	_, ok := ValidateTOTPStep(secret, code, t)
	return ok
}

// ValidateTOTPStep seperti ValidateTOTP, dan mengembalikan nomor langkah waktu (detik Unix dibagi periode)
// yang cocok. Penyimpan dapat menolak langkah yang sudah pernah dipakai agar satu kode tidak berlaku dua kali.
func ValidateTOTPStep(secret, code string, t time.Time) (int64, bool) {
	for i := -DefaultSkew; i <= DefaultSkew; i++ {
		testTime := t.Add(time.Duration(i) * DefaultPeriod * time.Second)
		expected, err := GenerateTOTP(secret, testTime)
		if err != nil {
			return 0, false
		}
		if hmac.Equal([]byte(expected), []byte(code)) {
			return testTime.Unix() / DefaultPeriod, true
		}
	}
	return 0, false
}

// GenerateProvisioningURI creates a otpauth:// URI for QR code generation.
func GenerateProvisioningURI(secret, email string) string {
	label := url.PathEscape(DefaultIssuer + ":" + email)
	return fmt.Sprintf(
		"otpauth://totp/%s?secret=%s&issuer=%s&algorithm=SHA1&digits=%d&period=%d",
		label, secret, url.QueryEscape(DefaultIssuer), DefaultDigits, DefaultPeriod,
	)
}

// Kode cadangan: 10 karakter dari abjad tanpa karakter yang mudah tertukar (0/O, 1/I/L), ditampilkan
// berkelompok "XXXXX-XXXXX" (50 bit entropi).
const (
	backupCodeAlphabet = "ABCDEFGHJKMNPQRSTUVWXYZ23456789"
	backupCodeLength   = 10
	backupCodeGroup    = 5
)

// GenerateBackupCodes generates a set of backup codes from the system CSPRNG.
func GenerateBackupCodes(count int) []string {
	codes := make([]string, count)
	for i := range codes {
		codes[i] = generateBackupCode()
	}
	return codes
}

func generateBackupCode() string {
	raw := make([]byte, backupCodeLength)
	if _, err := rand.Read(raw); err != nil {
		panic("totp: sumber acak sistem tidak tersedia: " + err.Error())
	}
	code := make([]byte, 0, backupCodeLength+1)
	for i, b := range raw {
		if i == backupCodeGroup {
			code = append(code, '-')
		}
		// 256 tidak habis dibagi panjang abjad; bias sekitar 1% per karakter tidak berarti untuk kode sekali pakai.
		code = append(code, backupCodeAlphabet[int(b)%len(backupCodeAlphabet)])
	}
	return string(code)
}

// NormalizeBackupCode menyeragamkan kode yang diketik pengguna: huruf besar tanpa spasi dan tanda hubung.
func NormalizeBackupCode(code string) string {
	return strings.ToUpper(strings.NewReplacer(" ", "", "-", "").Replace(code))
}
