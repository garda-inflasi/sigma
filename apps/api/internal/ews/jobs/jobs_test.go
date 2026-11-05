package jobs

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestParseInterval(t *testing.T) {
	cases := map[string]time.Duration{"": 15 * time.Minute, "  ": 15 * time.Minute, "0": 0, "off": 0, "OFF": 0, "30s": 30 * time.Second, "2h": 2 * time.Hour}
	for raw, want := range cases {
		got, err := ParseInterval(raw, 15*time.Minute)
		require.NoError(t, err, raw)
		assert.Equal(t, want, got, raw)
	}
	for _, raw := range []string{"abc", "-5m", "10"} {
		got, err := ParseInterval(raw, 15*time.Minute)
		assert.Error(t, err, raw)
		assert.Equal(t, 15*time.Minute, got, "nilai tak valid kembali ke bawaan")
	}
}

func TestConfigFromEnv(t *testing.T) {
	for _, k := range []string{"EWS_EVAL_INTERVAL", "EWS_NOTIF_INTERVAL", "EWS_ESKALASI_MENIT"} {
		t.Setenv(k, "")
	}
	cfg, err := ConfigFromEnv()
	require.NoError(t, err)
	assert.Equal(t, Config{EvaluasiInterval: DefaultEvaluasiInterval, NotifInterval: 30 * time.Second}, cfg, "bawaan aman: eskalasi otomatis mati")

	t.Setenv("EWS_EVAL_INTERVAL", "off")
	t.Setenv("EWS_NOTIF_INTERVAL", "10s")
	t.Setenv("EWS_ESKALASI_MENIT", "90")
	cfg, err = ConfigFromEnv()
	require.NoError(t, err)
	assert.Equal(t, Config{EvaluasiInterval: 0, NotifInterval: 10 * time.Second, EskalasiSetelah: 90 * time.Minute}, cfg)
}

func TestConfigFromEnv_InvalidValuesFallBackAndAreReported(t *testing.T) {
	t.Setenv("EWS_EVAL_INTERVAL", "sering")
	t.Setenv("EWS_NOTIF_INTERVAL", "")
	t.Setenv("EWS_ESKALASI_MENIT", "-5")

	cfg, err := ConfigFromEnv()

	require.Error(t, err)
	assert.ErrorContains(t, err, "EWS_EVAL_INTERVAL")
	assert.ErrorContains(t, err, "EWS_ESKALASI_MENIT")
	assert.Equal(t, DefaultEvaluasiInterval, cfg.EvaluasiInterval)
	assert.Zero(t, cfg.EskalasiSetelah)
}
