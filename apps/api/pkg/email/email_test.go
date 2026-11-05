package email

import (
	"bufio"
	"net"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestDefaultConfig(t *testing.T) {
	cfg := DefaultConfig()
	assert.Equal(t, "", cfg.Host)
	assert.Equal(t, 587, cfg.Port)
	assert.Equal(t, "", cfg.Username)
	assert.Equal(t, "", cfg.Password)
	assert.Equal(t, "noreply@gardainflasi.id", cfg.From)
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

func TestResetLink(t *testing.T) {
	assert.Equal(t, "https://gardainflasi.id/reset-password?token=abc-123", New(DefaultConfig()).ResetLink("abc-123"), "bawaan memakai alamat publik")
	assert.Equal(t, "http://localhost:3000/reset-password?token=abc-123", New(Config{BaseURL: "http://localhost:3000/"}).ResetLink("abc-123"), "garis miring akhir dibuang")
	assert.Equal(t, "https://x.id/reset-password?token=a%26b", New(Config{BaseURL: "https://x.id"}).ResetLink("a&b"), "token di-escape")
}

func TestAuth_UsernameKosongTanpaAutentikasi(t *testing.T) {
	assert.Nil(t, New(Config{Host: "mailpit", Port: 1025}).auth(), "relay tanpa username tidak memakai AUTH")
	assert.NotNil(t, New(Config{Host: "smtp.example.com", Port: 587, Username: "u", Password: "p"}).auth())
}

// smtpPalsu adalah server SMTP minimal di loopback yang mencatat pesan yang diterimanya.
// ehloAuth menentukan apakah EHLO mengiklankan AUTH (relay tanpa autentikasi seperti Mailpit tidak).
type smtpPalsu struct {
	addr        string
	mu          sync.Mutex
	pesan       []string
	authDipakai bool
}

func jalankanSMTPPalsu(t *testing.T, ehloAuth bool) *smtpPalsu {
	t.Helper()
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)
	t.Cleanup(func() { _ = ln.Close() })
	srv := &smtpPalsu{addr: ln.Addr().String()}
	go func() {
		for {
			conn, err := ln.Accept()
			if err != nil {
				return
			}
			go srv.layani(conn, ehloAuth)
		}
	}()
	return srv
}

func (s *smtpPalsu) layani(conn net.Conn, ehloAuth bool) {
	defer func() { _ = conn.Close() }()
	_ = conn.SetDeadline(time.Now().Add(5 * time.Second))
	r := bufio.NewReader(conn)
	tulis := func(b string) { _, _ = conn.Write([]byte(b + "\r\n")) }
	tulis("220 palsu ESMTP")
	for {
		baris, err := r.ReadString('\n')
		if err != nil {
			return
		}
		perintah := strings.ToUpper(strings.TrimSpace(baris))
		switch {
		case strings.HasPrefix(perintah, "EHLO"), strings.HasPrefix(perintah, "HELO"):
			if ehloAuth {
				tulis("250-palsu")
				tulis("250 AUTH PLAIN")
			} else {
				tulis("250 palsu")
			}
		case strings.HasPrefix(perintah, "AUTH"):
			s.mu.Lock()
			s.authDipakai = true
			s.mu.Unlock()
			tulis("235 ok")
		case strings.HasPrefix(perintah, "MAIL FROM"), strings.HasPrefix(perintah, "RCPT TO"):
			tulis("250 ok")
		case perintah == "DATA":
			tulis("354 mulai")
			var sb strings.Builder
			for {
				l, err := r.ReadString('\n')
				if err != nil {
					return
				}
				if l == ".\r\n" {
					break
				}
				sb.WriteString(l)
			}
			s.mu.Lock()
			s.pesan = append(s.pesan, sb.String())
			s.mu.Unlock()
			tulis("250 diterima")
		case perintah == "QUIT":
			tulis("221 bye")
			return
		default:
			tulis("250 ok")
		}
	}
}

func (s *smtpPalsu) konfigurasi(t *testing.T, username, password string) Config {
	t.Helper()
	host, portStr, err := net.SplitHostPort(s.addr)
	require.NoError(t, err)
	port, err := strconv.Atoi(portStr)
	require.NoError(t, err)
	return Config{Host: host, Port: port, Username: username, Password: password, From: "noreply@gardainflasi.id", FromName: "Garda Inflasi", BaseURL: "http://localhost:8090"}
}

func TestSend_RelayTanpaAutentikasi(t *testing.T) {
	srv := jalankanSMTPPalsu(t, false) // seperti Mailpit: tidak mengiklankan AUTH
	sender := New(srv.konfigurasi(t, "", ""))

	require.NoError(t, sender.SendResetPassword("budi@gardainflasi.id", "tok-123"))

	srv.mu.Lock()
	defer srv.mu.Unlock()
	require.Len(t, srv.pesan, 1)
	assert.False(t, srv.authDipakai, "username kosong tidak boleh memicu AUTH")
	assert.Contains(t, srv.pesan[0], "To: budi@gardainflasi.id")
	assert.Contains(t, srv.pesan[0], "http://localhost:8090/reset-password?token=tok-123")
}

func TestSend_DenganUsernameMemakaiAuth(t *testing.T) {
	srv := jalankanSMTPPalsu(t, true)
	sender := New(srv.konfigurasi(t, "user", "rahasia"))

	require.NoError(t, sender.SendHTML("budi@gardainflasi.id", "Tes", "<p>halo</p>"))

	srv.mu.Lock()
	defer srv.mu.Unlock()
	assert.True(t, srv.authDipakai, "username terisi harus memakai AUTH PLAIN")
	require.Len(t, srv.pesan, 1)
}

func TestSend_UsernameTerisiTanpaDukunganAuthGagal(t *testing.T) {
	srv := jalankanSMTPPalsu(t, false)
	err := New(srv.konfigurasi(t, "user", "rahasia")).SendHTML("budi@gardainflasi.id", "Tes", "<p>halo</p>")
	assert.Error(t, err, "konfigurasi kredensial ke server tanpa AUTH harus terlihat sebagai galat")
}
