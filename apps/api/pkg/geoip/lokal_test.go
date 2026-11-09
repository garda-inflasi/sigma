package geoip

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestLokal_Klasifikasi(t *testing.T) {
	kasus := []struct {
		nama, ip, want string
	}{
		// IPv4 loopback dan privat RFC 1918: tepi bawah dan atas tiap rentang ikut.
		{"loopback", "127.0.0.1", LabelJaringanLokal},
		{"loopback ujung atas", "127.255.255.255", LabelJaringanLokal},
		{"10/8 awal", "10.0.0.0", LabelJaringanLokal},
		{"10/8 akhir", "10.255.255.255", LabelJaringanLokal},
		{"172.16/12 awal", "172.16.0.1", LabelJaringanLokal},
		{"172.16/12 akhir", "172.31.255.254", LabelJaringanLokal},
		{"192.168/16", "192.168.1.20", LabelJaringanLokal},
		{"link-local IPv4", "169.254.10.10", LabelJaringanLokal},
		{"CGNAT awal", "100.64.0.1", LabelJaringanLokal},
		{"CGNAT akhir", "100.127.255.254", LabelJaringanLokal},

		// Tepat di luar rentang: publik.
		{"9.255.255.255 publik", "9.255.255.255", ""},
		{"11.0.0.0 publik", "11.0.0.0", ""},
		{"172.15.255.255 publik", "172.15.255.255", ""},
		{"172.32.0.0 publik", "172.32.0.0", ""},
		{"192.167.255.255 publik", "192.167.255.255", ""},
		{"192.169.0.0 publik", "192.169.0.0", ""},
		{"169.253.255.255 publik", "169.253.255.255", ""},
		{"169.255.0.0 publik", "169.255.0.0", ""},
		{"100.63.255.255 publik", "100.63.255.255", ""},
		{"100.128.0.0 publik", "100.128.0.0", ""},
		{"8.8.8.8 publik", "8.8.8.8", ""},
		{"dokumentasi TEST-NET bukan jaringan lokal", "203.0.113.7", ""},

		// IPv6.
		{"loopback IPv6", "::1", LabelJaringanLokal},
		{"ULA fc00", "fc00::1", LabelJaringanLokal},
		{"ULA fd", "fd12:3456:789a::1", LabelJaringanLokal},
		{"ULA akhir fdff", "fdff:ffff:ffff:ffff:ffff:ffff:ffff:ffff", LabelJaringanLokal},
		{"link-local fe80", "fe80::1", LabelJaringanLokal},
		{"link-local febf", "febf::1", LabelJaringanLokal},
		{"link-local bernama zona", "fe80::1%eth0", LabelJaringanLokal},
		{"IPv6 publik", "2001:4860:4860::8888", ""},
		{"fe00 bukan link-local", "fe00::1", ""},
		{"fec0 bukan link-local", "fec0::1", ""},
		{"fbff bukan ULA", "fbff::1", ""},
		{"fe00 bukan ULA", "fe00::", ""},

		// IPv4 yang dipetakan ke IPv6 diperlakukan sebagai IPv4.
		{"mapped privat 10/8", "::ffff:10.1.2.3", LabelJaringanLokal},
		{"mapped privat 192.168/16", "::ffff:192.168.0.5", LabelJaringanLokal},
		{"mapped loopback", "::ffff:127.0.0.1", LabelJaringanLokal},
		{"mapped CGNAT", "::ffff:100.64.1.1", LabelJaringanLokal},
		{"mapped publik", "::ffff:8.8.8.8", ""},
		{"mapped heksadesimal privat", "::ffff:0a01:0203", LabelJaringanLokal},

		// Tidak valid atau bukan alamat klien.
		{"kosong", "", ""},
		{"spasi saja", "   ", ""},
		{"teks bebas", "bukan-ip", ""},
		{"host:port", "10.0.0.1:8080", ""},
		{"oktet melewati 255", "10.0.0.256", ""},
		{"kurang oktet", "10.0.0", ""},
		{"CIDR", "10.0.0.0/8", ""},
		{"tak-tentu IPv4", "0.0.0.0", ""},
		{"tak-tentu IPv6", "::", ""},
		{"IPv6 terkurung kurung", "[::1]", ""},
		{"nol di depan ditolak", "010.0.0.1", ""},
	}
	for _, k := range kasus {
		t.Run(k.nama, func(t *testing.T) {
			assert.Equal(t, k.want, Lokal{}.Lokasi(k.ip))
		})
	}
}

func TestLokal_SpasiDiTepiDiabaikan(t *testing.T) {
	assert.Equal(t, LabelJaringanLokal, Lokal{}.Lokasi("  10.0.0.1\n"))
}

func TestLokasi_ResolverNilNonaktif(t *testing.T) {
	assert.Equal(t, "", Lokasi(nil, "10.0.0.1"), "tanpa resolver lokasi tidak ditampilkan")
	assert.Equal(t, LabelJaringanLokal, Lokasi(Lokal{}, "10.0.0.1"))
}
