package mongodb

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func env(kv map[string]string) func(string) string { return func(k string) string { return kv[k] } }

func TestConfigDariEnv(t *testing.T) {
	cfg := ConfigDariEnv(env(nil))
	assert.False(t, cfg.Dikonfigurasi())
	assert.Equal(t, "dataunseen", cfg.DBUnseen)
	assert.Equal(t, "ml_predictions", cfg.DBSafe)
	assert.Equal(t, DefaultTimeout, cfg.Timeout)

	cfg = ConfigDariEnv(env(map[string]string{"MONGO_URL": " mongodb://a:27017 "}))
	assert.Equal(t, "mongodb://a:27017", cfg.URI, "MONGO_URL adalah fallback")

	cfg = ConfigDariEnv(env(map[string]string{"MONGO_URI": "mongodb://utama", "MONGO_URL": "mongodb://cadangan",
		"MONGO_DB_UNSEEN": "u", "MONGO_DB_SAFE": "s"}))
	assert.Equal(t, "mongodb://utama", cfg.URI, "MONGO_URI didahulukan")
	assert.Equal(t, "u", cfg.DBUnseen)
	assert.Equal(t, "s", cfg.DBSafe)
}

func TestBuka_TanpaKonfigurasiNil(t *testing.T) {
	k, err := Buka(Config{})
	assert.NoError(t, err)
	assert.Nil(t, k)
	assert.NoError(t, k.Tutup(context.Background()), "Tutup aman pada klien nil")
}

func TestBuka_URITidakValidTanpaMembocorkanSandi(t *testing.T) {
	_, err := Buka(Config{URI: "postgres://pengguna:rahasia-sekali@host/db"})
	require.ErrorIs(t, err, ErrURITidakValid)
	assert.False(t, strings.Contains(err.Error(), "rahasia-sekali"))
}

// Buka tidak menunggu server: Mongo yang mati tidak menahan start API, dan operasi gagal dalam batas Timeout.
func TestBuka_ServerMatiTidakMenahan(t *testing.T) {
	mulai := time.Now()
	k, err := Buka(Config{URI: "mongodb://127.0.0.1:1/?directConnection=true", DBUnseen: "u", DBSafe: "s", Timeout: 200 * time.Millisecond})
	require.NoError(t, err)
	defer func() { _ = k.Tutup(context.Background()) }()
	assert.Less(t, time.Since(mulai), 500*time.Millisecond)
	mulai = time.Now()
	assert.Error(t, k.Ping(context.Background()))
	assert.Less(t, time.Since(mulai), 2*time.Second)
}
