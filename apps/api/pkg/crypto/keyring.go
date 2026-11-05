package crypto

import (
	"errors"
	"fmt"
	"strings"
)

const (
	// KeyLength adalah panjang kunci AES-256 dalam byte.
	KeyLength = 32
	// DevKey adalah kunci pengembangan yang dipakai bila AES_KEY kosong di luar produksi. Nilainya sama
	// dengan contoh di env.example dan tidak boleh dipakai di produksi (KeyringFromEnv menolaknya).
	DevKey = "0123456789abcdef0123456789abcdef"
	// KeyVersion adalah versi kunci yang berasal dari AES_KEY.
	KeyVersion = "v1"

	sealedSeparator = ":"
)

// Keyring menyimpan beberapa versi kunci AES-256. Data tersegel menyertakan versi kuncinya
// ("v1:<base64>"), sehingga kunci baru dapat ditambahkan tanpa melepaskan data lama.
type Keyring struct {
	current string
	keys    map[string][]byte
}

// NewKeyring membuat Keyring dengan versi current sebagai kunci penyegel. Semua kunci harus 32 byte
// dan versi tidak boleh memuat ":".
func NewKeyring(current string, keys map[string][]byte) (*Keyring, error) {
	if _, ok := keys[current]; !ok {
		return nil, fmt.Errorf("kunci untuk versi %q tidak ada", current)
	}
	copied := make(map[string][]byte, len(keys))
	for version, key := range keys {
		if version == "" || strings.Contains(version, sealedSeparator) {
			return nil, fmt.Errorf("versi kunci %q tidak valid", version)
		}
		if len(key) != KeyLength {
			return nil, fmt.Errorf("kunci %q harus tepat %d byte, bukan %d", version, KeyLength, len(key))
		}
		copied[version] = append([]byte(nil), key...)
	}
	return &Keyring{current: current, keys: copied}, nil
}

// KeyringFromEnv membaca AES_KEY (tepat 32 byte) sebagai versi KeyVersion. Bila AES_KEY kosong, DevKey
// dipakai kecuali APP_ENV=production, yang wajib menyetel AES_KEY dan menolak DevKey.
func KeyringFromEnv(getenv func(string) string) (*Keyring, error) {
	key := getenv("AES_KEY")
	production := getenv("APP_ENV") == "production"
	switch {
	case key == "" && production:
		return nil, errors.New("AES_KEY wajib diisi di produksi")
	case key == "":
		key = DevKey
	case production && key == DevKey:
		return nil, errors.New("AES_KEY contoh pengembangan tidak boleh dipakai di produksi")
	}
	return NewKeyring(KeyVersion, map[string][]byte{KeyVersion: []byte(key)})
}

// Seal mengenkripsi plaintext dengan kunci terkini; hasilnya "<versi>:<base64>".
func (k *Keyring) Seal(plaintext string) (string, error) {
	sealed, err := Encrypt([]byte(plaintext), k.keys[k.current])
	if err != nil {
		return "", fmt.Errorf("segel data: %w", err)
	}
	return k.current + sealedSeparator + sealed, nil
}

// Open mendekripsi hasil Seal dengan kunci sesuai versi yang tertera.
func (k *Keyring) Open(sealed string) (string, error) {
	version, body, ok := strings.Cut(sealed, sealedSeparator)
	if !ok {
		return "", errors.New("data tidak bersegel")
	}
	key, known := k.keys[version]
	if !known {
		return "", fmt.Errorf("versi kunci %q tidak dikenal", version)
	}
	plaintext, err := Decrypt(body, key)
	if err != nil {
		return "", fmt.Errorf("buka segel data: %w", err)
	}
	return string(plaintext), nil
}

// IsSealed melaporkan apakah nilai tampak seperti hasil Seal (diawali versi kunci yang dikenal).
func (k *Keyring) IsSealed(value string) bool {
	version, _, ok := strings.Cut(value, sealedSeparator)
	if !ok {
		return false
	}
	_, known := k.keys[version]
	return known
}
