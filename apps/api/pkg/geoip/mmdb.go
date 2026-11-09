package geoip

import (
	"fmt"
	"strings"

	"github.com/oschwald/maxminddb-golang/v2"
)

// bahasaNama adalah urutan bahasa nama tempat yang dipilih: Indonesia lebih dulu, lalu Inggris. MaxMind
// GeoLite2 hanya memuat de, en, es, fr, ja, pt-BR, ru, dan zh-CN, jadi umumnya yang terpakai adalah "en".
var bahasaNama = []string{"id", "en"}

// namaTempat memuat nama berbahasa-bahasa untuk satu tempat (kota, subdivisi, negara) pada rekaman MMDB.
type namaTempat struct {
	Names map[string]string `maxminddb:"names"`
}

// rekamanLokasi adalah bagian rekaman GeoLite2/GeoIP2-City dan DB-IP City yang dibaca: kota, subdivisi
// (urut dari yang paling umum, mis. provinsi), dan negara.
type rekamanLokasi struct {
	City         namaTempat   `maxminddb:"city"`
	Subdivisions []namaTempat `maxminddb:"subdivisions"`
	Country      namaTempat   `maxminddb:"country"`
}

// MMDB memperkirakan lokasi dari basis data MaxMind DB lokal (GeoLite2-City, DB-IP City Lite, atau
// padanannya). Berkas dipetakan ke memori dan hanya dibaca, sehingga aman dipakai banyak goroutine.
type MMDB struct {
	db *maxminddb.Reader
}

// BukaMMDB membuka basis data di path. Galat bila berkas tidak ada, kosong, bukan MMDB, atau jenisnya bukan
// basis data lokasi (kota atau negara), mis. basis data ASN yang tidak pernah menghasilkan nama tempat.
func BukaMMDB(path string) (*MMDB, error) {
	db, err := maxminddb.Open(path)
	if err != nil {
		return nil, fmt.Errorf("buka basis data lokasi %q: %w", path, err)
	}
	if !jenisBasisLokasi(db.Metadata.DatabaseType) {
		_ = db.Close()
		return nil, fmt.Errorf("basis data %q berjenis %q, bukan basis data lokasi (kota atau negara)", path, db.Metadata.DatabaseType)
	}
	return &MMDB{db: db}, nil
}

// jenisBasisLokasi: jenis basis data MaxMind/DB-IP yang memuat nama tempat ("GeoLite2-City", "GeoIP2-Country",
// "DBIP-City-Lite", "DBIP-Location (compat=City)", dst.).
func jenisBasisLokasi(jenis string) bool {
	jenis = strings.ToLower(jenis)
	return strings.Contains(jenis, "city") || strings.Contains(jenis, "country") || strings.Contains(jenis, "location")
}

// Tutup melepaskan pemetaan berkas. Tidak boleh dipanggil bersamaan dengan Lokasi.
func (m *MMDB) Tutup() error { return m.db.Close() }

// Lokasi memenuhi Resolver: "Kota, Provinsi, Negara" tanpa bagian kosong atau ganda ("Negara" saja bila hanya
// negaranya yang diketahui); kosong bila IP tidak valid atau tidak ada di basis data.
func (m *MMDB) Lokasi(ip string) string {
	addr, ok := bacaIP(ip)
	if !ok {
		return ""
	}
	hasil := m.db.Lookup(addr)
	if hasil.Err() != nil || !hasil.Found() {
		return ""
	}
	var rek rekamanLokasi
	if err := hasil.Decode(&rek); err != nil {
		return ""
	}
	return rek.label()
}

// label menyusun "Kota, Provinsi, Negara".
func (r rekamanLokasi) label() string {
	bagian := []string{namaTerbaik(r.City.Names)}
	for _, s := range r.Subdivisions {
		// Subdivisi diurutkan dari yang paling umum: yang pertama bernama dipakai (provinsi atau negara bagian).
		if nama := namaTerbaik(s.Names); nama != "" {
			bagian = append(bagian, nama)
			break
		}
	}
	bagian = append(bagian, namaTerbaik(r.Country.Names))
	return gabungTanpaGanda(bagian)
}

// namaTerbaik memilih nama menurut bahasaNama; kosong bila tidak ada satu pun.
func namaTerbaik(nama map[string]string) string {
	for _, bahasa := range bahasaNama {
		if n := strings.TrimSpace(nama[bahasa]); n != "" {
			return n
		}
	}
	return ""
}

// gabungTanpaGanda menggabungkan bagian dengan ", " tanpa bagian kosong dan tanpa bagian yang sama (tak
// peka huruf besar-kecil) dengan bagian sebelumnya: "Singapore, Singapore" menjadi "Singapore".
func gabungTanpaGanda(bagian []string) string {
	var keluar []string
	for _, b := range bagian {
		b = strings.TrimSpace(b)
		if b == "" || adaBagian(keluar, b) {
			continue
		}
		keluar = append(keluar, b)
	}
	return strings.Join(keluar, ", ")
}

func adaBagian(daftar []string, b string) bool {
	for _, d := range daftar {
		if strings.EqualFold(d, b) {
			return true
		}
	}
	return false
}
