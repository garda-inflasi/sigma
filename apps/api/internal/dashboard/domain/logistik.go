package domain

import (
	"time"

	"github.com/garda-inflasi/api/pkg/idx"
)

// LogistikQuery adalah permintaan pergerakan logistik. Jendela adalah [Awal, Akhir) menurut waktu Aceh
// (JendelaOperasionalHari hari kalender yang berakhir hari ini).
type LogistikQuery struct {
	// Cakupan membatasi pergerakan dan rute menurut kabupaten (02-B): masuk cakupan bila fasilitas asal ATAU tujuan
	// (rute: kabupaten asal ATAU tujuan) berada di kabupaten itu. Nilai nol tertutup: tidak ada baris.
	Cakupan Cakupan
	// KabupatenKode dan KomoditasKode adalah filter permintaan, digabung (AND) dengan Cakupan. Kosong: tanpa filter.
	KabupatenKode string
	KomoditasKode string

	Awal, Akhir time.Time
	// MaksTerbaru dan MaksRute membatasi jumlah baris daftar pergerakan terbaru dan rute yang dikembalikan.
	MaksTerbaru int
	MaksRute    int
}

// GerakanAgregat adalah jumlah dan tonase transaksi stok satu komoditas dan satu jenis dalam jendela.
type GerakanAgregat struct {
	KomoditasKode string  `gorm:"column:komoditas_kode"`
	KomoditasNama string  `gorm:"column:komoditas_nama"`
	Jenis         string  `gorm:"column:jenis"`
	Jumlah        int     `gorm:"column:jumlah"`
	VolumeTon     float64 `gorm:"column:volume_ton"`
}

// Pergerakan adalah satu transaksi stok beserta fasilitas asal dan tujuannya. Tidak membawa pelaku (oleh_id) maupun
// alasan bebasnya: keduanya bisa memuat data pribadi dan tidak dibutuhkan ringkasan.
//
// MASUK hanya punya tujuan, KELUAR hanya asal, PINDAH keduanya (dijaga CHECK chk_transaksi_stok_lokasi).
type Pergerakan struct {
	ID            idx.ID    `gorm:"column:id"`
	Waktu         time.Time `gorm:"column:waktu"`
	Jenis         string    `gorm:"column:jenis"`
	KomoditasKode string    `gorm:"column:komoditas_kode"`
	KomoditasNama string    `gorm:"column:komoditas_nama"`
	VolumeTon     float64   `gorm:"column:volume_ton"`

	AsalID            *string `gorm:"column:asal_id"`
	AsalKode          *string `gorm:"column:asal_kode"`
	AsalNama          *string `gorm:"column:asal_nama"`
	AsalKabupatenKode *string `gorm:"column:asal_kabupaten_kode"`
	AsalKabupatenNama *string `gorm:"column:asal_kabupaten_nama"`

	TujuanID            *string `gorm:"column:tujuan_id"`
	TujuanKode          *string `gorm:"column:tujuan_kode"`
	TujuanNama          *string `gorm:"column:tujuan_nama"`
	TujuanKabupatenKode *string `gorm:"column:tujuan_kabupaten_kode"`
	TujuanKabupatenNama *string `gorm:"column:tujuan_kabupaten_nama"`
}

// FasilitasRingkas adalah fasilitas cold storage pada sebuah pergerakan.
type FasilitasRingkas struct {
	ID            string
	Kode          *string
	Nama          string
	KabupatenKode string
	KabupatenNama string
}

// Asal mengembalikan fasilitas asal, nil bila tidak ada (MASUK).
func (p Pergerakan) Asal() *FasilitasRingkas {
	return fasilitasRingkas(p.AsalID, p.AsalKode, p.AsalNama, p.AsalKabupatenKode, p.AsalKabupatenNama)
}

// Tujuan mengembalikan fasilitas tujuan, nil bila tidak ada (KELUAR).
func (p Pergerakan) Tujuan() *FasilitasRingkas {
	return fasilitasRingkas(p.TujuanID, p.TujuanKode, p.TujuanNama, p.TujuanKabupatenKode, p.TujuanKabupatenNama)
}

func fasilitasRingkas(id, kode, nama, kabKode, kabNama *string) *FasilitasRingkas {
	if id == nil {
		return nil
	}
	f := &FasilitasRingkas{ID: *id, Kode: kode}
	if nama != nil {
		f.Nama = *nama
	}
	if kabKode != nil {
		f.KabupatenKode = *kabKode
	}
	if kabNama != nil {
		f.KabupatenNama = *kabNama
	}
	return f
}

// RuteAktif adalah satu rute distribusi yang masih berlaku (status selain DIGANTIKAN). Rute menghubungkan kabupaten
// (bukan fasilitas), jadi cakupannya mengikuti kabupaten asal atau tujuan.
type RuteAktif struct {
	ID            idx.ID    `gorm:"column:id"`
	Kode          *string   `gorm:"column:kode"`
	AsalKode      string    `gorm:"column:asal_kode"`
	Asal          string    `gorm:"column:asal"`
	TujuanKode    string    `gorm:"column:tujuan_kode"`
	Tujuan        string    `gorm:"column:tujuan"`
	KomoditasKode string    `gorm:"column:komoditas_kode"`
	KomoditasNama string    `gorm:"column:komoditas_nama"`
	VolumeTon     float64   `gorm:"column:volume_ton"`
	JarakKm       int       `gorm:"column:jarak_km"`
	JamTempuh     float64   `gorm:"column:jam_tempuh"`
	Biaya         int64     `gorm:"column:biaya"`
	Status        string    `gorm:"column:status"`
	Sumber        string    `gorm:"column:sumber"`
	DibuatPada    time.Time `gorm:"column:dibuat_pada"`
}

// RuteRingkas adalah rute aktif dalam cakupan: jumlah dan total atas SEMUA rute, Items hanya yang terbesar
// biayanya (batas MaksRute), terurut menurut biaya menurun lalu ID, seperti daftar rute modul spasial.
type RuteRingkas struct {
	Jumlah    int
	VolumeTon float64
	Biaya     int64
	Items     []RuteAktif
}

// LogistikFakta adalah hasil satu bacaan konsisten (satu snapshot) dari repository.
type LogistikFakta struct {
	Agregat []GerakanAgregat
	Terbaru []Pergerakan
	Rute    RuteRingkas
}

// GerakanJenis adalah jumlah transaksi dan tonasenya untuk satu jenis. Tonase dibulatkan satu desimal.
type GerakanJenis struct {
	Jumlah    int
	VolumeTon float64
}

// GerakanKomoditas adalah pergerakan satu komoditas per jenis; ketiga jenis selalu ada (nol bila tak ada transaksi).
type GerakanKomoditas struct {
	KomoditasKode string
	KomoditasNama string
	Masuk         GerakanJenis
	Keluar        GerakanJenis
	Pindah        GerakanJenis
}

// Total adalah tonase ketiga jenis; dasar urutan komoditas.
func (g GerakanKomoditas) Total() float64 {
	return g.Masuk.VolumeTon + g.Keluar.VolumeTon + g.Pindah.VolumeTon
}

// RingkasanGerakan adalah total tiap jenis atas seluruh komoditas dalam cakupan dan filter.
type RingkasanGerakan struct {
	Masuk  GerakanJenis
	Keluar GerakanJenis
	Pindah GerakanJenis
}

// Logistik adalah hasil GET /logistik.
type Logistik struct {
	// Dari dan Sampai adalah jendela (DATE waktu Aceh, inklusif).
	Dari, Sampai time.Time
	Ringkasan    RingkasanGerakan
	// Komoditas memuat komoditas yang punya transaksi dalam jendela, tonase terbesar lebih dulu; tidak pernah nil.
	Komoditas []GerakanKomoditas
	// Terbaru adalah paling banyak MaksPergerakanTerbaru transaksi terbaru dalam jendela; tidak pernah nil.
	Terbaru []Pergerakan
	Rute    RuteRingkas
}
