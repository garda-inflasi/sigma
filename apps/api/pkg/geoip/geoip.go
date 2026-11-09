// Package geoip memperkirakan lokasi dari alamat IP (daftar sesi dan riwayat aktivitas akun).
//
// Dasar UU PDP: alamat IP pengguna tidak pernah dikirim ke pihak ketiga. Lokasi hanya ditentukan secara lokal,
// dari klasifikasi alamat jaringan privat (tanpa data apa pun) dan, bila operator menyediakannya, basis data
// MMDB yang dibaca dari disk (MaxMind GeoLite2-City atau DB-IP City Lite; formatnya sama). Tanpa basis data,
// alamat publik tidak diberi lokasi.
package geoip

import (
	"net/netip"
	"strings"
)

// Resolver memperkirakan lokasi dari satu alamat IP. Implementasinya aman dipanggil dari banyak goroutine.
type Resolver interface {
	// Lokasi mengembalikan label lokasi ("Jaringan lokal", atau "Kota, Provinsi, Negara" dari basis data),
	// atau string kosong bila IP kosong, tidak valid, atau lokasinya tidak diketahui.
	Lokasi(ip string) string
}

// LabelJaringanLokal adalah label alamat jaringan privat: tidak punya lokasi geografis, tetapi memberi tahu
// pengguna bahwa sesi itu berasal dari dalam jaringan (mis. kantor atau VPN), bukan dari internet.
const LabelJaringanLokal = "Jaringan lokal"

// Lokasi memanggil r.Lokasi dengan aman terhadap r nil (fitur lokasi nonaktif).
func Lokasi(r Resolver, ip string) string {
	if r == nil {
		return ""
	}
	return r.Lokasi(ip)
}

// bacaIP mengurai ip menjadi alamat yang siap diklasifikasi: spasi di tepi dan zona IPv6 dibuang, dan alamat
// IPv4 yang dipetakan ke IPv6 (::ffff:a.b.c.d) diperlakukan sebagai IPv4. false untuk teks kosong, bukan alamat
// IP (termasuk "host:port"), dan alamat tak-tentu (0.0.0.0, ::), yang bukan asal permintaan mana pun.
func bacaIP(ip string) (netip.Addr, bool) {
	addr, err := netip.ParseAddr(strings.TrimSpace(ip))
	if err != nil {
		return netip.Addr{}, false
	}
	addr = addr.WithZone("").Unmap()
	if addr.IsUnspecified() {
		return netip.Addr{}, false
	}
	return addr, true
}
