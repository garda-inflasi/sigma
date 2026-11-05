package dto

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestRingkasPerangkat(t *testing.T) {
	cases := []struct{ nama, ua, want string }{
		{"Chrome macOS", "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0.0.0 Safari/537.36", "Chrome di macOS"},
		{"Chrome Windows", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0.0.0 Safari/537.36", "Chrome di Windows"},
		{"Edge Windows", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0.0.0 Safari/537.36 Edg/120.0.0.0", "Edge di Windows"},
		{"Firefox Linux", "Mozilla/5.0 (X11; Linux x86_64; rv:121.0) Gecko/20100101 Firefox/121.0", "Firefox di Linux"},
		{"Safari iPhone", "Mozilla/5.0 (iPhone; CPU iPhone OS 17_2 like Mac OS X) AppleWebKit/605.1.15 (KHTML, like Gecko) Version/17.2 Mobile/15E148 Safari/604.1", "Safari di iOS"},
		{"Chrome Android", "Mozilla/5.0 (Linux; Android 14; Pixel 8) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0.0.0 Mobile Safari/537.36", "Chrome di Android"},
		{"Chrome iOS", "Mozilla/5.0 (iPhone; CPU iPhone OS 17_2 like Mac OS X) AppleWebKit/605.1.15 (KHTML, like Gecko) CriOS/120.0.6099.119 Mobile/15E148 Safari/604.1", "Chrome di iOS"},
		{"Opera", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0.0.0 Safari/537.36 OPR/106.0.0.0", "Opera di Windows"},
		{"curl", "curl/8.4.0", "curl"},
		{"Node", "node", "Node.js"},
		{"kosong", "", "Perangkat tidak dikenal"},
		{"spasi", "   ", "Perangkat tidak dikenal"},
		{"tak dikenal", "Zzz/1.0", "Perangkat tidak dikenal"},
		{"hanya OS", "(Windows NT 10.0)", "Windows"},
	}
	for _, tc := range cases {
		t.Run(tc.nama, func(t *testing.T) {
			assert.Equal(t, tc.want, RingkasPerangkat(tc.ua))
		})
	}
}
