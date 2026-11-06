package email

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestAktivasiLink(t *testing.T) {
	assert.Equal(t, "https://gardainflasi.id/aktivasi?token=abc-123", New(DefaultConfig()).AktivasiLink("abc-123"), "bawaan memakai alamat publik")
	assert.Equal(t, "http://localhost:3000/aktivasi?token=abc-123", New(Config{BaseURL: "http://localhost:3000/"}).AktivasiLink("abc-123"), "garis miring akhir dibuang")
	assert.Equal(t, "https://x.id/aktivasi?token=a%26b%3Dc", New(Config{BaseURL: "https://x.id"}).AktivasiLink("a&b=c"), "token di-escape")
	assert.Equal(t, "https://x.id/aktivasi?token=A_b-9", New(Config{BaseURL: "https://x.id"}).AktivasiLink("A_b-9"), "base64url tidak berubah")
}

func TestSendUndangan_NoSMTPIsNoOp(t *testing.T) {
	assert.NoError(t, New(DefaultConfig()).SendUndangan("a@b.id", "Budi", "tok", 72*time.Hour))
}

func TestSendUndangan_IsiEmail(t *testing.T) {
	srv := jalankanSMTPPalsu(t, false)
	sender := New(srv.konfigurasi(t, "", ""))

	require.NoError(t, sender.SendUndangan("budi@gardainflasi.id", "Budi Santoso", "tok_ABC-123", 72*time.Hour))

	srv.mu.Lock()
	defer srv.mu.Unlock()
	require.Len(t, srv.pesan, 1)
	pesan := srv.pesan[0]
	assert.Contains(t, pesan, "To: budi@gardainflasi.id")
	assert.Contains(t, pesan, "Subject: Undangan Akun - Garda Inflasi")
	assert.Contains(t, pesan, "Content-Type: text/html; charset=\"UTF-8\"")
	assert.Contains(t, pesan, "http://localhost:8090/aktivasi?token=tok_ABC-123")
	assert.Contains(t, pesan, "Halo Budi Santoso,")
	assert.Contains(t, pesan, "berlaku 72 jam")
	assert.Contains(t, pesan, "satu kali")
}

func TestSendUndangan_NamaDariAdminDiEscape(t *testing.T) {
	srv := jalankanSMTPPalsu(t, false)
	sender := New(srv.konfigurasi(t, "", ""))

	require.NoError(t, sender.SendUndangan("budi@gardainflasi.id", `<script>alert("x")</script> & Co`, "tok", time.Hour))

	srv.mu.Lock()
	defer srv.mu.Unlock()
	require.Len(t, srv.pesan, 1)
	assert.NotContains(t, srv.pesan[0], "<script>")
	assert.Contains(t, srv.pesan[0], "&lt;script&gt;alert(&#34;x&#34;)&lt;/script&gt; &amp; Co")
}

func TestSendUndangan_BarisBaruDiPenerimaDibuang(t *testing.T) {
	srv := jalankanSMTPPalsu(t, false)
	sender := New(srv.konfigurasi(t, "", ""))

	require.NoError(t, sender.SendUndangan("budi@gardainflasi.id\r\nBcc: jahat@x.id", "Budi", "tok", time.Hour))

	srv.mu.Lock()
	defer srv.mu.Unlock()
	require.Len(t, srv.pesan, 1)
	assert.NotContains(t, srv.pesan[0], "\r\nBcc:", "tidak bisa menyisipkan header tambahan")
}

func TestDurasiTeks(t *testing.T) {
	for masukan, harapan := range map[time.Duration]string{
		72 * time.Hour:                  "72 jam",
		24 * time.Hour:                  "24 jam",
		time.Hour:                       "1 jam",
		90 * time.Minute:                "2 jam",
		30 * time.Minute:                "30 menit",
		45 * time.Second:                "1 menit",
		time.Second:                     "1 menit",
		0:                               "1 menit",
		59*time.Minute + 40*time.Second: "60 menit",
	} {
		assert.Equal(t, harapan, durasiTeks(masukan), masukan.String())
	}
}
