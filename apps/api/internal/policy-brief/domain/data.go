package domain

import "time"

// DataPeriode adalah data mentah (angka dari basis data) yang dikumpulkan untuk satu
// brief; BangunKonteks mengubahnya menjadi Konteks. Bagian yang datanya tidak ada nil/kosong.
type DataPeriode struct {
	// Periode adalah awal bulan brief.
	Periode  time.Time
	Inflasi  *InflasiData
	Target   *TargetData
	Andil    []AndilData
	Harga    []HargaData
	Ews      EwsData
	Proyeksi []ProyeksiData
}

// InflasiData adalah inflasi provinsi pada bulan data terbaru yang tidak melewati periode brief.
type InflasiData struct {
	Periode time.Time
	Mtm     float64
	Yoy     float64
	Ytd     float64
	Sumber  string
}

// TargetData adalah koridor sasaran inflasi tahun data inflasi.
type TargetData struct {
	Tahun int
	Bawah float64
	Atas  float64
}

// AndilData adalah sumbangan satu komoditas terhadap inflasi (poin persen).
type AndilData struct {
	Komoditas string
	Andil     float64
	Sumber    string
}

// HargaData adalah rata-rata harga konsumen tervalidasi satu komoditas pada bulan brief
// dibanding bulan sebelumnya.
type HargaData struct {
	Komoditas    string
	RataIni      int
	RataLalu     int
	PerubahanPct float64
}

// EwsData merangkum peringatan dini yang dibuat pada bulan brief.
type EwsData struct {
	Total, Kritis, Siaga, Waspada int
	// Teratas adalah peringatan terparah (maksimal tiga).
	Teratas []AlertData
}

// AlertData adalah satu peringatan dini terparah.
type AlertData struct {
	Komoditas string
	Wilayah   string
	Level     string
}

// ProyeksiData adalah proyeksi harga terbaru model produksi satu komoditas.
type ProyeksiData struct {
	Komoditas    string
	HorizonHari  int
	PerubahanPct float64
	// Sumber adalah sumber proyeksi: "MODEL" (hasil pelatihan) atau "DEMO".
	Sumber string
}
