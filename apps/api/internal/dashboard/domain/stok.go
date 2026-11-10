package domain

import "time"

// Kelas lot pada ringkasan stok. Satu lot (yang belum HABIS) jatuh tepat ke satu kelas, dinilai menurut hari ini
// waktu Aceh dengan aturan yang sama dengan KPI cs (migrasi 000124):
//   - LUAR_OPERASIONAL: fasilitasnya bukan OPERASIONAL (perawatan). Tidak ikut stok maupun kapasitas, seperti KPI cs.
//   - KEDALUWARSA: di fasilitas OPERASIONAL, tanggal kedaluwarsanya hari ini atau sebelumnya. Masih memenuhi gudang
//     (halaman cold storage menghitungnya) tetapi bukan stok yang berlaku.
//   - BELUM_MASUK: di fasilitas OPERASIONAL, tanggal masuknya setelah hari ini (data yang belum seharusnya ada).
//   - BERLAKU: di fasilitas OPERASIONAL, masuk paling lambat hari ini dan kedaluwarsa setelah hari ini.
const (
	KelasStokBerlaku         = "BERLAKU"
	KelasStokKedaluwarsa     = "KEDALUWARSA"
	KelasStokLuarOperasional = "LUAR_OPERASIONAL"
	KelasStokBelumMasuk      = "BELUM_MASUK"
)

// StokQuery adalah permintaan fakta stok. Batas waktu dihitung usecase dari hari ini waktu Aceh.
type StokQuery struct {
	// Cakupan membatasi fasilitas menurut kabupatennya (02-B); nilai nol tertutup: tidak ada baris.
	Cakupan Cakupan
	// KabupatenKode dan KomoditasKode adalah filter permintaan, digabung (AND) dengan Cakupan. Kosong: tanpa filter.
	// Filter komoditas hanya menyaring lot, tidak kapasitas fasilitas.
	KabupatenKode string
	KomoditasKode string

	// Batas adalah tengah malam waktu Aceh SESUDAH hari ini. Lot berlaku bila masuk_pada < Batas dan
	// kedaluwarsa_pada >= Batas, yang setara dengan tanggal masuk <= hari ini dan tanggal kedaluwarsa > hari ini.
	Batas time.Time
	// BatasDekat adalah tengah malam waktu Aceh setelah hari ini ditambah AmbangMendekatiKedaluwarsaHari: lot
	// berlaku yang kedaluwarsa_pada < BatasDekat mendekati kedaluwarsa.
	BatasDekat time.Time
}

// BarisStok adalah fakta stok pada grain komoditas x fasilitas x kelas (satu baris per kombinasi yang punya lot).
// Usecase menjumlahkannya menjadi ringkasan dan per komoditas, jadi jumlah-jumlah itu pasti cocok satu sama lain dan
// jumlah fasilitas berbeda dihitung dari himpunan yang sama.
type BarisStok struct {
	KomoditasKode string  `gorm:"column:komoditas_kode"`
	KomoditasNama string  `gorm:"column:komoditas_nama"`
	FasilitasID   string  `gorm:"column:fasilitas_id"`
	Kelas         string  `gorm:"column:kelas"`
	JumlahLot     int     `gorm:"column:jumlah_lot"`
	VolumeTon     float64 `gorm:"column:volume_ton"`
	// MendekatiTon dan RisikoTinggiTon adalah bagian VolumeTon yang mendekati kedaluwarsa dan berisiko tinggi. Hanya
	// bermakna pada kelas BERLAKU.
	MendekatiTon    float64 `gorm:"column:mendekati_ton"`
	RisikoTinggiTon float64 `gorm:"column:risiko_tinggi_ton"`
}

// KapasitasStok adalah kapasitas fasilitas OPERASIONAL dalam cakupan (tidak dipengaruhi filter komoditas).
type KapasitasStok struct {
	Fasilitas int   `gorm:"column:fasilitas"`
	Ton       int64 `gorm:"column:ton"`
}

// FaktaStok adalah hasil satu bacaan konsisten (satu snapshot) dari repository.
type FaktaStok struct {
	Baris     []BarisStok
	Kapasitas KapasitasStok
}

// StokKomoditas adalah stok berlaku satu komoditas. Tonase dibulatkan satu desimal dari jumlah yang belum dibulatkan.
type StokKomoditas struct {
	KomoditasKode           string
	KomoditasNama           string
	TotalTon                float64
	JumlahLot               int
	JumlahFasilitas         int
	MendekatiKedaluwarsaTon float64
	RisikoTinggiTon         float64
}

// RingkasanStok adalah stok seluruh cakupan dan filter.
//
// TotalTon dan UtilisasiPct memakai definisi KPI cs: lot berlaku di fasilitas OPERASIONAL terhadap kapasitas fasilitas
// OPERASIONAL (100 x TotalTon / KapasitasTon; 0 tanpa kapasitas). Dengan filter komoditas pembilangnya hanya komoditas
// itu sedangkan KapasitasTon tetap seluruh fasilitas. Dua kelas lot lain dilaporkan terpisah supaya angka dashboard
// dapat direkonsiliasi dengan halaman cold storage: TotalTon + KedaluwarsaTon + DiLuarOperasionalTon (+ lot yang
// belum masuk, yang tidak ada pada data wajar) sama dengan total volume lot yang belum habis di sana.
type RingkasanStok struct {
	TotalTon                float64
	JumlahLot               int
	JumlahFasilitas         int
	KapasitasTon            int64
	FasilitasOperasional    int
	UtilisasiPct            float64
	MendekatiKedaluwarsaTon float64
	RisikoTinggiTon         float64
	KedaluwarsaLot          int
	KedaluwarsaTon          float64
	DiLuarOperasionalTon    float64
}

// Stok adalah hasil GET /stok.
type Stok struct {
	// Tanggal adalah hari ini menurut waktu Aceh (DATE): acuan lot berlaku.
	Tanggal   time.Time
	Ringkasan RingkasanStok
	// Komoditas hanya memuat komoditas yang punya stok berlaku; terbesar lebih dulu, tidak pernah nil.
	Komoditas []StokKomoditas
}
