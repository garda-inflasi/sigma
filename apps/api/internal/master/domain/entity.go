package domain

// Provinsi memetakan tabel provinsi.
type Provinsi struct {
	Kode string `gorm:"column:kode;primaryKey"`
	Nama string `gorm:"column:nama"`
}

func (Provinsi) TableName() string {
	return "provinsi"
}

// Komoditas memetakan tabel komoditas.
type Komoditas struct {
	Kode          string `gorm:"column:kode;primaryKey"`
	Nama          string `gorm:"column:nama"`
	Kategori      string `gorm:"column:kategori"`
	Satuan        string `gorm:"column:satuan"`
	HargaAcuan    int    `gorm:"column:harga_acuan"`
	Pilot         bool   `gorm:"column:pilot;default:false"`
	ShelfLifeHari int    `gorm:"column:shelf_life_hari"`
	Versi         int    `gorm:"column:versi;default:1"`
	Status        string `gorm:"column:status;default:AKTIF"`
}

func (Komoditas) TableName() string {
	return "komoditas"
}

// Kabupaten adalah kabupaten/kota pada master wilayah (kode BPS).
type Kabupaten struct {
	Kode         string  `gorm:"column:kode;primaryKey"`
	Nama         string  `gorm:"column:nama"`
	Jenis        string  `gorm:"column:jenis"`
	Ibukota      string  `gorm:"column:ibukota"`
	Lat          float64 `gorm:"column:lat"`
	Lng          float64 `gorm:"column:lng"`
	ProvinsiKode string  `gorm:"column:provinsi_kode"`
}

func (Kabupaten) TableName() string {
	return "kabupaten"
}

// Pasar adalah pasar pada master data, dengan geofence untuk entri lapangan.
type Pasar struct {
	ID            string  `gorm:"column:id;primaryKey"`
	Nama          string  `gorm:"column:nama"`
	KabupatenKode string  `gorm:"column:kabupaten_kode"`
	Lat           float64 `gorm:"column:lat"`
	Lng           float64 `gorm:"column:lng"`
	RadiusMeter   int     `gorm:"column:radius_meter;default:400"`
	Aktif         bool    `gorm:"column:aktif;default:true"`
}

func (Pasar) TableName() string {
	return "pasar"
}

// BulkAction adalah aksi massal pada master komoditas.
type BulkAction string

const (
	BulkAjukanPerubahan BulkAction = "ajukan_perubahan"
	BulkSetujui         BulkAction = "setujui"
	BulkArsipkan        BulkAction = "arsipkan"
)

// Status master komoditas (enum status_master).
const (
	StatusAktif               = "AKTIF"
	StatusMenungguPersetujuan = "MENUNGGU_PERSETUJUAN"
	StatusDiarsipkan          = "DIARSIPKAN"
)

// Izin yang dibutuhkan tiap aksi massal: penyetuju (master.setujui) berbeda dari
// pengaju (master.ajukan) agar pengaju tidak menyetujui usulannya sendiri.
const (
	IzinAjukan  = "master.ajukan"
	IzinSetujui = "master.setujui"
)

// IzinUntuk mengembalikan izin yang dibutuhkan sebuah aksi massal.
func (a BulkAction) IzinUntuk() string {
	if a == BulkSetujui {
		return IzinSetujui
	}
	return IzinAjukan
}

// Transisi adalah perubahan status yang diizinkan sebuah aksi massal.
type Transisi struct {
	// Dari memuat status asal yang sah.
	Dari []string
	Ke   string
	// NaikVersi menaikkan versi saat transisi berlaku (persetujuan perubahan).
	NaikVersi bool
	// Syarat menjelaskan status asal yang sah, untuk pesan item yang ditolak.
	Syarat string
}

// TransisiUntuk mengembalikan aturan transisi aksi; ok false bila aksinya tidak dikenal.
func TransisiUntuk(action BulkAction) (Transisi, bool) {
	switch action {
	case BulkAjukanPerubahan:
		return Transisi{Dari: []string{StatusAktif}, Ke: StatusMenungguPersetujuan, Syarat: "hanya komoditas berstatus Aktif yang dapat diajukan"}, true
	case BulkSetujui:
		return Transisi{Dari: []string{StatusMenungguPersetujuan}, Ke: StatusAktif, NaikVersi: true, Syarat: "hanya komoditas yang menunggu persetujuan yang dapat disetujui"}, true
	case BulkArsipkan:
		return Transisi{Dari: []string{StatusAktif, StatusMenungguPersetujuan}, Ke: StatusDiarsipkan, Syarat: "hanya komoditas Aktif atau menunggu persetujuan yang dapat diarsipkan"}, true
	}
	return Transisi{}, false
}

// Boleh melaporkan apakah item berstatus ini boleh menjalani transisi.
func (t Transisi) Boleh(status string) bool {
	for _, d := range t.Dari {
		if d == status {
			return true
		}
	}
	return false
}

// RincianBulk adalah hasil satu item pada aksi massal.
type RincianBulk struct {
	Kode     string
	Berhasil bool
	// Pesan menjelaskan sebab item gagal; kosong bila berhasil.
	Pesan string
}

// HasilBulk merangkum aksi massal: Berhasil + Gagal = jumlah kode unik.
type HasilBulk struct {
	Berhasil int
	Gagal    int
	Rincian  []RincianBulk
}
