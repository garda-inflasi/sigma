package domain

import (
	"time"

	"github.com/garda-inflasi/api/pkg/idx"
)

// Provinsi memetakan tabel provinsi.
type Provinsi struct {
	ID   idx.ID `gorm:"column:id;primaryKey;autoIncrement"`
	Kode string `gorm:"column:kode"`
	Nama string `gorm:"column:nama"`
}

func (Provinsi) TableName() string {
	return "provinsi"
}

// Satuan memetakan tabel satuan (katalog satuan komoditas).
type Satuan struct {
	ID   idx.ID `gorm:"column:id;primaryKey;autoIncrement"`
	Kode string `gorm:"column:kode"`
	Nama string `gorm:"column:nama"`
}

func (Satuan) TableName() string {
	return "satuan"
}

// Komoditas memetakan tabel komoditas.
type Komoditas struct {
	ID            idx.ID `gorm:"column:id;primaryKey;autoIncrement"`
	Kode          string `gorm:"column:kode"`
	Nama          string `gorm:"column:nama"`
	Satuan        string `gorm:"column:satuan"`
	Kategori      string `gorm:"column:kategori"`
	HargaAcuan    int    `gorm:"column:harga_acuan"`
	Status        string `gorm:"column:status;default:AKTIF"`
	Versi         int    `gorm:"column:versi;default:1"`
	Pilot         bool   `gorm:"column:pilot;default:false"`
	ShelfLifeHari int    `gorm:"column:shelf_life_hari"`
}

func (Komoditas) TableName() string {
	return "komoditas"
}

// Kabupaten adalah kabupaten/kota pada master wilayah (kode BPS).
type Kabupaten struct {
	ID           idx.ID  `gorm:"column:id;primaryKey;autoIncrement"`
	Kode         string  `gorm:"column:kode"`
	Nama         string  `gorm:"column:nama"`
	ProvinsiKode string  `gorm:"column:provinsi_kode"`
	Jenis        string  `gorm:"column:jenis"`
	Ibukota      string  `gorm:"column:ibukota"`
	Lat          float64 `gorm:"column:lat"`
	Lng          float64 `gorm:"column:lng"`
}

func (Kabupaten) TableName() string {
	return "kabupaten"
}

// Kecamatan memetakan tabel kecamatan (kode BPS 7 digit; diimpor dari BPS, lihat seeder/SUMBER.md).
type Kecamatan struct {
	ID            idx.ID  `gorm:"column:id;primaryKey;autoIncrement"`
	Kode          string  `gorm:"column:kode"`
	Nama          string  `gorm:"column:nama"`
	KabupatenKode string  `gorm:"column:kabupaten_kode"`
	KodeDagri     *string `gorm:"column:kode_dagri"`
}

func (Kecamatan) TableName() string {
	return "kecamatan"
}

// Pasar adalah pasar pada master data, dengan geofence untuk entri lapangan. KecamatanKode opsional;
// Versi naik setiap usulan perubahan pasar disetujui.
type Pasar struct {
	ID            idx.ID  `gorm:"column:id;primaryKey;autoIncrement"`
	Kode          string  `gorm:"column:kode"`
	Nama          string  `gorm:"column:nama"`
	KabupatenKode string  `gorm:"column:kabupaten_kode"`
	KecamatanKode *string `gorm:"column:kecamatan_kode"`
	Lat           float64 `gorm:"column:lat"`
	Lng           float64 `gorm:"column:lng"`
	RadiusMeter   int     `gorm:"column:radius_meter;default:400"`
	Aktif         bool    `gorm:"column:aktif;default:true"`
	Versi         int     `gorm:"column:versi;default:1"`

	// KecamatanNama bukan kolom: repositori mengisinya pada daftar pasar (lihat ListPasar).
	KecamatanNama *string `gorm:"-"`
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

// Pedagang memetakan tabel pedagang: daftar pedagang per pasar. entri_harga.kode_pedagang tetap teks bebas dan
// dicocokkan menurut (PasarID, Kode).
type Pedagang struct {
	ID         idx.ID    `gorm:"column:id;primaryKey;autoIncrement"`
	PasarID    string    `gorm:"column:pasar_id"`
	Kode       string    `gorm:"column:kode"`
	Nama       *string   `gorm:"column:nama"`
	Aktif      bool      `gorm:"column:aktif;default:true"`
	DibuatPada time.Time `gorm:"column:dibuat_pada;default:now()"`
}

func (Pedagang) TableName() string {
	return "pedagang"
}

// KomoditasVersi memetakan tabel komoditas_versi (append-only): salinan definisi komoditas setiap kali isinya
// berubah, ditulis pemicu basis data. Aplikasi hanya membacanya.
type KomoditasVersi struct {
	ID            idx.ID    `gorm:"column:id;primaryKey;autoIncrement"`
	KomoditasKode string    `gorm:"column:komoditas_kode"`
	Versi         int       `gorm:"column:versi"`
	Nama          string    `gorm:"column:nama"`
	Kategori      string    `gorm:"column:kategori"`
	Satuan        string    `gorm:"column:satuan"`
	HargaAcuan    int       `gorm:"column:harga_acuan"`
	ShelfLifeHari int       `gorm:"column:shelf_life_hari"`
	Pilot         bool      `gorm:"column:pilot"`
	BerlakuDari   time.Time `gorm:"column:berlaku_dari"`
	PerubahanID   *idx.ID   `gorm:"column:perubahan_id"`
}

func (KomoditasVersi) TableName() string {
	return "komoditas_versi"
}
