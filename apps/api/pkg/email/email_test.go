package email

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestDefaultConfig(t *testing.T) {
	cfg := DefaultConfig()
	assert.Equal(t, "", cfg.Host)
	assert.Equal(t, 587, cfg.Port)
	assert.Equal(t, "", cfg.Username)
	assert.Equal(t, "", cfg.Password)
	assert.Equal(t, "noreply@garda-inflasi.id", cfg.From)
	assert.Equal(t, "Garda Inflasi", cfg.FromName)
}

func TestNew(t *testing.T) {
	cfg := Config{Host: "smtp.example.com", Port: 587, Username: "user", Password: "pass", From: "test@test.com", FromName: "Test"}
	s := New(cfg)
	assert.NotNil(t, s)
}

func TestSendResetPassword_NoSMTP(t *testing.T) {
	// When SMTP is not configured, SendResetPassword should return nil (no-op)
	cfg := DefaultConfig()
	s := New(cfg)
	err := s.SendResetPassword("test@example.com", "reset-token-123")
	assert.NoError(t, err)
}

func TestSendResetPassword_EmptyFrom(t *testing.T) {
	cfg := Config{Host: "", Port: 587, From: "", FromName: ""}
	s := New(cfg)
	err := s.SendResetPassword("test@example.com", "reset-token-123")
	assert.NoError(t, err)
}
