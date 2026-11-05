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
	t.Setenv("EWS_EVAL_INTERVAL", "")
	cfg, err := ConfigFromEnv()
	require.NoError(t, err)
	assert.Equal(t, DefaultEvaluasiInterval, cfg.EvaluasiInterval)

	t.Setenv("EWS_EVAL_INTERVAL", "off")
	cfg, err = ConfigFromEnv()
	require.NoError(t, err)
	assert.Zero(t, cfg.EvaluasiInterval)

	t.Setenv("EWS_EVAL_INTERVAL", "sering")
	cfg, err = ConfigFromEnv()
	assert.ErrorContains(t, err, "EWS_EVAL_INTERVAL")
	assert.Equal(t, DefaultEvaluasiInterval, cfg.EvaluasiInterval)
}
