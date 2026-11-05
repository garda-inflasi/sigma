package domain

import "time"

// Status siklus hidup entri harga (enum status_entri).
const (
	StatusDraf        = "DRAF"
	StatusDiajukan    = "DIAJUKAN"
	StatusDivalidasi  = "DIVALIDASI"
	StatusDipublikasi = "DIPUBLIKASI"
	StatusDitolak     = "DITOLAK"
)

// Tipe harga (enum tipe_harga).
const (
	TipeProdusen = "PRODUSEN"
	TipeKonsumen = "KONSUMEN"
)

// StatusValid memeriksa nilai enum status_entri.
func StatusValid(status string) bool {
	switch status {
	case StatusDraf, StatusDiajukan, StatusDivalidasi, StatusDipublikasi, StatusDitolak:
		return true
	}
	return false
}

// EntriHarga memetakan tabel entri_harga.
type EntriHarga struct {
	ID              string     `gorm:"column:id;primaryKey"`
	KomoditasKode   string     `gorm:"column:komoditas_kode"`
	PasarID         string     `gorm:"column:pasar_id"`
	Tipe            string     `gorm:"column:tipe"`
	Harga           int        `gorm:"column:harga"`
	Satuan          string     `gorm:"column:satuan"`
	Kualitas        string     `gorm:"column:kualitas;default:MEDIUM"`
	Sumber          string     `gorm:"column:sumber;default:LAPANGAN"`
	Status          string     `gorm:"column:status;default:DIAJUKAN"`
	DeviasiPct      float64    `gorm:"column:deviasi_pct"`
	SkorAnomali     float64    `gorm:"column:skor_anomali"`
	Lat             *float64   `gorm:"column:lat"`
	Lng             *float64   `gorm:"column:lng"`
	DalamGeofence   bool       `gorm:"column:dalam_geofence"`
	FotoURL         *string    `gorm:"column:foto_url"`
	KodePedagang    *string    `gorm:"column:kode_pedagang"`
	Catatan         *string    `gorm:"column:catatan"`
	IdempotencyKey  *string    `gorm:"column:idempotency_key"`
	DicatatPada     time.Time  `gorm:"column:dicatat_pada"`
	PetugasID       *string    `gorm:"column:petugas_id"`
	DivalidasiOleh  *string    `gorm:"column:divalidasi_oleh"`
	DivalidasiPada  *time.Time `gorm:"column:divalidasi_pada"`
	DipublikasiOleh *string    `gorm:"column:dipublikasi_oleh"`
	DipublikasiPada *time.Time `gorm:"column:dipublikasi_pada"`
	DibuatPada      time.Time  `gorm:"column:dibuat_pada;default:now()"`
	DiupdatePada    time.Time  `gorm:"column:diupdate_pada;default:now()"`
}

func (EntriHarga) TableName() string {
	return "entri_harga"
}

// TugasEntri menautkan entri harga ke tugas lapangan yang dipenuhinya (tabel tugas_entri).
type TugasEntri struct {
	TugasID       string    `gorm:"column:tugas_id;primaryKey"`
	EntriID       string    `gorm:"column:entri_id;primaryKey"`
	DitautkanPada time.Time `gorm:"column:ditautkan_pada;default:now()"`
}

func (TugasEntri) TableName() string {
	return "tugas_entri"
}

// EntriView adalah baris entri_harga beserta nama komoditas, pasar, wilayah,
// dan petugas, siap ditampilkan di daftar dan detail.
type EntriView struct {
	ID              string     `gorm:"column:id"`
	KomoditasKode   string     `gorm:"column:komoditas_kode"`
	KomoditasNama   string     `gorm:"column:komoditas_nama"`
	PasarID         string     `gorm:"column:pasar_id"`
	PasarNama       string     `gorm:"column:pasar_nama"`
	KabupatenKode   string     `gorm:"column:kabupaten_kode"`
	KabupatenNama   string     `gorm:"column:kabupaten_nama"`
	Tipe            string     `gorm:"column:tipe"`
	Harga           int        `gorm:"column:harga"`
	Satuan          string     `gorm:"column:satuan"`
	Kualitas        string     `gorm:"column:kualitas"`
	Sumber          string     `gorm:"column:sumber"`
	Status          string     `gorm:"column:status"`
	DeviasiPct      float64    `gorm:"column:deviasi_pct"`
	SkorAnomali     float64    `gorm:"column:skor_anomali"`
	Lat             *float64   `gorm:"column:lat"`
	Lng             *float64   `gorm:"column:lng"`
	DalamGeofence   bool       `gorm:"column:dalam_geofence"`
	AdaFoto         bool       `gorm:"column:ada_foto"`
	FotoURL         *string    `gorm:"column:foto_url"`
	Catatan         *string    `gorm:"column:catatan"`
	DicatatPada     time.Time  `gorm:"column:dicatat_pada"`
	PetugasID       *string    `gorm:"column:petugas_id"`
	PetugasNama     *string    `gorm:"column:petugas_nama"`
	DivalidasiOleh  *string    `gorm:"column:divalidasi_oleh"`
	DivalidasiPada  *time.Time `gorm:"column:divalidasi_pada"`
	DipublikasiOleh *string    `gorm:"column:dipublikasi_oleh"`
	DipublikasiPada *time.Time `gorm:"column:dipublikasi_pada"`
	DibuatPada      time.Time  `gorm:"column:dibuat_pada"`
	DiupdatePada    time.Time  `gorm:"column:diupdate_pada"`
}

// BulkAction adalah aksi massal pada entri harga.
type BulkAction string

const (
	BulkValidasi   BulkAction = "validasi"
	BulkPublikasi  BulkAction = "publikasi"
	BulkTolak      BulkAction = "tolak"
	BulkKembalikan BulkAction = "kembalikan"
)

// Valid memeriksa nama aksi.
func (a BulkAction) Valid() bool {
	switch a {
	case BulkValidasi, BulkPublikasi, BulkTolak, BulkKembalikan:
		return true
	}
	return false
}
