package crypto

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func env(values map[string]string) func(string) string {
	return func(key string) string { return values[key] }
}

func TestKeyring_SealOpenRoundTripWithVersionPrefix(t *testing.T) {
	ring, err := KeyringFromEnv(env(map[string]string{"AES_KEY": "abcdefghijklmnopqrstuvwxyz012345"}))
	require.NoError(t, err)

	sealed, err := ring.Seal("JBSWY3DPEHPK3PXP")
	require.NoError(t, err)

	assert.True(t, ring.IsSealed(sealed))
	assert.Contains(t, sealed, "v1:")
	assert.NotContains(t, sealed, "JBSWY3DPEHPK3PXP")
	opened, err := ring.Open(sealed)
	require.NoError(t, err)
	assert.Equal(t, "JBSWY3DPEHPK3PXP", opened)

	again, err := ring.Seal("JBSWY3DPEHPK3PXP")
	require.NoError(t, err)
	assert.NotEqual(t, sealed, again, "nonce acak: segel dua kali menghasilkan data berbeda")
}

func TestKeyring_OpenRejectsWrongKeyUnknownVersionAndPlaintext(t *testing.T) {
	a, _ := NewKeyring("v1", map[string][]byte{"v1": []byte("abcdefghijklmnopqrstuvwxyz012345")})
	b, _ := NewKeyring("v1", map[string][]byte{"v1": []byte("5432109876zyxwvutsrqponmlkjihgfe")})
	sealed, err := a.Seal("rahasia")
	require.NoError(t, err)

	_, err = b.Open(sealed)
	assert.Error(t, err, "kunci lain tidak bisa membuka")
	_, err = a.Open("v9:AAAA")
	assert.ErrorContains(t, err, "tidak dikenal")
	_, err = a.Open("JBSWY3DPEHPK3PXP")
	assert.Error(t, err, "plaintext lama bukan data bersegel")
	assert.False(t, a.IsSealed("JBSWY3DPEHPK3PXP"))
	assert.False(t, a.IsSealed("v9:AAAA"))
}

func TestKeyring_RotationKeepsOldVersionReadable(t *testing.T) {
	lama, _ := NewKeyring("v1", map[string][]byte{"v1": []byte("abcdefghijklmnopqrstuvwxyz012345")})
	sealedLama, err := lama.Seal("rahasia")
	require.NoError(t, err)

	baru, err := NewKeyring("v2", map[string][]byte{
		"v1": []byte("abcdefghijklmnopqrstuvwxyz012345"),
		"v2": []byte("5432109876zyxwvutsrqponmlkjihgfe"),
	})
	require.NoError(t, err)

	opened, err := baru.Open(sealedLama)
	require.NoError(t, err)
	assert.Equal(t, "rahasia", opened)
	sealedBaru, err := baru.Seal("rahasia")
	require.NoError(t, err)
	assert.Contains(t, sealedBaru, "v2:")
}

func TestNewKeyring_Validation(t *testing.T) {
	_, err := NewKeyring("v1", map[string][]byte{"v1": []byte("pendek")})
	assert.Error(t, err, "kunci harus 32 byte")
	_, err = NewKeyring("v2", map[string][]byte{"v1": []byte(DevKey)})
	assert.Error(t, err, "versi terkini harus ada")
	_, err = NewKeyring("v:1", map[string][]byte{"v:1": []byte(DevKey)})
	assert.Error(t, err, "versi tidak boleh memuat pemisah")
}

func TestKeyringFromEnv(t *testing.T) {
	t.Run("kosong di luar produksi memakai kunci pengembangan", func(t *testing.T) {
		ring, err := KeyringFromEnv(env(nil))
		require.NoError(t, err)
		_, err = ring.Seal("x")
		assert.NoError(t, err)
	})
	t.Run("produksi wajib menyetel AES_KEY", func(t *testing.T) {
		_, err := KeyringFromEnv(env(map[string]string{"APP_ENV": "production"}))
		assert.ErrorContains(t, err, "wajib")
	})
	t.Run("produksi menolak kunci contoh", func(t *testing.T) {
		_, err := KeyringFromEnv(env(map[string]string{"APP_ENV": "production", "AES_KEY": DevKey}))
		assert.ErrorContains(t, err, "produksi")
	})
	t.Run("produksi menerima kunci sendiri", func(t *testing.T) {
		_, err := KeyringFromEnv(env(map[string]string{"APP_ENV": "production", "AES_KEY": "abcdefghijklmnopqrstuvwxyz012345"}))
		assert.NoError(t, err)
	})
	t.Run("panjang salah ditolak", func(t *testing.T) {
		_, err := KeyringFromEnv(env(map[string]string{"AES_KEY": "terlalu-pendek"}))
		assert.ErrorContains(t, err, "32 byte")
	})
}
