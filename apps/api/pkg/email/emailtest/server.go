// Package emailtest menyediakan server SMTP palsu di loopback untuk tes modul yang mengirim email: pesan yang
// diterima ditangkap dan bisa diperiksa (penerima, isi, tautan), tanpa jaringan luar.
package emailtest

import (
	"bufio"
	"net"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/garda-inflasi/api/pkg/email"
	"github.com/stretchr/testify/require"
)

// Message adalah satu email yang diterima server.
type Message struct {
	// To adalah penerima amplop (RCPT TO), huruf apa adanya.
	To []string
	// Data adalah isi mentah pesan (header dan badan).
	Data string
}

// Server adalah server SMTP minimal (tanpa AUTH, seperti relay Mailpit) yang menangkap pesan.
type Server struct {
	addr string

	mu       sync.Mutex
	messages []Message
	// tolakPenerima, bila diisi, membuat RCPT TO dijawab 550 (simulasi SMTP yang menolak).
	tolakPenerima bool
	// diam, bila true, membuat server menerima koneksi tetapi tidak pernah menyapa (SMTP yang macet).
	diam bool
	// aktif adalah koneksi yang sedang dilayani; maks adalah puncaknya.
	aktif, maks int
	// conns adalah koneksi yang pernah diterima; ditutup saat tes selesai agar koneksi macet tidak menggantung.
	conns []net.Conn
}

// Start menyalakan server pada port acak loopback; dimatikan otomatis saat tes selesai.
func Start(t testing.TB) *Server {
	t.Helper()
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)
	s := &Server{addr: ln.Addr().String()}
	t.Cleanup(func() {
		_ = ln.Close()
		s.mu.Lock()
		defer s.mu.Unlock()
		for _, c := range s.conns {
			_ = c.Close()
		}
	})
	go func() {
		for {
			conn, err := ln.Accept()
			if err != nil {
				return
			}
			s.mu.Lock()
			s.conns = append(s.conns, conn)
			s.mu.Unlock()
			go s.layani(conn)
		}
	}()
	return s
}

// Config adalah konfigurasi pengirim yang mengarah ke server ini; baseURL dipakai untuk tautan di email.
func (s *Server) Config(baseURL string) email.Config {
	host, portStr, err := net.SplitHostPort(s.addr)
	if err != nil {
		panic(err)
	}
	port, err := strconv.Atoi(portStr)
	if err != nil {
		panic(err)
	}
	return email.Config{Host: host, Port: port, From: "noreply@garda.test", FromName: "Garda Inflasi", BaseURL: baseURL}
}

// Host dan Port memudahkan tes yang mengatur SMTP_HOST dan SMTP_PORT lewat lingkungan.
func (s *Server) Host() string { h, _, _ := net.SplitHostPort(s.addr); return h }

// Port adalah port TCP server.
func (s *Server) Port() string { _, p, _ := net.SplitHostPort(s.addr); return p }

// TolakPenerima membuat server menjawab 550 untuk setiap RCPT TO (SMTP yang menolak kirim) bila tolak true.
func (s *Server) TolakPenerima(tolak bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.tolakPenerima = tolak
}

// Diam membuat server menerima koneksi tetapi tidak pernah menjawab (SMTP yang macet) bila diam true.
func (s *Server) Diam(diam bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.diam = diam
}

// MaksKoneksiSerempak adalah puncak koneksi yang dilayani bersamaan sejak server menyala.
func (s *Server) MaksKoneksiSerempak() int {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.maks
}

// Messages mengembalikan salinan pesan yang sudah diterima, berurutan.
func (s *Server) Messages() []Message {
	s.mu.Lock()
	defer s.mu.Unlock()
	return append([]Message(nil), s.messages...)
}

// Untuk mengembalikan pesan yang dikirim ke penerima (tak peka huruf besar-kecil).
func (s *Server) Untuk(penerima string) []Message {
	var out []Message
	for _, m := range s.Messages() {
		for _, to := range m.To {
			if strings.EqualFold(to, penerima) {
				out = append(out, m)
				break
			}
		}
	}
	return out
}

// Tunggu menunggu sampai ada setidaknya n pesan untuk penerima (pengiriman bisa di latar belakang) dan
// mengembalikannya; gagal bila batas waktu lewat.
func (s *Server) Tunggu(t testing.TB, penerima string, n int) []Message {
	t.Helper()
	require.Eventually(t, func() bool { return len(s.Untuk(penerima)) >= n }, 5*time.Second, 10*time.Millisecond,
		"email untuk %s tidak sampai (diterima %d dari %d)", penerima, len(s.Untuk(penerima)), n)
	return s.Untuk(penerima)
}

var polaToken = regexp.MustCompile(`[?&]token=([A-Za-z0-9%._~-]+)`)

// Token mengambil token dari tautan `?token=` pertama di isi pesan.
func (m Message) Token() string {
	cocok := polaToken.FindStringSubmatch(m.Data)
	if cocok == nil {
		return ""
	}
	return cocok[1]
}

func (s *Server) layani(conn net.Conn) {
	s.mu.Lock()
	s.aktif++
	s.maks = max(s.maks, s.aktif)
	diam := s.diam
	s.mu.Unlock()
	// akhiri mengurangi hitungan koneksi sekali saja. Pada QUIT hitungan dikurangi SEBELUM "221" ditulis, jadi
	// klien yang membuka koneksi berikutnya setelah membaca "221" tidak pernah terhitung bersamaan dengan yang lama.
	akhiri := sync.OnceFunc(func() {
		s.mu.Lock()
		s.aktif--
		s.mu.Unlock()
	})
	defer func() {
		akhiri()
		_ = conn.Close()
	}()
	_ = conn.SetDeadline(time.Now().Add(10 * time.Second))
	if diam {
		// Tidak menyapa: baca sampai klien menutup atau tenggat habis.
		_, _ = conn.Read(make([]byte, 1))
		return
	}
	r := bufio.NewReader(conn)
	tulis := func(b string) { _, _ = conn.Write([]byte(b + "\r\n")) }
	tulis("220 palsu ESMTP")
	var penerima []string
	for {
		baris, err := r.ReadString('\n')
		if err != nil {
			return
		}
		teks := strings.TrimSpace(baris)
		perintah := strings.ToUpper(teks)
		switch {
		case strings.HasPrefix(perintah, "EHLO"), strings.HasPrefix(perintah, "HELO"):
			tulis("250 palsu")
		case strings.HasPrefix(perintah, "MAIL FROM"):
			penerima = nil
			tulis("250 ok")
		case strings.HasPrefix(perintah, "RCPT TO"):
			s.mu.Lock()
			tolak := s.tolakPenerima
			s.mu.Unlock()
			if tolak {
				tulis("550 penerima ditolak")
				continue
			}
			penerima = append(penerima, alamatRcpt(teks))
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
			s.messages = append(s.messages, Message{To: penerima, Data: sb.String()})
			s.mu.Unlock()
			tulis("250 diterima")
		case perintah == "QUIT":
			akhiri()
			tulis("221 bye")
			return
		default:
			tulis("250 ok")
		}
	}
}

// alamatRcpt mengambil alamat dari "RCPT TO:<a@b>".
func alamatRcpt(baris string) string {
	_, sisa, _ := strings.Cut(baris, ":")
	return strings.Trim(strings.TrimSpace(sisa), "<>")
}
