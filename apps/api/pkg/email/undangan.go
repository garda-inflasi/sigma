package email

import (
	"fmt"
	"html"
	"net/url"
	"strings"
	"time"
)

// AktivasiPath adalah halaman web tujuan tautan undangan (pengguna menetapkan kata sandi pertamanya).
const AktivasiPath = "/aktivasi"

// AktivasiLink menyusun tautan halaman aktivasi akun untuk token undangan.
func (s *Sender) AktivasiLink(token string) string {
	base := strings.TrimRight(s.cfg.BaseURL, "/")
	if base == "" {
		base = DefaultBaseURL
	}
	return base + AktivasiPath + "?token=" + url.QueryEscape(token)
}

// SendUndangan mengirim email undangan akun: tautan sekali pakai untuk menetapkan kata sandi pertama.
// nama berasal dari admin dan di-escape; berlaku adalah masa berlaku tautan yang disebut di isi email.
func (s *Sender) SendUndangan(to, nama, token string, berlaku time.Duration) error {
	subject := "Undangan Akun - Garda Inflasi"
	tautan := html.EscapeString(s.AktivasiLink(token))
	body := fmt.Sprintf(`
<!DOCTYPE html>
<html>
<head>
	<meta charset="UTF-8">
</head>
<body style="font-family: Arial, sans-serif; padding: 20px;">
	<h2>Selamat datang di Garda Inflasi</h2>
	<p>Halo %[1]s,</p>
	<p>Administrator telah membuatkan akun Garda Inflasi untuk Anda. Klik tombol berikut untuk menetapkan kata sandi dan mengaktifkan akun:</p>
	<p><a href="%[2]s" style="background-color: #2563eb; color: white; padding: 12px 24px; text-decoration: none; border-radius: 6px; display: inline-block;">Aktifkan Akun</a></p>
	<p>Atau salin tautan ini ke browser: %[2]s</p>
	<p>Tautan ini berlaku %[3]s dan hanya dapat dipakai satu kali. Jika kedaluwarsa, minta administrator mengirim undangan baru.</p>
	<p>Jika Anda tidak mengenal Garda Inflasi atau tidak mengharapkan email ini, abaikan saja.</p>
	<hr>
	<p style="color: #6b7280; font-size: 12px;">Garda Inflasi - Sistem Informasi Pengendalian Inflasi Daerah</p>
</body>
</html>
`, html.EscapeString(nama), tautan, durasiTeks(berlaku))

	return s.SendHTML(to, subject, body)
}

// durasiTeks menuliskan masa berlaku dalam bahasa Indonesia: "72 jam", "30 menit".
func durasiTeks(d time.Duration) string {
	if d >= time.Hour {
		return fmt.Sprintf("%d jam", int(d.Round(time.Hour)/time.Hour))
	}
	menit := int(d.Round(time.Minute) / time.Minute)
	if menit < 1 {
		menit = 1
	}
	return fmt.Sprintf("%d menit", menit)
}
