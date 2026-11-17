package usecase

import "time"

// liburNasionalTetap adalah daftar tanggal (bulan-hari) hari libur nasional tetap di Indonesia.
var liburNasionalTetap = map[string]bool{
	"01-01": true, // Tahun Baru Masehi
	"05-01": true, // Hari Buruh Internasional
	"06-01": true, // Hari Lahir Pancasila
	"08-17": true, // Hari Kemerdekaan RI
	"12-25": true, // Hari Raya Natal
}

// AdalahHariLibur menandai hari libur nasional tetap dan hari Minggu sebagai penanda fitur kalender ML.
func AdalahHariLibur(t time.Time) bool {
	if t.Weekday() == time.Sunday {
		return true
	}
	return liburNasionalTetap[t.Format("01-02")]
}
