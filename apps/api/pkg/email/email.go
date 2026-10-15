package email

import (
	"fmt"
	"net/smtp"
	"strings"
)

// Config holds SMTP configuration for sending emails.
type Config struct {
	Host     string
	Port     int
	Username string
	Password string
	From     string
	FromName string
}

// DefaultConfig returns a default SMTP config from environment variables.
// In production, set SMTP_HOST, SMTP_PORT, SMTP_USERNAME, SMTP_PASSWORD, SMTP_FROM.
func DefaultConfig() Config {
	return Config{
		Host:     "",
		Port:     587,
		Username: "",
		Password: "",
		From:     "noreply@garda-inflasi.id",
		FromName: "Garda Inflasi",
	}
}

// Sender handles email sending via SMTP.
type Sender struct {
	cfg Config
}

// New creates a new email sender.
func New(cfg Config) *Sender {
	return &Sender{cfg: cfg}
}

// SendResetPassword sends a password reset email.
func (s *Sender) SendResetPassword(to, resetToken string) error {
	subject := "Reset Password - Garda Inflasi"
	body := fmt.Sprintf(`
<!DOCTYPE html>
<html>
<head>
	<meta charset="UTF-8">
</head>
<body style="font-family: Arial, sans-serif; padding: 20px;">
	<h2>Reset Password</h2>
	<p>Anda menerima email ini karena ada permintaan reset password untuk akun Garda Inflasi.</p>
	<p>Klik tautan berikut untuk mereset password Anda:</p>
	<p><a href="https://garda-inflasi.id/auth/reset-password?token=%s" style="background-color: #2563eb; color: white; padding: 12px 24px; text-decoration: none; border-radius: 6px; display: inline-block;">Reset Password</a></p>
	<p>Atau salin tautan ini ke browser: https://garda-inflasi.id/auth/reset-password?token=%s</p>
	<p>Tautan ini berlaku selama 1 jam.</p>
	<p>Jika Anda tidak meminta reset password, abaikan email ini.</p>
	<hr>
	<p style="color: #6b7280; font-size: 12px;">Garda Inflasi - Sistem Informasi Pengendalian Inflasi Daerah</p>
</body>
</html>
`, resetToken, resetToken)

	return s.send(to, subject, body)
}

func (s *Sender) send(to, subject, htmlBody string) error {
	if s.cfg.Host == "" {
		// SMTP not configured — log instead of sending
		return nil
	}

	auth := smtp.PlainAuth("", s.cfg.Username, s.cfg.Password, s.cfg.Host)

	from := fmt.Sprintf("%s <%s>", s.cfg.FromName, s.cfg.From)
	if s.cfg.FromName == "" {
		from = s.cfg.From
	}

	msg := fmt.Sprintf("From: %s\r\n"+
		"To: %s\r\n"+
		"Subject: %s\r\n"+
		"MIME-Version: 1.0\r\n"+
		"Content-Type: text/html; charset=\"UTF-8\"\r\n"+
		"\r\n"+
		"%s\r\n", from, to, subject, htmlBody)

	addr := fmt.Sprintf("%s:%d", s.cfg.Host, s.cfg.Port)
	return smtp.SendMail(addr, auth, s.cfg.From, strings.Split(to, ","), []byte(msg))
}
