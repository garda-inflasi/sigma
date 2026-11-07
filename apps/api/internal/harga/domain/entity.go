package domain

import (
	"time"

	"github.com/garda-inflasi/api/pkg/idx"
)

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

// EntriHarga memetakan tabel entri_harga. ID adalah BIGINT identity yang diberi basis data; Kode berisi ID lama
// (HRG-D00-00-01-K, atau UUID dari aplikasi lama) sebagai kode tampilan dan alias tautan lama, NULL untuk baris baru.
type EntriHarga struct {
	ID              idx.ID     `gorm:"column:id;primaryKey;autoIncrement"`
	Kode            *string    `gorm:"column:kode"`
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
	PetugasID       *string    `gorm:"column:petugas_id;type:uuid"`
	DivalidasiOleh  *string    `gorm:"column:divalidasi_oleh;type:uuid"`
	DivalidasiPada  *time.Time `gorm:"column:divalidasi_pada"`
	DipublikasiOleh *string    `gorm:"column:dipublikasi_oleh;type:uuid"`
	DipublikasiPada *time.Time `gorm:"column:dipublikasi_pada"`
	// AlasanPenolakan wajib terisi bila Status DITOLAK (dijaga CHECK di basis data).
	AlasanPenolakan *string    `gorm:"column:alasan_penolakan"`
	DitolakOleh     *string    `gorm:"column:ditolak_oleh;type:uuid"`
	DitolakPada     *time.Time `gorm:"column:ditolak_pada"`
	// FlagValidasi adalah hasil cek otomatis (lihat Validasi).
	FlagValidasi FlagValidasi `gorm:"column:flag_validasi;type:jsonb"`
	DibuatPada   time.Time    `gorm:"column:dibuat_pada;default:now()"`
	DiupdatePada time.Time    `gorm:"column:diupdate_pada;default:now()"`
}

func (EntriHarga) TableName() string {
	return "entri_harga"
}

// TugasEntri menautkan entri harga ke tugas lapangan yang dipenuhinya (tabel tugas_entri).
type TugasEntri struct {
	TugasID       idx.ID    `gorm:"column:tugas_id;primaryKey"`
	EntriID       idx.ID    `gorm:"column:entri_id;primaryKey"`
	DitautkanPada time.Time `gorm:"column:ditautkan_pada;default:now()"`
}

func (TugasEntri) TableName() string {
	return "tugas_entri"
}

// EntriView adalah baris entri_harga beserta nama komoditas, pasar, wilayah,
// dan petugas, siap ditampilkan di daftar dan detail.
type EntriView struct {
	ID              idx.ID       `gorm:"column:id"`
	Kode            *string      `gorm:"column:kode"`
	KomoditasKode   string       `gorm:"column:komoditas_kode"`
	KomoditasNama   string       `gorm:"column:komoditas_nama"`
	PasarID         string       `gorm:"column:pasar_id"`
	PasarNama       string       `gorm:"column:pasar_nama"`
	KabupatenKode   string       `gorm:"column:kabupaten_kode"`
	KabupatenNama   string       `gorm:"column:kabupaten_nama"`
	Tipe            string       `gorm:"column:tipe"`
	Harga           int          `gorm:"column:harga"`
	Satuan          string       `gorm:"column:satuan"`
	Kualitas        string       `gorm:"column:kualitas"`
	Sumber          string       `gorm:"column:sumber"`
	Status          string       `gorm:"column:status"`
	DeviasiPct      float64      `gorm:"column:deviasi_pct"`
	SkorAnomali     float64      `gorm:"column:skor_anomali"`
	Lat             *float64     `gorm:"column:lat"`
	Lng             *float64     `gorm:"column:lng"`
	DalamGeofence   bool         `gorm:"column:dalam_geofence"`
	AdaFoto         bool         `gorm:"column:ada_foto"`
	FotoURL         *string      `gorm:"column:foto_url"`
	Catatan         *string      `gorm:"column:catatan"`
	DicatatPada     time.Time    `gorm:"column:dicatat_pada"`
	PetugasID       *string      `gorm:"column:petugas_id;type:uuid"`
	PetugasNama     *string      `gorm:"column:petugas_nama"`
	DivalidasiOleh  *string      `gorm:"column:divalidasi_oleh;type:uuid"`
	DivalidasiPada  *time.Time   `gorm:"column:divalidasi_pada"`
	DipublikasiOleh *string      `gorm:"column:dipublikasi_oleh;type:uuid"`
	DipublikasiPada *time.Time   `gorm:"column:dipublikasi_pada"`
	AlasanPenolakan *string      `gorm:"column:alasan_penolakan"`
	DitolakOleh     *string      `gorm:"column:ditolak_oleh;type:uuid"`
	DitolakPada     *time.Time   `gorm:"column:ditolak_pada"`
	FlagValidasi    FlagValidasi `gorm:"column:flag_validasi;type:jsonb"`
	DibuatPada      time.Time    `gorm:"column:dibuat_pada"`
	DiupdatePada    time.Time    `gorm:"column:diupdate_pada"`
}

// BulkAction adalah aksi massal pada entri harga.
type BulkAction string

const (
	BulkAjukan     BulkAction = "ajukan"
	BulkValidasi   BulkAction = "validasi"
	BulkPublikasi  BulkAction = "publikasi"
	BulkTolak      BulkAction = "tolak"
	BulkKembalikan BulkAction = "kembalikan"
)

// Valid memeriksa nama aksi.
func (a BulkAction) Valid() bool {
	switch a {
	case BulkAjukan, BulkValidasi, BulkPublikasi, BulkTolak, BulkKembalikan:
		return true
	}
	return false
}
