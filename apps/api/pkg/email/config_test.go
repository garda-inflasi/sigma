package email

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestConfigFromEnv(t *testing.T) {
	t.Setenv("SMTP_HOST", "")
	t.Setenv("SMTP_PORT", "")
	t.Setenv("SMTP_USERNAME", "")
	t.Setenv("SMTP_PASSWORD", "")
	t.Setenv("SMTP_FROM", "")
	cfg := ConfigFromEnv()
	assert.Equal(t, DefaultConfig(), cfg)
	assert.False(t, New(cfg).Configured(), "tanpa SMTP_HOST pengiriman mati")

	t.Setenv("SMTP_HOST", "smtp.example.com")
	t.Setenv("SMTP_PORT", "2525")
	t.Setenv("SMTP_USERNAME", "u")
	t.Setenv("SMTP_PASSWORD", "p")
	t.Setenv("SMTP_FROM", "ews@example.com")
	cfg = ConfigFromEnv()
	assert.Equal(t, Config{Host: "smtp.example.com", Port: 2525, Username: "u", Password: "p", From: "ews@example.com", FromName: "Garda Inflasi"}, cfg)
	assert.True(t, New(cfg).Configured())

	t.Setenv("SMTP_PORT", "bukan-angka")
	assert.Equal(t, 587, ConfigFromEnv().Port, "port tak valid memakai bawaan")
}

func TestSendHTML_NoSMTPIsNoOp(t *testing.T) {
	assert.NoError(t, New(DefaultConfig()).SendHTML("a@b.id", "Judul", "<p>isi</p>"))
}
