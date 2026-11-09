package geoip

import "net/netip"

// jaringanLokal adalah rentang alamat yang tidak dialamatkan di internet publik: pemiliknya berada di dalam
// jaringan yang sama dengan server (kantor, VPN, atau jaringan Docker), jadi basis data lokasi tidak punya
// jawaban untuknya.
var jaringanLokal = []netip.Prefix{
	netip.MustParsePrefix("127.0.0.0/8"),    // loopback IPv4
	netip.MustParsePrefix("10.0.0.0/8"),     // privat RFC 1918
	netip.MustParsePrefix("172.16.0.0/12"),  // privat RFC 1918
	netip.MustParsePrefix("192.168.0.0/16"), // privat RFC 1918
	netip.MustParsePrefix("169.254.0.0/16"), // link-local IPv4
	netip.MustParsePrefix("100.64.0.0/10"),  // CGNAT (RFC 6598)
	netip.MustParsePrefix("::1/128"),        // loopback IPv6
	netip.MustParsePrefix("fc00::/7"),       // ULA IPv6 (RFC 4193)
	netip.MustParsePrefix("fe80::/10"),      // link-local IPv6
}

// Lokal mengenali alamat jaringan lokal tanpa basis data apa pun: hasilnya LabelJaringanLokal, selain itu kosong.
type Lokal struct{}

// Lokasi memenuhi Resolver.
func (Lokal) Lokasi(ip string) string {
	addr, ok := bacaIP(ip)
	if !ok {
		return ""
	}
	for _, p := range jaringanLokal {
		if p.Contains(addr) {
			return LabelJaringanLokal
		}
	}
	return ""
}
