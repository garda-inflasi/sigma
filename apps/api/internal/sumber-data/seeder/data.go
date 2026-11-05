package seeder

import "github.com/garda-inflasi/api/internal/sumber-data/domain"

type sumberSeed struct {
	ID, Nama, Penyedia, Jenis, Cakupan, JadwalCron string
	RobotsTxtPatuh                                 bool
	Atribusi                                       string
	URLDasar                                       *string
	// Konfigurasi awal (JSON); kosong = bawaan basis data ({}). Hanya dipakai saat
	// sumber pertama kali dibuat: SeedReference tidak menimpa konfigurasi hasil operasi.
	Konfigurasi string
}

// katalog meniru mockSumber di web (id sama). Jadwal dalam cron WIB.
var katalog = []sumberSeed{
	{ID: "SRC-BPS", Nama: "BPS WebAPI — IHK & inflasi", Penyedia: "BPS", Jenis: domain.JenisAPI, Cakupan: "Provinsi & 3 kota IHK",
		JadwalCron: "0 6 * * *", RobotsTxtPatuh: true, Atribusi: "Sumber: Badan Pusat Statistik (BPS), webapi.bps.go.id",
		URLDasar: new("https://webapi.bps.go.id/v1/api"),
		// 1100 = domain BPS Provinsi Aceh. Variabel IHK/inflasi wajib diisi operator (lihat konektor BPS).
		Konfigurasi: `{"domain":"1100"}`},
	{ID: "SRC-BI", Nama: "PIHPS Nasional — harga pangan", Penyedia: "Bank Indonesia", Jenis: domain.JenisAPI, Cakupan: "Pasar tradisional & modern",
		JadwalCron: "0 10 * * *", RobotsTxtPatuh: true, Atribusi: "Sumber: Pusat Informasi Harga Pangan Strategis Nasional (PIHPS), Bank Indonesia",
		URLDasar: new("https://www.bi.go.id/hargapangan")},
	{ID: "SRC-DJPB", Nama: "Kanwil DJPb — realisasi belanja", Penyedia: "Kanwil DJPb Aceh", Jenis: domain.JenisAPI, Cakupan: "APBD & BTT 23 kab/kota",
		JadwalCron: "0 7 * * 1", RobotsTxtPatuh: true, Atribusi: "Sumber: Kanwil Direktorat Jenderal Perbendaharaan Provinsi Aceh"},
	{ID: "SRC-BMKG", Nama: "BMKG — prakiraan & curah hujan", Penyedia: "BMKG", Jenis: domain.JenisAPI, Cakupan: "23 kab/kota",
		JadwalCron: "0 */3 * * *", RobotsTxtPatuh: true, Atribusi: "Sumber: Badan Meteorologi, Klimatologi, dan Geofisika (BMKG)",
		URLDasar: new("https://api.bmkg.go.id")},
	{ID: "SRC-OWM", Nama: "OpenWeather — cuaca aktual", Penyedia: "OpenWeather", Jenis: domain.JenisAPI, Cakupan: "Titik sentra produksi",
		JadwalCron: "0 * * * *", RobotsTxtPatuh: true, Atribusi: "Weather data provided by OpenWeather",
		URLDasar: new("https://api.openweathermap.org/data/2.5")},
	{ID: "SRC-BAPANAS", Nama: "Panel Harga Bapanas", Penyedia: "Badan Pangan Nasional", Jenis: domain.JenisScraper, Cakupan: "Harga produsen & konsumen",
		JadwalCron: "0 11 * * *", RobotsTxtPatuh: true, Atribusi: "Sumber: Panel Harga Pangan, Badan Pangan Nasional",
		URLDasar: new("https://panelharga.badanpangan.go.id")},
	{ID: "SRC-PORTAL", Nama: "Portal harga Disperindag Aceh", Penyedia: "Pemerintah Aceh", Jenis: domain.JenisScraper, Cakupan: "Pasar induk provinsi",
		JadwalCron: "0 12 * * *", RobotsTxtPatuh: true, Atribusi: "Sumber: Dinas Perindustrian dan Perdagangan Aceh"},
	{ID: "SRC-CS", Nama: "Telemetri cold storage", Penyedia: "Sensor IoT fasilitas", Jenis: domain.JenisAPI, Cakupan: "3 fasilitas operasional",
		JadwalCron: "*/5 * * * *", RobotsTxtPatuh: true, Atribusi: "Sumber: sensor suhu fasilitas cold storage mitra"},
}

type demoSeed struct {
	Status         string
	MenitLalu      int
	Keberhasilan7h float64
	LatensiMs      int
}

// demoKeadaan meniru nilai mockSumber.
var demoKeadaan = map[string]demoSeed{
	"SRC-BPS":     {Status: domain.StatusSehat, MenitLalu: 142, Keberhasilan7h: 100, LatensiMs: 640},
	"SRC-BI":      {Status: domain.StatusSehat, MenitLalu: 38, Keberhasilan7h: 98.6, LatensiMs: 910},
	"SRC-DJPB":    {Status: domain.StatusSehat, MenitLalu: 2880, Keberhasilan7h: 100, LatensiMs: 1220},
	"SRC-BMKG":    {Status: domain.StatusSehat, MenitLalu: 51, Keberhasilan7h: 99.1, LatensiMs: 480},
	"SRC-OWM":     {Status: domain.StatusSehat, MenitLalu: 12, Keberhasilan7h: 99.8, LatensiMs: 210},
	"SRC-BAPANAS": {Status: domain.StatusTertunda, MenitLalu: 325, Keberhasilan7h: 86.4, LatensiMs: 3400},
	"SRC-PORTAL":  {Status: domain.StatusGagal, MenitLalu: 1460, Keberhasilan7h: 57.1, LatensiMs: 0},
	"SRC-CS":      {Status: domain.StatusSehat, MenitLalu: 3, Keberhasilan7h: 99.97, LatensiMs: 95},
}
