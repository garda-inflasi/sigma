package crypto

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestHashPassword(t *testing.T) {
	hash, err := HashPassword("Test1234!")
	require.NoError(t, err)
	assert.NotEmpty(t, hash)
	assert.NotEqual(t, "Test1234!", hash)
}

func TestVerifyPassword_Success(t *testing.T) {
	hash, err := HashPassword("Test1234!")
	require.NoError(t, err)

	assert.True(t, VerifyPassword("Test1234!", hash))
}

func TestVerifyPassword_Failure(t *testing.T) {
	hash, err := HashPassword("Test1234!")
	require.NoError(t, err)

	assert.False(t, VerifyPassword("WrongPassword1!", hash))
}

func TestHashPassword_Empty(t *testing.T) {
	hash, err := HashPassword("")
	// bcrypt accepts empty passwords and produces a hash
	require.NoError(t, err)
	assert.NotEmpty(t, hash)
}

func TestEncryptDecrypt(t *testing.T) {
	key := []byte("aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa") // exactly 32 bytes for AES-256
	plaintext := []byte("sensitive-data-123")

	ciphertext, err := Encrypt(plaintext, key)
	require.NoError(t, err)
	assert.NotEmpty(t, ciphertext)

	decrypted, err := Decrypt(ciphertext, key)
	require.NoError(t, err)
	assert.Equal(t, plaintext, decrypted)
}

func TestEncryptDecrypt_WrongKey(t *testing.T) {
	key := []byte("aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa") // exactly 32 bytes for AES-256
	wrongKey := []byte("bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb") // exactly 32 bytes
	plaintext := []byte("sensitive-data-123")

	ciphertext, err := Encrypt(plaintext, key)
	require.NoError(t, err)

	_, err = Decrypt(ciphertext, wrongKey)
	assert.Error(t, err)
}

func TestEncrypt_EmptyKey(t *testing.T) {
	_, err := Encrypt([]byte("data"), []byte{})
	assert.Error(t, err)
}

func TestDecrypt_InvalidCiphertext(t *testing.T) {
	_, err := Decrypt("invalid-base64!", []byte("test-key-32-bytes-for-aes-256-gcm!!"))
	assert.Error(t, err)
}

func TestRandomBytes(t *testing.T) {
	b, err := RandomBytes(32)
	require.NoError(t, err)
	assert.Equal(t, 32, len(b))
}

func TestConstantTimeCompare_Equal(t *testing.T) {
	a := []byte("secret")
	b := []byte("secret")
	assert.True(t, ConstantTimeCompare(a, b))
}

func TestConstantTimeCompare_NotEqual(t *testing.T) {
	a := []byte("secret")
	b := []byte("secret2")
	assert.False(t, ConstantTimeCompare(a, b))
}
